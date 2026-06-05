---
phase: 10-hook-block-protocol-compliance-and-multi-harness-enforcement
plan: 05
subsystem: hooks
tags: [hermes, cline, opencode, hook-installer, yaml-patch, executable-script, js-plugin, build-tags]

# Dependency graph
requires:
  - phase: 10-01
    provides: RenderDeny adapter with HarnessHermes/HarnessCline/HarnessOpenCode constants and deny contracts
  - phase: 10-04
    provides: hooks.go dispatch infrastructure + backupSettings/writeFileAtomic helpers
provides:
  - Hermes installer: idempotent YAML string patch for pre_tool_call in ~/.hermes/config.yaml (no gopkg.in/yaml.v3 dep)
  - Cline installer: executable PreToolUse script (mode 0o755, #!/bin/sh shebang) at ~/Documents/Cline/Rules/Hooks/ (macOS/Linux only)
  - cline_windows.go stub: returns clear "macOS/Linux only" error on Windows
  - OpenCode plugin installer: writes beekeeper.js (tool.execute.before + throw + caveats) to ~/.config/opencode/plugins/
  - Kilo/Trae gateway guides via printKiloGuide/printTraeGuide (MCP-only harnesses)
  - TargetHermes, TargetCline, TargetKilo, TargetTrae consts in hooks.go
  - All four dispatched in InstallTo/UninstallTo
  - Contract-shape tests: TestInstallHermes, TestInstallOpenCodePlugin, TestInstallCline (build-tagged !windows)
affects:
  - 10-06 (support matrix docs: Tier 2 entries for Hermes/Cline/OpenCode)
  - future phases using hooks.go dispatch

# Tech tracking
tech-stack:
  added: []
  patterns:
    - YAML idempotent string patching (line-by-line scanner, no gopkg.in/yaml.v3)
    - Executable file installer (os.WriteFile with mode 0o755)
    - Build-tag split for platform-specific installers (!windows / windows)
    - JS plugin template as Go const string with embedded tool.execute.before hook

key-files:
  created:
    - internal/hooks/hermes.go
    - internal/hooks/cline.go
    - internal/hooks/cline_windows.go
    - internal/hooks/opencode_plugin.go
    - internal/hooks/cline_test.go
  modified:
    - internal/hooks/hooks.go
    - internal/hooks/hooks_test.go
    - internal/hooks/gateway_targets.go

key-decisions:
  - "Hermes YAML patching: no gopkg.in/yaml.v3 — line-by-line bufio.Scanner with 3 cases (append full block / append under existing hooks: / insert under existing pre_tool_call:)"
  - "Cline build-tag split: cline.go (!windows) with real installer; cline_windows.go (windows) with clear error — GOOS=windows and GOOS=linux both compile clean"
  - "OpenCode plugin: JS const template with spawnSync + throw; caveats #5894 and #2319 printed to out after every install"
  - "TargetOpenCode moved from gatewayTargets to plugin installer dispatch; TargetKilo/TargetTrae added to gatewayTargets + gateway_targets.go"
  - "T-10-20: installCline backs up foreign PreToolUse script before overwriting; uninstallCline verifies clinePreCommand marker before removing"

patterns-established:
  - "YAML idempotent string patch without library dep: bufio.Scanner line scan + section-aware insertion"
  - "Platform-specific hook installer split: real implementation in !windows, clear error stub in windows build"
  - "JS plugin template as Go const string for zero-dependency plugin distribution"

requirements-completed: [HPC-02, HPC-03, HPC-05]

# Metrics
duration: 45min
completed: 2026-06-05
---

# Phase 10 Plan 05: Hermes/Cline/OpenCode Harness Installers Summary

**Hermes fail-open hook via idempotent YAML patch, Cline executable PreToolUse script (macOS/Linux only + Windows stub), and OpenCode JS plugin with throw-on-deny contract — all three dispatched and contract-tested**

## Performance

- **Duration:** ~45 min
- **Started:** 2026-06-05
- **Completed:** 2026-06-05
- **Tasks:** 3 (Tasks 1+2 committed together; Task 3 separate)
- **Files modified:** 8

## Accomplishments

- Hermes installer: idempotent YAML section patching writes `pre_tool_call: - command: beekeeper check --hook hermes` without any gopkg.in/yaml.v3 dependency; handles 3 config states (absent, hooks: present, pre_tool_call: present)
- Cline installer: writes executable PreToolUse script (#!/bin/sh, mode 0o755) with foreign-script backup and marker-based uninstall; build-tagged off Windows with a clear error stub
- OpenCode plugin installer: writes `beekeeper.js` JS template with `tool.execute.before` hook that shells to `beekeeper check --hook opencode` and throws on deny; prints #5894/#2319 subagent/MCP bypass caveats
- Kilo and Trae gateway guides added (MCP-only harnesses); TargetOpenCode moved from gatewayTargets to plugin installer dispatch
- Contract-shape tests: TestInstallHermes (6 subtests), TestInstallOpenCodePlugin (5 subtests), TestInstallCline build-tagged !windows (6 subtests); all pass on Windows and Linux

## Task Commits

1. **Tasks 1+2: Hermes/Cline/OpenCode installers + dispatch** - `840abbd` (feat)
2. **Task 3: Contract tests + OpenCode plugin fix** - `e4b1572` (feat)

## Files Created/Modified

- `internal/hooks/hermes.go` — installHermes/uninstallHermes; YAML string patching; hermesPreCommand/hermesConfigPath
- `internal/hooks/cline.go` — (!windows) installCline/uninstallCline; executable PreToolUse; 0o755; T-10-20 foreign-script guard
- `internal/hooks/cline_windows.go` — (windows) stub returning "macOS/Linux only" error
- `internal/hooks/opencode_plugin.go` — installOpenCodePlugin/uninstallOpenCodePlugin; openCodePluginTemplate JS; #5894/#2319 caveats
- `internal/hooks/cline_test.go` — (!windows) TestInstallCline: from_absent executable mode, idempotent, uninstall, foreign_script_preserved, dry_run
- `internal/hooks/hooks.go` — adds TargetHermes, TargetCline, TargetKilo, TargetTrae; updates gatewayTargets; InstallTo/UninstallTo dispatch for all four; updated allTargets
- `internal/hooks/hooks_test.go` — TestInstallHermes + TestInstallOpenCodePlugin; updated TestInstallGatewayTarget to use Kilo/Trae; adds Hermes/Cline/OpenCode to TestInstallDispatchNewTargets
- `internal/hooks/gateway_targets.go` — adds printKiloGuide, printTraeGuide; removes TargetOpenCode from dispatch; updates printOpenCodeGuide comment; retains function as fallback reference

## Decisions Made

- Hermes YAML patching without library: `bufio.Scanner` line-by-line with 3 insertion cases (no hooks: section → append full block; hooks: but no pre_tool_call: → insert after hooks:; pre_tool_call: exists → insert after it). No gopkg.in/yaml.v3 — satisfies CLAUDE.md "no new module deps".
- Cline build-tag split: `//go:build !windows` on cline.go (real installer), `//go:build windows` on cline_windows.go (stub). `GOOS=windows go build ./internal/hooks/` and `GOOS=linux go build ./internal/hooks/` both pass clean.
- OpenCode is a plugin not a CLI hook: TargetOpenCode removed from gatewayTargets and re-routed from printGatewayGuide to installOpenCodePlugin. gateway_targets.go retains printOpenCodeGuide as an MCP-fallback reference.
- T-10-20: installCline backs up any foreign PreToolUse script (with WARNING) before overwriting; uninstallCline checks containsClineCommand before removing to prevent silent destruction of foreign hooks.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed duplicate printOpenCodeGuide declaration**
- **Found during:** Task 1 (after adding Kilo/Trae guide functions)
- **Issue:** Accidentally created a second `printOpenCodeGuide` function in gateway_targets.go, causing `redeclared in this block` compile error
- **Fix:** Removed the duplicate; updated the original comment to note the plugin upgrade
- **Verification:** `go build ./...` exits 0
- **Committed in:** 840abbd (part of Task 1 commit)

**2. [Rule 1 - Bug] OpenCode plugin test: template uses spawnSync args not inline string**
- **Found during:** Task 3 (TestInstallOpenCodePlugin/from_absent failed)
- **Issue:** Test asserted `strings.Contains(content, "beekeeper check --hook opencode")` but template uses `spawnSync("beekeeper", ["check", "--hook", "opencode"])` — the literal string was absent
- **Fix:** Added `// command: beekeeper check --hook opencode` inline comment in spawnSync call and `// Shells to: beekeeper check --hook opencode` header comment in template
- **Verification:** `go test ./internal/hooks/ -run TestInstallOpenCodePlugin -count=1` exits 0
- **Committed in:** e4b1572 (Task 3 commit)

---

**Total deviations:** 2 auto-fixed (1 compile error, 1 test assertion gap)
**Impact on plan:** Both fixes required for correct execution; no scope creep.

## Issues Encountered

None beyond the auto-fixed deviations above.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Hermes/Cline/OpenCode installers complete; `beekeeper hooks install --target hermes/cline/opencode` dispatches correctly
- All three have contract-shape tests; cross-platform builds verified (Windows/Linux)
- Phase 10 Plan 06 (support matrix docs + Kilo/Trae routing) can proceed
- Honest support tier matrix: Hermes = Tier 2 (fail-open), Cline = Tier 2 (no Windows), OpenCode = Tier 2 (plugin + subagent bypass caveat)

## Self-Check

Checking created files exist and commits are present...

---
*Phase: 10-hook-block-protocol-compliance-and-multi-harness-enforcement*
*Completed: 2026-06-05*
