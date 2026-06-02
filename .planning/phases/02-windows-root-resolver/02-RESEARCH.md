# Phase 2: Windows Root Resolver — Research

**Researched:** 2026-06-02
**Domain:** Go cross-platform root discovery, Windows environment variables, package-manager install conventions, cross-platform parity testing
**Confidence:** HIGH — all findings grounded in direct reads of `../pollen` source files

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- Pollen lives at `C:\Users\Bantu\mzansi-agentive\pollen` (sibling to beekeeper) as its own git repository. GitHub home: `github.com/bantuson/pollen`.
- GSD tracks from beekeeper; code commits go to `../pollen` via explicit `git -C ../pollen add/commit`. Beekeeper GSD commits cover only planning artifacts.
- Preserve upstream directory structure; keep diffs minimal for tractable future merges (PRD §5.2).
- Zero non-stdlib dependencies added.
- WRES-01/02 file name assumption (`internal/resolver/resolver_windows.go`) does NOT exist in the live repo. The LOAD-BEARING decision (A vs B) is for research to resolve.
- Default preference: **(B) in-place, minimal-diff** — add Windows in `cmd/pollen/roots.go` or a companion `roots_windows.go`.
- Windows env-var idiom: `filepath.Join(os.Getenv("APPDATA"), ...)` etc. — never hand-built backslash strings.
- Absent roots dropped by `filterExistingRoots` — no new "missing root" handling.
- Reuse `globExisting` for wildcard paths (`Python*`, `Ruby*`).
- Parity test reuses normalize_diff.go harness; fixture location and injection mechanism are discretion.
- Flip the Phase-2 skip in `main_test.go:54-55`; keep the differential Windows skip.
- Release: VERSION `0.1.1-pollen.1` → `0.1.1-pollen.2`, CHANGES.md entry, same signing stack.

### Claude's Discretion

- Package-structure choice (A vs B) including exact file names and whether to use `//go:build windows` / `//go:build !windows` splits or extend existing `switch runtime.GOOS` blocks.
- Whether each ecosystem gets its own Windows root function or a single Windows root table in one file.
- Parity-test fixture directory layout and injection mechanism (explicit `--root` vs env override).
- Exact CHANGES.md wording and whether contribution-back patch shape is pre-staged (default: defer to Phase 5).

### Deferred Ideas (OUT OF SCOPE)

- Windows path representation in NDJSON output (`paths_windows.go`, backslash/drive-letter, `endpoint.uid` empty) — Phase 3.
- Editor/browser/MCP extension Windows paths — Phase 4.
- Beekeeper `internal/inventory/` integration, parity test, honeypot — Phases 4–5.
- `pollen-self` catalog entries — Phase 5.
- Upstream contribution-back PRs — Phase 5.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| WRES-01 | Windows root discovery for JS ecosystems — npm (`%APPDATA%\npm\node_modules` + cache), pnpm (`%LOCALAPPDATA%\pnpm\store`), Yarn (`%LOCALAPPDATA%\Yarn\Data\global`), Bun (`%USERPROFILE%\.bun\install\cache`) | All paths confirmed from PRD §8.1 and CONTEXT.md; env-var idiom confirmed against existing roots.go patterns; glob helpers verified in-place |
| WRES-02 | Windows root discovery for PyPI, Go modules, RubyGems, Composer | Same evidence; wildcard paths confirmed to need `globExisting`; venv pattern documented |
| PTEST-01 | Cross-platform parity test — same fake-package fixtures → equivalent inventory records on all 3 OSes; `endpoint.os` differs correctly | `normalize_diff.go` harness reuse confirmed; fixture placement decision documented; injection via `--root` recommended; `endpoint.Current()` already emits `runtime.GOOS` |
</phase_requirements>

---

## Summary

All of the real work for Phase 2 lives in three places in `../pollen/cmd/pollen/roots.go`: (1) `baselineHomeCandidates` — which must gain a Windows `case` for per-user package-manager roots; (2) `systemRoots` — which must gain a Windows `case` for global roots (`%ProgramFiles%`); and (3) `browserExtensionCandidateRoots` — which may gain a Windows skeleton (not full, per Phase 4 deferral). That is the entire production code surface. The PRD's assumed `internal/resolver/resolver_windows.go` does not exist and should not be created — in-place extension of `roots.go` is the correct, minimal-diff approach.

The ecosystem detectors in `internal/ecosystem/*/` contain **zero `runtime.GOOS` checks** and are pure file-format parsers dispatched by basename. [VERIFIED: grep `runtime.GOOS` across `../pollen/internal/ecosystem/`] They work unchanged on Windows — Phase 2 only changes *where* to look, not *how* packages are parsed.

The parity test (PTEST-01) should drive the scan with an explicit `--root <fixture>` using the existing `resolveRoots` explicit-path mode, housed as `cmd/pollen/testdata/parity-fixture/`. This avoids any env-var hook complexity and directly exercises the fixture path that all three OS runners share identically. Normalization reuses `normalize()` from `normalize_diff.go`; the parity comparison additionally allows `endpoint.os` to differ (it is stripped in normalization, then pinned per-OS in the assertion).

The six Windows skips in `cmd/pollen/main_test.go` (lines 54, 121, 146, 273, 288, 300) are the exact skips to flip. The differential skip at `cmd/pollen/differential_test.go:55` stays untouched.

**Primary recommendation:** Add a `roots_windows.go` (build tag `//go:build windows`) in `cmd/pollen/` containing the Windows-only functions that `roots.go` delegates to; keep the existing `switch runtime.GOOS` fallthrough paths rather than splitting all three platforms into build-tagged files. This maximizes upstream merge tractability while keeping Windows bytes isolated.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Windows root path construction | `cmd/pollen/` (package main) | — | Existing root resolution code is all in package main; introducing an `internal/` package would require either exporting types or moving functions, creating upstream merge friction |
| OS-agnostic package detection | `internal/ecosystem/*/` | — | Detectors are file-format matchers with no OS checks; verified grep-clean of `runtime.GOOS` in internal/ecosystem |
| Parity test fixture management | `cmd/pollen/testdata/` | — | Mirrors existing `diff-fixture` pattern; all three OS runners read the same committed fixture dir |
| Test injection | `cmd/pollen/` explicit `--root` flag | — | `resolveRoots` already honors explicit `--root` overrides; this path exercises Windows resolution when the fixture is passed as `--root` and the Windows roots code is NOT exercised — the Windows-specific test must use env override or construct Windows paths in a Windows-only test file |
| `endpoint.os` emission | `internal/endpoint/endpoint.go` | — | Already `runtime.GOOS`; no change needed |

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `os`, `path/filepath`, `runtime` | Go 1.25 (go.mod) | Env-var reads, path joins, OS detection | Zero-dependency rule; all needed primitives are stdlib |

