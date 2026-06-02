# Phase 3: Windows Path Representation - Pattern Map

**Mapped:** 2026-06-02
**Files analyzed:** 9 (3 source files modified, 5 existing test files EXTENDED; 1 optional build-tag pair)
**Analogs found:** 9 / 9

> **Correction (2026-06-02, post-plan-check):** An earlier draft of this map claimed
> `npm_test.go`, `pnpm_test.go`, and `endpoint_test.go` did not exist and had to be created from
> scratch. **That is wrong — all three exist.** Verified live test functions:
> - `npm_test.go`: `TestIsNodeModulesPackageJSONShapes`, `TestIsNodeModulesPackageJSONRelativeRoot`, `TestScanLockfileV3ScopedAndUnscoped`, … (extend, do NOT recreate)
> - `pnpm_test.go`: `TestIsPnpmStorePackageJSON`, `TestIsPnpmStorePackageJSONRelativeRoot`, … (extend)
> - `endpoint_test.go`: `TestCurrentPopulatesDeviceID`, `TestCurrentEmptyDeviceID` (extend — do NOT
>   redeclare these; adding a second `TestCurrentPopulatesDeviceID` is a duplicate-symbol compile error)
>
> All five test changes are **append a new test func to the existing file**, not create-from-scratch.

---

## File Classification

| New/Modified File | Repo | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|------|-----------|----------------|---------------|
| `internal/ecosystem/npm/npm.go` | pollen | utility (path helper) | transform | `internal/ecosystem/pnpm/pnpm.go` (symmetric defect) | exact |
| `internal/ecosystem/pnpm/pnpm.go` | pollen | utility (path helper) | transform | `internal/ecosystem/npm/npm.go` (symmetric defect) | exact |
| `internal/endpoint/endpoint.go` | pollen | utility (record builder) | request-response | `cmd/pollen/roots_windows.go` + `roots_notwindows.go` (Phase-2 build-tag precedent) | role-match |
| `internal/ecosystem/npm/npm_test.go` *(EXTEND existing)* | pollen | test | unit | self — existing `TestIsNodeModulesPackageJSONShapes` in same file | exact |
| `internal/ecosystem/pnpm/pnpm_test.go` *(EXTEND existing)* | pollen | test | unit | self — existing `TestIsPnpmStorePackageJSON` in same file | exact |
| `internal/endpoint/endpoint_test.go` *(EXTEND existing)* | pollen | test | unit | self — existing `TestCurrentPopulatesDeviceID` / `TestCurrentEmptyDeviceID` in same file | exact |
| `cmd/pollen/parity_test.go` | pollen | test (integration) | batch | `cmd/pollen/parity_test.go` (self — extend existing `assertEndpointOS`) | exact |
| `internal/scan/scanner_test.go` | beekeeper | test | unit | `internal/scan/scanner_test.go` (`TestScanWithBumblebee`) | exact |
| `internal/endpoint/endpoint_windows.go` *(optional — see discretion)* | pollen | config (build-tag override) | — | `cmd/pollen/roots_windows.go` | exact |
| `internal/endpoint/endpoint_notwindows.go` *(optional — see discretion)* | pollen | config (build-tag stub) | — | `cmd/pollen/roots_notwindows.go` | exact |

---

## Pattern Assignments

### `../pollen/internal/ecosystem/npm/npm.go` (utility, transform — WPATH-01)

**Change scope:** One line at line 114 inside `IsNodeModulesPackageJSON`. No structural changes.

**Analog:** `internal/ecosystem/pnpm/pnpm.go` line 87 (identical defect pattern, symmetric fix)

**Defect site** (`npm.go` lines 80–122):
```go
// Package: github.com/bantuson/pollen/internal/ecosystem/npm

// IsNodeModulesPackageJSON (line 80) — full function shown for context.
// The defect is the return of projectPath at line 114.
func IsNodeModulesPackageJSON(path string) (bool, string) {
    if filepath.Base(path) != "package.json" {
        return false, ""
    }
    parts := strings.Split(filepath.ToSlash(path), "/")   // line 84 — ToSlash for segment parsing (OK)
    if len(parts) < 3 {
        return false, ""
    }
    nmIdx := -1
    for i := len(parts) - 1; i >= 0; i-- {
        if parts[i] == "node_modules" {
            nmIdx = i
            break
        }
    }
    if nmIdx < 0 {
        return false, ""
    }
    tail := parts[nmIdx+1:]
    switch len(tail) {
    case 2:
        if strings.HasPrefix(tail[0], "@") {
            return false, ""
        }
    case 3:
        if !strings.HasPrefix(tail[0], "@") {
            return false, ""
        }
    default:
        return false, ""
    }
    projectPath := strings.Join(parts[:nmIdx], "/")   // ← LINE 114: DEFECT — leaks forward-slash on Windows
    if projectPath == "" {
        projectPath = "."
    }
    return true, projectPath
}
```

