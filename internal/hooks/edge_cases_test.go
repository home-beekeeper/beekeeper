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

// This file extends coverage for the under-tested branches across the
// per-harness installers: the uninstall edge paths (file absent, no hooks key,
// no beekeeper entry, dry-run-with-removal), the codex uninstall round-trip,
// the exported Install/Uninstall dispatch wrappers, the gateway/opencode
// guides, the low-level helpers, and the Windows cline stub. It uses only
// t.TempDir() and t.Setenv so no real home directory is touched.

// -----------------------------------------------------------------------
// Uninstall edge cases shared by the settings.json-style installers.
// Each installer's uninstall has four "nothing to do" branches that the
// happy-path round-trip tests in hooks_test.go do not exercise:
//   1. target file absent             -> "nothing to uninstall"
//   2. file exists but no hooks key   -> "No hooks key found"
//   3. hooks present but no beekeeper -> "No beekeeper hooks found"
//   4. dry-run with a real removal    -> reports count, file unchanged
// -----------------------------------------------------------------------

// claudeShapedUninstaller bundles a uninstall fn with the foreign-fixture that
// has a hooks key but no beekeeper entry, plus a fixture string that is valid
// JSON with NO hooks key at all. All Claude-schema installers share shapes.
type uninstallCase struct {
	name string
	// foreignHooks is a settings JSON that has a "hooks" key with a non-beekeeper
	// PreToolUse entry. Uninstall on it must hit the removed==0 branch.
	foreignHooks string
	uninstall    func(path string, dryRun bool, out *bytes.Buffer) error
	// install populates the file so the dry-run-with-removal branch is reachable.
	install func(path string, out *bytes.Buffer) error
}

func TestUninstallEdgeCases_ClaudeSchema(t *testing.T) {
	const foreignNoBeekeeper = `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Write", "hooks": [{"type": "command", "command": "someone-elses-tool"}]}
    ]
  }
}`

	cases := []uninstallCase{
		{
			name:         "claude-code",
			foreignHooks: foreignNoBeekeeper,
			uninstall: func(p string, d bool, o *bytes.Buffer) error {
				return uninstallClaudeCode(p, d, o)
			},
			install: func(p string, o *bytes.Buffer) error { return installClaudeCode(p, false, o) },
		},
		{
			name:         "augment",
			foreignHooks: foreignNoBeekeeper,
			uninstall: func(p string, d bool, o *bytes.Buffer) error {
				return uninstallAugment(p, d, o)
			},
			install: func(p string, o *bytes.Buffer) error { return installAugment(p, false, o) },
		},
		{
			name:         "antigravity",
			foreignHooks: foreignNoBeekeeper,
			uninstall: func(p string, d bool, o *bytes.Buffer) error {
				return uninstallAntigravity(p, d, o)
			},
			install: func(p string, o *bytes.Buffer) error { return installAntigravity(p, false, o) },
		},
		{
			name:         "codebuddy",
			foreignHooks: foreignNoBeekeeper,
			uninstall: func(p string, d bool, o *bytes.Buffer) error {
				return uninstallCodeBuddy(p, d, o)
			},
			install: func(p string, o *bytes.Buffer) error { return installCodeBuddy(p, false, o) },
		},
		{
			name:         "qwen",
			foreignHooks: foreignNoBeekeeper,
			uninstall: func(p string, d bool, o *bytes.Buffer) error {
				return uninstallQwen(p, d, o)
			},
			install: func(p string, o *bytes.Buffer) error { return installQwen(p, false, o) },
		},
		{
			// Copilot uses the camelCase "preToolUse" event key.
			name: "copilot",
			foreignHooks: `{
  "hooks": {
    "preToolUse": [
      {"matcher": "Write", "hooks": [{"type": "command", "command": "someone-elses-tool"}]}
    ]
  }
}`,
			uninstall: func(p string, d bool, o *bytes.Buffer) error {
				return uninstallCopilot(p, d, o)
			},
			install: func(p string, o *bytes.Buffer) error { return installCopilot(p, false, o) },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 1. file absent.
			t.Run("file_absent", func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, "settings.json")
				var buf bytes.Buffer
				if err := tc.uninstall(path, false, &buf); err != nil {
					t.Fatalf("uninstall on absent file: %v", err)
				}
				if !strings.Contains(buf.String(), "nothing to uninstall") {
					t.Fatalf("expected 'nothing to uninstall' message, got: %s", buf.String())
				}
			})

			// 2. file exists but no hooks key.
			t.Run("no_hooks_key", func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, "settings.json")
				if err := os.WriteFile(path, []byte(`{"theme": "dark"}`), 0o644); err != nil {
					t.Fatalf("write fixture: %v", err)
				}
				var buf bytes.Buffer
				if err := tc.uninstall(path, false, &buf); err != nil {
					t.Fatalf("uninstall on no-hooks file: %v", err)
				}
				if !strings.Contains(buf.String(), "No hooks key") {
					t.Fatalf("expected 'No hooks key' message, got: %s", buf.String())
				}
			})

			// 3. hooks present but no beekeeper entry.
			t.Run("no_beekeeper_entry", func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, "settings.json")
				if err := os.WriteFile(path, []byte(tc.foreignHooks), 0o644); err != nil {
					t.Fatalf("write fixture: %v", err)
				}
				var buf bytes.Buffer
				if err := tc.uninstall(path, false, &buf); err != nil {
					t.Fatalf("uninstall on foreign-only file: %v", err)
				}
				if !strings.Contains(buf.String(), "No beekeeper hooks found") {
					t.Fatalf("expected 'No beekeeper hooks found' message, got: %s", buf.String())
				}
			})

			// 4. dry-run with a real removal: reports count, leaves the file intact.
			t.Run("dry_run_with_removal", func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, "settings.json")
				var buf bytes.Buffer
				if err := tc.install(path, &buf); err != nil {
					t.Fatalf("install: %v", err)
				}
				before, _ := os.ReadFile(path)
				buf.Reset()
				if err := tc.uninstall(path, true, &buf); err != nil {
					t.Fatalf("uninstall dry-run: %v", err)
				}
				if !strings.Contains(buf.String(), "[dry-run]") {
					t.Fatalf("expected [dry-run] in output, got: %s", buf.String())
				}
				after, _ := os.ReadFile(path)
				if !bytes.Equal(before, after) {
					t.Fatal("dry-run uninstall must not modify the file")
				}
			})
		})
	}
}

