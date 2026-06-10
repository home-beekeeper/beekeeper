package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// bumblebeeContentsURL is the GitHub Contents API endpoint that lists the
// Bumblebee threat_intel/ directory. It is the single pinned host Sync fetches
// from; only files it enumerates are downloaded (mitigates off-host
// DownloadURL spoofing, T-02-05).
//
// It is a var (not a const) solely so the sync tests can point it at an
// httptest server; production value is never reassigned (mirrors the PipePath
// test-substitution convention in internal/ipc).
var bumblebeeContentsURL = "https://api.github.com/repos/perplexityai/bumblebee/contents/threat_intel"

// ghContentItem models a GitHub Contents API entry.
type ghContentItem struct {
	Name        string `json:"name"`
	DownloadURL string `json:"download_url"`
	Type        string `json:"type"`
}

// SyncResult reports the outcome of a SyncConditional call.
//
// On a 304 (upstream unchanged) NotModified is true, no raw files are fetched,
// no index is rebuilt, and Count is 0 (the caller keeps its prior count). ETag
// carries the list-call ETag to persist for the next If-None-Match request.
type SyncResult struct {
	// Count is the number of entries indexed on a 200 sync; 0 when NotModified.
	Count int
	// ETag is the GitHub Contents-list ETag to persist for the next sync.
	// On NotModified it echoes the prior ETag (still current).
	ETag string
	// NotModified is true when the upstream list returned 304 — no fetch, no
	// rebuild, the last-good index is left untouched.
	NotModified bool
}

// Sync is the backward-compatible entry point: an unconditional sync that
// always fetches and rebuilds (no If-None-Match), returning the entry count.
// Callers that track an ETag for conditional 304 skips should use
// SyncConditional instead.
func Sync(ctx context.Context, client *http.Client, catalogDir string) (int, error) {
	r, err := SyncConditional(ctx, client, catalogDir, "")
	if err != nil {
		return 0, err
	}
	return r.Count, nil
}

// SyncConditional fetches the Bumblebee threat_intel/ catalog over HTTP, parses
// and schema-validates every .json file, persists the raw concatenated catalog
// to <catalogDir>/bumblebee.json, and builds the memory-mappable binary index
// at <catalogDir>/bumblebee.idx. It returns a SyncResult with the entry count
// and the list ETag.
//
// When prevETag is non-empty it is sent as If-None-Match on the GitHub Contents
// LIST call only (raw file fetches are unmetered and unconditional — RESEARCH
// Tier-1). On a 304 it short-circuits: NO raw fetch, NO index rebuild, the
// existing index is left untouched, and SyncResult.NotModified is true.
//
// SyncConditional is a user-triggered, non-hot-path operation: any HTTP or
// parse failure returns an error and NO index is written (callers must not
// observe a partial index, and a failure never destroys the last-good index).
// Entries lacking a catalog_signature are counted and a single warning is
// emitted to stderr noting they are warn-only (CTLG-07).
//
// If the GITHUB_TOKEN environment variable is set, it is sent as an
// Authorization: Bearer header to raise the GitHub rate limit (Pitfall 6). The
// token is only ever used as a request header — never written to disk or logs
// (T-02-04).
func SyncConditional(ctx context.Context, client *http.Client, catalogDir, prevETag string) (SyncResult, error) {
	token := os.Getenv("GITHUB_TOKEN")

	items, etag, notModified, err := listThreatIntel(ctx, client, token, prevETag)
	if err != nil {
		return SyncResult{}, err
	}
	if notModified {
		// 304: upstream list unchanged. Skip all raw fetches + the index
		// rebuild; leave the last-good index on disk untouched. The caller
		// keeps its prior entry count and bumps LastSuccess/LastAttempt.
		return SyncResult{ETag: prevETag, NotModified: true}, nil
	}

	var allEntries []Entry
	var rawFiles []json.RawMessage

	for _, item := range items {
		if item.Type != "file" || !strings.HasSuffix(strings.ToLower(item.Name), ".json") {
			continue
		}
		if item.DownloadURL == "" {
			continue
		}

		body, err := fetch(ctx, client, item.DownloadURL, token)
		if err != nil {
			return SyncResult{}, fmt.Errorf("fetch %s: %w", item.Name, err)
		}

		cf, err := ParseCatalogFile(body)
		if err != nil {
			return SyncResult{}, fmt.Errorf("parse %s: %w", item.Name, err)
		}

		allEntries = append(allEntries, cf.Entries...)
		rawFiles = append(rawFiles, json.RawMessage(body))
	}

	// Count unsigned entries for the warn-only advisory (CTLG-07).
	unsigned := 0
	for i := range allEntries {
		if !VerifySignature(allEntries[i]) {
			unsigned++
		}
	}
	if unsigned > 0 {
		fmt.Fprintf(os.Stderr,
			"warning: %d catalog entries are unsigned; the policy engine treats unsigned entries as warn-only (CTLG-07)\n",
			unsigned)
	}

	// Persist the raw catalog cache (a JSON array of the source files) only
	// after all files parsed successfully.
	rawOut, err := json.MarshalIndent(rawFiles, "", "  ")
	if err != nil {
		return SyncResult{}, fmt.Errorf("marshal raw catalog cache: %w", err)
	}
	if err := os.WriteFile(filepath.Join(catalogDir, "bumblebee.json"), rawOut, 0o600); err != nil {
		return SyncResult{}, fmt.Errorf("write raw catalog cache: %w", err)
	}

	if err := BuildIndex(filepath.Join(catalogDir, "bumblebee.idx"), allEntries); err != nil {
		return SyncResult{}, fmt.Errorf("build index: %w", err)
	}

	return SyncResult{Count: len(allEntries), ETag: etag}, nil
}

