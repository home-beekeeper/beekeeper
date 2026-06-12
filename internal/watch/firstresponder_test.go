package watch_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bantuson/beekeeper/internal/policy"
	"github.com/bantuson/beekeeper/internal/watch"
)

// TestFirstResponderAuditRedacted verifies F-1 (TM-D-03): the audit record
// emitted by the first-responder is routed through audit.RedactRecord before it
// is written, so a credential-shaped string carried in the hit's policy decision
// (Decision.Reason / CatalogMatches[].Package) is masked on disk.
func TestFirstResponderAuditRedacted(t *testing.T) {
	quarantineDir := t.TempDir()
	auditPath := filepath.Join(t.TempDir(), "beekeeper.ndjson")

	pkgDir := filepath.Join(t.TempDir(), "evil-pkg")
	if err := os.MkdirAll(pkgDir, 0o700); err != nil {
		t.Fatalf("mkdir pkgDir: %v", err)
	}

	const bearer = "Authorization: Bearer secret-token-abc123"
	const jwt = "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyMSJ9.signature123"

	hits := []watch.ScanHit{
		{
			Ecosystem:          "npm",
			Package:            "evil-package",
			Version:            "1.0.0",
			InstalledPath:      pkgDir,
			PathResolved:       true,
			CorroborationCount: 2,
			Decision: policy.Decision{
				Level:  "block",
				Reason: "catalog match; tool output contained " + bearer,
				CatalogMatches: []policy.CatalogMatch{
					{CatalogSource: "bumblebee", Package: jwt, EntryID: "id-1"},
				},
			},
		},
	}

	cfg := watch.FirstResponderConfig{
		Enabled:           true,
		DryRun:            true, // dry-run: still writes the would-quarantine audit record
		Threshold:         2,
		QuarantineDir:     quarantineDir,
		AuditPath:         auditPath,
		SentryTargetsPath: filepath.Join(t.TempDir(), "sentry-targets.json"),
		CrossRefFn: func(_ context.Context, _ watch.CrossRefConfig) ([]watch.ScanHit, error) {
			return hits, nil
		},
	}

	if err := watch.RunFirstResponder(context.Background(), cfg); err != nil {
		t.Fatalf("RunFirstResponder error: %v", err)
	}

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit: %v", err)
	}
	got := string(data)
	if strings.Contains(got, "secret-token-abc123") {
		t.Errorf("first-responder audit leaked raw Bearer token:\n%s", got)
	}
	if strings.Contains(got, jwt) {
		t.Errorf("first-responder audit leaked raw JWT in CatalogMatches.Package:\n%s", got)
	}
	if !strings.Contains(got, "[REDACTED]") && !strings.Contains(got, "[JWT_REDACTED]") {
		t.Errorf("first-responder audit missing redaction marker (redaction not applied):\n%s", got)
	}
}

// TestFirstResponderDryRun verifies that in dry-run mode:
// - a scan hit above threshold produces a "would-quarantine" audit record
// - no artifact is moved (the pkg dir still exists)
func TestFirstResponderDryRun(t *testing.T) {
	quarantineDir := t.TempDir()
	auditPath := filepath.Join(t.TempDir(), "beekeeper.ndjson")

	pkgDir := filepath.Join(t.TempDir(), "evil-pkg")
	if err := os.MkdirAll(pkgDir, 0o700); err != nil {
		t.Fatalf("mkdir pkgDir: %v", err)
	}

	// Inject a scan hit with threshold-meeting corroboration.
	hits := []watch.ScanHit{
		{
			Ecosystem:          "npm",
			Package:            "evil-package",
			Version:            "1.0.0",
			InstalledPath:      pkgDir,
			PathResolved:       true,
			CorroborationCount: 2, // meets default threshold of 2
		},
	}

	cfg := watch.FirstResponderConfig{
		Enabled:          true,
		DryRun:           true,
		Threshold:        2,
		QuarantineDir:    quarantineDir,
		AuditPath:        auditPath,
		SentryTargetsPath: filepath.Join(t.TempDir(), "sentry-targets.json"),
		CrossRefFn: func(_ context.Context, _ watch.CrossRefConfig) ([]watch.ScanHit, error) {
			return hits, nil
		},
	}

	if err := watch.RunFirstResponder(context.Background(), cfg); err != nil {
		t.Fatalf("RunFirstResponder error: %v", err)
	}

	// pkgDir must still exist (dry-run never moves).
	if _, err := os.Stat(pkgDir); err != nil {
		t.Errorf("dry-run: pkgDir should still exist, got: %v", err)
	}

	// Audit file must contain a "would-quarantine" record.
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if !strings.Contains(string(data), "would-quarantine") {
		t.Errorf("audit must contain 'would-quarantine' on dry-run; got:\n%s", string(data))
	}
}

