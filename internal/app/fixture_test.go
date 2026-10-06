//go:build uitest

package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/kaiser-chris/pdx-asset-go/mesh/meshtest"

	"github.com/kaiser-chris/pdx-model-viewer/internal/uitest"
	"github.com/kaiser-chris/pdx-model-viewer/internal/workspace"
)

// The fixture game is a statue: a quad, drawn by several entities in several
// colours, in the files and folders the games keep such things in.
const (
	statueAsset = `
pdxmesh = {
	name = "statue_mesh"
	file = "statue.mesh"
	meshsettings = {
		name = "quadShape"
		index = 0
		texture_diffuse = "statue_diffuse.png"
		texture_normal = "statue_normal.png"
		texture_specular = "statue_properties.png"
		shader = "standard"
	}
}

entity = { name = "statue_entity" pdxmesh = "statue_mesh" }
entity = {
	name = "painted_entity"
	pdxmesh = "statue_mesh"
	meshsettings = {
		name = "quadShape"
		index = 0
		texture_diffuse = "painted_diffuse.png"
		texture_normal = "statue_normal.png"
		texture_specular = "statue_properties.png"
		shader = "painted"
	}
}
entity = {
	name = "weathered_entity"
	pdxmesh = "statue_mesh"
	meshsettings = { name = "quadShape" index = 0 texture_diffuse = "statue_diffuse.png" texture_normal = "weathered_normal.png" }
}
entity = { name = "broken_entity" pdxmesh = "missing_mesh" }
entity = {
	name = "skin_entity"
	pdxmesh = "statue_mesh"
	meshsettings = { name = "quadShape" index = 0 texture_diffuse = "statue_diffuse.png" shader = "portrait_skin" }
}
`

	// The pedestal's file defines one entity, which is shown at once, and
	// draws the statue's mesh from the file next door.
	pedestalAsset = `entity = { name = "pedestal_entity" pdxmesh = "statue_mesh" }`

	// A file of meshes alone, for entities elsewhere to draw.
	meshesAsset = `pdxmesh = { name = "spare_mesh" file = "statue.mesh" }`

	// A windmill plays two animations, one of two seconds and one of four,
	// so that a timeline reaches as far as the longer.
	windmillAsset = `
pdxmesh = {
	name = "windmill_mesh"
	file = "statue.mesh"
	animation = { id = "idle_animation" type = "windmill_idle.anim" }
	animation = { id = "turning_animation" type = "windmill_turning.anim" }
	meshsettings = {
		name = "quadShape"
		index = 0
		texture_diffuse = "statue_diffuse.png"
		shader = "standard"
	}
}
entity = { name = "windmill_entity" pdxmesh = "windmill_mesh" }
`

	// A plaza is nothing but the statues it attaches, and a fountain that is
	// not there.
	plazaAsset = `
entity = {
	name = "plaza_entity"
	locator = { name = "left" position = { -2 0 0 } }
	locator = { name = "right" position = { 2 0 0 } }
	attach = { left = "statue_entity" right = "painted_entity" }
	attach = { right = "fountain_entity" }
}
`

	// A skinned quad plays one animation that moves its bone, so that the
	// model can be seen to move.
	skinnedAsset = `
pdxmesh = {
	name = "skinned_mesh"
	file = "skinned.mesh"
	animation = { id = "moved_animation" type = "skinned_moved.anim" }
	meshsettings = {
		name = "skinnedShape"
		index = 0
		texture_diffuse = "statue_diffuse.png"
		shader = "standard"
	}
}
entity = { name = "skinned_entity" pdxmesh = "skinned_mesh" }
`

	// A flock is the same skinned entity attached twice, so its animation is
	// listed once with a copy of its own for each attachment.
	flockAsset = `
entity = {
	name = "flock_entity"
	locator = { name = "left" position = { -2 0 0 } }
	locator = { name = "right" position = { 2 0 0 } }
	attach = { left = "skinned_entity" right = "skinned_entity" }
}
`

	// A belt is a portrait accessory: its game data names the mask that says
	// where each pattern goes, and the variation that says what they are.
	beltAsset = `
pdxmesh = {
	name = "belt_mesh"
	file = "belt.mesh"
	meshsettings = {
		name = "quadShape"
		index = 0
		texture_diffuse = "statue_diffuse.png"
		shader = "portrait_attachment_pattern"
	}
}

entity = {
	name = "belt_entity"
	pdxmesh = "belt_mesh"
	game_data = {
		portrait_entity_user_data = {
			portrait_accessory = {
				pattern_mask = "gfx/models/portraits/belt/belt_masks.png"
				variation = "fixture_belt"
			}
		}
	}
}
`

	// The same accessory on a mesh drawn by an effect that lays no pattern,
	// which the game does not colour with it either.
	plainBeltAsset = `
pdxmesh = {
	name = "plain_belt_mesh"
	file = "belt.mesh"
	meshsettings = {
		name = "quadShape"
		index = 0
		texture_diffuse = "statue_diffuse.png"
		shader = "standard"
	}
}

entity = {
	name = "plain_belt_entity"
	pdxmesh = "plain_belt_mesh"
	game_data = {
		portrait_entity_user_data = {
			portrait_accessory = {
				pattern_mask = "gfx/models/portraits/belt/belt_masks.png"
				variation = "fixture_belt"
			}
		}
	}
}
`

	// The variation of the fixture belt: two whole ways of patterning it and
	// two of colouring it, which is what the viewer offers to pick between.
	beltVariation = `
pattern_textures = { name = "silk" colormask = "gfx/portraits/accessory_variations/textures/fixture_masks.png" }
pattern_textures = { name = "trim" colormask = "gfx/portraits/accessory_variations/textures/fixture_masks.png" }
pattern_layout = { name = "plain_layout" scale = 1 rotation = 0 offset = { x = 0 y = 0 } }

variation = {
	name = "fixture_belt"

	pattern = {
		weight = 1
		r = { textures = "silk" layout = "plain_layout" }
		g = { textures = "silk" layout = "plain_layout" }
		b = { textures = "silk" layout = "plain_layout" }
		a = { textures = "silk" layout = "plain_layout" }
	}

	pattern = {
		weight = 1
		r = { textures = "trim" layout = "plain_layout" }
		g = { textures = "trim" layout = "plain_layout" }
		b = { textures = "trim" layout = "plain_layout" }
		a = { textures = "trim" layout = "plain_layout" }
	}

	color_palette = { weight = 1 texture = "gfx/portraits/accessory_variations/textures/fixture_red.png" }
	color_palette = { weight = 1 texture = "gfx/portraits/accessory_variations/textures/fixture_blue.png" }
}
`
)

