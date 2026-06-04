package nudge

import (
	"testing"

	"github.com/bantuson/beekeeper/internal/pkgparse"
)

// TestRewriteToPnpm verifies the pnpm rewrite verb mapping.
func TestRewriteToPnpm(t *testing.T) {
	tests := []struct {
		cmd  string
		want string
	}{
		{"npm install foo", "pnpm add foo"},
		{"npm install foo@5.4.0", "pnpm add foo@5.4.0"},
		{"npm install", "pnpm install"},      // no-arg form (NUDGE-03)
		{"npm i foo", "pnpm add foo"},
		{"npm add foo", "pnpm add foo"},
		{"npx create-app", "pnpm dlx create-app"}, // §10-9
		{"npx create-app@latest", "pnpm dlx create-app@latest"},
		// Scoped package with version
		{"npm install @scope/pkg@1.0.0", "pnpm add @scope/pkg@1.0.0"},
	}
	for _, tc := range tests {
		parsed, ok := pkgparse.Parse(tc.cmd)
		if !ok {
			t.Fatalf("pkgparse.Parse(%q) returned ok=false", tc.cmd)
		}
		got := rewriteToPnpm(parsed)
		if got != tc.want {
			t.Errorf("rewriteToPnpm(Parse(%q)) = %q, want %q", tc.cmd, got, tc.want)
		}
	}
}

// TestRewriteToBun verifies the bun rewrite verb mapping.
func TestRewriteToBun(t *testing.T) {
	tests := []struct {
		cmd  string
		want string
	}{
		{"npm install foo", "bun add foo"},
		{"npm install foo@5.4.0", "bun add foo@5.4.0"},
		{"npm install", "bun install"},          // no-arg form
		{"npx create-app", "bun x create-app"}, // §10-9 bun form
		{"npx create-app@latest", "bun x create-app@latest"},
		// Scoped package
		{"npm install @scope/pkg@2.0.0", "bun add @scope/pkg@2.0.0"},
	}
	for _, tc := range tests {
		parsed, ok := pkgparse.Parse(tc.cmd)
		if !ok {
			t.Fatalf("pkgparse.Parse(%q) returned ok=false", tc.cmd)
		}
		got := rewriteToBun(parsed)
		if got != tc.want {
			t.Errorf("rewriteToBun(Parse(%q)) = %q, want %q", tc.cmd, got, tc.want)
		}
	}
}

// TestRewriteNoSudoPrefix verifies the rewrite functions never emit a sudo prefix.
// The Sudo guard in Evaluate ensures cmd.Sudo=true never reaches rewriteToPnpm/rewriteToBun,
// but as a defensive test we verify the output of a parsed sudo command has no "sudo".
func TestRewriteNoSudoPrefix(t *testing.T) {
	parsed, ok := pkgparse.Parse("sudo npm install foo")
	if !ok {
		t.Fatal("pkgparse.Parse(sudo npm install foo) returned ok=false")
	}
	if !parsed.Sudo {
		t.Fatal("expected Sudo=true for 'sudo npm install foo'")
	}
	// Even if called directly (shouldn't be — Evaluate guards this), output must not contain "sudo".
	pnpmResult := rewriteToPnpm(parsed)
	if len(pnpmResult) >= 5 && pnpmResult[:5] == "sudo " {
		t.Errorf("rewriteToPnpm emitted sudo prefix: %q", pnpmResult)
	}
	bunResult := rewriteToBun(parsed)
	if len(bunResult) >= 5 && bunResult[:5] == "sudo " {
		t.Errorf("rewriteToBun emitted sudo prefix: %q", bunResult)
	}
}
