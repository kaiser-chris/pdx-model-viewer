//go:build uitest

package app

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/kaiser-chris/pdx-model-viewer/internal/uitest"
)

// showStatue opens the fixture's statue and shows it, ready to export, with
// the exports going to a folder of the test's own.
func showStatue(t *testing.T) (*App, *uitest.Driver, *fakeDialogs, string) {
	t.Helper()

	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "statue.asset"))
	driver.Click(windowEntityList, "statue_entity")
	waitForEntity(application, driver, "statue_entity")

	folder := t.TempDir()
	application.settings.ExportFolder = folder

	// Small pictures, which are quick to draw and write.
	application.exportSettings.size = [2]int32{320, 200}

	dialogs := &fakeDialogs{}
	application.dialogs = dialogs

	return application, driver, dialogs, folder
}

// export clicks the export button and waits until the files are written.
func export(t *testing.T, application *App, driver *uitest.Driver) {
	t.Helper()

	driver.Click(popupExport, labelExportButton)
	driver.WaitFor("the export to be written", func() bool { return application.exportBusy() == "" })

	if driver.Exists(popupExport, labelExportButton) {
		t.Error("the export window is still open after exporting")
	}
}

func readPNG(t *testing.T, path string) image.Image {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("the export wrote no %s: %v", filepath.Base(path), err)
	}
	defer file.Close()

	picture, err := png.Decode(file)
	if err != nil {
		t.Fatalf("%s is no PNG: %v", filepath.Base(path), err)
	}

	return picture
}

func alphaAt(picture image.Image, at image.Point) uint32 {
	_, _, _, a := picture.At(at.X, at.Y).RGBA()

	return a >> 8
}

func middle(picture image.Image) image.Point {
	bounds := picture.Bounds()

	return image.Pt(bounds.Dx()/2, bounds.Dy()/2)
}

// One view is saved where the save dialog says, proposed under the entity's
// and the view's name, numbered past a file of that name already there; the
// picture is the size asked for, the model on a transparent background.
func TestExportOneView(t *testing.T) {
	application, driver, dialogs, folder := showStatue(t)

	writeFile(t, filepath.Join(folder, "statue_entity_current.png"), []byte("taken"))
	dialogs.answers = []string{filepath.Join(folder, "picked")}

	driver.Menu("File", "Export...")
	export(t, application, driver)

	if asked := dialogs.asked(); len(asked) != 1 || asked[0] != filepath.Join(folder, "statue_entity_current_1.png") {
		t.Errorf("the save dialog proposed %v, want statue_entity_current_1.png in the export folder", asked)
	}

	picture := readPNG(t, filepath.Join(folder, "picked.png"))

	if size := picture.Bounds().Size(); size != image.Pt(320, 200) {
		t.Errorf("picture of %v, want the 320x200 asked for", size)
	}

	if a := alphaAt(picture, middle(picture)); a != 255 {
		t.Errorf("the middle of the picture has an alpha of %d, want the model, opaque", a)
	}

	if a := alphaAt(picture, image.Point{}); a != 0 {
		t.Errorf("the corner of the picture has an alpha of %d, want a transparent background", a)
	}
}

// Several views go to a folder, each under the entity's and the view's name,
// numbered as one group, and a background colour fills what the model
// leaves.
func TestExportSeveralViews(t *testing.T) {
	application, driver, dialogs, folder := showStatue(t)

	writeFile(t, filepath.Join(folder, "statue_entity_front.png"), []byte("taken"))
	dialogs.answers = []string{folder}

	driver.Shortcut(imgui.ModCtrl, imgui.KeyE)
	driver.Frames(2)

	// The current view is picked to begin with; the front and the back
	// instead, on a background.
	driver.Click(popupExport, "Current view")
	driver.Click(popupExport, "Front")
	driver.Click(popupExport, "Back")
	driver.Click(popupExport, labelBackground)
	driver.Frames(2)

	export(t, application, driver)

	if asked := dialogs.asked(); len(asked) != 1 || asked[0] != folder {
		t.Errorf("the folder dialog started in %v, want the export folder", asked)
	}

	// The front view's name was taken, so both are numbered alike.
	front := readPNG(t, filepath.Join(folder, "statue_entity_front_1.png"))
	back := readPNG(t, filepath.Join(folder, "statue_entity_back_1.png"))

	if _, err := os.Stat(filepath.Join(folder, "statue_entity_current.png")); err == nil {
		t.Error("the current view was exported, which was not picked")
	}

	// The statue is a quad facing the front: seen from behind, there is
	// nothing but the background, which is opaque white.
	if r, g, b, _ := front.At(middle(front).X, middle(front).Y).RGBA(); r>>8 > 240 && g>>8 > 240 && b>>8 > 240 {
		t.Error("the front view shows no model in its middle")
	}

	if r, _, _, a := back.At(middle(back).X, middle(back).Y).RGBA(); a>>8 != 255 || r>>8 < 250 {
		t.Errorf("the back view's middle = %v, want the white background", back.At(middle(back).X, middle(back).Y))
	}

	if a := alphaAt(front, image.Point{}); a != 255 {
		t.Errorf("the front view's corner has an alpha of %d, want the opaque background", a)
	}
}

// Nothing on view, nothing to export.
func TestExportNeedsAnEntity(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "statue.asset"))

	driver.Shortcut(imgui.ModCtrl, imgui.KeyE)
	driver.Frames(2)

	if driver.Exists(popupExport, labelExportButton) {
		t.Error("the export window opened with no entity on view")
	}
}

// The size begins as the desktop's resolution, and the buttons set it to all
// of it, half of it and twice it; a size typed in is kept within the sizes
// there can be.
func TestExportSize(t *testing.T) {
	application, driver, _, _ := showStatue(t)

	application.desktopSize = [2]int32{1600, 900}
	application.exportSettings = newExportSettings(application.desktopSize, [2]int32{})

	if got := application.exportSettings.size; got != [2]int32{1600, 900} {
		t.Errorf("size to begin with = %v, want the desktop's 1600x900", got)
	}

	driver.Menu("File", "Export...")
	driver.Frames(2)

	for _, preset := range []struct {
		label string
		want  [2]int32
	}{
		{"50%", [2]int32{800, 450}},
		{"200%", [2]int32{3200, 1800}},
		{"Desktop", [2]int32{1600, 900}},
	} {
		driver.Click(popupExport, preset.label)

		if got := application.exportSettings.size; got != preset.want {
			t.Errorf("after %s: size = %v, want %v", preset.label, got, preset.want)
		}
	}

	driver.Fill(driver.Find(popupExport, labelExportHeight), "99999")
	driver.Press(imgui.KeyEnter)
	driver.Frames(2)

	if got := application.exportSettings.size[1]; got != maxExportSide {
		t.Errorf("height typed as 99999 = %d, want it kept to %d", got, maxExportSide)
	}
}

// The size set for an export is the one the next run begins with.
func TestExportSizeOutlivesTheRun(t *testing.T) {
	config := t.TempDir()

	first, _ := startAppIn(t, config)
	first.setExportSize([2]int32{1234, 567})
	first.Close()

	second, _ := startAppIn(t, config)

	if got := second.exportSettings.size; got != [2]int32{1234, 567} {
		t.Errorf("the next run exports in %v, want the 1234x567 set last", got)
	}
}
