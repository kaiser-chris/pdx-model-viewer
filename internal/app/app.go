// Package app is the application shell: the window, its panels and the state
// they share.
//
// The interface is built with Dear ImGui through internal/gui. The model is
// drawn by pdx-asset-go's renderer into an offscreen picture that the viewport
// panel shows. Panels are described from scratch every frame, so the whole
// interface is a function of the state in this package.
package app

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"slices"

	"github.com/AllenDang/cimgui-go/imgui"
	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/kaiser-chris/pdx-asset-go/environment"
	"github.com/kaiser-chris/pdx-asset-go/render"
	"github.com/kaiser-chris/pdx-asset-go/shader"
	"github.com/kaiser-chris/pdx-asset-go/texture"

	"github.com/kaiser-chris/pdx-model-viewer/assets"
	"github.com/kaiser-chris/pdx-model-viewer/internal/gui"
	"github.com/kaiser-chris/pdx-model-viewer/internal/workspace"
)

const (
	applicationName = "PDX Model Viewer"

	// configFolder is the folder below the user's configuration directory
	// the layout of the panels is kept in.
	configFolder = "pdx-model-viewer"

	defaultWindowWidth  = 1280
	defaultWindowHeight = 800

	minimumWindowWidth  = 640
	minimumWindowHeight = 400
)

// App owns the window and the state its panels read and write.
type App struct {
	window *gui.Window

	renderer *render.Renderer
	viewer   *render.Viewer

	// viewerSize is the size the viewport panel wants its picture in. The
	// picture is resized to it at the start of the next frame, before it is
	// drawn: resizing it while the interface is being built would delete the
	// texture the interface has already been told to show.
	viewerSize [2]int32

	// framed is the box the model on view fits in, and pan how far the point
	// the camera looks at has been moved away from its middle.
	framed [2][3]float32
	pan    [3]float32

	// games are the games and mods whose files have been read, by their root,
	// so that opening another file of the same game does not read it again.
	games map[string]*workspace.Game

	// environments are the environments picked for the games, by their
	// root, kept for the next file of the same game.
	environments map[string]string

	// document is the asset file open now, and opening the one being opened.
	document *document
	opening  *job[*document]

	// shown is the entity on the GPU, and loading the one being built, for
	// which file.
	shown      *shownEntity
	loading    *job[*workspace.Loaded]
	loadingFor *document

	// reselect is the entity to pick once the file being opened is, when a
	// file is opened again.
	reselect string

	// dialogs asks the user for a file, and dialog is the one open now.
	dialogs fileDialogs
	dialog  *pendingDialog

	// input feeds extra input into Dear ImGui each frame, and scale fixes the
	// interface scale. Only the interface tests set them.
	input func()
	scale float32

	// Interface state.
	search      string
	status      string
	openError   string
	layoutBuilt bool
	showAbout   bool

	showEntities bool
	showDetails  bool
	turning      bool

	// paletteChosen is set once the user picks a palette colour, which is
	// kept from then on rather than chosen for each entity.
	paletteChosen bool

	// shaderSource is what the renderer reads the games' shader files from
	// now.
	shaderSource shader.Source

	// dockWindowClass is handed to the dock space every frame. cimgui-go
	// dereferences that argument even when it is nil, so one default instance
	// is allocated for the lifetime of the application instead.
	dockWindowClass *imgui.WindowClass
}

// document is an asset file that has been opened: where it is, what it
// defines, and the game it belongs to.
type document struct {
	location workspace.Location
	listing  workspace.Listing
	game     *workspace.Game

	// environments are the environment files of the game the entities can
	// be lit with, and environment the one they are lit with, or
	// workspace.BuiltIn.
	environments []string
	environment  string

	// lighting is the environment the file's entities are lit with, and
	// lightingErr why there is none, or not all of it.
	lighting    *workspace.Lighting
	lightingErr error

	// selected is the entity picked from the file, and failure why it could
	// not be shown.
	selected string
	failure  string
}

