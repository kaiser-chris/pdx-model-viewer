//go:build windows

package workspace

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// steamRegistryKeys are where Steam writes the folder it was installed into on
// Windows, in the order they are trusted. A machine can have more than one
// installation, so the one of the user running the viewer comes first.
var steamRegistryKeys = []struct {
	hive  registry.Key
	path  string
	value string
}{
	{registry.CURRENT_USER, `Software\Valve\Steam`, "SteamPath"},
	{registry.CURRENT_USER, `Software\Valve\Steam`, "InstallPath"},
	{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Valve\Steam`, "InstallPath"},
	{registry.LOCAL_MACHINE, `SOFTWARE\Valve\Steam`, "InstallPath"},
}

// steamRoots are the folders Steam installations are looked for in on Windows,
// which Steam records in the registry.
func steamRoots() []string {
	var roots []string

	for _, key := range steamRegistryKeys {
		if root := registryValue(key.hive, key.path, key.value); root != "" {
			roots = append(roots, root)
		}
	}

	// The registry is not always there, such as for a Steam that has never
	// been run by this account, so the folder it installs itself into by
	// default is looked in as well.
	for _, name := range []string{"ProgramFiles(x86)", "ProgramFiles", "ProgramW6432"} {
		if base := os.Getenv(name); base != "" {
			roots = append(roots, filepath.Join(base, "Steam"))
		}
	}

	return roots
}

// registryValue reads a string out of the registry, or nothing when the key or
// the value is not there.
func registryValue(hive registry.Key, path, name string) string {
	key, err := registry.OpenKey(hive, path, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}

	defer func() { _ = key.Close() }()

	// Steam writes these with forward slashes even on Windows.
	if value, _, err := key.GetStringValue(name); err == nil {
		return filepath.FromSlash(value)
	}

	return ""
}
