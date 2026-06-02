---
phase: 03-windows-path-representation
reviewed: 2026-06-02T00:00:00Z
depth: standard
files_reviewed: 8
files_reviewed_list:
  - ../pollen/internal/ecosystem/npm/npm.go
  - ../pollen/internal/ecosystem/npm/npm_test.go
  - ../pollen/internal/ecosystem/pnpm/pnpm.go
  - ../pollen/internal/ecosystem/pnpm/pnpm_test.go
  - ../pollen/internal/endpoint/endpoint.go
  - ../pollen/internal/endpoint/endpoint_test.go
  - ../pollen/cmd/pollen/parity_test.go
  - internal/scan/scanner_test.go
findings:
  critical: 0
  warning: 2
  info: 4
  total: 6
status: issues_found
---

# Phase 3: Code Review Report

**Reviewed:** 2026-06-02
**Depth:** standard
**Files Reviewed:** 8
**Status:** issues_found

## Summary

Phase 3 ("Windows Path Representation") makes three small, surgical changes plus
supporting tests:

- **WPATH-01** — `npm.go` (`IsNodeModulesPackageJSON`, line 114) and `pnpm.go`
  (`IsPnpmStorePackageJSON`, line 87) wrap the `strings.Join(parts, "/")`
  projectPath reconstruction in `filepath.FromSlash`. Verified correct: on
  Linux/macOS the OS separator is `/`, so `filepath.FromSlash` is a literal
  no-op (replaces `/` with `/`), preserving byte-identical Unix output and
  keeping the upstream differential test (PTEST-02) byte-stable. On Windows it
  converts the slash-joined string back to native `\`. The `ToSlash`→split→
  `Join("/")`→`FromSlash` round-trip on the two reviewed code paths is sound and
  does not regress Unix behavior.

- **WPATH-02** — `endpoint.go` guards **both** `ep.UID` assignments with
  `runtime.GOOS != "windows"`: the `user.Current()` success path (line 29) and
  the `os.Getuid()` fallback (line 34). Verified correct: on Windows `ep.UID`
  is left at its zero value `""`; on Unix it remains the numeric UID. Both
  paths are guarded — no asymmetry.

- The schema (`model.Endpoint`, `model.Record`) is unchanged, satisfying the
  "no model.Endpoint edits" constraint.

No correctness regressions to Unix behavior were found in the WPATH changes
themselves, and no security issues were introduced. The findings below concern
test-coverage gaps and minor quality/consistency items. The two warnings are
about tests that assert less than they appear to, which weakens the safety net
guarding these fixes — relevant given the project's "fail closed" and
cross-platform-parity posture.

## Warnings

### WR-01: WPATH-01 npm `FromSlash` path is not exercised by any executing test

**File:** `../pollen/internal/ecosystem/npm/npm.go:114` (covered by `../pollen/internal/ecosystem/npm/npm_test.go:229-260` and `../pollen/cmd/pollen/parity_test.go:127-150`)
**Issue:** The actual line changed for WPATH-01 in npm.go is the
`filepath.FromSlash(strings.Join(parts[:nmIdx], "/"))` reconstruction inside
`IsNodeModulesPackageJSON`. The only tests that assert its native-Windows output
(`TestIsNodeModulesPackageJSONWindowsPath`, `TestIsNodeModulesPackageJSONScopedWindowsPath`)
are gated behind `if runtime.GOOS != "windows" { t.Skip(...) }`, so they do not
run in the primary Linux/macOS CI lanes. The parity test's
`assertWindowsPathShape` is likewise Windows-only AND the committed
`testdata/parity-fixture/npm-fixture/` contains only `package-lock.json` with no
`node_modules/` tree — so `IsNodeModulesPackageJSON` is never invoked by the
parity scan. Net effect: on the developer's primary machine (Windows) the
Windows-gated unit tests do run, but in the Linux/macOS CI matrix the
`FromSlash` branch on this function has zero behavioral assertions. A future
edit that broke the slash round-trip on Windows would only be caught if the
Windows CI lane is green, with no Unix-side guard that the reconstruction is at
least a no-op.
**Fix:** Add an unconditional (non-skipped) unit test that asserts the
`ToSlash → Join → FromSlash` round-trip is identity on the running platform for
a Unix-style input, e.g.:
```go
func TestIsNodeModulesPackageJSONFromSlashNoopOnUnix(t *testing.T) {
    if runtime.GOOS == "windows" {
        t.Skip("Unix no-op guard; Windows shape covered elsewhere")
    }
    ok, proj := IsNodeModulesPackageJSON("/home/u/proj/node_modules/foo/package.json")
    if !ok || proj != "/home/u/proj" {
        t.Fatalf("got ok=%v proj=%q, want true /home/u/proj", ok, proj)
    }
}
```
Alternatively, add a `node_modules/<pkg>/package.json` entry to the npm parity
fixture so `assertWindowsPathShape` exercises the function on the Windows runner.

### WR-02: `assertWindowsPathShape` does not fail when zero path fields are present (vacuous pass risk)

**File:** `../pollen/cmd/pollen/parity_test.go:127-150`
**Issue:** `assertWindowsPathShape` iterates records and only asserts on fields
that are present and non-empty/non-`"."`. If a regression (or fixture change)
caused `project_path` / `source_file` to be emitted empty, or caused the npm/pnpm
detectors to emit no records at all on the Windows lane, the function would
iterate, find nothing to check, and pass silently — reporting "Windows path
shape" as verified when it verified nothing. The sibling `assertEndpointOS`
(line 97) has the same vacuous-pass shape, and `assertParityRecordCoverage`
(line 183) only guards *ecosystem* coverage, not the presence of any
path-bearing record. The drive-letter check `v[1] != ':'` is also UNC-hostile:
a valid Windows UNC path (`\\server\share\...`) has no drive letter and would be
falsely flagged — acceptable for the fixture (always drive-rooted) but a latent
correctness trap if the fixture ever moves to a UNC root.
**Fix:** Track and assert a minimum count of path-bearing records checked:
```go
checked := 0
// ... inside the loop, after a successful drive-letter assertion:
checked++
// ... after the loop:
if checked == 0 {
    t.Errorf("assertWindowsPathShape: no project_path/source_file fields found — fixture produced no path-bearing records (vacuous pass)")
}
```
Optionally relax the drive-letter assertion to also accept a `\\` UNC prefix if
UNC roots are ever in scope.

## Info

### IN-01: Stale comment label "v5Section" does not match variable name

**File:** `../pollen/internal/ecosystem/pnpm/pnpm.go:384-387`
**Issue:** The comment block reads `// v5Section is non-empty when we're inside
a top-level ... block`, but the variable it documents is `inV5DepSection` (a
`bool`), not a string named `v5Section`. "non-empty" is also wrong wording for a
boolean. This is a doc/code drift that will mislead the next reader.
**Fix:** Update the comment to: `// inV5DepSection is true when we're inside a
top-level dependencies/devDependencies/optionalDependencies/peerDependencies
block (v5 layout); entries appear at indent 2 as 'name: version'.`

### IN-02: `name` return value computed but never trusted in `IsPnpmStorePackageJSON`

**File:** `../pollen/internal/ecosystem/pnpm/pnpm.go:81-86`
**Issue:** `splitPnpmStoreDir(storeDir)` returns `name2`, which is immediately
discarded via `_ = name2`. The comment says "Cross-check name parity, but trust
the on-disk directory name" — but no cross-check is actually performed; `name2`
is simply dropped. This is dead computation dressed up as a deliberate decision.
If the intent is a defensive parity check (store-dir name vs. on-disk
`node_modules/<name>` segment), it is missing; if not, the call and the
`_ = name2` line are noise.
**Fix:** Either perform the documented cross-check and emit a diagnostic on
mismatch, or drop `name2` from the destructuring and call a version-only helper.
At minimum, change the comment to state that the store-dir-derived name is
intentionally unused (only the version is extracted from `storeDir`).

### IN-03: pnpm `version` only assigned when non-empty leaves no diagnostic on parse failure

**File:** `../pollen/internal/ecosystem/pnpm/pnpm.go:82-84`
**Issue:** `if ver != "" { version = ver }` silently leaves `version` empty when
`splitPnpmStoreDir` cannot parse a version out of the store-dir name (e.g. an
unexpected store-dir format). The function then still returns `ok=true` with an
empty `version`, and the downstream caller's behavior on an empty-version store
record is not obvious from this site. Other parse-drift cases in this package
(see `parsePnpmPackages`, line 276) emit a one-shot `diag` warning so silent
drift is visible to operators; this path does not.
**Fix:** This is a minor robustness gap, not a bug for well-formed pnpm stores.
Consider returning `ok=false` (skip the record) when `version == ""`, mirroring
how empty `name`/`version` entries are skipped in `ScanLockfile` (line 168), so
a malformed store dir does not produce a versionless record.

### IN-04: Windows-shaped passthrough test does not assert byte-exact path preservation

**File:** `internal/scan/scanner_test.go:87-90`
**Issue:** `TestScanWindowsShapedRecord` asserts the output merely *contains*
`C:\\` (line 88) to prove the Windows drive+backslash path survived the JSON
round-trip. Because `Scan`'s passthrough writes the raw line verbatim
(`scanner.go:128`), this is correct today, but the assertion is weak: it would
still pass if the path were truncated or mangled as long as the substring `C:\\`
appeared somewhere. The test name implies full path fidelity.
**Fix:** Assert the full encoded path substrings to lock fidelity:
```go
if !strings.Contains(out, `"project_path":"C:\\Users\\fana\\code\\web-app"`) {
    t.Errorf("project_path not preserved verbatim: %s", out)
}
if !strings.Contains(out, `"source_file":"C:\\Users\\fana\\code\\web-app\\package-lock.json"`) {
    t.Errorf("source_file not preserved verbatim: %s", out)
}
```

---

_Reviewed: 2026-06-02_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
