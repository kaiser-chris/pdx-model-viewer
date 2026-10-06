package workspace

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The libraryfolders.vdf Steam writes now: a group per library with its path
// in it, and the applications it holds below that.
const currentLibraryFolders = `"libraryfolders"
{
	"0"
	{
		"path"		"D:\\Games\\Steam"
		"label"		""
		"contentid"		"-1028809783264187056"
		"apps"
		{
			"1158310"		"0"
			"3450310"		"15921371989"
		}
	}
	"1"
	{
		"path"		"E:\\SteamLibrary"
		"label"		""
		"apps"
		{
			"529340"		"18402068979"
		}
	}
	"2"
	{
		"path"		"C:\\SteamLibrary"
		"totalsize"		"499036188672"
		"apps"
		{
		}
	}
}
`

// The shape the older Steam clients wrote: a path for each numbered key, next
// to the entries that count something rather than name a folder.
const oldLibraryFolders = `"libraryfolders"
{
	"TimeNextStatsReport"	"1791180913"
	"ContentStatsID"		"-1028809783264187056"
	// a comment, which the files sometimes carry
	"1"		"D:\\Games\\Steam"
	"2"		"E:\\SteamLibrary"
}
`

// appManifest is an appmanifest as Steam writes it, trimmed to what the viewer
// reads.
func appManifest(name, folder string) string {
	return `"AppState"
{
	"appid"		"` + name + `"
	"name"		"` + name + `"
	"StateFlags"		"4"
	"installdir"		"` + folder + `"
	"UserConfig"
	{
		"language"		"english"
	}
}
`
}

// steamLibrary writes a library holding the games named, each as the folder
// Steam installs it in, and returns the library.
func steamLibrary(t *testing.T, where string, games map[string]string) string {
	t.Helper()

	files := map[string]string{"steamapps/": ""}

	for app, folder := range games {
		files["steamapps/appmanifest_"+app+".acf"] = appManifest(app, folder)
		// A game folder with the marker that makes it one.
		files["steamapps/common/"+folder+"/game/common/"] = ""
	}

	tree(t, where, files)

	return where
}

// TestParseKeyValues reads the shape of the files Steam writes: named values,
// groups of them, escapes in a Windows path, and comments.
func TestParseKeyValues(t *testing.T) {
	document, err := parseKeyValues(currentLibraryFolders)
	if err != nil {
		t.Fatal(err)
	}

	folders := document.group("libraryfolders")
	if folders == nil {
		t.Fatal("no libraryfolders group")
	}

	if len(folders.pairs) != 3 {
		t.Fatalf("libraries = %d, want 3", len(folders.pairs))
	}

	// In the order the file writes them, which is the order Steam reads them.
	for index, want := range []string{`D:\Games\Steam`, `E:\SteamLibrary`, `C:\SteamLibrary`} {
		library, ok := folders.pairs[index].value.(*keyValues)
		if !ok {
			t.Fatalf("library %d is no group", index)
		}

		if got := library.get("path"); got != want {
			t.Errorf("path of library %d = %q, want %q", index, got, want)
		}
	}

	if got := folders.pairs[0].value.(*keyValues).group("apps").get("1158310"); got != "0" {
		t.Errorf("the apps of the first library read as %q", got)
	}

	// A value with an escaped quote, and a comment that hides pairs.
	escaped, err := parseKeyValues("\"root\"\n{\n\t\"a\"\t\"x\\\"y\"\n\t// \"b\" \"z\"\n\t\"c\" \"w\"\n}\n")
	if err != nil {
		t.Fatal(err)
	}

	within := escaped.group("root")
	if within == nil {
		t.Fatal("no root group")
	}

	if got := within.get("a"); got != `x"y` {
		t.Errorf("an escaped quote reads as %q", got)
	}

	if got := within.get("b"); got != "" {
		t.Errorf("a commented pair reads as %q, want nothing", got)
	}

	if got := within.get("c"); got != "w" {
		t.Errorf("the pair after a comment reads as %q", got)
	}
}

func TestParseKeyValuesRefuses(t *testing.T) {
	for name, text := range map[string]string{
		"an unclosed value":      `"a" "b`,
		"an unclosed group":      "\"a\"\n{\n\t\"b\" \"c\"\n",
		"a brace alone":          `}`,
		"a value without a name": `{ "a" "b" }`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseKeyValues(text); err == nil {
				t.Error("want an error")
			}
		})
	}

	if document, err := parseKeyValues(""); err != nil || len(document.pairs) != 0 {
		t.Errorf("an empty file reads as %+v, %v", document, err)
	}
}

// TestReadLibraryFolders reads both shapes of the file Steam lists its
// libraries in.
func TestReadLibraryFolders(t *testing.T) {
	for name, test := range map[string]struct {
		text string
		want []string
	}{
		"the current shape": {currentLibraryFolders, []string{`D:\Games\Steam`, `E:\SteamLibrary`, `C:\SteamLibrary`}},
		"the older shape":   {oldLibraryFolders, []string{`D:\Games\Steam`, `E:\SteamLibrary`}},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			tree(t, dir, map[string]string{"libraryfolders.vdf": test.text})

			if got := readLibraryFolders(dir); !slices.Equal(got, test.want) {
				t.Errorf("libraries = %q, want %q", got, test.want)
			}
		})
	}

	if got := readLibraryFolders(t.TempDir()); got != nil {
		t.Errorf("a folder with no libraryfolders.vdf reads as %q", got)
	}
}

