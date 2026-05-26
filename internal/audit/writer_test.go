package audit

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mzansi-agentive/beekeeper/internal/policy"
)

// --- Task 1: FromDecision mapping tests ---

func TestFromDecisionAllow(t *testing.T) {
	tc := policy.ToolCall{AgentName: "claude-code", ToolName: "Bash"}
	d := policy.Decision{
		Allow:  true,
		Level:  "allow",
		Reason: "no catalog match",
	}

	rec := FromDecision(tc, d, "rec-1", "2026-05-26T12:00:00Z")

	if rec.RecordType != "policy_decision" {
		t.Errorf("RecordType = %q, want %q", rec.RecordType, "policy_decision")
	}
	if rec.ScannerName != "beekeeper" {
		t.Errorf("ScannerName = %q, want %q", rec.ScannerName, "beekeeper")
	}
	if rec.Decision != "allow" {
		t.Errorf("Decision = %q, want %q", rec.Decision, "allow")
	}
	if rec.Endpoint != "check" {
		t.Errorf("Endpoint = %q, want %q", rec.Endpoint, "check")
	}
	if rec.RecordID != "rec-1" {
		t.Errorf("RecordID = %q, want %q", rec.RecordID, "rec-1")
	}
	if rec.Timestamp != "2026-05-26T12:00:00Z" {
		t.Errorf("Timestamp = %q, want %q", rec.Timestamp, "2026-05-26T12:00:00Z")
	}
	if rec.AgentName != "claude-code" {
		t.Errorf("AgentName = %q, want %q", rec.AgentName, "claude-code")
	}
	if rec.ToolName != "Bash" {
		t.Errorf("ToolName = %q, want %q", rec.ToolName, "Bash")
	}
	if len(rec.CatalogMatches) != 0 {
		t.Errorf("CatalogMatches len = %d, want 0", len(rec.CatalogMatches))
	}
}

func TestFromDecisionWarnWithMatch(t *testing.T) {
	tc := policy.ToolCall{AgentName: "cursor", ToolName: "install_extension"}
	d := policy.Decision{
		Allow:   true,
		Level:   "warn",
		Reason:  "bumblebee catalog match: nx-console",
		RuleIDs: []string{"bumblebee-catalog-match"},
		CatalogMatches: []policy.CatalogMatch{
			{
				CatalogSource: "bumblebee",
				EntryID:       "stepsecurity-2026-05-18-vscode-nrwl-angular-console-compromised",
				Ecosystem:     "editor-extension",
				Package:       "nrwl.angular-console",
				Version:       "18.95.0",
				Severity:      "critical",
				Signed:        true,
			},
		},
	}

	rec := FromDecision(tc, d, "rec-2", "2026-05-26T12:01:00Z")

	if rec.Decision != "warn" {
		t.Errorf("Decision = %q, want %q", rec.Decision, "warn")
	}
	if len(rec.RuleIDs) != 1 || rec.RuleIDs[0] != "bumblebee-catalog-match" {
		t.Errorf("RuleIDs = %v, want [bumblebee-catalog-match]", rec.RuleIDs)
	}
	if len(rec.CatalogMatches) != 1 {
		t.Fatalf("CatalogMatches len = %d, want 1", len(rec.CatalogMatches))
	}

	m := rec.CatalogMatches[0]
	if m.CatalogSource != "bumblebee" {
		t.Errorf("CatalogSource = %q, want %q", m.CatalogSource, "bumblebee")
	}
	if m.EntryID != "stepsecurity-2026-05-18-vscode-nrwl-angular-console-compromised" {
		t.Errorf("EntryID = %q, unexpected", m.EntryID)
	}
	if m.Ecosystem != "editor-extension" {
		t.Errorf("Ecosystem = %q, want %q", m.Ecosystem, "editor-extension")
	}
	if m.Package != "nrwl.angular-console" {
		t.Errorf("Package = %q, want %q", m.Package, "nrwl.angular-console")
	}
	if m.Version != "18.95.0" {
		t.Errorf("Version = %q, want %q", m.Version, "18.95.0")
	}
	if m.Severity != "critical" {
		t.Errorf("Severity = %q, want %q", m.Severity, "critical")
	}
	if !m.Signed {
		t.Errorf("Signed = %v, want true (CTLG-07 provenance must pass through)", m.Signed)
	}
}

