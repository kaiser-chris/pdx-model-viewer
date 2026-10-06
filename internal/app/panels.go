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
		title, detail = "Pick an entity", "Choose one of the entities of "+filepath.Base(document.location.File)+" in the Entities panel."
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
	gui.TextWrapped(title)
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
	gui.TextWrapped(filepath.Base(document.location.File))
	gui.PopFont()
	gui.Tooltip(document.location.File)

	gui.TextDisabled(document.game.Name)
	gui.Record(document.game.Name)
	gui.Tooltip(document.location.Root)

	// A mod is named the way it names itself, which does not say which game
	// it is a mod of.
	if document.location.Mod {
		readWith := "A mod of " + document.game.Install.Product.String()
		gui.DimmedText(readWith)
		gui.Tooltip("Read with " + document.game.Install.Root)
	}

	if document.game.Loose() {
		gui.DimmedText(looseNote)
		gui.Tooltip("The meshes and textures are looked for around the file and by their names next to it. A missing texture shows as a checkerboard, a missing mesh as nothing.")
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
		gui.TextDisabled(plural(len(entities), "entity", "entities"))
	} else {
		gui.TextDisabled(fmt.Sprintf("%d of %s", len(shown), plural(len(entities), "entity", "entities")))
	}

	if gui.BeginList("##entities") {
		for _, name := range shown {
			selected := name == document.selected

			if gui.ListRow(name, selected) && !selected {
				a.selectEntity(name)
			}
		}

		// The arrow keys step through the list while it has the focus, which
		// is a quick way to look through the variants a file defines.
		if imgui.IsWindowFocused() && !imgui.CurrentIO().WantTextInput() {
			a.stepThrough(shown)
		}
	}

	gui.EndList()
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

	// The rows are all one height, so the row's place is known without
	// laying it out again.
	row := gui.ListRowPitch()
	visible := imgui.WindowHeight() - 2*gui.ListPadding()
	imgui.SetScrollYFloat(min(max(imgui.ScrollY(), float32(next+1)*row-visible), float32(next)*row))
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
		gui.BulletText(mesh)
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

		if details.Mesh != "" {
			propertyRow("Mesh", details.Mesh)
			propertyRow("Mesh file", details.MeshFile)
		} else {
			propertyRow("Mesh", "None")
		}

		imgui.EndTable()
	}

	imgui.SeparatorText("Parts")
	ownParts(details)

	if count := len(details.Attached); count > 0 {
		imgui.SeparatorText(fmt.Sprintf("Attached (%d)", count))
		attachedTree(details, 0)
	}

	// What the files did not have.
	if count := len(loaded.Diagnostics); count > 0 {
		imgui.SeparatorText(fmt.Sprintf("Problems (%d)", count))

		for _, problem := range loaded.Diagnostics {
			problemText(problem)
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
	gui.TextDisabled(name)
	imgui.TableNextColumn()
	gui.TextWrapped(value)
	gui.Record(value)
}

// ownParts lists the parts of the entity's own mesh, or says why it has none.
func ownParts(details workspace.Details) {
	parts := details.PartsOf(0)

	switch {
	case len(parts) > 0:
		partsTable("##parts", parts)
	case details.Mesh != "":
		gui.DimmedText(textNoGeometry)
	case len(details.Attached) > 0:
		gui.DimmedText(textOnlyAttached)
	default:
		gui.DimmedText(textNoMesh)
	}
}

// What the parts section says of an entity without parts of its own.
const (
	textNoGeometry   = "The mesh holds no geometry."
	textOnlyAttached = "The entity draws no mesh of its own, only what it attaches."
	textNoMesh       = "The entity draws no mesh."
)

// attachedTree lists the attachments that hang from one, by its number: each
// entity with the point it hangs from, opening onto its parts and onto what
// it attaches in turn. One that is not drawn is in the warning colour, and
// Problems says why.
func attachedTree(details workspace.Details, parent int) {
	for _, number := range details.AttachedTo(parent) {
		attached := details.Attached[number-1]
		parts := details.PartsOf(number)
		further := details.AttachedTo(number)
		leaf := attached.Missing || (len(parts) == 0 && len(further) == 0)

		if attached.Missing {
			gui.PushWarningColor()
		}

		open := gui.TreeNode(fmt.Sprintf("%s##attached%d", attached.Entity, number), leaf)

		if attached.Missing {
			imgui.PopStyleColor()
		}

		switch {
		case attached.Missing:
			gui.Tooltip("Not drawn: see Problems")
		case leaf:
			gui.Tooltip("Draws no mesh")
		}

		where := "at " + attached.Locator

		imgui.SameLine()
		gui.TextDisabled(where)
		gui.Record(where)

		if !open {
			continue
		}

		if len(parts) > 0 {
			partsTable(fmt.Sprintf("##parts%d", number), parts)
		}

		attachedTree(details, number)
		imgui.TreePop()
	}
}

// partsTable lists parts of the model: the shapes of a mesh, how each is
// drawn, which of their textures were found, and how large they are. The
// names the files give them are in a tooltip, for whoever needs them.
func partsTable(id string, parts []workspace.PartDetails) {
	flags := imgui.TableFlagsSizingStretchProp | imgui.TableFlagsRowBg | imgui.TableFlagsBordersInnerH | imgui.TableFlagsPadOuterX

	if !imgui.BeginTableV(id, 2, flags, imgui.Vec2{}, 0) {
		return
	}

	imgui.TableSetupColumnV("part", imgui.TableColumnFlagsWidthStretch, 0, 0)
	// Wide enough for the largest count, and for the word under it.
	sizeWidth := imgui.CalcTextSize("triangles").X
	for _, part := range parts {
		sizeWidth = max(sizeWidth, imgui.CalcTextSize(thousands(part.Triangles)).X)
	}

	imgui.TableSetupColumnV("size", imgui.TableColumnFlagsWidthFixed, sizeWidth, 0)

	padding := gui.Scaled(4)

	for _, part := range parts {
		imgui.TableNextRowV(0, 0)

		imgui.TableNextColumn()
		imgui.Dummy(imgui.Vec2{Y: padding})

		name := partName(part.Name)

		gui.PushStrongFont()
		imgui.TextUnformatted(name)
		gui.PopFont()
		gui.Record(name)
		gui.Tooltip(partTooltip(part))

		gui.DimmedText(partLook(part))

		if part.Drawn {
			textureMaps(part)
		}

		imgui.Dummy(imgui.Vec2{Y: padding})

		imgui.TableNextColumn()
		imgui.Dummy(imgui.Vec2{Y: padding})
		rightAligned(thousands(part.Triangles), false)
		gui.Tooltip(plural(part.Vertices, "vertex", "vertices"))
		rightAligned("triangles", true)
	}

	imgui.EndTable()
}

// partTooltip names a part the way its files do.
func partTooltip(part workspace.PartDetails) string {
	shader := part.Shader
	if shader == "" {
		shader = "none"
	}

	return "Shape: " + part.Name + "\nShader: " + shader
}

// textureMaps says which texture maps of a part were found, and which are
// drawn with a stand in.
func textureMaps(part workspace.PartDetails) {
	for index, texture := range []struct {
		name  string
		found bool
	}{
		{"Diffuse", part.Diffuse},
		{"Normal", part.Normal},
		{"Properties", part.Properties},
	} {
		if index > 0 {
			imgui.SameLine()
		}

		if texture.found {
			gui.TextDisabled(texture.name)
			gui.Tooltip("The " + strings.ToLower(texture.name) + " map was found")
		} else {
			label := "No " + strings.ToLower(texture.name)
			gui.WarningText(label)
			gui.Tooltip("The " + strings.ToLower(texture.name) + " map was not found; drawn with a stand in")
		}
	}
}

// rightAligned writes a line of text against the right edge of a column.
func rightAligned(text string, dimmed bool) {
	imgui.SetCursorPosX(imgui.CursorPosX() + imgui.ContentRegionAvail().X - imgui.CalcTextSize(text).X)

	if dimmed {
		gui.TextDisabled(text)
	} else {
		imgui.TextUnformatted(text)
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

		gui.TextDisabled(place)
		gui.Tooltip(problem.Path)
	}

	imgui.Spacing()
}

// displaySettings are what the model is drawn with beyond its own files.
func (a *App) displaySettings() {
	details := a.shown.loaded.Details

	// The palette colour only shows on skin, so it is only offered for an
	// entity that has some.
	if details.UsesPalette() {
		label := labelPalette
		if details.Portrait() {
			label = labelSkinTone
		}

		look := &a.viewer.Look

		gui.TextDisabled(label)
		gui.Record(label)
		imgui.SameLine()
		imgui.SetNextItemWidth(fullWidth)

		if imgui.ColorEdit3V("##palette", &look.PaletteColor, imgui.ColorEditFlagsNoLabel) {
			a.paletteChosen = true
		}

		gui.Tooltip("Blended into the parts drawn as skin, where the alpha of their diffuse map says")
	}

	gui.Checkbox("Turn automatically", &a.turning)

	if gui.Button("Reset Camera") {
		a.resetCamera()
	}
}

// The labels of the palette colour: on a portrait it is the skin tone.
const (
	labelPalette  = "Palette colour"
	labelSkinTone = "Skin tone"
)

// looseNote says what a loose file, one of no game, is drawn with.
const looseNote = "Outside any game: drawn with the files around it."

// colorVec4 turns a raylib colour into a Dear ImGui one.
func colorVec4(color rl.Color) imgui.Vec4 {
	return imgui.Vec4{X: float32(color.R) / 255, Y: float32(color.G) / 255, Z: float32(color.B) / 255, W: float32(color.A) / 255}
}