**Minimal diff fix** (single-line change):
```go
// Before (line 114):
projectPath := strings.Join(parts[:nmIdx], "/")

// After:
projectPath := filepath.FromSlash(strings.Join(parts[:nmIdx], "/"))
// filepath.FromSlash is a no-op on Linux/macOS (/ is native); PTEST-02 stays byte-identical.
// On Windows: converts "C:/Users/fana/code/web-app" → "C:\Users\fana\code\web-app".
```

**Import already present:** `"path/filepath"` is already imported at line 14. No new imports.

---

### `../pollen/internal/ecosystem/pnpm/pnpm.go` (utility, transform — WPATH-01)

**Change scope:** One line at line 87 inside `IsPnpmStorePackageJSON`. No structural changes.

**Analog:** `internal/ecosystem/npm/npm.go` line 114 (symmetric defect, same pattern)

**Defect site** (`pnpm.go` lines 39–94):
```go
// IsPnpmStorePackageJSON (line 39) — relevant excerpt.
func IsPnpmStorePackageJSON(path string) (ok bool, projectPath, name, version string) {
    if filepath.Base(path) != "package.json" {
        return false, "", "", ""
    }
    parts := strings.Split(filepath.ToSlash(path), "/")   // line 43 — ToSlash for segment parsing (OK)
    pnpmIdx := -1
    for i := len(parts) - 1; i >= 1; i-- {
        if parts[i] == ".pnpm" && parts[i-1] == "node_modules" {
            pnpmIdx = i
            break
        }
    }
    if pnpmIdx < 0 || pnpmIdx+4 >= len(parts) {
        return false, "", "", ""
    }
    // ... (name/version extraction from storeDir) ...
    projectPath = strings.Join(parts[:pnpmIdx-1], "/")   // ← LINE 87: DEFECT — leaks forward-slash on Windows
    if projectPath == "" {
        projectPath = "."
    }
    return true, projectPath, name, version
}
```

**Minimal diff fix** (single-line change):
```go
// Before (line 87):
projectPath = strings.Join(parts[:pnpmIdx-1], "/")

// After:
projectPath = filepath.FromSlash(strings.Join(parts[:pnpmIdx-1], "/"))
```

**Import already present:** `"path/filepath"` is already imported at line 17. No new imports.

---

### `../pollen/internal/endpoint/endpoint.go` (utility, request-response — WPATH-02)

**Change scope:** Two lines inside `Current()` (the `u.Uid` assignment and the `os.Getuid()` fallback). No structural changes; no new files required if using inline `runtime.GOOS` guard.

**Analog for inline guard approach:** The existing `endpoint.go` function body + RESEARCH.md Pattern 1 recommendation.

**Current function** (`endpoint.go` lines 18–34) — full file, it is only 34 lines:
```go
// Package endpoint collects host identity used in every record.
package endpoint

import (
    "os"
    "os/user"
    "runtime"
    "strconv"

    "github.com/bantuson/pollen/internal/model"
)

func Current(deviceID string) model.Endpoint {
    ep := model.Endpoint{
        OS:       runtime.GOOS,
        Arch:     runtime.GOARCH,
        DeviceID: deviceID,
    }
    if h, err := os.Hostname(); err == nil {
        ep.Hostname = h
    }
    if u, err := user.Current(); err == nil {
        ep.Username = u.Username
        ep.UID = u.Uid       // ← line 29: DEFECT — SID string on Windows, must be ""
    } else {
        ep.UID = strconv.Itoa(os.Getuid())   // ← line 31: DEFECT — returns "-1" on Windows, must be ""
    }
    return ep
}
```

