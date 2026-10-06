package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// The folders Steam keeps an installation and its applications in.
const (
	// steamapps is the folder of a library the applications and their
	// manifests are in.
	steamapps = "steamapps"

	// commonFolder is the folder below steamapps the games themselves are in.
	commonFolder = "common"

	// libraryFolders is the file listing the libraries of an installation,
	// which is in its own steamapps folder.
	libraryFolders = "libraryfolders.vdf"
)

// Installation is a game found on this machine.
type Installation struct {
	// Product is which of the games it is.
	Product Product

	// Install is the folder the game is installed in, with its engine folders
	// and its own game folder below it.
	Install string

	// Root is the game folder below Install, which the game's asset files are
	// read from.
	Root string
}

// String names an installation the way the chooser lists it.
func (i Installation) String() string {
	if i.Install == "" {
		return i.Product.String()
	}

	return i.Product.String() + "  (" + i.Install + ")"
}

// Installations returns the installations of the three games found on this
// machine, in the order of Products, and by library within a game.
//
// All three are on Steam alone, so Steam is what is asked: where it keeps
// itself, which libraries it has, and which of the games each library holds.
// The games themselves are read the way the engine reads them, from the game
// folder of an installation.
func Installations() []Installation {
	return installations(steamRoots)
}

// installations is Installations over the folders a system keeps Steam in,
// which the tests hand a folder of their own.
func installations(roots func() []string) []Installation {
	var found []Installation

	for _, library := range steamLibraries(roots) {
		for _, product := range Products {
			if install, ok := installedIn(library, product); ok {
				found = append(found, install)
			}
		}
	}

	return found
}

// InstallationAt is the installation of a game whose files are in a folder the
// user picked, which may be the game folder itself or the installation above
// it. It is what a game the viewer was told about by hand is read as, since
// nothing about the folder says better.
func InstallationAt(folder string, product Product) (Installation, bool) {
	absolute, err := filepath.Abs(folder)
	if err != nil {
		return Installation{}, false
	}

	// The folder holding a game folder below it is the installation, and is
	// checked first: the engine's own folders sit next to the game folder,
	// and are looked for from the installation.
	if below := filepath.Join(absolute, gameFolder); isRoot(below) {
		return Installation{Product: product, Install: absolute, Root: below}, true
	}

	if isRoot(absolute) {
		return Installation{Product: product, Install: filepath.Dir(absolute), Root: absolute}, true
	}

	return Installation{}, false
}

// steamLibraries are the folders Steam keeps its applications in: the root of
// every Steam installation found on this machine, and every library each of
// them lists. A folder that holds no steamapps folder is left out, which is
// what tells apart a library that was moved or deleted.
func steamLibraries(roots func() []string) []string {
	var libraries []string

	for _, root := range roots() {
		for _, library := range append([]string{root}, readLibraryFolders(filepath.Join(root, steamapps))...) {
			if !isDirectory(filepath.Join(library, steamapps)) {
				continue
			}

			// The same library is listed by every installation that knows it,
			// and an installation that is a link to another lists it twice.
			if !slices.ContainsFunc(libraries, func(seen string) bool { return sameFolder(seen, library) }) {
				libraries = append(libraries, library)
			}
		}
	}

	return libraries
}

// installedIn reports whether a library holds a game, and where it is. The
// game is looked for under both the folder name its appmanifest gives, which
// is where Steam put it, and the name the game is installed under by default.
func installedIn(library string, product Product) (Installation, bool) {
	apps := filepath.Join(library, steamapps)

	names := []string{product.installedFolder()}
	if installed := readAppManifest(filepath.Join(apps, "appmanifest_"+product.steamApp()+".acf")); installed != "" {
		names = append([]string{installed}, names...)
	}

	for _, name := range names {
		install := filepath.Join(apps, commonFolder, name)

		// A folder left behind by a game that was uninstalled, or moved to
		// another library, is no installation: its game files are gone.
		if root := filepath.Join(install, gameFolder); isRoot(root) {
			return Installation{Product: product, Install: install, Root: root}, true
		}
	}

	return Installation{}, false
}

// readLibraryFolders reads the paths of the libraries an installation has,
// from the libraryfolders.vdf of its steamapps folder.
//
// Both shapes of the file are read: the one Steam writes now, a group per
// library with a path in it, and the older one, a path for each numbered key.
func readLibraryFolders(steamapps string) []string {
	document, ok := readKeyValues(filepath.Join(steamapps, libraryFolders))
	if !ok {
		return nil
	}

	folders := document.group("libraryfolders")
	if folders == nil {
		folders = document
	}

	var libraries []string

	for _, pair := range folders.pairs {
		switch value := pair.value.(type) {
		case *keyValues:
			if path := value.get("path"); path != "" {
				libraries = append(libraries, path)
			}
		case string:
			// The older shape, "1" "D:\\SteamLibrary", next to the entries
			// that count something rather than name a folder.
			if value != "" && isNumber(pair.key) {
				libraries = append(libraries, value)
			}
		}
	}

	return libraries
}

