package workspace

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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

// The environments are the environment files of the folder the game's own is
// in, that one first; other files of the folder are not environments.
func TestEnvironments(t *testing.T) {
	root := game(t)

	tree(t, root, map[string]string{
		"paths.settings":                            `gfx_environment_file = "gfx/map/environment/environment.txt"`,
		"gfx/map/environment/environment.txt":       "sun_intensity = 5",
		"gfx/map/environment/a_ui_environment.txt":  "sun_intensity = 3\ncubemap_intensity = 1",
		"gfx/map/environment/daynight_settings.txt": "day_length = 10",
	})

	location, err := Locate(filepath.Join(root, "gfx", "models", "statue", "statue_entities.asset"))
	if err != nil {
		t.Fatal(err)
	}

	opened, err := OpenGame(location.Root)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"gfx/map/environment/environment.txt", "gfx/map/environment/a_ui_environment.txt"}
	if got := opened.Environments(location); !slices.Equal(got, want) {
		t.Errorf("environments = %v, want %v", got, want)
	}

	lighting, err := opened.Lighting(location, "gfx/map/environment/a_ui_environment.txt")
	if err != nil || lighting.Environment.Constants["SunIntensity"][0] != 3 {
		t.Errorf("lighting = %+v, %v, want the picked environment", lighting, err)
	}

	// It names no environment map, so it has none.
	if lighting.Map != nil {
		t.Errorf("environment map = %+v, want none", lighting.Map)
	}
}

// A game without environment files lists none, and its entities are lit by
// the built in environment.
func TestBuiltInEnvironment(t *testing.T) {
	location, err := Locate(filepath.Join(game(t), "gfx", "models", "statue", "statue_entities.asset"))
	if err != nil {
		t.Fatal(err)
	}

	opened, err := OpenGame(location.Root)
	if err != nil {
		t.Fatal(err)
	}

	if got := opened.Environments(location); len(got) != 0 {
		t.Errorf("environments = %v, want none", got)
	}

	lighting, err := opened.Lighting(location, BuiltIn)
	if err != nil || lighting.Environment == nil || lighting.Environment.Path != "" || len(lighting.Environment.Constants) == 0 {
		t.Errorf("lighting = %+v, %v, want the built in environment", lighting, err)
	}

	// With an environment map of its own, rather than none, which once lit
	// everything as if by a white sky.
	if lighting.Map == nil || lighting.Map.Size != 1 {
		t.Errorf("environment map = %+v, want the built in one", lighting.Map)
	}
}
