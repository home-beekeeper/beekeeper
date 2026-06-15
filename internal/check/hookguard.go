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
			// Finding #6: it is NOT enough that the marker survives — an agent can
			// keep the marker substring while NEUTERING enforcement (appending
			// `|| true`, `; exit 0`, a `#` comment, wrapping the command, or
			// repointing `command` to a stub while the marker lingers elsewhere).
			// Require that EVERY marker-bearing invocation present in the current
			// file survives byte-for-byte (delimiters and all) in the proposed
			// content. If any is removed OR altered, block. Conservative bias: any
			// inability to confirm the invocation is intact blocks.
			if !markerInvocationsIntact(string(current), proposed, markers) {
				return hookBlock()
			}
			return allow // entry preserved verbatim — GSD/other hooks edit freely
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

// markerInvocationsIntact reports whether EVERY marker-bearing command
// invocation in `current` survives byte-for-byte in `proposed` (finding #6).
//
// Rather than checking only that the marker SUBSTRING is still somewhere in the
// file (which an agent defeats by keeping the marker while neutering the command
// — `beekeeper check ... || true`, `... ; exit 0`, `echo beekeeper check # off`,
// or repointing `command` to a stub while the marker lingers in a comment), we
// extract the DELIMITED invocation token around each marker (the quoted JSON
// string value, or the trimmed line for shell-style configs) and require it to
// appear verbatim in the proposed content. A delimited token includes its
// boundaries, so appending `|| true` inside the quotes makes the original
// `"beekeeper check --hook claude-code"` token no longer present → block, while
// appending unrelated JSON AFTER the file (which leaves the token untouched)
// keeps it present → allow.
//
// Conservative bias: if `current` carries a marker but no invocation token can
// be isolated, OR any isolated token is missing/altered in `proposed`, return
// false (block).
func markerInvocationsIntact(current, proposed string, markers []string) bool {
	invocations := extractMarkerInvocations(current, markers)
	if len(invocations) == 0 {
		// current contains a marker (caller already checked) but we could not
		// isolate the invocation — fail closed.
		return false
	}
	for _, inv := range invocations {
		if !strings.Contains(proposed, inv) {
			return false
		}
	}
	return true
}

// extractMarkerInvocations returns the delimited invocation token surrounding
// every marker occurrence in content. For a marker inside a JSON/quoted string
// (the common harness case) the token is the full quoted value INCLUDING its
// surrounding quotes, so the boundaries are part of the comparison. For a marker
// on a bare (shell-style) line the token is the trimmed line. Tokens are
// de-duplicated, order-preserved.
func extractMarkerInvocations(content string, markers []string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(tok string) {
		if tok == "" || seen[tok] {
			return
		}
		seen[tok] = true
		out = append(out, tok)
	}
	for _, marker := range markers {
		from := 0
		for {
			rel := strings.Index(content[from:], marker)
			if rel < 0 {
				break
			}
			idx := from + rel
			add(invocationTokenAt(content, idx, len(marker)))
			from = idx + len(marker)
		}
	}
	return out
}

// invocationTokenAt isolates the invocation token that contains the marker found
// at content[idx:idx+markerLen]. If the marker sits inside a double-quoted JSON
// string, the token is the entire quoted value (quotes included). Otherwise the
// token is the trimmed line containing the marker.
func invocationTokenAt(content string, idx, markerLen int) string {
	// Find the nearest unescaped double-quote to the LEFT of the marker on the
	// same line, and the nearest unescaped double-quote to the RIGHT. If both
	// exist on the same line, treat the marker as a quoted JSON value.
	lineStart := strings.LastIndexByte(content[:idx], '\n') + 1 // 0 if none
	lineEndRel := strings.IndexByte(content[idx:], '\n')
	lineEnd := len(content)
	if lineEndRel >= 0 {
		lineEnd = idx + lineEndRel
	}

	left := lastUnescapedQuote(content[lineStart:idx])
	if left >= 0 {
		left += lineStart // absolute index of the opening quote
		rightRel := firstUnescapedQuote(content[idx+markerLen : lineEnd])
		if rightRel >= 0 {
			right := idx + markerLen + rightRel // absolute index of the closing quote
			// Token spans from the opening quote through the closing quote inclusive.
			return content[left : right+1]
		}
	}

	// Not a clean quoted value — fall back to the trimmed line, which still
	// captures appended `|| true`, `; exit 0`, or `# comment` neutering.
	return strings.TrimSpace(content[lineStart:lineEnd])
}

