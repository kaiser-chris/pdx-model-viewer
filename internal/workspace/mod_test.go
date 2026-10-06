package workspace

import (
	"path/filepath"
	"testing"
)

// The descriptions a mod can carry, written the way the games write them.
const (
	victoriaModMetadata = `{ "name": "Manaflow", "id": "com.github.kaiser-chris.gate", "game_id": "victoria3" }`
	eu5ModMetadata      = `{ "name": "Kaiser UI", "id": "com.github.kaiser-chris.kaiserui", "game_id": "eu5" }`
	namelessModMetadata = `{ "id": "com.github.someone.quiet", "version": "1.0" }`
	ck3ModDescriptor    = "version=\"v1.0\"\ntags={\n\t\"Balance\"\n}\nname=\"More Game Rules\"\nsupported_version=\"1.19.*\"\n"
)

// TestDescribe reads what each kind of mod folder says about itself, and what
// a folder that is no mod says.
func TestDescribe(t *testing.T) {
	for _, test := range []struct {
		name    string
		files   map[string]string
		mod     bool
		product Product
		modName string
	}{
		{
			name:    "a Victoria 3 mod names its game",
			files:   map[string]string{".metadata/metadata.json": victoriaModMetadata},
			mod:     true,
			product: Victoria3,
			modName: "Manaflow",
		},
		{
			// Editors on Windows write the mark, against the spec, and the
			// games read the file all the same.
			name:    "a byte order mark does not stop it",
			files:   map[string]string{".metadata/metadata.json": "\xef\xbb\xbf" + victoriaModMetadata},
			mod:     true,
			product: Victoria3,
			modName: "Manaflow",
		},
		{
			name:    "a Europa Universalis 5 mod names its game",
			files:   map[string]string{".metadata/metadata.json": eu5ModMetadata},
			mod:     true,
			product: EuropaUniversalis5,
			modName: "Kaiser UI",
		},
		{
			// The classifier is optional, which is what makes the game the
			// user's to say.
			name:  "a metadata.json without a classifier decides nothing",
			files: map[string]string{".metadata/metadata.json": namelessModMetadata},
			mod:   true,
		},
		{
			name:    "a Crusader Kings 3 mod is told by its descriptor alone",
			files:   map[string]string{"descriptor.mod": ck3ModDescriptor},
			mod:     true,
			product: CrusaderKings3,
			modName: "More Game Rules",
		},
		{
			// The classifier is the newer description of the two, so it
			// decides even next to a descriptor.mod.
			name: "a classifier wins over a descriptor",
			files: map[string]string{
				".metadata/metadata.json": victoriaModMetadata,
				"descriptor.mod":          ck3ModDescriptor,
			},
			mod:     true,
			product: Victoria3,
			modName: "Manaflow",
		},
		{
			// Both descriptions and neither naming a game: nothing says
			// whether the descriptor belongs to the mod or the metadata does.
			name: "both descriptions without a classifier decide nothing",
			files: map[string]string{
				".metadata/metadata.json": namelessModMetadata,
				"descriptor.mod":          ck3ModDescriptor,
			},
			mod:     true,
			modName: "More Game Rules",
		},
		{
			// A folder named after the mod, with no metadata.json in it: the
			// folder is the marker even so.
			name:  "a .metadata folder with no metadata.json is still a mod",
			files: map[string]string{".metadata/thumbnail.png": "png"},
			mod:   true,
		},
		{
			name:  "a metadata.json that cannot be read decides nothing",
			files: map[string]string{".metadata/metadata.json": "{ not json"},
			mod:   true,
		},
		{
			name: "a game folder is no mod",
			files: map[string]string{
				"common/":                 "",
				"gfx/models/statue.asset": "",
			},
		},
		{
			name:  "an empty folder is no mod",
			files: map[string]string{},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			tree(t, dir, test.files)

			described := Describe(dir)

			if described.Mod != test.mod {
				t.Errorf("mod = %v, want %v", described.Mod, test.mod)
			}

			if described.Product != test.product {
				t.Errorf("product = %v, want %v", described.Product, test.product)
			}

			if described.Name != test.modName {
				t.Errorf("name = %q, want %q", described.Name, test.modName)
			}
		})
	}
}

// TestDescribeOfNothing keeps a location without a folder from being read as a
// mod at the root of the disk.
func TestDescribeOfNothing(t *testing.T) {
	if described := Describe(""); described.Mod {
		t.Errorf("described %+v, want nothing", described)
	}
}

// TestProductOfKey reads the identifiers the games and their modders write.
func TestProductOfKey(t *testing.T) {
	for key, want := range map[string]Product{
		"victoria3":            Victoria3,
		"Victoria3":            Victoria3,
		"VIC3":                 Victoria3,
		"vic3":                 Victoria3,
		"ck3":                  CrusaderKings3,
		"crusader_kings_3":     CrusaderKings3,
		"CrusaderKingsIII":     CrusaderKings3,
		"eu5":                  EuropaUniversalis5,
		"europa_universalis_5": EuropaUniversalis5,
		"Europa Universalis V": EuropaUniversalis5,
		"":                     NoProduct,
		"stellaris":            NoProduct,
		"victoria":             NoProduct,
	} {
		if got := ProductOfKey(key); got != want {
			t.Errorf("ProductOfKey(%q) = %v, want %v", key, got, want)
		}
	}

	// Every product survives being written down and read back, which is what
	// a remembered choice is kept by.
	for _, product := range Products {
		if got := ProductOfKey(product.Key()); got != product {
			t.Errorf("%v written as %q reads back as %v", product, product.Key(), got)
		}
	}
}

// TestLocateMod locates a file of a mod, which is read with the game it names.
func TestLocateMod(t *testing.T) {
	dir := t.TempDir()
	tree(t, dir, map[string]string{
		".metadata/metadata.json":        victoriaModMetadata,
		"gfx/models/statue/statue.asset": "",
	})

	location, err := Locate(filepath.Join(dir, "gfx", "models", "statue", "statue.asset"))
	if err != nil {
		t.Fatal(err)
	}

	if !location.Mod || location.Product != Victoria3 || location.Name != "Manaflow" {
		t.Errorf("location = %+v, want a Victoria 3 mod called Manaflow", location)
	}

	if location.Root != dir {
		t.Errorf("root = %s, want %s", location.Root, dir)
	}
}

// TestLocateGameIsNoMod keeps a game folder, which carries no description of
// itself, from being read as a mod.
func TestLocateGameIsNoMod(t *testing.T) {
	root := game(t)

	location, err := Locate(filepath.Join(root, "gfx", "models", "statue", "statue_entities.asset"))
	if err != nil {
		t.Fatal(err)
	}

	if location.Mod || location.Product != NoProduct || location.Name != "" {
		t.Errorf("location = %+v, want a game folder", location)
	}
}
