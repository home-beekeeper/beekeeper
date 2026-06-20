package audit

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/home-beekeeper/beekeeper/internal/config"
)

// errWriter is an io.Writer whose Write always fails. Used to exercise the
// write-error branches in Query and exportCSV.
type errWriter struct{}

func (errWriter) Write(_ []byte) (int, error) { return 0, errors.New("boom: write failed") }

// --- writer.go: NewWriterWithOptions rotation + sink fan-out (Write/Close) ---

// TestWriterWithOptionsRotates exercises the maxBytes>0 rotation branch inside
// Writer.Write. The record is always written to the file first; the post-write
// Rotate call is then triggered because the file is over threshold.
//
// Platform note: on Windows the Writer still holds the log file open, so the
// in-Writer Rotate rename fails and is logged to stderr (the documented
// fire-and-forget behaviour — Write still returns nil). On Unix the rename
// succeeds and an archive is produced. The test asserts the record was written
// and Write returned nil on both, then asserts the archive only where the rename
// can succeed (non-Windows).
func TestWriterWithOptionsRotates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "beekeeper.ndjson")

	// maxBytes=1 forces a rotation check on the first write (any record exceeds 1 byte).
	w, err := NewWriterWithOptions(path, 1, nil)
	if err != nil {
		t.Fatalf("NewWriterWithOptions: %v", err)
	}

	rec := AuditRecord{RecordID: "rot-1", Decision: "block", ToolName: "Bash", AgentName: "claude"}
	// Write must return nil regardless of platform — rotation errors are
	// fire-and-forget (logged to stderr only).
	if err := w.Write(rec); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if runtime.GOOS == "windows" {
		// The rename inside Rotate fails because the file is held open; the live
		// log therefore still holds the record. Verify the record was written.
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			t.Fatalf("ReadFile: %v", rerr)
		}
		if !bytes.Contains(data, []byte("rot-1")) {
			t.Errorf("live log missing the record on Windows (rotation is best-effort); got %q", data)
		}
		return
	}

	// Unix: the archive .1 must exist and contain the written record.
	archive, err := os.ReadFile(path + ".1")
	if err != nil {
		t.Fatalf("expected rotated archive beekeeper.ndjson.1: %v", err)
	}
	if !bytes.Contains(archive, []byte("rot-1")) {
		t.Errorf("archive .1 does not contain the rotated record; got %q", archive)
	}

	// The live file must exist and be empty (freshly recreated by Rotate).
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("live audit log missing after rotation: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("live audit log size = %d, want 0 after rotation", info.Size())
	}
}

// TestWriterWriteFansOutToSinks verifies that Writer.Write delivers a copy of the
// record to every additional sink after the file write, and that a sink error is
// swallowed (fire-and-forget) rather than surfaced to the caller.
func TestWriterWriteFansOutToSinks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "beekeeper.ndjson")

	good := &mockSink{}
	bad := &mockSink{err: errors.New("sink down")} // exercises the stderr error branch

	w, err := NewWriterWithOptions(path, 0, []Sink{good, bad})
	if err != nil {
		t.Fatalf("NewWriterWithOptions: %v", err)
	}

	rec := AuditRecord{RecordID: "fanout-1", Decision: "allow"}
	// Write must return nil even though the bad sink errors.
	if err := w.Write(rec); err != nil {
		t.Fatalf("Write returned %v, want nil (sink errors are fire-and-forget)", err)
	}

	if len(good.written) != 1 || good.written[0].RecordID != "fanout-1" {
		t.Errorf("good sink got %v, want one fanout-1 record", good.written)
	}

	// Close must close every sink and return the file Close result (nil here).
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !good.closed {
		t.Error("good sink was not closed by Writer.Close")
	}
	if !bad.closed {
		t.Error("bad sink was not closed by Writer.Close")
	}

	// The file write must still have happened despite the sink error.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Contains(data, []byte("fanout-1")) {
		t.Errorf("audit file missing the record; got %q", data)
	}
}

// errCloseSink errors on Close to exercise the Writer.Close sink-close stderr branch.
type errCloseSink struct{ closed bool }

