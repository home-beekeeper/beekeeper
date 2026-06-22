package audit

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/home-beekeeper/beekeeper/internal/policy"
)

// TestNudgeFieldsRoundTrip verifies that the deprecated-but-retained nudge
// provenance fields (OriginalCommand/RewrittenCommand/ReasonCode/PMState/
// NudgeAction) still marshal and unmarshal correctly. The nudge feature was
// removed in v1.1.0, but these fields are RETAINED in the frozen CorpusRecord
// schema (CorpusSchemaVersion 1.0) — this test guards that the frozen schema
// can still serialize a record carrying them.
func TestNudgeFieldsRoundTrip(t *testing.T) {
	rec := AuditRecord{
		RecordType:       "nudge",
		RecordID:         "nudge-test-001",
		Timestamp:        "2026-06-04T10:00:00Z",
		ScannerName:      "beekeeper",
		ToolName:         "Bash",
		Decision:         "warn",
		OriginalCommand:  "npm install chalk@5.4.0",
		RewrittenCommand: "pnpm add chalk@5.4.0",
		ReasonCode:       "pnpm-available-soft",
		PMState:          `{"pnpm_version":"11.3.0","pnpm_hardened":true}`,
		NudgeAction:      "advise",
	}

	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	jsonStr := string(data)

	for _, want := range []string{
		`"record_type":"nudge"`,
		`"original_command":"npm install chalk@5.4.0"`,
		`"rewritten_command":"pnpm add chalk@5.4.0"`,
		`"reason_code":"pnpm-available-soft"`,
		`"pm_state":`,
		`"nudge_action":"advise"`,
	} {
		if !strings.Contains(jsonStr, want) {
			t.Errorf("JSON missing %q\nGot: %s", want, jsonStr)
		}
	}

	// Unmarshal and verify round-trip fidelity.
	var got AuditRecord
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got.RecordType != "nudge" {
		t.Errorf("RecordType = %q, want nudge", got.RecordType)
	}
	if got.OriginalCommand != rec.OriginalCommand {
		t.Errorf("OriginalCommand = %q, want %q", got.OriginalCommand, rec.OriginalCommand)
	}
	if got.RewrittenCommand != rec.RewrittenCommand {
		t.Errorf("RewrittenCommand = %q, want %q", got.RewrittenCommand, rec.RewrittenCommand)
	}
	if got.ReasonCode != rec.ReasonCode {
		t.Errorf("ReasonCode = %q, want %q", got.ReasonCode, rec.ReasonCode)
	}
	if got.PMState != rec.PMState {
		t.Errorf("PMState = %q, want %q", got.PMState, rec.PMState)
	}
	if got.NudgeAction != rec.NudgeAction {
		t.Errorf("NudgeAction = %q, want %q", got.NudgeAction, rec.NudgeAction)
	}
}

