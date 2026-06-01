# Phase 1: Foundation + Hook Handler - Research

**Researched:** 2026-05-26
**Domain:** Go CLI binary, mmap catalog index, Bumblebee threat intel schema, Sigstore/cosign v3, reproducible builds, Windows ACL, cross-platform CI
**Confidence:** HIGH (most topics verified against official docs or live source files)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- Single static binary, `cmd/beekeeper/main.go` is thin Cobra wiring only — no business logic in cmd/
- All business logic in `internal/` packages; no CGO in core binary; Go 1.25+
- Build flags: `-trimpath -buildvcs=false -mod=readonly` for reproducible builds
- `make verify-release VERSION=X.Y.Z` target reproduces and compares hashes
- Hook handler reads tool call JSON from stdin; sub-100ms p95; exit 0 = allow, exit non-zero = block
- Catalog loaded via mmap (pre-built by `beekeeper catalogs sync`); NEVER cold-loads JSON per invocation
- Fail closed by default: crash or timeout → block; `fail_open`/`fail_warn` are explicit opt-ins
- Hard caps: stdin 1MB, execution time 5s, memory cap
- `internal/policy` must be a pure function library — no I/O, no goroutines, no side effects
- Phase 1 policy engine: Bumblebee single-source matching only; corroboration semantics (PLCY-01) are Phase 2
- Phase 1 matching: single source → warn semantics only (no block enforcement from catalog alone in Phase 1)
- `beekeeper catalogs sync` fetches Bumblebee `threat_intel/` catalog, builds mmap index, caches to `~/.beekeeper/catalogs/`
- Windows: `%APPDATA%\beekeeper\` instead of `~/.beekeeper/`
- NDJSON audit log: every policy decision, `0600` Unix permissions from first write; equivalent Windows ACL
- Reproducible builds (SFDF-01): `make verify-release` MUST land in Phase 1
- Sigstore signing (SFDF-02): GitHub Actions OIDC, cosign v3 `--bundle artifact.sigstore.json`, GoReleaser v2.13.0+; MUST land in Phase 1
- Pinned dependencies (SFDF-03): `go.mod` + `go.sum` + CI `go mod verify`; Renovate-bot MUST land in Phase 1
- `SECURITY.md` (SFDF-04): 48h acknowledgment, 90-day coordinated disclosure; MUST land in Phase 1
- GitHub Actions matrix: `ubuntu-latest`, `macos-latest` (Apple Silicon + Intel where possible), `windows-latest`
- Library choices (locked): Cobra for CLI, `golang.org/x/sys` for platform ops, `encoding/json` for catalog parsing, GoReleaser for release pipeline, cosign v3 for signing
- Bubble Tea v2 import: `charm.land/bubbletea/v2` — NOT `github.com/charmbracelet` (Phase 8, not Phase 1)
- CLI subcommands in Phase 1: `beekeeper check`, `beekeeper catalogs sync`, `beekeeper audit tail`, `beekeeper version`, `beekeeper selftest`, `beekeeper init` (stub)

### Claude's Discretion

- Exact mmap index binary format (flat sorted slice vs. prefix tree — choose based on lookup performance profile)
- Config file schema details beyond what the PRD specifies
- Specific HTTP client configuration for Bumblebee catalog fetch (timeouts, retries, caching headers)
- Log rotation policy for Phase 1 (can be deferred to Phase 6)
- Error message formatting and exit code conventions beyond "0=allow, non-zero=block"
- Internal package naming conventions
- Whether to use `make` or `task` as the build tool (prefer `make` for portability)

### Deferred Ideas (OUT OF SCOPE)

- `beekeeper hooks install` (INTG-01) — Phase 4; Phase 1 may have CLI stub only
- Release-age policy (PLCY-02) — Phase 2
- OSV and Socket catalog sources (CTLG-02, CTLG-03) — Phase 2
- Corroboration-based block/warn multi-source semantics (PLCY-01) — Phase 2
- Full audit sinks: syslog, OTLP, HTTPS POST (AUDT-03/04) — Phase 6
- Log rotation — Phase 6
- `beekeeper catalogs watch` daemon (CTLG-06) — Phase 2
- SLSA Level 3 provenance (SFDF-05) — Phase 7
- Sentry daemon — Phase 5+; LlamaFirewall — Phase 6; TUI dashboard — Phase 8; Policy as code — Phase 9
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| HOOK-01 | `beekeeper check` reads tool call JSON from stdin, evaluates policy engine, exits 0 (allow) or non-zero (block) with structured reason — sub-100ms p95 | Cobra lazy init pattern; Go binary cold start 10–30ms; mmap load sub-millisecond; policy eval microseconds |
| HOOK-02 | Hook handler loads catalog index via mmap (pre-built by `catalogs sync`), not cold-loading from JSON per invocation | `edsrzf/mmap-go` v1.2.0 or `golang.org/x/exp/mmap` — both cross-platform; binary index format designed as flat sorted array with binary search |
| HOOK-03 | Hook handler fails closed by default (crash or timeout → block); `fail_open` and `fail_warn` modes configurable | Verified via process design: panic/recover in check; context.WithTimeout; structured config field with "reduces security" annotation |
| HOOK-04 | Hard caps on stdin size (1MB), execution time (5s), memory enforced | `io.LimitReader` for stdin; `context.WithTimeout` for execution; runtime.MemStats polling or OS-level RLIMIT |
| CTLG-01 | Bumblebee `threat_intel/` catalog sync — extended with `source_url`, `catalog_signature`, `catalog_source` fields; `scanner_name: "beekeeper"` | Bumblebee catalog schema verified: `schema_version: "0.1.0"`, `entries[]` with id/name/ecosystem/package/versions/severity; extended fields are Beekeeper additions |
| CTLG-05 | `beekeeper catalogs sync` — fetch and cache all enabled catalog sources | GitHub API or raw HTTP download of threat_intel/ directory; cached to `~/.beekeeper/catalogs/` |
| CTLG-07 | Catalog signature verification — unsigned sources treated as warning-only regardless of corroboration count | Phase 1: Bumblebee does not currently sign catalogs; design for signature field in extended schema; treat unsigned as warn-only |
| SFDF-01 | Reproducible builds from v0.1.0 — deterministic Go build flags, `make verify-release VERSION=X.Y.Z` | GoReleaser `mod_timestamp: "{{ .CommitTimestamp }}"` + `-trimpath` + commit-date ldflags; verified via GoReleaser reproducible builds blog |
| SFDF-02 | Sigstore signing from v0.1.0 — GitHub Actions OIDC, cosign v3 `--bundle artifact.sigstore.json`, GoReleaser v2.13.0+ | Verified via goreleaser-action v7.2.2, cosign-installer@v3; exact YAML pattern documented below |
| SFDF-03 | Pinned dependencies — `go.mod` and `go.sum` with CI `go mod verify`; Renovate-bot | `go mod verify` CI step; Renovate `renovate.json` with `gomod` manager |
| SFDF-04 | `SECURITY.md` — responsible disclosure, 48h acknowledgment SLA, 90-day coordinated disclosure | Standard OSS CVD template; GitHub private security advisory as intake mechanism |
</phase_requirements>

---

## Summary

Phase 1 delivers the minimal viable Beekeeper binary: a working `beekeeper check` hook handler that evaluates tool calls against the Bumblebee `threat_intel/` catalog and produces allow/block decisions with structured NDJSON audit records in under 100ms p95. The hook handler loads a pre-built mmap binary index to avoid JSON parse cost on every invocation. Every release binary is reproducibly buildable and Sigstore-signed from the first commit.

The primary technical complexity concentrates in three areas: (1) the mmap binary index format — must be writable on catalog sync and read-only at hook evaluation time, cross-platform on Windows/Linux/macOS; (2) Windows file permissions for the NDJSON audit log — `0600` semantics require explicit DACL manipulation via `hectane/go-acl` since Go's `os.Chmod` maps `0600` only partially on Windows; (3) the GoReleaser/cosign v3 release pipeline, which requires specific permission grants in the GitHub Actions workflow and the `--bundle ${artifact}.sigstore.json` flag combination.

The Bumblebee catalog schema is now fully verified from the live repository: `schema_version: "0.1.0"`, top-level `entries[]` array, each entry with `id`, `name`, `ecosystem`, `package`, `versions`, `severity`. Beekeeper extends entries with `source_url`, `catalog_signature`, `catalog_source`, `scanner_name`. The mmap index is a Beekeeper-specific construct — Bumblebee itself has no binary index; it parses JSON at scan time.

**Primary recommendation:** Use `edsrzf/mmap-go` v1.2.0 for the mmap layer (cross-platform, read/write on sync, read-only on check). Design the binary index as a flat sorted array of fixed-width records (`[32]byte ecosystem+package hash | [16]byte offset | [8]byte length`) enabling `sort.Search` binary search at O(log n) without any external dependency. Use `encoding/binary` with `binary.LittleEndian` for index serialization.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Tool call interception / policy decision | CLI (hook handler) | — | `beekeeper check` is a single-shot subprocess; no daemon in Phase 1 |
| Catalog data fetch + signature verification | CLI (catalogs sync) | — | One-time sync operation builds the mmap index |
| mmap index read | CLI (check) | — | Index loaded read-only on each `beekeeper check` invocation |
| Policy rule evaluation | `internal/policy` (pure lib) | Called by check, future: gateway | Pure function library; no I/O; shared across all consumers |
| NDJSON audit log write | CLI (check) | — | Append-only, file sink only in Phase 1 |
| Release pipeline / signing | GitHub Actions | GoReleaser | CI-only; no local signing keys |
| State directory management | CLI (init/any command) | OS (XDG / APPDATA) | `os.UserConfigDir()` on Windows → `%APPDATA%`, `~/.beekeeper` on Unix |

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/spf13/cobra` | v1.10.2 | CLI subcommand routing | Industry standard for Go CLI tools; lazy init pattern minimizes startup overhead |
| `github.com/edsrzf/mmap-go` | v1.2.0 | Cross-platform memory-mapped files | Tested on Linux/macOS/Windows; uses `CreateFileMapping`+`MapViewOfFile` on Windows; BSD-3 license |
| `github.com/hectane/go-acl` | v0.0.0-20230122... | Windows DACL for `0600`-equivalent audit log permissions | Only clean Go library that wraps Windows ACL API with `acl.Chmod(path, 0600)` interface |
| `golang.org/x/sys` | latest | Platform-specific syscalls (mmap fallback, file ops) | Official Go extended stdlib; already an indirect dependency of many packages |
| `encoding/json` | stdlib | Catalog JSON parsing, NDJSON serialization | Sufficient for catalog parse at sync time; not on hot path |
| `encoding/binary` | stdlib | Binary index serialization/deserialization | Fixed-width records for mmap index format |
| `sort` | stdlib | Binary search over sorted index records | `sort.Search` for O(log n) lookup without external deps |
| `context` | stdlib | Timeout enforcement for hook handler 5s hard cap | `context.WithTimeout` + `context.WithDeadline` |
| `io` | stdlib | `io.LimitReader` for 1MB stdin cap | Standard pattern for bounded I/O |

