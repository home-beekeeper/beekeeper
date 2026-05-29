package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	audit "github.com/mzansi-agentive/beekeeper/internal/audit"
)

// stepTickMsg advances the scan progress animation by one step.
type stepTickMsg struct{}

// scanStepDuration is 480ms per step, matching the prototype animation.
const scanStepDuration = 480 * time.Millisecond

// scanSteps are the four progress steps from the prototype (LOCKED).
var scanSteps = []string{
	"enumerating npm · pnpm · pip · cargo · gem · composer",
	"reading editor extensions (vscode, cursor, windsurf)",
	"matching against bumblebee + osv + socket",
	"cross-correlating with sentry inventory",
}

// scanCompleteText is the completion line from the prototype (LOCKED).
const scanCompleteText = "scan complete · 312 packages, 47 extensions"

// scanResultText is the result summary line from the prototype (LOCKED).
const scanResultText = "no threats matched · 1 package stale (>30d unmaintained)"

// ScanPanel implements PanelContent for the scan progress/history overlay.
type ScanPanel struct {
	scanMode    string // "deep", "quick", or "history"
	currentStep int    // how many steps have been revealed (0 = none yet)
	done        bool   // all steps complete
	history     []audit.AuditRecord
}

// NewScanPanel creates a ScanPanel.
// scanMode is "deep", "quick", or "history".
func NewScanPanel(scanMode string) *ScanPanel {
	return &ScanPanel{scanMode: scanMode}
}

// stepTickCmd arms the next 480ms step tick.
func stepTickCmd() tea.Cmd {
	return tea.Tick(scanStepDuration, func(_ time.Time) tea.Msg {
		return stepTickMsg{}
	})
}

// Update implements PanelContent.
func (p *ScanPanel) Update(msg tea.Msg) (PanelContent, tea.Cmd) {
	switch msg := msg.(type) {
	case stepTickMsg:
		if p.scanMode == "history" {
			return p, nil
		}
		if p.done {
			return p, nil
		}
		p.currentStep++
		if p.currentStep > len(scanSteps) {
			p.done = true
			return p, nil
		}
		return p, stepTickCmd() // arm next step

	case newRecordsMsg:
		if p.scanMode == "history" {
			for _, rec := range []audit.AuditRecord(msg) {
				if rec.RecordType == "scan_status" || rec.RecordType == "finding" {
					p.history = append(p.history, rec)
				}
			}
		}
	}
	return p, nil
}

// Title implements PanelContent.
func (p *ScanPanel) Title() string {
	if p.scanMode == "history" {
		return "Scan history"
	}
	return "Bumblebee scan"
}

// Count implements PanelContent.
func (p *ScanPanel) Count() string {
	switch p.scanMode {
	case "history":
		return "past runs"
	case "quick":
		return "quick · lockfiles + ext"
	default:
		return "deep · all ecosystems"
	}
}

// Padded implements PanelContent.
func (p *ScanPanel) Padded() bool { return true }

// Critical implements PanelContent.
func (p *ScanPanel) Critical() bool { return false }

// Body implements PanelContent.
func (p *ScanPanel) Body(width, height int) string {
	if p.scanMode == "history" {
		return p.historyBody()
	}
	return p.progressBody()
}

// progressBody renders the animated step progress view.
func (p *ScanPanel) progressBody() string {
	var sb strings.Builder
	sb.WriteString("\n")
	// Render completed/in-progress steps
	for i, step := range scanSteps {
		if i < p.currentStep {
			arrow := styleGreen.Render("▸")
			sb.WriteString("  " + arrow + " " + styleDim.Render(step) + "\n")
		}
	}
	// Completion lines
	if p.done {
		check := styleGreen.Render("✓")
		sb.WriteString("  " + check + " " + styleGreen.Render(scanCompleteText) + "\n")
		sb.WriteString("  " + styleDimmer.Render(scanResultText) + "\n")
	}
	return sb.String()
}

// historyBody renders past scan audit records.
func (p *ScanPanel) historyBody() string {
	if len(p.history) == 0 {
		return "\n  " + styleDim.Render("(no scan history)")
	}
	var sb strings.Builder
	sb.WriteString("\n")
	for _, rec := range p.history {
		sb.WriteString("  " + styleGreen.Render("✓") + "  " +
			styleDim.Render(rec.Timestamp) + "\n")
	}
	return sb.String()
}

// Footer implements PanelContent.
func (p *ScanPanel) Footer() string {
	if !p.done && p.scanMode != "history" {
		return styleDim.Render("scanning…")
	}
	return styleTeal.Render("esc") + styleDim.Render(" close")
}
