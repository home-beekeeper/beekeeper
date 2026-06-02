# Phase 2: Windows Root Resolver — Pattern Map

**Mapped:** 2026-06-02
**Files analyzed:** 4 new/modified files
**Analogs found:** 4 / 4

All analogs are in the sibling repo `C:\Users\Bantu\mzansi-agentive\pollen`. The beekeeper
`internal/` tree has no resolver analogs and was not searched.

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `cmd/pollen/roots_windows.go` (NEW) | utility / root-resolver | request-response (OS env → []scanner.Root) | `cmd/pollen/roots.go` | exact — same package, same types, same helpers |
| `cmd/pollen/roots_windows_test.go` (NEW, `//go:build windows`) | test | CRUD (create fixture dirs → assert roots) | `cmd/pollen/main_test.go` | exact — same test helpers, same `resolveRoots` call shape |
| `cmd/pollen/parity_test.go` (NEW, no build tag) | test | request-response (build binary → run → normalize → assert) | `cmd/pollen/differential_test.go` + `normalize_diff.go` | exact — same `buildCurrentPollen` + `runBinaryOnFixture` pattern |
| `cmd/pollen/testdata/parity-fixture/` (NEW dir) | fixture | file-I/O (on-disk fake package metadata) | `cmd/pollen/testdata/diff-fixture/` + `cmd/pollen/selftest/fixtures/` | exact — same per-ecosystem subdirectory layout |

---

## Pattern Assignments

### `cmd/pollen/roots_windows.go` (utility, request-response)

**Analog:** `C:\Users\Bantu\mzansi-agentive\pollen\cmd\pollen\roots.go`

**Imports pattern** (roots.go lines 29–38):
```go
import (
    "fmt"
    "os"
    "path/filepath"
    "runtime"
    "strings"

    "github.com/bantuson/pollen/internal/model"
    "github.com/bantuson/pollen/internal/scanner"
)
```
`roots_windows.go` only needs `"os"`, `"path/filepath"`, `"github.com/bantuson/pollen/internal/model"`, and `"github.com/bantuson/pollen/internal/scanner"` — drop `fmt`, `runtime`, `strings` (those stay in roots.go).

**Build tag — must be first line of file, before package declaration:**
```go
//go:build windows

package main
```
There are zero existing `//go:build` files in the repo today; this file introduces the pattern. The blank line between the build tag and `package main` is required by Go toolchain.

**Core `add` helper pattern** (roots.go lines 208–209 — `baselineHomeCandidates`):
```go
var out []scanner.Root
add := func(p, kind string) { out = append(out, scanner.Root{Path: p, Kind: kind}) }
```
`roots_windows.go` uses the same `add` closure idiom but wraps every env-var read with an explicit non-empty guard rather than a single upfront `if home == ""` check, because Windows has multiple independent env vars:
```go
// Pattern: guard at the env-var level (Pitfall 1 prevention)
if appdata := os.Getenv("APPDATA"); appdata != "" {
    add(filepath.Join(appdata, "npm", "node_modules"), model.RootKindGlobalPackage)
}
```

**`scanner.Root` construction pattern** (roots.go lines 292–306 — `systemRoots` darwin/linux cases):
```go
// darwin case — exact shape to mirror for Windows:
return []scanner.Root{
    {Path: "/opt/homebrew/lib", Kind: model.RootKindHomebrew},
    {Path: "/usr/local/lib",    Kind: model.RootKindHomebrew},
    {Path: "/Library/Python",   Kind: model.RootKindHomebrew},
}
// linux case — append pattern with glob:
roots := []scanner.Root{{Path: "/usr/local/lib", Kind: model.RootKindGlobalPackage}}
for _, pattern := range []string{"/usr/lib/python*"} {
    for _, p := range globExisting(pattern) {
        roots = append(roots, scanner.Root{Path: p, Kind: model.RootKindGlobalPackage})
    }
}
return roots
```
Windows functions use the `append` + `globExisting` form because Python and RubyGems paths need wildcard expansion.

**`globExisting` helper** (roots.go lines 309–321 — called unchanged from `roots_windows.go`):
```go
func globExisting(pattern string) []string {
    matches, err := filepath.Glob(pattern)
    if err != nil {
        return nil
    }
    var out []string
    for _, p := range matches {
        if info, err := os.Stat(p); err == nil && info.IsDir() {
            out = append(out, p)
        }
    }
    return out
}
```
`roots_windows.go` calls `globExisting(filepath.Join(appdata, "Python", "Python*", "site-packages"))` directly — `globExisting` is defined in `roots.go` (same package), no import needed.

