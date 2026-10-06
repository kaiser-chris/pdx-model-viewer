package app

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// exportView is a view of the model the export draws.
type exportView struct {
	// name is the view's part of a file name, and label what the export
	// window calls it.
	name, label string

	// current draws the view as the viewport shows it; the others are drawn
	// from straight in front, behind, beside, above or below the model, with
	// all of it in the picture.
	current    bool
	yaw, pitch float64
}

// The views the export offers. The yaw takes the camera around the model to
// the camera's left, so the model's own left side is seen from a yaw of a
// quarter turn the other way.
var exportViews = []exportView{
	{name: "current", label: "Current view", current: true},
	{name: "front", label: "Front"},
	{name: "back", label: "Back", yaw: math.Pi},
	{name: "left", label: "Left side", yaw: -math.Pi / 2},
	{name: "right", label: "Right side", yaw: math.Pi / 2},
	{name: "top", label: "Top", pitch: math.Pi / 2},
	{name: "bottom", label: "Bottom", pitch: -math.Pi / 2},
}

// exportSettings are the choices of the export window.
type exportSettings struct {
	// views says which of exportViews are drawn, by their index.
	views []bool

	// background fills what the model leaves of the picture with
	// backgroundColor; without it, that is transparent.
	background      bool
	backgroundColor [3]float32

	// size is the width and height of the pictures written.
	size [2]int32
}

// The sizes an export can be written in, as far as a side goes.
const (
	minExportSide = 16
	maxExportSide = 8192
)

// newExportSettings picks the current view and no background to begin with,
// in the size set last, or the desktop's resolution if none was.
func newExportSettings(desktop, kept [2]int32) exportSettings {
	views := make([]bool, len(exportViews))
	views[0] = true

	size := scaledExportSize(desktop, 1)
	if kept[0] > 0 && kept[1] > 0 {
		size = [2]int32{clampExportSide(kept[0]), clampExportSide(kept[1])}
	}

	return exportSettings{views: views, backgroundColor: [3]float32{1, 1, 1}, size: size}
}

// scaledExportSize is a size scaled by a factor, kept within the sizes an
// export can be written in. A desktop of no known size counts as 1920 by
// 1080.
func scaledExportSize(desktop [2]int32, factor float64) [2]int32 {
	if desktop[0] <= 0 || desktop[1] <= 0 {
		desktop = [2]int32{1920, 1080}
	}

	return [2]int32{clampExportSide(int32(float64(desktop[0])*factor + 0.5)), clampExportSide(int32(float64(desktop[1])*factor + 0.5))}
}

func clampExportSide(side int32) int32 {
	return min(max(side, minExportSide), maxExportSide)
}

// chosen are the views picked.
func (s exportSettings) chosen() []exportView {
	var views []exportView

	for index, picked := range s.views {
		if picked {
			views = append(views, exportViews[index])
		}
	}

	return views
}

// exportRequest is an export the user has asked for and said where to: the
// pictures are drawn at the start of the next frame.
type exportRequest struct {
	entity   string
	views    []exportView
	settings exportSettings

	// file is where a single view goes, and folder where several go.
	file, folder string
}

// exportedPicture is a view drawn, to be written to a file.
type exportedPicture struct {
	view    exportView
	picture *image.RGBA
}

// exportSupersampling is how many times larger than the picture written
// the views are drawn, then scaled down, which smooths their edges. A
// picture too large to draw at that size is drawn at its own.
const exportSupersampling = 2

// renderExport draws the views of an export. It runs while raylib drawing is
// active, before the viewport is drawn, and leaves the viewer as it was.
func (a *App) renderExport(request *exportRequest) []exportedPicture {
	viewer := a.viewer
	yaw, pitch, distance, background := viewer.Yaw, viewer.Pitch, viewer.Distance, viewer.Background

	defer func() {
		viewer.Yaw, viewer.Pitch, viewer.Distance, viewer.Background = yaw, pitch, distance, background
		a.frameModel()
	}()

	width, height := clampExportSide(request.settings.size[0]), clampExportSide(request.settings.size[1])

	scale := int32(exportSupersampling)
	if max(width, height)*scale > maxExportSide {
		scale = 1
	}

	viewer.Resize(width*scale, height*scale)

	viewer.Background = rl.Blank
	if request.settings.background {
		viewer.Background = colorRGBA(request.settings.backgroundColor)
	}

	pictures := make([]exportedPicture, 0, len(request.views))

	for _, view := range request.views {
		if view.current {
			viewer.Yaw, viewer.Pitch, viewer.Distance = yaw, pitch, distance
			a.frameModel()
		} else {
			// The whole model, from where the view says, without the pan.
			viewer.Reset()
			viewer.Yaw, viewer.Pitch = view.yaw, view.pitch
			viewer.Frame(a.framed[0], a.framed[1])
		}

		viewer.Draw(a.shown.model)
		pictures = append(pictures, exportedPicture{view: view, picture: shrink(viewer.Image(), int(scale))})
	}

	return pictures
}

