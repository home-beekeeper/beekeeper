package gateway

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bantuson/beekeeper/internal/audit"
	"github.com/bantuson/beekeeper/internal/nudge"
)

// newDriftTestHandler creates a gatewayHandler with an audit path pointing to a
// temp file, used by drift_test.go to capture emitted audit records.
func newDriftTestHandler(t *testing.T) (*gatewayHandler, string) {
	t.Helper()
	dir := t.TempDir()
	auditPath := filepath.Join(dir, "drift-audit.ndjson")

	cfg := Config{
		AuditPath: auditPath,
		Nudge:     nudge.DefaultConfig(),
	}
	h := &gatewayHandler{
		cfg:     cfg,
		advSeen: make(map[string]bool),
		nudgeCache: nudge.NewCache(func(_ context.Context, _ nudge.Config) nudge.PMState {
			return nudge.PMState{}
		}, 60e9), // 60s TTL, not used in drift tests
	}
	return h, auditPath
}

// readAllAuditRecords reads all NDJSON lines from path and returns decoded records.
func readAllAuditRecords(t *testing.T, auditPath string) []audit.AuditRecord {
	t.Helper()
	f, err := os.Open(auditPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		t.Fatalf("open audit file: %v", err)
	}
	defer f.Close()

	var records []audit.AuditRecord
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var rec audit.AuditRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("parse audit NDJSON: %v\nline: %s", err, line)
		}
		records = append(records, rec)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan audit: %v", err)
	}
	return records
}

// TestCheckDriftEmitsVersionDrift verifies §10-15: when the injected
// metadataFetchFn returns a version with a higher major than the floor,
// checkDrift emits exactly one record_type:"version_drift" audit record for pnpm.
func TestCheckDriftEmitsVersionDrift(t *testing.T) {
	h, auditPath := newDriftTestHandler(t)

	// Inject: pnpm 12.0.0 is a new major over the default floor (11.0.0).
	orig := metadataFetchFn
	metadataFetchFn = func(_ context.Context) (map[string]string, error) {
		return map[string]string{"pnpm": "12.0.0"}, nil
	}
	defer func() { metadataFetchFn = orig }()

	h.checkDrift(context.Background())

	records := readAllAuditRecords(t, auditPath)
	if len(records) == 0 {
		t.Fatal("expected at least one audit record, got none")
	}

	// Must have a version_drift record for pnpm.
	var driftRec *audit.AuditRecord
	for i := range records {
		if records[i].RecordType == "version_drift" {
			driftRec = &records[i]
			break
		}
	}
	if driftRec == nil {
		t.Fatalf("no version_drift record found; records: %v", records)
	}

	// Verify the record contains the pnpm drift information.
	if driftRec.RecordID == "" {
		t.Error("version_drift record has empty record_id")
	}
	if driftRec.Timestamp == "" {
		t.Error("version_drift record has empty timestamp")
	}
	if driftRec.ScannerName != "beekeeper" {
		t.Errorf("scanner_name = %q, want beekeeper", driftRec.ScannerName)
	}
}

// TestCheckDriftNoDrift verifies that when the latest version has the same major
// as the floor, no version_drift record is emitted (§10-15 — no-drift case).
func TestCheckDriftNoDrift(t *testing.T) {
	h, auditPath := newDriftTestHandler(t)

	// Inject: pnpm 11.5.0 is the same major as the floor (11.0.0) — no drift.
	orig := metadataFetchFn
	metadataFetchFn = func(_ context.Context) (map[string]string, error) {
		return map[string]string{"pnpm": "11.5.0", "bun": "1.3.14"}, nil
	}
	defer func() { metadataFetchFn = orig }()

	h.checkDrift(context.Background())

	// The audit file should either not exist or contain no version_drift records.
	records := readAllAuditRecords(t, auditPath)
	for _, rec := range records {
		if rec.RecordType == "version_drift" {
			t.Errorf("unexpected version_drift record emitted for no-drift case: %+v", rec)
		}
	}
}

