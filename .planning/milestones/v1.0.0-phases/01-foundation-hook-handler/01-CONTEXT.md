# Phase 1: Foundation + Hook Handler - Context

**Gathered:** 2026-05-26
**Status:** Ready for planning
**Source:** PRD Express Path (beekeeper-prd.md)

<domain>
## Phase Boundary

Phase 1 delivers a working `beekeeper check` hook handler that protects the developer's own machine from day one. The developer can evaluate real tool calls against the Bumblebee `threat_intel/` catalog, receiving allow (exit 0) or block (exit non-zero) with a structured reason in under 100ms p95. Every release binary is reproducibly buildable and Sigstore-signed. This is the minimum viable harness (v0.1.0): binary skeleton + hook handler + Bumblebee catalog sync + self-defense foundations.

Phase 1 does NOT include: OSV/Socket catalog sources, policy engine beyond Bumblebee matching, MCP gateway, Sentry daemon, LlamaFirewall, TUI, editor extension defense, or shim layer. Those are Phase 2+.

</domain>

<decisions>
## Implementation Decisions

### Go Binary Structure
- Single static binary, `cmd/beekeeper/main.go` is thin Cobra wiring only — no business logic in cmd/
- All business logic lives in `internal/` packages
- No CGO in core binary
- Go 1.25+ minimum
- Build flags: `-trimpath -buildvcs=false -mod=readonly` for reproducible builds
- `make verify-release VERSION=X.Y.Z` target reproduces and compares hashes against published artifacts

### Hook Handler (`beekeeper check`) — HOOK-01, HOOK-02, HOOK-03, HOOK-04
- Reads tool call JSON from stdin — sub-100ms p95 target on a realistic Bumblebee catalog
- Exit 0 = allow; exit non-zero = block; structured reason always returned to stdout
- Loads catalog index via mmap (pre-built by `beekeeper catalogs sync`) — NEVER cold-loads JSON per invocation
- Fails closed by default: crash or timeout → block decision, never silent allow
- `fail_open` and `fail_warn` configurable as explicit opt-ins; `fail_open` must be documented as reducing security
- Hard resource limits enforced: stdin size max 1MB, execution time max 5s, memory cap
- Beyond any limit → fail closed

### Policy Engine Shape (Phase 1 scope)
- Phase 1 policy engine covers Bumblebee catalog matching only — corroboration semantics are Phase 2 (OSV/Socket not yet integrated)
- `internal/policy` must be a pure function library: no I/O, no goroutines, no side effects
- Called synchronously from hook handler; same library will be reused by MCP gateway (Phase 4) and Sentry correlation (Phase 5+) without modification
- Phase 1 matching: single source (Bumblebee), so single-source warn semantics only — no block enforcement yet from catalog (block comes from other policy checks)
- Actually the ROADMAP says CTLG-01 and CTLG-07 are in Phase 1. CTLG-07 says unsigned sources treated as warning-only. Phase 1 uses Bumblebee as the only source; the corroboration-based block/warn semantics (PLCY-01) are Phase 2.

### Catalog Sync and Index — CTLG-01, CTLG-05, CTLG-07
- `beekeeper catalogs sync` fetches and caches Bumblebee `threat_intel/` catalog (only catalog in Phase 1)
- Catalog records extended with `source_url`, `catalog_signature`, `catalog_source` fields; Beekeeper records set `scanner_name: "beekeeper"`
- After sync, builds a mmap-loadable binary index — `beekeeper check` uses this index, not raw JSON
- Catalog signature verification: unsigned sources treated as warning-only regardless of corroboration count (Phase 1: Bumblebee is the only source, may or may not be signed in v0.1.0 — design for signature verification even if not yet enforced)
- Cached catalogs stored in `~/.beekeeper/catalogs/`
- On Windows: `%APPDATA%\beekeeper\` instead of `~/.beekeeper/`

### Fail-Closed Architecture
- Any crash, timeout, or resource violation → block decision with structured reason
- No silent allows under any failure mode in the default configuration
- `fail_open` is an explicit opt-in documented in config schema as "reduces security"
- Audit log entry written for every decision including failure-mode decisions

### NDJSON Audit Log (Phase 1 minimum)
- Every policy decision emits one NDJSON record
- Fields: `record_type`, `record_id`, `scanner_name: "beekeeper"`, `agent_name`, `tool_name`, `decision`, `reason`, `rule_ids`, `catalog_matches` (with provenance)
- Local file sink: `~/.beekeeper/audit/beekeeper.ndjson`, `0600` permissions enforced from first write on Unix; equivalent ACLs on Windows
- Schema Bumblebee-compatible (AUDT-01 partially; full audit sinks are Phase 6)
- `beekeeper audit tail` — stream live audit log to terminal (Phase 1 minimum CLI)

### Self-Defense Foundations — SFDF-01, SFDF-02, SFDF-03, SFDF-04
- **Reproducible builds** (SFDF-01): deterministic Go build flags (`-trimpath -buildvcs=false -mod=readonly`), `make verify-release VERSION=X.Y.Z` target — MUST land in Phase 1
- **Sigstore signing** (SFDF-02): GitHub Actions OIDC, cosign v3 (`--bundle artifact.sigstore.json`), GoReleaser v2.13.0+; no long-lived signing keys — MUST land in Phase 1
- **Pinned dependencies** (SFDF-03): `go.mod` and `go.sum` with CI `go mod verify`; Renovate-bot (or equivalent) configured for human-review updates — MUST land in Phase 1
- **`SECURITY.md`** (SFDF-04): responsible disclosure process, 48h acknowledgment SLA, 90-day coordinated disclosure default — MUST land in Phase 1
- GoReleaser v2.13.0+ for multi-platform release pipeline

### Cross-Platform CI Matrix (Phase 1 foundation)
- GitHub Actions matrix: `ubuntu-latest`, `macos-latest` (Apple Silicon + Intel where possible), `windows-latest`
- Every PR runs full test suite on all three platforms from the first commit
- Windows is primary dev machine — CI-driven iteration for macOS/Linux
- OS-specific integration tests use `//go:build linux`, `//go:build darwin`, `//go:build windows` tags
- No WSL integration tests (RAM/disk constraints on dev machine)

