# Phase 5: Contribution-Back & Milestone Close — Research

**Researched:** 2026-06-03
**Domain:** Multi-repo release orchestration, Windows ETW Sentry testing, Go subprocess-binary pinning, self-catalog extension, upstream-sync documentation
**Confidence:** HIGH (all findings verified against live code/config; no external API research required)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-2:** SYNC-02 DESCOPED. No contribution-back PRs against `perplexityai/bumblebee` this milestone. UPSTREAM.md records the prepared patch set + contribution-back-deferred rationale. SC2 relaxed/deferred — verifier MUST NOT flag absence of upstream PR.
- **D-3:** GitHub push is IN SCOPE. Both `bantuson/pollen` and `bantuson/beekeeper` may be pushed to GitHub this phase. Neither is pushed yet.
- **D-4:** Cut all four signed tags this phase — `v0.1.1-pollen.2`, `.3`, `.4` (deferred) and `.5` (milestone-close) via the existing cosign keyless / GitHub Actions OIDC pipeline in `../pollen`.
- **D-5:** Outward/auth-gated steps (`gh repo create`, `git push`, tag-push triggering signing, `cosign verify`) are `autonomous: false` checkpoint tasks. Executor does all local prep + exact release runbook; maintainer performs/approves outward steps.
- **D-6:** SYNC-01 (UPSTREAM.md repeatable sync workflow) ships; must be followable by a second maintainer cold (SC1). UPSTREAM.md already exists — extend/verify, don't recreate.
- **D-7:** BKINT-02: beekeeper pins Pollen at an explicit version; beekeeper CI installs Pollen; Windows inventory-test skip baseline is **zero** after this phase.
- **D-8:** PTEST-05: Windows honeypot E2E uses **synthetic** `%USERPROFILE%\.aws\credentials` (NOT real credentials). Must fire SENTRY-005 (exfil-signature-fusion rule) on Windows CI runner.
- **D-9:** SDEF-01: `pollen-self` entries added to the unified `beekeeper-self` catalog (`internal/catalog/selfcatalog.go`). NOT a separate catalog (SELF-02 is v2). `beekeeper selftest` must stay green.

### Claude's Discretion

- Plan/wave structure across the three repos; sequencing of signed-tag cuts (pollen.2→3→4→5).
- Exact `pollen-self` entry shape (version identifiers, hashes) consistent with `selfcatalog.go` schema.
- Honeypot test harness layout consistent with existing Sentry rule-fixture pattern.
- Whether BKINT-02 needs a beekeeper CI workflow edit to `go install` Pollen, and how Windows resolves the `pollen` binary on PATH.
- Precise content/structure of the release runbook for checkpointed steps (D-5).

### Deferred Ideas (OUT OF SCOPE)

- SYNC-02 / roadmap SC2 (upstream contribution-back PRs to perplexityai/bumblebee) — deferred to a future milestone.
- SELF-02 (separate `pollen-self` catalog) — v2.
- DIST-01 (public Pollen binary releases) — v2.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SYNC-01 | Documented, repeatable upstream sync workflow in UPSTREAM.md — second-maintainer-followable with real commands | UPSTREAM.md already has 8-step workflow stub; delta to extend with pollen.2/3/4 sync history, contribution-deferred rationale, and concrete version-history table entries |
| SYNC-02 | DESCOPED per D-2 — no upstream PRs | Verifier must accept absence; UPSTREAM.md records rationale |
| BKINT-02 | beekeeper go.mod pins Pollen at explicit version; CI installs Pollen; Windows skip baseline zero | Findings confirm: binary-only subprocess boundary (no Go import); pin = CI `go install @version`; no go.mod module import needed |
| PTEST-05 | Windows honeypot E2E — planted process tree reads synthetic `.aws/credentials` + outbound → SENTRY-005 fires on Windows | SENTRY-005 (`evalSENTRY005`) is the exfil-fusion rule; test pattern uses `EvaluateEvent` directly (no ETW daemon); synthetic events fed via `SentryEvent` structs |
| SDEF-01 | `pollen-self` entries in unified `beekeeper-self` catalog; `beekeeper selftest` passes | `selfCatalogEntry` schema confirmed; add entries for pollen.2/.3/.4 compromised-version scenarios; extend `selftestEntries` in `selftest.go` |
</phase_requirements>

---

## Summary

Phase 5 is the final phase of Beekeeper v1.1.0 "Pollen". It closes the milestone across two repos — `bantuson/pollen` (signed-release batch) and `bantuson/beekeeper` (CI pin, Windows honeypot, pollen-self catalog) — and ships the UPSTREAM.md sync runbook.

The five work streams are fully concrete from the live codebase:

1. **SYNC-01 (UPSTREAM.md):** The file already contains an 8-step sync workflow. The delta is: add the pollen.2/3/4 version-history table rows, add the contribution-back-deferred note (D-2 rationale), and add a "prepared patch set" appendix listing the Windows diffs available for upstream if they ever want them.

2. **BKINT-02 (Pollen pin + CI):** Beekeeper consumes Pollen exclusively as a subprocess binary (`lookPollenFn` + `runPollenFn` in `internal/scan/scanner.go`). There is no Go module import of `github.com/bantuson/pollen`. The correct BKINT-02 interpretation is: beekeeper CI installs the Pollen binary at a pinned version via `go install github.com/bantuson/pollen/cmd/pollen@v0.1.1-pollen.4`, adds a beekeeper `go.mod` `tool` or comment directive recording the pinned version, and adds a new step to `.github/workflows/ci.yml`. No go.mod module import is needed or appropriate.

3. **PTEST-05 (Windows honeypot):** SENTRY-005 (`evalSENTRY005`) is the exfil-signature-fusion rule. The test pattern is identical to the existing `rules_test.go` approach: construct a `map[uint32]ProcessNode` editor process tree, call `EvaluateEvent` directly with synthetic `SentryEvent` structs (no ETW daemon, no real filesystem writes, no live network). The test file lives in `internal/sentry/windows/` with `//go:build windows`. Credentials file is planted as a path string in a `SentryEvent{Kind: EventFileAccess, FilePath: ...}` — no actual file is created.

4. **SDEF-01 (pollen-self):** The `selfCatalogEntry` schema is fully defined in `selfcatalog.go`. Adding pollen-self entries means: (a) extending `selftestEntries` in `internal/check/selftest.go` with two new entries (one beekeeper-self entry for a hypothetical bad pollen release, one for a bad beekeeper release as regression anchor), and (b) adding a corresponding fixture to `internal/catalog/testdata/` and `internal/check/corpus/fixtures.json`. The `selfCatalogAdapter.LookupAll` only matches on `ecosystem == "beekeeper"` — pollen entries use `ecosystem: "beekeeper"`, `package: "pollen"` with a distinct identifier.

