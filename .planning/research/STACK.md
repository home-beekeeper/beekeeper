# Stack Research

**Domain:** Go security daemon / agent runtime safety harness
**Researched:** 2026-05-26
**Confidence:** MEDIUM-HIGH (versions verified via web; some API details LOW due to doc access limits)

---

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go | 1.25 (released Aug 2025) | Primary language for all binary components | Single static binary, no CGO for core, memory safety eliminates C-class bugs, `go.sum` integrity verification for all deps; 1.25 adds container-aware GOMAXPROCS and experimental GreenTeaGC |
| `github.com/fsnotify/fsnotify` | v1.10.1 (May 2026) | Cross-platform filesystem notifications for extension watcher | Wraps inotify/FSEvents/ReadDirectoryChangesW behind one API; only justified non-stdlib dep for OS-native file watching |
| `github.com/cilium/ebpf` | v0.21.0 (Mar 2026) | eBPF program loading/attachment for Sentry on Linux | Pure-Go, no CGO, vetted by the Cilium project, production-proven at scale; alternatives require CGO or are kernel-version-specific wrappers |
| `charm.land/bubbletea/v2` | v2.0.6 (Apr 2026) | TUI dashboard | Mature, no CGO, event-driven Elm-architecture, single-binary-friendly; v2 is the current stable release |
| Python | 3.11+ | LlamaFirewall sidecar only | PyTorch/PromptGuard 2 ecosystem is Python-native; keeping it as a sidecar preserves the Go binary boundary |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `charm.land/lipgloss/v2` | latest v2 | TUI styling (colors, layout, borders) | Always pair with bubbletea v2 — moved to same vanity domain |
| `github.com/charmbracelet/bubbles` | v0.x (verify v2 compat) | Reusable TUI components (list, viewport, spinner) | Use for live activity feed, catalog freshness panel — reduces boilerplate |
| `github.com/charmbracelet/x/exp/teatest` | exp | Snapshot testing for bubbletea programs | v2 teatest compatibility — API is experimental, wrap it in internal helpers |
| `github.com/google/osv-scanner/v2` | v2.3.8 (May 2026) | OSV vulnerability database integration | Use as Go library (`github.com/google/osv-scanner/v2`) rather than shelling out — avoids process spawning on the hot path |
| `github.com/sigstore/cosign/v2` | latest v2.x | Release signing in CI (not a runtime dep) | Build/release tooling only; use keyless signing via GitHub Actions OIDC |
| `github.com/slsa-framework/slsa-github-generator` | v2.1.0+ | SLSA Level 3 provenance (CI only) | Reference via `builder_go_slsa3.yml@v2.1.0` in release workflow |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| GoReleaser v2.13.0+ | Cross-platform binary release with cosign signing | Required for cosign v3 bundle format (`.sigstore.json`); earlier versions use incompatible v2 sig format |
| `cosign` v3.x | Keyless artifact signing | Use `--bundle` flag (not `--output-signature` + `--output-certificate`); requires GoReleaser v2.13+ |
| `osv-scanner` CLI v2.3.8 | Supply chain scanning (offline DB sync) | Invoke via `--offline --download-offline-databases` for local DB; use as library for programmatic use |
| `govulncheck` | Go stdlib vuln scanning in CI | Separate from OSV; covers Go module graph specifically |
| `golangci-lint` | Static analysis | Pin version in CI; run on all three OS targets |
| Renovate | Automated dependency updates | Pin `go.mod` and `go.sum`; Renovate PRs get second-account approval before merge |

---

## Go 1.25 — What Changed for This Use Case

**Release date:** August 2025. **Confidence: HIGH** (verified against go.dev/doc/go1.25)

### `encoding/json`
`encoding/json/v2` is now available as `GOEXPERIMENT=jsonv2`. It exposes `encoding/json/jsontext` for lower-level streaming JSON. **Do NOT enable for the policy engine hot path in v0.1.0** — it is still experimental and not subject to Go 1 compatibility guarantees. The working group targets Go 1.26 for stable adoption. Use stdlib `encoding/json` for the policy engine today; the performance profile is sufficient for sub-100ms targets at catalog-matching scale. Revisit when v2 stabilizes in ~Go 1.26 (Q1 2027 estimate).

