//go:build uitest

package app

import (
	"testing"
)

// fixturePattern is what the fixture's first pattern is described by: the
// textures its channels draw with, each named once and in the order the
// channels are read.
const fixturePattern = "generic_cotton_standard_fine_01, generic_silk_fine_plain_02, european_wool_standard_striped_02"

// A model whose entity names a portrait accessory lists it in the details,
// headed by the variation it is drawn with, with the patterns and the colour
// palettes of that variation listed to pick from.
func TestShowsTheAccessoriesOfAModel(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "belt.asset"))
	waitForEntity(application, driver, "belt_entity")

	for _, shown := range []string{labelPatterns, "fixture_belt"} {
		if !driver.Exists(panelDetails, shown) {
			t.Errorf("the details do not show %q", shown)
		}
	}

	// The alternatives are a list of their own, as the entities of a file are.
	for _, shown := range []string{
		fixturePattern + "##pattern0-0",
		"generic_silk_fine_plain_02##pattern0-1",
		"fixture_red.png##palette0-0",
		"fixture_blue.png##palette0-1",
	} {
		if !driver.Exists(windowList, shown) {
			t.Errorf("the list does not show %q", shown)
		}
	}

	// An entity with no accessory is listed without the section at all.
	openFile(t, application, driver, fixtureFile(t, "windmill.asset"))
	waitForEntity(application, driver, "windmill_entity")

	if driver.Exists(panelDetails, labelPatterns) {
		t.Error("an entity with no accessory is listed with patterns")
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

	// The second palette is blue, and picking it from the list draws it.
	driver.Click(windowList, "fixture_blue.png##palette0-1")

	if _, palette := accessory(); palette != 1 {
		t.Errorf("the palette picked is %d, want the second", palette)
	}

	driver.Frames(2)

	if got := centre(application); got.B <= got.R {
		t.Errorf("the belt is drawn %v, want the blue of the palette picked", got)
	}

	// The second pattern draws the same colour: what tells them apart in the
	// fixture is which of the two it is, not what it looks like.
	driver.Click(windowList, "generic_silk_fine_plain_02##pattern0-1")

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

	if !driver.Exists(panelDetails, labelPatterns) {
		t.Error("the accessory of an effect without a pattern is not listed")
	}

	if !driver.Exists(panelDetails, textAccessoryNotDrawn) {
		t.Errorf("the details do not say that the accessory is not drawn: %q", textAccessoryNotDrawn)
	}
}
