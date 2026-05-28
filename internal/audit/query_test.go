package audit

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

// makeNDJSON builds a minimal NDJSON line for the given decision, agent, tool, and timestamp.
func makeNDJSON(decision, agent, tool, ts string) string {
	return `{"record_type":"policy_decision","record_id":"r1","timestamp":"` + ts + `","scanner_name":"beekeeper","agent_name":"` + agent + `","tool_name":"` + tool + `","decision":"` + decision + `","reason":"test","rule_ids":[],"catalog_matches":[],"endpoint":"check","corroboration_count":0,"sources_agreed":[],"sources_dissented":[]}`
}

// TestQueryFilterDecision verifies that only records matching the Decision
// filter are emitted.
func TestQueryFilterDecision(t *testing.T) {
	ts := time.Now().UTC().Format(time.RFC3339)
	lines := strings.Join([]string{
		makeNDJSON("allow", "agent1", "bash", ts),
		makeNDJSON("block", "agent1", "bash", ts),
		makeNDJSON("block", "agent1", "bash", ts),
	}, "\n")

	var buf bytes.Buffer
	err := Query(context.Background(), strings.NewReader(lines), QueryOpts{Decision: "block"}, &buf)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	got := strings.TrimSpace(buf.String())
	gotLines := strings.Split(got, "\n")
	if len(gotLines) != 2 {
		t.Errorf("expected 2 block records, got %d: %q", len(gotLines), got)
	}
}

// TestQueryFilterSince verifies that records with timestamps before Since are
// excluded.
func TestQueryFilterSince(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	lines := strings.Join([]string{
		makeNDJSON("allow", "agent1", "bash", t1.Format(time.RFC3339)),
		makeNDJSON("allow", "agent1", "bash", t2.Format(time.RFC3339)),
	}, "\n")

	var buf bytes.Buffer
	err := Query(context.Background(), strings.NewReader(lines), QueryOpts{Since: t2}, &buf)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	got := strings.TrimSpace(buf.String())
	gotLines := strings.Split(got, "\n")
	if len(gotLines) != 1 {
		t.Errorf("expected 1 record (>= t2), got %d: %q", len(gotLines), got)
	}
	if !strings.Contains(gotLines[0], t2.Format(time.RFC3339)) {
		t.Errorf("expected record at t2, got: %q", gotLines[0])
	}
}

// TestQueryLimit verifies that at most Limit records are returned.
func TestQueryLimit(t *testing.T) {
	ts := time.Now().UTC().Format(time.RFC3339)
	var lineSlice []string
	for i := 0; i < 5; i++ {
		lineSlice = append(lineSlice, makeNDJSON("allow", "agent1", "bash", ts))
	}
	input := strings.Join(lineSlice, "\n")

	var buf bytes.Buffer
	err := Query(context.Background(), strings.NewReader(input), QueryOpts{Limit: 2}, &buf)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	got := strings.TrimSpace(buf.String())
	gotLines := strings.Split(got, "\n")
	if len(gotLines) != 2 {
		t.Errorf("expected 2 records (limit=2), got %d", len(gotLines))
	}
}

// TestQuerySkipsMalformed verifies that malformed lines are skipped, the
// surrounding well-formed lines are returned, and a skipped count is printed.
func TestQuerySkipsMalformed(t *testing.T) {
	ts := time.Now().UTC().Format(time.RFC3339)
	input := strings.Join([]string{
		makeNDJSON("allow", "agent1", "bash", ts),
		`{not valid json`,
		makeNDJSON("block", "agent1", "bash", ts),
	}, "\n")

	var buf bytes.Buffer
	err := Query(context.Background(), strings.NewReader(input), QueryOpts{}, &buf)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	out := buf.String()
	// 2 valid records + skipped count line
	if !strings.Contains(out, "1 malformed line(s) skipped") {
		t.Errorf("expected malformed line summary, got: %q", out)
	}
	// Both valid records must be present
	if strings.Count(out, "policy_decision") != 2 {
		t.Errorf("expected 2 valid records in output, got: %q", out)
	}
}
