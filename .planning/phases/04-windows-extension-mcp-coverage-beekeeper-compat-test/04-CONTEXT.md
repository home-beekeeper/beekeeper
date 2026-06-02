# Phase 4: Windows Extension & MCP Coverage + Beekeeper Compat Test - Context

**Gathered:** 2026-06-02
**Status:** Ready for planning
**Source:** PRD Express Path (beekeeper-m2-prd.md)

<domain>
## Phase Boundary

This phase completes Pollen's Windows surface coverage and connects it to Beekeeper.

**Two repos in play** (same pattern as Phase 3's "both repos" plan):
- **`bantuson/pollen`** (sibling clone at `../pollen`) — Windows editor-extension, browser-extension, and MCP host-config path enumeration (WEXT-01/02/03).
- **`beekeeper`** (this repo) — the `pollen scan` consumption boundary and the cross-OS compatibility test (BKINT-01, PTEST-04).

**What this phase delivers (the TRUE conditions from ROADMAP SC1–SC4):**
1. On Windows CI, `pollen scan` detects fake VS Code, Code Insiders, Cursor, Windsurf, and VSCodium extensions planted under `%USERPROFILE%` fixture trees — all five editor paths return inventory records (WEXT-01).
2. On Windows CI, `pollen scan` detects fake Chrome/Chromium/Edge/Brave (per-profile) and Firefox (per-profile `extensions.json`) browser extensions — all Chromium-family and Firefox paths return records (WEXT-02).
3. On Windows CI, `pollen scan` finds fake MCP config files at `%APPDATA%\Claude\`, `%USERPROFILE%\.cursor\mcp.json`, `%USERPROFILE%\.windsurf\mcp.json`, `%APPDATA%\cline\`, and `%USERPROFILE%\.gemini\settings.json` (WEXT-03).
4. Beekeeper's Pollen compatibility test (invoking `pollen scan`, asserting NDJSON schema-consistency, running beekeeper rules, asserting `scanner_name`) runs green on ubuntu/macos/windows with **zero `t.Skip` calls** (BKINT-01 + PTEST-04).

**Out of scope this phase** (deferred to Phase 5): upstream PRs (SYNC-01/02), Windows honeypot E2E (PTEST-05), `pollen-self` catalog (SDEF-01), `go.mod` version pin + CI flip narrative (BKINT-02), and the signed milestone release tag.

</domain>

<decisions>
## Implementation Decisions

Everything in the PRD is treated as a locked decision. The PRD coverage tables (§8.2–8.4) are the authoritative path specs.

### WEXT-01 — Windows editor-extension paths (PRD §8.2)
Pollen must enumerate these five Windows editor extension directories:
- VS Code → `%USERPROFILE%\.vscode\extensions\`
- VS Code Insiders → `%USERPROFILE%\.vscode-insiders\extensions\`
- Cursor → `%USERPROFILE%\.cursor\extensions\`
- Windsurf → `%USERPROFILE%\.windsurf\extensions\`
- VSCodium → `%USERPROFILE%\.vscode-oss\extensions\`

(Note: `%USERPROFILE%\.cursor\extensions\` was carried as Phase-3 deferred Assumption A1 needing live validation — this phase is where it gets exercised against fixtures.)

### WEXT-02 — Windows browser-extension paths (PRD §8.3), per profile
- Chrome → `%LOCALAPPDATA%\Google\Chrome\User Data\<Profile>\Extensions\`
- Chromium → `%LOCALAPPDATA%\Chromium\User Data\<Profile>\Extensions\`
- Edge → `%LOCALAPPDATA%\Microsoft\Edge\User Data\<Profile>\Extensions\`
- Brave → `%LOCALAPPDATA%\BraveSoftware\Brave-Browser\User Data\<Profile>\Extensions\`
- Firefox → `%APPDATA%\Mozilla\Firefox\Profiles\<profile>\extensions.json`

Chromium-family is per-profile directory enumeration; Firefox is a per-profile `extensions.json` file. Profile discovery must iterate profiles (e.g., `Default`, `Profile 1`, …).

### WEXT-03 — Windows MCP host-config paths (PRD §8.4)
- Claude Desktop → `%APPDATA%\Claude\claude_desktop_config.json`
- Cursor MCP → `%USERPROFILE%\.cursor\mcp.json`
- Windsurf MCP → `%USERPROFILE%\.windsurf\mcp.json`
- Cline → `%APPDATA%\cline\cline_mcp_settings.json`
- Gemini CLI → `%USERPROFILE%\.gemini\settings.json`
- Generic `mcp.json` / `.mcp.json` → project-local, path style preserved (no rewriting)

### BKINT-01 — Beekeeper consumes Pollen across a mockable boundary (PRD §5.3)
- Beekeeper invokes `pollen scan`, reads NDJSON from stdout, parses records, applies beekeeper-specific rules on top — Pollen is a black box behind a clean import boundary.
- The current `bumblebee` subprocess call in beekeeper's `internal/scan` (`runBumblebeeFn` injectable var) is swapped to invoke `pollen` **behind a mockable interface** so beekeeper unit tests don't require Pollen to run; only the compatibility integration test does.
- Boundary requirements: **replaceability** (switching back to upstream is a one-file import + binary-name change) and **testability** (beekeeper tests against an interface, not the Pollen implementation).
- ROADMAP names `internal/inventory/` as the BKINT-01 boundary package; Phase-3 RESEARCH deliberately kept the round-trip test in `internal/scan/scanner_test.go` and flagged `internal/inventory/` as "Phase 4's BKINT-01 boundary". RESEARCH must confirm whether BKINT-01 is a new `internal/inventory/` package or an evolution of the existing `internal/scan` `runBumblebeeFn` seam — and reconcile with the existing `runBumblebeeFn`/`scanOnDeltaFn` injectable-var pattern already in the repo.

### PTEST-04 — Pollen compatibility test, zero Windows skips (PRD §9.4)
The integration test the whole milestone exists to enable:
- Invokes `pollen scan` from beekeeper's test harness, parses NDJSON output.
- Asserts schema-consistency with beekeeper's audit-log schema.
- Runs beekeeper-specific rules on top; asserts **no double-counting** of findings.
- Asserts correct `scanner_name` attribution.
- Runs on ubuntu/macos/windows; the Windows skip baseline for these inventory tests is **zero** (no `t.Skip`).

### Scope guardrails (PRD §4.2 — locked)
- **No new ecosystems** beyond upstream Bumblebee. Pollen is parity-plus-Windows, not feature-extension.
- **No schema/CLI/matching-semantics changes.** `schema_version` stays `0.1.0`. Pollen is a behavioral fork, not a protocol fork.
- **Native Windows path representation preserved** (Phase 3 already landed `filepath.FromSlash` discipline + Windows `endpoint` record) — Phase 4 extension/MCP records must follow the same path discipline.

### Release-tag handling (carry-forward from Phase 2/3 precedent — D-06)
- ROADMAP SC5 lists "`v0.1.1-pollen.4` is tagged and signed." Per the established Phase 2/3 maintainer decision (D-06), **signed `pollen.N` tags are batched to M2 close.** This phase PREPARES the release locally (VERSION bump to `0.1.1-pollen.4` + `CHANGES.md` entry in `../pollen`, mirroring plans 02-04 and 03-03) but the signed git tag + push is DEFERRED to milestone close. Do NOT flag SC5 as failed for the missing tag — code-complete + prepared release satisfies the phase; the tag is a tracked deferral.

### Claude's Discretion
- The exact live-repo file locus for editor/browser/MCP enumeration in Pollen (PRD §5.2 sketches `internal/resolver/`, `internal/ecosystems/`, `internal/output/`, but the **live fork diverged** in Phases 2/3 — see Canonical References). RESEARCH must locate where Bumblebee's extension/browser/MCP scanning actually lives in `../pollen` and place the Windows additions there, using build-tagged `_windows.go` files consistent with the Phase 2 `roots_windows.go` pattern.
- Build-tag layout (`//go:build windows`), test fixture layout under `internal/testfixtures/` (or the live equivalent), profile-iteration helper structure, and whether per-OS path tables are functions or table literals.
- Whether the beekeeper boundary is a brand-new `internal/inventory/` package or an in-place evolution of `internal/scan` — pick whichever yields the cleaner mockable interface and the smallest diff; justify in the plan.
- Wave structure and how Pollen-repo work (cross-repo, in `../pollen`) is sequenced against beekeeper-repo work.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### PRD — authoritative spec for this phase
- `beekeeper-m2-prd.md` §8.2 — editor-extension path table (WEXT-01)
- `beekeeper-m2-prd.md` §8.3 — browser-extension path table (WEXT-02)
- `beekeeper-m2-prd.md` §8.4 — MCP host-config path table (WEXT-03)
- `beekeeper-m2-prd.md` §5.3 — Beekeeper's integration boundary / black-box `pollen scan` (BKINT-01)
- `beekeeper-m2-prd.md` §9.4 — Pollen compatibility test (PTEST-04)
- `beekeeper-m2-prd.md` §8.5 — NDJSON path representation discipline (must hold for new records)
- `beekeeper-m2-prd.md` §4.2 — out-of-scope guardrails (no new ecosystems, no schema change)

### Requirements
- `.planning/REQUIREMENTS.md` — WEXT-01/02/03, BKINT-01, PTEST-04 definitions and the live-locus correction note in WPATH-01 (the PRD §5.2 `internal/output/` path does not exist in the live fork)

### Prior-phase artifacts — LOCUS CORRECTIONS ARE BINDING
- `.planning/phases/03-windows-path-representation/03-RESEARCH.md` — confirms live fork uses `internal/ecosystem/<name>/` scanners (NOT `internal/output/`), and explicitly flags `internal/inventory/` as "Phase 4's BKINT-01 boundary"
- `.planning/phases/03-windows-path-representation/03-03-SUMMARY.md` — beekeeper `internal/scan` round-trip test pattern; VERSION/CHANGES bump-without-tag pattern (the release-deferral precedent)
- `.planning/phases/02-windows-root-resolver/02-01-PLAN.md` + `02-03-PLAN.md` — the `cmd/pollen/roots_windows.go` build-tag pattern and `parity_test.go` + `testdata/parity-fixture/` fixture pattern to mirror for extensions/MCP
- `.planning/phases/02-windows-root-resolver/02-04-SUMMARY.md` — exact release-cut commands deferred to M2 close

### Live code (read before editing)
- `../pollen` — the Pollen fork clone (extension/browser/MCP scanners live here; locate via RESEARCH)
- beekeeper `internal/scan/` — current `runBumblebeeFn` / `scanOnDeltaFn` injectable-var seam that BKINT-01 evolves
- beekeeper `internal/scan/scanner_test.go` — where the Phase-3 Windows round-trip test already lives

</canonical_refs>

<specifics>
## Specific Ideas

- The five editor paths, five browser families (4 Chromium + Firefox), and five MCP hosts in PRD §8.2–8.4 are exact strings — copy them verbatim into the resolver tables; do not paraphrase.
- Firefox differs from Chromium: it is a single `extensions.json` file per profile, not an `Extensions/` directory of unpacked extensions.
- Chromium-family browsers require **per-profile** iteration (`Default`, `Profile 1`, `Profile 2`, …) under `User Data\`.
- Fixtures should mirror the Phase 2 `testdata/parity-fixture/` approach: planted fake extensions/configs under a fake `%USERPROFILE%`/`%APPDATA%`/`%LOCALAPPDATA%` tree, with Windows env vars set via `t.Setenv` (never `HOME`) — the Phase 2 Windows test-isolation discipline (Pitfall 5 prevention).
- PTEST-04 must drive the beekeeper Windows skip baseline for inventory tests to **zero** — the success metric is an absence of `t.Skip`, not just passing tests. Any remaining skip is a failure of the phase goal.
- BKINT-01 should preserve the existing injectable-var test pattern (`runBumblebeeFn` → a `runPollenFn`/interface) so beekeeper unit tests stay network/binary-free.

</specifics>

<deferred>
## Deferred Ideas

- `v0.1.1-pollen.4` **signed** git tag + push — DEFERRED to M2 close (D-06 / Phase 2–3 precedent). This phase prepares VERSION + CHANGES.md locally only.
- SYNC-01 (UPSTREAM.md sync workflow) and SYNC-02 (upstream PRs) — Phase 5.
- PTEST-05 (Windows Sentry honeypot E2E) — Phase 5.
- BKINT-02 (beekeeper `go.mod` Pollen version pin + full CI green narrative) — Phase 5. (Phase 4 wires the boundary + compat test; the explicit version pin and milestone CI flip land in Phase 5.)
- SDEF-01 (`pollen-self` catalog entries) — Phase 5.
- Open question (PRD §13) — contribution-back PR timing (incremental vs coordinated at M2.5): a Phase 5 decision, not Phase 4.

</deferred>

---

*Phase: 04-windows-extension-mcp-coverage-beekeeper-compat-test*
*Context gathered: 2026-06-02 via PRD Express Path*
