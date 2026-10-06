package gui

import (
	"math"
	"strings"

	"github.com/AllenDang/cimgui-go/imgui"
)

// The interactive widgets the application uses.
//
// Each one calls Dear ImGui and then reports what it drew to the recorder, if
// one is installed. Dear ImGui has hooks of its own for exactly this, for its
// test engine, but they only exist when Dear ImGui itself is compiled with a
// define, and cimgui-go ships it precompiled. Reporting from here gives the
// interface tests the same view: every widget's label, window and rectangle,
// which is what they need to find a widget and click it.
//
// Without a recorder the cost is a nil check per widget.

// RecordedItem is one widget as it was laid out.
type RecordedItem struct {
	ID imgui.ID

	// Label is what the widget was created with, including any ## suffix
	// that keeps its id unique.
	Label string

	// Window is the name of the window the widget sits in. Menus open in
	// windows named ##Menu_00 and so on, the main menu bar is ##MainMenuBar,
	// and a scrolling table lives in a child window named after its parent.
	Window string

	Min, Max imgui.Vec2

	// ClipMin and ClipMax bound the part of the window the widget can be seen
	// in. A widget scrolled out of view is still laid out, just clipped away.
	ClipMin, ClipMax imgui.Vec2

	// Checked is set for a ticked menu item or check box.
	Checked bool
}

// Recorder receives every widget the interface lays out.
type Recorder interface {
	Record(item RecordedItem)
}

var recorder Recorder

// SetRecorder installs a recorder, or removes it when given nil. Only the
// interface tests install one.
func SetRecorder(r Recorder) {
	recorder = r
}

func record(label string, checked bool) {
	if recorder == nil {
		return
	}

	recordIn(currentPlace(), label, checked)
}

// recordIn is record for a widget that may have changed the current window by
// the time it returns, such as a menu that has just opened.
func recordIn(where place, label string, checked bool) {
	if recorder == nil {
		return
	}

	recorder.Record(RecordedItem{
		ID:      imgui.ItemID(),
		Label:   label,
		Window:  where.window,
		Min:     imgui.ItemRectMin(),
		Max:     imgui.ItemRectMax(),
		ClipMin: where.clipMin,
		ClipMax: where.clipMax,
		Checked: checked,
	})
}

// Record makes the interface tests aware of something drawn without one of the
// widgets here, such as an image drawn through the backend, under a name of
// its own. It records whatever was drawn last.
func Record(label string) {
	record(label, false)
}

// Button is imgui.Button.
func Button(label string) bool {
	pressed := imgui.Button(label)
	record(label, false)

	return pressed
}

// Checkbox is imgui.Checkbox.
func Checkbox(label string, value *bool) bool {
	changed := imgui.Checkbox(label, value)
	record(label, *value)

	return changed
}

// BeginMenu is imgui.BeginMenu. The recorded item is the menu's entry in the
// menu bar, which is what opens it.
func BeginMenu(label string) bool {
	// An open menu makes its own window current, so the window the entry sits
	// in has to be read before.
	var where place
	if recorder != nil {
		where = currentPlace()
	}

	open := imgui.BeginMenu(label)
	recordIn(where, label, false)

	return open
}

// place is where a widget is being laid out: the window and the part of it
// that is visible.
type place struct {
	window           string
	clipMin, clipMax imgui.Vec2
}

func currentPlace() place {
	current := imgui.InternalCurrentWindowRead()
	if current == nil {
		return place{}
	}

	drawList := imgui.WindowDrawList()

	return place{window: current.Name(), clipMin: drawList.ClipRectMin(), clipMax: drawList.ClipRectMax()}
}

// MenuItem is a menu item that runs an action.
func MenuItem(label, shortcut string, enabled bool) bool {
	clicked := imgui.MenuItemBoolV(label, shortcut, false, enabled)
	record(label, false)

	return clicked
}

// MenuToggle is a menu item that ticks and unticks a setting.
func MenuToggle(label, shortcut string, value *bool) bool {
	clicked := imgui.MenuItemBoolPtr(label, shortcut, value)
	record(label, *value)

	return clicked
}

// The room a list leaves around its rows, around the text of each row, and
// between one row and the next, in the units the interface was designed in.
var (
	listPadding    = imgui.Vec2{X: 6, Y: 6}
	listRowPadding = imgui.Vec2{X: 8, Y: 4}
)

const listRowGap = 2

