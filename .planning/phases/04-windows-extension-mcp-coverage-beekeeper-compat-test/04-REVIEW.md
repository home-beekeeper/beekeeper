---
phase: 04-windows-extension-mcp-coverage-beekeeper-compat-test
reviewed: 2026-06-02T00:00:00Z
depth: standard
files_reviewed: 5
files_reviewed_list:
  - ../pollen/cmd/pollen/roots.go
  - ../pollen/cmd/pollen/roots_windows_test.go
  - ../pollen/internal/ecosystem/editorext/editorext.go
  - internal/scan/scanner.go
  - internal/scan/scanner_test.go
findings:
  critical: 0
  warning: 3
  info: 4
  total: 7
status: issues_found
---

# Phase 4: Code Review Report

**Reviewed:** 2026-06-02T00:00:00Z
**Depth:** standard
**Files Reviewed:** 5
**Status:** issues_found

## Summary

The Phase 4 changes cover Windows extension/MCP root enumeration in Pollen (`roots.go`,
`editorext.go`, `roots_windows_test.go`) and the bumblebee→pollen subprocess seam rename in
Beekeeper (`scanner.go`, `scanner_test.go`). The env-var-guard discipline is correctly applied
throughout the Windows code paths and the fail-closed posture of `defaultRunPollen` is sound.

Three correctness issues were found. The most significant is a wrong `package_manager` label
emitted for `.vscode-oss` extensions in `editorext.go` — `hostFromExtRoot` falls through to
`"vscode"` for the alternate VSCodium path, silently mislabelling every `.vscode-oss` extension
in the output stream. Two warning-level issues concern test coverage gaps. No security
vulnerabilities were found.

---

## Warnings

### WR-01: `hostFromExtRoot` returns `"vscode"` for `.vscode-oss` extensions — wrong `package_manager` label

**File:** `../pollen/internal/ecosystem/editorext/editorext.go:125-137`

**Issue:** `hostFromExtRoot` matches `.vscodium` (line 132) but has no case for `.vscode-oss`,
the alternate VSCodium install path added by WEXT-01. Any extension whose `extRoot` contains
`/.vscode-oss` falls through to `default: return "vscode"`, causing every `.vscode-oss`
extension record to be emitted with `package_manager: "vscode"` instead of `"vscodium"`.
This creates a schema inconsistency downstream: operators querying `package_manager=vscodium`
will miss the alternate-path population entirely.

Note: `.vscodium/extensions` is already in `extensionRootSegments` (line 48) and
`baselineHomeCandidates` (line 253), so records _are_ emitted — they are just mislabelled.

**Fix:**
```go
func hostFromExtRoot(extRoot string) string {
    p := filepath.ToSlash(extRoot)
    switch {
    case strings.Contains(p, "/.cursor"):
        return "cursor"
    case strings.Contains(p, "/.windsurf"):
        return "windsurf"
    case strings.Contains(p, "/.vscodium"):
        return "vscodium"
    case strings.Contains(p, "/.vscode-oss"):   // add this case
        return "vscodium"
    default:
        return "vscode"
    }
}
```

---

### WR-02: `TestWindowsExtensionMCPRoots` does not assert `.vscodium/extensions` — WEXT-01 dual-path coverage is incomplete

**File:** `../pollen/cmd/pollen/roots_windows_test.go:148-206`

**Issue:** The test comment and the `CHANGES.md` entry both state that WEXT-01 requires VSCodium
to be covered via **both** `.vscodium/extensions` (line 253 of `roots.go`) and `.vscode-oss/extensions`
(line 255 of `roots.go`). The test creates and asserts only `.vscode-oss/extensions` (via the
variable `vscodium` at line 152). The `.vscodium/extensions` directory is neither created nor
asserted. If the `.vscodium/extensions` entry were accidentally removed from
`baselineHomeCandidates`, this test would not catch it.

**Fix:** Add `.vscodium/extensions` alongside `.vscode-oss/extensions`:

