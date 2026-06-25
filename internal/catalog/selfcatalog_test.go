package catalog

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// testSelfCatalogPrivKeyHex is the Ed25519 private key used to sign test
// fixtures. It is INDEPENDENT of the production key embedded in selfkey.go:
// tests verify fixtures against testSelfPubKey (this key's public half), never
// against the embedded production SelfCatalogPublicKey. This key is test-only
// and intentionally committed; its public half MUST NOT equal the embedded
// production key. Before the v1.1.x self-catalog key rotation the two were the
// same value, which published the production signing key in the repo. See
// THREAT-MODEL T-09-12.
const testSelfCatalogPrivKeyHex = "a0bd03e6e4738160557871ad4020418240dd5442227d1a091c8d443b12c2889e5d5d0314010707dbc494defd366900a6dfd5ac1e02d47a64b090c1d344afd2a1"

// testSelfPrivKey decodes the test private key for fixture signing helpers.
func testSelfPrivKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	b, err := hex.DecodeString(testSelfCatalogPrivKeyHex)
	if err != nil {
		t.Fatalf("decode test private key: %v", err)
	}
	return ed25519.PrivateKey(b)
}

// testSelfPubKey returns the public half of the test signing key. Tests verify
// fixtures against THIS key (via PubKeyOverride), keeping the test key fully
// independent of the embedded production SelfCatalogPublicKey.
func testSelfPubKey(t *testing.T) ed25519.PublicKey {
	t.Helper()
	return testSelfPrivKey(t).Public().(ed25519.PublicKey)
}

// signFeedEntries returns a base64-encoded Ed25519 signature over the canonical
// JSON of the given entries slice using the test private key.
func signFeedEntries(t *testing.T, entries []selfCatalogEntry) string {
	t.Helper()
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshal entries for signing: %v", err)
	}
	sig := ed25519.Sign(testSelfPrivKey(t), data)
	return base64.StdEncoding.EncodeToString(sig)
}

// readFixtureSelfCatalog reads a testdata fixture file relative to this package.
func readFixtureSelfCatalog(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %q: %v", name, err)
	}
	return data
}

