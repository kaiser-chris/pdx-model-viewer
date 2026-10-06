package app

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/kaiser-chris/pdx-model-viewer/internal/workspace"
)

// levelOfDetail matches the part of a shape's name that says which level of
// detail it belongs to, such as LOD_0 or lod1.
var levelOfDetail = regexp.MustCompile(`(?i)^lod_?(\d+)$`)

// partName turns the name a shape has in its mesh file, such as
// LOD_0|decal|decalShape, into one to read: Decal. A level of detail other
// than the first is added, so that the levels can be told apart.
func partName(name string) string {
	var words []string

	level := ""

	for _, segment := range strings.FieldsFunc(name, func(r rune) bool { return r == '|' || r == ':' }) {
		if match := levelOfDetail.FindStringSubmatch(segment); match != nil {
			level = match[1]

			continue
		}

		// Maya names every shape after its transform, with Shape on the
		// end, which says nothing.
		if trimmed := strings.TrimSuffix(segment, "Shape"); trimmed != "" {
			words = splitWords(trimmed)
		}
	}

	if len(words) == 0 {
		return name
	}

	readable := strings.Join(words, " ")
	readable = strings.ToUpper(readable[:1]) + readable[1:]

	if level != "" && level != "0" {
		readable += " (detail level " + level + ")"
	}

	return readable
}

// splitWords splits a name in snake case or camel case into its words, in
// lower case.
func splitWords(name string) []string {
	var words []string

	var word []rune

	flush := func() {
		if len(word) > 0 {
			words = append(words, strings.ToLower(string(word)))
			word = word[:0]
		}
	}

	runes := []rune(name)

	for index, r := range runes {
		switch {
		case r == '_' || r == '-' || r == ' ':
			flush()

			continue
		case unicode.IsUpper(r) && index > 0 && !unicode.IsUpper(runes[index-1]):
			flush()
		}

		word = append(word, r)
	}

	flush()

	return words
}

// partLook says in words how the viewer draws a part.
func partLook(part workspace.PartDetails) string {
	if !part.Drawn {
		return "Not drawn: the game uses it for something other than looks, such as collisions"
	}

	style := part.Style

	var look string

	switch {
	case style.Blend:
		look = "Laid over the model like a decal"
	case style.Cutout:
		look = "Cut out like leaves or hair"
	default:
		look = "Solid"
	}

	var more []string

	if style.Palette {
		more = append(more, "tinted with the palette colour")
	}

	if style.Tinted {
		more = append(more, "coloured by its tint")
	}

	if style.Foliage {
		more = append(more, "its grey coloured as leaves")
	}

	if style.TwoSided {
		more = append(more, "seen from both sides")
	}

	if style.Atlas {
		more = append(more, "textured from an atlas")
	}

	if len(more) > 0 {
		look += ", " + joinWords(more)
	}

	return look
}

// joinWords joins a list the way a sentence does: a, b and c.
func joinWords(words []string) string {
	if len(words) < 2 {
		return strings.Join(words, "")
	}

	return strings.Join(words[:len(words)-1], ", ") + " and " + words[len(words)-1]
}

// thousands writes a count with a comma between every three digits.
func thousands(count int) string {
	digits := strconv.Itoa(count)

	var out strings.Builder

	for index, digit := range digits {
		if index > 0 && (len(digits)-index)%3 == 0 {
			out.WriteByte(',')
		}

		out.WriteRune(digit)
	}

	return out.String()
}
