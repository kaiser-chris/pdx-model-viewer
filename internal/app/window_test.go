//go:build uitest

package app

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/AllenDang/cimgui-go/imgui"
	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/kaiser-chris/pdx-model-viewer/internal/gui"
)

// The window opens at the size it was left at, and its size is kept when it
// closes, for the next run.
func TestWindowSizeIsKept(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	dir := t.TempDir()
	path := filepath.Join(dir, "window.json")
	savePlacement(path, gui.Placement{X: 50, Y: 60, Width: 900, Height: 640})

	application, err := New(Options{ConfigDir: dir, Hidden: true, Scale: 1})
	if err != nil {
		t.Fatalf("start the application: %v", err)
	}

	application.Step()

	if width, height := rl.GetScreenWidth(), rl.GetScreenHeight(); width != 900 || height != 640 {
		t.Errorf("the window opened at %dx%d, want the 900x640 it was left at", width, height)
	}

	rl.SetWindowSize(1000, 700)
	application.Step()
	application.Close()

	got := loadPlacement(path)
	if got.Width != 1000 || got.Height != 700 || got.Maximized {
		t.Errorf("kept %+v, want a window of 1000x700, not maximized", got)
	}
}

// The default layout has the viewport on the left, and on the right the
// entities above the details.
func TestDefaultLayout(t *testing.T) {
	_, driver := startApp(t)
	driver.Frames(2)

	viewport := panelRect(t, panelViewport)
	entities := panelRect(t, panelEntities)
	details := panelRect(t, panelDetails)

	if !(entities.min.X >= viewport.max.X && details.min.X >= viewport.max.X) {
		t.Errorf("viewport %v, entities %v, details %v: want both panels right of the viewport", viewport, entities, details)
	}

	if !(details.min.Y >= entities.max.Y) {
		t.Errorf("entities %v, details %v: want the details below the entities", entities, details)
	}

	if entities.min.X != details.min.X {
		t.Errorf("entities %v, details %v: want them in one column", entities, details)
	}
}

type rect struct{ min, max imgui.Vec2 }

func panelRect(t *testing.T, name string) rect {
	t.Helper()

	window := imgui.InternalFindWindowByName(name)
	if window == nil || window.CData == nil {
		t.Fatalf("no panel %s", name)
	}

	position, size := window.Pos(), window.Size()

	return rect{position, imgui.Vec2{X: position.X + size.X, Y: position.Y + size.Y}}
}
