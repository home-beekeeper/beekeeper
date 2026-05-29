package policyloader

import "github.com/mzansi-agentive/beekeeper/internal/policy"

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
	}

	return t
}

// RunPolicyTest dry-runs the policy file's rules against tc and returns the
// Decision from policy.Evaluate. It uses an empty MultiCatalogLookup (no live
// catalog) so results reflect only the policy file's threshold overrides — not
// live catalog state. This makes test output deterministic and prevents false
// "allow" results caused by catalog absence rather than the policy file.
//
// Built-in additive policies remain active: corroboration thresholds derived
// from any corroboration_threshold rule in pf override the PLCY-01 defaults,
// but the underlying engine's lifecycle/path/egress checks are always active.
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
func runPolicyTestWithCatalog(pf PolicyFile, tc policy.ToolCall, idx policy.MultiCatalogLookup, ac policy.AgentContext) policy.Decision {
	thresholds := thresholdsFromPolicyFile(pf)
	return policy.Evaluate(tc, idx, thresholds, ac)
}
