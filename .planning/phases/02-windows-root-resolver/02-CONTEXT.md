# Phase 2: Windows Root Resolver - Context

**Gathered:** 2026-06-02
**Status:** Ready for planning
**Source:** PRD Express Path (`beekeeper-m2-prd.md` §8.1, §11 M2.2) + carried cross-repo model (Phase 1) + live codebase reconciliation (this session)

<domain>
## Phase Boundary

Phase 2 (PRD §11 **M2.2**) makes Pollen discover all **8 package-manager roots on Windows** — npm, pnpm, Yarn, Bun (JS ecosystems), PyPI, Go modules, RubyGems, Composer — using `%APPDATA%` / `%LOCALAPPDATA%` / `%USERPROFILE%` / `%ProgramFiles%`, and proves functional equivalence with a **cross-platform parity test** (`PTEST-01`). It ends with a tagged, signed release **`v0.1.1-pollen.2`** and Windows CI that no longer skips the root-resolver test.

**Requirements covered:** WRES-01 (JS ecosystems), WRES-02 (PyPI / Go / RubyGems / Composer), PTEST-01 (parity test).

**In scope:**
- Windows default root discovery for all 8 package ecosystems per PRD §8.1 (Windows column).
- Windows-specific root-discovery tests behind `//go:build windows`.
- Cross-platform parity test: identical fake-package fixtures → equivalent inventory records on Linux/macOS/Windows (same packages, same severity matches, equivalent counts modulo OS path strings, `endpoint.os` differs correctly).
- Flip the explicit Phase-2 Windows skip in the root-resolver test so Windows CI exercises real Windows behavior.
- Keep the differential test (PTEST-02) green on Linux + macOS — the fork must not drift from upstream on the platforms upstream supports.
- Tag + sign `v0.1.1-pollen.2` using the Phase-1 release/signing stack.

**Out of scope (later phases / locked elsewhere):**
- Windows **path representation** in NDJSON (`internal/output/paths_windows.go`, `endpoint.uid` emptiness, backslash/drive-letter preservation) → **Phase 3 / M2.3**. Phase 2 touches only *where to look*, not *how paths are rendered* in output — except where the parity test incidentally needs `endpoint.os` to differ (which already works).
- Windows **editor / browser / MCP** extension paths → **Phase 4 / M2.4**.
- Beekeeper-side `internal/inventory/` integration + Pollen compatibility test + honeypot → Phases 4–5.
- `pollen-self` catalog entries → Phase 5.
</domain>

<decisions>
## Implementation Decisions

### Repository & cross-repo model (LOCKED — carried verbatim from Phase 1)
- Pollen lives at **`C:\Users\Bantu\mzansi-agentive\pollen`** (sibling to beekeeper), as **its own git repository** — NOT vendored into beekeeper (PRD §5.1). GitHub home: `github.com/bantuson/pollen`.
- **GSD tracks this milestone from beekeeper.** Phase planning artifacts (CONTEXT/PLAN/SUMMARY) live in `beekeeper/.planning/phases/02-windows-root-resolver/`. The **code** is created in and committed to `../pollen` via explicit `git -C ../pollen add/commit` operations inside plan tasks. Executor tasks that produce pollen files MUST do their own `git -C ../pollen` commit and MUST NOT rely on beekeeper's auto-commit for pollen code.
- **Preserve upstream's directory structure**; keep diffs minimal so upstream merges stay tractable (PRD §5.2). New code is build-tag-isolated where possible so Linux/macOS bytes are unchanged (protects PTEST-02).
- **Zero non-stdlib dependencies** added (PRD §10.1).

