# Phase 4: Windows Extension & MCP Coverage + Beekeeper Compat Test — Research

**Researched:** 2026-06-02
**Domain:** Go cross-platform path enumeration (Pollen fork) + beekeeper subprocess boundary (BKINT-01)
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**WEXT-01 — Windows editor-extension paths (PRD §8.2)**
- VS Code → `%USERPROFILE%\.vscode\extensions\`
- VS Code Insiders → `%USERPROFILE%\.vscode-insiders\extensions\`
- Cursor → `%USERPROFILE%\.cursor\extensions\`
- Windsurf → `%USERPROFILE%\.windsurf\extensions\`
- VSCodium → `%USERPROFILE%\.vscode-oss\extensions\`

**WEXT-02 — Windows browser-extension paths (PRD §8.3)**
- Chrome → `%LOCALAPPDATA%\Google\Chrome\User Data\<Profile>\Extensions\`
- Chromium → `%LOCALAPPDATA%\Chromium\User Data\<Profile>\Extensions\`
- Edge → `%LOCALAPPDATA%\Microsoft\Edge\User Data\<Profile>\Extensions\`
- Brave → `%LOCALAPPDATA%\BraveSoftware\Brave-Browser\User Data\<Profile>\Extensions\`
- Firefox → `%APPDATA%\Mozilla\Firefox\Profiles\<profile>\extensions.json`
- Chromium-family uses per-profile directory enumeration (`Default`, `Profile 1`, ...); Firefox is a per-profile `extensions.json` file.

**WEXT-03 — Windows MCP host-config paths (PRD §8.4)**
- Claude Desktop → `%APPDATA%\Claude\claude_desktop_config.json`
- Cursor MCP → `%USERPROFILE%\.cursor\mcp.json`
- Windsurf MCP → `%USERPROFILE%\.windsurf\mcp.json`
- Cline → `%APPDATA%\cline\cline_mcp_settings.json`
- Gemini CLI → `%USERPROFILE%\.gemini\settings.json`
- Generic `mcp.json` / `.mcp.json` → project-local, path style preserved

**BKINT-01 — Beekeeper mockable boundary (PRD §5.3)**
- Swap `bumblebee` subprocess invocation to `pollen`; preserve mockable injectable-var test pattern.
- `internal/inventory/` named as Phase 4 boundary package; or in-place evolution of `internal/scan` seam.
- Replaceability + testability are the two required properties.

**PTEST-04 — Pollen compatibility test (PRD §9.4)**
- Invokes `pollen scan`, asserts NDJSON schema-consistency, runs beekeeper rules, asserts no double-counting, asserts `scanner_name`.
- Zero `t.Skip` calls for inventory tests on Windows.
- Runs on ubuntu/macos/windows.

**Scope guardrails (PRD §4.2)**
- No new ecosystems.
- No schema/CLI/matching-semantics changes (`schema_version` stays `0.1.0`).
- Native Windows path discipline already landed in Phase 3 — new records follow same discipline.

**Release-tag handling (D-06)**
- Prepare VERSION + CHANGES.md locally (`v0.1.1-pollen.4`). Signed tag deferred to M2 close.

### Claude's Discretion

- Exact live-repo file locus for editor/browser/MCP enumeration (confirmed in this research — see Architecture section).
- Build-tag layout (`//go:build windows`), test fixture layout, profile-iteration helper structure.
- Whether per-OS path tables are functions or table literals.
- Whether BKINT-01 is a new `internal/inventory/` package or in-place evolution of `internal/scan` (research gives a concrete recommendation — see BKINT-01 section).
- Wave structure and cross-repo sequencing.

### Deferred Ideas (OUT OF SCOPE)

- `v0.1.1-pollen.4` signed tag + push — deferred to M2 close (D-06).
- SYNC-01 (UPSTREAM.md sync workflow) and SYNC-02 (upstream PRs) — Phase 5.
- PTEST-05 (Windows Sentry honeypot E2E) — Phase 5.
- BKINT-02 (beekeeper `go.mod` Pollen version pin + full CI green) — Phase 5.
- SDEF-01 (`pollen-self` catalog entries) — Phase 5.

</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| WEXT-01 | Windows editor-extension paths — 5 editors under `%USERPROFILE%` | Live locus confirmed: `browserExtensionCandidateRoots` case "windows": in `roots.go`; scanner in `internal/ecosystem/editorext/`; needs `extensionRootSegments` awareness of Windows paths and `hostFromExtRoot` fix for Windows paths |
| WEXT-02 | Windows browser-extension paths — 4 Chromium-family (per-profile `Extensions/`) + Firefox (per-profile `extensions.json`) | Live locus confirmed: `browserExtensionCandidateRoots` has empty `case "windows":` skeletons; `IsFirefoxExtensionsJSON` needs Windows path patterns; Phase 4 fills both skeletons |
| WEXT-03 | Windows MCP host-config paths — Claude Desktop, Cursor, Windsurf, Cline, Gemini CLI | Live locus confirmed: `baselineHomeCandidates` has `switch runtime.GOOS` for MCP dirs; needs `case "windows":` with APPDATA-based entries; `IsKnownMCPConfig` already dispatches on basenames correctly |
| BKINT-01 | Beekeeper `internal/inventory/` boundary: swap `bumblebee` → `pollen` subprocess behind mockable interface | Live code: `runBumblebeeFn` in `internal/scan/scanner.go` invokes `exec.LookPath("bumblebee")`; recommendation: evolve in-place (rename var + lookup target) rather than new package — see BKINT-01 analysis |
| PTEST-04 | Pollen compatibility test runs on all 3 OSes; Windows skip baseline = zero | Live code: `TestScanWindowsShapedRecord` already in `scanner_test.go`; no current `TestPollenCompat` integration test exists; Phase 4 adds it |

</phase_requirements>

---

## Summary

Phase 4 completes the Pollen fork's Windows coverage (three extension/MCP path tables) and wires the beekeeper-pollen compatibility test. Both work streams land across two repos: `../pollen` for the path tables + tests, and `beekeeper/internal/scan/` for the subprocess rename + compat test.

**Pollen work (WEXT-01/02/03):** The live fork already has the correct skeleton. The Phase 2 plan deliberately planted empty `case "windows":` blocks in `browserExtensionCandidateRoots` with the comment "Windows browser-extension paths are Phase 4." The `baselineHomeCandidates` function already handles Windows editor-extension roots via the shared `.vscode/extensions` etc. segment list — but that list only matches Unix-style paths. Two code artifacts need fixing: (1) `extensionRootSegments` in `editorext.go` uses slash-normalized suffix matching which works for Unix paths but NOT for Windows backslash paths coming in through the walker; (2) `hostFromExtRoot` uses `filepath.ToSlash` + string Contains for `/.cursor`, `/.windsurf`, `/.vscodium` — these patterns work after `ToSlash`, so this is actually safe. (3) `IsFirefoxExtensionsJSON` checks for `/Firefox/Profiles/` using `filepath.ToSlash(filepath.Dir(path))` — so Windows paths ARE correctly normalized before the Contains check. The scanner code is largely OS-agnostic; the main Phase 4 work is in `roots.go` (filling the two empty `case "windows":` blocks in `browserExtensionCandidateRoots`) and optionally adding Windows-specific MCP entries in `baselineHomeCandidates`.

**BKINT-01 analysis:** The clean recommendation is an in-place evolution of `internal/scan/scanner.go`. Create a `runPollenFn` injectable var that replaces `runBumblebeeFn`, change the `lookBumblebee` lookup to target `"pollen"`, and update the `scan_status` message text. A new `internal/inventory/` package would add infrastructure (interface + wrapper) for no concrete benefit at this point — beekeeper already has a clean injectable-var seam. The PRD §5.3 language about `internal/inventory/` describes the *concept* (black-box boundary) not a required package name; the seam already exists in `internal/scan`.

**PTEST-04:** Write a new integration test (`TestPollenCompatibility` in beekeeper) that: (a) uses the `runBumblebeeFn`/`runPollenFn` injection to feed canned extension/browser/MCP records from all three Windows fixture shapes, (b) asserts `scanner_name = "pollen"`, (c) asserts no double-counting (same record appearing twice), (d) asserts the beekeeper `Scan` function accepts the records without emitting `scan_error`. The test is OS-independent (fixture-driven, no process spawn) — this is what removes the Windows skip requirement.

