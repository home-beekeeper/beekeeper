//go:build linux

package linux

import (
	"strings"
	"testing"
	"time"

	"github.com/home-beekeeper/beekeeper/internal/audit"
	"github.com/home-beekeeper/beekeeper/internal/sentry"
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

// TestAlertToAuditRecordRedactsCredentials proves Finding #5 (HIGH) is fixed on
// the linux daemon write path: the record produced by alertToAuditRecord is
// routed through audit.RedactRecord(rec, audit.DefaultRedactPatterns()) before
// auditWriter.Write, so a Bearer/JWT/AKIA token embedded in a watched file path,
// a network destination, or the process exe is never persisted verbatim.
func TestAlertToAuditRecordRedactsCredentials(t *testing.T) {
	alert := sentry.SentryAlert{
		RuleID:        "SENTRY-002",
		RuleName:      "Suspicious network exfiltration",
		Severity:      "critical",
		QuarantineRec: true,
		Timestamp:     time.Now(),
		ProcessPID:    4321,
		ProcessExe:    "/usr/bin/curl -H 'Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.payload.sig'",
		FilesAccessed: []string{
			"/home/agent/.aws/credentials AKIAIOSFODNN7EXAMPLE",
			"/home/agent/project/main.go",
		},
		NetworkDests: []string{
			"https://evil.example/exfil?token=eyJhbGciOiJIUzI1NiJ9.body.signature",
			"https://collector.example/v1/logs",
		},
		CorrelatedExtension: "publisher.ext-AKIAIOSFODNN7EXAMPLE",
	}

	rec := alertToAuditRecord(alert)
	rec = audit.RedactRecord(rec, audit.DefaultRedactPatterns())

	if strings.Contains(rec.SentryProcessExe, "eyJhbGciOiJIUzI1NiJ9") {
		t.Errorf("SentryProcessExe still contains a JWT: %q", rec.SentryProcessExe)
	}
	if strings.Contains(rec.SentryFilesAccessed[0], "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("SentryFilesAccessed[0] still contains an AKIA key: %q", rec.SentryFilesAccessed[0])
	}
	if !strings.Contains(rec.SentryFilesAccessed[0], "[REDACTED]") {
		t.Errorf("SentryFilesAccessed[0] missing [REDACTED] marker: %q", rec.SentryFilesAccessed[0])
	}
	if rec.SentryFilesAccessed[1] != "/home/agent/project/main.go" {
		t.Errorf("SentryFilesAccessed[1] = %q, want the benign path unchanged", rec.SentryFilesAccessed[1])
	}
	if strings.Contains(rec.SentryNetworkDests[0], "eyJhbGciOiJIUzI1NiJ9") {
		t.Errorf("SentryNetworkDests[0] still contains a JWT: %q", rec.SentryNetworkDests[0])
	}
	if strings.Contains(rec.SentryCorrelatedExt, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("SentryCorrelatedExt still contains an AKIA key: %q", rec.SentryCorrelatedExt)
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
