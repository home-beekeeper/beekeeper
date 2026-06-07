package check

// Package check — hookguard.go
//
// Two self-defense guards layered onto `beekeeper check`:
//
//   - evaluateHookGuard: content-aware protection of Beekeeper's OWN hook entry
//     inside a (possibly shared) hook-config file. It blocks only an edit that
//     would remove/disable Beekeeper's entry; edits that keep it — and edits to
//     files that do not contain it (e.g. a GSD-only settings file) — pass. This
//     avoids the collateral damage of locking the whole shared file.
//
//   - evaluateCLIGuard: blocks the agent from invoking Beekeeper's MUTATING CLI
//     subcommands via Bash (config set, hooks install/uninstall, protect
//     install/uninstall), so the file protection can't be sidestepped through the
//     sanctioned CLI. Mutation stays human-only.

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/bantuson/beekeeper/internal/hooks"
	"github.com/bantuson/beekeeper/internal/policy"
)

const (
	ruleHookGuard = "beekeeper-hook-guard"
	ruleCLIGuard  = "beekeeper-cli-guard"

	hookGuardReason = "beekeeper self-protection: this change would remove or disable Beekeeper's own hook entry. " +
		"Manage Beekeeper's hook with `beekeeper hooks install`/`uninstall` in your own terminal, or edit the file directly. " +
		"(Other hooks in this file are unaffected.)"
	cliGuardReason = "beekeeper self-protection: invoking Beekeeper's mutating CLI via the agent is blocked. " +
		"Run `beekeeper` yourself in a terminal, or use `beekeeper dashboard --admin`."
)

func allowDecision(reason string) policy.Decision {
	return policy.Decision{Allow: true, Level: "allow", Reason: reason}
}

// evaluateHookGuard returns a block Decision when the tool call would strip
// Beekeeper's hook entry from a hook-config file; otherwise an allow Decision.
func evaluateHookGuard(tc policy.ToolCall) policy.Decision {
	allow := allowDecision("no hook-guard match")
	if tc.ToolInput == nil {
		return allow
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return allow // can't resolve hook files — don't over-block
	}
	// Lexically canonicalized hook-config paths (no EvalSymlinks → no per-call
	// stat storm; these are well-known literal paths). Keyed lower-case → real path.
	hookFiles := make(map[string]string)
	for _, hf := range hooks.HookConfigFiles(home) {
		hookFiles[strings.ToLower(lexicalPath(hf))] = hf
	}
	markers := hooks.BeekeeperHookMarkers()

	isWriteTool := false
	switch tc.ToolName {
	case "Write", "Edit", "MultiEdit", "NotebookEdit":
		isWriteTool = true
	}

	// --- File-tool path (Write/Edit/MultiEdit) ---
	if isWriteTool {
		target := stringField(tc.ToolInput, "file_path")
		if target == "" {
			target = stringField(tc.ToolInput, "path")
		}
		if hf := matchHookFile(target, hookFiles); hf != "" {
			current, rerr := os.ReadFile(hf)
			if rerr != nil || !containsAny(string(current), markers) {
				return allow // not installed here (or unreadable) — nothing of ours to protect
			}
			proposed, ok := proposedContent(tc, string(current))
			if !ok {
				// Result indeterminate (unknown write shape) → conservatively block a
				// write to a file that currently carries OUR entry.
				return hookBlock()
			}
			if !containsAny(proposed, markers) {
				return hookBlock() // the edit removes Beekeeper's entry
			}
			return allow // entry preserved — GSD/other hooks edit freely
		}
		return allow
	}

	// --- Bash path: a write/delete verb (or redirect) targeting a hook file that
	// currently contains Beekeeper's entry. Content isn't reliably inspectable, so
	// block conservatively — only for a file we're actually installed in. ---
	if tc.ToolName == "Bash" {
		cmd := stringField(tc.ToolInput, "command")
		if cmd == "" {
			return allow
		}
		for _, raw := range extractBashWriteTargets(cmd) {
			if hf := matchHookFile(raw, hookFiles); hf != "" {
				if current, rerr := os.ReadFile(hf); rerr == nil && containsAny(string(current), markers) {
					return hookBlock()
				}
			}
		}
	}
	return allow
}

// matchHookFile returns the real hook-file path when target resolves to one of
// the known hook-config files, else "". The target is canonicalized in BOTH the
// lexical and symlink-resolved forms (so a symlinked target still matches), each
// compared against the lexical hook-file set.
func matchHookFile(target string, hookFiles map[string]string) string {
	if target == "" {
		return ""
	}
	for _, form := range canonicalizePathForms(target) {
		if hf, ok := hookFiles[strings.ToLower(form)]; ok {
			return hf
		}
	}
	return ""
}

