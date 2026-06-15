package policy

import (
	"go/parser"
	"go/token"
	"math"
	"os"
	"strings"
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

// highEntropyBlob returns a ~256-byte string whose Shannon entropy is above the
// default 4.5 bits/byte threshold (a base64-alphabet-like blob standing in for a
// leaked secret/key).
func highEntropyBlob() string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var b strings.Builder
	// 4 full passes of the 64-char alphabet → 256 bytes, ~6 bits/byte.
	for pass := 0; pass < 4; pass++ {
		b.WriteString(alphabet)
	}
	return b.String()
}

// TestExfilPaddingDilutionStillWarns is the core padding-dilution test (Fix 3).
// A short high-entropy secret is sandwiched between large low-entropy fillers.
// Over the single CONCATENATION the average entropy is pulled below threshold,
// but the windowed/per-output maximum keeps the secret detectable, so the
// decision must remain warn (NOT downgraded to allow by the padding).
func TestExfilPaddingDilutionStillWarns(t *testing.T) {
	cfg := DefaultExfilConfig()

	secret := highEntropyBlob() // ~256 bytes, > threshold on its own
	// Low-entropy filler: a long run of a single byte dilutes the concatenated
	// entropy toward ~0.
	filler := strings.Repeat("a", 8000)

	// Sanity: the diluted concatenation is below threshold, proving the OLD
	// (concatenation-only) logic would have allowed this.
	concat := filler + secret + filler
	if h := shannonEntropy(concat); h >= cfg.EntropyThreshold {
		t.Fatalf("test setup invalid: concatenated entropy %v is already >= threshold %v; increase filler", h, cfg.EntropyThreshold)
	}

	window := ExfilWindow{
		Outputs:     []string{filler, secret, filler},
		Base64Bytes: 0,
	}
	d := EvaluateExfil(window, cfg)
	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q — high-entropy secret must not be masked by low-entropy padding", d.Level, "warn")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true (warn is by design non-blocking)")
	}
}

// TestExfilPaddingDilutionInterleavedSingleOutput proves the windowed scan also
// catches a secret embedded WITHIN a single padded output (not split across
// outputs), where per-output entropy alone is diluted but a sliding window over
// the secret region is not.
func TestExfilPaddingDilutionInterleavedSingleOutput(t *testing.T) {
	cfg := DefaultExfilConfig()
	secret := highEntropyBlob()
	filler := strings.Repeat("x", 8000)
	single := filler + secret + filler

	if h := shannonEntropy(single); h >= cfg.EntropyThreshold {
		t.Fatalf("test setup invalid: single-output entropy %v already >= threshold %v", h, cfg.EntropyThreshold)
	}

	window := ExfilWindow{
		Outputs:     []string{single},
		Base64Bytes: 0,
	}
	d := EvaluateExfil(window, cfg)
	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q — windowed scan must catch an embedded high-entropy secret", d.Level, "warn")
	}
}

// TestExfilLowEntropyLargeWindowStillAllows guards against over-warning: a large
// purely low-entropy window must still allow (the windowed max stays below
// threshold). This proves the sliding window did not introduce false positives.
func TestExfilLowEntropyLargeWindowStillAllows(t *testing.T) {
	cfg := DefaultExfilConfig()
	window := ExfilWindow{
		Outputs:     []string{strings.Repeat("hello world ", 2000)},
		Base64Bytes: 0,
	}
	d := EvaluateExfil(window, cfg)
	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q (large low-entropy text must not warn)", d.Level, "allow")
	}
}

// TestExfilNoPanicOnAdversarialInput exercises adversarial inputs (NUL bytes,
// very long single byte, empty strings interleaved, full-byte-range content) to
// confirm the windowed entropy scan never panics or hangs (no ReDoS — there is
// no regexp; the scan is linear).
func TestExfilNoPanicOnAdversarialInput(t *testing.T) {
	cfg := DefaultExfilConfig()
	inputs := [][]string{
		{string([]byte{0, 0, 0, 0, 0})},
		{strings.Repeat("\x00", 100000)},
		{"", "", ""},
		{strings.Repeat(string([]byte{0xff, 0x00, 0x7f}), 50000)},
		{string(makeAllBytes())},
	}
	for i, outs := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("input %d panicked: %v", i, r)
				}
			}()
			d := EvaluateExfil(ExfilWindow{Outputs: outs}, cfg)
			// We don't assert on level here — only that it returns without panic.
			if d.RuleIDs == nil && d.Level == "" {
				t.Errorf("input %d returned an empty decision", i)
			}
		}()
	}
}

// makeAllBytes returns a slice containing every byte value 0..255 once.
func makeAllBytes() []byte {
	b := make([]byte, 256)
	for i := range b {
		b[i] = byte(i)
	}
	return b
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
