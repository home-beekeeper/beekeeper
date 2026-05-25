# Phase 1 / Plan 01 — Project Scaffold — Summary

**Plan:** `01-PLAN-project-scaffold.md`
**Executed:** 2026-05-26
**Status:** Complete — all acceptance criteria met on the Windows dev machine.

## What Was Built

The foundational Go module, Cobra CLI skeleton, cross-platform platform
primitives, and the GitHub Actions CI matrix that every other Phase 1 plan
builds on.

### Module

- **Module path:** `github.com/mzansi-agentive/beekeeper`
- **Go version line:** `go 1.25`
- **Toolchain directive:** `toolchain go1.25.0` (pinned for reproducible builds)
- **Go installed during execution:** Go 1.25.0 (windows/amd64) was not present on
  the dev machine; installed via `winget install GoLang.Go` to
  `C:\Program Files\Go`.

### Dependencies added (direct)

| Module | Version | Purpose |
|--------|---------|---------|
| `github.com/spf13/cobra` | v1.10.2 | CLI subcommand routing |
| `github.com/hectane/go-acl` | v0.0.0-20230122075934-ca0b05cb1adb | Windows DACL for 0600-equivalent perms (no CGO) |

Indirect (pulled transitively, recorded in `go.sum`): `spf13/pflag`,
`inconshreveable/mousetrap`, `golang.org/x/sys`.

### Files created

- `go.mod`, `go.sum` — pinned module + dependency graph
- `cmd/beekeeper/main.go` — Cobra wiring ONLY (no business logic)
- `internal/version/version.go` — `Version`/`Commit`/`Date` ldflags targets
- `internal/platform/dirs.go` — cross-platform state directory resolution
- `internal/platform/perms_unix.go` (`//go:build !windows`) — `SetOwnerOnly` via `os.Chmod`
- `internal/platform/perms_windows.go` (`//go:build windows`) — `SetOwnerOnly` via `acl.Chmod`
- `internal/platform/dirs_test.go` — `TestStateDirReturnsExpectedSuffix`, `TestCatalogDirUnderStateDir` (+ audit/config coverage)
- `internal/platform/perms_test.go` — `TestSetOwnerOnly`
- `.github/workflows/ci.yml` — 3-OS test matrix + `go mod verify` + tidy guard
- `.gitignore` — build artifacts, anchored binary patterns, editor cruft

### CLI subcommands registered

| Command | State |
|---------|-------|
| `version` | Fully implemented — prints `version`, `commit`, `date` |
| `init` | Fully implemented — creates state + `catalogs/` + `audit/` dirs |
| `check` | Stub — `RunE` returns `not yet implemented`, exits non-zero (Plan 05) |
| `catalogs sync` | Stub — not yet implemented (Plan 02) |
| `audit tail` | Stub — not yet implemented (Plan 05) |
| `selftest` | Stub — not yet implemented (later plan) |

`beekeeper hooks install` is intentionally **not** present (INTG-01, Phase 4).

## Key Interfaces Created (for downstream plans)

```go
// internal/platform/dirs.go
func StateDir() (string, error)    // Windows: %APPDATA%\beekeeper ; Unix: ~/.beekeeper
func CatalogDir() (string, error)  // <StateDir>/catalogs
func AuditDir() (string, error)    // <StateDir>/audit
func ConfigPath() (string, error)  // <StateDir>/config.json

// internal/platform/perms_unix.go + perms_windows.go (build-tagged)
func SetOwnerOnly(path string) error  // Unix: os.Chmod 0600 ; Windows: acl.Chmod 0600

// internal/version/version.go
var Version = "dev"
var Commit  = "none"
var Date    = "unknown"
```

State directory resolution branches on `runtime.GOOS == "windows"`:
Windows uses `os.UserConfigDir()` (%APPDATA%) + `beekeeper`; all other GOOS use
`os.UserHomeDir()` + `.beekeeper`. `os.UserConfigDir()` is deliberately **not**
used on Unix (it would yield `~/.config/beekeeper`).

