package app

import (
	"fmt"
	"path/filepath"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/kaiser-chris/pdx-model-viewer/internal/gui"
	"github.com/kaiser-chris/pdx-model-viewer/internal/workspace"
)

// pendingOpen is an asset file of a mod the viewer cannot read yet: which game
// the mod is for, or where that game is installed, is the user's to say.
type pendingOpen struct {
	location workspace.Location

	// said is the game the mod names for itself, when it names one. Such a
	// mod is not asked which game it is for but where that game is, so its
	// row is picked out already.
	said workspace.Product

	// browsed are the folders the user picked by hand, by the game each is
	// for: a folder does not say which game it holds, so the row it was
	// picked from does.
	browsed map[workspace.Product]workspace.Installation

	// chosen is the row picked out, and -1 until one is.
	chosen int

	// problem is what was wrong with the folder the user picked last, said in
	// the chooser, where the status bar behind it cannot be read.
	problem string
}

// gameChoice is one row of the chooser: a game, and where it is installed,
// which is empty for one that is on neither Steam library nor anywhere the
// user has pointed the viewer.
type gameChoice struct {
	Product workspace.Product
	Install workspace.Installation
	Found   bool
}

// The ends of a row of the chooser, and what the chooser says: a mod that
// names a game is missing the game rather than the name.
const (
	textNotFound  = "  (not found on this machine)"
	textAskFolder = "%s is a mod of %s, which was not found on this machine. Pick another game, or the folder %s is installed in."
	textAskGame   = "%s does not say which game it is for. Pick the game it is a mod of."
	textNoGames   = "None of the three games was found on this machine. Pick the game this mod is for, then the folder it is installed in."
	textGameRead  = "%s holds no game files, so it is not the folder a game is installed in."
	textBrowseFor = "Pick the folder %s is installed in, with Browse..."
	textReadFrom  = "Read from %s"
)

// label is how a row reads.
func (c gameChoice) label() string {
	if !c.Found {
		return c.Product.String() + textNotFound
	}

	return c.Install.String()
}

// installationFor is where a game is installed on this machine. Steam is what
// knows, so a game in none of its libraries has none.
func (a *App) installationFor(product workspace.Product) (workspace.Installation, bool) {
	if product == workspace.NoProduct {
		return workspace.Installation{}, false
	}

	for _, install := range a.installations {
		if install.Product == product {
			return install, true
		}
	}

	return workspace.Installation{}, false
}

// rememberedGame is the game the user said a mod is for, for a mod whose own
// description does not say.
func (a *App) rememberedGame(root string) workspace.Product {
	return workspace.ProductOfKey(a.settings.ModGames[filepath.Clean(root)])
}

// rememberGame keeps the game the user said a mod is for, so that the chooser
// does not open again for it. A mod without a classifier is the common case,
// not the odd one.
func (a *App) rememberGame(root string, product workspace.Product) {
	if a.settings.ModGames == nil {
		a.settings.ModGames = map[string]string{}
	}

	a.settings.ModGames[filepath.Clean(root)] = product.Key()
	saveSettings(a.settingsFile, a.settings)
}

// askForGame starts the chooser for a mod whose game the viewer could not work
// out, with the game the mod names picked out already when it names one.
func (a *App) askForGame(location workspace.Location, product workspace.Product) {
	pending := &pendingOpen{
		location: location,
		said:     product,
		browsed:  map[workspace.Product]workspace.Installation{},
		chosen:   -1,
	}

	// A game the mod names is not asked for but asked about, whether the
	// machine has it or not: the user is told which one is missing.
	if product != workspace.NoProduct {
		for index, choice := range a.gameChoices(pending) {
			if choice.Product == product {
				pending.chosen = index

				break
			}
		}
	}

	a.pending = pending
}

// gameChoices are the rows the chooser offers: every game of the three, with
// the installations of each that were found on this machine and the folder the
// user pointed at for one.
func (a *App) gameChoices(pending *pendingOpen) []gameChoice {
	var choices []gameChoice

	for _, product := range workspace.Products {
		offered := 0

		for _, install := range a.installations {
			if install.Product == product {
				choices = append(choices, gameChoice{Product: product, Install: install, Found: true})
				offered++
			}
		}

		if browsed, ok := pending.browsed[product]; ok && !sameRoot(choices, browsed) {
			choices = append(choices, gameChoice{Product: product, Install: browsed, Found: true})
			offered++
		}

		// A game nothing was found for is still listed, so that it can be
		// picked and a folder pointed out for it.
		if offered == 0 {
			choices = append(choices, gameChoice{Product: product})
		}
	}

	return choices
}

// sameRoot reports whether a folder is already one of the rows, which keeps a
// folder the user points at twice from being listed twice.
func sameRoot(choices []gameChoice, install workspace.Installation) bool {
	for _, choice := range choices {
		if choice.Found && samePath(choice.Install.Root, install.Root) {
			return true
		}
	}

	return false
}

// question is what the chooser asks.
func (p *pendingOpen) question() string {
	if p.said == workspace.NoProduct {
		return fmt.Sprintf(textAskGame, p.location.DisplayName())
	}

	return fmt.Sprintf(textAskFolder, p.location.DisplayName(), p.said, p.said)
}