5. **D-4 (signed-tag batch):** `../pollen` is 14 commits ahead of origin/main. The remote `origin` is already configured as `https://github.com/Bantuson/pollen.git`. Pollen.2/3/4 commit hashes are known. The `.goreleaser.yaml` and `release.yml` pipeline is confirmed intact. The exact runbook commands are documented in the prior SUMMARY files.

**Primary recommendation:** Implement in three waves — (1) local autonomous work (UPSTREAM.md delta, BKINT-02 CI edit, PTEST-05 test, SDEF-01 entries + VERSION/CHANGES pollen.5), (2) checkpoint: maintainer pushes pollen main + beekeeper main, cuts four tags in order, verifies cosign, (3) post-push verification: CI green on all 3 OSes, selftest passes, zero skips.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Pollen version pin | CI / Build | beekeeper source comment | Binary-only boundary; `go install @version` in CI; version recorded in beekeeper source for auditability |
| Windows honeypot rule firing | `internal/sentry` (pure rule engine) | `internal/sentry/windows/` (test file) | `EvaluateEvent` is OS-agnostic; test lives under `windows/` build tag to run only on Windows CI |
| pollen-self catalog entries | `internal/catalog/selfcatalog.go` (schema) | `internal/check/selftest.go` (test entries) | Feed schema owns the data shape; selftest owns the in-binary test corpus |
| Signed release pipeline | GitHub Actions CI (`../pollen/.github/workflows/release.yml`) | GoReleaser + cosign | Tag push triggers the pipeline; maintainer controls tag push (D-5) |
| UPSTREAM.md sync doc | `../pollen/UPSTREAM.md` | None | Pollen-repo artifact; beekeeper CI does not consume it |

---

## Standard Stack

### Core (confirmed via live codebase)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/tekert/golang-etw` | v0.6.2 | ETW event ingestion (Windows Sentry) | Already in beekeeper go.mod; used in `internal/sentry/windows/etw.go` |
| `crypto/ed25519` (stdlib) | Go 1.25 | Self-catalog feed signing/verification | Already used in `selfcatalog.go`/`selfkey.go`; no new dep |
| `cosign` (GitHub Actions) | v3 (sigstore/cosign-installer@v3) | Keyless OIDC signing of Pollen releases | Already wired in `release.yml` |
| `goreleaser` | ~v2 (goreleaser-action@v7) | Multi-platform binary builds + SBOM | Already wired; `checksum:` v2 schema confirmed |
| `syft` (GitHub Actions) | anchore/sbom-action | CycloneDX SBOM per archive | Already wired; `sboms:` block in `.goreleaser.yaml` |
| `slsa-github-generator` | @v2.1.0 (full semver — locked) | SLSA Level 3 provenance | Already wired; CRITICAL: must stay `@v2.1.0` per CLAUDE.md |

[VERIFIED: live codebase — go.mod, .goreleaser.yaml, release.yml]

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `go install` (Go toolchain) | 1.25.x | Install Pollen binary in CI at pinned version | BKINT-02: new CI step |
| `encoding/json` (stdlib) | Go 1.25 | Sign/verify test feed payloads in selftest | Already used in selfcatalog_test.go helpers |

**Installation (beekeeper CI — new step):**
```bash
go install github.com/bantuson/pollen/cmd/pollen@v0.1.1-pollen.4
```

**Version verification (confirmed):**
```bash
# pollen VERSION file is confirmed at 0.1.1-pollen.4 (HEAD a9db7b3)
# No npm view needed — internal Go tooling only
```

[VERIFIED: live codebase — ../pollen/VERSION, go.mod]

---

## Architecture Patterns

### System Architecture Diagram

```
 ┌─────────────────────────────────────────────────────────────────┐
 │  Phase 5 data flow                                              │
 │                                                                 │
 │  ../pollen (local repo)                                         │
 │    VERSION=0.1.1-pollen.4, CHANGES.md ready                     │
 │        │                                                        │
 │        ▼ (bump to pollen.5 + UPSTREAM.md delta)                 │
 │    VERSION=0.1.1-pollen.5, CHANGES.md, UPSTREAM.md             │
 │        │                                                        │
 │        ▼ [CHECKPOINT D-5: maintainer pushes + tags]             │
 │    github.com/Bantuson/pollen (origin/main + 4 tags)            │
 │        │                                                        │
 │        ▼ (release.yml triggered per tag)                        │
 │    GitHub Release: pollen.2/3/4/5 (cosign + SBOM + SLSA L3)    │
 │                                                                 │
 │  beekeeper (this repo)                                          │
 │    internal/sentry/windows/                                     │
 │        │ EventProcessCreate (editor tree)                       │
 │        │ EventFileAccess (.aws/credentials path)                │
 │        │ EventNetworkConnect (outbound IP:port)                 │
 │        ▼                                                        │
 │    internal/sentry.EvaluateEvent                                │
 │        │                                                        │
 │        ▼ SENTRY-005 fires? → SentryAlert (PTEST-05)            │
 │                                                                 │
 │    internal/catalog/selfcatalog.go                              │
 │        │ selfCatalogEntry{ecosystem:"beekeeper",pkg:"pollen"}   │
 │        ▼                                                        │
 │    internal/check/selftest.go (selftestEntries extended)        │
 │        │                                                        │
 │        ▼ beekeeper selftest PASS (SDEF-01)                      │
 │                                                                 │
 │    .github/workflows/ci.yml                                     │
 │        │ new step: go install pollen@v0.1.1-pollen.4            │
 │        ▼                                                        │
 │    Windows CI: zero skips (BKINT-02)                           │
 └─────────────────────────────────────────────────────────────────┘
```

### Recommended Project Structure (affected files only)

```
../pollen/
├── VERSION                    # bump 0.1.1-pollen.4 → 0.1.1-pollen.5
├── CHANGES.md                 # prepend v0.1.1-pollen.5 section
└── UPSTREAM.md                # add version history rows + contribution-deferred note

beekeeper/
├── .github/workflows/ci.yml   # new "Install Pollen" step
├── internal/
│   ├── sentry/
│   │   └── windows/
│   │       └── honeypot_test.go  # NEW: PTEST-05 (//go:build windows)
│   ├── catalog/
│   │   └── testdata/
│   │       └── selfcatalog_match_pollen.json  # NEW: pollen-self fixture
│   └── check/
│       ├── selftest.go        # extend selftestEntries with pollen-self entry
│       └── corpus/fixtures.json  # add pollen-self selftest fixture
└── docs/
    └── release-runbook.md     # NEW: exact D-5 commands for maintainer
```

---

## Research Question Answers

