# Phase 1: Fork Setup & Discipline — Research

**Researched:** 2026-06-01
**Domain:** Go module fork mechanics, reproducible builds, Sigstore/cosign v3 keyless signing, NDJSON determinism analysis, GitHub Actions CI matrix
**Confidence:** HIGH (core findings directly verified against upstream repo and beekeeper build files)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- Pollen lives at `C:\Users\Bantu\mzansi-agentive\pollen` (sibling to beekeeper), as its own git repository — NOT vendored into beekeeper.
- GitHub home: `github.com/bantuson/pollen` (handle is `bantuson`; `mzansi-agentive` is only local naming).
- GSD tracks this milestone from beekeeper. Phase planning artifacts live in `beekeeper/.planning/`. Code is committed to `../pollen` via explicit `git -C ../pollen ...` operations. Beekeeper's GSD commits cover only planning artifacts.
- Executor tasks that produce pollen files MUST perform their own `git -C ../pollen add/commit` and must NOT rely on beekeeper's auto-commit for pollen code.
- Pin to upstream **commit** (not branch): v0.1.1 tag SHA. Record in `UPSTREAM.md`.
- Preserve upstream's directory structure. ONLY structural changes: module path rewrite + `cmd/bumblebee/` → `cmd/pollen/`. (`_windows.go` files come in P2–P4.)
- `VERSION` file: append `-pollen.N` suffix (e.g. `0.1.1-pollen.1`).
- `threat_intel/` ships **empty** except upstream selftest fixtures — catalogs flow through beekeeper's `catalogs sync`, not duplicated in Pollen.
- **Zero non-stdlib dependencies added** beyond what upstream already has.
- `LICENSE`: verbatim upstream Apache-2.0, unmodified.
- `NOTICE`: exact text in PRD §7.2.
- `CHANGES.md`: §7.3 format — Added/Renamed/Modified/Removed sections.
- **Trademark discipline (FORK-04):** "Bumblebee" appears ONLY in attribution contexts.
- Reproducible builds: `-trimpath -buildvcs=false -mod=readonly`.
- Sigstore/cosign **keyless** signing via GitHub Actions OIDC — cert identity `github.com/bantuson/pollen/.github/workflows/...`.
- CycloneDX SBOM per release (syft).
- `pollen selftest` passes on all three OSes.
- Differential test: byte-for-byte identical NDJSON on Linux + macOS — determinism normalization is required (see NDJSON Determinism section).
- First tagged release: `v0.1.1-pollen.1`.

### Claude's Discretion

- Exact CI job names / file layout of `.github/workflows/ci.yml` and release workflow.
- How the differential test obtains the upstream `bumblebee` binary in CI (build from pinned commit vs download release artifact).
- Test-fixture directory layout for differential/selftest inputs.
- Whether to add an `upstream` git remote in the pollen repo now or in P5.

### Deferred Ideas (OUT OF SCOPE)

- Windows root resolver, path representation, extension/MCP coverage — Phases 2–4.
- Beekeeper-side `internal/inventory/` integration + compat test + honeypot — Phases 4–5.
- `pollen-self` catalog entries in `beekeeper-self` — Phase 5.
- Upstream contribution-back PRs — Phase 5.
- Pollen binary distribution — DIST-01, v2.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| FORK-01 | Pollen as separate Go module `github.com/bantuson/pollen`, forked at pinned v0.1.1 commit, `cmd/bumblebee/` → `cmd/pollen/`, CLI builds on ubuntu/macos/windows | Upstream structure verified; rename mechanics documented; Windows build confirmed via CGO_ENABLED=0 |
| FORK-02 | Apache-2.0 LICENSE verbatim; NOTICE; CHANGES.md; UPSTREAM.md with 40-char SHA | v0.1.1 SHA confirmed: `c24089804ee66ece4bec6f14638cb98985389cdb`; tag date 2026-05-22; upstream go.mod confirmed |
| FORK-03 | Reproducible builds + Sigstore keyless signing + tag `v0.1.1-pollen.1` | Beekeeper's exact stanza to mirror documented; Pollen-specific cosign identity string documented |
| FORK-04 | Trademark discipline — "Bumblebee" only in attribution contexts | Hardcoded "bumblebee" string locations identified in upstream (help text, binary name, selftest temp dir prefix) |
| PTEST-02 | Differential test: Pollen byte-for-byte identical to upstream Bumblebee on Linux + macOS | NON-DETERMINISTIC fields identified; normalization strategy documented; harness design specified |
| PTEST-03 | `pollen selftest` passes on all three OSes; CI matrix green | Selftest implementation verified (3 findings, embedded fixtures); Windows `-race` CGO constraint documented |
| SDEF-02 | CycloneDX SBOM per Pollen release, recording source upstream commit | syft stanza from beekeeper documented; adaptation for pollen specified |
</phase_requirements>

---

## Summary

Upstream `perplexityai/bumblebee` at v0.1.1 is a **zero-dependency** Go 1.25 module (no `go.sum` file; only stdlib). Its structure is simpler than the PRD §5.2 assumption: there is no `internal/resolver/` package — root discovery lives in `cmd/bumblebee/roots.go`. The binary name and binary-name strings are the only non-trivial rename targets beyond the module path. The selftest uses `//go:embed selftest/fixtures selftest/catalog.json` and asserts exactly 3 findings against embedded fixtures.

The hardest load-bearing unknown is **NDJSON determinism for the differential test (PTEST-02)**. Upstream emits four non-deterministic fields per scan: `run_id` (crypto/rand hex), `scan_time` (RFC3339Nano wall-clock), `end_time` (wall-clock), and `duration_ms` (elapsed). Additionally, concurrent workers emit package records in **non-deterministic order**. The differential test cannot be a raw byte comparison — it requires a normalization harness that strips these fields and sorts the record stream before comparing. This is the most architecturally significant finding in this research.

The self-defense stack (cosign keyless, SLSA, syft SBOM) mirrors beekeeper's `.goreleaser.yaml` + `release.yml` pattern almost exactly, with one critical adaptation: the cosign OIDC cert identity must use `github.com/bantuson/pollen/...` (not `mzansi-agentive/beekeeper`). Upstream's own `.goreleaser.yaml` lacks `-buildvcs=false`, signing, and SBOM — Pollen's release pipeline must add all three from scratch.