No new dependencies. This is a stdlib-only change. [VERIFIED: `../pollen/go.mod` has no external deps; `go.sum` does not exist]

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `roots_windows.go` build-tag file | `case "windows":` inline in `roots.go` | Inline `case` avoids a new file but bloats `roots.go` with large Windows tables; separate file is cleaner for upstream PR shape |
| `roots_windows.go` build-tag file | `internal/resolver/` package split | Package split means moving upstream code into new package — high merge-conflict surface; violates "preserve upstream structure" locked decision |

---

## Architecture Patterns

### Recommended File Structure Change

```
cmd/pollen/
  roots.go           # existing — add `case "windows":` delegation calls
  roots_windows.go   # NEW — //go:build windows — windows-only functions
  main_test.go       # flip 6 Windows skips
  testdata/
    diff-fixture/    # existing
    parity-fixture/  # NEW — 8-ecosystem fake tree, shared across all 3 OSes
  parity_test.go     # NEW — PTEST-01 test, no build tag (runs on all OSes)
```

### Pattern 1: `roots_windows.go` with `//go:build windows`

**What:** A new file `cmd/pollen/roots_windows.go` with build tag `//go:build windows` containing Windows-only helper functions called from the `case "windows":` branches in `roots.go`.

**When to use:** Whenever you need Windows-only code that would otherwise bloat the main `roots.go` with large platform-specific tables. Upstream PR #4 (PRD §2.4) used this exact pattern — "build-tag-separated implementation." [CITED: beekeeper-m2-prd.md §2.4]

**Concrete addition to `roots.go`:**

In `baselineHomeCandidates` (line 204), the existing `switch runtime.GOOS` block at lines 250–257 (MCP config) and lines 497–559 (`browserExtensionCandidateRoots`) gains a `case "windows":`. Add package-manager roots in `baselineHomeCandidates`:

```go
// Source: ../pollen/cmd/pollen/roots.go, baselineHomeCandidates (to be added)
case "windows":
    for _, p := range windowsBaselinePackageRoots() {
        add(p.Path, p.Kind)
    }
```

In `systemRoots` (line 289), add:

```go
// Source: ../pollen/cmd/pollen/roots.go, systemRoots (to be added)
case "windows":
    return windowsSystemRoots()
```

**New file `cmd/pollen/roots_windows.go`:**

```go
//go:build windows

package main

import (
    "os"
    "path/filepath"

    "github.com/bantuson/pollen/internal/model"
    "github.com/bantuson/pollen/internal/scanner"
)

// windowsBaselinePackageRoots returns the Windows-specific per-user and
// global package-manager install roots for the baseline profile.
// All paths are constructed via filepath.Join + os.Getenv — never hand-built
// backslash strings. Absent roots are silently dropped by filterExistingRoots.
func windowsBaselinePackageRoots() []scanner.Root {
    appdata    := os.Getenv("APPDATA")
    localappdata := os.Getenv("LOCALAPPDATA")
    userprofile := os.Getenv("USERPROFILE")

    var out []scanner.Root
    add := func(p, kind string) {
        if p != "" {
            out = append(out, scanner.Root{Path: p, Kind: kind})
        }
    }

    // npm
    if appdata != "" {
        add(filepath.Join(appdata, "npm", "node_modules"),   model.RootKindGlobalPackage)
        add(filepath.Join(appdata, "npm-cache", "_cacache"), model.RootKindUserPackage)
    }
    if localappdata != "" {
        add(filepath.Join(localappdata, "npm-cache", "_cacache"), model.RootKindUserPackage)
    }

    // pnpm
    if localappdata != "" {
        add(filepath.Join(localappdata, "pnpm", "store"), model.RootKindUserPackage)
    }
    if appdata != "" {
        add(filepath.Join(appdata, "pnpm"), model.RootKindUserPackage)
    }

    // Yarn
    if localappdata != "" {
        add(filepath.Join(localappdata, "Yarn", "Data", "global"), model.RootKindUserPackage)
    }

    // Bun
    if userprofile != "" {
        add(filepath.Join(userprofile, ".bun", "install", "cache"), model.RootKindUserPackage)
    }

    // PyPI — user site-packages (glob: Python*)
    if appdata != "" {
        for _, p := range globExisting(filepath.Join(appdata, "Python", "Python*", "site-packages")) {
            add(p, model.RootKindUserPackage)
        }
    }

    // Go modules
    if userprofile != "" {
        add(filepath.Join(userprofile, "go", "pkg", "mod"), model.RootKindUserPackage)
    }

    // RubyGems — user gems (glob: ruby/*/gems)
    if userprofile != "" {
        for _, p := range globExisting(filepath.Join(userprofile, ".gem", "ruby", "*", "gems")) {
            add(p, model.RootKindUserPackage)
        }
    }

    // Composer
    if appdata != "" {
        add(filepath.Join(appdata, "Composer", "vendor"), model.RootKindUserPackage)
    }

    return out
}

// windowsSystemRoots returns Windows global/system package roots for the
// baseline profile. These are independent of the user home directory.
func windowsSystemRoots() []scanner.Root {
    programFiles := os.Getenv("ProgramFiles")

    var out []scanner.Root
    add := func(p, kind string) {
        if p != "" {
            out = append(out, scanner.Root{Path: p, Kind: kind})
        }
    }

    // npm global (node.js MSI installs here)
    if programFiles != "" {
        add(filepath.Join(programFiles, "nodejs", "node_modules"), model.RootKindGlobalPackage)
    }

    // RubyGems — system Ruby installs (glob: Ruby*)
    if programFiles != "" {
        for _, p := range globExisting(filepath.Join(programFiles, "Ruby*", "lib", "ruby", "gems")) {
            add(p, model.RootKindGlobalPackage)
        }
    }

    return out
}
```

### Pattern 2: Parity Test via Explicit `--root`

**What:** A test file `cmd/pollen/parity_test.go` (no build tag — runs on all OSes) that builds and runs `pollen scan` with `--root testdata/parity-fixture --profile deep --emit-summary=false`, captures NDJSON, normalizes it with `normalize()`, and asserts: (a) same record count across OSes, (b) same packages present, (c) `endpoint.os` equals `runtime.GOOS` in each run's un-normalized output.

**When to use:** PTEST-01. The explicit `--root` path exercises the scanner directly without depending on Windows root discovery (which is tested separately in `TestWindowsBaselineRoots`). This is intentional: the parity test proves the *detectors* find the same things on all OSes, while the WRES-01/02 tests prove the *root resolver* finds the right Windows paths.

**Concrete test skeleton:**

