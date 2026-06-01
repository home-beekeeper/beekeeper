package audit

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/bantuson/beekeeper/internal/config"
)

// --- Mock sink helpers ---

type mockSink struct {
	written []AuditRecord
	closed  bool
	err     error // if non-nil, Write returns this error
}

func (m *mockSink) Write(rec AuditRecord) error {
	if m.err != nil {
		return m.err
	}
	m.written = append(m.written, rec)
	return nil
}

func (m *mockSink) Close() error {
	m.closed = true
	return nil
}

type errSink struct{ called *atomic.Bool }

func (e *errSink) Write(_ AuditRecord) error {
	e.called.Store(true)
	return io.ErrUnexpectedEOF
}
func (e *errSink) Close() error { return nil }

// --- MultiSink tests ---

func TestMultiSinkFanout(t *testing.T) {
	a := &mockSink{}
	b := &mockSink{}
	ms := NewMultiSinkFromSinks([]Sink{a, b})

	rec := AuditRecord{RecordID: "fan-1", Decision: "allow"}
	if err := ms.Write(rec); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if len(a.written) != 1 || a.written[0].RecordID != "fan-1" {
		t.Errorf("sink A: got %v, want [{RecordID:fan-1}]", a.written)
	}
	if len(b.written) != 1 || b.written[0].RecordID != "fan-1" {
		t.Errorf("sink B: got %v, want [{RecordID:fan-1}]", b.written)
	}
}

func TestMultiSinkContinuesOnError(t *testing.T) {
	called := &atomic.Bool{}
	bad := &errSink{called: called}
	ok := &mockSink{}

	ms := NewMultiSinkFromSinks([]Sink{bad, ok})
	rec := AuditRecord{RecordID: "err-test"}

	err := ms.Write(rec)
	if err == nil {
		t.Fatal("Write: expected non-nil error from errSink, got nil")
	}
	if !called.Load() {
		t.Error("errSink.Write was never called")
	}
	if len(ok.written) != 1 {
		t.Errorf("okSink received %d records, want 1 (MultiSink must not short-circuit)", len(ok.written))
	}
}

func TestMultiSinkCloseAll(t *testing.T) {
	a := &mockSink{}
	b := &mockSink{}
	ms := NewMultiSinkFromSinks([]Sink{a, b})

	if err := ms.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !a.closed {
		t.Error("sink A was not closed")
	}
	if !b.closed {
		t.Error("sink B was not closed")
	}
}

// --- WriterSink tests ---

func TestWriterSinkDelegates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "beekeeper.ndjson")
	w, err := NewWriter(path)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	ws := NewWriterSink(w)

	rec := AuditRecord{RecordID: "ws-1", Decision: "block"}
	if err := ws.Write(rec); err != nil {
		t.Fatalf("WriterSink.Write: %v", err)
	}
	if err := ws.Close(); err != nil {
		t.Fatalf("WriterSink.Close: %v", err)
	}
}

// --- NewMultiSink integration test ---

func TestNewMultiSinkFileOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "beekeeper.ndjson")

	sink, err := NewMultiSink(path, config.AuditConfig{})
	if err != nil {
		t.Fatalf("NewMultiSink: %v", err)
	}
	defer sink.Close()

	rec := AuditRecord{RecordID: "file-only", Decision: "allow"}
	if err := sink.Write(rec); err != nil {
		t.Fatalf("Write: %v", err)
	}
}

// --- OTLPSink tests ---

func TestOTLPFlushOnClose(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received = body
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sink := NewOTLPSink(srv.URL)

	recs := []AuditRecord{
		{RecordID: "o-1", Decision: "allow", AgentName: "ag", ToolName: "tool"},
		{RecordID: "o-2", Decision: "warn", AgentName: "ag", ToolName: "tool"},
		{RecordID: "o-3", Decision: "block", AgentName: "ag", ToolName: "tool"},
	}
	for _, r := range recs {
		if err := sink.Write(r); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if len(received) == 0 {
		t.Fatal("no POST received by mock server")
	}

	// Validate OTLP structure
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("unmarshal OTLP payload: %v — body: %s", err, received)
	}
	if _, ok := payload["resourceLogs"]; !ok {
		t.Fatalf("OTLP payload missing 'resourceLogs' key; got: %s", received)
	}

	// Drill into logRecords to confirm all 3 records are present.
	type scopeLog struct {
		LogRecords []json.RawMessage `json:"logRecords"`
	}
	type resourceLog struct {
		ScopeLogs []scopeLog `json:"scopeLogs"`
	}
	var rl []resourceLog
	if err := json.Unmarshal(payload["resourceLogs"], &rl); err != nil {
		t.Fatalf("unmarshal resourceLogs: %v", err)
	}
	if len(rl) == 0 || len(rl[0].ScopeLogs) == 0 {
		t.Fatal("resourceLogs/scopeLogs empty")
	}
	logRecords := rl[0].ScopeLogs[0].LogRecords
	if len(logRecords) != 3 {
		t.Errorf("logRecords count = %d, want 3", len(logRecords))
	}
}

func TestOTLPBatching(t *testing.T) {
	var postCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postCount.Add(1)
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sink := NewOTLPSink(srv.URL)

	// Write exactly 100 records — should trigger an auto-flush.
	for i := 0; i < 100; i++ {
		if err := sink.Write(AuditRecord{RecordID: "batch"}); err != nil {
			t.Fatalf("Write %d: %v", i, err)
		}
	}
	if postCount.Load() < 1 {
		t.Error("expected at least one auto-flush POST after 100 records")
	}

	// Close flushes remaining (batch is empty here, but Close must not error).
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// --- HTTPSink tests ---

func TestHTTPSinkPostsNDJSON(t *testing.T) {
	var gotContentType string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sink := NewHTTPSink(srv.URL)
	rec := AuditRecord{RecordID: "h-1", Decision: "allow", ToolName: "Bash", AgentName: "claude"}
	if err := sink.Write(rec); err != nil {
		t.Fatalf("HTTPSink.Write: %v", err)
	}

	if gotContentType != "application/x-ndjson" {
		t.Errorf("Content-Type = %q, want %q", gotContentType, "application/x-ndjson")
	}
	if len(gotBody) == 0 {
		t.Fatal("empty body received")
	}

	// Body must be valid JSON (NDJSON line).
	var decoded AuditRecord
	if err := json.Unmarshal(gotBody[:len(gotBody)-1], &decoded); err != nil { // strip trailing newline
		t.Fatalf("body not valid JSON: %v — body: %s", err, gotBody)
	}
	if decoded.RecordID != "h-1" {
		t.Errorf("decoded RecordID = %q, want %q", decoded.RecordID, "h-1")
	}
}

func TestHTTPSinkContinuesOnError(t *testing.T) {
	// Point at a non-existent server — Write must return nil (fire-and-forget).
	sink := NewHTTPSink("http://127.0.0.1:1") // port 1 is never open
	rec := AuditRecord{RecordID: "h-err"}
	if err := sink.Write(rec); err != nil {
		t.Errorf("HTTPSink.Write on bad endpoint returned error %v, want nil", err)
	}
}

// --- Syslog stub test ---

func TestSyslogNotSupportedStubOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("syslog stub is Windows-only")
	}
	_, err := NewSyslogSink("localhost:514")
	if err == nil {
		t.Fatal("expected ErrSyslogNotSupported, got nil")
	}
	if err != ErrSyslogNotSupported {
		t.Errorf("got %v, want ErrSyslogNotSupported", err)
	}
}
