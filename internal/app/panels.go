package app

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/AllenDang/cimgui-go/imgui"
	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/kaiser-chris/pdx-parser-go/report"

	"github.com/kaiser-chris/pdx-model-viewer/internal/gui"
	"github.com/kaiser-chris/pdx-model-viewer/internal/workspace"
)

// fullWidth is -FLT_MIN, Dear ImGui's way of saying "up to the right edge".
const fullWidth = -math.SmallestNonzeroFloat32

// The labels of widgets the interface tests look for.
const (
	labelViewport = "##viewport"
	labelSearch   = "##search"
)

// The viewport is drawn at twice its size and shown scaled down, which
// smooths the edges of the model the way multisampling would. Above a size of
// picture where that would take more memory than it is worth, it is drawn at
// its own size.
const (
	supersampling       = 2
	supersamplingPixels = 2560 * 1440
)

// How far the camera turns for a pixel dragged across the viewport, in
// radians, and how much closer a notch of the mouse wheel takes it.
const (
	turnPerPixel = 0.008
	zoomPerNotch = 1.15
)

// viewportPanel shows the model, and turns and zooms the camera around it
// with the mouse.
func (a *App) viewportPanel() {
	// The picture fills its panel edge to edge, so it gets no padding.
	imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, imgui.Vec2{})
	open := imgui.BeginV(panelViewport, nil, imgui.WindowFlagsNoScrollbar|imgui.WindowFlagsNoScrollWithMouse)
	imgui.PopStyleVar()

	if open {
		a.viewportBody()
	}

	imgui.End()
}

func (a *App) viewportBody() {
	size := imgui.ContentRegionAvail()
	if size.X < 1 || size.Y < 1 {
		return
	}

	scale := int32(supersampling)
	if size.X*size.Y > supersamplingPixels {
		scale = 1
	}

	a.viewerSize = [2]int32{int32(size.X) * scale, int32(size.Y) * scale}

	origin := imgui.CursorScreenPos()

	if a.shown != nil && len(a.shown.loaded.Model.Parts) > 0 {
		// OpenGL fills a render target bottom up, so the picture is flipped
		// back.
		a.window.Backend().DrawImageRenderTexture(a.viewer.Target(), size, true, false)
		imgui.SetCursorScreenPos(origin)
	} else {
		imgui.WindowDrawList().AddRectFilled(origin, imgui.Vec2{X: origin.X + size.X, Y: origin.Y + size.Y},
			imgui.ColorU32Vec4(colorVec4(viewportBackground)))
	}

	imgui.InvisibleButtonV(labelViewport, size,
		imgui.ButtonFlagsMouseButtonLeft|imgui.ButtonFlagsMouseButtonRight|imgui.ButtonFlagsMouseButtonMiddle)
	gui.Record(labelViewport)

	if a.shown != nil {
		a.steerCamera()
	}

	a.viewportMessage(origin, size)
}

// steerCamera turns the camera around the model while the left button drags
// across the viewport, moves the view along with a drag of the right or the
// middle button, and moves it closer or further with the wheel. A double click
// puts it back.
func (a *App) steerCamera() {
	io := imgui.CurrentIO()
	delta := io.MouseDelta()

	// The interface scale says how many pixels a point of the interface is,
	// which keeps a drag across the same share of the panel turning the model
	// as far on any display.
	perPixel := turnPerPixel / float64(a.window.Scale())

	if imgui.IsItemActive() {
		if imgui.IsMouseDraggingV(imgui.MouseButtonLeft, 0) {
			a.viewer.Rotate(float64(delta.X)*perPixel, float64(delta.Y)*perPixel)
		}

		if imgui.IsMouseDraggingV(imgui.MouseButtonRight, 0) || imgui.IsMouseDraggingV(imgui.MouseButtonMiddle, 0) {
			// The model follows the pointer: what was under it stays under it,
			// at the depth of the point the camera looks at.
			camera := a.viewer.Camera()
			distance := rl.Vector3Distance(camera.Position, camera.Target)
			visible := 2 * distance * float32(math.Tan(float64(camera.Fovy)*math.Pi/360))
			perPoint := visible / imgui.ItemRectSize().Y

			a.panCamera(-delta.X*perPoint, delta.Y*perPoint)
		}
	}

	if !imgui.IsItemHovered() {
		return
	}

	if wheel := io.MouseWheel(); wheel != 0 {
		a.viewer.Zoom(math.Pow(zoomPerNotch, float64(wheel)))
	}

	if imgui.IsMouseDoubleClicked(imgui.MouseButtonLeft) {
		a.resetCamera()
	}
}

