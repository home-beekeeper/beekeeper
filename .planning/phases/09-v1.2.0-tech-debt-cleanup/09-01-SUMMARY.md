---
phase: 09-v1.2.0-tech-debt-cleanup
plan: 01
subsystem: security
tags: [spath, credential-block, symlink, ntfs-ads, shell-parsing, purity, fail-closed]

# Dependency graph
requires:
  - phase: 07-sensitive-path-runtime-enforcement
    provides: "canonicalizePath + policy.EvaluatePath SPATH credential-read block (F2); extractBashCredentialPaths read-verb scan (CR-01)"
provides:
  - "canonicalizePathForms(raw) — dual-form (lexical + EvalSymlinks-resolved) canonicalizer that defeats ancestor-symlink fragment-stripping (HARDEN-01)"
  - "normalizeBasename(seg) — pure NTFS-ADS + trailing-dot/space basename normalizer applied on both block and allow basename branches (HARDEN-02)"
  - "isShellBoundary(byte) + left-boundary-anchored read-verb extraction (HARDEN-03)"
  - "Regression tests for all three SPATH evasion edges that fail on pre-fix code"
affects: [09-03-handler-wiring, spath, credential-block]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Dual-form path matching: block on EITHER lexical or symlink-resolved form (fail-closed, ancestor-symlink-proof)"
    - "Match-only basename normalization kept PURE in internal/policy; all FS/symlink/env I/O stays in internal/check adapter (purity gate held)"
    - "Left word-boundary anchoring for verb-prefix shell scanning"

key-files:
  created: []
  modified:
    - "internal/check/paths.go — canonicalizePathForms, isShellBoundary, boundary-anchored extractBashCredentialPaths"
    - "internal/check/paths_test.go — HARDEN-01/02/03 regression + Windows-gated adapter tests"
    - "internal/policy/path.go — normalizeBasename applied in matchesBlockPattern + isAllowedPath basename branches"
    - "internal/policy/path_test.go — ADS/trailing-dot pure table cases + normalizeBasename unit test + allowlist-alignment test"

key-decisions:
  - "canonicalizePath left UNCHANGED for backward compat; new dual-form lives in canonicalizePathForms (consumer wiring deferred to Plan 03 which owns handler.go)"
  - "ADS cut from the FIRST ':' at the segment level (lastSegment already stripped any drive-letter prefix), so any colon in a basename is an ADS separator"
  - "normalizeBasename mirrored on allow AND block branches so an attacker cannot un-allowlist a safe lookalike nor smuggle a blocked file past the allow check"
  - "Boundary bytes {space,tab,\\n,\\r,;,|,&,(} — '&&'/'||' end in &/|, subshell opens with '(' — cover the realistic standalone-verb cases without a full tokenizer"

patterns-established:
  - "Dual-form SPATH matching: a block on either pre- or post-EvalSymlinks form blocks (fail-closed)"
  - "Pure normalizer (strings-only) for match-only canonicalization; never mutates the stored/returned path"

requirements-completed: [HARDEN-01, HARDEN-02, HARDEN-03]

# Metrics
duration: 13min
completed: 2026-06-04
---

# Phase 9 Plan 01: SPATH Evasion Hardening Summary

**Closes the three deferred SPATH credential-exfil evasion edges (IN-01/02/03): ancestor-symlink fragment-stripping via a dual-form canonicalizer, Windows NTFS-ADS + trailing-dot/space basenames via a pure normalizer, and verb-substring false-handling via left word-boundary anchoring — each covered by a regression test that fails on the pre-fix code, with internal/policy kept pure.**

## Performance

- **Duration:** ~13 min
- **Started:** 2026-06-04T18:52Z (approx)
- **Completed:** 2026-06-04T19:05Z
- **Tasks:** 3
- **Files modified:** 4

## Accomplishments

- **HARDEN-01 (IN-01):** `canonicalizePathForms(raw) []string` returns BOTH the lexically-cleaned (pre-EvalSymlinks) form and the EvalSymlinks-resolved form, de-duplicated. An ancestor-directory symlink can no longer resolve a `/.aws/` or `/.ssh/` fragment out of the canonical path — the lexical form preserves the sensitive shape, and a downstream block on either form blocks. `canonicalizePath` is unchanged for existing callers; consumer wiring is explicitly deferred to Plan 03 (which owns `handler.go`). Fail-closed (EvalSymlinks→Abs fallback, Pitfall 3) is preserved.
- **HARDEN-02 (IN-02):** Pure `normalizeBasename(seg)` strips a trailing NTFS Alternate-Data-Stream suffix (`id_rsa:stream` → `id_rsa`) and trims trailing dots/spaces (`credentials.`, `credentials ` → `credentials`) for match purposes only. Applied in BOTH `matchesBlockPattern` and `isAllowedPath` basename branches so the block lookalikes block and the allow lookalikes (`.env.example:stream`, `.env.example.`) stay allowed. `internal/policy/path.go` still imports only `strings` — `TestPathImportsArePure` stays green.
- **HARDEN-03 (IN-03):** `isShellBoundary(byte)` + a left-boundary guard in `extractBashCredentialPaths` means a read verb matches only at start-of-string or after a shell separator. Embedded substrings (`cat ` inside `catalog.sh `, `cat` inside `scatter`) no longer false-trigger credential extraction, while real standalone reads (`more ~/.ssh/id_rsa`), chained reads (`cat a && cat ~/.ssh/id_rsa`, CR-01), and leading-flag skips (`cat -n …`) still extract. The trailing space in each verb keeps the right-hand boundary.