**Primary recommendation:** Fill `browserExtensionCandidateRoots` Windows skeletons + add Windows MCP entries in `baselineHomeCandidates`; rename `runBumblebeeFn` → `runPollenFn` + update lookup target in `internal/scan/scanner.go`; add `TestPollenCompatibility` in beekeeper. Commit to both repos separately with `git -C ../pollen` pattern.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Windows editor-extension root discovery | Pollen (`cmd/pollen/roots.go` `baselineHomeCandidates`) | — | `extensionRootSegments` already wired; adds `%USERPROFILE%` env-var expansion, which is already done via `filepath.FromSlash(seg)` for all OSes |
| Windows browser-extension root discovery | Pollen (`cmd/pollen/roots.go` `browserExtensionCandidateRoots`) | — | Empty `case "windows":` skeletons already planted in Phase 2; Phase 4 fills them |
| Windows MCP config root discovery | Pollen (`cmd/pollen/roots.go` `baselineHomeCandidates`) | — | Existing `switch runtime.GOOS` block already handles darwin/linux; Windows adds APPDATA-based entries |
| Extension/browser/MCP record scanning | Pollen (`internal/ecosystem/editorext/`, `browserext/`, `mcp/`) | — | Scanners are OS-agnostic; they receive paths from the walker; no Windows-specific logic needed in scanners themselves |
| Firefox Windows path detection | Pollen (`internal/ecosystem/browserext/browserext.go`) | — | `IsFirefoxExtensionsJSON` uses `filepath.ToSlash` before Contains check — safe; may need `/Mozilla/Firefox/Profiles/` Windows path variant |
| Subprocess rename (bumblebee→pollen) | Beekeeper (`internal/scan/scanner.go`) | — | `lookBumblebee` + `runBumblebeeFn` injectable vars live here |
| Pollen compatibility test | Beekeeper (`internal/scan/scanner_test.go`) | — | Extends the existing injectable-var test pattern; no new package |
| Release preparation | Pollen (`VERSION`, `CHANGES.md`) | — | Matches Phase 2/3 precedent (D-06) |

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `path/filepath` (stdlib) | Go 1.25 | OS-native path construction, `filepath.Join`, `filepath.FromSlash` | All existing code uses it; no external dependency |
| `os` (stdlib) | Go 1.25 | `os.Getenv("LOCALAPPDATA")` etc. | Established Windows env-var access pattern from Phase 2 |
| `runtime` (stdlib) | Go 1.25 | `runtime.GOOS` dispatch | Already used in `roots.go` for darwin/linux/windows branches |
| `testing` (stdlib) | Go 1.25 | Test framework | Both repos use stdlib testing only |

No new external dependencies for this phase. All work is pure stdlib + in-tree logic. [VERIFIED: go.mod in both repos]

**Installation:** None required.

---

## Architecture Patterns

### System Architecture Diagram

```
WEXT-01/02/03 — Pollen path enumeration flow (Windows)

  baselineHomeCandidates(home)
     │
     ├── Editor extension roots (already wired for all OSes):
     │   for seg in extensionRootSegments:
     │     add(filepath.Join(home, filepath.FromSlash(seg)), RootKindEditorExtension)
     │   → C:\Users\fana\.vscode\extensions\
     │   → C:\Users\fana\.cursor\extensions\       ← Assumption A1 validation in Phase 4
     │   → C:\Users\fana\.windsurf\extensions\
     │   → C:\Users\fana\.vscode-insiders\extensions\
     │   → C:\Users\fana\.vscode-oss\extensions\   (NOTE: ".vscodium" in segments, but PRD says ".vscode-oss")
     │
     ├── MCP config roots — NEW case "windows": block:
     │   case "windows":
     │     add(filepath.Join(APPDATA, "Claude"), RootKindMCPConfig)       ← claude_desktop_config.json
     │     add(filepath.Join(APPDATA, "cline"), RootKindMCPConfig)         ← cline_mcp_settings.json
     │     (cursor/windsurf/gemini already added via cross-platform .cursor/.windsurf/.gemini segments)
     │
     └── Browser extension roots:
         for r in browserExtensionCandidateRoots(home):
           add(r, RootKindBrowserExtension)

  browserExtensionCandidateRoots(home)  ← FILL the "case windows:" skeletons
     │
     ├── chromiumBases switch runtime.GOOS:
     │   case "windows":  ← CURRENTLY EMPTY (Phase 2 skeleton)
     │     localappdata = os.Getenv("LOCALAPPDATA")
     │     chromiumBases["chrome"] = [filepath.Join(localappdata, "Google", "Chrome", "User Data")]
     │     chromiumBases["chromium"] = [filepath.Join(localappdata, "Chromium", "User Data")]
     │     chromiumBases["edge"] = [filepath.Join(localappdata, "Microsoft", "Edge", "User Data")]
     │     chromiumBases["brave"] = [filepath.Join(localappdata, "BraveSoftware", "Brave-Browser", "User Data")]
     │   for each base+profile: add(filepath.Join(base, profile, "Extensions"))
     │
     └── Firefox switch runtime.GOOS:
         case "windows":  ← CURRENTLY EMPTY (Phase 2 skeleton)
           appdata = os.Getenv("APPDATA")
           roots = append(roots, filepath.Join(appdata, "Mozilla", "Firefox", "Profiles"))

  IsFirefoxExtensionsJSON(path)  ← NEEDS Windows path pattern
     filepath.ToSlash(filepath.Dir(path)) already normalizes Windows paths
     → need to add: strings.Contains(p, "/Mozilla/Firefox/Profiles/")
       (note: filepath.ToSlash converts %APPDATA%\Mozilla\Firefox\Profiles\<profile>
        → C:/Users/fana/AppData/Roaming/Mozilla/Firefox/Profiles/<profile>
        → already matches existing "/Firefox/Profiles/" pattern)
     → NO CHANGE NEEDED if the existing pattern handles it

BKINT-01 — Beekeeper subprocess rename (internal/scan/scanner.go)

  BEFORE:
    var lookBumblebee = func() (string, error) { return exec.LookPath("bumblebee") }
    var runBumblebeeFn = func(ctx, deep) (<-chan []byte, bool) { return defaultRunBumblebee(...) }
    func defaultRunBumblebee(...) { bin, err := lookBumblebee(); ... }

  AFTER:
    var lookPollenFn = func() (string, error) { return exec.LookPath("pollen") }
    var runPollenFn = func(ctx, deep) (<-chan []byte, bool) { return defaultRunPollen(...) }
    func defaultRunPollen(...) { bin, err := lookPollenFn(); ... }
    // scan_status message updated: "pollen_unavailable": true

PTEST-04 — Beekeeper Pollen compatibility test (internal/scan/scanner_test.go)

  TestPollenCompatibility:
    Uses runPollenFn injection (same pattern as TestScanWithBumblebee)
    Feeds 5 fixture records: 1 npm, 1 editor-extension, 1 browser-extension, 1 mcp, 1 scan_summary
    Each has scanner_name="pollen" and Windows-shaped paths (backslash + drive letter)
    Asserts:
      - No scan_error records emitted
      - All 5 record_types passed through
      - scanner_name="pollen" preserved
      - No double-counting (each record appears exactly once)
    NO t.Skip anywhere — runs on all 3 OSes (fixture-driven, no binary spawn)
```

### Recommended Project Structure

Pollen additions:
```
cmd/pollen/
  roots.go                # fill case "windows": blocks in browserExtensionCandidateRoots
                          # add case "windows": for APPDATA MCP paths in baselineHomeCandidates
  roots_windows_test.go   # add TestWindowsExtensionMCPRoots (browser+MCP paths)
  testdata/
    extension-mcp-fixture/  # new: fake extension/MCP tree for WEXT-01/02/03
      vscode-ext/         # fake VS Code extension package.json
      cursor-ext/         # fake Cursor extension package.json
      chrome-ext/         # fake Chromium manifest.json
      firefox-profile/    # fake extensions.json
      claude-mcp/         # fake claude_desktop_config.json
      cursor-mcp/         # fake cursor mcp.json
internal/ecosystem/
  browserext/
    browserext.go         # verify/fix IsFirefoxExtensionsJSON for Windows paths
  editorext/
    editorext.go          # verify hostFromExtRoot handles Windows backslash paths
VERSION                   # bump to 0.1.1-pollen.4
CHANGES.md                # add WEXT section
```

Beekeeper additions:
```
internal/scan/
  scanner.go              # rename runBumblebeeFn → runPollenFn, lookBumblebee → lookPollenFn
  scanner_test.go         # add TestPollenCompatibility
```

### Pattern 1: Fill `case "windows":` skeleton in `browserExtensionCandidateRoots`

**What:** The Phase 2 plan planted empty `case "windows":` blocks with explicit deferral comments. Phase 4 fills them with the Windows LOCALAPPDATA/APPDATA paths from PRD §8.3.