**`filterExistingRoots` is called from `baselineDefaultRoots`** (roots.go lines 340–341) — `roots_windows.go` does NOT call it; the caller in `roots.go` already calls it after collecting all candidates. `windowsBaselinePackageRoots()` just returns raw candidates.

**`switch runtime.GOOS` delegation pattern** (roots.go lines 250–257 for MCP config, lines 290–306 for systemRoots, lines 500–558 for browserExtensionCandidateRoots):
```go
// Existing pattern in baselineHomeCandidates (lines 250–257):
switch runtime.GOOS {
case "darwin":
    add(filepath.Join(home, "Library", "Application Support", "Claude"), model.RootKindMCPConfig)
case "linux":
    add(filepath.Join(home, ".config", "Claude"), model.RootKindMCPConfig)
    add(filepath.Join(home, ".config", "Claude Code"), model.RootKindMCPConfig)
    add(filepath.Join(home, ".continue"), model.RootKindMCPConfig)
}

// Existing pattern in systemRoots (lines 289–307):
func systemRoots() []scanner.Root {
    switch runtime.GOOS {
    case "darwin":
        return []scanner.Root{ ... }
    case "linux":
        ...
        return roots
    }
    return nil  // ← default: returns nil — filterExistingRoots handles empty correctly
}
```
Add `case "windows":` before the closing `}` in each switch. The `default: return nil` fallthrough already exists and handles unknown OSes correctly — Windows just needs its own case.

**`isBroadHomeRoot` Windows drive-root gap** (roots.go lines 171–196):
```go
func isBroadHomeRoot(path string) bool {
    // ... existing checks for "/" and "/Users/<name>" etc.
    // Missing: Windows drive-root "C:\" detection.
    // Add before the final `return false`:
    if vol := filepath.VolumeName(abs); vol != "" {
        if abs == vol+string(filepath.Separator) {
            return true // C:\ is a broad filesystem root
        }
    }
    return false
}
```
This change is in `roots.go` (not `roots_windows.go`) because `isBroadHomeRoot` is a non-build-tagged function. The Windows drive-root check uses `filepath.VolumeName` which returns `""` on Unix, so it is a no-op on non-Windows — safe to add unconditionally.

**RootKind constants to use** (model.go lines 87–96):
```go
RootKindGlobalPackage = "global_package_root"  // npm MSI, RubyGems system
RootKindUserPackage   = "user_package_root"    // everything else Windows
```

**Ecosystem constants** (model.go lines 38–46) — confirmed assignments for all JS managers:
```go
// bun.go line 27:  const Ecosystem = model.EcosystemNPM
// pnpm.go line 25: const Ecosystem = model.EcosystemNPM  // comment: "pnpm installs npm-registry packages"
// yarn.go line 25: const Ecosystem = model.EcosystemNPM
// composer.go line 28: const Ecosystem = model.EcosystemPackagist
// rubygems.go line 28:  const Ecosystem = model.EcosystemRubyGems
// gomod.go line 28:     const Ecosystem = model.EcosystemGo
```
Bun, pnpm, and Yarn all emit `EcosystemNPM`. The parity fixture needs only one JS-ecosystem entry to cover all three.

---

### `cmd/pollen/roots_windows_test.go` (test, `//go:build windows`)

**Analog:** `C:\Users\Bantu\mzansi-agentive\pollen\cmd\pollen\main_test.go`

**Build tag pattern** — same as production file:
```go
//go:build windows

package main
```

**Imports pattern** (main_test.go lines 1–11):
```go
import (
    "os"
    "path/filepath"
    "runtime"
    "testing"

    "github.com/bantuson/pollen/internal/model"
)
```
`roots_windows_test.go` uses the same imports; `runtime` is not needed if the file is already build-tagged `windows`, but is fine to include for `runtime.GOOS` guards inside tests.