## Task Commits

Each task was committed atomically (TDD `tdd="true"`; implementation + regression test landed together per task):

1. **Task 1: HARDEN-01 — ancestor-symlink dual-form helper** — `30abe32` (feat)
2. **Task 2: HARDEN-02 — ADS + trailing-dot basename normalization (pure + adapter)** — `6deba4f` (feat)
3. **Task 3: HARDEN-03 — word-boundary-anchored read-verb extraction** — `e868431` (feat)

**Plan metadata:** committed separately by the orchestrator (STATE/ROADMAP/REQUIREMENTS owned by orchestrator on this project per the phase tooling note).

## Files Created/Modified

- `internal/check/paths.go` — Added `canonicalizePathForms` (dual-form), `isShellBoundary`, and a left-boundary guard in `extractBashCredentialPaths`; refreshed the `bashReadVerbs` doc comment for the boundary rule. Impure adapter — all FS/symlink/env work stays here.
- `internal/check/paths_test.go` — `TestCanonicalizePathForms` (ancestor-symlink, skips cleanly when `os.Symlink` is unprivileged), `TestCanonicalizeEvaluateADS` (Windows-gated end-to-end), and HARDEN-03 boundary/embedded-substring cases in `TestExtractBashCredentialPaths`; `anyFormContains` / `assertNotContains` helpers.
- `internal/policy/path.go` — Added pure `normalizeBasename`; applied it in the basename branches of `matchesBlockPattern` and `isAllowedPath`. No new imports (still `strings` only).
- `internal/policy/path_test.go` — ADS/trailing-dot cases in the `TestEvaluatePath` table, `TestNormalizeBasename` unit test, `TestEvaluatePathBasenameADSBlock`, and `TestEvaluatePathAllowlistNormalizationAligned`.

## Decisions Made

- Kept `canonicalizePath` unchanged and introduced `canonicalizePathForms` alongside it — existing callers/tests stay green; the two SPATH loops adopt the dual-form in Plan 03. A doc comment on the helper points to Plan 03 for the consumer wiring.
- ADS cut from the **first** `:` at the segment level — `lastSegment` already strips any `C:` drive-letter prefix (it returns the component after the final separator), so any remaining `:` is unambiguously an ADS separator.
- Mirrored `normalizeBasename` on the allow branch too, so the normalization cannot be abused to un-allowlist a safe file or smuggle a blocked file past the allow check.

## Deviations from Plan

None — plan executed exactly as written. `internal/policy` purity preserved; no architectural changes; no out-of-scope fixes. `handler.go` / `integration_test.go` intentionally untouched (owned by Plan 03, per the plan).

## Issues Encountered

- **Task 3 (self-corrected test expectation, not a code/plan deviation):** The first draft of the subshell-boundary sub-test asserted `(cat ~/.aws/credentials)` extracts the bare token `~/.aws/credentials`. `firstShellToken` stops only at whitespace (not `)`), so it returns `~/.aws/credentials)` with the trailing paren — pre-existing tokenizer behavior unrelated to HARDEN-03. The boundary anchoring itself worked correctly (the verb after `(` matched). Fixed the test to use `(cat ~/.aws/credentials extra)` and assert the clean token, documenting the tokenizer caveat inline. No production-code change.

## TDD Gate Compliance

Each task is `tdd="true"`. The behavior added in each is provable by a regression test that fails on the pre-fix code:
- HARDEN-01: `canonicalizePathForms` did not exist pre-fix — the ancestor-symlink test cannot compile/pass against pre-fix code, and the single-form `canonicalizePath` can strip the fragment.
- HARDEN-02: `normalizeBasename` did not exist pre-fix — `id_rsa:stream` / `credentials.` would not match the basename branch.
- HARDEN-03: pre-fix `strings.Index` substring scan would extract a credential token from `./catalog.sh ~/.ssh/id_rsa`-shaped inputs at a non-boundary match; the boundary guard rejects it.

Tasks were committed as combined `feat(...)` commits (impl + test together) rather than split RED/GREEN commits — acceptable for a hardening plan where each fix and its proving test are a single coherent unit; the regression intent is documented above.

## Verification

- `go build ./...` — clean.
- `go vet ./internal/check/... ./internal/policy/...` — clean.
- `go test ./internal/check/ ./internal/policy/` — both green (full suites, no SPATH regression).
- `go test ./internal/policy/ -run TestPathImportsArePure` — green (purity preserved).
- HARDEN-01 ancestor-symlink sub-test and HARDEN-02 Windows-gated adapter test both **ran** (not skipped) on this Windows dev host (symlink privilege available) and passed.

_Note: `go test -race` is CGO-gated / CI-only on this Windows box; plain `go test` is authoritative locally, CI confirms the race pass._

## Next Phase Readiness

- The three helpers are delivered and unit-proven. **Plan 03** (handler-wiring) must replace the single `canonicalizePath` call in `handler.go`'s two SPATH loops (and `integration_test.go`) with a loop over `canonicalizePathForms`, blocking on any returned form, to land the HARDEN-01 dual-form fix end-to-end at the call sites. HARDEN-02/03 are already effective wherever the existing `EvaluatePath` / `extractBashCredentialPaths` paths run.
- No blockers.

---
*Phase: 09-v1.2.0-tech-debt-cleanup*
*Completed: 2026-06-04*
