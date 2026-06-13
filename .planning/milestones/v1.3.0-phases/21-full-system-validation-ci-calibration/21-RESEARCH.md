# Phase 21: Full-System Validation & CI Calibration - Research

**Researched:** 2026-06-11
**Domain:** Go test-coverage gating, cross-platform CI matrices, golden-file conformance, Go native fuzzing, live-block e2e, manual validation registers
**Confidence:** HIGH (all findings verified against the live codebase at commit `04857cc`; no external package decisions required — this is a validation phase over existing Go code)

## Summary

Phase 21 is the pre-ship release gate for the Go core. It is almost entirely **codebase-internal work** — there are no new external dependencies to vet, no library version decisions, and no slopcheck surface. The "research" is therefore an audit of the existing test/CI/fuzz/e2e surface to pin the exact mechanism for each of the eight VAL requirements. I read every gap file named in CONTEXT plus the deny renderer, the Sentry rule engine, both CI workflows, the GoReleaser before-hook, the existing e2e tests, and the `/understand` knowledge-graph audit.

The single most important finding resolves OQ-1 (coverage-gate mechanism): **same-name sibling-test linkage is the wrong model** — 70 of 184 prod files have no same-name `_test.go` but ARE tested by a package-level test file (e.g. all 17 `internal/hooks/*.go` installers are tested by `hooks_test.go`, not `cursor_test.go`). The correct, tractable linkage is **package-level** (a prod file is covered iff its package contains ≥1 `_test.go`), plus a **file-level reason-coded allowlist** for the genuine stubs even inside tested packages. Under package-level linkage, **exactly one package has zero tests: `internal/version`**. The "8 surfaced Tier-A gaps" are real but are *file-level* gaps inside otherwise-tested packages — they need real tests, but they are not what a naive package walk would flag. This distinction is the spine of the VAL-01 plan.

**Primary recommendation:** Implement the coverage gate as a **Go test** (`TestCoverageManifest` in a new `internal/coveragegate` package or a repo-root `coverage_gate_test.go`) that walks `internal/` + `cmd/` with `go/build`, classifies each prod file as *package-tested* / *allowlisted-with-reason* / *UNACCOUNTED*, and fails on any UNACCOUNTED file. Ship the 8 file-level gap tests + a reason-coded `coverage-allowlist.txt`. For VAL-02 reuse and extend the **already-existing** `TestRenderDeny` table (it already covers all 17 harnesses) by converting it to golden-file `testdata/*.golden` with a `-update` flag and adding the installer-config half. For VAL-03 extend `ci.yml` (do not touch `web.yml`/`release.yml` triggers). For VAL-04 add `FuzzEvaluateEvent` to `internal/sentry`. For VAL-05 add a Claude-Code **`--hook` exit-2** e2e (the existing `e2e_test.go` proves exit-**1** default-mode blocks — the Phase-10 exit-2 `--hook` contract has no live e2e yet). VAL-06/07 are docs.

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01** — The Tier A/B/C validation model is the organizing principle. Every deliverable files under exactly one tier; the phase succeeds only if Tier-A is 100% gate-enforced, Tier-B is green in the CI matrix, and Tier-C is captured + signed in the register. No behavior may sit in a fourth "untested and unregistered" bucket. This model is what VAL-07 documents publicly.
- **D-02** — Coverage gate = **file-linked-test + reason-coded allowlist, NOT a coverage-% threshold**. Presence + accountability gate: every Go prod file has a linked test OR a documented, reason-coded no-test allowlist entry. Legitimate no-test entries: pure type/const/build-metadata files and platform stubs (`//go:build`-guarded fail-closed loader stubs, `*_other.go`/`*_windows.go` shims, generated `bpf_*_bpfel.go`). Each allowlist entry carries a reason code. Gate is a script (Go or pure-Python) runnable locally on Windows + wired into CI. Baseline ~184 prod / ~137 test files. The *contract* is locked; the *mechanism* is the researcher's/planner's call.
- **D-03** — Scope: **APPLY ALL SURFACED FIXES** (validation AND remediation). The 8 enumerated Tier-A files get real tests, and any defect a new test reveals is fixed in the same phase. Broader audit-surfaced gaps are hardened even absent a failing test where the audit flagged a real weakness. **Guardrail:** every fix must trace to an audit finding or a test that fails on current behavior; cosmetic refactors with no validation backing are out. A wave/phase split is acceptable and expected if breadth exceeds one context budget.
- **D-04** — 17-harness conformance = **local, deterministic golden-file deny contracts + installer-config assertions** (no live harness). Per target: (1) installer writes correct config (exact keys, idempotent re-install, backup-on-overwrite); (2) deny renderer emits the exact per-harness block contract (exit code + JSON/stdout) against a committed golden file. Honesty seams are first-class test cases: Hermes fail-open (exit 0 + `action:"block"` JSON) and Kilo/Trae UNGUARDED. Roster pinned against `internal/hooks` + `docs/harness-support-matrix.md` (count grew 15→17).
- **D-05** — CI matrix is **CI-validated, not locally validated** (Windows dev box). eBPF bytecode is CI-generated at build time and NEVER committed; the matrix runs `go generate ./internal/sentry/linux/...` in the Linux job; loader stubs fail closed when bytecode absent. Matrix YAML is statically authored + locally build-verified; its live GitHub run is confirmed at first push (repo has never been pushed).
- **D-06** — Fuzz gate = existing 7 targets + a **new Sentry-rule-evaluator target**, blocking. VAL-04's required list maps: policy / IPC-proto / catalog / MCP-message (= `gateway/parser`) / Sentry-rule-evaluator (NEW). Bounded `-fuzztime` per target; CI is the authoritative blocking gate.
- **D-07** — Live e2e is **Claude-Code-only**; the other 16 harnesses + the gated-22M-model LlamaFirewall e2e are a **manual register** (`docs/validation-register.md`) with exact steps, expected result, sign-off.
- **D-08** — Honesty docs are part of the gate (VAL-07). README harness count corrected (15→17 / 14→16). Validation posture (no-test allowlist + Tier A/B/C model) documented so the coverage claim is auditable.
- **D-09** — Process: discuss skipped, research-first, plan + execute INLINE on main; **Go subagents are fine** (no node/pnpm needed); executor on Windows so Tier-B paths are CI-validated. Multiple waves / a phase split are expected if breadth warrants; do not compress fidelity.
- **D-10** — Self-defense non-negotiable (VAL-08). The no-test allowlist **fails closed** on unjustified growth: a new prod file with no test and no reason-coded allowlist entry breaks the gate; adding an allowlist entry requires a reason code (not a bare path). Fuzz-gate failures block release. The validation infra is tamper-evident.

### Claude's Discretion
- Coverage-gate implementation language — Go test vs pure-Python file-walk (recommend Go; see OQ-1).
- Allowlist file format + location — e.g. `coverage-allowlist.txt`/`.go` with `path # reason-code` lines. Must be reason-coded.
- Golden-file format + harness for VAL-02 — table-driven Go + `testdata/*.golden` per harness + `-update`, vs another shape.
- Whether VAL-02's 17-harness suite and VAL-01's coverage gate share a wave or split.
- Exact CI matrix job decomposition — one matrix job w/ conditional steps vs per-OS jobs; how the 2-kernel Linux split is expressed (container vs runner image).
- The depth of remediation per surfaced gap (bounded by "must trace to an audit finding or failing test").
- Whether VAL-05 is a committed scripted test (`//go:build e2e` or shell) or a recorded register procedure (recommend scripted, given Phase-10 proved it live).