// viewportMessage says what the viewport shows, or why it shows nothing.
func (a *App) viewportMessage(origin, size imgui.Vec2) {
	var title, detail string

	switch document := a.document; {
	case a.opening != nil:
		title, detail = "Reading the asset files...", "The game's definitions are read once, which takes a moment."
	case document == nil:
		title, detail = "Open an asset file", "Use File > Open (Ctrl+O), or drop an .asset file from a game's gfx folder onto this window."
	case document.failure != "":
		title, detail = "Could not load "+document.selected, document.failure
	case a.loading != nil:
		title = "Loading " + document.selected + "..."
	case document.selected == "" && len(document.listing.Entities) == 0:
		title, detail = "Nothing to show", filepath.Base(document.location.File)+" defines no entities."
	case document.selected == "":
		title, detail = "Pick an entity", "Choose one of the entities of "+filepath.Base(document.location.File)+" on the left."
	case a.shown != nil && len(a.shown.loaded.Model.Parts) == 0:
		title, detail = "Nothing to draw", "The mesh of "+document.selected+" holds no geometry, only points for other entities to attach to."
	default:
		hint(origin, size, "Drag to turn  |  Right-drag to move  |  Scroll to zoom  |  Double-click to reset")

		return
	}

	centeredMessage(origin, size, title, detail)
}

// centeredMessage writes a heading and a line of explanation in the middle of
// an area, wrapped to a comfortable width.
func centeredMessage(origin, size imgui.Vec2, title, detail string) {
	padding := gui.Scaled(24)
	width := min(size.X-2*padding, gui.Scaled(480))

	if width <= 0 {
		return
	}

	gui.PushStrongFont()
	titleSize := imgui.CalcTextSizeV(title, false, width)
	gui.PopFont()

	height := titleSize.Y
	if detail != "" {
		height += imgui.CurrentStyle().ItemSpacing().Y + imgui.CalcTextSizeV(detail, false, width).Y
	}

	imgui.SetCursorScreenPos(imgui.Vec2{
		X: origin.X + (size.X-width)/2,
		Y: origin.Y + max((size.Y-height)/2, padding),
	})

	imgui.PushTextWrapPosV(imgui.CursorPosX() + width)

	gui.PushStrongFont()
	imgui.TextWrapped(title)
	gui.Record(title)
	gui.PopFont()

	if detail != "" {
		imgui.SetCursorPosX(imgui.CursorPosX() + (size.X-width)/2)
		gui.DimmedText(detail)
	}

	imgui.PopTextWrapPos()
}

// hint writes a line of dimmed text along the bottom of an area, on a band of
// the panels' colour so that it reads over the model too.
func hint(origin, size imgui.Vec2, text string) {
	padding := gui.Scaled(8)
	textSize := imgui.CalcTextSizeV(text, false, -1)
	top := imgui.Vec2{X: origin.X + padding, Y: origin.Y + size.Y - textSize.Y - padding*2}

	drawList := imgui.WindowDrawList()
	drawList.AddRectFilledV(top,
		imgui.Vec2{X: top.X + textSize.X + padding*2, Y: top.Y + textSize.Y + padding},
		imgui.ColorU32ColV(imgui.ColWindowBg, 0.85), gui.Scaled(4), 0)
	drawList.AddTextVec2(imgui.Vec2{X: top.X + padding, Y: top.Y + padding/2},
		imgui.ColorU32Col(imgui.ColTextDisabled), text)
}

// entitiesPanel lists the entities of the open file, to pick the one to view.
func (a *App) entitiesPanel() {
	if !a.showEntities {
		return
	}

	if imgui.BeginV(panelEntities, &a.showEntities, 0) {
		a.entitiesBody()
	}

	imgui.End()
}