**Minimal diff fix — inline `runtime.GOOS` guard (recommended, fewer files):**
```go
func Current(deviceID string) model.Endpoint {
    ep := model.Endpoint{
        OS:       runtime.GOOS,
        Arch:     runtime.GOARCH,
        DeviceID: deviceID,
    }
    if h, err := os.Hostname(); err == nil {
        ep.Hostname = h
    }
    if u, err := user.Current(); err == nil {
        ep.Username = u.Username
        if runtime.GOOS != "windows" {
            ep.UID = u.Uid
            // On Windows: u.Uid is a SID string (S-1-5-21-...). WPATH-02 requires
            // endpoint.uid to be empty on Windows. Leave ep.UID as zero value "".
        }
    } else if runtime.GOOS != "windows" {
        ep.UID = strconv.Itoa(os.Getuid())
        // On Windows: os.Getuid() returns -1. WPATH-02 requires empty uid. Skip.
    }
    return ep
}
```

**Import `runtime` is already present** at line 7. No new imports.

**IMPORTANT — Pitfall 5 guard:** Both the happy path (`u.Uid`) AND the error fallback (`os.Getuid()`) must be guarded with `runtime.GOOS != "windows"`. Guarding only one leaves the other leaking a non-empty uid on Windows when `user.Current()` fails.

**Alternative — build-tagged files (mirrors Phase 2 pattern exactly):**
See "Shared Patterns: Build-Tag Stub Pattern" below. Use if planner prefers file-pair consistency over minimal-file count.

---

### `../pollen/internal/ecosystem/npm/npm_test.go` (test — **EXTEND existing file**, WPATH-01)

**EXISTS** with `TestIsNodeModulesPackageJSONShapes`, `TestIsNodeModulesPackageJSONRelativeRoot`,
`TestScanLockfileV3ScopedAndUnscoped`, etc. **APPEND** the new Windows test func(s) below; do NOT
recreate the file and do NOT redeclare existing funcs.

**Analog:** self — the existing `TestIsNodeModulesPackageJSONShapes` in the same file (calls the
helper directly and asserts the returned `projectPath`).

**Imports:** the file already imports `testing`. The Windows test below also needs `runtime` — add
it to the existing import block if not present.

**Core test pattern — LEAD with the `runtime.GOOS` skip, then use a raw Windows string literal**
(this is the recommended, portable form — `filepath.Join("C:\\", ...)` produces an invalid
`C:\\/Users/...` path on Unix and must NOT be used):
```go
func TestIsNodeModulesPackageJSONWindowsPath(t *testing.T) {
    if runtime.GOOS != "windows" {
        t.Skip("WPATH-01: Windows path-shape test — meaningful only on Windows (/ is native here)")
    }
    // Raw Windows string literal — backslash separators + drive letter.
    base := `C:\Users\fana\code\web-app\node_modules\left-pad\package.json`

    ok, projectPath := IsNodeModulesPackageJSON(base)
    if !ok {
        t.Fatalf("IsNodeModulesPackageJSON(%q): got ok=false, want true", base)
    }
    want := `C:\Users\fana\code\web-app`
    if projectPath != want {
        t.Errorf("IsNodeModulesPackageJSON projectPath = %q, want %q", projectPath, want)
    }
}

func TestIsNodeModulesPackageJSONScopedWindowsPath(t *testing.T) {
    if runtime.GOOS != "windows" {
        t.Skip("WPATH-01: Windows path-shape test — meaningful only on Windows")
    }
    base := `C:\Users\fana\code\web-app\node_modules\@scope\pkg\package.json`

    ok, projectPath := IsNodeModulesPackageJSON(base)
    if !ok {
        t.Fatalf("IsNodeModulesPackageJSON(%q): got ok=false, want true", base)
    }
    want := `C:\Users\fana\code\web-app`
    if projectPath != want {
        t.Errorf("IsNodeModulesPackageJSON projectPath = %q, want %q", projectPath, want)
    }
}
```

**Do NOT use** `filepath.Join("C:\\", ...)` — on Linux/macOS it yields `C:\\/Users/...` (invalid
Unix path) and the assertion becomes meaningless. The `runtime.GOOS != "windows"` skip + raw string
literal mirrors the Phase-2 structured-skip precedent (`differential_test.go` line 55) and is what
Plan 03-01 specifies.