// serveFeed starts an httptest.Server that serves the given response body on GET.
// The server is closed automatically via t.Cleanup.
func serveFeed(t *testing.T, body []byte, statusCode int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestSelfCatalog_VersionMatch verifies that when the feed lists the running
// version, CheckSelfCatalog returns a quarantine result with the matched entry.
func TestSelfCatalog_VersionMatch(t *testing.T) {
	feedData := readFixtureSelfCatalog(t, "selfcatalog_match.json")
	srv := serveFeed(t, feedData, http.StatusOK)

	opts := SelfCatalogOpts{
		FeedURL:    srv.URL,
		CacheDir:   t.TempDir(),
		Client:     srv.Client(),
		Version:    "test-v0.0.1", // matches the fixture
		StatePath:  filepath.Join(t.TempDir(), "state.json"),
		PubKeyOverride: testSelfPubKey(t),
	}

	result := CheckSelfCatalog(opts)

	if result.Outcome != SelfCatalogQuarantine {
		t.Errorf("Outcome: want SelfCatalogQuarantine, got %v", result.Outcome)
	}
	if result.MatchedEntry == nil {
		t.Fatal("MatchedEntry must not be nil on quarantine")
	}
	if result.MatchedEntry.ID != "beekeeper-self-2026-001" {
		t.Errorf("MatchedEntry.ID: want %q, got %q", "beekeeper-self-2026-001", result.MatchedEntry.ID)
	}
	if result.Err != nil {
		t.Errorf("Err must be nil on quarantine, got %v", result.Err)
	}
}

// TestSelfCatalog_InvalidSignature verifies that a tampered signature returns
// errIntegrity (fail-closed), distinct from a network error (warn-continue).
func TestSelfCatalog_InvalidSignature(t *testing.T) {
	feedData := readFixtureSelfCatalog(t, "selfcatalog_invalid_sig.json")
	srv := serveFeed(t, feedData, http.StatusOK)

	opts := SelfCatalogOpts{
		FeedURL:    srv.URL,
		CacheDir:   t.TempDir(),
		Client:     srv.Client(),
		Version:    "test-v0.0.1",
		StatePath:  filepath.Join(t.TempDir(), "state.json"),
		PubKeyOverride: testSelfPubKey(t),
	}

	result := CheckSelfCatalog(opts)

	if result.Outcome != SelfCatalogFailClosed {
		t.Errorf("Outcome: want SelfCatalogFailClosed for invalid signature, got %v", result.Outcome)
	}
	if !errors.Is(result.Err, errIntegrity) {
		t.Errorf("Err: want errors.Is(err, errIntegrity), got %v", result.Err)
	}
	// Must NOT be a network error — the failure is an integrity failure.
	if errors.Is(result.Err, errNetwork) {
		t.Error("Err must not wrap errNetwork for an integrity failure")
	}
}

// TestSelfCatalog_NetworkError_NoCache verifies that when the fetch fails AND
// there is no cached copy, the result is WARN+CONTINUE (not quarantine, not fail-closed).
// This ensures that a transient network failure does not brick the tool (Pitfall 2).
func TestSelfCatalog_NetworkError_NoCache(t *testing.T) {
	// Use an address that will fail immediately — not listening.
	opts := SelfCatalogOpts{
		FeedURL:    "http://127.0.0.1:1", // nothing listening here
		CacheDir:   t.TempDir(),           // empty cache dir
		Client:     &http.Client{Timeout: 100 * time.Millisecond},
		Version:    "test-v0.0.1",
		StatePath:  filepath.Join(t.TempDir(), "state.json"),
		PubKeyOverride: testSelfPubKey(t),
	}

	result := CheckSelfCatalog(opts)

	if result.Outcome != SelfCatalogWarnContinue {
		t.Errorf("Outcome: want SelfCatalogWarnContinue for network error + no cache, got %v", result.Outcome)
	}
	// Network errors must wrap errNetwork, not errIntegrity.
	if !errors.Is(result.Err, errNetwork) {
		t.Errorf("Err: want errors.Is(err, errNetwork), got %v", result.Err)
	}
	if errors.Is(result.Err, errIntegrity) {
		t.Error("Err must not wrap errIntegrity for a network failure")
	}
}

// TestSelfCatalog_NetworkError_FreshCache verifies that when the fetch fails BUT
// a fresh (<24h) cache exists, the cached feed is used and processing continues.
// If the cached feed matches the running version, the result is still quarantine.
func TestSelfCatalog_NetworkError_FreshCache(t *testing.T) {
	cacheDir := t.TempDir()

	// Write a fresh cache entry that does NOT match the running version.
	noMatchData := readFixtureSelfCatalog(t, "selfcatalog_no_match.json")
	cacheEntry := selfCatalogCacheEntry{
		CachedAt: time.Now().UTC(),
		FeedData: noMatchData,
	}
	cacheData, err := json.Marshal(cacheEntry)
	if err != nil {
		t.Fatalf("marshal cache entry: %v", err)
	}
	cachePath := selfCatalogCachePath(cacheDir)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o700); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	if err := writeFileAtomic(cachePath, cacheData); err != nil {
		t.Fatalf("write cache: %v", err)
	}

	// Network will fail — nothing is listening on port 1.
	opts := SelfCatalogOpts{
		FeedURL:    "http://127.0.0.1:1",
		CacheDir:   cacheDir,
		Client:     &http.Client{Timeout: 100 * time.Millisecond},
		Version:    "different-version", // does not match v99.99.99 in the no_match fixture
		StatePath:  filepath.Join(t.TempDir(), "state.json"),
		PubKeyOverride: testSelfPubKey(t),
	}

	result := CheckSelfCatalog(opts)

	// With a fresh cache and no version match, the outcome must be Continue.
	if result.Outcome != SelfCatalogContinue {
		t.Errorf("Outcome: want SelfCatalogContinue (cache hit, no match), got %v", result.Outcome)
	}
	if result.Err != nil {
		t.Errorf("Err must be nil on continue, got %v", result.Err)
	}
}

// TestSelfCatalog_NoMatch verifies that a valid, signed feed with no matching
// version produces a Continue outcome with no error.
func TestSelfCatalog_NoMatch(t *testing.T) {
	feedData := readFixtureSelfCatalog(t, "selfcatalog_no_match.json")
	srv := serveFeed(t, feedData, http.StatusOK)

	opts := SelfCatalogOpts{
		FeedURL:    srv.URL,
		CacheDir:   t.TempDir(),
		Client:     srv.Client(),
		Version:    "v1.0.0-unaffected", // not in any entry
		StatePath:  filepath.Join(t.TempDir(), "state.json"),
		PubKeyOverride: testSelfPubKey(t),
	}

	result := CheckSelfCatalog(opts)

	if result.Outcome != SelfCatalogContinue {
		t.Errorf("Outcome: want SelfCatalogContinue for no match, got %v", result.Outcome)
	}
	if result.MatchedEntry != nil {
		t.Errorf("MatchedEntry must be nil on no match, got %+v", result.MatchedEntry)
	}
	if result.Err != nil {
		t.Errorf("Err must be nil on continue, got %v", result.Err)
	}
}

