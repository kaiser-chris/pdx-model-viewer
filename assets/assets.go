// Package assets bundles every file the application needs at runtime into the
// executable.
//
// The shaders the models are drawn with belong to pdx-asset-go, which bundles
// them itself; what is left here is the interface's own: its icon and fonts.
package assets

import (
	_ "embed"
)

// Icon is the application icon as a PNG image, shown in the title bar and the
// task bar.
//
//go:embed icon.png
var Icon []byte

// The interface fonts are embedded as strings on purpose. Dear ImGui keeps the
// pointer to the font data for as long as its atlas lives, because it
// rasterises glyphs on demand. A string declared with go:embed is backed by
// memory in the binary itself, which is never moved, never freed and cannot
// be written to, which is exactly what that pointer needs.
var (
	//go:embed fonts/roboto/Roboto-Regular.ttf
	FontRegular string

	//go:embed fonts/roboto/Roboto-Medium.ttf
	FontMedium string
)
