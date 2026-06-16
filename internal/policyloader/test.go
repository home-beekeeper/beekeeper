package policyloader

import "github.com/home-beekeeper/beekeeper/internal/policy"

// emptyLookup is a no-op MultiCatalogLookup that always returns nil. It is used
// as the default catalog for RunPolicyTest so that dry-run results reflect only
// the policy file's own rules, not live catalog state (Pitfall 4: an empty
// catalog makes results deterministic and avoids the misleading "allow because
// not in catalog" confusion when the intent is to test policy file rules).
//
// Built-in policies (corroboration thresholds, allowlists) derived from the
// policy file remain active; only the catalog-lookup path returns empty.
type emptyLookup struct{}

func (emptyLookup) LookupAll(_, _ string) []policy.CatalogMatch { return nil }

// thresholdsFromPolicyFile starts from policy.DefaultCorroborationThresholds()
// (the PLCY-01 defaults: warn=1, block=2, quarantine=3) and applies any
// corroboration_threshold rule's WarnAt/BlockAt/QuarantineAt overrides.
//
// Only non-zero values in a rule override the corresponding threshold. This
// allows a policy file to tighten (lower) or relax (raise) the defaults while
// leaving unspecified fields at the default. If multiple corroboration_threshold
// rules exist, later rules override earlier ones for each field.
//
// This is the bridge between the declarative policy file and the pure
// policy.CorroborationThresholds the engine accepts.
func thresholdsFromPolicyFile(pf PolicyFile) policy.CorroborationThresholds {
	t := policy.DefaultCorroborationThresholds()

	for _, r := range pf.Rules {
		if r.RuleType != "corroboration_threshold" {
			continue
		}
		// Only override if the rule specifies a non-zero value, so that a rule
		// that only sets BlockAt does not accidentally zero out WarnAt.
		if r.WarnAt > 0 {
			t.WarnAt = r.WarnAt
		}
		if r.BlockAt > 0 {
			t.BlockAt = r.BlockAt
		}
		if r.QuarantineAt > 0 {
			t.QuarantineAt = r.QuarantineAt
		}
		// CORR-01: non-zero CriticalBlockAt merges into SeverityOverrides["critical"].
		// Zero means "use default" (mirrors the WarnAt/BlockAt/QuarantineAt pattern).
		if r.CriticalBlockAt > 0 {
			if t.SeverityOverrides == nil {
				t.SeverityOverrides = make(map[string]policy.SeverityThreshold)
			}
			existing := t.SeverityOverrides["critical"]
			existing.BlockAt = r.CriticalBlockAt
			if existing.QuarantineAt <= existing.BlockAt {
				existing.QuarantineAt = existing.BlockAt + 1 // CR-02: quarantine strictly above block (handles zero + raised-block collapse)
			}
			t.SeverityOverrides["critical"] = existing
		}
	}

	return t
}

// ThresholdsFromPolicyFiles merges corroboration_threshold rules from all
// loaded policy files into a single policy.CorroborationThresholds value,
// starting from the defaults. Later files (and rules within files) override
// earlier ones for each field (last-writer-wins per field).
//
// This exported helper bridges the declarative policy files and the pure
// policy.CorroborationThresholds accepted by policy.Evaluate, for use by
// all callers that need live-threshold behavior (check/handler, gateway,
// watch, scan). When no files set a threshold field, the PLCY-01 default
// for that field is preserved.
func ThresholdsFromPolicyFiles(files []PolicyFile) policy.CorroborationThresholds {
	t := policy.DefaultCorroborationThresholds()
	for _, pf := range files {
		for _, r := range pf.Rules {
			if r.RuleType != "corroboration_threshold" {
				continue
			}
			if r.WarnAt > 0 {
				t.WarnAt = r.WarnAt
			}
			if r.BlockAt > 0 {
				t.BlockAt = r.BlockAt
			}
			if r.QuarantineAt > 0 {
				t.QuarantineAt = r.QuarantineAt
			}
			// CORR-01: non-zero CriticalBlockAt merges into SeverityOverrides["critical"].
			// Zero means "use default" (mirrors the WarnAt/BlockAt/QuarantineAt pattern).
			if r.CriticalBlockAt > 0 {
				if t.SeverityOverrides == nil {
					t.SeverityOverrides = make(map[string]policy.SeverityThreshold)
				}
				existing := t.SeverityOverrides["critical"]
				existing.BlockAt = r.CriticalBlockAt
				if existing.QuarantineAt <= existing.BlockAt {
					existing.QuarantineAt = existing.BlockAt + 1 // CR-02: quarantine strictly above block (handles zero + raised-block collapse)
				}
				t.SeverityOverrides["critical"] = existing
			}
		}
	}
	return t
}

// RunPolicyTest dry-runs the policy file's rules against tc and returns the
// Decision that reflects the full overlay enforcement (corroboration thresholds
// + package_allowlist + sensitive_path rules). It uses an empty
// MultiCatalogLookup (no live catalog) so results reflect only the policy
// file's own rules — not live catalog state. This makes test output
// deterministic and prevents false "allow" results caused by catalog absence.
//
// The overlay (ApplyPolicyOverlay) is applied on top of the engine result so
// that `beekeeper policy test` reflects the same enforcement that live
// `beekeeper check` applies (CODE-01 requirement).
//
// Use --with-catalogs (CLI flag, wired in cmd/beekeeper) to test against live
// catalogs when you want to verify real-world catalog interaction.
func RunPolicyTest(pf PolicyFile, tc policy.ToolCall, ac policy.AgentContext) policy.Decision {
	return runPolicyTestWithCatalog(pf, tc, emptyLookup{}, ac)
}

// runPolicyTestWithCatalog is the internal implementation of RunPolicyTest that
// accepts an injectable MultiCatalogLookup. It is unexported so that tests can
// inject a fake catalog (to verify threshold overrides produce block decisions)
// while the public API always uses emptyLookup (Pitfall 4).
//
// The overlay is applied after the engine result so that package_allowlist and
// sensitive_path rules in the policy file are reflected in the dry-run output
// (CODE-01: policy test must mirror live check enforcement).
func runPolicyTestWithCatalog(pf PolicyFile, tc policy.ToolCall, idx policy.MultiCatalogLookup, ac policy.AgentContext) policy.Decision {
	// Use ThresholdsFromPolicyFiles (the exported helper) to derive thresholds
	// from the single policy file so that dry-run and live-check are consistent
	// (both call the same logic path). This closes INT-BLOCK-2 live/dry parity.
	thresholds := ThresholdsFromPolicyFiles([]PolicyFile{pf})
	base := policy.Evaluate(tc, idx, thresholds, ac)
	// Apply the overlay for package_allowlist and sensitive_path rules so that
	// `policy test` output matches what live `beekeeper check` would produce.
	return ApplyPolicyOverlay([]PolicyFile{pf}, tc, base)
}
