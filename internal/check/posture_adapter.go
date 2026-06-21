// Package check - impure install-posture adapter (PRD Layer 1, IPST-01/02/03).
//
// This file wires the three install-posture rules into the pre-exec hook, where
// the package-manager nudge used to sit (removed in v1.1.0). It is the impure
// adapter - it does the registry I/O and applies the configured posture action -
// while the rules themselves stay pure in internal/policy (EvaluateReleaseAge,
// EvaluateLifecycle, EvaluateRemoteSource). This mirrors paths.go's role for the
// sensitive-path feature and the old nudge_adapter.go's role for the nudge.
//
// -- Enforcement boundary (IPBND-01) --------------------------------------------
// Posture is enforced PRE-EXEC at the agent hook for hooked (Tier-1) harnesses
// only, inheriting each harness tier's caveats. Installs run OUTSIDE a hooked
// tool call are OBSERVED and AUDITED by the Sentry layer, not prevented here. The
// MCP gateway is not a general install surface, and the package-manager shim that
// would extend pre-exec enforcement to every install is experimental/roadmap, not
// a headline guarantee. The single source of truth for this statement is
// posture.BoundaryStatement (internal/posture/boundary.go) - see the handler
// wiring comment, which references that constant rather than re-typing it.
//
// -- Default action = WARN, opt-up to block per rule (IPOVR-03) -----------------
// The PRD default posture WARNS; it does not block. The pure EvaluateReleaseAge
// and EvaluateLifecycle return `block` on a violation - correct for the scan/watch
// EXTENSION supply-chain path (internal/scan, internal/watch), which is unchanged
// and remains fail-closed. Here at the hook, posturizeWithAction() re-maps a
// "fired" outcome to a WARN by default, OR keeps it a BLOCK when the user opted
// that rule UP to block via cfg.Posture (IPOVR-03, Plan 29-01). The action is
// resolved per rule via cfg.PostureRuleAction(...); the default (no Posture block)
// stays warn. An untrusted layer may only tighten warn->block (config layered
// merge), never loosen. Critically, block applies ONLY to a DEFINITE violation -
// the unknown/fail-soft path below stays warn even under block (see warnUnknown).
//
// -- Fail-SOFT on unknown/timeout (Gate-1 load-bearing) -------------------------
// Resolving release-age and lifecycle requires a registry fetch. On a missing
// publish timestamp, an unsupported ecosystem, a registry error, or a fetch
// timeout, posture WARNS ("release age unknown" / "lifecycle scripts unknown") -
// it does NOT block. This is a DELIBERATE divergence from the pure evaluators'
// fail-closed block (which stays for the extension path). Rationale: the PRD's
// explicit goal is "do not break the build"; the primary malware defense is the
// corroboration catalog block (separate, still fail-closed, still merged
// most-restrictive in handler.go). A registry outage must not turn into a blocked
// install. The seams below let unit tests drive every branch without a network.
package check

import (
	"context"
	"net/http"
	"time"

	"github.com/home-beekeeper/beekeeper/internal/catalog"
	"github.com/home-beekeeper/beekeeper/internal/config"
	"github.com/home-beekeeper/beekeeper/internal/pkgparse"
	"github.com/home-beekeeper/beekeeper/internal/policy"
)

// postureFetchTimeout caps the registry I/O for a single posture evaluation.
// It is applied as a child context derived from the handler ctx, so posture can
// never blow the hook budget - on deadline the rule warns-unknown (fail-soft)
// and moves on. ~2.5s leaves slack under the 8s hook deadline even when both the
// release-age and lifecycle fetches miss the cache.
const postureFetchTimeout = 2500 * time.Millisecond

// posturePublishAgeFn and postureLifecycleFn are package-level seams (mirroring
// the old nudge.DetectStateFn seam) so unit tests can inject synthetic registry
// results without a real network or cache. Production assigns the cached catalog
// adapters; a test reassigns these and defer-restores them.
var (
	posturePublishAgeFn = catalog.FetchPublishAge
	postureLifecycleFn  = catalog.FetchLifecycleScripts
)

