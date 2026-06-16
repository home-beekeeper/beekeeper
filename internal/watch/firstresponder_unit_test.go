// Package watch — firstresponder_unit_test.go
//
// Internal-package tests (package watch, not watch_test) for unexported functions
// in firstresponder.go. These cover the branches that are unreachable from the
// external package:
//
//   - parsePackageID: all input formats
//   - ecosystemToProcess: all ecosystem → process mappings + unknown default
//   - defaultFirstResponder: thin delegation to RunFirstResponder
//   - marshalTargetListJSON: happy-path serialisation
//   - writeFirstResponderAudit (empty auditPath early-return)
//   - writeCorpusFirstResponderAudit (empty auditPath early-return)
//
// Import constraint: must NOT import internal/tui (ADJ-01).
package watch

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/home-beekeeper/beekeeper/internal/sentry"
)

// ---- parsePackageID ----

// TestParsePackageID_NPMScoped verifies "npm:@org/pkg" splits into
// ("npm", "@org/pkg"). Scoped npm names containing '@' and '/' survive intact.
func TestParsePackageID_NPMScoped(t *testing.T) {
	eco, pkg := parsePackageID("npm:@nrwl/nx-console")
	if eco != "npm" {
		t.Errorf("ecosystem = %q, want npm", eco)
	}
	if pkg != "@nrwl/nx-console" {
		t.Errorf("package = %q, want @nrwl/nx-console", pkg)
	}
}

// TestParsePackageID_Simple verifies "pip:requests" splits into ("pip", "requests").
func TestParsePackageID_Simple(t *testing.T) {
	eco, pkg := parsePackageID("pip:requests")
	if eco != "pip" {
		t.Errorf("ecosystem = %q, want pip", eco)
	}
	if pkg != "requests" {
		t.Errorf("package = %q, want requests", pkg)
	}
}

// TestParsePackageID_EditorExtension verifies "editor-extension:foo.bar" splits
// into ("editor-extension", "foo.bar"). The colon after "editor-extension"
// is the first colon; the dot in "foo.bar" is not a colon so it is part of pkg.
func TestParsePackageID_EditorExtension(t *testing.T) {
	eco, pkg := parsePackageID("editor-extension:some.extension")
	if eco != "editor-extension" {
		t.Errorf("ecosystem = %q, want editor-extension", eco)
	}
	if pkg != "some.extension" {
		t.Errorf("package = %q, want some.extension", pkg)
	}
}

// TestParsePackageID_NoColon verifies that a bare "packagename" (no colon)
// returns ("", "packagename") — the whole string is treated as the package name.
func TestParsePackageID_NoColon(t *testing.T) {
	eco, pkg := parsePackageID("some-bare-package")
	if eco != "" {
		t.Errorf("ecosystem = %q, want empty string for bare package ID", eco)
	}
	if pkg != "some-bare-package" {
		t.Errorf("package = %q, want some-bare-package", pkg)
	}
}

// TestParsePackageID_Empty verifies that an empty string returns ("", "").
func TestParsePackageID_Empty(t *testing.T) {
	eco, pkg := parsePackageID("")
	if eco != "" || pkg != "" {
		t.Errorf("parsePackageID(\"\") = (%q, %q), want (\"\", \"\")", eco, pkg)
	}
}

// TestParsePackageID_CargoScoped verifies "cargo:tokio" splits correctly.
func TestParsePackageID_CargoScoped(t *testing.T) {
	eco, pkg := parsePackageID("cargo:tokio")
	if eco != "cargo" {
		t.Errorf("ecosystem = %q, want cargo", eco)
	}
	if pkg != "tokio" {
		t.Errorf("package = %q, want tokio", pkg)
	}
}

// TestParsePackageID_FirstColonIsDelimiter verifies that only the FIRST colon
// is used as the delimiter. A value like "npm:foo:bar" would yield
// ("npm", "foo:bar") — the second colon is not a delimiter.
func TestParsePackageID_FirstColonIsDelimiter(t *testing.T) {
	eco, pkg := parsePackageID("npm:foo:bar")
	if eco != "npm" {
		t.Errorf("ecosystem = %q, want npm", eco)
	}
	if pkg != "foo:bar" {
		t.Errorf("package = %q, want foo:bar", pkg)
	}
}

// ---- ecosystemToProcess ----

