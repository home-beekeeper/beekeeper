package policy

import (
	"go/parser"
	"go/token"
	"math"
	"os"
	"testing"
)

func TestShannonEntropyAllSameBytes(t *testing.T) {
	// "aaaa" — 4 identical bytes — all probability mass on one symbol → H = 0
	h := shannonEntropy("aaaa")
	if h != 0.0 {
		t.Errorf("shannonEntropy(%q) = %v, want 0", "aaaa", h)
	}
}

func TestShannonEntropyFourEquiprobableSymbols(t *testing.T) {
	// "abcd" — 4 distinct bytes each appearing once → H = log2(4) = 2.0
	h := shannonEntropy("abcd")
	if math.Abs(h-2.0) > 1e-9 {
		t.Errorf("shannonEntropy(%q) = %v, want 2.0", "abcd", h)
	}
}

func TestShannonEntropyEmptyString(t *testing.T) {
	// Empty input → 0 (no division by zero)
	h := shannonEntropy("")
	if h != 0.0 {
		t.Errorf("shannonEntropy(%q) = %v, want 0", "", h)
	}
}

func TestExfilLowEntropyAllows(t *testing.T) {
	cfg := DefaultExfilConfig()
	window := ExfilWindow{
		Outputs:     []string{"hello world this is normal english text"},
		Base64Bytes: 0,
	}
	d := EvaluateExfil(window, cfg)
	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q (low-entropy text should allow)", d.Level, "allow")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true")
	}
}

func TestExfilHighEntropyWarns(t *testing.T) {
	cfg := DefaultExfilConfig()
	// A string with near-maximum entropy (random-looking bytes represented as ASCII range)
	// Using a string with all 256 byte values appearing roughly equally → H ~ 8
	// We'll use a simpler approach: a long string of varied chars
	// "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ!@#$%^&*()" has ~6.1 bits
	highEntropyOutput := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ!@#$%^&*()"
	// Verify this is actually above threshold
	h := shannonEntropy(highEntropyOutput)
	if h < cfg.EntropyThreshold {
		t.Skipf("test string entropy %v is not above threshold %v — adjust test string", h, cfg.EntropyThreshold)
	}
	window := ExfilWindow{
		Outputs:     []string{highEntropyOutput},
		Base64Bytes: 0,
	}
	d := EvaluateExfil(window, cfg)
	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q (high-entropy output should warn)", d.Level, "warn")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true (warn does not block)")
	}
}

func TestExfilBase64AccumulationWarns(t *testing.T) {
	cfg := DefaultExfilConfig()
	window := ExfilWindow{
		Outputs:     []string{"some normal text"},
		Base64Bytes: 2 << 20, // 2MB > 1MB default threshold
	}
	d := EvaluateExfil(window, cfg)
	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q (base64 accumulation over threshold should warn)", d.Level, "warn")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true (warn does not block)")
	}
}

func TestExfilBothBelowThresholdsAllows(t *testing.T) {
	cfg := DefaultExfilConfig()
	window := ExfilWindow{
		Outputs:     []string{"normal text output"},
		Base64Bytes: 100, // well below 1MB threshold
	}
	d := EvaluateExfil(window, cfg)
	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q (both below thresholds → allow)", d.Level, "allow")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true")
	}
}

func TestExfilBase64AtExactThresholdWarns(t *testing.T) {
	cfg := DefaultExfilConfig()
	window := ExfilWindow{
		Outputs:     []string{"normal text"},
		Base64Bytes: cfg.Base64Threshold, // exactly at threshold → warn
	}
	d := EvaluateExfil(window, cfg)
	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q (at threshold should warn)", d.Level, "warn")
	}
}

func TestExfilEmptyWindowAllows(t *testing.T) {
	cfg := DefaultExfilConfig()
	window := ExfilWindow{
		Outputs:     []string{},
		Base64Bytes: 0,
	}
	d := EvaluateExfil(window, cfg)
	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q (empty window allows)", d.Level, "allow")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true")
	}
}

func TestExfilImportsArePure(t *testing.T) {
	const filePath = "exfil.go"
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
			t.Errorf("exfil.go imports forbidden package %q — violates pure-library contract", path)
		}
	}
}
