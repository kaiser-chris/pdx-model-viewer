package app

import (
	"fmt"
	"path/filepath"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/kaiser-chris/pdx-model-viewer/internal/gui"
)

// The export window and the labels of its widgets the interface tests look
// for.
const (
	popupExport       = "Export"
	labelExportButton = "Export..."
	labelBackground   = "Background colour"
)

// canExport reports whether there is something on view to export.
func (a *App) canExport() bool {
	return a.shown != nil && len(a.shown.loaded.Model.Parts) > 0 && a.exportBusy() == ""
}

// exportBusy says what an export is waiting on, if one is under way.
func (a *App) exportBusy() string {
	switch {
	case a.exportDialog != nil:
		return "choosing where to"
	case a.exportPending != nil:
		return "drawing"
	case a.exportWriting != nil:
		return "writing"
	}

	return ""
}

// exportWindow lets the user pick the views to export and a background, and
// export them.
func (a *App) exportWindow() {
	if a.showExport {
		a.showExport = false

		if a.canExport() {
			imgui.OpenPopupStr(popupExport)
		}
	}

	imgui.SetNextWindowSizeV(gui.ScaledVec2(340, 0), imgui.CondAppearing)

	open := true
	if !imgui.BeginPopupModalV(popupExport, &open, imgui.WindowFlagsNoSavedSettings|imgui.WindowFlagsAlwaysAutoResize) {
		return
	}

	settings := &a.exportSettings

	imgui.SeparatorText("Views")

	for index, view := range exportViews {
		gui.Checkbox(view.label, &settings.views[index])
	}

	imgui.SeparatorText("Background")

	gui.Checkbox(labelBackground, &settings.background)
	gui.Tooltip("Without one, what the model leaves of the picture is transparent")

	if settings.background {
		imgui.SameLine()
		imgui.SetNextItemWidth(fullWidth)
		imgui.ColorEdit3V("##exportBackground", &settings.backgroundColor, imgui.ColorEditFlagsNoLabel|imgui.ColorEditFlagsNoInputs)
	}

	imgui.SeparatorText("Size")
	a.exportSizeSettings()

	imgui.Spacing()

	chosen := len(settings.chosen())
	if chosen > 1 {
		gui.DimmedText("Several views are written to a folder you choose.")
		imgui.Spacing()
	}

	imgui.BeginDisabledV(chosen == 0 || a.shown == nil)

	if gui.Button(labelExportButton) {
		a.askWhereToExport()
		imgui.CloseCurrentPopup()
	}

	imgui.EndDisabled()

	if !open || imgui.IsKeyPressedBool(imgui.KeyEscape) {
		imgui.CloseCurrentPopup()
	}

	imgui.EndPopup()
}

// The labels of the size's fields and of the buttons that set it from the
// desktop's resolution, with the factor each scales it by.
const (
	labelExportWidth  = "Width"
	labelExportHeight = "Height"
)

var exportSizePresets = []struct {
	label  string
	factor float64
}{
	{"50%", 0.5},
	{"Desktop", 1},
	{"200%", 2},
}

// exportSizeSettings are buttons that set the size of the pictures from the
// desktop's resolution, and the width and height themselves.
func (a *App) exportSizeSettings() {
	size := &a.exportSettings.size

	for index, preset := range exportSizePresets {
		if index > 0 {
			imgui.SameLine()
		}

		scaled := scaledExportSize(a.desktopSize, preset.factor)

		if gui.Button(preset.label) {
			a.setExportSize(scaled)
		}

		gui.Tooltip(presetTooltip(scaled, preset.factor))
	}

	for index, label := range []string{labelExportWidth, labelExportHeight} {
		// Kept within the sizes there can be once the field is left, rather
		// than while a number is typed into it.
		gui.InputInt(label, &size[index])

		if imgui.IsItemDeactivatedAfterEdit() {
			a.setExportSize(*size)
		}
	}
}

// presetTooltip says what size a preset sets, and how much of the desktop's
// resolution that is.
func presetTooltip(size [2]int32, factor float64) string {
	share := "Full"
	if factor != 1 {
		share = fmt.Sprintf("%.0f%%", factor*100)
	}

	return fmt.Sprintf("Set to %dx%d: %s desktop resolution", size[0], size[1], share)
}

// askWhereToExport asks for the file to export one view to, or the folder to
// export several to, on a goroutine of its own so that the window keeps
// drawing meanwhile.
func (a *App) askWhereToExport() {
	views := a.exportSettings.chosen()
	if len(views) == 0 || a.shown == nil || a.exportBusy() != "" {
		return
	}

	request := &exportRequest{
		entity:   a.shown.loaded.Details.Entity,
		views:    views,
		settings: a.exportSettings,
	}

	folder := a.exportFolder()
	dialogs := a.dialogs

	if len(views) == 1 {
		proposed := uniqueExportPath(folder, request.entity, views[0].name)

		a.exportDialog = start(func() (*exportRequest, error) {
			path, err := dialogs.chooseExportFile(proposed)
			if err != nil || path == "" {
				return nil, err
			}

			request.file = withPNGExtension(path)

			return request, nil
		})

		return
	}

	a.exportDialog = start(func() (*exportRequest, error) {
		chosen, err := dialogs.chooseExportFolder(folder)
		if err != nil || chosen == "" {
			return nil, err
		}

		request.folder = chosen

		return request, nil
	})
}

// pollExport moves an export along: from the dialog to drawing, which the
// next frame does, and from writing the files to saying it is done.
func (a *App) pollExport() {
	if a.exportDialog != nil {
		if request, err, done := a.exportDialog.poll(); done {
			a.exportDialog = nil

			switch {
			case err != nil:
				warn(err)
				a.setStatus("The file dialog could not be shown: %v", err)
			case request != nil:
				a.exportPending = request
			}
		}
	}

	if a.exportWriting != nil {
		if message, err, done := a.exportWriting.poll(); done {
			a.exportWriting = nil

			if err != nil {
				warn(err)
				a.setStatus("Could not export: %v", err)
			} else {
				a.setStatus("%s", message)
			}
		}
	}
}

// drawExport draws the views of the export asked for, if any, and starts
// writing them. It runs while raylib drawing is active, before the viewport
// is drawn.
func (a *App) drawExport() {
	request := a.exportPending
	if request == nil {
		return
	}

	a.exportPending = nil

	if a.shown == nil || a.shown.loaded.Details.Entity != request.entity {
		a.setStatus("Nothing exported: %s is no longer on view", request.entity)

		return
	}

	pictures := a.renderExport(request)

	if request.file != "" {
		a.rememberExportFolder(filepath.Dir(request.file))
	} else {
		a.rememberExportFolder(request.folder)
	}

	a.setStatus("Exporting %s...", request.entity)

	a.exportWriting = start(func() (string, error) {
		return writeExport(request, pictures)
	})
}
