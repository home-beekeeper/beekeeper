//go:build darwin

package darwin

import (
	"testing"
	"time"

	"github.com/mzansi-agentive/beekeeper/internal/sentry"
)

func TestAlertToAuditRecordEnforcement(t *testing.T) {
	alert := sentry.SentryAlert{
		RuleID:        "SENTRY-001",
		RuleName:      "Credential File Access Cluster",
		Severity:      "critical",
		BaselineMode:  false,
		QuarantineRec: true,
		ProcessPID:    1234,
		ProcessExe:    "/Applications/Cursor.app/Contents/MacOS/Cursor",
		ParentChain:   []string{"/Applications/Cursor.app/Contents/MacOS/Cursor"},
		Timestamp:     time.Now(),
	}
	rec := alertToAuditRecord(alert)

	if rec.RecordType != "sentry_alert" {
		t.Errorf("RecordType: got %q, want %q", rec.RecordType, "sentry_alert")
	}
	if rec.Decision != "block" {
		t.Errorf("Decision: got %q, want %q", rec.Decision, "block")
	}
	if rec.SentryRuleID != "SENTRY-001" {
		t.Errorf("SentryRuleID: got %q, want %q", rec.SentryRuleID, "SENTRY-001")
	}
	if rec.SentryQuarantineRec != true {
		t.Errorf("SentryQuarantineRec: got %v, want true", rec.SentryQuarantineRec)
	}
	if rec.CatalogMatches == nil {
		t.Error("CatalogMatches must be non-nil empty slice (CTLG-09)")
	}
	if len(rec.CatalogMatches) != 0 {
		t.Errorf("CatalogMatches: got %d entries, want 0", len(rec.CatalogMatches))
	}
}

func TestAlertToAuditRecordBaseline(t *testing.T) {
	alert := sentry.SentryAlert{
		RuleID:        "SENTRY-001",
		Severity:      "critical",
		BaselineMode:  true,
		QuarantineRec: false,
		Timestamp:     time.Now(),
	}
	rec := alertToAuditRecord(alert)

	if rec.RecordType != "sentry_alert_baseline" {
		t.Errorf("RecordType: got %q, want %q", rec.RecordType, "sentry_alert_baseline")
	}
	if rec.Decision != "warn" {
		t.Errorf("Decision: got %q, want %q", rec.Decision, "warn")
	}
	if rec.SentryBaselineMode != true {
		t.Errorf("SentryBaselineMode: want true, got %v", rec.SentryBaselineMode)
	}
}

func TestAlertToAuditRecordPreservesParentChain(t *testing.T) {
	chain := []string{"/usr/local/bin/node", "/Applications/Cursor.app/Contents/MacOS/Cursor"}
	alert := sentry.SentryAlert{
		RuleID:        "SENTRY-002",
		RuleName:      "Credential CLI Spawn Cluster",
		Severity:      "critical",
		BaselineMode:  false,
		QuarantineRec: true,
		ProcessPID:    5678,
		ProcessExe:    "/usr/local/bin/gh",
		ParentChain:   chain,
		Timestamp:     time.Now(),
	}
	rec := alertToAuditRecord(alert)

	if len(rec.SentryParentChain) != len(chain) {
		t.Errorf("SentryParentChain length: got %d, want %d", len(rec.SentryParentChain), len(chain))
	}
	for i, c := range chain {
		if rec.SentryParentChain[i] != c {
			t.Errorf("SentryParentChain[%d]: got %q, want %q", i, rec.SentryParentChain[i], c)
		}
	}
}

func TestDaemonStateInitialRules(t *testing.T) {
	state := &daemonState{
		ruleStates: map[string]bool{
			"SENTRY-001": true,
			"SENTRY-002": true,
			"SENTRY-003": true,
			"SENTRY-004": true,
			"SENTRY-005": true,
		},
		startedAt: time.Now().UTC(),
	}

	if len(state.ruleStates) != 5 {
		t.Errorf("expected 5 initial rules, got %d", len(state.ruleStates))
	}
	for id, enabled := range state.ruleStates {
		if !enabled {
			t.Errorf("rule %s: expected enabled=true, got false", id)
		}
	}
}
