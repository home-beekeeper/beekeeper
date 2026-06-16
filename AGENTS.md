# Beekeeper — Codex Instructions

## Project

**Beekeeper** is a real-time safety harness for autonomous coding agents. Single Go binary (`beekeeper`), multiple subcommands, multiple daemon modes. Intercepts agent tool calls before they execute and evaluates them against unified threat intelligence.

See `.planning/PROJECT.md` for full context, requirements, and decisions.
See `.planning/ROADMAP.md` for phase structure and success criteria.
See `.planning/STATE.md` for current phase and progress.

## GSD Workflow

This project uses the GSD (Get Shit Done) framework. Always follow these gates:

1. **Plan before executing** — Run `/gsd-plan-phase N` before any implementation work on Phase N
2. **Execute from the plan** — Run `/gsd-execute-phase N` to execute plans; do not implement ad hoc
3. **Verify after execution** — Run `/gsd-verify-work N` after each phase completes
4. **One phase at a time** — Complete and verify Phase N before starting Phase N+1

Current phase and status: see `.planning/STATE.md`

## Architecture Constraints

- **Go 1.25+, single static binary** — No CGO in core. `internal/` for all business logic. `cmd/beekeeper/main.go` is thin Cobra wiring only.
- **`internal/policy` must be a pure function library** — No I/O, no goroutines, no side effects. Called synchronously from hook handler, gateway middleware, and Sentry correlation. One implementation, three consumers.
- **Hook handler (`beekeeper check`) loads catalog via mmap** — Never cold-load JSON catalog per invocation. Pre-built binary index created by `beekeeper catalogs sync`.
- **Fail closed by default** — Any crash, timeout, or unavailability in `beekeeper check` or the gateway must result in block, not allow. `fail_open` is an explicit opt-in documented as reducing security.
- **MCP gateway: stateless per-request proxy** — MCP July 2026 spec has no session state. Correlate JSON-RPC responses by `id` field, never by position.
- **Bubble Tea: `charm.land/bubbletea/v2` import path** — NOT `github.com/charmbracelet`. Windows resize polling workaround required (TUI-10).
- **eBPF: pre-compiled bytecode, embedded at build time via `bpf2go`** — Never compile at runtime. Must be done from the first Sentry commit. Bytecode is CI-generated at build time (`go generate ./internal/sentry/linux/...` runs in the CI Linux job and the GoReleaser before-hook), NEVER committed to the repo, NEVER compiled at runtime. The loader stubs in `bpf_beekeeper_*_bpfel.go` fail closed (return a clear error) when bytecode is absent. To regenerate: run on Linux with `clang`, `llvm`, `libelf-dev`, `libbpf-dev`, and `linux-headers` installed. Windows/macOS builds are unaffected — the linux bpf files are guarded by `//go:build linux` tags.
- **ETW: `tekert/golang-etw` (no CGO)** — NOT `bi-zone/etw` which requires CGO.

## Key Technical Decisions (locked)

| Decision | Do |
|----------|----|
| Catalog matching | Corroboration-based: 1 source → warn, 2 → block, 3 → block + quarantine |
| Sentry elevation | Opt-in via `beekeeper protect install`. Unprivileged tier is full-featured without elevation |
| LlamaFirewall | Python sidecar, supervised; fail-closed on crash |
| macOS Sentry | `eslogger` subprocess (no entitlement). EndpointSecurity entitlement is v2 |
| Release signing | Sigstore/cosign v3 via GitHub Actions OIDC. No long-lived keys |
| SLSA | Level 3 via `slsa-github-generator@v2.1.0` (full semver — NOT `@v2`) |

## Build Constraints

- **Windows primary dev machine** — Cross-platform (Linux, macOS) validated in CI only
- **No WSL integration tests** — RAM/disk constraints; integration tests are CI-only
- **Reproducible builds required from v0.1.0** — `-trimpath -buildvcs=false -mod=readonly`; `make verify-release` must work

## File Structure (intended)

