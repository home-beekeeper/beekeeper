# Phase 3: Windows Path Representation - Research

**Researched:** 2026-06-02
**Domain:** Go cross-platform path handling — Pollen NDJSON emitter (emitter side) + Beekeeper NDJSON consumer (consumer side)
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**D-01 — Native Windows path preservation (WPATH-01)**
- `project_path` and `source_file` fields MUST contain backslash separators and drive letters on Windows.
- No `/c/`-style or forward-slash artifacts anywhere in the output path.
- Implementation locus: `internal/output/paths_windows.go` in Pollen (verify against live code — may differ).

**D-02 — Windows endpoint record (WPATH-02)**
- `endpoint.os` == `"windows"` (already `runtime.GOOS`).
- `endpoint.arch` matches `runtime.GOARCH` (already `runtime.GOARCH`).
- `endpoint.username` non-empty from the Windows environment (already `user.Current().Username`).
- `endpoint.uid` is **EMPTY on Windows** (currently returns a SID string — this is the net-new requirement).
- Linux/macOS endpoint records MUST be unchanged (UID still populated on Unix).

**D-03 — Beekeeper consumer round-trip (WPATH-02, beekeeper side)**
- Beekeeper parses a Windows-shaped Pollen NDJSON record (backslash paths, empty `uid`, `os="windows"`) without error and round-trips endpoint fields correctly.
- Whether to create `internal/inventory/` or co-locate with `internal/scan/` is Claude's discretion.

**D-04 — No regression on Unix (differential test stays green)**
- Windows additions MUST NOT drift Pollen's behavior on Linux/macOS.
- `TestDifferential` (PTEST-02) MUST continue to pass byte-for-byte.
- Windows-specific behavior goes behind `//go:build windows` or `runtime.GOOS == "windows"` guard.

**D-05 — Cross-platform parity test extends to path/endpoint assertions**
- `TestParityAllEcosystems` (PTEST-01) gains assertions for Windows path shape + empty uid.

**D-06 — Release tagging deferred to M2 close**
- `v0.1.1-pollen.3` signed/tagged release is batched to M2 close, matching Phase 2 precedent.
- Plans treat SC4 as "prepare locally; defer signed tag."

### Claude's Discretion

- Exact mechanism for Windows path preservation: `paths_windows.go` build-tagged file vs. `runtime.GOOS` branch in `output.go`.
- Exact mechanism for empty Windows `uid`: `endpoint_windows.go` build-tagged override vs. `if runtime.GOOS == "windows"` guard.
- Whether beekeeper round-trip test creates `internal/inventory/` or co-locates with `internal/scan/`.
- Whether to add a Windows fixture NDJSON or hand-craft inline in test.

### Deferred Ideas (OUT OF SCOPE)

- Signed/tagged `v0.1.1-pollen.3` release (batched to M2 close).
- BKINT-01 (beekeeper subprocess swap `bumblebee` → `pollen`) — Phase 4.
- Editor/browser/MCP Windows path coverage (WEXT-01..03) — Phase 4.
- Windows honeypot E2E + full beekeeper Windows CI green (PTEST-05, BKINT-02) — Phase 5.

</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| WPATH-01 | `internal/output/paths_windows.go` preserves native Windows paths in NDJSON `project_path`/`source_file` — backslash separators and drive letters retained, no Unix-to-Windows conversion artifacts | Defect sources identified: 4 ecosystem scanners produce slash-joined `project_path` via `filepath.ToSlash` + string join. Fix is in ecosystem path-recognition helpers, not in `internal/output/`. |
| WPATH-02 | `endpoint` record emits `os="windows"`, `arch` from `runtime.GOARCH`, `username` from Windows env, and empty `uid`; beekeeper audit-log consumer handles Windows-shaped endpoint records (round-trip verified) | `endpoint.Current()` confirmed to return SID on Windows via `user.Current().Uid`. Empty-uid override is a build-tagged `endpoint_windows.go` file. Beekeeper consumer is pure JSON passthrough — round-trip is a unit test of JSON unmarshal into `map[string]any`. |

</phase_requirements>

---

## Summary

Phase 3 is a targeted two-repo correctness fix with no schema changes. The Pollen NDJSON emitter has two classes of defect on Windows:

**WPATH-01 (path fields):** Four ecosystem-scanner helper functions internally apply `filepath.ToSlash` to split path strings for parsing, then **re-join using forward slashes**. This leaks into the `project_path` field in NDJSON output on Windows. Specifically: `npm.IsNodeModulesPackageJSON` (npm.go:114) returns `projectPath` as a forward-slash string; `pnpm.IsPnpmStorePackageJSON` (pnpm.go:87) does the same. The `source_file` field is clean in both cases (uses the raw `path` argument from the walker, which is OS-native). The `editorext` and `browserext` `filepath.ToSlash` calls are internal to classification predicates only and do NOT flow into `r.SourceFile` or `r.ProjectPath` on emitted records — they are safe as-is. The `classifyRoot` function in `roots.go` uses `filepath.ToSlash` for pattern matching only; it does not affect emitted records. The fix is narrow: replace the slash-join return values in the two affected helpers with `filepath.FromSlash` reconstruction or an alternative that keeps the OS-native path.

