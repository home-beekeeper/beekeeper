---
phase: 06-corroboration-severity-hardening
reviewed: 2026-06-03T00:00:00Z
depth: deep
files_reviewed: 13
files_reviewed_list:
  - internal/policy/types.go
  - internal/policy/corroboration.go
  - internal/policy/corroboration_test.go
  - internal/policyloader/loader.go
  - internal/policyloader/validate.go
  - internal/policyloader/test.go
  - internal/policyloader/test_test.go
  - internal/check/sanity.go
  - internal/check/handler.go
  - internal/gateway/sanity.go
  - internal/gateway/policy.go
  - internal/watch/sanity.go
  - internal/watch/handler.go
  - internal/scan/sanity.go
  - internal/scan/scanner.go
  - internal/check/handler_test.go
findings:
  critical: 2
  warning: 4
  info: 3
  total: 9
status: criticals_resolved
resolution:
  resolved_commit: e8bdf8a
  critical_resolved: 2
  warnings_deferred: 4
  note: "CR-01 (ANY→ALL wildcard guard) and CR-02 (QuarantineAt recompute + strict validation) fixed with regression tests in commit e8bdf8a. The 4 warnings (resolveCatalogHealthy 4x duplication, no validate-time upper-bound, weak TestCorroborationOneSignedSource, undocumented fail-safe) are lower priority — tracked for a later hardening pass."
---

# Phase 6: Code Review Report

> ✅ **Both CRITICAL findings resolved in commit `e8bdf8a`** (CR-01 escalation-downgrade vector; CR-02 tier-collapse) with regression tests. The 4 WARNINGS remain as tracked lower-priority follow-ups.

**Reviewed:** 2026-06-03
**Depth:** deep
**Files Reviewed:** 13 source files + 3 test files
**Status:** issues_found

## Summary

Phase 6 introduces per-severity corroboration escalation (CORR-01) and a
catalog-sanity gate (CORR-02) through two well-structured changes: new fields on
`CorroborationThresholds` with a pure helper `findSeverityOverride`, and a
`resolveCatalogHealthy` function copied into four caller-tier packages. The
design correctly follows the caller-resolves-IO pattern and does not break the
`internal/policy` purity contract. All four call sites are correctly wired.

Two critical defects require attention before this ships. The most dangerous is a
subtle logic bypass in `findSeverityOverride`: the all-versions guard checks
`m.Version == "*"` against ALL matches, but the severity override is later keyed
off ANY match's `Severity` field — including the unsigned bumblebee match that
carries the severity tag. When the bumblebee entry has `Version == ""` (not `"*"`)
and the OSV entry has `Version == ""`, both guards pass and the escalation fires
correctly. However, if a bumblebee entry carries `Version == "*"` while OSV returns
`Version == ""`, the all-versions guard correctly suppresses escalation — but
there is a second issue: the guard fires on ANY non-dissented match having
`Version == "*"`, so a single wildcard match among otherwise version-specific matches
silently suppresses escalation across the entire evaluation. This is the design
intent, but it is untested for the mixed-version case (one `"*"` match + one
specific-version match from a second source). The second critical issue is that
`validateCorroborationThresholds` accepts a configuration where the global `BlockAt`
is lowered below the override's `QuarantineAt` by a policy file, enabling
quarantine to fire at lower signed-source counts than the override's own `BlockAt`
because `effectiveQuarantineAt` from the override is compared against `signedCount`
independently of the `effectiveBlockAt` guard path.

Four warnings cover: the `QuarantineAt` sticky-zero bug in `ThresholdsFromPolicyFiles`
when `CriticalBlockAt` is set to exactly its previous value, the state.json
`LoadState` missing-file behavior returning a success (no error, nil sources) that
silently makes `resolveCatalogHealthy` return `true` even when the file is
syntactically valid JSON but the Sources map is absent, the `resolveCatalogHealthy`
four-way duplication risk, and the missing upper-bound validation of
`CriticalBlockAt` at load time.