### Deferred Ideas (OUT OF SCOPE)
- macOS DNS Sentry, memory-read detection, Windows missing-PPID, legit-endpoint exfil — Phase-20 residual/v2; not reopened.
- The actual `git push` + `gh repo create` + live CI green badge — deferred v1.1.0 release runbook / milestone close, not Phase 21.
- The maintainer's HF-license completion for the gated-model e2e — a human gate; register sign-off may remain pending past phase close.
- SITE-03 live Vercel deploy; v1.1.0 Pollen release — both PARKED/separate.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| VAL-01 | Every Go prod file has a linked test OR a reason-coded allowlist entry; coverage gate enforces zero silent gaps; close the 8 surfaced Tier-A gaps | OQ-1 resolves the mechanism (package-level linkage + file allowlist); §"Tier-A Gap Test Contracts" gives each gap's unit-under-test + fail-closed assertions; only `internal/version` is a zero-test package |
| VAL-02 | Local 17-harness conformance suite: installer config (keys/idempotent/backup) + golden-file deny contract incl. Hermes fail-open + Kilo/Trae UNGUARDED | The 17-harness roster is pinned from `hooks.go allTargets`; `RenderDeny` (`deny_render.go`) already encodes all 17 contracts and `TestRenderDeny` already asserts them via substrings — convert to golden + add installer half; `hooks_test.go` already has the installer-assert pattern |
| VAL-03 | Cross-platform CI matrix (ubuntu-20.04/k5.4 + ubuntu-22.04/k5.15 + macos + windows): build (native+3 GOOS), vet, test, `-race`, eBPF gen+load, eslogger, ETW, peer-cred — all green | `ci.yml` already has most of this; §"CI Matrix Decomposition" lists exactly what is NEW vs present and reconciles the kernel-runner nuance |
| VAL-04 | Fuzz suite (policy, IPC-proto, catalog, MCP-message, Sentry-rule-evaluator) blocking CI gate | `EvaluateEvent` in `internal/sentry/rules.go` is the pure fuzzable entry point; §"Sentry Fuzz Target" gives signature, seed shape, CI wiring modeled on existing `FuzzEvaluate` |
| VAL-05 | Claude Code live e2e: canary `~/.ssh` + `~/.aws` read DENIED end-to-end (documented true-block reference) | EXISTING `e2e_test.go` proves exit-**1** default-mode block; the Phase-10 **`--hook` exit-2** contract has NO live e2e — §"VAL-05" specifies the new `--hook claude-code` exit-2 case |
| VAL-06 | `docs/validation-register.md`: 16 non-CC harness live-block procedures + gated-22M-model e2e, each steps/expected/sign-off | §"VAL-06 Register Schema" gives the table shape; gated-model entry sign-off stays pending on the human HF gate |
| VAL-07 | README count corrected (15→17 / 14→16); validation posture (allowlist + Tier A/B/C) documented | README.md says "15 agent harnesses" / "other 14"; harness-support-matrix.md is ALREADY 17 — only README + a posture section need editing |
| VAL-08 | Self-defense: coverage gate cannot be silently weakened (allowlist growth needs reason code, fails closed); fuzz failures block release | §"VAL-08 Self-Defense" specifies the fail-closed allowlist parser + a meta-test that proves an unreason-coded path is rejected |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Coverage-presence gate (VAL-01/08) | CI / build-tooling | local dev (`go test` on Windows) | Runs inside the same `go test ./...` CI already gates on; must also run on the Windows dev box |
| 17-harness deny + installer conformance (VAL-02) | `internal/check` + `internal/hooks` unit tier | — | Pure, deterministic, no live harness; golden files committed |
| Cross-platform build/test/race/eBPF/eslogger/ETW/peer-cred (VAL-03) | CI matrix (Tier B) | — | Platform/kernel/build-tag bound; CANNOT run on the single Windows box |
| Fuzz gate incl. Sentry evaluator (VAL-04) | CI fuzz jobs (Tier B blocking) | local `-tags fuzz` smoke (Tier A) | Authoritative gate is CI; local smoke is a dev convenience |
| Claude Code live block (VAL-05) | local/CI e2e (`//go:build e2e`) | — | Only CC is automatable; the real hook + real binary |
| 16-harness + gated-model live register (VAL-06) | manual / human (Tier C) | — | Irreducibly manual; each needs the real client installed |
| Honesty docs (VAL-07) | docs | — | README + posture section |

## Standard Stack

**No external packages are introduced by this phase.** Every mechanism uses the Go standard toolchain already in `go.mod` and existing CI actions already pinned in the workflows.

| Tool | Already present? | Purpose in Phase 21 |
|------|-----------------|---------------------|
| `go test` / `testing.F` (Go native fuzzing) | yes (Go 1.25, 7 fuzz targets live) | coverage gate test, golden tests, new `FuzzEvaluateEvent` |
| `go/build` + `go/parser` (stdlib) | yes (used by `imports_test.go`) | walk the package tree, read build tags, classify files for the coverage gate |
| `cilium/little-vm-helper@v0.0.21` | yes (`ci.yml` kernel-5.4/5.15 jobs) | the 2-kernel eBPF matrix already runs via VM images |
| `bpf2go` via `go generate ./internal/sentry/linux/...` | yes (`ci.yml` Linux step + `.goreleaser.yaml` before-hook) | eBPF generate+load; bytecode never committed |
| `actions/checkout@v4`, `actions/setup-go@v5` | yes | matrix job scaffolding |
| `eslogger` (macOS), ETW (`tekert/golang-etw`, no CGO) | yes (`test-eslogger-fields` job; ETW in `internal/sentry/windows`) | platform-bound Sentry validation |

### Alternatives Considered (mechanism, not packages)

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Go coverage-gate test | Pure-Python `web/tests/*_spec.py`-style file-walk | Python matches the web house-style BUT cannot read Go build constraints, runs OUTSIDE `go test ./...` (a second CI step + a Python dep on the Go matrix), and cannot self-test via Go. **Recommend Go** (D-09: Go subagents are fine; keeps the gate in the build the matrix already runs). |
| Golden-file `testdata/*.golden` + `-update` | Keep the existing in-line substring asserts in `TestRenderDeny` | Substring asserts already pass for all 17 but are weaker than byte-exact golden files and don't catch field-ordering / extra-field regressions. D-04 explicitly asks for golden files. **Recommend golden** (regenerate via `-update`, commit, then assert byte-equality). |
| One matrix job + conditional steps | Separate per-OS jobs | The existing `ci.yml` already mixes both (matrix `test` job + dedicated kernel/eslogger jobs). **Keep the hybrid** — extend the matrix `test` job, keep kernel/eslogger as dedicated jobs. |

**Installation:** none.

