package app

import (
	"fmt"
	"path/filepath"
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
	popupAbout = "About"
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

		a.recentMenu()

		if gui.MenuItem("Reload", "F5", a.document != nil && a.opening == nil) {
			a.reload()
		}

		if gui.MenuItem("Export...", "Ctrl+E", a.canExport()) {
			a.showExport = true
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

// The label of the Recent menu, and of what it says without recent files.
const (
	menuRecent     = "Recent"
	labelNoRecents = "No recent files"
)

// recentMenu lists the files opened last, to open one again.
func (a *App) recentMenu() {
	if !gui.BeginMenu(menuRecent) {
		return
	}
	defer imgui.EndMenu()

	files := a.settings.RecentFiles
	if len(files) == 0 {
		gui.MenuItem(labelNoRecents, "", false)

		return
	}

	for index, file := range files {
		// The folder the file is in tells apart files of one name, which
		// the assets of the games often share.
		name := filepath.Base(file)
		folder := filepath.Base(filepath.Dir(file))

		clicked := gui.MenuItem(fmt.Sprintf("%s##recent%d", name, index), folder, a.opening == nil)
		gui.Tooltip(file)

		if clicked {
			a.openRecent(file)
		}
	}
}

// openRecent opens a recent file again, or takes it off the list if it is
// not there any more.
func (a *App) openRecent(file string) {
	if !fileExists(file) {
		a.forgetRecentFile(file)
		a.setStatus("%s is no longer there, so it was taken off the recent files", file)

		return
	}

	a.Open(file)
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
			gui.TextDisabled(right)

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
	case io.KeyCtrl() && imgui.IsKeyPressedBool(imgui.KeyE):
		a.showExport = true
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

	gui.TextWrapped(a.openError)
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

	imgui.SetNextWindowSizeV(gui.ScaledVec2(460, 0), imgui.CondAlways)

	if !imgui.BeginPopupModalV(popupAbout, nil, imgui.WindowFlagsNoResize|imgui.WindowFlagsNoSavedSettings) {
		return
	}
	defer imgui.EndPopup()

	gui.PushStrongFont()
	imgui.TextUnformatted(applicationName + " " + applicationVersion)
	gui.PopFont()
	gui.Record(applicationName + " " + applicationVersion)

	gui.DimmedText(aboutDescription)

	imgui.Separator()

	gui.DimmedText("Rendered with raylib, interface built with Dear ImGui.")

	gui.PushStrongFont()
	imgui.SeparatorText("Credits")
	gui.PopFont()

	for _, credit := range credits {
		credit.show()
		imgui.Spacing()
	}

	imgui.Separator()

	if gui.Button("Close") || imgui.IsKeyPressedBool(imgui.KeyEscape) {
		imgui.CloseCurrentPopup()
	}
}

// aboutDescription says what the application is, under its name.
const aboutDescription = "Model viewer for asset files of Victoria 3, Europa Universalis 5 and Crusader Kings 3."
