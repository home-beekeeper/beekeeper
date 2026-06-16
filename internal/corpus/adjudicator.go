// Package corpus — adjudicator.go
//
// The adjudication engine assigns outcome labels (true_label) to unresolved
// CorpusRecords using six documented adjudication sources. The engine is split
// into two layers:
//
//  1. Pure inner function: Adjudicate(rec CorpusRecord, signals AdjudicationSignals)
//     — no I/O, no goroutines, fully unit-testable.
//  2. Impure batch driver: RunAdjudicationBatch(ctx, corpusPath, ...) — reads the
//     corpus NDJSON, calls Adjudicate, appends superseding records via StoreSink.
//
// Critical invariant (T-23-09 / Pitfall 3 / ADJ-01):
// RunAdjudicationBatch MUST NEVER run on the beekeeper check hot path
// (internal/check/handler.go). It lives exclusively in runCatalogsSync
// (cmd/beekeeper/catalogs_daemon.go). handler.go MUST NOT import this file.
//
// Operator sources (forensic_review, breach_confirmation, user_override,
// benign_explained) are synchronous CLI/TUI writes wired in Phase 24.
// OperatorAdjudication is a helper stub provided for Phase 24.
package corpus

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/home-beekeeper/beekeeper/internal/policy"
)

// Adjudication source constants (ADJ-03).
// These are the six documented values for CorpusRecord.AdjudicationSource.
// Confidence mapping is documented below and verified by TestAdjudicationSources.
const (
	// catalog_confirmation — confidence: medium
	// The package/version was re-queried against the catalog at adjudication time
	// and returned a match. Automatic; runs in RunAdjudicationBatch.
	AdjSourceCatalogConfirmation = "catalog_confirmation"

	// forensic_review — confidence: high
	// A human analyst reviewed the incident and confirmed the verdict.
	// Operator source; written by Phase 24 CLI/TUI.
	AdjSourceForensicReview = "forensic_review"

	// breach_confirmation — confidence: high
	// External breach intelligence confirmed the package/version as malicious.
	// Operator source; written by Phase 24 CLI/TUI.
	AdjSourceBreachConfirmation = "breach_confirmation"

	// user_override — confidence: weak
	// The operator explicitly set the label via CLI/TUI.
	// Operator source; written by Phase 24 CLI/TUI.
	AdjSourceUserOverride = "user_override"

	// downstream_clean — confidence: weak
	// No correlated follow-on incidents appeared within the configurable clean window.
	// Automatic; runs in RunAdjudicationBatch.
	AdjSourceDownstreamClean = "downstream_clean"

	// benign_explained — confidence: medium
	// The operator confirmed the behavior was benign and documented the explanation.
	// Operator source; written by Phase 24 CLI/TUI.
	AdjSourceBenignExplained = "benign_explained"
)

// Adjudication confidence tiers (ADJ-03).
const (
	AdjConfidenceHigh   = "high"
	AdjConfidenceMedium = "medium"
	AdjConfidenceWeak   = "weak"
)

// adjSourceConfidenceMap maps adjudication_source values to confidence tiers.
// This map is the single source of truth for TestAdjudicationSources (ADJ-03).
var adjSourceConfidenceMap = map[string]string{
	AdjSourceForensicReview:      AdjConfidenceHigh,
	AdjSourceBreachConfirmation:  AdjConfidenceHigh,
	AdjSourceCatalogConfirmation: AdjConfidenceMedium,
	AdjSourceBenignExplained:     AdjConfidenceMedium,
	AdjSourceDownstreamClean:     AdjConfidenceWeak,
	AdjSourceUserOverride:        AdjConfidenceWeak,
}

// AdjudicationSourceConfidence returns the documented confidence tier for the
// given adjudication_source value. Returns "" for unknown sources.
// Verified by TestAdjudicationSources (ADJ-03).
func AdjudicationSourceConfidence(source string) string {
	return adjSourceConfidenceMap[source]
}