[VERIFIED: pkg.go.dev for all versions listed above]

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `golang.org/x/exp/mmap` | latest | Alternative read-only mmap (ReaderAt interface) | If `edsrzf/mmap-go` causes issues; only supports read, not write during index build |
| `github.com/google/go-github/v68` | latest | GitHub API for catalog discovery | If direct raw HTTP proves fragile for threat_intel/ directory listing; adds dependency |
| `net/http` | stdlib | Direct HTTP fetch of Bumblebee catalog | Preferred for catalog download — avoids `go-github` dependency for simple raw file fetch |

[VERIFIED: pkg.go.dev]

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `edsrzf/mmap-go` | `golang.org/x/exp/mmap` | `x/exp/mmap` is read-only (ReaderAt only); can't write the index during sync; `edsrzf/mmap-go` supports RDWR on sync, RDONLY on check |
| `edsrzf/mmap-go` | `golang.org/x/sys` direct | Verbose platform-specific code (unix.Mmap + windows.CreateFileMapping separately); `edsrzf/mmap-go` abstracts this |
| `hectane/go-acl` | `golang.org/x/sys/windows` direct | Direct DACL manipulation is ~100 lines of Windows security API; `go-acl.Chmod(path, 0600)` is one line |
| Flat sorted array index | Prefix tree / trie | Prefix tree has faster prefix queries but higher complexity; flat sorted array + binary search is ~50 lines, zero deps, correct for exact (ecosystem, package, version) match |
| `encoding/json` | `github.com/json-iterator/go` | `json-iterator` is faster for hot paths; catalog parse is only at sync time, not check time — stdlib is sufficient |

**Installation:**
```bash
go get github.com/spf13/cobra@v1.10.2
go get github.com/edsrzf/mmap-go@v1.2.0
go get github.com/hectane/go-acl
go get golang.org/x/sys@latest
```

**Version verification:** Verified against pkg.go.dev on 2026-05-26.

---

## Architecture Patterns