---

## Critical Issues

### CR-01: All-Versions Guard Fires on Any Single Match — Mixed-Version Case Untested and Suppressible by Attacker

**File:** `internal/policy/corroboration.go:77-84`

**Issue:** `findSeverityOverride` iterates ALL non-dissented matches and returns
`nil` the moment ANY single match carries `Version == "*"`. This means an attacker
who can inject a bumblebee entry with `Versions: ["*"]` for a legitimate
critical package also causes the OSV version-specific match (e.g. `Version:
"1.0.0"`) to be ignored for escalation purposes — a single wildcard entry among
a set of specific-version matches suppresses critical escalation for all of them.

The research document (PITFALLS.md, Pitfall 8, REQUIREMENTS.md CORR-02) states
the guard's intent as: a wildcard entry by itself must not trigger single-source
block. The current code goes further: it suppresses escalation for the entire call
even when a non-wildcard match from a second source also matches. This is a
meaningful attack surface if an attacker can corrupt bumblebee to add a
`Versions: ["*"]` entry for a specific-version critical package — escalation is
suppressed catalog-wide for that package, even though the OSV specific-version
match is independently trustworthy.

The correct guard should be: "if ALL non-dissented matches have `Version == "*"`,
do not escalate" — not "if ANY match has `Version == "*"`, do not escalate."

The current `TestCorroborationAllVersionsCriticalWildcardStaysWarn` tests only the
all-wildcard case (both bumblebee and OSV have `Version: "*"`), which passes under
either interpretation. The mixed case is absent.

**Fix:**

Change the guard from "any" to "all":

```go
// CORR-02 all-versions guard: if ALL non-dissented matches have Version == "*",
// do not escalate. A mix of wildcard and specific-version matches still allows
// escalation based on the specific-version matches.
allWildcard := true
for _, m := range matches {
    if m.Dissented {
        continue
    }
    if m.Version != "*" {
        allWildcard = false
        break
    }
}
if allWildcard && len(matches) > 0 {
    return nil
}
```

Then add a test:

```go
// TestCorroborationMixedVersionWildcardAndSpecificEscalates: when one match has
// Version=="*" and another has Version=="1.0.0", escalation must still fire on
// the specific-version match.
func TestCorroborationMixedVersionWildcardAndSpecificEscalates(t *testing.T) {
    matches := []CatalogMatch{
        {CatalogSource: "bumblebee", Severity: "critical", Version: "*",   Signed: false},
        {CatalogSource: "osv",       Severity: "unknown",  Version: "1.0.0", Signed: true},
    }
    thresholds := DefaultCorroborationThresholds()
    level, _, _, _, _ := corroborate(matches, thresholds)
    // OSV provides the signed source; bumblebee provides the severity tag (Version="*"
    // should not suppress when a specific-version match also exists).
    if level != "block" {
        t.Errorf("level = %q, want block (specific-version signed source + critical severity must escalate)", level)
    }
}
```

---

### CR-02: `QuarantineAt` Override Can Be Bypassed by Lowering Global `BlockAt` Via Policy File

**File:** `internal/policy/corroboration.go:169` and `internal/policy/corroboration.go:48`

**Issue:** `validateCorroborationThresholds` correctly enforces that
`SeverityOverrides["critical"].BlockAt <= t.BlockAt` (the override cannot be looser
than global). However it does NOT enforce that
`SeverityOverrides["critical"].QuarantineAt <= t.BlockAt`. This creates a path
where:

1. Default thresholds: `BlockAt=2, QuarantineAt=3`, override `{BlockAt:1, QuarantineAt:2}`.
2. An operator writes a policy file with `block_at: 1`, making `t.BlockAt = 1`.
3. `validateCorroborationThresholds` checks: `ov.BlockAt (1) <= t.BlockAt (1)` — passes.
4. But `ov.QuarantineAt (2) > t.BlockAt (1)` — the quarantine threshold of the
   override is now looser than the global block threshold!