**WPATH-02 (endpoint uid):** `endpoint.Current()` in `internal/endpoint/endpoint.go` calls `user.Current().Uid` unconditionally. On Windows `Uid` is the user's SID string (`S-1-5-21-...`), not a Unix numeric UID and not empty. All other fields (`OS`, `Arch`, `Username`, `Hostname`) already produce correct Windows values. A build-tagged `internal/endpoint/endpoint_windows.go` file that overrides `uid` to `""` is the minimal-diff fix, mirroring the Phase-2 `roots_windows.go` / `roots_notwindows.go` stub pattern.

**Beekeeper consumer (D-03):** The existing `internal/scan/scanner.go` `runBumblebeeFn` passes all NDJSON lines through as raw bytes after a `json.Unmarshal` validity check. There is no struct-typed endpoint parsing in the consumer path — it is a pure JSON passthrough. Beekeeper's `audit.AuditRecord.Endpoint` is a `string` field (value `"check"`), not a `model.Endpoint` struct. Therefore a Windows-shaped record (backslash paths, empty uid) passes the validity check unchanged. The round-trip test is a unit test that constructs a synthetic Windows-shaped JSON record, feeds it through the same validity check, and asserts the fields survive intact. Co-locating this test with `internal/scan/` (rather than creating a new `internal/inventory/`) avoids prematurely building the Phase-4 BKINT-01 boundary.

**Primary recommendation:** Fix path leakage at the two specific helper return-value sites in `npm.go` and `pnpm.go` using `filepath.FromSlash`; add a build-tagged `endpoint_windows.go` for empty uid; add a unit test in `internal/scan/` for the Windows round-trip.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Native path emission in NDJSON | Pollen emitter (`internal/ecosystem/npm`, `internal/ecosystem/pnpm`) | — | Path values originate in ecosystem scanner helpers; the output package is a thin JSON encoder that encodes whatever `model.Record` fields it receives |
| Endpoint UID suppression on Windows | Pollen emitter (`internal/endpoint/`) | — | `Current()` is the single construction point; all callers receive the returned struct |
| Consumer round-trip correctness | Beekeeper consumer (`internal/scan/`) | — | `runBumblebeeFn` is the NDJSON ingestion path; no downstream struct parsing of endpoint |
| Cross-platform parity assertion | Pollen test harness (`cmd/pollen/parity_test.go`) | — | PTEST-01 already runs on all three OSes; Phase 3 extends its assertions |
| Differential test guard | Pollen test harness (`cmd/pollen/differential_test.go`) | — | PTEST-02 guards Linux/macOS byte-for-byte parity; Windows changes must not drift Unix bytes |

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `path/filepath` (stdlib) | Go 1.25 | `filepath.FromSlash`, `filepath.Clean`, OS-native joins | Correct cross-platform path handling; all existing code uses it |
| `os/user` (stdlib) | Go 1.25 | `user.Current()` — source of `Uid` field | Already used in `endpoint.go`; no new dependency |
| `runtime` (stdlib) | Go 1.25 | `runtime.GOOS`, `runtime.GOARCH` | Already used throughout |

No new external dependencies. This phase is pure stdlib + in-tree logic.

**Version verification:** No npm/external packages involved. All Go stdlib. [VERIFIED: go.mod — `go 1.25`, no external deps in pollen]

---

## Architecture Patterns

### System Architecture Diagram

```
pollen scan (Windows)
       |
       v
  walk.Walk()  — filepath.WalkDir returns OS-native backslash paths
       |
       v
  scanner.Run() dispatch
   ├── npm.IsNodeModulesPackageJSON(path)
   │      filepath.ToSlash(path) → split → rejoin with "/" → LEAK into projectPath
   │      [FIX: use filepath.FromSlash to convert back]
   │                                     ↓
   │                              r.ProjectPath = projectPath  ← NDJSON output
   │                              r.SourceFile  = path        ← clean (raw arg)
   │
   ├── pnpm.IsPnpmStorePackageJSON(path)
   │      filepath.ToSlash(path) → split → rejoin with "/" → LEAK into projectPath
   │      [FIX: same as npm]
   │
   ├── editorext.IsExtensionPackageJSON(path)
   │      filepath.ToSlash(parent) — for suffix matching ONLY; r.SourceFile = path (clean)
   │      [NO FIX NEEDED — ToSlash output never assigned to emitted fields]
   │
   └── browserext.IsFirefoxExtensionsJSON(path)
          filepath.ToSlash(dir) — for Contains matching ONLY
          [NO FIX NEEDED — same reason]

  endpoint.Current(deviceID)
   └── user.Current().Uid  ← SID on Windows (e.g. "S-1-5-21-...")
       [FIX: endpoint_windows.go overrides to empty string]
                          ↓
                   model.Endpoint{UID: ""}  ← NDJSON output

output.Emitter.Emit(r)
   └── json.Encoder.Encode(r)  — marshals r.ProjectPath, r.SourceFile, r.Endpoint as-is
                                  NO path transformation here
```