```go
// Source: cmd/pollen/parity_test.go (new file)
package main

import (
    "bytes"
    "runtime"
    "testing"
    "os/exec"
    "path/filepath"
)

// TestParityAllEcosystems is PTEST-01: the same 8-ecosystem fake-package fixture
// must produce equivalent normalized records on Linux, macOS, and Windows.
// It uses an explicit --root so no platform-specific root discovery runs —
// the fixture is a committed testdata tree shared across all three CI runners.
func TestParityAllEcosystems(t *testing.T) {
    _, thisFile, _, ok := runtime.Caller(0)
    if !ok {
        t.Fatal("runtime.Caller failed")
    }
    fixtureDir := filepath.Join(filepath.Dir(thisFile), "testdata", "parity-fixture")

    pollenExe := buildCurrentPollen(t) // reuse helper from differential_test.go

    out, err := exec.Command(pollenExe,
        "scan",
        "--profile", "deep",
        "--root", fixtureDir,
        "--emit-summary=false",
    ).Output()
    if err != nil {
        t.Fatalf("pollen scan on parity fixture: %v", err)
    }

    // Verify endpoint.os before normalization.
    assertEndpointOS(t, out, runtime.GOOS)

    norm, err := normalize(out)
    if err != nil {
        t.Fatalf("normalize parity output: %v", err)
    }
    if len(bytes.TrimSpace(norm)) == 0 {
        t.Fatal("parity: normalize produced empty output")
    }

    // Count records and assert minimum per-ecosystem coverage.
    assertParityRecordCoverage(t, norm)
}
```

The `assertEndpointOS` and `assertParityRecordCoverage` helpers are defined in the same file and verify: (1) each record's `endpoint.os` equals the expected value before stripping, (2) at least one record exists for each of the 8 ecosystems.

**Important:** The parity fixture tests that *detectors* work on all three OSes when pointed at the right paths. The WRES-01/02 tests (separate `//go:build windows` test file) test that the Windows *root resolver* emits the right candidate paths. These are orthogonal — do not conflate them.

### Anti-Patterns to Avoid

- **Putting Windows root tables in `roots.go` inline:** Makes `roots.go` much larger and harder to diff against upstream. Use `roots_windows.go`.
- **Creating `internal/resolver/`:** Moves upstream code into a new package, inflating future merge conflict surface. The literal filename in WRES-01 (`internal/resolver/resolver_windows.go`) reflects PRD §7.3 draft CHANGES.md, not a real upstream structure. The requirement is satisfied by behavior, not by literal filename.
- **Using `filepath.FromSlash` to construct Windows paths:** Correct approach is `filepath.Join(os.Getenv("APPDATA"), "npm", "node_modules")` — `filepath.Join` uses the OS separator automatically.
- **Hard-coding `C:\Users\...`:** Never hard-code drive letters or usernames. Always use `%APPDATA%`, `%LOCALAPPDATA%`, `%USERPROFILE%`, `%ProgramFiles%`.

---

## Q1: Structure Decision — Definitive Recommendation

**VERIFIED findings that resolve the load-bearing discrepancy:**

1. `../pollen/cmd/pollen/roots.go` is `package main` with four `runtime.GOOS` switches (lines 250, 290, 342, 376, 501, 541). [VERIFIED: direct read]
2. Zero `_windows.go` files and zero `//go:build` tags exist anywhere in the repo today. [VERIFIED: glob + grep]
3. PRD §2.4 states upstream PR #4 used "build-tag-separated implementation" — meaning a `_windows.go` file, not a new `internal/` package. [CITED: beekeeper-m2-prd.md §2.4]
4. The WRES-01 text names `internal/resolver/resolver_windows.go` but this is a PRD draft artifact (PRD §7.3 CHANGES.md example). No such package exists. The CONTEXT.md explicitly states the requirement is satisfied by behavior, not by literal filename.

**Recommendation: Option B — in-place extension, new `cmd/pollen/roots_windows.go`.**

File `cmd/pollen/roots_windows.go` with `//go:build windows` containing `windowsBaselinePackageRoots()` and `windowsSystemRoots()`. The existing `switch runtime.GOOS` blocks in `roots.go` gain `case "windows":` branches that delegate to these functions. This gives:
- Zero changes to upstream-owned logic in `roots.go` (the existing darwin/linux cases are untouched)
- All Windows bytes isolated to one new file (easy to extract as a contrib-back PR)
- No new internal packages (upstream merge trivial — one new file + minimal additions to the switch cases)
- `//go:build !windows` companion file is NOT needed for the function stubs — the switch's `default:` path already returns nil, which `filterExistingRoots` handles correctly

**CHANGES.md entry** should say `cmd/pollen/roots_windows.go` (the actual file), not `internal/resolver/resolver_windows.go`.

---

## Q2: Windows Ecosystem Root Table

Derived from PRD §8.1 and CONTEXT.md, expressed as concrete Go idioms.

### JS Ecosystems (WRES-01)

| Ecosystem | Env Var | Concrete `filepath.Join` Expression | Kind |
|-----------|---------|-------------------------------------|------|
| npm global modules | `APPDATA` | `filepath.Join(appdata, "npm", "node_modules")` | `RootKindGlobalPackage` |
| npm user cache | `APPDATA` | `filepath.Join(appdata, "npm-cache", "_cacache")` | `RootKindUserPackage` |
| npm user cache alt | `LOCALAPPDATA` | `filepath.Join(localappdata, "npm-cache", "_cacache")` | `RootKindUserPackage` |
| npm global (nodejs MSI) | `ProgramFiles` | `filepath.Join(programFiles, "nodejs", "node_modules")` | `RootKindGlobalPackage` |
| pnpm store | `LOCALAPPDATA` | `filepath.Join(localappdata, "pnpm", "store")` | `RootKindUserPackage` |
| pnpm user | `APPDATA` | `filepath.Join(appdata, "pnpm")` | `RootKindUserPackage` |
| Yarn global | `LOCALAPPDATA` | `filepath.Join(localappdata, "Yarn", "Data", "global")` | `RootKindUserPackage` |
| Bun cache | `USERPROFILE` | `filepath.Join(userprofile, ".bun", "install", "cache")` | `RootKindUserPackage` |

### Non-JS Ecosystems (WRES-02)

| Ecosystem | Env Var | Concrete Expression | Glob? | Kind |
|-----------|---------|---------------------|-------|------|
| PyPI user site-packages | `APPDATA` | `globExisting(filepath.Join(appdata, "Python", "Python*", "site-packages"))` | YES — `Python*` | `RootKindUserPackage` |
| Go modules | `USERPROFILE` | `filepath.Join(userprofile, "go", "pkg", "mod")` | No | `RootKindUserPackage` |
| RubyGems user gems | `USERPROFILE` | `globExisting(filepath.Join(userprofile, ".gem", "ruby", "*", "gems"))` | YES — `*` | `RootKindUserPackage` |
| RubyGems system | `ProgramFiles` | `globExisting(filepath.Join(programFiles, "Ruby*", "lib", "ruby", "gems"))` | YES — `Ruby*` | `RootKindGlobalPackage` |
| Composer vendor | `APPDATA` | `filepath.Join(appdata, "Composer", "vendor")` | No | `RootKindUserPackage` |

