package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestConfigSetCmd_RejectsUnknownKey verifies that `config set` rejects any key
// fail-closed: there are currently no settable keys (the nudge.* keys were
// removed in v1.1.0).
func TestConfigSetCmd_RejectsUnknownKey(t *testing.T) {
	cmd := newConfigSetCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	for _, key := range []string{"nudge.mode", "fail_mode", "anything"} {
		err := cmd.RunE(cmd, []string{key, "value"})
		if err == nil {
			t.Errorf("RunE(%q): expected a non-nil error (no settable keys), got nil", key)
			continue
		}
		if !strings.Contains(err.Error(), "no settable config keys") {
			t.Errorf("RunE(%q): error = %v, want it to mention 'no settable config keys'", key, err)
		}
	}
}

// TestParseBoolRoundTrip exercises the retained parseBool helper.
func TestParseBoolRoundTrip(t *testing.T) {
	truthy := []string{"true", "1", "yes", "TRUE", " Yes "}
	for _, s := range truthy {
		b, err := parseBool(s)
		if err != nil || !b {
			t.Errorf("parseBool(%q) = (%v, %v), want (true, nil)", s, b, err)
		}
	}
	falsy := []string{"false", "0", "no", "FALSE"}
	for _, s := range falsy {
		b, err := parseBool(s)
		if err != nil || b {
			t.Errorf("parseBool(%q) = (%v, %v), want (false, nil)", s, b, err)
		}
	}
	if _, err := parseBool("maybe"); err == nil {
		t.Error("parseBool(maybe) = nil error, want a parse error")
	}
}