```
cmd/beekeeper/        # Cobra wiring, no business logic
internal/
  config/             # Layered config merge
  audit/              # NDJSON writer, sinks
  catalog/            # Loader, mmap index, corroboration
  policy/             # Pure policy engine (no I/O)
  check/              # Hook handler entry point
  gateway/            # MCP proxy daemon
  sentry/
    linux/            # fanotify + cilium/ebpf
    darwin/           # eslogger subprocess
    windows/          # tekert/golang-etw
  llamafirewall/      # Python sidecar supervisor + IPC
  tui/                # Bubble Tea v2 dashboard
  ipc/                # Unix socket / named pipe (CLI ↔ Sentry)
~/.beekeeper/
  config.json
  catalogs/           # Cached threat intel
  policies/           # Active policy JSON files
  audit/              # NDJSON audit log (rotated)
  baselines/          # Per-project behavioral counters
  quarantine/         # Quarantined packages/extensions
  llamafirewall/      # Sidecar models + cache
  state.json          # Runtime state for daemons
```

## Testing Requirements

- Go test suite for unit + integration; embedded fixtures
- Adversarial corpus for policy engine (real malicious tool call patterns from May 2026 incidents)
- Fuzz testing in CI: policy engine, IPC protocol parser, catalog parser, MCP message parser, Sentry rule evaluator — fuzz failures block release
- Sentry rule fixtures: synthetic process trees, file access sequences, network connection events
- OS-specific integration tests behind `//go:build linux`, `//go:build darwin`, `//go:build windows`
- Cross-platform CI: `ubuntu-latest`, `macos-latest` (Intel + Apple Silicon), `windows-latest`
- eBPF CI matrix (explicit Ubuntu 20.04/kernel 5.4 and Ubuntu 22.04/kernel 5.15 coverage, not just `ubuntu-latest`) is currently **decoupled to a manual workflow** (`.github/workflows/ebpf-kernel.yml`, `workflow_dispatch`) pending a runner-toolchain rebuild; the blocking `ci.yml` builds against committed loader stubs. TODO: re-add to PR CI + the branch ruleset.

## Self-Defense Non-Negotiables

Every phase includes self-defense work. Never defer these.

**v1.0.0 milestone (Phases 1–11 — all shipped 2026-06-01):**
- Phase 1: Reproducible builds + Sigstore + `SECURITY.md` + pinned deps
- Phase 2: Corroboration sanity bounds + catalog signature verification
- Phase 4: MCP gateway fuzz tests (release gate, not backlog)
- Phase 7: SLSA Level 3 + CycloneDX SBOM
- Phase 9: `beekeeper-self` catalog live

> ⚠ **Phase-number collision:** v1.1.0 ("Pollen") and v1.2.0 ("Runtime Hardening") reuse phase
> numbers 1–8. The list above is **v1.0.0-indexed and already shipped** — it does NOT impose
> SLSA/SBOM (the old "Phase 7" item) on v1.2.0 Phase 7 (Sensitive-Path Runtime Enforcement).
> v1.2.0 phases carry their own self-defense: Phase 6 = per-severity corroboration + anti-poisoning
> sanity gate (shipped); Phase 7 = the credential-block wiring + `RunCheck` integration tests is
> itself the self-defense (no extra deliverable); Phase 8 = behavioral test suite + live-binary E2E
> as the release gate. Always disambiguate "Phase N" by milestone (see `.planning/ROADMAP.md`).

## Research Flags (open items per phase)

- **Phase 2**: Socket PURL API rate limits empirically unknown; fsnotify Windows junction point behavior needs live testing
- **Phase 4**: MCP client differences (Codex vs Cursor) expose different edge cases; July 2026 spec SDK lag
- **Phase 5**: eBPF rule threshold calibration — 60s windows and 2-occurrence triggers derived from Nx Console timeline, need empirical validation
- **Phase 7**: eslogger field names partially undocumented — parse against real `macos-latest` output; ETW buffer sizing needs measurement under load
