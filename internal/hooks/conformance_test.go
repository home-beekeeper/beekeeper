package hooks

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// isJSONFileTarget reports whether target is one of the settings.json
// file-writers (fileTargets) that use the backupSettings helper. Hermes, Cline,
// and OpenCode write custom formats and are not guaranteed to create a
// *.beekeeper-backup-* file on overwrite.
func isJSONFileTarget(target string) bool {
	for _, t := range fileTargets {
		if t == target {
			return true
		}
	}
	return false
}

func filesUnder(t *testing.T, dir string, backups bool) []string {
	t.Helper()
	var out []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		isBackup := strings.Contains(d.Name(), "beekeeper-backup")
		if isBackup == backups {
			out = append(out, p)
		}
		return nil
	})
	return out
}

// hookMarkerCount counts occurrences of the stable beekeeper PreToolUse hook
// marker across the given files. Idempotent installs keep this at exactly 1.
//
// The marker is "check --hook" rather than "beekeeper check" because abspath
// installs embed the binary path in JSON, which is JSON-escaped in the raw
// file bytes (e.g. `beekeeper\" check`). The suffix "check --hook" is present
// in both the bare-name form ("beekeeper check --hook X") and the abspath form
// ("\"...beekeeper...\" check --hook X" → raw bytes contain "check --hook").
//
// Hermes uses a YAML line ("    - command: ... check --hook hermes"), and
// Cline uses a shell script line ("... check --hook cline"). Both also contain
// "check --hook", so this counter covers all harness formats uniformly.
func hookMarkerCount(t *testing.T, files []string) int {
	t.Helper()
	n := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		n += strings.Count(string(b), "check --hook")
	}
	return n
}

// TestHarnessConformance asserts every one of the 17 allTargets has uniform
// installer-config conformance — no target is silently skipped (VAL-02). The
// deny-contract half is TestRenderDenyGolden (internal/check); the deep
// per-format preserve assertion is the existing per-harness tests (all JSON
// file-writers share the PatchSettings helper proven by
// TestInstallClaudeCodePreservesExistingHooks).
func TestHarnessConformance(t *testing.T) {
	if len(allTargets) != 17 {
		t.Fatalf("allTargets has %d entries, want 17 (roster drift)", len(allTargets))
	}

	for _, target := range allTargets {
		t.Run(target, func(t *testing.T) {
			home := t.TempDir()
			// InstallTo derives all paths from os.UserHomeDir(); redirect it.
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)

			var buf bytes.Buffer

			// --- Gateway targets: printed guide, NO file written ---
			if gatewayTargets[target] {
				if err := InstallTo(target, false, false, &buf); err != nil {
					t.Fatalf("InstallTo(%s) gateway guide: %v", target, err)
				}
				if buf.Len() == 0 {
					t.Errorf("%s should print a non-empty gateway guide", target)
				}
				if wrote := filesUnder(t, home, false); len(wrote) != 0 {
					t.Errorf("%s (gateway) wrote %d file(s); want none: %v", target, len(wrote), wrote)
				}
				if target == TargetKilo || target == TargetTrae {
					if !strings.Contains(strings.ToUpper(buf.String()), "UNGUARDED") {
						t.Errorf("%s guide must state UNGUARDED honestly (native tools have no enforceable hook)", target)
					}
				}
				return
			}

			// --- Cline on Windows: documented macOS/Linux-only error ---
			if target == TargetCline && runtime.GOOS == "windows" {
				if err := InstallTo(target, false, false, &buf); err == nil {
					t.Error("InstallTo(cline) on Windows should return a macOS/Linux-only error")
				}
				return
			}

			// --- File-writers: install creates a config with the hook marker ---
			if err := InstallTo(target, false, false, &buf); err != nil {
				t.Fatalf("InstallTo(%s) first install: %v", target, err)
			}
			files := filesUnder(t, home, false)
			if len(files) == 0 {
				t.Fatalf("%s wrote no config file under home", target)
			}
			// A harness may install >1 hook entry (e.g. Cursor wires
			// beforeShellExecution / beforeMCPExecution / beforeReadFile), so the
			// key contract is "at least one" and idempotency is "the count does not
			// grow on re-install" — NOT exactly one.
			count1 := hookMarkerCount(t, files)
			if count1 < 1 {
				t.Errorf("%s config does not contain the 'beekeeper check' hook marker (key not written)", target)
			}

			// Re-install: idempotent — the hook-entry count does not grow.
			buf.Reset()
			if err := InstallTo(target, false, false, &buf); err != nil {
				t.Fatalf("InstallTo(%s) re-install: %v", target, err)
			}
			if count2 := hookMarkerCount(t, filesUnder(t, home, false)); count2 != count1 {
				t.Errorf("%s: hook-entry count went %d -> %d on re-install (not idempotent — duplicate entries)", target, count1, count2)
			}

			// Backup-on-overwrite for the JSON settings file-writers.
			if isJSONFileTarget(target) {
				if b := filesUnder(t, home, true); len(b) == 0 {
					t.Errorf("%s: re-install must create a *.beekeeper-backup-* file (backup-on-overwrite)", target)
				}
			}
		})
	}
}