// A mod draws the statue's mesh, which only the game it is a mod of defines:
// an entity of it loads when the mod was read together with its game, and not
// when it was read on its own.
const modAsset = `entity = { name = "mod_entity" pdxmesh = "statue_mesh" }`

// The descriptions the fixture mods carry, which are what tells the viewer
// which game each is for: a classifier, none at all, and a descriptor.mod.
const (
	namedModMetadata     = `{ "name": "Named Mod", "id": "com.github.test.named", "game_id": "victoria3" }`
	namelessModMetadata  = `{ "name": "Nameless Mod", "id": "com.github.test.nameless", "version": "1.0" }`
	ambiguousModMetadata = `{ "name": "Ambiguous Mod", "id": "com.github.test.ambiguous", "version": "1.0" }`
	oldModDescriptor     = "name=\"Old Mod\"\nversion=\"1.0\"\nsupported_version=\"1.19.*\"\n"
)

var (
	fixtureRed  = color.NRGBA{R: 220, G: 30, B: 30, A: 255}
	fixtureBlue = color.NRGBA{R: 30, G: 30, B: 220, A: 255}

	// fixtureWhite covers everything and colours nothing, which is what a
	// pattern mask and a pattern of a plain accessory do.
	fixtureWhite = color.NRGBA{R: 255, G: 255, B: 255, A: 255}

	// A flat normal and a plain, rough material.
	fixtureNormal     = color.NRGBA{R: 128, G: 128, B: 255, A: 255}
	fixtureProperties = color.NRGBA{R: 0, G: 64, B: 0, A: 255}
)

// outsideAsset is an asset file kept outside the game, with its mesh next
// to it, named by the path it has in the game, and its diffuse map missing.
const outsideAsset = `
pdxmesh = {
	name = "outside_mesh"
	file = "gfx/models/outside/outside.mesh"
	meshsettings = {
		name = "quadShape"
		index = 0
		texture_diffuse = "outside_diffuse.dds"
		shader = "standard"
	}
}
entity = { name = "outside_entity" pdxmesh = "outside_mesh" }
`

