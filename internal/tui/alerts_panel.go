package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	audit "github.com/mzansi-agentive/beekeeper/internal/audit"
)

const maxAlertRows = 200

// quarantineAlertMsg is sent to App when the user presses q/Q in the alerts panel.
// App handles it by closing the panel and showing a toast.
type quarantineAlertMsg struct{ RecordID string }

// AlertRow is a single rendered row in the alerts panel.
type AlertRow struct {
	Time     string // HH:MM:SS
	Badge    string // rendered badge string (BadgeCrit, BadgeBlock, etc.)
	Label    string // event name or rule name
	Meta     string // right-aligned detail
	RecordID string
}

// AlertsPanel implements PanelContent for the sentry alert log panel.
type AlertsPanel struct {
	rows        []AlertRow
	selIdx      int
	critical    bool   // when true, panel uses red border (passed from App.critical)
	filterMode  bool   // true when typing a filter query
	filterQuery string // case-insensitive substring match on label+meta
}

// NewAlertsPanel creates a new AlertsPanel.
// critical=true means the panel is opened in critical mode (red border).
func NewAlertsPanel(critical bool) *AlertsPanel {
	return &AlertsPanel{critical: critical}
}

// Update implements PanelContent.
func (p *AlertsPanel) Update(msg tea.Msg) (PanelContent, tea.Cmd) {
	switch msg := msg.(type) {
	case newRecordsMsg:
		for _, rec := range []audit.AuditRecord(msg) {
			row, ok := p.recordToRow(rec)
			if !ok {
				continue
			}
			p.rows = append(p.rows, row)
		}
		// Cap at maxAlertRows — drop oldest
		if len(p.rows) > maxAlertRows {
			p.rows = p.rows[len(p.rows)-maxAlertRows:]
		}

	case tea.KeyPressMsg:
		k := msg.String()

		// Filter input mode: printable chars append, backspace trims, Esc clears.
		if p.filterMode {
			switch k {
			case "esc":
				p.filterMode = false
				p.filterQuery = ""
				p.selIdx = 0
			case "backspace":
				if len(p.filterQuery) > 0 {
					p.filterQuery = p.filterQuery[:len(p.filterQuery)-1]
				}
				p.selIdx = 0
			default:
				if len(k) == 1 && k[0] >= 0x20 && k[0] < 0x7f {
					p.filterQuery += k
					p.selIdx = 0
				}
			}
			return p, nil
		}

		// Normal navigation.
		visible := p.filteredRows()
		switch k {
		case "j", "down":
			if p.selIdx < len(visible)-1 {
				p.selIdx++
			}
		case "k", "up":
			if p.selIdx > 0 {
				p.selIdx--
			}
		case "/":
			p.filterMode = true
			p.filterQuery = ""
			p.selIdx = 0
		case "q", "Q":
			if len(visible) > 0 {
				id := visible[p.selIdx].RecordID
				return p, func() tea.Msg { return quarantineAlertMsg{RecordID: id} }
			}
		}
	}
	return p, nil
}

// filteredRows returns the rows that match the current filterQuery.
// When filterQuery is empty, all rows are returned.
func (p *AlertsPanel) filteredRows() []AlertRow {
	if p.filterQuery == "" {
		return p.rows
	}
	q := strings.ToLower(p.filterQuery)
	var out []AlertRow
	for _, r := range p.rows {
		if strings.Contains(strings.ToLower(r.Label), q) ||
			strings.Contains(strings.ToLower(r.Meta), q) {
			out = append(out, r)
		}
	}
	return out
}

