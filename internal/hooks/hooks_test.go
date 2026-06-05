package hooks

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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

		// Verify the three real Cursor v1.7+ events are written (NOT "preToolUse").
		for _, event := range cursorEvents {
			hooks := f.Hooks[event]
			if len(hooks) != 1 {
				t.Fatalf("expected 1 hook for event %q, got %d", event, len(hooks))
			}
			h := hooks[0]
			if !h.FailClosed {
				t.Fatalf("event %q: failClosed must be true", event)
			}
			if h.Command != "beekeeper check --hook cursor" {
				t.Fatalf("event %q: expected command %q, got %q", event, "beekeeper check --hook cursor", h.Command)
			}
		}

		// Idempotency: install again, must not duplicate.
		if err := installCursor(hooksPath, false, &buf); err != nil {
			t.Fatalf("installCursor (2nd): %v", err)
		}
		data2, _ := os.ReadFile(hooksPath)
		var f2 cursorHooksFile
		json.Unmarshal(data2, &f2)
		for _, event := range cursorEvents {
			if len(f2.Hooks[event]) != 1 {
				t.Fatalf("idempotency failure: expected 1 hook for event %q, got %d", event, len(f2.Hooks[event]))
			}
		}
	})

	// correct_event_names: regression gate for T-10-09. Asserts preToolUse is
	// ABSENT and all three real Cursor v1.7+ events are present with failClosed:true
	// and "beekeeper check --hook cursor".
	t.Run("correct_event_names", func(t *testing.T) {
		dir := t.TempDir()
		hooksPath := filepath.Join(dir, "hooks.json")

		var buf bytes.Buffer
		if err := installCursor(hooksPath, false, &buf); err != nil {
			t.Fatalf("installCursor: %v", err)
		}

		var f cursorHooksFile
		data, _ := os.ReadFile(hooksPath)
		if err := json.Unmarshal(data, &f); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		// "preToolUse" must NOT be written — it does not exist in Cursor.
		if _, ok := f.Hooks["preToolUse"]; ok {
			t.Fatal("preToolUse must NOT be written — it does not exist in Cursor")
		}

		// All three real events must be present with the beekeeper hook.
		for _, event := range []string{"beforeShellExecution", "beforeMCPExecution", "beforeReadFile"} {
			if len(f.Hooks[event]) == 0 {
				t.Fatalf("expected beekeeper hook under event %q", event)
			}
			for _, h := range f.Hooks[event] {
				if h.Command == "beekeeper check --hook cursor" {
					if !h.FailClosed {
						t.Fatalf("event %q: failClosed must be true (Cursor is fail-OPEN by default)", event)
					}
				}
			}
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

		// The fixture has a "some-other-linter" entry under "preToolUse".
		// After install, beekeeper is added to the three real events;
		// the existing "preToolUse" key (foreign tool's data) is preserved.
		preToolUse := f.Hooks["preToolUse"]
		found := false
		for _, h := range preToolUse {
			if h.Command == "some-other-linter" {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("expected original hook (some-other-linter) under preToolUse to be preserved")
		}

		// The three real events must have the beekeeper entry with failClosed.
		for _, event := range cursorEvents {
			bkFound := false
			for _, h := range f.Hooks[event] {
				if h.Command == "beekeeper check --hook cursor" {
					bkFound = true
					if !h.FailClosed {
						t.Fatalf("event %q: beekeeper check hook must have failClosed: true", event)
					}
					break
				}
			}
			if !bkFound {
				t.Fatalf("beekeeper check --hook cursor not found under event %q after install", event)
			}
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
		configPath := filepath.Join(dir, "config.toml")

		var buf bytes.Buffer
		// Use the lower-level installCodex; config.toml goes via UserHomeDir normally,
		// but for this test we call ensureCodexFeaturesFlag directly.
		if err := installCodex(hooksPath, false, &buf); err != nil {
			t.Fatalf("installCodex: %v", err)
		}

		var f codexHooksFile
		data, _ := os.ReadFile(hooksPath)
		if err := json.Unmarshal(data, &f); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		// PreToolUse must have beekeeper check --hook codex.
		if !containsCodexHookByCommand(f.Hooks["PreToolUse"], "beekeeper check --hook codex") {
			t.Fatal("beekeeper check --hook codex not found in PreToolUse")
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

		// config.toml [features] hooks=true via ensureCodexFeaturesFlag directly.
		var cfgBuf bytes.Buffer
		if err := ensureCodexFeaturesFlag(configPath, &cfgBuf); err != nil {
			t.Fatalf("ensureCodexFeaturesFlag: %v", err)
		}
		cfgData, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("read config.toml: %v", err)
		}
		cfgContent := string(cfgData)
		if !strings.Contains(cfgContent, "[features]") {
			t.Fatal("config.toml must contain [features] section")
		}
		if !strings.Contains(cfgContent, "hooks = true") {
			t.Fatal("config.toml must contain hooks = true")
		}

		// Idempotency: calling ensureCodexFeaturesFlag again must not duplicate.
		var cfgBuf2 bytes.Buffer
		if err := ensureCodexFeaturesFlag(configPath, &cfgBuf2); err != nil {
			t.Fatalf("ensureCodexFeaturesFlag (2nd): %v", err)
		}
		cfgData2, _ := os.ReadFile(configPath)
		featureCount := strings.Count(string(cfgData2), "[features]")
		if featureCount != 1 {
			t.Fatalf("idempotency: expected 1 [features] section, got %d", featureCount)
		}
		hooksCount := strings.Count(string(cfgData2), "hooks = true")
		if hooksCount != 1 {
			t.Fatalf("idempotency: expected 1 'hooks = true' line, got %d", hooksCount)
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

		// Beekeeper check --hook codex must be present.
		if !containsCodexHookByCommand(f.Hooks["PreToolUse"], "beekeeper check --hook codex") {
			t.Fatal("beekeeper check --hook codex not found in PreToolUse after install")
		}
	})

	t.Run("config_toml_absent_then_created", func(t *testing.T) {
		// Verify ensureCodexFeaturesFlag creates config.toml from scratch.
		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.toml")

		var buf bytes.Buffer
		if err := ensureCodexFeaturesFlag(configPath, &buf); err != nil {
			t.Fatalf("ensureCodexFeaturesFlag: %v", err)
		}
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("config.toml was not created: %v", err)
		}
		content := string(data)
		if !strings.Contains(content, "[features]") {
			t.Fatal("created config.toml must contain [features] section")
		}
		if !strings.Contains(content, "hooks = true") {
			t.Fatal("created config.toml must contain hooks = true")
		}
	})

	t.Run("config_toml_existing_no_features", func(t *testing.T) {
		// Verify ensureCodexFeaturesFlag appends [features] to an existing file
		// that has other content but no [features] section.
		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.toml")
		existing := "[model]\nname = \"o3\"\n"
		os.WriteFile(configPath, []byte(existing), 0o644)

		var buf bytes.Buffer
		if err := ensureCodexFeaturesFlag(configPath, &buf); err != nil {
			t.Fatalf("ensureCodexFeaturesFlag: %v", err)
		}
		data, _ := os.ReadFile(configPath)
		content := string(data)
		if !strings.Contains(content, "[features]") {
			t.Fatal("must add [features] section")
		}
		if !strings.Contains(content, "hooks = true") {
			t.Fatal("must add hooks = true")
		}
		// Original content must be preserved.
		if !strings.Contains(content, "[model]") {
			t.Fatal("original [model] section must be preserved")
		}
		if !strings.Contains(content, "name = \"o3\"") {
			t.Fatal("original model name must be preserved")
		}
	})

	t.Run("config_toml_existing_features_no_hooks", func(t *testing.T) {
		// Verify ensureCodexFeaturesFlag inserts hooks=true into an existing
		// [features] section that doesn't have it.
		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.toml")
		existing := "[features]\nsomeOtherFlag = true\n"
		os.WriteFile(configPath, []byte(existing), 0o644)

		var buf bytes.Buffer
		if err := ensureCodexFeaturesFlag(configPath, &buf); err != nil {
			t.Fatalf("ensureCodexFeaturesFlag: %v", err)
		}
		data, _ := os.ReadFile(configPath)
		content := string(data)
		if !strings.Contains(content, "hooks = true") {
			t.Fatal("must add hooks = true to existing [features] section")
		}
		if !strings.Contains(content, "someOtherFlag = true") {
			t.Fatal("existing feature flag must be preserved")
		}
	})

	t.Run("config_toml_already_correct", func(t *testing.T) {
		// Verify ensureCodexFeaturesFlag is a no-op when [features] hooks=true
		// is already present.
		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.toml")
		existing := "[features]\nhooks = true\n"
		os.WriteFile(configPath, []byte(existing), 0o644)

		var buf bytes.Buffer
		if err := ensureCodexFeaturesFlag(configPath, &buf); err != nil {
			t.Fatalf("ensureCodexFeaturesFlag: %v", err)
		}
		data, _ := os.ReadFile(configPath)
		// Must still have exactly one occurrence.
		if strings.Count(string(data), "hooks = true") != 1 {
			t.Fatalf("idempotency: expected exactly 1 'hooks = true' line, got file: %s", data)
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
	t.Run("removes_from_all_events", func(t *testing.T) {
		dir := t.TempDir()
		hooksPath := filepath.Join(dir, "hooks.json")

		var buf bytes.Buffer

		// Install first.
		if err := installCursor(hooksPath, false, &buf); err != nil {
			t.Fatalf("install: %v", err)
		}

		// Verify all three events have the beekeeper hook.
		var fBefore cursorHooksFile
		data, _ := os.ReadFile(hooksPath)
		json.Unmarshal(data, &fBefore)
		for _, event := range cursorEvents {
			if !containsCursorHookByCommand(fBefore.Hooks[event], "beekeeper check --hook cursor") {
				t.Fatalf("expected beekeeper hook under event %q before uninstall", event)
			}
		}

		// Uninstall.
		buf.Reset()
		if err := uninstallCursor(hooksPath, false, &buf); err != nil {
			t.Fatalf("uninstall: %v", err)
		}

		// Verify beekeeper hook is gone from all three events.
		var f cursorHooksFile
		data, _ = os.ReadFile(hooksPath)
		json.Unmarshal(data, &f)
		for _, event := range cursorEvents {
			if containsCursorHookByCommand(f.Hooks[event], "beekeeper check --hook cursor") {
				t.Fatalf("beekeeper check --hook cursor should be removed from event %q after uninstall", event)
			}
		}
	})

	t.Run("preserves_foreign_hooks", func(t *testing.T) {
		dir := t.TempDir()
		hooksPath := copyFixture(t, "cursor_hooks.json", dir)

		var buf bytes.Buffer
		if err := installCursor(hooksPath, false, &buf); err != nil {
			t.Fatalf("install: %v", err)
		}
		buf.Reset()
		if err := uninstallCursor(hooksPath, false, &buf); err != nil {
			t.Fatalf("uninstall: %v", err)
		}

		var f cursorHooksFile
		data, _ := os.ReadFile(hooksPath)
		json.Unmarshal(data, &f)

		// The original "some-other-linter" entry in "preToolUse" must survive.
		if !containsCursorHookByCommand(f.Hooks["preToolUse"], "some-other-linter") {
			t.Fatal("foreign hook (some-other-linter) must survive uninstall")
		}
	})
}

// -----------------------------------------------------------------------
// TestInstallAugment — contract-shape tests for the Augment installer
// -----------------------------------------------------------------------

func TestInstallAugment(t *testing.T) {
	t.Run("from_absent", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")

		var buf bytes.Buffer
		if err := installAugment(settingsPath, false, &buf); err != nil {
			t.Fatalf("installAugment: %v", err)
		}

		m := readJSON(t, settingsPath)
		if _, ok := m["hooks"]; !ok {
			t.Fatal("expected hooks key in settings.json after install")
		}

		hooks := m["hooks"].(map[string]any)
		pre, ok := hooks["PreToolUse"].([]any)
		if !ok || len(pre) == 0 {
			t.Fatal("expected non-empty PreToolUse after install")
		}
		if !claudeEntriesContainCommand(pre, augmentPreCommand) {
			t.Fatalf("expected command %q in PreToolUse", augmentPreCommand)
		}
	})

	t.Run("preserves_existing_hooks", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")

		// Seed with a foreign PreToolUse entry.
		original := `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Write", "hooks": [{"type": "command", "command": "my-augment-guard.sh"}]}
    ]
  }
}`
		os.WriteFile(settingsPath, []byte(original), 0o644)

		var buf bytes.Buffer
		if err := installAugment(settingsPath, false, &buf); err != nil {
			t.Fatalf("installAugment: %v", err)
		}

		m := readJSON(t, settingsPath)
		hooks := m["hooks"].(map[string]any)
		pre := hooks["PreToolUse"].([]any)

		if !claudeEntriesContainCommand(pre, "my-augment-guard.sh") {
			t.Fatal("pre-existing PreToolUse hook must be preserved after install")
		}
		if !claudeEntriesContainCommand(pre, augmentPreCommand) {
			t.Fatalf("beekeeper %q must be added", augmentPreCommand)
		}
		if len(pre) != 2 {
			t.Fatalf("expected 2 PreToolUse entries (existing + beekeeper), got %d", len(pre))
		}
	})

	t.Run("idempotent", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")

		var buf bytes.Buffer
		if err := installAugment(settingsPath, false, &buf); err != nil {
			t.Fatalf("installAugment (1st): %v", err)
		}
		if err := installAugment(settingsPath, false, &buf); err != nil {
			t.Fatalf("installAugment (2nd): %v", err)
		}

		m := readJSON(t, settingsPath)
		hooks := m["hooks"].(map[string]any)
		pre := hooks["PreToolUse"].([]any)
		if len(pre) != 1 {
			t.Fatalf("idempotency: expected 1 PreToolUse entry, got %d", len(pre))
		}
	})

	t.Run("dry_run", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")

		var buf bytes.Buffer
		if err := installAugment(settingsPath, true, &buf); err != nil {
			t.Fatalf("installAugment dry-run: %v", err)
		}

		// File must not have been created.
		if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
			t.Fatal("dry-run must not create the settings file")
		}
		if !strings.Contains(buf.String(), "[dry-run]") {
			t.Fatalf("dry-run output must contain [dry-run], got: %s", buf.String())
		}
	})

	t.Run("uninstall_only_removes_beekeeper", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")

		// Seed with a foreign entry + install beekeeper.
		original := `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Write", "hooks": [{"type": "command", "command": "my-augment-guard.sh"}]}
    ]
  }
}`
		os.WriteFile(settingsPath, []byte(original), 0o644)

		var buf bytes.Buffer
		if err := installAugment(settingsPath, false, &buf); err != nil {
			t.Fatalf("installAugment: %v", err)
		}
		buf.Reset()
		if err := uninstallAugment(settingsPath, false, &buf); err != nil {
			t.Fatalf("uninstallAugment: %v", err)
		}

		m := readJSON(t, settingsPath)
		hooks := m["hooks"].(map[string]any)
		pre := hooks["PreToolUse"].([]any)

		if claudeEntriesContainCommand(pre, augmentPreCommand) {
			t.Fatalf("beekeeper %q must be removed after uninstall", augmentPreCommand)
		}
		if !claudeEntriesContainCommand(pre, "my-augment-guard.sh") {
			t.Fatal("foreign hook must survive uninstall")
		}
	})
}

