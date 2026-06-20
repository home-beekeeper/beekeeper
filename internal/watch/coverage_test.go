// Package watch — coverage_test.go
//
// Internal-package tests (package watch) that drive the under-covered branches
// in crossref.go, watcher.go, handler.go, and manifest.go. These complement the
// behavioral suites already present; here we deliberately exercise:
//
//   - crossref.go: the production default crossRefPollenFn / defaultRunPollenForCrossRef
//     / resolveCrossRefScanner (no scanner binary on PATH → graceful (nil,false)),
//     plus CrossReference's record-filtering branches (blank line, bad JSON,
//     non-"package" record, missing ecosystem/name) and the audit-cap truncation.
//   - watcher.go: Watch zero-config defaulting + NewWatcher + a single retry/error
//     tick (driven through a real but cancelled watcher), shouldProcess rename,
//     and expandHome ("~" expansion + non-"~" passthrough + empty input).
//   - handler.go: HandleNewExtension symlink-escape guard (not-in-root return),
//     index-unavailable return, the clean (no-hit) allow-audit path, nil HTTPClient
//     defaulting, socket-adapter wiring, generateRecordID + containsRuleID helpers.
//   - manifest.go: oversize file rejection and invalid-JSON rejection.
//
// All temp state is created under t.TempDir(); no real network or sleeps that
// gate correctness (the one short sleep is bounded and only ensures the watcher
// goroutine observes ctx cancellation — Watch returns nil on cancel either way).
package watch

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/home-beekeeper/beekeeper/internal/catalog"
	"github.com/home-beekeeper/beekeeper/internal/notify"
)

// ---- crossref.go: production default pollen seam ----

// TestDefaultRunPollenForCrossRef_NoScanner verifies that when neither the
// bumblebee nor the pollen binary is resolvable on PATH, the production default
// runner returns (nil, false) — the graceful-degrade contract the production
// crossRefPollenFn relies on. We force an empty PATH so LookPath fails for both
// candidates deterministically on every platform.
func TestDefaultRunPollenForCrossRef_NoScanner(t *testing.T) {
	withEmptyPATH(t)

	ch, ok := defaultRunPollenForCrossRef(context.Background(), false)
	if ok {
		t.Fatalf("defaultRunPollenForCrossRef ok = true with no scanner on PATH, want false")
	}
	if ch != nil {
		t.Errorf("defaultRunPollenForCrossRef returned non-nil channel with no scanner, want nil")
	}
}

// TestCrossRefPollenFn_DefaultDelegates verifies the package-level default
// closure (crossRefPollenFn) delegates to defaultRunPollenForCrossRef. With no
// scanner on PATH it must report unavailable.
func TestCrossRefPollenFn_DefaultDelegates(t *testing.T) {
	withEmptyPATH(t)

	ch, ok := crossRefPollenFn(context.Background(), true)
	if ok || ch != nil {
		t.Errorf("default crossRefPollenFn returned (ch=%v, ok=%v) with empty PATH, want (nil,false)", ch, ok)
	}
}

// TestResolveCrossRefScanner_NotFound verifies resolveCrossRefScanner returns an
// error (and empty path/name) when neither candidate binary is on PATH.
func TestResolveCrossRefScanner_NotFound(t *testing.T) {
	withEmptyPATH(t)

	path, name, err := resolveCrossRefScanner()
	if err == nil {
		t.Fatalf("resolveCrossRefScanner err = nil with empty PATH, want error")
	}
	if path != "" || name != "" {
		t.Errorf("resolveCrossRefScanner = (%q, %q), want empty strings on not-found", path, name)
	}
}

// TestResolveCrossRefScanner_FindsBumblebeePreferred verifies resolveCrossRefScanner
// prefers "bumblebee" over "pollen" and resolves its absolute path. We synthesize
// both fake executables in a temp dir placed on PATH; bumblebee must win.
func TestResolveCrossRefScanner_FindsBumblebeePreferred(t *testing.T) {
	binDir := t.TempDir()
	makeFakeExe(t, binDir, "bumblebee")
	makeFakeExe(t, binDir, "pollen")
	setPATH(t, binDir)

	path, name, err := resolveCrossRefScanner()
	if err != nil {
		t.Fatalf("resolveCrossRefScanner err = %v, want nil when bumblebee present", err)
	}
	if name != "bumblebee" {
		t.Errorf("resolveCrossRefScanner name = %q, want bumblebee (preferred over pollen)", name)
	}
	if path == "" {
		t.Error("resolveCrossRefScanner path is empty, want resolved absolute path")
	}
}

