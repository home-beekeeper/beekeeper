// Package check — impure path adapter for SPATH-01/02/03/04.
//
// This file contains the I/O-bearing path functions that extract, canonicalize,
// and merge path-based policy decisions. All filesystem operations (os.UserHomeDir,
// filepath.Abs, filepath.EvalSymlinks) and environment reads (os.Getenv) live HERE,
// never in the pure internal/policy package (enforced by TestPathImportsArePure).
//
// The pure decision itself is made by policy.EvaluatePath; these functions are
// the impure adapter that feeds it already-resolved strings.
package check

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/bantuson/beekeeper/internal/policy"
)

// expandHome replaces a leading "~" with the current user's home directory.
// Copied verbatim from internal/watch/watcher.go:121-132 — we copy rather than
// import to avoid pulling fsnotify into internal/check.
// If os.UserHomeDir returns an error the original path is returned unchanged
// (fail-safe: a non-resolved tilde path still flows through canonicalizePath).
func expandHome(dir string) string {
	if len(dir) == 0 || dir[0] != '~' {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return dir
	}
	return filepath.Join(home, dir[1:])
}

// expandWinEnvVars performs a targeted %VAR% → os.Getenv(VAR) replacement for
// Windows environment variable forms in path strings (D-01).
//
// We handle at minimum %USERPROFILE% and %HOMEPATH% (case-insensitive on the
// variable name). We do NOT use os.ExpandEnv — that only handles $VAR / ${VAR}
// Unix-style variables. This is a matching-only expansion: the string is never
// passed to a shell or executed.
//
// Fail-closed: if the environment variable resolves to an empty string (unset),
// the raw %VAR% token is kept in place so a real credential path substring can
// still be detected. We never silently drop to a shorter path that would allow
// the operation.
//
// Deferred bypasses: nested variable expansion, computed variable names, and
// cmd.exe delayed expansion (%%) are out of scope for Phase 7.
func expandWinEnvVars(raw string) string {
	// Single left-to-right pass: scan for %...% sequences and append resolved
	// values (or the original %VAR% literal for unresolved vars) to a Builder.
	// Because we advance the cursor past each emitted value, substituted content
	// is NEVER re-scanned — no re-expansion, no in-band sentinel needed (WR-01/WR-02).
	// We do not use os.ExpandEnv (handles $VAR/${VAR} only, not %VAR%).
	var b strings.Builder
	rest := raw
	for {
		start := strings.Index(rest, "%")
		if start == -1 {
			b.WriteString(rest)
			break
		}
		// Append everything before the opening %.
		b.WriteString(rest[:start])
		rest = rest[start+1:] // rest now starts after the opening %

		end := strings.Index(rest, "%")
		if end == -1 {
			// No closing % — treat the trailing % as a literal and stop.
			b.WriteByte('%')
			b.WriteString(rest)
			break
		}

		varName := rest[:end]
		rest = rest[end+1:] // advance past the closing %

		if varName == "" {
			// "%%" — write a single literal percent.
			b.WriteByte('%')
			continue
		}

		val := os.Getenv(varName)
		if val == "" {
			// Also try upper-cased version: Windows env var names are
			// conventionally UPPER_CASE but user input may be mixed-case.
			val = os.Getenv(strings.ToUpper(varName))
		}

		if val == "" {
			// Fail-closed: unresolved var — preserve the raw %VAR% token so
			// that a real credential path substring can still be detected.
			// The literal %VAR% is appended to the builder and the cursor has
			// already moved past it, so it is treated as an opaque literal and
			// never re-scanned.
			b.WriteByte('%')
			b.WriteString(varName)
			b.WriteByte('%')
			continue
		}

		// Resolved: append the value. The cursor is already past the closing %
		// so the value content is never re-scanned.
		b.WriteString(val)
	}
	return b.String()
}

