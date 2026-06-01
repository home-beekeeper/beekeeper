# Beekeeper Milestone 2: Pollen

**A Beekeeper-owned Windows-compatible inventory layer, vendored from Bumblebee.**

Version 0.1 PRD. Mfanafuthi Mhlanga / Mzansi Agentive Pty Ltd. June 2026.

---

## 1. Why this milestone exists

Milestone 1 (Beekeeper v0.1.0 through v1.0.0) is built. The system is in production testing on the maintainer's machine and the cross-platform CI matrix established in PRD Section 13.4 is the gate that protects test discipline going forward.

That gate has hit an external dependency it cannot resolve internally: **upstream Bumblebee does not support Windows, and the merge timeline is unclear.**

The constraint chain:

1. PRD Section 14 (Phasing) requires the GitHub Actions matrix to run all tests on `ubuntu-latest`, `macos-latest`, and `windows-latest` from v0.1.0 onward.
2. PRD Section 13.4 includes the Bumblebee compatibility test as a first-class CI assertion: Beekeeper and Bumblebee run on the same machine, NDJSON output is schema-consistent, no double-counting of findings, audit log records correctly attribute `scanner_name`.
3. Upstream Bumblebee v0.1.1 (released May 22 2026) ships macOS and Linux binaries only. PR #4 (Windows root discovery) and PR #16 (full Windows support) have been open without merge signal for 3+ weeks. The upstream backlog is now 18 open PRs against 8 closed, suggesting the maintainers are working through the queue but have not prioritized Windows.
4. The Beekeeper CI matrix on Windows fails any test that depends on Bumblebee inventory, because the Bumblebee binary either crashes on Windows paths or returns empty results from a Unix-only root resolver.
5. The wrong response is to disable Windows CI or to wrap failing tests in silent `t.Skip()` calls. Both corrode the discipline the PRD specifically guards against. Skip-and-rot is how cross-platform support quietly dies.
6. The right response is to **own Windows inventory compatibility ourselves.** Apache 2.0 permits forking. Go vendoring is a standard practice. The fork is bounded in scope (Windows parity only) and the path back to upstream contribution is open whenever they are ready to accept it.

This milestone exists because the alternative is silent test rot.

## 2. Bumblebee dependency context

What Beekeeper actually depends on from Bumblebee, with current upstream status:

### 2.1 Functional dependency

Beekeeper consumes Bumblebee as the inventory source for the `beekeeper scan` orchestrator (Section 5.5 of the main PRD: Sentry process correlation engine, and Section 12.6: `beekeeper-self` catalog self-detection). Bumblebee enumerates on-disk metadata across:

- Eight language package ecosystems: npm, pnpm, Yarn, Bun (all emitted as `npm`), PyPI, Go modules, RubyGems, Composer.
- Editor extensions: VS Code, Cursor, Windsurf, VSCodium manifests.
- Browser extensions: Chromium-family `manifest.json` and Firefox `extensions.json` per profile.
- MCP host configs: `mcp.json`, `.mcp.json`, `claude_desktop_config.json`, `mcp_config.json`, `mcp_settings.json`, `cline_mcp_settings.json`, `~/.gemini/settings.json`.

Output is NDJSON records (`record_type: "package" | "finding" | "scan_summary"`) with a stable schema documented at `schema_version: "0.1.0"`. Beekeeper's audit log is schema-compatible with this format, deliberately.

### 2.2 Source dependency

Upstream repository: `github.com/perplexityai/bumblebee`.

Latest tagged release: **v0.1.1** (May 22 2026). One release in the repo's lifetime. Active development continues on `main`.

License: **Apache License 2.0.** Permits modification, redistribution, derivative works. Requires preservation of LICENSE and NOTICE files, attribution of changes, no use of upstream trademarks in our derivative.

Stars: 2.1k. Forks: 161. Five commits on main since launch.

### 2.3 Windows blocker, in detail

Upstream's default root resolver is implemented for Unix only. The README states scope explicitly: *"Bumblebee is a read-only inventory collector for package, extension, and developer-tool metadata on macOS and Linux developer endpoints."* The Windows code path is not present in `cmd/bumblebee/`, `internal/`, or the inventory source documentation.