// -----------------------------------------------------------------------
// TestInstallCodeBuddy — contract-shape tests for the CodeBuddy installer
// -----------------------------------------------------------------------

func TestInstallCodeBuddy(t *testing.T) {
	t.Run("from_absent", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")

		var buf bytes.Buffer
		if err := installCodeBuddy(settingsPath, false, &buf); err != nil {
			t.Fatalf("installCodeBuddy: %v", err)
		}

		m := readJSON(t, settingsPath)
		if _, ok := m["hooks"]; !ok {
			t.Fatal("expected hooks key in settings.json after install")
		}

		hooks := m["hooks"].(map[string]any)
		pre, ok := hooks["PreToolUse"].([]any)
		if !ok || len(pre) == 0 {
			t.Fatal("expected non-empty PreToolUse after install")
		}
		if !claudeEntriesContainCommand(pre, codebuddyPreCommand) {
			t.Fatalf("expected command %q in PreToolUse", codebuddyPreCommand)
		}
	})

	t.Run("preserves_existing_hooks", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")

		original := `{
  "hooks": {
    "PreToolUse": [
      {"matcher": ".*", "hooks": [{"type": "command", "command": "my-codebuddy-guard.sh"}]}
    ]
  }
}`
		os.WriteFile(settingsPath, []byte(original), 0o644)

		var buf bytes.Buffer
		if err := installCodeBuddy(settingsPath, false, &buf); err != nil {
			t.Fatalf("installCodeBuddy: %v", err)
		}

		m := readJSON(t, settingsPath)
		hooks := m["hooks"].(map[string]any)
		pre := hooks["PreToolUse"].([]any)

		if !claudeEntriesContainCommand(pre, "my-codebuddy-guard.sh") {
			t.Fatal("pre-existing PreToolUse hook must be preserved after install")
		}
		if !claudeEntriesContainCommand(pre, codebuddyPreCommand) {
			t.Fatalf("beekeeper %q must be added", codebuddyPreCommand)
		}
		if len(pre) != 2 {
			t.Fatalf("expected 2 PreToolUse entries, got %d", len(pre))
		}
	})

	t.Run("idempotent", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")

		var buf bytes.Buffer
		if err := installCodeBuddy(settingsPath, false, &buf); err != nil {
			t.Fatalf("installCodeBuddy (1st): %v", err)
		}
		if err := installCodeBuddy(settingsPath, false, &buf); err != nil {
			t.Fatalf("installCodeBuddy (2nd): %v", err)
		}

		m := readJSON(t, settingsPath)
		hooks := m["hooks"].(map[string]any)
		pre := hooks["PreToolUse"].([]any)
		if len(pre) != 1 {
			t.Fatalf("idempotency: expected 1 PreToolUse entry, got %d", len(pre))
		}
	})

	t.Run("dry_run", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")

		var buf bytes.Buffer
		if err := installCodeBuddy(settingsPath, true, &buf); err != nil {
			t.Fatalf("installCodeBuddy dry-run: %v", err)
		}
		if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
			t.Fatal("dry-run must not create the settings file")
		}
	})

	t.Run("uninstall_only_removes_beekeeper", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")

		original := `{
  "hooks": {
    "PreToolUse": [
      {"matcher": ".*", "hooks": [{"type": "command", "command": "my-codebuddy-guard.sh"}]}
    ]
  }
}`
		os.WriteFile(settingsPath, []byte(original), 0o644)

		var buf bytes.Buffer
		if err := installCodeBuddy(settingsPath, false, &buf); err != nil {
			t.Fatalf("installCodeBuddy: %v", err)
		}
		buf.Reset()
		if err := uninstallCodeBuddy(settingsPath, false, &buf); err != nil {
			t.Fatalf("uninstallCodeBuddy: %v", err)
		}

		m := readJSON(t, settingsPath)
		hooks := m["hooks"].(map[string]any)
		pre := hooks["PreToolUse"].([]any)

		if claudeEntriesContainCommand(pre, codebuddyPreCommand) {
			t.Fatalf("beekeeper %q must be removed after uninstall", codebuddyPreCommand)
		}
		if !claudeEntriesContainCommand(pre, "my-codebuddy-guard.sh") {
			t.Fatal("foreign hook must survive uninstall")
		}
	})
}

