package gui

import "testing"

func TestLiteral(t *testing.T) {
	for text, want := range map[string]string{"50%": "50%%", "plain": "plain", "%d of %s": "%%d of %%s"} {
		if got := literal(text); got != want {
			t.Errorf("literal(%q) = %q, want %q", text, got, want)
		}
	}
}
