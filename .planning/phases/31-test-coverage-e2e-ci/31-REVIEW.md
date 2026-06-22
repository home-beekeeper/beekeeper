---
phase: v1.5.0-install-posture (release-gate review)
reviewed: 2026-06-22
depth: deep
diff_base: 5d1db0b..HEAD
findings:
  critical: 0
  blocker: 0
  high: 1
  medium: 2
  low: 5
  info: 2
status: issues_found
---

# v1.5.0 Install Posture — Adversarial Code Review

Scope: `git diff 5d1db0b..HEAD`, ~47 production Go files. Focus on correctness,
fail-closed-vs-fail-soft boundaries, security, and "test passes but production is
subtly wrong" defects. Overall the milestone is high quality: the pure/impure
split is respected, the merge-order security model is sound, and the nudge removal
is complete. The findings below are the exceptions.

## HIGH

### H-01: `posture allow --once --rule <r>` records a rule-scoped exception but enforces an ALL-rules exception
**File:** `internal/check/posture_allowonce.go:49-54` + `cmd/beekeeper/posture_override.go:155-170`
The `--once` allow path accepts and AUDITS a `--rule` scope, but the on-disk
`allowOnceToken` struct has no `Rule` field, and `runPostureAllowOnce` calls
`addPostureAllowOnceFn(stateDir, ecosystem, pkg, reason)` — dropping `rule`
entirely. At enforcement time `evaluatePosture` (posture_adapter.go:120) consumes
the token and returns `allow` for the WHOLE posture evaluation (all three rules),
not just the named rule. So `beekeeper posture allow foo --once --rule release-age`
writes a `posture_override` record claiming `posture_rule: release-age`, but the
next install of `foo` is silenced for release-age AND lifecycle AND remote-source.
This is a scope-widening discrepancy between the audited promise and the enforced
behavior — exactly the class of "honest scope" defect this milestone elsewhere
guards against. The `--always` path handles `--rule` correctly (it lands on
`PostureAllow.Rule` and flows through `PostureRuleExcludes`), which makes the
`--once` divergence easy to miss. No test covers `--once --rule` enforcement scope.
**Impact:** an operator who narrowly scoped a one-shot exception unknowingly grants
a broader one-shot pass; the audit trail misrepresents the granted scope.
**Fix:** add `Rule string` to `allowOnceToken`; thread `rule` through
`AddAllowOnce` and `runPostureAllowOnce`; in `evaluatePosture`, when a token is
consumed, only suppress the matching rule(s) (or, simplest acceptable fix: reject
`--once` combined with `--rule` until per-rule one-shot is implemented, so the
audit record can never over-claim).

## MEDIUM

### M-01: `PostureIncidentFromRecord` reads package/ecosystem from `CatalogMatches`, which a real posture record never populates
**File:** `internal/tui/posture_incident.go:68-72`
The incident card resolves `pkg`/`eco` from `rec.CatalogMatches[0]`. But posture
decisions are produced by the pure evaluators (`EvaluateReleaseAge` etc.) and
carried through `audit.FromDecision`, which sets `CatalogMatches` from
`d.CatalogMatches` — posture decisions have NONE. So for any genuine posture warn
record, `CatalogMatches` is empty and the suggested `beekeeper posture allow ...`
command always renders the `<package>` placeholder instead of the real package
(which is only available in `rec.Reason`, e.g. "release age unknown for **foo**").
The unit test (`posture_incident_test.go:19`) hand-populates `CatalogMatches`, so
it passes while production output is degraded. Currently latent: `PostureIncidentModel`
is not yet wired into the TUI feed (`model.go` only wires `PosturePanel`), so this
ships as dead-but-buggy code. It will be wrong the moment it is connected.
**Impact:** when wired, the operator must hand-fill the package into every suggested
command; the card's core value (a copy-paste-ready command) is lost.
**Fix:** populate the package/ecosystem on the posture decision's audit record at
the producing site (e.g. add `PosturePackage`/`PostureEcosystem` when
`evaluatePosture` builds the decision, then map them in the card), or parse the
`PosturePackage` field if it is set on policy_decision records. Do not rely on
`CatalogMatches` for posture provenance.

### M-02: `alertToAuditRecord` dispatches the install-observed record on `Severity == "info"` rather than the rule ID
**File:** `internal/sentry/{linux,darwin,windows}/daemon.go` (alertToAuditRecord)
The SENTRY-009 detection-only branch is keyed on `alert.Severity == "info"` and is
the FIRST case in the switch. Today only SENTRY-009 emits "info", so it is
unambiguous. But this couples the audit record-type/decision mapping to a severity
STRING shared across all rules instead of the stable rule ID. Any future rule that
emits `"info"` would be silently relabeled `sentry_install_observed` / `observe`
and lose its alert semantics — a fail-quiet downgrade for an unrelated detection.
**Impact:** fragile; a future "info"-severity rule is silently misclassified as a
benign install observation (security-relevant if that rule is a real alert).
**Fix:** dispatch on `alert.RuleID == "SENTRY-009"` (the install-observe invariant)
rather than the severity string. Keep severity for display only.

## LOW