// evaluatePosture evaluates the three install-posture rules against a parsed
// install command and returns a single warn-capped decision plus ok.
//
// ok is false (skip) when:
//   - tc.ToolName is not "Bash" (posture only applies to shell commands)
//   - the Bash command string is absent or empty
//   - pkgparse.Parse reports !IsInstall (non-install commands never trigger I/O)
//
// When ok is true the returned decision is allow when every rule passed, warn when
// a rule fired at the default warn action, or BLOCK when a rule fired on a DEFINITE
// violation AND the user opted that rule UP to block via cfg.Posture (IPOVR-03).
// The warn-unknown (fail-soft) path NEVER blocks regardless of the configured
// action. The caller merges this via mergeDecisions (most-restrictive-wins), so a
// catalog/sensitive-path/self-protect BLOCK still wins and posture can never
// downgrade another block.
//
// client and cacheDir thread the handler's HTTP client and catalogs dir; now is
// the handler's wall-clock (time.Now().UTC() in production, synthetic in tests).
func evaluatePosture(
	ctx context.Context,
	tc policy.ToolCall,
	cfg config.Config,
	client *http.Client,
	cacheDir string,
	now time.Time,
) (policy.Decision, bool) {
	if tc.ToolName != "Bash" {
		return policy.Decision{}, false
	}
	cmd, ok := tc.ToolInput["command"].(string)
	if !ok || cmd == "" {
		return policy.Decision{}, false
	}
	parsed, parsedOK := pkgparse.Parse(cmd)
	if !parsedOK || !parsed.IsInstall {
		return policy.Decision{}, false
	}

	// Accumulate per-rule decisions, warn-capped, and merge most-restrictive.
	// Start at allow; any fired rule lifts the result to warn.
	result := policy.Decision{Allow: true, Level: "allow", Reason: "install posture: clean", RuleIDs: nil}

	// Resolve the per-rule action once (IPOVR-03). Default (no Posture block) is
	// "warn" for every rule; a trusted layer can opt a rule UP to "block".
	remoteAction := cfg.PostureRuleAction(config.PostureRuleRemoteSource)
	ageAction := cfg.PostureRuleAction(config.PostureRuleReleaseAge)
	lifecycleAction := cfg.PostureRuleAction(config.PostureRuleLifecycle)

	// -- Rule 1: remote source (pure, instant - no I/O) -------------------------
	// EvaluateRemoteSource already returns warn (never block) from the pure
	// evaluator; posturizeWithAction can still lift a FIRED remote-source rule to a
	// block when the user opted that rule up. A non-fired (allow) result is unchanged.
	remoteDec := policy.EvaluateRemoteSource(policy.RemoteSourceInput{
		Ecosystem: parsed.Ecosystem,
		Package:   remoteSpec(parsed),
		Kind:      parsed.RemoteSource,
	}, policy.DefaultRemoteSourceConfig())
	result = mergeDecisions(result, posturizeWithAction(remoteDec, remoteAction))

	// release-age and lifecycle only make sense for a registry package install
	// (a non-empty Package and no remote-source spec). A git/url/file install has
	// no registry name to resolve - the remote-source rule above already flagged it.
	registryInstall := parsed.RemoteSource == "" && parsed.Package != ""
	if registryInstall {
		// Child context so the registry fetches can never blow the hook budget;
		// on deadline both rules warn-unknown (fail-soft) below.
		fetchCtx, cancel := context.WithTimeout(ctx, postureFetchTimeout)
		defer cancel()

		version := parsed.Version
		if version == "" {
			version = "latest"
		}

		// -- Rule 2: release age (fail-soft) ------------------------------------
		ageMinutes, missing, ageErr := posturePublishAgeFn(
			fetchCtx, client, cacheDir, parsed.Ecosystem, parsed.Package, version, now,
		)
		if ageErr != nil || missing || fetchCtx.Err() != nil {
			// Unknown age (registry miss / unsupported / timeout / unexpected I/O).
			// Fail-soft: WARN, never block. The pure evaluator would block on
			// TimestampMissing; we intentionally do not call it on this path.
			result = mergeDecisions(result, warnUnknown(
				"release age unknown for "+parsed.Package+" (registry unavailable)",
				policy.RuleReleaseAge,
			))
		} else {
			ageDec := policy.EvaluateReleaseAge(policy.ReleaseAgeInput{
				Ecosystem:        parsed.Ecosystem,
				Package:          parsed.Package,
				AgeMinutes:       ageMinutes,
				TimestampMissing: false,
			}, policy.DefaultReleaseAgeConfig())
			result = mergeDecisions(result, posturizeWithAction(ageDec, ageAction))
		}

		// -- Rule 3: lifecycle scripts (fail-soft) ------------------------------
		scripts, failed, lifeErr := postureLifecycleFn(
			fetchCtx, client, cacheDir, parsed.Ecosystem, parsed.Package, version, now,
		)
		if lifeErr != nil || failed || fetchCtx.Err() != nil {
			// Unknown lifecycle (unsupported ecosystem / registry error / timeout).
			// Fail-soft: WARN, never block.
			result = mergeDecisions(result, warnUnknown(
				"lifecycle scripts unknown for "+parsed.Package+" (registry unavailable)",
				policy.RuleLifecycleScript,
			))
		} else {
			lifeDec := policy.EvaluateLifecycle(policy.LifecycleInput{
				Ecosystem:           parsed.Ecosystem,
				Package:             parsed.Package,
				ScriptsPresent:      scripts,
				RegistryCheckFailed: false,
			}, nil)
			result = mergeDecisions(result, posturizeWithAction(lifeDec, lifecycleAction))
		}
	}

	return result, true
}