### System Architecture Diagram

```
                    CATALOG SYNC PATH
                    ─────────────────
Developer runs: beekeeper catalogs sync
        │
        ▼
[net/http] → raw.githubusercontent.com (Bumblebee threat_intel/*.json)
        │
        ▼
[encoding/json] → Parse JSON catalog files
        │         schema_version + entries[]
        ▼
[internal/catalog] → Validate schema_version == "0.1.0"
        │             Extend entries with source_url, catalog_source
        │             Check catalog_signature (Phase 1: log warning if absent)
        ▼
[internal/catalog] → Build binary mmap index
        │             Sorted flat array of fixed-width records
        │             Keyed by hash(ecosystem||"::"||package) → offset+length
        ▼
[edsrzf/mmap-go RDWR] → Write index to ~/.beekeeper/catalogs/bumblebee.idx
        │                 Persist raw JSON to ~/.beekeeper/catalogs/bumblebee.json
        ▼
      DONE (index ready for check invocations)

                    HOOK HANDLER PATH (per agent tool call)
                    ─────────────────────────────────────────
Agent tool call triggers: beekeeper check (fresh subprocess)
        │
        ▼
[io.LimitReader(os.Stdin, 1MB)] → Read tool call JSON
        │
        ▼
[context.WithTimeout(5s)] → Enforce execution deadline
        │
        ▼
[internal/config] → Load ~/.beekeeper/config.json (fail_open / fail_warn settings)
        │
        ▼
[internal/catalog] → Open mmap index (RDONLY)
        │             [edsrzf/mmap-go RDONLY]
        │
        ▼
[internal/policy] → Pure function: Evaluate(toolCall, catalogIndex) → Decision
        │             1. Parse tool_name + tool_input from JSON
        │             2. Identify ecosystem + package + version from input
        │             3. Binary search index for (ecosystem, package) key
        │             4. If match found → single-source warn (Phase 1)
        │             5. Severity from catalog entry
        │
        ▼
[internal/audit] → Write NDJSON record to ~/.beekeeper/audit/beekeeper.ndjson
        │           0600 Unix permissions; hectane/go-acl on Windows
        ▼
[os.Stdout] → Print structured JSON reason
        │
        ▼
os.Exit(0) → allow   OR   os.Exit(1) → block
        │
    PANIC/RECOVER → any unhandled panic → os.Exit(2) → block (fail-closed)
```

### Recommended Project Structure

```
cmd/
  beekeeper/
    main.go              # Cobra root command + subcommand registration only
internal/
  config/
    config.go            # Layered config loader (user-level only in Phase 1)
    schema.go            # Config JSON schema types
  catalog/
    schema.go            # Bumblebee catalog JSON types + Beekeeper extensions
    loader.go            # JSON parse + validation from downloaded files
    index.go             # Binary mmap index build (sync path, RDWR) and read (check path, RDONLY)
    sync.go              # HTTP fetch from Bumblebee GitHub raw URLs
    verify.go            # Catalog signature verification (stub for Phase 1 — log warn if absent)
  policy/
    engine.go            # Pure function: Evaluate(input ToolCall, idx *catalog.Index) Decision
    types.go             # ToolCall, Decision, Reason types
  check/
    handler.go           # Hook handler entry point: stdin read, timeout, fail-closed wrapper
  audit/
    writer.go            # NDJSON append-only writer, permission enforcement
    types.go             # AuditRecord type
  platform/
    dirs.go              # StateDir() — cross-platform ~/.beekeeper / %APPDATA%\beekeeper
    perms.go             # SetOwnerOnly(path) — 0600 Unix / DACL Windows
Makefile                 # build, test, verify-release targets
.goreleaser.yaml         # Multi-platform builds + cosign v3 signing
.github/
  workflows/
    ci.yml               # PR: test matrix (ubuntu, macos, windows) + go mod verify
    release.yml          # Tag push: goreleaser + cosign + OIDC
  renovate.json          # Renovate bot config for go.mod and actions
SECURITY.md              # Responsible disclosure
```

### Pattern 1: Fail-Closed Hook Handler

**What:** Every exit path in `beekeeper check` either produces an explicit allow/block decision or the catch-all panic/recover produces a block. No silent allows.

**When to use:** All of `internal/check/handler.go`.

**Example:**
```go
// Source: CONTEXT.md architectural decision + PRD §12.4
func RunCheck(ctx context.Context, stdin io.Reader) (decision Decision, err error) {
    defer func() {
        if r := recover(); r != nil {
            // Fail closed: any panic → block
            decision = Decision{Allow: false, Reason: "internal error (fail-closed)"}
            err = fmt.Errorf("recovered panic: %v", r)
        }
    }()

    // Hard stdin cap
    limited := io.LimitReader(stdin, 1<<20) // 1MB

    // Enforce 5s execution deadline
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    var toolCall policy.ToolCall
    if err := json.NewDecoder(limited).Decode(&toolCall); err != nil {
        return Decision{Allow: false, Reason: "invalid tool call JSON (fail-closed)"}, nil
    }

    idx, err := catalog.OpenIndex(statedir.CatalogPath("bumblebee"))
    if err != nil {
        return Decision{Allow: false, Reason: "catalog unavailable (fail-closed)"}, nil
    }
    defer idx.Close()

    return policy.Evaluate(ctx, toolCall, idx), nil
}
```
[ASSUMED — pattern derived from PRD §12.4 and CONTEXT.md decisions; not from official Go docs]

### Pattern 2: mmap Binary Index Format

**What:** Flat sorted array of fixed-width index records enabling O(log n) binary search without external index libraries.

**When to use:** `internal/catalog/index.go` — build during `catalogs sync`, read during `beekeeper check`.

**Format:**

```
Index file layout:
  [4 bytes]  Magic: 0x42454549 ("BEEI")
  [4 bytes]  Version: uint32 little-endian (1)
  [4 bytes]  RecordCount: uint32 little-endian
  [4 bytes]  Reserved: 0x00000000
  [N * 48 bytes]  Sorted records:
    [32 bytes]  Key: SHA-256(ecosystem + "::" + package_normalized)[:32]
    [8 bytes]   DataOffset: uint64 little-endian (offset into data section)
    [8 bytes]   DataLength: uint64 little-endian
  [M bytes]   Data section: concatenated entry JSON blobs
```

