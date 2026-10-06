//go:build uitest

package app

import (
	"testing"

	"github.com/kaiser-chris/pdx-model-viewer/internal/uitest"
)

// A model whose entity names a portrait accessory lists it in the details,
// with the variation it is drawn with and the pattern and the colour palette
// of it: the two of each are what there is to choose between.
func TestShowsTheAccessoriesOfAModel(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "belt.asset"))
	waitForEntity(application, driver, "belt_entity")

	for _, shown := range []string{
		"Accessories (1)",
		"belt_entity",
		"fixture_belt",
		"Pattern##accessory0",
		"Palette##accessory0",
	} {
		if !driver.Exists(panelDetails, shown) {
			t.Errorf("the details do not show %q", shown)
		}
	}

	// An entity with no accessory is listed without the section at all.
	openFile(t, application, driver, fixtureFile(t, "windmill.asset"))
	waitForEntity(application, driver, "windmill_entity")

	if driver.Exists(panelDetails, "Accessories (1)") {
		t.Error("an entity with no accessory is listed with one")
	}
}

// The accessory colours the model, and another pattern or palette of the
// variation can be picked from the details: the model is drawn with what was
// picked without being opened again.
func TestPicksAnotherAccessoryAlternative(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "belt.asset"))
	waitForEntity(application, driver, "belt_entity")

	// The fixture's mask and pattern cover the whole quad, so the palette
	// colour is what the model is drawn in: the first of the two is red.
	if got := centre(application); !redder(got) {
		t.Errorf("the belt is drawn %v, want the red of its first palette", got)
	}

	accessory := func() (pattern, palette int) {
		choices := application.shown.model.Accessories()
		if len(choices) != 1 {
			t.Fatalf("the model has %d accessories, want one", len(choices))
		}

		return choices[0].Pattern, choices[0].Palette
	}

	if pattern, palette := accessory(); pattern != 0 || palette != 0 {
		t.Fatalf("the accessory is drawn with pattern %d and palette %d, want the first of each", pattern, palette)
	}

	// The second palette is blue.
	driver.Click(panelDetails, "Palette##accessory0")
	driver.Click(uitest.AnyCombo, "fixture_blue.png##Palette0-1")

	if _, palette := accessory(); palette != 1 {
		t.Errorf("the palette picked is %d, want the second", palette)
	}

	driver.Frames(2)

	if got := centre(application); got.B <= got.R {
		t.Errorf("the belt is drawn %v, want the blue of the palette picked", got)
	}

	// The second pattern draws the same colour: what tells them apart in the
	// fixture is which of the two it is, not what it looks like.
	driver.Click(panelDetails, "Pattern##accessory0")
	driver.Click(uitest.AnyCombo, "trim##Pattern0-1")

	if pattern, _ := accessory(); pattern != 1 {
		t.Errorf("the pattern picked is %d, want the second", pattern)
	}
}

// An accessory whose effect lays no pattern is listed, and said to be drawn by
// nothing: the game would not colour that part either.
func TestAnAccessoryOfAnEffectWithoutAPattern(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "plain_belt.asset"))
	waitForEntity(application, driver, "plain_belt_entity")

	if !driver.Exists(panelDetails, "Accessories (1)") {
		t.Error("the accessory of an effect without a pattern is not listed")
	}

	if !driver.Exists(panelDetails, textAccessoryNotDrawn) {
		t.Errorf("the details do not say that the accessory is not drawn: %q", textAccessoryNotDrawn)
	}
}
