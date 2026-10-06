// Package workspace finds what an .asset file belongs to and loads its
// entities, in plain Go with nothing on the GPU, so that all of it can run off
// the thread that draws and be tested without a window.
//
// The viewer is not set up with a game folder. It is handed an .asset file,
// usually by the file association, and works out the game from where that
// file is: the folder above the gfx folder the file is in is the root of the
// game or mod, and every file the entities name is looked for from there.
//
// A file in no gfx folder, such as one a modder keeps on its own, is read as
// a loose file: its folder stands in for the root, the files it names are
// looked for around it and by their names next to it, and what is missing is
// drawn as well as it can be.
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kaiser-chris/pdx-parser-go/folders"
)

// Extension is the extension of the files the viewer opens.
const Extension = ".asset"

// gfxFolder is the folder the games read their asset files from, anywhere
// below it.
const gfxFolder = "gfx"

// dlcFolder is where a game keeps its DLCs, each a root of its own.
const dlcFolder = "dlc"

// knownLayers are the folders Europa Universalis 5 splits its game files
// into, each with a gfx folder of its own.
var knownLayers = []string{"in_game", "main_menu", "loading_screen"}

// Location is where an asset file is within a game or mod.
type Location struct {
	// File is the asset file, as an absolute path.
	File string

	// Relative is the file's path below the root of the game, DLC or mod it
	// belongs to, with forward slashes, the way the game itself refers to it:
	// gfx/models/... or, in a game with layers, in_game/gfx/models/...
	Relative string

	// Root is the folder the game's files are read from: the game folder of
	// an installation, which brings its DLCs with it, or the root of a mod.
	// For a loose file it is the file's folder.
	Root string

	// Loose is set for a file in no gfx folder, which belongs to no game.
	Loose bool
}

// Key is what what is read for a file is kept by: its game's root, or for a
// loose file the file itself, since a loose file is read on its own.
func (l Location) Key() string {
	if l.Loose {
		return l.File
	}

	return l.Root
}

// Locate works out the game or mod an asset file belongs to.
//
// The file has to be somewhere below a gfx folder, which is where the games
// read asset files from. The folder holding that gfx folder is the root,
// except for:
//
//   - a layer, such as in_game in Europa Universalis 5, whose parent is the
//     root;
//   - a DLC, below the game's dlc folder, whose game is the root, so that the
//     files the DLC uses from the game itself are found too.
func Locate(file string) (Location, error) {
	absolute, err := filepath.Abs(file)
	if err != nil {
		return Location{}, err
	}

	info, err := os.Stat(absolute)
	if err != nil {
		return Location{}, err
	}

	if info.IsDir() {
		return Location{}, fmt.Errorf("%s is a folder, not an %s file", absolute, Extension)
	}

	if !strings.EqualFold(filepath.Ext(absolute), Extension) {
		return Location{}, fmt.Errorf("%s is not an %s file", filepath.Base(absolute), Extension)
	}

	gfx, ok := enclosingGfx(absolute)
	if !ok {
		return Location{File: absolute, Relative: filepath.Base(absolute), Root: filepath.Dir(absolute), Loose: true}, nil
	}

	// The root of the game, DLC or mod the file itself is in.
	own := filepath.Dir(gfx)
	if isLayer(own) {
		own = filepath.Dir(own)
	}

	relative, err := filepath.Rel(own, absolute)
	if err != nil {
		return Location{}, err
	}

	root := own
	if parent := filepath.Dir(own); strings.EqualFold(filepath.Base(parent), dlcFolder) {
		root = filepath.Dir(parent)
	}

	return Location{File: absolute, Relative: filepath.ToSlash(relative), Root: root}, nil
}

// enclosingGfx finds the gfx folder nearest above a file.
//
// The nearest one, rather than the first folder above the file that looks
// like a root, because the gfx folder itself holds folders named like the
// markers of a root: interface, in all three games.
func enclosingGfx(file string) (string, bool) {
	for dir := filepath.Dir(file); ; {
		if strings.EqualFold(filepath.Base(dir), gfxFolder) {
			return dir, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}

		dir = parent
	}
}

// isLayer reports whether a folder holding a gfx folder is a layer of a game
// or mod rather than its root: one of the layers the games are known to use,
// or a folder whose parent is a mod's root by its description of itself.
//
// A parent that holds game files of its own is a root already, which makes
// the folder below it a part of the game rather than a layer. The parent of a
// mod's root, such as the folder all workshop mods are in, holds neither game
// files nor a description, so a mod is not taken for a layer of it.
func isLayer(dir string) bool {
	parent := filepath.Dir(dir)
	if parent == dir || holdsMarker(parent) {
		return false
	}

	if slices.ContainsFunc(knownLayers, func(layer string) bool { return strings.EqualFold(layer, filepath.Base(dir)) }) {
		return true
	}

	return exists(filepath.Join(parent, ".metadata")) || exists(filepath.Join(parent, "descriptor.mod"))
}

func holdsMarker(dir string) bool {
	for _, marker := range folders.RootMarkers {
		if info, err := os.Stat(filepath.Join(dir, marker)); err == nil && info.IsDir() {
			return true
		}
	}

	return false
}

func exists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}
