//go:build uitest

package app

import (
	"path/filepath"
	"testing"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/kaiser-chris/pdx-model-viewer/internal/workspace"
)

// A mod that names the game it is for is read with it, without asking. The
// entity it defines draws a mesh only the game has, so the entity loading at
// all is what shows the game was read as well.
func TestModIsReadWithTheGameItNames(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureModFile(t, "named", "mod.asset"))

	if application.pending != nil {
		t.Fatal("the chooser was opened for a mod that names the game it is for")
	}

	if got := application.document.game.Install.Product; got != workspace.Victoria3 {
		t.Errorf("read with %v, want the Victoria 3 it names", got)
	}

	waitForEntity(application, driver, "mod_entity")

	// The mod is named the way it names itself, not after its folder, and the
	// game it is read with is said alongside it.
	if !driver.Exists(panelEntities, "Named Mod") {
		t.Error("the mod is not named the way it names itself")
	}

	if !driver.Exists(panelEntities, "A mod of Victoria 3") {
		t.Error("the game the mod is read with is not said")
	}
}

// A descriptor.mod on its own is what Crusader Kings 3 marks a mod with, so
// such a mod is read as one without asking.
func TestModWithADescriptorAloneIsCrusaderKings3(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureModFile(t, "old", "mod.asset"))

	if application.pending != nil {
		t.Fatal("the chooser was opened for a mod with a descriptor.mod alone")
	}

	if got := application.document.game.Install.Product; got != workspace.CrusaderKings3 {
		t.Errorf("read with %v, want Crusader Kings 3", got)
	}

	if got := application.document.game.Name; got != "Old Mod" {
		t.Errorf("the mod is called %q, want the name in its descriptor", got)
	}
}

// A mod whose metadata.json names no game and which has no descriptor.mod
// could be a mod of either of the two games that describe themselves that way,
// so the user is asked.
func TestModWithoutAClassifierAsks(t *testing.T) {
	application, driver := startApp(t)

	application.Open(fixtureModFile(t, "nameless", "mod.asset"))
	driver.Frames(3)

	if application.pending == nil {
		t.Fatal("a mod that does not say which game it is for was read without being asked")
	}

	if application.document != nil || application.opening != nil {
		t.Fatalf("document = %v, opening = %v; want nothing read yet", application.document, application.opening)
	}

	question := "Nameless Mod does not say which game it is for. Pick the game it is a mod of."

	if !driver.Exists(popupGame, question) {
		driver.Dump()
		t.Fatalf("the chooser does not ask %q", question)
	}

	// Every game is offered, whether this machine has it or not, so that one
	// that is not there can be picked and pointed at.
	for _, product := range workspace.Products {
		if !driver.Exists(popupGame, fixtureRow(application.installations, product)) {
			t.Errorf("%v is not offered", product)
		}
	}

	// Nothing is picked out to begin with, since the mod does not say.
	if application.pending.chosen >= 0 {
		t.Error("a game was picked out for a mod that does not say which it is")
	}

	driver.Click(popupGame, fixtureRow(application.installations, workspace.Victoria3))

	if !driver.Exists(popupGame, "Read from "+fixtureGame(t)) {
		t.Error("the chooser does not say what the mod would be read with")
	}

	driver.Click(popupGame, "Open")

	waitForFile(application, driver)
	waitForEntity(application, driver, "mod_entity")

	if got := application.document.game.Install.Product; got != workspace.Victoria3 {
		t.Errorf("read with %v, want the game that was picked", got)
	}
}

// A mod that carries both descriptions and names no game in either is the case
// the classifier cannot settle: which of the two describes it is the user's to
// say.
func TestModWithBothDescriptionsAndNoClassifierAsks(t *testing.T) {
	application, driver := startApp(t)

	application.Open(fixtureModFile(t, "ambiguous", "mod.asset"))
	driver.Frames(3)

	if application.pending == nil {
		t.Fatal("a mod with both descriptions and no classifier was read without being asked")
	}

	if application.document != nil || application.opening != nil {
		t.Errorf("document = %v, opening = %v; want nothing read yet", application.document, application.opening)
	}

	if !driver.Exists(popupGame, "Ambiguous Mod does not say which game it is for. Pick the game it is a mod of.") {
		driver.Dump()
		t.Error("the chooser does not say the mod does not name a game")
	}
}

// A mod of a game that is not on this machine is asked about rather than asked
// for: the user is told which game is missing, and points the viewer at it.
func TestModOfAGameThatIsNotInstalledAsks(t *testing.T) {
	application, driver := startApp(t)

	// The fixture machine has Victoria 3 and Crusader Kings 3, and not Europa
	// Universalis 5.
	application.settings.ModGames = map[string]string{
		filepath.Clean(fixtureMod(t, "nameless")): workspace.EuropaUniversalis5.Key(),
	}

	application.Open(fixtureModFile(t, "nameless", "mod.asset"))
	driver.Frames(3)

	if application.pending == nil {
		t.Fatal("a mod of a game that is not installed was read without being asked")
	}

	// The row of the game the mod is for is picked out, and says it is not
	// there.
	row := workspace.EuropaUniversalis5.String() + textNotFound

	if !driver.Exists(popupGame, row) {
		driver.Dump()
		t.Fatalf("the chooser does not offer %q", row)
	}

	if got := application.pending.chosen; got < 0 {
		t.Fatalf("chosen = %d, want the row of the game the mod is for", got)
	}

	if !driver.Exists(popupGame, "Pick the folder Europa Universalis 5 is installed in, with Browse...") {
		t.Error("the chooser does not say to point it at the game's folder")
	}

	// Nothing to read the mod with, so it does not open.
	driver.Click(popupGame, "Open")

	driver.Frames(3)

	if application.document != nil || application.opening != nil {
		t.Error("a mod was read without the game it is for")
	}
}

