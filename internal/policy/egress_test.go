package policy

import (
	"go/parser"
	"go/token"
	"os"
	"testing"
)

func TestEgressDefaultAllowRegistryHost(t *testing.T) {
	cfg := DefaultEgressConfig()
	input := EgressInput{
		ToolName:    "WebFetch",
		TargetURL:   "https://registry.npmjs.org/express",
		PayloadSize: 1024,
	}
	d := EvaluateEgress(input, cfg)
	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q (npm registry is default-allow)", d.Level, "allow")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true")
	}
}

func TestEgressPasteSiteBlocked(t *testing.T) {
	cfg := DefaultEgressConfig()
	input := EgressInput{
		ToolName:    "WebFetch",
		TargetURL:   "https://pastebin.com/raw/abc",
		PayloadSize: 1024,
	}
	d := EvaluateEgress(input, cfg)
	if d.Level != "block" {
		t.Errorf("Level = %q, want %q (pastebin is default-deny)", d.Level, "block")
	}
	if d.Allow {
		t.Errorf("Allow = true, want false (paste site must block)")
	}
}

func TestEgressWebhookSiteBlocked(t *testing.T) {
	cfg := DefaultEgressConfig()
	input := EgressInput{
		ToolName:    "WebFetch",
		TargetURL:   "https://webhook.site/xyz",
		PayloadSize: 1024,
	}
	d := EvaluateEgress(input, cfg)
	if d.Level != "block" {
		t.Errorf("Level = %q, want %q (webhook.site is default-deny)", d.Level, "block")
	}
	if d.Allow {
		t.Errorf("Allow = true, want false")
	}
}

func TestEgressOversizedPayloadBlocked(t *testing.T) {
	cfg := DefaultEgressConfig()
	input := EgressInput{
		ToolName:    "WebFetch",
		TargetURL:   "https://registry.npmjs.org/express",
		PayloadSize: 11 << 20, // 11MB > 10MB default limit
	}
	d := EvaluateEgress(input, cfg)
	if d.Level != "block" {
		t.Errorf("Level = %q, want %q (oversized payload must block)", d.Level, "block")
	}
	if d.Allow {
		t.Errorf("Allow = true, want false")
	}
}

func TestEgressAllowedHostWithinSizeLimit(t *testing.T) {
	cfg := DefaultEgressConfig()
	input := EgressInput{
		ToolName:    "WebFetch",
		TargetURL:   "https://pypi.org/simple/requests/",
		PayloadSize: 5 << 20, // 5MB — within 10MB limit
	}
	d := EvaluateEgress(input, cfg)
	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q", d.Level, "allow")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true")
	}
}

func TestEgressUnknownHostWarns(t *testing.T) {
	cfg := DefaultEgressConfig()
	input := EgressInput{
		ToolName:    "WebFetch",
		TargetURL:   "https://some-random-site.example.com/data",
		PayloadSize: 100,
	}
	d := EvaluateEgress(input, cfg)
	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q (unknown host must warn, not silently allow)", d.Level, "warn")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true (warn is non-blocking)")
	}
}

func TestEgressPerToolOverride(t *testing.T) {
	cfg := DefaultEgressConfig()
	// Add a per-tool size override for "LargeUpload" tool
	cfg.PerToolMaxBytes = map[string]int64{
		"LargeUpload": 50 << 20, // 50MB for this specific tool
	}
	// A host that would warn with default config (unknown host), but size is within per-tool limit
	input := EgressInput{
		ToolName:    "LargeUpload",
		TargetURL:   "https://some-random-site.example.com/upload",
		PayloadSize: 40 << 20, // 40MB — over default 10MB but under per-tool 50MB
	}
	d := EvaluateEgress(input, cfg)
	// Should warn (unknown host) but NOT block on size
	if d.Level == "block" {
		t.Errorf("Level = %q, want warn — per-tool override should raise the size limit", d.Level)
	}
}

func TestEgressSubdomainOfDenyHostBlocked(t *testing.T) {
	cfg := DefaultEgressConfig()
	// hastebin.com is in default deny list; sub.hastebin.com must also be denied (suffix match)
	input := EgressInput{
		ToolName:    "WebFetch",
		TargetURL:   "https://sub.hastebin.com/raw/xyz",
		PayloadSize: 100,
	}
	d := EvaluateEgress(input, cfg)
	if d.Level != "block" {
		t.Errorf("Level = %q, want %q (subdomain of deny host must block via suffix match)", d.Level, "block")
	}
}

// TestEgressLookalikeAllowHostNotMatched proves the suffix-boundary fix: an
// attacker-registrable lookalike of an allow host must NOT inherit its allow.
// "evilpypi.org" must not match allow entry "pypi.org" — it should fall through
// to the unknown-host warn.
func TestEgressLookalikeAllowHostNotMatched(t *testing.T) {
	cfg := DefaultEgressConfig()
	input := EgressInput{
		ToolName:    "WebFetch",
		TargetURL:   "https://evilpypi.org/upload",
		PayloadSize: 100,
	}
	d := EvaluateEgress(input, cfg)
	if d.Level == "allow" {
		t.Errorf("Level = %q, want warn — evilpypi.org must NOT match allow pypi.org (label boundary)", d.Level)
	}
	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q (unknown lookalike host warns)", d.Level, "warn")
	}
}

