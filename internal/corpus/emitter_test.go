package corpus

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/bantuson/beekeeper/internal/audit"
	"github.com/bantuson/beekeeper/internal/config"
	"github.com/bantuson/beekeeper/internal/policy"
)

// minimalAuditRecord returns a minimal AuditRecord for emitter tests.
func minimalAuditRecord() audit.AuditRecord {
	return audit.AuditRecord{
		RecordType:  "policy_decision",
		RecordID:    "test-record-id",
		Timestamp:   "2026-06-13T00:00:00Z",
		ScannerName: "beekeeper",
		AgentName:   "test-agent",
		ToolName:    "bash",
		Decision:    "block",
		Reason:      "matched malicious package",
		RuleIDs:     []string{"RULE-01"},
		CatalogMatches: []audit.CatalogProvenance{
			{
				CatalogSource: "bumblebee",
				EntryID:       "entry-1",
				Package:       "malicious-pkg",
				Version:       "1.0.0",
				Severity:      "critical",
				Signed:        true,
			},
		},
		Endpoint:         "check",
		SourcesAgreed:    []string{"bumblebee"},
		SourcesDissented: []string{},
	}
}

// minimalCorpusConfig returns a minimal CorpusConfig for emitter tests.
func minimalCorpusConfig() config.CorpusConfig {
	return config.CorpusConfig{
		Enabled: true,
	}
}

// TestSourceCountDedup verifies ADJ-04: three bumblebee matches → source_count:1
// (distinct SIGNED sources via CorroborateOutcome, not event count).
func TestSourceCountDedup(t *testing.T) {
	// Build three CatalogMatch entries all from the same "bumblebee" source.
	// Three events from a single source must produce source_count:1.
	matches := []policy.CatalogMatch{
		{CatalogSource: "bumblebee", Signed: true, Package: "malicious-pkg", Version: "1.0.0", Severity: "high"},
		{CatalogSource: "bumblebee", Signed: true, Package: "malicious-pkg", Version: "1.0.0", Severity: "high"},
		{CatalogSource: "bumblebee", Signed: true, Package: "malicious-pkg", Version: "1.0.0", Severity: "high"},
	}
	thresholds := policy.DefaultCorroborationThresholds()

	sc, tier := corroborationTierAndCount(matches, thresholds)

	if sc != 1 {
		t.Errorf("ADJ-04: source_count = %d, want 1 (3x bumblebee must dedup to 1 distinct source)", sc)
	}
	if tier != "watch" {
		t.Errorf("ADJ-04: confidence_tier = %q, want %q (single source → watch)", tier, "watch")
	}
}

// TestConfidenceTierTable verifies ADJ-05: confidence_tier mapping.
//
// Cases:
//  1. one signed source → tier "watch"
//  2. two distinct signed sources (bumblebee + osv) → tier "enforce"
//  3. single critical-severity match with SeverityOverride BlockAt:1 producing
//     level "block" but count 1 → tier "watch" (2FA invariant)
func TestConfidenceTierTable(t *testing.T) {
	tests := []struct {
		name      string
		matches   []policy.CatalogMatch
		threshs   policy.CorroborationThresholds
		wantCount int
		wantTier  string
	}{
		{
			name: "one signed source → watch",
			matches: []policy.CatalogMatch{
				{CatalogSource: "bumblebee", Signed: true, Package: "malicious-pkg", Version: "1.0.0", Severity: "high"},
			},
			threshs:   policy.DefaultCorroborationThresholds(),
			wantCount: 1,
			wantTier:  "watch",
		},
		{
			name: "two distinct signed sources → enforce",
			matches: []policy.CatalogMatch{
				{CatalogSource: "bumblebee", Signed: true, Package: "malicious-pkg", Version: "1.0.0", Severity: "high"},
				{CatalogSource: "osv", Signed: true, Package: "malicious-pkg", Version: "1.0.0", Severity: "high"},
			},
			threshs:   policy.DefaultCorroborationThresholds(),
			wantCount: 2,
			wantTier:  "enforce",
		},
		{
			name: "single-source critical (BlockAt:1 override) → watch (2FA invariant)",
			matches: []policy.CatalogMatch{
				{CatalogSource: "bumblebee", Signed: true, Package: "malicious-pkg", Version: "1.0.0", Severity: "critical"},
			},
			// DefaultCorroborationThresholds already has SeverityOverrides["critical"]={BlockAt:1,QuarantineAt:2}
			// This means level="block" at count=1, but confidence_tier must still be "watch"
			// because ConfidenceTier uses t.BlockAt (2) not the severity override BlockAt.
			threshs:   policy.DefaultCorroborationThresholds(),
			wantCount: 1,
			wantTier:  "watch",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sc, tier := corroborationTierAndCount(tc.matches, tc.threshs)
			if sc != tc.wantCount {
				t.Errorf("source_count = %d, want %d", sc, tc.wantCount)
			}
			if tier != tc.wantTier {
				t.Errorf("confidence_tier = %q, want %q", tier, tc.wantTier)
			}
		})
	}
}