// TestFirstResponderRealQuarantine verifies that when enabled+not-dry-run:
// - an artifact with resolved path and count >= threshold is moved
// - a "catalog_quarantine" audit record is written
func TestFirstResponderRealQuarantine(t *testing.T) {
	quarantineDir := t.TempDir()
	auditPath := filepath.Join(t.TempDir(), "beekeeper.ndjson")

	pkgDir := filepath.Join(t.TempDir(), "evil-pkg")
	if err := os.MkdirAll(pkgDir, 0o700); err != nil {
		t.Fatalf("mkdir pkgDir: %v", err)
	}

	hits := []watch.ScanHit{
		{
			Ecosystem:          "npm",
			Package:            "evil-package",
			Version:            "1.0.0",
			InstalledPath:      pkgDir,
			PathResolved:       true,
			CorroborationCount: 2,
		},
	}

	cfg := watch.FirstResponderConfig{
		Enabled:          true,
		DryRun:           false, // real quarantine
		Threshold:        2,
		QuarantineDir:    quarantineDir,
		AuditPath:        auditPath,
		SentryTargetsPath: filepath.Join(t.TempDir(), "sentry-targets.json"),
		CrossRefFn: func(_ context.Context, _ watch.CrossRefConfig) ([]watch.ScanHit, error) {
			return hits, nil
		},
	}

	if err := watch.RunFirstResponder(context.Background(), cfg); err != nil {
		t.Fatalf("RunFirstResponder error: %v", err)
	}

	// pkgDir must be gone (moved into quarantine).
	if _, err := os.Stat(pkgDir); !os.IsNotExist(err) {
		t.Errorf("real quarantine: pkgDir should be gone after MoveTyped, stat = %v", err)
	}

	// Audit must contain catalog_quarantine record.
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if !strings.Contains(string(data), "catalog_quarantine") {
		t.Errorf("audit must contain 'catalog_quarantine'; got:\n%s", string(data))
	}
}

// TestFirstResponderPendingQuarantine verifies that an unresolved path
// produces a "pending-quarantine" audit record and no move.
func TestFirstResponderPendingQuarantine(t *testing.T) {
	quarantineDir := t.TempDir()
	auditPath := filepath.Join(t.TempDir(), "beekeeper.ndjson")

	hits := []watch.ScanHit{
		{
			Ecosystem:          "npm",
			Package:            "evil-package",
			Version:            "1.0.0",
			InstalledPath:      "",   // unresolved
			PathResolved:       false,
			CorroborationCount: 2,
		},
	}

	cfg := watch.FirstResponderConfig{
		Enabled:          true,
		DryRun:           false,
		Threshold:        2,
		QuarantineDir:    quarantineDir,
		AuditPath:        auditPath,
		SentryTargetsPath: filepath.Join(t.TempDir(), "sentry-targets.json"),
		CrossRefFn: func(_ context.Context, _ watch.CrossRefConfig) ([]watch.ScanHit, error) {
			return hits, nil
		},
	}

	if err := watch.RunFirstResponder(context.Background(), cfg); err != nil {
		t.Fatalf("RunFirstResponder error: %v", err)
	}

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if !strings.Contains(string(data), "pending-quarantine") {
		t.Errorf("audit must contain 'pending-quarantine'; got:\n%s", string(data))
	}
}

// TestFirstResponderBelowThreshold verifies that a hit below threshold produces
// no quarantine action and no audit record.
func TestFirstResponderBelowThreshold(t *testing.T) {
	quarantineDir := t.TempDir()
	auditPath := filepath.Join(t.TempDir(), "beekeeper.ndjson")

	pkgDir := filepath.Join(t.TempDir(), "suspicious-pkg")
	if err := os.MkdirAll(pkgDir, 0o700); err != nil {
		t.Fatalf("mkdir pkgDir: %v", err)
	}

	hits := []watch.ScanHit{
		{
			Ecosystem:          "npm",
			Package:            "suspicious-package",
			Version:            "1.0.0",
			InstalledPath:      pkgDir,
			PathResolved:       true,
			CorroborationCount: 1, // below threshold of 2
		},
	}

	cfg := watch.FirstResponderConfig{
		Enabled:          true,
		DryRun:           false,
		Threshold:        2,
		QuarantineDir:    quarantineDir,
		AuditPath:        auditPath,
		SentryTargetsPath: filepath.Join(t.TempDir(), "sentry-targets.json"),
		CrossRefFn: func(_ context.Context, _ watch.CrossRefConfig) ([]watch.ScanHit, error) {
			return hits, nil
		},
	}

	if err := watch.RunFirstResponder(context.Background(), cfg); err != nil {
		t.Fatalf("RunFirstResponder error: %v", err)
	}

	// pkgDir must still exist (not moved).
	if _, err := os.Stat(pkgDir); err != nil {
		t.Errorf("below-threshold: pkgDir should still exist, got: %v", err)
	}

	// No audit record written.
	if _, err := os.Stat(auditPath); !os.IsNotExist(err) {
		data, _ := os.ReadFile(auditPath)
		t.Errorf("below-threshold: no audit record expected, but file exists with:\n%s", string(data))
	}
}

