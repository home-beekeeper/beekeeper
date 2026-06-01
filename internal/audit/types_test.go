package audit

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/bantuson/beekeeper/internal/policy"
)

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
