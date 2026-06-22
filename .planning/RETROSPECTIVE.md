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

## Milestone: v1.3.0 — Web Presence & Documentation

**Shipped:** 2026-06-11
**Phases:** 13 (10–21, incl. 18.1 inserted) | **Plans:** 43 (+1 quick task) | **Span:** ~7 days (2026-06-05 → 2026-06-11)

### What Was Built
Beekeeper's public-facing presence + pre-ship validation gate. A greenfield Next.js 16 static-export site under `web/` — marketing home (R3F hive hero, origin story, six shipped-capability cards, 15-harness matrix, honesty callout), a complete Fumadocs docs set authored accurate-to-binary, a versioned changelog, SEO/OG assets, and a path-filtered web CI (Vitest + Playwright). Plus Go-core hardening: Runtime Hardening II (catalog sync daemon, a now-functional LlamaFirewall opt-in, expanded Sentry coverage + honesty edits) and Full-System Validation (coverage gate + 17-harness conformance + cross-platform CI matrix + fuzz gate + live e2e + a signed-off validation register). The standalone Pollen fork was license-cleared and shipped publicly as signed v0.2.0 in-cycle.

### What Worked
- **Research-first docs paid for itself.** Authoring Phase 18 research-first caught real inaccuracies in the Phase-13 stubs before they shipped (`hooks install --target` not `--hook`; no `check --input`; `hooks status`→`diag`; `catalogs rebuild`→`catalogs sync`; single `beekeeper.ndjson` with no rotation; `allow_remote_gateway`/`release_age`/`lifecycle_script_allowlist` unenforced). An accuracy spec + a blocking human THREAT-MODEL.md review made "accurate to the shipped binary" an enforced gate, not an aspiration.
- **Code wins over transcription.** The nudge `mode:block` deviation — the plan, the research OQ, and `docs/nudge.md` all wrongly claimed `config set nudge.mode` rejects `block`; reading the real validator (`legalNudgeModes={soft,hard,block}`) corrected it. Verifying the enum beat trusting three layers of secondary docs.
- **Inline execution for the web phases was right.** The web toolchain (node/pnpm/Playwright) ran cleanly inline on main; treating each web phase as a sequential inline execution avoided worktree overhead and gave tight control over the iterative UAT loops (the hive took 3 rounds, the OG card 3 rounds).
- **Tier A/B/C validation model.** Splitting "what can be tested locally" (gate-enforced 100%) from "platform-bound" (CI matrix) from "irreducible" (signed manual register) made the release gate honest and complete — no silent gaps, and the coverage gate's fail-closed reason-coded allowlist is self-defending.
- **Dual-theme Playwright proofs caught real traps.** The Tailwind-v4 `@theme inline` lesson (shadcn slots aren't real CSS vars, so Fumadocs' `--color-fd-*` bridge read them empty) and a hardcoded-rgba header trap were both caught by both-theme Playwright runs that a body-bg-only check would have missed.

### What Was Inefficient
- **The phase-number-resolver regression kept biting.** `16-3d-layer` couldn't be resolved by the GSD phase-resolver (greedy `3d` token), and the v1.0.0/v1.1.0/v1.2.0/v1.3.0 phase-number overlap meant STATE/ROADMAP/REQUIREMENTS tracking was hand-managed all milestone — including this close, driven entirely by hand to protect the parked Pollen section.
- **Subagent capability note churned.** An early-milestone note said subagents lacked node/pnpm (driving inline execution); Phase 19's verifier re-ran `pnpm test` itself and disproved it. The inline choice was still fine, but the stale note cost some certainty.
- **Repeated UAT rounds on visual artifacts.** The hive hero (3 rounds) and the OG card (3 rounds) each re-gated finalization; good outcomes, but visual taste is hard to spec up front and ate review cycles.
- **A recurring plan-checker Dim-11 annotation gap** (`## Open Questions (RESOLVED)` on RESEARCH) reappeared across several phases and was fixed inline each time rather than once at the source.

