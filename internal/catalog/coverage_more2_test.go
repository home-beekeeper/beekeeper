package catalog

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- sync.go: public Sync wrapper + non-200 list + parse error ---

// TestSyncUnconditional200 drives the backward-compatible Sync entry point through
// a 200 list + raw fetch + index build, returning the entry count.
func TestSyncUnconditional200(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")

	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/raw/test.json" {
			_, _ = w.Write([]byte(validCatalogBody))
			return
		}
		w.Header().Set("ETag", `"e1"`)
		_, _ = w.Write([]byte(`[{"name":"test.json","type":"file","download_url":"` + srvURL + `/raw/test.json"}]`))
	}))
	defer srv.Close()
	srvURL = srv.URL
	withContentsURL(t, srv.URL)

	count, err := Sync(context.Background(), srv.Client(), t.TempDir())
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if count != 1 {
		t.Errorf("Sync count = %d, want 1", count)
	}
}

// TestSyncConditionalListNon200: a non-200, non-304 list response → error, no index.
func TestSyncConditionalListNon200(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	withContentsURL(t, srv.URL)

	dir := t.TempDir()
	if _, err := SyncConditional(context.Background(), srv.Client(), dir, ""); err == nil {
		t.Fatal("want error on 403 list, got nil")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "bumblebee.idx")); statErr == nil {
		t.Error("index written on a failed list, want none")
	}
}

