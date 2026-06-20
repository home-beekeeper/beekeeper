package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- selfcatalog: SelfCatalogOutcome.String ---

// TestSelfCatalogOutcomeString covers every named outcome plus the default arm.
func TestSelfCatalogOutcomeString(t *testing.T) {
	cases := map[SelfCatalogOutcome]string{
		SelfCatalogContinue:     "continue",
		SelfCatalogQuarantine:   "quarantine",
		SelfCatalogFailClosed:   "fail-closed",
		SelfCatalogWarnContinue: "warn-continue",
	}
	for o, want := range cases {
		if got := o.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", int(o), got, want)
		}
	}
	if got := SelfCatalogOutcome(99).String(); got != "SelfCatalogOutcome(99)" {
		t.Errorf("unknown outcome String() = %q, want SelfCatalogOutcome(99)", got)
	}
}

// TestSelfCacheEmptyDirNoOp: empty cacheDir → write is a no-op (nil), read is a miss.
func TestSelfCacheEmptyDirNoOp(t *testing.T) {
	if err := writeSelfCache("", []byte(`{}`)); err != nil {
		t.Errorf("writeSelfCache('') = %v, want nil no-op", err)
	}
	data, age, err := readSelfCache("")
	if err != nil || data != nil || age != 0 {
		t.Errorf("readSelfCache('') = (%v,%v,%v), want (nil,0,nil)", data, age, err)
	}
}

// TestSelfCacheRoundTrip: write then read returns the same bytes with a small age.
func TestSelfCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := []byte(`{"schema_version":"1","entries":[]}`)
	if err := writeSelfCache(dir, want); err != nil {
		t.Fatalf("writeSelfCache: %v", err)
	}
	got, age, err := readSelfCache(dir)
	if err != nil {
		t.Fatalf("readSelfCache: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("round-trip data = %q, want %q", got, want)
	}
	if age < 0 || age > time.Minute {
		t.Errorf("age = %v, want a small positive duration", age)
	}
}