```
beekeeper internal/scan (Windows-shaped record consumer)
       |
       v
  runBumblebeeFn → chan []byte  (raw NDJSON lines)
       |
       v
  json.Unmarshal(line, &probe)  — validates JSON syntax only
       |
       v
  fmt.Fprintf(out, "%s\n", line)  — passthrough, no struct parsing
       |
  [round-trip test: feed Windows-shaped fixture → assert valid JSON + field values preserved]
```

### Recommended Project Structure

Pollen additions:
```
internal/endpoint/
  endpoint.go           # unchanged
  endpoint_windows.go   # //go:build windows — overrides UID to ""
  endpoint_notwindows.go  # //go:build !windows — stub (if needed for compile)
  endpoint_test.go      # extend with Windows uid assertion
internal/ecosystem/npm/
  npm.go                # fix IsNodeModulesPackageJSON projectPath join
  npm_test.go           # add Windows path round-trip test
internal/ecosystem/pnpm/
  pnpm.go               # fix IsPnpmStorePackageJSON projectPath join
  pnpm_test.go          # add Windows path round-trip test
cmd/pollen/
  parity_test.go        # extend assertEndpointOS + add assertWindowsPathShape (Windows only)
CHANGES.md              # record WPATH deltas
VERSION                 # bump to 0.1.1-pollen.3
```

Beekeeper additions:
```
internal/scan/
  scanner_test.go       # add TestScanWindowsShapedRecord
```

### Pattern 1: Build-tagged Windows override (from Phase 2 precedent)

**What:** A `_windows.go` file with `//go:build windows` contains the Windows-specific implementation. A `_notwindows.go` file with `//go:build !windows` contains a no-op stub so Go compiles cleanly on all three OSes.

**When to use:** When a Windows-specific value must be returned from a function that also runs on Unix with different behavior. Used for `windowsBaselinePackageRoots` / `windowsSystemRoots` in Phase 2.

**Example:**
```go
// internal/endpoint/endpoint_windows.go
//go:build windows

package endpoint

// clearUID zeroes the UID field on Windows. On Windows, user.Current().Uid
// returns a SID string (e.g. "S-1-5-21-...") rather than a Unix numeric UID.
// WPATH-02 requires endpoint.uid to be empty on Windows.
func clearUID(ep *model.Endpoint) {
    ep.UID = ""
}
```

```go
// internal/endpoint/endpoint_notwindows.go
//go:build !windows

package endpoint

// clearUID is a no-op on Unix — UID is already the correct numeric string.
func clearUID(ep *model.Endpoint) {}
```

```go
// internal/endpoint/endpoint.go — call site
func Current(deviceID string) model.Endpoint {
    ep := model.Endpoint{...}
    if u, err := user.Current(); err == nil {
        ep.Username = u.Username
        ep.UID = u.Uid
    } else {
        ep.UID = strconv.Itoa(os.Getuid())
    }
    clearUID(&ep) // no-op on Unix; zeroes SID on Windows
    return ep
}
```

Alternatively, a `runtime.GOOS == "windows"` guard inline in `endpoint.go` removes the need for stub files entirely:
```go
    if u, err := user.Current(); err == nil {
        ep.Username = u.Username
        if runtime.GOOS != "windows" {
            ep.UID = u.Uid
        }
    } else if runtime.GOOS != "windows" {
        ep.UID = strconv.Itoa(os.Getuid())
    }
```
Both approaches produce identical observable behavior. The `runtime.GOOS` branch is fewer files; the build-tag approach mirrors Phase 2 exactly. Planner's discretion.

### Pattern 2: filepath.FromSlash reconstruction for path helpers

**What:** The `filepath.ToSlash` + split + forward-slash-join pattern in `npm.IsNodeModulesPackageJSON` and `pnpm.IsPnpmStorePackageJSON` converts OS paths to a uniform representation for segment parsing, then leaks the forward-slash representation into the return value. The fix reconstructs an OS-native path by applying `filepath.FromSlash` to the joined result.

**When to use:** Whenever a path has been converted to slash form for internal parsing but the return value flows into a `model.Record` field that appears in NDJSON output.

**Example (npm.go IsNodeModulesPackageJSON):**
```go
// Before (leaks forward-slash project_path on Windows):
projectPath := strings.Join(parts[:nmIdx], "/")

// After (restores OS-native separators):
projectPath := filepath.FromSlash(strings.Join(parts[:nmIdx], "/"))
```

**Example (pnpm.go IsPnpmStorePackageJSON):**
```go
// Before:
projectPath = strings.Join(parts[:pnpmIdx-1], "/")

// After:
projectPath = filepath.FromSlash(strings.Join(parts[:pnpmIdx-1], "/"))
```

`filepath.FromSlash` is a no-op on Linux/macOS (forward slash is already the native separator), so Unix output is byte-identical after this change — PTEST-02 stays green.

### Anti-Patterns to Avoid

