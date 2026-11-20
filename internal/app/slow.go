package app

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/AllenDang/cimgui-go/imgui"
)

// A graphics chip too slow for a model draws so few frames a second that the
// window seems frozen rather than slow, as it does on a laptop that runs the
// viewer on the chip built into its processor rather than its own. The status
// bar warns of it, and says what to do, rather than leave the user to think
// the application hung.

const (
	// rateWindow is how long the frames are counted for before the rate is
	// judged, which spreads a frame that took long, such as the first of a
	// model, over the frames around it.
	rateWindow = 5 * time.Second

	// slowBelow is the rate a model is drawn at too slowly to be worked with,
	// and fastAgain the rate it has to come back to for the warning to go: a
	// rate going back and forth around one would have it blink.
	slowBelow = 20
	fastAgain = 30
)

// frameRate judges how fast the model is drawn.
type frameRate struct {
	started time.Time
	frames  int

	// slow is set while the model is drawn too slowly, and rate is the rate
	// it was last judged at.
	slow bool
	rate float64
}

// frame counts a frame begun at a time. A frame of a window with nothing to
// judge by, a minimized one or one without a model, starts the count again,
// so that a window brought back is judged on frames drawn since.
func (f *frameRate) frame(now time.Time, judged bool) {
	if !judged {
		f.started, f.frames = time.Time{}, 0

		return
	}

	if f.started.IsZero() {
		f.started, f.frames = now, 0

		return
	}

	f.frames++

	elapsed := now.Sub(f.started)
	if elapsed < rateWindow {
		return
	}

	f.rate = float64(f.frames) / elapsed.Seconds()
	f.started, f.frames = now, 0

	switch {
	case f.rate < slowBelow:
		f.slow = true
	case f.rate >= fastAgain:
		f.slow = false
	}
}

// slowWarning is the label of the status bar's button that warns of a slow
// model, and nothing while the model is drawn fast enough.
func (a *App) slowWarning() string {
	if !a.frameRate.slow {
		return ""
	}

	return "Slow graphics"
}

// slowTooltip says why the model is slow and what to do about it.
func (a *App) slowTooltip() {
	if !imgui.BeginItemTooltip() {
		return
	}

	defer imgui.EndTooltip()

	imgui.PushTextWrapPosV(imgui.FontSize() * 30)
	defer imgui.PopTextWrapPos()

	imgui.TextUnformatted(fmt.Sprintf("The model is drawn at %.0f frames a second, with %s, too few for the viewer to answer quickly.", a.frameRate.rate, a.graphics))
	imgui.Spacing()
	imgui.TextUnformatted(slowAdvice())
}

// slowAdvice says how a computer with two graphics chips is told to draw the
// viewer with the faster.
func slowAdvice() string {
	advice := "A laptop with a graphics chip of its own may be drawing the viewer with the slower one built into its processor."

	switch runtime.GOOS {
	case "windows":
		advice += " Pick High performance for pdx-model-viewer.exe in Settings > System > Display > Graphics, and start it again."
	case "linux":
		advice += " Start it on the other chip, for example with prime-run, or with DRI_PRIME=1 set."
	}

	return strings.TrimSpace(advice)
}