// shownEntity is an entity uploaded to the GPU, and what was learned loading
// it.
type shownEntity struct {
	model  *render.Model
	loaded *workspace.Loaded
}

// Options changes how the application starts. The zero value is what a user
// gets; the interface tests fill it in.
type Options struct {
	// ConfigDir holds the saved layout of the panels. Empty means the
	// user's configuration directory.
	ConfigDir string

	// Hidden runs the application without showing its window.
	Hidden bool

	// Scale fixes the interface scale. Zero follows the monitor.
	Scale float32
}

// Run starts the application with an asset file, or none for an empty
// window, and returns when its window has been closed.
func Run(file string) error {
	application, err := New(Options{})
	if err != nil {
		return err
	}

	if file != "" {
		application.Open(file)
	}

	application.window.Run(application.frameSpec())

	return nil
}

// New creates the window and what the models are drawn with. It must be
// called from the main goroutine.
func New(options Options) (*App, error) {
	layout := layoutPath(options.ConfigDir)

	application := &App{
		games:        map[string]*workspace.Game{},
		environments: map[string]string{},
		showEntities: true,
		showDetails:  true,
		layoutBuilt:  fileExists(layout),
		status:       "Open an asset file to view its entities",
		scale:        options.Scale,
	}

	icon, err := png.Decode(bytes.NewReader(assets.Icon))
	if err != nil {
		warn(fmt.Errorf("decode the application icon: %w", err))
	}

	application.window = gui.NewWindow(gui.Config{
		Title:      applicationName,
		Width:      defaultWindowWidth,
		Height:     defaultWindowHeight,
		MinWidth:   minimumWindowWidth,
		MinHeight:  minimumWindowHeight,
		LayoutFile: layout,
		Icon:       icon,
		Hidden:     options.Hidden,
	})

	application.window.SizeForScale(defaultWindowWidth, defaultWindowHeight, application.interfaceScale())
	application.dialogs = systemDialogs{parent: dialogParent()}

	// Everything below needs the OpenGL context the window just created.
	renderer, err := render.NewRenderer()
	if err != nil {
		application.window.Close()

		// The shader is bundled with pdx-asset-go, so a failure here is a
		// graphics driver that cannot run it rather than anything in a file.
		return nil, fmt.Errorf("prepare the model renderer: %w", err)
	}

	application.renderer = renderer
	application.viewer = renderer.NewViewer(1, 1)
	application.viewer.Background = viewportBackground
	application.dockWindowClass = imgui.NewWindowClass()

	application.window.OnShutdown(func() {
		application.unloadShown()
		application.viewer.Unload()
		application.renderer.Unload()
		application.dockWindowClass.Destroy()
	})

	return application, nil
}

// Step runs one frame. The interface tests advance the application with it.
func (a *App) Step() {
	a.window.Step(a.frameSpec())
}

// Close shuts the application down and closes its window.
func (a *App) Close() {
	a.window.Close()
}

// SetInput installs a function that feeds input into Dear ImGui every frame,
// after the platform input and before the interface is built.
func (a *App) SetInput(input func()) {
	a.input = input
}

func (a *App) frameSpec() gui.Frame {
	return gui.Frame{
		Offscreen: a.drawOffscreen,
		Input:     a.input,
		UI:        a.frame,
	}
}

// interfaceScale is the scale the interface is drawn at: the monitor's,
// unless it was fixed.
func (a *App) interfaceScale() float32 {
	if a.scale > 0 {
		return a.scale
	}

	return gui.MonitorScale()
}

// viewportBackground is the colour behind the model: a shade lighter than the
// panels, so that a dark model still stands out against it.
var viewportBackground = rl.Color{R: 0x2B, G: 0x30, B: 0x38, A: 0xFF}

