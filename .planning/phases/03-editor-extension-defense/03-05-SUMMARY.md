# Plan 03-05 Summary — Editor Detection + JSONC Settings Patch + EDXT-01 Selftest Fixture

**Status:** DONE

## What Was Implemented

### 1. `internal/editorinit/detect.go` — Editor Detection

New package `editorinit`. Injectable package-level vars for testing:

```go
var lookPath = defaultLookPath   // swappable in tests
var statFunc  = os.Stat          // swappable in tests
```

#### Editor struct

```go
type Editor struct {
    Name            string
    Executable      string
    ExecutableFound bool
    ExtensionDir    string
    SettingsPath    string
}
```

#### Function signature

```go
func DetectEditors() ([]Editor, error)
```

Platform-aware helpers:
- `homeDir() string` — `os.UserHomeDir()`, fallback `"~"`
- `configBase() string` — Windows: `os.UserConfigDir()` (%APPDATA%); Unix: `~/.config`

Static editor table (evaluated at call time):

| Editor | Executables | ExtensionDir | SettingsPath |
|--------|-------------|--------------|--------------|
| VS Code | code, code-insiders, codium | `~/.vscode/extensions` | `<configBase>/Code/User/settings.json` |
| Cursor | cursor | `~/.cursor/extensions` | `<configBase>/Cursor/User/settings.json` |
| Windsurf | windsurf | `~/.windsurf/extensions` | `<configBase>/Windsurf/User/settings.json` |

Note: Cursor Windows path carries the comment `// Assumption A1 (LOW confidence): mirrors VS Code convention; needs empirical validation`.

`DetectEditors()` includes an editor if `ExecutableFound OR extensionDir exists`.

### 2. `internal/editorinit/settings.go` — JSONC-Safe Settings Patch

#### Function signatures

```go
func PatchSettings(path, key string, value any) error
func DisableExtensionAutoUpdate(settingsPath string) error
```

`PatchSettings` behavior:
- Missing file → starts from `{}`
- Strips JSONC comments via `jsonc.ToJSON()` before unmarshal
- Idempotent: sets key regardless of pre-existing value
- Atomic write: `path + ".tmp"` then `os.Rename`
- Creates parent dirs with `os.MkdirAll(..., 0o755)`
- Doc comment warns: "existing // and /* */ comments in the file are removed on write"

`DisableExtensionAutoUpdate(settingsPath) error` — thin wrapper:
```go
return PatchSettings(settingsPath, "extensions.autoUpdate", false)
```

### 3. EDXT-01 Fixture — `internal/check/corpus/fixtures.json`

Added as 9th fixture (array element after existing 8):

```json
{
  "name": "EDXT-01 editor extension CLI install routes through catalog (nrwl.angular-console)",
  "tool_call": {
    "agent_name": "test",
    "tool_name": "Bash",
    "tool_input": {
      "command": "code --install-extension nrwl.angular-console@18.95.0"
    }
  },
  "expect_level": "warn",
  "expect_allow": true,
  "expect_catalog_match": true,
  "expect_rule_id": "bumblebee-catalog-match"
}
```

`expect_level` is `"warn"` (not "block") because `selftestEntries` contains
nrwl.angular-console as a single unsigned bumblebee source → single-source
warn semantics per corroboration thresholds.

### 4. Dependencies

`github.com/tidwall/jsonc v0.3.3` added to `go.mod` (was missing from prior plan
execution; added by `go get` during this plan).

## Test Results

```
=== RUN   TestDetectEditors
--- PASS: TestDetectEditors (0.00s)
=== RUN   TestPatchEditorSettings
--- PASS: TestPatchEditorSettings (0.04s)
=== RUN   TestPatchEditorSettingsIdempotent
--- PASS: TestPatchEditorSettingsIdempotent (0.02s)
=== RUN   TestPatchEditorSettingsJSONC
--- PASS: TestPatchEditorSettingsJSONC (0.04s)
=== RUN   TestPatchEditorSettingsCreateFile
--- PASS: TestPatchEditorSettingsCreateFile (0.01s)
PASS
ok  github.com/mzansi-agentive/beekeeper/internal/editorinit

=== RUN   TestSelftestAllFixturesPass
--- PASS: TestSelftestAllFixturesPass (0.02s)
PASS
ok  github.com/mzansi-agentive/beekeeper/internal/check

go build ./...   → exit 0
go vet ./internal/editorinit/... → exit 0
Select-String "install-extension" internal\check\corpus\fixtures.json → line 126 match confirmed
```

## Files Created

- `internal/editorinit/detect.go`
- `internal/editorinit/lookup.go`
- `internal/editorinit/detect_test.go`
- `internal/editorinit/settings.go`
- `internal/editorinit/settings_test.go`
- `internal/check/corpus/fixtures.json` (extended with EDXT-01 fixture)