// gamePopup asks which game a mod is for, and where it is, when the viewer
// could not work it out.
func (a *App) gamePopup() {
	pending := a.pending
	if pending == nil {
		return
	}

	if !imgui.IsPopupOpenStr(popupGame) {
		imgui.OpenPopupStr(popupGame)
	}

	imgui.SetNextWindowSizeV(gui.ScaledVec2(700, 0), imgui.CondAppearing)

	open := true

	if !imgui.BeginPopupModalV(popupGame, &open, imgui.WindowFlagsNoResize|imgui.WindowFlagsNoSavedSettings) {
		// Escape and the window's close button both come back as this.
		if !open {
			a.pending = nil
		}

		return
	}
	defer imgui.EndPopup()

	a.gamePopupBody(pending)
}

func (a *App) gamePopupBody(pending *pendingOpen) {
	choices := a.gameChoices(pending)

	gui.TextWrapped(pending.question())
	gui.Record(pending.question())

	if len(a.installations) == 0 {
		imgui.Spacing()
		gui.DimmedText(textNoGames)
	}

	imgui.Spacing()

	for index, choice := range choices {
		if gui.ListRow(choice.label(), index == pending.chosen) {
			pending.chosen = index
			pending.problem = ""
		}
	}

	// What the row picked out will read the mod with, which is the folder to
	// point at when the game is not there.
	if pending.chosen >= 0 && pending.chosen < len(choices) {
		imgui.Spacing()

		if picked := choices[pending.chosen]; picked.Found {
			gui.DimmedText(fmt.Sprintf(textReadFrom, picked.Install.Root))
		} else {
			gui.DimmedText(fmt.Sprintf(textBrowseFor, picked.Product))
		}
	}

	if pending.problem != "" {
		imgui.Spacing()
		gui.WarningText(pending.problem)
	}

	imgui.Spacing()
	imgui.Separator()
	imgui.Spacing()

	a.gamePopupButtons(pending, choices)
}

// gamePopupButtons are what the chooser can be answered with: reading the mod
// with the game picked out, pointing the viewer at a game this machine keeps
// outside Steam, or giving up.
func (a *App) gamePopupButtons(pending *pendingOpen, choices []gameChoice) {
	picked := pending.chosen >= 0 && pending.chosen < len(choices)

	imgui.BeginDisabledV(!picked)

	if gui.Button("Browse...") {
		a.askForGameFolder()
	}

	imgui.EndDisabled()

	if picked {
		gui.Tooltip("Pick the folder " + choices[pending.chosen].Product.String() + " is installed in")
	} else {
		gui.Tooltip("Pick the game this mod is for first")
	}

	imgui.SameLine()

	imgui.BeginDisabledV(!picked || !choices[pending.chosen].Found)

	if gui.Button("Open") || imgui.IsKeyPressedBool(imgui.KeyEnter) {
		a.openPending(choices)
	}

	imgui.EndDisabled()

	imgui.SameLine()

	if gui.Button("Cancel") || imgui.IsKeyPressedBool(imgui.KeyEscape) {
		a.pending = nil
		imgui.CloseCurrentPopup()
	}
}

// openPending reads the mod with the game the user picked out, and remembers
// which it was.
func (a *App) openPending(choices []gameChoice) {
	pending := a.pending
	if pending.chosen < 0 || pending.chosen >= len(choices) {
		return
	}

	install := choices[pending.chosen].Install
	if !choices[pending.chosen].Found {
		return
	}

	a.rememberGame(pending.location.Root, install.Product)

	a.pending = nil
	imgui.CloseCurrentPopup()

	a.beginOpen(pending.location, install)
}

// askForGameFolder shows the system's folder dialog for a game this machine
// keeps outside Steam. The answer is empty when the user cancels.
func (a *App) askForGameFolder() {
	if a.gameFolder != nil {
		return
	}

	dialogs := a.dialogs

	a.gameFolder = start(func() (string, error) {
		return dialogs.chooseGameFolder()
	})
}

// pollGameFolder takes the folder the user picked for a game, once they have.
func (a *App) pollGameFolder() {
	if a.gameFolder == nil {
		return
	}

	folder, err, done := a.gameFolder.poll()
	if !done {
		return
	}

	a.gameFolder = nil

	pending := a.pending
	if pending == nil {
		return
	}

	choices := a.gameChoices(pending)

	switch {
	case err != nil:
		warn(err)
		pending.problem = "The folder dialog could not be shown: " + err.Error()
	case folder == "":
		// Cancelled, which says nothing about the folder picked before, if
		// any.
	case pending.chosen < 0 || pending.chosen >= len(choices):
		pending.problem = "Pick the game this mod is for first"
	default:
		a.pickedGameFolder(pending, choices, folder)
	}
}

// pickedGameFolder reads the folder the user picked as an installation of the
// game the chooser was on, and says so in the chooser when it is no game
// folder at all.
func (a *App) pickedGameFolder(pending *pendingOpen, choices []gameChoice, folder string) {
	product := choices[pending.chosen].Product

	install, ok := workspace.InstallationAt(folder, product)
	if !ok {
		pending.problem = fmt.Sprintf(textGameRead, folder)

		return
	}

	pending.problem = ""
	pending.browsed[product] = install

	// The row of the folder just picked is picked out.
	for index, choice := range a.gameChoices(pending) {
		if choice.Found && samePath(choice.Install.Root, install.Root) {
			pending.chosen = index

			return
		}
	}
}