**Primary recommendation:** Create the pollen repo, wire the CI guard rails (differential + selftest + repro-build) using the normalization harness before implementing any Windows code. Everything else in this phase is mechanical rename + file addition.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| CLI entry point (`pollen` binary) | `cmd/pollen/` | — | Thin wiring only; no business logic |
| Root discovery (scan profiles) | `cmd/pollen/roots.go` | — | Upstream places this in cmd, not internal/; preserve structure |
| Scanner orchestration | `internal/scanner/` | — | Upstream package; preserve unchanged |
| NDJSON emission | `internal/output/` | — | Upstream package; preserve unchanged |
| Endpoint collection (hostname/uid) | `internal/endpoint/` | — | Upstream package; preserve unchanged |
| Module path + version injection | `go.mod` + `cmd/pollen/version.go` | Makefile / goreleaser | ldflags `-X main.Version=...` |
| Selftest fixtures | `cmd/pollen/selftest/` | — | Embedded via go:embed; relocates with cmd rename |
| Differential test harness | `cmd/pollen/` or `internal/testutil/` | CI workflow | New code; normalizes NDJSON before byte comparison |
| Reproducible build | `.goreleaser.yaml` + `Makefile` | — | Flags: `-trimpath -buildvcs=false -mod=readonly` |
| Keyless signing | `.github/workflows/release.yml` | `.goreleaser.yaml` | cosign `sign-blob --bundle` on checksums.txt |
| SBOM generation | `.goreleaser.yaml` sboms stanza | anchore/sbom-action | syft CycloneDX per archive |
| SLSA provenance | `.github/workflows/release.yml` | — | slsa-github-generator@v2.1.0 |
| CI matrix | `.github/workflows/ci.yml` | — | 3-OS matrix; differential on Linux+macOS only |
| Legal/attribution files | repo root | — | LICENSE, NOTICE, CHANGES.md, UPSTREAM.md |

---

## Standard Stack

### Core (no external dependencies added)

Upstream bumblebee has **zero non-stdlib Go dependencies** — no `go.sum` exists at v0.1.1. This is verified, not assumed.

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib only | go 1.25 | All scanning, NDJSON output, HTTP sink | upstream constraint |
| `cmd/pollen/selftest/` | embedded | `go:embed selftest/fixtures selftest/catalog.json` | upstream pattern |

### Build / Release Tooling

| Tool | Version | Purpose | Notes |
|------|---------|---------|-------|
| GoReleaser | ~v2 (latest: v2.16.0) | Multi-platform build + release | Mirror beekeeper stanza |
| cosign | v3 (sigstore/cosign-installer@v3) | Keyless OIDC signing | Latest is v3.0.6 |
| syft | via anchore/sbom-action/download-syft@v0 | CycloneDX SBOM | Same as beekeeper |
| slsa-github-generator | @v2.1.0 (full semver, locked) | SLSA Level 3 provenance | CRITICAL: never @v2 alone |
| actions/checkout | @v4 | Checkout with full history | GoReleaser needs tags |
| actions/setup-go | @v5 | Pin Go to go.mod toolchain | `go-version-file: go.mod` |

**Version verification:** [VERIFIED: github.com/sigstore/cosign/releases] cosign latest: v3.0.6. [VERIFIED: github.com/goreleaser/goreleaser/releases] GoReleaser latest: v2.16.0. [VERIFIED: slsa-framework/slsa-github-generator — beekeeper CONTEXT.md locked] @v2.1.0.

---

## Architecture Patterns

### System Architecture Diagram

```
git clone upstream@v0.1.1 ──► fork mechanics (rename module + cmd) ──► pollen repo
                                                                           │
          ┌────────────────────────────────────────────────────────────────┤
          ▼                           ▼                                    ▼
  cmd/pollen/                internal/{scanner,output,...}        .github/workflows/
  ├── main.go                (preserved verbatim from upstream)   ├── ci.yml (3-OS matrix)
  ├── roots.go                                                     └── release.yml
  ├── selftest.go ──► go:embed selftest/fixtures + catalog.json        (cosign+SBOM+SLSA)
  ├── version.go (-X main.Version)
  └── selftest/
      ├── catalog.json                                            .goreleaser.yaml
      └── fixtures/                                              (trimpath+buildvcs=false
          ├── npm-fixture/                                        +cosign sign-blob
          ├── pypi-fixture/                                       +syft cdx sbom)
          └── mcp-fixture/

Differential test flow (Linux+macOS CI only):
  ┌─────────────┐       ┌────────────────────┐       ┌──────────────────────────────┐
  │ fixed       │──────►│ run pollen scan     │──────►│ normalize:                   │
  │ fixture dir │       │ (pollen binary)     │       │  strip run_id, scan_time,    │
  └─────────────┤       └────────────────────┘       │  end_time, duration_ms,      │
                │       ┌────────────────────┐       │  endpoint.{hostname,uid,user}│
                └──────►│ run bumblebee scan  │──────►│  sort records by record_id   │
                        │ (upstream binary)   │       └───────────┬──────────────────┘
                        └────────────────────┘                   ▼
                                                        byte-for-byte compare ──► PASS/FAIL
```

### Recommended Project Structure

```
pollen/
├── LICENSE                        # verbatim upstream Apache-2.0
├── NOTICE                         # PRD §7.2 text
├── CHANGES.md                     # §7.3 format
├── UPSTREAM.md                    # pinned SHA + sync workflow
├── README.md                      # Pollen purpose; points general users to upstream
├── VERSION                        # "0.1.1-pollen.1"
├── go.mod                         # module github.com/bantuson/pollen; go 1.25
├── Makefile                       # mirror beekeeper's repro-build targets
├── .goreleaser.yaml               # multi-OS + signing + SBOM
├── .github/
│   └── workflows/
│       ├── ci.yml                 # 3-OS matrix + differential + selftest
│       └── release.yml            # GoReleaser + cosign + SLSA
├── cmd/
│   └── pollen/                    # was cmd/bumblebee/
│       ├── main.go
│       ├── main_test.go
│       ├── roots.go
│       ├── selftest.go            # go:embed selftest/fixtures selftest/catalog.json
│       ├── selftest_test.go
│       ├── sink.go
│       ├── version.go
│       └── selftest/
│           ├── catalog.json
│           └── fixtures/
│               ├── npm-fixture/
│               ├── pypi-fixture/
│               └── mcp-fixture/
├── internal/                      # preserved verbatim from upstream
│   ├── ecosystem/
│   ├── endpoint/
│   ├── exposure/
│   ├── model/
│   ├── normalize/
│   ├── output/
│   ├── scanner/
│   └── walk/
├── docs/                          # preserved verbatim from upstream
└── threat_intel/                  # upstream catalogs; ships empty except selftest fixtures
    └── README.md                  # note: catalogs flow via beekeeper catalogs sync
```

