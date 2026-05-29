package tui

import (
	"encoding/json"
	"strings"

	tea "charm.land/bubbletea/v2"

	audit "github.com/mzansi-agentive/beekeeper/internal/audit"
)

// maxAuditLines is the maximum number of NDJSON records stored in AuditPanel.
// T-08-04-02: cap prevents unbounded memory growth.
const maxAuditLines = 20

// AuditPanel implements PanelContent for the audit tail overlay.
type AuditPanel struct {
	records []audit.AuditRecord
}

// NewAuditPanel creates an AuditPanel.
func NewAuditPanel() *AuditPanel { return &AuditPanel{} }

// Update implements PanelContent.
func (p *AuditPanel) Update(msg tea.Msg) (PanelContent, tea.Cmd) {
	if recs, ok := msg.(newRecordsMsg); ok {
		p.records = append(p.records, []audit.AuditRecord(recs)...)
		if len(p.records) > maxAuditLines {
			p.records = p.records[len(p.records)-maxAuditLines:]
		}
	}
	return p, nil
}

// Title implements PanelContent.
func (p *AuditPanel) Title() string { return "Audit tail" }

// Count implements PanelContent.
func (p *AuditPanel) Count() string { return "NDJSON · live" }

// Padded implements PanelContent.
func (p *AuditPanel) Padded() bool { return true }

// Critical implements PanelContent.
func (p *AuditPanel) Critical() bool { return false }

// isAlertRecord returns true for records that should be highlighted in red.
func isAlertRecord(rec audit.AuditRecord) bool {
	return rec.RecordType == "sentry_alert" || rec.Decision == "alert"
}

// Body implements PanelContent.
func (p *AuditPanel) Body(width, height int) string {
	if len(p.records) == 0 {
		return "\n  " + styleDim.Render("(no audit records yet)")
	}
	var lines []string
	lines = append(lines, "")
	for _, rec := range p.records {
		// Re-serialize to compact JSON for display
		b, err := json.Marshal(rec)
		if err != nil {
			continue
		}
		line := string(b)
		// Truncate to width to prevent horizontal overflow
		if width > 8 && len(line) > width-6 {
			line = line[:width-6] + "…"
		}
		if isAlertRecord(rec) {
			lines = append(lines, "  "+styleRed.Render(line))
		} else {
			lines = append(lines, "  "+styleDimmer.Render(line))
		}
	}
	return strings.Join(lines, "\n")
}

// Footer implements PanelContent.
func (p *AuditPanel) Footer() string {
	return styleTeal.Render("esc") + styleDim.Render(" close")
}