// TestStartDriftSchedulerBoundedConcurrency verifies WR-04: with a tick interval
// much shorter than the drift check duration, the scheduler must NOT pile up
// goroutines — at most one drift check runs at a time. The injected
// metadataFetchFn blocks long enough that many ticks fire while a check is
// in-flight; the in-flight guard must drop those ticks. The test asserts the
// observed peak concurrency never exceeds 1.
func TestStartDriftSchedulerBoundedConcurrency(t *testing.T) {
	h, _ := newDriftTestHandler(t)
	h.cfg.AuditPath = "" // suppress audit writes; we only care about concurrency
	h.cfg.Nudge.MajorDriftCheck.Enabled = true
	// Tiny interval so many ticks fire during a single slow fetch.
	h.cfg.Nudge.MajorDriftCheck.Interval = "5ms"

	var inFlight atomic.Int32
	var peak atomic.Int32
	var calls atomic.Int32

	orig := metadataFetchFn
	metadataFetchFn = func(_ context.Context) (map[string]string, error) {
		calls.Add(1)
		cur := inFlight.Add(1)
		// Track the peak number of concurrent in-flight fetches.
		for {
			p := peak.Load()
			if cur <= p || peak.CompareAndSwap(p, cur) {
				break
			}
		}
		// Hold the check in-flight far longer than the tick interval so the
		// scheduler would pile up unbounded goroutines without the guard.
		time.Sleep(60 * time.Millisecond)
		inFlight.Add(-1)
		return map[string]string{}, nil
	}
	defer func() { metadataFetchFn = orig }()

	ctx, cancel := context.WithCancel(context.Background())
	startDriftScheduler(ctx, h)

	// Let the scheduler tick many times while checks are slow.
	time.Sleep(300 * time.Millisecond)
	cancel()
	// Allow the final in-flight check to drain.
	time.Sleep(100 * time.Millisecond)

	if p := peak.Load(); p > 1 {
		t.Errorf("peak concurrent drift checks = %d, want <= 1 (in-flight guard failed — WR-04)", p)
	}
	if calls.Load() == 0 {
		t.Error("expected at least one drift check to run during the test window")
	}
}

// TestStartDriftSchedulerDisabled verifies that the scheduler does nothing when
// the drift check is disabled in config (no goroutine, no fetch).
func TestStartDriftSchedulerDisabled(t *testing.T) {
	h, _ := newDriftTestHandler(t)
	h.cfg.Nudge.MajorDriftCheck.Enabled = false
	h.cfg.Nudge.MajorDriftCheck.Interval = "5ms"

	var calls atomic.Int32
	orig := metadataFetchFn
	metadataFetchFn = func(_ context.Context) (map[string]string, error) {
		calls.Add(1)
		return map[string]string{}, nil
	}
	defer func() { metadataFetchFn = orig }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startDriftScheduler(ctx, h)

	time.Sleep(50 * time.Millisecond)
	if calls.Load() != 0 {
		t.Errorf("disabled scheduler ran %d drift checks, want 0", calls.Load())
	}
}

// TestCheckDriftFetchError verifies that a metadataFetchFn error does not emit
// any record and does not panic or block (T-08-24 — fail-open, non-blocking).
func TestCheckDriftFetchError(t *testing.T) {
	h, auditPath := newDriftTestHandler(t)

	// Inject: fetch always errors.
	orig := metadataFetchFn
	metadataFetchFn = func(_ context.Context) (map[string]string, error) {
		return nil, errors.New("registry unreachable")
	}
	defer func() { metadataFetchFn = orig }()

	// Must not panic.
	h.checkDrift(context.Background())

	// No audit record should be written on fetch error.
	records := readAllAuditRecords(t, auditPath)
	for _, rec := range records {
		if rec.RecordType == "version_drift" {
			t.Errorf("version_drift record emitted on fetch error (should be suppressed): %+v", rec)
		}
	}
}