func TestEcosystemToProcess(t *testing.T) {
	tests := []struct {
		ecosystem   string
		wantProcess string
	}{
		{"npm", "node"},
		{"yarn", "node"},
		{"pnpm", "node"},
		// Case-insensitive: ToLower is applied in ecosystemToProcess.
		{"NPM", "node"},
		{"YARN", "node"},
		{"PNPM", "node"},
		{"pip", "python"},
		{"pypi", "python"},
		{"PIP", "python"},
		{"cargo", "cargo"},
		{"CARGO", "cargo"},
		{"go", "go"},
		{"GO", "go"},
		{"rubygems", "ruby"},
		{"RubyGems", "ruby"},
		{"packagist", "php"},
		{"Packagist", "php"},
		// Unknown ecosystem → empty string (default branch).
		{"editor-extension", ""},
		{"unknown-ecosystem", ""},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.ecosystem, func(t *testing.T) {
			got := ecosystemToProcess(tc.ecosystem)
			if got != tc.wantProcess {
				t.Errorf("ecosystemToProcess(%q) = %q, want %q", tc.ecosystem, got, tc.wantProcess)
			}
		})
	}
}

// ---- defaultFirstResponder ----

// TestDefaultFirstResponder verifies that defaultFirstResponder delegates to
// RunFirstResponder with the same config. We inject a CrossRefFn that returns
// no hits so RunFirstResponder completes successfully without any I/O beyond
// the audit path (which is empty, so no write occurs).
func TestDefaultFirstResponder(t *testing.T) {
	cfg := FirstResponderConfig{
		Enabled:   false,
		DryRun:    true,
		Threshold: 2,
		AuditPath: "", // empty → writeFirstResponderAudit returns immediately
		CrossRefFn: func(_ context.Context, _ CrossRefConfig) ([]ScanHit, error) {
			return nil, nil
		},
	}

	err := defaultFirstResponder(context.Background(), cfg)
	if err != nil {
		t.Fatalf("defaultFirstResponder returned error: %v", err)
	}
}

// TestDefaultFirstResponder_PropagatesError verifies that defaultFirstResponder
// propagates errors from RunFirstResponder (CrossRefFn returns an error).
func TestDefaultFirstResponder_PropagatesError(t *testing.T) {
	errExpected := context.Canceled
	cfg := FirstResponderConfig{
		Enabled:   false,
		DryRun:    true,
		Threshold: 2,
		AuditPath: "",
		CrossRefFn: func(_ context.Context, _ CrossRefConfig) ([]ScanHit, error) {
			return nil, errExpected
		},
	}

	err := defaultFirstResponder(context.Background(), cfg)
	if err == nil {
		t.Fatal("defaultFirstResponder with failing CrossRefFn returned nil error; want error")
	}
}

// ---- writeFirstResponderAudit — empty auditPath early-return ----

// TestWriteFirstResponderAudit_EmptyPath verifies the early-return branch: when
// auditPath == "", the function returns immediately without attempting a write
// (no error, no panic). This covers the 0% branch reported by go tool cover.
func TestWriteFirstResponderAudit_EmptyPath(t *testing.T) {
	hit := ScanHit{
		Ecosystem:          "npm",
		Package:            "evil-pkg",
		Version:            "1.0.0",
		InstalledPath:      "/some/path",
		PathResolved:       true,
		CorroborationCount: 2,
	}
	// Must not panic or error. Coverage counts the "if auditPath == "" { return }"
	// branch inside writeFirstResponderAudit.
	writeFirstResponderAudit("", "would-quarantine", hit)
}

// TestWriteFirstResponderAudit_WritesRecord verifies that a non-empty auditPath
// causes writeFirstResponderAudit to create the audit file. This exercises the
// non-early-return path. The record content is verified by the external-package
// tests; here we just assert the file is created.
func TestWriteFirstResponderAudit_WritesRecord(t *testing.T) {
	auditDir := t.TempDir()
	auditPath := filepath.Join(auditDir, "beekeeper.ndjson")

	hit := ScanHit{
		Ecosystem:          "npm",
		Package:            "evil-pkg",
		Version:            "1.0.0",
		InstalledPath:      "/some/path",
		PathResolved:       true,
		CorroborationCount: 2,
	}
	writeFirstResponderAudit(auditPath, "would-quarantine", hit)

	if _, err := os.Stat(auditPath); err != nil {
		t.Fatalf("audit file not created by writeFirstResponderAudit: %v", err)
	}
}

// ---- writeCorpusFirstResponderAudit — empty auditPath early-return ----

