package workspace

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// installedGames are the game folders PDX_GAME_DIR points at, several of them
// separated the way PATH is, or the test is skipped.
func installedGames(t *testing.T) []string {
	t.Helper()

	value := os.Getenv("PDX_GAME_DIR")
	if value == "" {
		t.Skip("set PDX_GAME_DIR to one or more game folders to run this test")
	}

	return filepath.SplitList(value)
}

// assetFiles lists every asset file below a folder, its DLCs and layers
// included.
func assetFiles(t *testing.T, root string) []string {
	t.Helper()

	var files []string

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), Extension) {
			files = append(files, path)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	return files
}

// TestLocateInstalled locates every asset file a game ships, its DLCs' and
// its layers' included, and expects each to lead back to the game folder.
func TestLocateInstalled(t *testing.T) {
	for _, root := range installedGames(t) {
		t.Run(SourceName(root), func(t *testing.T) {
			root = filepath.Clean(root)

			files := assetFiles(t, root)
			if len(files) == 0 {
				t.Fatalf("%s has no asset files", root)
			}

			wrong := 0

			for _, file := range files {
				location, err := Locate(file)

				switch {
				case err != nil:
					t.Errorf("%s: %v", file, err)
					wrong++
				case !strings.EqualFold(location.Root, root):
					t.Errorf("%s: root = %s, want %s", file, location.Root, root)
					wrong++
				}

				if wrong > 10 {
					t.Fatal("too many to list them all")
				}
			}

			t.Logf("located %d asset files", len(files))
		})
	}
}

// TestLoadInstalled lists every asset file of a game and loads the entities
// of a sample of them. An entity that draws no mesh, such as one that only
// holds attachments, has nothing to show and is left out.
//
// A few of the shipped files leave a block unclosed or close one too many.
// The game reads them all the same, and so does the viewer, which shows the
// problem alongside; here they are only logged.
func TestLoadInstalled(t *testing.T) {
	const every = 20

	for _, root := range installedGames(t) {
		t.Run(SourceName(root), func(t *testing.T) {
			opened, err := OpenGame(root)
			if err != nil {
				t.Fatal(err)
			}

			t.Logf("read %d entities in %s", opened.Entities(), opened.Took)

			loaded, failed, syntax := 0, 0, 0

			for index, file := range assetFiles(t, root) {
				location, err := Locate(file)
				if err != nil {
					t.Fatal(err)
				}

				listing := List(location)
				syntax += len(listing.Diagnostics)

				if index%every != 0 {
					continue
				}

				for _, name := range listing.Entities {
					if _, err := opened.Load(name); err != nil {
						if !strings.Contains(err.Error(), "draws no mesh") {
							t.Logf("%s: %v", name, err)
							failed++
						}

						continue
					}

					loaded++
				}
			}

			t.Logf("loaded %d entities, %d failed; %d syntax problems in the files", loaded, failed, syntax)

			if failed > 0 {
				t.Errorf("%d entities that draw a mesh failed to load", failed)
			}
		})
	}
}