**When to use:** Any time a `switch runtime.GOOS` block has a `case "windows":` stub.

**Example (roots.go — chromiumBases section):**
```go
// Source: VERIFIED against live ../pollen/cmd/pollen/roots.go (lines 560-563)
case "windows":
    if localappdata := os.Getenv("LOCALAPPDATA"); localappdata != "" {
        userData := filepath.Join(localappdata, "Google", "Chrome", "User Data")
        chromiumBases["chrome"] = []string{userData}
        chromiumBases["chromium"] = []string{filepath.Join(localappdata, "Chromium", "User Data")}
        chromiumBases["edge"] = []string{filepath.Join(localappdata, "Microsoft", "Edge", "User Data")}
        chromiumBases["brave"] = []string{filepath.Join(localappdata, "BraveSoftware", "Brave-Browser", "User Data")}
    }
```

**Example (roots.go — Firefox section):**
```go
// Source: VERIFIED against live ../pollen/cmd/pollen/roots.go (lines 593-595)
case "windows":
    if appdata := os.Getenv("APPDATA"); appdata != "" {
        roots = append(roots,
            filepath.Join(appdata, "Mozilla", "Firefox", "Profiles"),
        )
    }
```

**Note:** The existing loop `for each base+profile: roots = append(roots, filepath.Join(b, prof, "Extensions"))` already runs for all platforms after the chromiumBases switch. The Windows profiles list (`Default`, `Profile 1`, etc.) is already declared at line 530 and shared across all platforms — no changes needed there. [VERIFIED: roots.go line 530-531]

### Pattern 2: Windows MCP entries in `baselineHomeCandidates`

**What:** The existing `switch runtime.GOOS` block in `baselineHomeCandidates` (lines 269-276) handles darwin and linux MCP paths. Windows needs APPDATA-based entries for Claude Desktop (`%APPDATA%\Claude`) and Cline (`%APPDATA%\cline`). The other three (Cursor, Windsurf, Gemini) are already covered by the cross-platform dotfile roots added unconditionally (`home + "/.cursor"`, `home + "/.windsurf"`, `home + "/.gemini"` — and on Windows, `os.UserHomeDir()` returns `%USERPROFILE%`).

```go
// Source: VERIFIED against live ../pollen/cmd/pollen/roots.go (lines 269-276)
// Extend the existing switch:
switch runtime.GOOS {
case "darwin":
    add(filepath.Join(home, "Library", "Application Support", "Claude"), model.RootKindMCPConfig)
case "linux":
    add(filepath.Join(home, ".config", "Claude"), model.RootKindMCPConfig)
    add(filepath.Join(home, ".config", "Claude Code"), model.RootKindMCPConfig)
    add(filepath.Join(home, ".continue"), model.RootKindMCPConfig)
case "windows":
    // Claude Desktop on Windows: %APPDATA%\Claude\claude_desktop_config.json
    // Cline on Windows: %APPDATA%\cline\cline_mcp_settings.json
    // (Cursor/.cursor, Windsurf/.windsurf, Gemini/.gemini already added unconditionally above)
    if appdata := os.Getenv("APPDATA"); appdata != "" {
        add(filepath.Join(appdata, "Claude"), model.RootKindMCPConfig)
        add(filepath.Join(appdata, "cline"), model.RootKindMCPConfig)
    }
}
```

**Important nuance:** `os.UserHomeDir()` on Windows returns `%USERPROFILE%` (e.g. `C:\Users\Bantu`). The existing cross-platform roots `filepath.Join(home, ".cursor")` → `C:\Users\Bantu\.cursor` and `filepath.Join(home, ".windsurf")` → `C:\Users\Bantu\.windsurf` already produce the correct Windows paths for Cursor MCP and Windsurf MCP. The `IsKnownMCPConfig` dispatch already matches `mcp.json` and `cline_mcp_settings.json` by basename — no scanner changes needed. [VERIFIED: mcp.go `IsKnownMCPConfig` function, roots.go lines 264-267]

### Pattern 3: Injectable-var rename (BKINT-01)

**What:** Rename the two injectable vars in `internal/scan/scanner.go` to target `pollen` instead of `bumblebee`. Preserve the exact same function signature and channel pattern so all existing tests continue to compile and pass.

```go
// Source: VERIFIED against live beekeeper/internal/scan/scanner.go
// BEFORE:
var lookBumblebee = func() (string, error) { return exec.LookPath("bumblebee") }
var runBumblebeeFn = func(ctx context.Context, deep bool) (<-chan []byte, bool) {
    return defaultRunBumblebee(ctx, deep)
}
func defaultRunBumblebee(ctx context.Context, deep bool) (<-chan []byte, bool) { ... }

// AFTER:
var lookPollenFn = func() (string, error) { return exec.LookPath("pollen") }
var runPollenFn = func(ctx context.Context, deep bool) (<-chan []byte, bool) {
    return defaultRunPollen(ctx, deep)
}
func defaultRunPollen(ctx context.Context, deep bool) (<-chan []byte, bool) {
    bin, err := lookPollenFn()
    // ... rest identical, just renamed ...
}
```

The `scan_status` record text `"bumblebee_unavailable": true` should become `"pollen_unavailable": true`. The `"source": "bumblebee"` in the scan_error record becomes `"source": "pollen"`.

**Cascade:** The `cmd/beekeeper/main_test.go` file tests against `scanOnDeltaFn` (which wraps `scan.Scan`) — it does NOT reference `runBumblebeeFn` directly. [VERIFIED: main_test.go grep] The only references to `runBumblebeeFn` are in `internal/scan/scanner.go` and `internal/scan/scanner_test.go`. Update all three locations.

### Pattern 4: PTEST-04 compatibility test (fixture-driven, no binary spawn)

**What:** A new test function in `internal/scan/scanner_test.go` that exercises the full `Scan` path with Pollen-origin records from all three new Windows record types.

```go
// Source: pattern mirrors TestScanWithBumblebee and TestScanWindowsShapedRecord
// (VERIFIED: internal/scan/scanner_test.go — both tests already present)
func TestPollenCompatibility(t *testing.T) {
    old := runPollenFn   // after BKINT-01 rename
    defer func() { runPollenFn = old }()

    // Feed 5 Pollen record types including the new extension/MCP variants.
    // All records use scanner_name="pollen" and Windows-shaped paths.
    fixtures := []string{
        // npm package record (already covered by TestScanWindowsShapedRecord)
        `{"record_type":"package","schema_version":"0.1.0","scanner_name":"pollen",` +
            `"ecosystem":"npm","normalized_name":"left-pad","version":"1.3.0",` +
            `"project_path":"C:\\Users\\fana\\code","source_file":"C:\\Users\\fana\\code\\package.json"}`,
        // editor-extension record (WEXT-01)
        `{"record_type":"package","schema_version":"0.1.0","scanner_name":"pollen",` +
            `"ecosystem":"editor-extension","normalized_name":"ms-python.python",` +
            `"version":"2026.4.0","source_type":"editor-extension",` +
            `"source_file":"C:\\Users\\fana\\.vscode\\extensions\\ms-python.python-2026.4.0\\package.json"}`,
        // browser-extension record (WEXT-02)
        `{"record_type":"package","schema_version":"0.1.0","scanner_name":"pollen",` +
            `"ecosystem":"browser-extension","normalized_name":"abcdefghijklmnopabcdefghijklmnop",` +
            `"version":"1.0.0","source_type":"browser-extension",` +
            `"source_file":"C:\\Users\\fana\\AppData\\Local\\Google\\Chrome\\User Data\\Default\\Extensions\\abcdefghijklmnopabcdefghijklmnop\\1.0.0\\manifest.json"}`,
        // mcp-config record (WEXT-03)
        `{"record_type":"package","schema_version":"0.1.0","scanner_name":"pollen",` +
            `"ecosystem":"mcp","package_manager":"mcp","source_type":"mcp-config",` +
            `"source_file":"C:\\Users\\fana\\AppData\\Roaming\\Claude\\claude_desktop_config.json"}`,
        // scan_summary record
        `{"record_type":"scan_summary","schema_version":"0.1.0","scanner_name":"pollen",` +
            `"status":"complete","total_records":4}`,
    }
    runPollenFn = func(_ context.Context, _ bool) (<-chan []byte, bool) {
        ch := make(chan []byte, len(fixtures))
        for _, f := range fixtures {
            ch <- []byte(f)
        }
        close(ch)
        return ch, true
    }

    var buf bytes.Buffer
    if err := Scan(context.Background(), Config{}, &buf); err != nil {
        t.Fatalf("Scan: %v", err)
    }
    out := buf.String()

    // PTEST-04 assertions:
    if strings.Contains(out, `"record_type":"scan_error"`) {
        t.Errorf("Pollen records rejected as malformed: %s", out)
    }
    if strings.Count(out, `"scanner_name":"pollen"`) < 4 {
        t.Errorf("scanner_name=pollen not preserved on all records")
    }
    // No double-counting: each record appears exactly once
    for _, fixture := range fixtures[:4] { // exclude scan_summary (may not have unique key)
        var rec map[string]any
        _ = json.Unmarshal([]byte(fixture), &rec)
        if sf, ok := rec["source_file"].(string); ok && sf != "" {
            sfJSON, _ := json.Marshal(sf)
            count := strings.Count(out, string(sfJSON))
            if count != 1 {
                t.Errorf("double-counting: source_file %s appears %d times", sf, count)
            }
        }
    }
}
```