### RQ-1: BKINT-02 pin shape — binary subprocess or Go module import?

**Answer: CI `go install @pinned-version` only. No go.mod module import.**

[VERIFIED: internal/scan/scanner.go lines 56-97]

`internal/scan/scanner.go` uses two injectable package-level vars:
```go
var lookPollenFn = func() (string, error) { return exec.LookPath("pollen") }
var runPollenFn = func(ctx context.Context, deep bool) (<-chan []byte, bool) {
    return defaultRunPollen(ctx, deep)
}
```

`defaultRunPollen` calls `exec.CommandContext(ctx, bin, args...)` where `bin` is the result of `exec.LookPath("pollen")`. There is **no** `import "github.com/bantuson/pollen/..."` anywhere in beekeeper's Go source. The subprocess boundary is the only integration point.

**go.mod analysis:** beekeeper's `go.mod` has no reference to `github.com/bantuson/pollen`. Adding one would require beekeeper to import a Go package from Pollen, which doesn't exist as an exported API and would violate the subprocess isolation boundary (BKINT-01 decision).

**Correct BKINT-02 implementation:**
- Add a step to `.github/workflows/ci.yml` before the `Test` step (on all three OS matrix runners):
  ```yaml
  - name: Install Pollen (BKINT-02)
    run: go install github.com/bantuson/pollen/cmd/pollen@v0.1.1-pollen.4
  ```
- Record the pinned version as a comment in `internal/scan/scanner.go` or a new `internal/scan/pollen_version.go` file (const string, not a Go dependency).
- The `go install` adds pollen to `$GOPATH/bin` which is on PATH in GitHub Actions runners.

**Sequencing constraint (D-3):** `go install @v0.1.1-pollen.4` requires Pollen to be pushed AND tagged on GitHub first. The CI workflow edit must be committed BEFORE or in the same push as the tag. The planner must sequence: (1) prepare CI edit locally, (2) checkpoint: push pollen + cut pollen.2/3/4/5 tags, (3) then push beekeeper main with CI edit.

