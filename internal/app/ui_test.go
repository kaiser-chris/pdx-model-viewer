//go:build uitest

package app

import (
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AllenDang/cimgui-go/imgui"
	rl "github.com/gen2brain/raylib-go/raylib"
)

// The windows the entity list and the error popup are laid out in. The list
// scrolls in a child window of the panel, named after it.
const (
	windowEntityList = panelEntities + "/"
)

func TestStartsEmpty(t *testing.T) {
	application, driver := startApp(t)

	if !driver.Exists(panelViewport, "Open an asset file") {
		t.Error("the viewport does not say how to open a file")
	}

	if !driver.Exists(panelEntities, "Open...") {
		t.Error("the entities panel has no way to open a file")
	}

	if application.shown != nil || application.document != nil {
		t.Error("something is open before anything was opened")
	}

	if outside := driver.OffScreen(); len(outside) > 0 {
		t.Errorf("widgets reach past the window: %v", outside)
	}
}

// A file of one entity has nothing to choose between, so it is shown at once.
func TestOpensTheOnlyEntityAtOnce(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "pedestal.asset"))
	waitForEntity(application, driver, "pedestal_entity")

	if !driver.Exists(panelDetails, "statue_mesh") || !driver.Exists(panelDetails, "gfx/models/statue/statue.mesh") {
		t.Error("the details do not name the mesh the pedestal draws from the file next door")
	}

	if !driver.Exists(panelEntities, "Statues") {
		t.Error("the entities panel does not name the game the file belongs to")
	}

	if got := centre(application); !redder(got) {
		t.Errorf("centre of the viewport = %v, want the statue's red", got)
	}
}

func TestPickAnEntity(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "statue.asset"))

	// Several entities, and none is picked for the user.
	if !driver.Exists(panelViewport, "Pick an entity") {
		t.Error("the viewport does not ask for an entity to be picked")
	}

	if application.loading != nil || application.shown != nil {
		t.Error("an entity was loaded before one was picked")
	}

	for _, name := range []string{"broken_entity", "painted_entity", "skin_entity", "statue_entity", "weathered_entity"} {
		if !driver.Exists(windowEntityList, name) {
			t.Errorf("%s is not listed", name)
		}
	}

	driver.Click(windowEntityList, "statue_entity")
	waitForEntity(application, driver, "statue_entity")

	red := centre(application)

	driver.Click(windowEntityList, "painted_entity")
	waitForEntity(application, driver, "painted_entity")

	blue := centre(application)

	// The painted statue draws the same mesh with a blue texture of its own.
	if !(blue.B > red.B+40 && red.R > blue.R+40) {
		t.Errorf("statue = %v, painted statue = %v; want the one red and the other blue", red, blue)
	}

	if !driver.Exists(panelDetails, "Quad") {
		t.Error("the details do not list the painted statue's part")
	}
}

// The details describe each part in words: its name without the Shape Maya
// adds, how it is drawn, and the texture maps that are missing.
func TestDetailsDescribeTheParts(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "statue.asset"))
	driver.Click(windowEntityList, "skin_entity")
	waitForEntity(application, driver, "skin_entity")

	for _, text := range []string{"Quad", "Solid, tinted with the palette colour", "No normal", "No properties"} {
		if !driver.Exists(panelDetails, text) {
			t.Errorf("the details do not say %q", text)
		}
	}

	if driver.Exists(panelDetails, "No diffuse") {
		t.Error("the details say the diffuse map is missing, which was found")
	}
}

func TestSearchNarrowsTheList(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "statue.asset"))
	driver.Fill(driver.Find(panelEntities, labelSearch), "PAINT")

	if !driver.Exists(windowEntityList, "painted_entity") {
		t.Error("painted_entity disappeared although it matches the search")
	}

	if driver.Exists(windowEntityList, "statue_entity") {
		t.Error("statue_entity is still listed after searching for paint")
	}
}

func TestArrowKeysStepThroughTheList(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "statue.asset"))

	// The list is in order of name: broken, painted, skin, statue, weathered.
	driver.Click(windowEntityList, "painted_entity")
	driver.Press(imgui.KeyDownArrow)

	if got := application.document.selected; got != "skin_entity" {
		t.Errorf("after the down arrow, %s is picked, want skin_entity", got)
	}

	driver.Press(imgui.KeyUpArrow)
	driver.Press(imgui.KeyUpArrow)
	driver.Press(imgui.KeyUpArrow)

	if got := application.document.selected; got != "broken_entity" {
		t.Errorf("after going up past the top, %s is picked, want the first, broken_entity", got)
	}
}

func TestEntityThatCannotBeLoadedSaysWhy(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "statue.asset"))
	driver.Click(windowEntityList, "broken_entity")

	driver.WaitFor("the load to fail", func() bool { return application.document.failure != "" })
	driver.Frame()

	if !driver.Exists(panelViewport, "Could not load broken_entity") {
		t.Error("the viewport does not say the entity could not be loaded")
	}

	if !driver.Exists(panelDetails, application.document.failure) {
		t.Errorf("the details do not say why: %s", application.document.failure)
	}

	if application.shown != nil {
		t.Error("a model is still shown for an entity that failed to load")
	}

	// Picking another entity puts things right.
	driver.Click(windowEntityList, "statue_entity")
	waitForEntity(application, driver, "statue_entity")

	if application.document.failure != "" {
		t.Errorf("the failure %q outlived picking another entity", application.document.failure)
	}
}