### L-01: allow-once consume is a read-then-write TOCTOU; two concurrent installs can both consume one token
**File:** `internal/check/posture_allowonce.go:206-231`
`ConsumeAllowOnce` reads the store, finds a match, then atomically rewrites the
remainder. The rename is atomic (no torn file), but two concurrent `beekeeper check`
processes can both read the same token before either rewrites, so a single
`--once` token can allow TWO installs. The file header frames the worst case as
"consumed again on a retry," but the real race is double-allow across distinct
installs. Acceptable because allow-once is explicitly a convenience, not a security
gate, and the catalog block still wins — but it is a wider one-shot than documented.
**Fix:** acceptable as-is given the convenience framing; if tightened, take an
OS file lock around read+rewrite, or accept the documented limitation explicitly.

### L-02: allow-once token is consumed even when the install would have been clean
**File:** `internal/check/posture_adapter.go:120-127`
The token is consumed before any rule is evaluated, so an install that would not
have warned anyway still burns the one-shot token. Wasteful, not incorrect.
**Fix:** optionally evaluate rules first and only consume the token if a rule
fired; or document the eager-consume behavior in the CLI help.

### L-03: SENTRY-009 install detection misses `npm ci` and uses a verb-token scan that can mislabel
**File:** `internal/sentry/install_observe.go:47-70`
`installVerbs` omits `ci` (`npm ci` is a real install) and `dlx`/`x`/`npx` exec
installs. Conversely the bare token scan over the full cmdline means a manager
process with an install verb appearing anywhere in argv (e.g. a path or flag value
that tokenizes to `add`/`get`) could over-match. Detection-only, so impact is a
missed/extra observation record, never enforcement.
**Fix:** add `ci` to the verb set; consider matching the verb only in the
command-word position (argv[1]) rather than anywhere in the line.

### L-04: Node and Bun version floors are configured/detected but never used
**File:** `internal/posture/config.go:28-31`, `internal/posture/detect.go:182`
`VersionFloors.Node` and `VersionFloors.Bun` and `PMState.NodeVersion` are wired
through `DefaultConfig`/`DetectState` but no code calls `meetsFloor` on them. The
comment "Required for the pnpm 11 Node >= 22 compatibility check" describes a check
that does not exist; bun hardening depends solely on `BunScannerOK`. Dead config /
unimplemented feature.
**Fix:** either implement the Node>=22 / bun-floor checks in `BuildComparison`, or
remove the unused floor fields and the misleading comment.

### L-05: `go get` / `go getfoo` prefix match (no trailing space) over-matches
**File:** `internal/pkgparse/pkgparse.go:97`
The `"go get"` install-table prefix has no trailing space, so `go getfoo` matches
and parses `foo` as an install. Harmless (not a real command), but inconsistent
with the space-terminated prefixes for other managers.
**Fix:** use `"go get "` for consistency, accepting that bare `go get` (no args)
then needs its own handling if desired.

## INFO

### I-01: cache key collision for scoped npm packages in the lifecycle/age caches (pre-existing)
**File:** `internal/catalog/lifecycle_cache.go:41-46`
`lifecycleCachePath` (mirroring the pre-existing `ageCachePath`) sanitizes each
segment with `filepath.Base`, which strips the npm scope: `@scope-a/foo` and
`@scope-b/foo` both resolve to a `foo/` cache dir and collide. Fail-soft (a stale
hit warns-unknown at worst), and the pattern predates this milestone, but the new
lifecycle cache inherits it. Note only.

### I-02: `handler.go` passes a fresh `time.Now().UTC()` to `evaluatePosture` instead of the `start` clock
**File:** `internal/check/handler.go:373`
Minor: the handler captures `start := time.Now()` for latency but passes a second
`time.Now().UTC()` into posture evaluation. Not a bug (release-age math wants
wall-clock UTC), just a second clock read; harmless.

---

## Areas reviewed and found clean

- **`internal/policy/{release_age,lifecycle,remote_source}.go`** — pure (imports
  only fmt/strings), correct allowlist-before-fail-closed ordering, no I/O. The
  pure evaluators correctly block fail-closed for the scan/watch path while the
  hook adapter re-maps to fail-soft warn.
- **Decision merge order** (`handler.go:295-375`) — posture is merged LAST and only
  ever adds a warn (or a block on a definite violation under opt-up); most-restrictive-
  wins guarantees a catalog/sensitive-path/self-protect block can never be downgraded.
- **Config layered merge** (`layered.go`) — `mergePosture`/`mergePostureUntrusted`
  are tighten-only from untrusted layers, drop untrusted `Allow` entries, and the
  merged block is validated fail-closed. No repeat of the v1.4.0 FRB-05 missing-merge.
- **`internal/posture/view.go`** — pure, no I/O, derives the release-age figure from
  `policy.DefaultReleaseAgeConfig` so the view cannot drift from enforcement.
- **`internal/pkgparse` remote-source classification** — pure, conservative, and
  the compound-command/quote-aware segment splitter is robust (the documented
  intra-word-quote evasion is out of scope and predates this change).
- **`scanners.go` parseInt overflow guard** — sound (bounded before wrap).
- **Nudge removal** — complete: `internal/nudge`, `cmd/beekeeper/nudge.go`,
  `internal/check/nudge_adapter.go` deleted; remaining references are comments only;
  gateway/shim correlation paths intact; deprecated audit fields retained for corpus
  schema compatibility and documented as no-longer-populated.
- **`redact.go`** — `PosturePackage` redaction added; reason flows through Reason.
- `go vet ./internal/posture/... ./internal/check/... ./internal/policy/...
  ./internal/pkgparse/... ./cmd/beekeeper/...` is clean.

---

_Reviewed: 2026-06-22_
_Reviewer: Claude (gsd-code-reviewer), deep, adversarial stance_
