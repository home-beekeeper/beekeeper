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

// EvaluateExfil detects potential multi-turn exfiltration via Shannon entropy
// spikes and base64 accumulation across tool outputs. Pure function; no I/O,
// no wall clock, no goroutines.
//
// Decision order:
//  1. If Base64Bytes >= cfg.Base64Threshold → warn (base64 accumulation)
//  2. Else if entropy of concatenated outputs >= cfg.EntropyThreshold → warn (high entropy)
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

	// 2. Check Shannon entropy over concatenated outputs.
	concatenated := strings.Join(window.Outputs, "")
	if len(concatenated) > 0 {
		entropy := shannonEntropy(concatenated)
		if entropy >= cfg.EntropyThreshold {
			return Decision{
				Allow:   true,
				Level:   "warn",
				Reason:  fmt.Sprintf("high-entropy output window: %.4f", entropy),
				RuleIDs: []string{ruleExfiltration},
			}
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