// -----------------------------------------------------------------------
// TestInstallQwen — contract-shape tests for the Qwen Code installer
// -----------------------------------------------------------------------

func TestInstallQwen(t *testing.T) {
	t.Run("from_absent", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")

		var buf bytes.Buffer
		if err := installQwen(settingsPath, false, &buf); err != nil {
			t.Fatalf("installQwen: %v", err)
		}

		m := readJSON(t, settingsPath)
		if _, ok := m["hooks"]; !ok {
			t.Fatal("expected hooks key in settings.json after install")
		}

		hooks := m["hooks"].(map[string]any)
		pre, ok := hooks["PreToolUse"].([]any)
		if !ok || len(pre) == 0 {
			t.Fatal("expected non-empty PreToolUse after install")
		}
		if !claudeEntriesContainCommand(pre, qwenPreCommand) {
			t.Fatalf("expected command %q in PreToolUse", qwenPreCommand)
		}
	})

	t.Run("preserves_existing_hooks", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")

		original := `{
  "hooks": {
    "PreToolUse": [
      {"matcher": ".*", "hooks": [{"type": "command", "command": "my-qwen-guard.sh"}]}
    ]
  }
}`
		os.WriteFile(settingsPath, []byte(original), 0o644)

		var buf bytes.Buffer
		if err := installQwen(settingsPath, false, &buf); err != nil {
			t.Fatalf("installQwen: %v", err)
		}

		m := readJSON(t, settingsPath)
		hooks := m["hooks"].(map[string]any)
		pre := hooks["PreToolUse"].([]any)

		if !claudeEntriesContainCommand(pre, "my-qwen-guard.sh") {
			t.Fatal("pre-existing PreToolUse hook must be preserved after install")
		}
		if !claudeEntriesContainCommand(pre, qwenPreCommand) {
			t.Fatalf("beekeeper %q must be added", qwenPreCommand)
		}
		if len(pre) != 2 {
			t.Fatalf("expected 2 PreToolUse entries, got %d", len(pre))
		}
	})

	t.Run("idempotent", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")

		var buf bytes.Buffer
		if err := installQwen(settingsPath, false, &buf); err != nil {
			t.Fatalf("installQwen (1st): %v", err)
		}
		if err := installQwen(settingsPath, false, &buf); err != nil {
			t.Fatalf("installQwen (2nd): %v", err)
		}

		m := readJSON(t, settingsPath)
		hooks := m["hooks"].(map[string]any)
		pre := hooks["PreToolUse"].([]any)
		if len(pre) != 1 {
			t.Fatalf("idempotency: expected 1 PreToolUse entry, got %d", len(pre))
		}
	})

	t.Run("dry_run", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")

		var buf bytes.Buffer
		if err := installQwen(settingsPath, true, &buf); err != nil {
			t.Fatalf("installQwen dry-run: %v", err)
		}
		if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
			t.Fatal("dry-run must not create the settings file")
		}
	})

	t.Run("uninstall_only_removes_beekeeper", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")

		original := `{
  "hooks": {
    "PreToolUse": [
      {"matcher": ".*", "hooks": [{"type": "command", "command": "my-qwen-guard.sh"}]}
    ]
  }
}`
		os.WriteFile(settingsPath, []byte(original), 0o644)

		var buf bytes.Buffer
		if err := installQwen(settingsPath, false, &buf); err != nil {
			t.Fatalf("installQwen: %v", err)
		}
		buf.Reset()
		if err := uninstallQwen(settingsPath, false, &buf); err != nil {
			t.Fatalf("uninstallQwen: %v", err)
		}

		m := readJSON(t, settingsPath)
		hooks := m["hooks"].(map[string]any)
		pre := hooks["PreToolUse"].([]any)

		if claudeEntriesContainCommand(pre, qwenPreCommand) {
			t.Fatalf("beekeeper %q must be removed after uninstall", qwenPreCommand)
		}
		if !claudeEntriesContainCommand(pre, "my-qwen-guard.sh") {
			t.Fatal("foreign hook must survive uninstall")
		}
	})
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

