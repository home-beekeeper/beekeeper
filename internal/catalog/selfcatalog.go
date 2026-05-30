package catalog

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mzansi-agentive/beekeeper/internal/policy"
	"github.com/mzansi-agentive/beekeeper/internal/version"
)

// errIntegrity is the sentinel error returned when the beekeeper-self feed
// signature verification fails. An integrity failure is distinct from a network
// failure: an invalid signature is a proven integrity violation (T-09-10) and
// must cause CheckSelfCatalog to return SelfCatalogFailClosed, never to degrade
// gracefully. Use errors.Is(err, errIntegrity) to test for this condition.
var errIntegrity = errors.New("beekeeper-self: feed signature invalid (integrity failure)")

// errNetwork is the sentinel error returned when the beekeeper-self feed cannot
// be fetched due to a transient network condition (DNS failure, timeout, HTTP
// error). A network failure is NOT an integrity failure: a flaky connection or
// cold-start with no cache must NOT brick the tool (Pitfall 2, T-09-11).
// Use errors.Is(err, errNetwork) to test for this condition.
var errNetwork = errors.New("beekeeper-self: feed fetch failed (network error)")

// selfCatalogCacheTTL is the maximum age of a cached beekeeper-self feed before
// it is considered stale. Matches the OSV cache TTL convention (24h).
const selfCatalogCacheTTL = 24 * time.Hour

// selfCatalogCacheFile is the filename of the cached feed under the cache dir.
const selfCatalogCacheFile = "beekeeper-self.json"

// selfCatalogCachePath returns the absolute path to the cache file.
func selfCatalogCachePath(cacheDir string) string {
	return filepath.Join(cacheDir, "beekeeper-self", selfCatalogCacheFile)
}

// selfCatalogEntry is one entry in the beekeeper-self feed.
type selfCatalogEntry struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Ecosystem     string   `json:"ecosystem"`
	Package       string   `json:"package"`
	Versions      []string `json:"versions"`
	Severity      string   `json:"severity"`
	CatalogSource string   `json:"catalog_source"`
}

// selfFeed is the top-level structure of the beekeeper-self JSON feed.
//
// The feed schema follows the beekeeper catalog convention:
//   - schema_version: must be "1"
//   - entries: the list of known-compromised beekeeper versions
//   - catalog_signature: base64-encoded Ed25519 signature over the canonical
//     JSON of the entries array (not the full feed, which would be circular)
type selfFeed struct {
	SchemaVersion    string             `json:"schema_version"`
	Entries          []selfCatalogEntry `json:"entries"`
	CatalogSignature string             `json:"catalog_signature"`
}

// selfCatalogCacheEntry is the on-disk cache format. CachedAt records when the
// entry was written; FeedData holds the raw signed JSON bytes (not parsed) so
// that signature verification can be re-run on the cached data on next load.
type selfCatalogCacheEntry struct {
	CachedAt time.Time `json:"cached_at"`
	FeedData []byte    `json:"feed_data"`
}

// SelfCatalogOutcome describes the result classification of CheckSelfCatalog.
// Callers must branch on the Outcome rather than inspecting Err alone.
type SelfCatalogOutcome int

const (
	// SelfCatalogContinue means the feed was checked, the signature is valid,
	// and the running version is NOT in any compromised-version list.
	// Normal operation; continue.
	SelfCatalogContinue SelfCatalogOutcome = iota

	// SelfCatalogQuarantine means the running version IS in the feed's
	// compromised-version list. The caller MUST self-quarantine and refuse to
	// continue with enforcement operations.
	SelfCatalogQuarantine

	// SelfCatalogFailClosed means the feed was fetched but its signature is
	// INVALID (T-09-10). This is a proven integrity failure — an attacker may
	// have tampered with the feed. The caller must fail closed.
	SelfCatalogFailClosed

	// SelfCatalogWarnContinue means the feed could not be fetched (network
	// error) AND there is no usable cache. The tool must emit a prominent
	// warning and continue, rather than bricking (T-09-11, Pitfall 2).
	SelfCatalogWarnContinue
)