// lastUnescapedQuote returns the byte index of the last unescaped double-quote
// in s, or -1 if none. A quote preceded by an odd number of backslashes is
// considered escaped.
func lastUnescapedQuote(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] != '"' {
			continue
		}
		if !isEscaped(s, i) {
			return i
		}
	}
	return -1
}

// firstUnescapedQuote returns the byte index of the first unescaped double-quote
// in s, or -1 if none.
func firstUnescapedQuote(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] != '"' {
			continue
		}
		if !isEscaped(s, i) {
			return i
		}
	}
	return -1
}

// isEscaped reports whether the byte at s[i] is preceded by an odd number of
// backslashes (and is therefore escaped).
func isEscaped(s string, i int) bool {
	bs := 0
	for j := i - 1; j >= 0 && s[j] == '\\'; j-- {
		bs++
	}
	return bs%2 == 1
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

// maxCLIGuardRecursion bounds the recursion through nested shell strings
// (sh -c "... sh -c '...'") so a pathological command can never blow the stack.
const maxCLIGuardRecursion = 8

// commandInvokesMutatingBeekeeper reports whether cmd executes a mutating
// Beekeeper subcommand anywhere in the UNQUOTED command stream. It is hardened
// against the indirection forms an agent uses to dodge a literal first-token
// scan (finding #7):
//
//   - quoted shell scripts run via `sh -c "..."`, `bash -lc "..."` (recurse into
//     the script argument);
//   - command-substitution programs: `$(which beekeeper) hooks uninstall`,
//     `` `which beekeeper` hooks uninstall `` (the resolved program is beekeeper);
//   - `env beekeeper hooks uninstall` (skip the env wrapper);
//   - variable indirection: `BK=beekeeper; $BK hooks uninstall` (track simple
//     NAME=beekeeper assignments and resolve $NAME / ${NAME} back to beekeeper).
//
// Quote-awareness is preserved so a mutating phrase that appears only as literal
// prose inside a quoted argument that is NOT a shell-script operand (e.g. a git
// commit message) does not false-positive: tokenizeShellSegments keeps such a
// phrase as a single opaque token, and only the recognized script-bearing flags
// (-c/-lc/...) reopen a quoted argument for execution analysis.
func commandInvokesMutatingBeekeeper(cmd string) bool {
	return cmdInvokesMutatingBeekeeperDepth(cmd, nil, 0)
}

// cmdInvokesMutatingBeekeeperDepth carries an env map (NAME→beekeeper-ness) and a
// recursion depth across nested shell-string analysis.
func cmdInvokesMutatingBeekeeperDepth(cmd string, env map[string]bool, depth int) bool {
	if depth > maxCLIGuardRecursion {
		// Conservative: a command nested deeper than we will follow is treated as
		// a potential dodge and blocked. (Real commands never nest this deep.)
		return true
	}
	if env == nil {
		env = map[string]bool{}
	}
	for _, toks := range tokenizeShellSegments(cmd) {
		// Skip leading env assignments (FOO=bar). Record simple NAME=beekeeper
		// bindings so a later `$NAME` resolves back to the program (finding #7:
		// `BK=beekeeper; $BK hooks uninstall`).
		i := 0
		for i < len(toks) {
			name, val, ok := splitAssignment(toks[i])
			if !ok {
				break
			}
			if isBeekeeperToken(val) {
				env[name] = true
			} else if _, computed := env[name]; computed {
				// Reassigned to something else — drop the binding.
				delete(env, name)
			}
			i++
		}
		if i >= len(toks) {
			continue
		}

		// Strip a leading `env [VAR=val...]` wrapper so `env beekeeper ...` and
		// `env FOO=1 beekeeper ...` are seen through (finding #7).
		i = skipEnvWrapper(toks, i, env)
		if i >= len(toks) {
			continue
		}

		prog := toks[i]

		// A program that is itself a sub-shell (`sh -c "..."`, `bash -lc '...'`)
		// re-runs an embedded script: recurse into the script operand. The
		// embedded script can ALSO carry a mutating beekeeper call, so analysing
		// only the literal `sh` token would miss it.
		if isShellProgram(prog) {
			if script, found := shellScriptOperand(toks[i+1:]); found {
				if cmdInvokesMutatingBeekeeperDepth(script, copyEnv(env), depth+1) {
					return true
				}
			}
			continue
		}

		if !isBeekeeperProgram(prog, env) {
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

// splitAssignment parses a `NAME=value` shell assignment token. It returns
// ok=false for anything that is not a leading variable assignment (a flag, a
// bare word, or a token whose name part is not a valid shell identifier).
func splitAssignment(tok string) (name, val string, ok bool) {
	if tok == "" || strings.HasPrefix(tok, "-") {
		return "", "", false
	}
	eq := strings.IndexByte(tok, '=')
	if eq <= 0 {
		return "", "", false
	}
	name = tok[:eq]
	for i := 0; i < len(name); i++ {
		if !isEnvNameChar(name[i]) {
			return "", "", false
		}
	}
	if !isEnvNameStart(name[0]) {
		return "", "", false
	}
	return name, tok[eq+1:], true
}

// skipEnvWrapper advances past a leading `env` program and any `VAR=val`
// operands that follow it, so the real program token after `env` is analysed.
// Assignments threaded through env are also recorded in the env map.
func skipEnvWrapper(toks []string, i int, env map[string]bool) int {
	if i >= len(toks) || !programBaseEquals(toks[i], "env") {
		return i
	}
	i++ // consume `env`
	for i < len(toks) {
		if name, val, ok := splitAssignment(toks[i]); ok {
			if isBeekeeperToken(val) {
				env[name] = true
			}
			i++
			continue
		}
		if strings.HasPrefix(toks[i], "-") {
			// env flag (e.g. -i, -u NAME): skip it (and a following operand for -u).
			i++
			continue
		}
		break
	}
	return i
}

// shellScriptOperand finds the script-string operand of a `sh -c`/`bash -lc`
// style invocation among the tokens AFTER the shell program. It returns the
// first non-flag token that follows a `-c`-bearing flag (covering -c, -lc, -ic,
// and long --command forms), or, failing an explicit flag, the first non-flag
// token (login shells often glue the script after combined flags).
func shellScriptOperand(args []string) (string, bool) {
	sawCFlag := false
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			if flagHasCommand(a) {
				sawCFlag = true
			}
			continue
		}
		if sawCFlag {
			return a, true
		}
		// First positional operand with no preceding -c flag is the script for
		// shells invoked as `sh "<script>"` only in odd forms; be conservative
		// and still analyse it.
		return a, true
	}
	return "", false
}

// flagHasCommand reports whether a shell flag token requests command execution,
// i.e. it is `--command` or a short-flag cluster containing 'c' (e.g. -c, -lc,
// -ic). This is what reopens a quoted operand as an executable script.
func flagHasCommand(flag string) bool {
	if flag == "--command" || strings.HasPrefix(flag, "--command=") {
		return true
	}
	if strings.HasPrefix(flag, "--") {
		return false
	}
	return strings.ContainsRune(flag[1:], 'c')
}

// isShellProgram reports whether the program token is a POSIX-ish shell that
// runs a script operand (sh, bash, dash, zsh, ksh), after stripping any path,
// quotes, and command-substitution wrapper.
func isShellProgram(tok string) bool {
	base := programBase(tok)
	switch base {
	case "sh", "bash", "dash", "zsh", "ksh":
		return true
	}
	return false
}

// isBeekeeperProgram reports whether the program token resolves to the beekeeper
// binary, accounting for path prefixes, quotes, command substitution
// (`$(which beekeeper)`, `` `which beekeeper` ``), and variable indirection
// (`$BK` where BK=beekeeper was previously assigned).
func isBeekeeperProgram(tok string, env map[string]bool) bool {
	if isBeekeeperToken(tok) {
		return true
	}
	// Variable indirection: $NAME / ${NAME} where NAME was bound to beekeeper.
	if name, ok := plainVarRef(tok); ok {
		if env[name] {
			return true
		}
	}
	// Command substitution: $(which beekeeper) / `which beekeeper`. The resolved
	// program is whatever the inner command would print; we treat an inner
	// command that locates "beekeeper" (which/command -v/type/whereis beekeeper,
	// or a literal path ending in beekeeper) as resolving to the binary.
	if inner, ok := commandSubstitutionBody(tok); ok {
		if substitutionResolvesBeekeeper(inner) {
			return true
		}
	}
	return false
}

// isBeekeeperToken reports whether a bare program token (after stripping path,
// quotes, and a .exe suffix) is the beekeeper binary.
func isBeekeeperToken(tok string) bool {
	return programBaseEquals(tok, "beekeeper")
}

// programBase returns the lowercased basename of a program token with surrounding
// quotes and any .exe suffix stripped.
func programBase(tok string) string {
	tok = strings.Trim(tok, `"'`)
	if idx := strings.LastIndexAny(tok, `/\`); idx >= 0 {
		tok = tok[idx+1:]
	}
	tok = strings.ToLower(tok)
	return strings.TrimSuffix(tok, ".exe")
}

// programBaseEquals reports whether the program token's basename equals name.
func programBaseEquals(tok, name string) bool {
	return programBase(tok) == name
}

// plainVarRef returns the variable name of a token that is exactly `$NAME` or
// `${NAME}` (nothing else), used to resolve `$BK` indirection.
func plainVarRef(tok string) (string, bool) {
	tok = strings.Trim(tok, `"'`)
	if len(tok) < 2 || tok[0] != '$' {
		return "", false
	}
	body := tok[1:]
	if len(body) >= 2 && body[0] == '{' && body[len(body)-1] == '}' {
		body = body[1 : len(body)-1]
	}
	if body == "" {
		return "", false
	}
	if !isEnvNameStart(body[0]) {
		return "", false
	}
	for i := 0; i < len(body); i++ {
		if !isEnvNameChar(body[i]) {
			return "", false
		}
	}
	return body, true
}

// commandSubstitutionBody returns the inner command of a token that is exactly a
// command substitution: `$(...)` or `` `...` `` (optionally surrounded by
// quotes). Returns ok=false otherwise.
func commandSubstitutionBody(tok string) (string, bool) {
	tok = strings.TrimSpace(strings.Trim(tok, `"'`))
	if strings.HasPrefix(tok, "$(") && strings.HasSuffix(tok, ")") {
		return tok[2 : len(tok)-1], true
	}
	if len(tok) >= 2 && tok[0] == '`' && tok[len(tok)-1] == '`' {
		return tok[1 : len(tok)-1], true
	}
	return "", false
}

// substitutionResolvesBeekeeper reports whether an inner command-substitution
// body resolves to the beekeeper binary. It treats path-locator commands
// (which/command -v/type/whereis) whose argument is beekeeper, and a bare path
// literal ending in beekeeper, as resolving to the binary.
func substitutionResolvesBeekeeper(inner string) bool {
	fields := strings.Fields(inner)
	if len(fields) == 0 {
		return false
	}
	switch programBase(fields[0]) {
	case "which", "whereis", "type":
		for _, f := range fields[1:] {
			if strings.HasPrefix(f, "-") {
				continue
			}
			return isBeekeeperToken(f)
		}
	case "command":
		for _, f := range fields[1:] {
			if strings.HasPrefix(f, "-") { // skip -v
				continue
			}
			return isBeekeeperToken(f)
		}
	}
	// A bare literal path inside the substitution (rare) — e.g. $(echo beekeeper).
	if programBase(fields[0]) == "echo" && len(fields) >= 2 {
		return isBeekeeperToken(fields[1])
	}
	return isBeekeeperToken(fields[0])
}

// copyEnv returns a shallow copy of the env map so a recursive call cannot
// mutate the caller's bindings.
func copyEnv(env map[string]bool) map[string]bool {
	out := make(map[string]bool, len(env))
	for k, v := range env {
		out[k] = v
	}
	return out
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
		case c == '$' && i+1 < len(cmd) && cmd[i+1] == '(':
			// Command substitution `$( ... )` is captured VERBATIM (including the
			// inner spaces and the wrapping $()) as ONE token so that
			// `$(which beekeeper) hooks uninstall` keeps the substitution intact
			// for isBeekeeperProgram to resolve (finding #7). Nested parens are
			// balanced so `$(dirname $(which x))` does not terminate early.
			b.WriteByte(c)   // '$'
			b.WriteByte('(') // '('
			i += 2
			depth := 1
			for i < len(cmd) && depth > 0 {
				switch cmd[i] {
				case '(':
					depth++
				case ')':
					depth--
				}
				if depth == 0 {
					break
				}
				b.WriteByte(cmd[i])
				i++
			}
			if i < len(cmd) {
				b.WriteByte(')') // closing ')'
			}
		case c == '`':
			// Backtick command substitution captured verbatim as one token, so
			// `` `which beekeeper` hooks uninstall `` survives tokenization.
			b.WriteByte('`')
			i++
			for i < len(cmd) && cmd[i] != '`' {
				b.WriteByte(cmd[i])
				i++
			}
			if i < len(cmd) {
				b.WriteByte('`') // closing backtick
			}
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