**What this means for the NDJSON audit log hot path:** `encoding/json` in Go 1.25 with the new experimental GC (`GOEXPERIMENT=greenteagc`) shows 10-40% GC overhead reduction. Keep the audit log writer as a simple `json.NewEncoder(f).Encode(record)` call — this is already the idiomatic pattern and benefits from GC improvements automatically.

### `net/http`
New `http.CrossOriginProtection` middleware and SHA-1 TLS handshake rejection (RFC 9155). The MCP gateway daemon uses `net/http` as its transport. **Enable CORS protection on the gateway** — even on localhost binding, defense in depth. SHA-1 rejection is a net positive.

### `os/exec`
No breaking changes in 1.25 for `os/exec`. The shim layer and Bumblebee invocation patterns from 1.24 carry forward unchanged.

### Crypto
4x signing speedup for ECDSA/Ed25519 in FIPS mode, 2x SHA-1 hashing via SHA-NI. Relevant if Beekeeper ever runs in a FIPS-140-3 environment (enterprise deployments). Not a blocker for v0.1.0.

### Container-aware GOMAXPROCS
Auto-adjusts for cgroup CPU limits on Linux. The Sentry daemon running inside a Docker container in CI gets correct parallelism without manual `runtime.GOMAXPROCS` calls.

---

## `fsnotify` — v1.10.1 Windows Gotchas

**Version:** v1.10.1, released May 4, 2026. **Confidence: HIGH** (verified pkg.go.dev)

**Requires Go 1.23+.** This is fine for Beekeeper's Go 1.25+ requirement.

### Windows ReadDirectoryChangesW — What to Know

**Recursive watching is NOT in the public API.** `fsnotify` does not expose `Watch("/path", Recursive)`. The recursive code path exists internally for test purposes only. This is a critical design constraint for the extension watcher watching `~/.vscode/extensions/` (flat directory with many subdirectories).

**Implication for Beekeeper:** You must enumerate watched directories explicitly or watch the parent and filter events by path prefix. Watching `~/.vscode/extensions/` catches new directory creation (each extension is a directory); you do not need recursive watching for the extension install detection use case. For the general file watcher, call `watcher.Add()` per directory.

**Buffer size.** Default is 64KB (`WithBufferSize` default). During `npm install` or extension installs, event bursts can overflow. Use `watcher.AddWith(path, fsnotify.WithBufferSize(262144))` — 256KB — for directories that see heavy churn. This applies specifically to `node_modules/` watching if ever needed; for extension dirs the 64KB default is sufficient.

**Windows Write events on parent dirs.** When a child entry is created inside a watched directory, the parent directory itself receives a `Write` event (NTFS last-write-time update). Filter by `event.Op == fsnotify.Create` for the extension directory watcher to avoid acting on spurious `Write` events.

**Chmod events never fire on Windows.** Do not write code that depends on `fsnotify.Chmod` to detect permission changes on Windows — it is silently dropped.

**Path formats.** Accept both `C:\path` and `C:/path` in config; fsnotify handles both.

**What NOT to do:** Do not attempt to enable recursive watching by calling `watcher.Add()` on every subdirectory dynamically — this creates a race condition during rapid directory creation (extension install). Watch the parent dir, filter `Create` events by directory type.

---

## `cilium/ebpf` — v0.21.0 Kernel Requirements

**Version:** v0.21.0, released March 5, 2026. **Confidence: MEDIUM** (releases page verified; feature/kernel mapping from kernel docs and community sources)

### Kernel Version Matrix for Sentry's Three Event Streams

| Feature | Minimum Kernel | Notes |
|---------|---------------|-------|
| Basic eBPF program loading | 4.4+ | EOL'd; CI tests against LTS kernels |
| kprobes/tracepoints | 4.4+ | Core process event capture |
| `CAP_BPF` capability | 5.8 | Before 5.8, need `CAP_SYS_ADMIN`; use `rlimit` shim for older kernels |
| `BPF_MAP_TYPE_RINGBUF` | 5.8 | Preferred over perf event arrays — lower overhead, no per-CPU allocation |
| `fentry`/`fexit` probes (BTF-based) | 5.5 | Better than kprobes for stable kernel ABI |
| CO-RE (Compile Once, Run Everywhere) | 5.8 | Required for shipping pre-compiled eBPF; alternatives need per-kernel compilation |
| `bpf_link` (stable attachment) | 5.7 | Without this, probe detaches on process exit |
| Network socket events via `sock_ops` | 5.4+ | TCP connection tracing |
| fanotify `FAN_REPORT_FID` | 5.1 | File identity in events |
| fanotify `FAN_REPORT_PIDFD` | 5.15 (also 5.10.220 LTS) | Process identity in file events |