## Package Legitimacy Audit

> Not applicable. Phase 21 installs **zero external packages**. All work uses the Go standard library, the existing `go.mod` dependency set, and CI actions already pinned in `.github/workflows/`. slopcheck/registry verification is moot — there is no install surface. (Confirmed by reading both workflows and `.goreleaser.yaml`: no new `go install`, no new `uses:` action beyond those already pinned.)

## Architecture Patterns

### System Architecture Diagram (Phase 21 validation flow)

```
                          ┌─────────────────────────────────────────────┐
   developer commit  ───► │  go test ./...  (Windows dev box, Tier A)    │
                          │  ├─ TestCoverageManifest  (VAL-01/08)        │──► FAIL if any
                          │  │    walks internal/+cmd/ via go/build      │    UNACCOUNTED
                          │  │    ├─ pkg has _test.go?  → covered         │    prod file
                          │  │    ├─ in reason-coded allowlist? → ok      │
                          │  │    └─ else → UNACCOUNTED → gate fails      │
                          │  ├─ TestRenderDeny (golden, 17 harnesses)     │  VAL-02 deny half
                          │  ├─ Test{Install*} (config/idempotent/backup) │  VAL-02 installer half
                          │  └─ 8 new Tier-A gap tests                    │  VAL-01 remediation
                          └─────────────────────────────────────────────┘
                                              │ push (first push only)
                                              ▼
        ┌──────────────────────── ci.yml matrix (Tier B, CI-only) ───────────────────────┐
        │ matrix test job: ubuntu-latest / macos-latest / windows-latest                 │
        │   build (native) · build 3×GOOS · vet · go test -race (CGO=1) · peer-cred       │
        │ Linux step: apt clang/llvm/libbpf · go generate sentry/linux · eBPF load        │
        │ test-sentry-kernel-5-4  (little-vm-helper 5.4 image)   ── eBPF on k5.4           │
        │ test-sentry-kernel-5-15 (little-vm-helper 5.15 image)  ── eBPF on k5.15          │
        │ test-eslogger-fields (macos-latest)  ── live eslogger schema                     │
        │ fuzz / fuzz-ipc / fuzz-llamafirewall / + FuzzEvaluateEvent (Sentry)  ◄ VAL-04    │
        │ release-gate: needs[all] ── blocking                                             │
        └────────────────────────────────────────────────────────────────────────────────┘
                                              │
                          ┌───────────────────┴────────────────────┐
                          ▼                                         ▼
       VAL-05  go test -tags e2e (Claude Code --hook exit 2)   VAL-06 docs/validation-register.md
       canary ~/.ssh + ~/.aws DENIED end-to-end                16 harnesses + gated-22M e2e
       (the documented true-block reference)                   (manual steps/expected/sign-off, Tier C)
```

### Pattern 1: Package-level coverage linkage + file-level reason-coded allowlist (VAL-01)
**What:** Walk every `*.go` non-`_test.go` file under `internal/` and `cmd/`. A file is ACCOUNTED if (a) its directory contains ≥1 `_test.go` (package-tested) OR (b) its path appears in `coverage-allowlist.txt` with a reason code. Any other file is UNACCOUNTED → gate fails.
**When to use:** This is the VAL-01 gate. It is deliberately NOT same-name-sibling linkage (that flags 70/184 false positives).
**Why this shape:** Verified by codebase walk — only `internal/version` has a zero-test package; all 17 `internal/hooks/*.go` installers are covered by `hooks_test.go`. Same-name linkage would force ~70 redundant allowlist entries and bury the real signal.
**Example (the linkage predicate):**
```go
// Source: pattern derived from internal/sentry/imports_test.go (go/parser walk)
// covered(file) := pkgHasTest(dir(file)) || allowlisted(file)
// gate fails iff exists prod file where !covered(file)
```

### Pattern 2: Golden-file deny conformance with `-update` (VAL-02)
**What:** Table-driven test over all 17 `HarnessID`s. For each (harness, block-decision) render `RenderDeny`, marshal `{ExitCode, Stdout, Stderr}` to a stable form, compare byte-exact to `testdata/deny/<harness>.golden`. A `-update` flag regenerates.
**When to use:** Replaces/augments the existing substring `TestRenderDeny`.
**Example:**
```go
// Source: existing internal/check/deny_render_test.go (already covers all 17;
// convert assert style from strings.Contains → golden byte-compare)
var update = flag.Bool("update", false, "regenerate golden files")
// for each harness: got := canonical(RenderDeny(h, blockDecision))
//   if *update { os.WriteFile(golden, got) } else { assertEqual(read(golden), got) }
```

### Pattern 3: Pure-engine fuzz target (VAL-04)
**What:** Fuzz the pure `EvaluateEvent` Sentry entry point; assert it never panics and returns only valid severities.
**Example:** see §"Sentry Fuzz Target".

### Anti-Patterns to Avoid
- **Same-name sibling-test linkage** for the coverage gate — produces 70 false positives, defeats the signal. Use package-level linkage.
- **A coverage-% threshold** — explicitly forbidden by D-02. The gate is presence+accountability, not a number.
- **Touching `web.yml` / `release.yml` triggers** — D-05 + the Phase-19 isolation precedent. Extend `ci.yml` only; do not regress the `paths-ignore` web isolation.
- **Committing eBPF bytecode** — CLAUDE.md locked. Generate in CI via `go generate`; stubs fail closed when absent.
- **Asserting a live harness in CI** — only Claude Code is live-testable, and only via the e2e build tag. The other 16 are golden-contract + manual register.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Walk Go files + read build tags | a regex/string scan of `//go:build` | `go/build.Context.MatchFile` / `go/parser` | The codebase already uses `go/parser` (`imports_test.go`); build-constraint parsing has edge cases (combined tags, file-name suffixes like `_windows.go`) that `go/build` handles correctly |
| Golden-file diffing | bespoke byte-compare loops | `testing` + a committed `testdata/` + a `-update` flag | Go's idiomatic golden pattern; `-update` makes regeneration trivial and reviewable |
| Fuzzing | a custom random input loop | Go native `testing.F` | 7 targets already use it; CI already wires `-fuzz`/`-fuzztime` |
| 2-kernel eBPF matrix | spinning custom VMs | `cilium/little-vm-helper@v0.0.21` | Already in `ci.yml` for the 5.4/5.15 split |
| Peer-cred test harness | a fake socket | real `net.Listen("unix")` + `SO_PEERCRED`/`LOCAL_PEERCRED` | The production code path is the test; CI Linux/macOS runners support it |

**Key insight:** Phase 21 is overwhelmingly *assembly of existing primitives*. The new code is: 1 coverage-gate test + allowlist, 8 small gap tests, golden-file conversion of an existing table test, 1 Sentry fuzz target, 1 `--hook` e2e case, and CI-YAML/Markdown edits. Resist building anything novel.

## Tier-A Gap Test Contracts (VAL-01 / D-03)

The "8 surfaced Tier-A gaps" are *file-level* gaps inside otherwise-tested packages. Each row gives the unit under test, the must-assert behavior (esp. fail-closed paths), and whether a production defect is visible (a D-03 remediation candidate).