## Acceptance Criteria Met

### Task 1 — module + CLI skeleton
- [x] `go.mod` has `module github.com/mzansi-agentive/beekeeper`, `go 1.25`, `toolchain go1.25.0`, `require github.com/spf13/cobra v1.10.2`
- [x] `go.sum` exists and is non-empty
- [x] `go build ./...` exits 0
- [x] `go vet ./...` exits 0
- [x] `cmd/beekeeper/main.go` imports `internal/version` and contains only Cobra wiring (no policy/catalog/file-I/O business logic)
- [x] `go run ./cmd/beekeeper version` prints `dev` / `none` / `unknown`
- [x] `go run ./cmd/beekeeper check` exits non-zero with `Error: not yet implemented`
- [x] No `hooks install` command exists in the codebase (only planning docs reference the phrase)

### Task 2 — platform primitives + init
- [x] `go test ./internal/platform/... -count=1` exits 0
- [x] `perms_unix.go` first line is `//go:build !windows` and calls `os.Chmod(path, 0600)`
- [x] `perms_windows.go` first line is `//go:build windows` and imports `github.com/hectane/go-acl`
- [x] `dirs.go` branches on `runtime.GOOS == "windows"`; uses `os.UserHomeDir` on non-Windows; `os.UserConfigDir` only inside the Windows branch
- [x] `go run ./cmd/beekeeper init` exits 0 and creates the `StateDir()` path plus `catalogs/` and `audit/` (verified: `C:\Users\Bantu\AppData\Roaming\beekeeper`)
- [x] `go.mod` contains `require github.com/hectane/go-acl`

### Task 3 — CI matrix
- [x] `.github/workflows/ci.yml` matrix contains `ubuntu-latest`, `macos-latest`, `windows-latest` with `fail-fast: false`
- [x] Explicit `go mod verify` step (SFDF-03 gate)
- [x] Build step uses `-trimpath -buildvcs=false`
- [x] Test step sets `CGO_ENABLED: 1` (race detector requirement)
- [x] `git diff --exit-code go.mod go.sum` guard after `go mod tidy`
- [x] No `goreleaser` or `cosign` (those are Plan 06)
- [x] Node verification one-liner passes (`ci.yml OK`)

### Cross-cutting verification
- [x] `go mod verify` → `all modules verified`
- [x] `go mod tidy` produces no diff (tidy guard green locally)
- [x] `go test ./... -count=1` green

## Deviations from the Plan

1. **`go.mod` `go` directive normalization.** `go mod init` on Go 1.25.0 wrote
   `go 1.25.0`. The plan specifies a `go 1.25` line plus a separate
   `toolchain go1.25.0` directive, so the `go` line was edited back to `go 1.25`
   and `toolchain go1.25.0` was added explicitly. Net result matches the plan.

2. **`.gitignore` binary pattern anchoring.** The initial `.gitignore` used bare
   `beekeeper` / `beekeeper.exe` patterns to ignore the build output. Git matched
   these against the `cmd/beekeeper/` source directory, hiding `main.go` from
   tracking. Fixed by anchoring to the repo root (`/beekeeper`, `/beekeeper.exe`)
   so only top-level build artifacts are ignored. No functional change to intent.

3. **Cobra error visibility.** The root command sets `SilenceUsage: true` but
   leaves `SilenceErrors` at its default (false) so that stub subcommands print
   `Error: not yet implemented` to stderr while still exiting non-zero — this is
   required to satisfy the Task 1 acceptance criterion that `check` returns a
   "not yet implemented" message. Usage spam on error is still suppressed.

4. **Go toolchain installation.** Go was absent from the dev machine; it was
   installed (Go 1.25.0) as a prerequisite. This is environment setup, not a
   change to the plan's deliverables.

No scope was added beyond Plan 01: no policy logic, no catalog logic, no
`beekeeper hooks install`.
