package tui

import (
	"strings"
	"testing"
)

// assertNoUnsafe fails if s contains any rune that must never reach the terminal.
func assertNoUnsafe(t *testing.T, s string) {
	t.Helper()
	for _, r := range s {
		if isUnsafeRune(r) {
			t.Fatalf("unsafe rune %U survived in %q", r, s)
		}
	}
}

func TestSanitizeForTUI(t *testing.T) {
	// The sanitizer collapses each run of control/unsafe runes to a single space
	// (so a newline-separated phrase keeps its word boundary), then trims.
	cases := []struct {
		name, in, want string
		max            int
	}{
		{"empty", "", "", 40},
		{"plain", "npm install react", "npm install react", 40},
		{"neutralizes ESC/ANSI", "\x1b[31mBLOCK\x1b[0m", "[31mBLOCK [0m", 40},
		{"collapses newline+tab", "a\nb\tc", "a b c", 40},
		{"strips DEL and BEL", "a\x7fb\x07c", "a b c", 40},
		{"strips bidi override", "‮evil-pkg", "evil-pkg", 40},
		{"strips zero-width", "ev​il", "ev il", 40},
		{"keeps unicode", "café 日本 😀", "café 日本 😀", 40},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := sanitizeForTUI(c.in, c.max)
			if got != c.want {
				t.Errorf("sanitizeForTUI(%q) = %q, want %q", c.in, got, c.want)
			}
			assertNoUnsafe(t, got)
		})
	}
}

func TestSanitizeForTUITruncates(t *testing.T) {
	got := sanitizeForTUI(strings.Repeat("x", 100), 10)
	if n := len([]rune(got)); n != 10 {
		t.Fatalf("truncated length = %d runes, want 10", n)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncated string should end with an ellipsis, got %q", got)
	}
}

func TestSanitizeForTUINoTruncateWhenMaxTiny(t *testing.T) {
	// max <= 1 disables truncation (no room for content + ellipsis); the content
	// is still sanitized (ESC collapsed to a space).
	got := sanitizeForTUI("a\x1bb", 0)
	if got != "a b" {
		t.Errorf("got %q, want %q", got, "a b")
	}
}