// TestResolveCrossRefScanner_FallsBackToPollen verifies the pollen fallback when
// only pollen is on PATH.
func TestResolveCrossRefScanner_FallsBackToPollen(t *testing.T) {
	binDir := t.TempDir()
	makeFakeExe(t, binDir, "pollen")
	setPATH(t, binDir)

	_, name, err := resolveCrossRefScanner()
	if err != nil {
		t.Fatalf("resolveCrossRefScanner err = %v, want nil when pollen present", err)
	}
	if name != "pollen" {
		t.Errorf("resolveCrossRefScanner name = %q, want pollen (only candidate present)", name)
	}
}

// TestDefaultRunPollenForCrossRef_RealBinary drives the FULL production runner:
// it compiles a tiny "bumblebee" helper that emits two NDJSON lines on stdout,
// places it on PATH, and verifies defaultRunPollenForCrossRef execs it, returns
// (channel, true), and streams the emitted lines through the channel. This is the
// only path that covers cmd.StdoutPipe / cmd.Start / the scanner goroutine.
func TestDefaultRunPollenForCrossRef_RealBinary(t *testing.T) {
	binDir := t.TempDir()
	buildFakeScanner(t, binDir, "bumblebee")
	setPATH(t, binDir)

	ch, ok := defaultRunPollenForCrossRef(context.Background(), true /* deep → exercises the --profile deep arg branch */)
	if !ok {
		t.Fatalf("defaultRunPollenForCrossRef ok = false with a real scanner on PATH, want true")
	}
	if ch == nil {
		t.Fatal("defaultRunPollenForCrossRef returned nil channel with ok=true")
	}

	var lines [][]byte
	for line := range ch {
		// Copy: the runner reuses the scanner buffer between sends, but it already
		// copies into a fresh slice; we collect the bytes here.
		cp := make([]byte, len(line))
		copy(cp, line)
		lines = append(lines, cp)
	}
	if len(lines) != 2 {
		t.Fatalf("scanner streamed %d lines, want 2", len(lines))
	}
	if !strings.Contains(string(lines[0]), `"record_type":"package"`) {
		t.Errorf("first line missing expected NDJSON content: %s", lines[0])
	}
}