// fixture is the game, written once for every test of the package, which
// only reads it.
var fixture struct {
	once sync.Once
	root string
}

// fixtureGame returns the game folder of the fixture.
func fixtureGame(t *testing.T) string {
	t.Helper()

	fixture.once.Do(func() {
		// Not t.TempDir: that would go with the first test, and the fixture
		// is shared by all of them.
		dir, err := os.MkdirTemp("", "pdx-model-viewer-fixture-")
		if err != nil {
			t.Fatal(err)
		}

		root := filepath.Join(dir, "Statues", "game")
		statue := "gfx/models/statue/"

		files := map[string][]byte{
			"common/README.txt":              nil,
			statue + "statue.asset":          []byte(statueAsset),
			statue + "statue.mesh":           meshtest.QuadFile(2, 2),
			statue + "statue_diffuse.png":    solid(t, fixtureRed),
			statue + "painted_diffuse.png":   solid(t, fixtureBlue),
			statue + "statue_normal.png":     solid(t, fixtureNormal),
			statue + "statue_properties.png": solid(t, fixtureProperties),
			statue + "pedestal.asset":        []byte(pedestalAsset),
			statue + "meshes.asset":          []byte(meshesAsset),
			statue + "plaza.asset":           []byte(plazaAsset),
			statue + "windmill.asset":        []byte(windmillAsset),
			statue + "skinned.asset":         []byte(skinnedAsset),
			statue + "skinned.mesh":          skinnedQuadFile(),
			statue + "skinned_moved.anim":    movedAnimationFile(),
			statue + "flock.asset":           []byte(flockAsset),
			statue + "belt.asset":            []byte(beltAsset),
			statue + "plain_belt.asset":      []byte(plainBeltAsset),
			statue + "belt.mesh":             meshtest.QuadFile(2, 2),

			// The variation of the belt, with the mask that says where each
			// of its patterns goes and the palettes it draws them in. The
			// pattern covers everything, as the plain silk of most of the
			// shipped variations does.
			"gfx/portraits/accessory_variations/fixture.txt":                []byte(beltVariation),
			"gfx/models/portraits/belt/belt_masks.png":                      solid(t, fixtureWhite),
			"gfx/portraits/accessory_variations/textures/fixture_masks.png": solid(t, fixtureWhite),
			"gfx/portraits/accessory_variations/textures/fixture_red.png":   solid(t, fixtureRed),
			"gfx/portraits/accessory_variations/textures/fixture_blue.png":  solid(t, fixtureBlue),

			// Two seconds of a clip made at fifteen frames a second, and
			// four of one made at thirty.
			statue + "windmill_idle.anim":    animationFile(15.5, 31),
			statue + "windmill_turning.anim": animationFile(30.25, 121),
			"../loose/outside.asset":         []byte(outsideAsset),
			"../loose/outside.mesh":          meshtest.QuadFile(2, 2),

			// A second installation, so that a mod can be read against the
			// wrong game as well as the right one.
			"../Kingdoms/game/common/README.txt": nil,

			// The mods: one that names its game, one that says nothing, one
			// that only has the descriptor Crusader Kings 3 writes, and one
			// that carries both descriptions and names no game.
			"../mods/named/.metadata/metadata.json":     []byte(namedModMetadata),
			"../mods/named/gfx/models/statue/mod.asset": []byte(modAsset),

			"../mods/nameless/.metadata/metadata.json":     []byte(namelessModMetadata),
			"../mods/nameless/gfx/models/statue/mod.asset": []byte(modAsset),

			"../mods/old/descriptor.mod":              []byte(oldModDescriptor),
			"../mods/old/gfx/models/statue/mod.asset": []byte(modAsset),

			"../mods/ambiguous/.metadata/metadata.json":     []byte(ambiguousModMetadata),
			"../mods/ambiguous/descriptor.mod":              []byte(oldModDescriptor),
			"../mods/ambiguous/gfx/models/statue/mod.asset": []byte(modAsset),
		}

		for name, content := range files {
			writeFile(t, filepath.Join(root, filepath.FromSlash(name)), content)
		}

		fixture.root = root
	})

	return fixture.root
}