**`t.Setenv` + `t.TempDir` pattern** (main_test.go lines 57–58, 125–126):
```go
home := t.TempDir()
t.Setenv("HOME", home)         // Unix pattern — do NOT copy this for Windows tests
```
Windows equivalent — MUST use `USERPROFILE`, `APPDATA`, `LOCALAPPDATA`:
```go
tmp := t.TempDir()
t.Setenv("USERPROFILE", tmp)
t.Setenv("APPDATA", filepath.Join(tmp, "AppData", "Roaming"))
t.Setenv("LOCALAPPDATA", filepath.Join(tmp, "AppData", "Local"))
t.Setenv("ProgramFiles", filepath.Join(tmp, "ProgramFiles"))
```
`t.Setenv` automatically restores the original value after the test — same guarantee as Unix tests.

**`os.MkdirAll` + `resolveRoots` call pattern** (main_test.go lines 127–142):
```go
codeDir := filepath.Join(home, "code")
if err := os.MkdirAll(codeDir, 0o755); err != nil {
    t.Fatal(err)
}
roots, _, err := resolveRoots(model.ProfileProject, nil, rootsOpts{})
if err != nil {
    t.Fatalf("resolveRoots project: %v", err)
}
found := false
for _, r := range roots {
    if r.Path == codeDir && r.Kind == model.RootKindProject {
        found = true
    }
}
if !found {
    t.Fatalf("project profile did not include %q, got %v", codeDir, roots)
}
```
Windows root-resolver tests follow the identical structure: create a fake dir under the overridden env-var path, call `resolveRoots(model.ProfileBaseline, nil, rootsOpts{})`, assert the dir is in the result with the expected `Kind`.

**Phase-2 Windows skip lines to flip** (main_test.go — exact line numbers):

| Line | Function | Current skip text (to remove/replace) |
|------|----------|---------------------------------------|
| 54–55 | `TestIsBroadHomeRoot` | `"broad-home detection uses Unix-style paths; Windows root-resolver tests arrive in Phase 2 (v0.1.1-pollen.2)"` |
| 121 | `TestResolveRootsProjectIncludesCodeDir` | `"HOME env override for resolveRoots is Unix-specific; Windows root-resolver tests arrive in Phase 2 (v0.1.1-pollen.2)"` |
| 146 | `TestResolveRootsBaselineIncludesUserLocalPython` | `"Unix .local/lib/python path structure; Windows root-resolver tests arrive in Phase 2 (v0.1.1-pollen.2)"` |
| 273 | `TestResolveRootsBaselineRefusesBroadHome` | `"HOME env override for broad-home detection is Unix-specific; Windows root-resolver tests arrive in Phase 2 (v0.1.1-pollen.2)"` |
| 288 | `TestResolveRootsProjectRefusesBroadHome` | same pattern |
| 300 | `TestResolveRootsDeepAllowsBroadHome` | `"HOME env override and isBroadHomeRoot Unix-path logic; Windows root-resolver tests arrive in Phase 2 (v0.1.1-pollen.2)"` |

Strategy: Tests that are inherently Unix-specific (lines 146, 96–118, 173–207 etc.) keep a skip with the message updated to remove "Phase 2" language and add "Unix-specific". Tests for functions that also need Windows behavior (lines 54, 273, 288, 300) either get un-skipped + adapted or redirect to new `roots_windows_test.go` coverage.

**`TestIsBroadHomeRoot` un-skip adaptation** — add Windows-shaped test cases to the existing `broad` and `narrow` slices:
```go
// Add to broad[] for Windows:
`C:\`,
`C:\Users`,
os.Getenv("USERPROFILE"),   // current user home

// Add to narrow[] for Windows:
filepath.Join(os.Getenv("USERPROFILE"), "code"),
filepath.Join(os.Getenv("USERPROFILE"), ".vscode", "extensions"),
`C:\Users\someone\code`,
```
Also add the drive-root detection in `isBroadHomeRoot` (see roots.go modification above).

---

### `cmd/pollen/parity_test.go` (test, no build tag)

**Analogs:** `C:\Users\Bantu\mzansi-agentive\pollen\cmd\pollen\differential_test.go` + `normalize_diff.go`

**Imports pattern** (differential_test.go lines 29–40):
```go
import (
    "bytes"
    "encoding/json"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "sort"
    "strings"
    "testing"
)
```
`parity_test.go` needs: `"bytes"`, `"encoding/json"`, `"os/exec"`, `"path/filepath"`, `"runtime"`, `"strings"`, `"testing"`. Drop `"sort"` if `normalize()` already sorts; keep `"encoding/json"` for the `endpoint.os` extraction before normalization.