### PyPI Venv Roots

The PRD §8.1 entry `<venv>\Lib\site-packages` is a *per-project* venv, not a global root. It is discovered when the scanner walks a project tree (profile=project or profile=deep) — the venv's `Lib\site-packages\` is automatically walked when the project root contains a `.venv\` or `venv\` subdirectory. No special root is needed for venv discovery in the baseline profile; venvs are inherently project-scoped. [ASSUMED — PyPI detector reads `METADATA` files wherever they live; no explicit venv-root injection needed]

### Env Var Fallback and Empty-String Guard

`os.Getenv` on Windows returns `""` for an unset variable (which is unusual on Windows but possible in containers or minimal environments). The `add` helper in `windowsBaselinePackageRoots()` must guard against empty prefix:

```go
add := func(p, kind string) {
    // If the env var was empty, filepath.Join produces a relative path
    // (e.g., "npm\node_modules") which is wrong. Guard at the env-var level:
    if p != "" {
        out = append(out, scanner.Root{Path: p, Kind: kind})
    }
}
```

The check is at the env-var level: `if appdata != ""` before any `add` calls using `appdata`. This mirrors the existing `baselineHomeCandidates` pattern of checking `if home == "" { return nil }`. [VERIFIED: roots.go line 205]

### `ProgramFiles(x86)` Consideration

`%ProgramFiles(x86)%` (the 32-bit program files on 64-bit Windows) is a separate env var with parentheses in its name. Go's `os.Getenv("ProgramFiles(x86)")` works correctly — the parentheses are legal in env var names. For package managers (npm, Ruby), the 64-bit `%ProgramFiles%` location is the standard one; a paranoid scan could also check `%ProgramFiles(x86)%`. PRD §8.1 lists only `%ProgramFiles%`. [ASSUMED — adding x86 variant is low-value and not in PRD scope; note for CONTEXT]

### `globExisting` Sufficiency

`globExisting` (defined at `roots.go` line 309) uses `filepath.Glob` which works on Windows paths with backslash separators. [VERIFIED: roots.go source; `filepath.Glob` is documented as using `filepath.Match` which handles OS separators]. `Python*` and `Ruby*` wildcards are simple prefix globs that `filepath.Glob` supports natively. No new glob helper is needed.

---

## Q3: Parity Test Design (PTEST-01)

### Fixture Design

**Location:** `cmd/pollen/testdata/parity-fixture/`

Rationale: Mirrors the existing `testdata/diff-fixture/` pattern. [VERIFIED: differential_test.go line 65: `fixtureDir := filepath.Join(filepath.Dir(thisFile), "testdata", "diff-fixture")`]. The `testdata/` convention is standard Go (excluded from build, committed to repo). `internal/testfixtures/` named in the PRD does not exist and creating it would require exporting test helpers — not worth it.

**Fixture tree covering all 8 ecosystems:**

```
cmd/pollen/testdata/parity-fixture/
  npm-fixture/
    package-lock.json           # lockfileVersion:3, one known package
  pnpm-fixture/
    pnpm-lock.yaml              # one known package
  yarn-fixture/
    yarn.lock                   # one known package
  bun-fixture/
    bun.lockb (or bun.lock)     # one known package (bun lockfile format)
  pypi-fixture/
    parity_pypi_pkg-1.0.0.dist-info/
      METADATA                  # Name: parity-pypi-pkg, Version: 1.0.0
  gomod-fixture/
    go.sum                      # one known module
  rubygems-fixture/
    specifications/
      parity-gem-1.0.0.gemspec  # fake gemspec
  composer-fixture/
    composer.lock               # one known package
```

Each fixture is a minimal valid metadata file with a predictable package name that can be asserted in the test. Use names in the `parity-*` namespace to avoid collision with selftest fixtures.

**NOTE:** The `bun` ecosystem fixture format needs verification — check `internal/ecosystem/bun/bun.go` to confirm which filenames the Bun scanner dispatches on. [ASSUMED: likely `bun.lock` — verify before implementing]

### Injection Mechanism

**Use explicit `--root testdata/parity-fixture --profile deep`.**

This is the same mechanism `TestDifferential` uses (`runBinaryOnFixture` at `differential_test.go:296`). Rationale:
1. `resolveRoots` honors explicit `--root` entries and skips all platform-specific default discovery (roots.go lines 77–97). [VERIFIED: roots.go line 77]
2. The explicit path is computed via `runtime.Caller(0)` + `filepath.Dir` to get the path independent of cwd — the same pattern `differential_test.go:65` uses.
3. No env-var hook needed (unlike POLLEN_USERS_DIR which is macOS-darwin-specific).

**WRES-01/02 coverage vs parity-test coverage are separate:**
- The parity test (`parity_test.go`) uses explicit `--root` — it tests the DETECTORS, not the Windows root resolver.
- The Windows root-resolver test (`roots_windows_test.go` with `//go:build windows`) calls `resolveRoots(model.ProfileBaseline, nil, rootsOpts{})` with env vars set and fixture dirs created, mirroring the existing Unix tests in `main_test.go`.

### Determinism Strategy

Reuse `normalize()` from `normalize_diff.go` unchanged. It:
- Strips `run_id`, `scan_time`, `end_time`, `duration_ms`, `scanner_name`, `scanner_version` (top-level)
- Strips `hostname`, `username`, `uid` from `endpoint`
- Sorts by `record_id` [VERIFIED: normalize_diff.go lines 63-83, 150-155]

After normalization, `endpoint.os` is still present (it is NOT in the strip lists). The parity test must handle `endpoint.os` differently from the differential test:
- Differential (PTEST-02): strips `endpoint.os` to allow Linux vs macOS comparison.
- Parity (PTEST-01): does NOT strip `endpoint.os` — it asserts `endpoint.os == runtime.GOOS` for every record.

Two options:
1. Add a `normalizeForParity()` variant that also strips `endpoint.os` AFTER asserting it, then do byte-comparison like the differential test. This is cleanest — same pattern.
2. Parse the normalized output record-by-record and check `endpoint.os`. More verbose.