// TestUninstallEdgeCases_GeminiCursorWindsurf covers the custom-schema
// installers (Gemini flat array, Cursor/Windsurf per-event maps): absent file,
// no-beekeeper-entry, and dry-run-with-removal branches.
func TestUninstallEdgeCases_GeminiCursorWindsurf(t *testing.T) {
	t.Run("gemini", func(t *testing.T) {
		t.Run("file_absent", func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "settings.json")
			var buf bytes.Buffer
			if err := uninstallGemini(path, false, &buf); err != nil {
				t.Fatalf("uninstall: %v", err)
			}
			if !strings.Contains(buf.String(), "nothing to uninstall") {
				t.Fatalf("expected nothing-to-uninstall, got: %s", buf.String())
			}
		})
		t.Run("no_beekeeper_entry", func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "settings.json")
			os.WriteFile(path, []byte(`{"hooks":[{"event":"BeforeTool","matcher":".*","command":"foreign"}]}`), 0o644)
			var buf bytes.Buffer
			if err := uninstallGemini(path, false, &buf); err != nil {
				t.Fatalf("uninstall: %v", err)
			}
			if !strings.Contains(buf.String(), "No beekeeper hooks found") {
				t.Fatalf("expected no-beekeeper message, got: %s", buf.String())
			}
		})
		t.Run("dry_run_with_removal", func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "settings.json")
			var buf bytes.Buffer
			if err := installGemini(path, false, &buf); err != nil {
				t.Fatalf("install: %v", err)
			}
			before, _ := os.ReadFile(path)
			buf.Reset()
			if err := uninstallGemini(path, true, &buf); err != nil {
				t.Fatalf("uninstall dry-run: %v", err)
			}
			if !strings.Contains(buf.String(), "[dry-run]") {
				t.Fatalf("expected [dry-run], got: %s", buf.String())
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(before, after) {
				t.Fatal("dry-run must not modify the file")
			}
		})
	})

	t.Run("cursor", func(t *testing.T) {
		t.Run("file_absent", func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "hooks.json")
			var buf bytes.Buffer
			if err := uninstallCursor(path, false, &buf); err != nil {
				t.Fatalf("uninstall: %v", err)
			}
			if !strings.Contains(buf.String(), "nothing to uninstall") {
				t.Fatalf("expected nothing-to-uninstall, got: %s", buf.String())
			}
		})
		t.Run("no_beekeeper_entry", func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "hooks.json")
			os.WriteFile(path, []byte(`{"version":1,"hooks":{"beforeShellExecution":[{"command":"foreign"}]}}`), 0o644)
			var buf bytes.Buffer
			if err := uninstallCursor(path, false, &buf); err != nil {
				t.Fatalf("uninstall: %v", err)
			}
			if !strings.Contains(buf.String(), "No beekeeper check hooks found") {
				t.Fatalf("expected no-beekeeper message, got: %s", buf.String())
			}
		})
		t.Run("dry_run_with_removal", func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "hooks.json")
			var buf bytes.Buffer
			if err := installCursor(path, false, &buf); err != nil {
				t.Fatalf("install: %v", err)
			}
			before, _ := os.ReadFile(path)
			buf.Reset()
			if err := uninstallCursor(path, true, &buf); err != nil {
				t.Fatalf("uninstall dry-run: %v", err)
			}
			if !strings.Contains(buf.String(), "[dry-run]") {
				t.Fatalf("expected [dry-run], got: %s", buf.String())
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(before, after) {
				t.Fatal("dry-run must not modify the file")
			}
		})
	})

	t.Run("windsurf", func(t *testing.T) {
		t.Run("file_absent", func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "hooks.json")
			var buf bytes.Buffer
			if err := uninstallWindsurf(path, false, &buf); err != nil {
				t.Fatalf("uninstall: %v", err)
			}
			if !strings.Contains(buf.String(), "nothing to uninstall") {
				t.Fatalf("expected nothing-to-uninstall, got: %s", buf.String())
			}
		})
		t.Run("no_beekeeper_entry", func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "hooks.json")
			os.WriteFile(path, []byte(`{"hooks":{"pre_run_command":[{"command":"foreign"}]}}`), 0o644)
			var buf bytes.Buffer
			if err := uninstallWindsurf(path, false, &buf); err != nil {
				t.Fatalf("uninstall: %v", err)
			}
			if !strings.Contains(buf.String(), "No beekeeper check hooks found") {
				t.Fatalf("expected no-beekeeper message, got: %s", buf.String())
			}
		})
		t.Run("dry_run_with_removal", func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "hooks.json")
			var buf bytes.Buffer
			if err := installWindsurf(path, false, &buf); err != nil {
				t.Fatalf("install: %v", err)
			}
			before, _ := os.ReadFile(path)
			buf.Reset()
			if err := uninstallWindsurf(path, true, &buf); err != nil {
				t.Fatalf("uninstall dry-run: %v", err)
			}
			if !strings.Contains(buf.String(), "[dry-run]") {
				t.Fatalf("expected [dry-run], got: %s", buf.String())
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(before, after) {
				t.Fatal("dry-run must not modify the file")
			}
		})
	})
}