// readAppManifest reads the folder a game is installed in, from the
// appmanifest Steam writes beside it. Steam is the only thing that knows,
// since a game can be installed under any name.
func readAppManifest(path string) string {
	document, ok := readKeyValues(path)
	if !ok {
		return ""
	}

	state := document.group("AppState")
	if state == nil {
		state = document
	}

	return state.get("installdir")
}

// sameFolder reports whether two paths name the same folder, ignoring case on
// Windows, whose file names do.
func sameFolder(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)

	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}

	return a == b
}

// isNumber reports whether a key is one of the numbers the older
// libraryfolders.vdf files its libraries under.
func isNumber(key string) bool {
	if key == "" {
		return false
	}

	for _, digit := range key {
		if digit < '0' || digit > '9' {
			return false
		}
	}

	return true
}

// keyValues is a Valve KeyValues document, the format Steam writes
// libraryfolders.vdf and its appmanifest files in: values with names, and
// groups of them with names, nested in braces.
//
// The pairs keep the order the file writes them in, which is the order Steam
// reads its libraries in.
type keyValues struct {
	pairs []keyValuePair
}

// keyValuePair is one named value, which is either a string or a group.
type keyValuePair struct {
	key   string
	value any
}

// get returns the value filed under a name, or nothing when it is not a
// string. Names are matched ignoring case, the way Steam spells them.
func (k *keyValues) get(name string) string {
	for _, pair := range k.pairs {
		if text, ok := pair.value.(string); ok && strings.EqualFold(pair.key, name) {
			return text
		}
	}

	return ""
}

// group returns the group filed under a name, or nothing.
func (k *keyValues) group(name string) *keyValues {
	for _, pair := range k.pairs {
		if found, ok := pair.value.(*keyValues); ok && strings.EqualFold(pair.key, name) {
			return found
		}
	}

	return nil
}

// readKeyValues reads a KeyValues file. A file that is not there, or that
// cannot be read as one, reads as nothing: Steam's files are nobody's to
// validate, and a viewer that refuses to start over one is worse than one
// that finds no games.
func readKeyValues(path string) (*keyValues, bool) {
	text, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	document, err := parseKeyValues(string(text))
	if err != nil {
		return nil, false
	}

	return document, true
}

// parseKeyValues reads a KeyValues document: "key" "value" pairs and
// "key" { ... } groups, in whatever whitespace and // comments surround them.
func parseKeyValues(text string) (*keyValues, error) {
	reader := &keyValuesReader{text: text}

	document, err := reader.group(true)
	if err != nil {
		return nil, err
	}

	return document, nil
}

// keyValuesReader reads a document a token at a time.
type keyValuesReader struct {
	text string
	at   int
}

// group reads the pairs up to the closing brace, or to the end of the file
// when it is the outermost group, which has none.
func (r *keyValuesReader) group(outermost bool) (*keyValues, error) {
	found := &keyValues{}

	for {
		r.skip()

		if r.at == len(r.text) {
			if outermost {
				return found, nil
			}

			return nil, errors.New("a group is left open")
		}

		if r.text[r.at] == '}' {
			if outermost {
				return nil, errors.New("a brace closes nothing")
			}

			r.at++

			return found, nil
		}

		key, err := r.token()
		if err != nil {
			return nil, err
		}

		r.skip()

		if r.at < len(r.text) && r.text[r.at] == '{' {
			r.at++

			nested, err := r.group(false)
			if err != nil {
				return nil, err
			}

			found.pairs = append(found.pairs, keyValuePair{key: key, value: nested})

			continue
		}

		value, err := r.token()
		if err != nil {
			return nil, err
		}

		found.pairs = append(found.pairs, keyValuePair{key: key, value: value})
	}
}

// token reads the quoted text at the reader's place.
func (r *keyValuesReader) token() (string, error) {
	if r.at == len(r.text) || r.text[r.at] != '"' {
		return "", errors.New("a value is expected")
	}

	r.at++

	var text strings.Builder

	for r.at < len(r.text) {
		switch character := r.text[r.at]; character {
		case '"':
			r.at++

			return text.String(), nil
		case '\\':
			// Steam's own files escape the separators of a Windows path.
			r.at++

			if r.at == len(r.text) {
				break
			}

			switch escaped := r.text[r.at]; escaped {
			case 'n':
				text.WriteByte('\n')
			case 't':
				text.WriteByte('\t')
			default:
				text.WriteByte(escaped)
			}

			r.at++
		default:
			text.WriteByte(character)
			r.at++
		}
	}

	return "", errors.New("a value is left open")
}

// skip steps over the whitespace and the comments between tokens.
func (r *keyValuesReader) skip() {
	for r.at < len(r.text) {
		switch {
		case r.text[r.at] == '/' && r.at+1 < len(r.text) && r.text[r.at+1] == '/':
			for r.at < len(r.text) && r.text[r.at] != '\n' {
				r.at++
			}
		case strings.ContainsRune(" \t\r\n\v\f", rune(r.text[r.at])):
			r.at++
		default:
			return
		}
	}
}
