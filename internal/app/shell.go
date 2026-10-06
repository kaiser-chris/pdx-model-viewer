package app

import (
	"fmt"
	"time"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/kaiser-chris/pdx-model-viewer/internal/gui"
)

// The titles of the panels, which are also how the docking layout knows them.
const (
	panelViewport = "Viewport"
	panelEntities = "Entities"
	panelDetails  = "Details"

	popupError = "Could Not Open the File"
	popupAbout = "About " + applicationName
)

// dockNodeFlagsDockSpace is ImGuiDockNodeFlags_DockSpace from imgui_internal.h.
// cimgui-go only generates the public flags, but building a default layout
// needs this one: it marks the root node as able to host a dock space.
const dockNodeFlagsDockSpace imgui.DockNodeFlags = 1 << 10

// The shares the side panels take in the default layout: the column of the
// window they share, and the part of that column the entities take.
const (
	sideRatio     = 0.25
	entitiesRatio = 0.4
)

// dockSpace covers the remaining viewport with a dock space so every panel can
// be rearranged, tabbed or torn off by the user.
func (a *App) dockSpace() {
	id := imgui.DockSpaceOverViewportV(0, imgui.MainViewport(), imgui.DockNodeFlagsPassthruCentralNode, a.dockWindowClass)

	if !a.layoutBuilt {
		a.layoutBuilt = true
		a.buildDefaultLayout(id)
	}
}

// buildDefaultLayout puts the viewport on the left and a column on the right,
// with the entities at its top and the details below them. It only runs when
// there is no saved layout, or when the user resets it.
func (a *App) buildDefaultLayout(dockSpaceID imgui.ID) {
	imgui.InternalDockBuilderRemoveNode(dockSpaceID)
	imgui.InternalDockBuilderAddNodeV(dockSpaceID, dockNodeFlagsDockSpace)
	imgui.InternalDockBuilderSetNodeSize(dockSpaceID, imgui.MainViewport().WorkSize())

	var side, center imgui.ID
	imgui.InternalDockBuilderSplitNode(dockSpaceID, imgui.DirRight, sideRatio, &side, &center)

	var top, bottom imgui.ID
	imgui.InternalDockBuilderSplitNode(side, imgui.DirUp, entitiesRatio, &top, &bottom)

	imgui.InternalDockBuilderDockWindow(panelViewport, center)
	imgui.InternalDockBuilderDockWindow(panelEntities, top)
	imgui.InternalDockBuilderDockWindow(panelDetails, bottom)

	imgui.InternalDockBuilderFinish(dockSpaceID)
}

func (a *App) menuBar() {
	if !imgui.BeginMainMenuBar() {
		return
	}
	defer imgui.EndMainMenuBar()

	if gui.BeginMenu("File") {
		if gui.MenuItem("Open...", "Ctrl+O", a.dialog == nil) {
			a.askForAssetFile()
		}

		if gui.MenuItem("Reload", "F5", a.document != nil && a.opening == nil) {
			a.reload()
		}

		imgui.Separator()

		if gui.MenuItem("Exit", "Alt+F4", true) {
			a.window.RequestClose()
		}

		imgui.EndMenu()
	}

	if gui.BeginMenu("View") {
		gui.MenuToggle(panelEntities, "", &a.showEntities)
		gui.MenuToggle(panelDetails, "", &a.showDetails)

		imgui.Separator()

		gui.MenuToggle("Turn Automatically", "T", &a.turning)

		if gui.MenuItem("Reset Camera", "Home", a.shown != nil) {
			a.resetCamera()
		}

		imgui.Separator()

		if gui.MenuItem("Reset Layout", "", true) {
			a.layoutBuilt = false
			a.showEntities = true
			a.showDetails = true
		}

		imgui.EndMenu()
	}

	if gui.BeginMenu("Help") {
		if gui.MenuItem("About", "", true) {
			a.showAbout = true
		}

		imgui.EndMenu()
	}
}

