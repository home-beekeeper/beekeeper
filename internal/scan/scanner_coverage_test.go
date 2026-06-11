package scan

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/bantuson/beekeeper/internal/catalog"
)

// seedMarketplaceCache writes a fresh (cached_at == now) marketplace cache entry
// so FetchMarketplaceAge resolves the extension age offline (no network). The
// extension's published_at is now-publishedAgo; callers choose publishedAgo to
// land on either side of the 1440-minute release-age threshold.
func seedMarketplaceCache(t *testing.T, cacheDir, publisher, name, version string, now time.Time, publishedAgo time.Duration) {
	t.Helper()
	mktDir := filepath.Join(cacheDir, "marketplace-cache", publisher, name)
	if err := os.MkdirAll(mktDir, 0o700); err != nil {
		t.Fatalf("mkdir marketplace cache: %v", err)
	}
	entry := map[string]any{
		"published_at": now.Add(-publishedAgo).UTC().Format(time.RFC3339),
		"cached_at":    now.UTC().Format(time.RFC3339),
		"missing":      false,
	}
	b, _ := json.Marshal(entry)
	if err := os.WriteFile(filepath.Join(mktDir, version+".json"), b, 0o600); err != nil {
		t.Fatalf("write marketplace cache entry: %v", err)
	}
}

// writeExtension creates dir/<publisher>.<name>-<version>/package.json so
// beekeeperScan discovers a parseable extension manifest.
func writeExtension(t *testing.T, root, publisher, name, version, displayName string) {
	t.Helper()
	extDir := filepath.Join(root, publisher+"."+name+"-"+version)
	if err := os.MkdirAll(extDir, 0o700); err != nil {
		t.Fatalf("mkdir ext dir: %v", err)
	}
	pkg := map[string]any{
		"publisher":   publisher,
		"name":        name,
		"version":     version,
		"displayName": displayName,
	}
	b, _ := json.Marshal(pkg)
	if err := os.WriteFile(filepath.Join(extDir, "package.json"), b, 0o600); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
}

// emptyIndexAt builds a minimal mmap catalog index containing only an unrelated
// entry (so the test extension never produces a catalog match — the FLAG comes
// solely from the deterministic release-age check, keeping the test offline).
func emptyIndexAt(t *testing.T) string {
	t.Helper()
	indexPath := filepath.Join(t.TempDir(), "beekeeper.idx")
	if err := catalog.BuildIndex(indexPath, []catalog.Entry{{
		ID:        "unrelated",
		Ecosystem: "editor-extension",
		Package:   "evil.unrelated",
	}}); err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	return indexPath
}