// posturizeWithAction applies the configured per-rule posture ACTION (IPOVR-03) to
// a pure evaluator's decision over a DEFINITE violation:
//
//   - If the rule did NOT fire (d.Allow && d.Level == "allow"), the decision passes
//     through unchanged (an allow is never lifted to warn or block).
//   - If the rule FIRED and action == "block", the decision is returned as a BLOCK
//     (Allow:false, Level:"block") keeping the evaluator's Reason and RuleIDs. This
//     is the opt-up path: a user (or a tightening untrusted layer) raised this rule.
//   - Otherwise (the rule fired and action is the default "warn"), the outcome is
//     normalised to a WARN (Allow:true, Level:"warn") -- the shipped Phase 27 default
//     and the block->warn re-mapping that keeps the hook fail-soft.
//
// This is applied ONLY to a DEFINITE violation (a fired rule on a known input). The
// unknown/fail-soft path (warnUnknown) does NOT go through here and stays warn even
// under block -- a registry outage cannot turn into a blocked install.
//
// Enforcement boundary: see the file header / posture.BoundaryStatement.
func posturizeWithAction(d policy.Decision, action string) policy.Decision {
	if d.Allow && d.Level == "allow" {
		return d // rule did not fire -- never lift an allow
	}
	if action == config.PostureActionBlock {
		return policy.Decision{
			Allow:   false,
			Level:   "block",
			Reason:  d.Reason,
			RuleIDs: d.RuleIDs,
		}
	}
	return policy.Decision{
		Allow:   true,
		Level:   "warn",
		Reason:  d.Reason,
		RuleIDs: d.RuleIDs,
	}
}

// warnUnknown builds the fail-soft "unknown" warn decision used when a registry
// fetch is missing/errored/timed out. It WARNS rather than blocks - the
// deliberate divergence from the pure evaluators' fail-closed block.
//
// IPOVR-03 invariant: this is INDEPENDENT of the configured per-rule action. Block
// mode applies only to a DEFINITE violation (see posturizeWithAction); an unknown
// input always warns, EVEN when the rule is opted up to block. This is the line that
// keeps a registry outage from breaking installs -- do not route this through
// posturizeWithAction.
func warnUnknown(reason, ruleID string) policy.Decision {
	return policy.Decision{
		Allow:   true,
		Level:   "warn",
		Reason:  "install posture: " + reason,
		RuleIDs: []string{ruleID},
	}
}

// remoteSpec returns the install spec to surface for the remote-source rule:
// the parsed Package when present, else the raw command token. For a remote
// install Package is empty (pkgparse leaves it blank for git/url/file specs), so
// fall back to the raw command so the reason names what was flagged.
func remoteSpec(parsed pkgparse.ParsedCommand) string {
	if parsed.Package != "" {
		return parsed.Package
	}
	return parsed.Raw
}
