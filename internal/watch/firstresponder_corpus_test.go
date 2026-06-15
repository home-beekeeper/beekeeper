package watch_test

// Corpus-adjudication integration tests for RunFirstResponder (FRB-01/02/04).
//
// These tests extend internal/watch/firstresponder_test.go with five test
// functions that cover the corpus-signal path added in Phase 24 Plan 02.
//
// RED phase: the FirstResponderConfig corpus fields (CorpusPath, CorpusEnabled,
// CorpusSentryThreshold) do not exist yet — these tests fail to compile until
// Task 2 adds them. That is intentional (TDD RED gate).
//
// Import boundary: MUST NOT import internal/tui. Audit records are read via
// os.ReadFile on the NDJSON file, not via the TUI model.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bantuson/beekeeper/internal/audit"
	"github.com/bantuson/beekeeper/internal/corpus"
	"github.com/bantuson/beekeeper/internal/policy"
	"github.com/bantuson/beekeeper/internal/quarantine"
	"github.com/bantuson/beekeeper/internal/watch"
)

// seedCorpusFile writes a single CorpusRecord as an NDJSON line to a temp
// file and returns its path. The record uses the Nx Console fixture from
// RESEARCH.md §Synthetic Nx Console Incident Fixture.
func seedCorpusFile(t *testing.T, rec corpus.CorpusRecord) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "beekeeper-corpus.ndjson")
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("seedCorpusFile: marshal: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("seedCorpusFile: write: %v", err)
	}
	return path
}

// nxConsoleRecord returns the synthetic Nx Console corpus record (enforce-tier,
// source_count=2, malicious) from RESEARCH.md §Synthetic Nx Console Incident Fixture.
func nxConsoleRecord(sourceCount int) corpus.CorpusRecord {
	return corpus.CorpusRecord{
		AuditRecord: audit.AuditRecord{
			RecordID:   "test-nx-console-001",
			ClusterID:  "nx-console-cluster-001",
			ToolName:   "@nrwl/nx-console",
			RecordType: "policy_decision",
			Decision:   "block",
			Timestamp:  time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339),
		},
		TrueLabel:           "malicious",
		AdjudicationSource:  "catalog_confirmation",
		CorpusSchemaVersion: "1.0",
		PushEnvelope: &corpus.PushEnvelope{
			Signature: corpus.EnvelopeSignature{
				PackageOrExtensionID: "npm:@nrwl/nx-console",
				Version:              "17.3.0",
			},
			TrueLabel:      "malicious",
			ConfidenceTier: "enforce",
			SourceCount:    sourceCount,
			ActionHint:     corpus.ActionHintWatchAndBlock,
		},
	}
}

// nxConsoleScanHits returns a matching ScanHit for @nrwl/nx-console with a
// resolved install path at pkgDir.
func nxConsoleScanHits(pkgDir string) []watch.ScanHit {
	pathResolved := pkgDir != ""
	return []watch.ScanHit{
		{
			Ecosystem:          "npm",
			Package:            "@nrwl/nx-console",
			Version:            "17.3.0",
			InstalledPath:      pkgDir,
			PathResolved:       pathResolved,
			CorroborationCount: 2,
			Decision: policy.Decision{
				Level:  "block",
				Reason: "corpus-adjudicated malicious package",
			},
		},
	}
}

// readAuditRecords reads all NDJSON lines from auditPath and returns a slice
// of AuditRecord structs. Missing file returns nil (not an error).
func readAuditRecords(t *testing.T, auditPath string) []audit.AuditRecord {
	t.Helper()
	data, err := os.ReadFile(auditPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("readAuditRecords: %v", err)
	}
	var out []audit.AuditRecord
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var rec audit.AuditRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		out = append(out, rec)
	}
	return out
}