// TestNudgeRecordConformsToPRDSection9 guards the FROZEN nudge record schema.
// The nudge feature was removed in v1.1.0, but the AuditRecord nudge fields are
// retained for corpus schema compatibility (CorpusSchemaVersion 1.0). This test
// constructs a representative nudge record, marshals it to JSON, and asserts the
// historical PRD §9 field set still serializes with legal closed-enum values, so
// a frozen-schema corpus reader can still parse such records.
//
// Closed sets are defined as test-local literals (the package that defined them
// is gone); they mirror the historical §9 schema.
func TestNudgeRecordConformsToPRDSection9(t *testing.T) {
	legalDecisions := map[string]bool{
		"allow": true,
		"warn":  true,
		"block": true,
	}
	legalNudgeActions := map[string]bool{
		"advise":  true,
		"proceed": true,
		"rewrite": true,
		"block":   true,
	}
	// Closed reason enum mirroring the historical §9 schema.
	legalReasonCodes := map[string]bool{
		"pnpm-available-soft":            true,
		"pnpm-available-hard":            true,
		"bun-available-soft":             true,
		"bun-available-hard":             true,
		"bun-available-no-scanner":       true,
		"no-hardened-pm":                 true,
		"no-hardened-pm-block":           true,
		"node-incompatible-with-pnpm-11": true,
		"sudo-passthrough":               true,
		"not-applicable":                 true,
	}

	// Construct a representative §9 nudge record.
	rec := AuditRecord{
		RecordType:       "nudge",
		RecordID:         "conformance-test-001",
		Timestamp:        "2026-06-04T14:22:07Z",
		ScannerName:      "beekeeper",
		AgentName:        "claude-code",
		ToolName:         "Bash",
		Decision:         "warn",      // repo enum: allow|warn|block
		NudgeAction:      "advise",    // §9 enum: advise|proceed|rewrite|block
		ReasonCode:       "pnpm-available-soft",
		OriginalCommand:  "npm install chalk@5.4.0",
		RewrittenCommand: "",          // empty for advise (not rewrite)
		PMState:          `{"pnpm_version":"11.3.0","pnpm_hardened":true,"bun_version":"","bun_scanner_ok":false,"node_version":"22.5.0"}`,
	}

	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	jsonStr := string(data)

	// Assert the COMPLETE §9 field set is present in the JSON output.
	// Fields from the §9 schema that map to AuditRecord fields:
	requiredFields := []string{
		`"record_type"`,
		`"timestamp"`,
		`"scanner_name"`,
		`"tool_name"`,
		`"original_command"`,
		`"decision"`,
		`"reason_code"`,
		`"pm_state"`,
		`"nudge_action"`,
	}
	for _, field := range requiredFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("§9 required field %s is missing from nudge record JSON\nGot: %s", field, jsonStr)
		}
	}

	// Assert record_type is "nudge" or "version_drift".
	if rec.RecordType != "nudge" && rec.RecordType != "version_drift" {
		t.Errorf("record_type %q: must be \"nudge\" or \"version_drift\"", rec.RecordType)
	}

	// Assert Decision ∈ {allow, warn, block} (repo enum).
	if !legalDecisions[rec.Decision] {
		t.Errorf("Decision %q is outside the closed enum {allow,warn,block}", rec.Decision)
	}

	// Assert NudgeAction ∈ {advise, proceed, rewrite, block} (§9 enum).
	if !legalNudgeActions[rec.NudgeAction] {
		t.Errorf("NudgeAction %q is outside the closed §9 enum {advise,proceed,rewrite,block}", rec.NudgeAction)
	}

	// Assert ReasonCode is non-empty and from the closed enum.
	if rec.ReasonCode == "" {
		t.Error("ReasonCode must be non-empty on a nudge record")
	}
	if !legalReasonCodes[rec.ReasonCode] {
		t.Errorf("ReasonCode %q is not in the closed reason enum", rec.ReasonCode)
	}

	// Assert that a NudgeAction outside the closed set fails the check.
	bad := AuditRecord{
		RecordType:  "nudge",
		RecordID:    "bad-001",
		Timestamp:   "2026-06-04T14:22:07Z",
		ScannerName: "beekeeper",
		ToolName:    "Bash",
		Decision:    "warn",
		NudgeAction: "unknown-action", // illegal
		ReasonCode:  "pnpm-available-soft",
	}
	if legalNudgeActions[bad.NudgeAction] {
		t.Errorf("NudgeAction %q should NOT be in the closed §9 enum", bad.NudgeAction)
	}

	// Assert version_drift record type also marshals correctly.
	driftRec := AuditRecord{
		RecordType:  "version_drift",
		RecordID:    "drift-001",
		Timestamp:   "2026-06-04T14:22:07Z",
		ScannerName: "beekeeper",
		Decision:    "allow",
		NudgeAction: "proceed",
		ReasonCode:  "not-applicable",
	}
	driftData, err := json.Marshal(driftRec)
	if err != nil {
		t.Fatalf("json.Marshal version_drift: %v", err)
	}
	if !strings.Contains(string(driftData), `"record_type":"version_drift"`) {
		t.Errorf("version_drift record type did not marshal correctly: %s", string(driftData))
	}
}

// TestFromDecisionMapsProvenance verifies that a Decision with CorroborationCount,
// SourcesAgreed, and CatalogMatches with Corroborated=true is fully mapped to an
// AuditRecord, and the JSON representation carries all CTLG-09 fields.
func TestFromDecisionMapsProvenance(t *testing.T) {
	tc := policy.ToolCall{
		AgentName: "test-agent",
		ToolName:  "Install",
		ToolInput: map[string]any{
			"ecosystem": "npm",
			"package":   "evil-pkg",
			"version":   "1.0.0",
		},
	}

	d := policy.Decision{
		Allow:  false,
		Level:  "block",
		Reason: "corroborated catalog match: bumblebee,osv",
		RuleIDs: []string{
			"bumblebee-catalog-match",
			"osv-catalog-match",
		},
		CatalogMatches: []policy.CatalogMatch{
			{
				CatalogSource:  "bumblebee",
				EntryID:        "bb-001",
				Ecosystem:      "npm",
				Package:        "evil-pkg",
				Version:        "1.0.0",
				Severity:       "critical",
				Signed:         false,
				Corroborated:   true,
				Dissented:      false,
				CatalogVersion: "bumblebee",
			},
			{
				CatalogSource:  "osv",
				EntryID:        "osv-001",
				Ecosystem:      "npm",
				Package:        "evil-pkg",
				Version:        "1.0.0",
				Severity:       "high",
				Signed:         true,
				Corroborated:   true,
				Dissented:      false,
				CatalogVersion: "osv-api",
			},
		},
		CorroborationCount: 2,
		SourcesAgreed:      []string{"bumblebee", "osv"},
		SourcesDissented:   nil,
		Quarantine:         false,
	}

	rec := FromDecision(tc, d, "test-record-id", "2026-05-26T00:00:00Z", policy.AgentContext{})

	// Verify corroboration fields on the AuditRecord.
	if rec.CorroborationCount != 2 {
		t.Errorf("CorroborationCount = %d, want 2", rec.CorroborationCount)
	}
	if len(rec.SourcesAgreed) != 2 {
		t.Errorf("SourcesAgreed = %v, want [bumblebee osv]", rec.SourcesAgreed)
	}
	if len(rec.SourcesDissented) != 0 {
		t.Errorf("SourcesDissented = %v, want empty", rec.SourcesDissented)
	}
	if rec.Quarantine {
		t.Error("Quarantine = true, want false")
	}

	// Verify per-match provenance fields are mapped.
	if len(rec.CatalogMatches) != 2 {
		t.Fatalf("CatalogMatches len = %d, want 2", len(rec.CatalogMatches))
	}
	if !rec.CatalogMatches[0].Corroborated {
		t.Error("CatalogMatches[0].Corroborated = false, want true")
	}
	if rec.CatalogMatches[1].CatalogVersion != "osv-api" {
		t.Errorf("CatalogMatches[1].CatalogVersion = %q, want osv-api", rec.CatalogMatches[1].CatalogVersion)
	}

	// Verify JSON shape contains all CTLG-09 fields.
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	jsonStr := string(data)

	for _, want := range []string{
		`"corroboration_count":2`,
		`"sources_agreed"`,
		`"sources_dissented"`,
		`"corroborated":true`,
		`"catalog_version":"osv-api"`,
	} {
		if !strings.Contains(jsonStr, want) {
			t.Errorf("JSON missing %q\nGot: %s", want, jsonStr)
		}
	}
}