// animationFile writes an .anim file of one joint that turns over the given
// frames, at the given rate, the way the games write theirs. The length of an
// animation is its frames over its rate, so 31 frames at 15.5 is two seconds.
func animationFile(fps float32, frames int) []byte {
	writer := meshtest.New()

	writer.Object(1, "info").
		Floats("fps", fps).
		Ints("sa", int32(frames)).
		Ints("j", 1)

	writer.Object(2, "quadShape:root").
		Strings("sa", "q").
		Floats("t", 0, 0, 0).
		Floats("q", 0, 0, 0, -1).
		Floats("s", 1)

	writer.Object(1, "samples")

	turns := make([]float32, 0, frames*4)
	for range frames {
		turns = append(turns, 0, 0, 0, 1)
	}

	writer.Floats("q", turns...)

	return writer.Bytes()
}

// skinnedQuadFile writes a .mesh of one quad skinned to a single bone at the
// origin, which an animation can move.
func skinnedQuadFile() []byte {
	quad := meshtest.Quad(2, 2)

	writer := meshtest.New().Object(1, "object").Object(2, "skinnedShape").Object(3, "mesh")

	writer.Floats("p", quad.Positions...).
		Floats("n", quad.Normals...).
		Floats("ta", quad.Tangents...).
		Floats("u0", quad.UV0...).
		Ints("tri", quad.Indices...)

	// The skin: one bone, the same one for every vertex.
	writer.Object(4, "skin").
		Ints("bones", 1).
		Ints("ix", 0, -1, -1, -1, 0, -1, -1, -1, 0, -1, -1, -1, 0, -1, -1, -1).
		Floats("w", 1, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0)

	writer.Object(4, "aabb").Floats("min", -1, -1, 0).Floats("max", 1, 1, 0)
	writer.Object(4, "material").Strings("shader", "standard").Strings("diff", "statue_diffuse.png")

	// The skeleton: one bone at the origin.
	writer.Object(3, "skeleton").
		Object(4, "root").
		Ints("ix", 0).
		Ints("pa", -1).
		Floats("tx", 1, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0)

	return writer.Bytes()
}

// movedAnimationFile writes an .anim file that moves its one bone a long way
// along x in the second frame, so the skinned quad leaves the picture.
func movedAnimationFile() []byte {
	writer := meshtest.New()

	writer.Object(1, "info").
		Floats("fps", 10).
		Ints("sa", 2).
		Ints("j", 1)

	writer.Object(2, "root").
		Strings("sa", "t").
		Floats("t", 0, 0, 0).
		Floats("q", 0, 0, 0, -1).
		Floats("s", 1)

	writer.Object(1, "samples").
		Floats("t", 0, 0, 0, 100, 0, 0)

	return writer.Bytes()
}

// fixtureFile is the path of a file of the fixture's statue folder.
func fixtureFile(t *testing.T, name string) string {
	t.Helper()

	return filepath.Join(fixtureGame(t), "gfx", "models", "statue", name)
}

// fixtureModFile is the path of a file of one of the fixture's mods, by the
// name of the mod's folder.
func fixtureModFile(t *testing.T, mod, name string) string {
	t.Helper()

	return filepath.Join(fixtureMod(t, mod), "gfx", "models", "statue", name)
}

// fixtureMod is the root of one of the fixture's mods.
func fixtureMod(t *testing.T, mod string) string {
	t.Helper()

	// The mods sit beside the game folder of the installation the fixture
	// game is in, the way a mod folder sits beside a game's own.
	return filepath.Join(filepath.Dir(fixtureGame(t)), "mods", mod)
}

// fixtureInstallations are the games a test pretends are on this machine: the
// fixture game as Victoria 3, and a second installation as Crusader Kings 3,
// which nothing but the mods of the fixture is in.
//
// A test never sees the installations of the machine it runs on, so what it
// tests does not depend on which games are bought and installed there.
func fixtureInstallations(t *testing.T) []workspace.Installation {
	t.Helper()

	game := fixtureGame(t)
	other := filepath.Join(filepath.Dir(game), "Kingdoms")

	return []workspace.Installation{
		{Product: workspace.Victoria3, Install: filepath.Dir(game), Root: game},
		{Product: workspace.CrusaderKings3, Install: other, Root: filepath.Join(other, "game")},
	}
}