func (s *errCloseSink) Write(_ AuditRecord) error { return nil }
func (s *errCloseSink) Close() error {
	s.closed = true
	return errors.New("close failed")
}

// TestWriterCloseSwallowsSinkCloseError verifies a sink Close error is logged but
// does not stop other sinks from closing, and the file Close result is returned.
func TestWriterCloseSwallowsSinkCloseError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "beekeeper.ndjson")

	ec := &errCloseSink{}
	ok := &mockSink{}
	w, err := NewWriterWithOptions(path, 0, []Sink{ec, ok})
	if err != nil {
		t.Fatalf("NewWriterWithOptions: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close returned %v, want nil (file closed cleanly)", err)
	}
	if !ec.closed {
		t.Error("erroring sink Close was not called")
	}
	if !ok.closed {
		t.Error("subsequent sink Close was not called after a prior sink Close error")
	}
}

// TestNewWriterWithOptionsMkdirError exercises the os.MkdirAll error branch by
// placing a regular file where a parent directory component is required, so
// MkdirAll cannot create the directory tree.
func TestNewWriterWithOptionsMkdirError(t *testing.T) {
	dir := t.TempDir()
	// Create a regular file, then ask for an audit log "inside" it. MkdirAll must
	// fail because a path component (the file) is not a directory.
	blocker := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	badPath := filepath.Join(blocker, "sub", "beekeeper.ndjson")
	if _, err := NewWriterWithOptions(badPath, 0, nil); err == nil {
		t.Fatal("NewWriterWithOptions with a file-as-directory parent = nil, want error")
	}
}

// --- otlp.go: flushLocked non-2xx and request-error branches ---

// TestOTLPFlushNon2xx exercises the path where the collector returns a non-2xx
// status. The sink reads/closes the response body and never returns an error.
func TestOTLPFlushNon2xx(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer srv.Close()

	sink := NewOTLPSink(srv.URL)
	if err := sink.Write(AuditRecord{RecordID: "x", Timestamp: time.Now().UTC().Format(time.RFC3339)}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	// Close flushes the single buffered record; non-2xx is fire-and-forget.
	if err := sink.Close(); err != nil {
		t.Fatalf("Close returned %v, want nil on non-2xx", err)
	}
	if hits == 0 {
		t.Error("collector never received the flush POST")
	}
}

// TestOTLPFlushUnreachable exercises the client.Do error branch (connection
// refused). flushLocked logs to stderr and returns nil.
func TestOTLPFlushUnreachable(t *testing.T) {
	sink := NewOTLPSink("https://127.0.0.1:1/v1/logs") // port 1 never accepts
	if err := sink.Write(AuditRecord{RecordID: "u"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Errorf("Close on unreachable endpoint returned %v, want nil", err)
	}
}

// TestOTLPFlushBadEndpoint exercises the http.NewRequest error branch via an
// endpoint with an illegal control character in the URL.
func TestOTLPFlushBadEndpoint(t *testing.T) {
	sink := NewOTLPSink("https://example.com/\x7f") // DEL byte -> NewRequest fails
	if err := sink.Write(AuditRecord{RecordID: "b"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Errorf("Close on malformed endpoint returned %v, want nil", err)
	}
}

// --- http_sink.go: Close (no-op) + non-2xx + bad endpoint ---

// TestHTTPSinkClose verifies Close is a no-op that returns nil.
func TestHTTPSinkClose(t *testing.T) {
	sink := NewHTTPSink("https://collector.example/logs")
	if err := sink.Close(); err != nil {
		t.Errorf("HTTPSink.Close() = %v, want nil", err)
	}
}

// TestHTTPSinkNon2xx verifies a non-2xx response is fire-and-forget.
func TestHTTPSinkNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "denied", http.StatusForbidden)
	}))
	defer srv.Close()

	sink := NewHTTPSink(srv.URL)
	if err := sink.Write(AuditRecord{RecordID: "n"}); err != nil {
		t.Errorf("Write on non-2xx returned %v, want nil", err)
	}
}

// TestHTTPSinkBadEndpoint exercises the http.NewRequest error branch.
func TestHTTPSinkBadEndpoint(t *testing.T) {
	sink := NewHTTPSink("https://example.com/\x7f")
	if err := sink.Write(AuditRecord{RecordID: "b"}); err != nil {
		t.Errorf("Write on malformed endpoint returned %v, want nil", err)
	}
}