### LOAD-BEARING structural decision — reconcile PRD's assumed layout with the real codebase (RESEARCH MUST RESOLVE; default leans in-place)
- WRES-01 text and PRD §5.2 name **`internal/resolver/resolver_windows.go` (build tag `windows`)**. **That package does not exist.** In the live `../pollen` repo, *all* scan-root resolution is `package main` in **`cmd/pollen/roots.go`**, which branches on **`switch runtime.GOOS`** with only `darwin` and `linux` cases (functions: `baselineHomeCandidates`, `systemRoots`, `browserExtensionCandidateRoots`, plus the `--all-users` darwin paths). There are **no `_windows.go` files and no `//go:build` tags anywhere** in the repo today.
- The decision the planner must make (informed by research): **(A)** honor the requirement literally by extracting root resolution into a new `internal/resolver/` package split across `resolver_unix.go` / `resolver_windows.go` build tags, or **(B)** follow the *actual* upstream structure and add Windows support where the code lives — a Windows branch/file for `cmd/pollen/roots.go` (e.g. `roots_windows.go` with `//go:build windows`, or a `case "windows":` in the existing `runtime.GOOS` switches).
- **Default preference: (B) in-place, minimal-diff**, because it directly serves the Phase-1 locked principle ("preserve upstream structure, keep merges tractable, don't drift") and minimizes risk to the Linux/macOS differential. **(A) is a refactor** that moves upstream code into a new package — higher merge-conflict surface against future upstream syncs. Research must confirm upstream PR #4's actual approach (PRD §2.4 claims "build-tag-separated implementation") and recommend; the planner records the final choice and, if it diverges from WRES-01's literal wording, notes that the **requirement intent** (Windows root discovery for 8 ecosystems) is satisfied even though the file path differs. **The requirement is satisfied by behavior, not by a literal filename.**

### Windows root coverage — the 8 ecosystems (PRD §8.1, authoritative target table)
Each ecosystem's Windows roots to discover (mirrors the Unix roots upstream already covers):
- **npm** — global `%APPDATA%\npm\node_modules`, `%ProgramFiles%\nodejs\node_modules`; user cache `%APPDATA%\npm-cache\_cacache`, `%LOCALAPPDATA%\npm-cache\_cacache`
- **pnpm** — `%LOCALAPPDATA%\pnpm\store`, `%APPDATA%\pnpm`
- **Yarn** — `%LOCALAPPDATA%\Yarn\Data\global`
- **Bun** — `%USERPROFILE%\.bun\install\cache`
- **PyPI** — user `%APPDATA%\Python\Python*\site-packages`; venv `<venv>\Lib\site-packages`
- **Go modules** — `%USERPROFILE%\go\pkg\mod`
- **RubyGems** — `%USERPROFILE%\.gem\ruby\*\gems`, `%ProgramFiles%\Ruby*\lib\ruby\gems`
- **Composer** — `%APPDATA%\Composer\vendor`
- Path construction uses `filepath.Join` + env-var reads (`os.Getenv("APPDATA")`, `LOCALAPPDATA`, `USERPROFILE`, `ProgramFiles`); never hand-built backslash strings. Glob (`%ProgramFiles%\Ruby*`, `Python*`) reuses the existing `globExisting` helper pattern.
- Absent candidate roots are dropped by the existing `filterExistingRoots` discipline — same as Unix. No new "missing root" handling invented.

### Cross-platform parity test (PTEST-01)
- Assert, on the SAME fake-package fixture tree across Linux/macOS/Windows: **same packages detected, same severity matches (given an exposure catalog), equivalent record counts modulo OS path strings, `endpoint.os` differs correctly per platform** (PRD §9.3, success criterion 2).
- **Reuse the existing NDJSON normalization harness** (`cmd/pollen/normalize_diff.go` — strips non-deterministic fields, sorts by `record_id`) rather than inventing a second normalizer. The parity comparison is "equivalent after normalization + OS-path-string allowance," not byte-identical (byte-identical is PTEST-02's job across upstream, not PTEST-01's job across OSes).
- `endpoint.os` already derives from `runtime.GOOS` (`internal/endpoint/endpoint.go:20`), so the per-OS `endpoint.os` divergence the test asserts is *already* a real signal — the test pins it, it does not need new production code.
- **Fixture location is research/planner discretion.** PRD §9.3 / PTEST-01 name `internal/testfixtures/`, which does not exist; live fixtures are under `cmd/pollen/selftest/fixtures/` and `cmd/pollen/testdata/diff-fixture/`. Decide whether to create `internal/testfixtures/` or extend an existing fixture dir; keep one canonical fake-package tree consumed by all three OS runs.
- **Fixture-injection mechanism is research/planner discretion.** Either drive the scan with explicit `--root <fixture>` (the `resolveRoots` explicit-roots path already honors this) or use an env-var override analogous to the existing `POLLEN_USERS_DIR` test hook to redirect default Windows roots at the fixture. Pick the lower-risk option that still exercises the Windows root code.

