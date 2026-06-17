//go:build fuzz

package tui

import (
	"path/filepath"
	"strings"
	"testing"

	config "github.com/home-beekeeper/beekeeper/internal/config"
)

// FuzzSettingsHandleKey fuzzes the first-responder settings panel's key handler.
//
// The panel writes the user config.json — a security-relevant file — so two
// invariants must hold under ANY sequence of key presses, including adversarial
// or malformed key tokens:
//
//  1. No panic, and selIdx stays in [0, len(rows)) after every key (a bad index
//     would panic curRow()/renderRow on the next interaction).
//  2. The persist-never-writes-invalid-config invariant: after any sequence,
//     config.Load(path) must succeed. Because persist() validates the WHOLE
//     candidate (auto_quarantine AND corpus) before the atomic Save, the TUI must
//     never be able to leave an out-of-range threshold, a negative window, or a
//     bad scope on disk. A missing file (no edit happened) also loads cleanly as
//     defaults — so a non-nil error here is always a real regression.
//
// Mirrors the project's existing policy/IPC/corpus fuzz discipline.
//
// Run: go test -tags fuzz -fuzz=FuzzSettingsHandleKey -fuzztime=30s ./internal/tui/...
func FuzzSettingsHandleKey(f *testing.F) {
	// Seed with real key tokens and representative sequences so the mutator starts
	// from meaningful input. strings.Fields splits the seq into key tokens, so the
	// fuzzer can synthesize multi-char keys ("space", "down", "left") too.
	seeds := []string{
		"j k + - space enter esc up down left right h l = _",
		"down down down + + + space space",
		"k k k - - - - space",
		"+ + + + + + +",
		"- - - - - - -",
		"space space space space",
		"j j j j j j j j j j + -",
		"",
		"\x00 \x01 garbage \t weird-key",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, seq string) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.json")
		p := &SettingsPanel{
			adminMode:  true, // admin so edit paths (the risky ones) are exercised
			configPath: path,
			auditPath:  filepath.Join(dir, "beekeeper.ndjson"),
		}
		p.reload()

		for _, key := range strings.Fields(seq) {
			// Drive the command if one is returned (executes the closure; must not panic).
			if cmd := p.handleKey(key); cmd != nil {
				_ = cmd()
			}
			if p.selIdx < 0 || p.selIdx >= len(p.rows) {
				t.Fatalf("selIdx %d out of range [0,%d) after key %q", p.selIdx, len(p.rows), key)
			}
		}

		// Invariant: whatever the panel persisted (if anything) must load cleanly.
		if _, err := config.Load(path); err != nil {
			t.Fatalf("config.Load after key sequence %q returned a non-nil error (the panel persisted an invalid config): %v", seq, err)
		}
	})
}
