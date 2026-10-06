package workspace

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kaiser-chris/pdx-parser-go/report"

	"github.com/kaiser-chris/pdx-asset-go/mesh/meshtest"
)

// picture is a one pixel PNG, which stands in for a texture.
func picture(t *testing.T) string {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatal(err)
	}

	return encoded.String()
}

// The statue's mesh is defined in a file of its own, as the games often do,
// and drawn by entities of another file and of a DLC.
const (
	statueMeshAsset = `
pdxmesh = {
	name = "statue_mesh"
	file = "statue.mesh"
	meshsettings = {
		name = "quadShape"
		index = 0
		texture_diffuse = "statue_diffuse.png"
		texture_normal = "statue_normal.png"
		shader = "standard"
	}
}
`

	statueEntityAsset = `
entity = { name = "statue_entity" pdxmesh = "statue_mesh" }
entity = { name = "copy_entity" clone = "statue_entity" }
entity = { name = "Another_entity" pdxmesh = "missing_mesh" }
entity = {
	name = "collision_entity"
	pdxmesh = "bare_mesh"
}
pdxmesh = { name = "loose_mesh" file = "loose.mesh" }
pdxmesh = { name = "bare_mesh" file = "statue.mesh" }
`

	dlcAsset = `
entity = { name = "dlc_statue_entity" pdxmesh = "statue_mesh" }
`

	// A plaza is nothing but the statues it attaches, from files of their
	// own, one of them on a pedestal, and one it attaches that is not there.
	plazaAsset = `
entity = {
	name = "plaza_entity"
	locator = { name = "left" position = { -5 0 0 } }
	locator = { name = "right" position = { 5 0 0 } }
	attach = { left = "statue_entity" right = "pedestal_entity" }
	attach = { right = "fountain_entity" }
}
entity = {
	name = "pedestal_entity"
	pdxmesh = "statue_mesh"
	locator = { name = "top" position = { 0 4 0 } }
	attach = { top = "copy_entity" }
}
`
)

// game writes a game folder with the statue and a DLC, and returns it.
func game(t *testing.T) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "Statues", "game")

	tree(t, dir, map[string]string{
		"common/":                                     "",
		"gfx/models/statue/statue_mesh.asset":         statueMeshAsset,
		"gfx/models/statue/statue.mesh":               string(meshtest.QuadFile(2, 4)),
		"gfx/models/statue/statue_diffuse.png":        picture(t),
		"gfx/models/statue/statue_entities.asset":     statueEntityAsset,
		"dlc/dlc001_statues/gfx/models/dlc/dlc.asset": dlcAsset,
		"gfx/models/plaza/plaza.asset":                plazaAsset,
	})

	return dir
}

func TestList(t *testing.T) {
	root := game(t)

	location, err := Locate(filepath.Join(root, "gfx", "models", "statue", "statue_entities.asset"))
	if err != nil {
		t.Fatal(err)
	}

	listing := List(location)

	// In order of name, whatever the case, and only those of the file.
	if want := []string{"Another_entity", "collision_entity", "copy_entity", "statue_entity"}; !slices.Equal(listing.Entities, want) {
		t.Errorf("entities = %v, want %v", listing.Entities, want)
	}

	if want := []string{"bare_mesh", "loose_mesh"}; !slices.Equal(listing.Meshes, want) {
		t.Errorf("meshes = %v, want %v", listing.Meshes, want)
	}

	// The mesh of statue_entity is in another file, which is no problem with
	// this one.
	if len(listing.Diagnostics) != 0 {
		t.Errorf("diagnostics = %v, want none", listing.Diagnostics)
	}
}

func TestListReportsSyntax(t *testing.T) {
	dir := t.TempDir()
	tree(t, dir, map[string]string{"game/gfx/broken.asset": "entity = { name = \"broken_entity\" pdxmesh = \"x\" "})

	location, err := Locate(filepath.Join(dir, "game", "gfx", "broken.asset"))
	if err != nil {
		t.Fatal(err)
	}

	if listing := List(location); len(listing.Diagnostics) == 0 {
		t.Error("an unclosed block should be reported")
	}
}

