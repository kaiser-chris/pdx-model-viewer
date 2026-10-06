//go:build uitest

package app

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AllenDang/cimgui-go/imgui"
	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/kaiser-chris/pdx-model-viewer/internal/gui"
	"github.com/kaiser-chris/pdx-model-viewer/internal/uitest"
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

// An entity of nothing but attachments shows them, and its details list what
// hangs where, each opening onto its parts. What is not there is marked and
// listed among the problems.
func TestDetailsListTheAttachments(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "plaza.asset"))
	waitForEntity(application, driver, "plaza_entity")

	if parts := len(application.shown.loaded.Model.Parts); parts != 2 {
		t.Errorf("the plaza is drawn with %d parts, want both statues", parts)
	}

	for _, text := range []string{textOnlyAttached, "statue_entity##attached1", "painted_entity##attached2", "fountain_entity##attached3", "at left", "at right"} {
		if !driver.Exists(panelDetails, text) {
			t.Errorf("the details do not show %q", text)
		}
	}

	if !driver.Exists(panelDetails, "entity plaza_entity attaches fountain_entity, which is not defined; drawn without it") {
		t.Error("the missing fountain is not among the problems")
	}

	// The parts of an attachment show once it is opened.
	if driver.Exists(panelDetails, "Quad") {
		t.Error("the parts of the statues show before they are opened")
	}

	driver.Click(panelDetails, "painted_entity##attached2")

	if !driver.Exists(panelDetails, "Quad") {
		t.Error("the painted statue opened without its part")
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

// The About window names the application and its version, says what it is
// and what it is built with, credits the works it uses, and closes again.
func TestAboutWindow(t *testing.T) {
	_, driver := startApp(t)

	driver.Menu("Help", "About")
	driver.Frames(2)

	for _, text := range []string{
		applicationName + " " + applicationVersion,
		aboutDescription,
		"Rendered with raylib, interface built with Dear ImGui.",
	} {
		if !driver.Exists(popupAbout, text) {
			t.Errorf("the About window does not say %q", text)
		}
	}

	for _, credit := range credits {
		if !driver.Exists(popupAbout, credit.work) {
			t.Errorf("the About window does not credit %s", credit.work)
		}
	}

	driver.Click(popupAbout, "Close")
	driver.Frames(2)

	if driver.Exists(popupAbout, "Close") {
		t.Error("the About window is still open after Close")
	}
}

// An entity that can play animations gets a timeline, docked below the
// viewport, whose controls run it and whose range reaches as far as its
// longest animation. An entity with none is shown without the panel.
func TestAnimationTimeline(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "windmill.asset"))
	waitForEntity(application, driver, "windmill_entity")

	animations := application.shown.loaded.Details.Animations
	if len(animations) != 2 {
		t.Fatalf("the windmill can play %+v, want two animations", animations)
	}

	if longest := application.shown.loaded.Details.LongestAnimation(); longest != 4 {
		t.Errorf("its longest animation runs %gs, want 4", longest)
	}

	for _, label := range []string{labelPlay, labelLoop, labelRewind, labelTime, "Animation", textNotMoved} {
		if !driver.Exists(panelAnimation, label) {
			t.Errorf("the timeline does not show %q", label)
		}
	}

	// It starts at the first animation, stopped at its beginning, and loops.
	if application.animation != 0 || application.playing || application.animationTime != 0 {
		t.Errorf("it starts at animation %d, playing %v, at %gs",
			application.animation, application.playing, application.animationTime)
	}

	if !application.looping {
		t.Error("it does not loop to begin with")
	}

	// The choices name each animation and how long it runs.
	driver.Click(panelAnimation, "Animation")

	for _, choice := range []string{"idle_animation  (2.00 s)##animation0", "turning_animation  (4.00 s)##animation1"} {
		if !driver.Exists(uitest.AnyCombo, choice) {
			t.Errorf("the picker does not offer %q", choice)
		}
	}

	driver.Press(imgui.KeyEscape)

	// Playing moves the clock on, and pausing leaves it where it got to.
	driver.Click(panelAnimation, labelPlay)

	if !application.playing {
		t.Fatal("it did not start playing")
	}

	driver.WaitFor("the clock to move on", func() bool { return application.animationTime > 0 })

	driver.Click(panelAnimation, labelPause)

	stopped := application.animationTime
	if application.playing {
		t.Error("it did not pause")
	}

	driver.Frames(3)

	if application.animationTime != stopped {
		t.Errorf("paused at %gs, it moved on to %gs", stopped, application.animationTime)
	}

	// Rewinding puts it back to the start.
	driver.Click(panelAnimation, labelRewind)

	if application.animationTime != 0 {
		t.Errorf("rewound to %gs", application.animationTime)
	}

	// An entity that can play nothing is shown without the panel.
	openFile(t, application, driver, fixtureFile(t, "pedestal.asset"))
	waitForEntity(application, driver, "pedestal_entity")

	if len(application.animations()) != 0 {
		t.Errorf("the statue can play %+v", application.animations())
	}

	if driver.Exists(panelAnimation, labelPlay) {
		t.Error("the timeline shows for an entity that can play nothing")
	}
}

