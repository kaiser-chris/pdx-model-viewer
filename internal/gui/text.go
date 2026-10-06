package gui

import (
	"strings"

	"github.com/AllenDang/cimgui-go/imgui"
)

// Dear ImGui takes the text of these widgets as a printf format, so a % in
// it, as in "50%" or a file name, would be read as the start of a directive
// and print whatever is on the stack. These take the text as it is.

// literal escapes a text for a printf format.
func literal(text string) string {
	return strings.ReplaceAll(text, "%", "%%")
}

// Tooltip is imgui.SetItemTooltip, for a text as it is.
func Tooltip(text string) {
	imgui.SetItemTooltip(literal(text))
}

// TextDisabled is imgui.TextDisabled, for a text as it is.
func TextDisabled(text string) {
	imgui.TextDisabled(literal(text))
}

// TextWrapped is imgui.TextWrapped, for a text as it is.
func TextWrapped(text string) {
	imgui.TextWrapped(literal(text))
}

// BulletText is imgui.BulletText, for a text as it is.
func BulletText(text string) {
	imgui.BulletText(literal(text))
}
