package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	audit "github.com/mzansi-agentive/beekeeper/internal/audit"
)

// ndjsonLine serialises rec as a JSON object followed by a newline.
func ndjsonLine(t *testing.T, rec audit.AuditRecord) []byte {
	t.Helper()
	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("ndjsonLine: marshal failed: %v", err)
	}
	return append(b, '\n')
}

// makeRecord returns a minimal AuditRecord with the given decision.
func makeRecord(decision string) audit.AuditRecord {
	return audit.AuditRecord{
		RecordType: "policy_decision",
		Decision:   decision,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		ScannerName: "beekeeper",
	}
}

// TestTailFromPartialLine is the CR-01 regression test.
//
// Scenario:
//   1. Write one complete NDJSON line and one partial line (no trailing newline)
//      to simulate a mid-write audit record.
//   2. First tailFrom call must return exactly 1 record (the complete line)
//      and an offset pointing to the END of that complete line — NOT past the
//      partial fragment.
//   3. Append the missing newline (completing the second record).
//   4. Second tailFrom call from the returned offset must return exactly 1 record
//      (the now-complete second line). This proves the record was NOT lost.
func TestTailFromPartialLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.ndjson")

	rec1 := makeRecord("allow")
	rec2 := makeRecord("block")

	line1 := ndjsonLine(t, rec1)
	// rec2 serialised WITHOUT a trailing newline — simulates mid-write.
	line2raw, err := json.Marshal(rec2)
	if err != nil {
		t.Fatalf("marshal rec2: %v", err)
	}

	// Write complete line1 + incomplete line2 (no newline).
	content := append(line1, line2raw...)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// First call: should only see rec1; partial line2 must NOT be emitted.
	records1, offset1 := tailFrom(path, 0)
	if len(records1) != 1 {
		t.Fatalf("first tailFrom: expected 1 record, got %d", len(records1))
	}
	if records1[0].Decision != "allow" {
		t.Errorf("first tailFrom: expected decision=allow, got %q", records1[0].Decision)
	}
	// offset1 must equal exactly the length of line1 (not past line2raw).
	if offset1 != int64(len(line1)) {
		t.Errorf("first tailFrom: offset = %d, want %d (end of complete line)", offset1, len(line1))
	}

	// Append the missing newline — now line2 is complete on disk.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatalf("OpenFile for append: %v", err)
	}
	if _, err := f.Write([]byte{'\n'}); err != nil {
		f.Close()
		t.Fatalf("append newline: %v", err)
	}
	f.Close()

	// Second call from offset1: must emit rec2 exactly once.
	records2, offset2 := tailFrom(path, offset1)
	if len(records2) != 1 {
		t.Fatalf("second tailFrom: expected 1 record, got %d (partial line must be re-read once newline lands)", len(records2))
	}
	if records2[0].Decision != "block" {
		t.Errorf("second tailFrom: expected decision=block, got %q", records2[0].Decision)
	}
	// offset2 should now be at end of file.
	expected2 := int64(len(line1) + len(line2raw) + 1 /* the newline */)
	if offset2 != expected2 {
		t.Errorf("second tailFrom: offset = %d, want %d", offset2, expected2)
	}
}

// TestTailFromCompleteLines verifies that two complete lines in one write are
// both returned in the first call and nothing is re-emitted on the second call.
func TestTailFromCompleteLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.ndjson")

	line1 := ndjsonLine(t, makeRecord("allow"))
	line2 := ndjsonLine(t, makeRecord("block"))

	if err := os.WriteFile(path, append(line1, line2...), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// First call: expect both records.
	records1, offset1 := tailFrom(path, 0)
	if len(records1) != 2 {
		t.Fatalf("first tailFrom: expected 2 records, got %d", len(records1))
	}
	if records1[0].Decision != "allow" {
		t.Errorf("records1[0].Decision = %q, want allow", records1[0].Decision)
	}
	if records1[1].Decision != "block" {
		t.Errorf("records1[1].Decision = %q, want block", records1[1].Decision)
	}

	// Second call from the returned offset: no new records (no re-read).
	records2, offset2 := tailFrom(path, offset1)
	if len(records2) != 0 {
		t.Errorf("second tailFrom: expected 0 records, got %d (double-emit)", len(records2))
	}
	if offset2 != offset1 {
		t.Errorf("second tailFrom: offset changed from %d to %d with no new data", offset1, offset2)
	}
}

// TestTailFromMalformedSkipped verifies that a malformed (non-JSON) complete line
// is silently skipped but its bytes still advance the offset, so it is not
// re-read on the next call.
func TestTailFromMalformedSkipped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.ndjson")

	badLine := []byte("this is not json\n")
	goodLine := ndjsonLine(t, makeRecord("allow"))

	if err := os.WriteFile(path, append(badLine, goodLine...), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// First call: bad line skipped, good line returned; offset advances past both.
	records1, offset1 := tailFrom(path, 0)
	if len(records1) != 1 {
		t.Fatalf("first tailFrom: expected 1 record (malformed skipped), got %d", len(records1))
	}
	if records1[0].Decision != "allow" {
		t.Errorf("records1[0].Decision = %q, want allow", records1[0].Decision)
	}
	expectedOffset := int64(len(badLine) + len(goodLine))
	if offset1 != expectedOffset {
		t.Errorf("offset1 = %d, want %d", offset1, expectedOffset)
	}

	// Second call: nothing re-emitted (malformed line not re-read).
	records2, _ := tailFrom(path, offset1)
	if len(records2) != 0 {
		t.Errorf("second tailFrom: expected 0 records, got %d (malformed line re-read)", len(records2))
	}
}

// TestTailFromMissingFile verifies that a non-existent path returns (nil, offset)
// without panicking or returning an error.
func TestTailFromMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does_not_exist.ndjson")

	const initialOffset int64 = 42
	records, offset := tailFrom(path, initialOffset)
	if records != nil {
		t.Errorf("expected nil records for missing file, got %v", records)
	}
	if offset != initialOffset {
		t.Errorf("expected offset unchanged (%d), got %d", initialOffset, offset)
	}
}