- **Touching `internal/output/output.go`:** The NDJSON emitter is a thin `json.Encode` wrapper. Path values enter it already formed via `model.Record` fields. There is no path normalization in the output layer. Adding path transformation here would be the wrong layer and would require the planner to fight existing abstractions.
- **PRD's `internal/output/paths_windows.go` locus (D-01 NOTE):** The PRD specifies `internal/output/paths_windows.go` as the locus, but the live code shows path values are set by ecosystem-scanner helpers, not by the output package. Creating a file there with no hook in the call chain would be dead code. The correct fix locus is the ecosystem helpers that set `r.ProjectPath`.
- **`filepath.ToSlash` as a normalization export:** Do not add a global path-normalization step that converts all paths to forward slash before emitting. This would break the Windows requirement (backslashes must be preserved in output) while appearing to "fix" tests that run with hardcoded forward-slash assertions.
- **Touching `normalize_diff.go`:** This file is LOCKED (its name `normalize` and behavior are frozen by the `TestDifferential` LOCK comment). Do not modify it.
- **Creating `internal/inventory/` prematurely:** The roadmap names this as Phase 4's BKINT-01 locus (subprocess swap boundary). Creating it now for a round-trip test would either duplicate or prematurely commit to the Phase-4 interface. Co-locate the round-trip test with `internal/scan/`.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| OS-native path reconstruction from slash-split | Custom path re-assembly logic | `filepath.FromSlash` (stdlib) | Handles all edge cases including drive letters (`C:/...` → `C:\...`) correctly |
| Build-tag stub files | Complex runtime detection | `//go:build windows` + `//go:build !windows` file pairs | Already established pattern in Phase 2; Go toolchain handles compilation correctly |
| Detecting whether path is Windows-shaped in tests | Custom regexp | Construct paths with `filepath.Join` and `t.Setenv` | Tests must use `t.Setenv("USERPROFILE", ...)` not HOME (Phase 2 Pitfall 5 prevention) |

---

## WPATH-01 Defect Trace

### Confirmed defect sites in Pollen (VERIFIED: source code read)

| File | Line | Code | Defect | Scope |
|------|------|------|--------|-------|
| `internal/ecosystem/npm/npm.go` | 84, 114 | `parts := strings.Split(filepath.ToSlash(path), "/")` → `projectPath = strings.Join(parts[:nmIdx], "/")` | `r.ProjectPath` in NDJSON = forward-slash path on Windows (e.g. `C:/Users/fana/code`) | `IsNodeModulesPackageJSON` return value |
| `internal/ecosystem/pnpm/pnpm.go` | 43, 87 | `parts := strings.Split(filepath.ToSlash(path), "/")` → `projectPath = strings.Join(parts[:pnpmIdx-1], "/")` | `r.ProjectPath` in NDJSON = forward-slash path on Windows | `IsPnpmStorePackageJSON` return value |
| `internal/ecosystem/editorext/editorext.go` | 57 | `parentSlash := filepath.ToSlash(parent)` used for `strings.HasSuffix` matching only | NOT a defect — `filepath.ToSlash` output never assigned to emitted `r.SourceFile` / `r.ProjectPath` | Classification predicate only |
| `internal/ecosystem/editorext/editorext.go` | 125 | `p := filepath.ToSlash(extRoot)` used for `strings.Contains` matching only | NOT a defect — same reason | `hostFromExtRoot` function |
| `internal/ecosystem/browserext/browserext.go` | 197 | `p := filepath.ToSlash(filepath.Dir(path))` for `strings.Contains` matching | NOT a defect — same reason | `IsFirefoxExtensionsJSON` predicate |
| `cmd/pollen/roots.go` | 125 | `p := filepath.ToSlash(filepath.Clean(path))` in `classifyRoot` | NOT a defect — `classifyRoot` sets `root_kind` metadata, not `source_file`/`project_path`. It is not called on record construction paths. | Root classification only |

### Fields NOT affected

- `r.SourceFile` in all ecosystem scanners: always set to the raw `path` argument from `walk.Walk`, which uses `filepath.WalkDir` — returns OS-native paths on all platforms. [VERIFIED: scanner.go `jobs <- job{path: path}` + ecosystem scanner `r.SourceFile = path`]
- `r.ProjectPath` in go, ruby, composer, pypi scanners: computed via `filepath.Dir(path)` which returns OS-native paths. [VERIFIED: these do not use `filepath.ToSlash` + string join patterns]
- `endpoint.OS`, `endpoint.Arch`, `endpoint.Username`, `endpoint.Hostname`: already correct on Windows (runtime.GOOS, runtime.GOARCH, user.Current().Username, os.Hostname). [VERIFIED: endpoint.go]

### WPATH-02 Defect Trace

`endpoint.Current()` line 29: `ep.UID = u.Uid` — on Windows, `user.Current().Uid` returns the SID string, not empty.
`endpoint.Current()` line 31 (fallback): `ep.UID = strconv.Itoa(os.Getuid())` — on Windows, `os.Getuid()` returns -1, so this would emit `"-1"` — also not empty.
Both code paths must be suppressed on Windows. [VERIFIED: endpoint.go source read]

---

## Common Pitfalls

### Pitfall 1: Mislocating the WPATH-01 fix in `internal/output/`

**What goes wrong:** The PRD (§5.2) names `internal/output/paths_windows.go` as the implementation locus. A planner following the PRD creates a file there but cannot hook it into the actual path values because the output package only receives already-formed `model.Record` structs. The fix has no effect.

