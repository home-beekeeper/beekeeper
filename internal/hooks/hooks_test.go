package hooks

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// -----------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------

// copyFixture copies the named testdata file into dst (a temp directory).
// Returns the full destination path.
func copyFixture(t *testing.T, fixture, dst string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatalf("copyFixture: read %s: %v", fixture, err)
	}
	dest := filepath.Join(dst, fixture)
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		t.Fatalf("copyFixture: write %s: %v", dest, err)
	}
	return dest
}

// readJSON reads the JSON file at path into a map[string]any.
func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readJSON: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("readJSON: unmarshal %s: %v", path, err)
	}
	return m
}

// globFiles returns the paths of all files in dir matching pattern.
func globFiles(t *testing.T, dir, pattern string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	return matches
}

// -----------------------------------------------------------------------
// TestInstallClaudeCode
// -----------------------------------------------------------------------

func TestInstallClaudeCode(t *testing.T) {
	dir := t.TempDir()
	settingsPath := copyFixture(t, "claude_settings.json", dir)

	var buf bytes.Buffer
	if err := installClaudeCode(settingsPath, false, &buf); err != nil {
		t.Fatalf("installClaudeCode: %v", err)
	}

	// Verify hooks key is present.
	m := readJSON(t, settingsPath)
	if _, ok := m["hooks"]; !ok {
		t.Fatal("expected hooks key in settings.json after install")
	}

	// Verify backup was created.
	backups := globFiles(t, dir, "claude_settings.json.beekeeper-backup-*")
	if len(backups) == 0 {
		t.Fatal("expected backup file to be created")
	}

	// Idempotency: install again and verify hooks key still appears exactly once.
	if err := installClaudeCode(settingsPath, false, &buf); err != nil {
		t.Fatalf("installClaudeCode (2nd): %v", err)
	}
	m2 := readJSON(t, settingsPath)
	hooksRaw, ok := m2["hooks"]
	if !ok {
		t.Fatal("hooks key missing after second install")
	}
	hooksMap, ok := hooksRaw.(map[string]any)
	if !ok {
		t.Fatalf("hooks is not a map: %T", hooksRaw)
	}
	preToolUse, ok := hooksMap["PreToolUse"]
	if !ok {
		t.Fatal("PreToolUse key missing")
	}
	arr, ok := preToolUse.([]any)
	if !ok {
		t.Fatalf("PreToolUse is not an array: %T", preToolUse)
	}
	if len(arr) != 1 {
		t.Fatalf("expected exactly 1 PreToolUse entry, got %d (idempotency failure)", len(arr))
	}

	// Verify other settings keys are preserved.
	if m2["theme"] == nil {
		t.Fatal("expected theme key to be preserved after install")
	}
}

// -----------------------------------------------------------------------
// TestInstallClaudeCodeDryRun
// -----------------------------------------------------------------------

func TestInstallClaudeCodeDryRun(t *testing.T) {
	dir := t.TempDir()
	settingsPath := copyFixture(t, "claude_settings.json", dir)

	// Record original content.
	origData, _ := os.ReadFile(settingsPath)

	var buf bytes.Buffer
	if err := installClaudeCode(settingsPath, true, &buf); err != nil {
		t.Fatalf("installClaudeCode dry-run: %v", err)
	}

	// File must not have been modified.
	newData, _ := os.ReadFile(settingsPath)
	if !bytes.Equal(origData, newData) {
		t.Fatal("dry-run must not modify the settings file")
	}

	// No backup should be created during dry-run.
	backups := globFiles(t, dir, "claude_settings.json.beekeeper-backup-*")
	if len(backups) != 0 {
		t.Fatal("dry-run must not create backup files")
	}

	// Output must mention the path.
	if !strings.Contains(buf.String(), settingsPath) {
		t.Fatalf("dry-run output should mention the settings path, got: %s", buf.String())
	}
}

// -----------------------------------------------------------------------
// TestInstallCursor
// -----------------------------------------------------------------------