// AdjudicationSignals carries the inputs to the pure Adjudicate function.
// All signal fields are resolved by RunAdjudicationBatch before calling Adjudicate.
type AdjudicationSignals struct {
	// CatalogConfirmed is true when a re-query of the catalog returned a match
	// for the package/version in the corpus record. Sets TrueLabel "malicious"
	// with source "catalog_confirmation".
	CatalogConfirmed bool

	// DownstreamCleanElapsed is true when no follow-on incident with the same
	// ClusterID has appeared within the configurable cleanWindowDays window.
	// Sets TrueLabel "benign" with source "downstream_clean".
	// Only applied when CatalogConfirmed is false.
	DownstreamCleanElapsed bool

	// Matches is the slice of CatalogMatch values from the catalog re-query.
	// Used to derive SourceCount and ConfidenceTier via corroborationTierAndCount.
	Matches []policy.CatalogMatch

	// Thresholds are the corroboration thresholds for the SourceCount/ConfidenceTier
	// derivation. Populated from the configured policy thresholds.
	Thresholds policy.CorroborationThresholds

	// Now is the current time (injected for testability; callers use time.Now().UTC()).
	Now time.Time
}

// Adjudicate is the pure inner function of the adjudication engine.
// It takes a CorpusRecord and its resolved signals and returns an AdjudicationResult.
//
// PURITY INVARIANT (T-23-13 / ADJ-01):
//   - No I/O, no goroutines, no side effects.
//   - Calls policy.CorroborateOutcome read-only (pure) for source_count/confidence_tier.
//   - Tests can call this directly without any file system access.
//
// Label transition rules (ADJ-02):
//  1. CatalogConfirmed → TrueLabel "malicious", source catalog_confirmation.
//  2. DownstreamCleanElapsed (and not CatalogConfirmed) → TrueLabel "benign", source downstream_clean.
//  3. Otherwise → record unchanged (TrueLabel stays "unresolved", result is the zero value).
//
// Only the 4-value set {malicious, benign, policy_correct, unresolved} is reachable.
// "policy_correct" is an operator-only label (Phase 24); this function produces
// "malicious" or "benign" from automatic sources only.
//
// WasCorrect derivation (ADJ-06):
//
//	verdict==block AND label==malicious → true (correct block)
//	verdict==allow AND label==benign   → true (correct allow)
//	verdict==block AND label==benign   → false (false positive)
//	verdict==allow AND label==malicious → false (missed)
//	policy_correct                      → always true (the operator marked it correct)
func Adjudicate(rec CorpusRecord, signals AdjudicationSignals) AdjudicationResult {
	// If no signal fires, return the record unchanged (still unresolved).
	if !signals.CatalogConfirmed && !signals.DownstreamCleanElapsed {
		return AdjudicationResult{
			TrueLabel: "unresolved",
		}
	}

	// Determine the new label and source.
	var trueLabel, adjSource string
	if signals.CatalogConfirmed {
		trueLabel = "malicious"
		adjSource = AdjSourceCatalogConfirmation
	} else {
		// DownstreamCleanElapsed (weakest signal; only applies when catalog did not confirm).
		trueLabel = "benign"
		adjSource = AdjSourceDownstreamClean
	}

	// Derive source_count and confidence_tier via the single-sourced corroboration helper.
	// This is purely read-only against policy.CorroborateOutcome — no I/O.
	sourceCount, confidenceTier := corroborationTierAndCount(signals.Matches, signals.Thresholds)

	// Derive was_correct (ADJ-06):
	// The record's Decision field records the enforcement action (allow|warn|block).
	// A block on a malicious package is correct; an allow on a benign package is correct.
	// policy_correct is an operator label — it always means was_correct=true.
	wasCorrect := deriveWasCorrect(rec.AuditRecord.Decision, trueLabel)

	// Set resolved_at to the current time (RFC3339 UTC).
	resolvedAt := signals.Now.UTC().Format(time.RFC3339)

	return AdjudicationResult{
		TrueLabel:          trueLabel,
		AdjudicationSource: adjSource,
		WasCorrect:         wasCorrect,
		ResolvedAt:         resolvedAt,
		SourceCount:        sourceCount,
		ConfidenceTier:     confidenceTier,
	}
}