**`buildCurrentPollen` helper** (differential_test.go lines 253–281 — reuse verbatim):
```go
func buildCurrentPollen(t *testing.T) string {
    t.Helper()
    _, thisFile, _, ok := runtime.Caller(0)
    if !ok {
        t.Fatal("runtime.Caller failed — cannot locate pollen source root")
    }
    // thisFile is cmd/pollen/differential_test.go — go up two levels.
    sourceRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
    buildDir := t.TempDir()
    binaryName := "pollen-under-test"
    if runtime.GOOS == "windows" {
        binaryName += ".exe"
    }
    binaryPath := filepath.Join(buildDir, binaryName)
    cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/pollen")
    cmd.Dir = sourceRoot
    if out, err := cmd.CombinedOutput(); err != nil {
        t.Fatalf("buildCurrentPollen: go build ./cmd/pollen failed: %v\n%s", err, out)
    }
    return binaryPath
}
```
`parity_test.go` calls `buildCurrentPollen(t)` directly — it is in the same package (`package main`), so no duplication if it appears only in `differential_test.go`. However, since both test files are in `package main`, the function is already in scope. Do NOT redefine it; just call it.

**`runBinaryOnFixture` helper** (differential_test.go lines 296–320 — same invocation shape):
```go
cmd := exec.Command(binaryPath,
    "scan",
    "--profile", "deep",
    "--root", fixtureDir,
    "--emit-summary=false",
)
out, err := cmd.Output()
// exit code 1 handling — partial scan is acceptable:
exitErr, isExit := err.(*exec.ExitError)
if !isExit || exitErr.ExitCode() == 2 {
    t.Fatalf("runBinaryOnFixture(%s): exit %v\nstdout: %s\nstderr: %s",
        binaryPath, err, out, exitErr.Stderr)
}
```
`parity_test.go` can call `runBinaryOnFixture(t, pollenExe, fixtureDir)` directly since it is in scope from `differential_test.go`.

**`runtime.Caller(0)` fixture path pattern** (differential_test.go lines 61–68):
```go
_, thisFile, _, ok := runtime.Caller(0)
if !ok {
    t.Fatal("runtime.Caller failed — cannot locate testdata")
}
fixtureDir := filepath.Join(filepath.Dir(thisFile), "testdata", "diff-fixture")
if _, err := os.Stat(fixtureDir); err != nil {
    t.Fatalf("testdata/diff-fixture not found at %q: %v", fixtureDir, err)
}
```
`parity_test.go` uses the identical pattern with `"testdata", "parity-fixture"`.

**`normalize()` call and usage** (differential_test.go lines 95–102 and normalize_diff.go lines 100–173):
```go
pollenNorm, err := normalize(pollenNDJSON)
if err != nil {
    t.Fatalf("normalize(pollen output): %v\nraw output:\n%s", err, pollenNDJSON)
}
```
`normalize()` (normalize_diff.go line 100) strips `run_id`, `scan_time`, `end_time`, `duration_ms`, `scanner_name`, `scanner_version` (top-level) and `hostname`, `username`, `uid` (endpoint). It does NOT strip `endpoint.os`. `parity_test.go` must extract `endpoint.os` BEFORE calling `normalize()`, then define `normalizeForParity()` as a thin wrapper:
```go
// normalizeForParity asserts endpoint.os == expectedOS on every record,
// then strips endpoint.os so the same normalized bytes can be compared
// across OS runs for record-count / ecosystem-coverage assertions.
// Does NOT modify normalize() — that function is locked for PTEST-02.
func normalizeForParity(ndjson []byte, expectedOS string) ([]byte, error) {
    // Step 1: verify endpoint.os before stripping
    lines := strings.Split(strings.TrimSpace(string(ndjson)), "\n")
    for i, line := range lines {
        if line == "" {
            continue
        }
        var rec map[string]any
        if err := json.Unmarshal([]byte(line), &rec); err != nil {
            return nil, fmt.Errorf("line %d: %w", i+1, err)
        }
        if ep, ok := rec["endpoint"].(map[string]any); ok {
            if os, _ := ep["os"].(string); os != expectedOS {
                return nil, fmt.Errorf("line %d: endpoint.os = %q, want %q", i+1, os, expectedOS)
            }
        }
    }
    // Step 2: strip endpoint.os by adding it to a local strip set, then call normalize()
    // Simplest approach: mutate the NDJSON before passing to normalize() by
    // re-marshalling each record with os removed.
    // (Implementation detail: define a small pre-processing pass here.)
    return normalize(ndjson) // normalize() already handles the determinism stripping
    // Note: endpoint.os will remain in the normalize() output because it is not
    // in endpointStripKeys. A dedicated assertAndStrip pass is needed if
    // byte-identical cross-OS comparison is required. For Phase 2 PTEST-01,
    // per-OS count/coverage assertion is sufficient — no cross-OS byte comparison.
}
```

