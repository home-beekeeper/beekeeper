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

// TestHookGuardBlocksNeuterVectors covers finding #6: an agent that keeps the
// Beekeeper marker substring while NEUTERING enforcement must be blocked. The
// pre-fix guard checked only that the marker survived, so each of these slipped
// through. Each Edit below preserves the marker text but alters the
// marker-bearing invocation; all must now BLOCK.
func TestHookGuardBlocksNeuterVectors(t *testing.T) {
	const marker = "beekeeper check --hook claude-code"

	tests := []struct {
		name   string
		oldS   string
		newS   string
		expect bool // true = must block
	}{
		{
			name:   "append || true (always succeed)",
			oldS:   marker,
			newS:   marker + " || true",
			expect: true,
		},
		{
			name:   "append ; exit 0",
			oldS:   marker,
			newS:   marker + " ; exit 0",
			expect: true,
		},
		{
			name:   "comment out with trailing # disabled",
			oldS:   marker,
			newS:   marker + " # disabled",
			expect: true,
		},
		{
			name:   "wrap in echo (not executed)",
			oldS:   marker,
			newS:   "echo " + marker,
			expect: true,
		},
		{
			name:   "repoint command to a stub (marker survives only as a tail token)",
			oldS:   marker,
			newS:   "stub-hook --was " + marker,
			expect: true,
		},
		{
			// Control: a no-op edit that reproduces the marker invocation verbatim
			// must NOT block (proves we are not over-blocking benign rewrites).
			name:   "verbatim no-op edit keeps invocation intact",
			oldS:   marker,
			newS:   marker,
			expect: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := setHome(t)
			path := seedClaudeSettings(t, home, settingsWithBeekeeperAndGSD)
			d := evaluateHookGuard(editTC(path, tt.oldS, tt.newS))
			if tt.expect && d.Allow {
				t.Errorf("%s: expected BLOCK, got allow", tt.name)
			}
			if !tt.expect && !d.Allow {
				t.Errorf("%s: expected ALLOW, got block (reason %q)", tt.name, d.Reason)
			}
		})
	}
}

// TestHookGuardNeuterViaStubRepointWrite covers the Write-tool variant of the
// "repoint command to a stub while the marker survives elsewhere in the file"
// vector: the original marker-bearing command is replaced by a stub, and the
// marker lingers only inside an inert JSON comment-like string. Must BLOCK.
func TestHookGuardNeuterViaStubRepointWrite(t *testing.T) {
	home := setHome(t)
	path := seedClaudeSettings(t, home, settingsWithBeekeeperAndGSD)

	// The real PreToolUse command is now "stub-hook"; the marker text only
	// survives inside an inert "_note" field — enforcement is dead.
	neutered := `{"_note":"was beekeeper check --hook claude-code","hooks":{"PreToolUse":[{"command":"stub-hook"},{"command":"gsd-guard"}]}}`
	d := evaluateHookGuard(writeTC(path, neutered))
	if d.Allow {
		t.Error("repointing the command to a stub while the marker lingers in an inert field must BLOCK")
	}
}

// TestHookGuardNeuterPreservesNoCollateral re-asserts that the integrity check
// does not over-block: appending an unrelated hook AFTER our (verbatim) entry,
// or editing a sibling hook, still passes.
func TestHookGuardNeuterPreservesNoCollateral(t *testing.T) {
	home := setHome(t)
	path := seedClaudeSettings(t, home, settingsWithBeekeeperAndGSD)

	// Append a new unrelated hook entry; the beekeeper invocation token is byte
	// identical → allow.
	withExtra := `{"hooks":{"PreToolUse":[` +
		`{"command":"beekeeper check --hook claude-code"},` +
		`{"command":"gsd-guard"},` +
		`{"command":"some-new-linter"}` +
		`]}}`
	if d := evaluateHookGuard(writeTC(path, withExtra)); !d.Allow {
		t.Errorf("appending an unrelated hook while keeping our invocation verbatim must ALLOW, got block (%q)", d.Reason)
	}
}

// TestCLIGuardBlocksIndirectionBypasses covers finding #7: a mutating beekeeper
// subcommand reached through shell-string, command-substitution, env, or
// variable indirection must be blocked. The pre-fix guard inspected only the
// literal first program token per segment, so every vector below evaded it.
func TestCLIGuardBlocksIndirectionBypasses(t *testing.T) {
	bash := func(cmd string) policy.ToolCall {
		return policy.ToolCall{ToolName: "Bash", ToolInput: map[string]any{"command": cmd}}
	}
	tests := []struct {
		name      string
		cmd       string
		wantBlock bool
	}{
		{"sh -c double-quoted", `sh -c "beekeeper hooks uninstall --target claude-code"`, true},
		{"bash -lc single-quoted", `bash -lc 'beekeeper config set nudge.mode soft'`, true},
		{"sh -c protect uninstall", `sh -c "beekeeper protect uninstall"`, true},
		{"command substitution $(which)", `$(which beekeeper) hooks uninstall`, true},
		{"command substitution backtick", "`which beekeeper` hooks uninstall", true},
		{"env wrapper", "env beekeeper hooks uninstall", true},
		{"env with assignment then beekeeper", "env FOO=1 beekeeper hooks uninstall", true},
		{"variable indirection", "BK=beekeeper; $BK hooks uninstall", true},
		{"variable indirection braces", "BK=beekeeper; ${BK} hooks uninstall", true},
		{"nested sh -c inside bash -c", `bash -c "sh -c 'beekeeper hooks uninstall'"`, true},
		{"command substitution dollar of path-ish which", "$(command -v beekeeper) protect uninstall", true},

		// False-positive guards — these must NOT block:
		{"commit message mentions phrase (prose)", `git commit -m "ran beekeeper config set earlier"`, false},
		{"echo of phrase inside quotes", `echo "a; beekeeper config set"`, false},
		{"sh -c with a benign beekeeper read", `sh -c "beekeeper scan"`, false},
		{"sh -c non-mutating check", `sh -c "beekeeper check --hook claude-code"`, false},
		{"which of a different tool", "$(which git) commit", false},
		{"variable bound to non-beekeeper", "BK=git; $BK config set x y", false},
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
