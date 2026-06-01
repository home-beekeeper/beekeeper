# Plan 03-02 Summary: Marketplace Timestamp Adapter + Quarantine Manager

## Status: DONE

## Deliverables

### 1. `internal/catalog/marketplace.go`

Added to the existing `catalog` package (same package as `age_cache.go`).

#### Function Signatures

```go
func FetchMarketplaceAge(
    ctx context.Context,
    client *http.Client,
    cacheDir, publisher, name, version string,
    now time.Time,
) (ageMinutes int64, missing bool, err error)

func fetchOpenVSXTimestamp(ctx context.Context, client *http.Client, publisher, name, version string) (string, error)
func fetchVSCodeMarketplaceTimestamp(ctx context.Context, client *http.Client, publisher, name string) (string, error)
func marketplaceCachePath(cacheDir, publisher, name, version string) string
```

#### Package-level overridable URL vars
```go
var openVSXBase = "https://open-vsx.org/api"
var vscodeMarketplaceBase = "https://marketplace.visualstudio.com/_apis/public/gallery/extensionquery/"
```

#### Cache path format
`<cacheDir>/marketplace-cache/<publisher>/<name>/<version>.json`
(filepath.Base sanitization on all three segments, matching ageCachePath pattern)

#### Strategy
- Primary: Open VSX (GET `openVSXBase/<publisher>/<name>/<version>`)
- Fallback: VS Code Marketplace (POST `vscodeMarketplaceBase` with extensionquery body)
- Cache: 24h TTL via `readAgeCacheEntry`/`writeAgeCacheEntry` (reused from age_cache.go)
- Fail-closed: both sources fail → write Missing:true, return (0, true, nil)
- Pitfall 4 documented: Open VSX timestamp is last-sync from VS Code Marketplace, not first-publish

---

### 2. `internal/quarantine/quarantine.go`

New `quarantine` package under `internal/quarantine/`.

#### Types

```go
type CatalogMatchSummary struct {
    CatalogSource string `json:"catalog_source"`
    EntryID       string `json:"entry_id"`
    Severity      string `json:"severity"`
}

type Manifest struct {
    ID             string               `json:"id"`
    Publisher      string               `json:"publisher"`
    Name           string               `json:"name"`
    Version        string               `json:"version"`
    DisplayName    string               `json:"display_name"`
    OriginalPath   string               `json:"original_path"`
    QuarantinedAt  time.Time            `json:"quarantined_at"`
    Reason         string               `json:"reason"`
    RuleIDs        []string             `json:"rule_ids"`
    AuditRecordID  string               `json:"audit_record_id"`
    CatalogMatches []CatalogMatchSummary `json:"catalog_matches,omitempty"`
}
```

#### Function Signatures

```go
func ExtensionsDir(quarantineDir string) string
func Move(quarantineDir, extensionPath string, m Manifest) (id string, err error)
func List(quarantineDir string) ([]Manifest, error)
func Restore(quarantineDir, id string) error
func Purge(quarantineDir string) (purged []string, err error)
```

#### Key behaviors
- `Move`: id = `<pub>.<name>-<ver>-<UnixNano>`, all components filepath.Base sanitized; path-traversal guard (prefix check against ExtensionsDir); cross-device os.Rename error surfaced (copy+delete not implemented in Phase 3); manifest written with `json.MarshalIndent`; `platform.SetOwnerOnly` enforces 0600 permissions
- `List`: silently skips entries with no/invalid manifest; empty dir → empty slice (not error)
- `Restore`: filepath.Base(id) strips traversal; prefix check; requires non-empty OriginalPath
- `Purge`: unconditional os.RemoveAll per entry; CLI layer owns confirmation; partial failure: already-purged IDs returned, first error surfaced

---

## Test Results

```
=== RUN   TestFetchMarketplaceAge
--- PASS: TestFetchMarketplaceAge (0.06s)
=== RUN   TestMarketplaceAgeCacheHit
--- PASS: TestMarketplaceAgeCacheHit (0.04s)
=== RUN   TestMarketplaceAgeMissing
--- PASS: TestMarketplaceAgeMissing (0.05s)
PASS
ok  github.com/mzansi-agentive/beekeeper/internal/catalog

=== RUN   TestQuarantineList
--- PASS: TestQuarantineList (0.05s)
=== RUN   TestQuarantineRestore
--- PASS: TestQuarantineRestore (0.04s)
=== RUN   TestQuarantinePurge
--- PASS: TestQuarantinePurge (0.02s)
=== RUN   TestQuarantineRestorePathTraversal
--- PASS: TestQuarantineRestorePathTraversal (0.00s)
PASS
ok  github.com/mzansi-agentive/beekeeper/internal/quarantine

Full suites: ok (catalog + quarantine)
go build ./...: SUCCESS
```

---

## Deviations from Plan

None. All requirements implemented as specified:
- `readAgeCacheEntry`/`writeAgeCacheEntry`/`writeFileAtomic` reused from `age_cache.go` (same package — no re-import)
- `quarantine` does NOT import `internal/policy` (CatalogMatchSummary is a local copy)
- VS Code Marketplace uses manual POST (fetchRegistryJSON is GET-only, as noted)
- Cross-device move: not implemented; error propagated with documentation note
- All tests use t.TempDir() and httptest.Server — no live network, no real filesystem side effects