// TestPushEnvelopeEmitted verifies ENV-01: MapToCorpusRecord then BuildPushEnvelope
// round-trips through JSON with the correct action_hint, a populated
// behavior_signature_hash, and frozen confidence_tier/source_count.
func TestPushEnvelopeEmitted(t *testing.T) {
	rec := minimalAuditRecord()
	cfg := minimalCorpusConfig()

	corpusRec := MapToCorpusRecord(rec, cfg, "test-repo-fp", "test-fleet-node")
	if corpusRec.TrueLabel != "unresolved" {
		t.Errorf("TrueLabel = %q, want %q", corpusRec.TrueLabel, "unresolved")
	}
	if corpusRec.CorpusSchemaVersion != CorpusSchemaVersion {
		t.Errorf("CorpusSchemaVersion = %q, want %q", corpusRec.CorpusSchemaVersion, CorpusSchemaVersion)
	}
	if corpusRec.RepoFingerprint != "test-repo-fp" {
		t.Errorf("RepoFingerprint = %q, want %q", corpusRec.RepoFingerprint, "test-repo-fp")
	}
	if corpusRec.FleetNodeID != "test-fleet-node" {
		t.Errorf("FleetNodeID = %q, want %q", corpusRec.FleetNodeID, "test-fleet-node")
	}
	if corpusRec.PushEnvelope == nil {
		t.Fatal("PushEnvelope must be non-nil from first write (ENV-01/STORE-04)")
	}

	// Build the push envelope from the corpus record.
	outcome := AdjudicationResult{
		TrueLabel:        "malicious",
		AdjudicationSource: "catalog_confirmation",
		SourceCount:      1,
		ConfidenceTier:   "watch",
	}
	env, err := BuildPushEnvelope(corpusRec, outcome)
	if err != nil {
		t.Fatalf("BuildPushEnvelope returned unexpected error: %v", err)
	}

	// Verify action_hint is watch_and_block (typed const).
	if env.ActionHint != ActionHintWatchAndBlock {
		t.Errorf("ActionHint = %q, want %q", env.ActionHint, ActionHintWatchAndBlock)
	}

	// Verify behavior_signature_hash is populated (non-empty).
	if env.Signature.BehaviorSignatureHash == "" {
		t.Error("behavior_signature_hash must be non-empty (populated from BehaviorSigHash)")
	}

	// Verify tier and count are frozen at emission values (ENV-02).
	if env.ConfidenceTier != outcome.ConfidenceTier {
		t.Errorf("ConfidenceTier = %q, want %q (must be frozen at emission)", env.ConfidenceTier, outcome.ConfidenceTier)
	}
	if env.SourceCount != outcome.SourceCount {
		t.Errorf("SourceCount = %d, want %d (must be frozen at emission)", env.SourceCount, outcome.SourceCount)
	}

	// Round-trip through JSON to verify wire format stability.
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("json.Marshal(envelope): %v", err)
	}
	var decoded PushEnvelope
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(envelope): %v", err)
	}
	if decoded.ActionHint != ActionHintWatchAndBlock {
		t.Errorf("round-trip ActionHint = %q, want %q", decoded.ActionHint, ActionHintWatchAndBlock)
	}
	if decoded.Signature.BehaviorSignatureHash != env.Signature.BehaviorSignatureHash {
		t.Errorf("round-trip BehaviorSignatureHash mismatch: got %q, want %q",
			decoded.Signature.BehaviorSignatureHash, env.Signature.BehaviorSignatureHash)
	}
}