// -----------------------------------------------------------------------
// Codex uninstall — currently 0% covered.
// -----------------------------------------------------------------------

func TestUninstallCodex(t *testing.T) {
	t.Run("round_trip_removes_beekeeper", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "hooks.json")
		var buf bytes.Buffer
		if err := installCodex(path, false, &buf); err != nil {
			t.Fatalf("install: %v", err)
		}
		buf.Reset()
		if err := uninstallCodex(path, false, &buf); err != nil {
			t.Fatalf("uninstall: %v", err)
		}
		var f codexHooksFile
		data, _ := os.ReadFile(path)
		mustUnmarshal(t, data, &f)
		if containsCodexHookByCommand(f.Hooks["PreToolUse"], codexCheckSuffix) {
			t.Fatal("beekeeper PreToolUse entry must be removed after uninstall")
		}
		if containsCodexHookByCommand(f.Hooks["PostToolUse"], codexAuditSuffix) {
			t.Fatal("beekeeper PostToolUse entry must be removed after uninstall")
		}
	})

	t.Run("preserves_foreign", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "hooks.json")
		foreign := `{"hooks":{"PreToolUse":[{"matcher":".*","hooks":[{"type":"command","command":"some-other-checker"}]}]}}`
		os.WriteFile(path, []byte(foreign), 0o644)
		var buf bytes.Buffer
		if err := installCodex(path, false, &buf); err != nil {
			t.Fatalf("install: %v", err)
		}
		buf.Reset()
		if err := uninstallCodex(path, false, &buf); err != nil {
			t.Fatalf("uninstall: %v", err)
		}
		var f codexHooksFile
		data, _ := os.ReadFile(path)
		mustUnmarshal(t, data, &f)
		if !codexEntriesHaveRawCommand(f.Hooks["PreToolUse"], "some-other-checker") {
			t.Fatal("foreign hook must survive uninstall")
		}
		if containsCodexHookByCommand(f.Hooks["PreToolUse"], codexCheckSuffix) {
			t.Fatal("beekeeper hook must be removed")
		}
	})

	t.Run("file_absent", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "hooks.json")
		var buf bytes.Buffer
		if err := uninstallCodex(path, false, &buf); err != nil {
			t.Fatalf("uninstall on absent file: %v", err)
		}
		if !strings.Contains(buf.String(), "nothing to uninstall") {
			t.Fatalf("expected nothing-to-uninstall, got: %s", buf.String())
		}
	})

	t.Run("no_beekeeper_entry", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "hooks.json")
		os.WriteFile(path, []byte(`{"hooks":{"PreToolUse":[{"matcher":".*","hooks":[{"type":"command","command":"foreign"}]}]}}`), 0o644)
		var buf bytes.Buffer
		if err := uninstallCodex(path, false, &buf); err != nil {
			t.Fatalf("uninstall: %v", err)
		}
		if !strings.Contains(buf.String(), "No beekeeper hooks found") {
			t.Fatalf("expected no-beekeeper message, got: %s", buf.String())
		}
	})

	t.Run("malformed_json_errors", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "hooks.json")
		os.WriteFile(path, []byte(`{not valid json`), 0o644)
		var buf bytes.Buffer
		if err := uninstallCodex(path, false, &buf); err == nil {
			t.Fatal("expected parse error on malformed hooks.json")
		}
	})

	t.Run("dry_run_with_removal", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "hooks.json")
		var buf bytes.Buffer
		if err := installCodex(path, false, &buf); err != nil {
			t.Fatalf("install: %v", err)
		}
		before, _ := os.ReadFile(path)
		buf.Reset()
		if err := uninstallCodex(path, true, &buf); err != nil {
			t.Fatalf("uninstall dry-run: %v", err)
		}
		if !strings.Contains(buf.String(), "[dry-run]") {
			t.Fatalf("expected [dry-run], got: %s", buf.String())
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(before, after) {
			t.Fatal("dry-run must not modify the file")
		}
	})
}

