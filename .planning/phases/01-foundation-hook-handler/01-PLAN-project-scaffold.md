---
phase: 01-foundation-hook-handler
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - go.mod
  - go.sum
  - cmd/beekeeper/main.go
  - internal/version/version.go
  - internal/platform/dirs.go
  - internal/platform/dirs_test.go
  - internal/platform/perms_unix.go
  - internal/platform/perms_windows.go
  - internal/platform/perms_test.go
  - .github/workflows/ci.yml
  - .gitignore
autonomous: true
requirements: [SFDF-03]
must_haves:
  truths:
    - "go build ./... succeeds with no business logic in cmd/beekeeper/main.go"
    - "beekeeper version prints version, commit, and date"
    - "beekeeper init creates the state directory (~/.beekeeper on Unix, %APPDATA%\\beekeeper on Windows)"
    - "go mod verify passes in CI on ubuntu-latest, macos-latest, and windows-latest"
    - "StateDir() returns ~/.beekeeper on Unix and %APPDATA%\\beekeeper on Windows"
    - "SetOwnerOnly restricts a file to owner-only access on both Unix (0600) and Windows (DACL)"
  artifacts:
    - path: "go.mod"
      provides: "Go module with pinned Go 1.25 toolchain and Cobra dependency"
      contains: "module github.com/mzansi-agentive/beekeeper"
    - path: "cmd/beekeeper/main.go"
      provides: "Cobra root command + subcommand registration only"
      min_lines: 20
    - path: "internal/platform/dirs.go"
      provides: "Cross-platform StateDir()"
      exports: ["StateDir"]
    - path: "internal/platform/perms_unix.go"
      provides: "Unix 0600 owner-only file permission"
      exports: ["SetOwnerOnly"]
    - path: "internal/platform/perms_windows.go"
      provides: "Windows DACL owner-only file permission via go-acl"
      exports: ["SetOwnerOnly"]
    - path: ".github/workflows/ci.yml"
      provides: "Cross-platform test matrix + go mod verify"
      contains: "windows-latest"
  key_links:
    - from: "cmd/beekeeper/main.go"
      to: "internal/version"
      via: "ldflags-injected version vars passed to version subcommand"
      pattern: "version\\."
    - from: "cmd/beekeeper/main.go"
      to: "internal/platform.StateDir"
      via: "init subcommand creates state dir"
      pattern: "platform\\.StateDir"
---

<objective>
Establish the Beekeeper Go module, the Cobra CLI skeleton, the cross-platform state-directory and file-permission primitives, and the GitHub Actions CI matrix. This is the foundation every other Phase 1 plan builds on: it must compile and pass `go build ./...` / `go test ./...` / `go mod verify` on Linux, macOS, and Windows from the first commit.

Purpose: Without a buildable module, pinned dependencies, and the platform abstraction layer (state directory + owner-only permissions), no catalog, policy, hook, or audit work can begin.
Output: A compiling `beekeeper` binary with `version`, `init` (stub), and placeholder subcommands wired through Cobra; the `internal/platform` package; a green CI matrix on all three OSes.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/01-foundation-hook-handler/01-CONTEXT.md
@.planning/phases/01-foundation-hook-handler/01-RESEARCH.md
@CLAUDE.md

<interfaces>
<!-- Contracts this plan CREATES that downstream plans consume. Define them exactly. -->

internal/platform/dirs.go:
```go
// StateDir returns the Beekeeper state directory:
//   Windows: %APPDATA%\beekeeper  (via os.UserConfigDir)
//   Unix:    ~/.beekeeper         (via os.UserHomeDir)
func StateDir() (string, error)

// CatalogDir returns filepath.Join(StateDir(), "catalogs")
func CatalogDir() (string, error)

// AuditDir returns filepath.Join(StateDir(), "audit")
func AuditDir() (string, error)

// ConfigPath returns filepath.Join(StateDir(), "config.json")
func ConfigPath() (string, error)
```

internal/platform/perms_unix.go and perms_windows.go (build-tagged):
```go
// SetOwnerOnly restricts the file at path to owner-only read/write.
// Unix: os.Chmod(path, 0600). Windows: acl.Chmod(path, 0600) via hectane/go-acl.
func SetOwnerOnly(path string) error
```