**Lookup:**
```go
// Source: [ASSUMED] — design based on standard binary search patterns in Go
func (idx *Index) Lookup(ecosystem, pkg string) (*Entry, bool) {
    key := indexKey(ecosystem, pkg)
    // sort.Search over mmap'd record array
    n := idx.recordCount
    i := sort.Search(int(n), func(i int) bool {
        rec := idx.record(i)
        return bytes.Compare(rec.Key[:], key[:]) >= 0
    })
    if i >= int(n) {
        return nil, false
    }
    rec := idx.record(i)
    if rec.Key != key {
        return nil, false
    }
    // Read entry JSON from data section
    data := idx.data[rec.DataOffset : rec.DataOffset+rec.DataLength]
    var entry Entry
    json.Unmarshal(data, &entry)
    return &entry, true
}
```
[ASSUMED — specific format design]

### Pattern 3: Cross-Platform State Directory

**What:** `os.UserConfigDir()` returns `%APPDATA%` on Windows, `$XDG_CONFIG_HOME` or `$HOME/.config` on Linux, `$HOME/Library/Preferences` on macOS. For Beekeeper, the PRD uses `~/.beekeeper/` on Unix and `%APPDATA%\beekeeper\` on Windows.

**Decision:** Use `os.UserConfigDir()` as the base on Windows (returns `%APPDATA%`); on Unix, use `os.UserHomeDir()` (returns `~`) for the `~/.beekeeper` convention rather than `os.UserConfigDir()` which would give `~/.config/beekeeper/` — this matches the PRD's stated paths.

```go
// Source: [VERIFIED: go.dev/pkg/os] + [CITED: CONTEXT.md §Beekeeper State Directory]
func StateDir() (string, error) {
    if runtime.GOOS == "windows" {
        base, err := os.UserConfigDir() // returns %APPDATA%
        if err != nil {
            return "", err
        }
        return filepath.Join(base, "beekeeper"), nil
    }
    home, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }
    return filepath.Join(home, ".beekeeper"), nil
}
```
[VERIFIED: Go stdlib docs for `os.UserConfigDir`]

### Pattern 4: Windows 0600-equivalent Audit Log Permissions

**What:** `os.Chmod(path, 0600)` on Windows only sets the `0200` bit (writable) — does not restrict read access to owner only. Use `hectane/go-acl` to set a proper DACL.

```go
// Source: [VERIFIED: hectane/go-acl README] + [CITED: Medium/@MichalPristas - Go file perms on Windows]
//go:build windows

import "github.com/hectane/go-acl"

func SetOwnerOnly(path string) error {
    return acl.Chmod(path, 0600)
}

// Unix implementation in perms_unix.go:
//go:build !windows

func SetOwnerOnly(path string) error {
    return os.Chmod(path, 0600)
}
```

### Pattern 5: Bumblebee Catalog Fetch

**What:** Direct HTTP GET to GitHub raw content URLs; no API key required for public repos within rate limits.

**URL pattern:**
```
https://raw.githubusercontent.com/perplexityai/bumblebee/main/threat_intel/{filename}.json
```

**Directory listing:** The GitHub Contents API at `https://api.github.com/repos/perplexityai/bumblebee/contents/threat_intel` returns JSON with `name` and `download_url` for each file. This allows discovery of new catalog files without hardcoding filenames.

```go
// Source: [VERIFIED: GitHub REST API docs] + [VERIFIED: live Bumblebee repo structure]
type ghContentItem struct {
    Name        string `json:"name"`
    DownloadURL string `json:"download_url"`
    Type        string `json:"type"`
}

func FetchThreatIntelCatalog(ctx context.Context, client *http.Client) ([]CatalogFile, error) {
    req, _ := http.NewRequestWithContext(ctx, "GET",
        "https://api.github.com/repos/perplexityai/bumblebee/contents/threat_intel", nil)
    req.Header.Set("Accept", "application/vnd.github+json")
    // ... rest of fetch + filter for .json files
}
```

### Anti-Patterns to Avoid

- **Anti-pattern: Cold JSON parse on every `beekeeper check`.** Loading `~/.beekeeper/catalogs/*.json` on every hook invocation adds 50-500ms for a realistic catalog (9+ files, ~500KB). The mmap binary index avoids this completely. [CITED: CONTEXT.md HOOK-02]

- **Anti-pattern: `fail_open` as default.** Any crash, timeout, or I/O error in `beekeeper check` must produce a block, not a silent allow. Always wrap the check handler entry point with `defer recover()` that maps to block. [CITED: CONTEXT.md Fail-Closed Architecture]

- **Anti-pattern: Goroutines or I/O in `internal/policy`.** The policy engine is a pure function library for future reuse in the MCP gateway and Sentry correlation. Any goroutine or file I/O in `internal/policy` breaks that contract. [CITED: CONTEXT.md Policy Engine Shape]

- **Anti-pattern: Embedding build timestamp via `ldflags -X main.buildTime=$(date)`.**  This embeds a different timestamp on every build, breaking reproducibility. Use `mod_timestamp: "{{ .CommitTimestamp }}"` in GoReleaser and inject only commit date. [VERIFIED: goreleaser.com/blog/reproducible-builds]

- **Anti-pattern: Using `os.UserConfigDir()` for Unix `~/.beekeeper`.** On Linux, `os.UserConfigDir()` returns `~/.config`, so the path would be `~/.config/beekeeper` — not `~/.beekeeper` as specified in the PRD. Use `os.UserHomeDir()` + `.beekeeper` on Unix. [VERIFIED: Go stdlib docs]

- **Anti-pattern: Using `github.com/charmbracelet` import for Bubble Tea.** Phase 1 does not use Bubble Tea, but any future `go.mod` addition must use `charm.land/bubbletea/v2`. [CITED: CLAUDE.md]

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cross-platform mmap | Custom CreateFileMapping/MapViewOfFile wrapping | `edsrzf/mmap-go` v1.2.0 | Windows mmap is a 2-step API + offset alignment; platform bugs are subtle |
| Windows file owner-only permissions | Direct DACL/SECURITY_DESCRIPTOR manipulation | `hectane/go-acl` | ~200 lines of Windows security API vs. `acl.Chmod(path, 0600)` |
| Multi-platform CLI routing | Hand-rolled argument parser | `spf13/cobra` v1.10.2 | Persistent flags, subcommand help, PersistentPreRun hooks for common init |
| Release pipeline + signing | Manual `cosign sign` + `gh release upload` scripts | GoReleaser v2.15.x + goreleaser-action@v7 | Handles cross-compilation, checksums, SBOM, artifact upload atomically |
| Go binary startup overhead measurement | Synthetic benchmarks | Rely on `time beekeeper check < fixture.json` in CI | PRD establishes 10–30ms as expected cold start; mmap adds sub-1ms |

