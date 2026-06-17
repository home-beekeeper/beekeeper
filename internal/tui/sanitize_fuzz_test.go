//go:build fuzz

package tui

import (
	"strings"
	"testing"
)

// FuzzSanitizeForTUI is the security gate for the audit viewer's rendering path.
//
// Audit fields are attacker-influenceable, so the structured view runs every one
// through sanitizeForTUI before it reaches the terminal. The invariant: for ANY
// input and any max, the output contains NO rune that could drive or spoof the
// terminal (C0/C1 controls, ESC, DEL, bidi overrides, zero-width/BOM), and never
// exceeds max runes when max > 1.
//
// Run: go test -tags fuzz -fuzz=FuzzSanitizeForTUI -fuzztime=30s ./internal/tui/...
func FuzzSanitizeForTUI(f *testing.F) {
	seeds := []string{
		"",
		"npm install react",
		"\x1b[31mBLOCK\x1b[0m",
		"a\nb\tc\r",
		"‮evil-pkg",
		"ev​il",
		"café 日本 😀",
		"\x00\x01\x02\x7f\x9f",
		strings.Repeat("x", 4096),
	}
	for _, s := range seeds {
		f.Add(s, 40)
	}

	f.Fuzz(func(t *testing.T, s string, max int) {
		out := sanitizeForTUI(s, max)
		for _, r := range out {
			if isUnsafeRune(r) {
				t.Fatalf("sanitizeForTUI left unsafe rune %U in %q (input %q, max %d)", r, out, s, max)
			}
		}
		if max > 1 {
			if n := len([]rune(out)); n > max {
				t.Fatalf("sanitizeForTUI output %d runes > max %d (input %q)", n, max, s)
			}
		}
	})
}