**CI skip-vs-fail pattern** (differential_test.go lines 78–80) — PTEST-01 does NOT need this because the parity fixture is committed (not cloned from network). The parity test never calls `buildUpstreamBumblebee`. No CI-vs-local skip asymmetry needed.

**Windows skip that stays** (differential_test.go lines 55–57):
```go
if runtime.GOOS == "windows" {
    t.Skip("PTEST-02 differential runs on Linux+macOS only; Windows behavior arrives Phase 2 (v0.1.1-pollen.2)")
}
```
This skip in `differential_test.go` is structural — it stays. Do not touch it. Update the message text only if it implies Windows differential is coming (it is not a Phase 2 item).

---

### `cmd/pollen/testdata/parity-fixture/` (fixture, file-I/O)

**Analog:** `C:\Users\Bantu\mzansi-agentive\pollen\cmd\pollen\testdata\diff-fixture\`

**Existing diff-fixture directory structure:**
```
testdata/diff-fixture/
  npm-fixture/
    package-lock.json              # lockfileVersion: 3
    node_modules/
      diff-fixture-canary/         # (empty dir — scanner reads lockfile, not node_modules)
  pypi-fixture/
    diff_fixture_canary-0.0.0.dist-info/
      METADATA                     # Name: ...\nVersion: ...
  mcp-fixture/
    mcp.json                       # {"mcpServers": {...}}
```

**`package-lock.json` shape** (diff-fixture/npm-fixture/package-lock.json lines 1–20):
```json
{
  "name": "diff-fixture-npm",
  "version": "0.0.0",
  "lockfileVersion": 3,
  "requires": true,
  "packages": {
    "": { "name": "diff-fixture-npm", "version": "0.0.0" },
    "node_modules/diff-fixture-canary": {
      "version": "1.2.3",
      "resolved": "https://registry.npmjs.org/..."
    }
  }
}
```

**`METADATA` shape** (diff-fixture/pypi-fixture/.../METADATA):
```
Metadata-Version: 2.1
Name: diff-fixture-canary
Version: 0.0.0
Summary: Differential test fixture package (NOT a real package).
```

**Parity fixture directory structure** — mirrors diff-fixture but with 8 ecosystems:
```
testdata/parity-fixture/
  npm-fixture/
    package-lock.json              # one npm package: parity-npm-canary@1.0.0
  pnpm-fixture/
    pnpm-lock.yaml                 # one pnpm package (ecosystem=npm)
  yarn-fixture/
    yarn.lock                      # one yarn package (ecosystem=npm)
  bun-fixture/
    bun.lock                       # one bun package (ecosystem=npm); text JSONC format
  pypi-fixture/
    parity_pypi_canary-1.0.0.dist-info/
      METADATA                     # Name: parity-pypi-canary, Version: 1.0.0
  gomod-fixture/
    go.sum                         # one module line: module v1.0.0 h1:...=
  rubygems-fixture/
    specifications/
      parity-gem-1.0.0.gemspec     # minimal gemspec: s.name + s.version
  composer-fixture/
    composer.lock                  # {"packages":[{"name":"parity/composer-canary","version":"1.0.0"}]}