func (a *App) entitiesBody() {
	document := a.document
	if document == nil {
		gui.DimmedText("No file open.")

		if gui.Button("Open...") {
			a.askForAssetFile()
		}

		return
	}

	gui.PushStrongFont()
	imgui.TextWrapped(filepath.Base(document.location.File))
	gui.PopFont()
	imgui.SetItemTooltip(document.location.File)

	imgui.TextDisabled(document.game.Name)
	gui.Record(document.game.Name)
	imgui.SetItemTooltip(document.location.Root)

	if document.game.Loose() {
		gui.DimmedText(looseNote)
		imgui.SetItemTooltip("The meshes and textures are looked for around the file and by their names next to it. A missing texture shows as a checkerboard, a missing mesh as nothing.")
	}

	for _, problem := range document.listing.Diagnostics {
		gui.WarningText(fmt.Sprintf("Line %d: %s", problem.Line, problem.Message))
	}

	imgui.Spacing()

	entities := document.listing.Entities
	if len(entities) == 0 {
		a.noEntities(document)

		return
	}

	if len(entities) > 1 {
		imgui.SetNextItemWidth(fullWidth)
		gui.InputText(labelSearch, "Search", &a.search)
	}

	shown := matching(entities, a.search)

	if len(shown) == len(entities) {
		imgui.TextDisabled(plural(len(entities), "entity", "entities"))
	} else {
		imgui.TextDisabled(fmt.Sprintf("%d of %s", len(shown), plural(len(entities), "entity", "entities")))
	}

	if imgui.BeginChildStrV("##entities", imgui.Vec2{}, imgui.ChildFlagsNone, 0) {
		for _, name := range shown {
			selected := name == document.selected

			if gui.Selectable(name, selected, 0) && !selected {
				a.selectEntity(name)
			}
		}

		// The arrow keys step through the list while it has the focus, which
		// is a quick way to look through the variants a file defines.
		if imgui.IsWindowFocused() && !imgui.CurrentIO().WantTextInput() {
			a.stepThrough(shown)
		}
	}

	imgui.EndChild()
}

// stepThrough picks the entity before or after the one picked when an arrow
// key is pressed, and scrolls it into view.
func (a *App) stepThrough(shown []string) {
	step := 0

	switch {
	case imgui.IsKeyPressedBool(imgui.KeyUpArrow):
		step = -1
	case imgui.IsKeyPressedBool(imgui.KeyDownArrow):
		step = 1
	default:
		return
	}

	current := -1

	for index, name := range shown {
		if name == a.document.selected {
			current = index
		}
	}

	next := min(max(current+step, 0), len(shown)-1)
	if next == current || next < 0 {
		return
	}

	a.selectEntity(shown[next])

	// The rows are all one line high, so the row's place is known without
	// laying it out again.
	row := imgui.TextLineHeightWithSpacing()
	imgui.SetScrollYFloat(min(max(imgui.ScrollY(), float32(next+1)*row-imgui.WindowHeight()), float32(next)*row))
}

// noEntities explains a file that has nothing to pick, and lists the meshes
// it defines instead, if any.
func (a *App) noEntities(document *document) {
	gui.DimmedText("This file defines no entities, so there is nothing to show.")

	meshes := document.listing.Meshes
	if len(meshes) == 0 {
		return
	}

	imgui.Spacing()
	gui.DimmedText("It defines these meshes, which entities in other files draw:")

	for _, mesh := range meshes {
		imgui.BulletText(mesh)
		gui.Record(mesh)
	}
}

// matching returns the names that contain a search, ignoring case.
func matching(names []string, search string) []string {
	search = strings.ToLower(strings.TrimSpace(search))
	if search == "" {
		return names
	}

	var found []string

	for _, name := range names {
		if strings.Contains(strings.ToLower(name), search) {
			found = append(found, name)
		}
	}

	return found
}

// detailsPanel shows how the entity on view is put together, what could not
// be read for it, and how it is drawn.
func (a *App) detailsPanel() {
	if !a.showDetails {
		return
	}

	if imgui.BeginV(panelDetails, &a.showDetails, 0) {
		a.detailsBody()
	}

	imgui.End()
}

