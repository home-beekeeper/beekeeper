package corpus

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/bantuson/beekeeper/internal/audit"
)

// TestCorpusRecordSchema verifies the four-layer CorpusRecord schema:
//   - AuditRecord fields are promoted to the top-level JSON object (NOT nested)
//   - TrueLabel is always present in JSON even when WasCorrect is nil
//   - WasCorrect is absent from JSON when nil (pointer semantics)
//
// Covers: SCHEMA-01 (embedding promotion, outcome-layer placeholders)
func TestCorpusRecordSchema(t *testing.T) {
	var rec CorpusRecord
	rec.AuditRecord = audit.AuditRecord{
		RecordType:  "policy_decision",
		ScannerName: "beekeeper",
		Decision:    "block",
	}
	rec.AuditRecord.SourceSurface = "sentry"
	rec.TrueLabel = "unresolved"
	rec.CorpusSchemaVersion = CorpusSchemaVersion

	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("json.Marshal(CorpusRecord): %v", err)
	}
	out := string(data)

	// AuditRecord.SourceSurface must be promoted to the top level:
	// JSON must contain "source_surface":"sentry" at the root.
	if !strings.Contains(out, `"source_surface":"sentry"`) {
		t.Errorf("expected top-level \"source_surface\":\"sentry\" in JSON; got: %s", out)
	}

	// The embedded struct must NOT produce a nested "AuditRecord" object.
	if strings.Contains(out, `"AuditRecord"`) {
		t.Errorf("JSON must not contain a nested \"AuditRecord\" key (unnamed embedding must promote fields); got: %s", out)
	}

	// TrueLabel must be present (no omitempty on this field).
	if !strings.Contains(out, `"true_label"`) {
		t.Errorf("expected \"true_label\" key to always be present in JSON; got: %s", out)
	}

	// WasCorrect must be absent when nil (omitempty on *bool).
	if strings.Contains(out, `"was_correct"`) {
		t.Errorf("expected \"was_correct\" to be absent from JSON when WasCorrect is nil; got: %s", out)
	}

	// Decision must be promoted from AuditRecord.
	if !strings.Contains(out, `"decision":"block"`) {
		t.Errorf("expected top-level \"decision\":\"block\" promoted from AuditRecord; got: %s", out)
	}
}

// TestPushEnvelopeRoundTrip verifies that PushEnvelope marshals and unmarshals
// round-trip with all fields present.
//
// Sub-test A: Signing nil — "signing" key must be absent (omitempty).
// Sub-test B: Signing non-nil zero-value — "signing" key must be present.
//
// Covers: SCHEMA-03 (envelope wire format frozen, round-trip, signing zero-value in v1)
func TestPushEnvelopeRoundTrip(t *testing.T) {
	t.Run("signing_nil", func(t *testing.T) {
		env := PushEnvelope{
			Signature: EnvelopeSignature{
				PackageOrExtensionID:  "npm:evil-pkg",
				Version:               "1.2.3",
				BehaviorSignatureHash: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab",
				IOCs: IOCBlock{
					Domains: []string{"evil.example.com"},
				},
			},
			TrueLabel:      "malicious",
			ConfidenceTier: "enforce",
			SourceCount:    2,
			Scope:          ScopeOrgOnly,
			ActionHint:     ActionHintWatchAndBlock,
			Signing:        nil, // v1 zero-value — must be omitted from JSON
		}

		data, err := json.Marshal(env)
		if err != nil {
			t.Fatalf("json.Marshal(PushEnvelope): %v", err)
		}
		out := string(data)

		// "signing" must be absent when nil (omitempty).
		if strings.Contains(out, `"signing"`) {
			t.Errorf("expected \"signing\" to be absent when Signing is nil; got: %s", out)
		}

		// All required fields must be present.
		for _, want := range []string{
			`"true_label":"malicious"`,
			`"confidence_tier":"enforce"`,
			`"source_count":2`,
			`"action_hint":"watch_and_block"`,
			`"scope":"org_only"`,
			`"package_or_extension_id":"npm:evil-pkg"`,
			`"behavior_signature_hash"`,
			`"domains":["evil.example.com"]`,
		} {
			if !strings.Contains(out, want) {
				t.Errorf("expected %q in marshalled PushEnvelope; got: %s", want, out)
			}
		}

		// Round-trip: unmarshal and check fields.
		var got PushEnvelope
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("json.Unmarshal(PushEnvelope): %v", err)
		}
		if got.TrueLabel != env.TrueLabel {
			t.Errorf("TrueLabel: got %q, want %q", got.TrueLabel, env.TrueLabel)
		}
		if got.ConfidenceTier != env.ConfidenceTier {
			t.Errorf("ConfidenceTier: got %q, want %q", got.ConfidenceTier, env.ConfidenceTier)
		}
		if got.SourceCount != env.SourceCount {
			t.Errorf("SourceCount: got %d, want %d", got.SourceCount, env.SourceCount)
		}
		if got.ActionHint != env.ActionHint {
			t.Errorf("ActionHint: got %q, want %q", got.ActionHint, env.ActionHint)
		}
		if got.Scope != env.Scope {
			t.Errorf("Scope: got %q, want %q", got.Scope, env.Scope)
		}
		if got.Signing != nil {
			t.Errorf("Signing: expected nil after round-trip with nil Signing; got %+v", got.Signing)
		}
		if got.Signature.PackageOrExtensionID != env.Signature.PackageOrExtensionID {
			t.Errorf("Signature.PackageOrExtensionID: got %q, want %q",
				got.Signature.PackageOrExtensionID, env.Signature.PackageOrExtensionID)
		}
	})

	t.Run("signing_zero_value", func(t *testing.T) {
		env := PushEnvelope{
			Signature:      EnvelopeSignature{PackageOrExtensionID: "npm:pkg"},
			TrueLabel:      "malicious",
			ConfidenceTier: "watch",
			SourceCount:    1,
			Scope:          ScopeOrgOnly,
			ActionHint:     ActionHintWatchAndBlock,
			Signing:        &SigningBlock{}, // non-nil zero value — must appear in JSON
		}

		data, err := json.Marshal(env)
		if err != nil {
			t.Fatalf("json.Marshal(PushEnvelope with signing): %v", err)
		}
		out := string(data)

		// "signing" must be present when Signing is non-nil.
		if !strings.Contains(out, `"signing"`) {
			t.Errorf("expected \"signing\" key present when Signing is non-nil zero-value; got: %s", out)
		}

		// Round-trip.
		var got PushEnvelope
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("json.Unmarshal: %v", err)
		}
		if got.Signing == nil {
			t.Errorf("Signing: expected non-nil after round-trip with &SigningBlock{}; got nil")
		}
	})
}

