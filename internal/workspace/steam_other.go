//go:build !windows

package workspace

import (
	"os"
	"path/filepath"
)

// steamRoots are the folders Steam installations are looked for in on the
// systems that are not Windows, which have no registry to ask: Steam keeps
// itself in one of a handful of places, and so does each way of installing it.
//
// The list is the one the tools that read Steam's files use: the two folders
// the client itself makes and the link it keeps between them, the Debian
// package's own folder, the copies the Flatpak and Snap packages keep to
// themselves, and the one Steam uses on macOS, which nothing here is put off
// by.
func steamRoots() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	below := []string{
		filepath.Join(".steam", "steam"),
		filepath.Join(".steam", "root"),
		filepath.Join(".steam", "debian-installation"),
		filepath.Join(".local", "share", "Steam"),
		filepath.Join(".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"),
		filepath.Join(".var", "app", "com.valvesoftware.Steam", "data", "Steam"),
		filepath.Join("snap", "steam", "common", ".local", "share", "Steam"),
		filepath.Join("Library", "Application Support", "Steam"),
	}

	roots := make([]string, 0, len(below))
	for _, folder := range below {
		roots = append(roots, filepath.Join(home, folder))
	}

	return roots
}
