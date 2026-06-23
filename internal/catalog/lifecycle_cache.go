package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// lifecycleCacheEntry is the on-disk format for a lifecycle-script cache record.
// Stored at <cacheDir>/lifecycle-cache/<ecosystem>/<pkg>/<version>.json.
//
//   - Scripts: the subset of {"preinstall","install","postinstall"} present in
//     the package manifest (npm). Empty means no lifecycle scripts (safe).
//   - CachedAt: wall-clock time when this entry was written (used for TTL).
//   - Failed: true when the registry fetch failed OR the ecosystem is unsupported
//     for lifecycle inspection. The caller maps this to LifecycleInput.RegistryCheckFailed.
//
// Both the success outcome (a script list, possibly empty) AND the failure
// outcome (unsupported ecosystem / registry error) are cached, so a non-npm
// ecosystem or a flaky registry is not re-fetched within the TTL window - this
// mirrors age_cache.go's Missing-entry caching of fetch failures.
type lifecycleCacheEntry struct {
	Scripts  []string  `json:"scripts"`
	CachedAt time.Time `json:"cached_at"`
	Failed   bool      `json:"failed"`
}

// lifecycleCacheTTL is the maximum age of a lifecycle cache entry before it is
// considered stale and a fresh registry request is made.
const lifecycleCacheTTL = 24 * time.Hour

// lifecycleCachePath returns the filesystem path for a lifecycle cache entry.
// Format: <cacheDir>/lifecycle-cache/<ecosystem>/<pkg>/<version>.json
//
// Each attacker-controlled segment is sanitized with filepath.Base to prevent
// directory traversal (e.g. pkg="../../state" writing outside the cache dir),
// mirroring ageCachePath.
func lifecycleCachePath(cacheDir, ecosystem, pkg, version string) string {
	return filepath.Join(cacheDir, "lifecycle-cache",
		filepath.Base(ecosystem),
		filepath.Base(pkg),
		filepath.Base(version)+".json")
}

// readLifecycleCacheEntry reads and deserializes a lifecycle cache entry from
// disk. Returns (entry, true) on success; (zero, false) if the file is absent or
// unreadable.
func readLifecycleCacheEntry(path string) (lifecycleCacheEntry, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return lifecycleCacheEntry{}, false
	}
	var entry lifecycleCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return lifecycleCacheEntry{}, false
	}
	return entry, true
}

// writeLifecycleCacheEntry atomically writes a lifecycleCacheEntry to path,
// creating all parent directories (mode 0o700) if needed.
func writeLifecycleCacheEntry(path string, entry lifecycleCacheEntry) error {
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

// FetchLifecycleScripts is the cached I/O adapter that ties the registry
// lifecycle fetcher (registry.go fetchLifecycleScripts) to the lifecycle policy
// (internal/policy/lifecycle.go). It implements a cache-first strategy with a
// 24h TTL, mirroring FetchPublishAge:
//
//  1. If a fresh (<24h-old) cache entry exists, use it (success OR failure).
//  2. On cache miss (or stale entry): fetch from the registry via
//     fetchLifecycleScripts.
//  3. On fetch error (registry error or unsupported ecosystem): write a
//     Failed:true entry (so the registry is not hammered within the TTL window)
//     and return (nil, true, nil) - the failed signal the caller maps to
//     LifecycleInput.RegistryCheckFailed.
//  4. On fetch success: write a fresh cache entry with the script list and return
//     (scripts, false, nil).
//
// The `now` parameter is caller-supplied so TTL math is testable without
// wall-clock flakiness. The production caller (internal/check/posture_adapter.go)
// passes time.Now().UTC(); tests supply synthetic times.
//
// Return contract:
//
//	(scripts, false, nil) - script list resolved (may be empty); build LifecycleInput with it
//	(nil, true, nil)      - registry/ecosystem failure; set RegistryCheckFailed:true
//	(_, _, non-nil)       - unexpected I/O error (cache write failure etc.)
//
// IMPORTANT: this adapter caches the FAILED outcome and returns it as a soft
// "failed" signal. The pure EvaluateLifecycle still BLOCKS on
// RegistryCheckFailed:true (fail-closed, correct for the scan/watch extension
// path). The HOOK posture adapter intentionally re-maps that block to a WARN
// (fail-soft) - see internal/check/posture_adapter.go. This adapter itself takes
// no posture stance; it only resolves the input.
func FetchLifecycleScripts(
	ctx context.Context,
	client *http.Client,
	cacheDir, ecosystem, pkg, version string,
	now time.Time,
) (scripts []string, failed bool, err error) {
	path := lifecycleCachePath(cacheDir, ecosystem, pkg, version)

	// 1. Cache-first: try to read a fresh entry.
	if entry, ok := readLifecycleCacheEntry(path); ok {
		if now.Sub(entry.CachedAt) < lifecycleCacheTTL {
			if entry.Failed {
				return nil, true, nil
			}
			return entry.Scripts, false, nil
		}
	}

	// 2. Cache miss or stale - fetch from registry.
	fetched, fetchErr := fetchLifecycleScripts(ctx, client, ecosystem, pkg, version)
	if fetchErr != nil {
		// 3. Fetch failed (registry error OR unsupported ecosystem): cache the
		//    failure to avoid re-fetching within the TTL window. Best-effort write.
		failedEntry := lifecycleCacheEntry{CachedAt: now, Failed: true}
		_ = writeLifecycleCacheEntry(path, failedEntry)
		return nil, true, nil
	}

	// 4. Fetch success: cache the (possibly empty) script list.
	freshEntry := lifecycleCacheEntry{
		Scripts:  fetched,
		CachedAt: now,
		Failed:   false,
	}
	if writeErr := writeLifecycleCacheEntry(path, freshEntry); writeErr != nil {
		// Cache write failure is unexpected I/O; surface it.
		return nil, false, writeErr
	}
	return fetched, false, nil
}
