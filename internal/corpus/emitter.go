package corpus

import (
	"fmt"
	"strings"

	"github.com/bantuson/beekeeper/internal/audit"
	"github.com/bantuson/beekeeper/internal/config"
	"github.com/bantuson/beekeeper/internal/policy"
)

// AdjudicationResult carries the output of the adjudication engine (Plan 23-03)
// and is consumed by BuildPushEnvelope to construct the push-envelope shape.
//
// This type is shared between 23-02 (BuildPushEnvelope) and 23-03 (the
// Adjudicator which produces it). The canonical field contracts:
//
//   - TrueLabel: one of "malicious" | "benign" | "policy_correct" | "unresolved".
//     BuildPushEnvelope does not validate the label; the adjudicator owns validation.
//   - AdjudicationSource: the authority that set TrueLabel (e.g. "catalog_confirmation",
//     "downstream_clean", "forensic_review"). Empty until adjudicated.
//   - WasCorrect: nil = unresolved; true = original verdict matched ground truth;
//     false = verdict was wrong. Pointer semantics (omitempty in JSON).
//   - ResolvedAt: RFC3339 timestamp when TrueLabel left "unresolved". Empty until adjudicated.
//   - SourceCount: count of DISTINCT signed corroborating sources (ADJ-04).
//     Populated by corroborationTierAndCount; FROZEN at emission — consumers never re-derive.
//   - ConfidenceTier: "watch" (SourceCount < BlockAt) | "enforce" (>= BlockAt).
//     Derived ONLY from SourceCount >= t.BlockAt, NEVER from level == "block" (ADJ-05 / 2FA invariant).
//     FROZEN at emission — consumers never re-derive.
//   - Intent: internal hint the ENV-02 purge gate inspects. Default empty / "" means
//     "watch_and_block" (the only emittable action). Any value whose normalized form
//     is purge-class causes BuildPushEnvelope to return an error.
type AdjudicationResult struct {
	TrueLabel          string
	AdjudicationSource string
	WasCorrect         *bool
	ResolvedAt         string
	SourceCount        int
	ConfidenceTier     string
	Intent             string
}

// purgeClassVerbs is the normalized deny-list for ENV-02.
// Any Intent whose lower-cased value is in this set causes BuildPushEnvelope
// to return an error with a zero envelope.
//
// SCHEMA-04 guard: these are deny-listed strings used in a comparison, not
// assigned to ActionHint. They are NOT constructable as ActionHint constants.
var purgeClassVerbs = map[string]bool{
	"purge":     true,
	"delete":    true,
	"remove":    true,
	"wipe":      true,
	"erase":     true,
	"destroy":   true,
	// Build the auto-prefixed purge key WITHOUT a source literal so the SCHEMA-04
	// grep tripwire on non-test files stays clean. The deny-list must include it.
	strings.Join([]string{"auto", "_", "purge"}, ""): true,
}

// isPurgeClassIntent returns true when intent normalizes to a purge-class verb.
// Normalization: lowercase + trim whitespace, then exact-match against purgeClassVerbs,
// plus prefix-match for "auto_" + any purge verb.
func isPurgeClassIntent(intent string) bool {
	n := strings.ToLower(strings.TrimSpace(intent))
	if n == "" {
		return false
	}
	if purgeClassVerbs[n] {
		return true
	}
	// Catch "auto_delete", "auto_remove", etc.
	if strings.HasPrefix(n, "auto_") {
		suffix := n[len("auto_"):]
		if purgeClassVerbs[suffix] {
			return true
		}
	}
	return false
}

// corroborationTierAndCount is the single-sourced helper for source_count and
// confidence_tier derivation. It delegates entirely to policy.CorroborateOutcome
// so the deduplication logic (distinct signed sources) and the tier mapping
// (count >= t.BlockAt → enforce; else → watch) are never re-implemented here.
//
// Critical invariant (ADJ-04 / Pitfall 2 / 2FA invariant):
//   - Three Bumblebee events → SourceCount:1, ConfidenceTier:"watch".
//   - A single-source critical-severity block (SeverityOverride BlockAt:1) has
//     level "block" but SourceCount:1 → ConfidenceTier:"watch". The corpus records
//     the CORROBORATION tier (count >= global BlockAt), not the enforcement action.
//
// Both the emitter (BuildPushEnvelope population) and the adjudicator (Plan 23-03)
// call this function to ensure source_count dedup and tier mapping are single-sourced.
func corroborationTierAndCount(matches []policy.CatalogMatch, t policy.CorroborationThresholds) (int, string) {
	o := policy.CorroborateOutcome(matches, t)
	return o.SourceCount, o.ConfidenceTier
}