// TestInstallCodexDryRun covers the codex install dry-run branch (describes the
// config.toml change too) without touching the real home directory's hooks.json.
func TestInstallCodexDryRun(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	path := filepath.Join(dir, "hooks.json")
	var buf bytes.Buffer
	if err := installCodex(path, true, &buf); err != nil {
		t.Fatalf("installCodex dry-run: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("dry-run must not create hooks.json")
	}
	out := buf.String()
	if !strings.Contains(out, "[dry-run]") {
		t.Fatalf("expected [dry-run] marker, got: %s", out)
	}
	if !strings.Contains(out, "[features] hooks = true") {
		t.Fatalf("dry-run output should describe the config.toml change, got: %s", out)
	}
}

// -----------------------------------------------------------------------
// Hermes — patchHermesConfig branches + uninstall edge cases.
// -----------------------------------------------------------------------

func TestPatchHermesConfigBranches(t *testing.T) {
	t.Run("hooks_exists_no_pre_tool_call", func(t *testing.T) {
		// hooks: section present but no pre_tool_call sub-key -> insert it.
		in := "model: hermes\nhooks:\n  post_tool_call:\n    - command: other\n"
		out := patchHermesConfig(in)
		if !strings.Contains(out, "pre_tool_call:") {
			t.Fatalf("expected pre_tool_call inserted, got:\n%s", out)
		}
		if !strings.Contains(out, hermesCheckSuffix) {
			t.Fatalf("expected beekeeper command, got:\n%s", out)
		}
		if !strings.Contains(out, "post_tool_call:") {
			t.Fatalf("existing post_tool_call must be preserved, got:\n%s", out)
		}
	})

	t.Run("pre_tool_call_exists", func(t *testing.T) {
		// hooks: with an existing pre_tool_call: -> append command under it.
		in := "hooks:\n  pre_tool_call:\n    - command: existing-pre\n"
		out := patchHermesConfig(in)
		if strings.Count(out, "- command:") != 2 {
			t.Fatalf("expected 2 command entries (existing + beekeeper), got:\n%s", out)
		}
		if !strings.Contains(out, "existing-pre") {
			t.Fatalf("existing command must be preserved, got:\n%s", out)
		}
		if !strings.Contains(out, hermesCheckSuffix) {
			t.Fatalf("beekeeper command must be added, got:\n%s", out)
		}
	})

	t.Run("hooks_then_top_level_section_breaks_scan", func(t *testing.T) {
		// A non-indented section after hooks: ends the section scan, so the
		// installer must insert a fresh pre_tool_call: under hooks:.
		in := "hooks:\nmodel: hermes\n"
		out := patchHermesConfig(in)
		if !strings.Contains(out, "pre_tool_call:") {
			t.Fatalf("expected pre_tool_call inserted under hooks:, got:\n%s", out)
		}
		if !strings.Contains(out, "model: hermes") {
			t.Fatalf("top-level model section must be preserved, got:\n%s", out)
		}
	})

	t.Run("empty_content", func(t *testing.T) {
		out := patchHermesConfig("")
		if !strings.Contains(out, hermesCheckSuffix) {
			t.Fatalf("empty input must yield the full hook block, got:\n%s", out)
		}
	})
}

func TestUninstallHermesEdgeCases(t *testing.T) {
	t.Run("file_absent", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		var buf bytes.Buffer
		if err := uninstallHermes(path, false, &buf); err != nil {
			t.Fatalf("uninstall: %v", err)
		}
		if !strings.Contains(buf.String(), "nothing to uninstall") {
			t.Fatalf("expected nothing-to-uninstall, got: %s", buf.String())
		}
	})

	t.Run("no_beekeeper_hook", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		os.WriteFile(path, []byte("model: hermes\n"), 0o644)
		var buf bytes.Buffer
		if err := uninstallHermes(path, false, &buf); err != nil {
			t.Fatalf("uninstall: %v", err)
		}
		if !strings.Contains(buf.String(), "No beekeeper hook found") {
			t.Fatalf("expected no-beekeeper message, got: %s", buf.String())
		}
	})

	t.Run("dry_run_with_removal", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		var buf bytes.Buffer
		if err := installHermes(path, false, &buf); err != nil {
			t.Fatalf("install: %v", err)
		}
		before, _ := os.ReadFile(path)
		buf.Reset()
		if err := uninstallHermes(path, true, &buf); err != nil {
			t.Fatalf("uninstall dry-run: %v", err)
		}
		if !strings.Contains(buf.String(), "[dry-run]") {
			t.Fatalf("expected [dry-run], got: %s", buf.String())
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(before, after) {
			t.Fatal("dry-run must not modify the file")
		}
	})

	t.Run("install_dry_run_with_existing", func(t *testing.T) {
		// installHermes dry-run when the file exists but lacks the hook: must
		// print the patched preview and not write.
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		os.WriteFile(path, []byte("model: hermes\n"), 0o644)
		before, _ := os.ReadFile(path)
		var buf bytes.Buffer
		if err := installHermes(path, true, &buf); err != nil {
			t.Fatalf("install dry-run: %v", err)
		}
		if !strings.Contains(buf.String(), "[dry-run]") {
			t.Fatalf("expected [dry-run], got: %s", buf.String())
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(before, after) {
			t.Fatal("dry-run must not modify the file")
		}
	})
}

// -----------------------------------------------------------------------
// OpenCode plugin — foreign-overwrite + uninstall edge cases.
// -----------------------------------------------------------------------