| Gap file | Unit under test | Must assert | Defect / remediation candidate? |
|----------|-----------------|-------------|---------------------------------|
| `internal/ipc/server.go` (`//go:build linux\|\|darwin`) | `NewServer` / `Serve` / `Close` | socket created at 0600, owner UID set; pre-existing socket removed; **unauthenticated peer connection is closed before handler runs** (fail-closed peer-auth); `Serve` returns on ctx cancel | No obvious defect; the test is the missing artifact. **Tier-B** to run (needs Unix socket) — runs in the CI matrix Linux/macOS jobs, NOT on Windows. |
| `internal/ipc/client.go` (`//go:build linux\|\|darwin`) | `Connect` / `SendCommand` / `ReadResponse` | round-trip encode/decode against a real `NewServer`; **write/read deadlines applied** (timeout path returns error, does not hang) | None expected. Tier-B (Unix). Pairs with the server test. |
| `internal/ipc/peer_linux.go` (`//go:build linux`) | `verifyPeerUID` (SO_PEERCRED) | same-UID connection passes; a mismatched expected-UID is **rejected** | Tier-B Linux-only; CI peer-cred step. |
| `internal/ipc/peer_darwin.go` (`//go:build darwin`) | `verifyPeerUID` (LOCAL_PEERCRED/Xucred) | same as Linux | Tier-B macОS-only. `pipe_windows_test.go` already exists for the Windows half. |
| `internal/check/sanity.go` | `resolveCatalogHealthy` (delegates to `catalog.ResolveHealthy`) | **all four are 4-line identical delegators** to `catalog.ResolveHealthy`. The real logic lives in `internal/catalog/health.go`. Test `ResolveHealthy` once (in `catalog`): empty cacheDir→true; missing state.json→true (**fail-OPEN on read by design**, documented); bumblebee `Degraded:true`→false; bumblebee absent→true. Optionally a thin per-package delegation test. | **Not a defect** — the fail-open-on-read is a documented, security-reasoned trade-off (health.go lines 40-47). Remediation candidate is only the missing TEST, not the behavior. The 4 delegators are ideal **allowlist candidates** (`reason: thin-delegator`) if a per-package test is judged redundant — but D-03 leans toward writing the `catalog.ResolveHealthy` test that covers all four. |
| `internal/watch/sanity.go` | (identical delegator) | as above | as above |
| `internal/scan/sanity.go` | (identical delegator) | as above | as above |
| `internal/gateway/sanity.go` | (identical delegator) | as above | as above |
| `internal/editorinit/lookup.go` | `execLookPath` (1-line wrapper over `exec.LookPath`) | This is a deliberate seam so `detect.go` can override `lookPath` in tests without importing `exec`. The wrapper itself is trivial. **Best treated as an allowlist entry** (`reason: exec-seam-stub`) OR a one-line test that asserts it finds a known binary (e.g. `go`). Low value; allowlist is honest. | No defect. Allowlist candidate. |
| `internal/hooks/protected.go` | `BeekeeperHookMarkers` / `HookConfigFiles` | markers include `"beekeeper check"` + `"beekeeper audit-record"`; `HookConfigFiles(home)` returns one path per supported harness and every path is rooted at `home` (no empty/absolute-escape) | No defect; pure metadata. Real test (cheap, meaningful) preferred over allowlist. |
| `internal/tui/model.go` (model logic gap) | `computeStatus`, `recentAuditRecords`, `runPaletteSelection` (per KG line-ranges) | `model_test.go` EXISTS and covers mode transitions/palette/incident. The gap is **specific uncovered functions**: assert `computeStatus` derives the right status from a synthetic `HealthState` + recent-critical record; assert `recentAuditRecords` reads only the bounded tail. | No defect; targeted coverage of the de-mocked status/audit-tail logic (the v1.3.0-seed real-data work). |

**Planner guidance:** Of the 8 "confirmed test-less" files, 4 (`*/sanity.go`) collapse into one `catalog.ResolveHealthy` test, and 1 (`editorinit/lookup.go`) is a legitimate allowlist stub. So the real new-test surface is: `catalog.ResolveHealthy` (covers the 4 sanity delegators), `ipc` server+client (Tier-B), `ipc` peer_linux/peer_darwin (Tier-B), `hooks/protected.go`, and the `tui/model.go` function-level gaps. This is a sane single-wave VAL-01 deliverable.

## 17-Harness Conformance (VAL-02 / D-04)

### The pinned roster (from `internal/hooks/hooks.go` `allTargets` — VERIFIED)

17 targets total, 16 non-Claude-Code:

| # | Target const | `--hook`/installer name | Deny family (from `deny_render.go`) | Notes |
|---|--------------|------------------------|--------------------------------------|-------|
| 1 | `TargetClaudeCode` | `claude-code` | A nested `hookSpecificOutput` | exit 2; **the only live-verified harness (VAL-05)** |
| 2 | `TargetCursor` | `cursor` | C `permission`/user_message/agent_message | exit 2 |
| 3 | `TargetCodex` | `codex` | A | exit 2 |
| 4 | `TargetAugment` | `augment` | A | exit 2 |
| 5 | `TargetCodeBuddy` | `codebuddy` | A | exit 2 |
| 6 | `TargetQwen` | `qwen` | A | exit 2 |
| 7 | `TargetCopilot` | `copilot` | B flat `permissionDecision` | exit 2 |
| 8 | `TargetAntigravity` | `antigravity` | E dual `decision`+`permissionDecision` | exit 2 |
| 9 | `TargetGemini` | `gemini` | D `decision`/`reason` | exit 2 |
| 10 | `TargetWindsurf` | `windsurf` | H exit-2-only (no stdout JSON) | exit 2 |
| 11 | `TargetHermes` | `hermes` | G `action:"block"` JSON | **exit 0** — fail-open seam (D-04) |
| 12 | `TargetCline` | `cline` | F `cancel:true`/errorMessage | exit 2; **!windows installer** (macOS/Linux only) |
| 13 | `TargetContinue` | `continue` | gateway-guide (no file write) | Tier-3; printed config |
| 14 | `TargetOpenCode` | `opencode` | H exit-2-only | exit 2; plugin throws |
| 15 | `TargetOpenClaw` | `openclaw` | gateway-guide | Tier-3; printed config |
| 16 | `TargetKilo` | `kilo` | H exit-2-only (`RenderDeny`) + gateway-guide installer | **Tier-3 UNGUARDED** native tools |
| 17 | `TargetTrae` | `trae` | H exit-2-only (`RenderDeny`) + gateway-guide installer | **Tier-3 UNGUARDED** native tools |

> Note the two-axis split: the **installer** has 4 `gatewayTargets` (continue, openclaw, kilo, trae) that print config rather than write a file; the **deny renderer** (`RenderDeny`) maps 15 `HarnessID`s (it has no Continue/OpenClaw consts because those never invoke `beekeeper check --hook`). VAL-02 must assert both axes: installer-config for all 17, deny-contract for the 15 that have a `HarnessID` + the fail-closed default for unknown.