// canonicalizePath resolves a raw path string into a normalized, absolute,
// forward-slash path suitable for policy.EvaluatePath matching.
//
// Sequence (Q3 / D-01):
//  1. expandWinEnvVars — targeted %VAR% → os.Getenv replacement (D-01, SPATH-02).
//     Must run FIRST so that %USERPROFILE%\.ssh\id_rsa is expanded before tilde
//     handling and filepath.Abs run.
//  2. expandHome — tilde → os.UserHomeDir + filepath.Join (SPATH-02).
//  3. filepath.Abs — resolves ".." traversal and relative paths (SPATH-02,
//     T-07-05). Does not require the file to exist.
//  4. filepath.EvalSymlinks — resolves symlinks. IMPORTANT (Pitfall 3): EvalSymlinks
//     returns an error when the target does not exist. On error, fall back to the
//     filepath.Abs result — NOT the raw input. This ensures that a non-existent
//     credential path (~/.aws/credentials before first AWS use) is still evaluated
//     against the block patterns and fails closed.
//  5. filepath.ToSlash — normalize all OS path separators to forward slashes for
//     cross-platform pattern matching (T-07-09).
//
// Returns "" only when raw is "" (caller should skip empty strings).
//
// The command string passed to extractBashCredentialPaths is NEVER executed here;
// expansion is for string matching only (T-07-10, D-01 rationale).
func canonicalizePath(raw string) string {
	if raw == "" {
		return ""
	}

	// Step 1: Windows %VAR% expansion (D-01). Must precede expandHome/Abs.
	p := expandWinEnvVars(raw)

	// Step 2: tilde expansion.
	p = expandHome(p)

	// Step 3: resolve traversal / relative paths.
	abs, err := filepath.Abs(p)
	if err != nil {
		// Abs fails only on very degenerate inputs; use the expandHome result.
		abs = p
	}

	// Step 4: resolve symlinks. Fall back to Abs result on any error (Pitfall 3).
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// Do NOT fall back to raw — use the Abs-resolved path so that
		// non-existent credential paths (e.g. ~/.aws/credentials) still
		// contain the credential fragment and EvaluatePath blocks them.
		resolved = abs
	}

	// Step 5: normalize to forward slashes.
	return filepath.ToSlash(resolved)
}

// canonicalizePathForms returns BOTH path forms that downstream SPATH matching
// must consider, de-duplicated (empty strings dropped, exact duplicates dropped):
//
//  1. The lexically-cleaned form: expandWinEnvVars -> expandHome -> filepath.Abs
//     -> filepath.ToSlash, WITHOUT EvalSymlinks. This preserves the textual
//     sensitive fragment (e.g. /.aws/, /.ssh/) even when an ANCESTOR directory
//     is a symlink.
//  2. The EvalSymlinks-resolved form: the current canonicalizePath output.
//
// HARDEN-01 (IN-01): canonicalizePath alone applies filepath.EvalSymlinks, which
// resolves ancestor-directory symlinks. An attacker can plant a symlink ancestor
// (link -> /tmp/realdir) and request link/.aws/credentials; EvalSymlinks rewrites
// the path to /tmp/realdir/.aws/credentials — which may no longer carry a /.aws/
// fragment in a matchable shape if the real layout differs, dodging the blocklist.
// Returning the lexically-cleaned form too means a downstream block on EITHER form
// blocks: the sensitive fragment survives normalization on the lexical form.
//
// Fail-closed is preserved: the EvalSymlinks fallback-to-Abs behavior inside
// canonicalizePath (Pitfall 3) is unchanged, so a non-existent credential path
// still yields a form containing the credential fragment.
//
// CONSUMER WIRING: the handler.go and integration_test.go SPATH loops are owned
// by Plan 03, which replaces their single canonicalizePath call with a loop over
// canonicalizePathForms (block on any form). This plan delivers and unit-proves
// the helper; canonicalizePath stays unchanged for existing callers.
func canonicalizePathForms(raw string) []string {
	if raw == "" {
		return nil
	}

	// Lexically-cleaned form: same prefix as canonicalizePath but WITHOUT
	// EvalSymlinks, so an ancestor symlink cannot strip the sensitive fragment.
	p := expandWinEnvVars(raw)
	p = expandHome(p)
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	lexical := filepath.ToSlash(abs)

	// Symlink-resolved form: the current canonicalizePath output (unchanged).
	resolved := canonicalizePath(raw)

	// De-duplicate: drop empties and exact duplicates while preserving order
	// (lexical first, then resolved).
	var forms []string
	for _, f := range []string{lexical, resolved} {
		if f == "" {
			continue
		}
		dup := false
		for _, existing := range forms {
			if existing == f {
				dup = true
				break
			}
		}
		if !dup {
			forms = append(forms, f)
		}
	}
	return forms
}

// bashReadVerbs is the conservative allowlist of read-command prefixes that
// indicate a file-path argument follows. Space is included in each verb so that
// "type" inside prose is not accidentally matched.
//
// Deferred bypasses (07-CONTEXT.md Deferred Ideas): nested shells ("zsh -c ..."),
// base64-encoded commands, here-strings, and compound redirections are NOT in scope
// for SPATH-03 Phase 7. %USERPROFILE% expansion in the extracted token is handled
// downstream in canonicalizePath (D-01) — the raw token is returned verbatim here.
var bashReadVerbs = []string{
	"cat ",         // Unix read
	"head ",        // Unix read
	"tail ",        // Unix read
	"less ",        // Unix pager
	"more ",        // Unix pager
	"type ",        // Windows CMD read verb (SPATH-03, SC2)
	"Get-Content ", // PowerShell (SPATH-03)
	"gc ",          // PowerShell alias for Get-Content (SPATH-03)
}