// --- syslog_stub.go (Windows): direct method coverage ---

// TestSyslogStubMethods constructs the stub directly and calls its Write/Close so
// the stub method bodies are covered on Windows hosts.
func TestSyslogStubMethods(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("syslogStub is the Windows-only build of NewSyslogSink")
	}
	s := &syslogStub{}
	if err := s.Write(AuditRecord{RecordID: "s"}); !errors.Is(err, ErrSyslogNotSupported) {
		t.Errorf("syslogStub.Write = %v, want ErrSyslogNotSupported", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("syslogStub.Close = %v, want nil", err)
	}
}

// --- sink.go: NewMultiSink syslog-skip + remote-name warning paths ---

// TestNewMultiSinkSkipsUnsupportedSyslog verifies that on a platform where
// NewSyslogSink returns ErrSyslogNotSupported (Windows), NewMultiSink skips the
// syslog sink gracefully and still builds the file sink.
func TestNewMultiSinkSkipsUnsupportedSyslog(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("syslog is unsupported only on Windows; the skip branch is Windows-only")
	}
	path := filepath.Join(t.TempDir(), "beekeeper.ndjson")
	cfg := config.AuditConfig{
		Sinks:         []string{"syslog"},
		SyslogAddress: "localhost:514",
	}
	sink, err := NewMultiSink(path, cfg)
	if err != nil {
		t.Fatalf("NewMultiSink should skip unsupported syslog, got error: %v", err)
	}
	defer sink.Close()

	// File sink must still work.
	if err := sink.Write(AuditRecord{RecordID: "syslog-skip"}); err != nil {
		t.Fatalf("Write after syslog-skip: %v", err)
	}
}

// TestNewMultiSinkRemoteWarningPath builds a sink graph with BOTH otlp and https
// remote sinks configured to accepted external https endpoints. This exercises
// the remoteNames accumulation and the "audit data will leave this machine"
// warning branch.
func TestNewMultiSinkRemoteWarningPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "beekeeper.ndjson")
	cfg := config.AuditConfig{
		Sinks:         []string{"otlp", "https"},
		OTLPEndpoint:  "https://collector.example/v1/logs",
		HTTPSEndpoint: "https://collector.example/ingest",
	}
	sink, err := NewMultiSink(path, cfg)
	if err != nil {
		t.Fatalf("NewMultiSink with two remote sinks: %v", err)
	}
	defer sink.Close()

	// The graph must be a *MultiSink with 3 sinks (file + otlp + https).
	ms, ok := sink.(*MultiSink)
	if !ok {
		t.Fatalf("NewMultiSink returned %T, want *MultiSink", sink)
	}
	if len(ms.sinks) != 3 {
		t.Errorf("MultiSink has %d sinks, want 3 (file+otlp+https)", len(ms.sinks))
	}
}

// TestNewMultiSinkRejectsHTTPSEndpointSSRF verifies the https-sink validation
// branch fails closed on an SSRF target.
func TestNewMultiSinkRejectsHTTPSEndpointSSRF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "beekeeper.ndjson")
	cfg := config.AuditConfig{
		Sinks:         []string{"https"},
		HTTPSEndpoint: "https://10.0.0.5/ingest",
	}
	if _, err := NewMultiSink(path, cfg); err == nil {
		t.Fatal("NewMultiSink with a private-range https endpoint = nil, want SSRF rejection")
	}
}

// TestNewMultiSinkWithCorpusPropagatesBaseError verifies that when the base
// NewMultiSink fails (bad remote endpoint), NewMultiSinkWithCorpus returns that
// error without attempting to attach the corpus sink.
func TestNewMultiSinkWithCorpusPropagatesBaseError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "beekeeper.ndjson")
	cfg := config.AuditConfig{
		Sinks:        []string{"otlp"},
		OTLPEndpoint: "http://169.254.169.254/latest/meta-data/", // SSRF -> base fails
	}
	corpus := &mockSink{}
	if _, err := NewMultiSinkWithCorpus(path, cfg, corpus); err == nil {
		t.Fatal("NewMultiSinkWithCorpus should propagate the base SSRF error, got nil")
	}
	if len(corpus.written) != 0 {
		t.Error("corpus sink should not receive writes when base construction fails")
	}
}

