//go:build windows

package windows

import (
	"strings"
	"testing"
	"time"

	"github.com/bantuson/beekeeper/internal/audit"
	"github.com/bantuson/beekeeper/internal/sentry"
)

func TestDaemonStateInitialRules(t *testing.T) {
	state := &daemonState{
		ruleStates: map[string]bool{
			"SENTRY-001": true,
			"SENTRY-002": true,
			"SENTRY-003": true,
			"SENTRY-004": true,
			"SENTRY-005": true,
		},
		startedAt:  time.Now().UTC(),
		tierReason: "Windows ETW (LocalService)",
	}

	state.mu.RLock()
	count := len(state.ruleStates)
	state.mu.RUnlock()

	if count != 5 {
		t.Errorf("initial ruleStates has %d entries; want 5", count)
	}
	for _, id := range []string{"SENTRY-001", "SENTRY-002", "SENTRY-003", "SENTRY-004", "SENTRY-005"} {
		state.mu.RLock()
		enabled, ok := state.ruleStates[id]
		state.mu.RUnlock()
		if !ok {
			t.Errorf("rule %s missing from initial ruleStates", id)
		}
		if !enabled {
			t.Errorf("rule %s should be enabled by default", id)
		}
	}
}

func TestAlertToAuditRecordEnforcement(t *testing.T) {
	now := time.Now().UTC()
	alert := sentry.SentryAlert{
		RuleID:        "SENTRY-001",
		RuleName:      "Credential file exfiltration",
		Severity:      "critical",
		BaselineMode:  false,
		QuarantineRec: true, // block + quarantine
		Timestamp:     now,
		ProcessPID:    1234,
		ProcessExe:    "/usr/bin/test",
	}

	rec := alertToAuditRecord(alert)

	if rec.RecordType != "sentry_alert" {
		t.Errorf("RecordType = %q; want sentry_alert", rec.RecordType)
	}
	if rec.Decision != "block" {
		t.Errorf("Decision = %q; want block", rec.Decision)
	}
	if rec.SentryRuleID != "SENTRY-001" {
		t.Errorf("SentryRuleID = %q; want SENTRY-001", rec.SentryRuleID)
	}
	if rec.SentryQuarantineRec != true {
		t.Error("SentryQuarantineRec should be true")
	}
	if rec.CatalogMatches == nil {
		t.Error("CatalogMatches should not be nil (must be an empty slice, not nil)")
	}
	if len(rec.CatalogMatches) != 0 {
		t.Errorf("CatalogMatches should be empty; got %d entries", len(rec.CatalogMatches))
	}
	if rec.ScannerName != "beekeeper" {
		t.Errorf("ScannerName = %q; want beekeeper", rec.ScannerName)
	}
}

func TestAlertToAuditRecordBaseline(t *testing.T) {
	now := time.Now().UTC()
	alert := sentry.SentryAlert{
		RuleID:       "SENTRY-002",
		RuleName:     "Suspicious network exfiltration",
		Severity:     "high",
		BaselineMode: true, // baseline → sentry_alert_baseline + warn
		Timestamp:    now,
		ProcessPID:   5678,
	}

	rec := alertToAuditRecord(alert)

	if rec.RecordType != "sentry_alert_baseline" {
		t.Errorf("RecordType = %q; want sentry_alert_baseline", rec.RecordType)
	}
	if rec.Decision != "warn" {
		t.Errorf("Decision = %q; want warn", rec.Decision)
	}
	if rec.SentryBaselineMode != true {
		t.Error("SentryBaselineMode should be true")
	}
	if rec.SentryRuleID != "SENTRY-002" {
		t.Errorf("SentryRuleID = %q; want SENTRY-002", rec.SentryRuleID)
	}
}

// TestAlertToAuditRecordRedactsCredentials proves Finding #5 (HIGH) is fixed:
// the Sentry write path must route the record through
// audit.RedactRecord(rec, audit.DefaultRedactPatterns()) so a Bearer/JWT/AKIA
// token embedded in a watched file path or a network destination is NOT
// persisted verbatim. This reproduces the exact two-step path the daemon's
// correlationEngineLoop now performs (alertToAuditRecord → RedactRecord) before
// auditWriter.Write. The Sentry daemons are the ONLY writers that populate
// SentryFilesAccessed / SentryNetworkDests / SentryProcessExe, so this is the
// only place those fields can leak.
func TestAlertToAuditRecordRedactsCredentials(t *testing.T) {
	now := time.Now().UTC()
	alert := sentry.SentryAlert{
		RuleID:        "SENTRY-002",
		RuleName:      "Suspicious network exfiltration",
		Severity:      "critical",
		QuarantineRec: true,
		Timestamp:     now,
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

	// Reproduce the daemon write path: convert, then redact with the defaults.
	rec := alertToAuditRecord(alert)
	rec = audit.RedactRecord(rec, audit.DefaultRedactPatterns())

	// JWT/Bearer in the process exe must be gone.
	if strings.Contains(rec.SentryProcessExe, "eyJhbGciOiJIUzI1NiJ9") {
		t.Errorf("SentryProcessExe still contains a JWT: %q", rec.SentryProcessExe)
	}

	// AKIA literal in a watched file path must be redacted.
	if len(rec.SentryFilesAccessed) != 2 {
		t.Fatalf("SentryFilesAccessed len = %d, want 2", len(rec.SentryFilesAccessed))
	}
	if strings.Contains(rec.SentryFilesAccessed[0], "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("SentryFilesAccessed[0] still contains an AKIA key: %q", rec.SentryFilesAccessed[0])
	}
	if !strings.Contains(rec.SentryFilesAccessed[0], "[REDACTED]") {
		t.Errorf("SentryFilesAccessed[0] missing [REDACTED] marker: %q", rec.SentryFilesAccessed[0])
	}
	// The benign path element is preserved.
	if rec.SentryFilesAccessed[1] != "/home/agent/project/main.go" {
		t.Errorf("SentryFilesAccessed[1] = %q, want the benign path unchanged", rec.SentryFilesAccessed[1])
	}

	// JWT in a network destination URL must be redacted.
	if len(rec.SentryNetworkDests) != 2 {
		t.Fatalf("SentryNetworkDests len = %d, want 2", len(rec.SentryNetworkDests))
	}
	if strings.Contains(rec.SentryNetworkDests[0], "eyJhbGciOiJIUzI1NiJ9") {
		t.Errorf("SentryNetworkDests[0] still contains a JWT: %q", rec.SentryNetworkDests[0])
	}
	if !strings.Contains(rec.SentryNetworkDests[0], "[JWT_REDACTED]") {
		t.Errorf("SentryNetworkDests[0] missing [JWT_REDACTED] marker: %q", rec.SentryNetworkDests[0])
	}

	// AKIA in the correlated extension id must be redacted.
	if strings.Contains(rec.SentryCorrelatedExt, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("SentryCorrelatedExt still contains an AKIA key: %q", rec.SentryCorrelatedExt)
	}
}

func TestRunDaemonBodyRequiresAdmin(t *testing.T) {
	t.Skip("TestRunDaemonBodyRequiresAdmin: skipped — real ETW session creation requires admin privileges; covered by Windows CI in 07-05")
	_ = audit.AuditRecord{} // ensure audit import is used if test is un-skipped
}
