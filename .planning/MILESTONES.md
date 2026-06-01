# Milestones

## v1.0.0 — Comprehensive Standalone Release (Shipped: 2026-06-01)

**Phases:** 11 (Phases 1–9 planned + Phase 10 integration closure + Phase 11 PRD-gap closure) · **Plans:** 51 · **Tasks:** 59
**Timeline:** 2026-05-26 → 2026-06-01 (7 days)
**Audit:** PASSED (re-verified after Phase 10 closed 4 cross-phase integration blockers)
**Pre-push PRD audit:** a direct `beekeeper-prd.md`-vs-code audit then found 6 more gaps the milestone audit missed (gateway PromptGuard scanned with an empty tool name → no-op; layered config not used by enforcement commands; Linux eBPF bytecode uncommitted/ungenerated; catalog-delta scan not triggered; `catalogs diff` missing; presence-only catalog signatures). **All 6 closed by Phase 11** (commits 3b79c90, 1f3682b, 86686d5, c42c681, 0b7f64f, deb8783) — re-verified green before the tag was moved to the fixed commit. Lesson: prior verification confirmed wiring *existed*, not that it was *effective* end-to-end (empty-arg no-op; helper present but never called by enforcement commands).

**Delivered:** A single static Go binary (`beekeeper`) that intercepts autonomous-agent tool calls before they execute and evaluates them against unified, corroboration-based threat intelligence — fail-closed by default, with a published self-defense threat model and a recursive self-quarantine feed.

**Key accomplishments:**

1. **Fail-closed hook handler + corroboration policy engine** — `beekeeper check` evaluates tool calls against an mmap catalog index under hard caps (1MB stdin / 5s / 256MB), with a pure `internal/policy` corroboration engine (1 source → warn, 2 → block, 3 → quarantine) across Bumblebee + OSV + Socket.
2. **Editor-extension defense** — agent `--install-extension` intercept, fsnotify watcher, and the watch → scan → quarantine workflow closing the Nx Console-class attack surface.
3. **Integration surfaces** — Claude Code / Cursor / Codex hook installers, a stateless fail-closed MCP gateway with per-session token auth and a fuzz-gated parser, and the PATH shim layer.
4. **Cross-platform Sentry** — Linux (eBPF + fanotify), macOS (eslogger), and Windows (ETW, no CGO) privileged daemons with a shared 5-rule correlation engine, talking to the unprivileged CLI over authenticated IPC.
5. **LlamaFirewall sidecar + full audit** — supervised Python sidecar (PromptGuard 2 / CodeShield), NDJSON audit log with syslog/OTLP/HTTPS sinks, and `audit query/tail/export`.
6. **Bubble Tea v2 TUI dashboard** — live activity, alerts, catalog, scan, policy, quarantine, and health panels, with admin mode and the Windows resize workaround.
7. **Policy as code + self-defense capstone** — declarative JSON policies (`policy validate/test/list`) enforced live across check/gateway/watch/scan, five-layer config merge, `beekeeper diag`, and the separately-signed `beekeeper-self` self-quarantine catalog.
8. **Self-defense from day one** — reproducible builds, Sigstore signing, SLSA Level 3 provenance + CycloneDX SBOM, and a public `docs/THREAT-MODEL.md` documenting the corroboration-poisoning surface and the fanotify mmap gap.

**Known deferred at close (carried to v1.x):**
- Live external `beekeeper-self` hosting (separate host + signing key) + end-to-end refuse-to-run validation — client side shipped; external ops gate.
- Independent external security review + VDP scope publication (PRD §15.5).
- Phases 02 and 05 verified via UAT (status approved/passed, 0 pending scenarios) rather than VERIFICATION.md — benign artifact-trail inconsistency.
- Distributed mode / team-shared catalogs; weighted corroboration (explicitly deferred per PRD §17).

---