### Deny-contract half (already 95% built)
`internal/check/deny_render.go` already encodes every contract and `internal/check/deny_render_test.go::TestRenderDeny` already asserts all 17 cases (Families A-H + Hermes exit-0 + unknown-harness fail-closed exit-2). VAL-02 work = **convert the substring asserts to byte-exact golden files** (`testdata/deny/<harness>.golden`) with a `-update` flag, and add explicit golden rows naming the honesty seams (Hermes exit-0+`action:block`; Kilo/Trae exit-2+no-JSON-stdout, documented-UNGUARDED).

### Installer-config half (pattern already established)
`internal/hooks/hooks_test.go` already demonstrates the assertion shape on Claude Code: hooks key present, **backup file created** (`*.beekeeper-backup-*`), **idempotent** (exactly 1 `PreToolUse` entry after 2nd install), **other keys preserved** (`theme`). Most installers already have a `TestInstall<Harness>` (Antigravity, Augment, Cline, CodeBuddy, Codex, Copilot, Cursor, Gemini, Hermes, OpenCodePlugin, Qwen, Windsurf, ClaudeCode + dispatch/gateway/unknown). VAL-02 = ensure **all 17** are covered uniformly: same four assertions (keys/idempotent/backup/preserve) for file-writers; printed-config + no-file-written assertion for the 4 gateway targets; the `!windows` explicit-error assertion for Cline on Windows; and the `TestInstallGatewayTargetKiloTraeUNGUARDED` honesty assertion already exists — extend its golden-ness.

**Backup-on-overwrite:** verified in `hooks.go::backupSettings` (copies to `path+".beekeeper-backup-<ts>"` at 0o600, no-op if file absent). Idempotency is the second-install-yields-one-entry contract.

## CI Matrix Decomposition (VAL-03 / D-05)

### What is ALREADY in `ci.yml` (do not re-add — verify/extend)
- matrix `test` job over `[ubuntu-latest, macos-latest, windows-latest]`: `go mod verify`, build (`-trimpath -buildvcs=false`), **`go test -v -race` with `CGO_ENABLED:1`**, `go vet`, `go mod tidy` drift check. **`-race`/CGO is already enforced here** (CONTEXT framed it as a deferred item; it is in fact present — confirm and keep).
- Linux-only eBPF deps + `go generate ./internal/sentry/linux/...` (apt clang/llvm/libelf-dev/libbpf-dev/linux-headers/bpftool).
- `test-sentry-kernel-5-4` and `test-sentry-kernel-5-15`: **both run on the `ubuntu-22.04` runner but use `cilium/little-vm-helper` VM images `5.4-main` / `5.15-main`** — this is how the 2-kernel split is expressed (VM image, NOT runner OS). **Reconcile CONTEXT's "ubuntu-20.04 runner for k5.4" framing: the existing, working approach is little-vm-helper images, which is more reliable than relying on a runner's host kernel.** Recommend keeping the VM-image approach.
- `test-eslogger-fields` (macos-latest): live eslogger schema validation.
- `fuzz`, `fuzz-ipc`, `fuzz-llamafirewall` jobs + `release-gate: needs[...]`.

### What is NEW for VAL-03/VAL-04 (the actual plan delta)
1. **eBPF *load* (not just generate)** — the current Linux step generates bytecode and builds; the kernel-5.4/5.15 jobs run `TestProbeTier`. Confirm these exercise an actual `ebpf` load path; if a dedicated load-smoke is missing, add it inside the existing kernel jobs (no new job).
2. **3×GOOS cross-compile build** — the matrix builds *native* only. Add `GOOS=linux/darwin/windows go build ./...` (build-only, no test) so cross-compilation is gated. Cheapest as one extra step in the matrix `test` job (or a small dedicated `cross-build` job).
3. **Explicit Unix peer-cred test** — the new `ipc` server/peer tests (VAL-01) ARE the peer-cred validation; ensure they run in the Linux + macOS matrix legs (they are `//go:build linux||darwin`, so `go test ./internal/ipc/...` already includes them on those OSes — no new job, just confirm coverage and that the kernel jobs or the matrix run them).
4. **ETW (Windows) Sentry test** — confirm `internal/sentry/windows` tests run in the windows-latest matrix leg (they should via `go test ./...`); if there's a live-ETW gate analogous to `test-eslogger-fields`, mirror it.
5. **`FuzzEvaluateEvent` job** — add a `fuzz-sentry` job (or a step in `fuzz`) modeled on `fuzz`/`fuzz-ipc`, added to `release-gate.needs` (VAL-04 blocking).

**YAML shape recommendation (illustrative — not to be copied verbatim by the planner):**
```yaml
# extend the existing matrix `test` job with a cross-compile step:
- name: Cross-compile (3 GOOS, build-only)
  run: |
    for os in linux darwin windows; do GOOS=$os go build -trimpath -buildvcs=false ./...; done
  shell: bash
# new blocking fuzz job, added to release-gate.needs:
fuzz-sentry:
  runs-on: ubuntu-22.04
  steps: [checkout, setup-go,
    {run: go test -tags fuzz -run FuzzEvaluateEvent -fuzz FuzzEvaluateEvent -fuzztime 30s ./internal/sentry/}]
```
Then `release-gate.needs` gains `fuzz-sentry`.

## Sentry Fuzz Target (VAL-04 / D-06)

**Fuzzable entry point:** `sentry.EvaluateEvent(event SentryEvent, state *RuleState, tree map[uint32]ProcessNode, inventory InventorySnapshot, cfg RuleConfig, baseline BaselineState, now time.Time) []SentryAlert` (in `internal/sentry/rules.go`). It is **pure** (no I/O, locked by `TestRulesImportsArePure`), so it is an ideal fuzz target.

**Fuzz strategy:** fuzz the variable surface (the `SentryEvent`) by feeding fuzz bytes into the event's string/IP/PID fields and the `EventKind`, with a fixed-but-initialized `RuleState` (`NewRuleState()`), a small synthetic process tree, and default `RuleConfig`. The fuzzer must not panic and must only ever return alerts whose `Severity` is `"critical"` or `"high"` (the two documented values).

**Seed corpus shape (synthetic event/process-tree records):**
```go
//go:build fuzz
// Source: modeled on internal/policy/fuzz_test.go::FuzzEvaluate
func FuzzEvaluateEvent(f *testing.F) {
    f.Add(uint8(1), uint32(1000), uint32(1), "code", "/home/u/.aws/credentials") // FileAccess cred read
    f.Add(uint8(0), uint32(1001), uint32(1), "gh", "")                            // ProcessCreate cred CLI
    f.Add(uint8(2), uint32(1002), uint32(1), "claude", "")                        // NetworkConnect
    f.Add(uint8(3), uint32(1003), uint32(1), "node", "/home/u/.config/systemd/user/x.service") // FileWrite persistence
    f.Add(uint8(255), uint32(0), uint32(0), "", "")                               // out-of-range kind / zero
    f.Fuzz(func(t *testing.T, kind uint8, pid, ppid uint32, exe, path string) {
        ev := SentryEvent{Kind: EventKind(kind), PID: pid, PPID: ppid, Exe: exe, FilePath: path}
        tree := map[uint32]ProcessNode{pid: {PID: pid, PPID: ppid, Exe: exe}}
        alerts := EvaluateEvent(ev, NewRuleState(), tree, InventorySnapshot{}, RuleConfig{}, BaselineState{}, time.Now())
        for _, a := range alerts {
            switch a.Severity { case "critical", "high": default:
                t.Errorf("invalid severity %q from kind=%d", a.Severity, kind) }
        }
    })
}
```
**CI wiring:** new `fuzz-sentry` job (`-fuzztime 30s`), added to `release-gate.needs` so a discovered panic blocks release (VAL-08). Local smoke: `go test -tags fuzz -run FuzzEvaluateEvent -fuzz FuzzEvaluateEvent -fuzztime 5s ./internal/sentry/`. (Confirm `BaselineState` zero-value is safe to pass; if not, seed it via the existing `baseline_test.go` helper — read it during planning.)

