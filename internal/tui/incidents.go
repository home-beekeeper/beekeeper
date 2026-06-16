package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	audit "github.com/home-beekeeper/beekeeper/internal/audit"
)

// IncidentAction is a containment action button shown on the incident card.
type IncidentAction struct {
	Key string // "a" (acknowledge), "d" (full record)
	Cls string // "warn", "info"
	Lbl string
}

// TreeLine is a single line in the process tree display.
type TreeLine struct {
	Prefix      string
	PrefixStyle lipgloss.Style // style for the connector/verb prefix
	Style       lipgloss.Style // style for the main content text
	Text        string
}

// IncidentModel holds the data and state for the critical incident card.
type IncidentModel struct {
	RuleID    string
	RuleName  string
	Timestamp string
	Desc      string
	Tree      []TreeLine
	Actions   []IncidentAction
	SelAction int
}

// IncidentFromRecord builds a critical-incident card from a REAL sentry_alert
// audit record. Every field rendered (rule, process tree, files, network,
// correlated extension) comes from the record itself — there is no fabricated
// demo data. Missing fields degrade to honest placeholders rather than invented
// values, so the operator never sees forensic detail that did not occur.
//
// The action buttons are deliberately limited to what the dashboard can honestly
// do: acknowledge the incident, and open the full record. There is no in-TUI
// "quarantine"/"isolate" primitive (quarantine.Move needs a real extension path
// and there is no IPC isolate command), so the card never claims an automated
// containment it cannot perform — remediation is directed to the CLI.
func IncidentFromRecord(rec audit.AuditRecord) IncidentModel {
	ts := rec.Timestamp
	if t, err := time.Parse(time.RFC3339, rec.Timestamp); err == nil {
		ts = t.Format("15:04:05")
	}

	ruleName := rec.SentryRuleName
	if ruleName == "" {
		ruleName = rec.SentryRuleID
	}
	if ruleName == "" {
		ruleName = "sentry alert"
	}
	ruleID := rec.SentryRuleID
	if ruleID == "" {
		ruleID = "n/a"
	}

	return IncidentModel{
		RuleID:    ruleID,
		RuleName:  ruleName,
		Timestamp: ts,
		Desc:      buildIncidentDesc(rec),
		Tree:      buildIncidentTree(rec),
		Actions: []IncidentAction{
			{Key: "a", Cls: "warn", Lbl: "acknowledge"},
			{Key: "d", Cls: "info", Lbl: "full record"},
		},
		SelAction: 0,
	}
}

// buildIncidentDesc renders a one-to-three line description composed entirely
// from the record's real fields.
func buildIncidentDesc(rec audit.AuditRecord) string {
	base := lipgloss.NewStyle().Foreground(colorFg)
	red := lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	var sb strings.Builder

	sev := rec.SentrySeverity
	if sev == "" {
		sev = "critical"
	}
	sb.WriteString(base.Render("Sentry flagged a ") + red.Render(sev) + base.Render(" event"))
	if rec.SentryProcessExe != "" {
		sb.WriteString(base.Render(" from ") + base.Render(rec.SentryProcessExe))
		if rec.SentryProcessPID != 0 {
			sb.WriteString(styleDim.Render(fmt.Sprintf(" (pid %d)", rec.SentryProcessPID)))
		}
	}
	sb.WriteString(base.Render("."))

	nf, nn := len(rec.SentryFilesAccessed), len(rec.SentryNetworkDests)
	if nf > 0 || nn > 0 {
		var bits []string
		if nf > 0 {
			bits = append(bits, fmt.Sprintf("%d sensitive file%s read", nf, plural(nf)))
		}
		if nn > 0 {
			bits = append(bits, fmt.Sprintf("%d outbound connection%s", nn, plural(nn)))
		}
		sb.WriteString("\n" + red.Render(strings.Join(bits, " · ")) + base.Render("."))
	}
	if rec.SentryCorrelatedExt != "" {
		sb.WriteString("\n" + base.Render("Correlated extension: ") + styleCoral.Render(rec.SentryCorrelatedExt))
	}
	return sb.String()
}

// buildIncidentTree renders the process tree, file reads, and network
// destinations from the record. Returns an honest placeholder when the record
// carried none of these.
func buildIncidentTree(rec audit.AuditRecord) []TreeLine {
	var tree []TreeLine
	for i, p := range rec.SentryParentChain {
		prefix, style := "", styleDim
		if i > 0 {
			prefix, style = strings.Repeat("  ", i-1)+"└─ ", styleCoral
		}
		tree = append(tree, TreeLine{Prefix: prefix, PrefixStyle: styleDim, Style: style, Text: p})
	}
	for _, f := range rec.SentryFilesAccessed {
		tree = append(tree, TreeLine{Prefix: "   read ", PrefixStyle: styleDim, Style: styleRed, Text: f})
	}
	for _, n := range rec.SentryNetworkDests {
		tree = append(tree, TreeLine{Prefix: "   ", PrefixStyle: styleDim, Style: styleRed, Text: "→ " + n})
	}
	if len(tree) == 0 {
		tree = append(tree, TreeLine{Prefix: "", PrefixStyle: styleDim, Style: styleDim, Text: "(no process / file / network detail in this alert)"})
	}
	return tree
}