// TestInstallations finds the three games the way a machine has them: an
// installation with a library of its own, a game whose appmanifest names a
// folder of its own, and a library that is not there any more.
func TestInstallations(t *testing.T) {
	dir := t.TempDir()

	root := steamLibrary(t, filepath.Join(dir, "Steam"), map[string]string{
		"1158310": "Crusader Kings III",
		"3450310": "Europa Universalis V",
	})

	// A second library, where Victoria 3 is installed under a name of its
	// own, which only its appmanifest knows.
	library := steamLibrary(t, filepath.Join(dir, "SteamLibrary"), map[string]string{
		"529340": "Victoria 3 - some edition",
	})

	// A library the file lists, which was moved or deleted since.
	tree(t, root, map[string]string{
		"steamapps/libraryfolders.vdf": `"libraryfolders"
{
	"0"
	{
		"path"		"` + strings.ReplaceAll(root, `\`, `\\`) + `"
	}
	"1"
	{
		"path"		"` + strings.ReplaceAll(library, `\`, `\\`) + `"
	}
	"2"
	{
		"path"		"` + strings.ReplaceAll(filepath.Join(dir, "Gone"), `\`, `\\`) + `"
	}
}
`,
	})

	found := installations(func() []string { return []string{root} })

	var got []string
	for _, install := range found {
		got = append(got, install.String())
		// The game folder of an installation, which is what the files are
		// read from.
		if filepath.Base(install.Root) != gameFolder {
			t.Errorf("%s is no game folder", install.Root)
		}

		if !isDirectory(filepath.Join(install.Install, "game", "common")) {
			t.Errorf("%s holds no game files", install.Install)
		}
	}

	want := []string{
		"Crusader Kings 3  (" + filepath.Join(root, "steamapps", "common", "Crusader Kings III") + ")",
		"Europa Universalis 5  (" + filepath.Join(root, "steamapps", "common", "Europa Universalis V") + ")",
		"Victoria 3  (" + filepath.Join(library, "steamapps", "common", "Victoria 3 - some edition") + ")",
	}
	if !slices.Equal(got, want) {
		t.Errorf("installations =\n\t%s\nwant\n\t%s", strings.Join(got, "\n\t"), strings.Join(want, "\n\t"))
	}
}

// TestInstallationsWithoutAManifest falls back on the folder a game is
// installed in by default when Steam's manifest for it cannot be read. A game
// that keeps its files in layers rather than in the game folder itself counts
// as installed there, and a folder left behind by one that is gone does not.
func TestInstallationsWithoutAManifest(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Steam")

	tree(t, root, map[string]string{
		steamapps + "/": "",
		// Victoria 3 keeps its files in the game folder itself.
		steamapps + "/common/Victoria 3/game/common/": "",
		// Europa Universalis 5 splits them into in_game and the like, so its
		// game folder holds no marker of its own.
		steamapps + "/common/Europa Universalis V/game/in_game/common/": "",
		// A folder left behind by a game that was uninstalled.
		steamapps + "/common/Crusader Kings III/game/": "",
	})

	found := installations(func() []string { return []string{root} })

	var products []string
	for _, install := range found {
		products = append(products, install.Product.String())
	}

	if want := []string{"Victoria 3", "Europa Universalis 5"}; !slices.Equal(products, want) {
		t.Errorf("products = %q, want %q", products, want)
	}
}

// TestInstallationsOfNoSteam keeps a machine without Steam from finding a game
// in nothing.
func TestInstallationsOfNoSteam(t *testing.T) {
	if found := installations(func() []string { return nil }); len(found) != 0 {
		t.Errorf("found %+v without any Steam", found)
	}

	if found := installations(func() []string { return []string{filepath.Join(t.TempDir(), "missing")} }); len(found) != 0 {
		t.Errorf("found %+v in a folder that is not there", found)
	}
}

// TestInstallationAt reads the installation a folder the user picked stands
// for: the game folder itself, or the installation above it, whether the game
// keeps its files there or in layers below it.
func TestInstallationAt(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Statues", "game")
	tree(t, root, map[string]string{"common/": "", "gfx/models/statue.asset": ""})

	game, ok := InstallationAt(root, Victoria3)
	if !ok || game.Root != root || game.Install != filepath.Dir(root) {
		t.Errorf("the game folder itself reads as %+v, %v", game, ok)
	}

	install, ok := InstallationAt(filepath.Dir(root), Victoria3)
	if !ok || install.Root != root || install.Install != filepath.Dir(root) {
		t.Errorf("the installation above it reads as %+v, %v", install, ok)
	}

	if wrong, ok := InstallationAt(t.TempDir(), Victoria3); ok {
		t.Errorf("a folder with no game files reads as %+v", wrong)
	}

	// Europa Universalis 5 keeps its files in layers, so its game folder
	// holds no common folder of its own.
	layered := filepath.Join(t.TempDir(), "Europa Universalis V", "game")
	tree(t, layered, map[string]string{"in_game/common/": ""})

	if found, ok := InstallationAt(layered, EuropaUniversalis5); !ok || found.Root != layered {
		t.Errorf("a game folder of layers reads as %+v, %v", found, ok)
	}
}