## VAL-05 — Claude Code Live e2e (the real gap)

**Critical finding:** `internal/check/e2e_test.go::TestE2ELiveBinary` (the v1.2.0 release gate) builds the real binary and proves the **default-mode** SPATH credential-read block returns **exit 1** + audit `decision:"block"`. But Phase 10 changed the *hook* contract to **exit 2 via `--hook <harness>`** (see `deny_render.go::exitHookBlock = 2` and `main.go` `--hook` wiring). **There is no live e2e proving the `--hook claude-code` exit-2 path** — that is exactly the VAL-05 deliverable.

**Recommendation (scripted, per D-09):** add a sub-case (or sibling test) under `//go:build e2e` that:
1. builds the real binary (reuse the existing `newHome`/`seedCatalog`/`runCase` helpers),
2. drives `beekeeper check --hook claude-code` with stdin canary reads of `~/.ssh/id_rsa` AND `~/.aws/credentials`,
3. asserts **exit code 2** (the hook deny contract) AND stdout contains `"permissionDecision":"deny"` (Family A) AND the audit record `decision:"block"`.

This makes the documented true-block reference exit-2-accurate and matches the Phase-10 live proof (`~/.ssh` + `~/.aws` DENIED). Keep the existing exit-1 default-mode case too (it validates the non-hook path). Run in CI via a `-tags e2e` step (the existing e2e job pattern) — but note the build tag means it is NOT in the default `go test ./...`, so it must be explicitly invoked.

## VAL-06 — Register Schema

`docs/validation-register.md` — one row per non-Claude-Code harness (16) + one for the gated-22M-model LlamaFirewall e2e:

```markdown
### <Harness> (Tier <1|2|3>)
- **Prereq:** <real client installed + version; config dir>
- **Install:** `beekeeper hooks install --target <name>`  (or: printed MCP config for gateway targets)
- **Drive:** <exact tool-call that should block — e.g. read ~/.aws/credentials>
- **Expected:** <exit 2 + deny JSON family X> | (Hermes: exit 0 + {"action":"block"}) | (Kilo/Trae: native UNGUARDED — only MCP-routed call blocks)
- **Result:** ☐ blocked  ☐ allowed (FAIL)  ☐ N/A-unguarded
- **Verified by / date:** ____________
```

Plus the gated-model entry:
```markdown
### LlamaFirewall gated-22M-model e2e (Tier C — human-gated)
- **Prereq:** accept Llama-Prompt-Guard-2 license on huggingface.co; `huggingface-cli login`; `beekeeper llamafirewall install`; set BEEKEEPER_LLMF_E2E=1
- **Run:** go test -tags e2e -run TestLlamaFirewallE2E ./internal/llamafirewall/
- **Expected:** benign→allow, injection→injection, unsafe-code→unsafe (CodeShield), crash→fail-closed (never clean)
- **Result / sign-off:** PENDING human HF-license gate (honest: may remain pending past phase close — D-07/deferred)
```

Source the per-harness deny families from the §17-Harness table above and the honesty seams from `docs/harness-support-matrix.md` (already accurate). The gated-model test (`internal/llamafirewall/e2e_test.go`) already exists and is double-gated by `e2e` tag + `BEEKEEPER_LLMF_E2E=1`.

## VAL-07 — Honesty Docs

- **README.md** (VERIFIED stale): line 32 "supports **15** agent harnesses"; line 66 "the other **14** harnesses". Correct to **17 / 16**. The Tier-1 list on README line 40 lists 10 Tier-1 harnesses — re-verify it against the matrix (10 Tier-1 + 3 Tier-2 [Hermes/Cline/OpenCode] + 4 Tier-3 [Kilo/Trae/Continue/OpenClaw] = 17) and fix counts.
- **`docs/harness-support-matrix.md`** is ALREADY 17 (header "17 targets total", updated 2026-06-10) — **no count change needed**, it is the source of truth README must match.
- **Validation posture** — document the Tier A/B/C model + the no-test allowlist mechanism (reason-code taxonomy) either as a new section in `docs/validation-register.md` or a `docs/validation-posture.md`, so an external reader can audit the coverage claim (D-08). Keep consistent with `THREAT-MODEL.md §8` honesty spine.

## VAL-08 — Self-Defense (the phase's self-defense surface)

The coverage allowlist is the new tamper-evident surface (CLAUDE.md "every phase includes self-defense").
- **Fail-closed parse:** the gate's allowlist reader must reject any line that is a bare path with no reason code (e.g. require `path<TAB># reason: <code>` or `path  REASON_CODE`). A path present without a recognized reason code is treated as UNACCOUNTED → gate fails.
- **Reason-code taxonomy (recommended, closed set):** `generated-bpf` (`bpf_*_bpfel.go`), `platform-stub` (`*_other.go`/`*_windows.go`/`*_unix.go` fail-closed shims with no logic), `type-only` (pure type/const/build-metadata, e.g. `policy/types.go`, `version/version.go`, `scan/pollen_version.go`), `exec-seam-stub` (`editorinit/lookup.go`), `thin-delegator` (the 4 `sanity.go` if not covered via `catalog.ResolveHealthy`), `gen-directive` (`sentry/linux/gen.go`). Any reason code outside the closed set → gate fails.
- **Meta-test:** a `TestAllowlistFailsClosed` that feeds the parser a bare path + an unknown-reason path and asserts both are rejected — proving an agent/careless commit cannot silently lower the bar.
- **Fuzz failures block release:** `FuzzEvaluateEvent` added to `release-gate.needs` (alongside the existing fuzz jobs).

## Common Pitfalls

### Pitfall 1: Same-name sibling-test linkage
**What goes wrong:** 70/184 prod files lack a same-name `_test.go` but are package-tested; a naive linkage flags them all, drowning the real `internal/version` gap and the 8 file-level gaps.
**How to avoid:** package-level linkage + file allowlist. Verified: only `internal/version` is a zero-test package.

### Pitfall 2: Mistaking the exit-1 default-mode block for the exit-2 hook contract
**What goes wrong:** the existing e2e asserts exit 1; VAL-05/Phase-10's hook contract is exit 2 via `--hook`. Asserting the wrong code passes a test that doesn't prove the shipped hook behavior.
**How to avoid:** VAL-05 must drive `--hook claude-code` and assert exit **2** + Family-A deny JSON.