### Project File Structure
```
cmd/beekeeper/          # Cobra wiring only, no business logic
internal/
  config/               # Layered config merge (Phase 1: user-level only)
  audit/                # NDJSON writer, local file sink
  catalog/              # Loader, mmap index, Bumblebee sync
  policy/               # Pure policy engine (no I/O, no goroutines)
  check/                # Hook handler entry point
~/.beekeeper/
  config.json
  catalogs/             # Cached threat intel
  audit/                # NDJSON audit log
  state.json            # Runtime state
```

### Key Library Choices (locked)
- Cobra for CLI subcommand routing
- `golang.org/x/sys` for mmap and platform-specific operations
- `github.com/google/go-github` or direct HTTP for Bumblebee GitHub catalog fetch
- Standard `encoding/json` for catalog parsing (custom if performance requires)
- GoReleaser for release pipeline
- cosign v3 for Sigstore signing

### Bumblebee Catalog Integration
- Primary catalog source: Bumblebee `threat_intel/` directory on GitHub
- Fetch via GitHub API or raw download, cache locally
- Catalog record schema (Bumblebee-extended):
  ```json
  {
    "id": "advisory-2026-XXXX",
    "name": "...",
    "ecosystem": "npm | pypi | go | rubygems | packagist | cargo | editor-extension",
    "package": "...",
    "versions": ["..."],
    "severity": "critical | high | medium | low",
    "source_url": "https://...",
    "catalog_signature": "...",
    "catalog_source": "bumblebee"
  }
  ```
- Binary mmap index format: flat sorted structure for O(log n) lookup per tool call invocation

### CLI Subcommands in Phase 1
- `beekeeper check` — one-shot policy eval, reads from stdin
- `beekeeper catalogs sync` — fetch and cache Bumblebee catalog, build mmap index
- `beekeeper audit tail` — stream live audit log
- `beekeeper version` — print version info
- `beekeeper selftest` — embedded fixture test (a handful of adversarial patterns from Phase 1 adversarial corpus)
- `beekeeper init` (stub only, no full onboarding logic) — creates `~/.beekeeper/` directory and default config

### Testing Requirements (Phase 1)
- Go unit tests for: policy engine (pure functions), catalog loader/mmap index, NDJSON writer, config parsing
- Integration tests: `beekeeper check` end-to-end with fixture tool call JSON (allow and block cases)
- Adversarial corpus: minimum set of real malicious tool call patterns from May 2026 incidents as regression fixtures
- `beekeeper selftest` command runs embedded fixture tests
- Fuzz targets for: policy engine, catalog parser (fuzz failures are tracked but not yet release-blocking in Phase 1 — Phase 4 is when fuzz blocking is enforced)
- Cross-platform build verification in CI on all three platforms