func TestLoad(t *testing.T) {
	root := game(t)

	opened, err := OpenGame(root)
	if err != nil {
		t.Fatal(err)
	}

	if opened.Name != "Statues" {
		t.Errorf("name = %s, want the install folder's", opened.Name)
	}

	loaded, err := opened.Load("copy_entity")
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded.Model.Parts) != 1 {
		t.Fatalf("parts = %d, want the quad", len(loaded.Model.Parts))
	}

	details := loaded.Details

	if details.Mesh != "statue_mesh" || details.MeshFile != "gfx/models/statue/statue.mesh" {
		t.Errorf("mesh = %s in %s", details.Mesh, details.MeshFile)
	}

	if !slices.Equal(details.Clones, []string{"statue_entity"}) {
		t.Errorf("clones = %v", details.Clones)
	}

	if details.Defined != "gfx/models/statue/statue_entities.asset:3" {
		t.Errorf("defined = %s", details.Defined)
	}

	part := details.Parts[0]
	if part.Name != "quadShape" || part.Shader != "standard" || part.Vertices != 4 || part.Triangles != 2 {
		t.Errorf("part = %+v", part)
	}

	// The normal map is missing, which leaves the part drawn without it.
	if !part.Diffuse || part.Normal || part.Properties {
		t.Errorf("textures found = %+v, want the diffuse alone", part)
	}

	if len(loaded.Diagnostics) != 1 || !strings.Contains(loaded.Diagnostics[0].Message, "statue_normal.png") {
		t.Errorf("diagnostics = %v, want the missing normal map", loaded.Diagnostics)
	}
}

// An entity of nothing but attachments is drawn with them, and its details
// say what hangs where, and which parts are whose.
func TestLoadAttachments(t *testing.T) {
	opened, err := OpenGame(game(t))
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := opened.Load("plaza_entity")
	if err != nil {
		t.Fatal(err)
	}

	details := loaded.Details

	var attached []string
	for _, attachment := range details.Attached {
		attached = append(attached, fmt.Sprintf("%s at %s of %s, missing %v", attachment.Entity, attachment.Locator, attachment.To, attachment.Missing))
	}

	want := []string{
		"statue_entity at left of plaza_entity, missing false",
		"pedestal_entity at right of plaza_entity, missing false",
		"copy_entity at top of pedestal_entity, missing false",
		"fountain_entity at right of plaza_entity, missing true",
	}
	if !slices.Equal(attached, want) {
		t.Errorf("attached = %q, want %q", attached, want)
	}

	if details.Mesh != "" || len(details.PartsOf(0)) != 0 {
		t.Errorf("mesh %q and parts %+v of its own, want none", details.Mesh, details.PartsOf(0))
	}

	if got := details.AttachedTo(0); !slices.Equal(got, []int{1, 2, 4}) {
		t.Errorf("attached to the plaza = %v, want the statue, the pedestal and the fountain", got)
	}

	if got := details.AttachedTo(2); !slices.Equal(got, []int{3}) {
		t.Errorf("attached to the pedestal = %v, want the copy on it", got)
	}

	for number, entity := range map[int]string{1: "statue_entity", 2: "pedestal_entity", 3: "copy_entity"} {
		if parts := details.PartsOf(number); len(parts) != 1 || parts[0].Entity != entity {
			t.Errorf("parts of attachment %d = %+v, want the quad of %s", number, parts, entity)
		}
	}

	if len(loaded.Model.Parts) != 3 {
		t.Errorf("model of %d parts, want the three statues", len(loaded.Model.Parts))
	}

	if !slices.ContainsFunc(loaded.Diagnostics, func(d report.Diagnostic) bool { return strings.Contains(d.Message, "fountain_entity") }) {
		t.Errorf("diagnostics = %v, want the missing fountain", loaded.Diagnostics)
	}
}

// A loose file attaches an entity of an asset file next to it.
func TestOpenLooseAttachesItsNeighbours(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "my_models")
	tree(t, dir, map[string]string{
		"plaza.asset":  plazaAsset,
		"statue.asset": statueMeshAsset + statueEntityAsset,
		"statue.mesh":  string(meshtest.QuadFile(2, 2)),
	})

	location, err := Locate(filepath.Join(dir, "plaza.asset"))
	if err != nil {
		t.Fatal(err)
	}

	game, err := OpenLoose(location)
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := game.Load("plaza_entity")
	if err != nil {
		t.Fatal(err)
	}

	// The statue next to it, the pedestal of its own file, and the copy the
	// pedestal attaches, next to that; the fountain nowhere.
	if len(loaded.Model.Parts) != 2 {
		t.Errorf("model of %d parts, want the statue and the copy; the pedestal's mesh is in another file", len(loaded.Model.Parts))
	}

	missing := 0
	for _, attachment := range loaded.Details.Attached {
		if attachment.Missing {
			missing++

			if attachment.Entity != "fountain_entity" {
				t.Errorf("%s is missing", attachment.Entity)
			}
		}
	}

	if missing != 1 {
		t.Errorf("attached = %+v, want the fountain alone missing", loaded.Details.Attached)
	}
}

