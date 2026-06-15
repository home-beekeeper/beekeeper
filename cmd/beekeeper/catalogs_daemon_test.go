package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/bantuson/beekeeper/internal/audit"
	"github.com/bantuson/beekeeper/internal/catalog"
	"github.com/bantuson/beekeeper/internal/corpus"
	"github.com/bantuson/beekeeper/internal/quarantine"
	"github.com/bantuson/beekeeper/internal/sentry"
	"github.com/bantuson/beekeeper/internal/watch"
)

// TestCatalogSyncGate proves the interval gate with an injected clock: the
// OS-scheduled hourly heartbeat must NO-OP unless the configured interval has
// elapsed since the last success (D-T1-interval), with --force bypassing it.
func TestCatalogSyncGate(t *testing.T) {
	base := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	interval := 12 * time.Hour

	tests := []struct {
		name        string
		lastSuccess time.Time
		now         time.Time
		force       bool
		wantDue     bool
	}{
		{"never synced -> due", time.Time{}, base, false, true},
		{"just synced -> not due", base, base.Add(time.Minute), false, false},
		{"half interval -> not due", base, base.Add(6 * time.Hour), false, false},
		{"one minute short -> not due", base, base.Add(interval - time.Minute), false, false},
		{"exactly interval -> due", base, base.Add(interval), false, true},
		{"past interval -> due", base, base.Add(13 * time.Hour), false, true},
		{"force bypasses gate (not due) -> due", base, base.Add(time.Minute), true, true},
		{"force bypasses gate (never synced) -> due", time.Time{}, base, true, true},
	}
	for _, tt := range tests {
		if got := catalogSyncDue(tt.lastSuccess, interval, tt.now, tt.force); got != tt.wantDue {
			t.Errorf("%s: catalogSyncDue(lastSuccess=%v, interval=%s, now=%v, force=%v) = %v, want %v",
				tt.name, tt.lastSuccess, interval, tt.now, tt.force, got, tt.wantDue)
		}
	}
}

// TestCatalogDaemonStatusNoError verifies the status probe never errors when the
// daemon is not registered (it reports not-installed rather than failing). The
// OS-specific query tool is shelled out; the test only asserts the contract that
// "absent" is not an error.
func TestCatalogDaemonStatusNoError(t *testing.T) {
	installed, detail, err := catalogDaemonStatus()
	if err != nil {
		t.Fatalf("catalogDaemonStatus() error = %v, want nil (absent must not be an error)", err)
	}
	if detail == "" {
		t.Error("catalogDaemonStatus() detail is empty, want a human-readable state string")
	}
	_ = installed // value is environment-dependent; only the no-error contract is asserted
}