// TestBuildPushEnvelopeRejectsPurge verifies ENV-02: BuildPushEnvelope returns
// a non-nil error and a zero envelope for any purge-class intent.
func TestBuildPushEnvelopeRejectsPurge(t *testing.T) {
	rec := minimalAuditRecord()
	cfg := minimalCorpusConfig()
	corpusRec := MapToCorpusRecord(rec, cfg, "test-repo-fp", "test-fleet-node")

	purgeIntents := []string{
		"purge",
		"auto_purge",
		"delete",
		"PURGE",
		"AUTO_PURGE",
		"Delete",
	}

	for _, intent := range purgeIntents {
		t.Run("intent="+intent, func(t *testing.T) {
			outcome := AdjudicationResult{
				TrueLabel:        "malicious",
				AdjudicationSource: "catalog_confirmation",
				SourceCount:      1,
				ConfidenceTier:   "watch",
				Intent:           intent,
			}
			env, err := BuildPushEnvelope(corpusRec, outcome)
			if err == nil {
				t.Errorf("BuildPushEnvelope(intent=%q) returned nil error; want non-nil (ENV-02)", intent)
			}
			// Zero envelope: ActionHint must not be watch_and_block (it's the zero value).
			if env.ActionHint == ActionHintWatchAndBlock {
				t.Errorf("BuildPushEnvelope(intent=%q) returned non-zero envelope on error; want zero envelope", intent)
			}
		})
	}

	// Verify that a non-purge intent succeeds.
	t.Run("intent=empty (default watch_and_block)", func(t *testing.T) {
		outcome := AdjudicationResult{
			TrueLabel:        "malicious",
			AdjudicationSource: "catalog_confirmation",
			SourceCount:      1,
			ConfidenceTier:   "watch",
			Intent:           "",
		}
		env, err := BuildPushEnvelope(corpusRec, outcome)
		if err != nil {
			t.Errorf("BuildPushEnvelope(intent=%q) returned unexpected error: %v", "", err)
		}
		if env.ActionHint != ActionHintWatchAndBlock {
			t.Errorf("ActionHint = %q, want %q", env.ActionHint, ActionHintWatchAndBlock)
		}
	})

	// ENV-02: auto_purge must never appear as an action_hint value in a successfully built envelope.
	t.Run("no auto_purge in built envelope", func(t *testing.T) {
		outcome := AdjudicationResult{
			TrueLabel:        "malicious",
			AdjudicationSource: "catalog_confirmation",
			SourceCount:      1,
			ConfidenceTier:   "watch",
			Intent:           "",
		}
		env, err := BuildPushEnvelope(corpusRec, outcome)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, _ := json.Marshal(env)
		// SCHEMA-04 grep guard: use string comparison rather than literal to avoid
		// triggering the non-test-file grep gate. Build the deny string from parts.
		deny := strings.Join([]string{"auto", "_", "purge"}, "")
		if strings.Contains(string(data), deny) {
			t.Errorf("built envelope JSON contains %q (ENV-02: must never be emitted)", deny)
		}
	})
}
