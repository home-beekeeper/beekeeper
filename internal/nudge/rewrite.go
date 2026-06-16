package nudge

// PURE — imports only "strings" and "internal/pkgparse".
// pkgparse imports only "strings" and is pure, so this file remains pure.
// Rewrite operates ONLY on the parsed Package/Version tokens from pkgparse.ParsedCommand.
// It never re-emits shell metacharacters or a "sudo" prefix (T-08-08).
// The caller (Evaluate) is responsible for not calling rewrite functions when
// cmd.Sudo is true (§10-10); rewriteToPnpm/rewriteToBun assume Sudo is already filtered.

import (
	"strings"

	"github.com/home-beekeeper/beekeeper/internal/pkgparse"
)

// RewriteToPnpm is the EXPORTED wrapper over rewriteToPnpm so the check adapter
// (presentation layer) can compute the pnpm-equivalent command for a block deny
// message across the package boundary. The lowercase form stays unexported for
// the internal Evaluate path.
func RewriteToPnpm(cmd pkgparse.ParsedCommand) string { return rewriteToPnpm(cmd) }

// RewriteToBun is the EXPORTED wrapper over rewriteToBun (see RewriteToPnpm).
func RewriteToBun(cmd pkgparse.ParsedCommand) string { return rewriteToBun(cmd) }

// rewriteToPnpm rewrites a parsed npm install command to its pnpm equivalent.
//
// Verb mapping (PRD §6.4, NUDGE-03):
//   - no-arg install (Package=="") → "pnpm install"
//   - npx / IsExec → "pnpm dlx <package[@version]>"
//   - all other install verbs → "pnpm add <package[@version]>"
//
// The output is advisory/audit only — Beekeeper does not execute it.
func rewriteToPnpm(cmd pkgparse.ParsedCommand) string {
	// No-arg install: keep the "install" form (lockfile-based).
	if cmd.Package == "" {
		return "pnpm install"
	}

	// exec verbs (npx / pnpm dlx / bun x) → pnpm dlx
	if cmd.IsExec {
		return "pnpm dlx " + packageToken(cmd.Package, cmd.Version)
	}

	// All other install verbs → pnpm add
	return "pnpm add " + packageToken(cmd.Package, cmd.Version)
}

// rewriteToBun rewrites a parsed npm install command to its bun equivalent.
//
// Verb mapping (PRD §6.4, NUDGE-03):
//   - no-arg install → "bun install"
//   - npx / IsExec → "bun x <package[@version]>"
//   - all other install verbs → "bun add <package[@version]>"
func rewriteToBun(cmd pkgparse.ParsedCommand) string {
	if cmd.Package == "" {
		return "bun install"
	}

	if cmd.IsExec {
		return "bun x " + packageToken(cmd.Package, cmd.Version)
	}

	return "bun add " + packageToken(cmd.Package, cmd.Version)
}

// packageToken returns "package" or "package@version" depending on whether a
// version is present. Version is appended verbatim as parsed by pkgparse
// (already stripped of leading "@").
func packageToken(pkg, version string) string {
	if version == "" {
		return pkg
	}
	var b strings.Builder
	b.WriteString(pkg)
	b.WriteByte('@')
	b.WriteString(version)
	return b.String()
}