### Pattern 1: Module Path Rewrite

**What:** Replace every occurrence of `github.com/perplexityai/bumblebee` with `github.com/bantuson/pollen` across all `.go` files and `go.mod`.

**When to use:** First step after cloning upstream at the pinned commit.

**Scope:** The import path appears only in:
1. `go.mod` (one line)
2. All `internal/` imports in `cmd/bumblebee/*.go` (confirmed: main.go, selftest.go import `internal/endpoint`, `internal/exposure`, `internal/model`, `internal/output`, `internal/scanner`)
3. No `go.sum` to update (zero external deps)

**Example:**
```bash
# On Linux/macOS (run from pollen repo root after cloning):
find . -name '*.go' -o -name 'go.mod' | xargs sed -i 's|github.com/perplexityai/bumblebee|github.com/bantuson/pollen|g'
# On Windows PowerShell equivalent:
Get-ChildItem -Recurse -Include '*.go','go.mod' | ForEach-Object {
    (Get-Content $_.FullName) -replace 'github.com/perplexityai/bumblebee','github.com/bantuson/pollen' |
    Set-Content $_.FullName
}
```

[VERIFIED: github.com/perplexityai/bumblebee go.mod] — module path confirmed `github.com/perplexityai/bumblebee`.

### Pattern 2: cmd Directory Rename

**What:** Rename `cmd/bumblebee/` to `cmd/pollen/`. Binary name in `.goreleaser.yaml` changes from `bumblebee` to `pollen`.

**Gotchas identified:**
1. `selftest.go` creates a temp dir with `os.MkdirTemp("", "bumblebee-selftest-*")` — this must change to `"pollen-selftest-*"` (trademark discipline, FORK-04).
2. `main.go` help/usage text may reference "bumblebee" — search and rename.
3. `cmd/bumblebee/version.go` has `fileDefault = "0.1.1"` — update to use VERSION file value `0.1.1-pollen.1`.
4. The `selftest/` subdirectory (containing embedded fixtures) stays at the same relative path within the renamed directory — `go:embed` directive paths are relative to the Go source file, so they remain `selftest/fixtures` and `selftest/catalog.json` unchanged.

[VERIFIED: github.com/perplexityai/bumblebee selftest.go] — `go:embed all:selftest/fixtures selftest/catalog.json` directive confirmed. Temp dir prefix confirmed `bumblebee-selftest-*`.

### Pattern 3: Version Injection

**What:** The version variable is `main.Version` in the `main` package (package `main` in `cmd/pollen/main.go`). Injected at build time.

**Exact ldflags path:**
```
-ldflags "-X main.Version=0.1.1-pollen.1"
```

**VERSION file:** Set to `0.1.1-pollen.1`. The version.go `fileDefault` constant references `"0.1.1"` — update to `"0.1.1-pollen.1"` or rely on ldflags injection which takes priority.

[VERIFIED: github.com/perplexityai/bumblebee version.go] — `main.Version` confirmed as the ldflags target. Three-tier precedence: ldflags > build info > fileDefault.

### Pattern 4: Reproducible Build (mirror beekeeper Makefile)

```makefile
MODULE     := github.com/bantuson/pollen
VERSION    ?= dev
COMMIT     := $(shell git rev-parse HEAD)
DATE       := $(shell git show -s --format=%cI HEAD)

GOFLAGS    := -trimpath -buildvcs=false -mod=readonly
LDFLAGS    := -s -w -X main.Version=$(VERSION)

build:
    go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/pollen ./cmd/pollen
```

Note: Pollen uses `main.Version` (not a `version` package), so ldflags path is simpler than beekeeper's (`-X github.com/bantuson/beekeeper/internal/version.Version`).

### Pattern 5: cosign Keyless Signing (pollen-specific identity)

The cosign OIDC cert identity for pollen's release workflow will be:

```
https://github.com/bantuson/pollen/.github/workflows/release.yml@refs/tags/v...
```

This differs from beekeeper's identity (`github.com/bantuson/beekeeper/...`). The `.goreleaser.yaml` `signs` stanza does NOT specify the identity — identity is bound by the OIDC token from GitHub Actions. No change to YAML needed; the identity is automatic from the repo context.

**THREAT-MODEL.md verify command** for pollen should use:
```bash
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp "^https://github.com/bantuson/pollen/" \
  checksums.txt
```

[VERIFIED: beekeeper .goreleaser.yaml + release.yml] — pattern confirmed. [ASSUMED] identity string shape — `github.com/bantuson/pollen/...` is the expected format based on GitHub Actions OIDC, but the exact URL in the sigstore cert should be validated after the first actual release.

### Anti-Patterns to Avoid

- **Copying beekeeper's eBPF before.hooks into pollen's goreleaser:** Pollen has no eBPF — the `sh -c 'if [ "$(uname -s)" = "Linux" ]; then go generate ...'` hook does not belong in pollen's `.goreleaser.yaml`.
- **Using `-mod=vendor`:** Pollen has zero external deps; vendoring is unnecessary and adds complexity.
- **Adding goarch arm64 for Windows:** Go 1.21+ supports `windows/arm64`, but upstream targets only `darwin/linux amd64+arm64`. Pollen's initial release can mirror upstream (amd64+arm64 for darwin+linux+windows), but arm64 Windows is optional given no current hardware target.
- **Committing threat_intel/ catalogs:** Only the selftest `catalog.json` (embedded in the binary) lives in the repo. The `threat_intel/` directory stays empty — catalogs flow through beekeeper.
- **Using `go.sum` with zero deps:** The initial `go mod tidy` will create a minimal `go.sum` after the module path rename — but since there are no external deps, it will be empty or contain only the Go toolchain reference. This is correct.

---

## NDJSON Determinism Analysis (PTEST-02 — Load-Bearing Finding)

This is the most architecturally significant finding. The differential test CANNOT be a raw `diff` or `sha256` comparison of raw output files.