// TestSelfCatalog_OfflinePersistence verifies that when state.json already has
// a SelfQuarantine for the running version, CheckSelfCatalog returns quarantine
// immediately without performing any network fetch.
func TestSelfCatalog_OfflinePersistence(t *testing.T) {
	stateDir := t.TempDir()
	statePath := filepath.Join(stateDir, "state.json")

	// Pre-populate state.json with a quarantine for the running version.
	st := WatchState{
		Sources: make(map[string]SourceState),
		SelfQuarantine: &SelfQuarantineState{
			Version: "v0.4.2",
			EntryID: "beekeeper-self-2026-001",
			Reason:  "Beekeeper v0.4.2 release pipeline compromise",
			FiredAt: time.Now().UTC().Format(time.RFC3339),
		},
	}
	if err := SaveState(statePath, st); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	// Server should never be called — quarantine comes from state.json.
	serverCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	opts := SelfCatalogOpts{
		FeedURL:    srv.URL,
		CacheDir:   t.TempDir(),
		Client:     srv.Client(),
		Version:    "v0.4.2", // matches the persisted quarantine
		StatePath:  statePath,
		PubKeyOverride: testSelfPubKey(t),
	}

	result := CheckSelfCatalog(opts)

	if serverCalled {
		t.Error("server must NOT be called when state.json has a quarantine for the running version")
	}
	if result.Outcome != SelfCatalogQuarantine {
		t.Errorf("Outcome: want SelfCatalogQuarantine from offline state, got %v", result.Outcome)
	}
}

// TestSelfCatalog_CustomKeyVerifiesAndEmbeddedFails verifies CR-01:
// a feed signed with a CUSTOM key verifies when PubKeyOverride is set to that
// custom key, but fails when verified against the embedded key. This proves
// that PubKeyOverride is plumbed correctly into parseAndVerifySelfFeed.
func TestSelfCatalog_CustomKeyVerifiesAndEmbeddedFails(t *testing.T) {
	// Generate a fresh custom key pair — completely independent from the
	// embedded SelfCatalogPublicKey.
	customPub, customPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate custom key: %v", err)
	}

	// Build a minimal self-feed signed with the custom key.
	entries := []selfCatalogEntry{
		{
			ID:            "custom-key-test-001",
			Name:          "Custom key test entry",
			Ecosystem:     "beekeeper",
			Package:       "beekeeper",
			Versions:      []string{"v0.99.0"},
			Severity:      "critical",
			CatalogSource: "beekeeper-self",
		},
	}
	entriesJSON, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshal entries: %v", err)
	}
	sig := ed25519.Sign(customPriv, entriesJSON)
	feed := selfFeed{
		SchemaVersion:    "1",
		Entries:          entries,
		CatalogSignature: base64.StdEncoding.EncodeToString(sig),
	}
	feedData, err := json.Marshal(feed)
	if err != nil {
		t.Fatalf("marshal feed: %v", err)
	}

	// --- Part A: verify WITH the custom key → should continue or quarantine ---
	srvA := serveFeed(t, feedData, http.StatusOK)
	optsA := SelfCatalogOpts{
		FeedURL:        srvA.URL,
		CacheDir:       t.TempDir(),
		Client:         srvA.Client(),
		Version:        "v1.0.0-unaffected", // not in the feed
		StatePath:      filepath.Join(t.TempDir(), "state.json"),
		PubKeyOverride: customPub, // use the custom key
	}
	resultA := CheckSelfCatalog(optsA)
	if resultA.Outcome == SelfCatalogFailClosed {
		t.Errorf("Part A (custom key): Outcome = SelfCatalogFailClosed, want Continue or Quarantine; err: %v", resultA.Err)
	}
	if resultA.Outcome != SelfCatalogContinue {
		t.Errorf("Part A (custom key): Outcome = %v, want SelfCatalogContinue (version not in feed)", resultA.Outcome)
	}

	// --- Part B: verify AGAINST the embedded key → must fail closed (integrity) ---
	srvB := serveFeed(t, feedData, http.StatusOK)
	optsB := SelfCatalogOpts{
		FeedURL:        srvB.URL,
		CacheDir:       t.TempDir(),
		Client:         srvB.Client(),
		Version:        "v1.0.0-unaffected",
		StatePath:      filepath.Join(t.TempDir(), "state.json"),
		PubKeyOverride: SelfCatalogPublicKey, // wrong key for this feed
	}
	resultB := CheckSelfCatalog(optsB)
	if resultB.Outcome != SelfCatalogFailClosed {
		t.Errorf("Part B (embedded key vs custom-signed feed): Outcome = %v, want SelfCatalogFailClosed", resultB.Outcome)
	}
	if !errors.Is(resultB.Err, errIntegrity) {
		t.Errorf("Part B: Err: want errors.Is(err, errIntegrity), got %v", resultB.Err)
	}
}

