package app

import (
	"testing"

	"github.com/kaiser-chris/pdx-asset-go/render"

	"github.com/kaiser-chris/pdx-model-viewer/internal/workspace"
)

func TestPartName(t *testing.T) {
	for _, test := range []struct{ name, want string }{
		{"LOD_0|decal|decalShape", "Decal"},
		{"LOD_0|meshShape", "Mesh"},
		{"LOD_0|tree|treeShape", "Tree"},
		{"LOD_1|meshShape", "Mesh (detail level 1)"},
		{"male_body_meshShape", "Male body mesh"},
		{"hairBackShape", "Hair back"},
		{"quad", "Quad"},
		// Nothing left to read keeps the name as it is.
		{"Shape", "Shape"},
		{"LOD_0", "LOD_0"},
	} {
		if got := partName(test.name); got != test.want {
			t.Errorf("partName(%q) = %q, want %q", test.name, got, test.want)
		}
	}
}

func TestPartLook(t *testing.T) {
	for _, test := range []struct {
		part workspace.PartDetails
		want string
	}{
		{workspace.PartDetails{Drawn: true}, "Solid"},
		{workspace.PartDetails{Drawn: true, Style: render.Style{Blend: true}}, "Laid over the model like a decal"},
		{workspace.PartDetails{Drawn: true, Style: render.Style{Cutout: true, TwoSided: true}}, "Cut out like leaves or hair, seen from both sides"},
		{workspace.PartDetails{Drawn: true, Style: render.Style{Palette: true, Atlas: true}}, "Solid, tinted with the palette colour and textured from an atlas"},
		{workspace.PartDetails{Drawn: true, Style: render.Style{Cutout: true, Tinted: true}}, "Cut out like leaves or hair, coloured by its tint"},
		{workspace.PartDetails{Drawn: true, Style: render.Style{Cutout: true, Foliage: true}}, "Cut out like leaves or hair, its grey coloured as leaves"},
		{workspace.PartDetails{}, "Not drawn: the game uses it for something other than looks, such as collisions"},
	} {
		if got := partLook(test.part); got != test.want {
			t.Errorf("partLook(%+v) = %q, want %q", test.part, got, test.want)
		}
	}
}

func TestThousands(t *testing.T) {
	for count, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 1896: "1,896", 1234567: "1,234,567"} {
		if got := thousands(count); got != want {
			t.Errorf("thousands(%d) = %q, want %q", count, got, want)
		}
	}
}
