# Phase 21: Full-System Validation & CI Calibration - Context

**Gathered:** 2026-06-11
**Status:** Ready for planning
**Source:** Inline capture (discuss-phase skipped by maintainer choice — Phase 13–20 precedent)

<domain>
## Phase Boundary

Phase 21 is the **pre-ship release gate for the Go safety harness**. It is the last substantive phase of v1.3.0 and the first to deliberately relax the milestone's web-only fence (REQUIREMENTS "Out of Scope" exception, added 2026-06-10). Its job: prove that *every behavior of the Go core is validated at the correct tier, with zero silent gaps*, and remediate the gaps the `/understand` knowledge-graph audit surfaced.

**The validation tier model (the spine of the whole phase):**
- **Tier A — locally testable** on the Windows dev box → target **100%** coverage, **gate-enforced**. Every Go production file has a linked test OR a documented, reason-coded no-test allowlist entry.
- **Tier B — platform / kernel / build-tag bound** (eBPF, eslogger, ETW, `-race`/CGO, peer-cred, 3×GOOS) → covered by a **cross-platform CI matrix**, validated in CI, not locally.
- **Tier C — irreducible / manual / gated** (a true live block on each of the 16 non-Claude-Code harnesses; the gated-22M-model LlamaFirewall e2e) → captured in a **signed-off manual register**.

"Fully validated" = 100% of what can be tested locally + a CI matrix for everything platform-bound + a documented manual register for the rest.

**IN scope (ROADMAP SC-1..6 / VAL-01..08):**
- **VAL-01** — Coverage gate: every Go prod file has a linked test or a reason-coded allowlist entry; close the surfaced Tier-A gaps (`ipc` server/client + Windows-pipe peer-auth, `check`/`watch`/`scan`/`gateway` `sanity.go`, `editorinit/lookup.go`, `hooks/protected.go`, TUI model logic).
- **VAL-02** — Local 17-harness conformance suite: installer writes correct config (keys, idempotency, backup-on-overwrite) + golden-file the exact per-harness deny contract (exit code + JSON/stdout), incl. the Hermes fail-open seam and Kilo/Trae UNGUARDED honesty.
- **VAL-03** — Cross-platform CI matrix (ubuntu-20.04/k5.4 + ubuntu-22.04/k5.15 + macos-latest + windows-latest): build (native + 3 GOOS), vet, test, `-race` (CGO), eBPF generate+load (CI-only bytecode), eslogger (macOS), ETW (Windows), Unix peer-cred auth — all green.
- **VAL-04** — Fuzz suite as a blocking CI release gate (policy engine, IPC proto parser, catalog parser, MCP message parser, **Sentry rule evaluator**).
- **VAL-05** — Claude Code live e2e: canary `~/.ssh` + `~/.aws` read DENIED end-to-end (the documented true-block reference).
- **VAL-06** — `docs/validation-register.md`: all 16 non-Claude-Code harness live-block procedures + the gated-22M-model LlamaFirewall e2e, each with exact steps, expected result, sign-off.
- **VAL-07** — Honesty docs: README harness count corrected (15→17 / 14→16); validation posture (no-test allowlist + Tier A/B/C model) documented so the coverage claim is auditable.
- **VAL-08** — Self-defense: the coverage gate cannot be silently weakened (allowlist growth needs a reason code, fails closed otherwise); fuzz-gate failures block release.

**OUT of scope (explicit):**
- **Web phases (11–19)** — complete; not re-touched. No `web/` changes except (at most) a one-line README harness-count correction if the README is shared.
- **v1.1.0 Pollen** — PARKED; untouched throughout.
- **New product *features*** — Phase 21 adds tests, a conformance suite, CI matrix, fuzz gate, live e2e, and a register, and remediates audit-surfaced **defects/gaps**. It does NOT add net-new user-facing capabilities. (Hardening a surfaced gap is in scope per D-03; inventing a new subcommand is not.)
- **macOS DNS Sentry, memory-read detection, Windows missing-PPID, legit-endpoint exfil** — explicit Phase-20 residual/v2 items; not reopened here.

</domain>

<decisions>
## Implementation Decisions (locked)