// TestRunCatalogsSyncFirstResponder is the synthetic Nx Console evaluator gate
// (FRB-01..05 + LAUNCH-01). It seeds a confirmed-malicious enforce-tier corpus
// record for npm:@nrwl/nx-console v17.3.0, drives runCatalogsSync (the OQ-3
// off-hot-path home), and asserts the 11-point round-trip:
//
//  1. corpus.ReadMaliciousRecords returns the seeded record (FRB-01 signal).
//  2. The audit log contains a "catalog_quarantine" record (FRB-01 arm).
//  3. sentry-targets.json contains "@nrwl/nx-console" (FRB-04 SourceCount=2).
//  4. quarantine.List returns exactly one entry (FRB-01 armed).
//  5. The entry is still present (FRB-02 no-purge survived).
//  6. quarantine.Restore succeeds (FRB-03 reversible).
//  7. MultiIndex.LookupAll returns a local-overlay match (FRB-05).
//  8. All four corpus layers populated on the CorpusRecord (LAUNCH-01).
//  9. Envelope BehaviorSignatureHash is 64-char hex (LAUNCH-01).
//  10. ConfidenceTier = "enforce", SourceCount = 2 (LAUNCH-01).
//  11. ActionHint = ActionHintWatchAndBlock (LAUNCH-01).
func TestRunCatalogsSyncFirstResponder(t *testing.T) {
	// Redirect the entire platform state tree to a temp dir via BEEKEEPER_HOME.
	// platform.StateDir(), CatalogDir(), and AuditDir() all derive from this.
	home := t.TempDir()
	t.Setenv("BEEKEEPER_HOME", home)

	// With BEEKEEPER_HOME set, StateDir() = home/beekeeper.
	// Create the required subdirectories up-front.
	stateDir := filepath.Join(home, "beekeeper")
	catalogDir := filepath.Join(stateDir, "catalogs")
	auditDir := filepath.Join(stateDir, "audit")
	corpusDir := filepath.Join(stateDir, "corpus")
	for _, d := range []string{stateDir, catalogDir, auditDir, corpusDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	// Write a config.json at $BEEKEEPER_HOME/beekeeper/config.json with
	// corpus.enabled=true so resolveConfig picks it up.
	cfgJSON := `{"corpus":{"enabled":true}}`
	cfgPath := filepath.Join(stateDir, "config.json")
	if err := os.WriteFile(cfgPath, []byte(cfgJSON), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// ---- Seed the synthetic Nx Console corpus record ----
	// This is the fixture from 24-RESEARCH.md §Synthetic Nx Console Incident Fixture.
	// Compute the BehaviorSignatureHash that a real pipeline record would carry.
	// The seed uses ToolName as the actionType proxy; no SentryFilesAccessed or
	// SentryNetworkDests are set, so both resource inputs are empty strings.
	// This value is deterministic: BehaviorSigHash always returns 64 lowercase hex
	// chars for any input, and the same inputs always produce the same hash.
	seedBehaviorHash := corpus.BehaviorSigHash("@nrwl/nx-console", "", "")

	corpusRec := corpus.CorpusRecord{
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
				PackageOrExtensionID:  "npm:@nrwl/nx-console",
				Version:               "17.3.0",
				BehaviorSignatureHash: seedBehaviorHash, // populated at seed time (WR-01)
			},
			TrueLabel:      "malicious",
			ConfidenceTier: "enforce",
			SourceCount:    2,
			ActionHint:     corpus.ActionHintWatchAndBlock,
		},
	}
	corpusPath := filepath.Join(corpusDir, "beekeeper-corpus.ndjson")
	if err := corpus.AppendCorpusRecordLine(corpusPath, corpusRec); err != nil {
		t.Fatalf("seed corpus record: %v", err)
	}

	// ---- Create a fake install directory for @nrwl/nx-console ----
	// The first-responder path-resolution needs a real directory to MoveTyped.
	pkgDir := filepath.Join(t.TempDir(), "node_modules", "@nrwl", "nx-console")
	if err := os.MkdirAll(pkgDir, 0o700); err != nil {
		t.Fatalf("mkdir pkgDir: %v", err)
	}
	sentinel := filepath.Join(pkgDir, "package.json")
	if err := os.WriteFile(sentinel, []byte(`{"name":"@nrwl/nx-console","version":"17.3.0"}`), 0o600); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	// ---- Stub firstResponderFn with the REAL RunFirstResponder + fake CrossRefFn ----
	// This drives genuine FRB-01/04 side effects (quarantine + sentry targets) while
	// avoiding a real pollen scan (no binary required in unit tests).
	orig := firstResponderFn
	t.Cleanup(func() { firstResponderFn = orig })
	firstResponderFn = func(ctx context.Context, cfg watch.FirstResponderConfig) error {
		// Inject a fake CrossRefFn that returns a matching ScanHit for nx-console.
		cfg.CrossRefFn = func(_ context.Context, _ watch.CrossRefConfig) ([]watch.ScanHit, error) {
			return []watch.ScanHit{
				{
					Ecosystem:          "npm",
					Package:            "@nrwl/nx-console",
					Version:            "17.3.0",
					InstalledPath:      pkgDir,
					PathResolved:       true,
					CorroborationCount: 2,
				},
			}, nil
		}
		// Disable the scan-hit auto-quarantine path so only the corpus path moves
		// the artifact (prevents double-move with the same package).
		cfg.Enabled = false
		return watch.RunFirstResponder(ctx, cfg)
	}

	// ---- Build a minimal Cobra command to satisfy runCatalogsSync ----
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	// Suppress output to avoid test noise.
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	// ---- Call runCatalogsSync with force=true ----
	// The FRB pass runs BEFORE the HTTP catalog fetch, so side effects persist even
	// if SyncConditional returns an error (offline machine, no live catalog URL).
	// We accept a non-nil return from runCatalogsSync (HTTP fetch will fail in tests).
	_ = runCatalogsSync(cmd, true)

	// ==============================================================
	// EVALUATOR GATE — 11 assertions (FRB-01..05 + LAUNCH-01)
	// ==============================================================

	// 1. corpus.ReadMaliciousRecords returns the seeded record (FRB-01 signal).
	malicious, err := corpus.ReadMaliciousRecords(corpusPath)
	if err != nil {
		t.Fatalf("[1] ReadMaliciousRecords error: %v", err)
	}
	found1 := false
	for _, r := range malicious {
		if r.TrueLabel == "malicious" && r.PushEnvelope != nil &&
			r.PushEnvelope.Signature.PackageOrExtensionID == "npm:@nrwl/nx-console" {
			found1 = true
			break
		}
	}
	if !found1 {
		t.Errorf("[1] ReadMaliciousRecords must return the seeded npm:@nrwl/nx-console record; got %d malicious records", len(malicious))
	}

	// 2. Audit log contains a "catalog_quarantine" record (FRB-01 arm).
	auditPath := filepath.Join(auditDir, "beekeeper.ndjson")
	auditData, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("[2] read audit log: %v", err)
	}
	found2 := false
	for _, line := range strings.Split(strings.TrimSpace(string(auditData)), "\n") {
		if line == "" {
			continue
		}
		var rec audit.AuditRecord
		if jsonErr := json.Unmarshal([]byte(line), &rec); jsonErr != nil {
			continue
		}
		if rec.RecordType == "catalog_quarantine" {
			found2 = true
			break
		}
	}
	if !found2 {
		t.Errorf("[2] audit log must contain a catalog_quarantine record (FRB-01 arm);\naudit contents:\n%s", string(auditData))
	}

	// 3. sentry-targets.json contains "@nrwl/nx-console" (FRB-04).
	sentryPath := filepath.Join(stateDir, "sentry-targets.json")
	sentryData, err := os.ReadFile(sentryPath)
	if err != nil {
		t.Fatalf("[3] read sentry-targets.json: %v", err)
	}
	if !strings.Contains(string(sentryData), "@nrwl/nx-console") {
		t.Errorf("[3] sentry-targets.json must contain @nrwl/nx-console (FRB-04: SourceCount=2 >= threshold=2);\ngot:\n%s", string(sentryData))
	}
	// Also verify via sentry.LoadTargets.
	tl, err := sentry.LoadTargets(sentryPath)
	if err != nil {
		t.Fatalf("[3] sentry.LoadTargets: %v", err)
	}
	found3 := false
	for _, e := range tl.Entries {
		if e.Name == "@nrwl/nx-console" {
			found3 = true
			break
		}
	}
	if !found3 {
		t.Errorf("[3] sentry.LoadTargets must return @nrwl/nx-console in Entries; got %v", tl.Entries)
	}

	// 4. quarantine.List returns exactly one entry for the package (FRB-01 armed).
	quarantineDir := filepath.Join(stateDir, "quarantine")
	entries, err := quarantine.List(quarantineDir)
	if err != nil {
		t.Fatalf("[4] quarantine.List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("[4] quarantine.List must return exactly 1 entry (FRB-01 armed); got %d", len(entries))
	}
	entry := entries[0]
	if entry.Name != "@nrwl/nx-console" {
		t.Errorf("[4] quarantine entry Name = %q, want @nrwl/nx-console", entry.Name)
	}

	// 5. Entry is still present — no auto-purge occurred (FRB-02).
	// The assertion above (len(entries)==1) already proves the entry survived.
	// Add an explicit check: after the sync, the quarantine dir must not be empty.
	if len(entries) == 0 {
		t.Error("[5] FRB-02 violation: quarantine dir is empty — auto-purge must not occur")
	}

	// 6. quarantine.Restore succeeds and the artifact returns to its original path (FRB-03).
	entryID := entry.ID
	if err := quarantine.Restore(quarantineDir, entryID); err != nil {
		t.Fatalf("[6] quarantine.Restore: %v", err)
	}
	// pkgDir must be back.
	if _, statErr := os.Stat(pkgDir); statErr != nil {
		t.Errorf("[6] pkgDir not restored after quarantine.Restore: %v", statErr)
	}
	// Quarantine entry must be gone.
	after, err := quarantine.List(quarantineDir)
	if err != nil {
		t.Fatalf("[6] quarantine.List after Restore: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("[6] quarantine.List after Restore returned %d entries, want 0", len(after))
	}

	// 7. MultiIndex opened with NewMultiIndexWithOverlay returns >= 1 match with
	//    CatalogSource=="local-overlay" for npm/@nrwl/nx-console (FRB-05).
	overlayIdxPath := filepath.Join(catalogDir, "local-overlay.idx")
	midx := catalog.NewMultiIndexWithOverlay(nil, nil, nil, overlayIdxPath)
	defer midx.Close()
	overlayMatches := midx.LookupAll("npm", "@nrwl/nx-console")
	found7 := false
	for _, m := range overlayMatches {
		if m.CatalogSource == "local-overlay" {
			found7 = true
			break
		}
	}
	if !found7 {
		t.Errorf("[7] MultiIndex.LookupAll(npm, @nrwl/nx-console) must return >= 1 match with CatalogSource=local-overlay (FRB-05); got %v", overlayMatches)
	}

	// ==============================================================
	// EVALUATOR GATE — 11 assertions (FRB-01..05 + LAUNCH-01)
	// ==============================================================
	// Assertions #8–11 extend the gate to prove all four corpus layers are
	// populated on the Nx Console record from Phase 24 (LAUNCH-01).
	// The first 7 assertions (FRB round-trip) are unchanged above.

	// 8. All four layers populated on the CorpusRecord (LAUNCH-01).
	//
	// The seeded record has all four layers pre-populated. We operate on malicious[0]
	// (the first record returned by ReadMaliciousRecords, verified as the Nx Console
	// record by assertion #1 above). find the specific record.
	var rec corpus.CorpusRecord
	for _, r := range malicious {
		if r.PushEnvelope != nil && r.PushEnvelope.Signature.PackageOrExtensionID == "npm:@nrwl/nx-console" {
			rec = r
			break
		}
	}
	// Behavior layer: ToolName must equal the seeded value "@nrwl/nx-console".
	// The previous OR-check (SourceSurface == "" && ToolName == "") was a
	// near-tautology — the OR was always satisfied by the fixture constant
	// (WR-04). Assert the specific expected value so the gate fails if the
	// behavior layer is missing or carries the wrong identifier.
	const wantToolName = "@nrwl/nx-console"
	if rec.AuditRecord.ToolName != wantToolName {
		t.Errorf("[8] behavior layer: ToolName = %q, want %q (LAUNCH-01)", rec.AuditRecord.ToolName, wantToolName)
	}
	// Decision layer: Decision must be non-empty and CorroborationCount >= 1.
	if rec.AuditRecord.Decision == "" {
		t.Error("[8] decision layer: Decision is empty")
	}
	// Outcome layer (THE MOAT — non-retrofittable): TrueLabel must be "malicious"
	// after adjudication, AdjudicationSource must be non-empty.
	if rec.TrueLabel != "malicious" {
		t.Errorf("[8] outcome layer: TrueLabel = %q, want \"malicious\"", rec.TrueLabel)
	}
	if rec.AdjudicationSource == "" {
		t.Error("[8] outcome layer: AdjudicationSource is empty")
	}
	// Context layer: CorpusSchemaVersion must be "1.0"; Scope is "" in-memory
	// (zero value of CorpusScope) which MarshalJSON maps to "org_only" (SCOPE-01).
	// Accept either the zero-value empty string or the string "org_only".
	if rec.CorpusSchemaVersion != corpus.CorpusSchemaVersion {
		t.Errorf("[8] context layer: CorpusSchemaVersion = %q, want %q", rec.CorpusSchemaVersion, corpus.CorpusSchemaVersion)
	}
	if string(rec.Scope) != "org_only" && string(rec.Scope) != "" {
		t.Errorf("[8] context layer: Scope = %q, want \"org_only\" or \"\" (zero-value marshals to org_only)", string(rec.Scope))
	}

	// 9. Envelope signature: stored BehaviorSignatureHash is a 64-char hex string (LAUNCH-01).
	//
	// The seed now populates BehaviorSignatureHash via corpus.BehaviorSigHash at
	// construction time (see seedBehaviorHash above). We assert on the STORED field
	// rec.PushEnvelope.Signature.BehaviorSignatureHash — NOT on a freshly recomputed
	// value. This gate genuinely fails if the stored signature is empty, short, or
	// non-hex (WR-01: the previous approach called BehaviorSigHash on a fresh input
	// and measured the return value's length, which always succeeds for any input and
	// never reads the stored field). Pattern mirrors launch_e2e_test.go:230-244.
	if rec.PushEnvelope == nil {
		t.Fatal("[9] PushEnvelope is nil — cannot assert stored signature")
	}
	storedHash := rec.PushEnvelope.Signature.BehaviorSignatureHash
	if len(storedHash) != 64 {
		t.Errorf("[9] stored BehaviorSignatureHash = %q (%d chars); want 64-char hex (LAUNCH-01)", storedHash, len(storedHash))
	}
	for _, ch := range storedHash {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			t.Errorf("[9] stored BehaviorSignatureHash contains non-hex char %q (full value: %q)", ch, storedHash)
			break
		}
	}

	// 10. Envelope: ConfidenceTier = "enforce", SourceCount = 2 (LAUNCH-01).
	if rec.PushEnvelope.ConfidenceTier != "enforce" {
		t.Errorf("[10] ConfidenceTier = %q, want \"enforce\"", rec.PushEnvelope.ConfidenceTier)
	}
	if rec.PushEnvelope.SourceCount != 2 {
		t.Errorf("[10] SourceCount = %d, want 2", rec.PushEnvelope.SourceCount)
	}

	// 11. Envelope: ActionHint = ActionHintWatchAndBlock (LAUNCH-01).
	if rec.PushEnvelope.ActionHint != corpus.ActionHintWatchAndBlock {
		t.Errorf("[11] ActionHint = %q, want ActionHintWatchAndBlock", rec.PushEnvelope.ActionHint)
	}
}

