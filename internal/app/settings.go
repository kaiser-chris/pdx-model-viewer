package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// maxRecentFiles is how many files the Recent menu lists.
const maxRecentFiles = 10

// settings are what the viewer remembers from one run to the next, beyond the
// window and the layout of the panels.
type settings struct {
	// RecentFiles are the asset files opened last, the latest first.
	RecentFiles []string `json:"recentFiles,omitempty"`

	// ExportFolder is where the last export went.
	ExportFolder string `json:"exportFolder,omitempty"`

	// ExportSize is the width and height last set for the pictures of an
	// export; none means the desktop's resolution.
	ExportSize [2]int32 `json:"exportSize,omitzero"`

	// LayoutVersion is the layout the saved panel arrangement was made for,
	// so that a panel added since can be put where it belongs. See
	// layoutVersion.
	LayoutVersion int `json:"layoutVersion,omitempty"`

	// ModGames are the games the viewer was told mods are for, by the root of
	// the mod. A mod whose own description does not name a game would
	// otherwise ask again every time it is opened.
	ModGames map[string]string `json:"modGames,omitempty"`
}

// loadSettings reads the settings, or none from a file that is missing or
// cannot be read.
func loadSettings(path string) settings {
	if path == "" {
		return settings{}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			warn(err)
		}

		return settings{}
	}

	var loaded settings
	if err := json.Unmarshal(data, &loaded); err != nil {
		warn(fmt.Errorf("read %s: %w", path, err))

		return settings{}
	}

	if len(loaded.RecentFiles) > maxRecentFiles {
		loaded.RecentFiles = loaded.RecentFiles[:maxRecentFiles]
	}

	return loaded
}

// saveSettings keeps the settings for the next run. It is called whenever
// they change, so that a viewer that does not close cleanly keeps them too.
func saveSettings(path string, kept settings) {
	if path == "" {
		return
	}

	data, err := json.MarshalIndent(kept, "", "  ")
	if err != nil {
		warn(err)

		return
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		warn(err)
	}
}

// rememberLayoutVersion records that the saved panel arrangement is up to
// date with the panels the viewer has.
func (a *App) rememberLayoutVersion() {
	a.settings.LayoutVersion = layoutVersion
	saveSettings(a.settingsFile, a.settings)
}

// withRecentFile puts a file at the top of a list of recent files, once, and
// keeps the list to its length.
func withRecentFile(files []string, file string) []string {
	file = filepath.Clean(file)

	kept := []string{file}

	for _, other := range files {
		if !samePath(other, file) && len(kept) < maxRecentFiles {
			kept = append(kept, other)
		}
	}

	return kept
}

// withoutRecentFile takes a file off a list of recent files.
func withoutRecentFile(files []string, file string) []string {
	var kept []string

	for _, other := range files {
		if !samePath(other, file) {
			kept = append(kept, other)
		}
	}

	return kept
}

// samePath reports whether two paths name the same file: ignoring case on
// Windows, whose file names do.
func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)

	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}

	return a == b
}

// rememberRecentFile adds a file that opened to the recent files.
func (a *App) rememberRecentFile(file string) {
	a.settings.RecentFiles = withRecentFile(a.settings.RecentFiles, file)
	saveSettings(a.settingsFile, a.settings)
}

// forgetRecentFile takes a file that is gone off the recent files.
func (a *App) forgetRecentFile(file string) {
	a.settings.RecentFiles = withoutRecentFile(a.settings.RecentFiles, file)
	saveSettings(a.settingsFile, a.settings)
}

// rememberExportFolder keeps where an export went, for the next one.
func (a *App) rememberExportFolder(folder string) {
	a.settings.ExportFolder = folder
	saveSettings(a.settingsFile, a.settings)
}

// setExportSize sets the size of the pictures of an export, kept within the
// sizes there can be, and keeps it for the next run.
func (a *App) setExportSize(size [2]int32) {
	size = [2]int32{clampExportSide(size[0]), clampExportSide(size[1])}

	a.exportSettings.size = size
	a.settings.ExportSize = size
	saveSettings(a.settingsFile, a.settings)
}

// openFolder is where the open dialog starts: the folder of the file opened
// last, in this run or an earlier one.
func (a *App) openFolder() string {
	if a.document != nil {
		return filepath.Dir(a.document.location.File)
	}

	if len(a.settings.RecentFiles) > 0 {
		return filepath.Dir(a.settings.RecentFiles[0])
	}

	return ""
}
