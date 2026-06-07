package check

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bantuson/beekeeper/internal/policy"
)

// setHome points os.UserHomeDir at a temp dir on both Windows and Unix.
func setHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)        // Unix
	t.Setenv("USERPROFILE", home) // Windows
	return home
}

func seedClaudeSettings(t *testing.T, home, content string) string {
	t.Helper()
	p := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

const settingsWithBeekeeperAndGSD = `{"hooks":{"PreToolUse":[` +
	`{"command":"beekeeper check --hook claude-code"},` +
	`{"command":"gsd-guard"}` +
	`]}}`

func editTC(path, oldS, newS string) policy.ToolCall {
	return policy.ToolCall{ToolName: "Edit", ToolInput: map[string]any{
		"file_path": path, "old_string": oldS, "new_string": newS,
	}}
}

func writeTC(path, content string) policy.ToolCall {
	return policy.ToolCall{ToolName: "Write", ToolInput: map[string]any{
		"file_path": path, "content": content,
	}}
}

func bashTC(cmd string) policy.ToolCall {
	return policy.ToolCall{ToolName: "Bash", ToolInput: map[string]any{"command": cmd}}
}

// TestHookGuardProtectsBeekeeperEntryNoCollateral is the headline: removing
// Beekeeper's hook entry is blocked, but editing OTHER hooks (GSD) in the same
// shared file is allowed.
func TestHookGuardProtectsBeekeeperEntryNoCollateral(t *testing.T) {
	home := setHome(t)
	path := seedClaudeSettings(t, home, settingsWithBeekeeperAndGSD)

	tests := []struct {
		name      string
		tc        policy.ToolCall
		wantBlock bool
	}{
		{"edit removes beekeeper entry", editTC(path, "beekeeper check --hook claude-code", ""), true},
		{"edit only touches GSD hook", editTC(path, "gsd-guard", "gsd-guard-v2"), false},
		{"write drops beekeeper entry", writeTC(path, `{"hooks":{"PreToolUse":[{"command":"gsd-guard"}]}}`), true},
		{"write keeps beekeeper + changes GSD", writeTC(path, settingsWithBeekeeperAndGSD+`{"x":"gsd-v2"}`), false},
		{"bash rewrites beekeeper-installed file", bashTC("echo {} > " + path), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := evaluateHookGuard(tt.tc)
			if tt.wantBlock && d.Allow {
				t.Errorf("%s: expected BLOCK, got allow", tt.name)
			}
			if !tt.wantBlock && !d.Allow {
				t.Errorf("%s: expected ALLOW, got block (reason %q)", tt.name, d.Reason)
			}
		})
	}
}

// TestHookGuardMultiEditAndIndeterminate covers MultiEdit and the conservative
// block when a write's resulting content can't be determined.
func TestHookGuardMultiEditAndIndeterminate(t *testing.T) {
	home := setHome(t)
	path := seedClaudeSettings(t, home, settingsWithBeekeeperAndGSD)

	multiEdit := func(edits []any) policy.ToolCall {
		return policy.ToolCall{ToolName: "MultiEdit", ToolInput: map[string]any{
			"file_path": path, "edits": edits,
		}}
	}

	// MultiEdit removing the Beekeeper entry → block.
	rm := multiEdit([]any{
		map[string]any{"old_string": "gsd-guard", "new_string": "gsd-guard-v2"},
		map[string]any{"old_string": "beekeeper check --hook claude-code", "new_string": ""},
	})
	if d := evaluateHookGuard(rm); d.Allow {
		t.Error("MultiEdit removing beekeeper entry must block")
	}

	// MultiEdit that keeps the Beekeeper entry → allow.
	keep := multiEdit([]any{
		map[string]any{"old_string": "gsd-guard", "new_string": "gsd-guard-v2"},
	})
	if d := evaluateHookGuard(keep); !d.Allow {
		t.Errorf("MultiEdit preserving beekeeper entry must allow, got block (%q)", d.Reason)
	}

	// Write with no content field on a beekeeper-installed file → indeterminate →
	// conservative block.
	indeterminate := policy.ToolCall{ToolName: "Write", ToolInput: map[string]any{"file_path": path}}
	if d := evaluateHookGuard(indeterminate); d.Allow {
		t.Error("indeterminate write to a beekeeper-installed hook file must block conservatively")
	}
}

