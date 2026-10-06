package gui

import (
	"strings"
	"testing"
)

// measure is a width of one per rune, which is not how a font measures but is
// enough to tell a text too long for its room from one that fits.
func measure(text string) float32 {
	return float32(len([]rune(text)))
}

// A text that fits its room is left as it is; one that does not is cut short
// and ended in dots, keeping as much of it as the room allows.
func TestFitted(t *testing.T) {
	if got := fittedBy("silk", 8, measure); got != "silk" {
		t.Errorf("a text that fits was changed to %q", got)
	}

	// Twelve runes of room: nine of the name and the three dots.
	if got := fittedBy("generic_cotton_standard_fine_01", 12, measure); got != "generic_c"+cut {
		t.Errorf("a long name was cut to %q, want the longest beginning that fits", got)
	}

	// One more rune of room keeps one more of the name, which is what says
	// the beginning kept is the longest one.
	if got := fittedBy("generic_cotton_standard_fine_01", 13, measure); got != "generic_co"+cut {
		t.Errorf("a long name was cut to %q, want the longest beginning that fits", got)
	}

	// A room too small for any of it holds the dots alone.
	if got := fittedBy("generic_cotton_standard_fine_01", 2, measure); got != cut {
		t.Errorf("a name with no room was cut to %q, want the dots alone", got)
	}

	// Nothing that was cut is wider than the room it was cut to.
	for room := 3; room < 20; room++ {
		got := fittedBy("generic_cotton_standard_fine_01", float32(room), measure)

		if !strings.HasSuffix(got, cut) {
			t.Errorf("with %d of room the name was cut to %q, which does not say it was cut", room, got)
		}

		if width := measure(got); width > float32(room) {
			t.Errorf("with %d of room the name was cut to %q, which is %g wide", room, got, width)
		}
	}
}