func TestInstallCursor(t *testing.T) {
	t.Run("from_absent", func(t *testing.T) {
		dir := t.TempDir()
		hooksPath := filepath.Join(dir, "hooks.json")

		var buf bytes.Buffer
		if err := installCursor(hooksPath, false, &buf); err != nil {
			t.Fatalf("installCursor: %v", err)
		}

		// Verify the file was created with the correct schema.
		var f cursorHooksFile
		data, _ := os.ReadFile(hooksPath)
		if err := json.Unmarshal(data, &f); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if f.Version != 1 {
			t.Fatalf("expected version 1, got %d", f.Version)
		}
		preToolUse := f.Hooks["preToolUse"]
		if len(preToolUse) != 1 {
			t.Fatalf("expected 1 preToolUse hook, got %d", len(preToolUse))
		}
		h := preToolUse[0]
		if !h.FailClosed {
			t.Fatal("failClosed must be true")
		}
		if h.Command != "beekeeper check" {
			t.Fatalf("expected command beekeeper check, got %q", h.Command)
		}

		// Idempotency: install again, must not duplicate.
		if err := installCursor(hooksPath, false, &buf); err != nil {
			t.Fatalf("installCursor (2nd): %v", err)
		}
		data2, _ := os.ReadFile(hooksPath)
		var f2 cursorHooksFile
		json.Unmarshal(data2, &f2)
		if len(f2.Hooks["preToolUse"]) != 1 {
			t.Fatalf("idempotency failure: expected 1 hook, got %d", len(f2.Hooks["preToolUse"]))
		}
	})

	t.Run("merge_with_existing", func(t *testing.T) {
		dir := t.TempDir()
		hooksPath := copyFixture(t, "cursor_hooks.json", dir)

		var buf bytes.Buffer
		if err := installCursor(hooksPath, false, &buf); err != nil {
			t.Fatalf("installCursor: %v", err)
		}

		var f cursorHooksFile
		data, _ := os.ReadFile(hooksPath)
		json.Unmarshal(data, &f)
		preToolUse := f.Hooks["preToolUse"]

		// Should have original entry + beekeeper entry = 2 entries.
		if len(preToolUse) != 2 {
			t.Fatalf("expected 2 preToolUse hooks (original + beekeeper), got %d", len(preToolUse))
		}

		// Verify the original entry was preserved.
		found := false
		for _, h := range preToolUse {
			if h.Command == "some-other-linter" {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("expected original hook (some-other-linter) to be preserved")
		}

		// Verify beekeeper entry has failClosed: true.
		bkFound := false
		for _, h := range preToolUse {
			if h.Command == "beekeeper check" {
				bkFound = true
				if !h.FailClosed {
					t.Fatal("beekeeper check hook must have failClosed: true")
				}
				break
			}
		}
		if !bkFound {
			t.Fatal("beekeeper check hook not found after install")
		}

		// Backup must exist.
		backups := globFiles(t, dir, "cursor_hooks.json.beekeeper-backup-*")
		if len(backups) == 0 {
			t.Fatal("expected backup file")
		}
	})
}

// -----------------------------------------------------------------------
// TestInstallCodex
// -----------------------------------------------------------------------

func TestInstallCodex(t *testing.T) {
	t.Run("from_absent", func(t *testing.T) {
		dir := t.TempDir()
		hooksPath := filepath.Join(dir, "hooks.json")

		var buf bytes.Buffer
		if err := installCodex(hooksPath, false, &buf); err != nil {
			t.Fatalf("installCodex: %v", err)
		}

		var f codexHooksFile
		data, _ := os.ReadFile(hooksPath)
		if err := json.Unmarshal(data, &f); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		// PreToolUse must have beekeeper check.
		if !containsCodexHookByCommand(f.Hooks["PreToolUse"], "beekeeper check") {
			t.Fatal("beekeeper check not found in PreToolUse")
		}

		// PostToolUse must have beekeeper audit-record.
		if !containsCodexHookByCommand(f.Hooks["PostToolUse"], "beekeeper audit-record") {
			t.Fatal("beekeeper audit-record not found in PostToolUse")
		}

		// Trust reminder must be printed.
		if !strings.Contains(buf.String(), "trust") {
			t.Fatalf("expected trust reminder in output, got: %s", buf.String())
		}

		// Idempotency: install again, must not duplicate.
		var buf2 bytes.Buffer
		if err := installCodex(hooksPath, false, &buf2); err != nil {
			t.Fatalf("installCodex (2nd): %v", err)
		}
		data2, _ := os.ReadFile(hooksPath)
		var f2 codexHooksFile
		json.Unmarshal(data2, &f2)
		if len(f2.Hooks["PreToolUse"]) != 1 {
			t.Fatalf("idempotency failure: expected 1 PreToolUse entry, got %d", len(f2.Hooks["PreToolUse"]))
		}
		if len(f2.Hooks["PostToolUse"]) != 1 {
			t.Fatalf("idempotency failure: expected 1 PostToolUse entry, got %d", len(f2.Hooks["PostToolUse"]))
		}
	})

	t.Run("merge_with_existing", func(t *testing.T) {
		dir := t.TempDir()
		hooksPath := copyFixture(t, "codex_hooks.json", dir)

		var buf bytes.Buffer
		if err := installCodex(hooksPath, false, &buf); err != nil {
			t.Fatalf("installCodex: %v", err)
		}

		var f codexHooksFile
		data, _ := os.ReadFile(hooksPath)
		json.Unmarshal(data, &f)

		// Should have 2 PreToolUse entries: original + beekeeper.
		if len(f.Hooks["PreToolUse"]) != 2 {
			t.Fatalf("expected 2 PreToolUse entries (original + beekeeper), got %d", len(f.Hooks["PreToolUse"]))
		}

		// Verify original hook was preserved.
		if !containsCodexHookByCommand(f.Hooks["PreToolUse"], "some-other-checker") {
			t.Fatal("expected original hook (some-other-checker) to be preserved")
		}

		// Beekeeper check must be present.
		if !containsCodexHookByCommand(f.Hooks["PreToolUse"], "beekeeper check") {
			t.Fatal("beekeeper check not found in PreToolUse after install")
		}
	})
}

// -----------------------------------------------------------------------
// TestInstallGatewayTarget
// -----------------------------------------------------------------------

func TestInstallGatewayTarget(t *testing.T) {
	targets := []string{TargetContinue, TargetOpenCode, TargetOpenClaw}

	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			dir := t.TempDir()
			var buf bytes.Buffer

			// No file should be written for gateway targets.
			filesBefore, _ := filepath.Glob(filepath.Join(dir, "*"))

			if err := printGatewayGuide(target, &buf); err != nil {
				t.Fatalf("printGatewayGuide(%s): %v", target, err)
			}

			filesAfter, _ := filepath.Glob(filepath.Join(dir, "*"))
			if len(filesAfter) != len(filesBefore) {
				t.Fatalf("printGatewayGuide must not write files; dir had %d files before, %d after",
					len(filesBefore), len(filesAfter))
			}

			// Output must mention the gateway URL.
			out := buf.String()
			if !strings.Contains(out, "127.0.0.1:7837") {
				t.Fatalf("expected gateway URL in output for %s, got: %s", target, out)
			}

			// Output must mention the token retrieval command.
			if !strings.Contains(out, "beekeeper gateway token") {
				t.Fatalf("expected 'beekeeper gateway token' in output for %s, got: %s", target, out)
			}
		})
	}
}