// String returns a human-readable name for the outcome constant.
func (o SelfCatalogOutcome) String() string {
	switch o {
	case SelfCatalogContinue:
		return "continue"
	case SelfCatalogQuarantine:
		return "quarantine"
	case SelfCatalogFailClosed:
		return "fail-closed"
	case SelfCatalogWarnContinue:
		return "warn-continue"
	default:
		return fmt.Sprintf("SelfCatalogOutcome(%d)", int(o))
	}
}

// SelfCatalogResult is the return value of CheckSelfCatalog.
type SelfCatalogResult struct {
	// Outcome is the primary classification of the check result.
	Outcome SelfCatalogOutcome

	// MatchedEntry is set when Outcome == SelfCatalogQuarantine. It is the
	// feed entry whose versions list contains the running version.
	MatchedEntry *selfCatalogEntry

	// Err carries the underlying error for Outcome values that are not
	// SelfCatalogContinue. For SelfCatalogQuarantine it is nil (the match
	// itself is the notable event, not an error). For SelfCatalogFailClosed
	// it wraps errIntegrity. For SelfCatalogWarnContinue it wraps errNetwork.
	Err error
}

// SelfCatalogOpts carries the configuration for CheckSelfCatalog. All fields
// are injectable for testability; zero values use safe production defaults.
type SelfCatalogOpts struct {
	// FeedURL is the HTTPS URL of the beekeeper-self feed.
	// Zero value defaults to the official endpoint.
	FeedURL string

	// CacheDir is the directory where the beekeeper-self cache file is stored.
	// Typically ~/.beekeeper/catalogs. Must be writable.
	CacheDir string

	// Client is the HTTP client used for the feed fetch. If nil, a default
	// client with a 10-second timeout is used.
	Client *http.Client

	// Version is the running beekeeper version to check against the feed.
	// Zero value uses version.Version (set via -ldflags at release time).
	Version string

	// StatePath is the path to ~/.beekeeper/state.json. Used to read and
	// write the SelfQuarantineState for offline persistence.
	StatePath string

	// PubKeyOverride allows callers to substitute a different Ed25519 public key
	// for feed signature verification. When non-nil, it takes precedence over
	// the embedded SelfCatalogPublicKey. This is used by enforceSelfQuarantine
	// when the operator has configured a self-hosted feed key in config.json
	// (SelfCatalogConfig.PubKey), and by tests that need to sign feeds with
	// an independent key.
	//
	// SECURITY: a misconfigured key (present but wrong length / undecodable)
	// must fail closed — see enforceSelfQuarantine in cmd/beekeeper/selfquarantine.go.
	// An empty/nil value falls back to the embedded SelfCatalogPublicKey.
	PubKeyOverride ed25519.PublicKey
}

// selfCatalogDefaultFeedURL is the official beekeeper-self feed endpoint.
// Configurable via config.json self_catalog.url field.
const selfCatalogDefaultFeedURL = "https://beekeeper-self.mzansi-agentive.io/beekeeper-self.json"