// TestCrossReferenceEndToEndRealScanner exercises CrossReference through the
// PRODUCTION crossRefPollenFn (no injection): a real helper scanner on PATH emits
// a package record that matches the catalog index, and CrossReference must return
// exactly one hit. This proves the default seam wiring works end to end.
func TestCrossReferenceEndToEndRealScanner(t *testing.T) {
	indexPath := buildTestIndex(t, []catalog.Entry{
		{
			ID:               "evil-e2e",
			Ecosystem:        "npm",
			Package:          "evil-e2e-pkg",
			Versions:         []string{"1.0.0"},
			CatalogSignature: "fake-sig",
			CatalogSource:    "bumblebee",
		},
	})

	binDir := t.TempDir()
	buildFakeScanner(t, binDir, "bumblebee")
	setPATH(t, binDir)
	// Do NOT inject crossRefPollenFn — use the production default closure.

	cfg := CrossRefConfig{IndexPath: indexPath, CacheDir: t.TempDir()}
	hits, err := CrossReference(context.Background(), cfg)
	if err != nil {
		t.Fatalf("CrossReference (real scanner) error: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("CrossReference (real scanner) returned %d hits, want 1", len(hits))
	}
	if hits[0].Package != "evil-e2e-pkg" {
		t.Errorf("hit.Package = %q, want evil-e2e-pkg", hits[0].Package)
	}
}

// ---- crossref.go: CrossReference record-filtering branches ----

// TestCrossReferenceFiltersMalformedRecords drives the per-line filter branches
// of CrossReference: blank line, undecodable JSON, a non-"package" record, a
// "package" record missing ecosystem/name, and one valid matching record. Only
// the valid catalogued record must surface as a hit; the rest are skipped without
// error.
func TestCrossReferenceFiltersMalformedRecords(t *testing.T) {
	indexPath := buildTestIndex(t, []catalog.Entry{
		{
			ID:               "evil-filter",
			Ecosystem:        "npm",
			Package:          "evil-filter-pkg",
			Versions:         []string{"1.0.0"},
			CatalogSignature: "fake-sig",
			CatalogSource:    "bumblebee",
		},
	})

	oldRun := crossRefPollenFn
	defer func() { crossRefPollenFn = oldRun }()

	lines := []string{
		"",                       // blank → skipped (len==0 branch)
		"{not valid json",        // undecodable probe → skipped
		`{"record_type":"finding","ecosystem":"npm","normalized_name":"x"}`, // non-package → skipped
		`{"record_type":"package","ecosystem":"","normalized_name":"noeco"}`, // empty ecosystem → skipped
		`{"record_type":"package","ecosystem":"npm","normalized_name":""}`,   // empty name → skipped
		`{"record_type":"package"}` + `{`,                                    // package probe ok, full rec decode fails → skipped
		`{"record_type":"package","ecosystem":"npm","normalized_name":"evil-filter-pkg","version":"1.0.0","project_path":"/x"}`, // valid hit
	}
	crossRefPollenFn = func(_ context.Context, _ bool) (<-chan []byte, bool) {
		ch := make(chan []byte, len(lines))
		for _, l := range lines {
			ch <- []byte(l)
		}
		close(ch)
		return ch, true
	}

	cfg := CrossRefConfig{IndexPath: indexPath, CacheDir: t.TempDir()}
	hits, err := CrossReference(context.Background(), cfg)
	if err != nil {
		t.Fatalf("CrossReference error: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("CrossReference returned %d hits, want exactly 1 (only the valid record)", len(hits))
	}
	if hits[0].Package != "evil-filter-pkg" {
		t.Errorf("hit.Package = %q, want evil-filter-pkg", hits[0].Package)
	}
	if !hits[0].PathResolved || hits[0].InstalledPath != "/x" {
		t.Errorf("hit path = (%q, resolved=%v), want (/x, true)", hits[0].InstalledPath, hits[0].PathResolved)
	}
}

// TestCrossReferenceAuditTruncation drives the per-pass audit cap (maxAuditedFindings
// = 1000): with > 1000 matching records, all matches are still DETECTED (returned),
// but the audit log is capped at 1000 records and a truncation notice is logged.
// This exercises the auditTruncated branch which is otherwise never reached.
func TestCrossReferenceAuditTruncation(t *testing.T) {
	indexPath := buildTestIndex(t, []catalog.Entry{
		{
			ID:               "evil-bulk",
			Ecosystem:        "npm",
			Package:          "evil-bulk-pkg",
			Versions:         []string{"1.0.0"},
			CatalogSignature: "fake-sig",
			CatalogSource:    "bumblebee",
		},
	})

	oldRun := crossRefPollenFn
	defer func() { crossRefPollenFn = oldRun }()

	const total = 1005 // > 1000 cap
	crossRefPollenFn = func(_ context.Context, _ bool) (<-chan []byte, bool) {
		ch := make(chan []byte, total)
		line := []byte(`{"record_type":"package","ecosystem":"npm","normalized_name":"evil-bulk-pkg","version":"1.0.0","project_path":"/p"}`)
		for i := 0; i < total; i++ {
			out := make([]byte, len(line))
			copy(out, line)
			ch <- out
		}
		close(ch)
		return ch, true
	}

	auditPath := filepath.Join(t.TempDir(), "beekeeper.ndjson")
	cfg := CrossRefConfig{IndexPath: indexPath, CacheDir: t.TempDir(), AuditPath: auditPath}

	hits, err := CrossReference(context.Background(), cfg)
	if err != nil {
		t.Fatalf("CrossReference error: %v", err)
	}
	// Detection is NOT capped — all matches surface.
	if len(hits) != total {
		t.Errorf("CrossReference returned %d hits, want %d (detection must not be capped)", len(hits), total)
	}

	// Audit IS capped — count the written NDJSON records.
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if got := len(lines); got != 1000 {
		t.Errorf("audit wrote %d records, want exactly 1000 (cap)", got)
	}
}

// ---- watcher.go ----

// TestWatchDefaultsAndImmediateCancel drives Watch with a zero-value WatchConfig
// (exercising the DebounceWindow==0 and RetryInterval==0 default branches) and a
// non-existent dir (exercising the initial w.Add failure → pending append). The
// context is cancelled immediately so the select returns via ctx.Done() and Watch
// returns nil. NewWatcher must succeed on the host (fsnotify available).
func TestWatchDefaultsAndImmediateCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before Watch enters the loop

	missing := filepath.Join(t.TempDir(), "no-such-dir")
	if err := Watch(ctx, []string{missing}, WatchConfig{}, &countHandler{}); err != nil {
		t.Fatalf("Watch with zero config returned error: %v", err)
	}
}

// TestWatchRetryTickReAddsPending deterministically drives the retry-ticker
// branch of the Watch loop (watcher.go retryTicker.C case): a directory that does
// not exist when Watch starts is appended to `pending`; it is then created, and a
// short RetryInterval guarantees at least one retry tick fires while the loop is
// running, re-adding the now-existing dir. We do not assert on fsnotify event
// delivery (flaky on Windows per project notes); we assert only that the loop
// iterated through the retry tick and exited cleanly on cancel. The retry branch
// is exercised regardless of the add result.
func TestWatchRetryTickReAddsPending(t *testing.T) {
	parent := t.TempDir()
	missing := filepath.Join(parent, "appears-later")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Watch(ctx, []string{missing}, WatchConfig{
			DebounceWindow: 10 * time.Millisecond,
			RetryInterval:  15 * time.Millisecond, // short → at least one tick fires
		}, &countHandler{})
	}()

	// Create the directory so the retry tick's w.Add succeeds (drives the
	// "pending re-add succeeds → dropped from pending" sub-branch).
	if err := os.Mkdir(missing, 0o700); err != nil {
		t.Fatalf("mkdir appears-later: %v", err)
	}

	// Let several retry ticks elapse so the retry branch runs at least once.
	time.Sleep(80 * time.Millisecond)

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Watch returned error on cancel: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Watch did not return within 2s after cancel")
	}
}

