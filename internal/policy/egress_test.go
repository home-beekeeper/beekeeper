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
