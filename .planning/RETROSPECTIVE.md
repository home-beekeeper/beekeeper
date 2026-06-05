# Beekeeper — Living Retrospective

## Milestone: v1.0.0 — Comprehensive Standalone Release

**Shipped:** 2026-06-01
**Phases:** 10 | **Plans:** 50 | **Tasks:** 53 | **Commits:** 270 | **Span:** 7 days (2026-05-26 → 2026-06-01)

### What Was Built
A single static Go binary that mediates autonomous-agent tool calls against corroboration-based threat intel, fail-closed by default — hook handler + pure policy engine, multi-source catalogs (Bumblebee/OSV/Socket), editor-extension defense, hook/gateway/shim integration surfaces, cross-platform Sentry (eBPF/eslogger/ETW), supervised LlamaFirewall sidecar + full audit sinks, a Bubble Tea v2 TUI, declarative policy-as-code, and the `beekeeper-self` self-quarantine loop. Reproducible builds + Sigstore + SLSA L3 + SBOM + a public threat model from day one.

### What Worked
- **Pure `internal/policy` library** held across all 10 phases — the discipline of keeping the engine I/O-free let the loader, gateway, watch, scan, and check paths all reuse one evaluator without import cycles. Verified untouched at close.
- **Worktree-parallel Wave 1 execution** on Windows was reliable for independent packages (zero file overlap) and cut wall-clock; single-plan waves ran sequentially on the main tree to avoid merge overhead.
- **Fail-closed-by-default as a standing constraint** made the LLMF and self-quarantine wiring decisions easy and safe.
- **The milestone audit earned its keep** — it caught 4 cross-phase integration blockers (LLMF supervisor never started, gateway bumblebee-only, live corroboration_threshold divergence, dead diag latency) that every per-phase verification missed because each phase compiled in isolation.

### What Was Inefficient
- **Per-phase verification didn't catch cross-phase wiring.** Several exports (`NewSupervisor`, `ScanProxiedResponse`, `RunAuditRecordWithLLMF`) shipped with zero production callsites because the consuming wiring was deferred "to a future plan" that never came. Phase 10 existed solely to close this.
- **The Phase 9 policy-as-code wiring introduced a live/dry-run divergence** (`corroboration_threshold` worked in `policy test` but not live `check`) with a comment that falsely claimed otherwise — caught by code review + audit, fixed in Phase 10.
- **Tracking-artifact drift:** REQUIREMENTS.md traceability stayed "Pending" for ~62 reqs despite shipped work; 5 phases used UAT instead of VERIFICATION.md. Bookkeeping lagged execution.

### Patterns Established
- Author a focused gap-closure plan (09-06, 10-01) executed by a fresh-context executor on the main tree for localized fixes — cheaper than full discuss→plan→execute when the audit/review already pins the spec.
- "Deferred to a future plan" notes in a SUMMARY must become a tracked item, or they become dead code.

### Key Lessons
- Run `/gsd-audit-milestone` BEFORE tagging — cross-phase integration is invisible to per-phase gates.
- A deferral without a scheduled home is a silent gap; the last phase of a milestone is the last chance to catch it.
- "Wired in isolation + tests green" ≠ "wired into the runtime path." Grep for production callsites of every exported integration seam.

### Cost Observations
- Model mix: planning/orchestration on Opus; researcher/executor/checker/verifier/integration agents on Sonnet.
- Notable: the two retroactive gap-closure phases (09-06, 10-01) were small, high-leverage, and only necessary because integration wiring was deferred during the original phases.

---

## Milestone: v1.2.0 — Runtime Behavioral Hardening

**Shipped:** 2026-06-04
**Phases:** 4 (6–9) | **Plans:** 19 (+1 fix) | **Span:** ~2 days (2026-06-03 → 2026-06-04)

### What Was Built
Closed three live-validation gaps (F1 critical-malware warn-only, F2 credential reads ALLOW, F3 pnpm/bun bypass), each locked by a behavioral test. Per-severity corroboration escalation (sanity-gated); `EvaluatePath` wired live into `check` with canonicalization; a single pure `internal/pkgparse` + `internal/nudge` (soft-advise/hard-rewrite + CLI); a hermetic live-binary E2E + fuzz release gate. An audit-inserted Phase 9 then cleared the residual tech debt before close.

