---
phase: 02-policy-engine-multi-source-catalogs
plan: "02"
subsystem: internal/policy
tags: [policy, sensitive-path, credential-redaction, pure-function, PLCY-04, PLCY-08]
dependency_graph:
  requires:
    - "01-04: internal/policy (Decision type, CatalogLookup, purity model)"
  provides:
    - "EvaluatePath: pure sensitive-path blocking with allowlist override"
    - "FilterCredentials: pure output credential redaction with six built-in classes"
  affects:
    - "02-08: wiring layer will append MCP config paths to BlockPatterns"
    - "hook handler (check/): applies EvaluatePath and FilterCredentials in-process"
    - "MCP gateway (Phase 4): same pure functions, same purity contract"
tech_stack:
  added: []
  patterns:
    - "package-level compiled regexp vars (compiled once, not per-call)"
    - "basename-glob matching via LastIndexAny + HasPrefix"
    - "allowlist-first evaluation (override before blocklist)"
    - "TDD: RED test commit → GREEN implementation commit"
key_files:
  created:
    - internal/policy/path.go
    - internal/policy/path_test.go
    - internal/policy/credentials.go
    - internal/policy/credentials_test.go
  modified: []
decisions:
  - "EvaluatePath blocklist uses fragment patterns (strings.Contains) for directory paths and basename-glob for .env family; caller normalizes ~ and OS separators before calling"
  - "matchesBlockPattern normalizes backslashes to forward slashes for Windows path compatibility with forward-slash fragment patterns"
  - "FilterCredentials compiles AdditionalPatterns per-call (acceptable: small caller-controlled set); built-in regexps remain package-level"
  - "credentials.go imports only 'regexp' (not 'strings') — no forbidden I/O packages"
  - "DetectedTypes is nil (not empty slice) when no credentials found — matches zero-value contract"
metrics:
  duration: "~15 min"
  completed: "2026-05-26"
  tasks_completed: 2
  files_created: 4
---

# Phase 02 Plan 02: Sensitive Path Policy + Credential Redaction Summary

Pure sensitive-path blocking via `EvaluatePath` (PLCY-04) and output credential redaction via `FilterCredentials` (PLCY-08), both in `internal/policy`, both pure functions with no forbidden imports.

## What Was Built

### Task 1: EvaluatePath (PLCY-04) — `internal/policy/path.go`

**Signature:**
```go
func EvaluatePath(resolvedPath string, cfg SensitivePathConfig) Decision
func DefaultSensitivePaths() SensitivePathConfig
```

**Default blocklist (PLCY-04):**
| Pattern | Match type | Covers |
|---------|-----------|--------|
| `/.ssh/` | fragment | SSH keys and config |
| `/.aws/` | fragment | AWS credentials |
| `/.gnupg/` | fragment | GPG keys |
| `/.config/Claude/` | fragment | Claude MCP settings |
| `/.config/op/` | fragment | 1Password CLI |
| `/.config/gh/` | fragment | GitHub CLI |
| `/.netrc` | fragment | netrc credential store |
| `/.npmrc` | fragment | npm credentials |
| `/.pypirc` | fragment | PyPI credentials |
| `/.cargo/credentials.toml` | fragment | Cargo registry token |
| `.env` | basename exact | dotenv files |
| `.env.local` | basename exact | local dotenv files |
| `.env.*` | basename glob | all dotenv variants (.env.production, etc.) |

**Caller contract:** `resolvedPath` must have `~` already substituted and OS separators normalized. `EvaluatePath` is a pure function — it does not call `os.UserHomeDir`, `filepath.Abs`, or any I/O.

**MCP config paths:** Per the plan, all MCP config files enumerated by Bumblebee are appended to `BlockPatterns` by the wiring layer at Plan 08 time. They are not hardcoded here.

**Allowlist override:** `AllowPatterns` entries (prefix or exact) are checked first; a match returns allow with reason "explicitly allowlisted" before the blocklist is evaluated.

