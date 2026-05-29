---
phase: 09-policy-as-code-self-defense-capstone
plan: "05"
subsystem: cmd/beekeeper + docs
tags: [cli, self-defense, policy, diag, threat-model]
dependency_graph:
  requires: [09-01-PLAN, 09-02-PLAN, 09-03-PLAN, 09-04-PLAN]
  provides:
    - "beekeeper policy validate|test|list commands"
    - "beekeeper diag command with four-section output"
    - "startup self-quarantine guard on enforcement commands"
    - "docs/THREAT-MODEL.md public threat model"
  affects:
    - cmd/beekeeper/main.go
    - internal/config (LoadLayered via resolveConfig)
    - internal/catalog (CheckSelfCatalog via checkSelfCatalogFn seam)
    - internal/policyloader (LoadPolicyFile, RunPolicyTest, ListPolicyFiles)
    - internal/check (CollectDiag, DiagReport)
tech_stack:
  added: []
  patterns:
    - "injectable checkSelfCatalogFn seam for self-quarantine testing"
    - "resolveConfig() helper for CODE-05 layered config in new commands"
    - "grouped Cobra pattern (newPolicyCmd) matching newCatalogsCmd idiom"
key_files:
  created:
    - cmd/beekeeper/policy.go
    - cmd/beekeeper/policy_test.go
    - cmd/beekeeper/diag.go
    - cmd/beekeeper/diag_test.go
    - cmd/beekeeper/selfquarantine.go
    - cmd/beekeeper/selfquarantine_test.go
    - docs/THREAT-MODEL.md
  modified:
    - cmd/beekeeper/main.go
decisions:
  - "Self-quarantine guard wired at TOP of enforcement RunE and END of catalogs sync RunE"
  - "Diagnostic commands (version, diag, selftest, policy validate) explicitly NOT guarded"
  - "checkSelfCatalogFn package-var injection seam used for testability without interface overhead"
  - "resolveConfig() helper uses config.LoadLayered for CODE-05 compliance in new commands"
  - "docs/THREAT-MODEL.md published as docs/ subdirectory per standard project layout"
metrics:
  duration: "~45 minutes"
  completed_date: "2026-05-29"
  tasks_completed: 3
  tasks_total: 3
  files_modified: 8
---

# Phase 9 Plan 05: CLI Capstone + Self-Defense Wiring Summary

**One-liner:** CLI capstone wiring policy validate/test/list and diag over Plan 01-04 packages, plus startup self-quarantine guard on enforcement commands and published threat model covering coordinated false-positive poisoning and fanotify mmap gap.

## What Was Built

### Task 1: policy + diag commands + policies/ init + layered config wiring (d28d489)

**cmd/beekeeper/policy.go** — `newPolicyCmd()` with three subcommands:
- `policy validate <file>`: calls `policyloader.LoadPolicyFile`, prints all errors to stderr (non-zero exit on any error), prints "OK" on success.
- `policy test <file> [--tool-call <path|->]`: reads tool-call JSON from file or stdin, calls `policyloader.RunPolicyTest` with empty catalog (deterministic dry-run), prints decision + reason.
- `policy list`: resolves `~/.beekeeper/policies/`, calls `policyloader.ListPolicyFiles`, prints rule counts per file; friendly "no policy files" message on empty directory.

**cmd/beekeeper/diag.go** — `newDiagCmd()` standalone command calling `check.CollectDiag(stateFile, hookLatencyRingPath)` and formatting four sections: Hook Handler (p95/p99), LlamaFirewall Sidecar (p95), Catalog Sources (hash/count/degraded per source), ETW Event Loss. Also contains `resolveConfig()` helper that uses `config.LoadLayered` for CODE-05 compliance.

**cmd/beekeeper/main.go** modifications:
- Added `newPolicyCmd()` and `newDiagCmd()` to `root.AddCommand` block.
- Extended `newInitCmd()` MkdirAll loop to include `filepath.Join(stateDir, "policies")` so `policy list` works on a fresh install.

Tests: `policy_test.go` (TestPolicyValidateCmd_Valid, TestPolicyValidateCmd_Invalid, TestPolicyTest_Cmd, TestPolicyList_Empty) and `diag_test.go` (TestDiagCmd_Output verifying all four section headings).

### Task 2: startup self-quarantine guard wired to enforcement commands (b458975)

