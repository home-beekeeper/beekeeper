package check

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/home-beekeeper/beekeeper/internal/policy"
)

// TestSelfProtectSymlinkedParentNonExistentLeaf covers finding #3: a Write to a
// not-yet-existing file under a directory that is a SYMLINK into the StateDir.
// filepath.EvalSymlinks on the full path errors on the missing leaf and the
// resolver falls back to the lexical (symlink) path, hiding the real StateDir —
// so the self-protection prefix match misses. With the parent-resolved form the
// new leaf still resolves under the StateDir and is blocked.
func TestSelfProtectSymlinkedParentNonExistentLeaf(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("BEEKEEPER_HOME", tmp)
	stateDir := filepath.Join(tmp, "beekeeper")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}

	// Plant a symlink that points INTO the StateDir, living OUTSIDE it so its
	// lexical path carries no StateDir prefix.
	linkDir := filepath.Join(tmp, "outside", "link-to-state")
	if err := os.MkdirAll(filepath.Dir(linkDir), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(stateDir, linkDir); err != nil {
		// Windows without the SeCreateSymbolicLink privilege (Developer Mode off)
		// cannot create symlinks — skip cleanly; CI Linux/macOS exercise this.
		if os.IsPermission(err) || strings.Contains(strings.ToLower(err.Error()), "privilege") {
			t.Skipf("os.Symlink requires privilege on this host, skipping: %v", err)
		}
		t.Fatal(err)
	}

	// The leaf does NOT exist yet — this is a Write creating a NEW file under the
	// symlinked-in directory. Its lexical path is outside/link-to-state/evil.json,
	// which carries no StateDir prefix; only parent-symlink resolution exposes the
	// real StateDir destination.
	target := filepath.Join(linkDir, "evil.json")
	if _, err := os.Stat(target); err == nil {
		t.Fatal("test precondition: leaf must not exist yet")
	}

	run := func(tool string, kv map[string]string) Result {
		return runCheckWithIndex(context.Background(), strings.NewReader(toolCallJSON(tool, kv)), closedConfig(), emptySelfIdx(), auditPathIn(t))
	}

	res := run("Write", map[string]string{"file_path": target})
	if res.Decision.Allow {
		t.Errorf("write to a new file under a StateDir-symlinked directory must BLOCK (finding #3); reason=%q", res.Decision.Reason)
	}
}

// TestCanonicalizePathFormsParentSymlinkResolves is the unit-level proof that
// canonicalizePathForms surfaces a form whose PARENT symlink is resolved, even
// when the leaf does not exist (finding #3).
func TestCanonicalizePathFormsParentSymlinkResolves(t *testing.T) {
	realDir := t.TempDir()
	link := filepath.Join(t.TempDir(), "lnk")
	if err := os.Symlink(realDir, link); err != nil {
		if os.IsPermission(err) || strings.Contains(strings.ToLower(err.Error()), "privilege") {
			t.Skipf("os.Symlink requires privilege on this host, skipping: %v", err)
		}
		t.Fatal(err)
	}

	// realDir resolves to its EvalSymlinks form (macOS /var -> /private/var etc.).
	resolvedReal, err := filepath.EvalSymlinks(realDir)
	if err != nil {
		t.Fatalf("EvalSymlinks(realDir): %v", err)
	}
	wantPrefix := filepath.ToSlash(resolvedReal)

	forms := canonicalizePathForms(filepath.Join(link, "newleaf.json"))
	found := false
	for _, f := range forms {
		if strings.HasPrefix(f, wantPrefix) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a form rooted at the resolved real dir %q for a non-existent leaf under a symlink; forms=%v", wantPrefix, forms)
	}
}

func emptySelfIdx() catalogIndex {
	return &mapMultiIndex{matchesByKey: map[string][]policy.CatalogMatch{}}
}

// toolCallJSON builds a minimal tool-call stdin body with a JSON-safe path.
func toolCallJSON(tool string, kv map[string]string) string {
	var b strings.Builder
	b.WriteString(`{"agent_name":"a","tool_name":` + strconv.Quote(tool) + `,"tool_input":{`)
	first := true
	for k, v := range kv {
		if !first {
			b.WriteString(",")
		}
		first = false
		b.WriteString(strconv.Quote(k) + ":" + strconv.Quote(v))
	}
	b.WriteString(`}}`)
	return b.String()
}

func TestBuildSelfProtectConfigResolvesStateDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("BEEKEEPER_HOME", tmp)
	stateDir := filepath.Join(tmp, "beekeeper")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}

	cfg := buildSelfProtectConfig()
	if len(cfg.ReadWritePrefixes) == 0 {
		t.Fatal("expected a state-dir read/write prefix")
	}
	if len(cfg.WriteOnlyPrefixes) == 0 {
		t.Error("expected a binary write-only prefix")
	}

	// A read of the state-dir config is blocked (treated as secret).
	target := canonicalizePath(filepath.Join(stateDir, "config.json"))
	if d := policy.EvaluateSelfPath(target, false, cfg); d.Allow {
		t.Errorf("state-dir config read should block; prefixes=%v target=%q", cfg.ReadWritePrefixes, target)
	}
}

// TestSelfProtectE2E drives the full check mirror (runCheckWithIndex) end-to-end.
func TestSelfProtectE2E(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("BEEKEEPER_HOME", tmp)
	stateDir := filepath.Join(tmp, "beekeeper")
	if err := os.MkdirAll(filepath.Join(stateDir, "policies"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(stateDir, "config.json")
	polPath := filepath.Join(stateDir, "policies", "evil.json")
	devFile := filepath.Join(tmp, "repo", "internal", "main.go") // NOT under state dir

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	run := func(tool string, kv map[string]string) Result {
		return runCheckWithIndex(context.Background(), strings.NewReader(toolCallJSON(tool, kv)), closedConfig(), emptySelfIdx(), auditPathIn(t))
	}

	tests := []struct {
		name      string
		tool      string
		kv        map[string]string
		wantBlock bool
	}{
		{"read config blocked", "Read", map[string]string{"file_path": cfgPath}, true},
		{"write policy blocked", "Write", map[string]string{"file_path": polPath}, true},
		{"bash redirect to config blocked", "Bash", map[string]string{"command": "echo pwned > " + cfgPath}, true},
		{"bash cat config blocked", "Bash", map[string]string{"command": "cat " + cfgPath}, true},
		{"bash $VAR path to config blocked", "Bash", map[string]string{"command": "cat $BEEKEEPER_HOME/beekeeper/config.json"}, true},
		{"bash ${VAR} path to config blocked", "Bash", map[string]string{"command": "cat ${BEEKEEPER_HOME}/beekeeper/config.json"}, true},
		{"write binary blocked", "Write", map[string]string{"file_path": exe}, true},
		{"bash overwrite binary blocked", "Bash", map[string]string{"command": "cp evil " + exe}, true},
		{"read binary allowed", "Read", map[string]string{"file_path": exe}, false},
		{"dev repo file allowed", "Write", map[string]string{"file_path": devFile}, false},
		{"unrelated read allowed", "Read", map[string]string{"file_path": filepath.Join(tmp, "notes.txt")}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := run(tt.tool, tt.kv)
			if tt.wantBlock && res.Decision.Allow {
				t.Errorf("%s: expected BLOCK, got allow (reason %q)", tt.name, res.Decision.Reason)
			}
			if !tt.wantBlock && !res.Decision.Allow {
				t.Errorf("%s: expected ALLOW, got block (reason %q)", tt.name, res.Decision.Reason)
			}
		})
	}
}