// CheckSelfCatalog fetches, verifies, and evaluates the beekeeper-self feed
// against the running binary version. The behaviour table:
//
//   - Offline state has quarantine for running version → return Quarantine immediately (no fetch)
//   - Fetch succeeds, signature valid, version matches → Quarantine + persist state
//   - Fetch succeeds, signature valid, no version match → Continue
//   - Fetch succeeds, signature INVALID → FailClosed (errIntegrity, NOT errNetwork)
//   - Fetch fails, cache fresh (<24h) → use cache, evaluate, return Continue or Quarantine
//   - Fetch fails, cache absent/stale → WarnContinue (errNetwork)
//
// The fail-closed-vs-network distinction is critical:
//   - errIntegrity → proven compromise signal → fail closed
//   - errNetwork → transient uncertainty → warn and continue
func CheckSelfCatalog(opts SelfCatalogOpts) SelfCatalogResult {
	// Resolve defaults.
	runningVersion := opts.Version
	if runningVersion == "" {
		runningVersion = version.Version
	}

	feedURL := opts.FeedURL
	if feedURL == "" {
		feedURL = selfCatalogDefaultFeedURL
	}

	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	pubKey := opts.PubKeyOverride
	if pubKey == nil {
		pubKey = SelfCatalogPublicKey
	}

	// Step 1: Check offline state before any network fetch.
	// If a quarantine was previously persisted for the running version,
	// honor it immediately without a network call.
	if opts.StatePath != "" {
		st, err := LoadState(opts.StatePath)
		if err == nil && st.SelfQuarantine != nil && st.SelfQuarantine.Version == runningVersion {
			// Previous quarantine decision stands offline.
			entry := &selfCatalogEntry{
				ID:       st.SelfQuarantine.EntryID,
				Name:     st.SelfQuarantine.Reason,
				Versions: []string{runningVersion},
			}
			return SelfCatalogResult{
				Outcome:      SelfCatalogQuarantine,
				MatchedEntry: entry,
			}
		}
	}

	// Step 2: Fetch the feed.
	rawFeed, fetchErr := fetchSelfFeed(client, feedURL)
	if fetchErr != nil {
		// Fetch failed — try the cache.
		cachedFeed, cacheAge, cacheErr := readSelfCache(opts.CacheDir)
		if cacheErr == nil && cachedFeed != nil && cacheAge < selfCatalogCacheTTL {
			// Fresh cache available — verify and evaluate.
			feed, err := parseAndVerifySelfFeed(cachedFeed, pubKey)
			if err != nil {
				// Cached feed has bad signature — integrity failure even from cache.
				return SelfCatalogResult{
					Outcome: SelfCatalogFailClosed,
					Err:     fmt.Errorf("%w: %w", errIntegrity, err),
				}
			}
			return evaluateSelfFeed(feed, runningVersion, opts.StatePath)
		}
		// No usable cache — warn and continue (Pitfall 2: don't brick on first run).
		return SelfCatalogResult{
			Outcome: SelfCatalogWarnContinue,
			Err:     fmt.Errorf("%w: %w", errNetwork, fetchErr),
		}
	}

	// Step 3: Verify the signature.
	feed, err := parseAndVerifySelfFeed(rawFeed, pubKey)
	if err != nil {
		// Invalid signature on a freshly fetched feed — proven integrity failure.
		return SelfCatalogResult{
			Outcome: SelfCatalogFailClosed,
			Err:     fmt.Errorf("%w: %w", errIntegrity, err),
		}
	}

	// Step 4: Cache the verified raw feed bytes atomically.
	_ = writeSelfCache(opts.CacheDir, rawFeed) // cache write failure is non-fatal

	// Step 5: Check if running version matches any entry.
	return evaluateSelfFeed(feed, runningVersion, opts.StatePath)
}

// fetchSelfFeed performs an HTTP GET to the given URL and returns the raw
// response body. Returns an error wrapping the transport/HTTP error on failure.
func fetchSelfFeed(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url) //nolint:noctx // self-catalog uses client-level timeout
	if err != nil {
		return nil, fmt.Errorf("get %q: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %q returned HTTP %d", url, resp.StatusCode)
	}

	// Limit body to 1MB — a legitimate self-catalog feed is tiny.
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return data, nil
}

// parseAndVerifySelfFeed parses raw JSON feed bytes, verifies the Ed25519
// signature of the entries array against pubKey, and returns the parsed feed.
// Returns an error (suitable for wrapping with errIntegrity) if the signature
// is absent, malformed, or invalid.
func parseAndVerifySelfFeed(data []byte, pubKey ed25519.PublicKey) (selfFeed, error) {
	var feed selfFeed
	if err := json.Unmarshal(data, &feed); err != nil {
		return selfFeed{}, fmt.Errorf("parse feed JSON: %w", err)
	}

	if feed.CatalogSignature == "" {
		return selfFeed{}, errors.New("feed has no catalog_signature")
	}

	sigBytes, err := base64.StdEncoding.DecodeString(feed.CatalogSignature)
	if err != nil {
		return selfFeed{}, fmt.Errorf("decode catalog_signature: %w", err)
	}

	// Verify over the canonical JSON of the entries array — same representation
	// used when signing (see signFeedEntries in test helper).
	entriesJSON, err := json.Marshal(feed.Entries)
	if err != nil {
		return selfFeed{}, fmt.Errorf("re-marshal entries for verification: %w", err)
	}

	if !ed25519.Verify(pubKey, entriesJSON, sigBytes) {
		return selfFeed{}, errors.New("catalog_signature does not match entries")
	}

	return feed, nil
}