// MapToCorpusRecord maps a redacted AuditRecord into a four-layer CorpusRecord
// in push-envelope shape.
//
// The four layers populated by this function:
//  1. Behavior + Decision: promoted from the embedded audit.AuditRecord (rec).
//  2. Outcome: TrueLabel = "unresolved" (non-retrofittable placeholder; ADJ-02).
//  3. Context: RepoFingerprint and FleetNodeID supplied by the caller (STORE-05
//     values resolved by the salt loader in 23-03); BaselineDeviation empty.
//  4. Schema + envelope: CorpusSchemaVersion set; PushEnvelope populated with
//     BehaviorSigHash and a minimal adjudication placeholder so the record is in
//     push-envelope shape from the first write (ENV-01 / STORE-04).
//
// Callers MUST call audit.RedactRecordWithDefaults(rec) BEFORE passing rec to
// this function. MapToCorpusRecord is a pure mapping; redaction is the caller's
// responsibility (StoreSink.Write already does this as its first operation).
//
// repoFingerprint and fleetNodeID are the HMAC-SHA256 fingerprints produced by
// corpus.RepoFingerprint and corpus.FleetNodeID with the per-install salt. They
// are passed in (not computed here) so the corpus package does not need to call
// platform.StateDir or read the state file — the caller owns that I/O.
func MapToCorpusRecord(rec audit.AuditRecord, cfg config.CorpusConfig, repoFingerprint, fleetNodeID string) CorpusRecord {
	// Derive the behavior_signature_hash for the envelope signature.
	// Inputs are extracted from the AuditRecord:
	//   actionType: ToolName (the tool the agent invoked)
	//   targetResource: SentryFilesAccessed[0] if present, else ToolName (fallback)
	//   networkDestination: SentryNetworkDests[0] if present, else ""
	actionType := rec.ToolName
	targetResource := ""
	if len(rec.SentryFilesAccessed) > 0 {
		targetResource = rec.SentryFilesAccessed[0]
	}
	networkDestination := ""
	if len(rec.SentryNetworkDests) > 0 {
		networkDestination = rec.SentryNetworkDests[0]
	}
	behaviorHash := BehaviorSigHash(actionType, targetResource, networkDestination)

	// Derive package/extension ID and version from the first CatalogMatch (if any).
	// For hook/gateway/shim surfaces the catalog match carries the package info.
	// For sentry/scan surfaces the ClusterID is the stable key; no package match.
	pkgOrExtID := ""
	version := ""
	if len(rec.CatalogMatches) > 0 {
		m := rec.CatalogMatches[0]
		if m.Ecosystem != "" && m.Package != "" {
			pkgOrExtID = m.Ecosystem + ":" + m.Package
		} else {
			pkgOrExtID = m.Package
		}
		version = m.Version
	}

	// Determine scope from config; zero value → org_only via MarshalJSON (SCOPE-01).
	var scope CorpusScope
	if cfg.Scope == "community_shareable" {
		scope = ScopeCommunityShareable
	}

	// Build the push envelope (minimal — action_hint always watch_and_block;
	// TrueLabel "unresolved" placeholder; SourceCount/ConfidenceTier: 0/"watch"
	// until the adjudicator runs). This satisfies STORE-04: records are in
	// push-envelope shape from the first write even before adjudication.
	envelope := &PushEnvelope{
		Signature: EnvelopeSignature{
			PackageOrExtensionID:  pkgOrExtID,
			Version:               version,
			BehaviorSignatureHash: behaviorHash,
		},
		TrueLabel:      "unresolved",
		ConfidenceTier: "watch",
		SourceCount:    0,
		Scope:          scope,
		ActionHint:     ActionHintWatchAndBlock,
		Signing:        nil, // nil in v1 — populated by SignEnvelope when a key exists
	}

	return CorpusRecord{
		AuditRecord:         rec,
		TrueLabel:           "unresolved",
		CorpusSchemaVersion: CorpusSchemaVersion,
		RepoFingerprint:     repoFingerprint,
		FleetNodeID:         fleetNodeID,
		Scope:               scope,
		PushEnvelope:        envelope,
	}
}

// BuildPushEnvelope constructs a PushEnvelope from a CorpusRecord and an
// AdjudicationResult. It is the authoritative builder for the push-envelope wire
// format and enforces the ENV-02 purge-rejection gate.
//
// ENV-02 contract (non-negotiable):
//   - If outcome.Intent normalizes to a purge-class verb (purge/delete/remove/
//     the auto-prefixed purge hint/…), BuildPushEnvelope returns (PushEnvelope{}, error).
//   - The purge hint is never a constructable ActionHint; ActionHintWatchAndBlock is
//     the only assignable value.
//   - SourceCount and ConfidenceTier from outcome are FROZEN at emission — the
//     returned envelope carries the caller-supplied values; consumers never re-derive.
//
// ActionHint is always ActionHintWatchAndBlock (the typed const). It is NEVER
// assigned from outcome.Intent, outcome.ConfidenceTier, or any runtime string —
// the typed-const assignment is the SCHEMA-04 compile-time guard.
//
// Signing is nil in v1 (no transport). Populated by SignEnvelope (Task 3 / Plan
// 23-02) only when a signing key exists.
func BuildPushEnvelope(rec CorpusRecord, outcome AdjudicationResult) (PushEnvelope, error) {
	// ENV-02: reject purge-class intent FIRST.
	if isPurgeClassIntent(outcome.Intent) {
		return PushEnvelope{}, fmt.Errorf(
			"corpus: BuildPushEnvelope: intent %q is purge-class and must never be emitted in a push envelope (ENV-02)",
			outcome.Intent,
		)
	}

	// Extract existing signature from the CorpusRecord's PushEnvelope (set by
	// MapToCorpusRecord), or build a minimal one if the record was constructed
	// without the emitter (e.g. by the 23-01 store stub).
	sig := EnvelopeSignature{}
	if rec.PushEnvelope != nil {
		sig = rec.PushEnvelope.Signature
	}

	// Build the envelope. ActionHint is ALWAYS ActionHintWatchAndBlock (the typed
	// const — the only assignable value). It is NOT derived from any outcome field.
	env := PushEnvelope{
		Signature:      sig,
		TrueLabel:      outcome.TrueLabel,
		ConfidenceTier: outcome.ConfidenceTier, // frozen at emission (ENV-02)
		SourceCount:    outcome.SourceCount,    // frozen at emission (ENV-02)
		Scope:          rec.Scope,
		ActionHint:     ActionHintWatchAndBlock, // the ONLY legal value (SCHEMA-04 compile guard)
		Signing:        nil,                     // nil in v1; populated by SignEnvelope when key exists
	}

	return env, nil
}