**Why it happens:** The PRD was written before the live code existed and describes an idealized module split. The live code stores path-emission logic in ecosystem scanners, not in the output package.

**How to avoid:** Fix the two affected return values in `npm.go` and `pnpm.go`. The `internal/output/paths_windows.go` file (if created at all) would be a dead file — skip it or document it as intentionally empty with a note explaining the actual fix locus.

**Warning signs:** A plan that creates `internal/output/paths_windows.go` with no call site in `output.go` / `emitter.go` — that file cannot intercept path values.

### Pitfall 2: Modifying `normalize_diff.go` to strip path fields

**What goes wrong:** An implementer notices that path fields differ between platforms and strips them from the differential normalization to make the test pass. This hides the defect from the differential test instead of fixing it.

**Why it happens:** The differential test strips non-deterministic fields; paths look like candidates.

**How to avoid:** `normalize_diff.go` is LOCKED (documented in `differential_test.go`). Path fields are deterministic — they depend on the fixture tree, not on run time. They must NOT be stripped.

**Warning signs:** Any edit to `normalize_diff.go` or `normalize_diff_test.go`.

### Pitfall 3: PTEST-02 doesn't run on Windows (by design)

**What goes wrong:** A developer tries to verify WPATH-01 by running `TestDifferential` on Windows and gets a skip, concluding the test doesn't cover the fix.

**Why it happens:** `TestDifferential` has an explicit `runtime.GOOS == "windows"` skip in `differential_test.go`. The skip reason is documented: "differential runs on Linux+macOS only."

**How to avoid:** WPATH-01 correctness is asserted by Windows CI via `TestParityAllEcosystems` (parity test) plus a new targeted `TestWindowsPathShape` test in `roots_windows_test.go` or `parity_test.go`. The differential test only confirms Linux/macOS parity — it does not need to run on Windows.

### Pitfall 4: Using `HOME` instead of `USERPROFILE` in Windows tests

**What goes wrong:** A test uses `t.Setenv("HOME", ...)` for Windows path isolation. On Windows, `HOME` is not the canonical home env-var; `USERPROFILE` is. The test may silently pass or have unexpected behavior.

**Why it happens:** Unix muscle memory.

**How to avoid:** Phase 2 established the pattern: use `t.Setenv("USERPROFILE", ...)` / `t.Setenv("APPDATA", ...)` etc. on Windows tests. Never `HOME`. [VERIFIED: STATE.md accumulated decisions Phase 02-02]

### Pitfall 5: `endpoint_windows.go` requires a stub for `os.Getuid()` fallback

**What goes wrong:** If the `clearUID` override approach is used, the fallback branch in `endpoint.go` (`ep.UID = strconv.Itoa(os.Getuid())`) still runs before `clearUID` is called. This is fine because `clearUID` zeros it afterward. But if the `runtime.GOOS` guard approach is used, both the `u.Uid` assignment AND the `os.Getuid()` fallback must be guarded.

**Why it happens:** The fallback for `user.Current()` failure is a separate code path.

**How to avoid:** When using the inline `runtime.GOOS` guard, guard both the happy path (`u.Uid`) and the error path (`os.Getuid()`) — not just one. The `clearUID` helper approach is safer because it handles both paths unconditionally in one call.

### Pitfall 6: Phase 4 BKINT-01 boundary pollution

**What goes wrong:** The beekeeper round-trip test for D-03 creates `internal/inventory/` with a full interface/stub structure, anticipating BKINT-01's subprocess swap. This prematurely locks the Phase-4 interface design.

**Why it happens:** The roadmap names `internal/inventory/` as the test locus.

**How to avoid:** For Phase 3, the round-trip test is a unit test in `internal/scan/scanner_test.go` that feeds a hand-crafted Windows-shaped JSON string through the same `json.Unmarshal` validity check that `Scan` uses (or directly through `Scan` via `runBumblebeeFn` injection). No new package. Phase 4 creates `internal/inventory/` as part of the subprocess-swap work.

---

## Code Examples

### Windows endpoint fix — inline runtime.GOOS guard (minimal files)

```go
// Source: verified against internal/endpoint/endpoint.go (live code)
// internal/endpoint/endpoint.go (modified)
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
        }
        // On Windows: u.Uid is a SID string (S-1-5-21-...). WPATH-02 requires
        // endpoint.uid to be empty on Windows. Leave ep.UID as its zero value "".
    } else if runtime.GOOS != "windows" {
        ep.UID = strconv.Itoa(os.Getuid())
    }
    return ep
}
```

### npm projectPath fix

```go
// Source: verified against internal/ecosystem/npm/npm.go (live code)
// internal/ecosystem/npm/npm.go — IsNodeModulesPackageJSON
// Before (line 114):
//   projectPath := strings.Join(parts[:nmIdx], "/")
// After:
projectPath := filepath.FromSlash(strings.Join(parts[:nmIdx], "/"))
// filepath.FromSlash is a no-op on Linux/macOS (/ is native separator),
// so PTEST-02 (differential test) byte output is unchanged on Unix.
```

