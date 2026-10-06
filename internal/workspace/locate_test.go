package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tree writes files below a folder. A name ending in a slash is a folder.
func tree(t *testing.T, root string, files map[string]string) {
	t.Helper()

	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))

		if strings.HasSuffix(name, "/") {
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}

			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLocate(t *testing.T) {
	for _, test := range []struct {
		name     string
		files    []string
		open     string
		root     string
		relative string
	}{
		{
			// gfx holds an interface folder, which marks a root, in all three
			// games: the root is the folder above gfx all the same.
			name:     "game",
			files:    []string{"game/common/", "game/gfx/interface/", "game/gfx/models/statue/statue.asset"},
			open:     "game/gfx/models/statue/statue.asset",
			root:     "game",
			relative: "gfx/models/statue/statue.asset",
		},
		{
			name:     "DLC of a game",
			files:    []string{"game/common/", "game/dlc/dlc004_people/gfx/models/statue.asset"},
			open:     "game/dlc/dlc004_people/gfx/models/statue.asset",
			root:     "game",
			relative: "gfx/models/statue.asset",
		},
		{
			name:     "layer of a game",
			files:    []string{"game/in_game/common/", "game/main_menu/gfx/", "game/in_game/gfx/models/statue.asset"},
			open:     "game/in_game/gfx/models/statue.asset",
			root:     "game",
			relative: "in_game/gfx/models/statue.asset",
		},
		{
			name:     "layer of a DLC",
			files:    []string{"game/in_game/common/", "game/dlc/D008_phoenix/in_game/gfx/models/statue.asset"},
			open:     "game/dlc/D008_phoenix/in_game/gfx/models/statue.asset",
			root:     "game",
			relative: "in_game/gfx/models/statue.asset",
		},
		{
			// The folder all workshop mods are in is no root, and a mod is no
			// layer of it.
			name:     "mod",
			files:    []string{"workshop/123/.metadata/metadata.json", "workshop/123/gfx/models/statue.asset", "workshop/456/gfx/"},
			open:     "workshop/123/gfx/models/statue.asset",
			root:     "workshop/123",
			relative: "gfx/models/statue.asset",
		},
		{
			name:     "layer of a mod",
			files:    []string{"mod/.metadata/metadata.json", "mod/custom_layer/gfx/models/statue.asset"},
			open:     "mod/custom_layer/gfx/models/statue.asset",
			root:     "mod",
			relative: "custom_layer/gfx/models/statue.asset",
		},
		{
			name:     "case of the folders",
			files:    []string{"game/GFX/Models/statue.ASSET"},
			open:     "game/GFX/Models/statue.ASSET",
			root:     "game",
			relative: "GFX/Models/statue.ASSET",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()

			files := map[string]string{}
			for _, file := range test.files {
				files[file] = ""
			}

			tree(t, dir, files)

			location, err := Locate(filepath.Join(dir, filepath.FromSlash(test.open)))
			if err != nil {
				t.Fatal(err)
			}

			if want := filepath.Join(dir, filepath.FromSlash(test.root)); location.Root != want {
				t.Errorf("root = %s, want %s", location.Root, want)
			}

			if location.Relative != test.relative {
				t.Errorf("relative = %s, want %s", location.Relative, test.relative)
			}

			if want := filepath.Join(dir, filepath.FromSlash(test.open)); location.File != want {
				t.Errorf("file = %s, want %s", location.File, want)
			}
		})
	}
}

func TestLocateRefuses(t *testing.T) {
	dir := t.TempDir()
	tree(t, dir, map[string]string{
		"game/gfx/models/statue.txt":    "",
		"game/gfx/models/folder.asset/": "",
	})

	for name, path := range map[string]string{
		"another kind":   "game/gfx/models/statue.txt",
		"a folder":       "game/gfx/models/folder.asset",
		"a missing file": "game/gfx/models/missing.asset",
	} {
		t.Run(name, func(t *testing.T) {
			if location, err := Locate(filepath.Join(dir, filepath.FromSlash(path))); err == nil {
				t.Errorf("located %+v, want an error", location)
			}
		})
	}
}

func TestSourceName(t *testing.T) {
	for root, want := range map[string]string{
		filepath.FromSlash("/steam/common/Victoria 3/game"): "Victoria 3",
		filepath.FromSlash("/workshop/123"):                 "123",
		filepath.FromSlash("/mods/my_mod"):                  "my_mod",
	} {
		if got := SourceName(root); got != want {
			t.Errorf("SourceName(%s) = %s, want %s", root, got, want)
		}
	}
}

// A file in no gfx folder is a loose file: its folder stands in for the
// root, and it is kept by itself rather than by its folder.
func TestLocateLoose(t *testing.T) {
	dir := t.TempDir()
	tree(t, dir, map[string]string{"my_models/statue.asset": ""})

	location, err := Locate(filepath.Join(dir, "my_models", "statue.asset"))
	if err != nil {
		t.Fatal(err)
	}

	if !location.Loose || location.Root != filepath.Join(dir, "my_models") || location.Relative != "statue.asset" {
		t.Errorf("location = %+v, want the loose file in its folder", location)
	}

	if location.Key() != location.File {
		t.Errorf("key = %s, want the file", location.Key())
	}

	if game := (Location{Root: dir, File: filepath.Join(dir, "a.asset")}); game.Key() != dir {
		t.Errorf("key of a game's file = %s, want its root", game.Key())
	}
}
