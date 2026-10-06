package workspace

import "strings"

// Product is one of the games the viewer reads the asset files of. A game
// folder says nothing about which game it is: what a folder is read as comes
// from the mod that names it, or from the application Steam installs it as.
type Product int

const (
	// NoProduct is a game the viewer does not know: a mod whose own
	// description does not name one, or an identifier none of the games use.
	// Which game such a mod is for is the user's to say.
	NoProduct Product = iota

	Victoria3
	CrusaderKings3
	EuropaUniversalis5
)

// Products are the games the viewer reads, in the order they are offered.
var Products = []Product{Victoria3, CrusaderKings3, EuropaUniversalis5}

// String is the game's name, spelled the way the viewer shows it to a user.
func (p Product) String() string {
	switch p {
	case Victoria3:
		return "Victoria 3"
	case CrusaderKings3:
		return "Crusader Kings 3"
	case EuropaUniversalis5:
		return "Europa Universalis 5"
	}

	return "Unknown game"
}

// Key is how a game is written down: the identifier a mod is expected to carry
// in its metadata.json, which is also what a remembered choice is kept by.
func (p Product) Key() string {
	switch p {
	case Victoria3:
		return "victoria3"
	case CrusaderKings3:
		return "ck3"
	case EuropaUniversalis5:
		return "eu5"
	}

	return ""
}

// steamApp is the application Steam installs the game as. All three are on
// Steam alone, so this is how a game on this machine is looked for.
func (p Product) steamApp() string {
	switch p {
	case Victoria3:
		return "529340"
	case CrusaderKings3:
		return "1158310"
	case EuropaUniversalis5:
		return "3450310"
	}

	return ""
}

// installedFolder is the folder Steam installs the game in, below the common
// folder of a library. It is the game's own name rather than the one the
// viewer shows, and only a fallback: the appmanifest Steam writes next to the
// game says where it really is.
func (p Product) installedFolder() string {
	switch p {
	case Victoria3:
		return "Victoria 3"
	case CrusaderKings3:
		return "Crusader Kings III"
	case EuropaUniversalis5:
		return "Europa Universalis V"
	}

	return ""
}

// ProductOfKey reads a game from the identifier it is written down by, in the
// spellings the games, their launchers and modders use. It is NoProduct for an
// identifier that names none of them.
//
// Victoria 3 asks for "victoria3"; the metadata.json of a Europa Universalis 5
// mod says "eu5", and both games let the field be left out altogether.
func ProductOfKey(key string) Product {
	switch normalizeKey(key) {
	case "victoria3", "vic3":
		return Victoria3
	case "ck3", "crusaderkings3", "crusaderkingsiii":
		return CrusaderKings3
	case "eu5", "europauniversalis5", "europauniversalisv":
		return EuropaUniversalis5
	}

	return NoProduct
}

// normalizeKey strips what only differs in spelling: case, and the separators
// put between the words of a name.
func normalizeKey(key string) string {
	return strings.ToLower(strings.NewReplacer("_", "", "-", "", " ", "", ".", "").Replace(strings.TrimSpace(key)))
}