### Windows CI skip discipline (success criterion 4)
- Flip the **explicit** Phase-2 skip in `cmd/pollen/main_test.go:54-55` (`t.Skip("broad-home detection uses Unix-style paths; Windows root-resolver tests arrive in Phase 2 (v0.1.1-pollen.2)")`) so the Windows root-resolver path is actually exercised on `windows-latest`. Any Windows skip that remains must carry a structured, non-Phase-2 reason — no silent `t.Skip()`, no rot (PRD §1.5).
- The differential test's Windows skip (`cmd/pollen/differential_test.go:55-56`) is a **different** concern: the differential (PTEST-02) is inherently Linux+macOS-only because upstream Bumblebee has no Windows build. That skip stays; update its message only if it implies Windows differential is coming (it is not).
- `go test -race ./...` already runs on all three OSes in `ci.yml` (`CGO_ENABLED=1`). New `//go:build windows` tests run automatically inside that matrix; no new CI job is required for WRES — only the parity test must land in the matrix.

### Release (mirror Phase 1, FORK-03 / SDEF-02 stack)
- Bump `VERSION` `0.1.1-pollen.1` → `0.1.1-pollen.2`; append a `CHANGES.md` entry (Added: Windows root resolver for 8 ecosystems; parity test). Tag + Sigstore-sign `v0.1.1-pollen.2` via the existing `../pollen/.github/workflows/release.yml` keyless-OIDC path. CycloneDX SBOM regenerates per release (SDEF-02, already wired).

### Claude's Discretion
- (A) vs (B) package-structure choice above, pending research — including exact file name(s) and whether to introduce `//go:build windows` / `//go:build !windows` splits or a `case "windows":` extension of the `runtime.GOOS` switches.
- Whether each ecosystem gets its own Windows root function or a single Windows root table in one file.
- Parity-test fixture directory layout and the inject mechanism (explicit `--root` vs env override).
- Exact `CHANGES.md` wording and whether the contribution-back patch shape (PRD §12.1) is pre-staged now or deferred to Phase 5 (default: defer execution, but keep the Windows code structured so it is cleanly extractable).
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents (researcher, planner, executor) MUST read these before planning or implementing.** All `../pollen/...` paths are in the sibling repo `C:\Users\Bantu\mzansi-agentive\pollen`.