### Pattern 5: Windows fixture for `roots_windows_test.go` (WEXT-01/02/03)

**What:** Extend the existing `TestWindowsBaselineRoots` pattern in `roots_windows_test.go` with a new test that creates fake extension/MCP directories and asserts they appear in `resolveRoots` output.

```go
// Source: pattern from roots_windows_test.go TestWindowsBaselineRoots (VERIFIED)
// //go:build windows  (build-tagged — only runs on Windows CI)
func TestWindowsExtensionMCPRoots(t *testing.T) {
    tmp := t.TempDir()
    appdata := filepath.Join(tmp, "AppData", "Roaming")
    localappdata := filepath.Join(tmp, "AppData", "Local")

    t.Setenv("USERPROFILE", tmp)
    t.Setenv("APPDATA", appdata)
    t.Setenv("LOCALAPPDATA", localappdata)

    // WEXT-01: editor extensions — created via filepath.Join(home, ".vscode", "extensions") etc.
    // (home = USERPROFILE = tmp)
    mustMkdir(t, filepath.Join(tmp, ".vscode", "extensions"))
    mustMkdir(t, filepath.Join(tmp, ".cursor", "extensions"))
    mustMkdir(t, filepath.Join(tmp, ".windsurf", "extensions"))
    mustMkdir(t, filepath.Join(tmp, ".vscode-insiders", "extensions"))
    mustMkdir(t, filepath.Join(tmp, ".vscode-oss", "extensions"))  // NOTE: check actual segment

    // WEXT-02: browser extension roots (per-profile)
    mustMkdir(t, filepath.Join(localappdata, "Google", "Chrome", "User Data", "Default", "Extensions"))
    mustMkdir(t, filepath.Join(localappdata, "BraveSoftware", "Brave-Browser", "User Data", "Default", "Extensions"))

    // WEXT-03: MCP config directories
    mustMkdir(t, filepath.Join(appdata, "Claude"))      // Claude Desktop
    mustMkdir(t, filepath.Join(appdata, "cline"))       // Cline
    mustMkdir(t, filepath.Join(tmp, ".cursor"))         // Cursor MCP
    mustMkdir(t, filepath.Join(tmp, ".gemini"))         // Gemini CLI

    roots, _, err := resolveRoots(model.ProfileBaseline, nil, rootsOpts{})
    if err != nil {
        t.Fatalf("resolveRoots: %v", err)
    }
    // Assert: all 5 editor extension roots present, browser roots present, MCP roots present
    // ... (path-based assertions)
}
```

### Anti-Patterns to Avoid

- **Creating a new `internal/inventory/` package for BKINT-01:** Unnecessary abstraction at this stage. The injectable-var seam already exists. Phase 5 (BKINT-02) is the right time to add a formal interface if the version-pin requires it. Adding a package now creates overhead with no benefit.
- **Modifying `extensionRootSegments` in `editorext.go`:** The existing segments list (`.vscode/extensions`, `.cursor/extensions`, etc.) is used via `filepath.ToSlash(parent)` suffix matching. `filepath.ToSlash` converts Windows backslashes to forward slashes BEFORE the suffix check, so the existing Unix-style segment strings work correctly on Windows. Do NOT add Windows-backslash variants to the list — that would break the invariant.
- **Hardcoding backslash paths in `roots.go`:** Always construct paths via `filepath.Join + os.Getenv`. Never use hardcoded `\\` or `C:\` strings (Phase 2 Pitfall 1).
- **Using `HOME` env var in Windows tests:** Always `t.Setenv("USERPROFILE", ...)`. Windows tests for browser/MCP paths also need `t.Setenv("APPDATA", ...)` and `t.Setenv("LOCALAPPDATA", ...)`.
- **Confusing `.vscodium` vs `.vscode-oss`:** The PRD §8.2 says `%USERPROFILE%\.vscode-oss\extensions\` for VSCodium. The existing `extensionRootSegments` list uses `.vscodium/extensions`. The `baselineHomeCandidates` loop uses `.vscodium/extensions` via `filepath.FromSlash(seg)`. This means on Windows the root is `USERPROFILE\.vscodium\extensions` but the PRD says `.vscode-oss`. **Research finding:** The upstream Bumblebee uses `.vscodium` for the VSCodium path; the PRD may use `.vscode-oss` interchangeably. Check the `extensionRootSegments` list in `editorext.go` — `.vscodium/extensions` is what the scanner recognizes. If the plan target is `.vscode-oss`, add `.vscode-oss/extensions` to `extensionRootSegments` AND to `baselineHomeCandidates`. [VERIFIED: editorext.go line 47: `.vscodium/extensions` present; `.vscode-oss/extensions` absent] This is a gap that must be resolved in planning — the PRD path and the live scanner do not match.
- **Calling `scanOnDeltaFn` vs `runPollenFn`:** The `scanOnDeltaFn` in `cmd/beekeeper/main.go` wraps `scan.Scan` — it is NOT the same as `runPollenFn` inside `scan/scanner.go`. Do not confuse the two layers.
- **Signing/tagging the release:** This phase only bumps `VERSION` and `CHANGES.md`. No `git tag`, no cosign. D-06 is locked.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| OS-native path construction | Hardcoded `\\` strings | `filepath.Join + os.Getenv(...)` | Same discipline as Phase 2 `roots_windows.go` — handles all separator edge cases |
| Profile iteration for browser extensions | Custom profile enumerator | The existing `chromiumProfiles` slice (line 530 in roots.go) | Already correct: `[]string{"Default", "Profile 1", ..., "Profile 9"}` |
| Firefox path detection on Windows | New Windows-specific function | `IsFirefoxExtensionsJSON` with `filepath.ToSlash` | Already uses `filepath.ToSlash` before Contains — works on Windows paths |
| Subprocess mock in tests | New mock framework | Injectable `runPollenFn` var (renamed from `runBumblebeeFn`) | Established pattern; zero new infrastructure |
| Extension scanner for Windows | New `editorext_windows.go` | None needed — existing scanner is path-agnostic | Scanner receives OS-native paths from walker; no Windows-specific parsing |

---

## WEXT-01: Editor Extension Locus Analysis

### Where editor-extension scanning lives

[VERIFIED: direct source code inspection]

**Discovery layer (roots.go):** `baselineHomeCandidates` already adds editor-extension roots for ALL platforms at lines 245-256:

```go
for _, seg := range []string{
    ".vscode/extensions",
    ".vscode-insiders/extensions",
    ".vscode-server/extensions",
    ".cursor/extensions",
    ".cursor-server/extensions",
    ".windsurf/extensions",
    ".windsurf-server/extensions",
    ".vscodium/extensions",   // ← ".vscode-oss" is NOT here
} {
    add(filepath.Join(home, filepath.FromSlash(seg)), model.RootKindEditorExtension)
}
```

On Windows with `home = C:\Users\Bantu` (from `os.UserHomeDir()`), this produces:
- `C:\Users\Bantu\.vscode\extensions` ← VS Code ✓
- `C:\Users\Bantu\.vscode-insiders\extensions` ← VS Code Insiders ✓
- `C:\Users\Bantu\.cursor\extensions` ← Cursor ✓ (validates Assumption A1)
- `C:\Users\Bantu\.windsurf\extensions` ← Windsurf ✓
- `C:\Users\Bantu\.vscodium\extensions` ← VSCodium (but PRD says `.vscode-oss`) ← GAP

**Scanner layer (editorext.go):** `IsExtensionPackageJSON` uses `filepath.ToSlash(parent)` for suffix matching against `extensionRootSegments`. This is correct for Windows — the input path is OS-native from `filepath.WalkDir`, and `filepath.ToSlash` normalizes it before the Contains check. [VERIFIED: editorext.go lines 57-63]

**`hostFromExtRoot` (editorext.go lines 124-136):** Uses `filepath.ToSlash(extRoot)` then `strings.Contains(p, "/.cursor")`, `"/.windsurf"`, `"/.vscodium"`. On Windows, `extRoot` is `C:\Users\Bantu\.cursor\extensions` → after `ToSlash` → `C:/Users/Bantu/.cursor/extensions` → `strings.Contains(..., "/.cursor")` → true. Works correctly on Windows. [VERIFIED: editorext.go lines 124-136]

**Conclusion:** Editor extension scanning is fully operational on Windows once the root paths are present. The only issue is the `.vscodium` vs `.vscode-oss` discrepancy.

### VSCodium path gap — CONFIRMED

The PRD §8.2 says `%USERPROFILE%\.vscode-oss\extensions\`. The live code uses `.vscodium/extensions`. Both `baselineHomeCandidates` and `extensionRootSegments` use `.vscodium`. 

**Decision required for planner:** Does Phase 4 add `.vscode-oss/extensions` to both `extensionRootSegments` (editorext.go) AND `baselineHomeCandidates` (roots.go)? The PRD path is authoritative per CONTEXT.md "Everything in the PRD is treated as a locked decision." The recommendation is: add BOTH `.vscodium/extensions` (keep existing for upstream parity) AND `.vscode-oss/extensions` (PRD-mandated) to both lists, since some VSCodium installs use one path and some use the other. [ASSUMED — needs planner decision]

---

## WEXT-02: Browser Extension Locus Analysis

### Where browser-extension scanning lives

[VERIFIED: direct source code inspection]

**Discovery layer (roots.go `browserExtensionCandidateRoots`):**

```
Lines 560-563 (chromiumBases switch):
case "windows":
    // Windows browser-extension paths are Phase 4 (WEXT-02). This skeleton
    // keeps the switch exhaustive for Windows and suppresses build warnings.

