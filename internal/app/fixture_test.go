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
)

var (
	fixtureRed  = color.NRGBA{R: 220, G: 30, B: 30, A: 255}
	fixtureBlue = color.NRGBA{R: 30, G: 30, B: 220, A: 255}

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
			"../loose/outside.asset":         []byte(outsideAsset),
			"../loose/outside.mesh":          meshtest.QuadFile(2, 2),

			// Two environments to light the statue with, and a file of the
			// same folder that is none.
			"paths.settings":                               []byte(`gfx_environment_file = "gfx/map/environment/environment.txt"`),
			"gfx/map/environment/environment.txt":          []byte("sun_intensity = 5"),
			"gfx/map/environment/portrait_environment.txt": []byte("sun_intensity = 3"),
			"gfx/map/environment/daynight_settings.txt":    []byte("day_length = 10"),
		}

		for name, content := range files {
			writeFile(t, filepath.Join(root, filepath.FromSlash(name)), content)
		}

		fixture.root = root
	})

	return fixture.root
}

// fixtureFile is the path of a file of the fixture's statue folder.
func fixtureFile(t *testing.T, name string) string {
	t.Helper()

	return filepath.Join(fixtureGame(t), "gfx", "models", "statue", name)
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
	mu      sync.Mutex
	answers []string
	folders []string
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

func (f *fakeDialogs) asked() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.folders...)
}

// startApp starts the real application in a hidden window, with its layout
// kept in a folder of the test's own, and attaches a driver to it.
func startApp(t *testing.T) (*App, *uitest.Driver) {
	t.Helper()

	// raylib and OpenGL belong to the thread that created the window, and a
	// test runs on a goroutine the scheduler may move between threads.
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)

	// A fixed scale, so that what the tests see does not depend on the
	// display of the machine running them.
	application, err := New(Options{ConfigDir: t.TempDir(), Hidden: true, Scale: 1})
	if err != nil {
		t.Fatalf("start the application: %v", err)
	}
	t.Cleanup(application.Close)

	// A test never gets to see a real dialog: this one cancels whatever it is
	// asked, until a test hands it answers.
	application.dialogs = &fakeDialogs{}

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