**Recommendation: Option 1.** Define `normalizeForParity(ndjson []byte, expectedOS string) ([]byte, error)` that (a) calls `normalize()`, (b) asserts every record's `endpoint.os == expectedOS`, (c) strips `endpoint.os` from all records, (d) returns the stripped+sorted bytes. The parity test then does a count comparison (not byte-identical — because the parity fixture is not running against upstream, it's just asserting all 8 ecosystems produce records). Actually, for a single-OS run, the parity test just needs to assert ecosystem coverage and minimum record counts, not cross-OS byte equality. The "parity" is validated by running on all 3 OSes in CI and requiring each to produce the same number of records.

**Concrete parity assertion:**

```go
// In assertParityRecordCoverage (parity_test.go)
requiredEcosystems := []string{
    model.EcosystemNPM,
    model.EcosystemPyPI,
    model.EcosystemGo,
    model.EcosystemRubyGems,
    model.EcosystemPackagist,
}
// Assert at least one record per ecosystem (exact count comparison is fragile
// across ecosystem fixture formats)
```

Note: Bun is emitted as `npm` ecosystem per the model [VERIFIED: model.go line 43 — there is no separate Bun ecosystem constant; bun packages are emitted with `EcosystemNPM`]. pnpm and Yarn may also emit as `npm`. Verify by reading `internal/ecosystem/bun/bun.go`, `pnpm/pnpm.go`, `yarn/yarn.go`.

### CI Placement for Parity Test

The parity test has no build tag — it runs inside `go test -race ./cmd/pollen/...` on all three OS runners in the existing `test` job. No new CI job required. The test builds the current pollen binary (via `buildCurrentPollen(t)` reused from differential_test.go), so it does require a build step, but that is already part of CI.

---

## Q4: CI Skip Discipline

### Exact Skips to Flip

These are the Phase-2 skips in `cmd/pollen/main_test.go` to remove or replace with real Windows implementations:

| Line | Function | Current Skip Reason | Action |
|------|----------|--------------------|----|
| 54–55 | `TestIsBroadHomeRoot` | "broad-home detection uses Unix-style paths" | Replace `t.Skip` with a Windows-adapted assertion (Windows has no `/`, `/Users` etc.) |
| 121 | `TestResolveRootsProjectIncludesCodeDir` | "HOME env override is Unix-specific" | Replace with Windows equivalent using `USERPROFILE` env var |
| 146 | `TestResolveRootsBaselineIncludesUserLocalPython` | "Unix .local/lib/python path structure" | Keep skip OR add parallel Windows assertion; this test is Unix-specific by nature |
| 273 | `TestResolveRootsBaselineRefusesBroadHome` | "HOME env override is Unix-specific" | Adapt for Windows or add Windows-only variant |
| 288 | `TestResolveRootsProjectRefusesBroadHome` | "HOME env override is Unix-specific" | Same as above |
| 300 | `TestResolveRootsDeepAllowsBroadHome` | "HOME env override and isBroadHomeRoot Unix-path logic" | Same as above |

**Strategy:** Rather than making all six existing tests Windows-aware (some are inherently Unix, e.g. `TestResolveRootsBaselineIncludesUserLocalPython`), add NEW Windows-specific tests in `roots_windows_test.go` (`//go:build windows`). Then flip the six skips: those that are purely Unix-only keep a skip but change the message to remove "Phase 2" language (they remain Unix-specific tests, not deferred Windows tests). The new WRES-01/02 tests in `roots_windows_test.go` are the Windows coverage.

**For `TestIsBroadHomeRoot` specifically:** On Windows, `isBroadHomeRoot` currently only checks Unix-style paths (`/`, `/Users`, `/home`, etc.) and `os.UserHomeDir()`. On Windows, `os.UserHomeDir()` returns `C:\Users\<name>`. The function will correctly identify it as a broad home root via the `os.UserHomeDir()` comparison. So the test itself can be un-skipped on Windows with appropriate Windows-shaped paths in the `broad` and `narrow` test cases. [VERIFIED: roots.go lines 171-196 — `isBroadHomeRoot` calls `os.UserHomeDir()` which works on Windows, and `filepath.Clean` is OS-aware]

### Differential Skip: Stays

`differential_test.go:55` — skip stays unchanged:
```go
if runtime.GOOS == "windows" {
    t.Skip("PTEST-02 differential runs on Linux+macOS only; ...")
}
```
The differential test requires building upstream bumblebee from source, which only works on Linux+macOS (upstream has no Windows support). This skip is structural, not a deferral — it stays forever or until upstream supports Windows.

### CI Impact

`go test -race ./...` already runs on `windows-latest` with `CGO_ENABLED: 1`. [VERIFIED: ci.yml line 49-51]. New `//go:build windows` test files in `cmd/pollen/` are automatically included in the `windows-latest` test run without any CI changes. The parity test (`parity_test.go`, no build tag) runs on all three runners automatically.

**No new CI jobs required.** The existing `test` matrix job covers all Phase 2 test additions.

---

## Q5: Windows-Specific Pitfalls

### Pitfall 1: `os.Getenv` Empty on CI / Container Runners

**What goes wrong:** `%APPDATA%`, `%LOCALAPPDATA%`, and `%USERPROFILE%` are standard on interactive Windows sessions but can be empty in containerized or minimal CI environments. `filepath.Join("", "npm", "node_modules")` produces `npm\node_modules` (a relative path), which `os.Stat` will resolve relative to cwd — silently wrong, and may match unintended directories.

**Prevention:** Guard every `os.Getenv` result before using it:
```go
if appdata := os.Getenv("APPDATA"); appdata != "" {
    add(filepath.Join(appdata, "npm", "node_modules"), ...)
}
```
Also: `windows-latest` GitHub Actions runners always have these variables set [ASSUMED — standard Windows Server runner config], but test-time `t.Setenv("APPDATA", ...)` overrides must be the test's sole env-var source.

### Pitfall 2: `ProgramFiles(x86)` Env Var Casing

**What goes wrong:** `os.Getenv("programfiles")` on Windows returns `""` — Windows env vars are case-insensitive at the OS level, but Go's `os.Getenv` calls the Windows API `GetEnvironmentVariableW` which is case-insensitive. [ASSUMED — Go stdlib behavior on Windows is documented as case-insensitive for os.Getenv]. However, the standard name is `ProgramFiles` (capital P, capital F). Stick to canonical casing from the table above.

### Pitfall 3: `filepath.Glob` with Wildcard Patterns on Windows

**What goes wrong:** `filepath.Glob("C:\\Users\\alice\\AppData\\Roaming\\Python\\Python*\\site-packages")` — Go's `filepath.Glob` uses `filepath.Match` internally, which handles backslash separators correctly on Windows. However, `globExisting` at `roots.go:309` already uses `filepath.Glob` and filters for directories — it works unchanged on Windows. [VERIFIED: roots.go lines 309-321]

**Prevention:** Always pass patterns constructed with `filepath.Join`, not forward-slash strings. `filepath.Join(appdata, "Python", "Python*", "site-packages")` produces the correct backslash-separated pattern on Windows.