// The game the user answered for a mod is kept, so the same mod does not ask
// again on the next run.
func TestTheGameChosenForAModIsRemembered(t *testing.T) {
	config := t.TempDir()
	file := fixtureModFile(t, "nameless", "mod.asset")

	first, driver := startAppIn(t, config)

	first.Open(file)
	driver.Frames(3)

	driver.Click(popupGame, fixtureRow(first.installations, workspace.CrusaderKings3))
	driver.Click(popupGame, "Open")

	waitForFile(first, driver)

	if got := first.document.game.Install.Product; got != workspace.CrusaderKings3 {
		t.Fatalf("read with %v, want the game that was picked", got)
	}

	first.Close()

	second, driver := startAppIn(t, config)

	second.Open(file)
	driver.Frames(3)

	if second.pending != nil {
		t.Fatal("the chooser was opened again for a mod that was answered before")
	}

	waitForFile(second, driver)

	if got := second.document.game.Install.Product; got != workspace.CrusaderKings3 {
		t.Errorf("read with %v, want the game that was picked before", got)
	}
}

// A game this machine keeps outside Steam is pointed at by hand: the folder is
// taken as an installation of the game the chooser is on, and the mod is read
// with it. A folder holding no game files is refused in the chooser itself,
// where it can be read.
func TestPointingAtAGameFolderByHand(t *testing.T) {
	application, driver := startApp(t)

	dialogs := &fakeDialogs{}
	application.dialogs = dialogs

	// Nothing that was picked before, and no game of the three.
	application.installations = nil

	game := fixtureGame(t)
	dialogs.answerGameFolders(t.TempDir(), filepath.Dir(game))

	application.Open(fixtureModFile(t, "nameless", "mod.asset"))
	driver.Frames(3)

	if application.pending == nil {
		t.Fatal("the chooser was not opened")
	}

	if !driver.Exists(popupGame, textNoGames) {
		t.Error("the chooser does not say that no game was found")
	}

	// Nothing is picked, so there is nothing to point a folder at.
	if application.pending.chosen >= 0 {
		t.Error("a game was picked out of nothing")
	}

	driver.Click(popupGame, fixtureRow(application.installations, workspace.Victoria3))
	driver.Click(popupGame, "Browse...")

	driver.WaitFor("the folder to be refused", func() bool {
		return application.pending != nil && application.pending.problem != ""
	})

	if dialogs.gameFoldersAsked() != 1 {
		t.Errorf("the folder dialog was shown %d times, want once", dialogs.gameFoldersAsked())
	}

	// The folder is the installation's, so the mod is read with the game in
	// it.
	driver.Click(popupGame, "Browse...")

	driver.WaitFor("the folder to be taken", func() bool {
		return application.pending != nil && application.pending.problem == "" &&
			application.pending.browsed[workspace.Victoria3].Root == game
	})

	// The folder is now offered as an installation of that game, and says
	// what the mod would be read from.
	if !driver.Exists(popupGame, application.pending.browsed[workspace.Victoria3].String()) {
		driver.Dump()
		t.Error("the folder picked by hand is not offered as an installation")
	}

	if !driver.Exists(popupGame, "Read from "+game) {
		t.Error("the chooser does not say what the mod would be read with")
	}

	driver.Click(popupGame, "Open")

	waitForFile(application, driver)
	waitForEntity(application, driver, "mod_entity")

	if got := application.document.game.Install.Root; got != game {
		t.Errorf("read from %s, want the folder that was picked", got)
	}
}

// Giving up on the chooser leaves nothing read, and can be done with Escape as
// well as with the button.
func TestGivingUpOnTheChooser(t *testing.T) {
	application, driver := startApp(t)
	file := fixtureModFile(t, "nameless", "mod.asset")

	application.Open(file)
	driver.Frames(3)

	driver.Press(imgui.KeyEscape)
	driver.Frames(3)

	if application.pending != nil {
		t.Error("the chooser is still open after Escape")
	}

	if application.document != nil || application.opening != nil {
		t.Error("a mod was read after the chooser was given up on")
	}

	// The file can be opened again, and asked about again.
	application.Open(file)
	driver.Frames(3)

	if application.pending == nil {
		t.Fatal("the chooser was not opened the second time")
	}

	driver.Click(popupGame, "Cancel")
	driver.Frames(3)

	if application.pending != nil {
		t.Error("the chooser is still open after Cancel")
	}
}

// A file of a game folder belongs to no mod, so nothing is asked about it.
func TestAGameFolderAsksNothing(t *testing.T) {
	application, driver := startApp(t)

	openFile(t, application, driver, fixtureFile(t, "statue.asset"))

	if application.pending != nil {
		t.Error("the chooser was opened for a file of a game folder")
	}

	if application.document.location.Mod {
		t.Error("a game folder was taken for a mod")
	}
}