// drawOffscreen draws the model into the viewport's picture, which the
// interface shows further down the same frame.
func (a *App) drawOffscreen() {
	if size := a.viewerSize; size[0] > 0 && size[1] > 0 {
		a.viewer.Resize(size[0], size[1])
	}

	if a.shown == nil {
		return
	}

	if a.turning {
		a.viewer.Rotate(float64(rl.GetFrameTime())*turnSpeed, 0)
	}

	a.viewer.Draw(a.shown.model)
}

// turnSpeed is how fast the model turns by itself, in radians a second.
const turnSpeed = 0.6

// frame builds one frame of the interface.
func (a *App) frame() {
	a.pollDialog()
	a.pollOpening()
	a.pollLoading()
	a.pollDroppedFiles()

	// Checked every frame so that the interface follows the window from one
	// monitor to another. Nothing happens unless the scale actually changes.
	a.window.SetScale(a.interfaceScale())

	// The menu bar and the status bar claim their strip of the viewport
	// first, so that the dock space covers exactly what is left.
	a.menuBar()
	a.statusBar()
	a.dockSpace()

	a.viewportPanel()
	a.entitiesPanel()
	a.detailsPanel()

	a.errorPopup()
	a.aboutPopup()

	a.handleShortcuts()
}

// Open starts opening an asset file: working out its game, listing what it
// defines and reading the game's definitions, which takes a moment, so it
// happens off the drawing thread. A file whose game has been read already
// skips that part.
func (a *App) Open(file string) {
	location, err := workspace.Locate(file)
	if err != nil {
		a.openError = err.Error()
		a.setStatus("Could not open %s", filepath.Base(file))

		return
	}

	// The game is looked up here rather than in the job, since the jobs must
	// not touch the application's state.
	known := a.games[location.Key()]
	picked, wasPicked := a.environments[location.Root]

	a.opening = start(func() (*document, error) {
		opened := &document{location: location, listing: workspace.List(location), game: known}

		if opened.game == nil {
			open := func() (*workspace.Game, error) { return workspace.OpenGame(location.Root) }
			if location.Loose {
				open = func() (*workspace.Game, error) { return workspace.OpenLoose(location) }
			}

			game, err := open()
			if err != nil {
				return nil, err
			}

			opened.game = game
		}

		// The environment picked for the game before, if it is still
		// there, or the game's own, or the built in one without any.
		opened.environments = opened.game.Environments(location)
		opened.environment = workspace.BuiltIn

		switch {
		case wasPicked && (picked == workspace.BuiltIn || slices.Contains(opened.environments, picked)):
			opened.environment = picked
		case len(opened.environments) > 0:
			opened.environment = opened.environments[0]
		}

		opened.lighting, opened.lightingErr = opened.game.Lighting(location, opened.environment)

		return opened, nil
	})

	if known == nil {
		a.setStatus("Reading the asset files of %s", workspace.SourceName(location.Root))
	} else {
		a.setStatus("Opening %s", filepath.Base(location.File))
	}
}

// pollOpening shows the file that was being opened, once it is.
func (a *App) pollOpening() {
	if a.opening == nil {
		return
	}

	opened, err, done := a.opening.poll()
	if !done {
		return
	}

	a.opening = nil

	if err != nil {
		a.openError = err.Error()
		a.setStatus("Could not open the file")

		return
	}

	a.games[opened.location.Key()] = opened.game
	a.document = opened
	a.search = ""
	a.unloadShown()

	// The game's own shaders draw the models, from the shader files of the
	// game and the layer the file is in. Switching releases the effects of
	// the last, which no model on the GPU uses any longer.
	if source := opened.game.ShaderSource(opened.location); source != a.shaderSource {
		a.renderer.UseShaders(source)
		a.shaderSource = source
	}

	a.useLighting(opened)

	rl.SetWindowTitle(filepath.Base(opened.location.File) + " - " + applicationName)

	entities := opened.listing.Entities

	reselect := a.reselect
	a.reselect = ""

	switch {
	case reselect != "" && slices.Contains(entities, reselect):
		a.selectEntity(reselect)
	case len(entities) == 0:
		a.setStatus("%s defines no entities", filepath.Base(opened.location.File))
	case len(entities) == 1:
		// Nothing to choose between, so the one there is is shown at once.
		a.selectEntity(entities[0])
	default:
		a.setStatus("%s defines %d entities; pick one to view it", filepath.Base(opened.location.File), len(entities))
	}
}

