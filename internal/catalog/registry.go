package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// ErrEcosystemLifecycleUnsupported is returned by fetchLifecycleScripts for
// non-npm ecosystems. Lifecycle script inspection (preinstall/install/postinstall)
// is only defined for npm in Phase 2; other ecosystems lack a standardised
// equivalent in their registry APIs.
//
// The caller that builds LifecycleInput MUST treat any non-nil error from
// fetchLifecycleScripts as RegistryCheckFailed:true, which causes
// EvaluateLifecycle to block (fail-closed). Users with legitimate non-npm
// packages that contain build scripts should add them to
// ~/.beekeeper/policies/lifecycle.json (the allowlist escape hatch).
var ErrEcosystemLifecycleUnsupported = errors.New("lifecycle script inspection not supported for this ecosystem")

// Package-level registry base URLs — overridable in tests so httptest servers
// can intercept registry calls without DNS or real network access.
var (
	npmRegistryBase        = "https://registry.npmjs.org"
	pypiRegistryBase       = "https://pypi.org"
	cratesRegistryBase     = "https://crates.io"
	rubygemsRegistryBase   = "https://rubygems.org"
	goProxyBase            = "https://proxy.golang.org"
	packagistRegistryBase  = "https://repo.packagist.org"
)

// lifecycleScriptKeys are the npm package.json script keys that Beekeeper
// treats as install-time lifecycle scripts requiring explicit allowlisting.
var lifecycleScriptKeys = map[string]bool{
	"preinstall":  true,
	"install":     true,
	"postinstall": true,
}

// fetchRegistryJSON is a shared helper: GETs url with the given context and
// client, enforces a 4 MB body limit, and JSON-decodes the response body into
// dest. Returns an error for any non-200 status or decode failure. This matches
// the fetch() idiom in sync.go but decodes JSON directly (no raw bytes needed).
func fetchRegistryJSON(ctx context.Context, client *http.Client, url string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registry returned HTTP %d for %s", resp.StatusCode, url)
	}

	// Cap read to 4 MB to prevent runaway responses.
	limited := io.LimitReader(resp.Body, 4<<20)
	return json.NewDecoder(limited).Decode(dest)
}

// fetchNPMPublishTime returns the RFC3339 publish timestamp for pkg@version
// from the npm full package document. The full package document (GET
// https://registry.npmjs.org/<pkg>) carries a ".time" map of version strings to
// RFC3339 timestamps. The per-version document lacks this field.
func fetchNPMPublishTime(ctx context.Context, client *http.Client, pkg, version string) (string, error) {
	url := npmRegistryBase + "/" + url.PathEscape(pkg)
	var doc struct {
		Time map[string]string `json:"time"`
	}
	if err := fetchRegistryJSON(ctx, client, url, &doc); err != nil {
		return "", fmt.Errorf("npm publish time for %s@%s: %w", pkg, version, err)
	}
	if doc.Time == nil {
		return "", fmt.Errorf("npm publish time for %s@%s: .time field missing", pkg, version)
	}
	ts, ok := doc.Time[version]
	if !ok || ts == "" {
		return "", fmt.Errorf("npm publish time for %s@%s: version not in .time map", pkg, version)
	}
	return ts, nil
}

// fetchPyPIPublishTime returns the upload timestamp for pkg@version from the
// PyPI JSON API: GET https://pypi.org/pypi/<pkg>/<version>/json
// → .urls[0].upload_time_iso_8601
func fetchPyPIPublishTime(ctx context.Context, client *http.Client, pkg, version string) (string, error) {
	url := pypiRegistryBase + "/pypi/" + url.PathEscape(pkg) + "/" + url.PathEscape(version) + "/json"
	var doc struct {
		URLs []struct {
			UploadTimeISO string `json:"upload_time_iso_8601"`
		} `json:"urls"`
	}
	if err := fetchRegistryJSON(ctx, client, url, &doc); err != nil {
		return "", fmt.Errorf("pypi publish time for %s@%s: %w", pkg, version, err)
	}
	if len(doc.URLs) == 0 || doc.URLs[0].UploadTimeISO == "" {
		return "", fmt.Errorf("pypi publish time for %s@%s: .urls[0].upload_time_iso_8601 missing", pkg, version)
	}
	return doc.URLs[0].UploadTimeISO, nil
}

// fetchCratesPublishTime returns the creation timestamp for pkg@version from
// the crates.io API: GET https://crates.io/api/v1/crates/<pkg>/<version>
// → .version.created_at
func fetchCratesPublishTime(ctx context.Context, client *http.Client, pkg, version string) (string, error) {
	url := cratesRegistryBase + "/api/v1/crates/" + url.PathEscape(pkg) + "/" + url.PathEscape(version)
	var doc struct {
		Version struct {
			CreatedAt string `json:"created_at"`
		} `json:"version"`
	}
	if err := fetchRegistryJSON(ctx, client, url, &doc); err != nil {
		return "", fmt.Errorf("crates.io publish time for %s@%s: %w", pkg, version, err)
	}
	if doc.Version.CreatedAt == "" {
		return "", fmt.Errorf("crates.io publish time for %s@%s: .version.created_at missing", pkg, version)
	}
	return doc.Version.CreatedAt, nil
}

// fetchRubyGemsPublishTime returns the build date for pkg@version from the
// RubyGems API: GET https://rubygems.org/api/v1/versions/<pkg>.json
// Returns the built_at timestamp for the matching version.
func fetchRubyGemsPublishTime(ctx context.Context, client *http.Client, pkg, version string) (string, error) {
	url := rubygemsRegistryBase + "/api/v1/versions/" + url.PathEscape(pkg) + ".json"
	var versions []struct {
		Number  string `json:"number"`
		BuiltAt string `json:"built_at"`
	}
	if err := fetchRegistryJSON(ctx, client, url, &versions); err != nil {
		return "", fmt.Errorf("rubygems publish time for %s@%s: %w", pkg, version, err)
	}
	for _, v := range versions {
		if v.Number == version {
			if v.BuiltAt == "" {
				return "", fmt.Errorf("rubygems publish time for %s@%s: built_at missing", pkg, version)
			}
			return v.BuiltAt, nil
		}
	}
	return "", fmt.Errorf("rubygems publish time for %s@%s: version not found in API response", pkg, version)
}