Lines 593-595 (Firefox switch):
case "windows":
    // Windows Firefox/browser-extension paths are Phase 4 (WEXT-02). This skeleton
    // keeps the switch exhaustive for Windows and suppresses build warnings.
```

Both skeletons are confirmed empty. Phase 4 fills them with the LOCALAPPDATA-based paths from PRD §8.3.

**Scanner layer (browserext.go):**
- `IsChromiumExtensionManifest`: uses `filepath.Base` only — OS-agnostic. [VERIFIED]
- `IsFirefoxExtensionsJSON`: uses `filepath.ToSlash(filepath.Dir(path))` then Contains checks. The Windows path `%APPDATA%\Mozilla\Firefox\Profiles\<profile>\extensions.json` converts to `.../.../AppData/Roaming/Mozilla/Firefox/Profiles/<profile>`, which contains `/Firefox/Profiles/` — already matched by the existing pattern. NO change needed to `IsFirefoxExtensionsJSON`. [VERIFIED: browserext.go lines 193-208]

**`chromiumProfiles` slice (line 530):** `[]string{"Default", "Profile 1", ..., "Profile 9"}` — already declared and shared across all OS branches. Windows uses the same set.

**Note on Brave Windows path:** PRD §8.3 says `%LOCALAPPDATA%\BraveSoftware\Brave-Browser\User Data\<Profile>\Extensions\`. The existing darwin/linux case uses `BraveSoftware/Brave-Browser` — consistent with the Windows path.

---

## WEXT-03: MCP Config Locus Analysis

### Where MCP config scanning lives

[VERIFIED: direct source code inspection]

**Discovery layer:** `baselineHomeCandidates` has the MCP switch at lines 269-276:
```go
switch runtime.GOOS {
case "darwin":
    add(filepath.Join(home, "Library", "Application Support", "Claude"), model.RootKindMCPConfig)
case "linux":
    add(filepath.Join(home, ".config", "Claude"), ...)
    add(filepath.Join(home, ".config", "Claude Code"), ...)
    add(filepath.Join(home, ".continue"), ...)
}
// cross-platform (ALL OSes including Windows already get):
add(filepath.Join(home, ".cursor"), model.RootKindMCPConfig)          // Cursor MCP
add(filepath.Join(home, ".codeium", "windsurf"), model.RootKindMCPConfig) // Windsurf MCP
add(filepath.Join(home, ".claude"), model.RootKindMCPConfig)
add(filepath.Join(home, ".codex"), model.RootKindMCPConfig)
add(filepath.Join(home, ".gemini"), model.RootKindMCPConfig)           // Gemini CLI
```

On Windows, `home` = `os.UserHomeDir()` = `%USERPROFILE%`. So `filepath.Join(home, ".cursor")` = `C:\Users\Bantu\.cursor` — which is the directory containing `mcp.json`. The `IsKnownMCPConfig("mcp.json")` dispatch already handles this file. [VERIFIED: mcp.go line 59-69]

**Windows-only additions needed (not already covered):**
- `%APPDATA%\Claude\claude_desktop_config.json` — `%APPDATA%` ≠ `%USERPROFILE%`, so `.claude` does NOT cover this. Add `filepath.Join(appdata, "Claude")`.
- `%APPDATA%\cline\cline_mcp_settings.json` — Add `filepath.Join(appdata, "cline")`.

Already covered by cross-platform roots (no action needed):
- `%USERPROFILE%\.cursor\mcp.json` ← `filepath.Join(home, ".cursor")` ✓
- `%USERPROFILE%\.windsurf\mcp.json` ← `.codeium/windsurf` does NOT cover this; `.windsurf` is not in the list. Check: `add(filepath.Join(home, ".codeium", "windsurf"), ...)` ← this is `C:\Users\Bantu\.codeium\windsurf`, NOT `C:\Users\Bantu\.windsurf`. GAP: `.windsurf` for MCP is missing from the cross-platform list! Need to add `add(filepath.Join(home, ".windsurf"), model.RootKindMCPConfig)`. [VERIFIED: roots.go lines 264-267]
- `%USERPROFILE%\.gemini\settings.json` ← `filepath.Join(home, ".gemini")` ✓

**`IsGeminiSettingsJSON` (mcp.go line 78):** `filepath.Base(path) == "settings.json" && filepath.Base(filepath.Dir(path)) == ".gemini"`. On Windows `filepath.Dir("C:\Users\fana\.gemini\settings.json")` returns `C:\Users\fana\.gemini` and `filepath.Base("C:\Users\fana\.gemini")` returns `.gemini`. Works correctly. [VERIFIED: mcp.go lines 78-81]

---

## BKINT-01: Boundary Analysis and Recommendation

### Recommendation: In-place evolution, not a new package

**Verdict:** Evolve `internal/scan/scanner.go` in place. Do NOT create `internal/inventory/`.

**Rationale:**
1. The injectable-var seam already provides 100% of the required properties: replaceability (one-file change to switch back to bumblebee) and testability (mock without binary spawn).
2. `internal/inventory/` as a new package would require: a new Go file, a new interface type, a wrapper function calling `scan.Scan`, and updates to every caller of `scan.Scan`. Current callers: `cmd/beekeeper/main.go` line 438 (`scanOnDeltaFn`) and `cmd/beekeeper/main.go` scan command. That refactor adds risk with no test benefit.
3. Phase-3 RESEARCH explicitly stated: "Co-locating the round-trip test with `internal/scan/` (rather than creating a new `internal/inventory/`) avoids prematurely building the Phase-4 BKINT-01 boundary." Phase 4 RESEARCH confirms the same conclusion: the round-trip already works, the rename is the only needed change.
4. BKINT-02 (Phase 5) pins the pollen version in `go.mod` and turns on full CI — that is the appropriate time to add formal module-level separation if needed.

**Interface shape (for reference, if planner wants to future-proof):**

If the planner prefers the new-package approach despite the recommendation, the minimal clean interface is:

```go
// internal/inventory/inventory.go
package inventory

import (
    "context"
    "io"
)

