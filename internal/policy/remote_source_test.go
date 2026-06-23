package policy

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestRemoteSourceRegistryInstallAllowed: Kind "" (normal registry install) → allow.
func TestRemoteSourceRegistryInstallAllowed(t *testing.T) {
	d := EvaluateRemoteSource(RemoteSourceInput{Ecosystem: "npm", Package: "left-pad", Kind: ""}, DefaultRemoteSourceConfig())
	if !d.Allow || d.Level != "allow" {
		t.Errorf("registry install: Allow=%v Level=%q, want allow/allow", d.Allow, d.Level)
	}
}

// TestRemoteSourceKindsWarn: each non-empty kind warns (Allow stays true — warn
// does not block per the PRD default posture) and names the kind in the reason.
func TestRemoteSourceKindsWarn(t *testing.T) {
	for _, kind := range []string{"git", "github", "url", "tarball", "file"} {
		d := EvaluateRemoteSource(RemoteSourceInput{Ecosystem: "npm", Package: "evil-spec", Kind: kind}, DefaultRemoteSourceConfig())
		if d.Level != "warn" {
			t.Errorf("kind %q: Level = %q, want warn", kind, d.Level)
		}
		if !d.Allow {
			t.Errorf("kind %q: Allow = false, want true (warn must not block per default posture)", kind)
		}
		if !strings.Contains(d.Reason, kind) {
			t.Errorf("kind %q: Reason %q should name the source kind", kind, d.Reason)
		}
		if len(d.RuleIDs) == 0 || d.RuleIDs[0] != ruleRemoteSource {
			t.Errorf("kind %q: RuleIDs = %v, want [%s]", kind, d.RuleIDs, ruleRemoteSource)
		}
	}
}

// TestRemoteSourceAllowlistExempt: an allowlisted spec is allowed despite a kind.
func TestRemoteSourceAllowlistExempt(t *testing.T) {
	cfg := RemoteSourceConfig{Exclude: []string{"git+https://github.com/our-org/internal-tool.git"}}
	d := EvaluateRemoteSource(RemoteSourceInput{
		Ecosystem: "npm",
		Package:   "git+https://github.com/our-org/internal-tool.git",
		Kind:      "git",
	}, cfg)
	if !d.Allow || d.Level != "allow" {
		t.Errorf("allowlisted git spec: Allow=%v Level=%q, want allow/allow", d.Allow, d.Level)
	}
	if !strings.Contains(d.Reason, "allowlisted") {
		t.Errorf("Reason %q should mention allowlist", d.Reason)
	}
}

// TestRemoteSourceBareSpecReason: an empty Package still yields a readable reason.
func TestRemoteSourceBareSpecReason(t *testing.T) {
	d := EvaluateRemoteSource(RemoteSourceInput{Ecosystem: "npm", Package: "", Kind: "git"}, DefaultRemoteSourceConfig())
	if d.Level != "warn" {
		t.Errorf("Level = %q, want warn", d.Level)
	}
	if strings.Contains(d.Reason, "()") {
		t.Errorf("Reason %q should not contain an empty () for a bare spec", d.Reason)
	}
}

// TestRemoteSourceImportsArePure enforces the pure-library contract on remote_source.go.
func TestRemoteSourceImportsArePure(t *testing.T) {
	const srcPath = "remote_source.go"
	src, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("reading %s: %v", srcPath, err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, srcPath, src, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parsing %s: %v", srcPath, err)
	}
	forbidden := map[string]bool{
		"os": true, "net": true, "net/http": true, "io": true,
		"sync": true, "time": true, "context": true,
	}
	for _, imp := range f.Imports {
		path := imp.Path.Value
		if len(path) >= 2 {
			path = path[1 : len(path)-1]
		}
		if forbidden[path] {
			t.Errorf("remote_source.go imports forbidden package %q — violates pure-library contract", path)
		}
	}
}