// writeExport writes the pictures of an export and says what it did.
func writeExport(request *exportRequest, pictures []exportedPicture) (string, error) {
	if request.file != "" {
		if err := writePNG(request.file, pictures[0].picture); err != nil {
			return "", err
		}

		return "Exported the " + pictures[0].view.label + " of " + request.entity + " to " + request.file, nil
	}

	views := make([]string, len(pictures))
	for index, exported := range pictures {
		views[index] = exported.view.name
	}

	for index, path := range exportGroupPaths(request.folder, request.entity, views) {
		if err := writePNG(path, pictures[index].picture); err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("Exported %d views of %s to %s", len(pictures), request.entity, request.folder), nil
}

func writePNG(path string, picture image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}

	if err := png.Encode(file, picture); err != nil {
		_ = file.Close()

		return fmt.Errorf("write %s: %w", path, err)
	}

	return file.Close()
}

// shrink scales a picture down by a whole factor, each pixel the average of
// the ones it covers. The pictures are premultiplied by their alpha, as the
// GPU blends them over a transparent background, which is what makes a plain
// average right at the edges of the model too.
func shrink(picture *image.RGBA, factor int) *image.RGBA {
	if factor <= 1 {
		return picture
	}

	bounds := picture.Bounds()
	width, height := bounds.Dx()/factor, bounds.Dy()/factor
	shrunk := image.NewRGBA(image.Rect(0, 0, width, height))
	count := uint32(factor * factor)

	for y := range height {
		for x := range width {
			var r, g, b, a uint32

			for dy := range factor {
				for dx := range factor {
					pixel := picture.RGBAAt(bounds.Min.X+x*factor+dx, bounds.Min.Y+y*factor+dy)
					r += uint32(pixel.R)
					g += uint32(pixel.G)
					b += uint32(pixel.B)
					a += uint32(pixel.A)
				}
			}

			shrunk.SetRGBA(x, y, color.RGBA{
				R: uint8((r + count/2) / count),
				G: uint8((g + count/2) / count),
				B: uint8((b + count/2) / count),
				A: uint8((a + count/2) / count),
			})
		}
	}

	return shrunk
}

// uniqueExportPath is where a view of an entity is written in a folder:
// <entity>_<view>.png, or with _1, _2 and so on before the extension when a
// file of that name is in the folder already.
func uniqueExportPath(folder, entity, view string) string {
	return exportGroupPaths(folder, entity, []string{view})[0]
}

// exportGroupPaths is where several views of an entity exported together are
// written: under the names uniqueExportPath gives, all with the same number,
// the first that leaves every one of them free, so that they stay one group.
func exportGroupPaths(folder, entity string, views []string) []string {
	paths := make([]string, len(views))

	for number := 0; ; number++ {
		free := true

		for index, view := range views {
			name := safeFileName(entity) + "_" + view
			if number > 0 {
				name += fmt.Sprintf("_%d", number)
			}

			paths[index] = filepath.Join(folder, name+".png")
			free = free && !fileExists(paths[index])
		}

		if free {
			return paths
		}
	}
}

// safeFileName replaces what a file name cannot hold on Windows.
func safeFileName(name string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < ' ' {
			return '_'
		}

		return r
	}, name)
}

// exportFolder is where the dialogs of an export start: where the last
// export went, or the user's pictures.
func (a *App) exportFolder() string {
	if folder := a.settings.ExportFolder; folder != "" && fileExists(folder) {
		return folder
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	if pictures := filepath.Join(home, "Pictures"); fileExists(pictures) {
		return pictures
	}

	return home
}

// withPNGExtension adds .png to a file name the user typed without it.
func withPNGExtension(path string) string {
	if strings.EqualFold(filepath.Ext(path), ".png") {
		return path
	}

	return path + ".png"
}

func colorRGBA(value [3]float32) rl.Color {
	channel := func(c float32) uint8 { return uint8(min(max(c, 0), 1)*255 + 0.5) }

	return rl.Color{R: channel(value[0]), G: channel(value[1]), B: channel(value[2]), A: 255}
}