// TestScanFlaggedExtensionWritesAudit drives a RECENTLY-published extension
// (age < 1440-minute release-age threshold) all the way through evaluateExtension
// with an AuditPath configured. This exercises the flag/hit path:
//   - release-age decision wins over an allowing catalog decision
//   - FindingRecord.decision == "block" (release-age fail-closed)
//   - an audit record is written with a generated scan ID (generateScanID, 0% → covered)
//
// It also sets SocketToken so the Socket adapter branch in evaluateExtension is
// exercised. No network is hit: the marketplace age is pre-seeded in the cache,
// and the OSV/Socket adapters degrade offline (the localhost HTTP client fails
// fast, leaving the catalog allowing — so the flag is purely release-age driven).
func TestScanFlaggedExtensionWritesAudit(t *testing.T) {
	old := runPollenFn
	defer func() { runPollenFn = old }()
	runPollenFn = func(_ context.Context, _ bool) (<-chan []byte, bool) { return nil, false }

	extRoot := t.TempDir()
	cacheDir := t.TempDir()
	testNow := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)

	// Extension published 60 minutes ago → 60 < 1440 → release-age BLOCKS.
	writeExtension(t, extRoot, "acme", "fresh-ext", "1.0.0", "Fresh Ext")
	seedMarketplaceCache(t, cacheDir, "acme", "fresh-ext", "1.0.0", testNow, 60*time.Minute)

	auditPath := filepath.Join(t.TempDir(), "audit.ndjson")

	var buf bytes.Buffer
	cfg := Config{
		ExtensionDirs: []string{extRoot},
		IndexPath:     emptyIndexAt(t),
		CacheDir:      cacheDir,
		AuditPath:     auditPath,
		SocketToken:   "test-socket-token", // exercises the Socket adapter branch
		// Bound network adapters to a dead localhost port so they fail fast offline.
		HTTPClient: &http.Client{Timeout: 200 * time.Millisecond},
		Now:        func() time.Time { return testNow },
	}
	if err := Scan(context.Background(), cfg, &buf); err != nil {
		t.Fatalf("Scan: %v", err)
	}

	// The finding record on stdout must report a non-allow (block) decision.
	var fr FindingRecord
	dec := json.NewDecoder(strings.NewReader(buf.String()))
	found := false
	for {
		var rec map[string]any
		if err := dec.Decode(&rec); err != nil {
			break
		}
		if rec["record_type"] == "finding" {
			b, _ := json.Marshal(rec)
			_ = json.Unmarshal(b, &fr)
			found = true
		}
	}
	if !found {
		t.Fatalf("no finding record emitted; got:\n%s", buf.String())
	}
	if fr.Decision != "block" {
		t.Errorf("finding decision = %q, want block (recent extension fails release-age); output:\n%s", fr.Decision, buf.String())
	}
	// EDXT-04 plus the release-age rule should be present.
	if !containsStr(fr.RuleIDs, "EDXT-04") {
		t.Errorf("rule_ids = %v, want EDXT-04 present", fr.RuleIDs)
	}

	// An audit record must have been written with a generated scan ID. The audit
	// file may also contain a scan_status line from the pollen-unavailable branch,
	// so locate the policy_decision record (the one evaluateExtension wrote).
	auditRec := readPolicyDecisionAudit(t, auditPath)
	// generateScanID returns 16 lowercase hex chars (8 random bytes) on the happy
	// path, or a "scan-<unixnano>" fallback if crypto/rand fails. Accept either.
	scanIDRe := regexp.MustCompile(`^([0-9a-f]{16}|scan-\d+)$`)
	if !scanIDRe.MatchString(auditRec.RecordID) {
		t.Errorf("audit record_id = %q, want generateScanID format (16 hex or scan-<n>)", auditRec.RecordID)
	}
	// The audited decision must reflect the flag, not the clean default.
	if auditRec.Decision != "block" {
		t.Errorf("audit decision = %q, want block (flagged recent extension)", auditRec.Decision)
	}
}

// TestScanCleanExtensionAuditsAllow drives an OLD extension (age > threshold,
// no catalog match) through evaluateExtension with an AuditPath. This covers the
// no-hit (clean) branch of the audit write — auditDecision is reset to the clean
// "allow" record — and still exercises generateScanID for the clean path.
func TestScanCleanExtensionAuditsAllow(t *testing.T) {
	old := runPollenFn
	defer func() { runPollenFn = old }()
	runPollenFn = func(_ context.Context, _ bool) (<-chan []byte, bool) { return nil, false }

	extRoot := t.TempDir()
	cacheDir := t.TempDir()
	testNow := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)

	// Published 10 days ago → well past 1440 minutes → release-age ALLOWS.
	writeExtension(t, extRoot, "ms-python", "python", "2026.4.0", "Python")
	seedMarketplaceCache(t, cacheDir, "ms-python", "python", "2026.4.0", testNow, 10*24*time.Hour)

	auditPath := filepath.Join(t.TempDir(), "audit.ndjson")

	var buf bytes.Buffer
	cfg := Config{
		ExtensionDirs: []string{extRoot},
		IndexPath:     emptyIndexAt(t),
		CacheDir:      cacheDir,
		AuditPath:     auditPath,
		HTTPClient:    &http.Client{Timeout: 200 * time.Millisecond},
		Now:           func() time.Time { return testNow },
	}
	if err := Scan(context.Background(), cfg, &buf); err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if !strings.Contains(buf.String(), `"decision":"allow"`) {
		t.Errorf("expected an allow finding for an old, unmatched extension; got:\n%s", buf.String())
	}

	auditRec := readPolicyDecisionAudit(t, auditPath)
	if auditRec.Decision != "allow" {
		t.Errorf("clean audit decision = %q, want allow", auditRec.Decision)
	}
	if auditRec.RecordID == "" {
		t.Errorf("clean audit record_id empty — generateScanID not invoked")
	}
}

