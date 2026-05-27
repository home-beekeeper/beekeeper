//go:build linux

package linux

import (
	"testing"
	"time"

	"github.com/mzansi-agentive/beekeeper/internal/audit"
	"github.com/mzansi-agentive/beekeeper/internal/sentry"
)

func TestAlertToAuditRecord(t *testing.T) {
	alert := sentry.SentryAlert{
		RuleID:        "SENTRY-001",
		RuleName:      "Credential File Access",
		Severity:      "critical",
		BaselineMode:  false,
		QuarantineRec: true,
		ProcessPID:    1234,
		ProcessExe:    "/usr/bin/cursor",
		ParentChain:   []string{"/usr/bin/cursor"},
		Timestamp:     time.Now(),
	}
	rec := alertToAuditRecord(alert)
	if rec.RecordType != "sentry_alert" {
		t.Errorf("RecordType: got %q, want %q", rec.RecordType, "sentry_alert")
	}
	if rec.SentryRuleID != "SENTRY-001" {
		t.Errorf("SentryRuleID: got %q, want %q", rec.SentryRuleID, "SENTRY-001")
	}
	if rec.SentryQuarantineRec != true {
		t.Errorf("SentryQuarantineRec: got %v, want true", rec.SentryQuarantineRec)
	}
	if rec.CatalogMatches == nil {
		t.Error("CatalogMatches must be non-nil empty slice")
	}
	if rec.Decision != "block" {
		t.Errorf("Decision: got %q, want %q", rec.Decision, "block")
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

func TestAuditRecordHasSentryFields(t *testing.T) {
	// Compile-time verification that AuditRecord has sentry fields.
	var rec audit.AuditRecord
	rec.SentryRuleID = "SENTRY-001"
	rec.SentryRuleName = "test"
	rec.SentryProcessPID = 1
	_ = rec
}