// -----------------------------------------------------------------------
// TestInstallCopilot — contract-shape tests for the Copilot installer
// -----------------------------------------------------------------------

func TestInstallCopilot(t *testing.T) {
	t.Run("from_absent", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")

		var buf bytes.Buffer
		if err := installCopilot(settingsPath, false, &buf); err != nil {
			t.Fatalf("installCopilot: %v", err)
		}

		m := readJSON(t, settingsPath)
		if _, ok := m["hooks"]; !ok {
			t.Fatal("expected hooks key in settings.json after install")
		}

		hooks := m["hooks"].(map[string]any)
		// Copilot uses "preToolUse" (camelCase) — the correct event for Copilot.
		pre, ok := hooks["preToolUse"].([]any)
		if !ok || len(pre) == 0 {
			t.Fatal("expected non-empty preToolUse after install")
		}
		if !claudeEntriesContainCommand(pre, copilotPreCommand) {
			t.Fatalf("expected command %q in preToolUse", copilotPreCommand)
		}
	})

	t.Run("preserves_existing_hooks", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")

		// Seed with a foreign preToolUse entry.
		original := `{
  "hooks": {
    "preToolUse": [
      {"matcher": "Write", "hooks": [{"type": "command", "command": "my-copilot-guard.sh"}]}
    ]
  }
}`
		if err := os.WriteFile(settingsPath, []byte(original), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}

		var buf bytes.Buffer
		if err := installCopilot(settingsPath, false, &buf); err != nil {
			t.Fatalf("installCopilot: %v", err)
		}

		m := readJSON(t, settingsPath)
		hooks := m["hooks"].(map[string]any)
		pre := hooks["preToolUse"].([]any)

		if !claudeEntriesContainCommand(pre, "my-copilot-guard.sh") {
			t.Fatal("pre-existing preToolUse hook must be preserved after install")
		}
		if !claudeEntriesContainCommand(pre, copilotPreCommand) {
			t.Fatalf("beekeeper %q must be added", copilotPreCommand)
		}
		if len(pre) != 2 {
			t.Fatalf("expected 2 preToolUse entries (existing + beekeeper), got %d", len(pre))
		}
	})

	t.Run("idempotent", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")

		var buf bytes.Buffer
		if err := installCopilot(settingsPath, false, &buf); err != nil {
			t.Fatalf("installCopilot (1st): %v", err)
		}
		if err := installCopilot(settingsPath, false, &buf); err != nil {
			t.Fatalf("installCopilot (2nd): %v", err)
		}

		m := readJSON(t, settingsPath)
		hooks := m["hooks"].(map[string]any)
		pre := hooks["preToolUse"].([]any)
		if len(pre) != 1 {
			t.Fatalf("idempotency: expected 1 preToolUse entry, got %d", len(pre))
		}
	})

	t.Run("dry_run", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")

		var buf bytes.Buffer
		if err := installCopilot(settingsPath, true, &buf); err != nil {
			t.Fatalf("installCopilot dry-run: %v", err)
		}

		// File must not have been created.
		if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
			t.Fatal("dry-run must not create the settings file")
		}
		if !strings.Contains(buf.String(), "[dry-run]") {
			t.Fatalf("dry-run output must contain [dry-run], got: %s", buf.String())
		}
	})

	t.Run("uninstall_only_removes_beekeeper", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")

		// Seed with a foreign entry + install beekeeper.
		original := `{
  "hooks": {
    "preToolUse": [
      {"matcher": "Write", "hooks": [{"type": "command", "command": "my-copilot-guard.sh"}]}
    ]
  }
}`
		if err := os.WriteFile(settingsPath, []byte(original), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}

		var buf bytes.Buffer
		if err := installCopilot(settingsPath, false, &buf); err != nil {
			t.Fatalf("installCopilot: %v", err)
		}
		buf.Reset()
		if err := uninstallCopilot(settingsPath, false, &buf); err != nil {
			t.Fatalf("uninstallCopilot: %v", err)
		}

		m := readJSON(t, settingsPath)
		hooks := m["hooks"].(map[string]any)
		pre := hooks["preToolUse"].([]any)

		if claudeEntriesContainCommand(pre, copilotPreCommand) {
			t.Fatalf("beekeeper %q must be removed after uninstall", copilotPreCommand)
		}
		if !claudeEntriesContainCommand(pre, "my-copilot-guard.sh") {
			t.Fatal("foreign hook must survive uninstall")
		}
	})
}