// evaluateSelfFeed checks whether runningVersion appears in any entry's
// versions list, writes a SelfQuarantineState to statePath on a match, and
// returns the appropriate SelfCatalogResult.
func evaluateSelfFeed(feed selfFeed, runningVersion, statePath string) SelfCatalogResult {
	for i := range feed.Entries {
		entry := &feed.Entries[i]
		for _, v := range entry.Versions {
			if v == runningVersion {
				// Version match — self-quarantine.
				if statePath != "" {
					persistSelfQuarantine(statePath, runningVersion, entry)
				}
				return SelfCatalogResult{
					Outcome:      SelfCatalogQuarantine,
					MatchedEntry: entry,
				}
			}
		}
	}
	return SelfCatalogResult{Outcome: SelfCatalogContinue}
}

// persistSelfQuarantine writes a SelfQuarantineState to state.json atomically.
// Write failures are silently ignored — the quarantine is still returned to the
// caller; the failure only affects offline persistence on the next run.
func persistSelfQuarantine(statePath, runningVersion string, entry *selfCatalogEntry) {
	st, err := LoadState(statePath)
	if err != nil {
		st = WatchState{Sources: make(map[string]SourceState)}
	}
	st.SelfQuarantine = &SelfQuarantineState{
		Version: runningVersion,
		EntryID: entry.ID,
		Reason:  entry.Name,
		FiredAt: time.Now().UTC().Format(time.RFC3339),
	}
	_ = SaveState(statePath, st) // failure is non-fatal for current quarantine
}

// readSelfCache reads the on-disk cache file. Returns (data, age, nil) on hit,
// (nil, 0, nil) on cache miss (file absent or corrupt), and (nil, 0, err) only
// on unexpected read errors.
func readSelfCache(cacheDir string) ([]byte, time.Duration, error) {
	if cacheDir == "" {
		return nil, 0, nil
	}
	path := selfCatalogCachePath(cacheDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, 0, nil // cache miss = not an error
		}
		return nil, 0, fmt.Errorf("read self cache %q: %w", path, err)
	}

	var ce selfCatalogCacheEntry
	if err := json.Unmarshal(data, &ce); err != nil {
		// Corrupt cache — treat as miss.
		return nil, 0, nil
	}

	age := time.Since(ce.CachedAt)
	return ce.FeedData, age, nil
}

// writeSelfCache writes rawFeedData to the on-disk cache atomically.
func writeSelfCache(cacheDir string, rawFeedData []byte) error {
	if cacheDir == "" {
		return nil
	}
	path := selfCatalogCachePath(cacheDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("mkdir self cache dir: %w", err)
	}
	ce := selfCatalogCacheEntry{
		CachedAt: time.Now().UTC(),
		FeedData: rawFeedData,
	}
	data, err := json.Marshal(ce)
	if err != nil {
		return fmt.Errorf("marshal self cache entry: %w", err)
	}
	return writeFileAtomic(path, data)
}

// selfCatalogAdapter implements policy.MultiCatalogLookup for the beekeeper-self
// feed. It exposes the feed entries as CatalogMatch records so the beekeeper-self
// source participates in MultiIndex.LookupAll for the "beekeeper" ecosystem.
//
// The adapter is constructed after a successful CheckSelfCatalog call; callers
// should pass it to MultiIndex only when CheckSelfCatalog returns SelfCatalogContinue.
type selfCatalogAdapter struct {
	entries []selfCatalogEntry
}

// LookupAll implements policy.MultiCatalogLookup. It returns CatalogMatch records
// for the "beekeeper" ecosystem only. Calls for any other ecosystem return nil.
func (a *selfCatalogAdapter) LookupAll(ecosystem, pkg string) []policy.CatalogMatch {
	if ecosystem != "beekeeper" {
		return nil
	}

	var matches []policy.CatalogMatch
	for i := range a.entries {
		e := &a.entries[i]
		if e.Package != pkg && pkg != "" {
			continue
		}
		for _, v := range e.Versions {
			matches = append(matches, policy.CatalogMatch{
				CatalogSource:  "beekeeper-self",
				EntryID:        e.ID,
				Ecosystem:      "beekeeper",
				Package:        e.Package,
				Version:        v,
				Severity:       e.Severity,
				Signed:         true, // beekeeper-self feed is always Ed25519-verified
				CatalogVersion: "beekeeper-self",
			})
		}
	}
	return matches
}