// --- sink_validate.go: remaining branches ---

func TestValidateRemoteSinkEndpointInvalidURL(t *testing.T) {
	// A control byte makes url.Parse fail.
	if err := ValidateRemoteSinkEndpoint("https://exa mple\x7f.com/x\n", true); err == nil {
		t.Error("expected invalid-URL error, got nil")
	}
}

func TestValidateRemoteSinkEndpointEmptyHost(t *testing.T) {
	// A scheme-only URL has an empty host.
	if err := ValidateRemoteSinkEndpoint("https:///v1/logs", true); err == nil {
		t.Error("expected empty-host error, got nil")
	}
}

func TestValidateRemoteSinkEndpointHTTPAllowedWhenNotRequiringHTTPS(t *testing.T) {
	// requireHTTPS=false: http is permitted to an external host.
	if err := ValidateRemoteSinkEndpoint("http://collector.example/v1/logs", false); err != nil {
		t.Errorf("http endpoint with requireHTTPS=false = %v, want nil", err)
	}
	// A bogus scheme is still rejected when requireHTTPS=false.
	if err := ValidateRemoteSinkEndpoint("ftp://collector.example/x", false); err == nil {
		t.Error("ftp scheme with requireHTTPS=false should be rejected, got nil")
	}
}

func TestIsSSRFTargetHostIPv6LinkLocal(t *testing.T) {
	// fe80::/10 is link-local unicast and must be rejected.
	if err := ValidateRemoteSinkEndpoint("https://[fe80::1]/x", true); err == nil {
		t.Error("fe80:: link-local IPv6 should be rejected as SSRF target")
	}
	// fc00::/7 unique-local must be rejected (ip.IsPrivate).
	if err := ValidateRemoteSinkEndpoint("https://[fc00::1]/x", true); err == nil {
		t.Error("fc00:: unique-local IPv6 should be rejected as SSRF target")
	}
}

// --- query.go: Agent/Tool filters + write-error branch ---

func TestQueryFilterAgentAndTool(t *testing.T) {
	ts := time.Now().UTC().Format(time.RFC3339)
	input := strings.Join([]string{
		makeNDJSON("allow", "agentX", "bash", ts),
		makeNDJSON("allow", "agentY", "bash", ts),
		makeNDJSON("allow", "agentX", "npm", ts),
	}, "\n")

	// Agent filter: only agentX records (2 of them).
	var agentBuf bytes.Buffer
	if err := Query(context.Background(), strings.NewReader(input), QueryOpts{Agent: "agentX"}, &agentBuf); err != nil {
		t.Fatalf("Query agent: %v", err)
	}
	if got := strings.Count(agentBuf.String(), "policy_decision"); got != 2 {
		t.Errorf("agent filter returned %d records, want 2", got)
	}

	// Tool filter: only npm (1 record).
	var toolBuf bytes.Buffer
	if err := Query(context.Background(), strings.NewReader(input), QueryOpts{Tool: "npm"}, &toolBuf); err != nil {
		t.Fatalf("Query tool: %v", err)
	}
	if got := strings.Count(toolBuf.String(), "policy_decision"); got != 1 {
		t.Errorf("tool filter returned %d records, want 1", got)
	}
}

func TestQueryWriteError(t *testing.T) {
	ts := time.Now().UTC().Format(time.RFC3339)
	input := makeNDJSON("allow", "a", "b", ts)
	err := Query(context.Background(), strings.NewReader(input), QueryOpts{}, errWriter{})
	if err == nil {
		t.Fatal("expected a write error from Query, got nil")
	}
}