// TestFirstResponderCorpusMaliciousArmsCard verifies FRB-01:
//
// A confirmed-malicious corpus record (enforce-tier, npm:@nrwl/nx-console) that
// matches a locally installed package arms the TUI quarantine card by:
//   - writing a "catalog_quarantine" audit record, and
//   - physically moving the artifact into the quarantine directory.
func TestFirstResponderCorpusMaliciousArmsCard(t *testing.T) {
	quarantineDir := t.TempDir()
	auditPath := filepath.Join(t.TempDir(), "beekeeper.ndjson")
	sentryPath := filepath.Join(t.TempDir(), "sentry-targets.json")

	// Seed an install directory for the package.
	pkgDir := filepath.Join(t.TempDir(), "nx-console-17.3.0")
	if err := os.MkdirAll(pkgDir, 0o700); err != nil {
		t.Fatalf("mkdir pkgDir: %v", err)
	}

	corpusPath := seedCorpusFile(t, nxConsoleRecord(2))

	cfg := watch.FirstResponderConfig{
		// Disable the scan-hit auto-quarantine so only the corpus path moves the
		// artifact. This ensures the catalog_quarantine record comes from the
		// corpus path specifically (FRB-01), not the existing scan-hit path.
		Enabled:               false,
		DryRun:                false,
		Threshold:             2,
		QuarantineDir:         quarantineDir,
		AuditPath:             auditPath,
		SentryTargetsPath:     sentryPath,
		CorpusPath:            corpusPath,
		CorpusEnabled:         true,
		CorpusSentryThreshold: 2,
		// CrossRefFn returns the matching scan hit so the corpus path can resolve
		// the install path of @nrwl/nx-console.
		CrossRefFn: func(_ context.Context, _ watch.CrossRefConfig) ([]watch.ScanHit, error) {
			return nxConsoleScanHits(pkgDir), nil
		},
	}

	if err := watch.RunFirstResponder(context.Background(), cfg); err != nil {
		t.Fatalf("RunFirstResponder: %v", err)
	}

	// Assert: audit log contains a "catalog_quarantine" record referencing the package.
	recs := readAuditRecords(t, auditPath)
	found := false
	for _, r := range recs {
		if r.RecordType == "catalog_quarantine" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("audit log must contain a catalog_quarantine record; got %d records", len(recs))
		for _, r := range recs {
			t.Logf("  record_type=%q tool_name=%q", r.RecordType, r.ToolName)
		}
	}

	// Assert: the quarantine directory has at least one entry (artifact moved).
	entries, err := quarantine.List(quarantineDir)
	if err != nil {
		t.Fatalf("quarantine.List: %v", err)
	}
	if len(entries) == 0 {
		t.Error("quarantine dir must have at least one entry after corpus-adjudication quarantine")
	}
}

// TestFirstResponderCorpusSentryGate verifies FRB-04:
//
// A malicious corpus record with SourceCount >= CorpusSentryThreshold (=2) causes
// the package to be added to sentry-targets.json.
func TestFirstResponderCorpusSentryGate(t *testing.T) {
	auditPath := filepath.Join(t.TempDir(), "beekeeper.ndjson")
	sentryPath := filepath.Join(t.TempDir(), "sentry-targets.json")

	corpusPath := seedCorpusFile(t, nxConsoleRecord(2)) // source_count=2 >= threshold=2

	cfg := watch.FirstResponderConfig{
		Enabled:               false, // auto-quarantine disabled; corpus path still runs
		DryRun:                true,
		Threshold:             2,
		QuarantineDir:         t.TempDir(),
		AuditPath:             auditPath,
		SentryTargetsPath:     sentryPath,
		CorpusPath:            corpusPath,
		CorpusEnabled:         true,
		CorpusSentryThreshold: 2,
		CrossRefFn: func(_ context.Context, _ watch.CrossRefConfig) ([]watch.ScanHit, error) {
			return nxConsoleScanHits(""), nil // no local install path needed for sentry gate
		},
	}

	if err := watch.RunFirstResponder(context.Background(), cfg); err != nil {
		t.Fatalf("RunFirstResponder: %v", err)
	}

	data, err := os.ReadFile(sentryPath)
	if err != nil {
		t.Fatalf("read sentry-targets.json: %v", err)
	}
	if !strings.Contains(string(data), "@nrwl/nx-console") {
		t.Errorf("sentry-targets.json must contain @nrwl/nx-console (source_count=2 >= threshold=2);\ngot:\n%s", string(data))
	}
}

