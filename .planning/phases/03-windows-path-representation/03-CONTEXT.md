# Phase 3: Windows Path Representation - Context

**Gathered:** 2026-06-02
**Status:** Ready for planning
**Source:** PRD Express Path (beekeeper-m2-prd.md §8.5, §11 M2.3)

<domain>
## Phase Boundary

Every NDJSON record emitted by **Pollen** (`github.com/bantuson/pollen`, working tree at
`../pollen`) on Windows must carry **native Windows path representation** and a correct
**Windows endpoint record**, and **beekeeper** (this repo) must parse and round-trip those
Windows-shaped records without error.

Two-repo phase:

1. **Pollen** (`../pollen`) — emit native Windows paths + Windows endpoint fields.
2. **Beekeeper** (this repo) — consumer-side round-trip verification of Windows-shaped records.

In scope (M2.3 / WPATH-01, WPATH-02):
- Native Windows path preservation in NDJSON `project_path` / `source_file` (backslashes +
  drive letters retained; no Unix-to-Windows conversion artifacts).
- Windows endpoint record: `os="windows"`, `arch` from `runtime.GOARCH`, non-empty `username`,
  **empty `uid`** (Windows has no Unix UID concept).
- Beekeeper consumer handles Windows-shaped endpoint records on round-trip; a test asserts it.

Explicitly OUT of scope (deferred / other phases / out-of-milestone):
- NDJSON **schema changes** — `schema_version` stays `0.1.0`. This is a behavioral fork, not a
  protocol fork (PRD §4.2). No new fields, no field renames.
- Editor / browser / MCP extension path coverage → **Phase 4** (WEXT-01..03).
- Swapping beekeeper's `internal/scan` subprocess from `bumblebee` to `pollen` (BKINT-01) →
  **Phase 4**. Phase 3's beekeeper-side work is consumer round-trip *verification only*, not the
  production subprocess swap.
- New ecosystems, SARIF, OSV live source (PRD §4.2 out of scope).

</domain>

<decisions>
## Implementation Decisions

Everything in the PRD and roadmap success criteria is treated as a **locked decision**.

### D-01 — Native Windows path preservation (WPATH-01)
- On Windows, NDJSON `project_path` and `source_file` fields MUST contain backslash separators
  and drive letters (`C:\Users\fana\code\web-app`), NOT forward-slash / `/c/...` forms.