**Key insight:** The binary index format IS hand-rolled (flat sorted array + `sort.Search`) because there is no standard Go library for immutable mmap-loadable search indexes. Everything else around it (mmap, permissions, signing) uses battle-tested libraries.

---

## Bumblebee Catalog Schema (VERIFIED)

This is the authoritative schema verified from the live Bumblebee repository on 2026-05-26.

**File format:** JSON object with two required top-level keys. Bare top-level arrays are rejected by Bumblebee.

```json
{
  "schema_version": "0.1.0",
  "_comment": "Optional human-readable comment field",
  "_indicators": {
    // Optional incident metadata (not consumed by scanner matching)
    "source": "...",
    "corroborating_source": "...",
    "exposure_window_published_utc": "...",
    "network_indicators": [],
    "capabilities": "..."
  },
  "entries": [
    {
      "id": "stepsecurity-2026-05-18-vscode-nrwl-angular-console-compromised",
      "name": "nrwl.angular-console (Nx Console VS Code extension 2026-05-18 compromise)",
      "ecosystem": "editor-extension",
      "package": "nrwl.angular-console",
      "versions": ["18.95.0"],
      "severity": "critical",
      "source": "https://...",
      "indicators": {
        // Optional per-entry forensic indicators (not consumed by scanner matching)
      }
    }
  ]
}
```

**Known catalog files (verified 2026-05-26):**
1. `nx-console-vscode-2026-05-18.json` — VS Code extension, `ecosystem: "editor-extension"`, `package: "nrwl.angular-console"`, `versions: ["18.95.0"]`, `severity: "critical"`
2. `mini-shai-hulud.json` — 170+ npm/PyPI packages, `severity: "critical"`
3. `antv-mini-shai-hulud.json` — npm worm wave
4. `laravel-lang-2026-05-23.json` — Packagist/Composer
5. `node-ipc-credential-stealer.json` — npm
6. `shopsprint-decimal-typosquat.json` — Go module typosquat
7. `gemstuffer.json` — RubyGems
8. `trapdoor-crypto-stealer.json` — npm/PyPI/Cargo

**Ecosystems in use:** `npm`, `pypi`, `go`, `rubygems`, `packagist`, `cargo`, `editor-extension`

**Normalized package name convention:** For `editor-extension`, Bumblebee normalizes as `publisher.name` lowercased (e.g., `nrwl.angular-console`). Match must be exact.

**Beekeeper-extended entry schema:**
```json
{
  "id": "...",
  "name": "...",
  "ecosystem": "npm | pypi | go | rubygems | packagist | cargo | editor-extension",
  "package": "...",
  "versions": ["..."],
  "severity": "critical | high | medium | low",
  "source_url": "https://...",
  "catalog_signature": "...",
  "catalog_source": "bumblebee"
}
```
[VERIFIED: raw.githubusercontent.com/perplexityai/bumblebee/main/threat_intel/nx-console-vscode-2026-05-18.json]

---

## Common Pitfalls

### Pitfall 1: Windows mmap Offset Alignment

**What goes wrong:** `MapViewOfFile` on Windows requires the offset parameter to be aligned to the system allocation granularity (typically 65536 bytes), not just the page size (4096 bytes). Hand-rolled code using page-size alignment crashes silently.

**Why it happens:** Unix `mmap` only requires page-size alignment; Windows requires allocation granularity alignment. Documentation buries this.

**How to avoid:** Use `edsrzf/mmap-go` which handles alignment internally. For the binary index, always mmap the entire file (offset 0) rather than a region — eliminates alignment concerns entirely.

**Warning signs:** Access violation on Windows with code that works on Linux; error from `MapViewOfFile`.

### Pitfall 2: Go Binary Cold Start vs. Process Pool Confusion

**What goes wrong:** Expecting `beekeeper check` sub-10ms total response time; getting 20-40ms; optimizing the wrong thing (policy eval instead of process startup).

**Why it happens:** The dominant cost is OS process creation + Go runtime initialization (~10–30ms per PRD §8.2). Policy eval is microseconds. mmap load is sub-millisecond.

**How to avoid:** Benchmark with `time beekeeper check < /dev/null` before any optimization. The 100ms p95 target includes cold start. Plan for a warm daemon path in Phase 2+ if cold-start latency becomes a user complaint.

**Warning signs:** Premature optimization of `internal/policy` hot path when profiler shows 90% time in Go runtime init.

### Pitfall 3: Reproducible Build Breakage from Embedded Timestamps

**What goes wrong:** `make verify-release` produces a different hash than the published artifact, failing the reproducibility check.

**Why it happens:** Any of: `ldflags -X main.buildTime=$(date)` in Makefile; Go 1.18+ embeds VCS info by default (the `-buildvcs=false` flag suppresses this); tool version differences between CI and local.

**How to avoid:**
- Use `mod_timestamp: "{{ .CommitTimestamp }}"` in GoReleaser builds section.
- Use ONLY `-X main.version`, `-X main.commit`, `-X main.date={{ .CommitDate }}` — no wall-clock timestamps.
- Mandate the same Go toolchain version in `go.mod` `toolchain` directive and CI `setup-go`.
- Use `-trimpath` AND `-buildvcs=false`.

**Warning signs:** `sha256sum` mismatch on `make verify-release`; CI passes but local build differs.

### Pitfall 4: `go mod verify` Failure After Renovate Update

**What goes wrong:** Renovate opens a PR that updates `go.mod` but `go.sum` is stale or missing; CI `go mod verify` fails.

**Why it happens:** Renovate updates `go.mod` but may not automatically run `go mod tidy` to refresh `go.sum` unless `postUpdateOptions: ["gomodTidy"]` is configured.

**How to avoid:** Add `"postUpdateOptions": ["gomodTidy"]` to `renovate.json` config. CI should run `go mod verify` AND `go mod tidy` with diff check.

**Warning signs:** Renovate PRs failing CI with "missing go.sum entry" errors.

### Pitfall 5: Windows ACL Permissions Not Sticky on File Recreation

**What goes wrong:** Audit log is created with `0600` DACL, then later truncated or recreated by a new process, and the DACL is reset to default (world-readable).

**Why it happens:** On Windows, creating a new file with `os.Create` inherits parent directory permissions, not the explicit DACL of the previous file at that path.

**How to avoid:** Apply `go-acl.Chmod(path, 0600)` after EVERY `os.Create` call for the audit file, not just the first one. Use `os.OpenFile` with `os.O_APPEND|os.O_CREATE|os.O_WRONLY` to avoid file recreation when the file already exists.

**Warning signs:** Audit log readable by other users on shared Windows machine.

