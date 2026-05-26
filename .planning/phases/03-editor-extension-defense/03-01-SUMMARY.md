# Plan 03-01 Summary — EDXT-01 Extension Install Recognition + Wave 1 Deps

**Status:** DONE

## What Was Implemented

### Go Dependencies Added (go.mod)

| Module | Version |
|--------|---------|
| `github.com/fsnotify/fsnotify` | `v1.10.1` |
| `github.com/gen2brain/beeep` | `v0.11.2` |
| `github.com/tidwall/jsonc` | `v0.3.3` |

`go mod verify` confirmed "all modules verified". `go build ./...` exits 0.

### EDXT-01: Agent-Initiated Editor-Extension CLI Install Recognition

**File:** `internal/policy/engine.go`

#### Package-level var added

```go
var editorInstallPatterns = []string{
    "code --install-extension ",
    "code-insiders --install-extension ",
    "cursor --install-extension ",
    "windsurf --install-extension ",
}
```

#### Functions added

```go
func extractExtensionInstall(cmd string) (ecosystem, pkg, version string, ok bool)
```
- Lowercase+trim cmd for pattern matching via `strings.Index`
- Extracts extension ID from original cmd (preserves case for display)
- Calls `firstPackageToken()` and `splitVersion()` (existing helpers)
- Returns `("editor-extension", normalize(name), ver, true)` on match

```go
func extractAllExtensionInstalls(cmd string) []string
```
- Returns every normalized publisher.name after each `--install-extension ` occurrence
- Used for bulk multi-flag commands

#### Modifications to existing functions

- `extract()`: calls `extractExtensionInstall(cmd)` BEFORE `extractFromCommand(cmd)` in the command-shape branch
- `Evaluate()`: detects bulk installs (2+ `--install-extension ` occurrences), evaluates each ID via recursive `Evaluate()` calls, returns worst decision (block > warn > allow)

**Purity constraint:** `engine.go` imports only `"strings"`. `TestEngineImportsArePure` passes.

## Test Results

All 3 new tests pass:

| Test | Cases | Result |
|------|-------|--------|
| `TestExtensionInstallExtract` | 8 sub-tests (with/without version, all 4 editors, case-insensitive, negative cases) | PASS |
| `TestExtensionInstallBulk` | extractAllExtensionInstalls + Evaluate worst-decision | PASS |
| `TestExtensionInstallVariants` | All 4 editor prefixes with ms-python.python@2026.4.0 | PASS |

Full policy test suite: `go test ./internal/policy/... -count=1` — **PASS** (all 17 tests)

## go.mod Deps Added

```
require (
    github.com/fsnotify/fsnotify v1.10.1
    github.com/gen2brain/beeep v0.11.2
    github.com/tidwall/jsonc v0.3.3
)
```