func (a *App) detailsBody() {
	document := a.document

	switch {
	case document == nil || document.selected == "":
		gui.DimmedText("Pick an entity to see how it is put together.")

		return
	case document.failure != "":
		gui.ErrorText(document.failure)

		return
	case a.shown == nil || a.shown.loaded.Details.Entity != document.selected:
		gui.DimmedText("Loading...")

		return
	}

	loaded := a.shown.loaded
	details := loaded.Details

	imgui.SeparatorText("Entity")

	if beginPropertyTable("##entity") {
		propertyRow("Name", details.Entity)
		propertyRow("Defined in", details.Defined)

		if len(details.Clones) > 0 {
			propertyRow("Clones", strings.Join(details.Clones, ", "))
		}

		propertyRow("Mesh", details.Mesh)
		propertyRow("Mesh file", details.MeshFile)

		imgui.EndTable()
	}

	imgui.SeparatorText("Parts")
	a.partsTable()

	// What the files did not have, and the effects that could not be
	// built, whose parts the viewer's own shader draws.
	if count := len(loaded.Diagnostics) + len(a.shown.model.Problems); count > 0 {
		imgui.SeparatorText(fmt.Sprintf("Problems (%d)", count))

		for _, problem := range loaded.Diagnostics {
			problemText(problem)
		}

		for _, problem := range a.shown.model.Problems {
			gui.WarningText(problem)
			imgui.Spacing()
		}
	}

	imgui.SeparatorText("Display")
	a.displaySettings()
}

func beginPropertyTable(id string) bool {
	if !imgui.BeginTableV(id, 2, imgui.TableFlagsSizingStretchProp, imgui.Vec2{}, 0) {
		return false
	}

	imgui.TableSetupColumnV("name", imgui.TableColumnFlagsWidthFixed, 0, 0)
	imgui.TableSetupColumnV("value", imgui.TableColumnFlagsWidthStretch, 0, 0)

	return true
}

func propertyRow(name, value string) {
	imgui.TableNextRow()
	imgui.TableNextColumn()
	imgui.TextDisabled(name)
	imgui.TableNextColumn()
	imgui.TextWrapped(value)
	gui.Record(value)
}

// partsTable lists the parts of the model: the shapes of its mesh, what they
// are drawn with, and which of their textures were found.
func (a *App) partsTable() {
	parts := a.shown.loaded.Details.Parts
	if len(parts) == 0 {
		gui.DimmedText("The mesh holds no geometry.")

		return
	}

	flags := imgui.TableFlagsSizingFixedFit | imgui.TableFlagsRowBg | imgui.TableFlagsBordersInnerH

	if !imgui.BeginTableV("##parts", 4, flags, imgui.Vec2{}, 0) {
		return
	}

	imgui.TableSetupColumnV("Shape", imgui.TableColumnFlagsWidthStretch, 0, 0)
	imgui.TableSetupColumnV("Shader", 0, 0, 0)
	imgui.TableSetupColumnV("Triangles", 0, 0, 0)
	imgui.TableSetupColumnV("Textures", 0, 0, 0)
	imgui.TableHeadersRow()

	for _, part := range parts {
		imgui.TableNextRow()

		imgui.TableNextColumn()
		imgui.TextUnformatted(part.Name)
		gui.Record(part.Name)

		imgui.TableNextColumn()
		imgui.TextUnformatted(part.Shader)
		gui.Record(part.Shader)

		imgui.TableNextColumn()
		imgui.TextUnformatted(fmt.Sprint(part.Triangles))
		imgui.SetItemTooltip(plural(part.Vertices, "vertex", "vertices"))

		imgui.TableNextColumn()

		if part.Drawn {
			textureMarks(part.Diffuse, part.Normal, part.Properties)
		} else {
			imgui.TextDisabled("not drawn")
			imgui.SetItemTooltip("The shape has no mesh settings, so the game does not draw it: a collision shape, for one")
			gui.Record("not drawn")
		}
	}

	imgui.EndTable()
}