**Recommended minimum for Sentry (Linux):** **Kernel 5.15** — gets you `bpf_link`, `BPF_MAP_TYPE_RINGBUF`, CO-RE, and `FAN_REPORT_PIDFD` in fanotify. This aligns with Ubuntu 22.04 LTS (kernel 5.15) and RHEL 9 (kernel 5.14). Kernel 5.10 LTS is acceptable if `FAN_REPORT_PIDFD` is backported (5.10.220+).

**Practical target:** Ubuntu 22.04+ (`ubuntu-latest` on GitHub Actions uses 22.04, kernel 5.15).

**eBPF for Windows:** `cilium/ebpf` lists Windows Server 2022 as tested. Beekeeper's Windows Sentry uses ETW, not eBPF — do not use `cilium/ebpf` on Windows.

**fanotify is separate from eBPF.** fanotify is a standard Linux syscall API (`CAP_SYS_ADMIN` required). Use it directly via `golang.org/x/sys/unix` — no eBPF library needed for file events. Use `cilium/ebpf` only for the process-creation and network-connection event streams via kprobe/tracepoint attachment.

**Key gotcha:** `cilium/ebpf` requires `CGO=0` is NOT set — it is pure Go, no CGO, which is correct. But you DO need the kernel headers or BTF info at compile time for CO-RE programs. The standard pattern is to embed pre-compiled eBPF bytecode using `go:generate` + `bpf2go`, then ship the bytecode in the binary. Do this from the start; retrofitting is painful.

---

## Bubble Tea v2 — Current State and Gotchas

**Version:** v2.0.6, released April 16, 2026. **Import path: `charm.land/bubbletea/v2`** (changed from `github.com/charmbracelet/bubbletea`). **Confidence: HIGH** (verified GitHub releases)

### v1 vs v2 — Use v2

v2 is the current stable release. v1 (last: v1.3.10) is in maintenance-only mode. Start on v2.

**Breaking changes that matter for Beekeeper:**
- `View()` returns `tea.View` (a struct), not `string`. Use `tea.NewView("content")`.
- `tea.KeyMsg` is now an interface; use `tea.KeyPressMsg` for key press handling.
- Import `charm.land/lipgloss/v2` not the old path.

### Windows Terminal Known Issues (Confirmed Bugs as of May 2026)