### Pitfall 3: Misexpressing the 2-kernel split as runner OS
**What goes wrong:** CONTEXT says "ubuntu-20.04/k5.4"; the working `ci.yml` uses `cilium/little-vm-helper` VM **images** on a 22.04 runner. Switching to host-kernel runners is fragile (GitHub runner kernels drift).
**How to avoid:** keep the little-vm-helper image approach (`5.4-main` / `5.15-main`).

### Pitfall 4: Entangling Go CI with web CI
**What goes wrong:** editing `web.yml` or removing `ci.yml`'s `paths-ignore: web/**` regresses the Phase-19 isolation.
**How to avoid:** extend `ci.yml` only; never touch the web triggers.

### Pitfall 5: Committing eBPF bytecode
**What goes wrong:** checking in `bpf_*_bpfel.go` generated objects violates CLAUDE.md and breaks reproducibility.
**How to avoid:** generate via `go generate` in the Linux CI job; the loader stubs fail closed when absent (already the design).

### Pitfall 6: Build-tagged tests silently skipped on the Windows dev box
**What goes wrong:** `ipc` server/client/peer tests are `//go:build linux||darwin` — they never run locally on Windows, so a green local `go test ./...` does NOT prove them. e2e tests need `-tags e2e`.
**How to avoid:** mark these Tier-B/e2e explicitly; the coverage gate must still ACCOUNT for the files (package-tested), and the register/CI notes must state they are CI-validated only.

## Runtime State Inventory

> Phase 21 is additive test/CI/docs work, not a rename/migration. No stored data, live-service config, OS-registered state, secrets, or build artifacts carry a string that this phase changes.
- **Stored data:** None — verified; no datastore keys touched.
- **Live service config:** None — `ci.yml` is the only CI surface extended; no external dashboards.
- **OS-registered state:** None.
- **Secrets/env vars:** `HF_TOKEN` / `BEEKEEPER_LLMF_E2E` are READ by the existing gated e2e; this phase documents them in the register, does not rename them.
- **Build artifacts:** eBPF bytecode is CI-generated, never committed (unchanged); no stale artifacts introduced.

## Validation Architecture

> This phase IS the validation phase — "the gate tests the gate." The deliverables validate themselves as follows.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` + native fuzzing (`testing.F`), Go 1.25 |
| Config file | none (Go convention); CI in `.github/workflows/ci.yml` |
| Quick run command | `go test ./...` (Windows dev box, Tier A) |
| Full suite command | `go test -race ./...` + `go test -tags fuzz -fuzz ... ` + `go test -tags e2e ...` (CI matrix, Tier B/e2e) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| VAL-01 | every prod file accounted | unit (gate) | `go test -run TestCoverageManifest ./...` | ❌ Wave 0 (new) |
| VAL-01 | 8 Tier-A gaps tested | unit | `go test ./internal/{ipc,catalog,hooks,tui}/...` | ❌ new tests (ipc Tier-B) |
| VAL-02 | 17-harness deny golden | unit | `go test -run TestRenderDeny ./internal/check/` | ✅ exists (convert to golden) |
| VAL-02 | installer config/idempotent/backup | unit | `go test -run TestInstall ./internal/hooks/` | ✅ mostly exists (fill 17) |
| VAL-03 | matrix build/test/race/eBPF/eslogger/ETW/peer-cred | CI matrix | (CI only) | ✅ ci.yml (extend) |
| VAL-04 | Sentry rule fuzz | fuzz | `go test -tags fuzz -fuzz FuzzEvaluateEvent ./internal/sentry/` | ❌ new target |
| VAL-05 | CC `--hook` exit-2 canary block | e2e | `go test -tags e2e -run TestE2E... ./internal/check/` | ⚠ e2e exists, exit-2 `--hook` case missing |
| VAL-06 | live-block register | manual | (human) | ❌ new doc |
| VAL-07 | honesty counts | doc | `grep -c` (or accuracy-style check) | ⚠ README edit |
| VAL-08 | allowlist fails closed | unit (meta) | `go test -run TestAllowlistFailsClosed ./...` | ❌ new |

### Sampling Rate
- **Per task commit:** `go test ./...` (Windows-local Tier A) + `go vet ./...`
- **Per wave merge:** full Tier-A suite green on Windows
- **Phase gate:** the matrix YAML statically authored + locally build-verified (`go build` of nothing-new; the YAML cannot run live pre-push — honest, per D-05 / Phase-19 precedent); fuzz smoke green locally; e2e green locally for Claude Code.

### Wave 0 Gaps
- [ ] `coverage_gate_test.go` (or `internal/coveragegate/`) + `coverage-allowlist.txt` — VAL-01/08
- [ ] `internal/catalog/health_test.go` (covers the 4 `sanity.go` delegators) — VAL-01
- [ ] `internal/ipc/server_test.go` + `client_test.go` + peer tests (Tier-B `//go:build linux||darwin`) — VAL-01
- [ ] `internal/hooks/protected_test.go`; `internal/tui` function-level model tests — VAL-01
- [ ] `internal/check/deny_render_test.go` golden conversion + `testdata/deny/*.golden` — VAL-02
- [ ] `internal/sentry/fuzz_test.go` (`FuzzEvaluateEvent`) — VAL-04
- [ ] `internal/check/e2e_test.go` `--hook claude-code` exit-2 case — VAL-05
- [ ] `docs/validation-register.md`; README count fix; posture section — VAL-06/07
- [ ] `ci.yml` matrix extensions (cross-build, `fuzz-sentry`, eBPF load) — VAL-03/04

## Security Domain