// TestSelfCatalogAdapter_PollenEntries verifies that the selfCatalogAdapter
// correctly discriminates pollen entries from beekeeper entries using the
// package field. This covers SDEF-01: a known-bad Pollen release must be
// detectable via the unified beekeeper-self catalog.
//
// The test uses a non-production version string ("pollen-test-v0.0.1") so the
// entry can never match a real shipped pollen version (v0.1.1-pollen.N) and
// trigger a false self-quarantine on production installations (T-05-06).
func TestSelfCatalogAdapter_PollenEntries(t *testing.T) {
	pollenEntry := selfCatalogEntry{
		ID:            "pollen-self-2026-001",
		Name:          "pollen (hypothetical compromised pollen release)",
		Ecosystem:     "beekeeper",
		Package:       "pollen",
		Versions:      []string{"pollen-test-v0.0.1"},
		Severity:      "critical",
		CatalogSource: "beekeeper-self",
	}
	adapter := &selfCatalogAdapter{entries: []selfCatalogEntry{pollenEntry}}

	// Test 1: LookupAll("beekeeper","pollen") returns exactly one match.
	matches := adapter.LookupAll("beekeeper", "pollen")
	if len(matches) != 1 {
		t.Fatalf("LookupAll(beekeeper,pollen): expected 1 match, got %d", len(matches))
	}

	// Test 2: the returned match has the correct fields.
	m := matches[0]
	if m.CatalogSource != "beekeeper-self" {
		t.Errorf("CatalogSource: want %q, got %q", "beekeeper-self", m.CatalogSource)
	}
	if m.Package != "pollen" {
		t.Errorf("Package: want %q, got %q", "pollen", m.Package)
	}
	if !m.Signed {
		t.Error("Signed must be true — beekeeper-self feed is always signature-verified")
	}
	if m.Severity != "critical" {
		t.Errorf("Severity: want %q, got %q", "critical", m.Severity)
	}
	if m.Version != "pollen-test-v0.0.1" {
		t.Errorf("Version: want %q, got %q", "pollen-test-v0.0.1", m.Version)
	}
	if m.EntryID != "pollen-self-2026-001" {
		t.Errorf("EntryID: want %q, got %q", "pollen-self-2026-001", m.EntryID)
	}

	// Test 3: LookupAll("beekeeper","beekeeper") on a pollen-only adapter returns nil
	// — the package discriminator must exclude pollen entries from beekeeper lookups.
	beekeeperMatches := adapter.LookupAll("beekeeper", "beekeeper")
	if beekeeperMatches != nil {
		t.Errorf("LookupAll(beekeeper,beekeeper) on pollen-only adapter: expected nil, got %v", beekeeperMatches)
	}

	// Test 4: LookupAll("npm","pollen") returns nil — non-beekeeper ecosystem.
	npmMatches := adapter.LookupAll("npm", "pollen")
	if npmMatches != nil {
		t.Errorf("LookupAll(npm,pollen): expected nil for non-beekeeper ecosystem, got %v", npmMatches)
	}
}

// TestSelfCatalogAdapter_LookupAll verifies that the selfCatalogAdapter satisfies
// policy.MultiCatalogLookup and returns CatalogMatch records for the "beekeeper"
// ecosystem when entries exist.
func TestSelfCatalogAdapter_LookupAll(t *testing.T) {
	entries := []selfCatalogEntry{
		{
			ID:            "beekeeper-self-2026-001",
			Name:          "Test entry",
			Ecosystem:     "beekeeper",
			Package:       "beekeeper",
			Versions:      []string{"v0.4.2"},
			Severity:      "critical",
			CatalogSource: "beekeeper-self",
		},
	}

	adapter := &selfCatalogAdapter{entries: entries}

	// Should return matches for the "beekeeper" ecosystem.
	matches := adapter.LookupAll("beekeeper", "beekeeper")
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	m := matches[0]
	if m.CatalogSource != "beekeeper-self" {
		t.Errorf("CatalogSource: want %q, got %q", "beekeeper-self", m.CatalogSource)
	}
	if m.EntryID != "beekeeper-self-2026-001" {
		t.Errorf("EntryID: want %q, got %q", "beekeeper-self-2026-001", m.EntryID)
	}
	if !m.Signed {
		t.Error("Signed must be true — beekeeper-self feed is always signature-verified")
	}
	if m.Severity != "critical" {
		t.Errorf("Severity: want %q, got %q", "critical", m.Severity)
	}

	// Should return nil for a different ecosystem.
	noMatches := adapter.LookupAll("npm", "beekeeper")
	if noMatches != nil {
		t.Errorf("LookupAll non-beekeeper ecosystem: expected nil, got %v", noMatches)
	}
}
