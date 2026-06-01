# Roadmap: Beekeeper

## Milestones

- ✅ **v1.0.0 — Comprehensive Standalone Release** — Phases 1–10 (shipped 2026-06-01)
  Full per-phase detail archived in [`milestones/v1.0.0-ROADMAP.md`](milestones/v1.0.0-ROADMAP.md).
  Audit: PASSED — [`milestones/v1.0.0-MILESTONE-AUDIT.md`](milestones/v1.0.0-MILESTONE-AUDIT.md).
  Summary: [`MILESTONES.md`](MILESTONES.md).

## Phases

<details>
<summary>✅ v1.0.0 Comprehensive Standalone Release (Phases 1–10) — SHIPPED 2026-06-01</summary>

- [x] Phase 1: Foundation + Hook Handler (6/6 plans) — fail-closed `beekeeper check`, Bumblebee mmap catalog, reproducible Sigstore builds
- [x] Phase 2: Policy Engine + Multi-Source Catalogs (9/9) — corroboration semantics, OSV + Socket adapters, lifecycle/path/egress/baseline policies, catalog watch daemon
- [x] Phase 3: Editor Extension Defense (5/5) — agent CLI intercept, fsnotify watcher, quarantine workflow, `beekeeper scan`
- [x] Phase 4: Integration Surfaces (5/5) — Claude Code/Cursor/Codex hook installers, MCP gateway, shim layer, multi-agent observability
- [x] Phase 5: Linux Sentry (5/5) — privileged systemd daemon, fanotify + cilium/ebpf ingestion, 5-rule correlation, 7-day baseline
- [x] Phase 6: LlamaFirewall + Audit Sinks (5/5) — supervised Python sidecar, syslog/OTLP/HTTPS sinks, audit query/export
- [x] Phase 7: Cross-Platform Sentry (5/5) — macOS eslogger, Windows ETW, SLSA Level 3 + CycloneDX SBOM
- [x] Phase 8: TUI Dashboard (9/9) — Bubble Tea v2, all panels, admin mode, Windows resize workaround
- [x] Phase 9: Policy as Code + Self-Defense Capstone (5/5) — declarative JSON policies, layered config, `beekeeper-self` catalog, `beekeeper diag`
- [x] Phase 10: Cross-Phase Integration Closure (1/1) — live corroboration_threshold, gateway corroboration, LlamaFirewall supervisor + scan wiring, diag sidecar latency, overlay coverage
- [ ] Phase 11: v1.0.0 PRD-Gap Closure (pre-push) — gateway PromptGuard real tool name, layered config in enforcement, eBPF build-pipeline generation, delta-triggered scan, `catalogs diff`, real Ed25519 catalog signatures

</details>

## Progress

| Phase | Milestone | Plans | Status | Completed |
|-------|-----------|-------|--------|-----------|
| 1. Foundation + Hook Handler | v1.0.0 | 6/6 | Complete | 2026-05-26 |
| 2. Policy Engine + Multi-Source Catalogs | v1.0.0 | 9/9 | Complete | 2026-05-26 |
| 3. Editor Extension Defense | v1.0.0 | 5/5 | Complete | 2026-05-26 |
| 4. Integration Surfaces | v1.0.0 | 5/5 | Complete | 2026-05-27 |
| 5. Linux Sentry | v1.0.0 | 5/5 | Complete | 2026-05-28 |
| 6. LlamaFirewall + Audit Sinks | v1.0.0 | 5/5 | Complete | 2026-05-28 |
| 7. Cross-Platform Sentry | v1.0.0 | 5/5 | Complete | 2026-05-28 |
| 8. TUI Dashboard | v1.0.0 | 9/9 | Complete | 2026-05-29 |
| 9. Policy as Code + Self-Defense Capstone | v1.0.0 | 5/5 | Complete | 2026-05-29 |
| 10. Cross-Phase Integration Closure | v1.0.0 | 1/1 | Complete | 2026-06-01 |
| 11. v1.0.0 PRD-Gap Closure (pre-push) | v1.0.0 | 0/1 | In progress | - |

*Next milestone: run `/gsd-new-milestone` to start v1.x (questioning → research → requirements → roadmap).*