**CRITICAL — Window resize events not detected (Issue #1601).** Beekeeper's TUI dashboard runs `beekeeper dashboard` on Windows. Terminal resize events (`WindowSizeMsg`) are never fired after the initial startup on Windows. This is a regression from v1 introduced by switching to VT input mode. The dashboard layout will not reflow when the user resizes their terminal window on Windows.

**Mitigation:** Implement a resize polling fallback — use a goroutine that polls `os.Stdout` console size via `golang.org/x/term` every 500ms and sends a synthetic `WindowSizeMsg` when dimensions change. This is the workaround until the upstream issue is resolved.

**Escape sequence leak in short-lived programs (Issue #1627).** v2 queries terminal capability on init (Synchronized Output mode 2026 / Unicode Core mode 2027). If the program exits too quickly, raw escape sequences leak to the shell. The `beekeeper check` hook handler is a short-lived process — **do not use Bubble Tea for the hook handler output**. Use plain `fmt.Println` / `os.Stderr.WriteString`. Bubble Tea is only for `beekeeper dashboard` which is long-lived.

**Window title not reset on panic (Issue #1474).** If Beekeeper panics during dashboard mode, the terminal title stays set. Add a `defer` that resets the title via ANSI escape before the panic propagates.

### Snapshot Testing

`teatest` lives in `github.com/charmbracelet/x/exp/teatest` — note `exp` namespace, API unstable. A new `charm-test` framework proposal opened April 1, 2026 (Issue #1654) but is not available yet.

**Recommendation:** Use `teatest` with `x/exp/golden` for TUI snapshot tests but wrap it in an internal `beekeepertest` package so you isolate the unstable API. When `charm-test` stabilizes, migration is a one-file change. Do not reference `teatest` directly from test files outside the wrapper.

**v2 compatibility of teatest:** As of May 2026, teatest is being updated for v2 but verify `charm.land/x/exp/teatest` (v2 namespace) availability before building TUI tests.

---

## MCP Protocol — 2026 Spec Changes Affecting the Gateway

**Confidence: HIGH** (verified against the 2026-07-28 release candidate blog post)

The final MCP 2026 spec ships **July 28, 2026**. The 10-week RC window started in May 2026, meaning the spec changes are finalized but SDKs may lag.

### What Changes for a Proxy/Gateway Implementation

**`Mcp-Method` and `Mcp-Name` required headers (SEP-2243).** Every Streamable HTTP request now carries these headers so load balancers and proxies can route by operation without body inspection. Beekeeper's gateway MUST:
1. Read and forward these headers on inbound requests.
2. Reject or flag requests where header and body disagree (servers do; gateway should too for defense in depth).
3. Use `Mcp-Method` for rate limiting specific operations without JSON parsing.

**Session model eliminated.** `Mcp-Session-Id` is gone. The `initialize`/`initialized` handshake is removed. Client metadata travels in `_meta` on every request. **This is the most impactful change for the gateway.** Beekeeper's v0.6.0 gateway was designed when sessions were stateful; the new model makes it stateless. Every request now carries full context — simpler per-request policy evaluation, no session state to maintain.

**`ttlMs` and `cacheScope` on list responses.** The gateway can now cache `tools/list` responses per the server's declared TTL without a long-lived SSE stream. Cache at the gateway layer for frequently-polled MCP clients.

**W3C Trace Context in `_meta`.** `traceparent`, `tracestate`, `baggage` keys are now standardized. Beekeeper's OTLP audit sink can correlate with distributed traces from MCP clients/servers by forwarding these headers.

**Authorization.** Clients must validate the `iss` parameter per RFC 9207. The gateway should validate this before forwarding to upstream MCP servers.

### Recommended Gateway Design for July 2026 Spec

Design the gateway as a **stateless HTTP proxy** from the start (not session-based). Each request carries full context in `_meta`. Per-request policy evaluation with no session state simplifies v0.6.0 implementation considerably. The old spec required session affinity; the new spec does not. This is a good thing — simpler implementation, easier horizontal scaling if ever needed.

**What NOT to do:** Do not implement session tracking for MCP routing. The spec explicitly removed it. Any code that tracks `Mcp-Session-Id` is dead code against the July 2026 spec.

---

## Sigstore / Cosign — Keyless Signing Toolchain

**Confidence: MEDIUM-HIGH** (cosign v3 bundle format verified; OIDC flow verified against official docs and GoReleaser blog)

### Current Toolchain (2026)

Use **cosign v3.x** with the `--bundle` flag. cosign v3 replaced the two-file output (`--output-signature` + `--output-certificate`) with a single `.sigstore.json` bundle. GoReleaser v2.13.0+ supports cosign v3 natively.

**Old pattern (v2, DO NOT USE):**
```
cosign sign-blob --output-signature sig.txt --output-certificate cert.pem artifact
```

**Current pattern (v3):**
```
cosign sign-blob --bundle artifact.sigstore.json --yes artifact
```

### GitHub Actions OIDC Flow

Required permissions on the release job:
```yaml
permissions:
  id-token: write    # OIDC token for keyless signing
  contents: write    # Upload release artifacts
  actions: read      # Read workflow path for SLSA provenance
```

No long-lived signing keys. Fulcio issues ephemeral certificates bound to the GitHub Actions OIDC token. The certificate identity is the workflow URL; anyone can verify with:
```
cosign verify-blob \
  --bundle artifact.sigstore.json \
  --certificate-identity "https://github.com/org/beekeeper/.github/workflows/release.yml@refs/tags/v*" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  artifact
```

### GoReleaser Integration

`.goreleaser.yaml` signing stanza:
```yaml
signs:
  - cmd: cosign
    signature: "${artifact}.sigstore.json"
    args:
      - sign-blob
      - "--bundle=${signature}"
      - "${artifact}"
      - "--yes"
    artifacts: all
```

**GoReleaser reference:** `goreleaser/example-supply-chain` repo demonstrates the full pattern including SBOM generation.

### SBOM

Use `syft` to generate CycloneDX SBOM as part of the GoReleaser pipeline. GoReleaser has native `sboms:` config with `syft` integration.

---

## SLSA Level 3 — GitHub Actions Setup

**Confidence: HIGH** (slsa-github-generator README verified; v2.1.0 is current)

### Current Recommended Setup

Use `slsa-framework/slsa-github-generator` v2.1.0 Go builder. Reference by full semantic version tag — not `@main` or `@v2`.

**Critical:** All versions through v1.9.0 have a TUF mirror error. Minimum viable version is **v1.10.0**; use v2.1.0 (current).

**Release workflow skeleton:**
```yaml
name: Release
on:
  push:
    tags: ['v*']

jobs:
  build:
    permissions:
      id-token: write
      contents: write
      actions: read
    uses: slsa-framework/slsa-github-generator/.github/workflows/builder_go_slsa3.yml@v2.1.0
    with:
      go-version: "1.25"
      config-file: .github/workflows/slsa-goreleaser.yml
      upload-assets: true
```

**Outputs:** `go-binary-name` (binary filename) and `go-provenance-name` (`.intoto.jsonl` provenance file).

**Artifact download:** Must use `actions/download-artifact@v3` (not v4) due to an incompatibility with the provenance artifact format.

**Private repos:** All builds post to the public Rekor transparency log by default. Set `private-repository: true` only if acceptable that repo name appears in public logs — for a public Apache 2.0 project this is not a concern.

### SLSA Phase Targeting

- **v0.1.0:** Sigstore keyless signing only (no SLSA provenance yet — acceptable for early releases).
- **v0.9.0:** Add SLSA Level 3 provenance via `builder_go_slsa3.yml` (as planned in the PRD).
- **v1.0.0:** SLSA + SBOM + reproducible build verification script (`make verify-release`).

### Verification for Users

```bash
slsa-verifier verify-artifact beekeeper-linux-amd64 \
  --provenance-path beekeeper-linux-amd64.intoto.jsonl \
  --source-uri github.com/your-org/beekeeper \
  --source-tag v1.0.0
```

---

## Bumblebee NDJSON Schema

**Confidence: HIGH** (verified against perplexityai/bumblebee GitHub repo, v0.1.1)

**Current Bumblebee version:** v0.1.1 (`go install github.com/perplexityai/bumblebee/cmd/bumblebee@v0.1.1`)

### Record Types

Bumblebee emits three record types: `package`, `finding`, `scan_summary`.

### Package Record (canonical fields)

```json
{
  "record_type": "package",
  "record_id": "<uuid>",
  "schema_version": "0.1.0",
  "scanner_name": "bumblebee",
  "scanner_version": "v0.1.1",
  "run_id": "<uuid>",
  "scan_time": "<RFC3339>",
  "endpoint": {
    "hostname": "...",
    "os": "darwin|linux|windows",
    "arch": "amd64|arm64",
    "username": "...",
    "uid": "...",
    "device_id": "..."
  },
  "profile": "...",
  "ecosystem": "npm|pypi|go|rubygems|packagist|cargo|editor-extension|browser-extension|mcp",
  "package_name": "...",
  "normalized_name": "...",
  "version": "...",
  "project_path": "...",
  "root_kind": "...",
  "package_manager": "...",
  "source_type": "...",
  "source_file": "...",
  "has_lifecycle_scripts": false,
  "confidence": "high|medium|low"
}
```

### Finding Record (exposure match)

```json
{
  "record_type": "finding",
  "finding_type": "package_exposure",
  "severity": "critical|high|medium|low",
  "catalog_id": "...",
  "catalog_name": "...",
  "evidence": "...",
  // ...plus all package base fields
}
```

### Exposure Catalog Format (threat_intel/)

```json
{
  "schema_version": "0.1.0",
  "entries": [
    {
      "id": "advisory-2026-XXXX",
      "name": "...",
      "ecosystem": "npm",
      "package": "nx-console-vscode",
      "versions": ["1.2.3"],
      "severity": "critical"
    }
  ]
}
```

**Key constraint:** The schema requires top-level `schema_version` and `entries` keys. Bare arrays are rejected. Beekeeper's extended catalog schema (with `source_url`, `catalog_signature`, `catalog_source`) must remain an extension — compatible with Bumblebee's schema, not a replacement.

**Beekeeper `scanner_name`:** Set `"scanner_name": "beekeeper"` in Beekeeper-generated records; `"scanner_name": "bumblebee"` in records that pass through from Bumblebee invocations.

---

## OSV Database — Offline Sync

**Confidence: HIGH** (osv.dev docs and osv-scanner v2 docs verified)

**OSV-Scanner version:** v2.3.8 (May 8, 2026)

### Offline DB Structure

```
{OSV_SCANNER_LOCAL_DB_CACHE_DIRECTORY}/osv-scanner/{ECOSYSTEM}/all.zip
```

`OSV_SCANNER_LOCAL_DB_CACHE_DIRECTORY` defaults to the OS cache dir. Beekeeper should set this explicitly to `~/.beekeeper/catalogs/osv/`.

### Download URLs (direct GCS, no SDK needed)

```
https://osv-vulnerabilities.storage.googleapis.com/{ECOSYSTEM}/all.zip
```

Ecosystem list: `https://osv-vulnerabilities.storage.googleapis.com/ecosystems.txt`

Example:
```
https://osv-vulnerabilities.storage.googleapis.com/npm/all.zip
https://osv-vulnerabilities.storage.googleapis.com/PyPI/all.zip
https://osv-vulnerabilities.storage.googleapis.com/Go/all.zip
```

Beekeeper can download these directly in the catalog sync daemon without invoking the `osv-scanner` CLI — just `http.Get` + write to the expected path. This is simpler, faster, and avoids a subprocess for the hourly sync.

### Programmatic Use

`github.com/google/osv-scanner/v2` is importable as a Go library. For Beekeeper's policy engine hot path, import the library rather than shelling out to the CLI. The library exposes the database query logic. Shelling out to `osv-scanner` is acceptable for the `beekeeper scan` command where latency is not critical; avoid it for per-tool-call evaluation.

### File Format Inside all.zip

Each ZIP contains individual vulnerability JSON files per OSV advisory ID, following the OSV schema (https://ossf.github.io/osv-schema/). Each file is one JSON object with fields: `id`, `aliases`, `related`, `published`, `modified`, `affected` (with `package.ecosystem`, `package.name`, `ranges`, `versions`), `severity`, `details`.

---

## Socket Public API

**Confidence: MEDIUM** (endpoint URL verified; rate limits and full ecosystem support partially verified; score endpoint marked deprecated)

### Known Endpoints

| Endpoint | Method | Notes |
|----------|--------|-------|
| `https://api.socket.dev/v0/npm/{package}/{version}/score` | GET | **Deprecated** — use successor |
| `https://api.socket.dev/v0/purl` | POST | PURL-based multi-ecosystem lookup; preferred |
| SBOM export | GET | CycloneDX, beta, requires `report:read` scope |

**Authentication:** Bearer token. API tokens available from Socket dashboard.

**Free tier:** Socket is free for open-source use. The public website (socket.dev) shows package scores without auth. The REST API requires a token but open-source projects get free quota. Each endpoint call consumes 1 quota unit. The public MCP server at `https://mcp.socket.dev/` requires no API key at all — explore this as a zero-auth catalog source.

### Supported Ecosystems (REST API)

npm and PyPI have full behavioral analysis. Go, Maven, RubyGems, Cargo, NuGet have vulnerability + supply-chain coverage (less deep than npm/PyPI). The CycloneDX SBOM export endpoint supports: `crates`, `go`, `maven`, `npm`, `nuget`, `pypi`, `rubygems`.

### Recommended Integration Pattern

```go
// Beekeeper catalog lookup pattern
type SocketScoreRequest struct {
    PURL string `json:"purl"`
}

// Use PURL endpoint for multi-ecosystem:
// POST https://api.socket.dev/v0/purl
// Body: {"purl": "pkg:npm/lodash@4.17.20"}
// Auth: Bearer <token>
```

**Corroboration note:** Single Socket hit = warn only (Beekeeper's corroboration model). Two independent sources (e.g., Socket + Bumblebee) = enforce. Do not over-weight Socket results.

**Rate limit mitigation:** Cache Socket API results per package+version in `~/.beekeeper/catalogs/socket/`. TTL 24h. On cache miss, query live API. This keeps daily API calls to the delta of new packages seen, not all packages.

---

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| `charm.land/bubbletea/v2` | `tview` / `tcell` | tview has richer widget set (tables, forms) but is less idiomatic for event-driven agent monitoring; use tview if you need a full-featured data grid |
| `github.com/cilium/ebpf` | `libbpfgo` (CGO wrapper) | libbpfgo if you need features newer than cilium/ebpf supports; CGO cost is real for a pure-Go binary |
| `github.com/fsnotify/fsnotify` | `golang.org/x/sys/windows` + manual ReadDirectoryChangesW | Direct syscall if you need USN journal-based watching (better for high-churn dirs); more code, same reliability |
| Direct GCS download for OSV | `osv-scanner` CLI subprocess | CLI subprocess is fine for the daily `beekeeper scan` command; use direct download + library for the hourly catalog sync daemon |
| Keyless cosign + SLSA | Long-lived signing keys | Long-lived keys only if deploying in an air-gapped environment without GitHub Actions OIDC access |
| `encoding/json` stdlib | `encoding/json/v2` (jsonv2) | Adopt jsonv2 when it stabilizes in Go 1.26; the performance gains (substantially faster decoding) are real but not worth an experimental dep for a security tool |

---

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| CGO in the core binary | Breaks cross-compilation, adds C-class memory bugs, complicates static binary | Pure-Go throughout; sidecars for Python-ecosystem code |
| `encoding/json/v2` (`GOEXPERIMENT=jsonv2`) in production | Experimental, not covered by Go 1 compat promise, API may change in Go 1.26 | `encoding/json` stdlib; revisit for Go 1.26 |
| `charm.land/bubbletea/v2` for short-lived processes | Escape sequence leak on quick exit (Issue #1627) | Plain `fmt.Fprintf` / `os.Stderr` for `beekeeper check`, hook handler, and any sub-100ms process |
| `fsnotify` recursive watching | Not in public API; internal implementation only | Watch each extension directory explicitly with `watcher.Add()` per path |
| `Mcp-Session-Id` tracking in the MCP gateway | Removed from July 2026 spec; dead code | Stateless per-request proxy with `_meta` context |
| `cosign sign-blob --output-signature --output-certificate` | cosign v2 format; GoReleaser v2.13+ uses v3 bundle | `cosign sign-blob --bundle artifact.sigstore.json --yes` |
| `slsa-github-generator` before v1.10.0 | TUF mirror error affects all versions ≤ v1.9.0 | v2.1.0+ |
| Socket score endpoint (`/v0/npm/{pkg}/{ver}/score`) | Marked deprecated | PURL endpoint (`/v0/purl`) |
| `github.com/charmbracelet/bubbletea` (old import) | v1, maintenance-only; v2 moved to vanity domain | `charm.land/bubbletea/v2` |
| Bare array in Bumblebee catalog files | Schema validation rejects arrays at top level | Wrap in `{"schema_version": "0.1.0", "entries": [...]}` |

---

## Stack Patterns by Variant

**For the hook handler (`beekeeper check`, sub-100ms, short-lived):**
- No Bubble Tea
- No HTTP client calls on the critical path (use cached catalog)
- `encoding/json` for stdin parsing — do not allocate; decode into pre-allocated structs
- Exit 0 (allow) or exit 1 (block) — no framework needed

**For the gateway daemon (long-lived, `net/http`):**
- `net/http` with `CrossOriginProtection` middleware
- Stateless per-request design (July 2026 MCP spec)
- Forward `Mcp-Method` and `Mcp-Name` headers; validate before forwarding
- Per-session token via `Authorization: Bearer` even on localhost

**For the Sentry daemon (Linux, privileged):**
- `cilium/ebpf` for process + network event streams via kprobe/tracepoint
- `golang.org/x/sys/unix` for fanotify file access events (not cilium/ebpf — fanotify is a syscall, not eBPF)
- Embed pre-compiled eBPF bytecode via `bpf2go` + `go:generate`
- Minimum kernel 5.15 for full feature set; degrade gracefully on older kernels

**For the TUI dashboard (`beekeeper dashboard`, long-lived):**
- `charm.land/bubbletea/v2` with resize-poll goroutine (Windows workaround)
- `charm.land/lipgloss/v2` for styling
- Wrap `teatest` in internal package for snapshot tests

**For Windows Sentry (ETW, no eBPF):**
- `golang.org/x/sys/windows` for ETW provider subscription
- No `cilium/ebpf` — ETW is a separate subsystem

---

## Version Compatibility

| Package | Compatible With | Notes |
|---------|----------------|-------|
| `charm.land/bubbletea/v2` v2.0.6 | `charm.land/lipgloss/v2` | Both must be v2; mixing old lipgloss with new bubbletea breaks |
| `github.com/cilium/ebpf` v0.21.0 | Go 1.23+ | v0.21.0 requires Go 1.23; Beekeeper's Go 1.25 requirement is compatible |
| `github.com/fsnotify/fsnotify` v1.10.1 | Go 1.23+ | Same minimum; compatible |
| `github.com/google/osv-scanner/v2` v2.3.8 | Go 1.21+ | Compatible with Go 1.25 |
| GoReleaser v2.13.0+ | cosign v3.x | Earlier GoReleaser versions ship `.sig` files (cosign v2 format) incompatible with v3 verification |
| `slsa-github-generator` v2.1.0 | `actions/download-artifact@v3` | NOT v4 — provenance artifact format incompatibility |

---

## Installation

```bash
# Core binary deps (go.mod)
go get github.com/fsnotify/fsnotify@v1.10.1
go get charm.land/bubbletea/v2@v2.0.6
go get charm.land/lipgloss/v2
go get github.com/charmbracelet/bubbles  # verify v2 compat tag
go get github.com/cilium/ebpf@v0.21.0    # Linux Sentry only; build-tag guarded
go get github.com/google/osv-scanner/v2@v2.3.8

# eBPF toolchain (Linux CI only)
go install github.com/cilium/ebpf/cmd/bpf2go@latest

# Release toolchain (CI only, not in go.mod)
go install github.com/sigstore/cosign/v2/cmd/cosign@latest
# GoReleaser via goreleaser-action in GitHub Actions
```

---

## Sources

- [go.dev/doc/go1.25](https://go.dev/doc/go1.25) — Go 1.25 release notes (HIGH confidence)
- [pkg.go.dev/github.com/fsnotify/fsnotify](https://pkg.go.dev/github.com/fsnotify/fsnotify) — v1.10.1 docs, Windows caveats (HIGH confidence)
- [github.com/cilium/ebpf/releases](https://github.com/cilium/ebpf/releases) — v0.21.0 release (HIGH confidence for version; MEDIUM for kernel-feature mapping)
- [github.com/charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) — v2.0.6 release, Windows issues #1601 #1627 (HIGH confidence)
- [blog.modelcontextprotocol.io/posts/2026-07-28-release-candidate/](https://blog.modelcontextprotocol.io/posts/2026-07-28-release-candidate/) — July 2026 MCP spec RC (HIGH confidence)
- [github.com/perplexityai/bumblebee](https://github.com/perplexityai/bumblebee) — NDJSON schema v0.1.0 (HIGH confidence)
- [google.github.io/osv-scanner/](https://google.github.io/osv-scanner/) — v2.3.8, offline mode (HIGH confidence)
- [docs.socket.dev/reference/getscorebynpmpackage](https://docs.socket.dev/reference/getscorebynpmpackage) — endpoint docs, deprecated status (MEDIUM confidence; full rate limit docs not publicly accessible)
- [github.com/slsa-framework/slsa-github-generator](https://github.com/slsa-framework/slsa-github-generator) — v2.1.0 Go builder README (HIGH confidence)
- [goreleaser.com/blog/cosign-v3/](https://goreleaser.com/blog/cosign-v3/) — cosign v3 bundle migration (HIGH confidence)
- [man7.org/linux/man-pages/man7/fanotify.7.html](https://www.man7.org/linux/man-pages/man7/fanotify.7.html) — fanotify kernel version matrix (HIGH confidence)
- [github.com/golang/go/issues/71497](https://github.com/golang/go/issues/71497) — encoding/json/v2 adoption timeline (MEDIUM confidence)

---
*Stack research for: Beekeeper — Go-based agent runtime safety harness*
*Researched: 2026-05-26*