### Pitfall 6: GitHub Contents API Rate Limit for Catalog Sync

**What goes wrong:** `beekeeper catalogs sync` fails with 403/429 when called frequently in CI or dev environments.

**Why it happens:** GitHub unauthenticated API: 60 requests/hour per IP. The Contents API call for `threat_intel/` directory + individual file downloads can hit this quickly in CI pipelines.

**How to avoid:** Cache the directory listing response. Use `If-None-Match` ETag header for conditional requests. For CI, set `GITHUB_TOKEN` env var — authenticated rate limit is 5000/hour. In Phase 1, accept that sync is a user-triggered operation (not automated daemon), so 60/hour is sufficient.

**Warning signs:** HTTP 403 with `X-RateLimit-Remaining: 0` header in sync logs.

---

## Code Examples

### GoReleaser Release Configuration (SFDF-01 + SFDF-02)

```yaml
# .goreleaser.yaml
# Source: [VERIFIED: goreleaser.com/blog/reproducible-builds + goreleaser.com/blog/cosign-v3]
version: 2

builds:
  - env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    mod_timestamp: "{{ .CommitTimestamp }}"  # Reproducibility: use commit time, not build time
    flags:
      - -trimpath
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}
      - -X main.date={{.CommitDate}}          # Commit date, NOT $(date)
    binary: beekeeper

checksums:
  name_template: "checksums.txt"

signs:
  - cmd: cosign
    signature: "${artifact}.sigstore.json"   # cosign v3 bundle format
    args:
      - sign-blob
      - "--bundle=${signature}"
      - "${artifact}"
      - "--yes"
    artifacts: checksum                      # Signs checksums.txt; covers all artifacts

sboms:
  - artifacts: archive

archives:
  - format: tar.gz
    format_overrides:
      - goos: windows
        format: zip

release:
  draft: false
```

### GitHub Actions Release Workflow (SFDF-02)

```yaml
# .github/workflows/release.yml
# Source: [VERIFIED: goreleaser/goreleaser-action README + goreleaser.com/ci/actions]
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write
  id-token: write    # Required for cosign OIDC keyless signing

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Install cosign
        uses: sigstore/cosign-installer@v3   # cosign v3

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v7    # v7.2.2 as of 2026-05-26
        with:
          distribution: goreleaser
          version: '~> v2'                   # Matches v2.13.0+ (currently v2.15.x)
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### GitHub Actions CI Workflow (SFDF-03 + cross-platform matrix)

```yaml
# .github/workflows/ci.yml
# Source: [VERIFIED: github.com/mvdan/github-actions-golang best practices]
name: CI

on:
  pull_request:
  push:
    branches: [main]

jobs:
  test:
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: Verify dependencies
        run: go mod verify

      - name: Build
        run: go build -v -trimpath -buildvcs=false ./...

      - name: Test
        run: go test -v -race ./...
        env:
          CGO_ENABLED: 1    # race detector requires CGO

      - name: Vet
        run: go vet ./...