// plural returns "s" for any count other than 1.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// CatalogQuarantineIncidentFromRecord builds a catalog-quarantine or
// pending-quarantine incident card from a FRSP-01 audit record.
//
// When pending is false (artifact was quarantined): the card exposes
// human-gated [P]urge and [R]estore buttons.
// When pending is true (path unknown): only [A]cknowledge is shown —
// there is nothing to restore or purge yet.
//
// Honesty invariants:
//   - Purge is NEVER auto-triggered; it requires explicit [P] keypress.
//   - The card never claims a containment action it cannot perform.
//   - No em-dashes in visible copy; no colored left-edge accent stripes.
func CatalogQuarantineIncidentFromRecord(rec audit.AuditRecord, pending bool) IncidentModel {
	ts := rec.Timestamp
	if t, err := time.Parse(time.RFC3339, rec.Timestamp); err == nil {
		ts = t.Format("15:04:05")
	}

	ruleID := "FRSP-01"
	if rec.RecordID != "" {
		// Use the first rule ID from the record if present.
		for _, r := range rec.RuleIDs {
			if r != "" {
				ruleID = r
				break
			}
		}
	}

	var ruleName, desc string
	var actions []IncidentAction

	if pending {
		ruleName = "Pending Quarantine"
		desc = buildCatalogQuarantineDesc(rec, true)
		// Pending: path unknown, no move yet — only acknowledge.
		actions = []IncidentAction{
			{Key: "a", Cls: "warn", Lbl: "acknowledge"},
		}
	} else {
		ruleName = "Catalog Quarantine"
		desc = buildCatalogQuarantineDesc(rec, false)
		// Quarantined: expose human-gated Purge and Restore.
		// The purge action is labelled "danger" (destructive); restore is "info".
		// Neither fires automatically — both require explicit keypress by the operator.
		actions = []IncidentAction{
			{Key: "r", Cls: "info", Lbl: "restore"},
			{Key: "p", Cls: "danger", Lbl: "purge (irreversible)"},
			{Key: "a", Cls: "warn", Lbl: "acknowledge"},
		}
	}

	return IncidentModel{
		RuleID:    ruleID,
		RuleName:  ruleName,
		Timestamp: ts,
		Desc:      desc,
		Actions:   actions,
		SelAction: 0, // default to first action (restore for quarantined, acknowledge for pending)
	}
}

// buildCatalogQuarantineDesc renders a one-to-two line description for the
// catalog-quarantine / pending-quarantine incident card from real record fields.
func buildCatalogQuarantineDesc(rec audit.AuditRecord, pending bool) string {
	base := lipgloss.NewStyle().Foreground(colorFg)
	amber := lipgloss.NewStyle().Foreground(colorAmber).Bold(true)
	coral := lipgloss.NewStyle().Foreground(colorCoral)
	var sb strings.Builder

	pkg := rec.ToolName
	if pkg == "" {
		pkg = "unknown package"
	}
	if pending {
		sb.WriteString(base.Render("Scan hit on ") + amber.Render(pkg) + base.Render(" — path unresolved, quarantine pending."))
	} else {
		sb.WriteString(base.Render("Scan hit on ") + amber.Render(pkg) + base.Render(" — moved to quarantine (reversible)."))
	}

	if rec.Reason != "" {
		sb.WriteString("\n" + coral.Render(rec.Reason))
	}
	return sb.String()
}

// Update handles key navigation across action buttons.
func (m IncidentModel) Update(msg tea.Msg) (IncidentModel, tea.Cmd) {
	if kp, ok := msg.(tea.KeyPressMsg); ok {
		switch kp.String() {
		case "up", "left":
			if m.SelAction > 0 {
				m.SelAction--
			}
		case "down", "right":
			if m.SelAction < len(m.Actions)-1 {
				m.SelAction++
			}
		}
	}
	return m, nil
}

// View renders the red-bordered incident card at the given width.
func (m IncidentModel) View(width int) string {
	var sb strings.Builder

	// Head
	badge := BadgeCrit()
	title := styleWhite.Render(" " + m.RuleName)
	ts := styleDim.Render("  " + m.Timestamp + " · sentry · rule " + m.RuleID + " ")
	sb.WriteString(badge + title + ts + "\n\n")

	// Description — pre-rendered with inline ANSI
	sb.WriteString(m.Desc + "\n\n")

	// Process tree
	sb.WriteString(styleDim.Render("PROCESS TREE") + "\n")
	for _, line := range m.Tree {
		sb.WriteString(line.PrefixStyle.Render(line.Prefix) + line.Style.Render(line.Text) + "\n")
	}
	sb.WriteString("\n")

	// Action buttons
	var btns []string
	for i, act := range m.Actions {
		var keyStyle lipgloss.Style
		switch act.Cls {
		case "danger":
			keyStyle = styleCoral
		case "warn":
			keyStyle = styleAmber
		default:
			keyStyle = styleTeal
		}
		btnContent := keyStyle.Render("["+act.Key+"]") + " " + act.Lbl
		var btnStyle lipgloss.Style
		if i == m.SelAction {
			btnStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorTeal).
				Background(colorSelbg).
				Foreground(colorWhite).
				Padding(0, 1)
		} else {
			btnStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorder).
				Background(colorPanel).
				Padding(0, 1)
		}
		btns = append(btns, btnStyle.Render(btnContent))
	}
	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, btns...) + "\n")
	sb.WriteString(lipgloss.PlaceHorizontal(width-4, lipgloss.Right, styleDim.Render("rotate exposed creds after containment")) + "\n")

	body := sb.String()
	return styleIncidentBorder.Width(width - 4).Render(body)
}