// Picking another animation puts the timeline back to its start, and the
// clock says which frame of it the time falls on.
func TestPickAnAnimation(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "windmill.asset"))
	waitForEntity(application, driver, "windmill_entity")

	application.animationTime = 1
	driver.Frame()

	// A second into a clip of 31 frames made at 15 frames a second is frame
	// fifteen.
	if !driver.Exists(panelAnimation, "1.00 / 2.00 s     frame 15 of 31 at 15 fps") {
		driver.Dump()
		t.Error("the clock does not say where it stands")
	}

	driver.Click(panelAnimation, "Animation")
	driver.Click(uitest.AnyCombo, "turning_animation  (4.00 s)##animation1")

	if application.animation != 1 || application.animationTime != 0 {
		t.Errorf("picked animation %d at %gs, want the second one from its start",
			application.animation, application.animationTime)
	}

	if !driver.Exists(panelAnimation, "0.00 / 4.00 s     frame 0 of 121 at 30 fps") {
		t.Error("the clock does not follow the animation picked")
	}
}

// An animation that loops starts again at its end; one that does not stops
// there.
func TestLoopingAnAnimation(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "windmill.asset"))
	waitForEntity(application, driver, "windmill_entity")

	// Just short of the end of the two second animation, running.
	application.animationTime = 1.99
	application.playing = true

	driver.WaitFor("the loop to come round", func() bool { return application.animationTime < 1.99 })

	if !application.playing {
		t.Error("a looping animation stopped at its end")
	}

	// Without the loop it stops at its end instead.
	driver.Click(panelAnimation, labelLoop)

	if application.looping {
		t.Fatal("the loop did not come off")
	}

	application.animationTime = 1.99
	application.playing = true

	driver.WaitFor("it to stop at the end", func() bool { return !application.playing })

	if application.animationTime != 2 {
		t.Errorf("it stopped at %gs, want the end at 2", application.animationTime)
	}
}

// A layout saved before the animation timeline existed knows nothing of it,
// so it would open floating over the viewport. The next run puts it along the
// bottom of the viewport, where the default layout puts it, and leaves the
// rest of the arrangement alone.
func TestTimelineIsPlacedInAnOlderLayout(t *testing.T) {
	config := t.TempDir()

	windmill := fixtureFile(t, "windmill.asset")

	first, driver := startAppIn(t, config)

	openFile(t, first, driver, windmill)
	waitForEntity(first, driver, "windmill_entity")

	// The panels of the first run are docked, the timeline among them.
	animation, ok := gui.FindWindow(panelAnimation)
	if !ok || animation.DockId() == 0 {
		t.Fatal("the timeline was not docked by the default layout")
	}

	if first.settings.LayoutVersion != layoutVersion {
		t.Errorf("the layout was saved as version %d, want %d", first.settings.LayoutVersion, layoutVersion)
	}

	first.Close()

	// Wind the saved layout back to one made before the timeline existed.
	forgetPanel(t, filepath.Join(config, "layout.ini"), panelAnimation)
	setLayoutVersion(t, filepath.Join(config, "settings.json"), 0)

	second, driver := startAppIn(t, config)

	// Nothing is shown yet, so the viewport is left whole: an entity that can
	// play nothing gets no strip below it.
	driver.Frames(3)

	if _, shown := gui.FindWindow(panelAnimation); shown && second.settings.LayoutVersion >= layoutVersion {
		t.Error("the viewport was split before anything wanted the timeline")
	}

	openFile(t, second, driver, windmill)
	waitForEntity(second, driver, "windmill_entity")

	// The entities and details keep the places the saved layout gives them.
	entities, ok := gui.FindWindow(panelEntities)
	if !ok || entities.DockId() == 0 {
		t.Fatal("the entities panel lost its place")
	}

	kept := entities.DockId()

	driver.WaitFor("the timeline to be placed", func() bool {
		placed, ok := gui.FindWindow(panelAnimation)

		return ok && placed.DockId() != 0
	})

	if second.settings.LayoutVersion != layoutVersion {
		t.Errorf("the layout was brought up to version %d, want %d", second.settings.LayoutVersion, layoutVersion)
	}

	if entities, ok := gui.FindWindow(panelEntities); !ok || entities.DockId() != kept {
		t.Error("placing the timeline moved the entities panel")
	}

	// It sits below the viewport, which is what the user asked for.
	timeline, _ := gui.FindWindow(panelAnimation)
	viewport, _ := gui.FindWindow(panelViewport)

	if timeline.Pos().Y <= viewport.Pos().Y {
		t.Errorf("the timeline is at y %g, the viewport at %g; it should be below it",
			timeline.Pos().Y, viewport.Pos().Y)
	}

	// A run after that leaves it wherever it ended up, rather than placing it
	// again.
	second.Close()

	third, driver := startAppIn(t, config)

	openFile(t, third, driver, windmill)
	waitForEntity(third, driver, "windmill_entity")

	if third.settings.LayoutVersion != layoutVersion {
		t.Errorf("the next run saved version %d", third.settings.LayoutVersion)
	}

	again, ok := gui.FindWindow(panelAnimation)
	if !ok || again.DockId() == 0 {
		t.Error("the timeline did not come back docked")
	}
}

// forgetPanel takes a window's section out of a saved layout, as a layout
// written before that window existed would be.
func forgetPanel(t *testing.T, path, panel string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var kept []string

	dropping := false

	for _, section := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(section, "[") {
			dropping = strings.HasPrefix(section, "[Window]["+panel+"]")
		}

		if !dropping {
			kept = append(kept, section)
		}
	}

	if err := os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
}

// setLayoutVersion rewrites the layout version of the saved settings.
func setLayoutVersion(t *testing.T, path string, version int) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var kept map[string]any
	if err := json.Unmarshal(data, &kept); err != nil {
		t.Fatal(err)
	}

	kept["layoutVersion"] = version

	written, err := json.Marshal(kept)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, written, 0o644); err != nil {
		t.Fatal(err)
	}
}