---

### `../pollen/internal/ecosystem/pnpm/pnpm_test.go` (test — **EXTEND existing file**, WPATH-01)

**EXISTS** with `TestIsPnpmStorePackageJSON`, `TestIsPnpmStorePackageJSONRelativeRoot`, etc.
**APPEND** the new Windows test func below; do NOT recreate the file.

**Analog:** self — existing `TestIsPnpmStorePackageJSON` in the same file (sibling of the npm test).

**Symmetric to npm_test.go.** The pnpm store path layout differs from npm node_modules layout:

```go
package pnpm

import (
    "path/filepath"
    "runtime"
    "testing"
)

func TestIsPnpmStorePackageJSONWindowsPath(t *testing.T) {
    if runtime.GOOS != "windows" {
        t.Skip("WPATH-01: Windows path shape test — meaningful only on Windows")
    }
    // pnpm store layout: <project>/node_modules/.pnpm/<name>@<ver>/node_modules/<name>/package.json
    base := `C:\Users\fana\code\web-app\node_modules\.pnpm\left-pad@1.3.0\node_modules\left-pad\package.json`

    ok, projectPath, name, version := IsPnpmStorePackageJSON(base)
    if !ok {
        t.Fatalf("IsPnpmStorePackageJSON(%q): got ok=false, want true", base)
    }
    wantProject := `C:\Users\fana\code\web-app`
    if projectPath != wantProject {
        t.Errorf("projectPath = %q, want %q", projectPath, wantProject)
    }
    if name != "left-pad" {
        t.Errorf("name = %q, want %q", name, "left-pad")
    }
    if version != "1.3.0" {
        t.Errorf("version = %q, want %q", version, "1.3.0")
    }
}
```

---

### `../pollen/internal/endpoint/endpoint_test.go` (test — **EXTEND existing file**, WPATH-02)

**EXISTS** with `TestCurrentPopulatesDeviceID` and `TestCurrentEmptyDeviceID`. **APPEND** the single
new func `TestCurrentWindowsUID` below. **CRITICAL: do NOT redeclare `TestCurrentPopulatesDeviceID`** —
it already exists; a second declaration is a duplicate-symbol compile error. The existing
`TestCurrentPopulatesDeviceID` already covers OS/Arch/DeviceID/Username population (the D-04
regression guard), so the new file only needs the uid-specific test.

**Analog:** self — existing `TestCurrentPopulatesDeviceID` in the same file (calls `Current()` and
asserts fields). The file already imports `runtime` and `testing`.

**New test to ADD — call Current(), assert uid empty on Windows, non-empty on Unix:**
```go
func TestCurrentWindowsUID(t *testing.T) {
    ep := Current("")
    if runtime.GOOS == "windows" {
        // WPATH-02: on Windows, user.Current().Uid is a SID string; endpoint.uid must be empty.
        if ep.UID != "" {
            t.Errorf("endpoint.uid on Windows = %q, want empty string (WPATH-02)", ep.UID)
        }
    } else {
        // D-04 regression guard: Unix UID must remain a non-empty numeric string.
        if ep.UID == "" {
            t.Errorf("endpoint.uid on %s = empty, want non-empty numeric UID (regression)", runtime.GOOS)
        }
    }
    // os/arch/username are already asserted by the existing TestCurrentPopulatesDeviceID — no need
    // to duplicate that here.
}
```

---

### `../pollen/cmd/pollen/parity_test.go` (test — extend existing file, WPATH-01+02)

**Analog:** Self — existing `assertEndpointOS` function (lines 89–112) is the direct template.

**File location:** `C:\Users\Bantu\mzansi-agentive\pollen\cmd\pollen\parity_test.go`

**Existing `assertEndpointOS` pattern** to mirror for new helpers (lines 89–112):
```go
func assertEndpointOS(t *testing.T, ndjson []byte, want string) {
    t.Helper()
    lines := strings.Split(strings.TrimSpace(string(ndjson)), "\n")
    for i, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" {
            continue
        }
        var rec map[string]any
        if err := json.Unmarshal([]byte(line), &rec); err != nil {
            t.Errorf("assertEndpointOS: line %d: not valid JSON: %v", i+1, err)
            continue
        }
        ep, ok := rec["endpoint"].(map[string]any)
        if !ok {
            continue  // no endpoint sub-object — skip
        }
        got, _ := ep["os"].(string)
        if got != want {
            t.Errorf("assertEndpointOS: line %d: endpoint.os = %q, want %q", i+1, got, want)
        }
    }
}
```