// -----------------------------------------------------------------------
// TestInstallGemini — contract-shape tests for the Gemini CLI installer
// -----------------------------------------------------------------------

func TestInstallGemini(t *testing.T) {
	t.Run("from_absent", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")

		var buf bytes.Buffer
		if err := installGemini(settingsPath, false, &buf); err != nil {
			t.Fatalf("installGemini: %v", err)
		}

		// Verify the file was created with a BeforeTool hook entry.
		var f geminiHooksFile
		data, _ := os.ReadFile(settingsPath)
		if err := json.Unmarshal(data, &f); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if !containsGeminiHookByCommand(f.Hooks, "beekeeper check --hook gemini") {
			t.Fatal("beekeeper check --hook gemini not found in hooks")
		}

		// Verify the entry has BeforeTool event.
		found := false
		for _, h := range f.Hooks {
			if h.Command == "beekeeper check --hook gemini" {
				if h.Event != "BeforeTool" {
					t.Fatalf("expected event BeforeTool, got %q", h.Event)
				}
				if h.Matcher == "" {
					t.Fatal("expected non-empty Matcher")
				}
				found = true
			}
		}
		if !found {
			t.Fatal("beekeeper check --hook gemini entry not found")
		}
	})

	t.Run("preserves_existing_hooks", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")

		// Seed with a foreign hook entry.
		original := `{"hooks": [{"event": "BeforeTool", "matcher": ".*", "command": "my-gemini-guard.sh"}]}`
		if err := os.WriteFile(settingsPath, []byte(original), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}

		var buf bytes.Buffer
		if err := installGemini(settingsPath, false, &buf); err != nil {
			t.Fatalf("installGemini: %v", err)
		}

		var f geminiHooksFile
		data, _ := os.ReadFile(settingsPath)
		json.Unmarshal(data, &f)

		// Both the original and beekeeper entry must exist.
		if !containsGeminiHookByCommand(f.Hooks, "my-gemini-guard.sh") {
			t.Fatal("pre-existing hook must be preserved")
		}
		if !containsGeminiHookByCommand(f.Hooks, "beekeeper check --hook gemini") {
			t.Fatal("beekeeper check --hook gemini must be added")
		}
		if len(f.Hooks) != 2 {
			t.Fatalf("expected 2 hook entries (existing + beekeeper), got %d", len(f.Hooks))
		}
	})

	t.Run("idempotent", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")

		var buf bytes.Buffer
		if err := installGemini(settingsPath, false, &buf); err != nil {
			t.Fatalf("installGemini (1st): %v", err)
		}
		if err := installGemini(settingsPath, false, &buf); err != nil {
			t.Fatalf("installGemini (2nd): %v", err)
		}

		var f geminiHooksFile
		data, _ := os.ReadFile(settingsPath)
		json.Unmarshal(data, &f)
		if len(f.Hooks) != 1 {
			t.Fatalf("idempotency: expected 1 hook entry, got %d", len(f.Hooks))
		}
	})

	t.Run("uninstall_removes_beekeeper_preserves_foreign", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")

		// Seed with a foreign entry + install beekeeper.
		original := `{"hooks": [{"event": "BeforeTool", "matcher": ".*", "command": "my-gemini-guard.sh"}]}`
		if err := os.WriteFile(settingsPath, []byte(original), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}

		var buf bytes.Buffer
		if err := installGemini(settingsPath, false, &buf); err != nil {
			t.Fatalf("installGemini: %v", err)
		}
		buf.Reset()
		if err := uninstallGemini(settingsPath, false, &buf); err != nil {
			t.Fatalf("uninstallGemini: %v", err)
		}

		var f geminiHooksFile
		data, _ := os.ReadFile(settingsPath)
		json.Unmarshal(data, &f)

		if containsGeminiHookByCommand(f.Hooks, "beekeeper check --hook gemini") {
			t.Fatal("beekeeper entry must be removed after uninstall")
		}
		if !containsGeminiHookByCommand(f.Hooks, "my-gemini-guard.sh") {
			t.Fatal("foreign hook must survive uninstall")
		}
	})
}