// Runner is the mockable boundary for the pollen subprocess.
type Runner interface {
    // Run invokes `pollen scan` (or equivalent), writes NDJSON lines to out.
    Run(ctx context.Context, deep bool, out io.Writer) error
}
```

But this is NOT recommended for Phase 4.

### scanOnDeltaFn vs runPollenFn — not the same

[VERIFIED: main.go line 50, scanner.go line 60]

- `scanOnDeltaFn` (main.go) wraps `scan.Scan(ctx, cfg, out)` — the outer orchestrator
- `runPollenFn` (scanner.go, after rename) is the inner subprocess runner within `scan.Scan`

These are two different injection points. BKINT-01 only renames `runBumblebeeFn` → `runPollenFn` inside `scanner.go`. The `scanOnDeltaFn` in `main.go` is unaffected.

---

## PTEST-04: Compat Test Analysis

### Current state of Windows skip baseline

[VERIFIED: `internal/scan/scanner_test.go` and `cmd/beekeeper/main_test.go`]

**Existing tests:** `TestScanWithBumblebee`, `TestScanWindowsShapedRecord`, `TestScanBumblebeeUnavailable` — none use `t.Skip`.

**`TestScanWindowsShapedRecord`** (added in Phase 3): already feeds a Windows-shaped Pollen record through the `Scan` function. This test is fixture-driven and runs on all 3 OSes.

**What's missing for PTEST-04:**
1. No test currently asserts `scanner_name="pollen"` specifically (they assert record type passes through, not scanner attribution).
2. No test feeds editor-extension or browser-extension or MCP Windows-shaped records.
3. No test asserts "no double-counting" across the full record set.

**New `TestPollenCompatibility`** covers all three gaps and explicitly verifies the Windows-shaped records for all three new WEXT types pass through without `scan_error`.

### What "zero t.Skip" means concretely

The PTEST-04 success criterion is: after Phase 4, `go test ./internal/scan/...` on `windows-latest` CI runner shows 0 skipped tests. The existing tests already have no skips. The new `TestPollenCompatibility` must also have no Windows skip — this is guaranteed by the fixture-driven design (no binary spawn, no OS-specific file system access).

**MCP fuzz-test open question:** STATE.md flags "MCP message parser must be fuzz-tested before v0.6.0 as a release gate." This refers to the beekeeper MCP gateway fuzz tests (Phase 4 of v1.0.0, not this Phase 4). It does NOT apply to `mcp.go`'s config parser in Pollen. The Pollen MCP config parser has no network input surface — it only reads files from the local filesystem whose paths are operator-controlled. No fuzz gate applies here. [ASSUMED from context analysis — verify with project owner if in doubt]

---

## Common Pitfalls

### Pitfall 1: VSCodium `.vscodium` vs `.vscode-oss` segment mismatch

**What goes wrong:** Plan uses only `.vscodium/extensions` (live code) but PRD says `.vscode-oss\extensions\` (locked decision). Phase 4 fixture for VSCodium is created at `.vscode-oss` but the scanner's `extensionRootSegments` doesn't contain `.vscode-oss/extensions`, so the fixture is never matched and the test fails.

**Why it happens:** PRD §8.2 and the live code diverge; PRD is locked.

**How to avoid:** Add `.vscode-oss/extensions` to `extensionRootSegments` AND to `baselineHomeCandidates`. Keep `.vscodium/extensions` for upstream parity. Both segments are correct for different VSCodium installs.

**Warning signs:** Fixture test creates a `.vscode-oss/extensions` directory but `TestWindowsExtensionMCPRoots` shows zero records for VSCodium.

### Pitfall 2: Windsurf MCP path missing from cross-platform list

**What goes wrong:** WEXT-03 requires `%USERPROFILE%\.windsurf\mcp.json`. The cross-platform `baselineHomeCandidates` adds `.codeium/windsurf` (Windsurf extension IDE), NOT `.windsurf` (Windsurf dotfile). These are different directories. `.windsurf` is not in the current list.

**Why it happens:** The `baselineHomeCandidates` addition at line 265 targets the Windsurf IDE's extension root (`.codeium/windsurf`), not its config root (`.windsurf`).

**How to avoid:** Add `add(filepath.Join(home, ".windsurf"), model.RootKindMCPConfig)` to the cross-platform MCP section (before the `switch runtime.GOOS` block, so all platforms get it). Then `mcp.json` inside that directory is found by `IsKnownMCPConfig`. [VERIFIED: roots.go lines 264-267 — `.windsurf` MCP root is absent]

### Pitfall 3: APPDATA-based paths require env-var guard in Windows case

**What goes wrong:** `case "windows":` block in `baselineHomeCandidates` tries `filepath.Join(os.Getenv("APPDATA"), "Claude")` without guarding for empty APPDATA. On a machine with unset APPDATA, this produces a relative `Claude\` path, which the scanner walks from CWD.

**Why it happens:** Inconsistent application of Phase 2's env-var guard discipline.

**How to avoid:** Always use `if appdata := os.Getenv("APPDATA"); appdata != "" { ... }`. [VERIFIED: roots_windows.go uses this pattern consistently]

### Pitfall 4: `browserExtensionCandidateRoots` uses `home` but Windows Chromium uses LOCALAPPDATA

**What goes wrong:** `browserExtensionCandidateRoots(home string)` receives `home` as `os.UserHomeDir()`. On Windows, Chrome's User Data is at `%LOCALAPPDATA%\Google\Chrome\...`, NOT `home\Google\Chrome\...`. Using `filepath.Join(home, "Local", ...)` would produce the wrong path.

**Why it happens:** The function signature passes `home` but Windows browser paths use `LOCALAPPDATA` which is a different env var.

**How to avoid:** In the `case "windows":` block, call `os.Getenv("LOCALAPPDATA")` directly — do NOT use the `home` parameter. [VERIFIED: roots_windows.go does exactly this for package roots]

### Pitfall 5: Firefox Profiles directory vs per-profile directory

**What goes wrong:** For Firefox, the existing darwin/linux cases add the `Profiles` parent directory (e.g. `Firefox/Profiles`), NOT per-profile directories. The walker then enters each profile subdirectory and finds `extensions.json`. On Windows, the equivalent is `%APPDATA%\Mozilla\Firefox\Profiles` (not `%APPDATA%\Mozilla\Firefox\Profiles\<profile>`).

**Why it happens:** Confusing the root-level discovery (add the Profiles parent) with the file-level detection (`IsFirefoxExtensionsJSON` matches the per-profile file).

**How to avoid:** Add `filepath.Join(appdata, "Mozilla", "Firefox", "Profiles")` (the parent), NOT any per-profile path. The walker recurses into each `<hash>.profile/` subdirectory automatically.

### Pitfall 6: Double-counting in PTEST-04 assertion

**What goes wrong:** `beekeeper scan` emits both Pollen's records (passthrough) AND beekeeper-own `finding` records for extensions. If `TestPollenCompatibility` asserts `strings.Count(out, scanner_name=pollen) == N` without excluding beekeeper's own records, it may be too strict.

**Why it happens:** `Scan` passes through Pollen's lines first, then runs `beekeeperScan` which emits `scanner_name=beekeeper` records. The two scanners don't share record IDs.

**How to avoid:** Double-counting assertion should check source_file uniqueness for Pollen-origin records only (as shown in the Pattern 4 code example). The `beekeeperScan` won't run if `cfg.ExtensionDirs` is empty — pass `Config{}` (no ExtensionDirs) in the compat test to avoid beekeeper-own scan output entirely.

### Pitfall 7: `t.Setenv` for APPDATA/LOCALAPPDATA requires both in test isolation

**What goes wrong:** A Windows fixture test sets only `USERPROFILE` but forgets `APPDATA` or `LOCALAPPDATA`. The test unexpectedly hits the real `%APPDATA%` directory, finds real installed software, and produces non-deterministic results.

**Why it happens:** Phase 2 decision "use t.Setenv for USERPROFILE/APPDATA/LOCALAPPDATA" — all three must be set.

**How to avoid:** Set ALL four env vars in every Windows fixture test: `USERPROFILE`, `APPDATA`, `LOCALAPPDATA`, `ProgramFiles`. [VERIFIED: roots_windows_test.go does this]

---

## Code Examples

### Fill `browserExtensionCandidateRoots` Windows case (roots.go)

```go
// Source: VERIFIED against ../pollen/cmd/pollen/roots.go (lines 528-597)
// Pattern: same as darwin/linux cases above; use os.Getenv not home param
// Chromium section:
case "windows":
    if localappdata := os.Getenv("LOCALAPPDATA"); localappdata != "" {
        chromiumBases["chrome"] = []string{
            filepath.Join(localappdata, "Google", "Chrome", "User Data"),
        }
        chromiumBases["chromium"] = []string{
            filepath.Join(localappdata, "Chromium", "User Data"),
        }
        chromiumBases["edge"] = []string{
            filepath.Join(localappdata, "Microsoft", "Edge", "User Data"),
        }
        chromiumBases["brave"] = []string{
            filepath.Join(localappdata, "BraveSoftware", "Brave-Browser", "User Data"),
        }
    }

// Firefox section:
case "windows":
    if appdata := os.Getenv("APPDATA"); appdata != "" {
        roots = append(roots, filepath.Join(appdata, "Mozilla", "Firefox", "Profiles"))
    }
```

### Add Windows MCP entries (roots.go `baselineHomeCandidates`)

```go
// Source: VERIFIED — extends existing switch at lines 269-276
// Cross-platform additions (before the switch, applies to Windows too):
add(filepath.Join(home, ".windsurf"), model.RootKindMCPConfig)  // MISSING — add this line