### Patterns Established
- **Accuracy gate = a pure-Python/JS spec + a blocking human review against the source-of-truth doc** is a reusable shape for any content-authoring phase (AC-1 provenance frontmatter + path-exists / AC-2 unenforced-feature labels / AC-3 no phantom commands).
- **Coverage gate = package-level linkage + a fail-closed reason-coded allowlist**, not a %-threshold and not same-name-sibling linkage (sibling = 70/184 false positives). The allowlist is the phase's self-defense surface.
- **Reusable add-a-content-type pattern** (a second fumadocs-mdx collection for the changelog) and the dual-theme callout component pattern (raw `var(--*)` tokens, never dark-only `--color-bk-*`).
- **Close a parked milestone's blocker opportunistically.** The Pollen license question (the real park reason) got resolved mid-cycle, unblocking a full standalone v0.2.0 release — without un-parking the GSD milestone bookkeeping.

### Key Lessons
- Read the real validator/enum/code, not the doc or research transcription of it — three secondary sources agreed and all three were wrong.
- For cross-platform Go, `filepath.ToSlash` only rewrites the HOST-OS separator — backslash-path test assertions pass on Windows and fail on Linux/macOS CI. Never assert backslash paths through ToSlash-based code in cross-platform tests.
- The LlamaFirewall sidecar is CI/Linux-only by design (`codeshield`→`semgrep` has no native Windows build); the gated-model DOWNLOAD is the only Windows-verifiable slice of that gate.
- When milestones reuse phase numbers, do not trust phase-number-keyed tooling — drive with explicit paths and manual tracking, including the milestone close itself.

### Cost Observations
- Model mix: orchestration + planning on Opus; researcher/planner-check/executor/verifier/code-review on Sonnet; the web phases ran inline on main rather than fanning out subagents.
- Notable: two phases (20, 21) relaxed the web-only fence to harden and validate the Go core as the pre-ship gate — high-leverage, and they added only ~15 prod LOC of test/doc with zero product behavior change.

---

## Milestone: v1.4.0 — Adjudicated Corpus (Local Loop)

**Shipped:** 2026-06-15
**Phases:** 4 (22–25) | **Plans:** 12 (+1 quick task) | **Span:** ~2 days (2026-06-13 → 2026-06-15)

### What Was Built
The moat: a frozen four-layer event schema (behavior/decision/outcome/context) + push-envelope wire format with `auto_purge` made compile-time-unrepresentable (Phase 22); an append-only owner-only corpus store as an `audit.Sink` + an off-hot-path adjudication engine with corroboration-gated confidence and HMAC fingerprints (Phase 23); First Responder corpus binding — confirmed-malicious arms the TUI quarantine card + a detection-only Sentry watch + a local catalog overlay, never auto-purging (Phase 24); and a launch-readiness gate proving the end-to-end moat loop + offline-protective + sub-100ms hot path + an honest threat model (Phase 25). Zero new dependencies; **no network transport stood up** (wire format emitted-in-shape so push is later wiring).

