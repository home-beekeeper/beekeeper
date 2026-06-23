//go:build !windows

package hooks

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// -----------------------------------------------------------------------
// TestInstallCline — contract-shape tests for the Cline installer
// (macOS/Linux only; build-tagged !windows)
// -----------------------------------------------------------------------

func TestInstallCline(t *testing.T) {
	t.Run("from_absent_creates_executable", func(t *testing.T) {
		dir := t.TempDir()
		hooksDir := filepath.Join(dir, "Hooks")

		var buf bytes.Buffer
		if err := installCline(hooksDir, false, &buf); err != nil {
			t.Fatalf("installCline: %v", err)
		}

		hookPath := clinePreToolUsePath(hooksDir)
		info, err := os.Stat(hookPath)
		if err != nil {
			t.Fatalf("PreToolUse not created: %v", err)
		}

		// Must be executable (mode 0o755 or at least 0o111 exec bits set).
		mode := info.Mode()
		if mode&0o111 == 0 {
			t.Fatalf("PreToolUse must be executable, got mode %o", mode)
		}

		data, _ := os.ReadFile(hookPath)
		content := string(data)
		if !strings.Contains(content, clineCheckSuffix) {
			t.Fatalf("PreToolUse must contain %q, got:\n%s", clineCheckSuffix, content)
		}
		if !strings.HasPrefix(content, "#!/bin/sh") {
			t.Fatalf("PreToolUse must start with #!/bin/sh shebang, got:\n%s", content)
		}
	})

	t.Run("idempotent", func(t *testing.T) {
		dir := t.TempDir()
		hooksDir := filepath.Join(dir, "Hooks")

		var buf bytes.Buffer
		if err := installCline(hooksDir, false, &buf); err != nil {
			t.Fatalf("installCline (1st): %v", err)
		}

		// Read the file after first install.
		hookPath := clinePreToolUsePath(hooksDir)
		data1, _ := os.ReadFile(hookPath)

		// Install again — must be a no-op.
		buf.Reset()
		if err := installCline(hooksDir, false, &buf); err != nil {
			t.Fatalf("installCline (2nd): %v", err)
		}

		out := buf.String()
		if !strings.Contains(out, "no change") {
			t.Fatalf("expected 'no change' on idempotent install, got: %s", out)
		}

		// Content must be unchanged.
		data2, _ := os.ReadFile(hookPath)
		if string(data1) != string(data2) {
			t.Fatal("idempotent install must not modify the file")
		}
	})

	t.Run("uninstall_removes_beekeeper_script", func(t *testing.T) {
		dir := t.TempDir()
		hooksDir := filepath.Join(dir, "Hooks")

		var buf bytes.Buffer
		if err := installCline(hooksDir, false, &buf); err != nil {
			t.Fatalf("installCline: %v", err)
		}

		// Uninstall.
		buf.Reset()
		if err := uninstallCline(hooksDir, false, &buf); err != nil {
			t.Fatalf("uninstallCline: %v", err)
		}

		hookPath := clinePreToolUsePath(hooksDir)
		if _, err := os.Stat(hookPath); !os.IsNotExist(err) {
			t.Fatal("PreToolUse should be removed after uninstall")
		}
	})

	t.Run("foreign_script_preserved", func(t *testing.T) {
		dir := t.TempDir()
		hooksDir := filepath.Join(dir, "Hooks")
		if err := os.MkdirAll(hooksDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		// Seed with a foreign PreToolUse script.
		hookPath := clinePreToolUsePath(hooksDir)
		foreignContent := "#!/bin/sh\nmy-other-guard\n"
		if err := os.WriteFile(hookPath, []byte(foreignContent), 0o755); err != nil {
			t.Fatalf("write foreign script: %v", err)
		}

		var buf bytes.Buffer
		if err := installCline(hooksDir, false, &buf); err != nil {
			t.Fatalf("installCline with foreign script: %v", err)
		}

		// The output must warn about the foreign script being backed up.
		out := buf.String()
		if !strings.Contains(out, "WARNING") && !strings.Contains(out, "acked up") {
			t.Fatalf("expected warning about backed-up foreign script, got: %s", out)
		}

		// A backup of the foreign script must exist.
		backups := globFiles(t, dir, "Hooks/PreToolUse.beekeeper-backup-*")
		if len(backups) == 0 {
			t.Fatal("expected backup of foreign PreToolUse script")
		}

		// The installed script must now be the beekeeper script.
		data, _ := os.ReadFile(hookPath)
		if !strings.Contains(string(data), clineCheckSuffix) {
			t.Fatal("PreToolUse must be the beekeeper script after install")
		}
	})

	t.Run("uninstall_preserves_foreign_script", func(t *testing.T) {
		dir := t.TempDir()
		hooksDir := filepath.Join(dir, "Hooks")
		if err := os.MkdirAll(hooksDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		// Seed with a foreign PreToolUse script (not our command).
		hookPath := clinePreToolUsePath(hooksDir)
		foreignContent := "#!/bin/sh\nmy-other-guard\n"
		if err := os.WriteFile(hookPath, []byte(foreignContent), 0o755); err != nil {
			t.Fatalf("write foreign script: %v", err)
		}

		var buf bytes.Buffer
		if err := uninstallCline(hooksDir, false, &buf); err != nil {
			t.Fatalf("uninstallCline: %v", err)
		}

		// File must still exist (foreign, not removed).
		if _, err := os.Stat(hookPath); os.IsNotExist(err) {
			t.Fatal("foreign PreToolUse script must not be removed by uninstall")
		}

		out := buf.String()
		if !strings.Contains(out, "foreign") && !strings.Contains(out, "not a beekeeper") {
			t.Fatalf("expected foreign script preservation message, got: %s", out)
		}
	})

	t.Run("dry_run_no_write", func(t *testing.T) {
		dir := t.TempDir()
		hooksDir := filepath.Join(dir, "Hooks")

		var buf bytes.Buffer
		if err := installCline(hooksDir, true, &buf); err != nil {
			t.Fatalf("installCline dry-run: %v", err)
		}

		// File must NOT have been created.
		hookPath := clinePreToolUsePath(hooksDir)
		if _, err := os.Stat(hookPath); !os.IsNotExist(err) {
			t.Fatal("dry-run must not create PreToolUse")
		}

		if !strings.Contains(buf.String(), "dry-run") {
			t.Fatalf("dry-run output should mention dry-run, got: %s", buf.String())
		}
	})
}