// -----------------------------------------------------------------------
// TestInstallAntigravity — contract-shape tests for the Antigravity installer
// -----------------------------------------------------------------------

func TestInstallAntigravity(t *testing.T) {
	t.Run("from_absent", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "hooks.json")

		var buf bytes.Buffer
		if err := installAntigravity(settingsPath, false, &buf); err != nil {
			t.Fatalf("installAntigravity: %v", err)
		}

		m := readJSON(t, settingsPath)
		if _, ok := m["hooks"]; !ok {
			t.Fatal("expected hooks key in hooks.json after install")
		}

		hooks := m["hooks"].(map[string]any)
		pre, ok := hooks["PreToolUse"].([]any)
		if !ok || len(pre) == 0 {
			t.Fatal("expected non-empty PreToolUse after install")
		}
		if !claudeEntriesContainCommand(pre, antigravityPreCommand) {
			t.Fatalf("expected command %q in PreToolUse", antigravityPreCommand)
		}
	})

	t.Run("preserves_existing_hooks", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "hooks.json")

		// Seed with a foreign PreToolUse entry.
		original := `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Write", "hooks": [{"type": "command", "command": "my-antigravity-guard.sh"}]}
    ]
  }
}`
		if err := os.WriteFile(settingsPath, []byte(original), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}

		var buf bytes.Buffer
		if err := installAntigravity(settingsPath, false, &buf); err != nil {
			t.Fatalf("installAntigravity: %v", err)
		}

		m := readJSON(t, settingsPath)
		hooks := m["hooks"].(map[string]any)
		pre := hooks["PreToolUse"].([]any)

		if !claudeEntriesContainCommand(pre, "my-antigravity-guard.sh") {
			t.Fatal("pre-existing PreToolUse hook must be preserved after install")
		}
		if !claudeEntriesContainCommand(pre, antigravityPreCommand) {
			t.Fatalf("beekeeper %q must be added", antigravityPreCommand)
		}
		if len(pre) != 2 {
			t.Fatalf("expected 2 PreToolUse entries (existing + beekeeper), got %d", len(pre))
		}
	})

	t.Run("idempotent", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "hooks.json")

		var buf bytes.Buffer
		if err := installAntigravity(settingsPath, false, &buf); err != nil {
			t.Fatalf("installAntigravity (1st): %v", err)
		}
		if err := installAntigravity(settingsPath, false, &buf); err != nil {
			t.Fatalf("installAntigravity (2nd): %v", err)
		}

		m := readJSON(t, settingsPath)
		hooks := m["hooks"].(map[string]any)
		pre := hooks["PreToolUse"].([]any)
		if len(pre) != 1 {
			t.Fatalf("idempotency: expected 1 PreToolUse entry, got %d", len(pre))
		}
	})

	t.Run("dry_run", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "hooks.json")

		var buf bytes.Buffer
		if err := installAntigravity(settingsPath, true, &buf); err != nil {
			t.Fatalf("installAntigravity dry-run: %v", err)
		}

		// File must not have been created.
		if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
			t.Fatal("dry-run must not create the settings file")
		}
		if !strings.Contains(buf.String(), "[dry-run]") {
			t.Fatalf("dry-run output must contain [dry-run], got: %s", buf.String())
		}
	})

	t.Run("uninstall_only_removes_beekeeper", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "hooks.json")

		// Seed with a foreign entry + install beekeeper.
		original := `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Write", "hooks": [{"type": "command", "command": "my-antigravity-guard.sh"}]}
    ]
  }
}`
		if err := os.WriteFile(settingsPath, []byte(original), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}

		var buf bytes.Buffer
		if err := installAntigravity(settingsPath, false, &buf); err != nil {
			t.Fatalf("installAntigravity: %v", err)
		}
		buf.Reset()
		if err := uninstallAntigravity(settingsPath, false, &buf); err != nil {
			t.Fatalf("uninstallAntigravity: %v", err)
		}

		m := readJSON(t, settingsPath)
		hooks := m["hooks"].(map[string]any)
		pre := hooks["PreToolUse"].([]any)

		if claudeEntriesContainCommand(pre, antigravityPreCommand) {
			t.Fatalf("beekeeper %q must be removed after uninstall", antigravityPreCommand)
		}
		if !claudeEntriesContainCommand(pre, "my-antigravity-guard.sh") {
			t.Fatal("foreign hook must survive uninstall")
		}
	})
}