// TestFirstResponderCorpusSingleSourceNoSentry verifies FRB-04 (negative gate):
//
// A malicious corpus record with SourceCount=1 (watch tier) below the
// CorpusSentryThreshold (=2) must NOT add a Sentry target via the corpus path.
// The CrossRefFn returns no scan hits so the scan-hit path does not interfere.
func TestFirstResponderCorpusSingleSourceNoSentry(t *testing.T) {
	auditPath := filepath.Join(t.TempDir(), "beekeeper.ndjson")
	sentryPath := filepath.Join(t.TempDir(), "sentry-targets.json")

	corpusPath := seedCorpusFile(t, nxConsoleRecord(1)) // source_count=1 < threshold=2

	cfg := watch.FirstResponderConfig{
		// Scan-hit path disabled; no scan hits returned — isolates the corpus path.
		Enabled:               false,
		DryRun:                true,
		Threshold:             2,
		QuarantineDir:         t.TempDir(),
		AuditPath:             auditPath,
		SentryTargetsPath:     sentryPath,
		CorpusPath:            corpusPath,
		CorpusEnabled:         true,
		CorpusSentryThreshold: 2,
		// Return no scan hits: the corpus path should be the only Sentry-target
		// writer. With SourceCount=1 it must not add any target.
		CrossRefFn: func(_ context.Context, _ watch.CrossRefConfig) ([]watch.ScanHit, error) {
			return nil, nil
		},
	}

	if err := watch.RunFirstResponder(context.Background(), cfg); err != nil {
		t.Fatalf("RunFirstResponder: %v", err)
	}

	// Sentry file may be absent or present with 0 targets — both are acceptable.
	data, err := os.ReadFile(sentryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return // absent = no target recorded — correct
		}
		t.Fatalf("read sentry-targets.json: %v", err)
	}
	if strings.Contains(string(data), "@nrwl/nx-console") {
		t.Errorf("single-source (SourceCount=1) must NOT add a Sentry target;\ngot:\n%s", string(data))
	}
}

// TestFirstResponderCorpusNoPurge verifies FRB-02 (behavioral half):
//
// After a corpus-adjudication quarantine (MoveTyped), the artifact must still
// be present in the quarantine directory (reversible). A Purge call would
// delete the artifact — this test asserts no purge occurred.
func TestFirstResponderCorpusNoPurge(t *testing.T) {
	quarantineDir := t.TempDir()
	auditPath := filepath.Join(t.TempDir(), "beekeeper.ndjson")
	sentryPath := filepath.Join(t.TempDir(), "sentry-targets.json")

	pkgDir := filepath.Join(t.TempDir(), "nx-console-nopurge")
	if err := os.MkdirAll(pkgDir, 0o700); err != nil {
		t.Fatalf("mkdir pkgDir: %v", err)
	}

	corpusPath := seedCorpusFile(t, nxConsoleRecord(2))

	cfg := watch.FirstResponderConfig{
		// Disable the scan-hit auto-quarantine so only the corpus path acts.
		// This ensures the artifact is moved exactly once (by the corpus path).
		Enabled:               false,
		DryRun:                false,
		Threshold:             2,
		QuarantineDir:         quarantineDir,
		AuditPath:             auditPath,
		SentryTargetsPath:     sentryPath,
		CorpusPath:            corpusPath,
		CorpusEnabled:         true,
		CorpusSentryThreshold: 2,
		CrossRefFn: func(_ context.Context, _ watch.CrossRefConfig) ([]watch.ScanHit, error) {
			return nxConsoleScanHits(pkgDir), nil
		},
	}

	if err := watch.RunFirstResponder(context.Background(), cfg); err != nil {
		t.Fatalf("RunFirstResponder: %v", err)
	}

	// FRB-02: the entry must still be present in the quarantine directory.
	// If Purge had been called, quarantine.List would return 0 entries.
	entries, err := quarantine.List(quarantineDir)
	if err != nil {
		t.Fatalf("quarantine.List: %v", err)
	}
	if len(entries) == 0 {
		t.Error("FRB-02 violation: quarantine dir is empty — either no artifact was quarantined or Purge was called (reversible MoveTyped must leave the entry in place)")
	}
}