func TestOpenCodePluginForeignAndUninstallEdges(t *testing.T) {
	t.Run("install_overwrites_foreign_with_backup", func(t *testing.T) {
		dir := t.TempDir()
		path := openCodePluginPath(dir)
		os.WriteFile(path, []byte("// some unrelated opencode plugin\n"), 0o644)
		var buf bytes.Buffer
		if err := installOpenCodePlugin(dir, false, &buf); err != nil {
			t.Fatalf("install over foreign: %v", err)
		}
		if !strings.Contains(buf.String(), "Backed up existing") {
			t.Fatalf("expected backup warning, got: %s", buf.String())
		}
		data, _ := os.ReadFile(path)
		if !strings.Contains(string(data), openCodePluginMarker) {
			t.Fatal("beekeeper plugin must be installed over the foreign one")
		}
		backups, _ := filepath.Glob(filepath.Join(dir, "beekeeper.js.beekeeper-backup-*"))
		if len(backups) == 0 {
			t.Fatal("expected a backup of the foreign plugin")
		}
	})

	t.Run("install_dry_run_over_foreign", func(t *testing.T) {
		dir := t.TempDir()
		path := openCodePluginPath(dir)
		os.WriteFile(path, []byte("// foreign\n"), 0o644)
		before, _ := os.ReadFile(path)
		var buf bytes.Buffer
		if err := installOpenCodePlugin(dir, true, &buf); err != nil {
			t.Fatalf("install dry-run over foreign: %v", err)
		}
		if !strings.Contains(buf.String(), "[dry-run]") {
			t.Fatalf("expected [dry-run], got: %s", buf.String())
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(before, after) {
			t.Fatal("dry-run must not modify the foreign plugin")
		}
	})

	t.Run("uninstall_absent", func(t *testing.T) {
		dir := t.TempDir()
		var buf bytes.Buffer
		if err := uninstallOpenCodePlugin(dir, false, &buf); err != nil {
			t.Fatalf("uninstall absent: %v", err)
		}
		if !strings.Contains(buf.String(), "nothing to uninstall") {
			t.Fatalf("expected nothing-to-uninstall, got: %s", buf.String())
		}
	})

	t.Run("uninstall_preserves_foreign", func(t *testing.T) {
		dir := t.TempDir()
		path := openCodePluginPath(dir)
		os.WriteFile(path, []byte("// foreign plugin, not beekeeper\n"), 0o644)
		var buf bytes.Buffer
		if err := uninstallOpenCodePlugin(dir, false, &buf); err != nil {
			t.Fatalf("uninstall: %v", err)
		}
		if !strings.Contains(buf.String(), "not a beekeeper plugin") {
			t.Fatalf("expected foreign-preserved message, got: %s", buf.String())
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatal("foreign plugin must NOT be removed")
		}
	})

	t.Run("uninstall_dry_run", func(t *testing.T) {
		dir := t.TempDir()
		var buf bytes.Buffer
		if err := installOpenCodePlugin(dir, false, &buf); err != nil {
			t.Fatalf("install: %v", err)
		}
		buf.Reset()
		if err := uninstallOpenCodePlugin(dir, true, &buf); err != nil {
			t.Fatalf("uninstall dry-run: %v", err)
		}
		if !strings.Contains(buf.String(), "[dry-run]") {
			t.Fatalf("expected [dry-run], got: %s", buf.String())
		}
		if _, err := os.Stat(openCodePluginPath(dir)); err != nil {
			t.Fatal("dry-run must not remove the plugin")
		}
	})
}

// -----------------------------------------------------------------------
// Exported Install/Uninstall dispatch (the os.Stdout wrappers + UninstallTo
// for every target via a redirected home directory).
// -----------------------------------------------------------------------

func TestExportedInstallUninstallWrappers(t *testing.T) {
	// Install / Uninstall (the os.Stdout variants) — gateway target is pure
	// stdout, no file I/O, so it is safe to exercise the real wrappers.
	if err := Install(TargetContinue, false, false); err != nil {
		t.Fatalf("Install(continue): %v", err)
	}
	if err := Uninstall(TargetContinue, false); err != nil {
		t.Fatalf("Uninstall(continue): %v", err)
	}
}

func TestUninstallToAllTargets(t *testing.T) {
	for _, target := range allTargets {
		t.Run(target, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			var buf bytes.Buffer

			// Cline on Windows returns a documented macOS/Linux-only error.
			if target == TargetCline && runtime.GOOS == "windows" {
				if err := UninstallTo(target, false, &buf); err == nil {
					t.Fatal("UninstallTo(cline) on Windows should return an error")
				}
				return
			}

			// Uninstall on a clean home is a no-op (no files installed) and must
			// never error for any target.
			if err := UninstallTo(target, false, &buf); err != nil {
				t.Fatalf("UninstallTo(%s) on clean home: %v", target, err)
			}
		})
	}
}

func TestUninstallUnknownTarget(t *testing.T) {
	var buf bytes.Buffer
	err := UninstallTo("not-a-real-agent", false, &buf)
	if err == nil {
		t.Fatal("expected error for unknown uninstall target")
	}
	if !strings.Contains(err.Error(), "unknown target") {
		t.Fatalf("expected 'unknown target', got: %v", err)
	}
}

func TestUninstallGatewayTargetsNoOp(t *testing.T) {
	for _, target := range []string{TargetContinue, TargetOpenClaw, TargetKilo, TargetTrae} {
		t.Run(target, func(t *testing.T) {
			var buf bytes.Buffer
			if err := UninstallTo(target, false, &buf); err != nil {
				t.Fatalf("UninstallTo(%s): %v", target, err)
			}
			if !strings.Contains(buf.String(), "nothing to uninstall") {
				t.Fatalf("gateway uninstall should say nothing-to-uninstall, got: %s", buf.String())
			}
		})
	}
}

// TestInstallToFileTargetsRoundTrip drives InstallTo + UninstallTo for the
// file-writing targets through the exported dispatch with a redirected home,
// covering the dispatch arms and round-trip on real config paths.
func TestInstallToFileTargetsRoundTrip(t *testing.T) {
	targets := []string{
		TargetClaudeCode, TargetCursor, TargetAugment, TargetCodeBuddy,
		TargetQwen, TargetCopilot, TargetAntigravity, TargetGemini,
		TargetWindsurf, TargetHermes, TargetOpenCode,
	}
	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			var buf bytes.Buffer
			if err := InstallTo(target, false, false, &buf); err != nil {
				t.Fatalf("InstallTo(%s): %v", target, err)
			}
			buf.Reset()
			if err := UninstallTo(target, false, &buf); err != nil {
				t.Fatalf("UninstallTo(%s): %v", target, err)
			}
		})
	}
}

// TestInstallToCodexDispatch exercises the codex install arm through the
// exported InstallTo with a redirected home so config.toml lands in the temp
// dir (codex install also touches ~/.codex/config.toml via os.UserHomeDir).
func TestInstallToCodexDispatch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	var buf bytes.Buffer
	if err := InstallTo(TargetCodex, false, false, &buf); err != nil {
		t.Fatalf("InstallTo(codex): %v", err)
	}
	// config.toml must have been created under the redirected home.
	cfg := codexConfigPath(home)
	if _, err := os.Stat(cfg); err != nil {
		t.Fatalf("codex config.toml not written under redirected home: %v", err)
	}
	buf.Reset()
	if err := UninstallTo(TargetCodex, false, &buf); err != nil {
		t.Fatalf("UninstallTo(codex): %v", err)
	}
}

// -----------------------------------------------------------------------
// Gateway guides — printOpenCodeGuide (0% covered) + dispatch error path.
// -----------------------------------------------------------------------