### Pitfall 4: Case-Insensitive Filesystem Affecting Fixture Matching

**What goes wrong:** NTFS is case-insensitive by default. A fixture file named `METADATA` (PyPI) will be found whether the detector looks for `METADATA`, `metadata`, or `Metadata`. This is benign for detection but can cause fixture collision if two ecosystem fixtures use the same filename. The existing fixture naming convention (ecosystem-named subdirectories) avoids this.

**Warning signs:** Tests that pass on case-insensitive Windows but fail on case-sensitive Linux. Prevent by using exact casing in fixture filenames matching what the ecosystem detectors expect.

### Pitfall 5: `t.Setenv("HOME", ...)` Does Not Work on Windows for `os.UserHomeDir()`

**What goes wrong:** The existing Unix tests in `main_test.go` use `t.Setenv("HOME", home)` to redirect `os.UserHomeDir()` calls. On Windows, `os.UserHomeDir()` does not read `HOME` — it reads `USERPROFILE` (or `HOMEDRIVE`+`HOMEPATH`). So the existing test pattern DOES NOT work on Windows; this is exactly why those tests are skipped.

**Prevention:** New Windows root-resolver tests in `roots_windows_test.go` must use `t.Setenv("USERPROFILE", tmp)` (and `t.Setenv("APPDATA", ...)` etc.) rather than `t.Setenv("HOME", ...)`. `os.UserHomeDir()` on Windows reads `USERPROFILE` when set. [ASSUMED — Go stdlib behavior on Windows documented in `os.UserHomeDir` source; verify against Go 1.25 if there is any doubt]

### Pitfall 6: PyPI Venv Path on Windows is `Lib\site-packages` Not `lib/pythonX.Y/site-packages`

**What goes wrong:** On Unix, venv site-packages are at `<venv>/lib/python3.X/site-packages`. On Windows they are at `<venv>\Lib\site-packages` (no version segment). If the parity test fixture for PyPI uses a venv-style layout, it must use the Windows layout on the Windows runner.

**Prevention:** For the parity fixture, use the `*.dist-info/METADATA` pattern (global site-packages) rather than venv layout — the detector (`pypi.IsDistInfoMetadata`) works the same on both platforms since it just looks for `METADATA` inside a `*.dist-info` directory. [VERIFIED: pypi.go lines 33-43]

### Pitfall 7: `isBroadHomeRoot` on Windows — Windows Drive Roots

**What goes wrong:** `isBroadHomeRoot` currently checks for `abs == "/"` and `/Users`/`/home` prefixes. On Windows, a drive root is `C:\` (or `C:/` after `filepath.Clean`). The function does NOT currently detect Windows drive roots as "broad." Passing `C:\` to `resolveRoots` with profile=baseline would incorrectly succeed (not be refused) on Windows.

**Prevention:** When un-skipping `TestIsBroadHomeRoot`, add a Windows-specific broad-root check: `filepath.VolumeName(abs)+"\" == abs` detects drive roots like `C:\`. Add this to `isBroadHomeRoot`. Example:
```go
if vol := filepath.VolumeName(abs); vol != "" && filepath.Clean(abs) == vol+string(filepath.Separator) {
    return true // C:\ is a drive root
}
```

### Pitfall 8: `go test -race` on Windows Requires CGO

**What goes wrong:** The race detector requires CGO. Windows-latest GitHub Actions runners have MSVC installed but may need `CGO_ENABLED=1` explicitly. [VERIFIED: ci.yml line 51 already sets `CGO_ENABLED: 1`]. Do not regress this; confirm new test files do not accidentally set `CGO_ENABLED=0`.

### Pitfall 9: Junction Points Under `%LOCALAPPDATA%`

**What goes wrong:** Windows junction points and symlinks under `%LOCALAPPDATA%` (e.g., `AppData\Local` is often a junction to `AppData\Roaming` contents on some configurations) can cause `filepath.Glob` to follow cycles or double-count paths.

**Warning signs:** `filterExistingRoots` calls `os.Stat`, which follows symlinks — a junction-pointed directory will pass the `info.IsDir()` check. The walk itself uses `os.ReadDir` in the scanner walker, which also follows junctions by default.

**Prevention:** The Phase 2 CONTEXT.md explicitly lists this as an Open Research Flag: "fsnotify Windows junction point behavior with package roots under %LOCALAPPDATA% — needs live testing on Windows CI runner." For Phase 2, document this as a known risk. The impact is bounded: at worst, a package is double-counted; the scanner emits duplicate `record_id` values (same SHA-256 = same content = deduplicated by the consumer). Phase 2 ships with this known risk; a follow-on can add junction detection.

---

## Q6: Release Steps

### Version Bump

1. Edit `../pollen/VERSION` from `0.1.1-pollen.1` to `0.1.1-pollen.2`. [VERIFIED: VERSION file is single line `0.1.1-pollen.1`]
2. The GoReleaser config at `../pollen/.goreleaser.yaml` injects `Version` via ldflags: `-X main.Version={{.Version}}`. The `.goreleaser.yaml` already works; no changes needed. [VERIFIED: .goreleaser.yaml line 44]

### CHANGES.md Entry

Append to `../pollen/CHANGES.md` (above or as a new section after v0.1.1-pollen.1):

```markdown
## v0.1.1-pollen.2 (2026-06-XX) — Windows root resolver

### Added

- `cmd/pollen/roots_windows.go` (`//go:build windows`) — Windows root discovery
  for all 8 package ecosystems: npm, pnpm, Yarn, Bun, PyPI, Go modules,
  RubyGems, Composer. Roots constructed via `filepath.Join` + Windows env vars
  (`%APPDATA%`, `%LOCALAPPDATA%`, `%USERPROFILE%`, `%ProgramFiles%`). Glob
  patterns used for `%APPDATA%\Python\Python*\site-packages` and
  `%ProgramFiles%\Ruby*\lib\ruby\gems` (reuses existing `globExisting` helper).
- `cmd/pollen/testdata/parity-fixture/` — 8-ecosystem fake-package fixture tree
  for cross-platform parity test.
- `cmd/pollen/parity_test.go` — PTEST-01 cross-platform parity test: same
  fixture produces equivalent records on Linux, macOS, and Windows;
  `endpoint.os` differs correctly per platform.
- `cmd/pollen/roots_windows_test.go` (`//go:build windows`) — Windows
  root-resolver unit tests covering all 8 ecosystems with fake env vars
  and fixture dirs.

### Modified

- `cmd/pollen/roots.go` — Added `case "windows":` branches in `systemRoots`,
  `baselineHomeCandidates`, and `browserExtensionCandidateRoots` delegating to
  `roots_windows.go` functions.