internal/version/version.go:
```go
// Populated via -ldflags -X at build time.
var Version = "dev"
var Commit = "none"
var Date = "unknown"
```
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Initialize Go module, dependencies, and Cobra CLI skeleton</name>
  <read_first>
    - CLAUDE.md (file structure section + architecture constraints — cmd/ is thin wiring only)
    - .planning/phases/01-foundation-hook-handler/01-CONTEXT.md (CLI subcommands list, Go binary structure)
    - .planning/phases/01-foundation-hook-handler/01-RESEARCH.md (Standard Stack table, Recommended Project Structure)
  </read_first>
  <files>go.mod, go.sum, cmd/beekeeper/main.go, internal/version/version.go, .gitignore</files>
  <action>
    Run `go mod init github.com/mzansi-agentive/beekeeper` to create go.mod. Add a `go 1.25` line and a `toolchain go1.25.0` directive (pin the toolchain exactly for reproducible builds per RESEARCH Pitfall 3). Add dependency `github.com/spf13/cobra@v1.10.2` via `go get`; this is the only direct dependency added in this task. Run `go mod tidy` so go.sum is populated and pinned.

    Create internal/version/version.go declaring package `version` with three package-level string vars: `Version = "dev"`, `Commit = "none"`, `Date = "unknown"`. These are overwritten at release time via `-ldflags -X github.com/mzansi-agentive/beekeeper/internal/version.Version=...` etc.

    Create cmd/beekeeper/main.go containing ONLY Cobra wiring (no business logic — enforced by CLAUDE.md). Define `rootCmd` with Use "beekeeper", a Short description, and SilenceUsage true. Register these subcommands now as separate Cobra commands wired in main.go (each can call a thin `newXxxCmd()` constructor): `version`, `init`, `check`, `catalogs` (with `sync` child), `audit` (with `tail` child), `selftest`. For this task, `version` is fully implemented (prints `version.Version`, `version.Commit`, `version.Date`); `init` is implemented in Task 2; `check`, `catalogs sync`, `audit tail`, and `selftest` are registered as commands whose RunE returns `fmt.Errorf("not yet implemented")` and exits non-zero — these are claimed by later plans (do NOT implement their logic here). main() calls `rootCmd.Execute()` and exits non-zero on error. Do NOT implement `beekeeper hooks install` — that is INTG-01, Phase 4, out of scope.

    Create a .gitignore covering `/dist/`, `*.exe`, build artifacts, and editor cruft.
  </action>
  <verify>
    <automated>go build ./... 2>&1 && go vet ./... 2>&1 && go run ./cmd/beekeeper version 2>&1</automated>
  </verify>
  <acceptance_criteria>
    - `go.mod` contains `module github.com/mzansi-agentive/beekeeper`, a `go 1.25` line, a `toolchain go1.25` directive, and `require github.com/spf13/cobra v1.10.2`
    - `go.sum` exists and is non-empty
    - `go build ./...` exits 0
    - `go vet ./...` exits 0
    - `cmd/beekeeper/main.go` imports `internal/version` and contains no policy/catalog/file-I/O business logic (only Cobra command wiring and version printing)
    - `go run ./cmd/beekeeper version` prints three lines or fields containing "dev", "none", and "unknown" (the default ldflags values)
    - `go run ./cmd/beekeeper check` exits non-zero with a "not yet implemented" message (claimed by plan 05)
    - No reference to `beekeeper hooks install` exists in the codebase (grep for "hooks install" returns nothing)
  </acceptance_criteria>
  <done>Module compiles on the dev machine; `beekeeper version` works; all Phase 1 subcommands are registered with later ones stubbed as not-implemented.</done>
</task>

