package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	audit "github.com/home-beekeeper/beekeeper/internal/audit"
	platform "github.com/home-beekeeper/beekeeper/internal/platform"
)

// maxAuditLines caps how many records the panel holds (newest kept). A bound
// keeps memory flat and the scroll window predictable on a multi-MB log.
const maxAuditLines = 200

// AuditPanel implements PanelContent for the audit viewer. Instead of dumping raw
// NDJSON it renders one structured, color-coded row per record (newest first) and
// an expandable detail view that shows the DECISION LOGIC: which corroboration
// sources agreed/dissented, the catalog matches, the rules that fired, the
// Sentry process/file/network evidence, and the LlamaFirewall result.
//
// SECURITY: audit fields carry attacker-influenceable content (package names,
// commands, paths). The legacy raw-JSON view was safe only because json.Marshal
// escaped control bytes; the structured view loses that, so EVERY record-derived
// string is run through sanitizeForTUI before it reaches the terminal (see
// sanitize.go). The panel renders only the already-redacted on-disk fields and
// never re-reads or unredacts.
type AuditPanel struct {
	records      []audit.AuditRecord // chronological; displayed newest-first
	seen         map[string]bool     // dedupe by RecordID across load + live appends
	selIdx       int                 // index into the (filtered, newest-first) display slice
	offset       int                 // scroll-window top
	expanded     bool                // detail view of the selected record
	enforcedOnly bool                // filter: only block/warn/alert/quarantine events
	auditPath    string
}

// NewAuditPanel builds a panel seeded with the recent audit tail so the viewer
// shows history on open, then keeps appending live records.
func NewAuditPanel() *AuditPanel {
	p := &AuditPanel{seen: map[string]bool{}}
	if auditDir, err := platform.AuditDir(); err == nil {
		p.auditPath = filepath.Join(auditDir, "beekeeper.ndjson")
		recs := recentAuditRecords(p.auditPath)
		if len(recs) > maxAuditLines {
			recs = recs[len(recs)-maxAuditLines:]
		}
		for _, r := range recs {
			if r.RecordID != "" {
				p.seen[r.RecordID] = true
			}
		}
		p.records = recs
	}
	return p
}

// Update implements PanelContent: append live records (deduped) and handle keys.
func (p *AuditPanel) Update(msg tea.Msg) (PanelContent, tea.Cmd) {
	switch msg := msg.(type) {
	case newRecordsMsg:
		for _, r := range []audit.AuditRecord(msg) {
			if r.RecordID != "" {
				if p.seen[r.RecordID] {
					continue
				}
				p.seen[r.RecordID] = true
			}
			p.records = append(p.records, r)
		}
		if len(p.records) > maxAuditLines {
			p.records = p.records[len(p.records)-maxAuditLines:]
		}
	case tea.KeyPressMsg:
		p.handleKey(msg.String())
	}
	return p, nil
}

func (p *AuditPanel) handleKey(k string) {
	if p.expanded {
		switch k {
		case "h", "left", "backspace":
			p.expanded = false
		}
		return
	}
	n := len(p.display())
	switch k {
	case "j", "down":
		if p.selIdx < n-1 {
			p.selIdx++
		}
	case "k", "up":
		if p.selIdx > 0 {
			p.selIdx--
		}
	case "enter", "l", "right":
		if n > 0 {
			p.expanded = true
		}
	case "f":
		p.enforcedOnly = !p.enforcedOnly
		p.selIdx, p.offset = 0, 0
	}
}

// display returns the records to show: filtered (when enforcedOnly) and reversed
// so the newest record is first.
func (p *AuditPanel) display() []audit.AuditRecord {
	out := make([]audit.AuditRecord, 0, len(p.records))
	for i := len(p.records) - 1; i >= 0; i-- {
		rec := p.records[i]
		if p.enforcedOnly && !isEnforcementEvent(rec) {
			continue
		}
		out = append(out, rec)
	}
	return out
}

// Title implements PanelContent.
func (p *AuditPanel) Title() string { return "Audit log" }

// Count implements PanelContent.
func (p *AuditPanel) Count() string {
	if p.enforcedOnly {
		return fmt.Sprintf("%d shown · enforced", len(p.display()))
	}
	return fmt.Sprintf("%d records", len(p.records))
}

// Padded implements PanelContent.
func (p *AuditPanel) Padded() bool { return true }

// Critical implements PanelContent.
func (p *AuditPanel) Critical() bool { return false }

