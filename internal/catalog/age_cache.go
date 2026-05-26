package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ageCacheEntry is the on-disk format for a publish-timestamp cache record.
// Stored at <cacheDir>/age-cache/<ecosystem>/<pkg>/<version>.json.
//
//   - PublishedAt: the registry-reported publish timestamp for the package.
//   - CachedAt: wall-clock time when this entry was written (used for TTL).
//   - Missing: true when the registry returned no usable timestamp; the caller
//     should treat this as RegistryCheckFailed and fail closed.
type ageCacheEntry struct {
	PublishedAt time.Time `json:"published_at"`
	CachedAt    time.Time `json:"cached_at"`
	Missing     bool      `json:"missing"`
}

// ageCacheTTL is the maximum age of a cache entry before it is considered stale
// and a fresh registry request is made.
const ageCacheTTL = 24 * time.Hour

// ageCachePath returns the filesystem path for a cache entry.
// Format: <cacheDir>/age-cache/<ecosystem>/<pkg>/<version>.json
//
// Each attacker-controlled segment is sanitized with filepath.Base to prevent
// directory traversal (e.g. pkg="../../state" writing outside the cache dir).
func ageCachePath(cacheDir, ecosystem, pkg, version string) string {
	return filepath.Join(cacheDir, "age-cache",
		filepath.Base(ecosystem),
		filepath.Base(pkg),
		filepath.Base(version)+".json")
}

// readAgeCacheEntry reads and deserializes a cache entry from disk.
// Returns (entry, true) on success; (zero, false) if the file is absent or
// unreadable.
func readAgeCacheEntry(path string) (ageCacheEntry, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ageCacheEntry{}, false
	}
	var entry ageCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return ageCacheEntry{}, false
	}
	return entry, true
}

// writeAgeCacheEntry atomically writes an ageCacheEntry to path, creating
// all parent directories (mode 0o700) if needed.
func writeAgeCacheEntry(path string, entry ageCacheEntry) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data)
}

// FetchPublishAge is the I/O adapter that ties the registry fetchers
// (registry.go) to the release-age policy (internal/policy/release_age.go).
// It implements a cache-first strategy with a 24h TTL:
//
//  1. If a fresh (<24h-old) cache entry exists, use it.
//  2. On cache miss (or stale entry): fetch from the registry via fetchPublishTime.
//  3. On fetch error: write a Missing:true entry (capped retry backoff — the
//     registry is not hammered on repeated calls within the TTL window) and
//     return (0, true, nil) — the fail-closed signal the policy layer turns into
//     a block (PLCY-02 T-02-06-02).
//  4. On fetch success: parse the timestamp, write a fresh cache entry, and
//     return (ageMinutes, false, nil).
//
// The `now` parameter is caller-supplied so that:
//   - TTL math (now.Sub(entry.CachedAt) < 24h) is testable without wall-clock
//     flakiness.
//   - Age math (now.Sub(entry.PublishedAt).Minutes()) is testable with synthetic
//     timestamps.
//
// The production caller (internal/check/handler.go, Plan 08) passes
// time.Now().UTC(). Tests supply synthetic times.
//
// Return contract:
//
//	(ageMinutes>0, false, nil)  — package publish age computed; use EvaluateReleaseAge
//	(0, true, nil)              — timestamp unavailable; EvaluateReleaseAge will block
//	(_, _, non-nil)             — unexpected I/O error (cache write failure etc.)
func FetchPublishAge(
	ctx context.Context,
	client *http.Client,
	cacheDir, ecosystem, pkg, version string,
	now time.Time,
) (ageMinutes int64, missing bool, err error) {
	path := ageCachePath(cacheDir, ecosystem, pkg, version)

	// 1. Cache-first: try to read a fresh entry.
	if entry, ok := readAgeCacheEntry(path); ok {
		if now.Sub(entry.CachedAt) < ageCacheTTL {
			// Cache hit — entry is still fresh.
			if entry.Missing {
				return 0, true, nil
			}
			age := int64(now.Sub(entry.PublishedAt).Minutes())
			if age < 0 {
				// Future timestamp in cache — treat as missing (fail-closed).
				missingEntry := ageCacheEntry{CachedAt: now, Missing: true}
				_ = writeAgeCacheEntry(path, missingEntry)
				return 0, true, nil
			}
			return age, false, nil
		}
	}

	// 2. Cache miss or stale — fetch from registry.
	tsStr, fetchErr := fetchPublishTime(ctx, client, ecosystem, pkg, version)
	if fetchErr != nil {
		// 3. Fetch failed: write Missing:true cache entry to avoid hammering the
		//    registry on repeated calls within the TTL window (T-02-06-02).
		missingEntry := ageCacheEntry{
			CachedAt: now,
			Missing:  true,
		}
		// Best-effort write; don't propagate cache write errors on fetch failures.
		_ = writeAgeCacheEntry(path, missingEntry)
		return 0, true, nil
	}

	// 4. Parse the timestamp — try RFC3339 then RFC3339Nano.
	var publishedAt time.Time
	var parseErr error
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano} {
		publishedAt, parseErr = time.Parse(layout, tsStr)
		if parseErr == nil {
			break
		}
	}
	if parseErr != nil {
		// Unparseable timestamp: treat as missing (fail closed).
		missingEntry := ageCacheEntry{
			CachedAt: now,
			Missing:  true,
		}
		_ = writeAgeCacheEntry(path, missingEntry)
		return 0, true, nil
	}

	// Write a successful cache entry.
	freshEntry := ageCacheEntry{
		PublishedAt: publishedAt,
		CachedAt:    now,
		Missing:     false,
	}
	if err := writeAgeCacheEntry(path, freshEntry); err != nil {
		// Cache write failure is unexpected I/O; surface it.
		return 0, false, err
	}

	age := int64(now.Sub(publishedAt).Minutes())
	if age < 0 {
		// Registry returned a future timestamp (clock skew or attacker-controlled).
		// Treat as missing so the policy engine fails closed (PLCY-02).
		missingEntry := ageCacheEntry{CachedAt: now, Missing: true}
		_ = writeAgeCacheEntry(path, missingEntry)
		return 0, true, nil
	}
	return age, false, nil
}