// TestSyncConditionalListBadJSON: a 200 list with malformed JSON → decode error.
func TestSyncConditionalListBadJSON(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{not an array`))
	}))
	defer srv.Close()
	withContentsURL(t, srv.URL)

	if _, err := SyncConditional(context.Background(), srv.Client(), t.TempDir(), ""); err == nil {
		t.Fatal("want decode error on malformed list, got nil")
	}
}

// TestSyncConditionalRawFetch404: a list pointing at a 404 raw file → fetch error.
func TestSyncConditionalRawFetch404(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/raw/missing.json" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`[{"name":"missing.json","type":"file","download_url":"` + srvURL + `/raw/missing.json"}]`))
	}))
	defer srv.Close()
	srvURL = srv.URL
	withContentsURL(t, srv.URL)

	if _, err := SyncConditional(context.Background(), srv.Client(), t.TempDir(), ""); err == nil {
		t.Fatal("want fetch error on raw 404, got nil")
	}
}

// TestSyncConditionalSkipsNonJSONAndDirs: non-file types and non-.json names are
// skipped; only the .json file is fetched and counted.
func TestSyncConditionalSkipsNonJSONAndDirs(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	var srvURL string
	var rawHits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/raw/test.json" {
			rawHits++
			_, _ = w.Write([]byte(validCatalogBody))
			return
		}
		_, _ = w.Write([]byte(`[
			{"name":"README.md","type":"file","download_url":"` + srvURL + `/raw/README.md"},
			{"name":"subdir","type":"dir","download_url":""},
			{"name":"empty.json","type":"file","download_url":""},
			{"name":"test.json","type":"file","download_url":"` + srvURL + `/raw/test.json"}
		]`))
	}))
	defer srv.Close()
	srvURL = srv.URL
	withContentsURL(t, srv.URL)

	res, err := SyncConditional(context.Background(), srv.Client(), t.TempDir(), "")
	if err != nil {
		t.Fatalf("SyncConditional: %v", err)
	}
	if res.Count != 1 {
		t.Errorf("Count = %d, want 1 (only test.json counted)", res.Count)
	}
	if rawHits != 1 {
		t.Errorf("rawHits = %d, want 1 (md/dir/empty-url skipped)", rawHits)
	}
}

// TestFetchStripsAuthOnCrossHostRedirect proves the redirect handler removes the
// Authorization header when redirected to a different host (WR-01).
func TestFetchStripsAuthOnCrossHostRedirect(t *testing.T) {
	// attacker host records whether it received an Authorization header.
	var gotAuth string
	attacker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte("payload"))
	}))
	defer attacker.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, attacker.URL+"/leak", http.StatusFound)
	}))
	defer origin.Close()

	body, err := fetch(context.Background(), origin.Client(), origin.URL+"/file", "secret-token")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if string(body) != "payload" {
		t.Errorf("body = %q, want payload", body)
	}
	if gotAuth != "" {
		t.Errorf("Authorization header leaked to attacker host: %q", gotAuth)
	}
}

// TestFetchNon200 surfaces an error for a non-200 raw fetch.
func TestFetchNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	if _, err := fetch(context.Background(), srv.Client(), srv.URL, ""); err == nil {
		t.Error("want error on 502 raw fetch, got nil")
	}
}

// --- selfcatalog.go: parseAndVerifySelfFeed error branches ---

// TestParseAndVerifySelfFeedNoSignature: a feed with an empty catalog_signature → error.
func TestParseAndVerifySelfFeedNoSignature(t *testing.T) {
	data := []byte(`{"schema_version":"1","entries":[],"catalog_signature":""}`)
	if _, err := parseAndVerifySelfFeed(data, SelfCatalogPublicKey); err == nil {
		t.Error("want error for missing signature, got nil")
	}
}

// TestParseAndVerifySelfFeedBadBase64: a non-base64 signature → error.
func TestParseAndVerifySelfFeedBadBase64(t *testing.T) {
	data := []byte(`{"schema_version":"1","entries":[],"catalog_signature":"!!!not base64!!!"}`)
	if _, err := parseAndVerifySelfFeed(data, SelfCatalogPublicKey); err == nil {
		t.Error("want error for malformed base64 signature, got nil")
	}
}

// TestParseAndVerifySelfFeedBadJSON: unparseable feed JSON → error.
func TestParseAndVerifySelfFeedBadJSON(t *testing.T) {
	if _, err := parseAndVerifySelfFeed([]byte(`{not json`), SelfCatalogPublicKey); err == nil {
		t.Error("want error for malformed feed JSON, got nil")
	}
}

// TestParseAndVerifySelfFeedWrongKey: a signature made with a different key fails
// verification against the embedded public key (fail-closed).
func TestParseAndVerifySelfFeedWrongKey(t *testing.T) {
	_, otherPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	entries := []selfCatalogEntry{{ID: "x", Versions: []string{"1.0.0"}}}
	entriesJSON, _ := json.Marshal(entries)
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(otherPriv, entriesJSON))
	feed := selfFeed{SchemaVersion: "1", Entries: entries, CatalogSignature: sig}
	data, _ := json.Marshal(feed)

	if _, err := parseAndVerifySelfFeed(data, SelfCatalogPublicKey); err == nil {
		t.Error("want verification failure for wrong-key signature, got nil")
	}
}

// TestParseAndVerifySelfFeedValid: a feed signed with the matching test key verifies.
func TestParseAndVerifySelfFeedValid(t *testing.T) {
	entries := []selfCatalogEntry{{ID: "ok", Versions: []string{"1.0.0"}}}
	sig := signFeedEntries(t, entries)
	feed := selfFeed{SchemaVersion: "1", Entries: entries, CatalogSignature: sig}
	data, _ := json.Marshal(feed)

	got, err := parseAndVerifySelfFeed(data, SelfCatalogPublicKey)
	if err != nil {
		t.Fatalf("valid feed failed to verify: %v", err)
	}
	if len(got.Entries) != 1 || got.Entries[0].ID != "ok" {
		t.Errorf("parsed entries = %+v, want one entry id=ok", got.Entries)
	}
}

// TestNormalizeSelfVersion covers the trim/strip-v/strip-build-metadata/lowercase rules.
func TestNormalizeSelfVersion(t *testing.T) {
	cases := map[string]string{
		"  v1.2.3  ":     "1.2.3",
		"V1.2.3+build99": "1.2.3",
		"1.2.3-rc1":      "1.2.3-rc1", // pre-release retained
		"":               "",
	}
	for in, want := range cases {
		if got := normalizeSelfVersion(in); got != want {
			t.Errorf("normalizeSelfVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

// --- marketplace.go: cache-hit, unparseable, future-cache paths ---

// TestFetchMarketplaceAgeCacheHitFresh: a pre-written fresh entry is served without
// any network access.
func TestFetchMarketplaceAgeCacheHitFresh(t *testing.T) {
	dir := t.TempDir()
	path := marketplaceCachePath(dir, "pub", "ext", "1.0.0")
	entry := ageCacheEntry{
		PublishedAt: fixedMarketplaceNow.Add(-90 * time.Minute),
		CachedAt:    fixedMarketplaceNow.Add(-30 * time.Minute),
	}
	data, _ := json.Marshal(entry)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	age, missing, err := FetchMarketplaceAge(context.Background(), &http.Client{}, dir, "pub", "ext", "1.0.0", fixedMarketplaceNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if missing {
		t.Fatal("missing = true, want false (fresh cache)")
	}
	if age < 89 || age > 91 {
		t.Errorf("age = %d, want ~90", age)
	}
}

// TestFetchMarketplaceAgeMissingCacheServed: a fresh Missing:true cache entry is honored.
func TestFetchMarketplaceAgeMissingCacheServed(t *testing.T) {
	dir := t.TempDir()
	path := marketplaceCachePath(dir, "pub", "ext", "2.0.0")
	entry := ageCacheEntry{CachedAt: fixedMarketplaceNow.Add(-1 * time.Hour), Missing: true}
	data, _ := json.Marshal(entry)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	age, missing, err := FetchMarketplaceAge(context.Background(), &http.Client{}, dir, "pub", "ext", "2.0.0", fixedMarketplaceNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !missing || age != 0 {
		t.Errorf("(age=%d,missing=%v), want (0,true)", age, missing)
	}
}

// TestFetchMarketplaceAgeUnparseable: Open VSX returns a non-RFC3339 timestamp →
// missing (fail-closed).
func TestFetchMarketplaceAgeUnparseable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"timestamp":"yesterday","error":""}`))
	}))
	defer srv.Close()
	withBase(t, &openVSXBase, srv.URL)
	// Point VS Code at the same server so the fallback also returns a non-timestamp.
	withBase(t, &vscodeMarketplaceBase, srv.URL)

	age, missing, err := FetchMarketplaceAge(context.Background(), srv.Client(), t.TempDir(), "pub", "ext", "1.0.0", fixedMarketplaceNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !missing || age != 0 {
		t.Errorf("(age=%d,missing=%v), want (0,true) for unparseable timestamp", age, missing)
	}
}

