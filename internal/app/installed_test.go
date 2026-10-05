//go:build uitest

package app

import (
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaiser-chris/pdx-model-viewer/internal/workspace"
)

// TestViewInstalled opens a sample of the asset files of real installations
// in the hidden window, views the first entity of each, and checks something
// was drawn. It is skipped unless PDX_GAME_DIR names one or more game folders,
// separated the way PATH is. With PDX_DUMP_DIR set, the viewport of each is
// written out as a PNG file, which is how to see that the models look right
// without a window on the desktop.
func TestViewInstalled(t *testing.T) {
	const files = 6

	value := os.Getenv("PDX_GAME_DIR")
	if value == "" {
		t.Skip("set PDX_GAME_DIR to one or more game folders to run this test")
	}

	application, driver := startApp(t)
	dump := os.Getenv("PDX_DUMP_DIR")

	for _, root := range filepath.SplitList(value) {
		for _, file := range sampleAssetFiles(t, root, files) {
			openFile(t, application, driver, file)

			entities := application.document.listing.Entities
			if len(entities) > 1 {
				application.selectEntity(entities[0])
			}

			name := entities[0]

			driver.WaitFor(name+" to load", func() bool {
				return application.loading == nil && (application.document.failure != "" ||
					application.shown != nil && application.shown.loaded.Details.Entity == name)
			})

			if failure := application.document.failure; failure != "" {
				t.Errorf("%s: %s", name, failure)

				continue
			}

			driver.Frames(2)

			picture := application.viewer.Image()
			if !drewSomething(picture) {
				t.Errorf("%s of %s: the viewport shows nothing but its background", name, file)
			}

			if dump != "" {
				writePicture(t, filepath.Join(dump, workspace.SourceName(root)+"_"+name+".png"), picture)
			}
		}
	}
}

// sampleAssetFiles picks files that define an entity drawing a mesh, spread
// over the whole game.
func sampleAssetFiles(t *testing.T, root string, count int) []string {
	t.Helper()

	var all []string

	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.EqualFold(filepath.Ext(path), workspace.Extension) {
			all = append(all, path)
		}

		return nil
	})

	var picked []string

	for index := 0; index < len(all) && len(picked) < count; index += max(len(all)/(count*4), 1) {
		data, err := os.ReadFile(all[index])
		if err != nil || !strings.Contains(string(data), "entity") || !strings.Contains(string(data), "pdxmesh") {
			continue
		}

		location, err := workspace.Locate(all[index])
		if err == nil && len(workspace.List(location).Entities) > 0 {
			picked = append(picked, all[index])
		}
	}

	return picked
}

// drewSomething reports whether a picture has any pixel other than the
// viewport's background.
func drewSomething(picture *image.RGBA) bool {
	background := color.RGBA(viewportBackground)

	for y := picture.Rect.Min.Y; y < picture.Rect.Max.Y; y += 4 {
		for x := picture.Rect.Min.X; x < picture.Rect.Max.X; x += 4 {
			if picture.RGBAAt(x, y) != background {
				return true
			}
		}
	}

	return false
}

func writePicture(t *testing.T, path string, picture *image.RGBA) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	if err := png.Encode(file, picture); err != nil {
		t.Fatal(err)
	}
}