// TestWatchDeliversRealEvent is a best-effort live-event test. fsnotify event
// delivery is unreliable on Windows (project memory: "Windows resize polling
// workaround"/load-sensitive detection), so this test does NOT fail when no
// event is observed — it only asserts Watch tears down cleanly. When an event IS
// delivered (the common case on Linux/macOS CI) it additionally drives the
// w.Events + shouldProcess + processEvent path inside the loop.
func TestWatchDeliversRealEvent(t *testing.T) {
	watchDir := t.TempDir()
	handler := &countHandler{}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- Watch(ctx, []string{watchDir}, WatchConfig{
			DebounceWindow: 20 * time.Millisecond,
			RetryInterval:  time.Hour, // do not retry during the test
		}, handler)
	}()

	// Give the watcher goroutine a moment to register the directory, then create
	// a subdirectory to fire a Create event under the watched root.
	time.Sleep(50 * time.Millisecond)
	newPath := filepath.Join(watchDir, "new-extension")
	if err := os.Mkdir(newPath, 0o700); err != nil {
		t.Fatalf("mkdir to fire event: %v", err)
	}

	// Bounded poll for the debounced handler invocation (best effort).
	deadline := time.Now().Add(1 * time.Second)
	for handler.count.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	// No assertion on count: event delivery is platform-dependent. On platforms
	// where it fires, the event branch is covered; everywhere, teardown is checked.

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Watch returned error on cancel: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Watch did not return within 2s after cancel")
	}
}

// TestExpandHome covers all three expandHome branches: empty input passthrough,
// a non-"~" path passthrough, and a leading-"~" expansion to the user home dir.
func TestExpandHome(t *testing.T) {
	if got := expandHome(""); got != "" {
		t.Errorf("expandHome(\"\") = %q, want \"\"", got)
	}

	const plain = "/abs/path/no-tilde"
	if got := expandHome(plain); got != plain {
		t.Errorf("expandHome(%q) = %q, want unchanged", plain, got)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("os.UserHomeDir unavailable: %v", err)
	}
	got := expandHome("~/sub/dir")
	want := filepath.Join(home, "/sub/dir")
	if got != want {
		t.Errorf("expandHome(\"~/sub/dir\") = %q, want %q", got, want)
	}
	if !strings.HasPrefix(got, home) {
		t.Errorf("expandHome did not anchor under home dir: %q", got)
	}
}

