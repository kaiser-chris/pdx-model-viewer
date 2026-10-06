package workspace

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/kaiser-chris/pdx-parser-go/script"
)

// The two ways a mod describes itself: the .metadata folder holding a
// metadata.json, which is what Victoria 3 and Europa Universalis 5 write, and
// the descriptor.mod of Crusader Kings 3. A mod may carry both.
const (
	metadataFolder = ".metadata"
	metadataFile   = "metadata.json"
	descriptorFile = "descriptor.mod"
)

// bom is the byte order mark editors on Windows like to start a file with,
// against the spec, which the JSON decoder refuses.
var bom = []byte("\xef\xbb\xbf")

// Description is what a folder says about itself: whether it is the root of a
// mod rather than the folder of a game installation, which game the mod is
// for, and what it calls itself.
type Description struct {
	// Mod is set for the root of a mod: one that carries a .metadata folder
	// or a descriptor.mod. It holds even when the file describing the mod is
	// missing or unreadable, since the folder itself is the marker.
	Mod bool

	// Product is the game the mod is for. It is NoProduct when the mod does
	// not say: a metadata.json without a game_id, or one naming a game the
	// viewer does not read. Which game that is is the user's to say.
	Product Product

	// Name is what the mod calls itself, when it says.
	Name string
}

// Describe reads what a folder is. A folder that is not a mod describes as the
// zero Description, which is every game folder and every folder of a loose
// file.
//
// A descriptor.mod on its own means Crusader Kings 3, which is the only one of
// the three that marks a mod that way. A game_id always decides where there is
// one, even next to a descriptor.mod: it is the newer description of the two,
// and the only one that carries a classifier at all.
func Describe(root string) Description {
	if root == "" {
		return Description{}
	}

	// The markers, which make a mod of a folder whose description is missing
	// or cannot be read: a mod of nothing but a thumbnail is still a mod.
	metadata := exists(filepath.Join(root, metadataFolder))
	descriptor := exists(filepath.Join(root, descriptorFile))

	described := Description{Mod: metadata || descriptor}

	described.Name, described.Product = readModMetadata(filepath.Join(root, metadataFolder, metadataFile))

	if described.Name == "" {
		described.Name = readDescriptor(filepath.Join(root, descriptorFile))
	}

	if described.Product == NoProduct && descriptor && !metadata {
		described.Product = CrusaderKings3
	}

	return described
}

// readModMetadata reads what a mod's metadata.json says. A file that is not
// there, or that cannot be read as JSON, says nothing.
func readModMetadata(path string) (string, Product) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", NoProduct
	}

	var parsed struct {
		Name   string `json:"name"`
		GameID string `json:"game_id"`
	}

	if json.Unmarshal(bytes.TrimPrefix(data, bom), &parsed) != nil {
		return "", NoProduct
	}

	return parsed.Name, ProductOfKey(parsed.GameID)
}

// readDescriptor reads the name out of a descriptor.mod, which is written in
// the script language the games use everywhere else, so the parser reads it.
func readDescriptor(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	document := script.Parse(string(bytes.TrimPrefix(data, bom)))

	name, ok := document.Get("name")
	if !ok {
		return ""
	}

	return name.Text
}
