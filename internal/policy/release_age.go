package policy

import "fmt"

// ruleReleaseAge is the rule ID for the release-age policy (PLCY-02).
const ruleReleaseAge = "release-age-policy"

// ReleaseAgeInput carries the caller-resolved publish timestamp for a package.
// The I/O adapter (internal/catalog/age_cache.go) fetches and caches it; the
// policy function receives only the resolved duration (pure, no I/O).
//
// The adapter computes AgeMinutes as int64(now.Sub(publishedAt).Minutes()) and
// sets TimestampMissing=true when the registry returned no data. This way,
// EvaluateReleaseAge is a pure function: no time.Now(), no HTTP, no I/O.
type ReleaseAgeInput struct {
	Ecosystem        string
	Package          string
	AgeMinutes       int64 // time.Since(publishedAt).Minutes() — computed by caller
	TimestampMissing bool  // true when registry returned no data (fail closed)
}

// ReleaseAgeConfig holds per-ecosystem thresholds and an allowlist.
//
//   - DefaultMinutes: the minimum age required across all ecosystems unless
//     overridden by PerEcosystemMinutes. Default 1440 (24h).
//   - PerEcosystemMinutes: per-ecosystem override keyed by ecosystem name.
//     If present for the input ecosystem, overrides DefaultMinutes.
//   - Exclude: packages whose names are in this list are exempt from the age
//     check (always allowed regardless of age or missing timestamp).
type ReleaseAgeConfig struct {
	DefaultMinutes      int64
	PerEcosystemMinutes map[string]int64
	Exclude             []string
}

// DefaultReleaseAgeConfig returns the PLCY-02 defaults: 1440 minutes (24h)
// across all ecosystems, matching pnpm v11's behavior. No per-ecosystem
// overrides; no excludes.
func DefaultReleaseAgeConfig() ReleaseAgeConfig {
	return ReleaseAgeConfig{
		DefaultMinutes:      1440,
		PerEcosystemMinutes: nil,
		Exclude:             nil,
	}
}

// EvaluateReleaseAge is a pure function: given caller-resolved age data and
// per-ecosystem thresholds, it returns a Decision without any I/O, goroutines,
// globals mutation, or wall-clock access.
//
// Decision logic (PLCY-02):
//  1. If input.Package is in cfg.Exclude → allow ("release-age allowlisted").
//  2. If input.TimestampMissing → block, fail-closed.
//  3. Resolve threshold: cfg.PerEcosystemMinutes[ecosystem] if present, else
//     cfg.DefaultMinutes.
//  4. If input.AgeMinutes < threshold → block.
//  5. Otherwise → allow.
//
// EvaluateReleaseAge is pure: imports only "fmt" and "strings" (no time, net,
// os, io, sync, context).
func EvaluateReleaseAge(input ReleaseAgeInput, cfg ReleaseAgeConfig) Decision {
	// 1. Allowlist check — takes priority over everything including missing timestamp.
	for _, excluded := range cfg.Exclude {
		if excluded == input.Package {
			return Decision{
				Allow:   true,
				Level:   "allow",
				Reason:  "release-age allowlisted",
				RuleIDs: []string{ruleReleaseAge},
			}
		}
	}

	// 2. Fail closed: missing publish timestamp.
	if input.TimestampMissing {
		return Decision{
			Allow:   false,
			Level:   "block",
			Reason:  "publish timestamp unavailable (fail-closed)",
			RuleIDs: []string{ruleReleaseAge},
		}
	}

	// 3. Resolve threshold: per-ecosystem override or global default.
	threshold := cfg.DefaultMinutes
	if cfg.PerEcosystemMinutes != nil {
		if override, ok := cfg.PerEcosystemMinutes[input.Ecosystem]; ok {
			threshold = override
		}
	}

	// 4. Age check.
	if input.AgeMinutes < threshold {
		return Decision{
			Allow:  false,
			Level:  "block",
			Reason: fmt.Sprintf("package age %dm below minimum %dm", input.AgeMinutes, threshold),
			RuleIDs: []string{ruleReleaseAge},
		}
	}

	// 5. Package is old enough.
	return Decision{
		Allow:   true,
		Level:   "allow",
		Reason:  fmt.Sprintf("package age %dm meets minimum %dm", input.AgeMinutes, threshold),
		RuleIDs: []string{ruleReleaseAge},
	}
}