[VERIFIED: beekeeper/.github/workflows/ci.yml — no pollen install step currently; ../pollen remote is origin=https://github.com/Bantuson/pollen.git]

### RQ-2: Beekeeper CI Windows-green status

**Current state (confirmed from ci.yml):**

The existing `test` job runs on `[ubuntu-latest, macos-latest, windows-latest]` with `go test -v -race ./...`. There is no Pollen install step. The `TestPollenCompatibility` test (PTEST-04) uses `runPollenFn` injection and is **fixture-driven with zero t.Skip** — it passes on all three OSes without a real binary.

**What needs to change for BKINT-02:**
- Add `go install github.com/bantuson/pollen/cmd/pollen@v0.1.1-pollen.4` step.
- Add `go install` to `GOPATH/bin` on PATH (standard in GitHub Actions runners via `actions/setup-go`).
- No existing tests skip on Windows pending Pollen. PTEST-04 (`TestPollenCompatibility`) already has zero skips. The only Windows-skip in the scan package is `TestScanBumblebeeUnavailable` — wait, checking: the test is called `TestScanPollenUnavailable` and it runs on all platforms (uses the mock fn).

**PTEST-05 (new honeypot test)** is the test that requires the Windows runner and `//go:build windows`. It will live in `internal/sentry/windows/honeypot_test.go` — the existing Windows Sentry test pattern.

[VERIFIED: .github/workflows/ci.yml, internal/scan/scanner_test.go]

### RQ-3: PTEST-05 honeypot test harness shape

**Rule to fire:** SENTRY-005 (`evalSENTRY005` in `internal/sentry/rules.go`)

**SENTRY-005 trigger conditions:**
1. `EventNetworkConnect` event arrives.
2. The sending PID is editor-descended (`isEditorDescendant`).
3. `state.CredAccessByPID[event.PID]` has a recent sensitive-file access within `ExfilFusionWindowMin` (default 5 min).
4. `inventory.RecentExtensions` has an extension installed within `ExfilFusionWindowMin`.

**Test pattern (from rules_test.go + parser_test.go):**

The existing test pattern in `internal/sentry/rules_test.go`:
```go
func editorTree() map[uint32]ProcessNode {
    return buildTree([]ProcessNode{
        {PID: 1, PPID: 0, Exe: "/usr/bin/cursor"},
        {PID: 100, PPID: 1, Exe: "/usr/bin/some-tool"},
    })
}
```

For Windows the exe paths must use Windows syntax. The honeypot test needs:
1. A process tree with an editor parent (e.g. `C:\Users\...\cursor.exe`).
2. A `EventFileAccess` event for `%USERPROFILE%\.aws\credentials` path (planted as a string — no file created).
3. A `InventorySnapshot` with a recently-installed extension.
4. A `EventNetworkConnect` event for an outbound IP (e.g. `203.0.113.1:443` — RFC 5737 documentation range).
5. Direct call to `sentry.EvaluateEvent`.

**No ETW daemon involvement.** The test calls `sentry.EvaluateEvent` directly — the same pattern as the existing 600-line `rules_test.go`. The `//go:build windows` tag means it only runs on the Windows CI runner. No real files are created; credentials path is a string literal in the `SentryEvent.FilePath` field.

**File placement:** `internal/sentry/windows/honeypot_test.go` with `//go:build windows` (matching the existing daemon_test.go / parser_test.go pattern).

**Windows-specific aspects:**
- Process tree uses `C:\Users\...\cursor.exe` or `%PROGRAMFILES%\cursor\cursor.exe` style exe paths (Windows backslash).
- FilePath is `filepath.Join(os.Getenv("USERPROFILE"), ".aws", "credentials")` — this resolves at test runtime on the Windows CI runner; `%USERPROFILE%` is `C:\Users\runneradmin` on GitHub Actions windows-latest.
- No actual file needs to exist. `sentry.EvaluateEvent` does not stat files — it evaluates the string via `isSensitivePath`.

**`isSensitivePath` analysis:** `rules.go` calls `strings.Contains(path, s)` for each entry in `defaultSensitivePaths`. The list includes `".aws/"`. On Windows, `filepath.Join(os.Getenv("USERPROFILE"), ".aws", "credentials")` produces `C:\Users\runneradmin\.aws\credentials` — which contains `\.aws\` not `.aws/`. **This is a pitfall:** the current `isSensitivePath` checks for `.aws/` (forward slash), but Windows paths use backslash.

[VERIFIED: internal/sentry/rules.go line 12: `".aws/"` — forward slash only]

The honeypot test must either:
- (a) Use `strings.ReplaceAll(path, "\\", "/")` normalization in `isSensitivePath` for Windows (code change), OR
- (b) Construct the `FilePath` in the test fixture with a forward-slash form: `"C:/Users/FAKE/.aws/credentials"` — this is a test fixture, not a real Windows path read from disk.

**Recommendation (b):** Use a forward-slash fixture path `"C:/Users/FAKEUSER/.aws/credentials"` in the test. SENTRY-005 needs the CredAccessByPID to be populated first (via a prior `EventFileAccess` with that path), and `isSensitivePath` must return true. Forward slash is sufficient for the path-matching string check. This avoids a production code change and keeps the honeypot test self-contained.

**Alternatively (code fix approach):** Normalize the path in `isSensitivePath` with `filepath.ToSlash` before matching. This is a real bug for production use on Windows (the ETW parser produces backslash paths; `isSensitivePath` won't recognize them). The plan should include this fix as part of PTEST-05 — it's a genuine production correctness issue.

[VERIFIED: internal/sentry/rules.go isSensitivePath + parser_test.go TestParseFileCreateEvent (uses `C:\Users\me\.ssh\id_rsa` with backslash)]

**Both approaches need to be presented to the planner.** The production-code-fix approach is more correct.

### RQ-4: SDEF-01 — pollen-self catalog entry schema

**Schema (confirmed from selfcatalog.go):**
```go
type selfCatalogEntry struct {
    ID            string   `json:"id"`
    Name          string   `json:"name"`
    Ecosystem     string   `json:"ecosystem"`
    Package       string   `json:"package"`
    Versions      []string `json:"versions"`
    Severity      string   `json:"severity"`
    CatalogSource string   `json:"catalog_source"`
}
```

**How `selfCatalogAdapter.LookupAll` works:** Only matches when `ecosystem == "beekeeper"`. The `package` field is the discriminator — existing entries use `package: "beekeeper"`. Pollen entries must use `ecosystem: "beekeeper"`, `package: "pollen"` with a new entry ID like `"pollen-self-2026-001"`.

**How `selftestEntries` is used (confirmed from selftest.go):** `selftestEntries` is a `[]catalog.Entry` (not `[]selfCatalogEntry`) used to build the hermetic mmap index for `beekeeper selftest`. It uses `catalog.Entry`, not the self-catalog schema. The self-catalog is a separate feed system checked via `CheckSelfCatalog`, not via `beekeeper check`.

**SDEF-01 implementation shape — two separate concerns:**

1. **Feed-level pollen-self entries:** In the actual `beekeeper-self` JSON feed (hosted at `selfCatalogDefaultFeedURL`), add entries for known-bad Pollen versions. The feed is maintained externally (not in the repo). For the selftest, what matters is the `selfCatalogAdapter` tests.

2. **Selftest coverage:** `TestSelfCatalogAdapter_LookupAll` in `selfcatalog_test.go` already tests the adapter. To extend for pollen-self: add a new test `TestSelfCatalogAdapter_PollenEntries` with a `selfCatalogEntry{Ecosystem:"beekeeper", Package:"pollen", Versions:["v0.1.1-pollen.4"]}` and assert `LookupAll("beekeeper", "pollen")` returns the match.

3. **`beekeeper selftest` coverage:** `RunSelftest` in `selftest.go` uses `selftestEntries` (a `catalog.Entry` slice for the mmap index, not the self-catalog feed). The "pollen-self" concept doesn't map directly to this path — the selftest exercises the catalog+policy engine, not `CheckSelfCatalog`. To make `beekeeper selftest` exercise the pollen-self path, the most natural approach is to add a fixture to `corpus/fixtures.json` that tests a pollen ecosystem entry being blocked (using the beekeeper catalog entry for a bad pollen package).

**Simplest correct approach for SDEF-01:**
- Add `selfCatalogEntry` records with `package: "pollen"` to `selfcatalog_test.go` unit tests.
- Add a `catalog.Entry` with `Ecosystem: "beekeeper", Package: "pollen"` to `selftestEntries` in `selftest.go`.
- Add a matching fixture to `corpus/fixtures.json` that expects a block for `ecosystem=beekeeper, package=pollen, version=v0.1.1-pollen.4` (hypothetical bad pollen version).
- The `beekeeper selftest` PASS count increases by 1 for the new fixture.

[VERIFIED: internal/catalog/selfcatalog.go, internal/catalog/selfcatalog_test.go, internal/check/selftest.go]

### RQ-5: UPSTREAM.md delta

**Current state (confirmed from ../pollen/UPSTREAM.md):**
- Fork metadata table: pinned commit + tag + date + verifier — complete.
- Sync workflow: 8-step procedure with prose — complete.
- Version history table: only pollen.1 row.

**Delta needed to satisfy SYNC-01 and D-6:**

1. **Version history table:** Add rows for pollen.2, pollen.3, pollen.4, pollen.5 (same upstream commit c24089804ee66ece — no upstream sync occurred; all four are Windows-addition releases, not upstream absorption releases).

2. **Contribution-back note:** New `## Contribution-back status` section explaining:
   - Windows additions prepared as patch set (WRES-01/02, WPATH-01/02, WEXT-01/02/03 — with commit references).
   - Upstream has open PRs #3/#4 for Windows support; their maintainers plan their own implementation.
   - Contribution-back deferred to a future milestone when upstream signals readiness.
   - The prepared patch set is available as commits `2c202ef..b906404` in this repo.

3. **Workflow concrete example:** The existing 8-step workflow is already concrete with real commands. No changes needed there per the current content.

[VERIFIED: ../pollen/UPSTREAM.md]

### RQ-6: Release pipeline state and runbook

**Pollen repo state (confirmed):**
- Remote `origin` = `https://github.com/Bantuson/pollen.git` (already configured).
- 14 commits ahead of `origin/main` (unpushed).
- No local tags (confirmed in SUMMARY files).
- `VERSION` = `0.1.1-pollen.4` (confirmed).
- Pipeline files: `.goreleaser.yaml` (confirmed intact), `.github/workflows/release.yml` (confirmed intact).

**Release pipeline mechanics (confirmed from release.yml):**
- Triggers on `push: tags: 'v*'`.
- `goreleaser` job: builds 3-OS × 2-arch binaries + checksums.txt + per-archive SBOM via syft.
- cosign keyless signing: `cosign sign-blob --bundle=checksums.txt.sigstore.json checksums.txt --yes`.
- `provenance` job (after goreleaser): SLSA Level 3 via `slsa-github-generator@v2.1.0`.
- Permissions required: `id-token: write` (for cosign OIDC) + `contents: write` (upload release assets).

**cosign identity casing (confirmed from pollen.1 fix commit 37c71e5):** GitHub OIDC uses `Bantuson` (capital B). The `--certificate-identity-regexp` in `cosign verify-blob` must be `^https://github\.com/Bantuson/pollen/`.

**Exact runbook for D-5 (verified against 02-04-SUMMARY.md + pollen CHANGES.md pattern):**

```bash
# Phase 5 prerequisite: pollen.5 VERSION+CHANGES prepared locally first.
# Then the maintainer runs these commands:

# Step 1: Create beekeeper repo on GitHub (if not done)
gh repo create bantuson/beekeeper --public --source=. --push
# OR if repo exists but remote not set:
git remote add origin https://github.com/bantuson/beekeeper.git

# Step 2: Push pollen main (14 commits ahead of origin/main)
git -C /path/to/pollen push origin main
gh -R Bantuson/pollen run watch  # wait: 3-OS CI green

# Step 3: Cut pollen.2 tag (at commit c94b271)
git -C /path/to/pollen tag -a v0.1.1-pollen.2 c94b271 \
  -m "Pollen v0.1.1-pollen.2 — Windows root resolver (WRES-01, WRES-02, PTEST-01)"
git -C /path/to/pollen push origin v0.1.1-pollen.2
gh -R Bantuson/pollen run watch  # wait for release job

# Step 4: Cut pollen.3 tag (at commit 19695e3)
git -C /path/to/pollen tag -a v0.1.1-pollen.3 19695e3 \
  -m "Pollen v0.1.1-pollen.3 — Windows path representation (WPATH-01, WPATH-02)"
git -C /path/to/pollen push origin v0.1.1-pollen.3
gh -R Bantuson/pollen run watch

# Step 5: Cut pollen.4 tag (at commit a9db7b3)
git -C /path/to/pollen tag -a v0.1.1-pollen.4 a9db7b3 \
  -m "Pollen v0.1.1-pollen.4 — Windows extension & MCP coverage (WEXT-01, WEXT-02, WEXT-03)"
git -C /path/to/pollen push origin v0.1.1-pollen.4
gh -R Bantuson/pollen run watch

# Step 6: Cut pollen.5 tag (at HEAD after Phase 5 local prep)
git -C /path/to/pollen tag -a v0.1.1-pollen.5 HEAD \
  -m "Pollen v0.1.1-pollen.5 — Milestone close (SYNC-01, BKINT-02, PTEST-05, SDEF-01)"
git -C /path/to/pollen push origin v0.1.1-pollen.5
gh -R Bantuson/pollen run watch

# Step 7: Cosign verify each release (download checksums.txt + .sigstore.json from GitHub Release first)
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github\.com/Bantuson/pollen/' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt
# Expected output: Verified OK

# Step 8: Push beekeeper main
git push origin main  # (from beekeeper repo)
gh run watch          # wait: beekeeper CI green on all 3 OSes
```

**Note on tag ordering:** Tags must be pushed in pollen.2→3→4→5 order. GoReleaser infers the version from the tag; pushing out-of-order creates no functional problem (tags are independent artifacts) but sequential is cleaner for CHANGES.md chronology.

**Note on beekeeper remote:** `git remote -v` returns empty for beekeeper — no remote is configured. `gh repo create bantuson/beekeeper --public --source=. --push` is the one-liner; or `git remote add origin https://github.com/bantuson/beekeeper.git && git push -u origin main` if the repo was created on GitHub web.

[VERIFIED: ../pollen/.goreleaser.yaml, ../pollen/.github/workflows/release.yml, ../pollen/CHANGES.md (pollen.1 fix noting Bantuson casing), ../pollen git status]

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Feed signing/verification | Custom signing scheme | ed25519 + `parseAndVerifySelfFeed` (already exists) | Schema fully defined; just add entries |
| Binary version pinning | Version comment only | `go install @version` + CI step | `go install` resolves the module graph; version comment alone is not machine-checked |
| SLSA provenance | Custom provenance generation | `slsa-github-generator@v2.1.0` (already wired) | Full semver pin is required; changing it breaks SLSA L3 |
| cosign verification | Manual hash check | `cosign verify-blob --bundle` (already documented) | OIDC-bound certificate identity check; manual hash is weaker |
| ETW event synthesis for tests | Live ETW session in tests | Direct `sentry.EvaluateEvent` call with synthetic `SentryEvent` structs | ETW sessions require admin; test pattern established in rules_test.go and parser_test.go |

---

## Runtime State Inventory

> This phase involves deploying new releases and pushing to GitHub — outward state changes.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | No database stores pollen version strings or beekeeper-self feed entries that need patching | None — feed is HTTP-fetched at runtime |
| Live service config | `origin` remote not configured in beekeeper repo; `origin` configured in pollen repo as `https://github.com/Bantuson/pollen.git` | beekeeper: set remote as checkpoint step (D-5); pollen: push to existing remote |
| OS-registered state | No Windows Task Scheduler / launchd / systemd tasks reference pollen or beekeeper version strings | None |
| Secrets/env vars | `GITHUB_TOKEN` (GitHub Actions built-in — no change); no `.env` files | None |
| Build artifacts | `../pollen`: 14 commits unpushed, 0 local tags. beekeeper: 0 commits unpushed | Push both as part of D-5 checkpoint |

**Nothing found in category:** OS-registered state — None, verified by reviewing daemon names (`BeekeeperSentry`, service registration); Stored data — None, self-catalog is HTTP-fetched; Secrets — None new (GITHUB_TOKEN is standard Actions credential).

---

## Common Pitfalls

### Pitfall 1: `isSensitivePath` forward-slash mismatch on Windows
**What goes wrong:** `defaultSensitivePaths` contains `".aws/"` (forward slash). Windows paths from ETW use backslashes (`C:\Users\x\.aws\credentials`). `strings.Contains` will not match — SENTRY-001 and SENTRY-005 silently miss credential-file accesses on Windows in production.
**Why it happens:** The rule was written for Linux/macOS where paths always use forward slashes.
**How to avoid:** Normalize path to forward slashes in `isSensitivePath` using `filepath.ToSlash(path)` before matching. **This is a production correctness fix that must land in Phase 5 as part of PTEST-05**.
**Warning signs:** PTEST-05 honeypot test fails if fixture uses a backslash path and isSensitivePath returns false.

### Pitfall 2: Tag order matters for CHANGES.md display but not for cosign
**What goes wrong:** Pushing pollen.4 before pollen.2 causes GoReleaser to generate a changelog starting from pollen.4, skipping pollen.2/3 history.
**Why it happens:** GoReleaser generates the release notes from git tag history.
**How to avoid:** Push tags in order: pollen.2 → pollen.3 → pollen.4 → pollen.5. Each tag triggers an independent release job; order does not affect cosign verification (each tag signs its own checksums.txt).

### Pitfall 3: `go install` version requires GitHub push first
**What goes wrong:** Adding `go install github.com/bantuson/pollen/cmd/pollen@v0.1.1-pollen.4` to beekeeper's CI before Pollen is pushed + tagged on GitHub causes CI to fail with `no such module`.
**Why it happens:** `go install @version` requires the module to be resolvable from the module proxy.
**How to avoid:** Sequence matters: (1) local prep, (2) push pollen + cut tags, (3) push beekeeper CI edit. The CI edit and the tag push must happen in the same checkpoint or the tag push must precede the beekeeper push.

### Pitfall 4: cosign identity casing
**What goes wrong:** Using `^https://github\.com/bantuson/pollen/` (lowercase) in `cosign verify-blob` fails. GitHub OIDC uses the canonical account casing `Bantuson` (capital B).
**Why it happens:** Go module paths are case-normalized to lowercase; GitHub OIDC is not.
**How to avoid:** Use `^https://github\.com/Bantuson/pollen/` in all verify commands (confirmed from pollen.1 fix commit 37c71e5 in CHANGES.md).
**Warning signs:** `cosign verify-blob` returns `Error: none of the expected identities matched`.

### Pitfall 5: beekeeper `selftest` uses `catalog.Entry` not `selfCatalogEntry`
**What goes wrong:** Adding `pollen-self` entries to `internal/check/selftest.go`'s `selftestEntries` using the `selfCatalogEntry` schema.
**Why it happens:** The selftest is a mmap-catalog-based test, not a self-catalog-feed test. The two use different schemas.
**How to avoid:** `selftestEntries` uses `catalog.Entry{Ecosystem:"beekeeper", Package:"pollen", Versions:["v0.1.1-pollen.4"]}` (the mmap schema). The `selfCatalogAdapter` tests use `selfCatalogEntry{Ecosystem:"beekeeper", Package:"pollen"}` (the feed schema). Both need a corresponding corpus fixture.

### Pitfall 6: pollen.5 release commit must include UPSTREAM.md delta
**What goes wrong:** Tagging pollen.5 at the current HEAD (a9db7b3) before Phase 5 local work is committed.
**Why it happens:** Phase 5's commit for pollen.5 needs to include the VERSION bump, CHANGES.md section, and UPSTREAM.md additions before tagging.
**How to avoid:** Commit VERSION/CHANGES/UPSTREAM.md in pollen repo as part of Phase 5 local work; tag pollen.5 at that new commit, not at the Phase 4 HEAD.

---

## Code Examples

### Pattern 1: SENTRY-005 test setup (for PTEST-05 honeypot)

```go
//go:build windows

package windows

import (
    "net"
    "testing"
    "time"

    "github.com/bantuson/beekeeper/internal/sentry"
)

func TestHoneypotExfilFusionFires(t *testing.T) {
    // Source: internal/sentry/rules_test.go (established pattern)
    now := time.Now().UTC()
    pid := uint32(500)
    editorPID := uint32(100)

    // Windows process tree: cursor.exe (pid=100) → subprocess (pid=500)
    tree := map[uint32]sentry.ProcessNode{
        editorPID: {PID: editorPID, PPID: 0, Exe: `C:\Program Files\cursor\cursor.exe`},
        pid:       {PID: pid, PPID: editorPID, Exe: `C:\Windows\System32\cmd.exe`},
    }
    state := sentry.NewRuleState()

    // Step 1: File access event — .aws/credentials (forward-slash path for isSensitivePath match)
    // Note: after isSensitivePath is fixed to use filepath.ToSlash, use native backslash path.
    credPath := `C:/Users/FAKEUSER/.aws/credentials`
    fileEv := sentry.SentryEvent{
        Kind:     sentry.EventFileAccess,
        PID:      pid, PPID: editorPID,
        FilePath: credPath,
        WallTime: now,
    }
    sentry.EvaluateEvent(fileEv, state, tree,
        sentry.InventorySnapshot{}, sentry.RuleConfig{}, sentry.BaselineState{}, now)

    // Step 2: Network connect — outbound to RFC 5737 documentation IP
    netEv := sentry.SentryEvent{
        Kind:     sentry.EventNetworkConnect,
        PID:      pid, PPID: editorPID,
        DstAddr:  net.ParseIP("203.0.113.1"),
        DstPort:  443,
        WallTime: now,
    }
    // Inventory: extension installed 2 min ago (within 5 min ExfilFusionWindowMin)
    inv := sentry.InventorySnapshot{
        RecentExtensions: map[string]time.Time{
            "evil-ext-id": now.Add(-2 * time.Minute),
        },
    }
    alerts := sentry.EvaluateEvent(netEv, state, tree, inv, sentry.RuleConfig{}, sentry.BaselineState{}, now)

    fired := false
    for _, a := range alerts {
        if a.RuleID == "SENTRY-005" {
            fired = true
        }
    }
    if !fired {
        t.Error("SENTRY-005 (exfil-fusion) did not fire on Windows honeypot scenario")
    }
}
```
[ASSUMED: exact EvaluateEvent call pattern — adapted from rules_test.go; confirmed SENTRY-005 trigger conditions from rules.go]

### Pattern 2: pollen-self selfCatalogEntry (for selfcatalog_test.go extension)

```go
// Source: internal/catalog/selfcatalog_test.go TestSelfCatalogAdapter_LookupAll pattern
pollenEntry := selfCatalogEntry{
    ID:            "pollen-self-2026-001",
    Name:          "Pollen v0.1.1-pollen.4 — hypothetical release compromise scenario",
    Ecosystem:     "beekeeper",
    Package:       "pollen",
    Versions:      []string{"v0.1.1-pollen.4"},
    Severity:      "critical",
    CatalogSource: "beekeeper-self",
}
adapter := &selfCatalogAdapter{entries: []selfCatalogEntry{pollenEntry}}
matches := adapter.LookupAll("beekeeper", "pollen")
// assert len(matches) == 1, matches[0].Package == "pollen"
```
[VERIFIED: internal/catalog/selfcatalog.go selfCatalogAdapter.LookupAll]

### Pattern 3: BKINT-02 CI step (for .github/workflows/ci.yml)

```yaml
# Source: beekeeper .github/workflows/ci.yml (new step)
- name: Install Pollen (BKINT-02 — pinned binary for inventory tests)
  run: go install github.com/bantuson/pollen/cmd/pollen@v0.1.1-pollen.4
```
[VERIFIED: existing ci.yml pattern; confirmed actions/setup-go adds GOPATH/bin to PATH]

### Pattern 4: selftestEntries pollen-self entry (for selftest.go extension)

```go
// Source: internal/check/selftest.go selftestEntries pattern
{
    ID:            "pollen-self-2026-001",
    Name:          "pollen (hypothetical compromised pollen release)",
    Ecosystem:     "beekeeper",
    Package:       "pollen",
    Versions:      []string{"v0.1.1-pollen.4"},
    Severity:      "critical",
    CatalogSource: "beekeeper-self",
},
```
[VERIFIED: internal/check/selftest.go, catalog.Entry struct]

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `runBumblebeeFn` (Phase 3) | `runPollenFn` + `lookPollenFn` (BKINT-01, Phase 4) | Phase 4 | Subprocess seam now uses `pollen` binary name; injectable for tests |
| pollen.2/3/4 signed tags (deferred) | Cut all four tags this phase (D-4) | Phase 5 decision | Milestone not complete until all four tags exist on GitHub |
| Local-only beekeeper + pollen (v1.0.0 posture) | Both pushed to GitHub (D-3) | Phase 5 decision | BKINT-02 depends on Pollen being publicly resolvable via `go install` |

**Deprecated/outdated:**
- `runBumblebeeFn` name (renamed to `runPollenFn` in Phase 4 — do not reference old name in Phase 5 plans).
- `lookPollenFn` is the correct injectable hook for the binary discovery path.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | SENTRY-005 honeypot test can use forward-slash credential path as a temporary workaround pending `isSensitivePath` fix | PTEST-05 / Code Examples | If the plan only ships the forward-slash workaround without the `filepath.ToSlash` fix, production Windows Sentry silently misses real credential access events — high security impact |
| A2 | `actions/setup-go` adds `GOPATH/bin` to PATH on Windows runners, making `go install` results immediately available | BKINT-02 | If PATH is not set correctly, the installed pollen binary won't be found; test would fail with "pollen not in PATH" |
| A3 | Tagging pollen.2/3/4 at their specific commit hashes (c94b271, 19695e3, a9db7b3) is safe — GoReleaser will build from those commits, not HEAD | Release runbook | If GoReleaser requires tags to be at the most recent commit (it doesn't — tags are independent), the old-commit tags would fail |

**A1** is the highest-risk assumption. The plan MUST include the `isSensitivePath` production fix (not just the test fixture workaround). The test fixture forward-slash is fine as a TEST artifact; the production fix ensures ETW-parsed backslash paths are matched correctly.

**A2** is LOW risk — `actions/setup-go` v5 documents PATH injection. Verified at: https://github.com/actions/setup-go.

**A3** is LOW risk — `git tag -a v0.1.1-pollen.2 c94b271` explicitly names the commit; GoReleaser reads the tag's target commit, not HEAD.

---

## Open Questions (RESOLVED)

> All three resolved in planning — inline recommendations adopted; plans 05-01/05-02/05-04 implement the answers.

1. **`isSensitivePath` production fix scope**
   - What we know: the function uses forward-slash sensitive path strings; Windows ETW paths use backslashes.
   - What's unclear: whether the plan should fix `isSensitivePath` in Phase 5 (as part of PTEST-05) or defer it.
   - Recommendation: Fix it in Phase 5. It's a 2-line change (`filepath.ToSlash(path)` in `isSensitivePath`). Not fixing it means the Windows Sentry silently misses real credential-file reads, which undermines PTEST-05's value as a real security test.

2. **pollen-self entry versions — real or hypothetical?**
   - What we know: no actual compromised Pollen release exists; the entries are precautionary fixtures.
   - What's unclear: should `selftestEntries` use the real version `"v0.1.1-pollen.4"` (which would cause `beekeeper selftest` to fire a false quarantine if the system runs `beekeeper check` with pollen v0.1.1-pollen.4), or a clearly-hypothetical version string like `"v0.1.1-pollen.4-COMPROMISED"`.
   - Recommendation: Use a non-production version string for the selftest fixture (e.g., `"pollen-test-v0.0.1"`) to prevent false alarms. The actual `beekeeper-self` feed (hosted externally) uses real version strings only for actually-compromised releases.

3. **beekeeper repo name on GitHub**
   - What we know: beekeeper has no `git remote` configured; `gh repo create` needs to run.
   - What's unclear: should beekeeper be pushed as `bantuson/beekeeper` (matching the go.mod module path) or `Bantuson/beekeeper` (capital B consistent with pollen).
   - Recommendation: `bantuson/beekeeper` (lowercase) matches `github.com/bantuson/beekeeper` in `go.mod`. GitHub normalizes the display case; the URL is lowercase regardless.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.25.x | BKINT-02 `go install`, all builds | ✓ (local dev, CI via actions/setup-go) | go1.25.0 (go.mod) | — |
| `git` | tag cutting, push | ✓ | (system) | — |
| `gh` CLI | `gh repo create`, `gh run watch` | ✓ (assumed, D-5 checkpoint) | (system) | Manual GitHub web UI |
| `cosign` | verify-blob post-release | ✓ (CI installs via sigstore/cosign-installer@v3) | v3 | Cannot verify without cosign |
| `../pollen` repo at HEAD a9db7b3 | All pollen work | ✓ | HEAD confirmed | — |
| GitHub Actions runners (ubuntu/macos/windows-latest) | CI green check (BKINT-02, PTEST-05) | ✓ (CI-only; not local) | (managed by GitHub) | Cannot test Windows ETW locally |
| `pollen` binary at v0.1.1-pollen.4 on PATH | BKINT-02 CI tests | ✗ currently | — | `go install @version` step in CI |

**Missing dependencies with no fallback:**
- GitHub push access (`gh auth`) — required for D-5 checkpoint; the maintainer provides this.

**Missing dependencies with fallback:**
- `pollen` binary on local PATH — not needed for local development or test authoring (all tests use `runPollenFn` injection); CI installs it via `go install`.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go standard testing + `go test` |
| Config file | none (go.mod specifies `go 1.25.0`) |
| Quick run command | `go test ./internal/sentry/windows/... ./internal/catalog/... ./internal/check/... ./internal/scan/...` |
| Full suite command | `go test -v -race ./...` (CI only — race requires CGO) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SYNC-01 | UPSTREAM.md contains 8-step workflow + version history | manual/review | N/A — doc review | ✅ (../pollen/UPSTREAM.md) |
| BKINT-02 | `go install pollen@v0.1.1-pollen.4` works; inventory tests pass | integration (CI) | `go test ./internal/scan/... -v` | ✅ (existing tests zero-skip) |
| PTEST-05 | SENTRY-005 fires on Windows honeypot scenario | unit (Windows) | `go test ./internal/sentry/windows/... -run TestHoneypotExfilFusion` | ❌ Wave 0 |
| PTEST-05 | `isSensitivePath` matches Windows backslash paths | unit (all OS) | `go test ./internal/sentry/ -run TestIsSensitivePathWindows` | ❌ Wave 0 |
| SDEF-01 | `selfCatalogAdapter.LookupAll("beekeeper","pollen")` returns match | unit | `go test ./internal/catalog/ -run TestSelfCatalogAdapter_PollenEntries` | ❌ Wave 0 |
| SDEF-01 | `beekeeper selftest` passes with pollen-self fixture | integration | `go test ./internal/check/ -run TestRunSelftest` | ❌ Wave 0 (fixture needed) |

### Sampling Rate

- **Per task commit:** `go test ./internal/sentry/... ./internal/catalog/... ./internal/check/... ./internal/scan/...`
- **Per wave merge:** `go test ./...` (minus race on Windows dev box)
- **Phase gate:** Full CI suite green on all 3 OSes before `/gsd-verify-work 5`

### Wave 0 Gaps

- [ ] `internal/sentry/windows/honeypot_test.go` — covers PTEST-05 (Windows honeypot E2E)
- [ ] `internal/sentry/rules_test.go` addition — `TestIsSensitivePathWindowsBackslash` — covers isSensitivePath fix regression
- [ ] `internal/catalog/selfcatalog_test.go` addition — `TestSelfCatalogAdapter_PollenEntries` — covers SDEF-01 adapter
- [ ] `internal/check/corpus/fixtures.json` — new pollen-self fixture entry — covers SDEF-01 selftest
- [ ] `internal/catalog/testdata/selfcatalog_match_pollen.json` — covers selfcatalog pollen feed test (optional, extends existing fixture pattern)

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | N/A (no auth in scope) |
| V3 Session Management | no | N/A |
| V4 Access Control | no | N/A |
| V5 Input Validation | yes | SENTRY-005 rule validates event fields; ETW-parsed paths normalized before matching |
| V6 Cryptography | yes | ed25519 feed signing (selfcatalog.go); cosign OIDC signing (release pipeline) — never hand-roll |
| V10 Malicious Code | yes | Release pipeline integrity (cosign + SLSA L3) protects against supply-chain injection |

### Known Threat Patterns for This Phase

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Compromised Pollen release (malicious binary pushed to GitHub) | Tampering | pollen-self catalog entries (SDEF-01) detect known-bad versions; beekeeper selftest exercises the detection |
| Tampered beekeeper-self feed (attacker injects false quarantine or suppresses real one) | Tampering | Ed25519 feed signature verification; `parseAndVerifySelfFeed` fails closed on invalid signature |
| `go install` fetches wrong module (typosquat or GOPROXY compromise) | Spoofing | Pin full version `@v0.1.1-pollen.4`; SLSA L3 attestation covers the release artifacts |
| Windows ETW backslash path bypass (isSensitivePath misses Windows paths) | Evasion | Phase 5 fix: `filepath.ToSlash` normalization in `isSensitivePath` |
| Synthetic honeypot credentials leak (test plants real credentials) | Information Disclosure | Test fixture uses string literals only; no real files created; `%USERPROFILE%\.aws\credentials` path used as a string in SentryEvent.FilePath only |

---

## Sources

### Primary (HIGH confidence)

- `beekeeper/internal/scan/scanner.go` — subprocess seam; `lookPollenFn`/`runPollenFn` injectable vars; no Go module import of pollen [VERIFIED]
- `beekeeper/internal/sentry/rules.go` — `evalSENTRY005`, `isSensitivePath` forward-slash issue, `applyDefaults` ExfilFusionWindowMin=5min [VERIFIED]
- `beekeeper/internal/catalog/selfcatalog.go` — `selfCatalogEntry` schema, `selfCatalogAdapter.LookupAll` ecosystem=="beekeeper" filter [VERIFIED]
- `beekeeper/internal/catalog/selfkey.go` — ed25519 public key; separate from release-signing identity [VERIFIED]
- `beekeeper/internal/catalog/selfcatalog_test.go` — test patterns, `signFeedEntries` helper, fixture filenames [VERIFIED]
- `beekeeper/internal/check/selftest.go` — `selftestEntries` `catalog.Entry` schema; `RunSelftest` flow [VERIFIED]
- `beekeeper/.github/workflows/ci.yml` — no Pollen install step currently; 3-OS matrix confirmed [VERIFIED]
- `beekeeper/go.mod` — no `github.com/bantuson/pollen` import; `github.com/tekert/golang-etw v0.6.2` confirmed [VERIFIED]
- `../pollen/UPSTREAM.md` — existing 8-step sync workflow; version history table (pollen.1 only) [VERIFIED]
- `../pollen/.goreleaser.yaml` — GoReleaser v2 schema; cosign sign-blob; syft CycloneDX SBOM [VERIFIED]
- `../pollen/.github/workflows/release.yml` — release trigger on `v*`; SLSA `@v2.1.0` locked; Bantuson casing [VERIFIED]
- `../pollen/VERSION` — `0.1.1-pollen.4` [VERIFIED]
- `../pollen git log` — 14 commits ahead of origin/main; commits c94b271 (pollen.2), 19695e3 (pollen.3), a9db7b3 (pollen.4) confirmed [VERIFIED]
- `.planning/phases/02-*/02-04-SUMMARY.md` — exact release commands for pollen.2 [VERIFIED]
- `.planning/phases/03-*/03-03-SUMMARY.md` — pollen.3 commit references [VERIFIED]
- `.planning/phases/04-*/04-03-SUMMARY.md` — pollen.4 HEAD a9db7b3 confirmed [VERIFIED]
- `beekeeper/internal/sentry/windows/` — `daemon_test.go`, `etw_test.go`, `parser_test.go` — build tag `//go:build windows`; `etwEventSummary` synthetic pattern [VERIFIED]

### Secondary (MEDIUM confidence)

- `beekeeper/internal/sentry/rules_test.go` — `editorTree`, `buildTree`, `freshInventory` helper patterns for honeypot test [VERIFIED from source]
- `beekeeper/internal/sentry/types.go` — `SentryEvent`, `ProcessNode`, `InventorySnapshot`, `RuleConfig` field names [VERIFIED]

### Tertiary (LOW confidence)

- `actions/setup-go` PATH injection behavior [ASSUMED — standard Actions behavior, not verified in this session via web search]

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all from verified live code
- Architecture/patterns: HIGH — from rules.go, selfcatalog.go, scanner.go, ci.yml
- Pitfalls: HIGH — isSensitivePath bug found in live source; cosign casing from CHANGES.md fix commit
- Release runbook: HIGH — commands from prior SUMMARY files + confirmed git log/remote state

**Research date:** 2026-06-03
**Valid until:** 2026-07-03 (stable Go module ecosystem; release pipeline unlikely to change)
