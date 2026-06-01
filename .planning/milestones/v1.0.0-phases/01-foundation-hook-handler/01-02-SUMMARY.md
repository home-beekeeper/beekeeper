# Phase 1 / Plan 02 — Catalog Sync — Summary

**Plan:** `01-PLAN-catalog-sync.md`
**Executed:** 2026-05-26
**Status:** Complete — all acceptance criteria met; live `catalogs sync` smoke-tested on the Windows dev machine (654 entries indexed).
**Commit:** `009284d` feat: Phase 1 Plan 02 — Bumblebee catalog loader, mmap index, sync command

## What Was Built

The `internal/catalog` package: Bumblebee catalog schema types, a schema-gated
JSON loader, a signature-presence check, a memory-mappable sorted binary index
for O(log n) lookup, and the HTTP fetch orchestration behind a now-functional
`beekeeper catalogs sync` subcommand.

### Files created

- `internal/catalog/schema.go` — `Entry`, `CatalogFile`, `SupportedSchemaVersion = "0.1.0"`
- `internal/catalog/loader.go` — `ValidateSchemaVersion`, `ParseCatalogFile` (rejects bare array + unknown version; defaults `catalog_source` to `bumblebee`)
- `internal/catalog/loader_test.go` — TestCatalogParse, TestUnknownSchemaVersion, TestRejectBareArray, TestDefaultCatalogSource
- `internal/catalog/verify.go` — `VerifySignature` (presence-only; crypto verification deferred to Phase 2)
- `internal/catalog/verify_test.go` — TestUnsignedReturnsFalse, TestSignedReturnsTrue
- `internal/catalog/index.go` — `BuildIndex`, `OpenIndex`, `Index.Lookup`, `Index.Count`, `Index.Close`
- `internal/catalog/index_test.go` — TestIndexBuildAndOpen, TestIndexLookupHit, TestIndexLookupMiss, TestIndexBinarySearchManyEntries (250 entries), TestIndexOpenDoesNotReadJSON, TestOpenIndexRejectsBadMagic
- `internal/catalog/sync.go` — `Sync(ctx, client, catalogDir) (int, error)`
- `testdata/catalog/nx-console.json` — verified Nx Console block-case fixture
- `testdata/catalog/clean.json` — clean npm allow-case fixture

### Files modified

- `cmd/beekeeper/main.go` — `catalogs sync` RunE now calls `platform.CatalogDir()` (MkdirAll), constructs a 30s `http.Client`, calls `catalog.Sync`, and prints entry count + index path. Stub removed.
- `go.mod` / `go.sum` — added `github.com/edsrzf/mmap-go v1.2.0` (bumped indirect `golang.org/x/sys`).

## Binary Index Format (BEEI v1)

All multi-byte integers little-endian. Whole file mmapped at offset 0 (Windows
allocation-granularity alignment pitfall avoided).

```
[16-byte header]  magic 0x42454549 ("BEEI") | version uint32(1) | count uint32 | reserved 0
[count * 48-byte records, sorted asc by Key]
    Key[32]  = sha256(ecosystem + "::" + strings.ToLower(package))[:32]
    DataOffset uint64 (relative to data section)
    DataLength uint64
[data section]  concatenated entry JSON blobs
```

`BuildIndex` deduplicates by key (last wins), sorts by key bytes, and writes via
temp-file-then-rename for atomicity (no partial index observable by a reader).
`Lookup` uses `sort.Search` + `bytes.Equal`, unmarshaling only the matched blob
inside the mmap — no source JSON is read on the lookup path. Package names are
lowercased in the key for case-insensitive matching.

## Key Interfaces Created (for downstream plans)

```go
// internal/catalog/schema.go
type Entry struct {
    ID, Name, Ecosystem, Package string
    Versions []string
    Severity, SourceURL, CatalogSignature, CatalogSource string
}
type CatalogFile struct { SchemaVersion string; Entries []Entry }
const SupportedSchemaVersion = "0.1.0"

// internal/catalog/loader.go
func ValidateSchemaVersion(v string) error
func ParseCatalogFile(data []byte) (CatalogFile, error)

// internal/catalog/index.go
type Index struct { /* mmap-backed, opaque */ }
func BuildIndex(path string, entries []Entry) error
func OpenIndex(path string) (*Index, error)
func (idx *Index) Lookup(ecosystem, pkg string) (Entry, bool)
func (idx *Index) Close() error
func (idx *Index) Count() int

// internal/catalog/verify.go
func VerifySignature(e Entry) bool

// internal/catalog/sync.go
func Sync(ctx context.Context, client *http.Client, catalogDir string) (int, error)
```

The policy engine (Plan 04) and hook handler (Plan 05) consume `Index` and
`VerifySignature`. `Lookup` returns an exact-match `Entry` by `(ecosystem,
package)`; version matching against `Entry.Versions` is the consumer's
responsibility (the policy engine).