// TestEgressRealSubdomainOfAllowHostMatched proves a genuine subdomain of an
// allow host still matches under the label-boundary rule.
func TestEgressRealSubdomainOfAllowHostMatched(t *testing.T) {
	cfg := DefaultEgressConfig()
	input := EgressInput{
		ToolName:    "WebFetch",
		TargetURL:   "https://sub.pypi.org/simple/requests/",
		PayloadSize: 100,
	}
	d := EvaluateEgress(input, cfg)
	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q (sub.pypi.org is a real subdomain of allow pypi.org)", d.Level, "allow")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true")
	}
}

// TestEgressLookalikeDenyHostNotMatched proves a lookalike of a deny host does
// NOT get blocked by the deny entry (avoids both over-block on unrelated hosts
// and, more importantly, confirms the boundary logic is symmetric). "notpastebin.com"
// must not match deny "pastebin.com" — it should warn as an unknown host.
func TestEgressLookalikeDenyHostNotMatched(t *testing.T) {
	cfg := DefaultEgressConfig()
	input := EgressInput{
		ToolName:    "WebFetch",
		TargetURL:   "https://notpastebin.com/raw/abc",
		PayloadSize: 100,
	}
	d := EvaluateEgress(input, cfg)
	if d.Level == "block" {
		t.Errorf("Level = %q, want warn — notpastebin.com must NOT match deny pastebin.com (label boundary)", d.Level)
	}
	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q (unrelated host warns)", d.Level, "warn")
	}
}

// TestEgressRealSubdomainOfDenyHostBlocked confirms a real subdomain of a deny
// host is still blocked (complements the existing sub.hastebin.com test for the
// new boundary helper).
func TestEgressRealSubdomainOfDenyHostBlocked(t *testing.T) {
	cfg := DefaultEgressConfig()
	input := EgressInput{
		ToolName:    "WebFetch",
		TargetURL:   "https://raw.pastebin.com/abc",
		PayloadSize: 100,
	}
	d := EvaluateEgress(input, cfg)
	if d.Level != "block" {
		t.Errorf("Level = %q, want %q (raw.pastebin.com is a real subdomain of deny pastebin.com)", d.Level, "block")
	}
}

// TestEgressExactDenyHostBlocked confirms the exact deny host still blocks under
// the boundary helper (no regression).
func TestEgressExactDenyHostBlocked(t *testing.T) {
	cfg := DefaultEgressConfig()
	input := EgressInput{
		ToolName:    "WebFetch",
		TargetURL:   "https://pastebin.com",
		PayloadSize: 100,
	}
	d := EvaluateEgress(input, cfg)
	if d.Level != "block" {
		t.Errorf("Level = %q, want %q (exact deny host)", d.Level, "block")
	}
}

// TestEgressHostFallbackDoesNotCollapseDenyToWarn covers Fix 2: a deny target
// wrapped to defeat url.Parse (so host extraction is ambiguous/fails) must still
// BLOCK via the raw-URL deny check, not collapse to a non-blocking warn.
func TestEgressHostFallbackDoesNotCollapseDenyToWarn(t *testing.T) {
	cfg := DefaultEgressConfig()
	tests := []struct {
		name string
		url  string
	}{
		// Backslash before userinfo "@" — many URL parsers diverge here, and
		// url.Parse may not surface "pastebin.com" as the Host.
		{"backslash userinfo wrap", `https://pastebin.com\@evil.example.com/raw/x`},
		// Scheme-relative URL: url.Parse("//pastebin.com/raw/x") yields Host set,
		// but the bare "pastebin.com\@x" form below exercises the fallback path.
		{"scheme-relative deny host", "//pastebin.com/raw/x"},
		{"bare deny host with backslash userinfo", `pastebin.com\@x`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := EgressInput{
				ToolName:    "WebFetch",
				TargetURL:   tc.url,
				PayloadSize: 100,
			}
			d := EvaluateEgress(input, cfg)
			if d.Level != "block" {
				t.Errorf("Level = %q, want %q — wrapped deny target must still block (no block→warn collapse)", d.Level, "block")
			}
			if d.Allow {
				t.Errorf("Allow = true, want false for wrapped deny target %q", tc.url)
			}
		})
	}
}

// TestEgressRawFallbackDoesNotOverBlockLookalike confirms the raw-URL fallback
// itself respects label boundaries: a lookalike that defeats url.Parse must not
// be blocked just because the deny entry appears as a substring.
func TestEgressRawFallbackRespectsBoundary(t *testing.T) {
	cfg := DefaultEgressConfig()
	// "notpastebin.com" as a raw, hard-to-parse string must not block.
	input := EgressInput{
		ToolName:    "WebFetch",
		TargetURL:   `notpastebin.com\@x`,
		PayloadSize: 100,
	}
	d := EvaluateEgress(input, cfg)
	if d.Level == "block" {
		t.Errorf("Level = %q, want non-block — notpastebin.com must not match deny pastebin.com even on the raw fallback", d.Level)
	}
}

func TestEgressImportsArePure(t *testing.T) {
	const filePath = "egress.go"
	src, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("reading %s: %v", filePath, err)
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, src, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parsing %s: %v", filePath, err)
	}

	forbidden := map[string]bool{
		"os":       true,
		"net":      true,
		"net/http": true,
		"io":       true,
		"sync":     true,
		"time":     true,
		"context":  true,
	}

	for _, imp := range f.Imports {
		path := imp.Path.Value
		if len(path) >= 2 {
			path = path[1 : len(path)-1]
		}
		if forbidden[path] {
			t.Errorf("egress.go imports forbidden package %q — violates pure-library contract", path)
		}
	}
}