### Milestone PRD (authoritative scope)
- `beekeeper-m2-prd.md` §8.1 (Windows package-manager root table — the work item), §8.5 (path representation — Phase 3 boundary), §9.1–9.3 (selftest, differential, **parity** test strategy), §11 **M2.2** (this phase's definition), §2.3–2.4 (the Windows blocker + upstream PR #4 build-tag approach), §5.1–5.2 (repo/module structure + "preserve upstream structure"), §12.1 (contribution-back patch shape).
- `.planning/REQUIREMENTS.md` — WRES-01, WRES-02, PTEST-01 (exact acceptance wording).
- `.planning/ROADMAP.md` Phase 2 section — Success Criteria 1–4.

### Live Pollen code — the real structure to reconcile against (READ BEFORE ASSUMING PRD LAYOUT)
- `../pollen/cmd/pollen/roots.go` — **the actual root-resolution code.** `resolveRoots`, `baselineHomeCandidates`, `systemRoots`, `browserExtensionCandidateRoots`, `globExisting`, `filterExistingRoots`, `usersDirOverride` (the `POLLEN_USERS_DIR` test hook pattern). Branches on `runtime.GOOS` (darwin/linux only). **No `internal/resolver/` exists — this file is the integration point.**
- `../pollen/internal/scanner/` — `scanner.Root{Path, Kind string}`; how roots feed the walk.
- `../pollen/internal/model/` — `RootKind*` constants, `Endpoint`, record types.
- `../pollen/internal/endpoint/endpoint.go` — `Current()` sets `OS: runtime.GOOS`, `Arch: runtime.GOARCH`, `Username`/`UID` from `os/user` (note: `endpoint.os` already works on Windows; `UID` emptiness is a **Phase 3** concern).
- `../pollen/internal/ecosystem/{npm,pnpm,yarn,bun,pypi,gomod,rubygems,composer}/` — per-ecosystem detectors. Confirm these are OS-agnostic manifest matchers (so Windows work is "point roots at the right dirs," not "rewrite detectors").
- `../pollen/cmd/pollen/normalize_diff.go` + `differential_test.go` — the NDJSON normalization harness and differential pattern PTEST-01 should reuse; also where the Windows skip discipline is modeled.
- `../pollen/cmd/pollen/main_test.go` (line ~54) — the **explicit Phase-2 skip** to flip.
- `../pollen/cmd/pollen/main.go` (lines 191, 367) — where `resolveRoots` is wired into the scan.
- `../pollen/cmd/pollen/selftest.go` + `selftest/fixtures/`, `cmd/pollen/testdata/diff-fixture/` — existing fixture conventions for the parity test.
- `../pollen/.github/workflows/ci.yml` — the 3-OS matrix (`go test -race`, selftest) + Linux/macOS-only differential job.
- `../pollen/.github/workflows/release.yml`, `.goreleaser.yaml`, `Makefile`, `VERSION`, `CHANGES.md` — the release/signing/SBOM stack to reuse for `v0.1.1-pollen.2`.

### Phase 1 artifacts (decisions already locked)
- `.planning/phases/01-fork-setup-discipline/01-CONTEXT.md` — cross-repo model, fork mechanics, trademark/legal discipline.
- `.planning/phases/01-fork-setup-discipline/01-RESEARCH.md` — upstream pinned SHA `c24089804ee66ece4bec6f14638cb98985389cdb`, NDJSON determinism findings, race-detector/CGO pitfall (Pitfall 5), differential normalization design.
- `.planning/phases/01-fork-setup-discipline/01-0{1..5}-SUMMARY.md` — what shipped: `TestDifferential` LOCKED name, selftest, CI matrix, signing.
</canonical_refs>

<specifics>
## Specific Ideas

- Upstream pinned commit (do not drift): `c24089804ee66ece4bec6f14638cb98985389cdb` (v0.1.1). The differential test clones and builds upstream at this SHA.
- `endpoint.os` divergence assertion in PTEST-01 is *already* satisfiable — `runtime.GOOS` drives it. The test pins behavior; production change for it is zero.
- Reuse `globExisting` for `%ProgramFiles%\Ruby*` and `%APPDATA%\Python\Python*` wildcard roots; reuse `filterExistingRoots` so absent roots are silently/diagnostically dropped exactly as on Unix.
- Reuse the `POLLEN_USERS_DIR`-style env-override idiom if the parity test needs to redirect default Windows roots at a fixture without hardcoding a real user profile.
- Success criterion target: on a Windows runner with the fake-package fixture tree, `pollen scan` returns records for **all 8** ecosystems with non-empty Windows paths; parity test green on all 3 OSes; differential still green on Linux+macOS; `v0.1.1-pollen.2` tagged + signed.
- CI race detector requires `CGO_ENABLED=1` on Windows (`windows-latest` has MSVC) — already set in `ci.yml`; do not regress it.
</specifics>

<deferred>
## Deferred Ideas

- Windows path representation in NDJSON output (backslash/drive-letter preservation, `endpoint.uid` empty-on-Windows, `paths_windows.go`) — **Phase 3 / M2.3**.
- Editor / browser / MCP host config Windows paths — **Phase 4 / M2.4** (the `browserExtensionCandidateRoots` Windows branch and MCP/editor Windows trees belong there, NOT Phase 2; Phase 2 may add the Windows `case` skeleton to that function but should not implement the full extension/MCP table).
- Beekeeper `internal/inventory/` integration + Pollen compatibility test + Windows honeypot — Phases 4–5.
- `pollen-self` catalog entries in `beekeeper-self` — Phase 5.
- Executing an actual upstream sync / opening contribution-back PRs — Phase 5 (Phase 2 only keeps the Windows code cleanly extractable).
</deferred>

---

*Phase: 02-windows-root-resolver*
*Context gathered: 2026-06-02 via PRD Express Path + live codebase reconciliation*
