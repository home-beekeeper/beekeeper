package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// windsurfHooksFile is the top-level structure of ~/.codeium/windsurf/hooks.json.
// Windsurf uses a map from event name to an array of hook entries.
// Schema verified from docs.windsurf.com/windsurf/cascade/hooks.
type windsurfHooksFile struct {
	Hooks map[string][]windsurfHook `json:"hooks"`
}

// windsurfHook is a single hook entry in Windsurf's hooks.json.
// On Linux/macOS the Command field is used; on Windows the PowerShell field
// is used instead. Windsurf uses exit 2 ONLY to signal deny — there is no
// stdout-JSON deny form. RenderDeny(HarnessWindsurf) emits exit 2 with no
// stdout JSON accordingly.
type windsurfHook struct {
	Command    string `json:"command,omitempty"`    // Linux/macOS
	PowerShell string `json:"powershell,omitempty"` // Windows
	Timeout    int    `json:"timeout,omitempty"`
}

// windsurfEvents lists the Windsurf hook event names that Beekeeper installs
// into. Windsurf fires hooks for shell command execution, MCP tool calls, and
// file reads.
var windsurfEvents = []string{
	"pre_run_command",
	"pre_mcp_tool_use",
	"pre_read_code",
}

// windsurfHooksPath returns the path to Windsurf's hooks.json.
func windsurfHooksPath(homeDir string) string {
	return homeDir + "/.codeium/windsurf/hooks.json"
}

// beekeeperWindsurfHook returns the canonical Windsurf hook entry for the
// current OS. On Windows, Windsurf requires the "powershell" key; on
// Linux/macOS it uses "command". This distinction is critical: using the wrong
// key means the hook silently never executes on the primary platform.
func beekeeperWindsurfHook() windsurfHook {
	const cmd = "beekeeper check --hook windsurf"
	if runtime.GOOS == "windows" {
		return windsurfHook{
			PowerShell: cmd,
			Timeout:    10,
		}
	}
	return windsurfHook{
		Command: cmd,
		Timeout: 10,
	}
}

// containsWindsurfHookByCommand reports whether the given hook slice already
// contains a beekeeper entry (matched by either Command or PowerShell field).
// Used for idempotent install and targeted uninstall.
func containsWindsurfHookByCommand(hooks []windsurfHook, cmd string) bool {
	for _, h := range hooks {
		if h.Command == cmd || h.PowerShell == cmd {
			return true
		}
	}
	return false
}

// installWindsurf appends Beekeeper's hook to the three Windsurf pre-exec
// event arrays (pre_run_command, pre_mcp_tool_use, pre_read_code) in
// ~/.codeium/windsurf/hooks.json. The function is idempotent.
//
// CRITICAL: Windsurf's deny contract is exit 2 ONLY — there is no stdout-JSON
// deny form. RenderDeny(HarnessWindsurf) emits nil Stdout. The installer wires
// the command only; no deny JSON is configured.
//
// On Windows, the "powershell" key is set (not "command") because Windsurf
// uses that key to locate the executable on Windows.
//
// Existing hooks are preserved. A backup is created before modification.
func installWindsurf(hooksPath string, dryRun bool, out io.Writer) error {
	existing := windsurfHooksFile{
		Hooks: make(map[string][]windsurfHook),
	}

	if data, err := os.ReadFile(hooksPath); err == nil {
		// Tolerate parse errors — start fresh if the file is malformed.
		_ = json.Unmarshal(data, &existing)
		if existing.Hooks == nil {
			existing.Hooks = make(map[string][]windsurfHook)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("windsurf: read %q: %w", hooksPath, err)
	}

	const cmd = "beekeeper check --hook windsurf"
	hook := beekeeperWindsurfHook()

	// Append beekeeper hook to each of the three Windsurf events, skipping if
	// already present (idempotent).
	for _, event := range windsurfEvents {
		if !containsWindsurfHookByCommand(existing.Hooks[event], cmd) {
			existing.Hooks[event] = append(existing.Hooks[event], hook)
		}
	}

	if dryRun {
		data, _ := json.MarshalIndent(existing, "", "    ")
		fmt.Fprintf(out, "[dry-run] Would write to %s:\n%s\n", hooksPath, string(data))
		return nil
	}

	if err := backupSettings(hooksPath); err != nil {
		return err
	}

	// Ensure the parent directory exists (e.g., ~/.codeium/windsurf/ may not exist yet).
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0o755); err != nil {
		return fmt.Errorf("windsurf: create dir %q: %w", filepath.Dir(hooksPath), err)
	}

	data, err := json.MarshalIndent(existing, "", "    ")
	if err != nil {
		return fmt.Errorf("windsurf: marshal hooks: %w", err)
	}

	return writeFileAtomic(hooksPath, data)
}

// uninstallWindsurf removes the "beekeeper check --hook windsurf" entries from
// all three Windsurf event keys in ~/.codeium/windsurf/hooks.json.
// Other hooks are preserved. A backup is created before modification.
func uninstallWindsurf(hooksPath string, dryRun bool, out io.Writer) error {
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(out, "No Windsurf hooks.json found at %s — nothing to uninstall.\n", hooksPath)
			return nil
		}
		return fmt.Errorf("windsurf uninstall: read %q: %w", hooksPath, err)
	}

	var existing windsurfHooksFile
	if err := json.Unmarshal(data, &existing); err != nil {
		return fmt.Errorf("windsurf uninstall: parse %q: %w", hooksPath, err)
	}
	if existing.Hooks == nil {
		existing.Hooks = make(map[string][]windsurfHook)
	}

	const cmd = "beekeeper check --hook windsurf"
	totalRemoved := 0

	// Iterate all three Windsurf event keys, filtering beekeeper entries.
	for _, event := range windsurfEvents {
		arr := existing.Hooks[event]
		filtered := make([]windsurfHook, 0, len(arr))
		for _, h := range arr {
			if h.Command == cmd || h.PowerShell == cmd {
				totalRemoved++
				continue
			}
			filtered = append(filtered, h)
		}
		existing.Hooks[event] = filtered
	}

	if totalRemoved == 0 {
		fmt.Fprintf(out, "No beekeeper check hooks found in %s — nothing to uninstall.\n", hooksPath)
		return nil
	}

	if dryRun {
		fmt.Fprintf(out, "[dry-run] Would remove %d beekeeper check hook(s) from all events in %s\n", totalRemoved, hooksPath)
		return nil
	}

	if err := backupSettings(hooksPath); err != nil {
		return err
	}

	out2, err := json.MarshalIndent(existing, "", "    ")
	if err != nil {
		return fmt.Errorf("windsurf uninstall: marshal: %w", err)
	}

	return writeFileAtomic(hooksPath, out2)
}
