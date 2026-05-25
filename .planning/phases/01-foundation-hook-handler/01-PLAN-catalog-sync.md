---
phase: 01-foundation-hook-handler
plan: 02
type: execute
wave: 2
depends_on: [01]
files_modified:
  - internal/catalog/schema.go
  - internal/catalog/loader.go
  - internal/catalog/loader_test.go
  - internal/catalog/index.go
  - internal/catalog/index_test.go
  - internal/catalog/sync.go
  - internal/catalog/verify.go
  - internal/catalog/verify_test.go
  - cmd/beekeeper/main.go
  - testdata/catalog/nx-console.json
  - testdata/catalog/clean.json
  - go.mod
  - go.sum
autonomous: true
requirements: [CTLG-01, CTLG-05, CTLG-07, HOOK-02]
must_haves:
  truths:
    - "beekeeper catalogs sync fetches Bumblebee threat_intel/ JSON and writes a binary mmap index to the catalog dir"
    - "The Bumblebee schema_version 0.1.0 entries parse into typed Beekeeper-extended entries"
    - "An unknown schema_version is rejected with an error, not silently accepted"
    - "The mmap index supports O(log n) exact lookup by (ecosystem, package) without reading the source JSON"
    - "An entry missing catalog_signature is marked unsigned so the policy engine can treat it warn-only"
  artifacts:
    - path: "internal/catalog/schema.go"
      provides: "Bumblebee catalog JSON types + Beekeeper extensions"
      contains: "schema_version"
    - path: "internal/catalog/loader.go"
      provides: "JSON parse + schema validation"
      exports: ["ParseCatalogFile", "ValidateSchemaVersion"]
    - path: "internal/catalog/index.go"
      provides: "Binary mmap index build (RDWR) and read (RDONLY) with binary-search lookup"
      exports: ["BuildIndex", "OpenIndex", "Index"]
    - path: "internal/catalog/sync.go"
      provides: "HTTP fetch of Bumblebee threat_intel/ + index build orchestration"
      exports: ["Sync"]
    - path: "internal/catalog/verify.go"
      provides: "Catalog signature presence check (Phase 1 warn-only)"
      exports: ["VerifySignature"]
  key_links:
    - from: "internal/catalog/sync.go"
      to: "raw.githubusercontent.com/perplexityai/bumblebee"
      via: "net/http GET of threat_intel directory + files"
      pattern: "githubusercontent|api.github.com"
    - from: "internal/catalog/sync.go"
      to: "internal/catalog/index.go"
      via: "BuildIndex after fetch+parse"
      pattern: "BuildIndex"
    - from: "cmd/beekeeper/main.go"
      to: "internal/catalog.Sync"
      via: "catalogs sync subcommand RunE"
      pattern: "catalog\\.Sync"
---

<objective>
Implement `beekeeper catalogs sync`: fetch the Bumblebee `threat_intel/` catalog over HTTP, parse and validate its schema, extend entries with Beekeeper fields, and build a memory-mappable binary index for O(log n) lookup at hook-evaluation time. This satisfies HOOK-02 (catalog loaded via mmap, never cold JSON parse per check) and CTLG-01/05/07.

Purpose: The hook handler must consult threat intelligence in sub-millisecond time. Parsing 9+ JSON files (~500KB) on every `beekeeper check` invocation would add 50-500ms (RESEARCH anti-pattern). A pre-built sorted binary index solves this. This plan produces the index the policy engine and hook handler depend on.
Output: `internal/catalog` package (schema, loader, index, sync, verify) plus a working `catalogs sync` subcommand and the `catalog.Index` type consumed by the policy engine.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/phases/01-foundation-hook-handler/01-CONTEXT.md
@.planning/phases/01-foundation-hook-handler/01-RESEARCH.md
@CLAUDE.md
@.planning/phases/01-foundation-hook-handler/01-01-SUMMARY.md

<interfaces>
<!-- From plan 01 (already built): -->
```go
// internal/platform
func StateDir() (string, error)
func CatalogDir() (string, error)
```

