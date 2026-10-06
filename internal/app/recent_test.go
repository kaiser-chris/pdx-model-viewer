//go:build uitest

package app

import (
	"path/filepath"
	"testing"

	"github.com/kaiser-chris/pdx-model-viewer/internal/uitest"
)

// The Recent menu lists the files opened, the latest first, and opens one
// again; a file that is gone is taken off it.
func TestRecentFiles(t *testing.T) {
	application, driver := startApp(t)

	driver.Menu("File", menuRecent)
	if !driver.Exists(uitest.AnyMenu, labelNoRecents) {
		t.Error("a first run's Recent menu does not say there are no recent files")
	}
	// Clicking the menu again closes it.
	driver.Click(uitest.MainMenuBar, "File")
	driver.Frames(2)

	statue, pedestal := fixtureFile(t, "statue.asset"), fixtureFile(t, "pedestal.asset")

	// A first file is the only recent one.
	openFile(t, application, driver, statue)

	if got := application.settings.RecentFiles; len(got) != 1 || !samePath(got[0], statue) {
		t.Errorf("recent files = %v, want just the file open", got)
	}

	openFile(t, application, driver, pedestal)

	if got := application.settings.RecentFiles; len(got) != 2 || !samePath(got[0], pedestal) {
		t.Errorf("recent files = %v, want the pedestal, then the statue", got)
	}

	driver.Menu("File", menuRecent, "statue.asset")
	driver.WaitFor("the statue to be opened again", func() bool {
		return application.opening == nil && application.document != nil &&
			filepath.Base(application.document.location.File) == "statue.asset"
	})

	// A file since deleted is taken off the list when picked.
	gone := filepath.Join(t.TempDir(), "gone.asset")
	application.rememberRecentFile(gone)

	driver.Menu("File", menuRecent, "gone.asset")
	driver.Frames(2)

	for _, file := range application.settings.RecentFiles {
		if samePath(file, gone) {
			t.Error("a file that is gone stays on the recent files")
		}
	}
}

// The recent files, the folder the open dialog starts in and the folder of
// the last export are there again in the next run.
func TestSettingsOutliveTheRun(t *testing.T) {
	config := t.TempDir()
	exports := t.TempDir()
	statue := fixtureFile(t, "statue.asset")

	first, driver := startAppIn(t, config)

	openFile(t, first, driver, statue)
	driver.Click(windowEntityList, "statue_entity")
	waitForEntity(first, driver, "statue_entity")

	first.dialogs = &fakeDialogs{answers: []string{filepath.Join(exports, "picked.png")}}
	first.showExport = true
	driver.Frames(2)
	driver.Click(popupExport, labelExportButton)
	driver.WaitFor("the export to be written", func() bool { return first.exportBusy() == "" })

	first.Close()

	second, driver := startAppIn(t, config)
	dialogs := &fakeDialogs{}
	second.dialogs = dialogs

	if got := second.settings.RecentFiles; len(got) != 1 || !samePath(got[0], statue) {
		t.Errorf("recent files of the next run = %v, want the statue", got)
	}

	if got := second.exportFolder(); !samePath(got, exports) {
		t.Errorf("the next run exports to %s, want %s, where the last export went", got, exports)
	}

	driver.Menu("File", "Open...")
	driver.WaitFor("the open dialog to be answered", func() bool { return second.dialog == nil && len(dialogs.asked()) == 1 })

	if asked := dialogs.asked(); !samePath(asked[0], filepath.Dir(statue)) {
		t.Errorf("the open dialog of the next run started in %s, want the statue's folder", asked[0])
	}
}