// listThreatIntel calls the GitHub Contents API and returns the directory
// listing for threat_intel/, the response ETag, and a notModified flag.
//
// When prevETag is non-empty it is sent as If-None-Match: a 304 short-circuits
// (notModified=true, nil items) so the caller can skip every raw fetch and the
// index rebuild. This is the ONLY conditional request — the list call is the
// only one metered against GitHub's rate bucket (RESEARCH Tier-1).
func listThreatIntel(ctx context.Context, client *http.Client, token, prevETag string) (items []ghContentItem, etag string, notModified bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, bumblebeeContentsURL, nil)
	if err != nil {
		return nil, "", false, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if prevETag != "" {
		req.Header.Set("If-None-Match", prevETag)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", false, fmt.Errorf("list threat_intel: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		// 304: upstream unchanged — no body to decode, no fetch/rebuild needed.
		return nil, prevETag, true, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", false, fmt.Errorf("list threat_intel: GitHub returned HTTP %d", resp.StatusCode)
	}

	etag = resp.Header.Get("ETag")
	if derr := json.NewDecoder(resp.Body).Decode(&items); derr != nil {
		return nil, "", false, fmt.Errorf("decode threat_intel listing: %w", derr)
	}
	return items, etag, false, nil
}

// fetch GETs url and returns its body, capping the read to a sane bound.
// It creates a one-off http.Client with a CheckRedirect policy that strips
// the Authorization header on any cross-host redirect, preventing token
// leakage to an attacker-controlled DownloadURL (WR-01).
func fetch(ctx context.Context, client *http.Client, url, token string) ([]byte, error) {
	// Build a per-request client that inherits the caller's transport and
	// timeout but overrides redirect behaviour to strip auth on host change.
	fetchClient := *client
	fetchClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 0 && req.URL.Host != via[0].URL.Host {
			req.Header.Del("Authorization")
		}
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := fetchClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub returned HTTP %d", resp.StatusCode)
	}

	// Bound the per-file read to 16MB; catalogs are far smaller in practice.
	return io.ReadAll(io.LimitReader(resp.Body, 16<<20))
}