// In the switch:
case "windows":
    if appdata := os.Getenv("APPDATA"); appdata != "" {
        add(filepath.Join(appdata, "Claude"), model.RootKindMCPConfig)
        add(filepath.Join(appdata, "cline"), model.RootKindMCPConfig)
    }
```

### BKINT-01 rename (scanner.go)

```go
// Source: VERIFIED against beekeeper/internal/scan/scanner.go
// Replace these three declarations:
var lookPollenFn = func() (string, error) { return exec.LookPath("pollen") }

var runPollenFn = func(ctx context.Context, deep bool) (<-chan []byte, bool) {
    return defaultRunPollen(ctx, deep)
}

func defaultRunPollen(ctx context.Context, deep bool) (<-chan []byte, bool) {
    bin, err := lookPollenFn()
    if err != nil {
        return nil, false
    }
    args := []string{"scan"}
    if deep {
        args = append(args, "--profile", "deep")
    }
    cmd := exec.CommandContext(ctx, bin, args...)
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return nil, false
    }
    if err := cmd.Start(); err != nil {
        return nil, false
    }
    ch := make(chan []byte, 64)
    go func() {
        defer close(ch)
        sc := bufio.NewScanner(stdout)
        for sc.Scan() {
            line := sc.Bytes()
            out := make([]byte, len(line))
            copy(out, line)
            ch <- out
        }
        _ = cmd.Wait()
    }()
    return ch, true
}
// Also update the Scan function body to use runPollenFn and update record text:
// "pollen_unavailable": true (was "bumblebee_unavailable")
// "source": "pollen" (was "bumblebee")
```

### Test fixture isolation (roots_windows_test.go)

```go
// Source: VERIFIED pattern from TestWindowsBaselineRoots in roots_windows_test.go
// mustMkdir helper (already defined in the test file — do NOT redefine):
mustMkdir(t, filepath.Join(localappdata, "Google", "Chrome", "User Data", "Default", "Extensions"))
// t.Setenv pattern:
t.Setenv("USERPROFILE", tmp)
t.Setenv("APPDATA", filepath.Join(tmp, "AppData", "Roaming"))
t.Setenv("LOCALAPPDATA", filepath.Join(tmp, "AppData", "Local"))
t.Setenv("ProgramFiles", filepath.Join(tmp, "ProgramFiles"))
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `bumblebee` subprocess invocation | `pollen` subprocess invocation (BKINT-01) | Phase 4 | beekeeper now exclusively invokes the Windows-compatible fork |
| Empty `case "windows":` skeletons in `browserExtensionCandidateRoots` | Filled with LOCALAPPDATA/APPDATA paths (WEXT-02) | Phase 4 | Windows browser extension roots enumerated |
| No Windows MCP APPDATA paths | Added `%APPDATA%\Claude` + `%APPDATA%\cline` (WEXT-03) | Phase 4 | Windows Claude Desktop and Cline MCP configs discovered |
| `"bumblebee_unavailable": true` in scan_status | `"pollen_unavailable": true` | Phase 4 | Status record reflects current binary name |

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Cursor Windows extension directory is `%USERPROFILE%\.cursor\extensions\` (carried from Phase 3 as unvalidated) | WEXT-01 locus analysis | Medium: the existing `extensionRootSegments` list has `.cursor/extensions` which via `filepath.FromSlash` produces `.cursor\extensions` on Windows; if real Cursor installs use a different path the fixture will pass but live detection fails — validated when Phase 4 test runs on real Windows CI |
| A2 | `.windsurf` (MCP config) and `.codeium/windsurf` (Windsurf IDE extension root) are separate directories on Windows | WEXT-03 locus analysis | Low: Windsurf is a VS Code fork; its MCP config lives at `~/.windsurf/mcp.json` (same pattern as Cursor's `~/.cursor/mcp.json`); independently confirmed by PRD §8.4 |
| A3 | MCP fuzz test gate in STATE.md "Phase 4 blocker" refers to beekeeper's gateway MCP message parser, not to Pollen's MCP config file parser | PTEST-04 analysis | Low: the gateway MCP parser (HTTP/JSON-RPC) is a different code path; Pollen's config parser reads local files, not network input |
| A4 | `os.UserHomeDir()` on Windows returns `%USERPROFILE%` (not `%HOMEDRIVE%%HOMEPATH%` or other variants) | MCP/editor root construction | Low: documented Go behavior; returns `%USERPROFILE%` on Windows per stdlib docs |
| A5 | VSCodium on Windows installs extensions at BOTH `.vscodium\extensions\` and `.vscode-oss\extensions\` depending on install variant | VSCodium path gap | Low: adding both to the segments list is safe (only existing dirs are enumerated); adds a small redundancy if only one variant is present |

---

## Open Questions (RESOLVED)

> All three resolved in planning — answers embedded below and reflected in 04-01/04-02 task actions.

1. **VSCodium `.vscodium` vs `.vscode-oss` — which does Phase 4 add?**
   - What we know: PRD §8.2 says `.vscode-oss\extensions\`. Live code has `.vscodium/extensions`.
   - What's unclear: Whether to add `.vscode-oss/extensions` to both `extensionRootSegments` AND `baselineHomeCandidates`, or replace `.vscodium`, or add both.
   - Recommendation: Add `.vscode-oss/extensions` to both lists (keeping `.vscodium` for upstream compatibility). The fixture should create BOTH directories to verify both paths are found.

2. **`parity_test.go` extension coverage — should Phase 4 add `editor-extension` and `browser-extension` to `assertParityRecordCoverage`?**
   - What we know: `assertParityRecordCoverage` currently asserts 5 ecosystems: npm, pypi, go, rubygems, packagist. The parity fixture uses `testdata/parity-fixture/` which has package-manager fixtures only.
   - What's unclear: Should Phase 4 extend `assertParityRecordCoverage` to also require `editor-extension`, `browser-extension`, and `mcp` records? That would require adding extension/MCP fixtures to `testdata/parity-fixture/`.
   - Recommendation: Add a SEPARATE `testdata/extension-mcp-fixture/` used by a new Windows-only test (not extending the locked parity fixture). Keep `assertParityRecordCoverage` unchanged to avoid touching the locked `normalize_diff.go` path.

3. **`scanOnDeltaFn` string reference to `bumblebee` in main.go — update?**
   - What we know: `scan_status` record uses `"bumblebee_unavailable": true` in `scanner.go`. The `main.go` `scanOnDeltaFn` simply calls `scan.Scan` — it has no hardcoded `bumblebee` string.
   - What's unclear: Are there other places in beekeeper that hardcode `"bumblebee"` as a string?
   - Action: Grep `beekeeper` for `"bumblebee"` string literals before committing BKINT-01.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Build + test | ✓ | 1.25 (per go.mod) | — |
| Windows dev machine | WEXT-01/02/03 fixture tests | ✓ | Windows 11 | CI `windows-latest` |
| Linux/macOS CI | Differential test guard, non-Windows paths | ✓ (CI only) | ubuntu-latest, macos-latest | — |
| `pollen` binary in PATH | PTEST-04 integration test | ✗ (dev machine) | — | Fixture-driven test uses `runPollenFn` injection — no binary needed |

**No blocking missing dependencies.** The compat test is explicitly designed fixture-driven (no binary spawn) so it runs without `pollen` in PATH.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing stdlib (`go test`) |
| Config file | none (go.mod only) |
| Quick run command (Pollen, Windows) | `go test ./cmd/pollen/ -run TestWindowsExtensionMCPRoots` |
| Quick run command (Beekeeper, Windows) | `go test ./internal/scan/ -run TestPollenCompatibility` |
| Full suite command (Pollen) | `cd ../pollen && go test ./...` |
| Full suite command (Beekeeper) | `go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| WEXT-01 | `resolveRoots(baseline)` returns VS Code/Cursor/Windsurf/Insiders/VSCodium extension roots on Windows | unit (windows) | `go test ./cmd/pollen/ -run TestWindowsExtensionMCPRoots` | ❌ Wave 0 (new in roots_windows_test.go) |
| WEXT-02 | `resolveRoots(baseline)` returns Chrome/Chromium/Edge/Brave/Firefox roots per-profile on Windows | unit (windows) | `go test ./cmd/pollen/ -run TestWindowsExtensionMCPRoots` | ❌ Wave 0 (same test) |
| WEXT-03 | `resolveRoots(baseline)` returns Claude Desktop, Cursor, Windsurf, Cline, Gemini MCP roots on Windows | unit (windows) | `go test ./cmd/pollen/ -run TestWindowsExtensionMCPRoots` | ❌ Wave 0 (same test) |
| WEXT-01 | Pollen scanner emits editor-extension records for Windows fixtures (end-to-end) | integration (windows) | `go test ./cmd/pollen/ -run TestParityAllEcosystems` (extend) or new test | ❌ Wave 0 (parity extension or new) |
| WEXT-02 | Pollen scanner emits browser-extension records for Windows fixtures | integration (windows) | `go test ./cmd/pollen/ -run TestParityAllEcosystems` (extend) | ❌ Wave 0 |
| WEXT-03 | Pollen scanner emits mcp records for Windows fixtures | integration (windows) | `go test ./cmd/pollen/ -run TestParityAllEcosystems` (extend) | ❌ Wave 0 |
| WEXT-01/02/03 | Linux/macOS paths unaffected (regression) | regression | `go test ./cmd/pollen/ -run TestDifferential` (CI) + `go test ./cmd/pollen/ -run TestParityAllEcosystems` | ✅ (existing tests) |
| BKINT-01 | `bumblebee` → `pollen` rename compiles; existing tests pass | unit | `go test ./internal/scan/` | ✅ (update existing 3 tests) |
| BKINT-01 | `pollen_unavailable` status emitted when pollen not in PATH | unit | `go test ./internal/scan/ -run TestScanPollenUnavailable` | ❌ Wave 0 (rename TestScanBumblebeeUnavailable or add variant) |
| PTEST-04 | Pollen compatibility test: passes all 5 record types through, asserts scanner_name, no double-counting | integration | `go test ./internal/scan/ -run TestPollenCompatibility` | ❌ Wave 0 (new test) |
| PTEST-04 | Zero t.Skip in inventory tests on Windows | verification | `go test ./internal/scan/ -v 2>&1 \| Select-String SKIP` should be empty | N/A (achieved by fixture-driven design) |