**`TestParityAllEcosystems` call site** to extend (lines 37–83) — add a Windows-only block after line 64:
```go
// Existing call (line 64):
assertEndpointOS(t, out, runtime.GOOS)

// ADD after the existing assertEndpointOS call (before normalize()):
if runtime.GOOS == "windows" {
    assertWindowsPathShape(t, out)
    assertWindowsEndpointUID(t, out)
}
```

**New helpers to add** (see RESEARCH.md Code Examples for full bodies — copied here for completeness):
```go
// assertWindowsPathShape asserts that every non-empty project_path / source_file
// field contains a drive letter (e.g. "C:") and no forward-slash separators.
// Only called on Windows. Mirrors assertEndpointOS pattern: iterate lines,
// unmarshal, check fields.
func assertWindowsPathShape(t *testing.T, ndjson []byte) {
    t.Helper()
    lines := strings.Split(strings.TrimSpace(string(ndjson)), "\n")
    for i, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" {
            continue
        }
        var rec map[string]any
        if err := json.Unmarshal([]byte(line), &rec); err != nil {
            continue
        }
        for _, field := range []string{"project_path", "source_file"} {
            if v, ok := rec[field].(string); ok && v != "" && v != "." {
                if strings.Contains(v, "/") {
                    t.Errorf("assertWindowsPathShape: line %d field %q = %q contains forward slash", i+1, field, v)
                }
                if len(v) < 2 || v[1] != ':' {
                    t.Errorf("assertWindowsPathShape: line %d field %q = %q missing drive letter", i+1, field, v)
                }
            }
        }
    }
}

// assertWindowsEndpointUID asserts every record with an endpoint sub-object
// has an empty uid field. Only called on Windows. Mirrors assertEndpointOS pattern.
func assertWindowsEndpointUID(t *testing.T, ndjson []byte) {
    t.Helper()
    lines := strings.Split(strings.TrimSpace(string(ndjson)), "\n")
    for i, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" {
            continue
        }
        var rec map[string]any
        if err := json.Unmarshal([]byte(line), &rec); err != nil {
            continue
        }
        ep, ok := rec["endpoint"].(map[string]any)
        if !ok {
            continue
        }
        if uid, exists := ep["uid"]; exists {
            if uidStr, _ := uid.(string); uidStr != "" {
                t.Errorf("assertWindowsEndpointUID: line %d: endpoint.uid = %q, want empty on Windows", i+1, uidStr)
            }
        }
    }
}
```

**Existing imports in parity_test.go** (lines 25–32) already include all needed packages:
```go
import (
    "encoding/json"
    "os"
    "path/filepath"
    "runtime"
    "strings"
    "testing"
)
```

No new imports required. The new helpers use only `encoding/json`, `strings`, and `testing` — all present.

---

### `beekeeper/internal/scan/scanner_test.go` (test — extend existing, WPATH-02 D-03)

**Analog:** Self — `TestScanWithBumblebee` (lines 17–44) is the direct template.

**Existing `runBumblebeeFn` injection pattern** (lines 17–44):
```go
func TestScanWithBumblebee(t *testing.T) {
    old := runBumblebeeFn
    defer func() { runBumblebeeFn = old }()

    line1 := `{"record_type":"package","name":"test-package"}`
    line2 := `{"record_type":"finding","severity":"high"}`
    runBumblebeeFn = func(_ context.Context, _ bool) (<-chan []byte, bool) {
        ch := make(chan []byte, 2)
        ch <- []byte(line1)
        ch <- []byte(line2)
        close(ch)
        return ch, true
    }

    var buf bytes.Buffer
    cfg := Config{}
    if err := Scan(context.Background(), cfg, &buf); err != nil {
        t.Fatalf("Scan: %v", err)
    }

    out := buf.String()
    if !strings.Contains(out, `"record_type":"package"`) {
        t.Errorf("want record_type:package in output; got:\n%s", out)
    }
}
```