### pnpm projectPath fix

```go
// Source: verified against internal/ecosystem/pnpm/pnpm.go (live code)
// internal/ecosystem/pnpm/pnpm.go — IsPnpmStorePackageJSON
// Before (line 87):
//   projectPath = strings.Join(parts[:pnpmIdx-1], "/")
// After:
projectPath = filepath.FromSlash(strings.Join(parts[:pnpmIdx-1], "/"))
```

### Beekeeper round-trip test (co-located with internal/scan/)

```go
// Source: pattern mirrors TestScanWithBumblebee in scanner_test.go (verified)
// internal/scan/scanner_test.go (new test function)
func TestScanWindowsShapedRecord(t *testing.T) {
    old := runBumblebeeFn
    defer func() { runBumblebeeFn = old }()

    // Hand-crafted Windows-shaped Pollen NDJSON record:
    //   - project_path and source_file use backslash separators + drive letter
    //   - endpoint.os = "windows", endpoint.uid = ""
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
    // Assert: record passed through without error (not rewritten as scan_error)
    if strings.Contains(out, `"record_type":"scan_error"`) {
        t.Errorf("Windows-shaped record was rejected as malformed: %s", out)
    }
    if !strings.Contains(out, `"os":"windows"`) {
        t.Errorf("endpoint.os=windows not preserved in passthrough: %s", out)
    }
    if !strings.Contains(out, `"uid":""`) {
        t.Errorf("empty uid not preserved: %s", out)
    }
    // Assert backslash paths survive JSON round-trip
    // JSON encoding doubles backslash: C:\\ → C:\\\\
    if !strings.Contains(out, `C:\\`) {
        t.Errorf("Windows drive+backslash path not preserved: %s", out)
    }
}
```

### Parity test extension — Windows path shape assertion

