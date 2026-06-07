---
phase: 11
slug: scaffold-toolchain-isolation
status: approved
nyquist_compliant: true
wave_0_complete: true
created: 2026-06-07
---

# Phase 11 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Phase 11 is infra scaffolding — there is no business logic to unit-test yet (Vitest/Playwright arrive in Phase 19). Validation here = a **build smoke check** + a **toolchain-isolation assertion**. Proportionate to scaffold work.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | none yet — build smoke + isolation assertion (Vitest/Playwright deferred to Phase 19) |
| **Config file** | `web/next.config.ts` (static export config) |
| **Quick run command** | `cd web && pnpm build` |
| **Full suite command** | `cd web && pnpm build && test -s out/index.html && git status --porcelain go.mod go.sum` |
| **Estimated runtime** | ~30–60 seconds (cold Next build) |

---

## Sampling Rate

- **After every task commit:** Run `cd web && pnpm build` (or `pnpm dev` start check for the dev-server task)
- **After every plan wave:** Run the full suite command (build → out/index.html exists → Go files unchanged)
- **Before `/gsd-verify-work`:** Full suite green; `git status` shows no untracked build artifacts
- **Max feedback latency:** ~60 seconds

---

## Per-Task Verification Map

> Tasks live in `11-01-PLAN.md` (single-plan phase). The phase's 4 success criteria map to their validation evidence and the owning task below.

| Success Criterion | Requirement | Owning Task | Validation | Test Type | Automated Command |
|-------------------|-------------|-------------|------------|-----------|-------------------|
| SC1: `pnpm dev` serves localhost without errors | SITE-01 | 11-01 Task 4 (human-verify) | dev server starts, responds 200 on `/` | smoke (manual) | `cd web && pnpm dev` (probe `http://localhost:3000`, then stop) |
| SC2: `pnpm build` emits non-empty `out/index.html` | SITE-01 | 11-01 Task 2 | static export succeeds | build smoke | `cd web && pnpm build && test -s out/index.html` |
| SC3: `pnpm install` never modifies Go files | SITE-02 | 11-01 Task 3 | go.mod/go.sum unchanged across install | isolation | `git status --porcelain go.mod go.sum` returns empty after `pnpm install` |
| SC4: build artifacts gitignored | SITE-02 | 11-01 Task 3 | no artifacts in `git status` | isolation | `git check-ignore node_modules web/.next web/out web/.source web/node_modules`; `git status --porcelain` shows no artifacts |

---

## Wave 0 Requirements

- [x] No test framework installed in Phase 11 — Vitest + Playwright are introduced in Phase 19 (Test Suite & CI). Phase 11 validation relies on the build + git assertions above. No Wave 0 file gaps.

*Existing infrastructure: none (greenfield web/). Build-smoke + isolation assertions cover all Phase 11 success criteria.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `pnpm dev` renders a page in a browser | SITE-01 | dev-server liveness is awkward to assert headlessly pre-Playwright | `cd web && pnpm dev`, open `http://localhost:3000`, confirm the starter page renders, Ctrl-C (11-01 Task 4 checkpoint) |

*All other Phase 11 behaviors have automated (build/git) verification.*

---

## Validation Sign-Off

- [x] All success criteria have an automated build/git assertion (or the single documented manual dev-server check)
- [x] Sampling continuity: build smoke after every task; full suite after the wave
- [x] Wave 0 N/A (no test framework this phase)
- [x] No watch-mode flags (use `pnpm build`, not `pnpm dev --watch`, in CI/checks)
- [x] Feedback latency < 60s
- [x] `nyquist_compliant: true` set in frontmatter — tasks mapped in 11-01-PLAN.md

**Approval:** approved (tasks mapped 2026-06-07)
