// Command pdx-model-viewer views the 3D models of the asset files of
// Victoria 3, Europa Universalis 5 and Crusader Kings 3.
//
// It is meant to be what .asset files open with: the file is the one argument,
// and everything it needs is found from where the file is.
//
//	pdx-model-viewer path/to/game/gfx/models/statue/statue.asset
package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/kaiser-chris/pdx-model-viewer/internal/app"
)

func init() {
	// raylib creates the window and the OpenGL context on the calling thread
	// and both have to stay there, so the main goroutine is pinned before
	// anything else runs.
	runtime.LockOSThread()
}

func main() {
	// The file to open, as the file association passes it. Anything after it
	// is ignored: the viewer shows one file at a time.
	var file string
	if len(os.Args) > 1 {
		file = os.Args[1]
	}

	if err := app.Run(file); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "pdx-model-viewer: %v\n", err)
		os.Exit(1)
	}
}