// TestScanWithPolicyOverlay covers the "policy overlay present" branch of
// evaluateExtension: when policies/*.json exists, ApplyPolicyOverlay runs. The
// overlay here is a non-matching warn rule, so the clean extension's finding is
// unchanged — proving the overlay path executes safely and does not corrupt the
// decision. The policies dir lives at <dir(CacheDir)>/policies (see scanner.go).
func TestScanWithPolicyOverlay(t *testing.T) {
	old := runPollenFn
	defer func() { runPollenFn = old }()
	runPollenFn = func(_ context.Context, _ bool) (<-chan []byte, bool) { return nil, false }

	extRoot := t.TempDir()
	testNow := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)

	// CacheDir is a SUBDIR of base so that filepath.Dir(CacheDir) == base, and the
	// policies dir resolves to base/policies as evaluateExtension expects.
	base := t.TempDir()
	cacheDir := filepath.Join(base, "cache")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		t.Fatal(err)
	}
	policiesDir := filepath.Join(base, "policies")
	if err := os.MkdirAll(policiesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	overlay := `{"schema_version":"1.0.0","name":"test-overlay","rules":[` +
		`{"id":"ALLOW-1","rule_type":"package_allowlist","ecosystem":"editor-extension",` +
		`"packages":["someone.else"],"action":"allow"}]}`
	if err := os.WriteFile(filepath.Join(policiesDir, "overlay.json"), []byte(overlay), 0o600); err != nil {
		t.Fatal(err)
	}

	// Old, unmatched extension → clean allow; overlay rule does not match it.
	writeExtension(t, extRoot, "ms-python", "python", "2026.4.0", "Python")
	seedMarketplaceCache(t, cacheDir, "ms-python", "python", "2026.4.0", testNow, 10*24*time.Hour)

	var buf bytes.Buffer
	cfg := Config{
		ExtensionDirs: []string{extRoot},
		IndexPath:     emptyIndexAt(t),
		CacheDir:      cacheDir,
		HTTPClient:    &http.Client{Timeout: 200 * time.Millisecond},
		Now:           func() time.Time { return testNow },
	}
	if err := Scan(context.Background(), cfg, &buf); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !strings.Contains(buf.String(), `"decision":"allow"`) {
		t.Errorf("non-matching overlay should leave a clean allow finding; got:\n%s", buf.String())
	}
}

