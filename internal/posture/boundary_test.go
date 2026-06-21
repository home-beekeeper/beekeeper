package posture

import (
	"strings"
	"testing"
)

// TestBoundaryStatementHonestyContent guards the canonical enforcement-boundary
// statement (IPBND-01) against silent drift: it must stay non-empty, em-dash-free,
// and name all four surfaces (hook, Sentry, gateway, shim-roadmap) so the honesty
// content described to the maintainer at Gate 1 cannot be quietly weakened.
func TestBoundaryStatementHonestyContent(t *testing.T) {
	if strings.TrimSpace(BoundaryStatement) == "" {
		t.Fatal("BoundaryStatement is empty")
	}

	// No em dashes (em-dash U+2014, en-dash U+2013). The honesty standard mandates
	// plain ASCII dashes so the copy renders identically across product, help text,
	// and docs. The two string literals below are the forbidden characters we scan
	// for; they are the one intentional exception to the no-em-dash rule.
	for _, bad := range []string{"—", "–"} {
		if strings.Contains(BoundaryStatement, bad) {
			t.Errorf("BoundaryStatement contains an em/en dash %q; use a plain ASCII dash", bad)
		}
		if strings.Contains(BoundaryShort, bad) {
			t.Errorf("BoundaryShort contains an em/en dash %q; use a plain ASCII dash", bad)
		}
	}

	// Name the four surfaces. Matching is case-insensitive on substrings that are
	// unlikely to appear incidentally.
	lower := strings.ToLower(BoundaryStatement)
	for _, surface := range []struct{ name, needle string }{
		{"hook (pre-exec enforcement)", "hook"},
		{"Sentry (observe/audit)", "sentry"},
		{"MCP gateway (not a general install surface)", "gateway"},
		{"shim (experimental/roadmap)", "shim"},
	} {
		if !strings.Contains(lower, surface.needle) {
			t.Errorf("BoundaryStatement does not mention the %s surface (missing %q)", surface.name, surface.needle)
		}
	}

	// The roadmap framing for the shim must be explicit (experimental / roadmap),
	// not implied, this is the specific honesty point the PRD divergence calls out.
	if !strings.Contains(lower, "roadmap") && !strings.Contains(lower, "experimental") {
		t.Error("BoundaryStatement must frame the shim as experimental/roadmap")
	}

	// The short form must also be non-empty and mention the hook + Sentry split.
	shortLower := strings.ToLower(BoundaryShort)
	if !strings.Contains(shortLower, "hook") || !strings.Contains(shortLower, "sentry") {
		t.Errorf("BoundaryShort must name the hook/Sentry split; got %q", BoundaryShort)
	}
}