**Windows support:** `matchesBlockPattern` normalizes backslashes to forward slashes when comparing against forward-slash fragment patterns. The last-segment extractor splits on both `/` and `\` for basename-glob patterns.

### Task 2: FilterCredentials (PLCY-08) — `internal/policy/credentials.go`

**Signature:**
```go
func FilterCredentials(output string, cfg CredentialFilterConfig) CredentialFilterResult
```

**Built-in redaction patterns (package-level compiled regexps):**
| Name | Pattern | Example match |
|------|---------|---------------|
| `aws-access-key` | `AKIA[0-9A-Z]{16}` | `AKIAIOSFODNN7EXAMPLE` |
| `jwt` | `eyJ[A-Za-z0-9_-]{2,}\.[...]\.[...]` | Three-segment base64url JWT |
| `bearer` | `(?i)Bearer\s+[A-Za-z0-9._-]+` | `Bearer abc123def456...` |
| `github-token` | `gh[pousr]_[A-Za-z0-9]{36,}` | ghp_, gho_, ghu_, ghs_, ghr_ |
| `npm-token` | `npm_[A-Za-z0-9]{36}` | npm registry token |
| `openai-key` | `sk-[A-Za-z0-9]{20,}` | OpenAI API key |

**Redaction format:** `[REDACTED:<type>]` (e.g., `[REDACTED:aws-access-key]`)

**DetectedTypes:** Deduplicated list — the same credential type appearing multiple times is reported only once. The list is `nil` (not an empty slice) when no credentials are found.

**Custom patterns:** `CredentialFilterConfig.AdditionalPatterns` accepts raw regex strings. Each match is labeled `"custom"` in `DetectedTypes`. Compiled per-call (acceptable: small caller-controlled set).

**Go RE2 guarantee:** All patterns use bounded character-class repeats with no nested quantifiers. Go's RE2 engine is linear-time by construction — catastrophic backtracking is impossible (T-02-02-04 accept disposition).

## Purity Contract

Both files satisfy `internal/policy` purity:
- `path.go` imports only `"strings"` — no os, net, net/http, io, sync, time, context
- `credentials.go` imports only `"regexp"` — no os, net, net/http, io, sync, time, context
- Both verified by `TestPathImportsArePure` and `TestCredentialsImportsArePure` (AST-based import analysis)

## TDD Gate Compliance

| Gate | Commit | Status |
|------|--------|--------|
| RED (path) | `f6b6669` test(02-02): add failing tests for EvaluatePath | PASS |
| GREEN (path) | `579e0ec` feat(02-02): implement EvaluatePath sensitive-path policy | PASS |
| RED (credentials) | `f8674fd` test(02-02): add failing tests for FilterCredentials | PASS |
| GREEN (credentials) | `fbda945` feat(02-02): implement FilterCredentials output redaction | PASS |

## Verification Results

```
go test ./internal/policy/... -count=1
ok  github.com/mzansi-agentive/beekeeper/internal/policy  2.343s

go vet ./internal/policy/...
(no output — clean)
```

Full policy package: 30 tests, 0 failures.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed test corpus: GitHub/npm token lengths too short**
- **Found during:** Task 2 GREEN phase — tests failed because test data had 34-char suffix vs 36-char minimum in regex
- **Issue:** Test values like `ghp_abcdefghijklmnopqrstuvwxyz012345AB` had only 34 chars after the `ghp_` prefix, but the regex requires `{36,}`
- **Fix:** Extended test token values to 40 chars after the prefix (e.g., `ghp_abcdefghijklmnopqrstuvwxyz0123456789abcd`)
- **Files modified:** `internal/policy/credentials_test.go`
- **Commit:** `fbda945` (combined with GREEN implementation)

## Known Stubs

None. Both functions are fully implemented with real logic. No hardcoded empty values or TODO placeholders that affect function behavior.

## Threat Flags

None. Both files are pure in-process functions with no new network endpoints, auth paths, file access patterns, or schema changes. The threat model coverage in the plan (T-02-02-01 through T-02-02-04) is complete.

## Self-Check: PASSED

Files created:
- `internal/policy/path.go` — FOUND
- `internal/policy/path_test.go` — FOUND
- `internal/policy/credentials.go` — FOUND
- `internal/policy/credentials_test.go` — FOUND

Commits exist:
- `f6b6669` (RED: path tests) — FOUND
- `579e0ec` (GREEN: path impl) — FOUND
- `f8674fd` (RED: credentials tests) — FOUND
- `fbda945` (GREEN: credentials impl) — FOUND