func TestPrintOpenCodeGuide(t *testing.T) {
	var buf bytes.Buffer
	if err := printOpenCodeGuide(&buf); err != nil {
		t.Fatalf("printOpenCodeGuide: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "127.0.0.1:7837") {
		t.Fatalf("expected gateway URL, got: %s", out)
	}
	if !strings.Contains(out, "beekeeper gateway token") {
		t.Fatalf("expected token command, got: %s", out)
	}
	if !strings.Contains(out, "opencode.json") {
		t.Fatalf("expected opencode.json config reference, got: %s", out)
	}
}

func TestPrintGatewayGuideUnknown(t *testing.T) {
	var buf bytes.Buffer
	err := printGatewayGuide("not-a-gateway", &buf)
	if err == nil {
		t.Fatal("expected error for unknown gateway target")
	}
	if !strings.Contains(err.Error(), "unknown gateway target") {
		t.Fatalf("expected 'unknown gateway target', got: %v", err)
	}
}

// -----------------------------------------------------------------------
// Low-level helpers: printDryRun, backupSettings, writeFileAtomic.
// -----------------------------------------------------------------------

func TestPrintDryRun(t *testing.T) {
	var buf bytes.Buffer
	printDryRun("/some/path", "hooks", map[string]any{"k": "v"}, &buf)
	out := buf.String()
	if !strings.Contains(out, "[dry-run]") {
		t.Fatalf("expected [dry-run] marker, got: %s", out)
	}
	if !strings.Contains(out, "/some/path") {
		t.Fatalf("expected the path, got: %s", out)
	}
	if !strings.Contains(out, "hooks") {
		t.Fatalf("expected the label, got: %s", out)
	}
}

func TestBackupSettings(t *testing.T) {
	t.Run("absent_file_is_noop", func(t *testing.T) {
		dir := t.TempDir()
		if err := backupSettings(filepath.Join(dir, "missing.json")); err != nil {
			t.Fatalf("backupSettings on missing file should be nil, got: %v", err)
		}
	})

	t.Run("creates_0600_backup", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "settings.json")
		if err := os.WriteFile(path, []byte(`{"a":1}`), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		if err := backupSettings(path); err != nil {
			t.Fatalf("backupSettings: %v", err)
		}
		backups, _ := filepath.Glob(path + ".beekeeper-backup-*")
		if len(backups) == 0 {
			t.Fatal("expected a backup file")
		}
		data, _ := os.ReadFile(backups[0])
		if string(data) != `{"a":1}` {
			t.Fatalf("backup content mismatch: %s", data)
		}
		// On non-Windows, assert the 0o600 perm (WR-06). Windows perms differ.
		if runtime.GOOS != "windows" {
			info, _ := os.Stat(backups[0])
			if perm := info.Mode().Perm(); perm != 0o600 {
				t.Fatalf("expected backup perm 0600, got %o", perm)
			}
		}
	})
}

func TestWriteFileAtomic(t *testing.T) {
	t.Run("writes_content", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "out.json")
		if err := writeFileAtomic(path, []byte("hello")); err != nil {
			t.Fatalf("writeFileAtomic: %v", err)
		}
		data, _ := os.ReadFile(path)
		if string(data) != "hello" {
			t.Fatalf("content mismatch: %s", data)
		}
	})

	t.Run("temp_create_fails_on_missing_dir", func(t *testing.T) {
		// CreateTemp in a non-existent directory returns an error (no MkdirAll
		// inside writeFileAtomic itself).
		path := filepath.Join(t.TempDir(), "does", "not", "exist", "out.json")
		if err := writeFileAtomic(path, []byte("x")); err == nil {
			t.Fatal("expected error writing into a non-existent directory")
		}
	})
}

// -----------------------------------------------------------------------
// Install dry-run + malformed-file-tolerance branches for the custom-schema
// installers (cursor/gemini/windsurf) that lacked dry-run subtests.
// -----------------------------------------------------------------------

func TestInstallDryRunCustomSchema(t *testing.T) {
	t.Run("cursor", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "hooks.json")
		var buf bytes.Buffer
		if err := installCursor(path, true, &buf); err != nil {
			t.Fatalf("installCursor dry-run: %v", err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("dry-run must not create the file")
		}
		if !strings.Contains(buf.String(), "[dry-run]") {
			t.Fatalf("expected [dry-run], got: %s", buf.String())
		}
	})
	t.Run("gemini", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "settings.json")
		var buf bytes.Buffer
		if err := installGemini(path, true, &buf); err != nil {
			t.Fatalf("installGemini dry-run: %v", err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("dry-run must not create the file")
		}
		if !strings.Contains(buf.String(), "[dry-run]") {
			t.Fatalf("expected [dry-run], got: %s", buf.String())
		}
	})
	t.Run("windsurf", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "hooks.json")
		var buf bytes.Buffer
		if err := installWindsurf(path, true, &buf); err != nil {
			t.Fatalf("installWindsurf dry-run: %v", err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("dry-run must not create the file")
		}
		if !strings.Contains(buf.String(), "[dry-run]") {
			t.Fatalf("expected [dry-run], got: %s", buf.String())
		}
	})
}