5. Escalation table: `signedCount >= effectiveQuarantineAt (2)` requires 2 signed
   sources to quarantine, but `signedCount >= effectiveBlockAt (1)` blocks at 1.
   This is internally consistent for the override path. But it means that an
   operator who meant to tighten things to block=1 might be surprised that
   quarantine still requires 2 signed sources.

More critically, consider the inverse: a policy file sets `block_at: 2` (global
default stays the same) but `critical_block_at: 2` as well (making the override
equal to global). Now `ov.QuarantineAt = 3` (from the default), which is equal to
the global `t.QuarantineAt`. `validateCorroborationThresholds` accepts this. The
escalation table now reaches `effectiveQuarantineAt = 3` but the switch case at
line 169 is `signedCount >= effectiveQuarantineAt && hasSignedSource`. If
`signedCount = 3` and `effectiveQuarantineAt = 3`, this fires — this is correct
and not a bug. But the real defect is: when a policy file sets `critical_block_at:
2` (matching global `block_at: 2`), the override fires at the same threshold as
global — but `effectiveQuarantineAt = 3` (from default override), while the global
`t.QuarantineAt` is also 3. So no change in behavior, but a false sense that
"critical" is being handled differently.

The real actionable defect: after a `CriticalBlockAt` policy-file merge in
`ThresholdsFromPolicyFiles`, the resulting `SeverityOverrides["critical"].QuarantineAt`
is set to `r.CriticalBlockAt + 1` only when the existing `QuarantineAt == 0`.
When the existing `QuarantineAt` is already `2` (from the default), subsequent
policy-file rules that raise `CriticalBlockAt` to `2` do NOT update `QuarantineAt`
— it stays at `2`. This makes quarantine fire at `BlockAt=2` (same source count
as block), which means quarantine and block trigger simultaneously on 2 signed
sources rather than requiring 3 for quarantine. This violates the PLCY-01 principle
that quarantine requires a stricter threshold than block.

**Concrete scenario:**
1. First policy rule: `CriticalBlockAt: 1` → override `{BlockAt:1, QuarantineAt:2}`.
2. Second policy rule in a different file: `CriticalBlockAt: 2` → code checks
   `existing.QuarantineAt == 0` → it is `2`, not 0 → QuarantineAt stays `2`.
   Override becomes `{BlockAt:2, QuarantineAt:2}` — quarantine and block at same threshold.
3. `validateCorroborationThresholds` checks `ov.QuarantineAt >= ov.BlockAt` (line 47) →
   `2 >= 2` — passes. The misconfiguration is accepted.

**Fix — in `validateCorroborationThresholds`:**

```go
if ov.QuarantineAt <= ov.BlockAt {
    return fmt.Errorf("corroboration: SeverityOverrides[%q].QuarantineAt (%d) must be > BlockAt (%d)", sev, ov.QuarantineAt, ov.BlockAt)
}
```

Change `<` to `<=` (enforce strictly greater, not greater-or-equal) so that
`{BlockAt:2, QuarantineAt:2}` is rejected, not accepted.

**Fix — in `ThresholdsFromPolicyFiles` and `thresholdsFromPolicyFile`:**

When `CriticalBlockAt` changes and the existing `QuarantineAt` is now equal to
or less than the new `BlockAt`, reset it:

```go
if r.CriticalBlockAt > 0 {
    if t.SeverityOverrides == nil {
        t.SeverityOverrides = make(map[string]policy.SeverityThreshold)
    }
    existing := t.SeverityOverrides["critical"]
    existing.BlockAt = r.CriticalBlockAt
    // Always recompute QuarantineAt to stay strictly above BlockAt.
    if existing.QuarantineAt <= existing.BlockAt {
        existing.QuarantineAt = existing.BlockAt + 1
    }
    t.SeverityOverrides["critical"] = existing
}
```

---

## Warnings