// TestReadSelfCacheCorrupt: corrupt cache JSON → miss (nil,0,nil), not an error.
func TestReadSelfCacheCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := selfCatalogCachePath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{not json`), 0o600); err != nil {
		t.Fatalf("write corrupt cache: %v", err)
	}
	data, age, err := readSelfCache(dir)
	if err != nil || data != nil || age != 0 {
		t.Errorf("readSelfCache(corrupt) = (%v,%v,%v), want (nil,0,nil)", data, age, err)
	}
}

// --- diff.go: snapshot read error path drives startsWith + (read error) marker ---

// TestDiffSnapshotReadErrorMarker: when the snapshot reader errors, Diff records
// a "(read error: ...)" CurrentHash and Changed stays false (startsWith guard).
func TestDiffSnapshotReadErrorMarker(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")
	// Seed a state with a bumblebee source so Diff iterates it.
	st := WatchState{Sources: map[string]SourceState{
		bumblebeeSource: {Hash: "deadbeef", Count: 5},
	}}
	if err := SaveState(stateFile, st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	// Point the catalogDir at a directory whose bumblebee.json is itself a
	// directory, so os.ReadFile returns a non-IsNotExist error → snapErr != nil.
	catalogDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(catalogDir, "bumblebee.json"), 0o700); err != nil {
		t.Fatalf("mkdir bumblebee.json: %v", err)
	}

	results, err := Diff(context.Background(), stateFile, catalogDir, http.DefaultClient)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	var found bool
	for _, r := range results {
		if r.Source == bumblebeeSource {
			found = true
			if !startsWithRead(r.CurrentHash) {
				t.Errorf("CurrentHash = %q, want a (read error ...) marker", r.CurrentHash)
			}
			if r.Changed {
				t.Error("Changed = true, want false (read-error marker must not count as changed)")
			}
		}
	}
	if !found {
		t.Fatal("bumblebee source not present in diff results")
	}
}

// startsWithRead mirrors the production startsWith guard for the assertion.
func startsWithRead(s string) bool {
	const p = "(read error"
	return len(s) >= len(p) && s[:len(p)] == p
}

// TestDiffNoChangesWhenHashMatches: equal hash/count yields no diff.
func TestDiffNoChangesWhenHashMatches(t *testing.T) {
	dir := t.TempDir()
	catalogDir := t.TempDir()
	// Build a real bumblebee snapshot (json + idx) so the snapshot reader succeeds.
	entries := nxEntries()
	if err := BuildIndex(filepath.Join(catalogDir, "bumblebee.idx"), entries); err != nil {
		t.Fatalf("build index: %v", err)
	}
	rawJSON := []byte(`{"schema_version":"0.1.0","entries":[]}`)
	if err := os.WriteFile(filepath.Join(catalogDir, "bumblebee.json"), rawJSON, 0o600); err != nil {
		t.Fatalf("write bumblebee.json: %v", err)
	}
	sum := sha256.Sum256(rawJSON)
	hash := hex.EncodeToString(sum[:])

	stateFile := filepath.Join(dir, "state.json")
	st := WatchState{Sources: map[string]SourceState{
		bumblebeeSource: {Hash: hash, Count: len(entries)},
	}}
	if err := SaveState(stateFile, st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	results, err := Diff(context.Background(), stateFile, catalogDir, http.DefaultClient)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	for _, r := range results {
		if r.Source == bumblebeeSource && r.HasChanges() {
			t.Errorf("expected no changes, got %+v", r)
		}
	}
}

// TestDiffLoadStateError: a non-parseable state file makes Diff return an error.
func TestDiffLoadStateError(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")
	if err := os.WriteFile(stateFile, []byte(`{not valid json`), 0o600); err != nil {
		t.Fatalf("write bad state: %v", err)
	}
	if _, err := Diff(context.Background(), stateFile, t.TempDir(), http.DefaultClient); err == nil {
		t.Error("Diff with corrupt state must return an error")
	}
}

// --- state.go: error branches ---

// TestLoadStateParseError: corrupt state file → parse error.
func TestLoadStateParseError(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(p, []byte(`{"sources": [oops]}`), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := LoadState(p); err == nil {
		t.Error("LoadState(corrupt) = nil error, want parse error")
	}
}

// TestLoadStateMissingIsEmpty: a missing file is a first-run, not an error.
func TestLoadStateMissingIsEmpty(t *testing.T) {
	st, err := LoadState(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatalf("LoadState(missing): %v", err)
	}
	if st.Sources == nil {
		t.Error("Sources map is nil on first run, want non-nil")
	}
}

// TestLoadStateNullSourcesNormalised: a JSON null sources field is normalised to
// a non-nil map.
func TestLoadStateNullSourcesNormalised(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(p, []byte(`{"sources":null}`), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	st, err := LoadState(p)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if st.Sources == nil {
		t.Error("null sources not normalised to a non-nil map")
	}
}

// TestSaveStateRoundTrip: SaveState writes a parseable file that LoadState reads.
func TestSaveStateRoundTrip(t *testing.T) {
	// Nested path forces the MkdirAll branch.
	p := filepath.Join(t.TempDir(), "nested", "dir", "state.json")
	want := WatchState{Sources: map[string]SourceState{
		"bumblebee": {Hash: "h", Count: 3, Degraded: true, DegradedReason: "spike"},
	}}
	if err := SaveState(p, want); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	got, err := LoadState(p)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	src := got.Sources["bumblebee"]
	if src.Hash != "h" || src.Count != 3 || !src.Degraded || src.DegradedReason != "spike" {
		t.Errorf("round-trip mismatch: %+v", src)
	}
}

// --- watch.go: readBumblebeeSnapshot file-present path opens the mmap index ---

// TestReadBumblebeeSnapshotWithIndex: a present bumblebee.json + idx yields the
// index count and the sha256 of the raw json.
func TestReadBumblebeeSnapshotWithIndex(t *testing.T) {
	dir := t.TempDir()
	entries := nxEntries()
	if err := BuildIndex(filepath.Join(dir, "bumblebee.idx"), entries); err != nil {
		t.Fatalf("build index: %v", err)
	}
	raw := []byte(`{"schema_version":"0.1.0","entries":[]}`)
	if err := os.WriteFile(filepath.Join(dir, "bumblebee.json"), raw, 0o600); err != nil {
		t.Fatalf("write json: %v", err)
	}

	count, hash, err := readBumblebeeSnapshot(dir)(context.Background())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if count != len(entries) {
		t.Errorf("count = %d, want %d (from mmap index)", count, len(entries))
	}
	sum := sha256.Sum256(raw)
	if hash != hex.EncodeToString(sum[:]) {
		t.Errorf("hash = %q, want sha256 of raw json", hash)
	}
}

// TestReadBumblebeeSnapshotMissingFile: absent json → (0,"",nil) (unsynced state).
func TestReadBumblebeeSnapshotMissingFile(t *testing.T) {
	count, hash, err := readBumblebeeSnapshot(t.TempDir())(context.Background())
	if err != nil {
		t.Fatalf("snapshot(missing): %v", err)
	}
	if count != 0 || hash != "" {
		t.Errorf("snapshot(missing) = (%d,%q), want (0,\"\")", count, hash)
	}
}

// TestReadBumblebeeSnapshotJSONNoIndex: json present but no idx → count 0, valid hash.
func TestReadBumblebeeSnapshotJSONNoIndex(t *testing.T) {
	dir := t.TempDir()
	raw := []byte(`{"schema_version":"0.1.0","entries":[]}`)
	if err := os.WriteFile(filepath.Join(dir, "bumblebee.json"), raw, 0o600); err != nil {
		t.Fatalf("write json: %v", err)
	}
	count, hash, err := readBumblebeeSnapshot(dir)(context.Background())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 (no index)", count)
	}
	if hash == "" {
		t.Error("hash empty, want sha256 of json")
	}
}

// --- index.go: OpenIndex error paths ---

// TestOpenIndexMissingFile: a nonexistent path → error.
func TestOpenIndexMissingFile(t *testing.T) {
	if _, err := OpenIndex(filepath.Join(t.TempDir(), "nope.idx")); err == nil {
		t.Error("OpenIndex(missing) = nil error, want error")
	}
}

// TestOpenIndexTooSmall: a file shorter than the header → error.
func TestOpenIndexTooSmall(t *testing.T) {
	p := filepath.Join(t.TempDir(), "tiny.idx")
	if err := os.WriteFile(p, []byte{1, 2, 3}, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := OpenIndex(p); err == nil {
		t.Error("OpenIndex(too small) = nil error, want error")
	}
}

// --- age_cache.go: future-timestamp + unparseable + cache future paths ---

// TestFetchPublishAgeFutureTimestamp: a registry timestamp in the future is
// treated as missing (fail-closed) and a Missing cache entry is written.
func TestFetchPublishAgeFutureTimestamp(t *testing.T) {
	future := fixedNow.Add(48 * time.Hour)
	srv := npmPublishStub(t, "future-pkg", "1.0.0", future.Format(time.RFC3339))
	defer srv.Close()
	withBase(t, &npmRegistryBase, srv.URL)

	age, missing, err := FetchPublishAge(context.Background(), srv.Client(), t.TempDir(), "npm", "future-pkg", "1.0.0", fixedNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !missing || age != 0 {
		t.Errorf("future ts: (age=%d,missing=%v), want (0,true)", age, missing)
	}
}

// TestFetchPublishAgeUnparseableTimestamp: a non-RFC3339 timestamp → missing.
func TestFetchPublishAgeUnparseableTimestamp(t *testing.T) {
	srv := npmPublishStub(t, "weird-pkg", "1.0.0", "not-a-timestamp")
	defer srv.Close()
	withBase(t, &npmRegistryBase, srv.URL)

	age, missing, err := FetchPublishAge(context.Background(), srv.Client(), t.TempDir(), "npm", "weird-pkg", "1.0.0", fixedNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !missing || age != 0 {
		t.Errorf("unparseable ts: (age=%d,missing=%v), want (0,true)", age, missing)
	}
}

// TestFetchPublishAgeFutureCacheServed: a fresh cache entry with a future
// PublishedAt is treated as missing on the cache-hit path.
func TestFetchPublishAgeFutureCacheServed(t *testing.T) {
	dir := t.TempDir()
	path := ageCachePath(dir, "npm", "cached-future", "1.0.0")
	entry := ageCacheEntry{
		PublishedAt: fixedNow.Add(10 * time.Hour), // future
		CachedAt:    fixedNow.Add(-1 * time.Hour), // fresh
	}
	data, _ := json.Marshal(entry)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	// Server must not be consulted on a fresh cache hit.
	age, missing, err := FetchPublishAge(context.Background(), &http.Client{}, dir, "npm", "cached-future", "1.0.0", fixedNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !missing || age != 0 {
		t.Errorf("future cache: (age=%d,missing=%v), want (0,true)", age, missing)
	}
}

// --- marketplace.go: VS Code fallback success + future timestamp ---

// TestFetchMarketplaceAgeVSCodeFallback: Open VSX fails, VS Code Marketplace
// supplies the publishedDate → age computed from the fallback source.
func TestFetchMarketplaceAgeVSCodeFallback(t *testing.T) {
	// Open VSX returns an error payload.
	openVSX := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"timestamp":"","error":"not found"}`))
	}))
	defer openVSX.Close()

	published := fixedMarketplaceNow.Add(-3 * time.Hour)
	pubStr := published.Format(time.RFC3339Nano)
	vscode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"extensions":[{"publishedDate":"` + pubStr + `"}]}]}`))
	}))
	defer vscode.Close()

	withBase(t, &openVSXBase, openVSX.URL)
	withBase(t, &vscodeMarketplaceBase, vscode.URL)

	age, missing, err := FetchMarketplaceAge(context.Background(), openVSX.Client(),
		t.TempDir(), "pub", "ext", "1.0.0", fixedMarketplaceNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if missing {
		t.Fatal("missing = true, want false (VS Code fallback supplied a timestamp)")
	}
	// 3h = 180 minutes, allow rounding.
	if age < 179 || age > 181 {
		t.Errorf("age = %d, want ~180 (3h from fallback)", age)
	}
}

// TestFetchOpenVSXTimestampAPIError: an error field in the Open VSX response → error.
func TestFetchOpenVSXTimestampAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"timestamp":"","error":"extension not found"}`))
	}))
	defer srv.Close()
	withBase(t, &openVSXBase, srv.URL)

	if _, err := fetchOpenVSXTimestamp(context.Background(), srv.Client(), "p", "n", "1.0.0"); err == nil {
		t.Error("want error when Open VSX returns an error field, got nil")
	}
}

// TestFetchVSCodeMarketplaceTimestampMissing: empty results → error.
func TestFetchVSCodeMarketplaceTimestampMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()
	withBase(t, &vscodeMarketplaceBase, srv.URL)

	if _, err := fetchVSCodeMarketplaceTimestamp(context.Background(), srv.Client(), "p", "n"); err == nil {
		t.Error("want error for empty results, got nil")
	}
}