// fetchGoPublishTime returns the publish timestamp for a Go module@version from
// the Go module proxy: GET https://proxy.golang.org/<module>/@v/<version>.info
// → .Time (RFC3339)
func fetchGoPublishTime(ctx context.Context, client *http.Client, pkg, version string) (string, error) {
	url := goProxyBase + "/" + url.PathEscape(pkg) + "/@v/" + url.PathEscape(version) + ".info"
	var doc struct {
		Time string `json:"Time"`
	}
	if err := fetchRegistryJSON(ctx, client, url, &doc); err != nil {
		return "", fmt.Errorf("go proxy publish time for %s@%s: %w", pkg, version, err)
	}
	if doc.Time == "" {
		return "", fmt.Errorf("go proxy publish time for %s@%s: .Time missing", pkg, version)
	}
	return doc.Time, nil
}

// fetchPackagistPublishTime returns the publish timestamp for a Packagist
// package@version: GET https://repo.packagist.org/p2/<pkg>.json
// → packages[pkg][].version == version → .time
func fetchPackagistPublishTime(ctx context.Context, client *http.Client, pkg, version string) (string, error) {
	url := packagistRegistryBase + "/p2/" + url.PathEscape(pkg) + ".json"
	var doc struct {
		Packages map[string][]struct {
			Version string `json:"version"`
			Time    string `json:"time"`
		} `json:"packages"`
	}
	if err := fetchRegistryJSON(ctx, client, url, &doc); err != nil {
		return "", fmt.Errorf("packagist publish time for %s@%s: %w", pkg, version, err)
	}
	entries, ok := doc.Packages[pkg]
	if !ok {
		return "", fmt.Errorf("packagist publish time for %s@%s: package not in response", pkg, version)
	}
	for _, e := range entries {
		if e.Version == version {
			if e.Time == "" {
				return "", fmt.Errorf("packagist publish time for %s@%s: .time missing", pkg, version)
			}
			return e.Time, nil
		}
	}
	return "", fmt.Errorf("packagist publish time for %s@%s: version not found", pkg, version)
}

// fetchPublishTime dispatches to the appropriate per-ecosystem registry fetcher
// and returns a timestamp string (RFC3339 or ISO 8601) on success. Unknown
// ecosystems return an error; the caller treats any error as a missing timestamp
// and fails closed (PLCY-02).
func fetchPublishTime(ctx context.Context, client *http.Client, ecosystem, pkg, version string) (string, error) {
	switch ecosystem {
	case "npm":
		return fetchNPMPublishTime(ctx, client, pkg, version)
	case "pypi":
		return fetchPyPIPublishTime(ctx, client, pkg, version)
	case "cargo":
		return fetchCratesPublishTime(ctx, client, pkg, version)
	case "rubygems":
		return fetchRubyGemsPublishTime(ctx, client, pkg, version)
	case "go":
		return fetchGoPublishTime(ctx, client, pkg, version)
	case "packagist":
		return fetchPackagistPublishTime(ctx, client, pkg, version)
	default:
		return "", fmt.Errorf("unsupported ecosystem: %q", ecosystem)
	}
}

// fetchLifecycleScripts returns the subset of lifecycle script keys
// {"preinstall","install","postinstall"} present in pkg@version's package
// manifest at the relevant registry.
//
// For non-npm ecosystems, ErrEcosystemLifecycleUnsupported is returned. The
// caller MUST treat any non-nil error as RegistryCheckFailed:true, which
// causes EvaluateLifecycle to block (fail-closed). This is intentional: the
// non-npm ecosystems (PyPI, Cargo, RubyGems, Composer, Go) do not expose a
// standardised "lifecycle scripts" concept in their registry APIs. Packages on
// those ecosystems that legitimately need build hooks should be explicitly added
// to ~/.beekeeper/policies/lifecycle.json (the per-package allowlist).
func fetchLifecycleScripts(ctx context.Context, client *http.Client, ecosystem, pkg, version string) ([]string, error) {
	switch ecosystem {
	case "npm":
		return fetchNPMLifecycleScripts(ctx, client, pkg, version)
	default:
		// All non-npm ecosystems are unsupported for lifecycle script inspection.
		// Caller must set RegistryCheckFailed:true → EvaluateLifecycle blocks.
		return nil, ErrEcosystemLifecycleUnsupported
	}
}

// fetchNPMLifecycleScripts inspects the npm registry version document for
// pkg@version and returns the subset of {"preinstall","install","postinstall"}
// keys present in the .scripts object. An empty slice means no lifecycle
// scripts are defined (safe to allow). Any fetch or parse error is returned;
// the caller treats it as RegistryCheckFailed:true.
func fetchNPMLifecycleScripts(ctx context.Context, client *http.Client, pkg, version string) ([]string, error) {
	url := npmRegistryBase + "/" + url.PathEscape(pkg) + "/" + url.PathEscape(version)
	var doc struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := fetchRegistryJSON(ctx, client, url, &doc); err != nil {
		return nil, fmt.Errorf("npm lifecycle scripts for %s@%s: %w", pkg, version, err)
	}

	var present []string
	for key := range doc.Scripts {
		if lifecycleScriptKeys[key] {
			present = append(present, key)
		}
	}
	return present, nil
}
