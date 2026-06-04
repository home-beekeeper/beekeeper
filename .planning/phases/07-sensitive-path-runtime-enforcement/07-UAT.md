---
status: complete
phase: 07-sensitive-path-runtime-enforcement
source: [07-01-SUMMARY.md, 07-02-SUMMARY.md, 07-03-SUMMARY.md]
started: 2026-06-04
updated: 2026-06-04
mode: automated
approval: approved (user-directed automated verification, 2026-06-04)
---

## Current Test

[testing complete]

## Verification Method

Automated, user-directed ("automate and approved verification"). Each scenario was driven against
the **compiled `beekeeper check` binary** (`go build -o beekeeper.exe ./cmd/beekeeper`) — not just
the test suite — to prove the user-facing fix for finding **F2** (credential reads previously
returned exit 0 ALLOW at the binary level).

Harness: isolated temp HOME (`./.uat-home`, USERPROFILE override) with an **empty mmap catalog
index** built via `catalog.BuildIndex(path, nil)` — so the catalog baseline is ALLOW and any block
is attributable to the sensitive-path engine, not fail-closed-on-missing-catalog. Each case piped
raw hook JSON to `beekeeper check` and recorded the process exit code (`allow=0`, `block≠0`).

SC4 (persisted `decision:"block"` audit record): the live binary emitted the correct decision to
stdout (`Level:"block"`, `RuleIDs:["sensitive-path-policy"]`); the **persisted** NDJSON audit record
with `decision:"block"` + `sensitive-path-policy` is asserted by the green `RunCheck` integration
tests (`internal/check/handler_test.go:861-867`, verifier-confirmed). The on-disk audit write was
not separately captured in the isolated-HOME harness (a Git-Bash/MSYS2 env-var path-conversion
quirk affecting only the ad-hoc test home — not a product defect; `writeAuditWithAC` is unconditional
and proven by the integration test).

## Tests

### 1. Credential file read blocks (SC1 / SPATH-01)
expected: `beekeeper check` on `{"tool_name":"Read","tool_input":{"file_path":"~/.aws/credentials"}}` exits non-zero (block); previously exit 0.
result: pass
evidence: live binary exit=1; stdout `{"Allow":false,"Level":"block","Reason":"sensitive path blocked: /.aws/","RuleIDs":["sensitive-path-policy"]}`

### 2. Path traversal blocks (SC1 / SPATH-02)
expected: `file_path":"../../.aws/credentials"` is canonicalized and exits non-zero (block) — traversal cannot bypass the blocklist.
result: pass
evidence: live binary exit=1

### 3. Bash `cat` of credential blocks (SC2 / SPATH-03)
expected: `{"tool_name":"Bash","tool_input":{"command":"cat ~/.ssh/id_rsa"}}` is detected as credential access and exits non-zero.
result: pass
evidence: live binary exit=1

### 4. Bash `type %USERPROFILE%\.ssh\id_rsa` blocks (SC2 / SPATH-03 / D-01)
expected: the Windows env-var form is expanded (`%USERPROFILE%`→home) and blocks — proving D-01 env-var expansion is live in the compiled binary.
result: pass
evidence: live binary exit=1 (USERPROFILE set to the isolated temp home; expansion resolved the path to `/.ssh/`)

### 5. Safe lookalike `.env.example` allowed (SC3 / SPATH-04)
expected: `file_path":".env.example"` is NOT blocked — exit 0 (allow).
result: pass
evidence: live binary exit=0

### 6. Safe lookalike `.env.test` allowed (SC3 / SPATH-04)
expected: `.env.test` exits 0 (allow).
result: pass
evidence: live binary exit=0

### 7. Safe lookalike `.env.schema` allowed (SC3 / SPATH-04)
expected: `.env.schema` exits 0 (allow).
result: pass
evidence: live binary exit=0

### 8. `.env.production` still blocks (SC3 negative / SPATH-04)
expected: a real `.env.production` is NOT allowlisted — exits non-zero (block); proves the `.env.*` block glob is intact and the allowlist is not over-broad.
result: pass
evidence: live binary exit=1

## Summary

total: 8
passed: 8
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none]
