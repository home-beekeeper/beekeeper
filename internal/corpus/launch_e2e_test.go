// Package corpus — LAUNCH-02 evaluator gate.
//
// TestAllSentryPatternsProduceMoatRecord proves that each of the eight Sentry
// patterns (SENTRY-001..008) maps through the corpus write path to a CorpusRecord
// with all four layers populated and a signed push envelope.
//
// LAUNCH-02 "moat-grade" = all four layers PRESENT, TrueLabel="unresolved" accepted
// at capture time (RESEARCH.md Assumptions Log A2, MEDIUM risk, maintainer-flagged).
// To require TrueLabel="malicious" for all 8 patterns, drive RunAdjudicationBatch
// with a catalog hit per pattern (heavier — not done here per A2).
package corpus

// Import cycle note: internal/corpus CAN safely import internal/sentry if needed
// because internal/sentry does NOT import internal/corpus. This file does not import
// sentry because the test drives the corpus seam directly via audit.AuditRecord
// (the same shape the production Sentry daemon produces), without needing sentry types.

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/home-beekeeper/beekeeper/internal/audit"
	"github.com/home-beekeeper/beekeeper/internal/config"
)

// TestAllSentryPatternsProduceMoatRecord is the LAUNCH-02 evaluator gate.
//
// LAUNCH-02 "moat-grade" = all four layers PRESENT, TrueLabel="unresolved" accepted
// at capture time (RESEARCH.md Assumptions Log A2, MEDIUM risk, maintainer-flagged).
// To require TrueLabel="malicious" for all 8 patterns, drive RunAdjudicationBatch
// with a catalog hit per pattern (heavier — not done here per A2).
//
// The test drives the corpus write path at the AuditRecord → MapToCorpusRecord seam
// (the same production path used by NewMultiSinkWithCorpus → StoreSink.Write →
// MapToCorpusRecord). It does NOT call EvaluateEvent — the sentry surface import is
// verified here only for import-cycle safety; the eight AuditRecord values are
// constructed directly, exactly as the production Sentry daemon does before calling
// writeCorpusRecord.
func TestAllSentryPatternsProduceMoatRecord(t *testing.T) {
	type sentryPatternCase struct {
		name        string
		ruleID      string
		severity    string
		description string // human-readable for test documentation
	}

	cases := []sentryPatternCase{
		{
			name:        "SENTRY-001 cred-file-access",
			ruleID:      "SENTRY-001",
			severity:    "critical",
			description: "credential-file-access cluster (EventFileAccess, >= CredAccessThreshold within CredAccessWindowSec)",
		},
		{
			name:        "SENTRY-002 cred-cli-spawn",
			ruleID:      "SENTRY-002",
			severity:    "critical",
			description: "credential-CLI spawn cluster (EventProcessCreate, >= CredCLIThreshold within CredCLIWindowSec)",
		},
		{
			name:        "SENTRY-003 phone-home",
			ruleID:      "SENTRY-003",
			severity:    "high",
			description: "outbound phone-home (EventNetworkConnect, first external outbound within PhoneHomeWindowMin)",
		},
		{
			name:        "SENTRY-004 fresh-extension",
			ruleID:      "SENTRY-004",
			severity:    "high",
			description: "fresh-extension install with suspicious activity (post-001/002/003, within FreshExtWindowMin)",
		},
		{
			name:        "SENTRY-005 exfil-fusion",
			ruleID:      "SENTRY-005",
			severity:    "critical",
			description: "exfil-fusion: cred-access + fresh-extension + outbound within ExfilFusionWindowMin",
		},
		{
			name:        "SENTRY-006 agent-standalone-cred-access",
			ruleID:      "SENTRY-006",
			severity:    "critical",
			description: "standalone-agent cred access: agent-descended but NOT editor-descended + non-self-config reads",
		},
		{
			name:        "SENTRY-007 exfil-fusion-persist",
			ruleID:      "SENTRY-007",
			severity:    "critical",
			description: "exfil-fusion with recent persistence-write (EventNetworkConnect + recent cred or persist)",
		},
		{
			name:        "SENTRY-008 persistence-write",
			ruleID:      "SENTRY-008",
			severity:    "high",
			description: "persistence-path write (EventFileWrite, isPersistencePath, per-path-per-session dedup)",
		},
	}

	now := time.Now().UTC()

	for _, tc := range cases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			// Step 1: Build a representative audit.AuditRecord for this Sentry pattern.
			//
			// The corpus write path consumes an AuditRecord (not a raw SentryAlert).
			// The production Sentry daemon converts SentryAlert → AuditRecord inside
			// the daemon's audit sink before calling writeCorpusRecord. We construct
			// the same AuditRecord shape directly — the minimal set of fields the
			// corpus mapper requires to populate all four layers.
			rec := audit.AuditRecord{
				RecordType:         "policy_decision",
				RecordID:           "launch-02-" + tc.ruleID,
				Timestamp:          now.Format(time.RFC3339),
				ScannerName:        "beekeeper",
				SourceSurface:      "sentry",
				SentryRuleID:       tc.ruleID,
				SentryRuleName:     tc.description, // non-empty rule name proxy
				SentrySeverity:     tc.severity,
				ToolName:           tc.ruleID,     // action_type proxy for sentry surface
				Decision:           "alert",       // sentry surface uses "alert" (detection-only)
				Reason:             tc.severity,   // non-empty reason
				CorroborationCount: 1,
				RulesetVersion:     "1.0",
				ClusterID:          "launch-02-" + tc.ruleID,
				SourcesAgreed:      []string{},
				SourcesDissented:   []string{},
			}

			// Step 2: Call MapToCorpusRecord — the production corpus write seam.
			//
			// This is exactly what StoreSink.Write calls after redaction.
			// cfg.Enabled=true mirrors the production corpus-enabled config.
			// repoFingerprint and fleetNodeID use test sentinels (HMAC outputs
			// are not computed here — this is STORE-05 territory, exercised in
			// the adjudicator tests).
			corpusRec := MapToCorpusRecord(rec, config.CorpusConfig{Enabled: true}, "launch02-repo-fp", "launch02-node")

			// Step 3: Assert all four corpus layers are populated.
			//
			// Pattern: []fieldCheck table + range loop (from schema_lock_test.go lines 208–285).
			// "LAUNCH-02 gap: <field>" messages name the layer for easy diagnosis.
			type fieldCheck struct {
				name    string
				value   string
				wantAny bool   // true = any non-empty value is acceptable
				allow   string // expected exact value (if wantAny==false)
				skip    bool
				skipMsg string
			}

			checks := []fieldCheck{
				// --- Behavior layer ---
				// SourceSurface must be "sentry" — the corpus emitter uses this to
				// identify the Sentry surface in schema_lock_test.go line 219.
				{name: "source_surface", value: corpusRec.AuditRecord.SourceSurface, allow: "sentry"},
				// SentryRuleID must be non-empty (the specific rule that fired).
				{name: "sentry_rule_id", value: corpusRec.AuditRecord.SentryRuleID, wantAny: true},

				// --- Decision layer ---
				// Decision must be "alert" for Sentry surface (detection-only).
				{name: "decision", value: corpusRec.AuditRecord.Decision, allow: "alert"},
				// Reason or SentryRuleName must be non-empty (diagnostic detail).
				{name: "reason_or_sentry_rule_name", value: func() string {
					if corpusRec.AuditRecord.Reason != "" {
						return corpusRec.AuditRecord.Reason
					}
					return corpusRec.AuditRecord.SentryRuleName
				}(), wantAny: true},

				// --- Outcome layer (THE MOAT — non-retrofittable) ---
				// TrueLabel must be "unresolved" at run-1 (the moat-grade placeholder).
				// A2: "moat-grade" for LAUNCH-02 = all four layers PRESENT with
				// TrueLabel="unresolved"; resolving to "malicious" requires
				// RunAdjudicationBatch with a catalog hit (not done here per A2).
				{name: "true_label (unresolved moat)", value: corpusRec.TrueLabel, allow: "unresolved"},

				// --- Context layer ---
				// CorpusSchemaVersion must be the frozen "1.0" constant.
				{name: "corpus_schema_version", value: corpusRec.CorpusSchemaVersion, allow: CorpusSchemaVersion},
			}

			for _, c := range checks {
				if c.skip {
					t.Logf("LAUNCH-02 skip: %s — %s", c.name, c.skipMsg)
					continue
				}
				if c.wantAny {
					if c.value == "" {
						t.Errorf("LAUNCH-02 gap [%s]: %s is empty (want any non-empty value)", tc.ruleID, c.name)
					}
				} else {
					if c.value != c.allow {
						t.Errorf("LAUNCH-02 gap [%s]: %s = %q; want %q", tc.ruleID, c.name, c.value, c.allow)
					}
				}
			}

			// Scope check: zero-value CorpusScope ("") marshals to "org_only" via
			// MarshalJSON (SCOPE-01 guarantee). Accept either the zero in-memory string
			// or the explicit "org_only" constant.
			scope := string(corpusRec.Scope)
			if scope != "org_only" && scope != "" {
				t.Errorf("LAUNCH-02 gap [%s]: scope = %q; want \"org_only\" or \"\" (zero-value marshals to org_only via SCOPE-01)", tc.ruleID, scope)
			}

			// Step 4: Assert push envelope is present and carries the moat invariants.
			//
			// MapToCorpusRecord populates PushEnvelope from the first write (STORE-04 /
			// ENV-01). Assertions use the envelope-fragment pattern from
			// schema_lock_test.go lines 370–395.
			if corpusRec.PushEnvelope == nil {
				t.Fatalf("LAUNCH-02 gap [%s]: PushEnvelope is nil — must be populated from first write (STORE-04)", tc.ruleID)
			}

			// ActionHint must be ActionHintWatchAndBlock (the only typed value — SCHEMA-04).
			if corpusRec.PushEnvelope.ActionHint != ActionHintWatchAndBlock {
				t.Errorf("LAUNCH-02 [%s]: ActionHint = %q; want ActionHintWatchAndBlock", tc.ruleID, corpusRec.PushEnvelope.ActionHint)
			}

			// ConfidenceTier must be non-empty (initial: "watch"; enforced after adjudication).
			if corpusRec.PushEnvelope.ConfidenceTier == "" {
				t.Errorf("LAUNCH-02 [%s]: ConfidenceTier is empty; want non-empty", tc.ruleID)
			}

			// BehaviorSignatureHash must be 64-char hex (SCHEMA-06 gate, frozen Phase 22).
			// MapToCorpusRecord computes it via BehaviorSigHash(ToolName, "", "") because
			// no SentryFilesAccessed or SentryNetworkDests are set in the minimal record.
			if len(corpusRec.PushEnvelope.Signature.BehaviorSignatureHash) != 64 {
				t.Errorf("LAUNCH-02 [%s]: BehaviorSignatureHash = %q; want 64-char hex (got %d chars)",
					tc.ruleID,
					corpusRec.PushEnvelope.Signature.BehaviorSignatureHash,
					len(corpusRec.PushEnvelope.Signature.BehaviorSignatureHash))
			}

			// Verify the hash is valid hex (not just 64 arbitrary chars).
			hash := corpusRec.PushEnvelope.Signature.BehaviorSignatureHash
			for _, ch := range hash {
				if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
					t.Errorf("LAUNCH-02 [%s]: BehaviorSignatureHash contains non-hex char %q", tc.ruleID, ch)
					break
				}
			}

			// Envelope JSON fragment assertions (pattern from schema_lock_test.go lines 370–395).
			envJSON, err := json.Marshal(corpusRec.PushEnvelope)
			if err != nil {
				t.Fatalf("LAUNCH-02 [%s]: json.Marshal(PushEnvelope): %v", tc.ruleID, err)
			}
			envStr := string(envJSON)
			envChecks := []struct {
				fragment string
				desc     string
			}{
				{`"action_hint":"watch_and_block"`, "action_hint must be watch_and_block"},
				{`"confidence_tier"`, "confidence_tier must be present"},
				{`"behavior_signature_hash"`, "behavior_signature_hash must be present"},
			}
			for _, ec := range envChecks {
				if !strings.Contains(envStr, ec.fragment) {
					t.Errorf("LAUNCH-02 [%s]: %s — fragment %q not in envelope JSON: %s", tc.ruleID, ec.desc, ec.fragment, envStr)
				}
			}
		})
	}
}