// Body implements PanelContent.
func (p *AuditPanel) Body(width, height int) string {
	disp := p.display()
	if len(disp) == 0 {
		msg := "(no audit records yet)"
		if p.enforcedOnly {
			msg = "(no blocks, warns, or alerts in the recent log)"
		}
		return "\n  " + styleDim.Render(msg)
	}
	p.clampSel(len(disp))

	if p.expanded {
		return p.renderDetail(disp[p.selIdx])
	}

	maxRows := height - 2
	if maxRows < 3 {
		maxRows = 3
	}
	p.ensureVisible(len(disp), maxRows)

	reasonW := width - 54
	if reasonW < 10 {
		reasonW = 10
	}

	lines := []string{""}
	end := p.offset + maxRows
	if end > len(disp) {
		end = len(disp)
	}
	for i := p.offset; i < end; i++ {
		lines = append(lines, p.renderRow(i, disp[i], reasonW))
	}
	if len(disp) > maxRows {
		lines = append(lines, "",
			"  "+styleDimmer.Render(fmt.Sprintf("%d-%d of %d · ↑↓ scroll", p.offset+1, end, len(disp))))
	}
	return strings.Join(lines, "\n")
}

func (p *AuditPanel) renderRow(i int, rec audit.AuditRecord, reasonW int) string {
	label, st := badgeFor(rec)
	ts := tsShort(rec.Timestamp)
	subj := recordSubject(rec, 28)
	reason := recordReason(rec, reasonW)

	text := "  " +
		styleDimmer.Render(ts) + "  " +
		st.Render(fmt.Sprintf("%-6s", label)) + "  " +
		styleWhite.Render(fmt.Sprintf("%-28s", subj)) + "  " +
		styleDim.Render(reason)

	if i == p.selIdx {
		text = styleSelRow.Render(strings.TrimRight(text, " "))
	}
	return text
}

// renderDetail shows the full decision logic for one record.
func (p *AuditPanel) renderDetail(rec audit.AuditRecord) string {
	label, st := badgeFor(rec)
	head := "  " + st.Render(fmt.Sprintf("%-6s", label)) + "  " +
		styleWhite.Render(recordSubject(rec, 60)) + "  " +
		styleDimmer.Render(tsShort(rec.Timestamp))
	lines := []string{"", head, ""}
	lines = append(lines, detailLines(rec)...)
	return strings.Join(lines, "\n")
}

// Footer implements PanelContent.
func (p *AuditPanel) Footer() string {
	if p.expanded {
		return styleTeal.Render("←/h") + styleDim.Render(" back · ") +
			styleTeal.Render("esc") + styleDim.Render(" close")
	}
	filter := "all"
	if p.enforcedOnly {
		filter = "enforced"
	}
	return styleTeal.Render("↑↓") + styleDim.Render(" select · ") +
		styleTeal.Render("enter") + styleDim.Render(" detail · ") +
		styleTeal.Render("f") + styleDim.Render(" filter ("+filter+") · ") +
		styleTeal.Render("esc") + styleDim.Render(" close")
}

func (p *AuditPanel) clampSel(n int) {
	if n == 0 {
		p.selIdx = 0
		return
	}
	if p.selIdx >= n {
		p.selIdx = n - 1
	}
	if p.selIdx < 0 {
		p.selIdx = 0
	}
}

func (p *AuditPanel) ensureVisible(n, maxRows int) {
	if p.selIdx < p.offset {
		p.offset = p.selIdx
	}
	if p.selIdx >= p.offset+maxRows {
		p.offset = p.selIdx - maxRows + 1
	}
	if p.offset > n-maxRows {
		p.offset = n - maxRows
	}
	if p.offset < 0 {
		p.offset = 0
	}
}

// --- helpers ---

// badgeFor maps a record to a short label and its color. It keys on the record
// type first (alerts/config/llmf), then the allow/warn/block decision.
//
// NOTE: the "nudge" and "version_drift" record types were emitted by the
// package-manager nudge feature, removed in v1.1.0. They are no longer written;
// any such records left in an old audit log fall through to the decision-based
// badge below (typically EVENT) rather than getting a dedicated label.
func badgeFor(rec audit.AuditRecord) (string, lipgloss.Style) {
	switch rec.RecordType {
	case "sentry_alert":
		return "ALERT", styleRed
	case "config_change":
		return "CONFIG", styleTeal
	case "posture_override":
		return "POSTURE", styleTeal
	case "llmf_alert":
		if rec.LLMFResult != "" && rec.LLMFResult != "clean" {
			return "LLMF", styleRed
		}
		return "LLMF", styleDim
	}
	switch rec.Decision {
	case "block":
		return "BLOCK", styleRed
	case "warn":
		return "WARN", styleAmber
	case "alert":
		return "ALERT", styleRed
	case "allow":
		return "ALLOW", styleGreen
	}
	return "EVENT", styleDimmer
}

// isEnforcementEvent is the "enforced only" filter: a block/warn/alert decision,
// a Sentry alert, a non-clean LlamaFirewall scan, or a quarantine.
func isEnforcementEvent(rec audit.AuditRecord) bool {
	switch rec.Decision {
	case "block", "warn", "alert":
		return true
	}
	if rec.Quarantine || rec.RecordType == "sentry_alert" {
		return true
	}
	if rec.RecordType == "llmf_alert" && rec.LLMFResult != "" && rec.LLMFResult != "clean" {
		return true
	}
	return false
}