// -----------------------------------------------------------------------
// TestUninstallClaudeCode
// -----------------------------------------------------------------------

func TestUninstallClaudeCode(t *testing.T) {
	dir := t.TempDir()
	settingsPath := copyFixture(t, "claude_settings.json", dir)

	var buf bytes.Buffer

	// Install first.
	if err := installClaudeCode(settingsPath, false, &buf); err != nil {
		t.Fatalf("install: %v", err)
	}

	// Verify hooks key is present.
	m := readJSON(t, settingsPath)
	if _, ok := m["hooks"]; !ok {
		t.Fatal("hooks key should be present after install")
	}

	// Uninstall.
	buf.Reset()
	if err := uninstallClaudeCode(settingsPath, false, &buf); err != nil {
		t.Fatalf("uninstall: %v", err)
	}

	// Verify hooks key is gone.
	m2 := readJSON(t, settingsPath)
	if _, ok := m2["hooks"]; ok {
		t.Fatal("hooks key should be removed after uninstall")
	}

	// Other keys must be preserved.
	if m2["theme"] == nil {
		t.Fatal("theme key should be preserved after uninstall")
	}
}

// -----------------------------------------------------------------------
// TestInstallClaudeCodePreservesExistingHooks — regression for the clobber bug
// -----------------------------------------------------------------------