### Claude's Discretion
- Exact mmap index binary format (flat sorted slice vs. prefix tree — choose based on Go ecosystem and lookup performance profile)
- Config file schema details beyond what the PRD specifies
- Specific HTTP client configuration for Bumblebee catalog fetch (timeouts, retries, caching headers)
- Log rotation policy for Phase 1 (can be deferred to Phase 6 where full audit sinks are spec'd)
- Error message formatting and exit code conventions beyond "0=allow, non-zero=block"
- Internal package naming conventions
- Whether to use `make` or `task` as the build tool (prefer `make` for portability)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project Foundation
- `.planning/PROJECT.md` — Core value, constraints, key decisions, architecture overview
- `.planning/ROADMAP.md` — Phase 1 goal, success criteria, requirements list (HOOK-01 through SFDF-04)
- `.planning/REQUIREMENTS.md` — Full requirement definitions for HOOK-01, HOOK-02, HOOK-03, HOOK-04, CTLG-01, CTLG-05, CTLG-07, SFDF-01, SFDF-02, SFDF-03, SFDF-04
- `CLAUDE.md` — Architecture constraints, key technical decisions (locked), build constraints, file structure

### PRD Source
- `beekeeper-prd.md` — Full PRD, especially §4 Architecture, §5.1 Catalog matching, §8.1 Cadences, §12 Self-defense, §14 Stack, §15 Phasing (v0.1.0 deliverables)

### External References
- Bumblebee repository: primary threat intel source and catalog schema reference
- GoReleaser v2.13.0+ docs: release pipeline configuration
- cosign v3 docs: `--bundle artifact.sigstore.json` signing
- `slsa-github-generator` docs: SLSA provenance (Phase 7, but CI scaffold goes in now)
- `cilium/ebpf` docs: eBPF (Phase 5, but pre-compiled bytecode embedded at build time per architecture constraint)

</canonical_refs>

<specifics>
## Specific Ideas

### v0.1.0 Milestone Targets (from PRD §15)
The PRD explicitly lists v0.1.0 deliverables:
1. Go binary skeleton, project structure, CI matrix on all three OSes
2. `beekeeper check` hook handler
3. `beekeeper hooks install --target claude-code` (this is INTG-01, which is Phase 4 in ROADMAP — may be a stub only in Phase 1)
4. Bumblebee `threat_intel/` catalog sync and matching
5. Release-age policy for npm and PyPI (this is PLCY-02, Phase 2 in ROADMAP — stub or basic implementation)
6. Basic NDJSON audit log to local file
7. `beekeeper catalogs sync`, `beekeeper audit tail`

**Resolution:** Phase 1 ROADMAP requirements are HOOK-01/02/03/04, CTLG-01/05/07, SFDF-01/02/03/04. The PRD v0.1.0 section also mentions `beekeeper hooks install` and release-age policy, but these are NOT in the Phase 1 ROADMAP requirements. The ROADMAP takes precedence for what is IN Phase 1. Hook install and release-age policy are Phase 4 and Phase 2 respectively. Phase 1 stubs CLI structure but does not implement those features.

### Adversarial Corpus — May 2026 Incidents
Key patterns to include in Phase 1 regression fixtures (from PRD §1):
- Nx Console-class: editor extension install with known-bad publisher
- Package ecosystem patterns: npm typosquat, PyPI poisoning
- Bumblebee catalog match: package name + version in `threat_intel/`

### Beekeeper State Directory
- Unix: `~/.beekeeper/`
- Windows: `%APPDATA%\beekeeper\`
- Created by `beekeeper init` or on first use of any command that needs it
- `0600` permissions on audit log files from first write

### Hook Handler Latency Target
- Sub-100ms p95 is the target (from HOOK-01)
- Cold start cost of a fresh Go process: ~10-30ms per PRD §8.2
- mmap load is sub-millisecond once index is pre-built
- Policy evaluation: microseconds for the hot path
- Latency breakdown: cold start dominates; warm path via pre-loaded daemon is v2

### `beekeeper selftest` Design
- Embedded Go fixtures (not separate file at runtime)
- At minimum: 3-5 allow cases and 3-5 block cases covering Bumblebee matches and clean packages
- Must pass on all three platforms in CI
- Designed as the quick sanity-check that any user can run after install

</specifics>

<deferred>
## Deferred Ideas

- `beekeeper hooks install` (INTG-01) — Phase 4 per ROADMAP; Phase 1 may have CLI stub only
- Release-age policy (PLCY-02) — Phase 2 per ROADMAP; not in Phase 1 requirements
- OSV and Socket catalog sources (CTLG-02, CTLG-03) — Phase 2
- Corroboration-based block/warn multi-source semantics (PLCY-01) — Phase 2
- Full audit sinks: syslog, OTLP, HTTPS POST (AUDT-03/04) — Phase 6
- Log rotation — Phase 6
- `beekeeper catalogs watch` daemon (CTLG-06) — Phase 2
- SLSA Level 3 provenance (SFDF-05) — Phase 7 (CI scaffold may be started in Phase 1 pipeline)
- Sentry daemon — Phase 5+
- LlamaFirewall — Phase 6
- TUI dashboard — Phase 8
- Policy as code — Phase 9

</deferred>

---

*Phase: 01-foundation-hook-handler*
*Context gathered: 2026-05-26 via PRD Express Path*