// statusBar claims a strip along the bottom of the viewport. It is submitted
// before the dock space so that panels do not overlap it.
func (a *App) statusBar() {
	flags := imgui.WindowFlagsNoScrollbar | imgui.WindowFlagsNoSavedSettings | imgui.WindowFlagsMenuBar

	if imgui.InternalBeginViewportSideBar("##StatusBar", imgui.MainViewport(), imgui.DirDown, imgui.FrameHeight(), flags) {
		if imgui.BeginMenuBar() {
			imgui.TextUnformatted(a.status)

			right := a.diagnostics()

			// Right align the diagnostics.
			imgui.SameLine()
			imgui.SetCursorPosX(imgui.ContentRegionAvail().X - imgui.CalcTextSize(right).X)
			imgui.TextDisabled(right)

			imgui.EndMenuBar()
		}

		imgui.End()
	}
}

// diagnostics is the right hand side of the status bar: the game the open file
// belongs to, and how the interface itself is doing.
func (a *App) diagnostics() string {
	fps := fmt.Sprintf("%.0f FPS", imgui.CurrentIO().Framerate())

	if a.opening != nil {
		return "reading files...  |  " + fps
	}

	if a.document == nil {
		return fps
	}

	game := a.document.game

	return fmt.Sprintf("%s: %d entities, read in %s  |  %s", game.Name, game.Entities(), game.Took.Round(time.Millisecond), fps)
}

// handleShortcuts implements the keyboard shortcuts that are not attached to a
// single panel.
func (a *App) handleShortcuts() {
	io := imgui.CurrentIO()

	// Typing into the search box is not a shortcut.
	if io.WantTextInput() {
		return
	}

	switch {
	case io.KeyCtrl() && imgui.IsKeyPressedBool(imgui.KeyO):
		a.askForAssetFile()
	case imgui.IsKeyPressedBool(imgui.KeyF5):
		a.reload()
	case imgui.IsKeyPressedBool(imgui.KeyHome):
		a.resetCamera()
	case !io.KeyCtrl() && imgui.IsKeyPressedBoolV(imgui.KeyT, false):
		a.turning = !a.turning
	}
}

// errorPopup says why a file could not be opened.
func (a *App) errorPopup() {
	if a.openError != "" && !imgui.IsPopupOpenStr(popupError) {
		imgui.OpenPopupStr(popupError)
	}

	imgui.SetNextWindowSizeV(gui.ScaledVec2(440, 0), imgui.CondAppearing)

	if !imgui.BeginPopupModalV(popupError, nil, imgui.WindowFlagsNoSavedSettings) {
		return
	}

	imgui.TextWrapped(a.openError)
	gui.Record(a.openError)

	imgui.Spacing()
	gui.DimmedText("The viewer finds the meshes and textures of an asset file relative to the game or mod it belongs to, so the file has to be inside the gfx folder of a game or mod.")
	imgui.Spacing()

	if gui.Button("OK") || imgui.IsKeyPressedBool(imgui.KeyEscape) || imgui.IsKeyPressedBool(imgui.KeyEnter) {
		a.openError = ""
		imgui.CloseCurrentPopup()
	}

	imgui.SetItemDefaultFocus()
	imgui.EndPopup()
}

func (a *App) aboutPopup() {
	if a.showAbout {
		a.showAbout = false
		imgui.OpenPopupStr(popupAbout)
	}

	imgui.SetNextWindowSizeV(gui.ScaledVec2(420, 0), imgui.CondAppearing)

	if !imgui.BeginPopupModalV(popupAbout, nil, imgui.WindowFlagsNoSavedSettings) {
		return
	}

	gui.PushStrongFont()
	imgui.TextUnformatted(applicationName)
	gui.PopFont()
	imgui.TextDisabled("Version " + applicationVersion)

	imgui.Spacing()
	imgui.TextWrapped("Views the 3D models of the asset files of Victoria 3, Europa Universalis 5 and Crusader Kings 3.")
	imgui.Spacing()

	gui.DimmedText("Reads the games' files with pdx-asset-go and pdx-parser-go, and draws with raylib and Dear ImGui. Set in Roboto, under the SIL Open Font License.")
	imgui.Spacing()

	if gui.Button("Close") || imgui.IsKeyPressedBool(imgui.KeyEscape) {
		imgui.CloseCurrentPopup()
	}

	imgui.EndPopup()
}