### WR-01: `resolveCatalogHealthy` Duplicated Verbatim in Four Packages — Divergence Risk

**File:** `internal/check/sanity.go:34`, `internal/gateway/sanity.go:26`,
`internal/watch/sanity.go:26`, `internal/scan/sanity.go:26`

**Issue:** The four copies are byte-for-byte identical at this moment. The design
correctly places the function in the I/O caller tier rather than `internal/policy`
(to avoid an import cycle and preserve purity). However, this is a fragile
arrangement: any future change to the state-reading logic (e.g., checking a second
source like "osv" for degradation, or reading a new field like `HealthScore`) must
be applied in all four places. A partial update — patching `check/sanity.go` but
forgetting `scan/sanity.go` — would silently create divergent security behavior
across `beekeeper check` vs. `beekeeper scan`.

This was an accepted design trade-off in Phase 6 (noted in 06-RESEARCH.md:
"Place in each package's existing utility file or a new small `internal/check/sanity.go`").
The risk is documented but the maintenance surface is real.

**Fix:** Extract to a shared internal package, e.g. `internal/cataloghealth`, that
each caller tier imports. The package performs I/O (reads state.json) and therefore
does not belong in `internal/policy`, but it need not be duplicated. Alternatively,
add a `//nolint:goduplicate` comment with an explicit cross-reference to all four
copies so that any future change triggers a reviewer prompt. Minimum: add a
`// MAINTENANCE: keep in sync with internal/{check,gateway,watch,scan}/sanity.go`
comment to each copy.

---

### WR-02: `CriticalBlockAt` Upper-Bound Not Validated at Policy Load Time

**File:** `internal/policyloader/validate.go:56-63`

