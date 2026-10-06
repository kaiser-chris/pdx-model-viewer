package app

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestWithRecentFile(t *testing.T) {
	files := withRecentFile(nil, "a.asset")
	files = withRecentFile(files, "b.asset")

	// Opening one again moves it to the top, once.
	files = withRecentFile(files, "a.asset")
	if want := []string{"a.asset", "b.asset"}; !slices.Equal(files, want) {
		t.Errorf("recent files = %v, want %v", files, want)
	}

	for index := range 12 {
		files = withRecentFile(files, fmt.Sprintf("%d.asset", index))
	}

	if len(files) != maxRecentFiles || files[0] != "11.asset" || files[maxRecentFiles-1] != "2.asset" {
		t.Errorf("after twelve more: %v, want the latest ten", files)
	}

	if files = withoutRecentFile(files, "5.asset"); slices.Contains(files, "5.asset") || len(files) != maxRecentFiles-1 {
		t.Errorf("after taking one off: %v", files)
	}
}

// Windows ignores the case of file names, so one file opened as A and as a
// is listed once.
func TestRecentFilesIgnoreCaseOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("file names ignore case on Windows only")
	}

	if files := withRecentFile([]string{`C:\Game\Statue.asset`}, `c:\game\statue.asset`); len(files) != 1 {
		t.Errorf("recent files = %v, want the file once", files)
	}
}

func TestSettingsAreKept(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	if got := loadSettings(path); len(got.RecentFiles) != 0 || got.ExportFolder != "" {
		t.Errorf("settings of no file = %+v, want none", got)
	}

	kept := settings{
		RecentFiles:  []string{"a.asset", "b.asset"},
		ExportFolder: "out",
		ExportSize:   [2]int32{800, 600},
		ModGames:     map[string]string{"/mods/gate": "victoria3"},
	}
	saveSettings(path, kept)

	got := loadSettings(path)
	if !slices.Equal(got.RecentFiles, kept.RecentFiles) || got.ExportFolder != kept.ExportFolder || got.ExportSize != kept.ExportSize {
		t.Errorf("read back %+v, want %+v", got, kept)
	}

	if !maps.Equal(got.ModGames, kept.ModGames) {
		t.Errorf("the games kept for mods read back as %v, want %v", got.ModGames, kept.ModGames)
	}

	if err := os.WriteFile(path, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := loadSettings(path); len(got.RecentFiles) != 0 {
		t.Errorf("settings of a broken file = %+v, want none", got)
	}
}