- No Unix-to-Windows path conversion artifacts anywhere in the output path.
- Implementation locus (PRD §5.2): `internal/output/paths_windows.go` in Pollen. **NOTE:** the
  live fork structure differs from the PRD's idealized layout — `internal/output/` exists
  (`output.go`), but the actual normalization path must be confirmed against the live code (see
  Canonical References + Claude's Discretion).

### D-02 — Windows endpoint record (WPATH-02)
- `endpoint.os` == `"windows"` (already `runtime.GOOS`).
- `endpoint.arch` matches `runtime.GOARCH` (`amd64` / `arm64`) (already `runtime.GOARCH`).
- `endpoint.username` non-empty from the Windows environment (already `user.Current().Username`).
- `endpoint.uid` is **EMPTY on Windows**. This is the net-new requirement: the current
  `endpoint.Current()` (`../pollen/internal/endpoint/endpoint.go`) sets `UID` from
  `user.Current().Uid`, which on Windows is a **SID string** (`S-1-5-21-…`), not empty. Windows
  must produce an empty `uid`.
- Linux/macOS endpoint records MUST be unchanged (UID still populated on Unix).

### D-03 — Beekeeper consumer round-trip (WPATH-02, beekeeper side)
- Beekeeper's audit-log / inventory consumer parses a Windows-shaped Pollen NDJSON record
  (backslash paths, empty `uid`, `os="windows"`) **without error** and round-trips the endpoint
  fields correctly.
- Roadmap names `internal/inventory/` as the test locus. **NOTE:** `internal/inventory/` does
  NOT yet exist in beekeeper; the current Pollen/Bumblebee consumer is `internal/scan/scanner.go`.
  Whether to create `internal/inventory/` fresh or co-locate the round-trip test with the existing
  consumer is left to research/planning (see Claude's Discretion).

### D-04 — No regression on Unix (differential test stays green)
- Windows path/endpoint additions MUST NOT drift Pollen's behavior on Linux/macOS. The Phase-1
  differential test (Pollen byte-for-byte identical to upstream Bumblebee on Linux/macOS) MUST
  continue to pass. Windows-specific behavior goes behind `//go:build windows` or a
  `runtime.GOOS == "windows"` guard so Unix output is untouched.

### D-05 — Cross-platform parity test extends to path/endpoint assertions
- The existing parity test (PTEST-01, `../pollen` parity_test.go + testfixtures) should gain (or
  already partially covers) assertions that `endpoint.os` differs correctly per platform and that
  Windows path shapes are asserted. Reuse the Phase-2 parity harness rather than building new.

### D-06 — Release tagging deferred to M2 close (maintainer pattern)
- Roadmap SC4 is "`v0.1.1-pollen.3` is tagged and signed." Following the **established Phase-2
  maintainer decision** (STATE.md Deferred Items; Phase 2's `v0.1.1-pollen.2` tag is prepared
  locally but deferred to M2 close), the signed/tagged release for Phase 3 is also **batched to
  M2 close**. Phase 3 is "done" when code is complete, tests pass, and Windows skips (if any) are
  flipped — the VERSION/CHANGES bump may be prepared locally and committed, but `git tag` +
  Sigstore signing are deferred. Plans should treat SC4 as "prepare release locally; defer signed
  tag" unless the maintainer states otherwise.

### Claude's Discretion
- Exact mechanism for Windows path preservation (`paths_windows.go` build-tagged file vs. a
  `runtime.GOOS` branch in `output.go`) — pick whichever produces the smallest diff and keeps the
  Unix differential test byte-identical. Mirror the Phase-2 "Option B, in-place minimal-diff"
  precedent (`cmd/pollen/roots_windows.go`) rather than the PRD's idealized `internal/resolver/`
  module split.
- Exact mechanism for empty Windows `uid` (`endpoint_windows.go` build-tagged override vs.
  `if runtime.GOOS == "windows"` guard in `endpoint.go`). Keep Unix behavior identical.
- Whether beekeeper's round-trip test creates a new `internal/inventory/` package or lives beside
  `internal/scan/`. The roadmap prefers `internal/inventory/`; confirm against the BKINT-01
  boundary work scheduled for Phase 4 so Phase 3 doesn't prematurely build the subprocess swap.
- Whether to add a Windows fixture / synthetic NDJSON sample for the beekeeper round-trip test, or
  hand-craft a Windows-shaped record inline in the test.
- Test naming and file layout, subject to project conventions.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Source PRD & milestone artifacts
- `beekeeper-m2-prd.md` — §8.5 (path representation in NDJSON), §11 M2.3 (sub-phase scope),
  §5.2 (module structure — NOTE: idealized, see live-structure caveats below), §4.2 (out of scope)
- `.planning/REQUIREMENTS.md` — WPATH-01, WPATH-02 (lines 26–27); traceability table
- `.planning/ROADMAP.md` — Phase 3 section (goal, repo locus, 4 success criteria)
- `.planning/STATE.md` — Deferred Items (Phase 2 release-deferral precedent → D-06); accumulated
  Phase 2 decisions (build-tag stubs, `roots_notwindows.go` pattern, Windows test isolation)

### Pollen — emitter side (`../pollen`, repo `github.com/bantuson/pollen`, HEAD c94b271)
- `internal/output/output.go` — NDJSON emitter; **WPATH-01 target** (path field handling)
- `internal/output/output_test.go` — existing output test patterns to extend
- `internal/endpoint/endpoint.go` — `Current(deviceID)` builds the endpoint record;
  **WPATH-02 target** (the `UID` line is the one that must become empty on Windows)
- `internal/endpoint/endpoint_test.go` — existing endpoint test patterns to extend
- `internal/model/` — `model.Endpoint` struct definition (field shapes; do NOT change schema)
- `cmd/pollen/roots_windows.go` + `roots.go` (Phase 2) — the **precedent pattern** for Windows
  build-tagged, minimal-diff additions; mirror its style
- `cmd/pollen/parity_test.go` + `testdata/parity-fixture/` (Phase 2, PTEST-01) — parity harness to
  extend for D-05
- `CHANGES.md`, `VERSION`, `UPSTREAM.md` — fork-discipline files; CHANGES.md must record WPATH deltas

### Beekeeper — consumer side (this repo)
- `internal/scan/scanner.go` — current Pollen/Bumblebee NDJSON consumer (`runBumblebeeFn`);
  the existing parse path the round-trip test must be consistent with
- `internal/scan/scanner_test.go` — consumer test patterns
- `internal/audit/` — audit-log schema the consumer round-trips into (schema-compatibility target)
- `CLAUDE.md` — project constraints (fail-closed, `internal/` business logic, Windows-primary dev)

### Live-structure caveats (verified 2026-06-02 — do not trust the PRD's idealized §5.2 layout)
- Pollen `internal/` actual dirs: `ecosystem/ endpoint/ exposure/ model/ normalize/ output/
  scanner/ walk/` — the PRD's `internal/resolver/` and `internal/ecosystems/` do **not** exist.
- Beekeeper has **no** `internal/inventory/` directory yet (roadmap names it as the test locus).

</canonical_refs>

<specifics>
## Specific Ideas

- WPATH-01 concrete assertion (roadmap SC1): a Windows CI `pollen scan` produces NDJSON where
  `project_path` and `source_file` contain backslashes and drive letters with zero `/c/`-style
  artifacts.
- WPATH-02 concrete assertion (roadmap SC2): every Windows NDJSON `endpoint` has `os="windows"`,
  `arch == runtime.GOARCH`, non-empty `username`, empty `uid`; Unix records unchanged.
- Round-trip concrete assertion (roadmap SC3): a beekeeper test parses a Windows-shaped record
  without error and preserves the endpoint fields.
- PRD §8.5 spells out the exact endpoint field expectations — copy them verbatim into acceptance
  criteria.
- Mirror Phase 2's verified patterns: per-OS build tags with `!windows` stub files where Go
  compiles all switch-case bodies; `t.Setenv(USERPROFILE/...)` for Windows test isolation (never
  `HOME`); structured `t.Skip` reasons if any test must stay Windows-skipped pending CI.

</specifics>

<deferred>
## Deferred Ideas

- Signed/tagged `v0.1.1-pollen.3` release → **batched to M2 close** (D-06), matching the Phase-2
  maintainer decision in STATE.md Deferred Items.
- BKINT-01 (swap beekeeper `internal/scan` subprocess `bumblebee`→`pollen` behind a mockable
  interface) → **Phase 4**.
- Editor/browser/MCP Windows path coverage (WEXT-01..03) → **Phase 4**.
- Windows honeypot E2E + full beekeeper Windows CI green (PTEST-05, BKINT-02) → **Phase 5**.

</deferred>

---

*Phase: 03-windows-path-representation*
*Context gathered: 2026-06-02 via PRD Express Path*
