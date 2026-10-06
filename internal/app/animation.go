package app

import (
	"fmt"

	"github.com/AllenDang/cimgui-go/imgui"
	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/kaiser-chris/pdx-asset-go/anim"
	"github.com/kaiser-chris/pdx-asset-go/model"

	"github.com/kaiser-chris/pdx-model-viewer/internal/gui"
	"github.com/kaiser-chris/pdx-model-viewer/internal/workspace"
)

// What the timeline says about itself.
const (
	labelPlay  = "Play"
	labelPause = "Pause"
	labelStop  = "Stop"
	labelLoop  = "Loop"
	labelTime  = "##time"

	// labelMoved heads the list of attachments the animation picked moves,
	// each with a tick of its own.
	labelMoved = "Moved"
)

// animationPanel shows the animations the entity on view can play, with a
// timeline over the one picked. It only appears for an entity that has any, so
// that a model with none is shown without it.
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
// nothing is shown. One is listed for every attachment that plays it, so an
// entity attached many times lists the same animation many times.
func (a *App) animations() []model.Animation {
	if a.shown == nil {
		return nil
	}

	return a.shown.loaded.Details.Animations
}

// animationGroup is one animation the entity on view can play, and the
// attachments that play it.
type animationGroup struct {
	Animation   model.Animation
	Attachments []int
}

// animationGroups are the animations the entity can play, each listed once
// with every attachment that plays it: an entity attached many times, as the
// seagulls of a port are, lists its animations once rather than once per copy.
func (a *App) animationGroups() []animationGroup {
	var (
		groups []animationGroup
		seen   = map[string]int{}
	)

	for _, animation := range a.animations() {
		// The same animation of the same entity is one animation, however
		// many copies of that entity are attached.
		key := animation.Entity + "\x00" + animation.ID + "\x00" + animation.File

		if at, ok := seen[key]; ok {
			groups[at].Attachments = append(groups[at].Attachments, animation.Attachment)

			continue
		}

		seen[key] = len(groups)
		groups = append(groups, animationGroup{Animation: animation, Attachments: []int{animation.Attachment}})
	}

	return groups
}

// chosenGroup is the animation the timeline is showing, with the attachments
// that play it, and whether there is one.
func (a *App) chosenGroup() (animationGroup, bool) {
	groups := a.animationGroups()
	if a.animation < 0 || a.animation >= len(groups) {
		return animationGroup{}, false
	}

	return groups[a.animation], true
}

// chosen is the animation the timeline is showing, and whether there is one.
func (a *App) chosen() (model.Animation, bool) {
	group, ok := a.chosenGroup()

	return group.Animation, ok
}

// resetAnimation puts the timeline back to the start of the first animation,
// for an entity that has just been picked.
func (a *App) resetAnimation() {
	a.animation = 0
	a.animationTime = 0
	a.playing = false
	a.animationEnabledFor = -1
	a.animationEnabled = nil
	a.clearAnimationSamples()
}

// clearAnimationSamples drops the samples of the animation picked, which
// belong to the entity on view and no other.
func (a *App) clearAnimationSamples() {
	a.animationSamples = nil
	a.animationSamplesFor = 0
	a.animationReading = nil
	a.animationReadingFor = 0
}

// ensureAnimationEnabled fills in the attachments of the animation picked, all
// of them ticked to begin with, when a different one is picked.
func (a *App) ensureAnimationEnabled(group animationGroup) {
	if a.animationEnabledFor == a.animation && a.animationEnabled != nil {
		return
	}

	a.animationEnabledFor = a.animation
	a.animationEnabled = make(map[int]bool, len(group.Attachments))

	for _, attachment := range group.Attachments {
		a.animationEnabled[attachment] = true
	}
}

// animatedAttachments are the attachments the animation picked moves, those
// left ticked, in the order the model numbers them.
func (a *App) animatedAttachments() []int {
	group, ok := a.chosenGroup()
	if !ok {
		return nil
	}

	a.ensureAnimationEnabled(group)

	var animated []int

	for _, attachment := range group.Attachments {
		if a.animationEnabled[attachment] {
			animated = append(animated, attachment)
		}
	}

	return animated
}

func (a *App) animationBody() {
	group, ok := a.chosenGroup()

	// The controls, then the timeline below them over its whole width.
	if a.playing {
		if gui.Button(labelPause) {
			a.playing = false
		}
	} else if gui.Button(labelPlay) {
		a.playing = true
	}

	imgui.SameLine()

	if gui.Button(labelStop) {
		// Back to the start of the animation, standing still.
		a.animationTime = 0
		a.playing = false
	}

	imgui.SameLine()
	gui.Checkbox(labelLoop, &a.looping)

	imgui.SameLine()

	// The clock, right aligned, so the controls do not shift as it counts.
	clock := a.animationClock(group.Animation, ok)
	imgui.SetCursorPosX(imgui.CursorPosX() + imgui.ContentRegionAvail().X - imgui.CalcTextSize(clock).X)
	gui.TextDisabled(clock)
	gui.Record(clock)

	a.animationPicker()

	// The timeline reaches as far as the animation picked, not as far as the
	// longest of them all.
	span := float32(group.Animation.Seconds)
	if span <= 0 {
		span = 1
	}

	elapsed := float32(a.animationTime)

	gui.FullWidth()

	if gui.Slider(labelTime, &elapsed, 0, span, "%.2f s") {
		a.animationTime = float64(elapsed)
		a.playing = false
	}

	// An animation of an entity attached many times moves every copy of that
	// entity: each is listed here with a tick of its own, so that some can be
	// left standing still while the rest play.
	if ok && len(group.Attachments) > 1 {
		a.animationMoved(a.shown.loaded.Details, group)
	}
}