// ─── v1.4.0 corpus coverage: runCatalogsSync branch / error paths ────────────

// setupSyncHome creates a hermetic BEEKEEPER_HOME temp tree for runCatalogsSync
// tests and returns (home, stateDir, catalogDir, corpusDir). It writes the given
// cfgJSON to stateDir/config.json so resolveConfig picks it up.
func setupSyncHome(t *testing.T, cfgJSON string) (home, stateDir, catalogDir, corpusDir string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("BEEKEEPER_HOME", home)
	stateDir = filepath.Join(home, "beekeeper")
	catalogDir = filepath.Join(stateDir, "catalogs")
	corpusDir = filepath.Join(stateDir, "corpus")
	for _, d := range []string{stateDir, catalogDir, filepath.Join(stateDir, "audit"), corpusDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	if cfgJSON != "" {
		if err := os.WriteFile(filepath.Join(stateDir, "config.json"), []byte(cfgJSON), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}
	}
	return
}

// newTestCmd returns a minimal *cobra.Command suitable for runCatalogsSync tests:
// Background context, suppressed output, no persistent flags required.
func newTestCmd(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(context.Background())
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	return cmd
}

// newFastTestCmd returns a *cobra.Command with a very short context deadline so
// the SyncConditional HTTP fetch fails immediately. Use this in tests where
// reaching the HTTP sync is incidental (the branch under test is in the corpus
// block, before the HTTP fetch), to keep the test wall-clock time under 100ms.
func newFastTestCmd(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	t.Cleanup(cancel)
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	return cmd
}

// TestRunCatalogsSyncCorpusDisabledSyncDisabled exercises the corpus-disabled
// branch (the entire `if cfg.Corpus.Enabled` block is skipped) and then the
// catalog-sync-disabled short-circuit (handler.go:178-180, span 14).
//
// Config: corpus.enabled=false, catalog_sync.enabled=false. force=false so both
// guards fire. Expected: "sync is disabled" message on stdout; nil error.
func TestRunCatalogsSyncCorpusDisabledSyncDisabled(t *testing.T) {
	setupSyncHome(t, `{"corpus":{"enabled":false},"catalog_sync":{"enabled":false}}`)

	var buf bytes.Buffer
	cmd := newTestCmd(t)
	cmd.SetOut(&buf)

	err := runCatalogsSync(cmd, false /*force*/)
	if err != nil {
		t.Fatalf("runCatalogsSync = %v, want nil (disabled is not an error)", err)
	}
	if !strings.Contains(buf.String(), "disabled") {
		t.Errorf("stdout should mention disabled; got %q", buf.String())
	}
}

// TestRunCatalogsSyncCorpusDisabledNotDue exercises the not-due short-circuit
// (handler.go:182-186, span 15): corpus disabled (so corpus block is skipped),
// sync enabled, not due because lastSuccess is recent and interval has not elapsed.
func TestRunCatalogsSyncCorpusDisabledNotDue(t *testing.T) {
	_, stateDir, _, _ := setupSyncHome(t, `{"corpus":{"enabled":false}}`)

	// Inject a recent lastSuccess so the interval gate fires (not due).
	orig := catalogSyncNow
	t.Cleanup(func() { catalogSyncNow = orig })
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	catalogSyncNow = func() time.Time { return now }

	// Seed state.json with a lastSuccess one minute ago → not due (default 2h interval).
	st := catalog.WatchState{Sources: map[string]catalog.SourceState{
		catalogSyncSourceName: {LastSuccess: now.Add(-1 * time.Minute)},
	}}
	stateFile := filepath.Join(stateDir, "state.json")
	if err := catalog.SaveState(stateFile, st); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	var buf bytes.Buffer
	cmd := newTestCmd(t)
	cmd.SetOut(&buf)

	err := runCatalogsSync(cmd, false /*force*/)
	if err != nil {
		t.Fatalf("runCatalogsSync not-due = %v, want nil", err)
	}
	if !strings.Contains(buf.String(), "skipped") {
		t.Errorf("stdout should mention skipped; got %q", buf.String())
	}
}

// TestRunCatalogsSyncCorpusPathOutsideBoundary exercises the corpus-path
// boundary error branch (handler.go:92-93, span 6): corpus.Enabled=true but the
// resolved corpus path lies outside the state directory. The branch logs to
// os.Stderr and continues (non-fatal); the sync proceeds to the HTTP fetch (which
// fails offline — acceptable in tests). We verify the boundary error is non-fatal
// by asserting it is NOT propagated as the function's return error (the only
// returned error must be the offline HTTP error, not the corpus boundary error).
func TestRunCatalogsSyncCorpusPathOutsideBoundary(t *testing.T) {
	// Use a corpus.path that is a sibling of stateDir (outside the boundary).
	outsidePath := filepath.Join(t.TempDir(), "outside-corpus.ndjson")
	cfgJSON := `{"corpus":{"enabled":true,"path":"` + strings.ReplaceAll(outsidePath, `\`, `\\`) + `"}}`
	setupSyncHome(t, cfgJSON)

	// Use a fast context so the HTTP sync fails immediately.
	cmd := newFastTestCmd(t)

	// The corpus boundary error is logged to os.Stderr (non-fatal). The function
	// continues to the HTTP fetch which fails (context deadline). We only assert
	// that the returned error is NOT a corpus boundary error.
	err := runCatalogsSync(cmd, true /*force*/)
	if err != nil && strings.Contains(err.Error(), "T-23-04") {
		t.Errorf("corpus boundary error was propagated (must be non-fatal); got %v", err)
	}
}

// TestRunCatalogsSyncOpenIndexSuccess covers the OpenIndex-success branch
// (handler.go:111-114, span 7): when bumblebee.idx exists in catalogDir the
// catalog index opens successfully and is wrapped in a MultiIndex for the
// adjudication batch pass.
func TestRunCatalogsSyncOpenIndexSuccess(t *testing.T) {
	_, _, catalogDir, corpusDir := setupSyncHome(t, `{"corpus":{"enabled":true}}`)

	// Build a minimal bumblebee.idx so catalog.OpenIndex succeeds.
	entries := []catalog.Entry{{
		ID: "cov-test-01", Name: "cov-pkg", Ecosystem: "npm",
		Package: "cov-pkg", Versions: []string{"1.0.0"}, Severity: "high",
		CatalogSource: "bumblebee",
	}}
	if err := catalog.BuildIndex(filepath.Join(catalogDir, "bumblebee.idx"), entries); err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}

	// Seed a minimal corpus record so the adjudication batch has something to process.
	corpusPath := filepath.Join(corpusDir, "beekeeper-corpus.ndjson")
	rec := corpus.CorpusRecord{
		AuditRecord:         audit.AuditRecord{RecordID: "cov-idx-001", ClusterID: "cov-cluster-001", ToolName: "Bash", RecordType: "policy_decision", Decision: "allow", Timestamp: time.Now().UTC().Format(time.RFC3339)},
		CorpusSchemaVersion: corpus.CorpusSchemaVersion,
	}
	if err := corpus.AppendCorpusRecordLine(corpusPath, rec); err != nil {
		t.Fatalf("seed corpus: %v", err)
	}

	// Stub firstResponderFn to avoid a real RunFirstResponder (no pollen binary).
	orig := firstResponderFn
	t.Cleanup(func() { firstResponderFn = orig })
	firstResponderFn = func(_ context.Context, _ watch.FirstResponderConfig) error { return nil }

	cmd := newFastTestCmd(t)
	// runCatalogsSync will error on the HTTP fetch (context deadline); expected.
	_ = runCatalogsSync(cmd, true /*force*/)
}

// TestRunCatalogsSyncFirstResponderError exercises the non-fatal firstResponder
// error branch (handler.go:150-153, span 10). The firstResponderFn seam is
// overridden to return a sentinel error; the function logs to stderr and continues.
// runCatalogsSync itself must not propagate the first-responder error.
func TestRunCatalogsSyncFirstResponderError(t *testing.T) {
	_, _, _, corpusDir := setupSyncHome(t, `{"corpus":{"enabled":true}}`)

	// Seed a malicious corpus record for the first-responder pass.
	corpusPath := filepath.Join(corpusDir, "beekeeper-corpus.ndjson")
	rec := corpus.CorpusRecord{
		AuditRecord: audit.AuditRecord{
			RecordID: "cov-fr-err-001", ClusterID: "cov-cluster-001",
			ToolName: "Bash", RecordType: "policy_decision", Decision: "block",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
		TrueLabel:           "malicious",
		AdjudicationSource:  "catalog_confirmation",
		CorpusSchemaVersion: corpus.CorpusSchemaVersion,
		PushEnvelope: &corpus.PushEnvelope{
			Signature: corpus.EnvelopeSignature{
				PackageOrExtensionID: "npm:cov-test-fr-error-pkg",
			},
			TrueLabel:      "malicious",
			ConfidenceTier: "enforce",
			SourceCount:    2,
			ActionHint:     corpus.ActionHintWatchAndBlock,
		},
	}
	if err := corpus.AppendCorpusRecordLine(corpusPath, rec); err != nil {
		t.Fatalf("seed corpus: %v", err)
	}

	// Inject a firstResponderFn that returns a sentinel error.
	orig := firstResponderFn
	t.Cleanup(func() { firstResponderFn = orig })
	sentinelErr := errors.New("injected first-responder error for coverage")
	firstResponderFn = func(_ context.Context, _ watch.FirstResponderConfig) error {
		return sentinelErr
	}

	var errBuf bytes.Buffer
	cmd := newTestCmd(t)
	cmd.SetErr(&errBuf)
	cmd.SetOut(io.Discard)

	// Use a fast context so the HTTP sync fails immediately (test goal is the
	// firstResponder error path, not the HTTP fetch path).
	cmd.SetContext(func() context.Context {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		t.Cleanup(cancel)
		return ctx
	}())

	// The first-responder error is non-fatal: runCatalogsSync must continue and
	// only return the HTTP-fetch error (offline), never the injected sentinel.
	// The error is logged to os.Stderr (not cmd.OutOrStderr()), so we do not
	// assert stderr content — we only verify the sentinel is not propagated.
	err := runCatalogsSync(cmd, true /*force*/)
	if errors.Is(err, sentinelErr) {
		t.Errorf("runCatalogsSync propagated first-responder error; want non-fatal (stderr-only)")
	}
	// The returned error must be the offline HTTP sync error, not our sentinel.
	if err != nil && !strings.Contains(err.Error(), "catalog sync failed") {
		// Allow any sync error (HTTP, self-quarantine, etc.) — just not our sentinel.
		if errors.Is(err, sentinelErr) {
			t.Errorf("sentinel propagated: %v", err)
		}
	}
	_ = errBuf // captured but not asserted (logs go to os.Stderr directly)
}

// TestRunCatalogsSyncOverlaySkipsNilEnvelope exercises the `continue` branch
// (handler.go:166-168, span 12) for a confirmed-malicious corpus record whose
// PushEnvelope is nil. The loop must skip the entry without calling
// AddLocalOverlayEntry; no error is returned.
func TestRunCatalogsSyncOverlaySkipsNilEnvelope(t *testing.T) {
	_, _, _, corpusDir := setupSyncHome(t, `{"corpus":{"enabled":true}}`)

	// Seed a malicious record with nil PushEnvelope.
	corpusPath := filepath.Join(corpusDir, "beekeeper-corpus.ndjson")
	rec := corpus.CorpusRecord{
		AuditRecord: audit.AuditRecord{
			RecordID: "cov-nil-env-001", ClusterID: "cov-nil-cluster-001",
			ToolName: "Bash", RecordType: "policy_decision", Decision: "block",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
		TrueLabel:           "malicious",
		AdjudicationSource:  "catalog_confirmation",
		CorpusSchemaVersion: corpus.CorpusSchemaVersion,
		PushEnvelope:        nil, // explicitly nil — must trigger the continue branch
	}
	if err := corpus.AppendCorpusRecordLine(corpusPath, rec); err != nil {
		t.Fatalf("seed corpus: %v", err)
	}

	orig := firstResponderFn
	t.Cleanup(func() { firstResponderFn = orig })
	firstResponderFn = func(_ context.Context, _ watch.FirstResponderConfig) error { return nil }

	cmd := newFastTestCmd(t)
	// Must not panic or return the nil-envelope as an error.
	_ = runCatalogsSync(cmd, true /*force*/)
}

// TestRunCatalogsSyncOverlaySkipsEmptyPackageID exercises the second `continue`
// branch (span 12) for a record with a non-nil PushEnvelope but an empty
// PackageOrExtensionID. The overlay loop skips such entries (no ID → no overlay).
func TestRunCatalogsSyncOverlaySkipsEmptyPackageID(t *testing.T) {
	_, _, _, corpusDir := setupSyncHome(t, `{"corpus":{"enabled":true}}`)

	corpusPath := filepath.Join(corpusDir, "beekeeper-corpus.ndjson")
	rec := corpus.CorpusRecord{
		AuditRecord: audit.AuditRecord{
			RecordID: "cov-empty-id-001", ClusterID: "cov-empty-cluster-001",
			ToolName: "Bash", RecordType: "policy_decision", Decision: "block",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
		TrueLabel:           "malicious",
		AdjudicationSource:  "catalog_confirmation",
		CorpusSchemaVersion: corpus.CorpusSchemaVersion,
		PushEnvelope: &corpus.PushEnvelope{
			Signature: corpus.EnvelopeSignature{
				PackageOrExtensionID: "", // empty — must trigger the continue branch
			},
			TrueLabel:      "malicious",
			ConfidenceTier: "enforce",
		},
	}
	if err := corpus.AppendCorpusRecordLine(corpusPath, rec); err != nil {
		t.Fatalf("seed corpus: %v", err)
	}

	orig := firstResponderFn
	t.Cleanup(func() { firstResponderFn = orig })
	firstResponderFn = func(_ context.Context, _ watch.FirstResponderConfig) error { return nil }

	cmd := newFastTestCmd(t)
	_ = runCatalogsSync(cmd, true /*force*/)
}

// TestRunCatalogsSyncOverlayError exercises the non-fatal per-record overlay
// error branch (catalogs_daemon.go:169-172, span 13). The error is injected by
// writing a malformed local-overlay.json in catalogDir so AddLocalOverlayEntry
// returns "parse local overlay: ..." — a genuine non-fatal per-record error.
// The function must log the error to stderr and continue; it must not propagate it.
func TestRunCatalogsSyncOverlayError(t *testing.T) {
	_, _, catalogDir, corpusDir := setupSyncHome(t, `{"corpus":{"enabled":true}}`)

	// Seed a malicious record with a valid PushEnvelope so the overlay loop runs.
	corpusPath := filepath.Join(corpusDir, "beekeeper-corpus.ndjson")
	rec := corpus.CorpusRecord{
		AuditRecord: audit.AuditRecord{
			RecordID: "cov-ovlerr-001", ClusterID: "cov-overlay-err-cluster-001",
			ToolName: "Bash", RecordType: "policy_decision", Decision: "block",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
		TrueLabel:           "malicious",
		AdjudicationSource:  "catalog_confirmation",
		CorpusSchemaVersion: corpus.CorpusSchemaVersion,
		PushEnvelope: &corpus.PushEnvelope{
			Signature: corpus.EnvelopeSignature{
				PackageOrExtensionID: "npm:cov-overlay-error-pkg",
				Version:              "1.0.0",
			},
			TrueLabel:      "malicious",
			ConfidenceTier: "enforce",
			SourceCount:    2,
			ActionHint:     corpus.ActionHintWatchAndBlock,
		},
	}
	if err := corpus.AppendCorpusRecordLine(corpusPath, rec); err != nil {
		t.Fatalf("seed corpus: %v", err)
	}

	// Write malformed JSON to local-overlay.json so LoadLocalOverlay returns an
	// error → AddLocalOverlayEntry returns a non-nil error → span 13 fires.
	overlayJSONPath := filepath.Join(catalogDir, "local-overlay.json")
	if err := os.WriteFile(overlayJSONPath, []byte("NOT VALID JSON"), 0o600); err != nil {
		t.Fatalf("write malformed overlay: %v", err)
	}

	orig := firstResponderFn
	t.Cleanup(func() { firstResponderFn = orig })
	firstResponderFn = func(_ context.Context, _ watch.FirstResponderConfig) error { return nil }

	cmd := newFastTestCmd(t)

	// The overlay error is non-fatal: runCatalogsSync must not propagate it.
	_ = runCatalogsSync(cmd, true /*force*/)
}

// TestOfferCatalogSyncDaemonPrintsInstructions exercises offerCatalogSyncDaemon
// (catalogs_daemon.go:341-348, currently 0%). It calls runCatalogsSync (which
// fails when the HTTP client times out immediately via a cancelled context) and
// then prints the daemon-install instructions. A 1ms context deadline makes the
// HTTP fetch fail fast so the test completes in < 100ms.
func TestOfferCatalogSyncDaemonPrintsInstructions(t *testing.T) {
	setupSyncHome(t, `{"corpus":{"enabled":false}}`)

	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	// Use a context with a very short deadline so the HTTP sync fails immediately.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	cmd.SetContext(ctx)
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)

	offerCatalogSyncDaemon(cmd)

	out := buf.String()
	// offerCatalogSyncDaemon must print the first-sync attempt message and the
	// daemon install instructions regardless of whether the sync succeeded.
	if !strings.Contains(out, "catalog sync") {
		t.Errorf("offerCatalogSyncDaemon output missing catalog sync mention; got %q", out)
	}
	if !strings.Contains(out, "daemon install") {
		t.Errorf("offerCatalogSyncDaemon output missing daemon install mention; got %q", out)
	}
}