// recordSubject picks the most identifying string for a record and sanitizes it.
func recordSubject(rec audit.AuditRecord, max int) string {
	var s string
	switch {
	case len(rec.CatalogMatches) > 0:
		m := rec.CatalogMatches[0]
		s = m.Ecosystem + ":" + m.Package
		if m.Version != "" {
			s += "@" + m.Version
		}
	case rec.SentryProcessExe != "":
		s = rec.SentryProcessExe
	case rec.SentryCorrelatedExt != "":
		s = rec.SentryCorrelatedExt
	case rec.RecordType == "config_change" && rec.ReasonCode != "":
		s = rec.ReasonCode
	case rec.RecordType == "posture_override":
		// Prefer the package (allow override); fall back to the rule (enforce override).
		if rec.PosturePackage != "" {
			s = rec.PosturePackage
		} else {
			s = rec.PostureRule
		}
	default:
		s = rec.ToolName
	}
	return sanitizeForTUI(s, max)
}

func recordReason(rec audit.AuditRecord, max int) string {
	return sanitizeForTUI(rec.Reason, max)
}

// tsShort renders an RFC3339 timestamp as local HH:MM:SS, or a placeholder.
func tsShort(ts string) string {
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t.Local().Format("15:04:05")
	}
	return "--:--:--"
}

// detailLines builds the decision-logic block for one record. Only fields that
// are present render; every value is sanitized.
func detailLines(rec audit.AuditRecord) []string {
	var out []string
	add := func(label, val string) {
		v := sanitizeForTUI(val, 240)
		if v == "" {
			return
		}
		out = append(out, "    "+styleDim.Render(fmt.Sprintf("%-15s", label))+styleWhite.Render(v))
	}

	add("decision", rec.Decision)
	if rec.RecordType == "posture_override" {
		add("override", rec.PostureOverrideAction)
		add("posture rule", rec.PostureRule)
		add("ecosystem", rec.PostureEcosystem)
		add("package", rec.PosturePackage)
	}
	add("reason", rec.Reason)
	add("surface", strings.TrimSpace(rec.Endpoint+" "+rec.SourceSurface))
	add("agent", rec.AgentName)
	add("tool", rec.ToolName)
	if len(rec.RuleIDs) > 0 {
		add("rules", strings.Join(rec.RuleIDs, ", "))
	}

	if rec.CorroborationCount > 0 || len(rec.SourcesAgreed) > 0 || len(rec.SourcesDissented) > 0 {
		add("corroboration", fmt.Sprintf("%d source(s)", rec.CorroborationCount))
		if len(rec.SourcesAgreed) > 0 {
			add("  agreed", strings.Join(rec.SourcesAgreed, ", "))
		}
		if len(rec.SourcesDissented) > 0 {
			add("  dissented", strings.Join(rec.SourcesDissented, ", "))
		}
	}
	for _, m := range rec.CatalogMatches {
		sig := "unsigned"
		if m.Signed {
			sig = "signed"
		}
		add("  match", fmt.Sprintf("%s · %s:%s@%s · %s · %s",
			m.CatalogSource, m.Ecosystem, m.Package, m.Version, m.Severity, sig))
	}

	if rec.SentryRuleName != "" {
		add("sentry rule", rec.SentryRuleName+" ("+rec.SentrySeverity+")")
	}
	if rec.SentryProcessExe != "" {
		add("  process", fmt.Sprintf("%s [pid %d]", rec.SentryProcessExe, rec.SentryProcessPID))
	}
	if len(rec.SentryParentChain) > 0 {
		add("  parents", strings.Join(rec.SentryParentChain, " <- "))
	}
	if len(rec.SentryFilesAccessed) > 0 {
		add("  files", strings.Join(rec.SentryFilesAccessed, ", "))
	}
	if len(rec.SentryNetworkDests) > 0 {
		add("  network", strings.Join(rec.SentryNetworkDests, ", "))
	}
	add("  extension", rec.SentryCorrelatedExt)

	if rec.LLMFScanned {
		add("llamafirewall", fmt.Sprintf("%s · %s · %.2f", rec.LLMFScanKind, rec.LLMFResult, rec.LLMFConfidence))
	}

	add("agent id", rec.AgentID)
	if len(rec.AgentLineage) > 0 {
		add("  lineage", strings.Join(rec.AgentLineage, " > "))
	}
	add("cluster", rec.ClusterID)
	add("ruleset", rec.RulesetVersion)

	if len(out) == 0 {
		out = append(out, "    "+styleDimmer.Render("(no further detail)"))
	}
	return out
}