**cmd/beekeeper/selfquarantine.go** — `enforceSelfQuarantine(cmd *cobra.Command) error`:
- Resolves state dir, catalog dir, and config via `resolveConfig`.
- Builds `catalog.SelfCatalogOpts` with running version, short HTTP client timeout.
- Dispatches on `SelfCatalogResult.Outcome`:
  - `Continue` → nil (silent).
  - `WarnContinue` → WARNING to stderr, nil (don't brick on network failure).
  - `Quarantine` → writes `self_quarantine` audit record + prominent stderr warning with `verify-release`/`cosign`/SLSA verification path, returns non-nil error.
  - `FailClosed` → same as quarantine but with integrity-failure message.
- Package var `checkSelfCatalogFn = catalog.CheckSelfCatalog` is the injectable seam for tests.

**cmd/beekeeper/main.go** modifications:
- Guard called at TOP of `check`, `gateway`, `watch`, `sentry` RunE bodies (4 enforcement commands).
- Guard called at END of `catalogs sync` RunE (CTLG-04: every sync triggers check).
- `version`, `diag`, `selftest`, `policy validate` NOT guarded (T-09-21 / Open Question 3).

Tests: `selfquarantine_test.go` covering quarantine/continue/warn-continue/fail-closed outcomes and asserting diagnostic commands do not invoke the guard.

### Task 3: docs/THREAT-MODEL.md (2cf479b)

Eight required sections published:

1. **Beekeeper's own threat model** — what compromising Beekeeper gives an attacker, attack surface table ordered by risk.
2. **Build and release pipeline hardening** — reproducible builds, cosign verify, SLSA Level 3, pinned deps, CycloneDX SBOM.
3. **Catalog feed integrity: the 2FA principle** — corroboration semantics, signature verification, sanity bounds, degraded mode.
4. **Coordinated false-positive poisoning attack surface** — explicitly documents that an adversary controlling ≥2 sources can manufacture false-positive blocks; no complete fix without human-in-the-loop review; known gap acknowledged.
5. **The fanotify mmap gap on Linux** — files mmap-loaded before fanotify is active evade Sentry file-access detection; documented as known gap with partial mitigations.
6. **The beekeeper-self catalog** — how it works, single-maintainer governance honesty note for v1.0.0 with intent to separate, verification path during self-quarantine events.
7. **Verification path** — step-by-step: `make verify-release`, `cosign verify`, `slsa-verifier`, SBOM inspection; links SECURITY.md.
8. **Known gaps and explicit non-defenses** — kernel rootkits, pre-existing malware, direct human malice, sophisticated prompt injection, catalog source unavailability.

## Verification Results

- `go build ./cmd/...`: PASS
- `GOOS=windows go build ./cmd/...`: PASS
- `go test ./cmd/... -count=1`: PASS (all task tests green)
- `go test ./... -count=1`: PASS (full suite, 22 packages)
- `go vet ./cmd/...`: PASS (clean)
- `docs/THREAT-MODEL.md` exists and covers fanotify, false-positive poisoning, verification path, governance note: PASS

## Deviations from Plan

**1. [Rule 2 - Missing critical functionality] selfCatalogEntry is unexported**

Found during Task 2: `selfCatalogEntry` is an unexported type in `internal/catalog`. The test stub for `enforceSelfQuarantine` could not set `MatchedEntry` to a concrete `*selfCatalogEntry`. Fixed by: passing `nil` for `MatchedEntry` in the test stub (the production code handles nil MatchedEntry with fallback "unknown" values). The actual quarantine behavior is tested by the catalog package's own tests (09-03 plan).

**2. [Rule 1 - Bug] Missing CatalogMatches/SourcesAgreed/SourcesDissented fields in audit record**

Found during Task 2: `audit.AuditRecord` validation requires non-nil slices for `CatalogMatches`, `SourcesAgreed`, `SourcesDissented`. Fixed inline in `writeQuarantineAuditRecord` by initializing them as empty slices (`[]audit.CatalogProvenance{}`, `[]string{}`, `[]string{}`).

## Known Stubs

None. All commands wire to real Plan 01-04 implementations. No hardcoded empty values flow to output.

## Threat Flags

No new network endpoints, auth paths, or schema changes at trust boundaries introduced in this plan beyond what is already documented in the plan's threat model (T-09-18 through T-09-23). The `enforceSelfQuarantine` guard adds a new outbound HTTP call to `selfCatalogDefaultFeedURL` on enforcement command startup — this is the intended behavior (CTLG-04/SFDF-06) and is documented in the threat model.

## Self-Check: PASSED

- `cmd/beekeeper/policy.go`: FOUND
- `cmd/beekeeper/diag.go`: FOUND
- `cmd/beekeeper/selfquarantine.go`: FOUND
- `docs/THREAT-MODEL.md`: FOUND
- Commit d28d489 (Task 1): FOUND
- Commit b458975 (Task 2): FOUND
- Commit 2cf479b (Task 3): FOUND