// firstShellToken extracts the first non-whitespace token from rest.
// It strips surrounding single or double quotes and stops at the first
// unquoted whitespace character.
//
// Examples:
//
//	firstShellToken(`  "~/.aws/credentials" extra`) == "~/.aws/credentials"
//	firstShellToken(`  ~/.ssh/id_rsa`) == "~/.ssh/id_rsa"
//	firstShellToken(`  '~/.npmrc'`) == "~/.npmrc"
func firstShellToken(rest string) string {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return ""
	}

	// Handle quoted tokens.
	if len(rest) > 0 && (rest[0] == '"' || rest[0] == '\'') {
		quote := rest[0]
		end := strings.IndexByte(rest[1:], quote)
		if end >= 0 {
			return rest[1 : end+1]
		}
		// Unclosed quote — treat the remainder as the token.
		return rest[1:]
	}

	// Unquoted: stop at first whitespace.
	end := strings.IndexAny(rest, " \t\n\r")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

// extractBashCredentialPaths scans a Bash command string for read-verb prefixes
// and returns the path tokens that follow each match.
//
// This is conservative verb-prefix matching, NOT a full shell tokenizer.
// All occurrences of each verb are found (not just the first), so chained
// commands like "cat /tmp/banner.txt && cat ~/.ssh/id_rsa" extract BOTH
// paths (CR-01 fix). Leading flag tokens (starting with "-") are skipped
// so "cat -n ~/.ssh/id_rsa" still extracts "~/.ssh/id_rsa" (CR-01 fix).
//
// The extracted path tokens are returned verbatim — %USERPROFILE% or other
// env-var forms are NOT expanded here. Expansion happens in canonicalizePath
// (D-01), so `type %USERPROFILE%\.ssh\id_rsa` returns the raw token and
// canonicalizePath later resolves it to a path containing "/.ssh/".
//
// Returns nil when no read verb is found or when all tokens after verbs are empty.
func extractBashCredentialPaths(cmd string) []string {
	var paths []string
	for _, verb := range bashReadVerbs {
		// Scan ALL occurrences of this verb (not just the first) so that
		// chained commands like "cat safe.txt && cat ~/.ssh/id_rsa" extract
		// every credential path read by the command (CR-01).
		from := 0
		for {
			rel := strings.Index(cmd[from:], verb)
			if rel == -1 {
				break
			}
			idx := from + rel
			rest := strings.TrimSpace(cmd[idx+len(verb):])

			// Skip leading flag tokens (e.g. "-n", "-c 100") so that
			// "cat -n ~/.ssh/id_rsa" reaches "~/.ssh/id_rsa" (CR-01).
			for {
				tok := firstShellToken(rest)
				if tok == "" {
					break
				}
				if !strings.HasPrefix(tok, "-") {
					paths = append(paths, tok)
					break
				}
				// Advance rest past this flag token.
				rest = strings.TrimSpace(rest[len(tok):])
			}

			from = idx + len(verb)
		}
	}
	return paths
}

// extractPathTargets extracts all file-path strings from a ToolCall that should
// be evaluated by policy.EvaluatePath.
//
// Sources (in order):
//  1. ToolInput["file_path"] — primary key for Read/Write/Edit/MultiEdit (Claude Code)
//  2. ToolInput["path"] — legacy key used by policyloader overlay; kept for compat
//  3. Bash "command" value — scanned by extractBashCredentialPaths for read-verb targets (SPATH-03)
//
// Returns nil (not an empty slice) when no path targets are found.
// Never panics on a nil ToolInput.
func extractPathTargets(tc policy.ToolCall) []string {
	if tc.ToolInput == nil {
		return nil
	}

	var paths []string

	// file_path: Read/Write/Edit/MultiEdit tools (Claude Code).
	if p, ok := tc.ToolInput["file_path"].(string); ok && p != "" {
		paths = append(paths, p)
	}

	// path: legacy key used by policyloader overlay; keep for backward compat.
	if p, ok := tc.ToolInput["path"].(string); ok && p != "" {
		paths = append(paths, p)
	}

	// Bash command: scan for credential-read verb patterns (SPATH-03).
	if tc.ToolName == "Bash" {
		if cmd, ok := tc.ToolInput["command"].(string); ok && cmd != "" {
			paths = append(paths, extractBashCredentialPaths(cmd)...)
		}
	}

	return paths
}

// mergeDecisions returns the most restrictive of base and overlay.
// Rank: block(2) > warn(1) > allow(0).
//
// This mirrors the rank logic in policyloader/enforce.go but operates on two
// already-computed policy.Decision values rather than on policy-file rule slices.
// Do NOT use ApplyPolicyOverlay here — it is designed for JSON policy-file rules.
func mergeDecisions(base, overlay policy.Decision) policy.Decision {
	rank := map[string]int{"allow": 0, "warn": 1, "block": 2}
	if rank[overlay.Level] > rank[base.Level] {
		return overlay
	}
	return base
}