### Non-Deterministic Fields Identified

| Field | Present In | Source | Why Non-Deterministic |
|-------|-----------|--------|----------------------|
| `run_id` | All record types | `newRunID()` in main.go | `crypto/rand.Read(16 bytes)` — different every run |
| `scan_time` | All record types | `time.Now().UTC()` at scan start | Wall-clock timestamp |
| `end_time` | `scan_summary` only | `time.Now().UTC()` at scan completion | Wall-clock timestamp |
| `duration_ms` | `scan_summary` only | elapsed time calculation | Varies with machine load |
| `endpoint.hostname` | All record types | `os.Hostname()` | Differs across machines |
| `endpoint.username` | All record types | `user.Current()` | Differs across machines / CI vs local |
| `endpoint.uid` | All record types | `user.Uid` | Differs across machines |

[VERIFIED: github.com/perplexityai/bumblebee internal/model/model.go, internal/endpoint/endpoint.go, cmd/bumblebee/main.go]

### Record Ordering Is Also Non-Deterministic

The scanner spawns **4 concurrent workers** (default) processing files from a shared channel. Package records are emitted as workers finish, not in filesystem order. Two runs on identical fixtures will produce records in different order.

[VERIFIED: github.com/perplexityai/bumblebee internal/scanner/scanner.go] — "256-item buffered job channel, 4 goroutines, wg.Wait"

### Deterministic Fields (safe for comparison)

- `record_id` — SHA-256 of (record_type, ecosystem, package_name, version, source paths). Same input → same hash. [VERIFIED: model.go StableID()]
- All package metadata fields (ecosystem, package_name, version, source_file, etc.)
- `schema_version` — always `"0.1.0"`
- `record_type` — enum constant
- Finding fields (catalog_id, severity, etc.)

### Differential Harness Design

The normalization function applied to BOTH the pollen output AND the upstream bumblebee output before comparison:

```go
// Source: derived from field analysis above [VERIFIED upstream model.go]
type NormalizedRecord struct {
    RecordType  string `json:"record_type"`
    RecordID    string `json:"record_id"`
    // ... all fields EXCEPT: run_id, scan_time, end_time, duration_ms,
    //     endpoint.hostname, endpoint.username, endpoint.uid
}

func normalize(ndjson []byte) ([]NormalizedRecord, error) {
    // 1. Parse each line as a JSON object
    // 2. Delete keys: run_id, scan_time, end_time, duration_ms,
    //    endpoint.hostname, endpoint.username, endpoint.uid
    // 3. Sort records by record_id (deterministic stable sort)
    // 4. Return sorted slice for comparison
}
```

The comparison asserts:
1. Same number of records
2. Same record types in same proportion
3. Each normalized record matches exactly (byte-for-byte after normalization)

**Fixture environment control:** The differential test must control the `--root` explicitly (pass the fixture directory, not a dynamic home dir). The `--profile` must be `deep` or `project` with explicit `--root` so that root discovery doesn't vary between pollen and bumblebee. The `endpoint.device_id` can be controlled via the `--device-id-env` flag (pass a fixed env var).

### Obtaining the Upstream Bumblebee Binary in CI