// TestHookGuardIgnoresGSDOnlyFile: a hook file WITHOUT a Beekeeper entry is not
// our concern — removing a GSD hook from it is allowed (zero collateral).
func TestHookGuardIgnoresGSDOnlyFile(t *testing.T) {
	home := setHome(t)
	path := seedClaudeSettings(t, home, `{"hooks":{"PreToolUse":[{"command":"gsd-guard"}]}}`)
	d := evaluateHookGuard(editTC(path, "gsd-guard", ""))
	if !d.Allow {
		t.Errorf("editing a GSD-only file must be allowed, got block (reason %q)", d.Reason)
	}
}

// TestHookGuardIgnoresUnrelatedFile: a normal file edit is never a hook concern.
func TestHookGuardIgnoresUnrelatedFile(t *testing.T) {
	home := setHome(t)
	other := filepath.Join(home, "project", "main.go")
	if d := evaluateHookGuard(writeTC(other, "package main")); !d.Allow {
		t.Errorf("unrelated file write must be allowed, got block")
	}
}

func TestCLIGuard(t *testing.T) {
	bash := func(cmd string) policy.ToolCall {
		return policy.ToolCall{ToolName: "Bash", ToolInput: map[string]any{"command": cmd}}
	}
	tests := []struct {
		name      string
		cmd       string
		wantBlock bool
	}{
		{"config set blocked", "beekeeper config set nudge.mode soft", true},
		{"hooks uninstall blocked", "beekeeper hooks uninstall --target claude-code", true},
		{"compound hooks uninstall blocked", "cd /tmp && beekeeper hooks uninstall", true},
		{"env-prefixed protect install blocked", "FOO=bar beekeeper protect install", true},
		{"path-prefixed config set blocked", "./beekeeper config set nudge.mode soft", true},
		{"scan allowed", "beekeeper scan", false},
		{"policy list allowed", "beekeeper policy list", false},
		{"check allowed", "beekeeper check --hook claude-code", false},
		{"quoted phrase in commit msg allowed", `git commit -m "ran beekeeper config set earlier"`, false},
		{"quoted phrase in echo allowed", `echo "a; beekeeper config set"`, false},
		{"non-beekeeper program allowed", "config set foo bar", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := evaluateCLIGuard(bash(tt.cmd))
			if tt.wantBlock && d.Allow {
				t.Errorf("%q: expected BLOCK, got allow", tt.cmd)
			}
			if !tt.wantBlock && !d.Allow {
				t.Errorf("%q: expected ALLOW, got block (reason %q)", tt.cmd, d.Reason)
			}
		})
	}
}

func TestExtractBashWriteTargets(t *testing.T) {
	cases := []struct {
		cmd  string
		want string // a substring that must appear among extracted targets
	}{
		{"echo x > ~/.beekeeper/config.json", "~/.beekeeper/config.json"},
		{"echo x >> /tmp/log", "/tmp/log"},
		{"cp evil ~/.beekeeper/policies/y.json", "~/.beekeeper/policies/y.json"},
		{"rm ~/.beekeeper/audit/beekeeper.ndjson", "~/.beekeeper/audit/beekeeper.ndjson"},
		{"tee /etc/hosts", "/etc/hosts"},
	}
	for _, c := range cases {
		got := extractBashWriteTargets(c.cmd)
		found := false
		for _, g := range got {
			if g == c.want {
				found = true
			}
		}
		if !found {
			t.Errorf("extractBashWriteTargets(%q) = %v, want to contain %q", c.cmd, got, c.want)
		}
	}
}