// textureMarks writes D, N and P for the diffuse, normal and properties map,
// dimmed for one that was not found and is drawn with a stand in.
func textureMarks(found ...bool) {
	for index, mark := range []struct{ letter, name string }{
		{"D", "diffuse map"},
		{"N", "normal map"},
		{"P", "properties map"},
	} {
		if index > 0 {
			imgui.SameLine()
		}

		if found[index] {
			imgui.TextUnformatted(mark.letter)
			imgui.SetItemTooltip("The " + mark.name + " was found")
		} else {
			imgui.TextDisabled(mark.letter)
			imgui.SetItemTooltip("No " + mark.name + "; drawn with a neutral stand in")
		}
	}
}

// problemText writes a problem met loading the entity, with where it is.
func problemText(problem report.Diagnostic) {
	if problem.Severity >= report.SeverityError {
		gui.ErrorText(problem.Message)
	} else {
		gui.WarningText(problem.Message)
	}

	if problem.Path != "" {
		place := filepath.Base(problem.Path)
		if problem.Line > 0 {
			place += fmt.Sprintf(":%d", problem.Line)
		}

		imgui.TextDisabled(place)
		imgui.SetItemTooltip(problem.Path)
	}

	imgui.Spacing()
}

// displaySettings are what the model is drawn with beyond its own files.
func (a *App) displaySettings() {
	look := &a.viewer.Look

	a.environmentChoice()

	// The palette colour is the viewer's own shader's; the games' effects
	// take their colours from the asset and the engine.
	if a.shown == nil || !a.shown.model.UsesOwnShader() {
		gui.Checkbox("Turn automatically", &a.turning)

		if gui.Button("Reset Camera") {
			a.resetCamera()
		}

		return
	}

	imgui.SetNextItemWidth(fullWidth)
	if imgui.ColorEdit3V("##palette", &look.PaletteColor, imgui.ColorEditFlagsNoLabel) {
		a.paletteChosen = true
	}
	imgui.SetItemTooltip("The palette colour, such as a skin tone: blended in where the alpha of a part's diffuse map says")

	gui.Checkbox("Turn automatically", &a.turning)

	if gui.Button("Reset Camera") {
		a.resetCamera()
	}
}

// environmentChoice picks the environment the entities are lit with, among
// the game's environment files, as the game's own model editor does. A game
// without any is lit by the built in environment.
func (a *App) environmentChoice() {
	opened := a.document
	if opened == nil {
		return
	}

	imgui.TextDisabled("Environment")
	imgui.SetNextItemWidth(fullWidth)

	if len(opened.environments) == 0 {
		imgui.BeginDisabled()
		if gui.BeginCombo("##environment", builtInEnvironment) {
			imgui.EndCombo()
		}
		imgui.EndDisabled()

		if imgui.IsItemHoveredV(imgui.HoveredFlagsAllowWhenDisabled) {
			imgui.SetTooltip("The game has no environment files; the built in environment lights the models")
		}

		return
	}

	preview := opened.environment
	if preview == workspace.BuiltIn {
		preview = builtInEnvironment
	}

	if gui.BeginCombo("##environment", preview) {
		for _, file := range opened.environments {
			if gui.Selectable(file, file == opened.environment, 0) {
				a.chooseEnvironment(file)
			}
		}

		if gui.Selectable(builtInEnvironment, opened.environment == workspace.BuiltIn, 0) {
			a.chooseEnvironment(workspace.BuiltIn)
		}

		imgui.EndCombo()
	}

	if opened.lightingErr != nil {
		gui.WarningText(opened.lightingErr.Error())
	}

	imgui.Spacing()
}

// builtInEnvironment is how the built in environment is listed.
const builtInEnvironment = "Built in"

// looseNote says what a loose file, one of no game, is drawn with.
const looseNote = "Outside any game: drawn with the viewer's own shader."

// colorVec4 turns a raylib colour into a Dear ImGui one.
func colorVec4(color rl.Color) imgui.Vec4 {
	return imgui.Vec4{X: float32(color.R) / 255, Y: float32(color.G) / 255, Z: float32(color.B) / 255, W: float32(color.A) / 255}
}
