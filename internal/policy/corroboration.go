package policy

import (
	"fmt"
	"sort"
)

// corroborate counts independent signed and unsigned catalog sources from a
// slice of CatalogMatch values and determines the corroboration level.
//
// Logic (PLCY-01):
//   - "Independent" = distinct CatalogSource values; the same source appearing
//     in multiple matches counts as ONE independent source.
//   - Only SIGNED sources contribute to the signed count. Unsigned sources are
//     warn-only (0.5 weight) and can never alone reach the block threshold.
//   - Escalation uses signedCount only:
//     signedCount >= t.QuarantineAt && hasSignedSource → ("block", true)
//     signedCount >= t.BlockAt && hasSignedSource      → ("block", false)
//     signedCount >= t.WarnAt || len(unsignedSet) > 0  → ("warn",  false)
//     else                                             → ("allow", false)
//
// Returns:
//
//	level      — "allow", "warn", or "block"
//	quarantine — true when the quarantine threshold is met
//	count      — number of distinct SIGNED sources (CorroborationCount)
//	agreed     — sorted distinct list of ALL matched source names (signed+unsigned)
//	dissented  — empty slice (reserved for Phase 3+; no dissent model in Phase 2)
// validateCorroborationThresholds checks that WarnAt <= BlockAt <= QuarantineAt.
// Returns a non-nil error if the thresholds are mis-ordered (which would make
// the quarantine or block cases unreachable dead code).
func validateCorroborationThresholds(t CorroborationThresholds) error {
	if t.WarnAt > t.BlockAt {
		return fmt.Errorf("corroboration: WarnAt (%d) must be <= BlockAt (%d)", t.WarnAt, t.BlockAt)
	}
	if t.BlockAt > t.QuarantineAt {
		return fmt.Errorf("corroboration: BlockAt (%d) must be <= QuarantineAt (%d)", t.BlockAt, t.QuarantineAt)
	}
	// CORR-02: validate per-severity overrides.
	for sev, ov := range t.SeverityOverrides {
		if ov.BlockAt < 1 {
			return fmt.Errorf("corroboration: SeverityOverrides[%q].BlockAt (%d) must be >= 1 (zero blocks unconditionally)", sev, ov.BlockAt)
		}
		if ov.BlockAt > t.BlockAt {
			return fmt.Errorf("corroboration: SeverityOverrides[%q].BlockAt (%d) must be <= global BlockAt (%d)", sev, ov.BlockAt, t.BlockAt)
		}
		if ov.QuarantineAt < ov.BlockAt {
			return fmt.Errorf("corroboration: SeverityOverrides[%q].QuarantineAt (%d) must be >= BlockAt (%d)", sev, ov.QuarantineAt, ov.BlockAt)
		}
	}
	return nil
}

// findSeverityOverride returns the most-restrictive SeverityThreshold override
// from overrides that applies to any match, or nil when:
//   - catalogHealthy is false (sanity gate: degraded catalog suppresses escalation)
//   - any non-dissented match has Version == "*" (all-versions guard)
//   - no match severity is in overrides
//
// "Most restrictive" means the override with the lowest BlockAt.
// Pure: reads only matches, overrides map, and the healthy flag — no I/O.
// Imports: only "fmt" and "sort" (existing) — never add "os", "net", etc.
func findSeverityOverride(
	matches []CatalogMatch,
	overrides map[string]SeverityThreshold,
	catalogHealthy bool,
) *SeverityThreshold {
	if !catalogHealthy {
		return nil // CORR-02: sanity gate — no escalation on degraded catalog
	}
	if len(overrides) == 0 {
		return nil
	}

	// CORR-02 all-versions guard: if ANY non-dissented match has Version == "*",
	// do not escalate. A mis-tagged wildcard entry must never block at single-source.
	for _, m := range matches {
		if m.Dissented {
			continue
		}
		if m.Version == "*" {
			return nil
		}
	}

	var best *SeverityThreshold
	for _, m := range matches {
		if m.Dissented {
			continue
		}
		if ov, ok := overrides[m.Severity]; ok {
			if best == nil || ov.BlockAt < best.BlockAt {
				cp := ov // copy to avoid aliasing map value
				best = &cp
			}
		}
	}
	return best
}

func corroborate(matches []CatalogMatch, t CorroborationThresholds) (level string, quarantine bool, count int, agreed, dissented []string) {
	if err := validateCorroborationThresholds(t); err != nil {
		// Misconfigured thresholds — fail closed to block.
		return "block", false, 0, nil, nil
	}
	if len(matches) == 0 {
		return "allow", false, 0, nil, nil
	}

	signedSet := make(map[string]bool)
	unsignedSet := make(map[string]bool)
	allSources := make(map[string]bool)
	// CTLG-09: collect sources that were queried but found no match (dissent).
	dissentSet := make(map[string]bool)

	for _, m := range matches {
		if m.Dissented {
			// Dissent sentinel: source was queried but found no match.
			// Only record as dissenting if the source has NOT also agreed
			// (a source returning both match and no-match is a degenerate case
			// that should not happen in practice; we prioritize agreement).
			dissentSet[m.CatalogSource] = true
			continue
		}
		allSources[m.CatalogSource] = true
		if m.Signed {
			signedSet[m.CatalogSource] = true
		} else {
			unsignedSet[m.CatalogSource] = true
		}
	}

	// A source that also agreed (has real matches) is NOT dissenting.
	for src := range allSources {
		delete(dissentSet, src)
	}

	signedCount := len(signedSet)
	hasSignedSource := signedCount >= 1
	hasUnsigned := len(unsignedSet) > 0

	// Build sorted agreed list from all matched sources (signed and unsigned).
	agreedList := make([]string, 0, len(allSources))
	for src := range allSources {
		agreedList = append(agreedList, src)
	}
	sort.Strings(agreedList)

	// Build sorted dissent list (CTLG-09 forensic provenance).
	dissentList := make([]string, 0, len(dissentSet))
	for src := range dissentSet {
		dissentList = append(dissentList, src)
	}
	sort.Strings(dissentList)

	// CORR-01/02: check for per-severity threshold override.
	// findSeverityOverride returns nil when: catalog is degraded (CatalogHealthy=false),
	// any non-dissented match is an all-versions wildcard (Version=="*"), or no severity
	// matches SeverityOverrides keys.
	effectiveBlockAt := t.BlockAt
	effectiveQuarantineAt := t.QuarantineAt
	if ov := findSeverityOverride(matches, t.SeverityOverrides, t.CatalogHealthy); ov != nil {
		effectiveBlockAt = ov.BlockAt
		effectiveQuarantineAt = ov.QuarantineAt
	}

	// Escalation decision table (PLCY-01 + CORR-01 severity override).
	switch {
	case signedCount >= effectiveQuarantineAt && hasSignedSource:
		return "block", true, signedCount, agreedList, dissentList
	case signedCount >= effectiveBlockAt && hasSignedSource:
		return "block", false, signedCount, agreedList, dissentList
	case signedCount >= t.WarnAt || hasUnsigned:
		return "warn", false, signedCount, agreedList, dissentList
	default:
		return "allow", false, signedCount, agreedList, dissentList
	}
}
