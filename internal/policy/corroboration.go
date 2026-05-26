package policy

import "sort"

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
func corroborate(matches []CatalogMatch, t CorroborationThresholds) (level string, quarantine bool, count int, agreed, dissented []string) {
	if len(matches) == 0 {
		return "allow", false, 0, nil, nil
	}

	signedSet := make(map[string]bool)
	unsignedSet := make(map[string]bool)
	allSources := make(map[string]bool)

	for _, m := range matches {
		allSources[m.CatalogSource] = true
		if m.Signed {
			signedSet[m.CatalogSource] = true
		} else {
			unsignedSet[m.CatalogSource] = true
		}
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

	// Escalation decision table (PLCY-01).
	switch {
	case signedCount >= t.QuarantineAt && hasSignedSource:
		return "block", true, signedCount, agreedList, nil
	case signedCount >= t.BlockAt && hasSignedSource:
		return "block", false, signedCount, agreedList, nil
	case signedCount >= t.WarnAt || hasUnsigned:
		return "warn", false, signedCount, agreedList, nil
	default:
		return "allow", false, signedCount, agreedList, nil
	}
}