**New test to add** — `TestScanWindowsShapedRecord` using identical injection pattern:
```go
func TestScanWindowsShapedRecord(t *testing.T) {
    old := runBumblebeeFn
    defer func() { runBumblebeeFn = old }()

    // Windows-shaped Pollen NDJSON record:
    //   - project_path and source_file use backslash separators + drive letter
    //   - endpoint.os = "windows", endpoint.uid = "" (WPATH-02)
    // JSON-encoded backslashes: C:\Users\fana → C:\\Users\\fana in raw string literal.
    windowsRecord := `{"record_type":"package","record_id":"package:abc123",` +
        `"schema_version":"0.1.0","scanner_name":"pollen",` +
        `"endpoint":{"hostname":"WIN-BOX","os":"windows","arch":"amd64",` +
        `"username":"fana","uid":""},"ecosystem":"npm",` +
        `"normalized_name":"left-pad","version":"1.3.0",` +
        `"project_path":"C:\\Users\\fana\\code\\web-app",` +
        `"source_type":"npm-lockfile",` +
        `"source_file":"C:\\Users\\fana\\code\\web-app\\package-lock.json",` +
        `"confidence":"high","has_lifecycle_scripts":false}`

    runBumblebeeFn = func(_ context.Context, _ bool) (<-chan []byte, bool) {
        ch := make(chan []byte, 1)
        ch <- []byte(windowsRecord)
        close(ch)
        return ch, true
    }

    var buf bytes.Buffer
    if err := Scan(context.Background(), Config{}, &buf); err != nil {
        t.Fatalf("Scan: %v", err)
    }

    out := buf.String()
    // Assert: record passed through — NOT rewritten as scan_error.
    if strings.Contains(out, `"record_type":"scan_error"`) {
        t.Errorf("Windows-shaped record rejected as malformed: %s", out)
    }
    if !strings.Contains(out, `"os":"windows"`) {
        t.Errorf("endpoint.os=windows not preserved in passthrough: %s", out)
    }
    if !strings.Contains(out, `"uid":""`) {
        t.Errorf("empty uid not preserved in passthrough: %s", out)
    }
    // Backslash paths survive JSON round-trip (JSON doubles them: C:\ → C:\\).
    if !strings.Contains(out, `C:\\`) {
        t.Errorf("Windows drive+backslash path not preserved in passthrough: %s", out)
    }
}
```

**Existing imports in scanner_test.go** (lines 3–15) already include `bytes`, `context`, `strings`, `testing`. No new imports needed.

---

## Shared Patterns

### Build-Tag Stub Pattern (Phase 2 precedent — `roots_windows.go` / `roots_notwindows.go`)

**Source:** `../pollen/cmd/pollen/roots_windows.go` (lines 1–2, 21) + `roots_notwindows.go` (lines 1–2, 11–13)
**Apply to:** `endpoint_windows.go` / `endpoint_notwindows.go` IF the planner chooses the build-tag approach over the inline `runtime.GOOS` guard.

```go
// roots_windows.go header pattern:
//go:build windows

package main
// ... Windows-specific implementation ...
```

```go
// roots_notwindows.go stub pattern:
//go:build !windows

package main

import "github.com/bantuson/pollen/internal/scanner"

// windowsBaselinePackageRoots is a no-op stub for non-Windows builds.
// The real implementation lives in roots_windows.go (//go:build windows).
func windowsBaselinePackageRoots() []scanner.Root { return nil }
func windowsSystemRoots() []scanner.Root { return nil }
```

**If using build-tagged approach for endpoint.go**, create:

`internal/endpoint/endpoint_windows.go`:
```go
//go:build windows

package endpoint

import "github.com/bantuson/pollen/internal/model"

// clearUID zeroes the UID field on Windows. On Windows, user.Current().Uid
// returns a SID string (e.g. "S-1-5-21-...") rather than a Unix numeric UID.
// WPATH-02 requires endpoint.uid to be empty on Windows.
func clearUID(ep *model.Endpoint) {
    ep.UID = ""
}
```

`internal/endpoint/endpoint_notwindows.go`:
```go
//go:build !windows

package endpoint

import "github.com/bantuson/pollen/internal/model"

// clearUID is a no-op on Unix — UID is already the correct numeric string.
func clearUID(ep *model.Endpoint) {}
```

