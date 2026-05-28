//go:build windows

package windows

import (
	"testing"
	"time"

	"github.com/mzansi-agentive/beekeeper/internal/audit"
	"github.com/mzansi-agentive/beekeeper/internal/sentry"
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

func TestRunDaemonBodyRequiresAdmin(t *testing.T) {
	t.Skip("TestRunDaemonBodyRequiresAdmin: skipped — real ETW session creation requires admin privileges; covered by Windows CI in 07-05")
	_ = audit.AuditRecord{} // ensure audit import is used if test is un-skipped
}