// TestInstallToleratesMalformedExisting drives the "tolerate parse errors —
// start fresh" branch in the installers that read an existing file with a
// permissive json.Unmarshal (cursor/codex/gemini/windsurf). The installer must
// still produce a valid file containing the beekeeper hook.
func TestInstallToleratesMalformedExisting(t *testing.T) {
	t.Run("cursor", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "hooks.json")
		os.WriteFile(path, []byte(`{ this is not valid json `), 0o644)
		var buf bytes.Buffer
		if err := installCursor(path, false, &buf); err != nil {
			t.Fatalf("installCursor over malformed: %v", err)
		}
		var f cursorHooksFile
		data, _ := os.ReadFile(path)
		mustUnmarshal(t, data, &f)
		for _, ev := range cursorEvents {
			if !containsCursorHookByCommand(f.Hooks[ev], cursorCheckSuffix) {
				t.Fatalf("event %q must have beekeeper hook after recovering from malformed file", ev)
			}
		}
	})
	t.Run("codex", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "hooks.json")
		os.WriteFile(path, []byte(`garbage{`), 0o644)
		var buf bytes.Buffer
		if err := installCodex(path, false, &buf); err != nil {
			t.Fatalf("installCodex over malformed: %v", err)
		}
		var f codexHooksFile
		data, _ := os.ReadFile(path)
		mustUnmarshal(t, data, &f)
		if !containsCodexHookByCommand(f.Hooks["PreToolUse"], codexCheckSuffix) {
			t.Fatal("beekeeper hook must be present after recovering from malformed file")
		}
	})
	t.Run("gemini", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "settings.json")
		os.WriteFile(path, []byte(`}}}`), 0o644)
		var buf bytes.Buffer
		if err := installGemini(path, false, &buf); err != nil {
			t.Fatalf("installGemini over malformed: %v", err)
		}
		var f geminiHooksFile
		data, _ := os.ReadFile(path)
		mustUnmarshal(t, data, &f)
		if !containsGeminiHookByCommand(f.Hooks, geminiCheckSuffix) {
			t.Fatal("beekeeper hook must be present after recovering from malformed file")
		}
	})
	t.Run("windsurf", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "hooks.json")
		os.WriteFile(path, []byte(`not json at all`), 0o644)
		var buf bytes.Buffer
		if err := installWindsurf(path, false, &buf); err != nil {
			t.Fatalf("installWindsurf over malformed: %v", err)
		}
		var f windsurfHooksFile
		data, _ := os.ReadFile(path)
		mustUnmarshal(t, data, &f)
		for _, ev := range windsurfEvents {
			if len(f.Hooks[ev]) == 0 {
				t.Fatalf("event %q must have beekeeper hook after recovering from malformed file", ev)
			}
		}
	})
}

// TestClaudeEntriesContainCommandShapes exercises the defensive type-assertion
// branches of claudeEntriesContainCommand against the loosely-typed shapes
// json.Unmarshal can produce (non-map entry, missing inner hooks, non-map inner
// hook, non-string command).
func TestClaudeEntriesContainCommandShapes(t *testing.T) {
	cases := []struct {
		name    string
		entries []any
		cmd     string
		want    bool
	}{
		{"nil", nil, "x", false},
		{"entry_not_map", []any{"a string entry"}, "x", false},
		{"entry_missing_hooks", []any{map[string]any{"matcher": ".*"}}, "x", false},
		{"inner_hooks_not_array", []any{map[string]any{"hooks": "nope"}}, "x", false},
		{"inner_hook_not_map", []any{map[string]any{"hooks": []any{"str"}}}, "x", false},
		{
			"command_not_string",
			[]any{map[string]any{"hooks": []any{map[string]any{"command": 42}}}},
			"x", false,
		},
		{
			// The "found" branch requires a beekeeper-anchored command, since
			// claudeEntriesContainCommand uses matchesBeekeeperCommand (T-w7y-03).
			"match",
			[]any{map[string]any{"hooks": []any{map[string]any{"command": "beekeeper audit-record"}}}},
			"audit-record", true,
		},
		{
			// A non-beekeeper command must NOT match even when the suffix string
			// appears in its args — the anchoring regression guard.
			"non_beekeeper_command_does_not_match",
			[]any{map[string]any{"hooks": []any{map[string]any{"command": "audit-logger audit-record"}}}},
			"audit-record", false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := claudeEntriesContainCommand(tc.entries, tc.cmd); got != tc.want {
				t.Fatalf("claudeEntriesContainCommand(%v) = %v, want %v", tc.entries, got, tc.want)
			}
		})
	}
}

// TestEnsureCodexFeaturesFlagAlreadyCorrect covers the fast-path branch (already
// has [features] hooks=true) which prints a no-change message.
func TestEnsureCodexFeaturesFlagAlreadyCorrect(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	os.WriteFile(path, []byte("[features]\nhooks=true\n"), 0o644)
	var buf bytes.Buffer
	if err := ensureCodexFeaturesFlag(path, &buf); err != nil {
		t.Fatalf("ensureCodexFeaturesFlag: %v", err)
	}
	if !strings.Contains(buf.String(), "no change") {
		t.Fatalf("expected no-change message, got: %s", buf.String())
	}
}

// TestBeekeeperWindsurfHookOSKey asserts the OS-correct key is populated, which
// also covers both arms of beekeeperWindsurfHook across the matrix (the active
// arm runs on the current host; CI exercises the other OSes).
func TestBeekeeperWindsurfHookOSKey(t *testing.T) {
	h := beekeeperWindsurfHook()
	// The installed command embeds the absolute binary path (machine-specific),
	// so assert via the stable suffix rather than exact equality.
	if runtime.GOOS == "windows" {
		if !matchesBeekeeperCommand(h.PowerShell, windsurfCheckSuffix) || h.Command != "" {
			t.Fatalf("windows: want PowerShell matching suffix %q Command empty, got PowerShell=%q Command=%q", windsurfCheckSuffix, h.PowerShell, h.Command)
		}
	} else {
		if !matchesBeekeeperCommand(h.Command, windsurfCheckSuffix) || h.PowerShell != "" {
			t.Fatalf("unix: want Command matching suffix %q PowerShell empty, got Command=%q PowerShell=%q", windsurfCheckSuffix, h.Command, h.PowerShell)
		}
	}
}

// -----------------------------------------------------------------------
// Error-return branches: pass a *directory* path so the underlying ReadFile /
// ReadSettings returns a non-ErrNotExist error, which every installer and
// uninstaller wraps and returns. This deterministically covers the
// "read %q: %w" / "parse %q: %w" error arms across the package.
// -----------------------------------------------------------------------