// TestScopeZeroValue verifies the SCOPE-01 guarantee:
//   - CorpusRecord{} (zero value) serializes scope as "org_only"
//   - CorpusScope("").MarshalJSON() returns "org_only"
//
// Covers: SCOPE-01
func TestScopeZeroValue(t *testing.T) {
	// Zero-value CorpusRecord must serialize scope as "org_only".
	rec := CorpusRecord{}
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("json.Marshal(CorpusRecord{}): %v", err)
	}
	out := string(data)

	if !strings.Contains(out, `"scope":"org_only"`) {
		t.Errorf("expected \"scope\":\"org_only\" in zero-value CorpusRecord JSON; got: %s", out)
	}

	// Direct MarshalJSON on empty CorpusScope.
	bs, err := CorpusScope("").MarshalJSON()
	if err != nil {
		t.Fatalf("CorpusScope(\"\").MarshalJSON(): %v", err)
	}
	if string(bs) != `"org_only"` {
		t.Errorf("CorpusScope(\"\").MarshalJSON(): got %s, want \"org_only\"", string(bs))
	}

	// community_shareable round-trips correctly.
	bs2, err := ScopeCommunityShareable.MarshalJSON()
	if err != nil {
		t.Fatalf("ScopeCommunityShareable.MarshalJSON(): %v", err)
	}
	if string(bs2) != `"community_shareable"` {
		t.Errorf("ScopeCommunityShareable.MarshalJSON(): got %s, want \"community_shareable\"", string(bs2))
	}
}

// TestPromoteScopeReturnsErrorInV1 verifies the SCOPE-02 guarantee:
//   - PromoteScope returns a non-nil error
//   - PromoteScope leaves r.Scope unchanged
//
// Covers: SCOPE-02
func TestPromoteScopeReturnsErrorInV1(t *testing.T) {
	rec := &CorpusRecord{Scope: ScopeOrgOnly}
	originalScope := rec.Scope

	err := PromoteScope(rec)

	if err == nil {
		t.Error("PromoteScope: expected non-nil error in v1, got nil")
	}
	if rec.Scope != originalScope {
		t.Errorf("PromoteScope: expected Scope to be unchanged (%q), got %q", originalScope, rec.Scope)
	}
}

// TestActionHintTypeSafety verifies the SCHEMA-04 type-safety guarantee:
//   - ActionHintWatchAndBlock has the correct underlying string value
//   - A PushEnvelope with ActionHint set to ActionHintWatchAndBlock round-trips
//     as "watch_and_block" in JSON
//
// The compile-time guarantee (only ActionHintWatchAndBlock is defined; assigning
// any unlisted string to the ActionHint field is a compile error) is enforced
// by the type system and verified by:
//   - go build ./internal/corpus/... (must succeed)
//   - grep for unlisted action hint strings in internal/corpus/ (must return 0 matches)
//
// Covers: SCHEMA-04
func TestActionHintTypeSafety(t *testing.T) {
	// ActionHintWatchAndBlock must equal the typed ActionHint("watch_and_block").
	if ActionHintWatchAndBlock != ActionHint("watch_and_block") {
		t.Errorf("ActionHintWatchAndBlock: got %q, want ActionHint(\"watch_and_block\")", ActionHintWatchAndBlock)
	}

	// PushEnvelope with ActionHintWatchAndBlock round-trips as "watch_and_block".
	env := PushEnvelope{
		ActionHint: ActionHintWatchAndBlock,
	}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("json.Marshal(PushEnvelope{ActionHint: ActionHintWatchAndBlock}): %v", err)
	}
	if !strings.Contains(string(data), `"action_hint":"watch_and_block"`) {
		t.Errorf("expected \"action_hint\":\"watch_and_block\" in JSON; got: %s", string(data))
	}

	var got PushEnvelope
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got.ActionHint != ActionHintWatchAndBlock {
		t.Errorf("ActionHint after round-trip: got %q, want %q", got.ActionHint, ActionHintWatchAndBlock)
	}
}