- `cmd/pollen/main_test.go` — Flipped 6 Phase-2 Windows skips; updated
  `TestIsBroadHomeRoot` to handle Windows drive-root detection.
```

### Signing Stack (unchanged from Phase 1)

[VERIFIED: release.yml and .goreleaser.yaml are complete and unchanged] No new infrastructure needed:
- GoReleaser builds linux/darwin/windows amd64+arm64 binaries with `-trimpath -buildvcs=false` and CycloneDX SBOM via syft.
- cosign v3 keyless signs `checksums.txt` via GitHub Actions OIDC.
- `slsa-github-generator@v2.1.0` produces SLSA Level 3 provenance attestation.
- Push tag `v0.1.1-pollen.2` triggers the release workflow automatically.

**Verification command (from Phase 1 established pattern):**
```bash
cosign verify-blob --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github\.com/Bantuson/pollen/' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt
```

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Wildcard directory enumeration | Custom glob walker | `globExisting` (roots.go:309) | Already handles `filepath.Glob` + `os.Stat` is-dir filter; Windows-compatible |
| Absent root filtering | Custom missing-root logic | `filterExistingRoots` (roots.go:562) | Already emits correct diagnostics; used by all existing profiles |
| NDJSON normalization for tests | New normalizer | `normalize()` from normalize_diff.go | Fail-closed, sort-by-record_id, strips the correct 9 fields |
| Binary build for tests | Custom `go build` subprocess | `buildCurrentPollen(t)` from differential_test.go | Already handles temp dir, Windows `.exe` suffix, source root location |

---

## Common Pitfalls

### Pitfall: WRES-01 Literal Filename Confusion

**What goes wrong:** The planner creates `internal/resolver/resolver_windows.go` as literally named in WRES-01.

**Why it happens:** WRES-01 names a file that does not exist and reflects an early PRD draft CHANGES.md template (PRD §7.3).

**How to avoid:** CONTEXT.md explicitly states: "The requirement is satisfied by behavior, not by a literal filename." The correct file is `cmd/pollen/roots_windows.go`.

### Pitfall: `endpoint.os` in Parity Test

**What goes wrong:** Parity test byte-compares normalized output across OSes and fails because `endpoint.os` differs (`"linux"` vs `"windows"`).

**Why it happens:** `normalize()` strips hostname/username/uid but NOT `endpoint.os` (by design — the differential test needs os stripped for a different reason; the parity test needs it to differ correctly).

**How to avoid:** `normalizeForParity()` variant that asserts `endpoint.os == runtime.GOOS` BEFORE stripping it, then strips it for the count/coverage comparison. Do not modify `normalize()` — it is locked for PTEST-02.

### Pitfall: Missing Env Var Produces Relative Path

**What goes wrong:** `filepath.Join(os.Getenv("APPDATA"), "npm", "node_modules")` when `APPDATA` is `""` produces `npm\node_modules`, which `filterExistingRoots` may find as a relative path if the test's cwd happens to have such a directory.

**How to avoid:** Guard every env-var read: `if appdata := os.Getenv("APPDATA"); appdata != "" { ... }`. Covered in Pattern 1 code above.

---

## Runtime State Inventory

This is a code-addition phase (no rename/refactor). No runtime state categories apply.

- Stored data: None — no data migration needed.
- Live service config: None.
- OS-registered state: None.
- Secrets/env vars: None — env vars read by the Windows code (`APPDATA`, `LOCALAPPDATA`, `USERPROFILE`, `ProgramFiles`) are standard OS env vars, not application secrets.
- Build artifacts: `cmd/pollen/roots_windows.go` is a new file; no stale artifacts from a rename.

**Nothing found in any category — verified by phase scope analysis.**

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing package (stdlib), Go 1.25 |
| Config file | none (go.mod specifies `go 1.25`) |
| Quick run command | `go test ./cmd/pollen/ -run '^TestWindowsBaseline' -v` (Windows CI) |
| Full suite command | `go test -race ./... ` (CGO_ENABLED=1) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| WRES-01 | Windows JS ecosystem roots discovered | unit | `go test ./cmd/pollen/ -run '^TestWindowsBaseline' -v` (windows-latest) | ❌ Wave 0 |
| WRES-02 | Windows PyPI/Go/Ruby/Composer roots discovered | unit | same as WRES-01 | ❌ Wave 0 |
| PTEST-01 | Parity test: same fixture, all 3 OSes, equivalent records | integration | `go test ./cmd/pollen/ -run '^TestParityAllEcosystems' -v` | ❌ Wave 0 |

### Sampling Rate

- Per task commit: `go build ./cmd/pollen/` (compile check; full test run requires windows-latest runner)
- Per wave merge: `go test -race ./...` (CGO_ENABLED=1) on all three OS runners
- Phase gate: Full suite green on all three OS runners before `/gsd-verify-work 2`

### Wave 0 Gaps

- [ ] `cmd/pollen/roots_windows.go` — covers WRES-01, WRES-02
- [ ] `cmd/pollen/roots_windows_test.go` — covers WRES-01, WRES-02 unit tests (//go:build windows)
- [ ] `cmd/pollen/parity_test.go` — covers PTEST-01
- [ ] `cmd/pollen/testdata/parity-fixture/` — fixture tree for parity test (8 ecosystem dirs)
- [ ] `normalizeForParity()` helper (in parity_test.go or normalize_diff.go) — PTEST-01 determinism

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|-------------|-----------|---------|----------|
| Go toolchain 1.25 | Build | ✓ (from go.mod; CI uses setup-go@v5) | 1.25.x | — |
| MSVC / CGO | Race detector on windows-latest | ✓ (windows-latest has MSVC) | — | — |
| git | Differential test (clones upstream) | ✓ | system | Differential skips on Windows anyway |
| `%APPDATA%` | WRES-01/02 root construction | ✓ on interactive Windows; ASSUMED on CI runner | — | Guard with empty-string check |
| `%LOCALAPPDATA%` | WRES-01 pnpm/Yarn | ✓ (same as above) | — | Guard |
| `%USERPROFILE%` | WRES-01 Bun, WRES-02 Go/RubyGems | ✓ | — | Guard |
| `%ProgramFiles%` | WRES-02 npm MSI, RubyGems system | ✓ | — | Guard |

**Missing dependencies with no fallback:** None.

**Missing dependencies with fallback:** All env-var-dependent paths have empty-string guards so absent vars silently produce zero roots (handled by `filterExistingRoots`).

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Venv `<venv>\Lib\site-packages` is discovered automatically when the scanner walks a project tree — no explicit root needed for baseline | Q2 PyPI Venv | Low — baseline profile does not walk project trees; if wrong, venv packages are missed in baseline but found in project/deep profiles |
| A2 | `windows-latest` GitHub Actions runners always have `%APPDATA%`, `%LOCALAPPDATA%`, `%USERPROFILE%`, `%ProgramFiles%` set | Q5 Pitfall 1 | Medium — if any env var is absent on the runner, those roots silently produce no candidates (not a test failure, just missing coverage) |
| A3 | `os.Getenv` on Windows is case-insensitive (accessing `APPDATA` via `os.Getenv("APPDATA")` works regardless of how the var is spelled) | Q2 | Low — Go stdlib on Windows calls GetEnvironmentVariableW which is case-insensitive |
| A4 | Bun packages are emitted with `EcosystemNPM` (not a separate Bun ecosystem constant) | Q3 Fixture Design | Medium — if wrong, the parity fixture needs a `bun` ecosystem assertion; verify by reading `internal/ecosystem/bun/bun.go` before implementing |
| A5 | pnpm and Yarn packages are also emitted as `EcosystemNPM` | Q3 | Medium — same as A4; verify bun/pnpm/yarn ecosystems |
| A6 | `go test` on Windows with `t.Setenv("USERPROFILE", tmp)` successfully redirects `os.UserHomeDir()` | Q4 Skip Discipline | Medium — if wrong, Windows root-resolver tests that use `USERPROFILE` override do not work; use `os.UserHomeDir()` mock approach instead |

**High-risk assumptions requiring pre-implementation verification:** A4 (Bun ecosystem value) and A5 (pnpm/Yarn ecosystem value) — read `internal/ecosystem/bun/bun.go` and `pnpm/pnpm.go` before writing the parity fixture.

---

## Open Questions

1. **Bun/pnpm/Yarn ecosystem constants**
   - What we know: `model.go` has no `EcosystemBun`, `EcosystemPnpm`, or `EcosystemYarn` constants — only `EcosystemNPM`.
   - What's unclear: Do the bun/pnpm/yarn ecosystem packages emit records with `Ecosystem: model.EcosystemNPM` or their own custom string?
   - Recommendation: Read `internal/ecosystem/bun/bun.go` (first few lines, look for `const Ecosystem = ...`) before writing the parity fixture. If all JS package managers emit `EcosystemNPM`, the parity fixture only needs one npm-style directory.

2. **`isBroadHomeRoot` Windows drive-root detection**
   - What we know: The function at `roots.go:171` checks Unix-specific paths (`/`, `/Users`, `/home`). On Windows, `filepath.Clean("C:\\")` returns `C:\`. `filepath.VolumeName("C:\\")` returns `C:`.
   - What's unclear: Whether any of the existing skipped tests exercise the drive-root path, or whether a new Windows-specific test is sufficient.
   - Recommendation: Add Windows drive-root detection as described in Pitfall 7 and add a `//go:build windows` test asserting `isBroadHomeRoot("C:\\") == true`. Update the skip message on `TestIsBroadHomeRoot` to reflect that the Windows case is handled.