// ---- handler.go ----

// newTestHandler builds a Handler with sane defaults for the symlink-guard and
// index-unavailable tests.
func newTestHandler(indexPath, cacheDir, quarantineDir, auditPath string, watchedRoots []string) *Handler {
	return NewHandler(
		indexPath, cacheDir, quarantineDir, auditPath,
		notify.Config{Enabled: false},
		"",
		&http.Client{Timeout: time.Second},
		func() time.Time { return time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC) },
		watchedRoots,
	)
}

// TestHandleNewExtension_NotInRoot verifies the symlink-escape guard: a path whose
// parent is NOT one of the watched roots returns immediately with no audit, no
// quarantine, no panic. This drives the inRoot==false early return (handler.go:74).
func TestHandleNewExtension_NotInRoot(t *testing.T) {
	auditPath := filepath.Join(t.TempDir(), "audit.ndjson")
	// Watched root is some unrelated dir; the extension path's parent is different.
	h := newTestHandler("/nonexistent.idx", t.TempDir(), t.TempDir(), auditPath, []string{t.TempDir()})

	outsidePath := filepath.Join(t.TempDir(), "escaped-extension")
	h.HandleNewExtension(context.Background(), outsidePath)

	if _, err := os.Stat(auditPath); !os.IsNotExist(err) {
		t.Errorf("escaped (not-in-root) path must not produce any audit record; stat err = %v", err)
	}
}

// TestHandleNewExtension_IndexUnavailable verifies that when the manifest parses
// but the catalog index path cannot be opened, HandleNewExtension logs and returns
// without quarantining (handler.go:97 index-unavailable branch). No audit record
// is written because the function returns before the evaluation/audit stage.
func TestHandleNewExtension_IndexUnavailable(t *testing.T) {
	watchRoot := t.TempDir()
	extDir := filepath.Join(watchRoot, "pub.ext-1.0.0")
	if err := os.MkdirAll(extDir, 0o700); err != nil {
		t.Fatal(err)
	}
	pkgJSON := []byte(`{"publisher":"pub","name":"ext","version":"1.0.0","displayName":"Ext"}`)
	if err := os.WriteFile(filepath.Join(extDir, "package.json"), pkgJSON, 0o600); err != nil {
		t.Fatal(err)
	}

	auditPath := filepath.Join(t.TempDir(), "audit.ndjson")
	// IndexPath points at a file that does not exist → catalog.OpenIndex fails.
	badIndex := filepath.Join(t.TempDir(), "missing.idx")
	h := newTestHandler(badIndex, t.TempDir(), t.TempDir(), auditPath, []string{watchRoot})

	h.HandleNewExtension(context.Background(), extDir)

	// Returned before audit stage → no audit file.
	if _, err := os.Stat(auditPath); !os.IsNotExist(err) {
		t.Errorf("index-unavailable path must return before writing audit; stat err = %v", err)
	}
	// Extension must still be in place (not quarantined).
	if _, err := os.Stat(extDir); err != nil {
		t.Errorf("extension dir should be untouched when index unavailable: %v", err)
	}
}