**Issue:** `ValidateSchema` enforces `CriticalBlockAt >= 1` at load time. It does
NOT enforce `CriticalBlockAt <= global block_at` at load time, deferring that
check to `validateCorroborationThresholds` at eval time. The research document
acknowledges this split intentionally (comment at line 62-63: "Upper bound
validated at eval time by validateCorroborationThresholds, which has the resolved
global BlockAt"). However, the consequence is that `beekeeper policy validate`
reports a file as valid even when `critical_block_at: 5` with `block_at: 2` is
present — an operator gets no immediate feedback that their file is misconfigured.
The error only fires at eval time (fail-closed, which is correct for security but
confusing for operators).

Additionally, when `block_at` is NOT set in the same rule (so global default 2
applies), `ValidateSchema` cannot validate the upper bound without knowing the
effective global `BlockAt`. But when `block_at` IS set in the same rule (e.g.,
`block_at: 1, critical_block_at: 2`), ValidateSchema has both values and could
catch the inversion immediately.

**Fix:** In `ValidateSchema`, when a `corroboration_threshold` rule specifies both
`BlockAt` and `CriticalBlockAt` in the same rule, validate the upper bound at load
time:

```go
if r.RuleType == "corroboration_threshold" && r.CriticalBlockAt != 0 {
    if r.CriticalBlockAt < 1 {
        errs = append(errs, fmt.Errorf("rule[%d] %q: critical_block_at (%d) must be >= 1",
            i, r.ID, r.CriticalBlockAt))
    }
    // When block_at is also set in this rule, validate the upper bound here too.
    if r.BlockAt > 0 && r.CriticalBlockAt > r.BlockAt {
        errs = append(errs, fmt.Errorf("rule[%d] %q: critical_block_at (%d) must be <= block_at (%d)",
            i, r.ID, r.CriticalBlockAt, r.BlockAt))
    }
}
```

---

### WR-03: `TestCorroborationOneSignedSource` Unexpectedly Tests the Override Path

**File:** `internal/policy/corroboration_test.go:32-52`

**Issue:** `TestCorroborationOneSignedSource` (a pre-existing test from Phase 2)
uses `DefaultCorroborationThresholds()` and a single signed match with `Severity:
""` (empty string — the zero value). After Phase 6, `DefaultCorroborationThresholds()`
includes `SeverityOverrides["critical"]`. The test still passes because:
- `m.Severity == ""` is not in `SeverityOverrides` (only `"critical"` is), so
  `findSeverityOverride` returns nil.
- `effectiveBlockAt` remains `t.BlockAt = 2`.
- `signedCount = 1 < 2` → "warn".

This is correct behavior, but the test's intent — "one signed source warns at
default thresholds" — is now silently testing only non-critical severity. If a
future developer adds a severity field to this match, the test might accidentally
start exercising the override path and fail for the wrong reason.

**Fix:** Add an explicit `Severity: "high"` (non-critical) to the match in
`TestCorroborationOneSignedSource` and add a comment clarifying the test verifies
the NON-override path:

```go
// Explicitly use "high" severity (not "critical") to exercise the global-threshold
// path, not the SeverityOverrides["critical"] override path.
matches := []CatalogMatch{
    {CatalogSource: "bumblebee", Severity: "high", Signed: true},
}
```

---

### WR-04: `LoadState` Returns Success for Missing File — `resolveCatalogHealthy` Silently Returns `true` on Corrupt or Truncated State

**File:** `internal/check/sanity.go:39-41` (and all three other copies)

**Issue:** `catalog.LoadState` treats a missing state.json as success, returning
`WatchState{Sources: make(map[string]SourceState)}` with `err == nil`. This means
`resolveCatalogHealthy` correctly returns `true` for a first-run missing file.

However, `LoadState` also returns `err != nil` for a CORRUPT or UNREADABLE
state.json (permission denied, partial write). In that case `resolveCatalogHealthy`
returns `true` — the healthy default — which is also intentional ("inability to
read state file is not evidence of degradation"). This is documented and is the
correct policy choice.

The actual defect is narrower: a state.json that is valid JSON but has a `sources`
key that is `null` (or an empty object `{}`) will successfully parse and return a
`WatchState{Sources: make(map[string]SourceState)}` with `err == nil`. Then the
`state.Sources["bumblebee"]` lookup returns the zero `SourceState{Degraded: false}`.
So `resolveCatalogHealthy` returns `true`. This is correct behavior — an empty
sources map means bumblebee has never synced — but it is the same return value as
the "confirmed healthy" case, which means: if an attacker can truncate state.json
to `{}` (e.g., via a race with the write daemon, or by removing write permissions
on state.json before the watch daemon can update it), they permanently suppress
degradation detection. The write is atomic (`writeFileAtomic` uses temp+rename),
but READ permission removal or directory attribute manipulation could cause
`os.ReadFile` to fail, forcing the `err != nil` → healthy=true path.

This is a known-acceptable trade-off per the research document, but it should be
explicitly documented as a security assumption in the code comment, not just in
the research doc.

**Fix:** Add to the `resolveCatalogHealthy` comment in all four sanity.go files:

```go
// Security note: this function defaults to healthy=true on any read failure
// (missing file, permissions error, parse error). An attacker who can make
// state.json unreadable (e.g. by removing read permission) will suppress
// degradation detection and re-enable severity escalation. This is a conscious
// trade-off: the watch daemon and the check handler run under the same user
// account, so an attacker with permission to modify state.json already has
// file-system access broader than Beekeeper's trust boundary. Verify that
// ~/.beekeeper/ has owner-only permissions (0o700) on installation.
```

---

## Info

### IN-01: Escalation Table Has Redundant `hasSignedSource` Guard on Both Block Cases

**File:** `internal/policy/corroboration.go:169-172`

**Issue:** Both block switch cases guard with `&& hasSignedSource`:

```go
case signedCount >= effectiveQuarantineAt && hasSignedSource:
case signedCount >= effectiveBlockAt && hasSignedSource:
```

`hasSignedSource` is defined as `signedCount >= 1`. If `signedCount >=
effectiveBlockAt` is true, and `effectiveBlockAt >= 1` (enforced by
`validateCorroborationThresholds`), then `hasSignedSource` is always true when
either block case fires. The `&& hasSignedSource` guard is therefore dead code
on the block cases, though harmless. The guard matters on the warn case
(`signedCount >= t.WarnAt || hasUnsigned`) where `t.WarnAt` could be 0 in a
misconfigured threshold (though that would fail validation).

**Fix:** Either remove the `hasSignedSource` guards from the block cases as
dead code, or keep them for documentation value but add a comment explaining
they are invariant-preserving assertions, not runtime guards:

```go
// hasSignedSource is invariant when signedCount >= effectiveBlockAt >= 1,
// but kept as an explicit assertion of the signed-source requirement.
case signedCount >= effectiveQuarantineAt && hasSignedSource:
```

---

### IN-02: `TestCatalogMatchWarns` Asserts Phase 1 Behavior That Phase 6 Changes

**File:** `internal/check/handler_test.go:67-87`

**Issue:** `TestCatalogMatchWarns` builds a test index where the nrwl.angular-console
entry has `Severity: "critical"` and `CatalogSignature: ""` (unsigned). It asserts
"exit 0, level warn" based on the Phase 1 comment: "single-source catalog match is
warn, NOT block — exit 0".

After Phase 6, with `DefaultCorroborationThresholds()` including
`SeverityOverrides["critical"] = {BlockAt:1}`, this entry would trigger the
critical override IF it were signed. Because `CatalogSignature: ""` makes
`Signed: false`, the entry contributes to `unsignedSet` but not `signedSet`.
`signedCount = 0`. `effectiveBlockAt = 1` (from override). `signedCount (0) >=
effectiveBlockAt (1)` → false. Falls through to warn.

The test still passes, but its comment ("Phase 1: single-source catalog match is
warn, NOT block") is now misleading — it implies the warn result is because Phase
2 multi-source corroboration is required, when actually it's because the single
bumblebee source is unsigned. The comment should be updated to reflect Phase 6
reality.

**Fix:** Update the comment:

```go
// Phase 6: single UNSIGNED bumblebee source → warn even with SeverityOverrides["critical"]
// active, because unsigned sources never contribute to signedCount (PLCY-01).
// signedCount=0 < effectiveBlockAt=1 → warn, exit 0.
```

---

### IN-03: `validate.go` Redundant Condition — `CriticalBlockAt < 1` Can Never Be True When `CriticalBlockAt != 0`

**File:** `internal/policyloader/validate.go:56-61`

**Issue:** The validation reads:

```go
if r.RuleType == "corroboration_threshold" && r.CriticalBlockAt != 0 {
    if r.CriticalBlockAt < 1 {
        errs = append(errs, ...)
    }
}
```

`CriticalBlockAt` is `int`. The outer condition is `!= 0`, so inside the block,
`CriticalBlockAt` is either positive (>= 1) or negative (<= -1). The inner `< 1`
check fires only for negative values, not for zero. So the comment "CriticalBlockAt
zero means use default" is correct and the outer check correctly skips zero. But
the code could be made clearer: the real intent is to reject negative values.

**Fix:** Rewrite for clarity:

```go
if r.RuleType == "corroboration_threshold" && r.CriticalBlockAt != 0 {
    // Zero is valid (means "use default"). Negative values are rejected.
    if r.CriticalBlockAt < 1 {
        errs = append(errs, fmt.Errorf("rule[%d] %q: critical_block_at (%d) must be >= 1 (or 0 for default)",
            i, r.ID, r.CriticalBlockAt))
    }
}
```

This is already the correct code — just needs a comment clarifying that negative
(not zero) is the actual invalid case, since `omitempty` on an int field means
JSON `0` and absent field are indistinguishable at decode time (both decode as 0).
Add: `// Note: JSON omitempty makes 0 and absent field identical; both mean "use default".`

---

_Reviewed: 2026-06-03_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