```go
// WEXT-01: editor-extension roots under USERPROFILE (home).
vsCode          := filepath.Join(tmp, ".vscode", "extensions")
vsCodeInsiders  := filepath.Join(tmp, ".vscode-insiders", "extensions")
cursor          := filepath.Join(tmp, ".cursor", "extensions")
windsurf        := filepath.Join(tmp, ".windsurf", "extensions")
vscodiumAlt     := filepath.Join(tmp, ".vscodium", "extensions")   // add
vscodiumOSS     := filepath.Join(tmp, ".vscode-oss", "extensions")
mustMkdir(vsCode)
mustMkdir(vsCodeInsiders)
mustMkdir(cursor)
mustMkdir(windsurf)
mustMkdir(vscodiumAlt)   // add
mustMkdir(vscodiumOSS)
// ...
assertRoot(vscodiumAlt, model.RootKindEditorExtension)  // add
assertRoot(vscodiumOSS, model.RootKindEditorExtension)
```

---

### WR-03: `evaluateExtension` opens a new `audit.Writer` per extension — file descriptor churn and O_APPEND race under concurrent callers

**File:** `internal/scan/scanner.go:307-310`

**Issue:** `evaluateExtension` calls `audit.NewWriter(cfg.AuditPath)` on every invocation and
closes it immediately after one write (lines 307–310). `audit.NewWriter` calls `os.OpenFile` +
`platform.SetOwnerOnly` on every call. Two problems arise:

1. **File descriptor churn:** A scan over N extensions opens and closes the audit file N times.
   For large extension trees this is measurable I/O overhead and burns file descriptors. The
   beekeeper scan loop (lines 176–194 of `scanner.go`) is currently sequential so FD exhaustion
   is unlikely, but the pattern is fragile.

2. **O_APPEND correctness under concurrent future callers:** `audit.Writer` is documented as
   mutex-protected for concurrent hook-handler calls (`writer.go` comment). That protection
   is bypassed here because each call gets its own `Writer` instance. If `beekeeperScan` is
   ever called concurrently, two `Writer` instances could interleave writes across the
   `os.OpenFile` → `SetOwnerOnly` → `Write` sequence even though each individual write is
   atomic at the kernel level.