func TestMissingTexturesAreReported(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "statue.asset"))
	driver.Click(windowEntityList, "weathered_entity")
	waitForEntity(application, driver, "weathered_entity")

	found := false

	for _, item := range driver.Items() {
		if item.Window == panelDetails && strings.Contains(item.Label, "weathered_normal.png") {
			found = true
		}
	}

	if !found {
		driver.Dump()
		t.Error("the missing normal map is not among the problems in the details")
	}

	// Drawn all the same, with a stand in for the normal map.
	if got := centre(application); !redder(got) {
		t.Errorf("centre of the viewport = %v, want the statue's red", got)
	}
}

// A file outside any game is drawn with what is around it: its mesh found
// by its name next to it, its missing diffuse map shown as the checkerboard
// of a missing texture.
func TestFileOutsideAGameIsDrawn(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, filepath.Join(filepath.Dir(fixtureGame(t)), "loose", "outside.asset"))
	waitForEntity(application, driver, "outside_entity")

	if application.openError != "" || !application.document.location.Loose {
		t.Fatalf("opened %+v, error %q; want the loose file", application.document.location, application.openError)
	}

	if !driver.Exists(panelEntities, looseNote) {
		t.Error("nothing says the file is outside any game")
	}

	// Magenta or black, wherever the centre falls on the checkerboard.
	picture := application.viewer.Image()
	magenta, black := 0, 0

	for y := 0; y < picture.Bounds().Dy(); y += 4 {
		for x := 0; x < picture.Bounds().Dx(); x += 4 {
			pixel := picture.RGBAAt(x, y)

			switch {
			case pixel.A == 0:
			case int(pixel.R) > 2*int(pixel.G)+20 && int(pixel.B) > 2*int(pixel.G)+20:
				magenta++
			case pixel.R < 30 && pixel.G < 30 && pixel.B < 30:
				black++
			}
		}
	}

	if magenta < 20 || black < 20 {
		t.Errorf("magenta %d, black %d pixels; want the checkerboard", magenta, black)
	}
}

func TestFileWithoutEntitiesListsItsMeshes(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "meshes.asset"))
	driver.Frame()

	if !driver.Exists(panelViewport, "Nothing to show") {
		t.Error("the viewport does not say there is nothing to show")
	}

	if !driver.Exists(panelEntities, "spare_mesh") {
		t.Error("the meshes of the file are not listed")
	}
}

func TestOpenThroughTheMenu(t *testing.T) {
	application, driver := startApp(t)

	dialogs := &fakeDialogs{answers: []string{fixtureFile(t, "statue.asset")}}
	application.dialogs = dialogs

	driver.Menu("File", "Open...")
	waitForFile(application, driver)

	if got := filepath.Base(application.document.location.File); got != "statue.asset" {
		t.Fatalf("open file = %s, want the one the dialog answered", got)
	}

	// The next dialog starts in the folder of the file open now, and Ctrl+O
	// opens it too.
	driver.Shortcut(imgui.ModCtrl, imgui.KeyO)
	driver.WaitFor("the dialog to be answered", func() bool { return application.dialog == nil && len(dialogs.asked()) == 2 })

	if asked := dialogs.asked(); asked[1] != filepath.Dir(fixtureFile(t, "statue.asset")) {
		t.Errorf("the second dialog started in %q, want the open file's folder", asked[1])
	}
}

// Opening another file of a game whose files have been read does not read
// them again.
func TestAnotherFileOfTheSameGame(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "statue.asset"))
	game := application.document.game

	openFile(t, application, driver, fixtureFile(t, "pedestal.asset"))

	if application.document.game != game {
		t.Error("the game was read again for a file of the same game")
	}

	waitForEntity(application, driver, "pedestal_entity")
}

func TestReloadKeepsTheEntity(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "statue.asset"))
	driver.Click(windowEntityList, "painted_entity")
	waitForEntity(application, driver, "painted_entity")

	game := application.document.game

	driver.Press(imgui.KeyF5)
	waitForFile(application, driver)
	waitForEntity(application, driver, "painted_entity")

	if application.document.game == game {
		t.Error("reloading did not read the game again")
	}
}

func TestDraggingTurnsTheCamera(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "pedestal.asset"))
	waitForEntity(application, driver, "pedestal_entity")

	yaw, pitch := application.viewer.Yaw, application.viewer.Pitch

	driver.Drag(driver.Find(panelViewport, labelViewport), 120, -60)

	if application.viewer.Yaw <= yaw {
		t.Errorf("yaw = %.3f after dragging right, want more than %.3f", application.viewer.Yaw, yaw)
	}

	if application.viewer.Pitch >= pitch {
		t.Errorf("pitch = %.3f after dragging up, want less than %.3f", application.viewer.Pitch, pitch)
	}

	// Home puts the camera back.
	driver.Press(imgui.KeyHome)

	if application.viewer.Yaw != 0 || application.viewer.Pitch != defaultPitch {
		t.Errorf("after Home the camera is at yaw %.3f, pitch %.3f", application.viewer.Yaw, application.viewer.Pitch)
	}
}

