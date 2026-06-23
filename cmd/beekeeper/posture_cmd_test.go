package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/home-beekeeper/beekeeper/internal/posture"
)

// TestPostureCmd_Output_NpmAndPnpm covers npm AND pnpm together: it injects a
// synthetic PMState (npm with three gaps, hardened pnpm aligned) via the
// detection seam, runs `beekeeper posture`, and asserts both managers appear with
// their gaps and that the canonical boundary text is present (IPBND-01).
func TestPostureCmd_Output_NpmAndPnpm(t *testing.T) {
	restoreDetect := postureDetectFn
	restoreWeak := postureWeaknessFn
	t.Cleanup(func() {
		postureDetectFn = restoreDetect
		postureWeaknessFn = restoreWeak
	})

	postureDetectFn = func(_ context.Context, _ posture.Config) posture.PMState {
		return posture.PMState{
			NpmInstalled:  true,
			NpmVersion:    "11.0.0",
			PnpmInstalled: true,
			PnpmVersion:   "11.1.0",
			PnpmHardened:  true,
		}
	}
	postureWeaknessFn = func() string { return "" }

	cmd := newPostureCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("posture RunE error: %v", err)
	}
	output := out.String()

	// npm appears with its three gaps.
	if !strings.Contains(output, "npm v11.0.0 detected") {
		t.Errorf("output missing npm detection line:\n%s", output)
	}
	if !strings.Contains(output, "Covering 3 gaps your npm version does not") {
		t.Errorf("output missing npm gap count:\n%s", output)
	}
	for _, gap := range []string{"scripts warned", "release-age 24h", "git deps flagged"} {
		if !strings.Contains(output, gap) {
			t.Errorf("output missing gap %q:\n%s", gap, output)
		}
	}

	// pnpm appears and is aligned.
	if !strings.Contains(output, "pnpm v11.1.0 detected") {
		t.Errorf("output missing pnpm detection line:\n%s", output)
	}
	if !strings.Contains(output, "aligned, no gap") {
		t.Errorf("output missing pnpm aligned line:\n%s", output)
	}

	// The canonical boundary statement must be present (IPBND-01).
	if !strings.Contains(output, posture.BoundaryShort) {
		t.Errorf("output missing BoundaryShort:\n%s", output)
	}

	// Style rule: no em dash anywhere in the output.
	if strings.ContainsRune(output, '—') {
		t.Errorf("posture output must not contain an em dash:\n%s", output)
	}
}

// TestPostureCmd_FullBoundary verifies --full prints the long boundary statement.
func TestPostureCmd_FullBoundary(t *testing.T) {
	restoreDetect := postureDetectFn
	restoreWeak := postureWeaknessFn
	t.Cleanup(func() {
		postureDetectFn = restoreDetect
		postureWeaknessFn = restoreWeak
	})
	postureDetectFn = func(_ context.Context, _ posture.Config) posture.PMState {
		return posture.PMState{NpmInstalled: true, NpmVersion: "11.0.0"}
	}
	postureWeaknessFn = func() string { return "" }

	cmd := newPostureCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Flags().Set("full", "true"); err != nil {
		t.Fatalf("set --full: %v", err)
	}
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("posture RunE error: %v", err)
	}
	if !strings.Contains(out.String(), posture.BoundaryStatement) {
		t.Errorf("--full output missing full BoundaryStatement:\n%s", out.String())
	}
}

// TestPostureCmd_ReadOnlyGuarantee is the LOAD-BEARING Layer-2 self-defense test
// (IPVIEW-02). It places fixture package-manager config files (.npmrc,
// pnpm-workspace.yaml, bunfig.toml) in a temp working directory, runs the REAL
// detection path (the default postureDetectFn, which reads pnpm-workspace.yaml
// and bunfig.toml) plus the REAL pnpm weakness reader, renders the view, and
// asserts every fixture file is byte-for-byte UNCHANGED afterwards. The view must
// never write any package-manager config.
func TestPostureCmd_ReadOnlyGuarantee(t *testing.T) {
	dir := t.TempDir()

	fixtures := map[string]string{
		".npmrc":              "registry=https://registry.npmjs.org/\nsave-exact=true\n",
		"pnpm-workspace.yaml": "packages:\n  - 'packages/*'\nminimumReleaseAge: 60\nblockExoticSubdeps: false\n",
		"bunfig.toml":         "[install.security]\nscanner = \"@socketsecurity/bun-security-scanner\"\n",
	}
	hashesBefore := make(map[string][]byte)
	for name, content := range fixtures {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatalf("write fixture %q: %v", name, err)
		}
		hashesBefore[name] = hashFile(t, p)
	}

	// chdir into the fixture dir so the real detection reads these files (it reads
	// pnpm-workspace.yaml and bunfig.toml from the cwd).
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Run the command with the REAL detection + weakness reader (no seam override),
	// so the read-only file-scanning path is exercised against the fixtures.
	cmd := newPostureCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("posture RunE error: %v", err)
	}

	// Assert every fixture file is byte-for-byte unchanged AND that no extra file
	// was created in the directory.
	for name := range fixtures {
		p := filepath.Join(dir, name)
		after := hashFile(t, p)
		if !bytes.Equal(hashesBefore[name], after) {
			t.Errorf("fixture %q was modified by `beekeeper posture` (read-only violation)", name)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != len(fixtures) {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("posture created or removed files in the working dir; want %d, got %d: %v",
			len(fixtures), len(entries), names)
	}
}

func hashFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return sum[:]
}
