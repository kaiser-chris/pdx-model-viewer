package app

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func TestUniqueExportPath(t *testing.T) {
	folder := t.TempDir()

	if got, want := uniqueExportPath(folder, "oak_entity", "top"), filepath.Join(folder, "oak_entity_top.png"); got != want {
		t.Errorf("free name = %s, want %s", got, want)
	}

	for _, taken := range []string{"oak_entity_top.png", "oak_entity_top_1.png"} {
		if err := os.WriteFile(filepath.Join(folder, taken), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if got, want := uniqueExportPath(folder, "oak_entity", "top"), filepath.Join(folder, "oak_entity_top_2.png"); got != want {
		t.Errorf("taken name = %s, want %s", got, want)
	}

	if got, want := uniqueExportPath(folder, `odd:name/x`, "front"), filepath.Join(folder, "odd_name_x_front.png"); got != want {
		t.Errorf("name of characters a file cannot hold = %s, want %s", got, want)
	}
}

// Views exported together share one number: past the highest any of their
// names has taken, so that they stay one group.
func TestExportGroupPaths(t *testing.T) {
	folder := t.TempDir()

	for _, taken := range []string{"oak_entity_front.png", "oak_entity_back_1.png"} {
		if err := os.WriteFile(filepath.Join(folder, taken), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got := exportGroupPaths(folder, "oak_entity", []string{"front", "back", "top"})

	for index, name := range []string{"oak_entity_front_2.png", "oak_entity_back_2.png", "oak_entity_top_2.png"} {
		if want := filepath.Join(folder, name); got[index] != want {
			t.Errorf("view %d = %s, want %s", index, got[index], want)
		}
	}

	// Nothing taken, nothing numbered.
	got = exportGroupPaths(folder, "elm_entity", []string{"front", "back"})
	if want := filepath.Join(folder, "elm_entity_back.png"); got[1] != want {
		t.Errorf("free group = %s, want %s", got[1], want)
	}
}

func TestWithPNGExtension(t *testing.T) {
	for path, want := range map[string]string{"a": "a.png", "a.png": "a.png", "a.PNG": "a.PNG", "a.jpg": "a.jpg.png"} {
		if got := withPNGExtension(path); got != want {
			t.Errorf("withPNGExtension(%q) = %q, want %q", path, got, want)
		}
	}
}

// Scaling down averages the premultiplied pixels, so an edge half covered
// by the model comes out half transparent in the model's colour.
func TestShrink(t *testing.T) {
	picture := image.NewRGBA(image.Rect(0, 0, 4, 2))
	picture.SetRGBA(0, 0, color.RGBA{R: 200, A: 255})
	picture.SetRGBA(0, 1, color.RGBA{R: 200, A: 255})

	for x := 2; x < 4; x++ {
		for y := range 2 {
			picture.SetRGBA(x, y, color.RGBA{G: 100, A: 255})
		}
	}

	shrunk := shrink(picture, 2)

	if got := shrunk.Bounds().Size(); got != image.Pt(2, 1) {
		t.Fatalf("size = %v, want 2x1", got)
	}

	if got, want := shrunk.RGBAAt(0, 0), (color.RGBA{R: 100, A: 128}); got != want {
		t.Errorf("half covered = %v, want %v", got, want)
	}

	if got, want := shrunk.RGBAAt(1, 0), (color.RGBA{G: 100, A: 255}); got != want {
		t.Errorf("covered = %v, want %v", got, want)
	}
}

func TestScaledExportSize(t *testing.T) {
	for _, test := range []struct {
		desktop [2]int32
		factor  float64
		want    [2]int32
	}{
		{[2]int32{2560, 1440}, 1, [2]int32{2560, 1440}},
		{[2]int32{2560, 1440}, 0.5, [2]int32{1280, 720}},
		{[2]int32{2560, 1440}, 2, [2]int32{5120, 2880}},
		// Twice a desktop of 5K is more than a picture can be.
		{[2]int32{5120, 2880}, 2, [2]int32{maxExportSide, 5760}},
		// A desktop of no known size counts as 1920 by 1080.
		{[2]int32{}, 1, [2]int32{1920, 1080}},
	} {
		if got := scaledExportSize(test.desktop, test.factor); got != test.want {
			t.Errorf("%v at %v = %v, want %v", test.desktop, test.factor, got, test.want)
		}
	}
}

// An export begins in the size set last, or the desktop's without one.
func TestNewExportSettingsSize(t *testing.T) {
	desktop := [2]int32{2560, 1440}

	if got := newExportSettings(desktop, [2]int32{}).size; got != desktop {
		t.Errorf("size without one kept = %v, want the desktop's %v", got, desktop)
	}

	if got := newExportSettings(desktop, [2]int32{800, 600}).size; got != [2]int32{800, 600} {
		t.Errorf("size kept as 800x600 = %v", got)
	}

	if got := newExportSettings(desktop, [2]int32{4, 99999}).size; got != [2]int32{minExportSide, maxExportSide} {
		t.Errorf("size kept out of bounds = %v, want it kept within them", got)
	}
}

func TestPresetTooltip(t *testing.T) {
	for _, test := range []struct {
		size   [2]int32
		factor float64
		want   string
	}{
		{[2]int32{960, 540}, 0.5, "Set to 960x540: 50% desktop resolution"},
		{[2]int32{1920, 1080}, 1, "Set to 1920x1080: Full desktop resolution"},
		{[2]int32{3840, 2160}, 2, "Set to 3840x2160: 200% desktop resolution"},
	} {
		if got := presetTooltip(test.size, test.factor); got != test.want {
			t.Errorf("presetTooltip(%v, %v) = %q, want %q", test.size, test.factor, got, test.want)
		}
	}
}