// TestFromDecisionEmptyMatchesSerializesEmptyArray verifies that a Decision
// with no catalog matches produces `"catalog_matches":[]` not `"catalog_matches":null`
// per CTLG-09 (the field must always be present, even for non-catalog decisions).
func TestFromDecisionEmptyMatchesSerializesEmptyArray(t *testing.T) {
	tc := policy.ToolCall{AgentName: "agent", ToolName: "Bash"}
	d := policy.Decision{
		Allow:  true,
		Level:  "allow",
		Reason: "no package identified",
	}

	rec := FromDecision(tc, d, "rec-id", "2026-05-26T00:00:00Z", policy.AgentContext{})

	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	jsonStr := string(data)

	// Must serialize as [] not null.
	if !strings.Contains(jsonStr, `"catalog_matches":[]`) {
		t.Errorf("catalog_matches must serialize as [] for empty slice\nGot: %s", jsonStr)
	}
	// sources_agreed must also be [] not null.
	if !strings.Contains(jsonStr, `"sources_agreed":[]`) {
		t.Errorf("sources_agreed must serialize as [] when nil\nGot: %s", jsonStr)
	}
	// sources_dissented must also be [] not null.
	if !strings.Contains(jsonStr, `"sources_dissented":[]`) {
		t.Errorf("sources_dissented must serialize as [] when nil\nGot: %s", jsonStr)
	}
}

// TestFromDecisionQuarantineFlag verifies the Quarantine field propagates when
// three signed sources agree.
func TestFromDecisionQuarantineFlag(t *testing.T) {
	tc := policy.ToolCall{AgentName: "a", ToolName: "Install"}
	d := policy.Decision{
		Allow:              false,
		Level:              "block",
		CorroborationCount: 3,
		SourcesAgreed:      []string{"bumblebee", "osv", "socket"},
		Quarantine:         true,
	}

	rec := FromDecision(tc, d, "qid", "2026-05-26T00:00:00Z", policy.AgentContext{})

	if !rec.Quarantine {
		t.Error("Quarantine = false, want true")
	}
	data, _ := json.Marshal(rec)
	if !strings.Contains(string(data), `"quarantine":true`) {
		t.Errorf("JSON missing quarantine:true\nGot: %s", data)
	}
}

// TestFromDecisionPhase1FieldsIntact verifies that all Phase 1 AuditRecord
// fields are still correctly populated after the Phase 2 extension.
func TestFromDecisionPhase1FieldsIntact(t *testing.T) {
	tc := policy.ToolCall{
		AgentName: "my-agent",
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "npm install express"},
	}
	d := policy.Decision{
		Allow:   true,
		Level:   "allow",
		Reason:  "no catalog match",
		RuleIDs: []string{},
	}

	rec := FromDecision(tc, d, "record-123", "2026-05-26T12:00:00Z", policy.AgentContext{})

	if rec.RecordType != "policy_decision" {
		t.Errorf("RecordType = %q, want policy_decision", rec.RecordType)
	}
	if rec.RecordID != "record-123" {
		t.Errorf("RecordID = %q, want record-123", rec.RecordID)
	}
	if rec.ScannerName != "beekeeper" {
		t.Errorf("ScannerName = %q, want beekeeper", rec.ScannerName)
	}
	if rec.AgentName != "my-agent" {
		t.Errorf("AgentName = %q, want my-agent", rec.AgentName)
	}
	if rec.ToolName != "Bash" {
		t.Errorf("ToolName = %q, want Bash", rec.ToolName)
	}
	if rec.Decision != "allow" {
		t.Errorf("Decision = %q, want allow", rec.Decision)
	}
	if rec.Endpoint != "check" {
		t.Errorf("Endpoint = %q, want check", rec.Endpoint)
	}
}