### What Worked
- **Schema-freeze-first captured the non-retrofittable asset.** Locking the outcome layer (confirmed `true_label` + how established) as Go types with a maintainer sign-off *before* the first corpus write meant the moat is present from record #1 — exactly the thing a competitor can't backfill.
- **Off-hot-path discipline held.** The adjudicator lives in an impure package consuming the pure `internal/policy`; `beekeeper check` never imports it (grep-verified), so the corpus loop never touched the fail-closed latency budget (`BenchmarkRunCheck` ~25ms).
- **The synthetic E2E gate earned its keep mid-milestone** — it caught a prod-blocking config-merge bug (`cfg.Corpus.Enabled` always false because `layered.go merge()` didn't merge the new sub-structs) that every per-requirement unit test missed.
- **The milestone audit earned its keep again (the headline catch).** It found that the local overlay — the mechanism that lets a machine enforce ahead of the upstream feed — was written but never read by any production enforcement path. Four green phase verifications, a passing secure-phase, and a green 27-package suite all missed it.

### What Was Inefficient
- **A "written but never read" seam shipped through every per-phase gate.** FRB-05's overlay was created, the query machinery (`NewMultiIndexWithOverlay`) was implemented, and the daemon wrote entries — but no enforcement path constructed the MultiIndex *with* the overlay. The tests asserted the component (`NewMultiIndexWithOverlay` directly) instead of the path (a live `RunCheck`), so the gap was invisible until the cross-phase integration check.
- **False-confidence gates, three times in one milestone.** Phase 25 code review caught two launch gates that would stay green even if LAUNCH-01/03 regressed; the audit caught the FRB-05 enforcement gap. All three were tests that proved a component in isolation rather than the runtime path.
- **The malformed-ROADMAP resolver forced full hand-management.** The summary-list ROADMAP format returns `malformed_roadmap` for phases 22–25, so every plan/execute/secure/close step ran with explicit paths and no SDK state verbs (which would corrupt the frontmatter) — and this close was hand-driven like v1.3.0.
- **SUMMARY `requirements-completed` frontmatter drifted** (only populated for 22-03 + the 25-* plans); the VERIFICATION.md tables were the load-bearing coverage record.

### Patterns Established
- **On a "prove-the-property" phase (validation / launch-readiness / enforcement-wiring), code-review the assertions, not just pass/fail.** A green verifier + a passing secure-phase do not prove a test asserts the right thing.
- **Test the runtime path, not the component.** For any "written → read → enforced" seam, the regression test must drive the real entry point (`RunCheck`), or the wiring gap hides behind a direct-constructor test.
- **Audit → small quick-task fix → merge → close** is a clean loop when the audit pins a single concrete blocker (vs. inserting a full closure phase).
- **Measure milestone coverage per-FUNCTION, not per-package** — mature-repo package %s are diluted by out-of-scope code.

### Key Lessons
- The milestone audit's cross-phase integration check is the only gate that catches "compiles + per-phase-verified + tests green, but the seam isn't wired into the runtime path." Run it before tagging, every time.
- An unsigned local signal is warn-only by design (CTLG-07 anti-poisoning) — wiring it in delivers allow→warn, not block; name that outcome explicitly rather than letting "confirmed-malicious" imply a hard block.

### Cost Observations
- Model mix: planning/orchestration on Opus; researcher/executor/checker/verifier/integration agents on Sonnet.
- Notable: the highest-leverage spend was the integration-checker pass at audit time — one Sonnet agent surfaced the headline FRB-05 gap that four phase verifications had missed; the fix was a ~20-line handler change + one RunCheck-level test.

---

## Milestone: v1.5.0 — Install Posture (released as v1.1.0)

**Shipped:** 2026-06-22
**Phases:** 6 (26–31) | **Plans:** 11 | **Span:** 2 days (2026-06-21 → 2026-06-22)

### What Was Built
Retired the package-manager nudge and shipped tool-agnostic install posture: three rules (release-age / lifecycle / git-remote) enforced at the pre-exec hook at warn + fail-soft; a read-only machine-wide `beekeeper posture` view (CLI + TUI); scoped audited overrides (`posture allow`/`enforce`) with per-rule warn→block opt-up; SENTRY-009 detection-only install observation; and a canonical enforcement-boundary statement propagated to code, help, docs, and the web site. Shipped across two repos (Go core `home-beekeeper/beekeeper` + docs `beekeeper-web`).

### What Worked
- **The two-gate structure paid off.** Gate 1 (enforcement-boundary review) is where the maintainer pulled IPOVR-03 (per-rule opt-up) into v1.0 and ratified warn/fail-soft — a scope decision made at exactly the right moment, before propagation. Gate 2 (release signing) kept the irreversible signature with the human by design.
- **Release-gate reviews beat a formal milestone audit at finding real issues.** The security review (all 8 invariants HELD) + code review caught the H-01 ship-gate (`posture allow --once --rule` recorded a scope it didn't enforce) *before* merge; the milestone audit then confirmed zero blockers. Running the reviews on the full merged diff, adversarially, was higher-yield than the checkbox audit.
- **The posture layer's structural isolation was verifiable.** Designing posture as warn-only + most-restrictive-merged + a separate allowlist (never `package_allowlist`) meant the T-09-31 "can't bypass a malware block" invariant was provable by both a live-RunCheck test and the security review, not just asserted.
- **Independent re-verification per phase** (orchestrator re-ran build/vet/test/e2e after each delegated phase) caught the load-flaky p99 latency gate as environmental, not a regression — no false alarm at the gate.

### What Was Inefficient
- **A docs agent that ran build + accuracy but not lint** shipped a biome-format miss that only surfaced in web CI (red on the first PR run). Lesson: reproduce the *full* CI gate set locally before pushing, not a subset.
- **Cross-repo `source_doc` drift was latent until CI.** Deleting `cmd/beekeeper/nudge.go` silently red-ed the web accuracy gate (dead `source_doc` paths) — caught only because the gate resolves against the sibling checkout. A pre-delete grep for inbound `source_doc` references would have surfaced it at edit time.
- **The hand-managed milestone (no per-phase VERIFICATION.md/SUMMARY.md) doesn't fit the stock audit-milestone workflow**, which mechanically flags every phase "unverified." The audit had to be run evidence-based against merged `main` — worth formalizing as the convention for this project.

### Patterns Established
- **Decoupled internal-vs-public version numbers.** GSD milestone `v1.5.0` ships as release `v1.1.0` to avoid colliding with the parked Pollen `v1.1.0` GSD milestone — archives use the internal number, the git tag uses the public one.
- **Two-repo release via paired PRs** (Go core + beekeeper-web), each CI-gated, merged independently, with the signed tag on the core repo only.
- **Audit-as-evidence for hand-managed milestones:** verify each REQ-ID against merged code + release-gate reviews + integration checker + green CI, not against per-phase planning files.

### Key Lessons
- Put the irreversible, outward-facing step (signing, publishing) behind an explicit human gate and prepare everything else fully — the agent took the release to a one-command signature.
- Adversarial security + code review on the merged diff is the highest-leverage pre-ship gate; the formal milestone audit is a confirmation, not the primary net.
- When a feature is a *safety* layer, design it so its safety invariant is *testable* (structural isolation + a live-path test), not just documented.

### Cost Observations
- Model mix: orchestration on Opus; the web reframe, Go test executor, security/code reviewers, and integration checker on delegated agents.
- Notable: the security review (8 invariants HELD) + code review (1 HIGH fixed) on the merged diff was the highest-yield spend — it caught the only ship-gate bug before merge, which the subsequent milestone audit then confirmed clear.

---

## Cross-Milestone Trends

### Recurring wins
- **Pure-policy discipline** has held across all milestones (v1.0.0 → v1.2.0): every new evaluator (`policy`, then `nudge`/`pkgparse`) stayed I/O-free and reused across consumers without import cycles.
- **The milestone audit catches cross-phase/integration defects that per-phase gates miss — every time.** v1.0.0: 4 integration blockers + 6 PRD-gap no-ops. v1.2.0: a non-hermetic release gate + a config-merge root cause.

### Recurring drags
- **Deferred wiring / latent breaks become silent gaps.** v1.0.0: exported seams with zero production callsites (needed Phase 10). v1.2.0: a fuzz file broken since v1.0.0 Phase 4, invisible until full-set coverage ran.
- **Tracking-artifact drift** (REQUIREMENTS traceability, VALIDATION frontmatter, STATE bold rows) lags execution and needs a manual reconciliation pass at close — worsened in v1.2.0 by phase-number-keyed SDK mis-resolution.

### Watch items for the next milestone
- The v1.0.0/v1.1.0/v1.2.0/v1.3.0 phase-number overlap keeps biting `gsd-sdk` (and bit the v1.3.0 close itself — driven by hand to protect the parked Pollen section). Until milestones renumber monotonically or the resolvers become milestone-aware, explicit paths + manual tracking is the standing workaround.
- **v1.1.0 "Pollen" GSD milestone is still PARKED** (not closed) even though the standalone tool shipped publicly as signed **v0.2.0** in the v1.3.0 cycle (the `-pollen.N` per-phase-tag scheme was scrapped for clean semver). Closing the GSD milestone is a separate bookkeeping step via `docs/release-runbook.md` + the 05-05 checkpoint.
- Open deferrals needing a scheduled home, not a note: **SITE-03** (live Vercel deploy of the v1.3.0 site), v1.0.0's external `beekeeper-self` hosting + refuse-to-run E2E + VDP publication, and the nudge/corroboration follow-ups (NUDGE-F1/F2/F3, CORR-F1).

| Milestone | Phases | Plans | Span | Audit verdict |
|-----------|--------|-------|------|---------------|
| v1.0.0 | 10 (+1) | 50 | 7 days | PASSED (after Phase 10 closed 4 blockers) |
| v1.2.0 | 4 (6–9) | 19 (+1 fix) | ~2 days | tech_debt → cleared by inserted Phase 9 |
| v1.3.0 | 13 (10–21, incl. 18.1) | 43 (+1 quick task) | ~7 days | none — every phase individually verified; 4 audit-open items confirmed stale at close |