// fixtureRow is the chooser's row for one of the games, reading the way the
// chooser reads it for the installations a test gave the application.
func fixtureRow(installations []workspace.Installation, product workspace.Product) string {
	for _, install := range installations {
		if install.Product == product {
			return install.String()
		}
	}

	return product.String() + textNotFound
}

// solid is a PNG of one colour, which stands in for a texture.
func solid(t *testing.T, fill color.NRGBA) []byte {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := range 4 {
		for x := range 4 {
			img.SetNRGBA(x, y, fill)
		}
	}

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatal(err)
	}

	return encoded.Bytes()
}

// fakeDialogs answers the file dialog with paths a test hands it, and cancels
// once it has none left.
type fakeDialogs struct {
	mu       sync.Mutex
	answers  []string
	folders  []string
	games    []string
	askedFor []string
}

func (f *fakeDialogs) chooseAssetFile(folder string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.folders = append(f.folders, folder)

	if len(f.answers) == 0 {
		return "", nil
	}

	answer := f.answers[0]
	f.answers = f.answers[1:]

	return answer, nil
}

// chooseGameFolder answers with the folders a test hands it, in order, and
// cancels once it has none left.
func (f *fakeDialogs) chooseGameFolder() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.games) == 0 {
		return "", nil
	}

	answer := f.games[0]
	f.games = f.games[1:]

	f.askedFor = append(f.askedFor, answer)

	return answer, nil
}

// answerGameFolders hands the folder dialog the folders a test wants picked,
// in order.
func (f *fakeDialogs) answerGameFolders(folders ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.games = append(f.games, folders...)
}

// gameFoldersAsked is how many times the folder dialog for a game was
// answered, which is how many times it was shown.
func (f *fakeDialogs) gameFoldersAsked() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.askedFor)
}

func (f *fakeDialogs) chooseExportFile(proposed string) (string, error) {
	return f.chooseAssetFile(proposed)
}

func (f *fakeDialogs) chooseExportFolder(folder string) (string, error) {
	return f.chooseAssetFile(folder)
}

func (f *fakeDialogs) asked() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.folders...)
}

// startApp starts the real application in a hidden window, with its layout
// kept in a folder of the test's own, and attaches a driver to it.
func startApp(t *testing.T) (*App, *uitest.Driver) {
	t.Helper()

	return startAppIn(t, t.TempDir())
}

// startAppIn is startApp with the settings kept in a folder of the test's
// choosing, which a second run started after closing the first reads.
func startAppIn(t *testing.T, config string) (*App, *uitest.Driver) {
	t.Helper()

	// raylib and OpenGL belong to the thread that created the window, and a
	// test runs on a goroutine the scheduler may move between threads.
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)

	// A fixed scale, so that what the tests see does not depend on the
	// display of the machine running them.
	application, err := New(Options{ConfigDir: config, Hidden: true, Scale: 1})
	if err != nil {
		t.Fatalf("start the application: %v", err)
	}
	t.Cleanup(application.Close)

	// A test never gets to see a real dialog: this one cancels whatever it is
	// asked, until a test hands it answers.
	application.dialogs = &fakeDialogs{}
	application.installations = fixtureInstallations(t)

	return application, uitest.New(t, application)
}

// openFile opens an asset file the way the file association does, and runs
// frames until it is open.
func openFile(t *testing.T, application *App, driver *uitest.Driver, path string) {
	t.Helper()

	application.Open(path)
	waitForFile(application, driver)
}

func waitForFile(application *App, driver *uitest.Driver) {
	driver.WaitFor("the file to be opened", func() bool {
		return application.opening == nil && application.document != nil
	})
}

// waitForEntity runs frames until an entity is on the GPU and has been drawn.
func waitForEntity(application *App, driver *uitest.Driver, name string) {
	driver.WaitFor("entity "+name+" to be shown", func() bool {
		return application.loading == nil && application.shown != nil &&
			application.shown.loaded.Details.Entity == name
	})

	// One more frame, so that the picture is drawn in the viewport's size.
	driver.Frames(2)
}

// centre reads the pixel in the middle of the viewport's picture.
func centre(application *App) color.RGBA {
	picture := application.viewer.Image()
	bounds := picture.Bounds()

	return picture.RGBAAt(bounds.Dx()/2, bounds.Dy()/2)
}

func writeFile(t *testing.T, path string, contents []byte) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