// proposedContent computes the file content that a write tool call would produce,
// given the current content. Returns ok=false when the result can't be determined.
func proposedContent(tc policy.ToolCall, current string) (string, bool) {
	switch tc.ToolName {
	case "Write":
		c, ok := tc.ToolInput["content"].(string)
		return c, ok
	case "Edit":
		oldS, _ := tc.ToolInput["old_string"].(string)
		newS, _ := tc.ToolInput["new_string"].(string)
		if oldS == "" {
			return "", false
		}
		return strings.ReplaceAll(current, oldS, newS), true
	case "MultiEdit":
		edits, ok := tc.ToolInput["edits"].([]any)
		if !ok {
			return "", false
		}
		out := current
		for _, e := range edits {
			m, ok := e.(map[string]any)
			if !ok {
				return "", false
			}
			oldS, _ := m["old_string"].(string)
			newS, _ := m["new_string"].(string)
			if oldS == "" {
				return "", false
			}
			out = strings.ReplaceAll(out, oldS, newS)
		}
		return out, true
	}
	return "", false // unknown write shape
}

func hookBlock() policy.Decision {
	return policy.Decision{Allow: false, Level: "block", Reason: hookGuardReason, RuleIDs: []string{ruleHookGuard}}
}

// lexicalPath canonicalizes p WITHOUT EvalSymlinks (no filesystem stat): env-var
// + tilde expansion, Abs, ToSlash. Used to canonicalize the known hook-file paths
// cheaply on the hot path.
func lexicalPath(p string) string {
	p = expandWinEnvVars(p)
	p = expandHome(p)
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	return filepath.ToSlash(p)
}

func stringField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// --- CLI-mutation guard ---

// mutatingCLISubcommands are the two-word Beekeeper subcommand sequences that
// change protected state. Invoking any via the agent's shell is blocked.
var mutatingCLISubcommands = [][2]string{
	{"config", "set"},
	{"hooks", "install"},
	{"hooks", "uninstall"},
	{"protect", "install"},
	{"protect", "uninstall"},
}

// evaluateCLIGuard blocks a Bash command that invokes a mutating Beekeeper
// subcommand; otherwise allow.
func evaluateCLIGuard(tc policy.ToolCall) policy.Decision {
	allow := allowDecision("no cli-guard match")
	if tc.ToolName != "Bash" || tc.ToolInput == nil {
		return allow
	}
	cmd := stringField(tc.ToolInput, "command")
	if cmd == "" {
		return allow
	}
	if commandInvokesMutatingBeekeeper(cmd) {
		return policy.Decision{Allow: false, Level: "block", Reason: cliGuardReason, RuleIDs: []string{ruleCLIGuard}}
	}
	return allow
}

func commandInvokesMutatingBeekeeper(cmd string) bool {
	for _, toks := range tokenizeShellSegments(cmd) {
		// Skip leading env assignments (FOO=bar) to find the program token.
		i := 0
		for i < len(toks) && strings.Contains(toks[i], "=") && !strings.HasPrefix(toks[i], "-") {
			i++
		}
		if i >= len(toks) || !isBeekeeperProgram(toks[i]) {
			continue
		}
		// Collect the first two non-flag subcommand words.
		var sub []string
		for _, t := range toks[i+1:] {
			if strings.HasPrefix(t, "-") {
				continue
			}
			sub = append(sub, t)
			if len(sub) == 2 {
				break
			}
		}
		if len(sub) < 2 {
			continue
		}
		for _, pat := range mutatingCLISubcommands {
			if sub[0] == pat[0] && sub[1] == pat[1] {
				return true
			}
		}
	}
	return false
}

func isBeekeeperProgram(tok string) bool {
	tok = strings.Trim(tok, `"'`)
	if idx := strings.LastIndexAny(tok, `/\`); idx >= 0 {
		tok = tok[idx+1:]
	}
	tok = strings.ToLower(tok)
	return tok == "beekeeper" || tok == "beekeeper.exe"
}

// tokenizeShellSegments splits a command into segments on unquoted ; | & and
// newlines, returning each segment's whitespace-separated tokens (quotes stripped).
// Quote-aware so a separator or the literal phrase "beekeeper config set" INSIDE
// quotes (e.g. a commit message) is one token and never treated as a program
// invocation — avoiding false-positive blocks.
func tokenizeShellSegments(cmd string) [][]string {
	var segs [][]string
	var cur []string
	var b strings.Builder
	flush := func() {
		if b.Len() > 0 {
			cur = append(cur, b.String())
			b.Reset()
		}
	}
	endSeg := func() {
		flush()
		if len(cur) > 0 {
			segs = append(segs, cur)
			cur = nil
		}
	}
	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		switch {
		case c == '"' || c == '\'':
			q := c
			i++
			for i < len(cmd) && cmd[i] != q {
				b.WriteByte(cmd[i])
				i++
			}
			// i lands on the closing quote (or EOF); loop's i++ advances past it.
		case c == ' ' || c == '\t':
			flush()
		case c == ';' || c == '\n' || c == '\r':
			endSeg()
		case c == '|' || c == '&':
			endSeg()
			if i+1 < len(cmd) && (cmd[i+1] == '|' || cmd[i+1] == '&') {
				i++ // consume the second byte of || or &&
			}
		default:
			b.WriteByte(c)
		}
	}
	endSeg()
	return segs
}
