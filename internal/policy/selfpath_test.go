package policy

import "testing"

func TestEvaluateSelfPath(t *testing.T) {
	cfg := SelfProtectConfig{
		ReadWritePrefixes: []string{"c:/users/dev/appdata/roaming/beekeeper"},
		WriteOnlyPrefixes: []string{"c:/users/dev/go/bin"},
	}

	tests := []struct {
		name      string
		path      string
		isWrite   bool
		wantBlock bool
	}{
		// State dir: blocked for BOTH read and write (secret).
		{"state config read", "c:/users/dev/appdata/roaming/beekeeper/config.json", false, true},
		{"state config write", "c:/users/dev/appdata/roaming/beekeeper/config.json", true, true},
		{"state policies write", "c:/users/dev/appdata/roaming/beekeeper/policies/x.json", true, true},
		{"state audit read", "c:/users/dev/appdata/roaming/beekeeper/audit/beekeeper.ndjson", false, true},
		{"state dir itself", "c:/users/dev/appdata/roaming/beekeeper", true, true},

		// Binary: write blocked, read allowed.
		{"binary overwrite", "c:/users/dev/go/bin/beekeeper.exe", true, true},
		{"binary read ok", "c:/users/dev/go/bin/beekeeper.exe", false, false},

		// Case-insensitive (Windows path case varies).
		{"state mixed case", "C:/Users/Dev/AppData/Roaming/Beekeeper/config.json", false, true},

		// Path-boundary safety: a sibling that merely shares the prefix string.
		{"sibling not blocked", "c:/users/dev/appdata/roaming/beekeeper-notes/x", true, false},

		// THE dev-repo concern: source tree under mzansi-agentive/beekeeper is NOT
		// under the state-dir prefix, so it is never blocked.
		{"dev repo source allowed", "c:/users/dev/mzansi-agentive/beekeeper/internal/check/handler.go", true, false},

		// Unrelated paths.
		{"unrelated read", "c:/users/dev/project/main.go", false, false},
		{"empty path", "", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := EvaluateSelfPath(tt.path, tt.isWrite, cfg)
			if tt.wantBlock {
				if d.Allow || d.Level != "block" {
					t.Errorf("EvaluateSelfPath(%q, write=%v) = Allow %v/Level %q, want block", tt.path, tt.isWrite, d.Allow, d.Level)
				}
				if len(d.RuleIDs) == 0 || d.RuleIDs[0] != ruleSelfPath {
					t.Errorf("block decision must carry rule %q, got %v", ruleSelfPath, d.RuleIDs)
				}
			} else if !d.Allow {
				t.Errorf("EvaluateSelfPath(%q, write=%v) = block, want allow", tt.path, tt.isWrite)
			}
		})
	}
}

// TestEvaluateSelfPathEmptyConfig: with no prefixes configured, nothing is blocked.
func TestEvaluateSelfPathEmptyConfig(t *testing.T) {
	d := EvaluateSelfPath("c:/users/dev/appdata/roaming/beekeeper/config.json", true, SelfProtectConfig{})
	if !d.Allow {
		t.Errorf("empty config must not block, got %+v", d)
	}
}