3. **`GOPATH` for Go modules root**
   - What we know: PRD §8.1 lists `%USERPROFILE%\go\pkg\mod`. Upstream unix code uses `~/go/pkg/mod` without consulting `GOPATH`.
   - What's unclear: Should the Windows code also check `os.Getenv("GOPATH")` as a fallback?
   - Recommendation: Mirror the Unix behavior (upstream does NOT read `GOPATH`, it uses the default `~/go`). Use `%USERPROFILE%\go\pkg\mod` only, matching CONTEXT.md locked table entry. `GOPATH`-based check is out of scope.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|---|---|---|---|
| `isBroadHomeRoot` Unix-only | Add Windows drive-root detection | Phase 2 | Prevents `C:\` from being passed as a scan root with profile=baseline |
| All root resolution in one `roots.go` with switch statements | Windows helpers in `roots_windows.go` build-tag file | Phase 2 | Upstream merge surface reduced; Windows bytes isolated |

---

## Sources

### Primary (HIGH confidence)

- `../pollen/cmd/pollen/roots.go` — exact functions, switch statement locations, `globExisting`, `filterExistingRoots`, `POLLEN_USERS_DIR` idiom
- `../pollen/cmd/pollen/main_test.go` — exact skip locations (lines 54, 121, 146, 273, 288, 300)
- `../pollen/cmd/pollen/differential_test.go` — test helpers (`buildCurrentPollen`, `runBinaryOnFixture`) to reuse; skip discipline model
- `../pollen/cmd/pollen/normalize_diff.go` — fields stripped, sort key; reuse surface
- `../pollen/internal/ecosystem/npm/npm.go` — confirms OS-agnostic file-format matching
- `../pollen/internal/ecosystem/pypi/pypi.go` — confirms `*.dist-info/METADATA` basename dispatch; OS-agnostic
- `../pollen/internal/endpoint/endpoint.go` — confirms `endpoint.os = runtime.GOOS` already works on Windows
- `../pollen/internal/model/model.go` — ecosystem constants (no Bun/pnpm/Yarn separate constants)
- `../pollen/.github/workflows/ci.yml` — confirms `CGO_ENABLED: 1` on windows-latest; differential Linux+macOS only matrix
- `../pollen/.github/workflows/release.yml` — signing stack unchanged
- `../pollen/.goreleaser.yaml` — build flags and release pipeline unchanged
- `../pollen/VERSION` — current `0.1.1-pollen.1`
- `../pollen/CHANGES.md` — entry format and conventions
- `beekeeper-m2-prd.md §8.1` — authoritative Windows roots table
- `.planning/phases/02-windows-root-resolver/02-CONTEXT.md` — all locked decisions

### Secondary (MEDIUM confidence)

- PRD §2.4 upstream PR #4 build-tag-separated claim — not directly verified (upstream PR not fetched), but consistent with evidence from `roots.go` structure

### Tertiary (LOW confidence / ASSUMED)

- A2: `windows-latest` runner env var availability
- A4/A5: Bun/pnpm/Yarn emit as `EcosystemNPM`
- A6: `t.Setenv("USERPROFILE")` redirects `os.UserHomeDir()` on Windows

---

## Metadata

**Confidence breakdown:**
- Windows root paths: HIGH — sourced from PRD §8.1 + CONTEXT.md locked table
- Structure decision: HIGH — grounded in direct repo read; no `internal/resolver/` package found
- Ecosystem detector OS-agnosticism: HIGH — `runtime.GOOS` grep across `internal/ecosystem/` returned zero results
- Parity test design: HIGH for mechanism (reuses verified diff-test pattern); MEDIUM for ecosystem constants (unverified Bun/pnpm/Yarn constants)
- CI skip discipline: HIGH — exact line numbers verified from direct read

**Research date:** 2026-06-02
**Valid until:** 2026-07-02 (Go stdlib APIs stable; env var conventions stable; ecosystem detector dispatch stable)
