package app

import (
	"fmt"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/kaiser-chris/pdx-asset-go/model"

	"github.com/kaiser-chris/pdx-model-viewer/internal/gui"
)

// What the timeline says about itself.
const (
	labelPlay   = "Play"
	labelPause  = "Pause"
	labelLoop   = "Loop"
	labelRewind = "Rewind"
	labelTime   = "##time"

	// textNotMoved says what the timeline does and does not do yet, so that a
	// model standing still is not taken for a model that is broken.
	textNotMoved = "The geometry is not moved yet: the timeline reads the animations and runs their clock, but the model is drawn in the pose its mesh is stored in."
)

// animationPanel shows the animations the entity on view can play, with a
// timeline over the longest of them. It only appears for an entity that has
// any, so that a model with none is shown without it.
func (a *App) animationPanel() {
	if !a.showAnimation || len(a.animations()) == 0 {
		return
	}

	a.placeTimeline()

	if imgui.BeginV(panelAnimation, &a.showAnimation, 0) {
		a.animationBody()
	}

	imgui.End()
}

// animations are the animations the entity on view can play, or none while
// nothing is shown.
func (a *App) animations() []model.Animation {
	if a.shown == nil {
		return nil
	}

	return a.shown.loaded.Details.Animations
}

// chosen is the animation the timeline is showing, and whether there is one.
func (a *App) chosen() (model.Animation, bool) {
	animations := a.animations()
	if a.animation < 0 || a.animation >= len(animations) {
		return model.Animation{}, false
	}

	return animations[a.animation], true
}

// resetAnimation puts the timeline back to the start of the first animation,
// for an entity that has just been picked.
func (a *App) resetAnimation() {
	a.animation = 0
	a.animationTime = 0
	a.playing = false
}

func (a *App) animationBody() {
	animations := a.animations()

	longest := model.Longest(animations)
	chosen, ok := a.chosen()

	a.advanceAnimation(chosen, longest)

	// The controls, then the timeline below them over its whole width.
	if a.playing {
		if gui.Button(labelPause) {
			a.playing = false
		}
	} else if gui.Button(labelPlay) {
		a.playing = true
	}

	imgui.SameLine()

	if gui.Button(labelRewind) {
		a.animationTime = 0
	}

	imgui.SameLine()
	gui.Checkbox(labelLoop, &a.looping)

	imgui.SameLine()

	// The clock, right aligned, so the controls do not shift as it counts.
	clock := a.animationClock(chosen, ok)
	imgui.SetCursorPosX(imgui.CursorPosX() + imgui.ContentRegionAvail().X - imgui.CalcTextSize(clock).X)
	gui.TextDisabled(clock)
	gui.Record(clock)

	a.animationPicker(animations)

	// The timeline reaches as far as the longest animation of the entity, so
	// that the lengths of them all can be seen against one another.
	elapsed := float32(a.animationTime)

	gui.FullWidth()

	if gui.Slider(labelTime, &elapsed, 0, float32(longest), "%.2f s") {
		a.animationTime = float64(elapsed)
		a.playing = false
	}

	imgui.Spacing()
	gui.DimmedText(textNotMoved)
}

// animationClock says where the timeline stands: the time, how long the
// animation picked runs, and which of its frames that time falls on.
func (a *App) animationClock(chosen model.Animation, ok bool) string {
	if !ok {
		return ""
	}

	frame := 0
	if chosen.FPS > 0 {
		frame = min(int(a.animationTime*float64(chosen.FPS)), max(chosen.Frames-1, 0))
	}

	return fmt.Sprintf("%.2f / %.2f s     frame %d of %d at %.0f fps",
		a.animationTime, chosen.Seconds, frame, chosen.Frames, chosen.Rate())
}

// advanceAnimation moves the clock on by the time the last frame took, while
// the animation is running. One that loops starts again at its end; one that
// does not stops there.
func (a *App) advanceAnimation(chosen model.Animation, longest float64) {
	if !a.playing || chosen.Seconds <= 0 {
		return
	}

	a.animationTime += float64(imgui.CurrentIO().DeltaTime())

	switch {
	case a.animationTime < chosen.Seconds:
	case a.looping:
		// Wound back by whole lengths, so that a frame long enough to
		// overshoot several times still lands where it should.
		a.animationTime -= chosen.Seconds * float64(int(a.animationTime/chosen.Seconds))
	default:
		a.animationTime = chosen.Seconds
		a.playing = false
	}

	a.animationTime = min(a.animationTime, longest)
}

// animationPicker is the drop down of the entity's animations, by the entity
// whose mesh plays each.
func (a *App) animationPicker(animations []model.Animation) {
	chosen, ok := a.chosen()

	preview := "None"
	if ok {
		preview = animationLabel(chosen)
	}

	if !gui.BeginCombo("Animation", preview) {
		return
	}
	defer imgui.EndCombo()

	for index, animation := range animations {
		label := fmt.Sprintf("%s##animation%d", animationLabel(animation), index)

		if gui.ComboItem(label, index == a.animation) {
			a.animation = index
			a.animationTime = 0
		}
	}
}

// animationLabel names an animation in the picker: its own name, how long it
// runs, and, for one played by an entity attached rather than the entity on
// view, that entity's name.
func animationLabel(animation model.Animation) string {
	label := fmt.Sprintf("%s  (%.2f s)", animation.ID, animation.Seconds)

	if animation.Attachment != 0 {
		label += "  of " + animation.Entity
	}

	return label
}