### D-01 — The Tier A/B/C validation model is the organizing principle
Every deliverable is filed under exactly one tier (above). The phase succeeds only if all three are satisfied: Tier-A at 100% gate-enforced, Tier-B green in the CI matrix, Tier-C captured + signed off in the register. This model is also what VAL-07 documents publicly so the coverage claim is auditable. No behavior may sit in a fourth "untested and unregistered" bucket — that is the silent gap the phase exists to eliminate.

### D-02 — Coverage gate = file-linked-test + reason-coded allowlist, NOT a coverage-% threshold
VAL-01/VAL-08 wording is deliberate: the gate asserts **every Go production file has a linked test OR a documented, reason-coded no-test allowlist entry** — a *presence + accountability* gate, not a line-coverage percentage. Legitimate no-test entries: pure type/const/build-metadata files and platform stubs (the `//go:build`-guarded fail-closed loader stubs, `*_other.go`/`*_windows.go` resize/pid shims, generated `bpf_*_bpfel.go`). Each allowlist entry carries a reason code. The gate is a script (Go or pure-Python, matching the `web/tests/*_spec.py` house style) runnable locally on Windows and wired into CI. Baseline today: ~184 prod files / ~137 test files — the gap is the allowlist + the new tests for the 8 surfaced files. Exact mechanism is the researcher's + planner's call; the *contract* (presence-or-reasoned-allowlist) is locked.

### D-03 — Scope: APPLY ALL SURFACED FIXES (maintainer decision 2026-06-11)
Phase 21 is validation **and** remediation. The maintainer chose the broader disposition: actively refactor/harden every gap the audit surfaces — not merely register it. Concretely:
- The 8 enumerated Tier-A files get **real tests**, and any defect a new test reveals is **fixed** in the same phase.
- Broader audit-surfaced gaps (e.g. `ipc` server/client + Windows-pipe peer-auth edges, the four `sanity.go` fail-closed paths, `editorinit/lookup.go`, `hooks/protected.go`) are **hardened** even absent a failing test, where the audit flagged a real weakness.
- **Guardrail against scope creep:** "hardening a surfaced gap" ≠ "inventing a new feature." Fixes must trace to an audit finding or a test that fails on current behavior. Cosmetic refactors with no validation backing are out. This phase ships before milestone close — every fix must be defensible as closing a validation gap, and the planner should size accordingly (a wave/phase split is acceptable and expected if the breadth exceeds one context budget — see D-09).

