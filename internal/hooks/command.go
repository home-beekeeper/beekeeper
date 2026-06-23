package hooks

// command.go — shared absolute-binary-path command builder and stable-suffix
// matcher used by all harness installers.
//
// Fail-closed wiring fix (exit-127 elimination):
//
// Every agent-harness installer previously wrote the BARE command name
// "beekeeper" into the hook config (e.g. "beekeeper check --hook claude-code").
// When beekeeper is not on the PATH snapshot the harness captured at launch
// (e.g. installed while the agent is running, or PATH drift), the harness
// shell returns exit 127 ("command not found"). Exit 127 is NON-BLOCKING, so
// the agent runs UNPROTECTED until restart. This was observed live on the
// maintainer's machine.
//
// The fix: embed the running binary's absolute path (resolved via os.Executable
// at install time, filepath.ToSlash'd, double-quoted) so the harness shell
// can always resolve the hook regardless of PATH. A safe fallback to the bare
// name is used when os.Executable() returns an error.
//
// Detection, idempotency, migration, and uninstall all key off a STABLE
// INVARIANT SUFFIX ("check --hook <harness>" / "audit-record") rather than the
// full command string — so both old bare-name and new abspath forms are matched.

import (
	"os"
	"path/filepath"
	"strings"
)

// execResolver is the injectable seam for os.Executable.
// Tests substitute a stub; production code uses the real os.Executable.
// This package-level var is the documented test seam for command_test.go.
var execResolver = os.Executable

// resolveBeekeeperBin returns the quoted, forward-slashed absolute path of the
// running beekeeper binary. If the injectable resolver (execResolver) returns an
// error, it falls back to the bare literal "beekeeper" — the pre-existing
// behavior, which is strictly no worse than before this change (T-w7y-02).
//
// The returned token is ready to use as the first word of a shell command:
//   - Success path: `"C:/Users/x/beekeeper.exe"` (double-quoted, ToSlash)
//   - Fallback path: `beekeeper` (bare, unquoted — safe since no spaces)
//
// Quote + ToSlash prevents shell word-splitting via a path with spaces (T-w7y-01).
func resolveBeekeeperBin() string {
	execPath, err := execResolver()
	if err != nil {
		// Fallback: bare name. Never empty or garbage (T-w7y-02).
		return "beekeeper"
	}
	// Convert backslashes to forward slashes (cross-platform shell compatibility)
	// and wrap in double quotes to handle paths with spaces (T-w7y-01).
	slashed := filepath.ToSlash(execPath)
	return `"` + slashed + `"`
}

// beekeeperCmd returns the full hook command for the given argument string, built
// using the resolved absolute binary path.
//
// Example: beekeeperCmd("check --hook claude-code")
//   - Success: `"/home/user/beekeeper" check --hook claude-code`
//   - Fallback: `beekeeper check --hook claude-code`
//
// The stable suffix (the args string) is always a substring of the returned
// command, so BeekeeperHookMarkers() detection and matchesBeekeeperCommand still
// work on both old and new forms (T-w7y-04).
func beekeeperCmd(args string) string {
	return resolveBeekeeperBin() + " " + args
}

// matchesBeekeeperCommand reports whether cmd is a beekeeper hook command for
// the given stable suffix. It matches BOTH:
//   - Old bare-name form:  "beekeeper check --hook claude-code"
//   - New abspath form:    `"/path/to/beekeeper" check --hook claude-code`
//
// The match is ANCHORED on the beekeeper program token (the leading word/quoted
// path of the command), NOT on the suffix substring alone. This is the T-w7y-03
// requirement: a third-party command whose args happen to contain a weak suffix
// like "audit-record" (e.g. `audit-logger audit-record`) must NOT match — because
// mergeClaudeHookEntry REPLACES any matched entry during migration, an unanchored
// match would silently clobber the user's own hook.
//
// Anchoring algorithm:
//  1. Extract the leading program token: the double-quoted abspath if cmd starts
//     with a quote, otherwise the first whitespace-delimited word.
//  2. Require the program token's basename to be "beekeeper" (quotes/path/.exe
//     stripped, lowercased — mirrors hookguard.go programBase for consistency).
//  3. Require the remaining args (everything after the program token) to contain
//     the stable suffix.
//
// Callers pass the harness-specific stable suffix such as:
//   - "check --hook claude-code"
//   - "check --hook cursor"
//   - "audit-record"
func matchesBeekeeperCommand(cmd, suffix string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return false
	}
	var prog, rest string
	if cmd[0] == '"' {
		end := strings.IndexByte(cmd[1:], '"')
		if end < 0 {
			return false // unterminated quote
		}
		prog = cmd[1 : 1+end]
		rest = strings.TrimSpace(cmd[1+end+1:])
	} else if sp := strings.IndexAny(cmd, " \t"); sp >= 0 {
		prog = cmd[:sp]
		rest = strings.TrimSpace(cmd[sp+1:])
	} else {
		prog = cmd
		rest = ""
	}
	if !programIsBeekeeper(prog) {
		return false
	}
	return strings.Contains(rest, suffix)
}

// programIsBeekeeper reports whether a program token's basename is the beekeeper
// binary (surrounding quotes, directory path, and a .exe suffix stripped;
// lowercased). Mirrors internal/check/hookguard.go programBase so the installer
// and the self-protection guard agree on what "the beekeeper binary" means.
func programIsBeekeeper(prog string) bool {
	prog = strings.Trim(prog, `"'`)
	if idx := strings.LastIndexAny(prog, `/\`); idx >= 0 {
		prog = prog[idx+1:]
	}
	prog = strings.ToLower(prog)
	prog = strings.TrimSuffix(prog, ".exe")
	return prog == "beekeeper"
}
