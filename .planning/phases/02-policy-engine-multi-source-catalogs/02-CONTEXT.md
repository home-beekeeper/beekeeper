# Phase 2: Policy Engine + Multi-Source Catalogs - Context

**Gathered:** 2026-05-26
**Status:** Ready for planning
**Source:** PRD Express Path (beekeeper-prd.md)

<domain>
## Phase Boundary

Phase 2 completes the corroboration-based policy engine — the full multi-source threat intelligence architecture that Phase 1 deferred. The deliverables are: OSV offline database integration, Socket PURL API catalog source, corroboration semantics (1-source → warn, 2-source → block, 3-source → block + quarantine), release-age policy for all covered ecosystems, lifecycle script policy (allowlist-only deny), sensitive path policy, network egress policy with multi-turn exfiltration detection, behavioral baseline engine, output credential filtering, `beekeeper catalogs watch` daemon with catalog-delta-triggered scans, catalog sanity bounds and degraded mode, and full catalog provenance in every NDJSON audit record.

Phase 2 does NOT include: editor extension defense (Phase 3), MCP gateway (Phase 4), hook installation (Phase 4), Sentry daemon (Phase 5+), LlamaFirewall (Phase 6), TUI dashboard (Phase 8), policy as code (Phase 9), `beekeeper-self` catalog (Phase 9).

Phase 2 builds directly on Phase 1's `internal/policy`, `internal/catalog`, `internal/audit`, and `internal/config` packages. The `CatalogLookup` interface introduced in Phase 1 must be extended (not replaced) to support multi-source corroboration. The pure-function constraint on `internal/policy` is absolute — no I/O introduced in Phase 2.

</domain>

<decisions>
## Implementation Decisions

### Corroboration Semantics — PLCY-01
- 1 independent catalog source match → warn-level decision (existing Phase 1 behavior preserved)
- 2 independent sources agree → block (enforce)
- 3 independent sources agree → block + quarantine recommendation surfaced in audit record
- "Independent" = different `catalog_source` values (bumblebee, osv, socket, user); same source in two catalog files does NOT count as two sources
- Thresholds are configurable per-ecosystem in config (users can lower to single-source enforce or raise threshold)
- The corroboration count and which sources agreed/dissented MUST appear in every audit record (`catalog_matches` field with full provenance)
- Unsigned catalog sources (`catalog_signature` absent or unverified) count as 0.5 toward corroboration threshold (warning-only even with 2 unsigned sources; requires at least one signed source for enforcement) — this is the catalog signature requirement (CTLG-07, carried forward from Phase 1 design intent)