// useLighting lights the games' effects with the environment of the file's
// game. Without one, it is the library's defaults; without its map, the
// effects keep their basic defaults for it. The problem goes to the standard
// error.
func (a *App) useLighting(opened *document) {
	if opened.lightingErr != nil {
		warn(opened.lightingErr)
	}

	var (
		lighting *environment.Environment
		cube     *texture.Cube
	)

	if opened.lighting != nil {
		lighting, cube = opened.lighting.Environment, opened.lighting.Map
	}

	if err := a.renderer.SetEnvironment(lighting, cube); err != nil {
		warn(err)
		_ = a.renderer.SetEnvironment(lighting, nil)
	}
}

// chooseEnvironment lights the open file's entities with another of the
// game's environments, or the built in one.
func (a *App) chooseEnvironment(file string) {
	opened := a.document
	if opened == nil || file == opened.environment {
		return
	}

	opened.environment = file
	opened.lighting, opened.lightingErr = opened.game.Lighting(opened.location, file)
	a.environments[opened.location.Root] = file

	a.useLighting(opened)

	if file == workspace.BuiltIn {
		a.setStatus("Lit by the built in environment")
	} else {
		a.setStatus("Lit by %s", file)
	}
}

// reload reads the game of the open file again, for files that changed since
// it was read, and shows the same entity again.
func (a *App) reload() {
	if a.document == nil || a.opening != nil {
		return
	}

	delete(a.games, a.document.location.Key())

	// The entity picked is picked again once the file is open, if it is still
	// there.
	a.reselect = a.document.selected
	a.Open(a.document.location.File)
}

// selectEntity picks an entity of the open file and starts building it.
// Picking another while one is being built starts that one as soon as the
// first is done, and drops the first.
func (a *App) selectEntity(name string) {
	if a.document == nil {
		return
	}

	a.document.selected = name
	a.document.failure = ""

	if a.loading != nil {
		return
	}

	game := a.document.game

	a.loadingFor = a.document
	a.loading = start(func() (*workspace.Loaded, error) {
		return game.Load(name)
	})

	a.setStatus("Loading %s", name)
}

// pollLoading uploads the entity that was being built, once it is.
func (a *App) pollLoading() {
	if a.loading == nil {
		return
	}

	loaded, err, done := a.loading.poll()
	if !done {
		return
	}

	a.loading = nil

	// Another file was opened meanwhile, which has started its own load if
	// it wanted one.
	if a.document == nil || a.document != a.loadingFor {
		if a.document != nil && a.document.selected != "" {
			a.selectEntity(a.document.selected)
		}

		return
	}

	wanted := a.document.selected

	// Another entity was picked meanwhile: this one is no longer wanted.
	if loaded != nil && loaded.Details.Entity != wanted {
		a.selectEntity(wanted)

		return
	}

	if err != nil {
		a.unloadShown()
		a.document.failure = err.Error()
		a.setStatus("Could not load %s", wanted)

		return
	}

	uploaded, err := a.renderer.Upload(loaded.Model)
	if err != nil {
		a.unloadShown()
		a.document.failure = err.Error()
		a.setStatus("Could not upload %s to the graphics card", wanted)

		return
	}

	a.unloadShown()
	a.shown = &shownEntity{model: uploaded, loaded: loaded}

	if !a.paletteChosen {
		a.viewer.Look.PaletteColor = paletteFor(loaded.Details)
	}

	a.framed = [2][3]float32{uploaded.Min, uploaded.Max}
	a.resetCamera()

	message := fmt.Sprintf("Loaded %s", wanted)
	if count := len(loaded.Diagnostics) + len(uploaded.Problems); count > 0 {
		message += ", " + plural(count, "problem", "problems")
	}

	a.setStatus("%s", message)
}