### Sampling Rate

- **Per task commit (Pollen):** `cd ../pollen && go test ./cmd/pollen/ -run TestWindowsExtensionMCPRoots && go vet ./...`
- **Per task commit (Beekeeper):** `go test ./internal/scan/ && go vet ./internal/scan/`
- **Per wave merge:** `cd ../pollen && go test ./...` + `go test ./...` in beekeeper
- **Phase gate:** Full suite green in both repos before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `../pollen/cmd/pollen/roots_windows_test.go` — add `TestWindowsExtensionMCPRoots` covering WEXT-01/02/03 root discovery assertions (browser + MCP + editor)
- [ ] `../pollen/cmd/pollen/testdata/extension-mcp-fixture/` (optional) — fake extension/MCP files for an extended parity test; or extend `testdata/parity-fixture/` with Windows-specific sub-trees
- [ ] `beekeeper/internal/scan/scanner_test.go` — add `TestPollenCompatibility` (PTEST-04); update existing `TestScanBumblebeeUnavailable` → `TestScanPollenUnavailable` (or keep name but check new status string)
- [ ] `beekeeper/internal/scan/scanner_test.go` — update `runBumblebeeFn` references to `runPollenFn` after BKINT-01 rename

*(Framework already present — both repos use stdlib `testing` only)*

---

## Security Domain

`security_enforcement` is not explicitly `false` in `.planning/config.json`. Phase 4 adds OS path enumeration (read-only) and a subprocess rename. No new attack surface beyond Phase 2/3.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | partial | MCP config JSON parsing is already guarded by `json.Unmarshal` + `sanitizeRemoteURL`; no new validation surface introduced |
| V6 Cryptography | no | — |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Windows junction point under `%LOCALAPPDATA%\Google\Chrome\User Data\` redirecting to sensitive areas | Information Disclosure | Same as Phase 2 T-02-02 (accepted): bounded risk — at worst double-counting; `filterExistingRoots` uses `os.Stat` (follows junctions to verify is-dir); Phase 2 open flag carried forward |
| Unset `LOCALAPPDATA`/`APPDATA` producing relative path via `filepath.Join("", ...)` | Elevation of Privilege | Mitigated by env-var guard pattern (`if appdata := os.Getenv(...); appdata != ""`) established in Phase 2 |
| `pollen` binary name collision in PATH (attacker plants fake `pollen` binary) | Spoofing | Same risk as `bumblebee` — user's PATH is controlled by the user; no new surface introduced by the rename |

---

## Project Constraints (from CLAUDE.md)

| Directive | Impact on Phase 4 |
|-----------|-----------------|
| Go 1.25+, single static binary, no CGO | No new CGO deps; all code is pure Go stdlib |
| `internal/` for all business logic | Pollen code stays in `internal/ecosystem/` and `cmd/pollen/` (package main); beekeeper code stays in `internal/scan/` |
| `fail_open` requires explicit opt-in | `defaultRunPollen` returns `(nil, false)` on failure — beekeeper falls back gracefully (existing behavior preserved) |
| Native Windows path discipline | All new `filepath.Join` constructions use env vars, never hardcoded separators |
| Reproducible builds required | No build-affecting changes; VERSION bump is metadata only |
| `go test -race` CI-only | Phase 4 tests written to pass without `-race` (no new goroutines introduced beyond existing `defaultRunPollen` goroutine) |
| Build-tagged `_windows.go` pattern | New `roots_windows_test.go` additions use `//go:build windows` tag; scanner code additions use `os.Getenv` guards inside `case "windows":` blocks (no new build-tagged non-test files) |

---

## Sources

### Primary (HIGH confidence)
- `C:\Users\Bantu\mzansi-agentive\pollen\cmd\pollen\roots.go` — confirmed empty Windows skeletons in `browserExtensionCandidateRoots`, existing MCP switch, editor-extension segment loop
- `C:\Users\Bantu\mzansi-agentive\pollen\cmd\pollen\roots_windows.go` — confirmed Phase 2 env-var guard pattern
- `C:\Users\Bantu\mzansi-agentive\pollen\cmd\pollen\roots_windows_test.go` — confirmed test fixture pattern
- `C:\Users\Bantu\mzansi-agentive\pollen\internal\ecosystem\editorext\editorext.go` — confirmed `extensionRootSegments`, `hostFromExtRoot`, `IsExtensionPackageJSON` mechanics
- `C:\Users\Bantu\mzansi-agentive\pollen\internal\ecosystem\browserext\browserext.go` — confirmed `IsFirefoxExtensionsJSON` filepath.ToSlash approach, `IsChromiumExtensionManifest`
- `C:\Users\Bantu\mzansi-agentive\pollen\internal\ecosystem\mcp\mcp.go` — confirmed `IsKnownMCPConfig`, `IsGeminiSettingsJSON`, `ScanConfig`
- `C:\Users\Bantu\mzansi-agentive\beekeeper\internal\scan\scanner.go` — confirmed `lookBumblebee`, `runBumblebeeFn`, `defaultRunBumblebee` injection pattern
- `C:\Users\Bantu\mzansi-agentive\beekeeper\internal\scan\scanner_test.go` — confirmed existing tests, `runBumblebeeFn` injection, `TestScanWindowsShapedRecord`
- `C:\Users\Bantu\mzansi-agentive\pollen\cmd\pollen\parity_test.go` — confirmed `assertWindowsPathShape`, `assertWindowsEndpointUID` already present; `assertParityRecordCoverage` asserts 5 ecosystems
- `C:\Users\Bantu\mzansi-agentive\beekeeper\cmd\beekeeper\main.go` — confirmed `scanOnDeltaFn` is separate from `runBumblebeeFn`

### Secondary (MEDIUM confidence)
- `C:\Users\Bantu\mzansi-agentive\beekeeper\.planning\phases\03-windows-path-representation\03-RESEARCH.md` — confirmed locus corrections, injectable-var patterns, `internal/inventory/` deferral recommendation
- `C:\Users\Bantu\mzansi-agentive\beekeeper\.planning\STATE.md` — confirmed `scanOnDeltaFn` decision from Phase 11, accumulated env-var guard decisions

### Tertiary (LOW confidence)
- A1–A5 in Assumptions Log — training knowledge claims, marked accordingly

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — pure stdlib; no external dependencies; verified against live go.mod
- Architecture: HIGH — all locus claims verified by direct source code inspection
- Pitfalls: HIGH — every pitfall has a specific line-number reference or verified code path
- BKINT-01 recommendation: HIGH — verified full call graph in scanner.go and main.go
- VSCodium path gap: HIGH — discrepancy verified in editorext.go vs PRD §8.2

**Research date:** 2026-06-02
**Valid until:** 2026-07-02 (stable; no external dependency churn; pollen is a controlled fork)