**Fix:** Open the writer once before the extension loop and pass it in (or close it in the
loop's deferred cleanup):

```go
func beekeeperScan(ctx context.Context, cfg Config, out io.Writer) error {
    // ...
    var auditW *audit.Writer
    if cfg.AuditPath != "" {
        if w, err := audit.NewWriter(cfg.AuditPath); err == nil {
            auditW = w
            defer auditW.Close()
        }
    }
    for _, dir := range cfg.ExtensionDirs {
        // ... pass auditW to evaluateExtension instead of cfg.AuditPath
    }
}
```

---

## Info

### IN-01: `TestScanWithBumblebee` function name retains the old `bumblebee` identity after the rename

**File:** `internal/scan/scanner_test.go:17`

**Issue:** The test function is named `TestScanWithBumblebee`. The rename task (BKINT-01) updated
`runPollenFn` / `defaultRunPollen` in `scanner.go` but the corresponding test function name was
not updated. This is a stale identifier — it causes confusion when grepping for the old
`bumblebee` name as a completeness check.

**Fix:** Rename to `TestScanWithPollen`.

---

### IN-02: Dead branch in `IsExtensionPackageJSON` — `parentSlash == seg` can never be true in production

**File:** `../pollen/internal/ecosystem/editorext/editorext.go:60`

**Issue:** The condition `parentSlash == seg` compares the absolute path of the extension root
(e.g. `C:\Users\fana\.vscode\extensions` converted to forward slashes) against a bare relative
segment like `.vscode/extensions`. These can only be equal if the scan is rooted at the working
directory and the path is literally `.vscode/extensions` — which cannot happen given
`filterExistingRoots` produces absolute paths from `os.UserHomeDir()` / `os.Getenv`. The
`HasSuffix` branch on the same line covers all real cases. The `== seg` short-circuit is
harmless dead code.

**Fix:** Remove the dead `||` branch to clarify intent:

```go
if strings.HasSuffix(parentSlash, "/"+seg) {
    return true, parent, dir
}
```

---

### IN-03: `TestPollenCompatibility` MCP fixture record is missing `source_file` — double-counting assertion silently skips it

**File:** `internal/scan/scanner_test.go:193-195`

**Issue:** The MCP fixture at line 193 does not include a `source_file` key:

```json
{"record_type":"package","schema_version":"0.1.0","scanner_name":"pollen",
 "ecosystem":"mcp","package_manager":"mcp","source_type":"mcp-config",
 "source_file":"C:\\Users\\fana\\AppData\\Roaming\\Claude\\claude_desktop_config.json"}
```

Wait — the MCP record _does_ include `source_file`. But re-reading the double-counting loop
(lines 228–243): it iterates `fixtures[:4]`, unmarshals each, and then checks
`rec["source_file"].(string)` with `ok && sf != ""`. For the MCP record (index 3), the
`source_file` key is present and non-empty, so it IS checked.

However, the MCP record has no `normalized_name` or `package_name`, only `package_manager` and
`source_type`. The `strings.Count(out, string(sfJSON))` check looks for the JSON-encoded
`source_file` value in the output. Because Beekeeper only passes through lines verbatim (line
122 in `scanner.go`), the source_file value appears exactly once. The double-counting check
passes vacuously — it would fail to detect a bug where Beekeeper _re-emits_ the same record
based on content matching rather than raw pass-through. This is a gap in the assertion's
adversarial power, not an incorrect test.

The real gap is that the MCP fixture record has no `normalized_name` field. If a future Beekeeper
change attempts to parse and re-emit the MCP record, the absence of `normalized_name` would
produce a different output record and the `source_file` count check would still pass. Adding a
`normalized_name` to the MCP fixture would make the double-counting check more meaningful.

**Fix:** Add `normalized_name` to the MCP fixture to make the pass-through assertion more robust:

```go
`{"record_type":"package","schema_version":"0.1.0","scanner_name":"pollen",` +
    `"ecosystem":"mcp","normalized_name":"playwright/mcp","package_manager":"mcp",` +
    `"source_type":"mcp-config",` +
    `"source_file":"C:\\Users\\fana\\AppData\\Roaming\\Claude\\claude_desktop_config.json"}`,
```

---

### IN-04: `browserExtensionCandidateRoots` Windows block generates 10 per-profile paths per browser but only "Default" is tested

**File:** `../pollen/cmd/pollen/roots.go:539` / `../pollen/cmd/pollen/roots_windows_test.go:160-165`

**Issue:** `browserExtensionCandidateRoots` iterates `chromiumProfiles` (10 entries: `Default`,
`Profile 1`…`Profile 9`) for each Chromium browser, generating 40 candidates for the four
Windows browsers (Chrome, Chromium, Edge, Brave). `filterExistingRoots` drops absent
directories, so only paths that exist are returned. The test creates and asserts only the
`Default` profile path for Chrome and Brave (lines 160–165). The test comment says "at least
one Chromium-family Extensions dir" which acknowledges this is intentional minimal coverage.
No functional bug, but the assertion label `// WEXT-02: at least one Chromium-family...` could
mislead a future reader into thinking multi-profile coverage is proven.

**Fix:** Add a `Profile 1` directory and assertion for at least one browser to explicitly
exercise the profile-iteration loop, and update the comment to say "Default and Profile 1"
rather than "at least one":

```go
chromeP1Ext := filepath.Join(localappdata, "Google", "Chrome", "User Data", "Profile 1", "Extensions")
mustMkdir(chromeP1Ext)
// ...
assertRoot(chromeP1Ext, model.RootKindBrowserExtension)
```

---

_Reviewed: 2026-06-02T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