// deriveWasCorrect computes was_correct from the original verdict (decision field)
// and the new true_label. Returns a *bool — nil only when called on an unresolved
// record (which Adjudicate never does; callers must handle that case).
//
//	malicious + block → true (correct block)
//	benign    + allow/warn → true (correct allow)
//	malicious + allow/warn → false (missed; allowed something malicious)
//	benign    + block → false (false positive; blocked something benign)
//	policy_correct → always true (operator marks verdict correct)
func deriveWasCorrect(verdict, trueLabel string) *bool {
	t := true
	f := false
	switch trueLabel {
	case "policy_correct":
		return &t
	case "malicious":
		if verdict == "block" {
			return &t // correct block
		}
		return &f // allow/warn on malicious = missed
	case "benign":
		if verdict == "block" {
			return &f // false positive: blocked benign
		}
		return &t // allow/warn on benign = correct
	default:
		return nil // unresolved — should not be called but be safe
	}
}

// maxRecordsToScan is the maximum number of NDJSON lines to scan per batch pass.
// At v1 corpus sizes (<10MB), a full scan with this cap is simpler than an index
// and completes well within the 5s bounded deadline (OQ-3 resolution).
const maxRecordsToScan = 50_000

// RunAdjudicationBatch is the impure batch driver for the adjudication engine.
// It reads unresolved CorpusRecords from the NDJSON corpus file, evaluates each
// against automatic adjudication signals (catalog_confirmation, downstream_clean),
// and appends SUPERSEDING CorpusRecords for any that transition to a resolved label.
//
// Design decisions (OQ-3 / ADJ-07 / T-23-12):
//   - Full scan with maxRecordsToScan cap (simpler than tail/index; corpus <10MB in v1).
//   - Collapses to latest record per ClusterID: reads ALL records, then processes only
//     the latest unresolved one per cluster.
//   - Superseding records are append-only: prior "unresolved" lines are preserved.
//     Consumers take the latest record per ClusterID.
//   - Clusters are processed in a DETERMINISTIC order (WR-02): oldest Timestamp
//     first, tie-broken by ClusterID. So when the 5s deadline truncates a large
//     batch, the frontier always advances the OLDEST unresolved clusters first
//     (guaranteed forward progress) and the output order is reproducible.
//   - ctx deadline is honored between records: hitting the deadline after writing
//     partial progress is an EXPECTED, designed outcome and returns nil (IN-01),
//     so operators don't see a scary error on every large-corpus sync. The error
//     return is reserved for genuine I/O failures.
//   - Errors are returned for logging; the CALLER decides they are non-fatal.
//     A batch-pass error MUST NOT cause runCatalogsSync to fail.
//
// Parameters:
//
//	ctx          — carries the 5s deadline from runCatalogsSync (or context.Background in tests)
//	corpusPath   — path to the beekeeper-corpus.ndjson file
//	stateFile    — path to state.json for the corpus signing key / salt (unused in v1 batch; reserved)
//	idx          — MultiCatalogLookup for catalog_confirmation re-queries (mmap index or fake)
//	t            — corroboration thresholds (from policyloader or defaults)
//	cleanWindowDays — downstream_clean window; default 30 (OQ-1)
func RunAdjudicationBatch(ctx context.Context, corpusPath, stateFile string, idx policy.MultiCatalogLookup, t policy.CorroborationThresholds, cleanWindowDays int) error {
	// Step 1: Read all records from the corpus NDJSON, capped at maxRecordsToScan.
	f, err := os.Open(corpusPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no corpus file yet — nothing to adjudicate
		}
		return fmt.Errorf("open corpus for adjudication: %w", err)
	}
	defer f.Close()

	var allRecords []CorpusRecord
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1MB line buffer
	scanned := 0
	for scanner.Scan() {
		if scanned >= maxRecordsToScan {
			break
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec CorpusRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			// Skip malformed lines — do not abort the batch.
			continue
		}
		allRecords = append(allRecords, rec)
		scanned++
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan corpus: %w", err)
	}
	f.Close() // close before writing (Windows cannot write to open-for-read files)

	// Step 2: Collapse to the latest record per ClusterID.
	// "Latest" = last occurrence in the NDJSON file (NDJSON is append-only).
	// In the same pass, build an O(1) cluster-occurrence count (WR-05) so the
	// downstream_clean branch never re-scans allRecords per record (O(n^2)).
	latestByCluster := make(map[string]CorpusRecord)
	clusterCounts := make(map[string]int)
	for _, rec := range allRecords {
		id := clusterKeyOf(rec)
		if id == "" {
			continue // no identifier — skip
		}
		clusterCounts[id]++
		latestByCluster[id] = rec // last write wins (NDJSON order = append order)
	}

	// Step 3: For each still-unresolved record, run the automatic adjudication signals.
	if cleanWindowDays <= 0 {
		cleanWindowDays = 30
	}
	cleanWindow := time.Duration(cleanWindowDays) * 24 * time.Hour
	now := time.Now().UTC()

	// WR-02: process clusters in a DETERMINISTIC order — oldest Timestamp first,
	// tie-broken by ClusterID — so the bounded-deadline frontier always advances
	// the oldest unresolved clusters first and output is reproducible.
	orderedKeys := make([]string, 0, len(latestByCluster))
	for k := range latestByCluster {
		orderedKeys = append(orderedKeys, k)
	}
	sort.Slice(orderedKeys, func(i, j int) bool {
		ri := latestByCluster[orderedKeys[i]]
		rj := latestByCluster[orderedKeys[j]]
		ti := ri.AuditRecord.Timestamp
		tj := rj.AuditRecord.Timestamp
		if ti != tj {
			return ti < tj // lexical RFC3339 sort == chronological; oldest first
		}
		return orderedKeys[i] < orderedKeys[j] // stable tie-break by ClusterID
	})

	for _, key := range orderedKeys {
		rec := latestByCluster[key]

		// Honor context deadline between records (T-23-12 / OQ-3). Hitting the
		// deadline after writing partial progress is an expected outcome, not an
		// error (IN-01): return nil so the caller doesn't log a scary message.
		if ctx.Err() != nil {
			return nil
		}

		// Only process unresolved records.
		if rec.TrueLabel != "unresolved" {
			continue
		}

		// Resolve catalog_confirmation: re-query the mmap index for the
		// package/version recorded in the corpus record.
		var matches []policy.CatalogMatch
		var catalogConfirmed bool
		if idx != nil {
			pkg := ""
			ecosystem := ""
			if rec.PushEnvelope != nil {
				// PackageOrExtensionID may be "ecosystem:package" or just "package".
				id := rec.PushEnvelope.Signature.PackageOrExtensionID
				for i, c := range id {
					if c == ':' {
						ecosystem = id[:i]
						pkg = id[i+1:]
						break
					}
				}
				if pkg == "" {
					pkg = id
				}
			}
			// Fall back to ToolName if no package info available.
			if pkg == "" {
				pkg = rec.AuditRecord.ToolName
			}
			if pkg != "" {
				matches = idx.LookupAll(ecosystem, pkg)
				catalogConfirmed = len(matches) > 0
			}
		}

		// Resolve downstream_clean: check if the record's Timestamp is older than
		// the clean window AND no correlated follow-on exists (per-machine, per-ClusterID).
		// For v1, "no follow-on" is approximated by: only ONE record exists for this
		// ClusterID in the entire corpus (single occurrence = no follow-on incident).
		var downstreamCleanElapsed bool
		if !catalogConfirmed && rec.AuditRecord.Timestamp != "" {
			ts, err := time.Parse(time.RFC3339, rec.AuditRecord.Timestamp)
			if err == nil && now.Sub(ts) >= cleanWindow {
				// O(1) occurrence lookup from the pre-built count map (WR-05).
				// Only label benign if there is exactly one occurrence (no follow-on).
				downstreamCleanElapsed = clusterCounts[key] == 1
			}
		}

		// Call the pure Adjudicate function.
		signals := AdjudicationSignals{
			CatalogConfirmed:      catalogConfirmed,
			DownstreamCleanElapsed: downstreamCleanElapsed,
			Matches:               matches,
			Thresholds:            t,
			Now:                   now,
		}
		result := Adjudicate(rec, signals)

		// If the result is still unresolved, skip — no superseding record needed.
		if result.TrueLabel == "unresolved" {
			continue
		}

		// Build the superseding CorpusRecord (ADJ-07):
		// - NEW RecordID (generated via newAdjudicationRecordID)
		// - SAME ClusterID as the original
		// - outcome fields updated from AdjudicationResult
		// - prior "unresolved" line is preserved on disk (append-only)
		superseding := rec // shallow copy
		superseding.AuditRecord.RecordID = newAdjudicationRecordID()
		superseding.TrueLabel = result.TrueLabel
		superseding.AdjudicationSource = result.AdjudicationSource
		superseding.WasCorrect = result.WasCorrect
		superseding.ResolvedAt = result.ResolvedAt
		if superseding.PushEnvelope != nil && result.ConfidenceTier != "" {
			// Update the push envelope outcome fields. Copy the pointer target so
			// mutating the superseding envelope never aliases the original record
			// still held in latestByCluster.
			env := *superseding.PushEnvelope
			env.TrueLabel = result.TrueLabel
			env.ConfidenceTier = result.ConfidenceTier
			env.SourceCount = result.SourceCount
			superseding.PushEnvelope = &env
		}

		// WR-01/IN-03/WR-03: append the fully-resolved superseding record via the
		// single shared writer (redaction-first, O_CREATE, single-Write, size cap).
		// The dead StoreSink lazy-open machinery is gone — every write goes through
		// AppendCorpusRecordLine, so there is no spurious second Windows write
		// handle and the redaction invariant is uniform across both write paths.
		if writeErr := appendCorpusRecord(corpusPath, superseding); writeErr != nil {
			// Log to stderr via error return — caller decides non-fatal.
			return fmt.Errorf("append superseding record for cluster %q: %w",
				rec.AuditRecord.ClusterID, writeErr)
		}
	}

	return nil
}