### OSV Database Integration — CTLG-02
- **CORRECTION from research:** `github.com/google/osv-scanner/v2` library's `localmatcher` package is internal/unexportable. Use the public OSV REST API: `POST https://api.osv.dev/v1/query` (no auth required, fully public, no large DB download)
- OSV ecosystem names are CASE-SENSITIVE: npm→`npm`, pypi→`PyPI`, go→`Go`, cargo→`crates.io`, rubygems→`RubyGems`, packagist→`Packagist`
- Results cached by package+version with 24h TTL on disk (see osv-cache/ directory below)
- Stored in `~/.beekeeper/catalogs/osv/` (Windows: `%APPDATA%\beekeeper\catalogs\osv\`)
- Sync as part of `beekeeper catalogs sync`; OSV DB is large — only pull ecosystems relevant to the user's projects (configurable, default: npm + pypi + go + cargo + rubygems + packagist)
- OSV DB queries happen synchronously in policy engine (no goroutines in `internal/policy`); the caller thread provides all inputs
- Cache OSV query results per package+version with a short in-memory TTL to avoid re-querying within a single `beekeeper check` invocation (but the cache lives outside `internal/policy` — the policy package receives pre-resolved catalog results)

### Socket Public API — CTLG-03
- Socket PURL endpoint: `POST https://api.socket.dev/v0/purl` — **requires Bearer token authentication** (live test confirmed 401 Unauthorized without token; "no key required" was incorrect)
- Free tier: 500 quota units/hour with a registered token; users must register at socket.dev and configure token in `~/.beekeeper/config.json` as `socket.api_token`
- **DEPRECATION ALERT:** `v0/purl` is deprecated since 2026-01-05, removal announced for 2026-07-30; Phase 2 uses this endpoint; migration to `POST /v0/packages` must be planned before July 30
- If Socket token is absent or empty: treat Socket source as disabled (not a failure); degrade gracefully; log a warning; continue with Bumblebee + OSV only
- Results cached by package+version with 24h TTL on disk in `~/.beekeeper/catalogs/socket-cache/`
- Exponential backoff on HTTP 429: base 1s, max 60s, up to 5 retries; check `Retry-After` header from Socket
- Cache-first: if cached result exists and age < 24h, use it without network call
- Socket query is NOT in the `internal/policy` hot path directly — a catalog source adapter resolves the Socket result before handing a `CatalogMatch` struct to the policy engine
- If Socket API is unavailable (network error, 5xx): degrade to warn-only for packages that would have been corroborated by Socket; log the degradation to audit

### Release-Age Policy — PLCY-02
- Ecosystems covered: npm, PyPI, Cargo, RubyGems, Composer, Go modules
- Default minimum age: 1440 minutes (24 hours), matching pnpm v11
- Configurable per-ecosystem in config (`release_age.npm_minutes`, `release_age.pypi_minutes`, etc.)
- Configurable per-package via allowlist (`release_age.exclude: ["my-org/trusted-pkg"]`)
- Publish timestamp queried from the relevant registry API (npm registry, PyPI JSON API, crates.io API, etc.)
- Registry timestamp query results cached with TTL (24h default) in `~/.beekeeper/catalogs/age-cache/`
- If publish timestamp is unavailable (API error, new package not yet indexed), fail closed: block with reason "publish timestamp unavailable"
- The release-age check is a first-class policy rule, not a catalog match — it fires independently of corroboration

### Lifecycle Script Policy — PLCY-03
- Default deny for `preinstall`, `postinstall`, `install` script fields in package manifests across npm, PyPI (setup.py), Cargo (build scripts), RubyGems (.gemspec), Composer
- Allowlist maintained in `~/.beekeeper/policies/lifecycle.json` (or project-level `.beekeeper/policies/lifecycle.json`)
- Policy check fires when `beekeeper check` receives a tool call that installs a package (shape: `npm install`, `pip install`, `cargo add`, `gem install`, `composer require`)
- Block returns a structured reason citing the specific lifecycle script field and recommends adding the package to the allowlist
- "Lifecycle script present" determination: for npm, inspect `package.json` in the registry response; for others, similar registry inspection
- If registry inspection is unavailable (network error), fail closed (block with reason "lifecycle script check unavailable")

### Sensitive Path Policy — PLCY-04
- Default blocklist (exact paths and prefix patterns, cross-platform resolved):
  - `~/.ssh/` (all files)
  - `~/.aws/` (all files)
  - `~/.gnupg/`
  - `~/.config/Claude/` and all MCP config files enumerated by Bumblebee
  - `~/.config/op/` (1Password CLI)
  - `~/.config/gh/` (GitHub CLI)
  - `~/.netrc`, `~/.npmrc`, `~/.pypirc`, `~/.cargo/credentials.toml`
  - `.env`, `.env.local`, `.env.*` files (glob, anywhere in the tree)
- Policy fires on any `beekeeper check` tool call whose target path matches the blocklist
- User can extend blocklist via config (`sensitive_paths.additional: [...]`)
- User can add a per-path or per-prefix allowlist for agents that legitimately need access
- Structured block returned: includes matched pattern, which config rule matched
- Path matching is platform-aware: `~` resolved to actual home dir, Windows paths normalized

### Network Egress Policy — PLCY-05
- Per-tool egress allowlists in `~/.beekeeper/policies/egress.json`
- Default allow for: common package registries (registry.npmjs.org, pypi.org, crates.io, rubygems.org, pkg.go.dev, etc.), official documentation domains (docs.anthropic.com, etc.)
- Default deny for: paste sites (pastebin.com, hastebin.com, etc.), generic webhooks (webhook.site, etc.), known telemetry endpoints without provenance
- Outbound size limits per tool call (default: 10MB; configurable per tool type)
- Multi-turn exfiltration detection — PLCY-06:
  - Rolling entropy calculation over recent tool outputs (last N tool calls, configurable window)
  - Base64 detection across turns (detect large base64-encoded payloads accumulating across tool calls)
  - Threshold-based: anomalous entropy spike or base64 accumulation triggers warn-level decision
  - No ML — pure counter + entropy math; all thresholds documented in policy config

### Behavioral Baseline Engine — PLCY-07
- Per-project frequency counters keyed by `(tool_name, target_pattern)` stored in `~/.beekeeper/baselines/<project-hash>.json`
- Rolling window (default: 7 days; configurable)
- Counter incremented on every allow/warn decision
- Deviation threshold: if `current_frequency > mean + 3*stddev` for the rolling window, trigger a warn-level decision
- Threshold configurable in config (`baseline.deviation_threshold_sigma`)
- Counter-based only — no ML model training, no external dependencies
- Baseline file is owner-only (`0600`) — contains frequency data that could reveal developer's work patterns

### Output Credential Filtering — PLCY-08
- Redact known credential patterns from tool outputs before returning to agent
- Default redaction patterns: API key prefixes (AKIA*, sk-*, etc.), JWT tokens (`eyJ`... base64 pattern), `Bearer` header values, GitHub tokens (ghp_*, gho_*, etc.), npm tokens (npm_*), AWS secret key patterns
- Redacted payload: `[REDACTED:credential_type]` replacement
- Full original payload recorded in audit log with `contains_credential_patterns: true` but with redacted field (to avoid audit log becoming a credential store)
- Configurable additional regex patterns via config (`output_filter.additional_patterns`)
- This check fires on PostToolUse (tool output filtering), not PreToolUse

### `beekeeper catalogs watch` Daemon — CTLG-06
- Background daemon mode: `beekeeper catalogs watch` (or started by `beekeeper init --daemon`)
- Polls all enabled catalog sources on configurable interval (default: 1h; range 5m–24h)
- On detecting new Bumblebee `threat_intel/` entries (delta from last known state):
  - Records the delta to audit log with full catalog provenance
  - Triggers an immediate targeted scan of locally-installed packages matching the new entries
  - Triggers re-evaluation of recent (last 24h) tool call decisions against the updated catalog
- Delta detection: compare catalog entry count + entry hash fingerprint against cached state in `~/.beekeeper/state.json`
- Runs as a foreground process with `SIGHUP`/`SIGTERM` handling (daemon management is Phase 4+)

### Catalog Sanity Bounds — CTLG-08
- Per-sync delta size limits:
  - Default: alert if delta > 1000 new entries in a single sync (configurable)
  - Hard block if delta > 10000 entries (configurable)
- Per-catalog rule count limits:
  - Alert if total catalog entries > 100000 (configurable)
- Per-package version-list length: alert if a single package has > 1000 version entries
- On exceeding alert thresholds: new entries from that source treated as warning-only until user reviews; existing entries unaffected
- On exceeding hard limits: entire source degraded to warning-only mode
- Degraded-mode state recorded in `~/.beekeeper/state.json` and emitted to audit log with full catalog provenance (source name, version/hash before and after, delta count)
- User can clear degraded mode with `beekeeper catalogs verify --source <name>` (Phase 2 new command)

### Catalog Provenance in Audit Records — CTLG-09
- Every NDJSON audit record that involves a catalog match MUST include the full `catalog_matches` field:
  ```json
  {
    "catalog_matches": [
      {
        "catalog_source": "bumblebee",
        "catalog_version": "<hash or timestamp>",
        "entry_id": "advisory-2026-XXXX",
        "severity": "critical",
        "corroborated": true,
        "dissented": false
      }
    ],
    "corroboration_count": 2,
    "sources_agreed": ["bumblebee", "osv"],
    "sources_dissented": []
  }
  ```
- Decisions where no catalog match was made but other policies fired (release-age, sensitive path, etc.) still record `catalog_matches: []`
- The `internal/policy.Evaluate` return value must include this provenance data; the hook handler writes it to the audit record

### Claude's Discretion
- Exact registry API endpoints and response parsing for each ecosystem's publish timestamp
- In-memory cache architecture for OSV query results within a single `beekeeper check` invocation
- Socket PURL API request body format (beyond what the public API docs specify)
- Specific entropy algorithm for multi-turn exfiltration detection (Shannon entropy is the natural choice)
- State file format for baseline counters and catalog watch delta state
- Daemon process management details for `catalogs watch` (process lifecycle is Phase 4 for full daemon support; Phase 2 may use simple foreground process with signal handling)
- Error message formatting for each new policy block reason
- Whether to expose a `beekeeper catalogs diff` command in Phase 2 (it's in the CLI surface in PRD §10)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project Foundation
- `.planning/PROJECT.md` — Core value, constraints, key decisions, architecture overview
- `.planning/ROADMAP.md` — Phase 2 goal, success criteria, requirements (PLCY-01 through CTLG-09)
- `.planning/REQUIREMENTS.md` — Full requirement definitions for all Phase 2 requirements
- `CLAUDE.md` — Architecture constraints (especially: `internal/policy` pure function constraint, fail-closed requirement, mmap catalog loading, Windows primary dev machine)

### PRD Source
- `beekeeper-prd.md` — Full PRD, especially:
  - §5.1 Catalog matching (corroboration semantics)
  - §5.2 Release-age policy
  - §5.3 Lifecycle script policy
  - §5.6 Sensitive path policy
  - §5.7 Network egress policy
  - §5.8 Behavioral baseline
  - §12.3 Catalog feed integrity (2FA principle, sanity bounds, degraded mode)
  - §15 Phasing (v0.3.0 deliverables)

### Phase 1 Foundation (read before modifying any shared packages)
- `.planning/phases/01-foundation-hook-handler/01-CONTEXT.md` — Phase 1 decisions (CatalogLookup interface, BEEI mmap format, audit record schema)
- `.planning/phases/01-foundation-hook-handler/01-04-SUMMARY.md` — Policy engine implementation summary
- `.planning/phases/01-foundation-hook-handler/01-02-SUMMARY.md` — Catalog sync/mmap index implementation summary

### External References
- OSV REST API: `POST https://api.osv.dev/v1/query` — no auth, public; ecosystem names are CASE-SENSITIVE (npm→`npm`, pypi→`PyPI`, go→`Go`, cargo→`crates.io`). NOTE: `github.com/google/osv-scanner/v2` localmatcher is internal/unexportable — use the REST API, not the library.
- Socket API docs: `https://docs.socket.dev/reference/purl-lookup` — PURL endpoint spec (Bearer token required; deprecated 2026-01-05, removal 2026-07-30)
- `github.com/fsnotify/fsnotify` v1.10.1 — filesystem watcher for `catalogs watch` (already in CLAUDE.md as the locked library for Phase 3)

</canonical_refs>

<specifics>
## Specific Ideas

### v0.3.0 Milestone Targets (from PRD §15)
The PRD explicitly lists v0.3.0 deliverables that map to Phase 2:
- OSV and Socket public API catalog sources
- Lifecycle script policy for npm, pip, pnpm, cargo, gem, composer
- Sensitive path policy
- Network egress policy
- `beekeeper catalogs watch` daemon with catalog-delta-triggered scans
- Corroboration-based catalog matching (Section 5.1): single-source warn, two-source enforce
- Catalog signature verification for all sources that support signing
- Catalog provenance fields in every NDJSON audit record
- Sanity bounds on catalog deltas with degraded-mode trigger
- Read-and-notify degraded mode when catalog source integrity is uncertain
- No remote URL fetching from catalog data; schema strictly static JSON
- Fuzz testing in CI for catalog parser and policy engine (Phase 2 is when fuzz tests become release-gating)

### Key Architectural Constraint from Phase 1
The `CatalogLookup` interface in `internal/catalog` decouples the policy engine from the concrete mmap type. In Phase 2, this interface needs to expand to support multi-source lookups that return `[]CatalogMatch` with source attribution, not just a boolean hit. The Phase 1 mmap index is Bumblebee-only; Phase 2 adds OSV and Socket as additional `CatalogLookup` implementations. The policy engine receives results from all sources and applies corroboration logic.

### Socket PURL API Rate Limit Empirical Validation
Per STATE.md: "Socket PURL API free-tier rate limits undocumented — implement 24h TTL cache per package+version aggressively; validate empirically during Phase 2". The research phase should attempt to find any documentation or empirical data on Socket's public API limits. The 24h TTL cache must be the primary mechanism, with exponential backoff as the fallback.

### Fuzz Testing — Phase 2 Gating (from CLAUDE.md and PRD §12.4)
Phase 2 is when fuzz testing in CI becomes release-gating (not just tracked) for the catalog parser and policy engine. Phase 4 gates on the MCP message parser. Each fuzz target must be behind its own `//go:build fuzz` constraint and wired to the CI release gate.

### fsnotify Windows Behavior
Per STATE.md: "fsnotify Windows behavior with VS Code extension junction points needs live testing". For `catalogs watch`, fsnotify is used in a simpler context (watching `~/.beekeeper/catalogs/` for new files after sync) rather than the full extension directory watching of Phase 3. The Windows junction point issue is more likely to surface in Phase 3 than Phase 2. Phase 2 should still be careful with Windows NTFS and note this as a known research question.

</specifics>

<deferred>
## Deferred Ideas

- Editor extension defense (EDXT-01 through EDXT-06) — Phase 3
- Hook installation (INTG-01, INTG-02) — Phase 4
- MCP gateway (INTG-03 through INTG-07) — Phase 4
- Sentry daemon on any platform — Phase 5+
- LlamaFirewall sidecar (LLMF-01 through LLMF-06) — Phase 6
- Full audit sinks: syslog, OTLP, HTTPS POST (AUDT-03, AUDT-04) — Phase 6
- `beekeeper audit query` and `beekeeper audit export` commands — Phase 6
- TUI dashboard (TUI-01 through TUI-10) — Phase 8
- Policy as code (CODE-01 through CODE-06) — Phase 9
- `beekeeper-self` catalog (CTLG-04, SFDF-06) — Phase 9
- SLSA Level 3 provenance (SFDF-05) — Phase 7
- Desktop notifications for catalog watch — Phase 3 (file watcher with notification is Phase 3 scope)
- `beekeeper catalogs diff` command — may be Phase 2 or Phase 3; implement if straightforward alongside catalogs watch
- Behavioral baseline multi-turn exfiltration: base64 accumulation across turns (PLCY-06) — included in Phase 2 as part of network egress work; multi-turn state requires the baseline engine so it makes sense to colocate

</deferred>

---

*Phase: 02-policy-engine-multi-source-catalogs*
*Context gathered: 2026-05-26 via PRD Express Path*