func TestQueryFilterSinceUnparseableTimestamp(t *testing.T) {
	// A record with a non-RFC3339 timestamp must be excluded by the Since filter
	// (filterRecord returns false when time.Parse fails).
	badTs := `{"record_type":"policy_decision","record_id":"r","timestamp":"not-a-time","scanner_name":"beekeeper","agent_name":"a","tool_name":"t","decision":"allow","reason":"x","rule_ids":[],"catalog_matches":[],"endpoint":"check","corroboration_count":0,"sources_agreed":[],"sources_dissented":[]}`
	var buf bytes.Buffer
	if err := Query(context.Background(), strings.NewReader(badTs), QueryOpts{Since: time.Unix(0, 0)}, &buf); err != nil {
		t.Fatalf("Query: %v", err)
	}
	if strings.Contains(buf.String(), "policy_decision") {
		t.Error("record with unparseable timestamp should be excluded by Since filter")
	}
}

// --- export.go: CSV/OTLP filters, context cancellation, write errors ---

func TestExportCSVAppliesFilter(t *testing.T) {
	ts := time.Now().UTC().Format(time.RFC3339)
	input := strings.Join([]string{
		makeNDJSON("allow", "a", "bash", ts),
		makeNDJSON("block", "a", "bash", ts),
	}, "\n")

	var buf bytes.Buffer
	opts := ExportOpts{Format: "csv", QueryOpts: QueryOpts{Decision: "block"}}
	if err := Export(context.Background(), strings.NewReader(input), opts, &buf); err != nil {
		t.Fatalf("Export csv: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	// header + exactly one block row
	if len(lines) != 2 {
		t.Fatalf("expected header + 1 filtered row, got %d lines:\n%s", len(lines), buf.String())
	}
	if !strings.Contains(lines[1], "block") {
		t.Errorf("filtered CSV row missing decision=block: %q", lines[1])
	}
}

func TestExportCSVSkipsMalformed(t *testing.T) {
	ts := time.Now().UTC().Format(time.RFC3339)
	input := strings.Join([]string{
		makeNDJSON("allow", "a", "bash", ts),
		`{ broken`,
		makeNDJSON("allow", "a", "bash", ts),
	}, "\n")
	var buf bytes.Buffer
	if err := Export(context.Background(), strings.NewReader(input), ExportOpts{Format: "csv"}, &buf); err != nil {
		t.Fatalf("Export csv: %v", err)
	}
	// header + 2 valid rows (malformed line silently skipped).
	if got := len(strings.Split(strings.TrimSpace(buf.String()), "\n")); got != 3 {
		t.Errorf("expected 3 CSV lines (header + 2 valid), got %d", got)
	}
}

func TestExportCSVHeaderWriteError(t *testing.T) {
	// errWriter fails immediately, so the header write fails.
	err := Export(context.Background(), strings.NewReader(""), ExportOpts{Format: "csv"}, errWriter{})
	if err == nil {
		t.Fatal("expected a CSV header write error, got nil")
	}
}

func TestExportOTLPAppliesFilter(t *testing.T) {
	ts := time.Now().UTC().Format(time.RFC3339)
	input := strings.Join([]string{
		makeNDJSON("allow", "a", "bash", ts),
		makeNDJSON("block", "a", "npm", ts),
	}, "\n")

	var buf bytes.Buffer
	opts := ExportOpts{Format: "otlp", QueryOpts: QueryOpts{Tool: "npm"}}
	if err := Export(context.Background(), strings.NewReader(input), opts, &buf); err != nil {
		t.Fatalf("Export otlp: %v", err)
	}
	// Only the npm record should survive the filter -> exactly one logRecord body.
	if c := strings.Count(buf.String(), `"npm"`); c == 0 {
		t.Errorf("filtered OTLP output missing npm record: %s", buf.String())
	}
	if strings.Contains(buf.String(), `\"tool_name\":\"bash\"`) {
		t.Errorf("filtered OTLP output should not contain bash record: %s", buf.String())
	}
}

func TestExportOTLPSkipsMalformedAndBadTimestamp(t *testing.T) {
	// One malformed line (skipped) and one valid record with a non-RFC3339
	// timestamp (the nanos stays 0 — exercises the time.Parse failure branch).
	badTs := `{"record_type":"policy_decision","record_id":"r","timestamp":"nope","scanner_name":"beekeeper","agent_name":"a","tool_name":"t","decision":"allow","reason":"x","rule_ids":[],"catalog_matches":[],"endpoint":"check","corroboration_count":0,"sources_agreed":[],"sources_dissented":[]}`
	input := strings.Join([]string{`{not json`, badTs}, "\n")

	var buf bytes.Buffer
	if err := Export(context.Background(), strings.NewReader(input), ExportOpts{Format: "otlp"}, &buf); err != nil {
		t.Fatalf("Export otlp: %v", err)
	}
	if !strings.Contains(buf.String(), "resourceLogs") {
		t.Errorf("OTLP output missing resourceLogs: %s", buf.String())
	}
	// timeUnixNano should be "0" for the unparseable timestamp.
	if !strings.Contains(buf.String(), `"timeUnixNano":"0"`) {
		t.Errorf("expected timeUnixNano=0 for unparseable timestamp, got: %s", buf.String())
	}
}

func TestExportUnknownFormat(t *testing.T) {
	err := Export(context.Background(), strings.NewReader(""), ExportOpts{Format: "xml"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected an unknown-format error, got nil")
	}
}

func TestExportCSVContextCancelled(t *testing.T) {
	// Build > 100 lines so the lineNum%100 ctx check is reached, with an already
	// cancelled context so it returns ctx.Err().
	ts := time.Now().UTC().Format(time.RFC3339)
	var sb strings.Builder
	for i := 0; i < 250; i++ {
		sb.WriteString(makeNDJSON("allow", "a", "bash", ts))
		sb.WriteByte('\n')
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Export(ctx, strings.NewReader(sb.String()), ExportOpts{Format: "csv"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected context cancellation error from exportCSV, got nil")
	}
}

func TestExportOTLPContextCancelled(t *testing.T) {
	ts := time.Now().UTC().Format(time.RFC3339)
	var sb strings.Builder
	for i := 0; i < 250; i++ {
		sb.WriteString(makeNDJSON("allow", "a", "bash", ts))
		sb.WriteByte('\n')
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Export(ctx, strings.NewReader(sb.String()), ExportOpts{Format: "otlp"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected context cancellation error from exportOTLP, got nil")
	}
}

// --- rotate.go: retention deletion of a multi-archive set + shift survivors ---

// TestRotateMixedRetention sets up two existing archives: .1 (old -> deleted) and
// .2 (fresh -> survives & shifts to .3). This exercises both the retention-delete
// branch and the reverse-order survivor shift with a real shift target.
func TestRotateMixedRetention(t *testing.T) {
	dir := t.TempDir()
	auditPath := filepath.Join(dir, "beekeeper.ndjson")

	// .1 old (deleted), .2 fresh (survives -> .3).
	if err := os.WriteFile(auditPath+".1", []byte("old-archive"), 0600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-40 * 24 * time.Hour)
	if err := os.Chtimes(auditPath+".1", old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(auditPath+".2", []byte("fresh-archive"), 0600); err != nil {
		t.Fatal(err)
	}

	// Current log over the threshold.
	if err := os.WriteFile(auditPath, make([]byte, 100), 0600); err != nil {
		t.Fatal(err)
	}

	if err := Rotate(auditPath, 50, 30); err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	// .1 must now be the rotated current log (100 bytes).
	if info, err := os.Stat(auditPath + ".1"); err != nil {
		t.Errorf("expected new .1: %v", err)
	} else if info.Size() != 100 {
		t.Errorf("new .1 size = %d, want 100 (rotated current log)", info.Size())
	}

	// The fresh .2 must have shifted to .3 with its original content.
	got, err := os.ReadFile(auditPath + ".3")
	if err != nil {
		t.Fatalf("expected fresh archive shifted to .3: %v", err)
	}
	if string(got) != "fresh-archive" {
		t.Errorf(".3 content = %q, want fresh-archive", got)
	}
}

// TestRotateMissingFileNoOp verifies Rotate returns nil when the audit log does
// not exist (os.IsNotExist branch).
func TestRotateMissingFileNoOp(t *testing.T) {
	auditPath := filepath.Join(t.TempDir(), "does-not-exist.ndjson")
	if err := Rotate(auditPath, 1, 30); err != nil {
		t.Errorf("Rotate on missing file = %v, want nil", err)
	}
}