> `security_enforcement` posture: this phase's entire purpose is security validation. ASVS framing below maps the validation surface.

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V1 Architecture / SDLC | yes | the coverage gate + Tier A/B/C model + register IS the auditable assurance artifact |
| V5 Input Validation | yes | fuzz targets (policy/IPC/catalog/MCP/**Sentry**) prove parsers don't panic / fail open |
| V10 Malicious Code | yes | the 17-harness deny contract + live e2e prove a hijacked agent is blocked |
| V14 Config / Build | yes | reproducible build flags, eBPF-never-committed, `-race`/CGO, SLSA/cosign unregressed |

### Known Threat Patterns for this validation surface
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Silent coverage erosion (agent lowers the bar) | Tampering | allowlist fails closed on bare/unknown-reason paths (VAL-08 meta-test) |
| Fail-open deny contract (Hermes/Windsurf seam) | Repudiation/EoP | golden-file asserts exact exit code + JSON per harness incl. fail-open seams |
| Parser panic on hostile event/tool-call | DoS / fail-open | native fuzzing as a blocking release gate |
| Peer-spoofing the IPC socket | Spoofing/EoP | `SO_PEERCRED`/`LOCAL_PEERCRED` peer-UID tests; 0600 owner-only socket |

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| exit-1 hook block | exit-2 `--hook` deny contract | Phase 10 (2026-06-05) | VAL-05 must assert exit 2, not the exit-1 in the existing e2e default case |
| 15-harness count in README | 17 targets (`allTargets`) | matrix updated 2026-06-10 | VAL-07 README correction (matrix already correct) |
| coverage = %-threshold (common industry default) | presence + reason-coded allowlist | D-02 (this phase) | a %-gate would be the wrong tool; deliberately rejected |

**Deprecated/outdated:** none introduced; Phase 21 removes no behavior.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `BaselineState{}` zero-value is safe to pass into `EvaluateEvent` in the fuzz target | Sentry Fuzz Target | LOW — if not, seed via `baseline_test.go` helper (read during planning); fuzz still works, just needs a non-zero seed |
| A2 | The `tui/model.go` "gap" is specifically `computeStatus`/`recentAuditRecords`/`runPaletteSelection` (not the whole file) | Tier-A Gap Contracts | LOW — `model_test.go` exists; planner should run `go test -cover ./internal/tui/` during execution to pin the exact uncovered funcs |
| A3 | The little-vm-helper VM-image approach satisfies CONTEXT's "ubuntu-20.04/k5.4" intent | CI Matrix | LOW — it is the existing working mechanism and covers k5.4 + k5.15 explicitly; if the maintainer specifically wants a 20.04 *runner*, that's a one-line `runs-on` change, but the VM image is more robust |
| A4 | No new external package is needed for any deliverable | Standard Stack / Legitimacy Audit | LOW — verified by reading both workflows + `.goreleaser.yaml`; all primitives are stdlib or already-pinned actions |

**Note:** This research is unusually low-assumption because the phase is an audit of existing code, not a greenfield build. Every structural claim (rosters, deny families, CI jobs, gap files, exit codes) was read directly from source at commit `04857cc`.

## Open Questions (RESOLVED)

1. **Should the 4 `sanity.go` delegators be tested (via `catalog.ResolveHealthy`) or allowlisted as `thin-delegator`?** (RESOLVED, MEDIUM) — Recommend testing `catalog.ResolveHealthy` once (covers all 4, plus it is the fail-open-on-read security path worth a real test) and NOT allowlisting them. Planner may allowlist if a per-package delegation test is judged pure ceremony.
2. **One coverage-gate test at repo root vs an `internal/coveragegate` package?** (RESOLVED, MEDIUM) — Recommend a small `internal/coveragegate` package with the walker + allowlist parser + `TestCoverageManifest` + `TestAllowlistFailsClosed`, so the gate logic is itself unit-tested and importable. Repo-root `_test.go` also works but can't be imported.
3. **Does `internal/sentry/windows` have a live-ETW CI gate analogous to `test-eslogger-fields`?** (RESOLVED, LOW — confirmed 2026-06-11) — **No.** `ci.yml` jobs are `test` (matrix incl. `windows-latest`), `fuzz`, `fuzz-ipc`, `fuzz-llamafirewall`, `test-sentry-kernel-5-4`, `test-sentry-kernel-5-15`, `test-eslogger-fields` (macOS only), and `release-gate` — there is no Windows/ETW live job (the lone eslogger reference is the macOS job). Resolution per the documented fallback: the `windows-latest` matrix leg's `go test ./...` IS the ETW coverage (build + unit), and `docs/validation-register.md` notes ETW as CI-build-validated. A live-ETW field-validation mirroring eslogger is feasible-in-principle but OUT of scope for Phase 21 (no audit finding mandates it; D-03 bounds fixes to audit-/test-traceable work) — captured as a future Tier-B enhancement, not a Phase-21 task.

## Environment Availability

| Dependency | Required By | Available (Windows dev box) | Version | Fallback |
|------------|------------|-----------------------------|---------|----------|
| Go toolchain | all Tier-A | ✓ | go.mod (1.25+) | — |
| CGO / C compiler (`-race`) | VAL-03 race | ✗ (CGO disabled locally) | — | CI-only (Tier B) — by design |
| clang/llvm/libbpf/linux-headers | eBPF generate+load | ✗ (Windows) | — | CI Linux job only |
| `cilium/little-vm-helper` | k5.4/k5.15 eBPF | ✗ (local) | v0.0.21 (CI) | CI only |
| eslogger | macOS Sentry | ✗ (Windows) | — | CI macos-latest only |
| Claude Code (real client) | VAL-05 live e2e | ✓ (this machine, Phase-10 proven) | — | — |
| 15 other real harness clients | VAL-06 register | ✗ (not installed) | — | manual register, irreducibly (Tier C) |
| HF account + Llama-Prompt-Guard-2 license | gated-model e2e | ✗ (human-only web action) | — | register entry stays PENDING (D-07) |

**Missing dependencies with no fallback:** the 15 non-CC harness clients and the HF license are irreducibly manual — that is precisely why VAL-06 exists (Tier C). Not a blocker; the *register* is the deliverable, not a green automated run.
**Missing dependencies with fallback:** CGO/eBPF/eslogger/ETW are all CI-validated (Tier B) — the expected, locked posture (D-05).

## Sources

### Primary (HIGH confidence — read directly at commit 04857cc)
- `internal/hooks/hooks.go` (`allTargets`, `gatewayTargets`, `backupSettings`) — the 17-harness roster + installer/backup contract
- `internal/check/deny_render.go` + `deny_render_test.go` — all 17 deny families + the existing `TestRenderDeny` table
- `internal/check/sanity.go`, `watch/sanity.go`, `scan/sanity.go`, `gateway/sanity.go`, `catalog/health.go` — the 4 delegators + the shared fail-open-on-read logic
- `internal/ipc/{server,client,peer_linux,peer_darwin}.go` — IPC + peer-cred gap files
- `internal/editorinit/lookup.go`, `internal/hooks/protected.go` — stub + metadata gaps
- `internal/sentry/{rules,types,imports_test}.go` — `EvaluateEvent` fuzz entry point + purity guard
- `internal/check/e2e_test.go`, `internal/llamafirewall/e2e_test.go` — existing e2e (exit-1 default; gated model)
- `.github/workflows/ci.yml`, `release.yml`, `web.yml`; `.goreleaser.yaml` — the CI surface + eBPF before-hook
- `docs/harness-support-matrix.md` (already 17), `README.md` (stale 15/14)
- `.understand-anything/knowledge-graph.json` (commit `04857cc`, 332 files) — per-file summaries confirming gap nodes + `tui/model.go` function line-ranges
- `.planning/{ROADMAP,REQUIREMENTS}.md`, `21-CONTEXT.md`

### Secondary / Tertiary
- None needed — no external-source claims; nothing tagged `[ASSUMED]` for package legitimacy (no packages).

## Metadata

**Confidence breakdown:**
- Standard stack (mechanisms): HIGH — all primitives verified present in-repo; no external decisions
- Architecture (gate/golden/fuzz/CI shapes): HIGH — modeled on existing, working in-repo patterns
- Pitfalls: HIGH — each derived from a concrete codebase discovery (exit-1 vs exit-2, 70/184 linkage, VM-image kernel split)
- 17-harness roster + deny contracts: HIGH — read verbatim from `hooks.go` + `deny_render.go`

**Research date:** 2026-06-11
**Valid until:** ~30 days (stable Go core; no fast-moving external deps). Re-verify only if `internal/hooks/allTargets` or `ci.yml` changes before planning.