<task type="auto">
  <name>Task 2: Cross-platform state directory + owner-only permissions, plus beekeeper init</name>
  <read_first>
    - .planning/phases/01-foundation-hook-handler/01-RESEARCH.md (Pattern 3 Cross-Platform State Directory, Pattern 4 Windows 0600-equivalent permissions, anti-pattern on os.UserConfigDir for Unix)
    - CLAUDE.md (state directory layout under ~/.beekeeper/)
    - cmd/beekeeper/main.go (the init command stub created in Task 1)
  </read_first>
  <files>internal/platform/dirs.go, internal/platform/dirs_test.go, internal/platform/perms_unix.go, internal/platform/perms_windows.go, internal/platform/perms_test.go, cmd/beekeeper/main.go, go.mod, go.sum</files>
  <action>
    Add dependency `github.com/hectane/go-acl` via `go get` (Windows DACL, no CGO — per RESEARCH Standard Stack), then `go mod tidy`.

    Create internal/platform/dirs.go (package `platform`, no build tag) implementing `StateDir()`, `CatalogDir()`, `AuditDir()`, and `ConfigPath()` exactly as in the interfaces block. CRITICAL per RESEARCH anti-pattern: on Windows use `os.UserConfigDir()` (returns %APPDATA%) joined with "beekeeper"; on all other GOOS use `os.UserHomeDir()` joined with ".beekeeper". Do NOT use `os.UserConfigDir()` on Unix (it would yield ~/.config/beekeeper). Branch on `runtime.GOOS == "windows"`.

    Create internal/platform/perms_unix.go with build tag `//go:build !windows` implementing `SetOwnerOnly(path string) error` as `os.Chmod(path, 0600)`.

    Create internal/platform/perms_windows.go with build tag `//go:build windows` implementing `SetOwnerOnly(path string) error` as `acl.Chmod(path, 0600)` importing `github.com/hectane/go-acl`.

    Create internal/platform/dirs_test.go: TestStateDirReturnsExpectedSuffix asserts the returned path ends with ".beekeeper" on non-Windows and with `filepath.Join("beekeeper")` (i.e. ends in "beekeeper") on Windows, branching on runtime.GOOS. TestCatalogDirUnderStateDir asserts CatalogDir is StateDir + "catalogs".

    Create internal/platform/perms_test.go: TestSetOwnerOnly creates a temp file, calls SetOwnerOnly, and on Unix asserts `os.Stat(...).Mode().Perm() == 0600`; on Windows asserts SetOwnerOnly returns nil error (DACL content assertion is out of scope for unit test — covered behaviorally by audit plan).

    Implement the `init` subcommand in main.go (or its newInitCmd constructor): call `platform.StateDir()`, `os.MkdirAll` the state dir, `os.MkdirAll` the catalog dir and audit dir, and print the created path. This is the Phase 1 stub — no editor detection, no full onboarding (that is EDXT-06, Phase 3, out of scope). Print a one-line success message with the path.
  </action>
  <verify>
    <automated>go test ./internal/platform/... -count=1 2>&1 && go run ./cmd/beekeeper init 2>&1</automated>
  </verify>
  <acceptance_criteria>
    - `go test ./internal/platform/... -count=1` exits 0
    - `internal/platform/perms_unix.go` first line is `//go:build !windows` and it calls `os.Chmod(path, 0600)`
    - `internal/platform/perms_windows.go` first line is `//go:build windows` and it imports `github.com/hectane/go-acl`
    - `internal/platform/dirs.go` branches on `runtime.GOOS == "windows"` and uses `os.UserHomeDir` on the non-Windows path (grep confirms `os.UserHomeDir` present and `os.UserConfigDir` only appears inside the windows branch)
    - `go run ./cmd/beekeeper init` exits 0 and creates a directory whose path is returned by `platform.StateDir()`, plus `catalogs/` and `audit/` subdirectories
    - `go.mod` contains `require github.com/hectane/go-acl`
  </acceptance_criteria>
  <done>State directory and owner-only permission primitives exist and are unit-tested cross-platform; `beekeeper init` creates the directory tree.</done>
</task>