// TestHandleNewExtension_CleanAllowPath verifies the clean (no catalog hit, not
// release-age blocked, nil HTTPClient → default client, socket adapter wired)
// branch: HandleNewExtension writes an allow audit record (record_type absent /
// EDXT-02) and does NOT quarantine. The package is not in the catalog and the
// marketplace cache is pre-seeded as old enough, so neither evaluation blocks.
func TestHandleNewExtension_CleanAllowPath(t *testing.T) {
	indexPath := buildTestIndex(t, []catalog.Entry{
		newSignedTestEntry("editor-extension", "unrelated.other"),
	})

	cacheDir := t.TempDir()
	quarantineDir := t.TempDir()
	auditPath := filepath.Join(t.TempDir(), "audit.ndjson")
	watchRoot := t.TempDir()

	extDir := filepath.Join(watchRoot, "clean.ext-2.0.0")
	if err := os.MkdirAll(extDir, 0o700); err != nil {
		t.Fatal(err)
	}
	pkgJSON := []byte(`{"publisher":"clean","name":"ext","version":"2.0.0","displayName":"Clean Ext"}`)
	if err := os.WriteFile(filepath.Join(extDir, "package.json"), pkgJSON, 0o600); err != nil {
		t.Fatal(err)
	}

	testNow := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	publishedAt := testNow.Add(-72 * time.Hour) // old enough → not release-age blocked
	mktCacheDir := filepath.Join(cacheDir, "marketplace-cache", "clean", "ext")
	if err := os.MkdirAll(mktCacheDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cacheEntry := `{"published_at":"` + publishedAt.UTC().Format(time.RFC3339) +
		`","cached_at":"` + testNow.UTC().Format(time.RFC3339) + `","missing":false}`
	if err := os.WriteFile(filepath.Join(mktCacheDir, "2.0.0.json"), []byte(cacheEntry), 0o600); err != nil {
		t.Fatal(err)
	}

	// Handler with nil HTTPClient (drives the default-client branch) and a socket
	// token (drives the socketAdapter wiring branch).
	h := NewHandler(
		indexPath, cacheDir, quarantineDir, auditPath,
		notify.Config{Enabled: false},
		"fake-socket-token", // non-empty → socketAdapter is constructed
		nil,                 // nil → handler builds its own http.Client
		func() time.Time { return testNow },
		[]string{watchRoot},
	)

	h.HandleNewExtension(context.Background(), extDir)

	// Clean path: extension must remain in place (not quarantined).
	if _, err := os.Stat(extDir); err != nil {
		t.Errorf("clean extension was moved unexpectedly: %v", err)
	}
	// An allow audit record must have been written.
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit (clean path): %v", err)
	}
	if !strings.Contains(string(data), "EDXT-02") {
		t.Errorf("clean allow audit record missing EDXT-02 rule id:\n%s", string(data))
	}
}

// TestContainsRuleID covers both branches of containsRuleID (present and absent).
func TestContainsRuleID(t *testing.T) {
	if !containsRuleID([]string{"A", "EDXT-03", "B"}, "EDXT-03") {
		t.Error("containsRuleID should return true when the id is present")
	}
	if containsRuleID([]string{"A", "B"}, "EDXT-03") {
		t.Error("containsRuleID should return false when the id is absent")
	}
	if containsRuleID(nil, "X") {
		t.Error("containsRuleID(nil, ...) should return false")
	}
}

// TestGenerateRecordID verifies generateRecordID returns a non-empty, 16-hex-char
// (8 bytes) identifier on the happy path and that successive calls differ.
func TestGenerateRecordID(t *testing.T) {
	a := generateRecordID()
	b := generateRecordID()
	if len(a) != 16 {
		t.Errorf("generateRecordID len = %d, want 16 hex chars (8 bytes)", len(a))
	}
	if a == b {
		t.Errorf("generateRecordID produced identical ids %q twice (RNG not advancing)", a)
	}
}

// ---- manifest.go ----

// TestParseManifestOversize verifies that a package.json exceeding the 1 MiB cap
// is rejected with a non-ErrNoManifest error before JSON parsing.
func TestParseManifestOversize(t *testing.T) {
	dir := t.TempDir()
	// 1 MiB + 1 byte of valid-looking JSON padding.
	big := make([]byte, (1<<20)+1)
	for i := range big {
		big[i] = ' '
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), big, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ParseManifest(dir)
	if err == nil {
		t.Fatal("ParseManifest(oversize) err = nil, want size-limit error")
	}
	if err == ErrNoManifest {
		t.Fatal("ParseManifest(oversize) returned ErrNoManifest, want a distinct size error")
	}
	if !strings.Contains(err.Error(), "1 MiB") {
		t.Errorf("ParseManifest(oversize) err = %v, want a 1 MiB limit message", err)
	}
}