Two options (Claude's Discretion per CONTEXT.md):

**Option A — Build from pinned commit (recommended):**
```yaml
- name: Build upstream bumblebee for differential test
  run: |
    git clone https://github.com/perplexityai/bumblebee.git /tmp/bumblebee-upstream
    cd /tmp/bumblebee-upstream
    git checkout c24089804ee66ece4bec6f14638cb98985389cdb
    go build -o /tmp/bumblebee-bin ./cmd/bumblebee
```
Advantage: pinned to exact commit; reproducible; no dependency on upstream release artifacts. Disadvantage: adds a git clone step (~10s).

**Option B — Download from upstream release:**
```yaml
- name: Install upstream bumblebee
  run: go install github.com/perplexityai/bumblebee/cmd/bumblebee@v0.1.1
```
Advantage: faster. Disadvantage: relies on upstream module proxy availability; pulls latest module cache.

**Recommendation:** Option A (build from pinned commit SHA). Aligns with the "pin to commit" discipline in CONTEXT.md §6.1. The differential test is specifically about proving pollen matches the pinned upstream commit, not any later bumblebee version.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Keyless binary signing | Custom GPG/PGP infrastructure | cosign `sign-blob --bundle` + GitHub Actions OIDC | Long-lived keys are the attack surface |
| SBOM generation | Manual dependency enumeration | syft CycloneDX via GoReleaser sboms stanza | syft handles transitive deps, format specs |
| SLSA attestation | Manual provenance JSON | slsa-github-generator@v2.1.0 | Meets SLSA Level 3; DIY cannot meet builder requirements |
| Reproducibility verification | Trust the build | `make verify-release` (double-build + hash compare) | Needed to prove `-trimpath` actually works |
| NDJSON normalization | Per-field heuristics | Structured field-strip + sort by record_id | record_id is already a stable SHA-256 content key |
| Module path rewrite | Manual editing | `sed`/PowerShell bulk replace + `go mod tidy` | Small codebase; automated is safer than manual |

---

## Common Pitfalls

### Pitfall 1: Missing `-buildvcs=false` in Upstream's goreleaser

**What goes wrong:** Upstream's `.goreleaser.yaml` does NOT include `-buildvcs=false`. If pollen copies upstream's goreleaser config directly, the builds will embed VCS info (commit hash, dirty status) that differs between machines → reproducibility failure.

**Why it happens:** Upstream's goreleaser only has `-trimpath` (not `-buildvcs=false`). Beekeeper's goreleaser adds `-buildvcs=false` explicitly.

**How to avoid:** Copy from beekeeper's `.goreleaser.yaml`, not upstream's. Specifically add `-buildvcs=false` to both `flags:` and the Makefile GOFLAGS.

**Warning signs:** `make verify-release` hash mismatch even with identical source.

[VERIFIED: beekeeper .goreleaser.yaml, upstream .goreleaser.yaml comparison]

### Pitfall 2: go:embed Path Breaks After cmd/ Rename

**What goes wrong:** If the `go:embed` directive path in `selftest.go` is not preserved exactly relative to the file, the embedded assets silently produce an empty filesystem → `pollen selftest` fails with 0 findings.

**Why it happens:** `go:embed` paths are relative to the Go source file. After renaming `cmd/bumblebee/` → `cmd/pollen/`, the `selftest/` subdirectory physically moves with it. The directive `//go:embed all:selftest/fixtures selftest/catalog.json` is still correct because it refers to `cmd/pollen/selftest/` which exists.

**How to avoid:** Do NOT change the `go:embed` directive text. Only rename the parent directory. Verify: `go build ./cmd/pollen` compiles without `no matching files` errors.

**Warning signs:** Compile error `pattern selftest/fixtures: no matching files found`.

[VERIFIED: github.com/perplexityai/bumblebee selftest.go]

### Pitfall 3: Trademark Violation in Temp Dir Prefix

**What goes wrong:** `os.MkdirTemp("", "bumblebee-selftest-*")` creates temp dirs with "bumblebee" in the name. This is a command name context that violates FORK-04.

**Why it happens:** Upstream uses the binary name in the temp dir prefix. After renaming the binary, the string literal in selftest.go must be updated.

**How to avoid:** Change to `"pollen-selftest-*"` in `cmd/pollen/selftest.go`.

**Warning signs:** `ls /tmp/bumblebee-selftest-*` entries visible after `pollen selftest`.

[VERIFIED: github.com/perplexityai/bumblebee selftest.go — temp dir prefix confirmed]

### Pitfall 4: Differential Test Fails Due to Record Ordering

**What goes wrong:** Running `pollen scan --root ./fixture` and `bumblebee scan --root ./fixture` produces NDJSON with records in different orders (concurrent workers). Even though both tools find exactly the same packages, a line-by-line diff reports them as different.

**Why it happens:** Scanner spawns 4 goroutines. Worker completion order is non-deterministic based on filesystem and scheduler timing.

**How to avoid:** Normalize both outputs: parse NDJSON, strip non-deterministic fields, sort by `record_id` (stable SHA-256 content key), then compare. Never compare raw NDJSON files directly.

**Warning signs:** Differential test passes locally then fails intermittently in CI (nondeterministic).

[VERIFIED: github.com/perplexityai/bumblebee internal/scanner/scanner.go — goroutine architecture confirmed]

### Pitfall 5: `go test -race` on Windows Requires CGO

**What goes wrong:** `go test -race ./...` fails on Windows CI with `CGO_ENABLED=0` because the race detector requires a C compiler.

**Why it happens:** Go's race detector uses CGO under the hood on all platforms. Windows CI runners (windows-latest) have MSVC available, but the environment may not be configured.

**How to avoid:** Mirror beekeeper's pattern: set `CGO_ENABLED: 1` in the CI `Test` step env. Windows-latest has MSVC. Alternatively, skip `-race` on Windows and accept this as a known constraint (documented in STATE.md under deferred items from v1.0.0 Phase 1).

**Decision for this phase:** Use `CGO_ENABLED: 1` in the CI test step to mirror beekeeper. Since pollen has zero CGO in its source code (CGO_ENABLED=0 for build), the race detector is the only CGO consumer. This is consistent with beekeeper's established pattern.

**Warning signs:** CI test step: `error: -race is not supported on windows/amd64 without cgo`.

[VERIFIED: beekeeper STATE.md "go test -race requires CGO + C compiler (not installed on Windows dev machine); race gate runs in CI"; beekeeper ci.yml `CGO_ENABLED: 1`]

### Pitfall 6: Windows Build Includes No Resolver Logic Yet

**What goes wrong:** `pollen scan` runs on Windows but returns 0 packages because there is no Windows root resolver. This is EXPECTED behavior for Phase 1, but selftest might fail if the selftest fixture paths use Unix-style paths.

**Why it happens:** selftest uses embedded fixtures extracted to a temp dir, not the system package roots. The fixtures are valid on all OSes because they are just POSIX-style directory trees in a temp location. Selftest should pass regardless.

**How to avoid:** Selftest passes on Windows because it uses explicit `--root <tmpdir>` (not the system root resolver). The "Windows tests explicitly skipped with structured reasons" in CONTEXT.md refers to the roots-based tests in `main_test.go`, not selftest.

**Warning signs:** If `pollen selftest` fails on Windows with "0 findings found, expected 3" — investigate fixture extraction, not the resolver.

[VERIFIED: github.com/perplexityai/bumblebee selftest.go — extracts to temp dir, uses explicit root, not system resolver]

### Pitfall 7: `go mod tidy` Surprises After Module Path Rename

**What goes wrong:** After renaming the module path in `go.mod`, `go mod tidy` may create a minimal `go.sum` referencing `go 1.25.0` toolchain. Since bumblebee has zero external deps, this is fine — but CI's "Check go.mod/go.sum are tidy" step (mirrored from beekeeper) must be run before committing.

**How to avoid:** Run `go mod tidy` immediately after the module path rename. Commit the resulting state. CI will verify on every PR.

---

## Upstream Structure: Verified Facts

### v0.1.1 Tag Details

| Property | Value | Source |
|----------|-------|--------|
| Tag name | `v0.1.1` | [VERIFIED: gh api repos/perplexityai/bumblebee/git/refs/tags] |
| Commit SHA (40-char) | `c24089804ee66ece4bec6f14638cb98985389cdb` | [VERIFIED: gh API] |
| Tag date | 2026-05-22T15:37:53Z | [VERIFIED: gh api repos/perplexityai/bumblebee/commits/v0.1.1] |
| Commits ahead on main | 6 commits (through 2026-05-29) | [VERIFIED: github.com/perplexityai/bumblebee/commits/main] |
| Go version | `go 1.25` | [VERIFIED: go.mod base64 decoded] |
| Module path | `github.com/perplexityai/bumblebee` | [VERIFIED: go.mod] |
| External dependencies | Zero (no go.sum) | [VERIFIED: top-level file listing at v0.1.1 — go.sum absent] |
| LICENSE | Apache-2.0 | [VERIFIED: github.com/perplexityai/bumblebee tree] |

### Real Internal Package Layout (vs PRD §5.2 Assumption)

| PRD §5.2 Assumption | Reality (Verified) | Impact |
|--------------------|--------------------|--------|
| `internal/resolver/resolver.go` | DOES NOT EXIST — root discovery is in `cmd/bumblebee/roots.go` | No `internal/resolver/` package to rename; only `cmd/` changes needed |
| `internal/output/ndjson.go` | Actual files: `internal/output/output.go` + `internal/output/httpsink.go` | Rename search target is correct; filename differs |
| `cmd/bumblebee/` exists | CONFIRMED | Rename to `cmd/pollen/` as planned |
| Selftest in `cmd/bumblebee/selftest.go` | CONFIRMED + `selftest/` subdirectory with `catalog.json` + `fixtures/{npm-fixture,pypi-fixture,mcp-fixture}` | Embedded via `go:embed all:selftest/fixtures selftest/catalog.json` |

[VERIFIED: gh api repos/perplexityai/bumblebee/contents/* + WebFetch on repo tree]

### Key Upstream Internal Packages

| Package | Files | Role |
|---------|-------|------|
| `internal/ecosystem` | (unknown count) | Per-ecosystem file parsers |
| `internal/endpoint` | `endpoint.go`, `endpoint_test.go` | `model.Endpoint` struct + `Current()` constructor |
| `internal/exposure` | unknown | Catalog matching logic |
| `internal/model` | `model.go`, `model_test.go` | Record/Finding/ScanSummary/Diagnostic structs |
| `internal/normalize` | unknown | Package name normalization |
| `internal/output` | `output.go`, `output_test.go`, `httpsink.go`, `httpsink_test.go` | NDJSON + HTTP output |
| `internal/scanner` | `scanner.go`, `scanner_test.go`, `scanner_integration_test.go`, `findings_test.go` | Scan orchestrator (4 goroutines, 256-item channel) |
| `internal/walk` | unknown | Filesystem walker |

### Upstream CI (upstream `.github/workflows/ci.yml`)

| Property | Value |
|----------|-------|
| OS matrix | `[ubuntu-latest, macos-latest]` — **NO Windows** |
| Go version | `"1.25"` with `check-latest: true` |
| Test commands | `gofmt -l .`, `go vet ./...`, `go test -race ./...`, `./bumblebee selftest` |
| Separate govulncheck job | Yes (`govulncheck ./...` on ubuntu-latest) |
| Signing/SBOM | None in ci.yml; release.yml uses GoReleaser (no cosign, no SBOM) |

Pollen's CI will **exceed** upstream's: adds Windows to the matrix, adds the differential test, adds the repro-build check.

### Upstream Selftest: Verified Behavior

- Subcommand: `bumblebee selftest` (maps to → `pollen selftest`)
- Embedded directive: `//go:embed all:selftest/fixtures selftest/catalog.json`
- Fixture directories: `selftest/fixtures/{npm-fixture, pypi-fixture, mcp-fixture}`
- Expected findings: **3** (npm-fixture, pypi-fixture, mcp-fixture each yield 1 finding against embedded catalog)
- RunID: Not asserted in selftest (new random hex each run)
- Network calls: None (all local)
- Temp dir cleanup: deferred `os.RemoveAll`

[VERIFIED: github.com/perplexityai/bumblebee selftest.go]

---

## Beekeeper Patterns to Mirror

### `.goreleaser.yaml` for Pollen

Derive from beekeeper's `.goreleaser.yaml` with these adaptations:

```yaml
version: 2

# No before.hooks: Pollen has no eBPF code generation

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
    mod_timestamp: "{{ .CommitTimestamp }}"
    flags:
      - -trimpath
      - -buildvcs=false          # CRITICAL: upstream lacks this; add it
    ldflags:
      - -s -w
      - -X main.Version={{.Version}}   # NOTE: main.Version, not a version package
    binary: pollen               # was: bumblebee

checksums:
  name_template: "checksums.txt"

signs:
  - cmd: cosign
    signature: "${artifact}.sigstore.json"
    args:
      - sign-blob
      - "--bundle=${signature}"
      - "${artifact}"
      - "--yes"
    artifacts: checksum

sboms:
  - id: pollen-sbom
    cmd: syft
    artifacts: archive
    args:
      - "$artifact"
      - "--output"
      - "cyclonedx-json=$document"
    documents:
      - "${artifact}.cdx.json"

archives:
  - formats: [tar.gz]
    format_overrides:
      - goos: windows
        formats: [zip]
```

Key difference from beekeeper: ldflags uses `-X main.Version` not `-X github.com/bantuson/pollen/internal/version.Version` (pollen keeps version in `main` package).

### `release.yml` for Pollen

Mirror beekeeper's `release.yml` verbatim, with pollen-specific tag trigger:

```yaml
on:
  push:
    tags:
      - 'v*'
```

The SLSA provenance job (`slsa-github-generator@v2.1.0`) requires the `provenance` job to use `needs: [goreleaser]` and pass `base64-subjects: "${{ needs.goreleaser.outputs.hashes }}"`. This is identical to beekeeper — no changes needed.

### `ci.yml` for Pollen

```yaml
strategy:
  fail-fast: false
  matrix:
    os: [ubuntu-latest, macos-latest, windows-latest]
    go: ['1.25.x']

# Per-OS test job (all 3 OSes):
steps:
  - go mod verify
  - go build -v -trimpath -buildvcs=false ./...
  - go test -v -race ./...          # CGO_ENABLED: 1 in env
  - go vet ./...
  - pollen selftest
  - go mod tidy + git diff --exit-code go.mod go.sum

# Differential job (Linux + macOS only):
  if: runner.os != 'Windows'
  steps:
    - Build upstream bumblebee from pinned commit SHA
    - Build pollen
    - Run both against shared fixture
    - Normalize NDJSON (strip non-det fields, sort by record_id)
    - Assert normalized outputs are identical
```

Windows test skips: upstream's `main_test.go` contains explicit `t.Skipf("profile defaults are darwin/linux specific")` for darwin-only tests. These will skip automatically on Windows — no additional skip wrappers needed for upstream-inherited tests. For any NEW Windows-specific tests in Phase 1 (there are none), the convention is `t.Skip("Windows root resolver arrives in Phase 2")` with a structured reason.

---

## Code Examples

### Verified: Version string (upstream pattern)

```go
// Source: github.com/perplexityai/bumblebee/cmd/bumblebee/version.go [VERIFIED]
// Variable declared at package level in main:
var Version string // set by: -ldflags "-X main.Version=..."

// fileDefault hardcoded as fallback:
const fileDefault = "0.1.1" // update to "0.1.1-pollen.1" in pollen fork
```

### Verified: selftest go:embed directive

```go
// Source: github.com/perplexityai/bumblebee/cmd/bumblebee/selftest.go [VERIFIED]
//go:embed all:selftest/fixtures selftest/catalog.json
var selftestFS embed.FS

// expectedSelftestFindings = 3
// Temp dir: os.MkdirTemp("", "bumblebee-selftest-*")  ← rename to "pollen-selftest-*"
```

### Verified: NDJSON non-deterministic fields (for normalization harness)

```go
// Source: github.com/perplexityai/bumblebee/cmd/bumblebee/main.go [VERIFIED]
// run_id generation:
func newRunID() string {
    var b [16]byte
    _, _ = rand.Read(b[:])
    return hex.EncodeToString(b[:])
}

// scan_time:
scanStart := time.Now().UTC()
// set on base record: ScanTime: scanStart.Format(time.RFC3339Nano)

// end_time + duration_ms: set on scan_summary at completion
```

### Verified: Differential harness sketch

```go
// Source: [ASSUMED] — pattern derived from field analysis, not from upstream code
// Fields to strip before comparison:
var stripFields = []string{
    "run_id", "scan_time", "end_time", "duration_ms",
    // endpoint sub-fields:
    "endpoint.hostname", "endpoint.username", "endpoint.uid",
}
// Sort key: "record_id" (stable SHA-256 content key) [VERIFIED: model.go StableID()]
```

### Pattern: UPSTREAM.md content for FORK-02

```markdown
# Upstream Sync

upstream: github.com/perplexityai/bumblebee
pinned commit: c24089804ee66ece4bec6f14638cb98985389cdb
pinned tag: v0.1.1
pinned date: 2026-05-22
verified by: bantuson

[sync workflow documentation follows...]
```

---

## Runtime State Inventory

> Phase 1 is a greenfield repo creation — not a rename/refactor phase. Pollen does not exist yet.

| Category | Items Found | Action Required |
|----------|-------------|-----------------|
| Stored data | None — pollen repo does not exist yet | None |
| Live service config | None | None |
| OS-registered state | None | None |
| Secrets/env vars | None pre-existing; GITHUB_TOKEN provided by Actions environment | None — token is ephemeral per job |
| Build artifacts | None — first build | None |

**Note for executor:** The pollen directory `C:\Users\Bantu\mzansi-agentive\pollen` does not currently exist and must be created as a new git repo before any code tasks execute.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|-------------|-----------|---------|----------|
| git | Fork clone + all pollen commits | ✓ | (system git) | — |
| Go 1.25 | Build + test | verify in CI | go-version-file: go.mod | — |
| cosign | Release signing | CI-only (sigstore/cosign-installer@v3) | v3.0.6 | — |
| syft | SBOM generation | CI-only (anchore/sbom-action/download-syft@v0) | latest | — |
| GoReleaser v2 | Release build | CI-only (goreleaser/goreleaser-action@v7) | v2.16.0 | — |
| MSVC/CGO | `-race` on Windows CI | CI-only (windows-latest has MSVC) | — | Skip -race on Windows (documented) |
| gh CLI | Phase tasks that interact with GitHub API | ✓ (dev machine) | — | — |

**Missing dependencies with no fallback:** None that block Phase 1 execution. All release/signing tooling is CI-only.

**Local dev machine (Windows) constraint:** `make verify-release` requires `make` (not installed). This is a known deferred item from beekeeper Phase 1. For pollen, the double-build repro check runs in CI. Locally, `go build` can be verified manually.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | cosign OIDC cert identity shape is `https://github.com/bantuson/pollen/.github/workflows/release.yml@refs/tags/v...` | Architecture Patterns §5 | If identity string format differs, the THREAT-MODEL.md `--certificate-identity-regexp` will need correction after first release. Build still works; verification fails with wrong regexp. |
| A2 | `go test -race ./...` on windows-latest CI passes with `CGO_ENABLED: 1` (MSVC present) | Common Pitfalls §5 | If MSVC absent on windows-latest, `-race` fails; mitigation: add `if: runner.os != 'Windows'` guard or `CGO_ENABLED: 0` on Windows (accept no race detection on Windows for Phase 1) |
| A3 | `windows-latest` GitHub Actions runner has MSVC available for CGO | Environment Availability | Same as A2 |
| A4 | upstream `internal/normalize`, `internal/walk`, `internal/ecosystem` package layouts are preserved verbatim at v0.1.1 (not inspected directly; internal packages are not directly consumed by pollen's additions in Phase 1) | Architecture | If any package has hardcoded "bumblebee" strings in help/error text, those need additional renaming. Low risk: internal packages are unlikely to have binary-name strings. |
| A5 | 6 commits on main since v0.1.1 are NOT being absorbed into Phase 1 (we pin to v0.1.1 SHA per CONTEXT.md) | Standard Stack | If a security fix is in those 6 commits, it's deferred. The PRD explicitly defers sync to Phase 5. |

**If this table is empty:** All claims were verified. It is not empty — A1–A3 are material.

---

## Open Questions

1. **Does upstream `main_test.go` have any tests that compile-fail on Windows (not just skip)?**
   - What we know: `main_test.go` uses `t.Skipf("profile defaults are darwin/linux specific")` for darwin-only tests.
   - What's unclear: Whether any tests reference `syscall` or Unix-specific APIs that would cause a compile error (not just a skip) on Windows.
   - Recommendation: Run `go build ./...` with `GOOS=windows` locally before committing. If compile errors surface, add build tags to those test files.

2. **Should we add `upstream` as a git remote in the pollen repo during Phase 1?**
   - What we know: CONTEXT.md marks this as Claude's Discretion; Phase 5 is when sync is "executed."
   - What's unclear: Whether having the remote configured from day 1 aids the Phase 5 sync workflow with no downside.
   - Recommendation: Yes, add `git remote add upstream https://github.com/perplexityai/bumblebee.git` during repo initialization. It costs nothing and makes UPSTREAM.md's sync workflow immediately usable.

3. **Does `govulncheck` belong in pollen's CI from Phase 1?**
   - What we know: Upstream runs `govulncheck ./...` on ubuntu-latest. With zero external deps, there are no transitive vulnerabilities to find.
   - What's unclear: Whether govulncheck is useful for stdlib-only modules.
   - Recommendation: Include it (mirrors upstream's security posture; trivially fast with zero deps).

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` package (no test framework dependency) |
| Config file | `go.mod` (`go test` is the runner) |
| Quick run command | `go test ./cmd/pollen/...` |
| Full suite command | `go test -race -count=1 ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| FORK-01 | `pollen` binary builds on all 3 OSes | build smoke | `go build -trimpath -buildvcs=false ./cmd/pollen` | ❌ Wave 0 (repo doesn't exist yet) |
| FORK-02 | UPSTREAM.md has correct SHA + date | manual review | `grep c24089804ee66ece4bec6f14638cb98985389cdb UPSTREAM.md` | ❌ Wave 0 |
| FORK-03 | Repro build: two builds hash-identical | build verification | `make verify-release VERSION=0.1.1-pollen.1` (CI only) | ❌ Wave 0 |
| FORK-04 | "bumblebee" absent from cmd names, package names, binary name | grep audit | `grep -r "bumblebee" cmd/ go.mod --include="*.go" \| grep -v "NOTICE\|UPSTREAM\|attribution"` | ❌ Wave 0 |
| PTEST-02 | Differential: pollen == bumblebee on Linux+macOS | differential CI job | `go test ./cmd/pollen/ -run TestDifferential -v` or CI job | ❌ Wave 0 |
| PTEST-03 | selftest: 3 findings on all 3 OSes | selftest | `pollen selftest` (exit 0) | ❌ Wave 0 (repo doesn't exist) |
| SDEF-02 | CycloneDX SBOM present in release artifacts | release CI | Verified by GoReleaser syft stanza producing `*.cdx.json` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go build ./cmd/pollen && pollen selftest`
- **Per wave merge:** `go test -race -count=1 ./...` on Linux
- **Phase gate:** Full CI matrix green (all 3 OSes) + differential test green on Linux+macOS + `v0.1.1-pollen.1` tag signed before `/gsd-verify-work`

### Wave 0 Gaps

All of the following must be created during Phase 1 plan execution (the repo itself is Wave 0):

- [ ] `C:/Users/Bantu/mzansi-agentive/pollen/` — new git repo, initialize
- [ ] Clone/fork upstream at SHA `c24089804ee66ece4bec6f14638cb98985389cdb`
- [ ] `go.mod` — module path renamed
- [ ] `cmd/pollen/` — directory renamed, `main.Version` ldflags path confirmed
- [ ] All attribution files: `LICENSE`, `NOTICE`, `CHANGES.md`, `UPSTREAM.md`
- [ ] `VERSION` — `0.1.1-pollen.1`
- [ ] `Makefile` — mirroring beekeeper's repro-build targets
- [ ] `.goreleaser.yaml` — adding `-buildvcs=false`, cosign signing, syft SBOM, pollen binary name
- [ ] `.github/workflows/ci.yml` — 3-OS matrix + differential test + govulncheck
- [ ] `.github/workflows/release.yml` — cosign + SLSA Level 3
- [ ] Differential test code (normalization harness) — `go test` runnable

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | n/a (CLI tool, no auth surface) |
| V3 Session Management | No | n/a |
| V4 Access Control | No | n/a |
| V5 Input Validation | Partial | Upstream's flag parsing; `--exposure-catalog` path validation |
| V6 Cryptography | Yes | cosign keyless OIDC (no hand-rolled crypto); syft SBOM |
| V10 Malicious Code | Yes | Pinned upstream commit; diff review before each absorption |
| V14 Configuration | Yes | Reproducible builds; SLSA Level 3 provenance |

### Known Threat Patterns for Fork Supply Chain

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Compromised upstream commit absorption | Tampering | Pin to explicit commit SHA; diff review; don't auto-absorb main |
| Malicious dependency injection | Tampering | Zero external deps (verified); `go mod verify` in CI |
| Build artifact substitution | Tampering | cosign sign-blob on checksums.txt; SLSA Level 3 provenance |
| Stale/unsigned release artifact | Repudiation | Sigstore transparency log; SBOM records source commit |
| "bumblebee" trademark misuse | — | FORK-04 discipline; trademark check in CI (grep audit) |

---

## Sources

### Primary (HIGH confidence)

- `github.com/perplexityai/bumblebee` @ v0.1.1 — structure, go.mod, cmd/bumblebee/*, internal/*, go:embed, selftest behavior, NDJSON fields, version injection, goreleaser config, CI config
- `gh api repos/perplexityai/bumblebee/git/refs/tags` — v0.1.1 SHA: `c24089804ee66ece4bec6f14638cb98985389cdb`
- `gh api repos/perplexityai/bumblebee/commits/v0.1.1` — commit date 2026-05-22T15:37:53Z
- `beekeeper/.goreleaser.yaml` — exact build flags, cosign stanza, syft stanza to mirror
- `beekeeper/.github/workflows/release.yml` — SLSA Level 3 pattern (slsa-github-generator@v2.1.0)
- `beekeeper/.github/workflows/ci.yml` — 3-OS matrix pattern, CGO_ENABLED: 1 for -race
- `beekeeper/Makefile` — verify-release double-build pattern, GOFLAGS, LDFLAGS

### Secondary (MEDIUM confidence)

- `github.com/perplexityai/bumblebee/commits/main` — 6 commits on main since v0.1.1
- `github.com/goreleaser/goreleaser/releases/latest` — v2.16.0 (May 2026)
- `github.com/sigstore/cosign/releases/latest` — v3.0.6

### Tertiary (LOW confidence)

- A2/A3 (`windows-latest` CGO availability): inferred from beekeeper STATE.md note about race detector + CI — not empirically tested for pollen

---

## Metadata

**Confidence breakdown:**
- Upstream structure: HIGH — directly fetched from GitHub API + WebFetch
- NDJSON determinism analysis: HIGH — all four non-det fields verified from source code
- Standard stack: HIGH — zero deps confirmed from go.mod + absent go.sum
- Architecture (pollen layout): HIGH — derived from verified upstream + locked CONTEXT.md decisions
- Pitfalls: HIGH for pitfalls 1–4 (verified from source); MEDIUM for pitfalls 5–7 (inferred from beekeeper experience)
- Security (cosign identity): MEDIUM — identity shape is standard GitHub Actions OIDC format but needs first-release validation

**Research date:** 2026-06-01
**Valid until:** 2026-07-01 (upstream is in active development; v0.1.1 is the only tag, but main has 6+ commits; this research is pinned to v0.1.1 SHA, so it remains valid as long as the fork pins to that SHA)