// TestFirstResponderMoveTypedErrorFailClosed verifies that a MoveTyped error
// leaves the artifact in place (fail-closed) and still writes an audit record.
func TestFirstResponderMoveTypedErrorFailClosed(t *testing.T) {
	quarantineDir := t.TempDir()
	auditPath := filepath.Join(t.TempDir(), "beekeeper.ndjson")

	// Use a path that doesn't exist — Rename will fail.
	nonExistent := filepath.Join(t.TempDir(), "non-existent-pkg")

	hits := []watch.ScanHit{
		{
			Ecosystem:          "npm",
			Package:            "evil-package",
			Version:            "1.0.0",
			InstalledPath:      nonExistent,
			PathResolved:       true,
			CorroborationCount: 2,
		},
	}

	cfg := watch.FirstResponderConfig{
		Enabled:          true,
		DryRun:           false,
		Threshold:        2,
		QuarantineDir:    quarantineDir,
		AuditPath:        auditPath,
		SentryTargetsPath: filepath.Join(t.TempDir(), "sentry-targets.json"),
		CrossRefFn: func(_ context.Context, _ watch.CrossRefConfig) ([]watch.ScanHit, error) {
			return hits, nil
		},
	}

	// Must not return error — fail-closed means log+continue, not crash.
	if err := watch.RunFirstResponder(context.Background(), cfg); err != nil {
		t.Fatalf("RunFirstResponder error: %v", err)
	}

	// The audit file should exist (the attempt was recorded).
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit after move error: %v", err)
	}
	// Should record the attempt (quarantine_error or catalog_quarantine).
	if len(data) == 0 {
		t.Error("audit must not be empty after move error (fail-closed: log the attempt)")
	}
}

// TestFirstResponderSentryTargetsWritten verifies that a scan hit records into
// the Sentry target list JSON file.
func TestFirstResponderSentryTargetsWritten(t *testing.T) {
	quarantineDir := t.TempDir()
	sentryTargetsPath := filepath.Join(t.TempDir(), "sentry-targets.json")

	pkgDir := filepath.Join(t.TempDir(), "evil-pkg")
	if err := os.MkdirAll(pkgDir, 0o700); err != nil {
		t.Fatalf("mkdir pkgDir: %v", err)
	}

	hits := []watch.ScanHit{
		{
			Ecosystem:          "npm",
			Package:            "evil-package",
			Version:            "1.0.0",
			InstalledPath:      pkgDir,
			PathResolved:       true,
			CorroborationCount: 2,
		},
	}

	cfg := watch.FirstResponderConfig{
		Enabled:          true,
		DryRun:           true, // dry-run so we don't need a real pkg
		Threshold:        2,
		QuarantineDir:    quarantineDir,
		AuditPath:        filepath.Join(t.TempDir(), "beekeeper.ndjson"),
		SentryTargetsPath: sentryTargetsPath,
		CrossRefFn: func(_ context.Context, _ watch.CrossRefConfig) ([]watch.ScanHit, error) {
			return hits, nil
		},
	}

	if err := watch.RunFirstResponder(context.Background(), cfg); err != nil {
		t.Fatalf("RunFirstResponder error: %v", err)
	}

	// sentry-targets.json must be written and contain the package name.
	data, err := os.ReadFile(sentryTargetsPath)
	if err != nil {
		t.Fatalf("read sentry-targets.json: %v", err)
	}
	if !strings.Contains(string(data), "evil-package") {
		t.Errorf("sentry-targets.json must contain 'evil-package'; got:\n%s", string(data))
	}

	// Validate it is valid JSON.
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Errorf("sentry-targets.json is not valid JSON: %v", err)
	}
}