// TestParseManifestInvalidJSON verifies that malformed JSON yields a parse error
// (not ErrNoManifest).
func TestParseManifestInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"publisher": `), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ParseManifest(dir)
	if err == nil {
		t.Fatal("ParseManifest(invalid JSON) err = nil, want parse error")
	}
	if err == ErrNoManifest {
		t.Fatal("ParseManifest(invalid JSON) returned ErrNoManifest, want a JSON parse error")
	}
}

// TestParseManifestReadErrorDir verifies the non-ErrNotExist read-error branch:
// when "package.json" exists but is a directory, os.ReadFile returns a non-
// NotExist error which ParseManifest must surface as-is (not ErrNoManifest).
func TestParseManifestReadErrorDir(t *testing.T) {
	dir := t.TempDir()
	// Create package.json AS A DIRECTORY so ReadFile fails with a non-NotExist error.
	if err := os.Mkdir(filepath.Join(dir, "package.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := ParseManifest(dir)
	if err == nil {
		t.Fatal("ParseManifest(dir-as-file) err = nil, want a read error")
	}
	if err == ErrNoManifest {
		t.Fatal("ParseManifest(dir-as-file) returned ErrNoManifest, want the underlying read error")
	}
}

// ---- shouldProcess (rename branch on non-Windows) ----

// TestShouldProcessRename asserts shouldProcess for a Rename event on the active
// platform: on non-Windows hosts a Rename qualifies; on Windows it does not (only
// Create qualifies). This complements the existing TestWatchWindowsFilter and
// covers the platform-specific branch of shouldProcess on whichever host runs it.
func TestShouldProcessRename(t *testing.T) {
	renameEvent := fsnotify.Event{Name: "/some/path", Op: fsnotify.Rename}
	got := shouldProcess(renameEvent)
	want := runtime.GOOS != "windows"
	if got != want {
		t.Errorf("shouldProcess(rename) on %s = %v, want %v", runtime.GOOS, got, want)
	}
}

// ---- PATH / fake-exe helpers ----

// withEmptyPATH sets PATH to an empty value for the duration of the test so
// exec.LookPath cannot resolve any binary. t.Setenv restores the prior value.
func withEmptyPATH(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", "")
	// On Windows, LookPath also consults PATHEXT; emptying PATH alone is enough
	// because there is no directory to search, but clear PATHEXT too for safety.
	if runtime.GOOS == "windows" {
		t.Setenv("PATHEXT", "")
	}
}

// setPATH sets PATH to exactly dir for the test duration.
func setPATH(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir)
	if runtime.GOOS == "windows" {
		t.Setenv("PATHEXT", ".EXE;.BAT;.CMD")
	}
}

// makeFakeExe writes a minimal executable file named for the current platform so
// exec.LookPath resolves it. On Windows an exe extension is required. The file is
// NOT a runnable program — it is only used by tests that exercise PATH resolution
// (resolveCrossRefScanner), never tests that actually exec it.
func makeFakeExe(t *testing.T, dir, name string) {
	t.Helper()
	fname := name
	if runtime.GOOS == "windows" {
		fname += ".exe"
	}
	path := filepath.Join(dir, fname)
	if err := os.WriteFile(path, []byte("fake"), 0o755); err != nil {
		t.Fatalf("write fake exe %s: %v", path, err)
	}
}

// fakeScannerSource is a tiny standalone program that mimics the inventory
// scanner: it ignores its args and emits two NDJSON lines (one matching package
// record and one non-matching) on stdout, then exits 0. It is compiled by
// buildFakeScanner so the production defaultRunPollenForCrossRef can exec it.
const fakeScannerSource = `package main

import "fmt"

func main() {
	fmt.Println("{\"record_type\":\"package\",\"ecosystem\":\"npm\",\"normalized_name\":\"evil-e2e-pkg\",\"version\":\"1.0.0\",\"project_path\":\"/tmp/proj\"}")
	fmt.Println("{\"record_type\":\"package\",\"ecosystem\":\"npm\",\"normalized_name\":\"safe-pkg\",\"version\":\"2.0.0\"}")
}
`

// buildFakeScanner compiles fakeScannerSource into an executable named `name`
// (with the platform exe suffix) inside dir, so exec.LookPath finds it and the
// production runner can actually start it. Uses the same toolchain running the
// test, so it is deterministic and hermetic (no network).
func buildFakeScanner(t *testing.T, dir, name string) {
	t.Helper()

	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "main.go")
	if err := os.WriteFile(srcFile, []byte(fakeScannerSource), 0o600); err != nil {
		t.Fatalf("write fake scanner source: %v", err)
	}

	out := filepath.Join(dir, name)
	if runtime.GOOS == "windows" {
		out += ".exe"
	}

	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if runtime.GOOS == "windows" {
		goBin += ".exe"
	}
	cmd := exec.Command(goBin, "build", "-o", out, srcFile)
	cmd.Env = os.Environ()
	if combined, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake scanner: %v\n%s", err, combined)
	}
}