```go
// Source: pattern mirrors assertEndpointOS in parity_test.go (verified)
// cmd/pollen/parity_test.go — new helper (Windows-only block in TestParityAllEcosystems)

// assertWindowsPathShape asserts that every record with a non-empty project_path
// or source_file field contains a drive letter + backslash (e.g. "C:\") rather
// than a forward-slash form. Only called on Windows.
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
                // Must contain a drive letter + backslash, not a /c/ form.
                if strings.Contains(v, "/") {
                    t.Errorf("assertWindowsPathShape: line %d field %q = %q contains forward slash (want backslash+drive)", i+1, field, v)
                }
                // Must have a drive letter (volume name like "C:")
                if len(v) < 2 || v[1] != ':' {
                    t.Errorf("assertWindowsPathShape: line %d field %q = %q missing drive letter", i+1, field, v)
                }
            }
        }
    }
}

// assertWindowsEndpointUID asserts that every record with an endpoint
// sub-object has an empty uid field. Only called on Windows.
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
                t.Errorf("assertWindowsEndpointUID: line %d: endpoint.uid = %q, want empty string on Windows", i+1, uidStr)
            }
        }
    }
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `filepath.ToSlash` + string join returning slash path | `filepath.FromSlash(strings.Join(...))` restoring OS-native path | Phase 3 | `project_path` in NDJSON uses backslash on Windows |
| `ep.UID = u.Uid` unconditionally | `ep.UID = u.Uid` only on non-Windows | Phase 3 | `endpoint.uid` is empty string on Windows |

**No deprecated/outdated patterns:** This is a greenfield fix, not a replacement of a deprecated library.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `go`, `pypi`, `ruby`, `composer` ecosystem scanners do NOT use `filepath.ToSlash` + join for `project_path` — they use `filepath.Dir(path)` which returns OS-native paths | Defect Trace | Low risk: confirmed by reading the npm/pnpm scanner code and searching the entire Pollen codebase for `filepath.ToSlash` occurrences. All 7 hits reviewed; only 2 flow into emitted record fields. |
| A2 | `bun`, `yarn` ecosystem scanners do NOT set `project_path` via slash-join | Defect Trace | Low risk: search confirmed no `filepath.ToSlash` in bun/ or yarn/. These scanners only set `r.SourceFile = path` (OS-native). |
| A3 | Windows CI runner (`windows-latest`) produces SID-shaped UIDs from `user.Current().Uid` (e.g. `S-1-5-21-...`) rather than a numeric string | WPATH-02 analysis | Medium risk: documented Go behavior on Windows; SID is the standard Windows user identifier returned by the Win32 API. [ASSUMED — not empirically tested in this session on a real Windows CI run] |
| A4 | `os.Getuid()` returns -1 on Windows (the `user.Current()` error fallback path) | WPATH-02 analysis | Low risk: documented Go stdlib behavior (`syscall.Getuid()` returns -1 on Windows). [ASSUMED from training knowledge] |

**Assumptions A3 and A4** are low-impact: even if the exact SID format differs, the requirement is `uid == ""` (not "not a SID"), so the fix (clear unconditionally on Windows) is robust regardless of what `user.Current().Uid` returns.

---

## Open Questions

1. **Does the parity fixture produce pnpm/npm `node_modules` records (not just lockfile records)?**
   - What we know: The fixture in `testdata/parity-fixture/` includes `npm-fixture/package-lock.json` and `pnpm-fixture/` subdirectory. Lockfile scanning sets `projectPath := filepath.Dir(path)` (not slash-joined — clean). Only `IsNodeModulesPackageJSON` and `IsPnpmStorePackageJSON` produce slash-joined `projectPath`.
   - What's unclear: Whether the parity fixture includes a `node_modules/` subtree that would trigger the buggy code paths, or whether it only exercises the lockfile-scanning path (which is already correct).
   - Recommendation: If the fixture only exercises lockfile scanning, add a `node_modules/` subtree to the parity fixture, or add a targeted Windows unit test in `npm_test.go` / `pnpm_test.go` that directly calls `IsNodeModulesPackageJSON` / `IsPnpmStorePackageJSON` with a Windows-style absolute path and asserts the return value uses backslashes.

2. **Should `endpoint_windows.go` / `endpoint_notwindows.go` be added or is inline `runtime.GOOS` guard preferred?**
   - What we know: Phase 2 used `roots_windows.go` + `roots_notwindows.go` for `windowsBaselinePackageRoots()` and `windowsSystemRoots()` — that was a whole function, not a single field assignment.
   - What's unclear: For a single field override (`ep.UID = ""`), the build-tagged approach adds two files for what is effectively a one-liner.
   - Recommendation: Use inline `runtime.GOOS != "windows"` guard in `endpoint.go`. It is readable, self-documenting, and avoids file proliferation. Reserve build-tagged files for cases where the entire function body differs between OSes (as in Phase 2).

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Build + test | ✓ | 1.25 (per go.mod) | — |
| Windows test environment | WPATH-01/02 assertions | ✓ (dev machine) | Windows 11 | CI `windows-latest` |
| Linux/macOS CI | PTEST-02 differential guard | ✓ (CI only) | ubuntu-latest, macos-latest | — (Windows dev cannot run) |

**Missing dependencies with no fallback:** None. All required tools are available.

**Windows-primary dev machine note:** Phase-3 work can be locally verified on the Windows dev machine (go test ./internal/endpoint/..., go test ./internal/ecosystem/npm/..., go test ./internal/scan/..., go test ./cmd/pollen/ -run TestParityAllEcosystems). The differential test (PTEST-02) always skips on Windows by design and is only verified by CI.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing stdlib (`go test`) |
| Config file | none (go.mod only) |
| Quick run command (Pollen, Windows) | `go test ./internal/endpoint/... ./internal/ecosystem/npm/... ./internal/ecosystem/pnpm/... ./cmd/pollen/ -run TestParityAllEcosystems` |
| Quick run command (Beekeeper, Windows) | `go test ./internal/scan/... -run TestScanWindowsShapedRecord` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| WPATH-01 | `project_path` in NDJSON contains backslash+drive-letter on Windows for npm/pnpm ecosystem records | unit | `go test ./internal/ecosystem/npm/... -run TestIsNodeModulesPackageJSONWindowsPath` | ❌ Wave 0 (new test in npm_test.go) |
| WPATH-01 | `project_path` in NDJSON contains backslash+drive-letter on Windows for pnpm records | unit | `go test ./internal/ecosystem/pnpm/... -run TestIsPnpmStorePackageJSONWindowsPath` | ❌ Wave 0 (new test in pnpm_test.go) |
| WPATH-01 | Parity fixture records on Windows have no forward-slash paths | integration | `go test ./cmd/pollen/ -run TestParityAllEcosystems` (Windows CI) | ✅ (extend parity_test.go) |
| WPATH-01 | PTEST-02 differential unchanged on Linux/macOS | regression | `go test ./cmd/pollen/ -run TestDifferential` (Linux/macOS CI) | ✅ (no change to differential_test.go) |
| WPATH-02 | `endpoint.uid` is empty string on Windows | unit | `go test ./internal/endpoint/... -run TestCurrentWindowsUID` | ❌ Wave 0 (new test in endpoint_test.go) |
| WPATH-02 | `endpoint.uid` non-empty on Linux/macOS (regression) | unit | `go test ./internal/endpoint/... -run TestCurrentPopulatesDeviceID` | ✅ (existing test in endpoint_test.go) |
| WPATH-02 | Beekeeper consumer passes Windows-shaped record without emitting scan_error | unit | `go test ./internal/scan/... -run TestScanWindowsShapedRecord` | ❌ Wave 0 (new test in scanner_test.go) |
| WPATH-01+02 | Windows NDJSON has backslash paths AND empty uid in parity fixture run | integration | `go test ./cmd/pollen/ -run TestParityAllEcosystems` (Windows CI) | ✅ (extend parity_test.go) |

### Sampling Rate

- **Per task commit:** `go test ./internal/endpoint/... ./internal/ecosystem/npm/... ./internal/ecosystem/pnpm/... ./internal/scan/...`
- **Per wave merge:** `go test ./...` in Pollen; `go test ./internal/scan/...` in Beekeeper
- **Phase gate:** Full suite green (`go test ./...`) in both repos before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `internal/endpoint/endpoint_test.go` — add `TestCurrentWindowsUID` (asserts `UID == ""` when `runtime.GOOS == "windows"`, and that existing `TestCurrentPopulatesDeviceID` still passes on all OSes)
- [ ] `internal/ecosystem/npm/npm_test.go` — add `TestIsNodeModulesPackageJSONWindowsPath` (call with `C:\Users\fana\code\web-app\node_modules\left-pad\package.json` absolute path; assert returned `projectPath = "C:\\Users\\fana\\code\\web-app"`)
- [ ] `internal/ecosystem/pnpm/pnpm_test.go` — add `TestIsPnpmStorePackageJSONWindowsPath` (call with Windows absolute pnpm store path; assert returned `projectPath` uses backslash)
- [ ] `cmd/pollen/parity_test.go` — add `assertWindowsPathShape` and `assertWindowsEndpointUID` helpers; call them inside `TestParityAllEcosystems` under `if runtime.GOOS == "windows"` block
- [ ] `internal/scan/scanner_test.go` (Beekeeper) — add `TestScanWindowsShapedRecord` as described in Code Examples

*(No new framework install needed — both repos use stdlib `testing`)*

---

## Security Domain

`security_enforcement` is not explicitly `false` in `.planning/config.json`. However, this phase is a narrowly-scoped path-format correctness fix with no new attack surface.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | partial | The `endpoint.uid` field accepts any string; the fix sets it to `""`. No validation of user input introduced. |
| V6 Cryptography | no | — |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path injection via malformed Windows path in NDJSON | Tampering | Not applicable — Pollen only EMITS paths it constructed from `filepath.WalkDir`; no path is parsed from user input |
| SID leakage in NDJSON | Information Disclosure | Fix: `endpoint.uid` is set to `""` on Windows, eliminating SID presence in emitted records |

---

## Sources

### Primary (HIGH confidence)

- `C:\Users\Bantu\mzansi-agentive\pollen\internal\endpoint\endpoint.go` — confirmed `user.Current().Uid` unconditional assignment (WPATH-02 defect)
- `C:\Users\Bantu\mzansi-agentive\pollen\internal\ecosystem\npm\npm.go` — confirmed slash-join `projectPath` leak (WPATH-01 defect, npm)
- `C:\Users\Bantu\mzansi-agentive\pollen\internal\ecosystem\pnpm\pnpm.go` — confirmed slash-join `projectPath` leak (WPATH-01 defect, pnpm)
- `C:\Users\Bantu\mzansi-agentive\pollen\internal\ecosystem\editorext\editorext.go` — confirmed `filepath.ToSlash` NOT leaking into emitted fields
- `C:\Users\Bantu\mzansi-agentive\pollen\internal\ecosystem\browserext\browserext.go` — confirmed `filepath.ToSlash` NOT leaking into emitted fields
- `C:\Users\Bantu\mzansi-agentive\pollen\cmd\pollen\roots.go` — confirmed `filepath.ToSlash` in `classifyRoot` NOT flowing into emitted record fields
- `C:\Users\Bantu\mzansi-agentive\pollen\internal\model\model.go` — confirmed `Endpoint.UID string` field; `schema_version = "0.1.0"` unchanged
- `C:\Users\Bantu\mzansi-agentive\pollen\internal\output\output.go` — confirmed no path transformation; thin JSON encoder only
- `C:\Users\Bantu\mzansi-agentive\pollen\internal\scanner\scanner.go` — confirmed `r.SourceFile = path` (raw OS-native path from walker)
- `C:\Users\Bantu\mzansi-agentive\pollen\cmd\pollen\parity_test.go` — confirmed `assertEndpointOS` pattern to mirror
- `C:\Users\Bantu\mzansi-agentive\pollen\cmd\pollen\roots_windows.go` — confirmed Phase-2 build-tag precedent pattern
- `C:\Users\Bantu\mzansi-agentive\beekeeper\internal\scan\scanner.go` — confirmed pure JSON passthrough; no struct parsing of endpoint
- `C:\Users\Bantu\mzansi-agentive\beekeeper\internal\audit\types.go` — confirmed `Endpoint string` (not `model.Endpoint`); no structural conflict

### Secondary (MEDIUM confidence)

- `C:\Users\Bantu\mzansi-agentive\beekeeper\.planning\STATE.md` — Phase 2 accumulated decisions confirming `t.Setenv(USERPROFILE/...)` pattern, build-tag stubs
- `C:\Users\Bantu\mzansi-agentive\beekeeper\.planning\phases\03-windows-path-representation\03-CONTEXT.md` — locked decisions; all confirmed against live code

### Tertiary (LOW confidence)

- A3/A4 in Assumptions Log — Go stdlib behavior for `user.Current().Uid` and `os.Getuid()` on Windows (training knowledge; empirically verifiable in CI)

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — pure stdlib Go; no external dependencies
- Architecture: HIGH — all defect sites verified by direct source code inspection
- Pitfalls: HIGH — live code confirms every pitfall hypothesis
- Test patterns: HIGH — existing parity and scanner tests provide direct templates

**Research date:** 2026-06-02
**Valid until:** 2026-07-02 (stable; upstream Pollen has no external dependency churn)
