package app

import (
	"errors"
	"path/filepath"

	"github.com/ncruces/zenity"

	"github.com/kaiser-chris/pdx-model-viewer/internal/workspace"
)

// fileDialogs asks the user for an asset file, starting in a folder, or
// anywhere when it is empty. It returns an empty path and no error when the
// user cancels.
//
// The application shows the system's own dialog. The interface tests answer
// with paths of their own, since a test cannot click through a real one.
type fileDialogs interface {
	chooseAssetFile(folder string) (string, error)
}

// systemDialogs shows the native dialogs: the common dialogs on Windows, and
// zenity or kdialog on Linux, whichever the desktop has.
type systemDialogs struct {
	// parent is the window the dialogs belong to, so that they open over it
	// and keep it from being used meanwhile.
	parent zenity.Option
}

func (d systemDialogs) chooseAssetFile(folder string) (string, error) {
	options := []zenity.Option{
		zenity.Title("Open Asset File"),
		zenity.FileFilter{Name: "Asset files", Patterns: []string{"*" + workspace.Extension}, CaseFold: true},
		d.parent,
	}

	if folder != "" {
		// A trailing separator opens the folder rather than proposing a file
		// name in it.
		options = append(options, zenity.Filename(folder+string(filepath.Separator)))
	}

	path, err := zenity.SelectFile(options...)
	if errors.Is(err, zenity.ErrCanceled) {
		return "", nil
	}

	return path, err
}

// pendingDialog is an open file dialog, answered on a goroutine of its own so
// that the window keeps drawing meanwhile.
type pendingDialog = job[string]

// askForAssetFile shows the system's file dialog for an asset file, starting
// in the folder of the file open now, if any. The answer is empty when the
// user cancels.
func (a *App) askForAssetFile() {
	if a.dialog != nil {
		return
	}

	var folder string
	if a.document != nil {
		folder = filepath.Dir(a.document.location.File)
	}

	dialogs := a.dialogs

	a.dialog = start(func() (string, error) {
		return dialogs.chooseAssetFile(folder)
	})
}

// pollDialog opens the file the user picked, once they have.
func (a *App) pollDialog() {
	if a.dialog == nil {
		return
	}

	path, err, done := a.dialog.poll()
	if !done {
		return
	}

	a.dialog = nil

	switch {
	case err != nil:
		warn(err)
		a.setStatus("The file dialog could not be shown: %v", err)
	case path != "":
		a.Open(path)
	}
}