// TestScanMalformedPollenLine covers the empty-line skip and the malformed-NDJSON
// fail-closed branch in Scan: a non-JSON pollen line must be surfaced as an
// observable scan_error, never silently dropped.
func TestScanMalformedPollenLine(t *testing.T) {
	old := runPollenFn
	defer func() { runPollenFn = old }()
	runPollenFn = func(_ context.Context, _ bool) (<-chan []byte, bool) {
		ch := make(chan []byte, 3)
		ch <- []byte("")              // empty line → skipped (continue)
		ch <- []byte("not-json-here") // malformed → scan_error
		ch <- []byte(`{"record_type":"scan_summary","total":1}`)
		close(ch)
		return ch, true
	}

	var buf bytes.Buffer
	if err := Scan(context.Background(), Config{}, &buf); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"record_type":"scan_error"`) {
		t.Errorf("malformed pollen line did not produce scan_error; got:\n%s", out)
	}
	if !strings.Contains(out, "malformed NDJSON from the inventory scanner subprocess") {
		t.Errorf("expected malformed-NDJSON reason; got:\n%s", out)
	}
	// The valid record after the malformed one must still pass through (loop continues).
	if !strings.Contains(out, `"record_type":"scan_summary"`) {
		t.Errorf("valid record after malformed line was dropped; got:\n%s", out)
	}
}

// TestScanPollenUnavailableWritesAuditStatus covers the pollen-unavailable branch
// WITH an AuditPath: the scan_status record must be appended to the audit log
// (appendRawAuditLine), not just streamed to stdout.
func TestScanPollenUnavailableWritesAuditStatus(t *testing.T) {
	old := runPollenFn
	defer func() { runPollenFn = old }()
	runPollenFn = func(_ context.Context, _ bool) (<-chan []byte, bool) { return nil, false }

	auditPath := filepath.Join(t.TempDir(), "nested", "audit.ndjson") // nested → exercises MkdirAll

	var buf bytes.Buffer
	cfg := Config{AuditPath: auditPath} // no ExtensionDirs → beekeeperScan no-ops
	if err := Scan(context.Background(), cfg, &buf); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !strings.Contains(buf.String(), `"pollen_unavailable":true`) {
		t.Errorf("want pollen_unavailable status on stdout; got:\n%s", buf.String())
	}

	auditBytes, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}
	if !strings.Contains(string(auditBytes), `"record_type":"scan_status"`) {
		t.Errorf("scan_status not appended to audit log; got:\n%s", auditBytes)
	}
	if !strings.Contains(string(auditBytes), `"pollen_unavailable":true`) {
		t.Errorf("scan_status audit line missing pollen_unavailable flag; got:\n%s", auditBytes)
	}
}

// TestBeekeeperScanSkipBranches covers beekeeperScan's skip/continue branches:
//   - a bad IndexPath → "catalog index unavailable" scan_error, scan continues
//   - an unreadable extension dir entry (a regular file, not a directory) → skipped
//   - a directory without a manifest (ErrNoManifest) → skipped
// No extension produces a finding here, so the only finding-less scan still
// returns nil and reports the index error.
func TestBeekeeperScanSkipBranches(t *testing.T) {
	old := runPollenFn
	defer func() { runPollenFn = old }()
	runPollenFn = func(_ context.Context, _ bool) (<-chan []byte, bool) { return nil, false }

	extRoot := t.TempDir()
	// A regular file at the top level → entry.IsDir() == false → skipped.
	if err := os.WriteFile(filepath.Join(extRoot, "loose-file.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A directory with NO package.json → ParseManifest returns ErrNoManifest → skipped.
	if err := os.MkdirAll(filepath.Join(extRoot, "not-an-extension"), 0o700); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	cfg := Config{
		ExtensionDirs: []string{
			extRoot,
			filepath.Join(extRoot, "does-not-exist"), // ReadDir error → continue
		},
		IndexPath:  filepath.Join(t.TempDir(), "missing.idx"), // open fails → scan_error
		HTTPClient: &http.Client{Timeout: 200 * time.Millisecond},
	}
	if err := Scan(context.Background(), cfg, &buf); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "catalog index unavailable") {
		t.Errorf("expected catalog-index-unavailable scan_error; got:\n%s", out)
	}
	// No findings: nothing was a valid extension.
	if strings.Contains(out, `"record_type":"finding"`) {
		t.Errorf("did not expect any finding record; got:\n%s", out)
	}
}

// TestScanDefaultsNowAndClient covers the nil-Now / nil-HTTPClient default
// branches at the top of Scan (it must not panic and must run a clean scan).
func TestScanDefaultsNowAndClient(t *testing.T) {
	old := runPollenFn
	defer func() { runPollenFn = old }()
	runPollenFn = func(_ context.Context, _ bool) (<-chan []byte, bool) { return nil, false }

	var buf bytes.Buffer
	// Config{} leaves Now and HTTPClient nil → Scan fills both in.
	if err := Scan(context.Background(), Config{}, &buf); err != nil {
		t.Fatalf("Scan with zero Config: %v", err)
	}
	if !strings.Contains(buf.String(), `"pollen_unavailable":true`) {
		t.Errorf("want pollen_unavailable status; got:\n%s", buf.String())
	}
}

// auditDecisionRec is the subset of an NDJSON audit line the coverage tests assert on.
type auditDecisionRec struct {
	RecordType string `json:"record_type"`
	RecordID   string `json:"record_id"`
	Decision   string `json:"decision"`
}

// readPolicyDecisionAudit returns the policy_decision record from an audit log
// that may also contain a scan_status line (the pollen-unavailable branch writes
// to the same AuditPath). It fails the test if no policy_decision record exists.
func readPolicyDecisionAudit(t *testing.T, path string) auditDecisionRec {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		var rec auditDecisionRec
		if err := dec.Decode(&rec); err != nil {
			break
		}
		if rec.RecordType == "policy_decision" {
			return rec
		}
	}
	t.Fatalf("no policy_decision record in audit log:\n%s", data)
	return auditDecisionRec{}
}

func containsStr(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