// animationMoved lists the attachments the animation picked moves, each with a
// tick: one left unticked stands still while the others play.
//
// The timeline runs along the bottom of the viewport, so the panel is much
// wider than it is tall: the attachments flow across it and wrap onto the next
// line, rather than stacking up out of sight.
func (a *App) animationMoved(details workspace.Details, group animationGroup) {
	a.ensureAnimationEnabled(group)

	imgui.Spacing()
	gui.TextDisabled(labelMoved)

	avail := imgui.ContentRegionAvail().X
	spacing := imgui.CurrentStyle().ItemSpacing().X
	inner := imgui.CurrentStyle().ItemInnerSpacing().X

	used := float32(0)

	for index, attachment := range group.Attachments {
		text := attachmentLabel(details, attachment)

		// A checkbox is its square, the room between that and its label, and
		// the label itself.
		width := imgui.CalcTextSize(text).X + imgui.FrameHeight() + inner

		if index > 0 {
			if used+spacing+width <= avail {
				imgui.SameLine()
				used += spacing + width
			} else {
				used = width
			}
		} else {
			used = width
		}

		ticked := a.animationEnabled[attachment]
		label := fmt.Sprintf("%s##animation%d", text, attachment)

		if gui.Checkbox(label, &ticked) {
			a.animationEnabled[attachment] = ticked
		}
	}
}

// attachmentLabel names an attachment the way the details do: the entity
// attached and the point it hangs from, or the name of the entity itself for
// the model's own, which is attachment zero.
func attachmentLabel(details workspace.Details, attachment int) string {
	if attachment <= 0 || attachment > len(details.Attached) {
		return details.Entity
	}

	attached := details.Attached[attachment-1]

	return attached.Entity + " at " + attached.Locator
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
// does not stops there. It runs before the picture is drawn, so the pose the
// picture shows is the pose the timeline stands at.
func (a *App) advanceAnimation() {
	chosen, ok := a.chosen()

	if !a.playing || !ok || chosen.Seconds <= 0 {
		return
	}

	a.animationTime += float64(rl.GetFrameTime())

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

	a.animationTime = min(a.animationTime, chosen.Seconds)
}

// ensureAnimation starts reading the samples of the animation picked, if they
// are not already read. Listing reads only the head of each file; moving the
// geometry needs the samples of the one played.
func (a *App) ensureAnimation() {
	if a.shown == nil || a.document == nil {
		return
	}

	group, ok := a.chosenGroup()
	if !ok {
		return
	}

	if a.animationSamples != nil && a.animationSamplesFor == a.animation {
		return
	}

	if a.animationReading != nil && a.animationReadingFor == a.animation {
		return
	}

	game := a.document.game
	chosen := group.Animation

	a.animationReadingFor = a.animation
	a.animationReading = start(func() (*anim.Animation, error) {
		return game.ReadAnimation(chosen.File)
	})
}

// pollAnimationReading stores the samples of the animation picked once they
// are read, unless the pick moved on meanwhile.
func (a *App) pollAnimationReading() {
	if a.animationReading == nil {
		return
	}

	samples, err, done := a.animationReading.poll()
	if !done {
		return
	}

	readFor := a.animationReadingFor
	a.animationReading = nil

	if a.animation != readFor {
		a.ensureAnimation()

		return
	}

	if err != nil {
		a.setStatus("Could not read the animation")

		return
	}

	a.animationSamples = samples
	a.animationSamplesFor = readFor
}

// pose is the animation picked read whole, when the one read is the one
// picked, and nothing otherwise, which leaves the model in the pose its mesh
// is stored in.
func (a *App) pose() (*anim.Animation, bool) {
	if a.shown == nil || a.animationSamples == nil || a.animationSamplesFor != a.animation {
		return nil, false
	}

	return a.animationSamples, true
}

// animationPicker is the drop down of the entity's animations, each listed
// once by the entity whose mesh plays it.
func (a *App) animationPicker() {
	groups := a.animationGroups()

	preview := "None"
	if chosen, ok := a.chosenGroup(); ok {
		preview = animationLabel(chosen.Animation)
	}

	if !gui.BeginCombo("Animation", preview) {
		return
	}
	defer imgui.EndCombo()

	for index, group := range groups {
		label := fmt.Sprintf("%s##animation%d", animationLabel(group.Animation), index)

		if gui.ComboItem(label, index == a.animation) {
			a.animation = index
			a.animationTime = 0
			a.animationEnabledFor = -1
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