// BeginList starts a scrolling list of rows that fills the space left, on a
// background of its own with room around the rows. It is ended with EndList,
// whatever it returns.
func BeginList(id string) bool {
	imgui.PushStyleColorVec4(imgui.ColChildBg, *imgui.StyleColorVec4(imgui.ColFrameBg))
	imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, ScaledVec2(listPadding.X, listPadding.Y))
	imgui.PushStyleVarFloat(imgui.StyleVarChildRounding, imgui.CurrentStyle().FrameRounding())

	open := imgui.BeginChildStrV(id, imgui.Vec2{}, imgui.ChildFlagsAlwaysUseWindowPadding, 0)

	imgui.PopStyleVarV(2)
	imgui.PopStyleColor()

	imgui.PushStyleVarVec2(imgui.StyleVarItemSpacing, imgui.Vec2{X: imgui.CurrentStyle().ItemSpacing().X, Y: Scaled(listRowGap)})

	return open
}

// EndList ends a list BeginList started.
func EndList() {
	imgui.PopStyleVar()
	imgui.EndChild()
}

// ListRow is a row of a list that can be picked, as wide as the list, with
// room around its text. It reports whether it was clicked.
func ListRow(label string, selected bool) bool {
	padding := ScaledVec2(listRowPadding.X, listRowPadding.Y)

	// The list sits on the colour selectables are highlighted in elsewhere,
	// so its rows are highlighted in the accent.
	imgui.PushStyleColorVec4(imgui.ColHeader, withAlpha(colorAccent, 0.55))
	imgui.PushStyleColorVec4(imgui.ColHeaderHovered, withAlpha(colorAccent, 0.35))
	imgui.PushStyleColorVec4(imgui.ColHeaderActive, colorAccent)

	clicked := imgui.SelectableBoolV("##"+label, selected, 0,
		imgui.Vec2{Y: imgui.TextLineHeight() + 2*padding.Y})
	record(label, selected)

	imgui.PopStyleColorV(3)

	origin := imgui.ItemRectMin()
	imgui.WindowDrawList().AddTextVec2(imgui.Vec2{X: origin.X + padding.X, Y: origin.Y + padding.Y},
		imgui.ColorU32Col(imgui.ColText), label)

	return clicked
}

// ListPadding is the room a list leaves above and below its rows.
func ListPadding() float32 {
	return Scaled(listPadding.Y)
}

// ListRowPitch is how far one row of a list is below the one before it.
func ListRowPitch() float32 {
	return imgui.TextLineHeight() + 2*Scaled(listRowPadding.Y) + Scaled(listRowGap)
}

// InputText is a text field with a hint shown while it is empty.
func InputText(label, hint string, text *string) bool {
	changed := imgui.InputTextWithHint(fieldLabel(label), hint, text, 0, nil)
	record(label, false)

	return changed
}

// labelColumn is how far in from the left the fields of a form start, in the
// units the interface was designed in. Labels longer than that push their own
// field further right.
const labelColumn = 110

// fieldLabel lays out a form row: the label on the left, in a column shared
// by every row, and the field filling the rest of the line. Dear ImGui puts
// labels to the right of their field by default, which reads backwards in a
// form. It returns the id the field is created with, which hides the label
// Dear ImGui would otherwise draw a second time.
//
// A label with no visible text, such as "##search", is left alone.
func fieldLabel(label string) string {
	visible, _, _ := strings.Cut(label, "##")
	if visible == "" {
		return label
	}

	Label(visible)

	// -FLT_MIN is Dear ImGui's way of saying "up to the right edge".
	imgui.SetNextItemWidth(-math.SmallestNonzeroFloat32)

	return "##" + label
}

// Label draws the label of a form row and moves the cursor to where the row's
// value starts, for a row whose value is not a single field.
func Label(text string) {
	start := imgui.CursorPosX()

	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted(text)

	spacing := imgui.CurrentStyle().ItemSpacing().X
	imgui.SameLineV(start+max(Scaled(labelColumn), imgui.CalcTextSize(text).X+spacing), 0)
}

// FindWindow looks a window up by name. cimgui-go wraps whatever Dear ImGui
// returns, a null pointer included, so a missing window would otherwise look
// found and crash on first use.
func FindWindow(name string) (*imgui.Window, bool) {
	window := imgui.InternalFindWindowByName(name)
	if window == nil || window.CData == nil {
		return nil, false
	}

	return window, true
}