// TestFirstResponderCorpusPendingQuarantine verifies FRB-01 (pending case):
//
// A malicious corpus record whose package is NOT locally installed (no matching
// ScanHit with a resolved path) produces a "pending-quarantine" audit record
// and leaves the quarantine directory empty (nothing to move).
func TestFirstResponderCorpusPendingQuarantine(t *testing.T) {
	quarantineDir := t.TempDir()
	auditPath := filepath.Join(t.TempDir(), "beekeeper.ndjson")
	sentryPath := filepath.Join(t.TempDir(), "sentry-targets.json")

	corpusPath := seedCorpusFile(t, nxConsoleRecord(2))

	cfg := watch.FirstResponderConfig{
		Enabled:               true,
		DryRun:                false,
		Threshold:             2,
		QuarantineDir:         quarantineDir,
		AuditPath:             auditPath,
		SentryTargetsPath:     sentryPath,
		CorpusPath:            corpusPath,
		CorpusEnabled:         true,
		CorpusSentryThreshold: 2,
		// CrossRefFn returns NO hits — the package is not locally installed.
		CrossRefFn: func(_ context.Context, _ watch.CrossRefConfig) ([]watch.ScanHit, error) {
			return nil, nil
		},
	}

	if err := watch.RunFirstResponder(context.Background(), cfg); err != nil {
		t.Fatalf("RunFirstResponder: %v", err)
	}

	// Assert: audit log contains a "pending-quarantine" record.
	recs := readAuditRecords(t, auditPath)
	found := false
	for _, r := range recs {
		if r.RecordType == "pending-quarantine" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("audit log must contain a pending-quarantine record when package is not locally installed; got %d records", len(recs))
		for _, r := range recs {
			t.Logf("  record_type=%q tool_name=%q", r.RecordType, r.ToolName)
		}
	}

	// Assert: quarantine dir is empty (nothing to move when path is unresolved).
	entries, err := quarantine.List(quarantineDir)
	if err != nil {
		t.Fatalf("quarantine.List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("quarantine dir must be empty when no local install found; got %d entries", len(entries))
	}
}

// TestFirstResponderCorpusDryRunPathResolvedNoMove verifies finding #3: the
// corpus-adjudication move branch honors cfg.DryRun. This is the PATH-RESOLVED
// case — a matching ScanHit gives the corpus record a real install path, so the
// only thing that prevents the move is the DryRun gate inside the function.
//
// Distinct from TestFirstResponderCorpusSentryGate, which uses a path-UNRESOLVED
// record (the move is skipped because there is nothing to move, not because of
// DryRun). Here the artifact dir exists and is resolvable: a "would-quarantine"
// record must be written AND the artifact must remain in place (not moved).
func TestFirstResponderCorpusDryRunPathResolvedNoMove(t *testing.T) {
	quarantineDir := t.TempDir()
	auditPath := filepath.Join(t.TempDir(), "beekeeper.ndjson")
	sentryPath := filepath.Join(t.TempDir(), "sentry-targets.json")

	// A real install directory the package resolves to. With DryRun it must stay.
	pkgDir := filepath.Join(t.TempDir(), "nx-console-dryrun")
	if err := os.MkdirAll(pkgDir, 0o700); err != nil {
		t.Fatalf("mkdir pkgDir: %v", err)
	}
	sentinel := filepath.Join(pkgDir, "package.json")
	if err := os.WriteFile(sentinel, []byte(`{"name":"@nrwl/nx-console"}`), 0o600); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	corpusPath := seedCorpusFile(t, nxConsoleRecord(2))

	cfg := watch.FirstResponderConfig{
		Enabled:               false, // scan-hit path disabled; isolate the corpus path
		DryRun:                true,  // finding #3: corpus move branch must honor this
		Threshold:             2,
		QuarantineDir:         quarantineDir,
		AuditPath:             auditPath,
		SentryTargetsPath:     sentryPath,
		CorpusPath:            corpusPath,
		CorpusEnabled:         true,
		CorpusSentryThreshold: 2,
		// PATH-RESOLVED: the scan hit supplies a real install path.
		CrossRefFn: func(_ context.Context, _ watch.CrossRefConfig) ([]watch.ScanHit, error) {
			return nxConsoleScanHits(pkgDir), nil
		},
	}

	if err := watch.RunFirstResponder(context.Background(), cfg); err != nil {
		t.Fatalf("RunFirstResponder: %v", err)
	}

	// Assert: the artifact was NOT moved — the install dir and its sentinel remain.
	if _, statErr := os.Stat(sentinel); statErr != nil {
		t.Errorf("finding #3 violation: artifact was moved under DryRun — sentinel missing: %v", statErr)
	}
	entries, err := quarantine.List(quarantineDir)
	if err != nil {
		t.Fatalf("quarantine.List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("finding #3 violation: quarantine dir has %d entries under DryRun; want 0 (no move)", len(entries))
	}

	// Assert: a "would-quarantine" record was written (the dry-run audit path).
	recs := readAuditRecords(t, auditPath)
	found := false
	for _, r := range recs {
		if r.RecordType == "would-quarantine" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("audit log must contain a would-quarantine record under DryRun; got %d records", len(recs))
		for _, r := range recs {
			t.Logf("  record_type=%q tool_name=%q", r.RecordType, r.ToolName)
		}
	}
}