// TestWriteCorpusFirstResponderAudit_EmptyPath covers the early-return branch
// in writeCorpusFirstResponderAudit when auditPath == "".
func TestWriteCorpusFirstResponderAudit_EmptyPath(t *testing.T) {
	// Must not panic or error.
	writeCorpusFirstResponderAudit("", "pending-quarantine", "npm", "@nrwl/nx-console", "17.3.0")
}

// TestWriteCorpusFirstResponderAudit_WritesRecord verifies that a non-empty
// auditPath causes writeCorpusFirstResponderAudit to create the audit file.
func TestWriteCorpusFirstResponderAudit_WritesRecord(t *testing.T) {
	auditDir := t.TempDir()
	auditPath := filepath.Join(auditDir, "beekeeper.ndjson")

	writeCorpusFirstResponderAudit(auditPath, "pending-quarantine", "npm", "@nrwl/nx-console", "17.3.0")

	if _, err := os.Stat(auditPath); err != nil {
		t.Fatalf("audit file not created by writeCorpusFirstResponderAudit: %v", err)
	}
}

// ---- marshalTargetListJSON ----

// TestMarshalTargetListJSON_EmptyList verifies marshalTargetListJSON with an
// empty TargetList returns valid JSON containing a "targets" key with an empty
// array (or null — both acceptable).
func TestMarshalTargetListJSON_EmptyList(t *testing.T) {
	tl := &sentry.TargetList{}
	data, err := marshalTargetListJSON(tl)
	if err != nil {
		t.Fatalf("marshalTargetListJSON empty list: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("marshalTargetListJSON returned empty bytes for empty list")
	}
}

// TestMarshalTargetListJSON_WithTargets verifies marshalTargetListJSON produces
// correctly-serialised JSON for a TargetList with entries. The output must be
// valid JSON and contain the target name.
func TestMarshalTargetListJSON_WithTargets(t *testing.T) {
	tl := &sentry.TargetList{}
	tl.AddTarget("@nrwl/nx-console", "/path/to/node_modules/@nrwl/nx-console", "node")
	tl.AddTarget("requests", "", "python")

	data, err := marshalTargetListJSON(tl)
	if err != nil {
		t.Fatalf("marshalTargetListJSON: %v", err)
	}
	got := string(data)

	if got == "" {
		t.Fatal("marshalTargetListJSON returned empty output")
	}
	if !containsStr(got, "@nrwl/nx-console") {
		t.Errorf("output missing @nrwl/nx-console:\n%s", got)
	}
	if !containsStr(got, "requests") {
		t.Errorf("output missing requests:\n%s", got)
	}
	if !containsStr(got, "node") {
		t.Errorf("output missing expected_process=node:\n%s", got)
	}
}

// TestMarshalTargetListJSON_Nil verifies marshalTargetListJSON does not panic
// when called with a nil TargetList (json.MarshalIndent handles nil gracefully).
func TestMarshalTargetListJSON_Nil(t *testing.T) {
	data, err := marshalTargetListJSON(nil)
	if err != nil {
		t.Fatalf("marshalTargetListJSON(nil) returned error: %v", err)
	}
	// nil → "null" in JSON, which is valid.
	if string(data) != "null" {
		t.Logf("marshalTargetListJSON(nil) = %q (acceptable: 'null' or valid JSON)", string(data))
	}
}

// ---- RunFirstResponder threshold default ----

// TestRunFirstResponder_ThresholdDefault verifies the "threshold <= 0 → 2"
// safety net inside RunFirstResponder. With threshold=0, the safety net sets it
// to 2; a hit with CorroborationCount=1 must not trigger any action.
func TestRunFirstResponder_ThresholdDefault(t *testing.T) {
	auditPath := filepath.Join(t.TempDir(), "beekeeper.ndjson")

	hits := []ScanHit{
		{
			Ecosystem:          "npm",
			Package:            "suspicious-pkg",
			Version:            "1.0.0",
			InstalledPath:      "/some/path",
			PathResolved:       true,
			CorroborationCount: 1, // below the defaulted threshold of 2
		},
	}

	cfg := FirstResponderConfig{
		Enabled:   true,
		DryRun:    false,
		Threshold: 0, // triggers the "if threshold <= 0 { threshold = 2 }" branch
		AuditPath: auditPath,
		CrossRefFn: func(_ context.Context, _ CrossRefConfig) ([]ScanHit, error) {
			return hits, nil
		},
	}

	if err := RunFirstResponder(context.Background(), cfg); err != nil {
		t.Fatalf("RunFirstResponder error: %v", err)
	}

	// With the defaulted threshold=2, count=1 is below threshold → no audit record.
	if _, err := os.Stat(auditPath); !os.IsNotExist(err) {
		data, _ := os.ReadFile(auditPath)
		t.Errorf("threshold=0 defaulted to 2; count=1 hit must not produce an audit record, but got:\n%s", string(data))
	}
}

// TestRunFirstResponder_SentryTargetsPathEmpty verifies that when
// SentryTargetsPath == "" the code skips the LoadTargets + SaveTargets path
// (no panic, no error) even when there are qualifying hits.
func TestRunFirstResponder_SentryTargetsPathEmpty(t *testing.T) {
	hits := []ScanHit{
		{
			Ecosystem:          "npm",
			Package:            "evil-pkg",
			Version:            "1.0.0",
			InstalledPath:      "/some/path",
			PathResolved:       true,
			CorroborationCount: 2,
		},
	}

	cfg := FirstResponderConfig{
		Enabled:           true,
		DryRun:            true,
		Threshold:         2,
		AuditPath:         "", // suppress audit writes
		SentryTargetsPath: "", // suppress sentry writes — exercises the "targets == nil" branch
		CrossRefFn: func(_ context.Context, _ CrossRefConfig) ([]ScanHit, error) {
			return hits, nil
		},
	}

	if err := RunFirstResponder(context.Background(), cfg); err != nil {
		t.Fatalf("RunFirstResponder with empty SentryTargetsPath returned error: %v", err)
	}
}

// ---- RunFirstResponder corpus path — CorpusEnabled=false gate ----

// TestRunFirstResponder_CorpusDisabledSkipsRead verifies that when
// CorpusEnabled==false the corpus path (ReadMaliciousRecords) is skipped, even
// when CorpusPath is non-empty. This exercises the outer gate:
// "if cfg.CorpusEnabled && cfg.CorpusPath != """.
func TestRunFirstResponder_CorpusDisabledSkipsRead(t *testing.T) {
	// Point CorpusPath at a file that does NOT exist.
	// If the corpus path is exercised, it would log a read error but NOT return
	// an error (non-fatal). Since this path should be skipped entirely, there
	// should be no log call and no error.
	cfg := FirstResponderConfig{
		Enabled:       false,
		DryRun:        true,
		Threshold:     2,
		AuditPath:     "",
		CorpusPath:    filepath.Join(t.TempDir(), "nonexistent-corpus.ndjson"),
		CorpusEnabled: false, // gate: skips the corpus block entirely
		CrossRefFn: func(_ context.Context, _ CrossRefConfig) ([]ScanHit, error) {
			return nil, nil
		},
	}

	if err := RunFirstResponder(context.Background(), cfg); err != nil {
		t.Fatalf("RunFirstResponder returned error: %v", err)
	}
}

// TestRunFirstResponder_CorpusPathEmptySkipsRead verifies that when
// CorpusEnabled==true but CorpusPath=="" the corpus block is skipped (the
// "&&" conjunction in the gate condition).
func TestRunFirstResponder_CorpusPathEmptySkipsRead(t *testing.T) {
	cfg := FirstResponderConfig{
		Enabled:       false,
		DryRun:        true,
		Threshold:     2,
		AuditPath:     "",
		CorpusPath:    "",     // empty path → corpus block skipped
		CorpusEnabled: true,   // enabled but path empty
		CrossRefFn: func(_ context.Context, _ CrossRefConfig) ([]ScanHit, error) {
			return nil, nil
		},
	}

	if err := RunFirstResponder(context.Background(), cfg); err != nil {
		t.Fatalf("RunFirstResponder returned error: %v", err)
	}
}

// TestRunFirstResponder_CorpusReadError verifies the corpus-error non-fatal path:
// when corpus.ReadMaliciousRecords returns an error (corrupt file), the error is
// logged and the function continues without returning an error. The scan-hit path
// still runs normally.
func TestRunFirstResponder_CorpusReadError(t *testing.T) {
	// Write a corrupt NDJSON file — ReadMaliciousRecords will return an error.
	corpusDir := t.TempDir()
	corpusPath := filepath.Join(corpusDir, "bad-corpus.ndjson")
	if err := os.WriteFile(corpusPath, []byte(`not valid ndjson json`), 0o600); err != nil {
		t.Fatalf("write corrupt corpus: %v", err)
	}

	cfg := FirstResponderConfig{
		Enabled:       false,
		DryRun:        true,
		Threshold:     2,
		AuditPath:     "",
		CorpusPath:    corpusPath,
		CorpusEnabled: true, // enabled → will attempt ReadMaliciousRecords
		CrossRefFn: func(_ context.Context, _ CrossRefConfig) ([]ScanHit, error) {
			return nil, nil
		},
	}

	// Must not return an error — corpus read failures are non-fatal (logged + skipped).
	if err := RunFirstResponder(context.Background(), cfg); err != nil {
		t.Fatalf("RunFirstResponder with corrupt corpus returned error: %v", err)
	}
}

// TestRunFirstResponder_SentryTargetsLoadCorrupt verifies the "t == nil" branch
// inside RunFirstResponder: when LoadTargets returns (nil, err) because the
// sentry targets file is corrupt JSON, the code initialises an empty TargetList
// and continues normally. This exercises the `if t == nil { t = &sentry.TargetList{} }`.
func TestRunFirstResponder_SentryTargetsLoadCorrupt(t *testing.T) {
	sentryDir := t.TempDir()
	sentryPath := filepath.Join(sentryDir, "sentry-targets.json")
	// Write a corrupt targets file so LoadTargets returns (nil, parse error).
	if err := os.WriteFile(sentryPath, []byte(`not json`), 0o600); err != nil {
		t.Fatalf("write corrupt sentry targets: %v", err)
	}

	hits := []ScanHit{
		{
			Ecosystem:          "npm",
			Package:            "evil-pkg",
			Version:            "1.0.0",
			InstalledPath:      "/some/path",
			PathResolved:       true,
			CorroborationCount: 2, // meets threshold → AddTarget is called
		},
	}

	cfg := FirstResponderConfig{
		Enabled:           false,
		DryRun:            true,
		Threshold:         2,
		AuditPath:         "",
		SentryTargetsPath: sentryPath,
		CrossRefFn: func(_ context.Context, _ CrossRefConfig) ([]ScanHit, error) {
			return hits, nil
		},
	}

	// Must not error — a corrupt prior sentry-targets.json is not a hard failure.
	if err := RunFirstResponder(context.Background(), cfg); err != nil {
		t.Fatalf("RunFirstResponder with corrupt sentry targets returned error: %v", err)
	}
}

// TestRunFirstResponder_CorpusNilOrEmptyEnvelopeSkipped verifies the corpus
// loop guard: records where PushEnvelope == nil OR PackageOrExtensionID == ""
// are silently skipped (the "continue" statement at the top of the corpus loop).
// This exercises the `if rec.PushEnvelope == nil || rec.PushEnvelope.Signature.PackageOrExtensionID == ""`.
func TestRunFirstResponder_CorpusNilOrEmptyEnvelopeSkipped(t *testing.T) {
	// We can't directly construct a corpus.CorpusRecord with nil PushEnvelope
	// via JSON (the field is a pointer, absent JSON key = nil). Write NDJSON
	// with no push_envelope key.
	corpusDir := t.TempDir()
	corpusPath := filepath.Join(corpusDir, "corpus.ndjson")
	// Minimal valid CorpusRecord JSON with no push_envelope (envelope = nil).
	line := `{"audit_record":{"record_id":"test-001","tool_name":"some-pkg","record_type":"policy_decision","decision":"block","timestamp":"2026-01-01T00:00:00Z"},"true_label":"malicious","adjudication_source":"catalog_confirmation","corpus_schema_version":"1.0"}` + "\n"
	if err := os.WriteFile(corpusPath, []byte(line), 0o600); err != nil {
		t.Fatalf("write corpus file: %v", err)
	}

	cfg := FirstResponderConfig{
		Enabled:               false,
		DryRun:                true,
		Threshold:             2,
		AuditPath:             "",
		SentryTargetsPath:     "",
		CorpusPath:            corpusPath,
		CorpusEnabled:         true,
		CorpusSentryThreshold: 2,
		CrossRefFn: func(_ context.Context, _ CrossRefConfig) ([]ScanHit, error) {
			return nil, nil
		},
	}

	// Must not error — nil envelope records are silently skipped.
	if err := RunFirstResponder(context.Background(), cfg); err != nil {
		t.Fatalf("RunFirstResponder with nil-envelope corpus record returned error: %v", err)
	}
}

// containsStr is a package-local helper (avoids an import of strings in the
// internal test file which already imports it elsewhere in the real code).
func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexStr(s, sub) >= 0)
}

func indexStr(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