// -----------------------------------------------------------------------
// TestInstallWindsurf — contract-shape tests for the Windsurf installer
// -----------------------------------------------------------------------

func TestInstallWindsurf(t *testing.T) {
	t.Run("from_absent", func(t *testing.T) {
		dir := t.TempDir()
		hooksPath := filepath.Join(dir, "hooks.json")

		var buf bytes.Buffer
		if err := installWindsurf(hooksPath, false, &buf); err != nil {
			t.Fatalf("installWindsurf: %v", err)
		}

		var f windsurfHooksFile
		data, _ := os.ReadFile(hooksPath)
		if err := json.Unmarshal(data, &f); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		// Assert all three pre_* events are written.
		for _, event := range windsurfEvents {
			if len(f.Hooks[event]) == 0 {
				t.Fatalf("expected beekeeper hook under event %q, got none", event)
			}
		}
	})

	t.Run("os_correct_key", func(t *testing.T) {
		// Assert the correct key is set for the current OS.
		// On Windows, PowerShell field must be set; on Linux/macOS, Command field.
		dir := t.TempDir()
		hooksPath := filepath.Join(dir, "hooks.json")

		var buf bytes.Buffer
		if err := installWindsurf(hooksPath, false, &buf); err != nil {
			t.Fatalf("installWindsurf: %v", err)
		}

		var f windsurfHooksFile
		data, _ := os.ReadFile(hooksPath)
		json.Unmarshal(data, &f)

		const cmd = "beekeeper check --hook windsurf"
		hooks := f.Hooks["pre_run_command"]
		if len(hooks) == 0 {
			t.Fatal("expected hook under pre_run_command")
		}
		h := hooks[0]

		if runtime.GOOS == "windows" {
			if h.PowerShell != cmd {
				t.Fatalf("on Windows: expected PowerShell=%q, got %q (Command=%q)", cmd, h.PowerShell, h.Command)
			}
			if h.Command != "" {
				t.Fatalf("on Windows: Command field must be empty, got %q", h.Command)
			}
		} else {
			if h.Command != cmd {
				t.Fatalf("on Linux/macOS: expected Command=%q, got %q (PowerShell=%q)", cmd, h.Command, h.PowerShell)
			}
			if h.PowerShell != "" {
				t.Fatalf("on Linux/macOS: PowerShell field must be empty, got %q", h.PowerShell)
			}
		}
	})

	t.Run("idempotent", func(t *testing.T) {
		dir := t.TempDir()
		hooksPath := filepath.Join(dir, "hooks.json")

		var buf bytes.Buffer
		if err := installWindsurf(hooksPath, false, &buf); err != nil {
			t.Fatalf("installWindsurf (1st): %v", err)
		}
		if err := installWindsurf(hooksPath, false, &buf); err != nil {
			t.Fatalf("installWindsurf (2nd): %v", err)
		}

		var f windsurfHooksFile
		data, _ := os.ReadFile(hooksPath)
		json.Unmarshal(data, &f)
		for _, event := range windsurfEvents {
			if len(f.Hooks[event]) != 1 {
				t.Fatalf("idempotency failure: expected 1 hook for event %q, got %d", event, len(f.Hooks[event]))
			}
		}
	})

	t.Run("uninstall_removes_beekeeper", func(t *testing.T) {
		dir := t.TempDir()
		hooksPath := filepath.Join(dir, "hooks.json")

		var buf bytes.Buffer
		if err := installWindsurf(hooksPath, false, &buf); err != nil {
			t.Fatalf("installWindsurf: %v", err)
		}
		buf.Reset()
		if err := uninstallWindsurf(hooksPath, false, &buf); err != nil {
			t.Fatalf("uninstallWindsurf: %v", err)
		}

		var f windsurfHooksFile
		data, _ := os.ReadFile(hooksPath)
		json.Unmarshal(data, &f)
		const cmd = "beekeeper check --hook windsurf"
		for _, event := range windsurfEvents {
			if containsWindsurfHookByCommand(f.Hooks[event], cmd) {
				t.Fatalf("beekeeper hook must be removed from event %q after uninstall", event)
			}
		}
	})

	t.Run("preserves_foreign_hooks", func(t *testing.T) {
		dir := t.TempDir()
		hooksPath := filepath.Join(dir, "hooks.json")

		// Seed with a foreign hook in pre_run_command.
		original := `{"hooks":{"pre_run_command":[{"command":"my-other-tool","timeout":5}]}}`
		if err := os.WriteFile(hooksPath, []byte(original), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}

		var buf bytes.Buffer
		if err := installWindsurf(hooksPath, false, &buf); err != nil {
			t.Fatalf("installWindsurf: %v", err)
		}
		buf.Reset()
		if err := uninstallWindsurf(hooksPath, false, &buf); err != nil {
			t.Fatalf("uninstallWindsurf: %v", err)
		}

		var f windsurfHooksFile
		data, _ := os.ReadFile(hooksPath)
		json.Unmarshal(data, &f)

		// Foreign hook must survive.
		if !containsWindsurfHookByCommand(f.Hooks["pre_run_command"], "my-other-tool") {
			t.Fatal("foreign hook must survive uninstall")
		}
	})
}