<task type="auto">
  <name>Task 3: Cross-platform CI matrix with go mod verify</name>
  <read_first>
    - .planning/phases/01-foundation-hook-handler/01-RESEARCH.md (GitHub Actions CI Workflow code example, note on -race requiring CGO_ENABLED=1)
    - CLAUDE.md (Build constraints, cross-platform CI requirements)
    - go.mod (created in Task 1 — referenced by go-version-file)
  </read_first>
  <files>.github/workflows/ci.yml</files>
  <action>
    Create .github/workflows/ci.yml triggered on `pull_request` and `push` to `main`. Define a single job `test` with a matrix `os: [ubuntu-latest, macos-latest, windows-latest]` and `fail-fast: false`, running on `${{ matrix.os }}`. Steps in order: (1) actions/checkout@v4; (2) actions/setup-go@v5 with `go-version-file: go.mod` and `cache: true`; (3) `go mod verify` (this satisfies the SFDF-03 CI gate); (4) `go build -v -trimpath -buildvcs=false ./...`; (5) `go test -v -race ./...` with env `CGO_ENABLED: 1` (the race detector requires CGO per RESEARCH note); (6) `go vet ./...`.

    Add a second step in the test job after vet that runs `go mod tidy` followed by a git-diff check that fails if go.mod or go.sum changed (this guards against the Renovate stale-go.sum pitfall and keeps deps honest). On Windows use a cross-platform diff approach: run `git diff --exit-code go.mod go.sum`.

    Do NOT add the release workflow here — that is plan 06 (self-defense). This file is CI only.
  </action>
  <verify>
    <automated>node -e "const y=require('fs').readFileSync('.github/workflows/ci.yml','utf8'); ['ubuntu-latest','macos-latest','windows-latest','go mod verify','-buildvcs=false','CGO_ENABLED'].forEach(s=>{if(!y.includes(s)){console.error('MISSING: '+s);process.exit(1)}}); console.log('ci.yml OK')"</automated>
  </verify>
  <acceptance_criteria>
    - `.github/workflows/ci.yml` matrix contains all three of `ubuntu-latest`, `macos-latest`, `windows-latest`
    - The workflow runs `go mod verify` as an explicit step (satisfies SFDF-03 CI dependency-pinning gate)
    - The build step uses `-trimpath -buildvcs=false`
    - The test step sets `CGO_ENABLED: 1` (race detector requirement)
    - The workflow includes a `git diff --exit-code go.mod go.sum` guard after `go mod tidy`
    - The workflow does NOT contain `goreleaser` or `cosign` (those belong to plan 06)
  </acceptance_criteria>
  <done>CI workflow validates build, test (race), vet, and go mod verify on all three OSes for every PR.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| filesystem → process | State directory paths derived from OS env (HOME, APPDATA) influence where Beekeeper writes |
| CI → release | go.mod/go.sum pinning is the supply-chain trust anchor; CI must verify it |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-01-01 | Tampering | go.mod/go.sum dependency pinning | mitigate | CI step `go mod verify` plus `git diff --exit-code go.mod go.sum` after `go mod tidy` blocks unpinned/mutated dependencies on every PR (SFDF-03) |
| T-01-02 | Information Disclosure | State directory creation in `beekeeper init` | mitigate | State dir created via MkdirAll; owner-only enforcement (`SetOwnerOnly`) is provided by `internal/platform` and applied to sensitive files by the audit plan; directory layout under user-private home/APPDATA |
| T-01-03 | Elevation of Privilege | `cmd/beekeeper/main.go` accidentally containing business logic | accept | Architectural review gate (CLAUDE.md constraint); acceptance criteria assert no policy/catalog logic in cmd/ — low exploitability, design-time control |
| T-01-04 | Spoofing | Resolved Cobra subcommand routing to wrong handler | mitigate | Later subcommands are explicit not-implemented stubs that exit non-zero, never silently no-op (avoids a check command silently allowing) |
</threat_model>

<verification>
- `go build ./...`, `go vet ./...`, and `go test ./internal/platform/... -count=1` all exit 0 on the dev machine
- `go run ./cmd/beekeeper version` and `go run ./cmd/beekeeper init` both succeed
- CI workflow YAML contains the three-OS matrix and `go mod verify`
</verification>

<success_criteria>
- A compiling Go module with pinned Go 1.25 toolchain and pinned Cobra + go-acl dependencies
- Cross-platform `StateDir()` and `SetOwnerOnly()` primitives, unit-tested
- `beekeeper version` and `beekeeper init` functional; all other Phase 1 subcommands registered (stubbed)
- CI matrix green-able on ubuntu/macos/windows with `go mod verify` enforced
</success_criteria>

<output>
After completion, create `.planning/phases/01-foundation-hook-handler/01-01-SUMMARY.md`
</output>