Then in `endpoint.go`, call `clearUID(&ep)` after the `if u, err := user.Current()` block.

**Planner note:** RESEARCH.md recommends the inline `runtime.GOOS` guard for this single-field override (fewer files). The build-tagged approach is available if consistency with Phase 2 file-pair pattern is preferred.

### Structured Skip Pattern (differential_test.go precedent)

**Source:** `../pollen/cmd/pollen/differential_test.go` lines 55–57
**Apply to:** Any test that is Windows-only or explicitly cannot run on non-Windows.

```go
if runtime.GOOS == "windows" {
    t.Skip("PTEST-02 differential runs on Linux+macOS only; Windows behavior arrives Phase 2 (v0.1.1-pollen.2)")
}
```

Invert the condition for Windows-only tests:
```go
if runtime.GOOS != "windows" {
    t.Skip("WPATH-01: Windows path shape test — meaningful only on Windows (forward slash is native on this OS)")
}
```

### JSON Passthrough Validity Check (scanner.go consumer pattern)

**Source:** `beekeeper/internal/scan/scanner.go` lines 116–127
**Apply to:** The round-trip test assertion logic — confirms beekeeper's NDJSON ingestion path does NOT parse endpoint as a struct (it is `json.RawMessage` validation only).

```go
// From scanner.go lines 116–127 — this is what Windows-shaped records pass through:
var probe json.RawMessage
if err := json.Unmarshal(line, &probe); err != nil {
    warn := map[string]any{
        "record_type":  "scan_error",
        "scanner_name": "beekeeper",
        "source":       "bumblebee",
        "error":        "malformed NDJSON from bumblebee subprocess",
    }
    _ = writeJSONLine(out, warn)
    continue
}
// Pass through unknown record_types unmodified.
_, _ = fmt.Fprintf(out, "%s\n", line)
```

Key insight: `json.RawMessage` validation accepts any valid JSON. Backslash path strings (`"C:\\Users\\"`) are valid JSON. Empty string `""` is valid JSON. A Windows-shaped record with these values will ALWAYS pass the validity check and be passed through unchanged. The round-trip test therefore asserts absence of `scan_error` rather than structural correctness.

---

## No Analog Found

All files have close analogs. No new dependencies, no new frameworks, no uncharted territory.

---

## Implementation Order Recommendation

For the planner, the natural wave ordering is:

**Wave 1 — Production code fixes (Pollen):**
1. `internal/ecosystem/npm/npm.go` line 114 — single-line `filepath.FromSlash` wrap
2. `internal/ecosystem/pnpm/pnpm.go` line 87 — single-line `filepath.FromSlash` wrap
3. `internal/endpoint/endpoint.go` lines 29, 31 — `runtime.GOOS != "windows"` guard on both UID assignments

**Wave 2 — Unit tests (Pollen) — all EXTEND existing files (append funcs):**
4. `internal/ecosystem/npm/npm_test.go` — extend: add `TestIsNodeModulesPackageJSONWindowsPath`
5. `internal/ecosystem/pnpm/pnpm_test.go` — extend: add `TestIsPnpmStorePackageJSONWindowsPath`
6. `internal/endpoint/endpoint_test.go` — extend: add `TestCurrentWindowsUID` (do NOT redeclare existing `TestCurrentPopulatesDeviceID`)

**Wave 3 — Integration tests (Pollen + Beekeeper):**
7. `cmd/pollen/parity_test.go` — extend with `assertWindowsPathShape` + `assertWindowsEndpointUID` + call in `TestParityAllEcosystems`
8. `beekeeper/internal/scan/scanner_test.go` — add `TestScanWindowsShapedRecord`

**Wave 4 — Release prep:**
9. `CHANGES.md`, `VERSION` in Pollen — record WPATH deltas, bump version (defer signed tag per D-06)

---

## Metadata

**Analog search scope:** `../pollen/internal/ecosystem/`, `../pollen/internal/endpoint/`, `../pollen/cmd/pollen/`, `beekeeper/internal/scan/`
**Files read:** 12 (npm.go, pnpm.go, endpoint.go, roots_windows.go, roots_notwindows.go, parity_test.go, differential_test.go, scanner.go, scanner_test.go, model.go, 03-CONTEXT.md, 03-RESEARCH.md)
**Pattern extraction date:** 2026-06-02