func TestAuditRecordJSONKeys(t *testing.T) {
	rec := FromDecision(
		policy.ToolCall{AgentName: "a", ToolName: "t"},
		policy.Decision{Level: "allow", Reason: "r"},
		"id",
		"2026-05-26T12:00:00Z",
	)

	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var generic map[string]json.RawMessage
	if err := json.Unmarshal(data, &generic); err != nil {
		t.Fatalf("Unmarshal to map: %v", err)
	}

	wantKeys := []string{
		"record_type", "record_id", "timestamp", "scanner_name",
		"agent_name", "tool_name", "decision", "reason",
		"rule_ids", "catalog_matches", "endpoint",
	}
	if len(wantKeys) != 11 {
		t.Fatalf("test misconfigured: expected 11 keys, listed %d", len(wantKeys))
	}
	for _, k := range wantKeys {
		if _, ok := generic[k]; !ok {
			t.Errorf("marshalled AuditRecord missing JSON key %q; got %s", k, data)
		}
	}
	if len(generic) != 11 {
		t.Errorf("marshalled AuditRecord has %d keys, want 11: %s", len(generic), data)
	}
}

// --- Task 2: writer tests ---

func TestNDJSONWriteAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit", "beekeeper.ndjson")

	w, err := NewWriter(path)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	rec1 := FromDecision(
		policy.ToolCall{AgentName: "a1", ToolName: "t1"},
		policy.Decision{Level: "allow", Reason: "r1"},
		"id-1", "2026-05-26T12:00:00Z",
	)
	rec2 := FromDecision(
		policy.ToolCall{AgentName: "a2", ToolName: "t2"},
		policy.Decision{Level: "warn", Reason: "r2"},
		"id-2", "2026-05-26T12:01:00Z",
	)

	if err := w.Write(rec1); err != nil {
		t.Fatalf("Write rec1: %v", err)
	}
	if err := w.Write(rec2); err != nil {
		t.Fatalf("Write rec2: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var lines [][]byte
	sc := bufio.NewScanner(bytes.NewReader(raw))
	for sc.Scan() {
		lines = append(lines, append([]byte(nil), sc.Bytes()...))
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("got %d NDJSON lines, want 2", len(lines))
	}

	var got1, got2 AuditRecord
	if err := json.Unmarshal(lines[0], &got1); err != nil {
		t.Fatalf("unmarshal line 1: %v", err)
	}
	if err := json.Unmarshal(lines[1], &got2); err != nil {
		t.Fatalf("unmarshal line 2: %v", err)
	}
	if got1.RecordID != "id-1" || got1.AgentName != "a1" {
		t.Errorf("line 1 = %+v, want RecordID id-1 / AgentName a1", got1)
	}
	if got2.RecordID != "id-2" || got2.Decision != "warn" {
		t.Errorf("line 2 = %+v, want RecordID id-2 / Decision warn", got2)
	}
}

func TestPermissionsEnforced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit", "beekeeper.ndjson")

	w, err := NewWriter(path)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()

	if runtime.GOOS == "windows" {
		// DACL byte assertion is out of scope; assert the calls succeed (the
		// hectane/go-acl path applied an owner-only DACL without error).
		if err := w.Write(AuditRecord{}); err != nil {
			t.Fatalf("Write returned error on Windows: %v", err)
		}
		return
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat after NewWriter: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0600 {
		t.Errorf("after NewWriter perm = %o, want 0600", perm)
	}

	if err := w.Write(AuditRecord{}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	fi, err = os.Stat(path)
	if err != nil {
		t.Fatalf("Stat after Write: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0600 {
		t.Errorf("after Write perm = %o, want 0600", perm)
	}
}

func TestWriteReappliesPermsAfterWrite(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based perm assertion is Unix-only; Windows DACL is out of scope")
	}

	path := filepath.Join(t.TempDir(), "audit", "beekeeper.ndjson")

	w, err := NewWriter(path)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()

	if err := w.Write(AuditRecord{RecordID: "first"}); err != nil {
		t.Fatalf("first Write: %v", err)
	}

	// Simulate an external actor loosening the permissions.
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatalf("chmod 0644: %v", err)
	}

	if err := w.Write(AuditRecord{RecordID: "second"}); err != nil {
		t.Fatalf("second Write: %v", err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0600 {
		t.Errorf("after Write following external chmod, perm = %o, want 0600 (Pitfall 5 re-application)", perm)
	}
}