### D-04 — 17-harness conformance = golden-file deny contracts + installer-config assertions
VAL-02 is a **local, deterministic** suite (no live harness needed). For each of the 17 targets it asserts two things: (1) the installer writes the correct config — exact keys, idempotent re-install, backup-on-overwrite — and (2) the deny renderer emits the **exact per-harness block contract** (exit code + JSON/stdout), asserted against a committed **golden file**. The honesty seams are first-class test cases, not footnotes: the **Hermes fail-open** seam (exit 0 + `action:"block"` JSON, never silently allowing) and the **Kilo/Trae UNGUARDED** cases (no enforceable hook → documented-unguarded, asserted honestly). The exact harness roster (the 17 / 16-non-Claude-Code list) is pinned by the researcher against `internal/hooks` + `docs/harness-support-matrix.md` (note the count grew 15→17 since Phase 10 — reconcile, don't assume).

### D-05 — CI matrix is CI-validated, not locally validated (Windows dev box)
VAL-03's matrix (ubuntu-20.04/kernel-5.4 + ubuntu-22.04/kernel-5.15 + macos-latest + windows-latest) runs build (native + 3 GOOS), vet, test, `-race` (CGO), eBPF generate+load, eslogger, ETW, peer-cred. The executor is on Windows, so these paths are **statically authored + CI-validated**, NOT run locally — matching every prior CI deliverable on this project (`web.yml`, the v1.2.0 `-race` pass, the Phase-20 eBPF/ETW DNS work, all CI-only). **eBPF bytecode is CI-generated at build time and NEVER committed** (CLAUDE.md constraint) — the matrix runs `go generate ./internal/sentry/linux/...` in the Linux job; the loader stubs fail closed when bytecode is absent. CI is build-verified as far as the unpushed repo allows: the matrix YAML is statically inspected; its live GitHub run is confirmed at first push (the repo has never been pushed — `gh repo create home-beekeeper/beekeeper` is part of the deferred v1.1.0 runbook).

### D-06 — Fuzz gate = existing 7 targets + a new Sentry-rule-evaluator target, blocking
Seven fuzz targets already exist (`catalog`, `gateway/parser`, `ipc/proto`, `llamafirewall/proto`, `nudge/scanners`, `pkgparse`, `policy`). VAL-04's required list is policy / IPC-proto / catalog / **MCP-message (= gateway/parser)** / **Sentry-rule-evaluator**. The Sentry rule evaluator has **no fuzz target today** — Phase 21 adds it. All targets run in CI as a **blocking release gate** (a fuzz failure blocks release — VAL-08), with a bounded `-fuzztime` per target. Local `-tags fuzz` smoke + CI as the authoritative gate (the v1.0.0/v1.2.0 fuzz precedent).

### D-07 — Live e2e is Claude-Code-only; the other 16 harnesses are a manual register
Only Claude Code is live-block-testable in an automated way (proven in Phase 10: canary `~/.ssh` + `~/.aws` DENIED end-to-end via the real hook). VAL-05 makes that the **documented true-block reference** (a repeatable scripted e2e or a recorded procedure). The other 16 harnesses are **irreducibly manual** (each needs its real client installed + driven) → VAL-06 captures them in `docs/validation-register.md` with exact steps, expected result, and a sign-off field. The gated-22M-model LlamaFirewall e2e (blocked on the human HF-license gate — see specifics) is the 17th register entry. The register is the honest artifact that says "here is exactly what we verified by hand, and how you reproduce it."

### D-08 — Honesty docs are part of the gate, not an afterthought (VAL-07)
README harness count corrected (15→17 total / 14→16 non-Claude-Code — reconcile the real number first). The validation posture — the no-test allowlist mechanism + the Tier A/B/C model — is documented (likely in `docs/validation-register.md` and/or a `docs/` posture section) so an external reader can audit the coverage claim rather than take it on faith. This mirrors the project's established honesty discipline (THREAT-MODEL §8, harness-support-matrix tiers, the home/docs known-gaps callouts).

### D-09 — Process: discuss skipped, research-first, plan + execute INLINE on main; split if needed
discuss-phase skipped (maintainer choice, Phase 13–20 precedent). **Research-first chosen** — the researcher pins: the exact coverage-gate mechanism + allowlist taxonomy, the precise surfaced Tier-A gaps and what each test must assert, golden-file conformance patterns, the CI-matrix job specifics (eBPF gen+load, eslogger, ETW, peer-cred, `-race`), and the Sentry-fuzz wiring. **Go subagents ARE fine here** (unlike the web phases — the executor needs no node/pnpm for Go work), so research/plan/plan-check/execute can all run as subagents; but the executor is on Windows so Tier-B paths are CI-validated. Given the breadth (8 VAL reqs spanning tests + conformance + CI + fuzz + e2e + register + remediation under the "apply all fixes" scope), the planner is **expected** to produce multiple waves and MAY recommend a phase split — both are acceptable; do not compress fidelity to force a single small plan set.

### D-10 — Self-defense is non-negotiable (VAL-08, CLAUDE.md "every phase includes self-defense")
The coverage gate defends itself: the no-test allowlist **fails closed** on unjustified growth — a new prod file with no test and no reason-coded allowlist entry breaks the gate; adding an allowlist entry requires a reason code (not a bare path). Fuzz-gate failures block release. This makes the validation infrastructure tamper-evident: an agent (or a careless commit) cannot silently lower the coverage bar.

### Claude's Discretion
- **Coverage-gate implementation language** — Go test (`TestCoverageAllowlist`-style, walks the package tree) vs a pure-Python file-walk like `web/tests/*_spec.py`. Recommend Go (keeps the gate in the same `go test ./...` run that CI already gates on, and lets it import build constraints), but the researcher/planner decides.
- **Allowlist file format + location** — e.g. `.planning/` vs an in-repo `coverage-allowlist.txt`/`.go` with `path # reason-code` lines. Must be reason-coded (D-02/D-10).
- **Golden-file format + harness for VAL-02** — table-driven Go test with `testdata/*.golden` per harness, `-update` flag to regenerate, vs another shape.
- **Whether VAL-02's 17-harness suite and VAL-01's coverage gate share a wave** or split.
- **Exact CI matrix job decomposition** — one matrix job with conditional steps vs separate per-OS jobs; how the 2-kernel Linux split is expressed (container vs runner image).
- **The depth of remediation per surfaced gap** under D-03 — bounded by "must trace to an audit finding or failing test."
- **Whether the live e2e (VAL-05) is a committed scripted test (`//go:build e2e` or a shell harness) or a recorded register procedure** — recommend scripted given Phase-10 already proved it live.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents (researcher, planner, executor) MUST read these before working.**

### Phase scope & requirements
- `.planning/ROADMAP.md` — Phase 21 section (goal, SC-1..6, Depends on the whole Go core + Phase 20) and the Tier A/B/C framing
- `.planning/REQUIREMENTS.md` — VAL-01 through VAL-08 (full text) + the "Out of Scope" Phase-21 exception
- `CLAUDE.md` — locked architecture + the **Testing Requirements** + **Self-Defense Non-Negotiables** sections (fuzz-failures-block-release, eBPF bytecode CI-only, OS build-tag test layout, eBPF CI matrix = explicit Ubuntu 20.04/22.04)

### The audit that birthed this phase
- `.understand-anything/knowledge-graph.json` — the `/understand` knowledge-graph audit (per-package coverage + stub/gated/fail-open signals) that surfaced the Tier-A gaps; the researcher should mine it for the precise gap list rather than trusting the ROADMAP summary alone
- `.understand-anything/meta.json` — audit metadata/scope

### Surfaced Tier-A gap files (VAL-01 — confirmed test-less 2026-06-11)
- `internal/ipc/server.go`, `internal/ipc/client.go` — IPC server/client (proto.go/pipe_windows.go HAVE tests; server/client do NOT)
- `internal/ipc/peer_linux.go`, `internal/ipc/peer_darwin.go`, `internal/ipc/pipe_windows.go` — peer-cred / named-pipe peer-auth (Tier-B platform-bound; pipe_windows_test.go exists)
- `internal/check/sanity.go`, `internal/watch/sanity.go`, `internal/scan/sanity.go`, `internal/gateway/sanity.go` — fail-closed sanity paths, no tests
- `internal/editorinit/lookup.go`, `internal/hooks/protected.go` — no tests
- `internal/tui/model.go` (+ panels) — `model_test.go` exists; the gap is specific uncovered model logic, not the whole file

### Conformance + deny-contract truth (VAL-02)
- `internal/hooks/**` — the installer + harness target definitions (the 17-harness roster lives here); deny renderer / `RenderDeny`
- `docs/harness-support-matrix.md` — the authoritative honesty doc (Tier-1 testable = Claude Code; Tier-2 = Hermes/Cline/OpenCode; Tier-3 = Kilo/Trae UNGUARDED) — reconcile the 15→17 count
- Phase-10 artifacts (`.planning/phases/10-*` SUMMARYs) — the deny-contract decisions (RenderDeny ExitCode semantics, Hermes exit-0+JSON-block, unknown-harness fail-closed exit 2)

### CI + fuzz surface (VAL-03/VAL-04)
- `.github/workflows/ci.yml` — the existing Go CI to extend into the full matrix
- `.github/workflows/release.yml` — release/SLSA/cosign + GoReleaser before-hook eBPF generate (do not regress)
- `.github/workflows/web.yml` — the path-filtered web job (the isolation precedent; Phase 21 must not entangle Go CI with web CI)
- existing fuzz targets: `internal/{catalog,gateway,ipc,llamafirewall,nudge,pkgparse,policy}/*fuzz_test.go` — the 7 present; the Sentry-rule-evaluator target is the one to add
- `internal/sentry/**` (+ `internal/sentry/linux/...` bpf2go) — the rule evaluator to fuzz + the eBPF generate path

### Live e2e + register truth (VAL-05/VAL-06)
- `.planning/phases/10-*` + `memory/dogfood-fixes-and-harness-status.md` — the proven Claude Code canary-block e2e (`~/.ssh` + `~/.aws` DENIED)
- `.planning/phases/20-runtime-hardening/20-02-*` — the gated-22M-model LlamaFirewall e2e + the pending HF-license human gate (the 17th register entry)
- `docs/THREAT-MODEL.md` §8 — the honest residual-gaps spine to keep the register consistent with

### Process / tracking
- `.planning/STATE.md` — hand-managed tracking. The init/roadmap resolvers DO resolve Phase 21 correctly (`expected_phase_dir = .planning/phases/21-full-system-validation-ci-calibration`, slug `full-system-validation-ci-calibration`), UNLIKE `16-3d-layer`; but still hand-verify STATE/ROADMAP/REQUIREMENTS after any SDK state verb (frontmatter-regression caveat)
- `.planning/phases/18-full-content-authoring/18-CONTEXT.md` — format precedent for this file

</canonical_refs>

<specifics>
## Specific Ideas

- **The 8 Tier-A gap files are confirmed test-less** (checked 2026-06-11): `ipc/server.go`, `ipc/client.go`, `check/sanity.go`, `watch/sanity.go`, `scan/sanity.go`, `gateway/sanity.go`, `editorinit/lookup.go`, `hooks/protected.go`. These are the concrete VAL-01 work items; the broader allowlist covers the rest of the ~184/137 prod/test gap.
- **The Sentry rule evaluator has no fuzz target** — the only missing VAL-04 target. "MCP message parser" maps to the existing `gateway/parser_fuzz_test.go`.
- **Harness count is 17 / 16-non-Claude-Code**, not 15 — the README + harness-support-matrix predate the growth (Kilo/Trae + others). Pin the exact roster before writing the conformance table or the README correction (VAL-02/VAL-07).
- **eBPF bytecode is never committed** (CLAUDE.md, locked) — the CI matrix generates it via `go generate ./internal/sentry/linux/...` in the Linux job; the loader stubs (`bpf_beekeeper_*_bpfel.go`) fail closed when absent. The matrix must explicitly cover **both** Ubuntu 20.04 (k5.4) and 22.04 (k5.15), not just `ubuntu-latest`.
- **`-race` requires CGO/a C compiler** → CI-only (CGO disabled on the local Windows box). This is a long-standing deferred item (Phase 1 v1.0.0) the matrix finally enforces.
- **The repo has never been pushed.** `gh repo create home-beekeeper/beekeeper` is part of the deferred v1.1.0 release runbook. So the CI matrix's live GitHub run is confirmed at first push — Phase 21 statically authors + locally build-verifies the YAML; it does not get a green GitHub badge until the push happens (note this honestly in the verification, like Phase 19's D-03).
- **The gated-22M-model LlamaFirewall e2e is blocked on a human HF-license gate** (accept Llama-Prompt-Guard-2 license + `huggingface-cli login` + run `beekeeper llamafirewall install`). Claude cannot perform the HF license (human-only web action). So VAL-06's register entry for it is authored, but its actual sign-off may stay pending until the maintainer completes the gate — the register's sign-off field captures that state honestly.
- **Self-defense framing** — the coverage allowlist is the new self-defense surface for this phase (CLAUDE.md: "Every phase includes self-defense work. Never defer these."). Make the allowlist tamper-evident (reason-coded, fails closed).

</specifics>

<deferred>
## Deferred Ideas

- **macOS DNS Sentry, memory-read detection, Windows missing-PPID, legit-endpoint exfil** — explicit Phase-20 residual/v2 items; not reopened.
- **The actual `git push` + `gh repo create` + live CI green badge** — part of the deferred v1.1.0 release runbook / milestone close, not Phase 21 itself (Phase 21 authors + build-verifies the matrix locally).
- **The maintainer's HF-license completion for the gated-model e2e** — a human gate; its register sign-off may remain pending past phase close.
- **SITE-03 live Vercel deploy** — separate deferred track, unrelated to Go validation.
- **v1.1.0 Pollen release** — PARKED; out of scope.

</deferred>

---

*Phase: 21-full-system-validation-ci-calibration*
*Context gathered: 2026-06-11 via inline capture (discuss-phase skipped; research-first chosen; scope = apply all surfaced fixes per maintainer)*
