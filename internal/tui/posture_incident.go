package tui

import (
	"strings"
	"time"

	audit "github.com/home-beekeeper/beekeeper/internal/audit"
)

// PostureIncidentModel is a DISPLAY-ONLY card shown when a posture warn (or block)
// appears in the activity/alerts feed. It surfaces the three graduated responses
// the operator has and the EXACT `beekeeper posture ...` command to enact each.
//
// READ-ONLY guarantee (Plan 29-02, Task 4): this card NEVER writes an override.
// The dashboard stays read-only by default; the CLI is the actionable surface. The
// card has no action buttons and no key handler that mutates state - it only renders
// the commands the operator copies and runs. This mirrors the read-only nature of
// the posture VIEW panel (posture_panel.go) rather than the action-bearing sentry
// IncidentModel (which still only acknowledges / opens records, never auto-contains).
type PostureIncidentModel struct {
	// Package is the package the posture rule fired on (may be empty for a bare
	// remote-source spec, in which case AllowCmd omits a package argument).
	Package string
	// Ecosystem is the parsed ecosystem (e.g. npm), used to scope the suggested
	// --ecosystem flag. Empty when unknown.
	Ecosystem string
	// Rule is the posture rule that fired (release-age|lifecycle|git-remote), used
	// to scope the suggested --rule flag and the enforce command. Empty when the
	// firing rule is not identifiable from the record.
	Rule string
	// Reason is the posture decision reason, shown verbatim (already redacted on disk).
	Reason string
	// Decision is the level shown in the badge: "warn" (default) or "block".
	Decision string
	// Timestamp is the formatted local time of the event.
	Timestamp string
}

// postureRuleFromRecordRuleIDs maps the pure-evaluator rule ID recorded in a
// posture decision back to the user-facing posture rule name used by the CLI
// flags. Returns "" when no posture rule ID is present (an all-rules suggestion).
func postureRuleFromRecordRuleIDs(ruleIDs []string) string {
	for _, id := range ruleIDs {
		switch id {
		case "release-age-policy":
			return "release-age"
		case "lifecycle-script-policy":
			return "lifecycle"
		case "remote-source-policy":
			return "git-remote"
		}
	}
	return ""
}

// PostureIncidentFromRecord builds the display-only posture incident card from a
// posture decision audit record. Every field comes from the record; nothing is
// fabricated. The card is built from a policy_decision record whose reason names
// an install-posture outcome (the caller decides which records qualify).
func PostureIncidentFromRecord(rec audit.AuditRecord) PostureIncidentModel {
	ts := rec.Timestamp
	if t, err := time.Parse(time.RFC3339, rec.Timestamp); err == nil {
		ts = t.Format("15:04:05")
	}

	// Resolve the package + ecosystem from the catalog match when present (the
	// install spec); otherwise leave empty and the suggested command omits the arg.
	pkg, eco := "", ""
	if len(rec.CatalogMatches) > 0 {
		pkg = rec.CatalogMatches[0].Package
		eco = rec.CatalogMatches[0].Ecosystem
	}

	decision := rec.Decision
	if decision == "" {
		decision = "warn"
	}

	return PostureIncidentModel{
		Package:   pkg,
		Ecosystem: eco,
		Rule:      postureRuleFromRecordRuleIDs(rec.RuleIDs),
		Reason:    rec.Reason,
		Decision:  decision,
		Timestamp: ts,
	}
}

// allowOnceCommand returns the exact `beekeeper posture allow ... --once` command
// the operator runs to allow the next matching install once.
func (m PostureIncidentModel) allowOnceCommand() string {
	return m.allowCommand("--once")
}

// allowAlwaysCommand returns the exact `beekeeper posture allow ... --always`
// command (with a placeholder reason the operator replaces).
func (m PostureIncidentModel) allowAlwaysCommand() string {
	return m.allowCommand("--always --reason \"<why this is safe>\"")
}

// allowCommand assembles a `beekeeper posture allow` command with the resolved
// package / ecosystem / rule scope and the given mode flags.
func (m PostureIncidentModel) allowCommand(mode string) string {
	var b strings.Builder
	b.WriteString("beekeeper posture allow")
	if m.Package != "" {
		b.WriteString(" " + m.Package)
	} else {
		b.WriteString(" <package>")
	}
	if m.Ecosystem != "" {
		b.WriteString(" --ecosystem " + m.Ecosystem)
	}
	if m.Rule != "" {
		b.WriteString(" --rule " + m.Rule)
	}
	b.WriteString(" " + mode)
	return b.String()
}

// enforceBlockCommand returns the `beekeeper posture enforce <rule> --block`
// command to opt the firing rule up to block. When no specific rule is identified
// it shows a placeholder so the operator picks the rule.
func (m PostureIncidentModel) enforceBlockCommand() string {
	rule := m.Rule
	if rule == "" {
		rule = "<release-age|lifecycle|git-remote>"
	}
	return "beekeeper posture enforce " + rule + " --block"
}

// View renders the display-only card. It shows the WARN/BLOCK badge, the reason,
// the three graduated options, and the exact command for each. No action buttons:
// the operator copies a command and runs it in their own shell.
func (m PostureIncidentModel) View(width int) string {
	var sb strings.Builder

	badge := BadgeWarn()
	if m.Decision == "block" {
		badge = BadgeBlock()
	}
	subject := m.Package
	if subject == "" {
		subject = "install"
	}
	sb.WriteString(badge + styleWhite.Render(" install posture: "+subject) +
		styleDim.Render("  "+m.Timestamp+" · posture") + "\n\n")

	if m.Reason != "" {
		sb.WriteString(styleDim.Render(m.Reason) + "\n\n")
	}

	sb.WriteString(styleDim.Render("YOUR OPTIONS (run one in your shell; the dashboard does not change anything)") + "\n\n")

	option := func(n, title, cmd string) {
		sb.WriteString(styleTeal.Render(n) + " " + styleWhite.Render(title) + "\n")
		sb.WriteString("    " + styleAmber.Render(cmd) + "\n\n")
	}
	option("1.", "Allow once (next matching install only, then warn again)", m.allowOnceCommand())
	option("2.", "Allow always (records a reason; never bypasses a malware block)", m.allowAlwaysCommand())
	option("3.", "Enforce block (opt this rule up so a definite violation blocks)", m.enforceBlockCommand())

	sb.WriteString(styleDim.Render("Block is the default: do nothing and the posture stands."))

	return styleIncidentBorder.Width(width - 4).Render(sb.String())
}