// An entity of a DLC draws a mesh of its game, which is found because the
// game is the root of the DLC's files.
func TestLoadFromDLC(t *testing.T) {
	root := game(t)

	location, err := Locate(filepath.Join(root, "dlc", "dlc001_statues", "gfx", "models", "dlc", "dlc.asset"))
	if err != nil {
		t.Fatal(err)
	}

	if listing := List(location); !slices.Equal(listing.Entities, []string{"dlc_statue_entity"}) {
		t.Fatalf("entities = %v", listing.Entities)
	}

	opened, err := OpenGame(location.Root)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := opened.Load("dlc_statue_entity"); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRefuses(t *testing.T) {
	opened, err := OpenGame(game(t))
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"Another_entity", "no_such_entity"} {
		if _, err := opened.Load(name); err == nil {
			t.Errorf("loaded %s, want an error", name)
		}
	}
}

func TestOpenGameRefusesEmptyFolder(t *testing.T) {
	if _, err := OpenGame(t.TempDir()); err == nil {
		t.Error("a folder without game files should be refused")
	}
}

// A shape no mesh settings say how to draw is one the game does not draw,
// such as a collision shape: it is listed, but left out of the model.
func TestLoadLeavesOutShapesWithoutSettings(t *testing.T) {
	opened, err := OpenGame(game(t))
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := opened.Load("collision_entity")
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded.Model.Parts) != 0 {
		t.Errorf("model parts = %d, want the bare shape left out", len(loaded.Model.Parts))
	}

	if parts := loaded.Details.Parts; len(parts) != 1 || parts[0].Drawn {
		t.Errorf("details = %+v, want the shape listed as not drawn", parts)
	}
}

func TestPortrait(t *testing.T) {
	for shader, want := range map[string]bool{
		"portrait_skin":         true,
		"portrait_skin_face":    true,
		"portrait_attachment":   false,
		"snap_to_terrain_atlas": false,
		"":                      false,
	} {
		details := Details{Parts: []PartDetails{{Shader: "standard"}, {Shader: shader}}}
		if got := details.Portrait(); got != want {
			t.Errorf("Portrait with a part drawn with %q = %v, want %v", shader, got, want)
		}
	}
}

// A loose file is read on its own, with what is around it: its entities
// load, one of a missing mesh as nothing.
func TestOpenLoose(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "my_models")
	tree(t, dir, map[string]string{
		"statue.asset": statueMeshAsset + statueEntityAsset,
		"statue.mesh":  string(meshtest.QuadFile(2, 2)),
	})

	location, err := Locate(filepath.Join(dir, "statue.asset"))
	if err != nil {
		t.Fatal(err)
	}

	game, err := OpenLoose(location)
	if err != nil {
		t.Fatal(err)
	}

	if !game.Loose() {
		t.Error("a loose file is taken for a game's")
	}

	statue, err := game.Load("statue_entity")
	if err != nil || len(statue.Model.Parts) != 1 {
		t.Fatalf("statue = %+v, %v", statue, err)
	}

	// The textures it names are not there: the diffuse map shows as the
	// checkerboard, the normal map is left out.
	if textures := statue.Model.Parts[0].Textures; textures.Diffuse == nil || textures.Normal != nil {
		t.Errorf("textures = %+v, want the checkerboard for the diffuse map alone", textures)
	}

	// One whose mesh the file does not define, as a mesh defined in another
	// file, which is not read with it, is drawn as nothing.
	missing, err := game.Load("Another_entity")
	if err != nil || len(missing.Model.Parts) != 0 || len(missing.Diagnostics) == 0 {
		t.Errorf("entity of a mesh no file defines = %+v, %v; want nothing, reported", missing, err)
	}
}