// TestFetchMarketplaceAgeFutureTimestamp: Open VSX timestamp in the future → missing.
func TestFetchMarketplaceAgeFutureTimestamp(t *testing.T) {
	future := fixedMarketplaceNow.Add(72 * time.Hour)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"timestamp":"` + future.Format(time.RFC3339Nano) + `","error":""}`))
	}))
	defer srv.Close()
	withBase(t, &openVSXBase, srv.URL)

	age, missing, err := FetchMarketplaceAge(context.Background(), srv.Client(), t.TempDir(), "pub", "ext", "1.0.0", fixedMarketplaceNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !missing || age != 0 {
		t.Errorf("(age=%d,missing=%v), want (0,true) for future timestamp", age, missing)
	}
}

// --- age_cache.go: readAgeCacheEntry corrupt + writeAgeCacheEntry round-trip ---

// TestReadAgeCacheEntryCorrupt: a corrupt cache file is a miss.
func TestReadAgeCacheEntryCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := ageCachePath(dir, "npm", "p", "1.0.0")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{bad`), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, ok := readAgeCacheEntry(path); ok {
		t.Error("readAgeCacheEntry(corrupt) ok=true, want false")
	}
}

// TestWriteAgeCacheEntryCreatesDirs: writeAgeCacheEntry creates nested dirs and a
// readable round-trip entry.
func TestWriteAgeCacheEntryCreatesDirs(t *testing.T) {
	dir := t.TempDir()
	path := ageCachePath(dir, "npm", "deep", "9.9.9")
	want := ageCacheEntry{PublishedAt: fixedNow, CachedAt: fixedNow}
	if err := writeAgeCacheEntry(path, want); err != nil {
		t.Fatalf("writeAgeCacheEntry: %v", err)
	}
	got, ok := readAgeCacheEntry(path)
	if !ok {
		t.Fatal("readAgeCacheEntry after write: ok=false")
	}
	if !got.PublishedAt.Equal(want.PublishedAt) {
		t.Errorf("PublishedAt = %v, want %v", got.PublishedAt, want.PublishedAt)
	}
}
