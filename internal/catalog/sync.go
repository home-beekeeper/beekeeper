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
const bumblebeeContentsURL = "https://api.github.com/repos/perplexityai/bumblebee/contents/threat_intel"

// ghContentItem models a GitHub Contents API entry.
type ghContentItem struct {
	Name        string `json:"name"`
	DownloadURL string `json:"download_url"`
	Type        string `json:"type"`
}

// Sync fetches the Bumblebee threat_intel/ catalog over HTTP, parses and
// schema-validates every .json file, persists the raw concatenated catalog to
// <catalogDir>/bumblebee.json, and builds the memory-mappable binary index at
// <catalogDir>/bumblebee.idx. It returns the number of entries indexed.
//
// Sync is a user-triggered, non-hot-path operation: any HTTP or parse failure
// returns an error and NO index is written (callers must not observe a partial
// index). Entries lacking a catalog_signature are counted and a single warning
// is emitted to stderr noting they are warn-only (CTLG-07).
//
// If the GITHUB_TOKEN environment variable is set, it is sent as an
// Authorization: Bearer header to raise the GitHub rate limit (Pitfall 6). The
// token is only ever used as a request header — never written to disk or logs
// (T-02-04).
func Sync(ctx context.Context, client *http.Client, catalogDir string) (int, error) {
	token := os.Getenv("GITHUB_TOKEN")

	items, err := listThreatIntel(ctx, client, token)
	if err != nil {
		return 0, err
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
			return 0, fmt.Errorf("fetch %s: %w", item.Name, err)
		}

		cf, err := ParseCatalogFile(body)
		if err != nil {
			return 0, fmt.Errorf("parse %s: %w", item.Name, err)
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
		return 0, fmt.Errorf("marshal raw catalog cache: %w", err)
	}
	if err := os.WriteFile(filepath.Join(catalogDir, "bumblebee.json"), rawOut, 0o600); err != nil {
		return 0, fmt.Errorf("write raw catalog cache: %w", err)
	}

	if err := BuildIndex(filepath.Join(catalogDir, "bumblebee.idx"), allEntries); err != nil {
		return 0, fmt.Errorf("build index: %w", err)
	}

	return len(allEntries), nil
}

// listThreatIntel calls the GitHub Contents API and returns the directory
// listing for threat_intel/.
func listThreatIntel(ctx context.Context, client *http.Client, token string) ([]ghContentItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, bumblebeeContentsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list threat_intel: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list threat_intel: GitHub returned HTTP %d", resp.StatusCode)
	}

	var items []ghContentItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("decode threat_intel listing: %w", err)
	}
	return items, nil
}

// fetch GETs url and returns its body, capping the read to a sane bound.
func fetch(ctx context.Context, client *http.Client, url, token string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
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