// A user with pre-existing Claude Code hooks must KEEP them after
// `beekeeper hooks install`, and uninstall must remove ONLY beekeeper's entries.
// Before the merge fix the installer overwrote the whole "hooks" key, silently
// destroying every non-beekeeper hook.
func TestInstallClaudeCodePreservesExistingHooks(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	original := `{
  "hooks": {
    "SessionStart": [
      {"hooks": [{"type": "command", "command": "my-session-init.sh"}]}
    ],
    "PreToolUse": [
      {"matcher": "Write|Edit", "hooks": [{"type": "command", "command": "my-existing-guard.js"}]}
    ]
  },
  "statusLine": {"type": "command", "command": "my-statusline.js"}
}`
	if err := os.WriteFile(settingsPath, []byte(original), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var buf bytes.Buffer
	if err := installClaudeCode(settingsPath, false, &buf); err != nil {
		t.Fatalf("install: %v", err)
	}

	m := readJSON(t, settingsPath)
	hooks := m["hooks"].(map[string]any)

	pre := hooks["PreToolUse"].([]any)
	if !claudeEntriesContainCommand(pre, "my-existing-guard.js") {
		t.Fatal("pre-existing PreToolUse guard was clobbered by install")
	}
	if !claudeEntriesContainCommand(pre, "beekeeper check --hook claude-code") {
		t.Fatal("beekeeper check --hook claude-code was not added to PreToolUse")
	}
	if len(pre) != 2 {
		t.Fatalf("expected 2 PreToolUse entries (existing + beekeeper), got %d", len(pre))
	}
	if _, ok := hooks["SessionStart"]; !ok {
		t.Fatal("SessionStart hooks were lost")
	}
	if m["statusLine"] == nil {
		t.Fatal("statusLine was lost")
	}
	post := hooks["PostToolUse"].([]any)
	if !claudeEntriesContainCommand(post, "beekeeper audit-record") {
		t.Fatal("beekeeper audit-record was not added to PostToolUse")
	}

	// Idempotent re-install.
	if err := installClaudeCode(settingsPath, false, &buf); err != nil {
		t.Fatalf("install (2nd): %v", err)
	}
	m = readJSON(t, settingsPath)
	hooks = m["hooks"].(map[string]any)
	if got := len(hooks["PreToolUse"].([]any)); got != 2 {
		t.Fatalf("idempotency: expected 2 PreToolUse entries, got %d", got)
	}

	// Uninstall removes ONLY beekeeper; the user's guard + SessionStart survive.
	if err := uninstallClaudeCode(settingsPath, false, &buf); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	m = readJSON(t, settingsPath)
	hooks = m["hooks"].(map[string]any)
	pre = hooks["PreToolUse"].([]any)
	if claudeEntriesContainCommand(pre, "beekeeper check --hook claude-code") {
		t.Fatal("beekeeper check --hook claude-code should be removed after uninstall")
	}
	if !claudeEntriesContainCommand(pre, "my-existing-guard.js") {
		t.Fatal("pre-existing guard must survive uninstall")
	}
	if _, ok := hooks["PostToolUse"]; ok {
		t.Fatal("empty PostToolUse array should be dropped after removing beekeeper")
	}
	if _, ok := hooks["SessionStart"]; !ok {
		t.Fatal("SessionStart hooks must survive uninstall")
	}
	if m["statusLine"] == nil {
		t.Fatal("statusLine must survive uninstall")
	}
}

// -----------------------------------------------------------------------
// TestInstallClaudeCodeWiresHookFlag — regression gate for HPC-01
// -----------------------------------------------------------------------

// TestInstallClaudeCodeWiresHookFlag asserts that the installed Claude Code hook
// uses "beekeeper check --hook claude-code" (not the bare "beekeeper check" that
// exits 1 on block — which every harness treats as a non-blocking soft error).
// This test is the installer-level counterpart of TestRenderDeny: it proves the
// installed command string ACTUALLY delivers the exit-2 deny adapter.
func TestInstallClaudeCodeWiresHookFlag(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	var buf bytes.Buffer
	if err := installClaudeCode(settingsPath, false, &buf); err != nil {
		t.Fatalf("installClaudeCode: %v", err)
	}

	// Read back and navigate to the installed PreToolUse command.
	m := readJSON(t, settingsPath)
	hooks, ok := m["hooks"].(map[string]any)
	if !ok {
		t.Fatal("hooks key missing or wrong type after install")
	}
	pre, ok := hooks["PreToolUse"].([]any)
	if !ok || len(pre) == 0 {
		t.Fatal("PreToolUse key missing or empty after install")
	}

	// Walk the entry array looking for the inner hooks command.
	const wantCmd = "beekeeper check --hook claude-code"
	found := false
	for _, entry := range pre {
		em, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		inner, ok := em["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range inner {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if cmd, _ := hm["command"].(string); cmd == wantCmd {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("installed PreToolUse command must be %q (not bare 'beekeeper check'); got entries: %v", wantCmd, pre)
	}

	// Idempotency: install again in the same settings; must not duplicate the entry.
	if err := installClaudeCode(settingsPath, false, &buf); err != nil {
		t.Fatalf("installClaudeCode (2nd): %v", err)
	}
	m2 := readJSON(t, settingsPath)
	hooks2 := m2["hooks"].(map[string]any)
	pre2 := hooks2["PreToolUse"].([]any)
	if len(pre2) != 1 {
		t.Fatalf("idempotency failure: expected 1 PreToolUse entry after 2nd install, got %d", len(pre2))
	}
}

// -----------------------------------------------------------------------
// TestUninstallCursor
// -----------------------------------------------------------------------

func TestUninstallCursor(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")

	var buf bytes.Buffer

	// Install first.
	if err := installCursor(hooksPath, false, &buf); err != nil {
		t.Fatalf("install: %v", err)
	}

	// Uninstall.
	buf.Reset()
	if err := uninstallCursor(hooksPath, false, &buf); err != nil {
		t.Fatalf("uninstall: %v", err)
	}

	// Verify preToolUse no longer contains beekeeper check.
	var f cursorHooksFile
	data, _ := os.ReadFile(hooksPath)
	json.Unmarshal(data, &f)
	if containsCursorHookByCommand(f.Hooks["preToolUse"], "beekeeper check") {
		t.Fatal("beekeeper check should be removed after uninstall")
	}
}

// -----------------------------------------------------------------------
// TestInstallUnknownTarget
// -----------------------------------------------------------------------

func TestInstallUnknownTarget(t *testing.T) {
	var buf bytes.Buffer
	err := InstallTo("not-a-real-agent", false, false, &buf)
	if err == nil {
		t.Fatal("expected non-nil error for unknown target")
	}
	if !strings.Contains(err.Error(), "unknown target") {
		t.Fatalf("expected 'unknown target' in error message, got: %v", err)
	}
}

// -----------------------------------------------------------------------
// TestInstallDispatch (full Install/Uninstall round-trip via exported API)
// -----------------------------------------------------------------------

func TestInstallDispatch(t *testing.T) {
	// Override home directory lookup by using installClaudeCode directly
	// through the internal path — this is covered by TestInstallClaudeCode.
	// This test checks that the dispatch itself works without error for a
	// gateway target (no file I/O, just guide printing).
	var buf bytes.Buffer
	if err := InstallTo(TargetContinue, false, false, &buf); err != nil {
		t.Fatalf("InstallTo(continue): %v", err)
	}
	if !strings.Contains(buf.String(), "Continue") {
		t.Fatalf("expected Continue guide, got: %s", buf.String())
	}
}
