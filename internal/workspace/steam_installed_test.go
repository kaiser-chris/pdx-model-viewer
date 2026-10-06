package workspace

import (
	"path/filepath"
	"slices"
	"testing"
)

// TestInstallationsInstalled asks Steam where the games are, on a machine
// where the tests are told where they are. Every installation found has to be
// a game folder that is really there, and none of them may be another one's.
//
// It is skipped unless PDX_GAME_DIR names one or more game folders, the way
// PATH is. The folders named there need not be found: a game folder that was
// copied somewhere, or an installation of a branch of a game, is not what
// Steam installed, and finding it is not what is being checked.
func TestInstallationsInstalled(t *testing.T) {
	installedGames(t)

	found := Installations()

	for _, install := range found {
		t.Logf("%s at %s", install.Product, install.Root)

		if !isDirectory(install.Root) {
			t.Errorf("%s is not there", install.Root)

			continue
		}

		if !isRoot(install.Root) {
			t.Errorf("%s holds no game files", install.Root)
		}

		if install.Install != filepath.Dir(install.Root) {
			t.Errorf("%s is not below %s", install.Root, install.Install)
		}
	}

	// Two installations of one game are told apart by their folders, which is
	// what a chooser between them offers.
	for index, install := range found {
		if slices.ContainsFunc(found[:index], func(seen Installation) bool {
			return seen.Product == install.Product && sameFolder(seen.Root, install.Root)
		}) {
			t.Errorf("%s is found twice at %s", install.Product, install.Root)
		}
	}
}
