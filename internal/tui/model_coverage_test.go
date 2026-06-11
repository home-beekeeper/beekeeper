package tui

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bantuson/beekeeper/internal/audit"
)

// writeAuditNDJSON writes records as NDJSON to a temp file and returns the path.
func writeAuditNDJSON(t *testing.T, recs []audit.AuditRecord) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "beekeeper.ndjson")
	var buf bytes.Buffer
	for _, r := range recs {
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("marshal audit record: %v", err)
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write audit file: %v", err)
	}
	return p
}

// TestComputeStatusEmpty asserts an empty/unreadable audit log yields an honest
// "monitoring" line rather than fabricated counts.
func TestComputeStatusEmpty(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.ndjson")
	if got := computeStatus(missing); got != "monitoring · press : to act" {
		t.Errorf("computeStatus(missing) = %q, want the monitoring line", got)
	}
}

// TestComputeStatusCounts asserts computeStatus counts today's distinct agents,
// block decisions, and critical sentry alerts — and skips records that are not
// from today or whose timestamp does not parse.
func TestComputeStatusCounts(t *testing.T) {
	today := time.Now().Format(time.RFC3339)
	twoDaysAgo := time.Now().Add(-48 * time.Hour).Format(time.RFC3339)
	recs := []audit.AuditRecord{
		{RecordType: "policy_decision", Timestamp: today, AgentName: "claude", Decision: "block"},
		{RecordType: "policy_decision", Timestamp: today, AgentName: "cursor", Decision: "allow"},
		{RecordType: "sentry_alert", Timestamp: today, AgentName: "claude", SentrySeverity: "critical"},
		{RecordType: "policy_decision", Timestamp: twoDaysAgo, AgentName: "stale", Decision: "block"}, // skipped: not today
		{RecordType: "policy_decision", Timestamp: "not-a-timestamp", AgentName: "bad", Decision: "block"}, // skipped: parse error
	}
	got := computeStatus(writeAuditNDJSON(t, recs))
	// distinct agents today = {claude, cursor} = 2; blocks today = 1; criticals today = 1
	for _, want := range []string{"2 agents today", "1 block today", "1 critical today"} {
		if !strings.Contains(got, want) {
			t.Errorf("computeStatus = %q, missing %q", got, want)
		}
	}
	if strings.Contains(got, "stale") || strings.Contains(got, "monitoring") {
		t.Errorf("computeStatus = %q, should not include skipped/empty markers", got)
	}
}

// TestRecentAuditRecordsBoundedTail asserts recentAuditRecords reads only the
// bounded tail of the log (the last ~512KB), not the whole file — the v1.3.0
// de-mock fix that stopped the dashboard re-parsing tens of MB every tick.
func TestRecentAuditRecordsBoundedTail(t *testing.T) {
	p := filepath.Join(t.TempDir(), "beekeeper.ndjson")
	now := time.Now().Format(time.RFC3339)
	earlyLine, _ := json.Marshal(audit.AuditRecord{RecordType: "policy_decision", Timestamp: now, AgentName: "EARLY", Decision: "allow"})
	earlyLine = append(earlyLine, '\n')

	var buf bytes.Buffer
	for buf.Len() < 600*1024 { // exceed the 512KB scan window
		buf.Write(earlyLine)
	}
	totalEarly := buf.Len() / len(earlyLine)
	tailLine, _ := json.Marshal(audit.AuditRecord{RecordType: "policy_decision", Timestamp: now, AgentName: "TAIL", Decision: "block"})
	buf.Write(append(tailLine, '\n'))
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	recs := recentAuditRecords(p)
	foundTail := false
	for _, r := range recs {
		if r.AgentName == "TAIL" {
			foundTail = true
		}
	}
	if !foundTail {
		t.Error("recentAuditRecords should include the final (TAIL) record")
	}
	if len(recs) >= totalEarly {
		t.Errorf("recentAuditRecords read %d records but %d were written before the tail — not bounded to the last 512KB", len(recs), totalEarly)
	}
}
