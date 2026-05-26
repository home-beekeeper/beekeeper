package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// Package-level marketplace base URLs — overridable in tests so httptest servers
// can intercept marketplace calls without DNS or real network access.
// Uses the same pattern as npmRegistryBase in registry.go.
var (
	openVSXBase             = "https://open-vsx.org/api"
	vscodeMarketplaceBase   = "https://marketplace.visualstudio.com/_apis/public/gallery/extensionquery/"
)

// marketplaceCachePath returns the filesystem path for a marketplace cache entry.
// Format: <cacheDir>/marketplace-cache/<publisher>/<name>/<version>.json
//
// Each attacker-controlled segment is sanitized with filepath.Base to prevent
// directory traversal (e.g. publisher="../../state" writing outside the cache dir).
func marketplaceCachePath(cacheDir, publisher, name, version string) string {
	return filepath.Join(cacheDir, "marketplace-cache",
		filepath.Base(publisher),
		filepath.Base(name),
		filepath.Base(version)+".json")
}

// openVSXResponse is the JSON response from the Open VSX REST API.
// GET /api/<publisher>/<name>/<version>
type openVSXResponse struct {
	Timestamp string `json:"timestamp"`
	Error     string `json:"error"`
}

// fetchOpenVSXTimestamp fetches the publish timestamp for a VS Code extension
// from Open VSX Registry. Returns the timestamp string on success.
//
// Open VSX's "timestamp" field reflects the last sync from VS Code Marketplace,
// not necessarily the original first-publish time. This means the age computed
// from Open VSX may under-estimate how old an extension truly is (Pitfall 4).
func fetchOpenVSXTimestamp(ctx context.Context, client *http.Client, publisher, name, version string) (string, error) {
	url := openVSXBase + "/" + publisher + "/" + name + "/" + version
	var resp openVSXResponse
	if err := fetchRegistryJSON(ctx, client, url, &resp); err != nil {
		return "", fmt.Errorf("open vsx timestamp for %s.%s@%s: %w", publisher, name, version, err)
	}
	if resp.Error != "" {
		return "", fmt.Errorf("open vsx timestamp for %s.%s@%s: API error: %s", publisher, name, version, resp.Error)
	}
	if resp.Timestamp == "" {
		return "", fmt.Errorf("open vsx timestamp for %s.%s@%s: timestamp field empty", publisher, name, version)
	}
	return resp.Timestamp, nil
}

// vscodeMarketplaceResponse models the relevant portion of the VS Code
// Marketplace extensionquery API response.
type vscodeMarketplaceResponse struct {
	Results []struct {
		Extensions []struct {
			PublishedDate string `json:"publishedDate"`
		} `json:"extensions"`
	} `json:"results"`
}

// fetchVSCodeMarketplaceTimestamp fetches the publish timestamp for a VS Code
// extension from the VS Code Marketplace gallery API.
//
// Note: Open VSX's timestamp field is the last-sync time from VS Code Marketplace,
// not the original first-publish date (Pitfall 4). When Open VSX is the primary
// source, its timestamp may under-estimate the extension's true age. VS Code
// Marketplace's publishedDate is used here as a fallback because it reflects the
// true first-publish date on the authoritative registry.
func fetchVSCodeMarketplaceTimestamp(ctx context.Context, client *http.Client, publisher, name string) (string, error) {
	extensionID := publisher + "." + name
	queryBody := fmt.Sprintf(`{"filters":[{"criteria":[{"filterType":7,"value":%q}]}],"flags":914}`, extensionID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, vscodeMarketplaceBase, strings.NewReader(queryBody))
	if err != nil {
		return "", fmt.Errorf("vscode marketplace request for %s: %w", extensionID, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json;api-version=3.0-preview.1")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("vscode marketplace fetch for %s: %w", extensionID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("vscode marketplace returned HTTP %d for %s", resp.StatusCode, extensionID)
	}

	// Cap read to 4 MB to prevent runaway responses (mirrors fetchRegistryJSON).
	limited := io.LimitReader(resp.Body, 4<<20)
	var result vscodeMarketplaceResponse
	if err := json.NewDecoder(limited).Decode(&result); err != nil {
		return "", fmt.Errorf("vscode marketplace decode for %s: %w", extensionID, err)
	}

	if len(result.Results) == 0 ||
		len(result.Results[0].Extensions) == 0 ||
		result.Results[0].Extensions[0].PublishedDate == "" {
		return "", fmt.Errorf("vscode marketplace: publishedDate missing for %s", extensionID)
	}
	return result.Results[0].Extensions[0].PublishedDate, nil
}

// FetchMarketplaceAge is the I/O adapter for editor extension timestamp lookup.
// It mirrors FetchPublishAge exactly, using Open VSX as the primary source and
// falling back to the VS Code Marketplace gallery API.
//
// Cache-first strategy with 24h TTL:
//
//  1. If a fresh (<24h-old) cache entry exists, use it.
//  2. On cache miss (or stale entry): fetch from Open VSX.
//  3. On Open VSX error: fall back to VS Code Marketplace.
//  4. If both fail: write Missing:true entry (fail-closed) and return (0, true, nil).
//  5. On success: parse timestamp, write fresh cache entry, return (ageMinutes, false, nil).
//
// Return contract:
//
//	(ageMinutes>0, false, nil) — extension age computed; caller runs EvaluateReleaseAge
//	(0, true, nil)             — timestamp unavailable; EvaluateReleaseAge blocks (fail-closed)
//	(_, _, non-nil)            — unexpected I/O error (cache write failure etc.)
func FetchMarketplaceAge(
	ctx context.Context,
	client *http.Client,
	cacheDir, publisher, name, version string,
	now time.Time,
) (ageMinutes int64, missing bool, err error) {
	path := marketplaceCachePath(cacheDir, publisher, name, version)

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

	// 2. Cache miss or stale — fetch from Open VSX.
	var tsStr string
	tsStr, err = fetchOpenVSXTimestamp(ctx, client, publisher, name, version)
	if err != nil {
		// 3. Open VSX failed — fall back to VS Code Marketplace.
		tsStr, err = fetchVSCodeMarketplaceTimestamp(ctx, client, publisher, name)
	}

	if err != nil {
		// 4. Both sources failed: write Missing:true cache entry to avoid hammering
		//    registries on repeated calls within the TTL window.
		missingEntry := ageCacheEntry{
			CachedAt: now,
			Missing:  true,
		}
		// Best-effort write; don't propagate cache write errors on fetch failures.
		_ = writeAgeCacheEntry(path, missingEntry)
		return 0, true, nil
	}

	// 5. Parse the timestamp — try RFC3339Nano then RFC3339.
	var publishedAt time.Time
	var parseErr error
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
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
	if writeErr := writeAgeCacheEntry(path, freshEntry); writeErr != nil {
		// Cache write failure is unexpected I/O; surface it.
		return 0, false, writeErr
	}

	age := int64(now.Sub(publishedAt).Minutes())
	if age < 0 {
		// Registry returned a future timestamp (clock skew or attacker-controlled).
		// Treat as missing so the policy engine fails closed.
		missingEntry := ageCacheEntry{CachedAt: now, Missing: true}
		_ = writeAgeCacheEntry(path, missingEntry)
		return 0, true, nil
	}
	return age, false, nil
}