// pollDroppedFiles opens an asset file dropped onto the window.
func (a *App) pollDroppedFiles() {
	if !rl.IsFileDropped() {
		return
	}

	dropped := rl.LoadDroppedFiles()
	rl.UnloadDroppedFiles()

	if len(dropped) > 0 {
		a.Open(dropped[0])
	}
}

// unloadShown releases the entity on the GPU, if any.
func (a *App) unloadShown() {
	if a.shown != nil {
		a.shown.model.Unload()
		a.shown = nil
	}
}

// setStatus replaces the message shown in the status bar.
func (a *App) setStatus(format string, args ...any) {
	a.status = fmt.Sprintf(format, args...)
}

// layoutPath is where Dear ImGui keeps the layout of the panels, or nothing
// when there is no configuration directory to keep it in.
func layoutPath(dir string) string {
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			warn(err)

			return ""
		}

		dir = filepath.Join(base, configFolder)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		warn(err)

		return ""
	}

	return filepath.Join(dir, "layout.ini")
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}

	_, err := os.Stat(path)

	return err == nil
}

func plural(count int, one, many string) string {
	if count == 1 {
		return "1 " + one
	}

	return fmt.Sprintf("%d %s", count, many)
}

// warn reports a non fatal problem. The application keeps running.
func warn(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "pdx-model-viewer: %v\n", err)
}

// The palette colours the viewer picks: a skin tone for a portrait's body, and
// white, which leaves every colour as it is, for everything else. The games
// multiply the palette colour in where a diffuse map's alpha says, which on a
// building is anything but skin.
var (
	skinTone = render.DefaultLook.PaletteColor
	noTint   = [3]float32{1, 1, 1}
)

func paletteFor(details workspace.Details) [3]float32 {
	if details.Portrait() {
		return skinTone
	}

	return noTint
}

// defaultPitch is how far the camera looks down on a model to begin with, in
// radians: a little, so that the top of a building shows as well as its
// front, which gives the eye something to read its shape by.
const defaultPitch = 0.25

// resetCamera turns the camera back to the front of the model, looking down
// on its middle a little, at a distance that shows all of it.
func (a *App) resetCamera() {
	a.pan = [3]float32{}
	a.frameModel()

	a.viewer.Reset()
	a.viewer.Rotate(0, defaultPitch)
}

// panCamera moves the point the camera looks at across the view, by a
// distance in the plane of the screen: right and up, in the units of the
// model.
func (a *App) panCamera(right, up float32) {
	camera := a.viewer.Camera()

	forward := rl.Vector3Normalize(rl.Vector3Subtract(camera.Target, camera.Position))
	screenRight := rl.Vector3Normalize(rl.Vector3CrossProduct(forward, camera.Up))
	screenUp := rl.Vector3CrossProduct(screenRight, forward)

	moved := rl.Vector3Add(rl.Vector3Scale(screenRight, right), rl.Vector3Scale(screenUp, up))

	a.pan = [3]float32{a.pan[0] + moved.X, a.pan[1] + moved.Y, a.pan[2] + moved.Z}
	a.frameModel()
}

// frameModel points the camera at the middle of the model, moved by the pan.
//
// The viewer looks at the middle of the box it frames and keeps its distance
// in proportion to the box's size, so framing the same box moved by the pan
// moves the point it looks at and nothing else.
func (a *App) frameModel() {
	low, high := a.framed[0], a.framed[1]

	for axis := range 3 {
		low[axis] += a.pan[axis]
		high[axis] += a.pan[axis]
	}

	a.viewer.Frame(low, high)
}