```

Key notes:
- Use `parity-*` names in all fixtures to avoid collision with selftest or diff-fixture packages.
- `bun.lock` (not `bun.lockb`) — bun.go line 35 `IsTextLockfile` dispatches on `"bun.lock"`. Binary `bun.lockb` is logged as diagnostic but emits no records. Use text format.
- `pnpm-lock.yaml` — pnpm.go `IsPnpmLockfile` dispatches on this filename.
- `yarn.lock` — yarn.go `IsYarnLock` dispatches on this filename.
- `go.sum` — gomod.go line 36 `IsGoSum` dispatches on `"go.sum"`.
- `*.gemspec` under `specifications/` — rubygems.go line 137 `IsInstalledGemspec` checks parent dir name is `"specifications"`.
- `composer.lock` — composer.go line 36 `IsComposerLock` dispatches on `"composer.lock"`.

**Ecosystem constant → expected `ecosystem` field** (verified from detector source):

| Fixture dir | Detector file | `const Ecosystem` | NDJSON `ecosystem` value |
|---|---|---|---|
| npm-fixture | npm/npm.go | `EcosystemNPM` | `"npm"` |
| pnpm-fixture | pnpm/pnpm.go line 25 | `EcosystemNPM` | `"npm"` |
| yarn-fixture | yarn/yarn.go line 25 | `EcosystemNPM` | `"npm"` |
| bun-fixture | bun/bun.go line 27 | `EcosystemNPM` | `"npm"` |
| pypi-fixture | pypi/pypi.go | `EcosystemPyPI` | `"pypi"` |
| gomod-fixture | gomod/gomod.go line 28 | `EcosystemGo` | `"go"` |
| rubygems-fixture | rubygems/rubygems.go line 28 | `EcosystemRubyGems` | `"rubygems"` |
| composer-fixture | composer/composer.go line 28 | `EcosystemPackagist` | `"packagist"` |

The parity test asserts records for `"npm"`, `"pypi"`, `"go"`, `"rubygems"`, `"packagist"` — five distinct ecosystem strings cover all 8 package managers.

---

## Shared Patterns

### `package main` — same package for all new files

All files under `cmd/pollen/` are `package main`. `roots_windows.go`, `roots_windows_test.go`, and `parity_test.go` all use `package main` (not a separate package). This is why helpers like `buildCurrentPollen`, `runBinaryOnFixture`, `normalize`, and `globExisting` are all directly callable without import.

**Source:** All existing files in `cmd/pollen/` (roots.go line 27, differential_test.go line 1, normalize_diff.go line 1).
**Apply to:** All four new/modified Phase 2 files.

### `filterExistingRoots` — absent root handling

**Source:** `cmd/pollen/roots.go` lines 562–586
**Apply to:** `roots_windows.go` (indirectly — the caller in roots.go already calls it; Windows functions only return raw candidates)

The Windows functions return raw `[]scanner.Root` slices. `filterExistingRoots` is called by `baselineDefaultRoots` (roots.go line 340) after all candidates are assembled. Do NOT call `filterExistingRoots` from inside `roots_windows.go` — that would double-filter.

### `globExisting` — wildcard directory enumeration

**Source:** `cmd/pollen/roots.go` lines 309–321
**Apply to:** `roots_windows.go` — for `Python*` and `Ruby*` wildcard paths

`globExisting` is defined in `roots.go` (same package). Call it directly from `roots_windows.go`. The pattern for wildcard expansion:
```go
for _, p := range globExisting(filepath.Join(appdata, "Python", "Python*", "site-packages")) {
    add(p, model.RootKindUserPackage)
}
```

### `t.TempDir()` + `t.Setenv()` — test isolation

**Source:** `cmd/pollen/main_test.go` lines 57–58, 99–100, 125–126
**Apply to:** `roots_windows_test.go`

Go's `testing.T.TempDir()` creates a temp dir that is automatically cleaned up; `t.Setenv` restores the original env var after the test. Use both for Windows test isolation. Never use `os.Setenv` directly in tests.

### `runtime.Caller(0)` + `filepath.Dir` — fixture path location

**Source:** `cmd/pollen/differential_test.go` lines 61–65
**Apply to:** `parity_test.go`

Always locate test fixtures relative to the test source file path, not the current working directory. The test file path is stable across all three OS runners because the fixture is committed to the repo.

---

## No Analog Found

None. All four files have close analogs in the pollen codebase.

---

## Metadata

**Analog search scope:** `C:\Users\Bantu\mzansi-agentive\pollen\cmd\pollen\` and `internal/`
**Files read:** `roots.go`, `main_test.go`, `differential_test.go`, `normalize_diff.go`, `internal/model/model.go`, `internal/ecosystem/bun/bun.go`, `internal/ecosystem/pnpm/pnpm.go`, `internal/ecosystem/yarn/yarn.go`, `internal/ecosystem/gomod/gomod.go`, `internal/ecosystem/rubygems/rubygems.go`, `internal/ecosystem/composer/composer.go`, `testdata/diff-fixture/` tree
**Pattern extraction date:** 2026-06-02