<!-- Contracts this plan CREATES — the policy engine (plan 04) and hook handler (plan 05) consume Index: -->
```go
// internal/catalog/schema.go
type Entry struct {
    ID               string   `json:"id"`
    Name             string   `json:"name"`
    Ecosystem        string   `json:"ecosystem"`   // npm|pypi|go|rubygems|packagist|cargo|editor-extension
    Package          string   `json:"package"`
    Versions         []string `json:"versions"`
    Severity         string   `json:"severity"`    // critical|high|medium|low
    SourceURL        string   `json:"source_url"`        // Beekeeper extension
    CatalogSignature string   `json:"catalog_signature"` // Beekeeper extension; empty => unsigned
    CatalogSource    string   `json:"catalog_source"`    // Beekeeper extension; "bumblebee"
}
type CatalogFile struct {
    SchemaVersion string  `json:"schema_version"`
    Entries       []Entry `json:"entries"`
}

// internal/catalog/index.go
type Index struct { /* mmap-backed, opaque */ }
// BuildIndex writes a sorted binary index file from entries (RDWR, sync path).
func BuildIndex(path string, entries []Entry) error
// OpenIndex memory-maps an existing index file read-only (check path).
func OpenIndex(path string) (*Index, error)
// Lookup returns the matching entry for (ecosystem, package) or ok=false.
func (idx *Index) Lookup(ecosystem, pkg string) (Entry, bool)
func (idx *Index) Close() error
func (idx *Index) Count() int

// internal/catalog/verify.go
// VerifySignature reports whether an entry carries a non-empty catalog_signature.
// Phase 1: presence-only; cryptographic verification is Phase 2.
func VerifySignature(e Entry) (signed bool)
```
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Catalog schema types, loader, and signature presence check</name>
  <read_first>
    - .planning/phases/01-foundation-hook-handler/01-RESEARCH.md (Bumblebee Catalog Schema VERIFIED section, Open Question 1 on schema stability)
    - .planning/phases/01-foundation-hook-handler/01-CONTEXT.md (Bumblebee Catalog Integration, CTLG-01/CTLG-07 decisions)
  </read_first>
  <files>internal/catalog/schema.go, internal/catalog/loader.go, internal/catalog/loader_test.go, internal/catalog/verify.go, internal/catalog/verify_test.go, testdata/catalog/nx-console.json, testdata/catalog/clean.json</files>
  <behavior>
    - ParseCatalogFile on a valid Bumblebee 0.1.0 file returns the entries with ecosystem/package/versions populated
    - ParseCatalogFile on a file with schema_version "0.2.0" returns a non-nil error (unknown version rejected, not silently accepted)
    - ParseCatalogFile on a bare top-level JSON array returns an error (Bumblebee requires an object with entries[])
    - VerifySignature(entry with empty CatalogSignature) returns false (unsigned)
    - VerifySignature(entry with non-empty CatalogSignature) returns true
    - ParseCatalogFile sets CatalogSource to "bumblebee" when not present in source JSON
  </behavior>
  <action>
    Create internal/catalog/schema.go (package `catalog`) with the `Entry` and `CatalogFile` types exactly as in the interfaces block. Use struct tags matching the verified Bumblebee schema. Define a constant `SupportedSchemaVersion = "0.1.0"`.

    Create internal/catalog/loader.go with `ValidateSchemaVersion(v string) error` (returns error if v != SupportedSchemaVersion, with a message naming both expected and actual) and `ParseCatalogFile(data []byte) (CatalogFile, error)`. ParseCatalogFile must use a `json.Decoder` over the bytes; reject a bare array by decoding into CatalogFile (an array will fail to decode into the struct — return that error wrapped with context). After decode, call ValidateSchemaVersion and return its error if non-nil. For each entry, if CatalogSource is empty, default it to "bumblebee" (Beekeeper records set catalog_source per CTLG-01). Leave SourceURL/CatalogSignature as parsed.

    Create internal/catalog/verify.go with `VerifySignature(e Entry) bool` returning `e.CatalogSignature != ""`. Document with a comment that Phase 1 is presence-only and cryptographic verification (Ed25519/cosign) is Phase 2 (CTLG-07).

    Create testdata/catalog/nx-console.json mirroring the verified Bumblebee Nx Console entry: schema_version "0.1.0", one entry id "stepsecurity-2026-05-18-vscode-nrwl-angular-console-compromised", ecosystem "editor-extension", package "nrwl.angular-console", versions ["18.95.0"], severity "critical". Create testdata/catalog/clean.json with schema_version "0.1.0" and an entries array containing an unrelated npm package (e.g. some-internal-test-pkg) used as an allow case.

    Write loader_test.go (TestCatalogParse, TestUnknownSchemaVersion, TestRejectBareArray, TestDefaultCatalogSource) and verify_test.go (TestUnsignedReturnsFalse, TestSignedReturnsTrue) per the behavior block. Tests read the testdata files via os.ReadFile.
  </action>
  <verify>
    <automated>go test ./internal/catalog/... -run "TestCatalogParse|TestUnknownSchemaVersion|TestRejectBareArray|TestDefaultCatalogSource|TestUnsigned|TestSigned" -count=1 2>&1</automated>
  </verify>
  <acceptance_criteria>
    - `go test ./internal/catalog/... -run TestCatalogParse -count=1` exits 0
    - `internal/catalog/loader.go` defines `SupportedSchemaVersion = "0.1.0"` and `ValidateSchemaVersion` returns an error for any other value
    - TestUnknownSchemaVersion proves a "0.2.0" file yields a non-nil error
    - TestRejectBareArray proves a `[...]` top-level JSON yields a non-nil error
    - `VerifySignature` returns false for empty signature and true for non-empty
    - testdata/catalog/nx-console.json contains `"package": "nrwl.angular-console"` and `"versions": ["18.95.0"]`
  </acceptance_criteria>
  <done>Catalog JSON parses into typed entries with schema-version gating and unsigned-entry detection, all unit-tested against fixtures.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Binary mmap index build and read with binary-search lookup</name>
  <read_first>
    - .planning/phases/01-foundation-hook-handler/01-RESEARCH.md (Pattern 2 mmap Binary Index Format, Pitfall 1 Windows mmap offset alignment, Standard Stack edsrzf/mmap-go)
    - internal/catalog/schema.go (Entry type from Task 1)
    - CLAUDE.md (HOOK-02 mmap constraint)
  </read_first>
  <files>internal/catalog/index.go, internal/catalog/index_test.go, go.mod, go.sum</files>
  <behavior>
    - BuildIndex writes a file; OpenIndex on that file succeeds and Count() equals the number of unique (ecosystem,package) keys built
    - Lookup("editor-extension","nrwl.angular-console") on an index containing that entry returns ok=true and the matching Entry
    - Lookup for a package NOT in the index returns ok=false
    - Lookup is correct across many entries (binary search) — build 100+ sorted entries and assert random hits and misses resolve correctly
    - OpenIndex does NOT read or parse the source catalog JSON (it only mmaps the .idx file) — verified by opening an index whose source JSON has been deleted
  </behavior>
  <action>
    Add dependency `github.com/edsrzf/mmap-go@v1.2.0` via `go get`, then `go mod tidy`.

    Create internal/catalog/index.go implementing the binary index format from RESEARCH Pattern 2: a 16-byte header (4-byte magic 0x42454549 "BEEI", 4-byte uint32 version=1, 4-byte uint32 record count, 4-byte reserved 0), followed by N fixed-width 48-byte records sorted ascending by Key, followed by a data section of concatenated entry JSON blobs. Each record: 32-byte Key = `sha256(ecosystem + "::" + strings.ToLower(package))[:32]`, 8-byte uint64 little-endian DataOffset (relative to start of data section), 8-byte uint64 little-endian DataLength. Use `encoding/binary` with `binary.LittleEndian`.

    `BuildIndex(path string, entries []Entry) error`: compute the key for each entry, deduplicate by key (last wins), sort records ascending by Key bytes (`bytes.Compare`), marshal each entry to JSON for the data section, write header+records+data to a temp file then rename into place (atomic). Use os.WriteFile or an os.Create with explicit Close. Always mmap/write the whole file (offset 0) to avoid the Windows allocation-granularity alignment pitfall.

    `OpenIndex(path string) (*Index, error)`: open the file read-only, `mmap.Map(f, mmap.RDONLY, 0)`, validate magic and version (return error on mismatch — fail-closed friendly), parse record count from header. Store the mmap slice, record count, and computed offsets to the records region and data region on the Index struct. Do NOT read any JSON file.

    `(idx *Index) Lookup(ecosystem, pkg string) (Entry, bool)`: compute the key the same way as build, use `sort.Search` over `[0,count)` comparing record keys via `bytes.Compare`, on exact key match read the data slice at `dataRegion[off:off+len]` and `json.Unmarshal` into an Entry. Return ok=false on no match.

    `(idx *Index) Close() error` unmaps; `(idx *Index) Count() int` returns the record count.

    Write index_test.go (TestIndexBuildAndOpen, TestIndexLookupHit, TestIndexLookupMiss, TestIndexBinarySearchManyEntries, TestIndexOpenDoesNotReadJSON) per the behavior block. TestIndexOpenDoesNotReadJSON builds an index in a temp dir, deletes any JSON, then OpenIndex+Lookup must still work.
  </action>
  <verify>
    <automated>go test ./internal/catalog/... -run "TestIndex" -count=1 2>&1</automated>
  </verify>
  <acceptance_criteria>
    - `go test ./internal/catalog/... -run TestIndex -count=1` exits 0
    - `internal/catalog/index.go` imports `github.com/edsrzf/mmap-go` and `encoding/binary` and uses `sort.Search`
    - OpenIndex validates the magic header and returns an error on a corrupt/wrong-magic file
    - TestIndexOpenDoesNotReadJSON passes with the source JSON deleted (proves HOOK-02: no cold JSON parse on the read path)
    - TestIndexBinarySearchManyEntries builds at least 100 entries and resolves both hits and misses correctly
    - `go.mod` contains `require github.com/edsrzf/mmap-go v1.2.0`
  </acceptance_criteria>
  <done>A sorted binary mmap index can be built on sync and read with O(log n) lookup at check time, with no JSON parsing on the read path.</done>