```

**Note on race detector:** `-race` requires `CGO_ENABLED=1`. For the production build, `CGO_ENABLED=0`. Use `CGO_ENABLED=1` in CI test runs only. `go test -race` is the correct incantation; the binary itself is built with `CGO_ENABLED=0`.

[VERIFIED: github.com/goreleaser/goreleaser-action, sigstore/cosign-installer, actions/setup-go]

---

## Adversarial Corpus — Phase 1 Regression Fixtures

These are the minimum required test cases for `beekeeper selftest` and the test suite, derived from live Bumblebee catalog entries.

**Block cases (should produce exit 1 + structured warning):**
1. `npm install nrwl.angular-console@18.95.0` — Nx Console VS Code extension (catalog match)
2. Any package from `mini-shai-hulud.json` e.g. `npm install @tanstack/router@...` (exact match from catalog)
3. Tool call for `editor-extension:nrwl.angular-console:18.95.0` direct match

**Allow cases (should produce exit 0):**
1. `npm install express@4.18.2` — not in any catalog
2. `editor-extension:nrwl.angular-console:18.100.0` — remediated version, not in catalog
3. `go get github.com/spf13/cobra@v1.10.2` — not in catalog
4. Empty tool call (malformed input with `fail_closed` → block)

**Fail-closed cases:**
1. Oversized stdin (>1MB) → block with reason
2. Malformed JSON on stdin → block with reason
3. Missing catalog index (not synced) → block with reason, structured output

[VERIFIED: live catalog entries from perplexityai/bumblebee/threat_intel/]

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| cosign v2 separate `.sig` + `.pem` files | cosign v3 `--bundle artifact.sigstore.json` | GoReleaser v2.13.0 (2025) | Single artifact for signature; simpler verification |
| `syscall.Mmap` directly | `edsrzf/mmap-go` or `golang.org/x/exp/mmap` | Ongoing | Cross-platform abstraction; Windows `CreateFileMapping` handled |
| `go.sum` without Renovate | Renovate with `gomod` manager + `postUpdateOptions: ["gomodTidy"]` | Industry standard circa 2023 | Automated pinned-dep updates with human review |
| SLSA Level 1/2 in release CI | `slsa-github-generator@v2.1.0` for Level 3 | Phase 7 target for Beekeeper | Cryptographic provenance; pipeline = attestation |
| `os.Chmod(path, 0600)` on Windows | `hectane/go-acl acl.Chmod(path, 0600)` | Always required; frequently ignored | Actual DACL restriction on Windows, not just write-bit |

**Deprecated/outdated:**
- cosign v2 `.sig` detached signature format: GoReleaser `signs` section now uses `--bundle` not `--output-signature`/`--output-certificate`
- `GOPATH`-based builds: not relevant with Go modules; `go.mod`/`go.sum` is the standard
- `go get` for installing tools (without `@version`): now requires explicit version pin

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Bumblebee does not currently sign its catalog files cryptographically | User Constraints / CTLG-07 | If Bumblebee adds signatures before Phase 1 ships, signature verification could be implemented (upside risk, not downside) |
| A2 | GitHub unauthenticated API rate limit is 60 requests/hour per IP | Pitfall 6 | If stricter, catalog sync will fail without `GITHUB_TOKEN`; add token auth from the start |
| A3 | `edsrzf/mmap-go` v1.2.0 handles Windows allocation-granularity alignment internally | Pattern 2 / mmap index | If it does not, raw mmap with `golang.org/x/sys/windows` direct calls are needed |
| A4 | Go binary cold start on CI runners is 10–30ms (matching PRD §8.2) | HOOK-01 latency | If slower (e.g., 80ms on Windows CI), the 100ms p95 target may be tight; measure early |
| A5 | GoReleaser `mod_timestamp: "{{ .CommitTimestamp }}"` is sufficient for byte-for-byte reproducible builds (same Go toolchain version, same source) | SFDF-01 | If other non-determinism exists (e.g., build cache headers), `make verify-release` will fail; run reproducibility check in CI from day one |
| A6 | `hectane/go-acl` v0.0.0-20230122 is the appropriate dependency (no CGO, Windows-native ACL) | Pattern 4 | If go-acl has unpatched bugs, direct `golang.org/x/sys/windows` DACL manipulation is the fallback |

---

## Open Questions (RESOLVED)

1. **Bumblebee API stability** (RESOLVED)
   - What we know: Schema `0.1.0` is stable; `entries[]` array with id/name/ecosystem/package/versions/severity is the core contract
   - What's unclear: Whether Bumblebee will add a `schema_version: "0.2.0"` soon; whether the GitHub raw URL path will change
   - Recommendation: Pin the schema version check in `internal/catalog/loader.go`; reject unknown versions with a clear error rather than silent parse
   - **RESOLUTION:** Plan 02 Task 1 implements `ValidateSchemaVersion` that pins `SupportedSchemaVersion = "0.1.0"` and returns an error on unknown versions. Any schema change from Bumblebee is detected immediately rather than silently accepted. The risk is mitigated by hard rejection; only an upside risk (Bumblebee adds signatures earlier than expected) remains.

2. **Go toolchain version pinning for reproducible builds** (RESOLVED)
   - What we know: `-trimpath -buildvcs=false -mod_timestamp` covers most non-determinism
   - What's unclear: Whether the `toolchain go1.25.X` directive in `go.mod` pins the toolchain exactly enough for byte-for-byte reproducibility across different CI runners
   - Recommendation: Run `make verify-release` as a CI step from the first release tag, not just locally; fail loudly if hashes differ
   - **RESOLUTION:** Plan 01 Task 1 pins `toolchain go1.25.0` in `go.mod`. Plan 03 Task 1 implements `make verify-release VERSION=X.Y.Z` which runs the reproducible build and compares hashes in CI — any non-determinism will fail the gate loudly on the first release. The uncertainty is operationally resolved: the measurement gate is the mitigation.

3. **Windows CI performance** (RESOLVED)
   - What we know: `windows-latest` on GitHub Actions uses Windows Server 2022; Go cold start may be slower than Linux due to antivirus scanning
   - What's unclear: Whether p95 latency target of <100ms is achievable on Windows CI for `beekeeper check`
   - Recommendation: Add a latency benchmark test in the Windows CI job from Phase 1 to measure actual cold start overhead
   - **RESOLUTION:** Plan 06 Task 2 implements `BenchmarkCheck` in `handler_bench_test.go` that measures cold-start latency against a realistic catalog. The benchmark runs in the Windows CI job from Phase 1, providing empirical measurements. If Windows p95 exceeds 100ms, the mitigation path (warm daemon mode) is documented in CONTEXT.md as a Phase 2 option — the measurement gap is closed now; action is conditional on results.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All builds | [ASSUMED] ✓ | 1.25+ (per project spec) | — (hard requirement) |
| GitHub Actions | CI/CD, release signing | ✓ | Hosted runners | — |
| cosign | SFDF-02 signing | ✓ via cosign-installer@v3 | v3.x | — (required, no fallback) |
| GoReleaser | Release pipeline | ✓ via goreleaser-action@v7 | v2.15.x | — (required) |
| Bumblebee `threat_intel/` | CTLG-01 | ✓ (public GitHub repo) | schema_version 0.1.0 | Cached copy if GitHub unavailable |
| Renovate bot | SFDF-03 | ✓ (GitHub app) | — | Manual `go mod tidy` PRs |

**Missing dependencies with no fallback:** None identified for Phase 1.

[ASSUMED: Go 1.25 is installable on dev machine; Windows primary dev machine may not have Go pre-installed — install required before development begins]

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go standard `testing` package + `go test` |
| Config file | None — `go test ./...` discovers tests automatically |
| Quick run command | `go test ./internal/... -count=1` |
| Full suite command | `go test -race -count=1 ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| HOOK-01 | `beekeeper check` allows clean package (exit 0), blocks catalog match (exit 1), returns structured JSON | integration | `go test ./internal/check/... -run TestHookHandler -v` | ❌ Wave 0 |
| HOOK-01 | p95 latency < 100ms on realistic catalog | benchmark | `go test ./internal/check/... -bench BenchmarkCheck -benchtime=100x` | ❌ Wave 0 |
| HOOK-02 | mmap index loads in < 1ms; JSON catalog NOT loaded on check path | unit | `go test ./internal/catalog/... -run TestIndexOpenDoesNotReadJSON` | ❌ Wave 0 |
| HOOK-03 | Panic in policy eval → block decision returned, no allow | unit | `go test ./internal/check/... -run TestFailClosed` | ❌ Wave 0 |
| HOOK-03 | Timeout (> 5s execution) → block decision | unit | `go test ./internal/check/... -run TestTimeoutFailClosed` | ❌ Wave 0 |
| HOOK-04 | Stdin > 1MB → block decision | unit | `go test ./internal/check/... -run TestStdinCapEnforced` | ❌ Wave 0 |
| CTLG-01 | Catalog JSON parsed correctly from Bumblebee schema_version 0.1.0 | unit | `go test ./internal/catalog/... -run TestCatalogParse` | ❌ Wave 0 |
| CTLG-01 | Unknown schema_version → error (not silent accept) | unit | `go test ./internal/catalog/... -run TestUnknownSchemaVersion` | ❌ Wave 0 |
| CTLG-07 | Missing catalog_signature field → warn-only decision (not block) | unit | `go test ./internal/policy/... -run TestUnsignedCatalogIsWarnOnly` | ❌ Wave 0 |
| SFDF-01 | `make verify-release` target exists and runs successfully | smoke | `make verify-release VERSION=0.0.1-test` (manual) | ❌ Wave 0 |
| SFDF-03 | `go mod verify` passes | CI check | `go mod verify` (in ci.yml) | ❌ Wave 0 (ci.yml needed) |
| HOOK-01 | `beekeeper selftest` exits 0 with all fixtures passing | integration | `go test ./... -run TestSelftest` OR `beekeeper selftest` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./internal/... -count=1` (< 30 seconds)
- **Per wave merge:** `go test -race -count=1 ./...` on all three OS matrix
- **Phase gate:** Full suite green on all three platforms before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `internal/check/handler_test.go` — TestHookHandler, TestFailClosed, TestTimeoutFailClosed, TestStdinCapEnforced
- [ ] `internal/catalog/loader_test.go` — TestCatalogParse, TestUnknownSchemaVersion, TestIndexOpenDoesNotReadJSON
- [ ] `internal/catalog/index_test.go` — TestIndexBuild, TestIndexLookup, TestIndexBinarySearch
- [ ] `internal/policy/engine_test.go` — TestUnsignedCatalogIsWarnOnly, TestBumblebeeCatalogMatch
- [ ] `internal/audit/writer_test.go` — TestNDJSONWrite, TestPermissionsEnforced
- [ ] `internal/platform/dirs_test.go` — TestStateDirWindows, TestStateDirUnix
- [ ] `testdata/fixtures/` — allow and block case tool call JSON fixtures
- [ ] `.github/workflows/ci.yml` — matrix test workflow
- [ ] `Makefile` — `build`, `test`, `verify-release` targets

*(Framework install: `go test` is built into Go toolchain — no additional install needed)*

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | No auth in Phase 1 (single-user local binary) |
| V3 Session Management | No | Stateless one-shot `beekeeper check` |
| V4 Access Control | Yes — audit log | `0600` Unix + DACL Windows via `go-acl` |
| V5 Input Validation | Yes — stdin tool call JSON | `io.LimitReader(1MB)` + `json.Decoder` strict parsing |
| V6 Cryptography | Yes — catalog signature verification | Phase 1: log warning if absent; design for Ed25519 or cosign in Phase 2 |
| V7 Error Handling | Yes — fail-closed | All error paths → block; panic/recover in check handler |

### Known Threat Patterns for This Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Oversized stdin causing OOM | Denial of Service | `io.LimitReader(1MB)` hard cap |
| Malicious JSON crafted to exploit parser | Tampering | `encoding/json` stdlib decoder; hard cap on nesting with `json.Decoder.SetDepth` (Go 1.25+) |
| Catalog poisoning via malicious `threat_intel/` PR | Tampering | Signature verification (Phase 1: warn only); `schema_version` validation; reject unknown fields from trusted-but-unexpected payloads |
| Audit log world-readable → credential leakage | Information Disclosure | `0600` Unix; `go-acl` DACL Windows; set permissions on first `os.Create` |
| Supply chain compromise of Beekeeper itself | Repudiation | Sigstore/cosign v3 + reproducible builds; `SECURITY.md` disclosure |
| Single-source catalog false positive → accidental block | Denial of Service | Phase 1: single-source → warn only, not block (corroboration enforcement is Phase 2) |
| Timeout during hook → silent allow | Elevation of Privilege | `context.WithTimeout(5s)` with deadline check before decision emission; fail closed |

---

## Project Constraints (from CLAUDE.md)

- Go 1.25+ single static binary; `internal/` for all business logic; no CGO in core
- `internal/policy` is a pure function library — no I/O, no goroutines, no side effects
- Hook handler loads catalog via mmap; NEVER cold-load JSON per invocation
- Fail closed by default; `fail_open` is an explicit opt-in documented as reducing security
- MCP gateway is stateless per-request proxy (Phase 4, not Phase 1)
- Bubble Tea import: `charm.land/bubbletea/v2` (NOT `github.com/charmbracelet`) — Phase 8
- eBPF: pre-compiled bytecode embedded at build time via `bpf2go` — Phase 5, but CI scaffold begins now (N/A for Phase 1 builds)
- ETW: `tekert/golang-etw` (no CGO) — Phase 7
- Windows primary dev machine; no WSL integration tests; CI-driven Linux/macOS iteration
- Reproducible builds required from v0.1.0
- Every phase includes self-defense work; never defer SFDF items

---

## Sources

### Primary (HIGH confidence)

- `raw.githubusercontent.com/perplexityai/bumblebee/main/threat_intel/nx-console-vscode-2026-05-18.json` — Verified live Bumblebee catalog schema and Nx Console catalog entry
- `raw.githubusercontent.com/perplexityai/bumblebee/main/threat_intel/mini-shai-hulud.json` — Verified catalog scope and format
- `pkg.go.dev/github.com/edsrzf/mmap-go` — v1.2.0 API, Windows CreateFileMapping/MapViewOfFile, access flags
- `pkg.go.dev/github.com/spf13/cobra` — v1.10.2 confirmed as latest (Dec 2025)
- `pkg.go.dev/github.com/hectane/go-acl` — `acl.Chmod(path, 0600)` Windows DACL
- `pkg.go.dev/golang.org/x/exp/mmap` — ReaderAt API, cross-platform support
- `goreleaser.com/blog/reproducible-builds` — `mod_timestamp: CommitTimestamp`, ldflags pattern
- `goreleaser.com/blog/cosign-v3` — `--bundle ${signature}` signs section format
- `github.com/goreleaser/goreleaser-action` — v7.2.2 current, `version: '~> v2'` pattern, `id-token: write` permission
- Go stdlib `os.UserConfigDir` documentation — Windows `%APPDATA%`, Linux `$XDG_CONFIG_HOME`

### Secondary (MEDIUM confidence)

- `medium.com/@MichalPristas` — Go file permissions on Windows; `0600` partial behavior; DACL required
- `github.com/mvdan/github-actions-golang` — Go CI best practices matrix, `CGO_ENABLED=1` for race detector
- `words.filippo.io/reproducing-go-binaries-byte-by-byte` — remaining non-determinism sources
- `go.dev/blog/rebuild` — Go toolchain reproducibility guarantees

### Tertiary (LOW confidence / ASSUMED)

- Go binary cold start 10–30ms — cited from PRD §8.2; not independently measured in this session
- `edsrzf/mmap-go` alignment handling — stated behavior from documentation; assumed Windows alignment is handled internally
- Binary index format design (flat sorted array + `sort.Search`) — Claude's training + standard CS knowledge; not from official docs

---

## Metadata

**Confidence breakdown:**
- Bumblebee catalog schema: HIGH — verified from live repo files
- Standard stack (cobra, mmap-go, go-acl): HIGH — verified on pkg.go.dev
- GoReleaser + cosign v3 pipeline: HIGH — verified from official goreleaser docs and action README
- Architecture patterns (fail-closed, mmap index format): MEDIUM — design decisions from PRD/CONTEXT.md; not independently verified against real implementation
- Pitfalls: MEDIUM — derived from official docs + known platform behavior; some from training knowledge

**Research date:** 2026-05-26
**Valid until:** 2026-06-25 (30 days for stable stack; GoReleaser releases frequently but config patterns are stable across v2.x)
