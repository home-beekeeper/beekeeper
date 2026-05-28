package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestExportNDJSON verifies that Format="ndjson" produces identical output to Query.
func TestExportNDJSON(t *testing.T) {
	ts := time.Now().UTC().Format(time.RFC3339)
	lines := strings.Join([]string{
		makeNDJSON("allow", "agent1", "bash", ts),
		makeNDJSON("block", "agent1", "bash", ts),
	}, "\n")

	var queryBuf, exportBuf bytes.Buffer

	if err := Query(context.Background(), strings.NewReader(lines), QueryOpts{}, &queryBuf); err != nil {
		t.Fatalf("Query: %v", err)
	}
	if err := Export(context.Background(), strings.NewReader(lines), ExportOpts{Format: "ndjson"}, &exportBuf); err != nil {
		t.Fatalf("Export ndjson: %v", err)
	}

	if queryBuf.String() != exportBuf.String() {
		t.Errorf("Export ndjson != Query output\nQuery:  %q\nExport: %q", queryBuf.String(), exportBuf.String())
	}
}

// TestExportCSV verifies that Format="csv" produces a header row followed by
// one data row per record, with rule_ids pipe-joined.
func TestExportCSV(t *testing.T) {
	ts := time.Now().UTC().Format(time.RFC3339)
	// Build a record with rule_ids to check pipe-joining.
	lineWithRules := `{"record_type":"policy_decision","record_id":"r2","timestamp":"` + ts + `","scanner_name":"beekeeper","agent_name":"agent1","tool_name":"npm","decision":"block","reason":"test","rule_ids":["RULE-01","RULE-02"],"catalog_matches":[],"endpoint":"check","corroboration_count":0,"sources_agreed":[],"sources_dissented":[]}`
	input := makeNDJSON("allow", "agent2", "bash", ts) + "\n" + lineWithRules

	var buf bytes.Buffer
	if err := Export(context.Background(), strings.NewReader(input), ExportOpts{Format: "csv"}, &buf); err != nil {
		t.Fatalf("Export csv: %v", err)
	}

	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")

	// Header + 2 data rows = 3 lines minimum
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 CSV lines (header + 2 rows), got %d:\n%s", len(lines), out)
	}

	// First line must be the header
	if !strings.HasPrefix(lines[0], "record_type,") {
		t.Errorf("first line is not CSV header: %q", lines[0])
	}

	// Rule IDs must be pipe-joined in the row that has rules
	found := false
	for _, l := range lines[1:] {
		if strings.Contains(l, "RULE-01|RULE-02") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected rule_ids pipe-joined as RULE-01|RULE-02 in CSV output:\n%s", out)
	}
}

// TestExportOTLP verifies that Format="otlp" produces JSON with a "resourceLogs"
// key and each record appears as a logRecord with body.stringValue set to the
// original JSON line.
func TestExportOTLP(t *testing.T) {
	ts := time.Now().UTC().Format(time.RFC3339)
	line1 := makeNDJSON("allow", "agent1", "bash", ts)
	line2 := makeNDJSON("block", "agent2", "npm", ts)
	input := line1 + "\n" + line2

	var buf bytes.Buffer
	if err := Export(context.Background(), strings.NewReader(input), ExportOpts{Format: "otlp"}, &buf); err != nil {
		t.Fatalf("Export otlp: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal OTLP output: %v\noutput: %s", err, buf.String())
	}

	if _, ok := payload["resourceLogs"]; !ok {
		t.Fatalf("expected 'resourceLogs' key in OTLP output, got keys: %v", keys(payload))
	}

	// Navigate to logRecords and verify body.stringValue is the original raw line.
	rls, _ := payload["resourceLogs"].([]any)
	if len(rls) == 0 {
		t.Fatal("resourceLogs is empty")
	}
	rl := rls[0].(map[string]any)
	scopeLogs, _ := rl["scopeLogs"].([]any)
	if len(scopeLogs) == 0 {
		t.Fatal("scopeLogs is empty")
	}
	sl := scopeLogs[0].(map[string]any)
	lrs, _ := sl["logRecords"].([]any)
	if len(lrs) != 2 {
		t.Fatalf("expected 2 logRecords, got %d", len(lrs))
	}

	// Check first logRecord body.stringValue matches line1.
	lr0 := lrs[0].(map[string]any)
	body, _ := lr0["body"].(map[string]any)
	sv, _ := body["stringValue"].(string)
	if sv != line1 {
		t.Errorf("logRecord[0].body.stringValue mismatch\nwant: %q\ngot:  %q", line1, sv)
	}
}

// keys returns sorted map key slice for debug messages.
func keys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