### What Worked
- **Dogfooding surfaced the requirements.** F1/F2/F3 came from running `beekeeper check` against the agent's own tool calls — the milestone's scope was discovered empirically, not guessed.
- **The milestone audit earned its keep again** — it found a release-gate that *passed only because of live OSV* (the CORR E2E wasn't hermetic) plus a `LoadLayered` root-cause masked by consumer workarounds. Both would have been invisible to per-phase verification.
- **Purity-as-standing-constraint held** across the two new packages (`pkgparse`, `nudge`) — import-purity tests kept all exec/net/FS I/O in adapters, so `Evaluate` stayed table-testable.
- **Fail-open vs fail-closed split made explicit per subsystem** (detection fail-open; catalog/path enforcement fail-closed; drift fail-open + never auto-bump floors) removed ambiguity from every wiring decision.

### What Was Inefficient
- **The v1.0.0/v1.2.0 phase-number reuse fought the tooling all milestone.** `gsd-sdk` `init.plan-phase`/`state.begin-phase`/`phase.complete`/`roadmap.update-plan-progress`/`audit-open` and commit-message stats all mis-resolved bare phase numbers (6–9) to the *archived* v1.0.0 phases. Every plan/execute/verify/close step had to pass explicit live paths and update tracking manually.
- **A latent build break rode along undetected.** `internal/policy/fuzz_test.go` had been broken since v1.0.0 Phase 4 (3-arg `Evaluate`) because no fuzz run ever included that package — only caught when Phase 9's SC9 ran the full `-tags fuzz` set.
- **Tracking churn from the inserted phase** — reopening a "ready to close" milestone to add Phase 9 meant re-editing ROADMAP/REQUIREMENTS/STATE twice (insert, then complete).

### Patterns Established
- **Audit → insert one closure phase → execute → re-verify → close** is a clean loop for `tech_debt`-status milestones (vs. accepting debt or hand-patching). Scope the phase from the audit findings directly; skip discuss when they're concrete.
- **Hermetic release gates beat realistic ones.** An E2E that depends on a live external service (OSV) is a flake waiting to red-line a release; seed a signed local fixture that reproduces the same decision path offline.
- **Fix root causes, not just consumers.** The `LoadLayered` Nudge-merge fix removed a class of silent-nil bugs that the consumer-layer workaround only masked per-callsite.

### Key Lessons
- Run the **full** `-tags fuzz`/`-tags e2e` set across **every** package at the milestone gate — partial gate coverage hid a pre-existing build break for two milestones.
- When milestones reuse phase numbers, **do not trust phase-number-keyed tooling** — drive with explicit paths and manual tracking from the first plan.
- A `tech_debt` audit verdict with no blockers is still worth closing inline when you're about to cut a release tag — the biggest find (non-hermetic release gate) directly threatened the tag.

### Cost Observations
- Model mix: orchestration + the two subtle/dependency-critical executors (09-01, 09-03) on Opus; researcher/planner-check/executor/verifier/integration on Sonnet.
- Notable: a single audit-inserted cleanup phase (5 plans, sequential on main, ~no worktree overhead) cleared every debt item plus a carried-over build break — high leverage for the close.

---

## Cross-Milestone Trends

### Recurring wins
- **Pure-policy discipline** has held across all milestones (v1.0.0 → v1.2.0): every new evaluator (`policy`, then `nudge`/`pkgparse`) stayed I/O-free and reused across consumers without import cycles.
- **The milestone audit catches cross-phase/integration defects that per-phase gates miss — every time.** v1.0.0: 4 integration blockers + 6 PRD-gap no-ops. v1.2.0: a non-hermetic release gate + a config-merge root cause.

### Recurring drags
- **Deferred wiring / latent breaks become silent gaps.** v1.0.0: exported seams with zero production callsites (needed Phase 10). v1.2.0: a fuzz file broken since v1.0.0 Phase 4, invisible until full-set coverage ran.
- **Tracking-artifact drift** (REQUIREMENTS traceability, VALIDATION frontmatter, STATE bold rows) lags execution and needs a manual reconciliation pass at close — worsened in v1.2.0 by phase-number-keyed SDK mis-resolution.

### Watch items for v1.3.0+
- The v1.0.0/v1.1.0/v1.2.0 phase-number overlap will keep biting `gsd-sdk` until milestones either renumber monotonically or the resolvers become milestone-aware. Until then: explicit paths + manual tracking is the standing workaround.
- Two parked/deferred release trains remain open: **v1.1.0 "Pollen"** (signed tags `pollen.2/.3/.4/.5`) and v1.0.0's external `beekeeper-self` hosting + VDP. Each needs a scheduled home, not a deferral note.

| Milestone | Phases | Plans | Span | Audit verdict |
|-----------|--------|-------|------|---------------|
| v1.0.0 | 10 (+1) | 50 | 7 days | PASSED (after Phase 10 closed 4 blockers) |
| v1.2.0 | 4 (6–9) | 19 (+1 fix) | ~2 days | tech_debt → cleared by inserted Phase 9 |