// recordToRow converts an AuditRecord to an AlertRow if it should appear in alerts.
// Returns (row, true) if the record belongs here, (zero, false) otherwise.
func (p *AlertsPanel) recordToRow(rec audit.AuditRecord) (AlertRow, bool) {
	ts, _ := time.Parse(time.RFC3339, rec.Timestamp)
	timeStr := ts.Format("15:04:05")

	switch rec.RecordType {
	case "sentry_alert":
		var badge string
		switch rec.SentrySeverity {
		case "critical":
			badge = BadgeCrit()
		case "high", "medium":
			badge = BadgeWarn()
		default:
			badge = BadgeWarn()
		}
		label := rec.SentryRuleName
		if label == "" {
			label = rec.SentryRuleID
		}
		meta := rec.SentryCorrelatedExt
		if len(rec.SentryNetworkDests) > 0 {
			if meta != "" {
				meta += " → " + rec.SentryNetworkDests[0]
			} else {
				meta = rec.SentryNetworkDests[0]
			}
		}
		if meta == "" && len(rec.SentryFilesAccessed) > 0 {
			meta = strings.Join(rec.SentryFilesAccessed[:minInt(3, len(rec.SentryFilesAccessed))], " ")
		}
		return AlertRow{
			Time:     timeStr,
			Badge:    badge,
			Label:    label,
			Meta:     meta,
			RecordID: rec.RecordID,
		}, true

	case "policy_decision":
		switch rec.Decision {
		case "block":
			label := rec.ToolName
			meta := rec.Reason
			return AlertRow{
				Time:     timeStr,
				Badge:    BadgeBlock(),
				Label:    label,
				Meta:     meta,
				RecordID: rec.RecordID,
			}, true
		case "warn":
			label := rec.ToolName
			meta := rec.Reason
			return AlertRow{
				Time:     timeStr,
				Badge:    BadgeWarn(),
				Label:    label,
				Meta:     meta,
				RecordID: rec.RecordID,
			}, true
		}
	}
	return AlertRow{}, false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (p *AlertsPanel) Title() string { return "Sentry alert log" }

func (p *AlertsPanel) Count() string {
	n := len(p.rows)
	if p.critical && n > 0 {
		return fmt.Sprintf("%d alerts · 1 active critical", n)
	}
	return fmt.Sprintf("%d alerts · today", n)
}

func (p *AlertsPanel) Padded() bool { return false }

func (p *AlertsPanel) Critical() bool { return p.critical }

func (p *AlertsPanel) Body(width, height int) string {
	visible := p.filteredRows()

	var lines []string

	// Filter bar — shown when filter mode is active or a query is set.
	if p.filterMode || p.filterQuery != "" {
		cursor := ""
		if p.filterMode {
			cursor = "▌"
		}
		filterBar := "  " + styleTeal.Render("/") + " " +
			lipgloss.NewStyle().Foreground(colorFg).Render(p.filterQuery) +
			styleTeal.Render(cursor) +
			"  " + styleDimmer.Render(fmt.Sprintf("%d match", len(visible)))
		lines = append(lines, filterBar)
		lines = append(lines, "  "+lipgloss.NewStyle().Foreground(colorBorderDim).Render(strings.Repeat("─", max(0, width-6))))
	}

	if len(visible) == 0 {
		if p.filterQuery != "" {
			lines = append(lines, "  "+styleDim.Render("(no matches)"))
		} else {
			lines = append(lines, "  "+styleDim.Render("(no alerts)"))
		}
		return strings.Join(lines, "\n")
	}

	start := 0
	panelHeight := height - len(lines)
	if len(visible) > panelHeight && panelHeight > 0 {
		start = len(visible) - panelHeight
	}
	for i := start; i < len(visible); i++ {
		row := visible[i]
		timeStr := styleDim.Render(fmt.Sprintf("%-8s", row.Time))
		labelStr := lipgloss.NewStyle().Foreground(colorFg).Render(fmt.Sprintf("%-30s", row.Label))
		metaStr := styleDim.Render(row.Meta)

		line := "  " + timeStr + "  " + row.Badge + "  " + labelStr + "  " + metaStr
		if i == p.selIdx {
			line = styleSelRow.Render(line)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (p *AlertsPanel) Footer() string {
	if p.filterMode {
		return styleTeal.Render("type") + styleDim.Render(" to filter · ") +
			styleTeal.Render("esc") + styleDim.Render(" clear filter")
	}
	return styleTeal.Render("↑↓") + styleDim.Render(" select · ") +
		styleTeal.Render("enter") + styleDim.Render(" inspect · ") +
		styleTeal.Render("q") + styleDim.Render(" quarantine · ") +
		styleTeal.Render("esc") + styleDim.Render(" close")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