// Dragging with the right button moves the view along with the pointer
// rather than turning it or moving closer.
func TestRightDragMovesTheView(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "pedestal.asset"))
	waitForEntity(application, driver, "pedestal_entity")

	before := application.viewer.Camera()
	yaw, pitch, distance := application.viewer.Yaw, application.viewer.Pitch, application.viewer.Distance

	// Dragged right and down, the model follows: the camera's target moves
	// to the left of the screen and up.
	driver.DragWith(1, driver.Find(panelViewport, labelViewport), 80, 40)

	after := application.viewer.Camera()

	if application.viewer.Yaw != yaw || application.viewer.Pitch != pitch || application.viewer.Distance != distance {
		t.Error("a right-drag turned the camera or moved it closer")
	}

	forward := rl.Vector3Normalize(rl.Vector3Subtract(before.Target, before.Position))
	screenRight := rl.Vector3Normalize(rl.Vector3CrossProduct(forward, before.Up))
	screenUp := rl.Vector3CrossProduct(screenRight, forward)
	moved := rl.Vector3Subtract(after.Target, before.Target)

	if rightward := rl.Vector3DotProduct(moved, screenRight); rightward >= 0 {
		t.Errorf("the target moved %.3f to the right, want to the left of the screen", rightward)
	}

	if upward := rl.Vector3DotProduct(moved, screenUp); upward <= 0 {
		t.Errorf("the target moved %.3f up, want up the screen", upward)
	}

	// The camera moved with its target, keeping its view of the model.
	beforeDistance := rl.Vector3Distance(before.Position, before.Target)
	if got := rl.Vector3Distance(after.Position, after.Target); math.Abs(float64(got-beforeDistance)) > 1e-4 {
		t.Errorf("the camera is %.4f from its target after the drag, want %.4f as before", got, beforeDistance)
	}

	// Home puts it back on the middle of the model.
	driver.Press(imgui.KeyHome)

	if got := application.viewer.Camera().Target; rl.Vector3Distance(got, before.Target) > 1e-5 {
		t.Errorf("after Home the camera looks at %v, want the middle of the model %v", got, before.Target)
	}
}

func TestPanelsCanBeClosedAndReopened(t *testing.T) {
	application, driver := startApp(t)

	driver.Menu("View", panelDetails)

	if application.showDetails || driver.Exists(panelDetails, "") {
		t.Error("the details panel is still open")
	}

	driver.Menu("View", panelDetails)

	if !application.showDetails {
		t.Error("the details panel did not come back")
	}
}

// The palette colour is a skin tone for a portrait's skin and leaves other
// models as they are, until the user picks one, which is kept.
func TestPaletteFollowsTheEntity(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "statue.asset"))

	driver.Click(windowEntityList, "skin_entity")
	waitForEntity(application, driver, "skin_entity")

	if got := application.viewer.Look.PaletteColor; got != skinTone {
		t.Errorf("palette for a portrait's skin = %v, want the skin tone", got)
	}

	driver.Click(windowEntityList, "statue_entity")
	waitForEntity(application, driver, "statue_entity")

	if got := application.viewer.Look.PaletteColor; got != noTint {
		t.Errorf("palette for a statue = %v, want none", got)
	}

	// Picking one by hand, here as the colour picker would.
	application.paletteChosen = true
	application.viewer.Look.PaletteColor = [3]float32{0.2, 0.4, 0.6}

	driver.Click(windowEntityList, "skin_entity")
	waitForEntity(application, driver, "skin_entity")

	if got := application.viewer.Look.PaletteColor; got != [3]float32{0.2, 0.4, 0.6} {
		t.Errorf("palette after picking one = %v, want the one picked", got)
	}
}

// redder reports whether a colour is clearly red rather than grey or blue.
func redder(c interface{ RGBA() (r, g, b, a uint32) }) bool {
	r, g, b, _ := c.RGBA()

	return r > g+0x2000 && r > b+0x2000
}

// The palette colour is offered only for an entity it shows on, named the
// skin tone on a portrait.
func TestPaletteIsOfferedWhereItShows(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "statue.asset"))

	driver.Click(windowEntityList, "skin_entity")
	waitForEntity(application, driver, "skin_entity")

	if !driver.Exists(panelDetails, labelSkinTone) {
		t.Error("a portrait's skin offers no skin tone")
	}

	driver.Click(windowEntityList, "statue_entity")
	waitForEntity(application, driver, "statue_entity")

	if driver.Exists(panelDetails, labelSkinTone) || driver.Exists(panelDetails, labelPalette) {
		t.Error("a statue offers a palette colour, which does not show on it")
	}
}