</task>

<task type="auto">
  <name>Task 3: HTTP catalog fetch + catalogs sync subcommand wiring</name>
  <read_first>
    - .planning/phases/01-foundation-hook-handler/01-RESEARCH.md (Pattern 5 Bumblebee Catalog Fetch, Pitfall 6 GitHub rate limit, Claude's Discretion on HTTP client config)
    - internal/catalog/loader.go and internal/catalog/index.go (Tasks 1-2)
    - cmd/beekeeper/main.go (catalogs sync stub from plan 01)
  </read_first>
  <files>internal/catalog/sync.go, cmd/beekeeper/main.go</files>
  <action>
    Create internal/catalog/sync.go with `Sync(ctx context.Context, client *http.Client, catalogDir string) (int, error)` returning the number of entries indexed. Implementation: GET `https://api.github.com/repos/perplexityai/bumblebee/contents/threat_intel` with header `Accept: application/vnd.github+json` to list files; decode into a slice of `{Name, DownloadURL, Type string}`; filter to `Type == "file"` and `.json` suffix. For each, GET its DownloadURL, ParseCatalogFile the body, and accumulate entries. Persist the raw concatenated JSON to `filepath.Join(catalogDir, "bumblebee.json")` and call `BuildIndex(filepath.Join(catalogDir, "bumblebee.idx"), allEntries)`. For each parsed entry, if `!VerifySignature(entry)` increment an unsigned counter and, when >0, print a warning line to stderr noting unsigned entries are warn-only (CTLG-07). If `GITHUB_TOKEN` is set in the environment, add it as an `Authorization: Bearer` header to raise the rate limit (Pitfall 6). Use a client with a sane timeout (e.g. 30s) per Claude's Discretion. On any HTTP/parse failure, return the error (sync is user-triggered; failing loudly is correct — do NOT write a partial index).

    Wire the `catalogs sync` subcommand RunE in main.go to call `platform.CatalogDir()` (MkdirAll it), construct an `http.Client`, call `catalog.Sync(cmd.Context(), client, dir)`, and print the entry count and index path on success. Replace the not-implemented stub from plan 01.
  </action>
  <verify>
    <automated>go build ./... 2>&1 && go vet ./internal/catalog/... 2>&1</automated>
  </verify>
  <acceptance_criteria>
    - `go build ./...` exits 0
    - `internal/catalog/sync.go` references `api.github.com/repos/perplexityai/bumblebee/contents/threat_intel` and calls `BuildIndex`
    - Sync adds an `Authorization: Bearer` header when `GITHUB_TOKEN` is set (grep confirms `GITHUB_TOKEN` and `Authorization`)
    - Sync emits a stderr warning when any entry is unsigned (grep confirms a warn message referencing unsigned)
    - `cmd/beekeeper/main.go` `catalogs sync` RunE calls `catalog.Sync` (no longer returns "not yet implemented")
    - Sync does not write a partial index on error (BuildIndex is only called after all files parsed successfully)
  </acceptance_criteria>
  <done>`beekeeper catalogs sync` fetches the live Bumblebee catalog, builds the mmap index, and warns on unsigned entries.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| network → process | Bumblebee catalog JSON fetched over HTTPS from GitHub is untrusted-but-relied-upon threat intel |
| catalog file → mmap index | The .idx file is read via mmap on every check; a corrupt index must not crash the handler |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-02-01 | Tampering | Catalog poisoning via malicious threat_intel/ content | mitigate | `schema_version` validation rejects unexpected versions; `VerifySignature` marks unsigned entries so the policy engine treats them warn-only (CTLG-07); cryptographic verification deferred to Phase 2 (documented) |
| T-02-02 | Tampering | Corrupt or attacker-supplied .idx file | mitigate | OpenIndex validates magic (0x42454549) and version header, returning an error (consumed fail-closed by the hook handler in plan 05) rather than reading arbitrary offsets |
| T-02-03 | Denial of Service | Oversized catalog response causing memory blowup at sync time | accept | Sync is a user-triggered, non-hot-path operation; full parse acceptable; catalog delta sanity bounds are CTLG-08 (Phase 2, out of scope) |
| T-02-04 | Information Disclosure | GITHUB_TOKEN read from env | mitigate | Token only added as a request header to api.github.com; never written to the index, the raw catalog cache, or logs |
| T-02-05 | Spoofing | DownloadURL pointing off-host | mitigate | Only files listed by the pinned perplexityai/bumblebee contents API are fetched; HTTPS enforced (https URLs from GitHub) |
</threat_model>

<verification>
- `go test ./internal/catalog/... -count=1` exits 0 (loader, index, verify tests)
- `go build ./...` exits 0
- TestIndexOpenDoesNotReadJSON confirms the mmap read path never parses JSON (HOOK-02)
- Manual smoke (CI / dev): `beekeeper catalogs sync` produces `bumblebee.idx` in the catalog dir
</verification>

<success_criteria>
- Bumblebee schema parsing with version gating (CTLG-01)
- Working `beekeeper catalogs sync` that fetches and caches the live catalog (CTLG-05)
- Unsigned-entry detection enabling warn-only treatment (CTLG-07)
- Memory-mapped binary index with O(log n) lookup and no JSON parse on the read path (HOOK-02)
</success_criteria>

<output>
After completion, create `.planning/phases/01-foundation-hook-handler/01-02-SUMMARY.md`
</output>