// -----------------------------------------------------------------------
// TestInstallDispatchNewTargets — dispatch coverage for all four new targets
// -----------------------------------------------------------------------

// TestInstallDispatchNewTargets ensures each new target routes without error
// via the exported InstallTo and UninstallTo APIs. Uses temp files via
// internal helpers to avoid touching the real home directory.
func TestInstallDispatchNewTargets(t *testing.T) {
	targets := []struct {
		name    string
		install func(dir string) error
	}{
		{
			name: TargetCopilot,
			install: func(dir string) error {
				return installCopilot(filepath.Join(dir, "settings.json"), false, &bytes.Buffer{})
			},
		},
		{
			name: TargetAntigravity,
			install: func(dir string) error {
				return installAntigravity(filepath.Join(dir, "hooks.json"), false, &bytes.Buffer{})
			},
		},
		{
			name: TargetGemini,
			install: func(dir string) error {
				return installGemini(filepath.Join(dir, "settings.json"), false, &bytes.Buffer{})
			},
		},
		{
			name: TargetWindsurf,
			install: func(dir string) error {
				return installWindsurf(filepath.Join(dir, "hooks.json"), false, &bytes.Buffer{})
			},
		},
	}
	for _, tc := range targets {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := tc.install(dir); err != nil {
				t.Fatalf("install %s: %v", tc.name, err)
			}
		})
	}
}
