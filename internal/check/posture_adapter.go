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
// -- Default action = WARN (Gate-1 load-bearing) --------------------------------
// The PRD default posture WARNS; it does not block. The pure EvaluateReleaseAge
// and EvaluateLifecycle return `block` on a violation - correct for the scan/watch
// EXTENSION supply-chain path (internal/scan, internal/watch), which is unchanged
// and remains fail-closed. Here at the hook, posturize() re-maps any "fired"
// outcome to a WARN decision. Raising a rule to block per ecosystem is roadmap
// (deep per-rule editing); there is deliberately no severity knob.
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
// When ok is true the returned decision is allow when every rule passed, or warn
// when any rule fired (including warn-unknown on a registry miss). It NEVER
// returns a block - that is the WARN-default + fail-soft contract above. The
// caller merges it via mergeDecisions, so a catalog/sensitive-path/self-protect
// BLOCK still wins (posture can never downgrade a block).
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
	_ = cfg // reserved: per-rule allowlists/thresholds are roadmap; defaults today.

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

	// -- Rule 1: remote source (pure, instant - no I/O) -------------------------
	// EvaluateRemoteSource already returns warn (never block); posturize is a
	// no-op for it but applied uniformly for clarity.
	remoteDec := policy.EvaluateRemoteSource(policy.RemoteSourceInput{
		Ecosystem: parsed.Ecosystem,
		Package:   remoteSpec(parsed),
		Kind:      parsed.RemoteSource,
	}, policy.DefaultRemoteSourceConfig())
	result = mergeDecisions(result, posturize(remoteDec))

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
			result = mergeDecisions(result, posturize(ageDec))
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
			result = mergeDecisions(result, posturize(lifeDec))
		}
	}

	return result, true
}

// posturize applies the PRD Layer-1 default posture ACTION (warn) to a pure
// evaluator's decision. If the rule did not fire (d.Allow with Level "allow"),
// the decision passes through unchanged. If the rule "fired" - the pure
// evaluators express that as a block (release-age too young, lifecycle scripts
// present) or as a warn (remote source) - the outcome is normalised to a WARN:
// Allow:true, Level:"warn". This is the single place the block→warn re-mapping
// happens, and it is WHY a registry outage at the hook cannot block an install.
//
// Enforcement boundary: see the file header / posture.BoundaryStatement. Raising
// a rule to block per ecosystem is roadmap (no severity knob today).
func posturize(d policy.Decision) policy.Decision {
	if d.Allow && d.Level == "allow" {
		return d
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
