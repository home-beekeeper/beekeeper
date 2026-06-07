package tui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	audit "github.com/bantuson/beekeeper/internal/audit"
	config "github.com/bantuson/beekeeper/internal/config"
	editorinit "github.com/bantuson/beekeeper/internal/editorinit"
	platform "github.com/bantuson/beekeeper/internal/platform"
	scan "github.com/bantuson/beekeeper/internal/scan"
)

// stepTickMsg advances the scan progress animation by one step.
type stepTickMsg struct{}

// scanResultMsg carries the outcome of a real scan back to the panel. Exactly
// one of res / err is meaningful (err non-nil means the scan failed).
type scanResultMsg struct {
	res scanResult
	err error
}

// scanResult holds the REAL counts parsed from a completed scan's NDJSON output.
type scanResult struct {
	packages          int  // pollen inventory "package" records
	findings          int  // "finding" records (pollen + beekeeper-own extension scan)
	threats           int  // findings whose decision is not "allow"
	pollenUnavailable bool // pollen not installed — editor-extension scan only
}

// scanStepDuration is 480ms per step, matching the prototype animation.
const scanStepDuration = 480 * time.Millisecond

// scanSteps are the progress steps shown while a scan runs. They describe the
// work the scan actually performs; the completion line below is computed from
// the scan's real output, never hardcoded.
var scanSteps = []string{
	"enumerating npm · pnpm · pip · cargo · gem · composer",
	"reading editor extensions (vscode, cursor, windsurf)",
	"matching against bumblebee + osv + socket",
	"cross-correlating with sentry inventory",
}

// ScanPanel implements PanelContent for the scan progress/history overlay.
type ScanPanel struct {
	scanMode    string // "deep", "quick", or "history"
	currentStep int    // how many steps have been revealed (0 = none yet)
	done        bool   // the real scan has returned
	result      *scanResult
	err         error
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

// runScanCmd runs a REAL scan asynchronously and returns its result as a
// scanResultMsg. It is invoked by the App when a deep/quick scan panel opens.
func (p *ScanPanel) runScanCmd() tea.Cmd {
	deep := p.scanMode == "deep"
	return func() tea.Msg {
		cfg, ok := buildScanConfig(deep)
		if !ok {
			return scanResultMsg{err: errors.New("could not resolve scan directories")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		var buf bytes.Buffer
		if err := scan.Scan(ctx, cfg, &buf); err != nil {
			return scanResultMsg{err: err}
		}
		return scanResultMsg{res: parseScanOutput(buf.Bytes())}
	}
}

// buildScanConfig assembles the same scan.Config the `beekeeper scan` CLI uses:
// real catalog/audit paths, watch directories from layered config (falling back
// to editor auto-detection), and the configured Socket token.
func buildScanConfig(deep bool) (scan.Config, bool) {
	catalogDir, err := platform.CatalogDir()
	if err != nil {
		return scan.Config{}, false
	}
	auditDir, err := platform.AuditDir()
	if err != nil {
		return scan.Config{}, false
	}

	var dirs []string
	var token string
	if userPath, perr := platform.ConfigPath(); perr == nil {
		if cfg, cerr := config.LoadLayered(config.LayerOpts{UserPath: userPath, Environ: os.Environ()}); cerr == nil {
			dirs = cfg.WatchDirectories()
			token = cfg.SocketAPIToken()
		}
	}
	if len(dirs) == 0 {
		if editors, derr := editorinit.DetectEditors(); derr == nil {
			for _, e := range editors {
				if e.ExtensionDir != "" {
					dirs = append(dirs, e.ExtensionDir)
				}
			}
		}
	}

	return scan.Config{
		Deep:          deep,
		ExtensionDirs: dirs,
		IndexPath:     filepath.Join(catalogDir, "bumblebee.idx"),
		CacheDir:      catalogDir,
		AuditPath:     filepath.Join(auditDir, "beekeeper.ndjson"),
		SocketToken:   token,
		HTTPClient:    &http.Client{Timeout: 4 * time.Second},
		Now:           func() time.Time { return time.Now().UTC() },
	}, true
}

// parseScanOutput tallies the REAL record counts from a scan's NDJSON stream.
func parseScanOutput(data []byte) scanResult {
	var r scanResult
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var probe struct {
			RecordType        string `json:"record_type"`
			Decision          string `json:"decision"`
			PollenUnavailable bool   `json:"pollen_unavailable"`
		}
		if err := json.Unmarshal(line, &probe); err != nil {
			continue
		}
		switch probe.RecordType {
		case "package":
			r.packages++
		case "finding":
			r.findings++
			if probe.Decision != "" && probe.Decision != "allow" {
				r.threats++
			}
		case "scan_status":
			if probe.PollenUnavailable {
				r.pollenUnavailable = true
			}
		}
	}
	return r
}

// Update implements PanelContent.
func (p *ScanPanel) Update(msg tea.Msg) (PanelContent, tea.Cmd) {
	switch msg := msg.(type) {
	case stepTickMsg:
		if p.scanMode == "history" || p.done {
			return p, nil
		}
		// Reveal one more step. Stop re-arming once all steps are shown — the
		// panel then waits on the real scanResultMsg rather than fabricating a
		// completion. done is NEVER set by the animation.
		if p.currentStep < len(scanSteps) {
			p.currentStep++
			if p.currentStep < len(scanSteps) {
				return p, stepTickCmd()
			}
		}
		return p, nil

	case scanResultMsg:
		p.done = true
		if msg.err != nil {
			p.err = msg.err
		} else {
			r := msg.res
			p.result = &r
		}
		return p, nil

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

// progressBody renders the animated step progress view and, once the real scan
// has returned, its real completion summary (or the error).
func (p *ScanPanel) progressBody() string {
	var sb strings.Builder
	sb.WriteString("\n")
	for i, step := range scanSteps {
		if i < p.currentStep {
			arrow := styleGreen.Render("▸")
			sb.WriteString("  " + arrow + " " + styleDim.Render(step) + "\n")
		}
	}
	if !p.done {
		return sb.String()
	}
	if p.err != nil {
		sb.WriteString("  " + styleRed.Render("✗ scan failed: "+p.err.Error()) + "\n")
		return sb.String()
	}
	r := p.result
	if r == nil {
		r = &scanResult{}
	}
	complete := fmt.Sprintf("scan complete · %d package%s, %d finding%s",
		r.packages, plural(r.packages), r.findings, plural(r.findings))
	sb.WriteString("  " + styleGreen.Render("✓") + " " + styleGreen.Render(complete) + "\n")

	var summary string
	if r.threats == 0 {
		summary = "no threats matched"
	} else {
		summary = fmt.Sprintf("%d threat%s flagged — see the alert log", r.threats, plural(r.threats))
	}
	if r.pollenUnavailable {
		summary += " · pollen unavailable (editor-extension scan only)"
	}
	sb.WriteString("  " + styleDimmer.Render(summary) + "\n")
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
