package policy

import (
	"fmt"
	"math"
	"strings"
)

// ruleExfiltration is the rule ID for multi-turn exfiltration detection decisions.
const ruleExfiltration = "multi-turn-exfiltration"

// ExfilWindow carries the rolling window of recent tool outputs (pre-collected
// by the caller from the baseline store; pure inputs only).
type ExfilWindow struct {
	Outputs     []string // last N tool outputs
	Base64Bytes int64    // accumulated base64-encoded byte count across outputs
}

// ExfilConfig holds the thresholds for exfiltration detection.
type ExfilConfig struct {
	EntropyThreshold float64 // bits per byte; default 4.5
	Base64Threshold  int64   // bytes of accumulated base64 before warning; default 1MB
}

// DefaultExfilConfig returns the default exfiltration detection configuration.
// EntropyThreshold of 4.5 bits/byte is the standard security literature value for
// detecting compressed, encrypted, or encoded data.
func DefaultExfilConfig() ExfilConfig {
	return ExfilConfig{
		EntropyThreshold: 4.5,
		Base64Threshold:  1 << 20, // 1MB
	}
}

// entropyWindowSize is the byte width of the sliding window used to detect a
// localized high-entropy run (e.g. an embedded secret/key) inside an otherwise
// low-entropy stream. It is large enough to give a stable Shannon estimate yet
// small enough that low-entropy filler padded around a short secret cannot
// dilute the secret's own entropy below the threshold. 256 bytes ≈ a typical
// base64/hex key blob.
const entropyWindowSize = 256

// EvaluateExfil detects potential multi-turn exfiltration via Shannon entropy
// spikes and base64 accumulation across tool outputs. Pure function; no I/O,
// no wall clock, no goroutines.
//
// Decision order:
//  1. If Base64Bytes >= cfg.Base64Threshold → warn (base64 accumulation)
//  2. Else if the MAX windowed entropy across outputs >= cfg.EntropyThreshold →
//     warn (high entropy). Entropy is taken over a sliding window AND per-output,
//     never over the single diluted concatenation, so low-entropy padding
//     interleaved around a high-entropy secret cannot mask it (padding-dilution).
//  3. Otherwise → allow
//
// Warn keeps Allow true — it is a signal for the caller to act, not a block.
func EvaluateExfil(window ExfilWindow, cfg ExfilConfig) Decision {
	// 1. Check base64 accumulation first.
	if window.Base64Bytes >= cfg.Base64Threshold {
		return Decision{
			Allow:   true,
			Level:   "warn",
			Reason:  fmt.Sprintf("base64 accumulation across turns: %d bytes", window.Base64Bytes),
			RuleIDs: []string{ruleExfiltration},
		}
	}

	// 2. Check Shannon entropy using the MAX over per-output and sliding-window
	// measurements rather than the single concatenation. A concatenated entropy
	// is dilutable: an attacker can sandwich a short high-entropy secret between
	// large low-entropy fillers and pull the average below the threshold. Taking
	// the maximum localized entropy defeats that — the secret's own window stays
	// above threshold regardless of surrounding padding.
	if entropy := maxWindowedEntropy(window.Outputs); entropy >= cfg.EntropyThreshold {
		return Decision{
			Allow:   true,
			Level:   "warn",
			Reason:  fmt.Sprintf("high-entropy output window: %.4f", entropy),
			RuleIDs: []string{ruleExfiltration},
		}
	}

	// 3. Both below thresholds → allow.
	return Decision{
		Allow:   true,
		Level:   "allow",
		Reason:  "",
		RuleIDs: []string{ruleExfiltration},
	}
}

// maxWindowedEntropy returns the maximum Shannon entropy (bits/byte) observed
// over the outputs, measured both per-output and across a fixed-size sliding
// window (entropyWindowSize, step = half the window) that can straddle output
// boundaries via the concatenation. This is the padding-dilution defense: the
// single concatenated entropy is replaced by the maximum localized entropy so a
// high-entropy secret cannot be hidden by surrounding low-entropy filler.
//
// Pure and bounded: it scans the concatenation once per window step (O(n) total,
// each window is O(entropyWindowSize)) with no recursion, no regexp, and no
// unbounded allocation, so it cannot ReDoS or panic on adversarial input. An
// empty/whitespace-only input yields 0.
func maxWindowedEntropy(outputs []string) float64 {
	var maxH float64

	// Per-output entropy: a single short high-entropy output is caught even if
	// the rest of the window is benign.
	for _, out := range outputs {
		if h := shannonEntropy(out); h > maxH {
			maxH = h
		}
	}

	// Sliding window over the concatenation. The window can straddle boundaries
	// so a secret split across two outputs is still measured locally.
	concatenated := strings.Join(outputs, "")
	n := len(concatenated)
	if n == 0 {
		return maxH
	}
	if n <= entropyWindowSize {
		if h := shannonEntropy(concatenated); h > maxH {
			maxH = h
		}
		return maxH
	}

	// Step by half the window so every byte is covered by at least one window
	// without quadratic blow-up. step >= 1 is guaranteed (entropyWindowSize > 1).
	step := entropyWindowSize / 2
	for start := 0; start < n; start += step {
		end := start + entropyWindowSize
		if end > n {
			end = n
		}
		if h := shannonEntropy(concatenated[start:end]); h > maxH {
			maxH = h
		}
		if end == n {
			break
		}
	}
	return maxH
}

// shannonEntropy computes the Shannon entropy (bits per symbol) of a string.
// It counts byte frequencies across the full string and applies H = -sum(p * log2(p)).
// Returns 0 for an empty string (no division by zero).
// Uses only math.Log2; no I/O, no wall clock.
func shannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	var freq [256]int
	for i := 0; i < len(s); i++ {
		freq[s[i]]++
	}
	total := float64(len(s))
	var h float64
	for _, c := range freq {
		if c > 0 {
			p := float64(c) / total
			h -= p * math.Log2(p)
		}
	}
	return h
}