## Acceptance Criteria Met

### Task 1 — schema, loader, verify
- [x] `go test ./internal/catalog/... -run TestCatalogParse -count=1` exits 0
- [x] `loader.go` defines `SupportedSchemaVersion = "0.1.0"` (in `schema.go`); `ValidateSchemaVersion` errors on any other value
- [x] TestUnknownSchemaVersion proves a "0.2.0" file yields a non-nil error
- [x] TestRejectBareArray proves a `[...]` top-level JSON yields a non-nil error
- [x] `VerifySignature` returns false for empty signature, true for non-empty
- [x] `testdata/catalog/nx-console.json` contains `"package": "nrwl.angular-console"` and `"versions": ["18.95.0"]`

### Task 2 — mmap binary index
- [x] `go test ./internal/catalog/... -run TestIndex -count=1` exits 0
- [x] `index.go` imports `github.com/edsrzf/mmap-go` and `encoding/binary`, uses `sort.Search`
- [x] `OpenIndex` validates magic + version, returns error on mismatch (TestOpenIndexRejectsBadMagic)
- [x] TestIndexOpenDoesNotReadJSON passes with source JSON deleted (HOOK-02)
- [x] TestIndexBinarySearchManyEntries builds 250 entries; random hits and misses resolve correctly
- [x] `go.mod` contains `require github.com/edsrzf/mmap-go v1.2.0`

### Task 3 — sync + CLI wiring
- [x] `go build ./...` exits 0; `go vet ./internal/catalog/...` clean
- [x] `sync.go` references `api.github.com/repos/perplexityai/bumblebee/contents/threat_intel` and calls `BuildIndex`
- [x] `Sync` adds `Authorization: Bearer` header when `GITHUB_TOKEN` is set
- [x] `Sync` emits a stderr warning when entries are unsigned (warn-only, CTLG-07)
- [x] `catalogs sync` RunE calls `catalog.Sync` (stub removed)
- [x] `Sync` writes no partial index on error (raw cache + `BuildIndex` only run after all files parse)

### Live smoke test
- [x] `go run ./cmd/beekeeper catalogs sync` exited 0, fetched **654 entries**, emitted the unsigned warning, and wrote `bumblebee.idx` to `%APPDATA%\beekeeper\catalogs\`.

## Requirements Satisfied

- **CTLG-01** — Bumblebee schema parsing with version gating; entries extended with `source_url`/`catalog_signature`/`catalog_source` (defaulted to `bumblebee`).
- **CTLG-05** — Working `beekeeper catalogs sync` that fetches and caches the live catalog (raw `bumblebee.json` + `bumblebee.idx`).
- **CTLG-07** — Unsigned-entry detection via `VerifySignature`; sync warns that unsigned entries are warn-only.
- **HOOK-02** — Memory-mapped binary index with O(log n) lookup and zero JSON parsing on the read path (proven by TestIndexOpenDoesNotReadJSON).

## Threat Mitigations Implemented

- **T-02-01 (catalog poisoning)** — `ValidateSchemaVersion` hard-rejects unknown versions; `VerifySignature` flags unsigned entries (crypto verification deferred to Phase 2, documented).
- **T-02-02 (corrupt .idx)** — `OpenIndex` validates magic + version + region bounds and returns an error rather than reading arbitrary offsets; bounds-checked `recordData`.
- **T-02-04 (GITHUB_TOKEN disclosure)** — token only attached as a request header; never written to the index, raw cache, or logs.
- **T-02-05 (off-host DownloadURL)** — only files enumerated by the pinned `perplexityai/bumblebee` Contents API are fetched.

## Deviations from the Plan

1. **`SupportedSchemaVersion` lives in `schema.go`, not `loader.go`.** The plan
   prose mentions it under both files. It is defined as a package-level constant
   in `schema.go` (alongside the types it gates) and used by `ValidateSchemaVersion`
   in `loader.go`. Net contract is identical; the acceptance criterion referencing
   `loader.go` is satisfied at the package level.

2. **Raw catalog cache is a JSON array of source files.** `Sync` persists the
   concatenated source files to `bumblebee.json` as a JSON array (`[]json.RawMessage`)
   rather than a single merged object, because multiple `threat_intel/*.json`
   files are fetched. The authoritative artifact for the read path is
   `bumblebee.idx`; the raw cache is for provenance/debugging.

3. **Added `TestOpenIndexRejectsBadMagic`** beyond the listed tests to directly
   cover the magic-header validation acceptance criterion (T-02-02). No scope
   expansion — it tests existing behavior.

No scope beyond Plan 02 was implemented: no policy engine, no hook handler, no
audit writer.
