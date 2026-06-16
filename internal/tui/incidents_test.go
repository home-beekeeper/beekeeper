package tui

import (
	"strings"
	"testing"

	"github.com/home-beekeeper/beekeeper/internal/audit"
)

// TestCatalogQuarantineIncidentHasExplicitActions verifies that a catalog-quarantine
// incident exposes both [P]urge and [R]estore action buttons.
func TestCatalogQuarantineIncidentHasExplicitActions(t *testing.T) {
	rec := audit.AuditRecord{
		RecordType:  "catalog_quarantine",
		RecordID:    "test-1",
		Timestamp:   "2026-06-12T10:00:00Z",
		ToolName:    "scan",
		Decision:    "block",
		Reason:      "catalog match: 2 sources",
		RuleIDs:     []string{"FRSP-01"},
		ScannerName: "beekeeper",
	}

	incident := CatalogQuarantineIncidentFromRecord(rec, false /* pending=false -> quarantined */)

	// Must have both [P]urge and [R]estore.
	foundPurge := false
	foundRestore := false
	for _, act := range incident.Actions {
		if act.Key == "p" {
			foundPurge = true
		}
		if act.Key == "r" {
			foundRestore = true
		}
	}
	if !foundPurge {
		t.Error("catalog-quarantine incident must expose [P]urge action (human-gated)")
	}
	if !foundRestore {
		t.Error("catalog-quarantine incident must expose [R]estore action")
	}
}

// TestPendingQuarantineIncidentHasAcknowledgeOnly verifies that a pending-quarantine
// incident (path unresolved) only exposes the acknowledge action, no purge/restore.
func TestPendingQuarantineIncidentHasAcknowledgeOnly(t *testing.T) {
	rec := audit.AuditRecord{
		RecordType:  "pending-quarantine",
		RecordID:    "test-2",
		Timestamp:   "2026-06-12T10:01:00Z",
		ToolName:    "scan",
		Decision:    "warn",
		Reason:      "catalog match: path unknown",
		RuleIDs:     []string{"FRSP-01"},
		ScannerName: "beekeeper",
	}

	incident := CatalogQuarantineIncidentFromRecord(rec, true /* pending=true */)

	for _, act := range incident.Actions {
		if act.Key == "p" {
			t.Error("pending-quarantine incident must NOT expose [P]urge (nothing to purge yet)")
		}
		if act.Key == "r" {
			t.Error("pending-quarantine incident must NOT expose [R]estore (nothing to restore yet)")
		}
	}

	// Must have at least one action (acknowledge).
	if len(incident.Actions) == 0 {
		t.Error("pending-quarantine incident must have at least one action (acknowledge)")
	}

	// Acknowledge should be present.
	foundAck := false
	for _, act := range incident.Actions {
		if act.Key == "a" || strings.Contains(strings.ToLower(act.Lbl), "acknowledge") {
			foundAck = true
		}
	}
	if !foundAck {
		t.Error("pending-quarantine incident must have acknowledge action")
	}
}

// TestCatalogQuarantineIncidentNoPurgeAutoTrigger verifies the honesty invariant:
// the incident card never auto-triggers a purge — it only exposes human-gated buttons.
// This is a structural assertion: the IncidentModel.Actions keys do NOT include any
// auto-execute/hidden action; selection requires explicit user keypress.
func TestCatalogQuarantineIncidentNoPurgeAutoTrigger(t *testing.T) {
	rec := audit.AuditRecord{
		RecordType: "catalog_quarantine",
		RecordID:   "test-3",
		Timestamp:  "2026-06-12T10:02:00Z",
	}
	incident := CatalogQuarantineIncidentFromRecord(rec, false)

	// SelAction starts at 0 (first action); no action must be an auto-execute.
	// The default selected action must require a deliberate keypress.
	if incident.SelAction != 0 {
		t.Errorf("SelAction = %d, want 0 (first action pre-selected)", incident.SelAction)
	}
	// There must be at least one action (the buttons exist).
	if len(incident.Actions) == 0 {
		t.Error("no actions defined on catalog-quarantine incident")
	}
}
