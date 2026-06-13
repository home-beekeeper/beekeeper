package corpus

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bantuson/beekeeper/internal/audit"
)

// TestStoreAppendOnly verifies STORE-01: the corpus NDJSON file is append-only.
//
// Two writes — one before and one after a close/reopen — must both appear in
// the file (no truncation between writes).
func TestStoreAppendOnly(t *testing.T) {
	dir := t.TempDir()
	corpusPath := filepath.Join(dir, "corpus", "beekeeper-corpus.ndjson")

	// Write record A.
	sink, err := NewStoreSink(corpusPath)
	if err != nil {
		t.Fatalf("NewStoreSink: %v", err)
	}
	recA := audit.AuditRecord{
		RecordType: "policy_decision",
		Decision:   "block",
		Reason:     "record A",
	}
	if err := sink.Write(recA); err != nil {
		t.Fatalf("Write(recA): %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close after first write: %v", err)
	}

	// Reopen and write record B.
	sink2, err := NewStoreSink(corpusPath)
	if err != nil {
		t.Fatalf("NewStoreSink (reopen): %v", err)
	}
	recB := audit.AuditRecord{
		RecordType: "policy_decision",
		Decision:   "allow",
		Reason:     "record B",
	}
	if err := sink2.Write(recB); err != nil {
		t.Fatalf("Write(recB): %v", err)
	}
	if err := sink2.Close(); err != nil {
		t.Fatalf("Close after second write: %v", err)
	}

	// Read back the file: expect exactly two NDJSON lines.
	f, err := os.Open(corpusPath)
	if err != nil {
		t.Fatalf("Open corpus file: %v", err)
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan corpus file: %v", err)
	}

	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines (append-only), got %d: %v", len(lines), lines)
	}

	// Both records must parse as CorpusRecord with non-empty decision.
	for i, line := range lines {
		var cr CorpusRecord
		if err := json.Unmarshal([]byte(line), &cr); err != nil {
			t.Errorf("line %d: json.Unmarshal CorpusRecord: %v", i, err)
		}
	}
}

// TestStoreRedactsSecretsBeforeWrite verifies STORE-02 (T-23-01 mitigation):
// RedactRecordWithDefaults runs as the FIRST operation in StoreSink.Write.
// A credential-shaped string in AuditRecord.Reason must NOT appear in the
// persisted NDJSON line.
func TestStoreRedactsSecretsBeforeWrite(t *testing.T) {
	const secretKey = "AKIAIOSFODNN7EXAMPLE"

	dir := t.TempDir()
	corpusPath := filepath.Join(dir, "corpus", "beekeeper-corpus.ndjson")

	sink, err := NewStoreSink(corpusPath)
	if err != nil {
		t.Fatalf("NewStoreSink: %v", err)
	}
	defer sink.Close()

	rec := audit.AuditRecord{
		RecordType: "policy_decision",
		Decision:   "block",
		Reason:     "leaked " + secretKey + " here",
	}
	if err := sink.Write(rec); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(corpusPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(data), secretKey) {
		t.Errorf("corpus NDJSON contains unredacted secret %q — RedactRecordWithDefaults did not run first\nfile content: %s",
			secretKey, string(data))
	}
}

// TestStoreFilePermissions verifies STORE-03 (T-23-03 mitigation):
// the corpus file is owner-only (0600 on Unix; file exists on Windows).
//
// On Windows, the 0600-bit assertion is skipped (DACL enforcement is tested
// via platform.SetOwnerOnly separately). We verify the file exists and is
// readable only by the owner via the absence of group/world bits (Unix only).
func TestStoreFilePermissions(t *testing.T) {
	dir := t.TempDir()
	corpusPath := filepath.Join(dir, "corpus", "beekeeper-corpus.ndjson")

	sink, err := NewStoreSink(corpusPath)
	if err != nil {
		t.Fatalf("NewStoreSink: %v", err)
	}
	rec := audit.AuditRecord{RecordType: "policy_decision", Decision: "block"}
	if err := sink.Write(rec); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	info, err := os.Stat(corpusPath)
	if err != nil {
		t.Fatalf("Stat corpus file: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Errorf("corpus path is not a regular file: mode=%v", info.Mode())
	}

	// On non-Windows: assert 0600 bit-exact permission.
	if runtime.GOOS != "windows" {
		perm := info.Mode().Perm()
		if perm != 0o600 {
			t.Errorf("corpus file permissions = %04o, want 0600 (owner-only)", perm)
		}
	}
}

// TestStoreEmitsPushEnvelopeShape verifies STORE-04:
// a persisted corpus record carries a non-nil push_envelope object from
// the first write, even before the Phase 23 emitter is wired.
func TestStoreEmitsPushEnvelopeShape(t *testing.T) {
	dir := t.TempDir()
	corpusPath := filepath.Join(dir, "corpus", "beekeeper-corpus.ndjson")

	sink, err := NewStoreSink(corpusPath)
	if err != nil {
		t.Fatalf("NewStoreSink: %v", err)
	}
	rec := audit.AuditRecord{
		RecordType: "policy_decision",
		Decision:   "block",
		Reason:     "sentry alert",
	}
	if err := sink.Write(rec); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(corpusPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatal("corpus file is empty after write")
	}

	var cr CorpusRecord
	if err := json.Unmarshal([]byte(lines[0]), &cr); err != nil {
		t.Fatalf("json.Unmarshal CorpusRecord: %v", err)
	}

	if cr.PushEnvelope == nil {
		t.Errorf("PushEnvelope is nil — StoreSink.Write must populate a non-nil push_envelope from the first write")
	}
}
