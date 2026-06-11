package hooks

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestBeekeeperHookMarkers asserts the self-protection guard's marker set
// contains both the PreToolUse and PostToolUse command substrings. If either
// drifts, the content-aware guard could fail to recognize a Beekeeper hook entry
// and let an agent remove it.
func TestBeekeeperHookMarkers(t *testing.T) {
	want := map[string]bool{
		"beekeeper check":         false,
		"beekeeper audit-record":  false,
	}
	for _, m := range BeekeeperHookMarkers() {
		if _, ok := want[m]; ok {
			want[m] = true
		}
	}
	for m, found := range want {
		if !found {
			t.Errorf("BeekeeperHookMarkers() is missing required marker %q", m)
		}
	}
}

// TestHookConfigFilesAreHomeRooted asserts every hook-config path returned by
// HookConfigFiles(home) is non-empty and either rooted under home OR a
// documented "~"-prefixed sentinel for a harness not native to this OS — no
// empty entries and no absolute-path escape to an unexpected location.
//
// The lone sentinel case is Cline on Windows: Cline is a macOS/Linux-only
// harness (its Windows installer returns a "macOS/Linux only" error), so
// clineHooksDir on Windows (cline_windows.go) deliberately returns the
// non-expandable sentinel "~\\Documents\\Cline\\Rules\\Hooks" — it never matches
// a real (absolute) tool-call target, which is correct because there is no Cline
// hook file to protect on Windows. A genuine absolute-path escape (e.g. to
// C:\\Windows or /etc) would still fail this test.
func TestHookConfigFilesAreHomeRooted(t *testing.T) {
	home := t.TempDir()
	paths := HookConfigFiles(home)
	if len(paths) == 0 {
		t.Fatal("HookConfigFiles returned no paths")
	}

	cleanHome := filepath.Clean(home)
	prefix := cleanHome + string(filepath.Separator)
	for _, p := range paths {
		if p == "" {
			t.Error("HookConfigFiles returned an empty path")
			continue
		}
		if strings.HasPrefix(p, "~") {
			continue // documented non-native-harness sentinel (Cline on Windows)
		}
		cp := filepath.Clean(p)
		if cp == cleanHome || !strings.HasPrefix(cp, prefix) {
			t.Errorf("hook-config path %q is not rooted under home %q (and is not a ~ sentinel)", p, home)
		}
	}
}

// TestHookConfigFilesCoverSupportedHarnesses asserts the list covers the
// expected breadth of harness config files (one per supported harness path); a
// silent shrink would narrow the self-protection guard's coverage.
func TestHookConfigFilesCoverSupportedHarnesses(t *testing.T) {
	home := t.TempDir()
	paths := HookConfigFiles(home)
	// The current set spans 14 harness config-file paths (claude, cursor, codex
	// hooks+config, augment, codebuddy, qwen, copilot, antigravity, gemini,
	// windsurf, hermes, cline PreToolUse, opencode plugin). Guard against an
	// accidental shrink without pinning the exact number too tightly.
	if len(paths) < 14 {
		t.Errorf("HookConfigFiles returned %d paths, want >= 14 (one per supported harness config file)", len(paths))
	}
}