Specific missing pieces:

- Default root discovery for Windows: no enumeration of `%APPDATA%`, `%LOCALAPPDATA%`, `%ProgramFiles%`, `%USERPROFILE%`, drive-letter handling.
- Path representation in NDJSON records: upstream normalizes paths in ways that do not round-trip cleanly through Windows backslash separators and drive letters. PR #1 (open) flags this.
- Editor extension paths for Windows (`%APPDATA%\Code\User\extensions\`, `%USERPROFILE%\.cursor\extensions\`, etc.).
- Browser extension paths for Windows (`%LOCALAPPDATA%\Google\Chrome\User Data\<Profile>\Extensions\`, Firefox profile paths under `%APPDATA%\Mozilla\Firefox\Profiles\`).
- MCP host config locations for Windows-installed Claude Desktop, Cursor, and equivalents.

These are not architectural changes; they are enumeration tables plus path-handling discipline using `filepath.Join`. The Go stdlib supports cross-platform file system operations cleanly; what is missing is the Windows knowledge of *where to look.*

### 2.4 Upstream signals worth tracking

- **Issue #2** (Windows root discovery): open since launch, last activity recent. The canonical tracking issue.
- **PR #4** (austinconnor): claims to fix both #1 and #2 in a single commit with build-tag-separated implementation. 3+ weeks open. No reviews, no labels, no maintainer comments.
- **PR #16**: parallel Windows attempt by a different contributor. Also open.
- **Issue #21** (live OSV.dev source): not blocking us, but a useful upstream feature when it lands.
- **Issue #22** (human-readable terminal output): not blocking, also useful.

We monitor these but do not block our milestone on their merge timing.

## 3. Naming and identity: Pollen

The fork needs a name distinct from "Bumblebee" because the Apache 2.0 license's trademark clause (Section 6) prohibits use of the upstream's name in our derivative. The name also signals the architectural relationship.

**Pollen.** The inventory layer Beekeeper consumes to produce policy decisions. Continues the bee metaphor (Bumblebee, Beekeeper, Sentry, Pollen) and semantically maps to the role: pollen is what bumblebees gather; Pollen is the raw inventory the beekeeper collects before processing it into policy enforcement.

The relationship to upstream is acknowledged in `README.md` and `NOTICE`: Pollen is derived from `github.com/perplexityai/bumblebee` under Apache 2.0, with Windows compatibility additions specific to the Beekeeper integration. The intent is to contribute Windows support back upstream when they are ready to accept it, at which point Pollen either retires or becomes a thin compatibility shim.

## 4. Scope

### 4.1 In scope for Milestone 2

- Fork of upstream Bumblebee at a pinned commit (v0.1.1 tag plus any post-tag main commits we explicitly choose to absorb).
- Windows root resolver: default discovery for npm, pnpm, Yarn, Bun, PyPI, Go modules, RubyGems, Composer global and user package roots on Windows.
- Windows editor extension paths: VS Code, Code Insiders, Cursor, Windsurf, VSCodium.
- Windows browser extension paths: Chrome, Chromium, Edge, Brave (Chromium family); Firefox.
- Windows MCP host config paths: Claude Desktop, Cursor MCP, Windsurf MCP, Gemini CLI.
- Path representation discipline in NDJSON output: native Windows paths preserved, no Unix-to-Windows path conversion artifacts, `endpoint.os = "windows"` correctly emitted.
- Cross-platform parity tests: identical fake-package fixtures produce equivalent inventory records on Linux, macOS, and Windows (modulo OS-specific path strings).
- Differential test against upstream Bumblebee on Linux and macOS: Pollen output is byte-for-byte identical to upstream output on those platforms, proving we have not broken anything in our fork.
- CI matrix green on all three OSes for the full Beekeeper test suite, including the Bumblebee compatibility test (now: the Pollen compatibility test).
- Upstream sync discipline: a documented, automatable process for pulling upstream changes, applying our Windows patches, and producing a new Pollen build.
- Contribution-back artifacts: our Windows implementation prepared as PRs against upstream `perplexityai/bumblebee`, ready to submit when upstream signals readiness.

### 4.2 Out of scope for Milestone 2

- Adding new ecosystems beyond what upstream Bumblebee covers. Pollen is parity-plus-Windows, not feature-extension. Cargo support, SARIF export, OSV.dev live source: all stay as upstream concerns.
- Changing upstream's matching semantics, NDJSON schema, or CLI surface. The schema_version stays `0.1.0`. Pollen is a behavioral fork, not a protocol fork.
- Making Pollen a standalone product. It exists to serve Beekeeper. The README is explicit about this and points users wanting general-purpose supply chain scanning to upstream Bumblebee.
- Maintaining Pollen indefinitely. The goal is upstream contribution. Pollen has a planned end-of-life condition: when upstream merges equivalent Windows support, Pollen retires.
- Distributing Pollen binaries publicly. The fork exists for our CI and our internal use. We do not publish releases to a third-party registry, do not advertise it as an alternative to Bumblebee. If we ever do (post-upstream-rejection scenario), it gets its own product PRD.

## 5. Architecture

### 5.1 Where Pollen lives

Pollen is a separate Go module in its own GitHub repository under the maintainer's personal account: `github.com/bantuson/pollen`. Apache 2.0 licensed. Public repo with the upstream attribution clearly stated.

It is **not** vendored into the Beekeeper repository directly. Vendoring would mix two licensed codebases in one repo and complicate attribution. A separate module with a clean import boundary is cleaner legally and architecturally.

Beekeeper consumes Pollen as a standard Go module dependency, pinned in `go.sum`. The import path in Beekeeper code is `github.com/bantuson/pollen/...` rather than `github.com/perplexityai/bumblebee/...`. This makes the dependency relationship visible and prevents accidental drift back to upstream consumption.

### 5.2 Module structure

Pollen preserves upstream's structure to keep diffs minimal and merges from upstream tractable:

```
pollen/
├── LICENSE                  # Apache 2.0, upstream-derived
├── NOTICE                   # Apache 2.0 attribution to upstream
├── README.md                # Pollen's own readme, points to upstream
├── UPSTREAM.md              # Sync discipline, see Section 6
├── VERSION                  # Pollen-specific version, e.g. 0.1.1-pollen.1
├── go.mod                   # github.com/bantuson/pollen
├── cmd/
│   └── pollen/              # was cmd/bumblebee/
│       ├── main.go
│       └── selftest.go
├── internal/
│   ├── resolver/
│   │   ├── resolver.go
│   │   ├── resolver_unix.go      # build tag: linux,darwin
│   │   └── resolver_windows.go   # build tag: windows (NEW)
│   ├── ecosystems/
│   │   ├── npm/
│   │   ├── pypi/
│   │   └── ... (one per ecosystem, with _windows.go variants where needed)
│   └── output/
│       ├── ndjson.go
│       └── paths_windows.go      # Windows path preservation (NEW)
├── docs/
│   ├── inventory-sources.md      # extended with Windows paths
│   ├── state-model.md
│   └── transport.md
├── threat_intel/                 # mirrored from upstream, see Section 6.3
└── .github/workflows/
    └── ci.yml                    # extended matrix
```

Two changes from upstream structure: the `_windows.go` build-tagged files we add, and the renamed `cmd/pollen/` directory. Everything else is preserved verbatim where possible.

### 5.3 Beekeeper's integration boundary

The Beekeeper code that consumes Pollen lives in `beekeeper/internal/inventory/`. It treats Pollen as a black box: it invokes `pollen scan ...`, reads NDJSON from stdout, parses records, applies Beekeeper-specific rules on top.

This boundary matters for two reasons:

- **Replaceability.** If upstream Bumblebee adds Windows support tomorrow and we want to switch back, we change the import and the binary invocation in one file. Beekeeper does not know or care which inventory tool runs underneath.
- **Testability.** The inventory boundary is mockable. Beekeeper tests against a Pollen interface, not a Pollen implementation. Beekeeper unit tests do not require Pollen to run; only the Pollen compatibility integration test does.

## 6. Upstream sync discipline

The hardest part of maintaining a fork is staying mergeable. Pollen has explicit rules for this so the fork does not drift into a permanent hostile state.

### 6.1 Pinning strategy

Pollen pins to upstream commits, not branches. The current pinned commit is recorded in `UPSTREAM.md` at the top of the file, in the form:

```
upstream: github.com/perplexityai/bumblebee
pinned commit: <40-char SHA>
pinned tag (if applicable): v0.1.1
pinned date: YYYY-MM-DD
verified by: <maintainer GitHub handle>
```

New upstream commits do not automatically flow in. They are reviewed, audited, and absorbed deliberately. This is part of self-defense (Section 9): an unreviewed upstream commit is an unreviewed supply chain change.

### 6.2 Sync workflow

When the maintainer decides to absorb upstream changes:

1. Run `git remote update upstream` to fetch the latest upstream main.
2. Review the diff between the current pinned commit and the new target commit. Particular attention to: new files (could be malicious), changes to NDJSON schema (breaks our consumer), changes to root resolver (conflicts with our Windows code), changes to license or NOTICE (legal compliance).
3. Run upstream's full test suite against the new commit on Linux and macOS to verify upstream did not break itself.
4. Cherry-pick or merge the new upstream changes into Pollen, resolving conflicts in favor of preserving our Windows code paths.
5. Run Pollen's full CI matrix on all three OSes.
6. Run the differential test: Pollen on Linux must produce byte-for-byte identical output to upstream Bumblebee on Linux for a fixed test fixture. Any divergence is a bug to fix before merge.
7. Update `UPSTREAM.md` with the new pinned commit and a one-line summary of what was absorbed.
8. Tag a new Pollen release (e.g. `v0.1.1-pollen.2`).

This is the same two-person-rule discipline from PRD Section 12.2, applied to upstream absorption.

### 6.3 Threat intel catalog handling

Upstream's `threat_intel/` directory contains the eight maintained exposure catalogs (Mini Shai-Hulud, AntV worm wave, Laravel Lang, node-ipc, shopsprint typosquat, GemStuffer, TrapDoor, Nx Console). These update frequently as new campaigns are reported.

Pollen has two options:

- **Mirror.** Copy upstream's `threat_intel/` directory into Pollen on each sync. Pollen users get the same catalogs upstream users get, with the sync lag from our absorption cadence.
- **Reference.** Beekeeper consumes upstream's `threat_intel/` directory directly via the `beekeeper catalogs sync` daemon, independent of Pollen. Pollen ships only with its embedded selftest fixtures.

We choose **reference.** Beekeeper already has the catalog sync daemon (PRD Section 8.5, catalog sync cadence) that pulls from upstream `threat_intel/` over HTTPS as part of its normal operation. Duplicating catalogs in Pollen creates two paths for the same data, with two possible drift conditions. The single-source path is cleaner.

This means Pollen ships with the upstream selftest fixtures and nothing else in `threat_intel/`. The directory exists for compatibility with upstream's CLI surface (`--exposure-catalog`) but is empty by default. Catalogs flow through Beekeeper.

## 7. Legal and attribution discipline

Apache 2.0 obligations, codified into Pollen's repository structure:

### 7.1 LICENSE preservation

Pollen's `LICENSE` file is the verbatim upstream Apache 2.0 LICENSE. No modifications.

### 7.2 NOTICE attribution

Pollen ships a `NOTICE` file at the repository root with the following content:

```
Pollen

This product includes software developed by Perplexity AI Inc. and
contributors to the Bumblebee project (github.com/perplexityai/bumblebee),
licensed under the Apache License 2.0.

Pollen is a derivative work that adds Windows compatibility for use within
the Beekeeper supply-chain safety harness (github.com/bantuson/beekeeper).

Pollen is not affiliated with, endorsed by, or supported by Perplexity AI Inc.
For general-purpose supply-chain scanning on macOS and Linux, use upstream
Bumblebee directly.

Changes from upstream are documented in CHANGES.md.
```

### 7.3 CHANGES.md discipline

A `CHANGES.md` file tracks every significant change from the pinned upstream commit. Format:

```
## Changes from upstream

### Added
- Windows root resolver (`internal/resolver/resolver_windows.go`)
- Windows-aware path normalization in NDJSON output
- Editor extension paths for Windows: VS Code, Cursor, Windsurf, VSCodium
- Browser extension paths for Windows: Chromium family, Firefox
- MCP host config paths for Windows installations

### Renamed
- `cmd/bumblebee/` → `cmd/pollen/`

### Modified
- Module path: `github.com/perplexityai/bumblebee` → `github.com/bantuson/pollen`
- VERSION: appended `-pollen.N` suffix
- README: rewritten to describe Pollen's purpose and relationship to upstream

### Removed
- (none in initial fork)
```

Every sync from upstream that introduces a new change appends to this file. This is the audit trail.

### 7.4 Trademark discipline

The word "Bumblebee" appears in Pollen only in attribution contexts: the NOTICE file, the README's "derived from" paragraph, the UPSTREAM.md sync log. It does not appear in command names, package names, README headlines, marketing copy, or anywhere a reasonable user might mistake Pollen for an official Bumblebee distribution.

The Apache 2.0 license does not include a trademark grant. Apache 2.0 Section 6 explicitly excludes trademark rights from the license grant. We respect this.

## 8. Windows coverage matrix

The actual work of M2: what Pollen knows about Windows that upstream Bumblebee does not.

### 8.1 Package manager roots

| Ecosystem | Unix locations (upstream) | Windows locations (Pollen) |
|---|---|---|
| npm global | `/usr/local/lib/node_modules`, `~/.npm-global` | `%APPDATA%\npm\node_modules`, `%ProgramFiles%\nodejs\node_modules` |
| npm user | `~/.npm/_cacache` | `%APPDATA%\npm-cache\_cacache`, `%LOCALAPPDATA%\npm-cache\_cacache` |
| pnpm | `~/.pnpm-store`, `~/.local/share/pnpm` | `%LOCALAPPDATA%\pnpm\store`, `%APPDATA%\pnpm` |
| Yarn | `~/.yarn/global`, `~/.config/yarn/global` | `%LOCALAPPDATA%\Yarn\Data\global` |
| Bun | `~/.bun/install/cache` | `%USERPROFILE%\.bun\install\cache` |
| PyPI (user) | `~/.local/lib/python*/site-packages` | `%APPDATA%\Python\Python*\site-packages` |
| PyPI (venv) | `<venv>/lib/python*/site-packages` | `<venv>\Lib\site-packages` |
| Go modules | `~/go/pkg/mod`, `$GOPATH/pkg/mod` | `%USERPROFILE%\go\pkg\mod` |
| RubyGems | `~/.gem/ruby/*/gems`, `/usr/local/lib/ruby/gems` | `%USERPROFILE%\.gem\ruby\*\gems`, `%ProgramFiles%\Ruby*\lib\ruby\gems` |
| Composer | `~/.composer/vendor`, `~/.config/composer/vendor` | `%APPDATA%\Composer\vendor` |

Each row drives one function in `resolver_windows.go` returning a slice of paths to enumerate.

### 8.2 Editor extension paths

| Editor | Unix path | Windows path |
|---|---|---|
| VS Code | `~/.vscode/extensions/` | `%USERPROFILE%\.vscode\extensions\` |
| VS Code Insiders | `~/.vscode-insiders/extensions/` | `%USERPROFILE%\.vscode-insiders\extensions\` |
| Cursor | `~/.cursor/extensions/` | `%USERPROFILE%\.cursor\extensions\` |
| Windsurf | `~/.windsurf/extensions/` | `%USERPROFILE%\.windsurf\extensions\` |
| VSCodium | `~/.vscode-oss/extensions/` | `%USERPROFILE%\.vscode-oss\extensions\` |

### 8.3 Browser extension paths

| Browser | Unix path | Windows path |
|---|---|---|
| Chrome | `~/.config/google-chrome/<Profile>/Extensions/` | `%LOCALAPPDATA%\Google\Chrome\User Data\<Profile>\Extensions\` |
| Chromium | `~/.config/chromium/<Profile>/Extensions/` | `%LOCALAPPDATA%\Chromium\User Data\<Profile>\Extensions\` |
| Edge | `~/.config/microsoft-edge/<Profile>/Extensions/` | `%LOCALAPPDATA%\Microsoft\Edge\User Data\<Profile>\Extensions\` |
| Brave | `~/.config/BraveSoftware/Brave-Browser/<Profile>/Extensions/` | `%LOCALAPPDATA%\BraveSoftware\Brave-Browser\User Data\<Profile>\Extensions\` |
| Firefox | `~/.mozilla/firefox/<profile>/extensions.json` | `%APPDATA%\Mozilla\Firefox\Profiles\<profile>\extensions.json` |

### 8.4 MCP host config paths

| Host | Unix path | Windows path |
|---|---|---|
| Claude Desktop | `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) | `%APPDATA%\Claude\claude_desktop_config.json` |
| Cursor MCP | `~/.cursor/mcp.json` | `%USERPROFILE%\.cursor\mcp.json` |
| Windsurf MCP | `~/.windsurf/mcp.json` | `%USERPROFILE%\.windsurf\mcp.json` |
| Cline | `~/.config/cline/cline_mcp_settings.json` | `%APPDATA%\cline\cline_mcp_settings.json` |
| Gemini CLI | `~/.gemini/settings.json` | `%USERPROFILE%\.gemini\settings.json` |
| Generic mcp.json | Project-local | Project-local (path style preserved) |

### 8.5 Path representation in NDJSON output

The `endpoint`, `project_path`, and `source_file` fields in each NDJSON record carry native Windows paths when running on Windows:

- Backslash separators preserved, not converted to forward slashes.
- Drive letters preserved (`C:\Users\fana\code\web-app`, not `/c/Users/fana/code/web-app`).
- `endpoint.os` is `"windows"`.
- `endpoint.arch` is `"amd64"` or `"arm64"` as detected by Go's `runtime.GOOS` and `runtime.GOARCH`.
- `endpoint.username` is the Windows username from `os.Getenv("USERNAME")` or equivalent.
- `endpoint.uid` is empty on Windows (no Unix-style UID concept); receivers must handle this.

Beekeeper's audit log consumer is updated to handle Windows-shaped endpoint records gracefully. The Pollen compatibility test asserts this round-trip works.

## 9. Test strategy

The whole point of this milestone is preserving test discipline. The test plan is explicit about what each layer proves.

### 9.1 Pollen's own test suite

Pollen inherits upstream Bumblebee's 161 Go test functions across 23 test files. These all pass on Linux and macOS unchanged. New Windows-specific tests live behind `//go:build windows` tags and exercise the new code paths.

The `pollen selftest` command (renamed from `bumblebee selftest`) runs upstream's embedded fixture tests plus new Windows fixtures on Windows builds. Selftest passes on all three OSes from the first release.

### 9.2 Differential testing against upstream

A dedicated CI job runs both Pollen and upstream Bumblebee against the same test fixture directory on Linux and macOS, then asserts byte-for-byte identical NDJSON output. This proves Pollen has not silently drifted from upstream behavior on the platforms upstream supports.

The differential test runs on every PR to Pollen. If upstream releases a new tag, the differential test is run against that new tag manually before absorbing.

### 9.3 Cross-platform parity testing

A test harness in `internal/testfixtures/` provides identical fake package trees: a `node_modules/` with known packages, a `~/.cargo/registry/` with known crates, fake VS Code extensions, fake MCP configs.

The parity test runs Pollen on Linux, macOS, and Windows against equivalent fixtures and asserts that:

- The same packages are detected on all three OSes.
- The same severity matches fire when given an exposure catalog.
- Record counts are equivalent (modulo OS-specific path differences in metadata).
- `endpoint.os` correctly differs per platform.

This is the test that proves Pollen's Windows implementation is functionally equivalent, not just structurally similar.

### 9.4 Pollen compatibility test (the test that unblocks PRD Section 13.4)

This is the integration test the entire milestone exists to enable.

The test invokes `pollen scan` from Beekeeper's test harness, parses the NDJSON output, asserts schema-consistency with Beekeeper's audit log schema, runs Beekeeper-specific rules on top, asserts no double-counting, asserts correct `scanner_name` attribution.

Before M2: this test was Windows-skipped. After M2: this test runs on all three OSes and is the assertion the PRD's quality bar depends on.

### 9.5 Honeypot extension

The Beekeeper Sentry honeypot test (PRD Section 13.4) gets a Windows variant. A planted process tree on Windows that reads `%USERPROFILE%\.aws\credentials` (synthetic, not real) and makes an outbound connection asserts the Sentry exfil-signature-fusion rule fires correctly on Windows.

### 9.6 CI matrix

GitHub Actions matrix for Pollen:

```yaml
strategy:
  matrix:
    os: [ubuntu-latest, macos-latest, windows-latest]
    go: ['1.25.x']
```

Per-OS jobs run: `go vet`, `go test -race ./...`, `pollen selftest`, the differential test (Linux and macOS only), and the build with `go build -ldflags "-X main.Version=..."`.

Beekeeper's CI matrix adds Pollen as a dependency: install Pollen, run the compatibility test, run the honeypot end-to-end test. This is the change that flips Beekeeper's Windows CI from "skipped Bumblebee tests" to "fully green."

## 10. Self-defense extension to the fork

PRD Section 12 lays out Beekeeper's internal threat model. Pollen extends that surface area: a new dependency we control means a new supply chain link to harden.

### 10.1 New attack surfaces introduced by Pollen

- **Pollen's release pipeline.** Same risks as Beekeeper's (Section 12.2): stolen OIDC tokens, compromised CI runners, backdoored dependencies in our additions. We have zero non-stdlib deps to keep this small, same as upstream.
- **Pollen's import path.** Beekeeper imports `github.com/bantuson/pollen`. If our GitHub account is compromised, an attacker can push a malicious Pollen release and Beekeeper's next dependency update absorbs it.
- **The upstream sync process.** Each absorption is a moment where an attacker who compromised upstream Bumblebee could push code through us to Beekeeper users.

### 10.2 Mitigations

- **Reproducible builds for Pollen** from day one, matching Beekeeper's discipline.
- **Sigstore signing via GitHub Actions OIDC** for every Pollen release.
- **Pinned upstream commits** with cryptographic verification before absorption. The sync workflow (Section 6.2) explicitly includes a diff review step that checks for new files, schema changes, and license/NOTICE modifications.
- **Two-account release approval** for Pollen releases, same as Beekeeper.
- **`pollen-self` catalog entries** in `beekeeper-self`. Beekeeper's self-detection includes known-bad Pollen versions, so a compromised Pollen release is detectable by Beekeeper itself. The recursive principle (PRD Section 12.6) extends across the dependency boundary.
- **Pinned Pollen version in Beekeeper's go.mod.** Beekeeper does not auto-update Pollen. Pollen version bumps require explicit Beekeeper PRs.
- **SBOM for Pollen** published with each release in CycloneDX format, showing exactly which upstream commit was the source and what Windows additions were made.

## 11. Sub-phasing

Milestone 2 breaks into five sub-phases. Each is a tagged Pollen release with corresponding Beekeeper integration changes.

### M2.1: Fork setup and discipline

- New GitHub repo `bantuson/pollen`, Apache 2.0 license file, NOTICE file with proper attribution.
- Initial fork at upstream v0.1.1 commit, with module path renamed and CLI binary renamed to `pollen`.
- `UPSTREAM.md` with pinned commit and sync workflow documented.
- `CHANGES.md` with initial fork notes.
- CI matrix established on three OSes (Linux and macOS pass; Windows tests skipped explicitly with structured reasons).
- Reproducible builds and Sigstore signing wired into the release workflow.
- First tagged release: `v0.1.1-pollen.1` (no Windows functionality yet; this proves the fork hygiene works).

### M2.2: Windows root resolver

- `internal/resolver/resolver_windows.go` with default root discovery for all eight package ecosystems.
- Windows-specific tests for root discovery behind `//go:build windows`.
- Parity test against Linux for the same fake package fixtures.
- Differential test continues to pass on Linux and macOS.
- Tagged release: `v0.1.1-pollen.2`.

### M2.3: Windows path representation in NDJSON output

- `internal/output/paths_windows.go` with native path preservation.
- `endpoint` record correctly emits `os: "windows"`, Windows-shaped paths in `project_path` and `source_file`, no UID, correct username.
- Tests verify round-trip through Beekeeper's audit log consumer.
- Tagged release: `v0.1.1-pollen.3`.

### M2.4: Windows extension and MCP path coverage

- Editor extension paths: VS Code, Code Insiders, Cursor, Windsurf, VSCodium.
- Browser extension paths: Chromium family and Firefox per-profile.
- MCP host config paths: Claude Desktop, Cursor MCP, Windsurf MCP, Cline, Gemini CLI.
- Tests for each path under Windows fixtures.
- Pollen compatibility test integrated into Beekeeper's CI matrix; Windows skip baseline drops to zero for these tests.
- Tagged release: `v0.1.1-pollen.4`.

### M2.5: Contribution-back and milestone close

- Prepare upstream-shaped PRs against `perplexityai/bumblebee` covering the Windows additions: root resolver, path representation, extension paths, MCP path coverage.
- Open the PRs (or comment on the existing #4 / #16 offering to test on real Windows hardware if those are still active).
- Beekeeper's full CI matrix green on all three OSes including the honeypot end-to-end test.
- Documentation pass on Pollen's README and Beekeeper's `docs/inventory.md` explaining the relationship for users and contributors.
- Tagged release: `v0.1.1-pollen.5` (the "milestone complete" tag).

## 12. Contribution-back plan

The honest position: we forked because upstream wasn't ready, not because we wanted to. Pollen retires when upstream is ready.

### 12.1 What we offer upstream

Each sub-phase produces a clean, reviewable patch series:

- M2.2's root resolver work, structured as `resolver_windows.go` with the same build tag pattern as PR #4, but with corrections and additions from our testing.
- M2.3's path representation work, structured to close upstream issue #1.
- M2.4's extension and MCP coverage, structured as additive files.

We submit these as PRs against `perplexityai/bumblebee` after the corresponding Pollen release is stable. Each PR references the equivalent Pollen tag and links to our parity tests as evidence the implementation works.

### 12.2 What we ask for in return

Nothing. The PRs are offered without conditions. If upstream merges them, Pollen retires. If upstream takes years to review them, Pollen continues serving Beekeeper. If upstream rejects them for stylistic reasons we accept and adapt; if they reject the entire premise of Windows support we keep Pollen indefinitely.

### 12.3 Pollen's planned end-of-life

If upstream merges equivalent Windows support, Pollen does one of two things:

- **Retire entirely.** Beekeeper's go.mod switches back to `github.com/perplexityai/bumblebee`. The Pollen repo becomes a deprecated archive with a README pointing to upstream. This is the preferred outcome.
- **Become a thin compatibility shim.** If upstream's Windows support diverges from our implementation in ways that would require breaking changes to Beekeeper, Pollen continues briefly as an adapter layer, with a documented sunset date.

Either way, the goal is upstream alignment, not permanent independence.

## 13. Open questions

- **Timing of contribution-back PRs.** Submit incrementally per sub-phase, or wait until M2.5 has all four PRs ready as a coordinated submission? Coordinated is cleaner but slower to surface upstream signal.
- **Should Pollen ship binaries?** Source-only via `go install` matches upstream's distribution model and avoids the macOS notarization and Windows code signing friction. But for users without a Go toolchain, source-only is a barrier. For v0.1, source-only is the answer. Revisit if external users start requesting binaries.
- **Pollen's relationship to Beekeeper's `beekeeper-self` catalog.** Should Pollen versions be tracked there too, or only in a separate `pollen-self` catalog? Operationally simpler to have one catalog; cleaner separation to have two. Leaning one for v0.1.
- **What if upstream contributes back to us?** If `perplexityai/bumblebee` ships Windows support that uses different conventions than Pollen, we adapt to match upstream rather than maintaining a divergent style. We are not the canonical implementation; upstream is.
- **Naming risk.** "Pollen" is currently unclaimed in the Go security tool space as far as I can tell. Worth a search before tagging the first release to confirm no naming collisions.

---

*End of M2 v0.1 PRD.*