func TestInstallReadErrorBranches(t *testing.T) {
	type fn func(path string, dryRun bool, out *bytes.Buffer) error
	installers := map[string]fn{
		"claude-code": func(p string, d bool, o *bytes.Buffer) error { return installClaudeCode(p, d, o) },
		"augment":     func(p string, d bool, o *bytes.Buffer) error { return installAugment(p, d, o) },
		"antigravity": func(p string, d bool, o *bytes.Buffer) error { return installAntigravity(p, d, o) },
		"codebuddy":   func(p string, d bool, o *bytes.Buffer) error { return installCodeBuddy(p, d, o) },
		"qwen":        func(p string, d bool, o *bytes.Buffer) error { return installQwen(p, d, o) },
		"copilot":     func(p string, d bool, o *bytes.Buffer) error { return installCopilot(p, d, o) },
		"cursor":      func(p string, d bool, o *bytes.Buffer) error { return installCursor(p, d, o) },
		"codex":       func(p string, d bool, o *bytes.Buffer) error { return installCodex(p, d, o) },
		"gemini":      func(p string, d bool, o *bytes.Buffer) error { return installGemini(p, d, o) },
		"windsurf":    func(p string, d bool, o *bytes.Buffer) error { return installWindsurf(p, d, o) },
		"hermes":      func(p string, d bool, o *bytes.Buffer) error { return installHermes(p, d, o) },
	}
	for name, install := range installers {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir() // a directory path triggers a non-ErrNotExist read error
			var buf bytes.Buffer
			if err := install(dir, false, &buf); err == nil {
				t.Fatalf("%s install on a directory path should error", name)
			}
		})
	}
}

func TestUninstallReadErrorBranches(t *testing.T) {
	type fn func(path string, dryRun bool, out *bytes.Buffer) error
	uninstallers := map[string]fn{
		"claude-code": func(p string, d bool, o *bytes.Buffer) error { return uninstallClaudeCode(p, d, o) },
		"augment":     func(p string, d bool, o *bytes.Buffer) error { return uninstallAugment(p, d, o) },
		"antigravity": func(p string, d bool, o *bytes.Buffer) error { return uninstallAntigravity(p, d, o) },
		"codebuddy":   func(p string, d bool, o *bytes.Buffer) error { return uninstallCodeBuddy(p, d, o) },
		"qwen":        func(p string, d bool, o *bytes.Buffer) error { return uninstallQwen(p, d, o) },
		"copilot":     func(p string, d bool, o *bytes.Buffer) error { return uninstallCopilot(p, d, o) },
		"cursor":      func(p string, d bool, o *bytes.Buffer) error { return uninstallCursor(p, d, o) },
		"codex":       func(p string, d bool, o *bytes.Buffer) error { return uninstallCodex(p, d, o) },
		"gemini":      func(p string, d bool, o *bytes.Buffer) error { return uninstallGemini(p, d, o) },
		"windsurf":    func(p string, d bool, o *bytes.Buffer) error { return uninstallWindsurf(p, d, o) },
		"hermes":      func(p string, d bool, o *bytes.Buffer) error { return uninstallHermes(p, d, o) },
		"opencode":    func(p string, d bool, o *bytes.Buffer) error { return uninstallOpenCodePlugin(p, d, o) },
	}
	for name, uninstall := range uninstallers {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			// For the opencode plugin the path read is openCodePluginPath(dir);
			// make THAT path a directory so its ReadFile errors non-ErrNotExist.
			target := dir
			if name == "opencode" {
				target = dir
				if err := os.MkdirAll(openCodePluginPath(dir), 0o755); err != nil {
					t.Fatalf("mkdir plugin-as-dir: %v", err)
				}
			}
			var buf bytes.Buffer
			if err := uninstall(target, false, &buf); err == nil {
				t.Fatalf("%s uninstall on a directory path should error", name)
			}
		})
	}
}

// TestInstallMkdirErrorBranches covers the os.MkdirAll(...) failure return arm
// in the installers that create the parent directory. A path whose parent
// component is a regular FILE makes MkdirAll fail deterministically (after the
// read step treats the missing child as ErrNotExist and proceeds).
func TestInstallMkdirErrorBranches(t *testing.T) {
	type fn func(path string, dryRun bool, out *bytes.Buffer) error
	installers := map[string]fn{
		"cursor":   func(p string, d bool, o *bytes.Buffer) error { return installCursor(p, d, o) },
		"gemini":   func(p string, d bool, o *bytes.Buffer) error { return installGemini(p, d, o) },
		"windsurf": func(p string, d bool, o *bytes.Buffer) error { return installWindsurf(p, d, o) },
		"hermes":   func(p string, d bool, o *bytes.Buffer) error { return installHermes(p, d, o) },
	}
	for name, install := range installers {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			fileAsParent := filepath.Join(dir, "blocker")
			if err := os.WriteFile(fileAsParent, []byte("x"), 0o644); err != nil {
				t.Fatalf("write blocker file: %v", err)
			}
			// path = <dir>/blocker/sub/target — parent "blocker" is a file, so
			// MkdirAll(filepath.Dir(path)) must fail.
			path := filepath.Join(fileAsParent, "sub", "target.json")
			var buf bytes.Buffer
			if err := install(path, false, &buf); err == nil {
				t.Fatalf("%s install with file-as-parent should error on MkdirAll", name)
			}
		})
	}
}

// TestBackupSettingsReadError covers the non-ErrNotExist read-error branch of
// backupSettings (a directory cannot be read as a file).
func TestBackupSettingsReadError(t *testing.T) {
	dir := t.TempDir()
	if err := backupSettings(dir); err == nil {
		t.Fatal("backupSettings on a directory path should return a read error")
	}
}

// mustUnmarshal is a small JSON helper local to this file.
func mustUnmarshal(t *testing.T, data []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}