// clusterKeyOf returns the correlation key for a record: ClusterID, falling back
// to RecordID when ClusterID is empty. Centralizes the key derivation used by
// both the latest-per-cluster collapse and the occurrence count.
func clusterKeyOf(rec CorpusRecord) string {
	if rec.AuditRecord.ClusterID != "" {
		return rec.AuditRecord.ClusterID
	}
	return rec.AuditRecord.RecordID
}

// appendCorpusRecord appends a fully-resolved superseding CorpusRecord to the
// NDJSON file. It delegates to the shared AppendCorpusRecordLine writer (IN-03)
// so the superseding path honors the same redaction-first (WR-03), O_CREATE
// (IN-04), single-Write, and size-cap (WR-06) behaviour as the hook hot path,
// with no duplicated open/marshal logic to drift.
func appendCorpusRecord(corpusPath string, rec CorpusRecord) error {
	return AppendCorpusRecordLine(corpusPath, rec)
}

// newAdjudicationRecordID generates a random 128-bit hex identifier for a
// superseding corpus record. Matches the newRecordID pattern in handler.go.
func newAdjudicationRecordID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("adj-ts-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// OperatorAdjudication builds an AdjudicationResult for operator-driven sources
// (forensic_review, breach_confirmation, user_override, benign_explained).
// This is the stub for Phase 24 CLI/TUI wiring. Phase 24 calls this to produce
// the AdjudicationResult that is then written as a superseding CorpusRecord via
// AppendSupersedingRecord.
//
// trueLabel must be one of: "malicious" | "benign" | "policy_correct".
// source must be one of the operator AdjSource* constants.
// Returns an error if trueLabel or source is invalid.
func OperatorAdjudication(source, trueLabel, verdict string, matches []policy.CatalogMatch, t policy.CorroborationThresholds) (AdjudicationResult, error) {
	validLabels := map[string]bool{
		"malicious":      true,
		"benign":         true,
		"policy_correct": true,
	}
	if !validLabels[trueLabel] {
		return AdjudicationResult{}, fmt.Errorf("operator adjudication: invalid true_label %q (must be malicious|benign|policy_correct)", trueLabel)
	}
	operatorSources := map[string]bool{
		AdjSourceForensicReview:     true,
		AdjSourceBreachConfirmation: true,
		AdjSourceUserOverride:       true,
		AdjSourceBenignExplained:    true,
	}
	if !operatorSources[source] {
		return AdjudicationResult{}, fmt.Errorf("operator adjudication: invalid source %q (must be forensic_review|breach_confirmation|user_override|benign_explained)", source)
	}

	sourceCount, confidenceTier := corroborationTierAndCount(matches, t)
	wasCorrect := deriveWasCorrect(verdict, trueLabel)
	resolvedAt := time.Now().UTC().Format(time.RFC3339)

	return AdjudicationResult{
		TrueLabel:          trueLabel,
		AdjudicationSource: source,
		WasCorrect:         wasCorrect,
		ResolvedAt:         resolvedAt,
		SourceCount:        sourceCount,
		ConfidenceTier:     confidenceTier,
	}, nil
}
