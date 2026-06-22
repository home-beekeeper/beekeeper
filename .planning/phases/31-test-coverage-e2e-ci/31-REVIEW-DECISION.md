# v1.5.0 Install Posture — Release-Gate Review Disposition (2026-06-22)

Two reviews were run over the full milestone diff (`5d1db0b..HEAD`, ~47 production Go files) before opening the PR to main:
- **Security review** (security-engineer): all 8 load-bearing invariants **HELD**, recommendation **APPROVE**. Only 1 Low (pre-existing, non-exploitable shim-quoting) + 2 Info. Full report in the agent transcript; summary in STATE.
- **Code review** (gsd-code-reviewer): 0 critical / 0 blocker / 1 high / 2 medium / 5 low / 2 info. Full report: `31-REVIEW.md`.

The security verdict confirms NONE of the code-review findings can downgrade a catalog malware block or break the fail-closed path (posture is a warn-only, most-restrictive-merged, structurally-isolated layer).

## Fixed before the PR

- **H-01 (HIGH)** — `posture allow --once --rule <r>` recorded a rule-scoped `posture_override` audit entry but enforced an all-rules one-shot (the `allowOnceToken` has no rule field). **Fixed** by rejecting the `--once`+`--rule` combination fail-closed (a one-shot is inherently all-rules), so the audit record can never over-claim its scope. Help text + `docs/install-posture.md` clarified that `--rule` pairs with `--always`; per-rule one-shot is a documented follow-up. Locked by `TestPostureAllowOnceRejectsRule`. Commit: see `test(31)/fix(31)` below.

## Deferred to follow-up (tracked, not release-blocking)

Release-scope discipline: fix the one live ship-gate bug; do not churn the audit path or the 3 cross-platform Sentry daemons right before release for non-live issues. Each item below is genuine but either latent/dead-code, currently-correct-but-fragile, or a pre-existing pattern; none affects the security invariants.

- **M-01 (MED)** — `PostureIncidentFromRecord` reads package/ecosystem from `CatalogMatches`, which a hook posture `policy_decision` record never populates. **Latent**: `PostureIncidentModel` is not wired into the live feed (only `PosturePanel` is). The proper fix threads the package into the hook posture decision audit record; do it when the incident card is wired in. The current test hand-populates `CatalogMatches` (false-confidence) — fix the test honesty at the same time.
- **M-02 (MED)** — `alertToAuditRecord` maps the install-observed record by `Severity == "info"` rather than `RuleID == "SENTRY-009"`. Currently correct (only SENTRY-009 emits info); fix is a 1-line dispatch change per daemon (linux/darwin/windows) but those files are only cross-vet'd locally, not run. Fix in a Sentry-focused follow-up.
- **L-01** allow-once read-then-write TOCTOU: two concurrent same-package installs can both consume one token. Benign (both were operator-authorized once; cannot fabricate an allow for an un-tokened package). Documented in `posture_allowonce.go`.
- **L-02** allow-once token consumed even when the install was already clean (minor UX waste of the one-shot).
- **L-03** SENTRY-009 misses `npm ci` and uses an anywhere-in-cmdline verb scan (detection-completeness only; observe-only rule).
- **L-04** Node/Bun version floors are configured/detected but unused (dead config carried over from the nudge; safe to keep, remove in a cleanup pass).
- **L-05** `"go get"` prefix match lacks a trailing space (pkgparse nit).
- **I-01** scoped-npm cache-key collision in `lifecycle_cache.go` (`@scope-a/foo` vs `@scope-b/foo` via `filepath.Base`); mirrors the pre-existing `ageCachePath` pattern. Fix both caches consistently in a follow-up.
- **I-02** second `time.Now()` read in `handler.go:373` (harmless).
- **Security-Low** — the experimental Unix shim embeds `realBin`/`tool` into a `/bin/sh` template via `fmt.Sprintf`. Inputs are controlled today (locked tool list + `LookPath`), so not exploitable; single-quote the embedded paths in a defensive follow-up.

Suggested: capture the deferred items via `/gsd-capture` into `.planning/todos/pending/` for the next milestone.
