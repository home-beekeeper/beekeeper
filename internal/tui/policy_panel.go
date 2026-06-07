package tui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	platform "github.com/bantuson/beekeeper/internal/platform"
	policyloader "github.com/bantuson/beekeeper/internal/policyloader"
)

// policyEditErrMsg is emitted by the policy panel when an edit is rejected by the
// validation gate (policyloader.SavePolicyFile). The App shows it as a warn toast
// and the on-disk policy file is left unchanged.
type policyEditErrMsg struct{ msg string }

// policySavedMsg is emitted after an add/remove edit persists successfully.
type policySavedMsg struct{ msg string }

type policyEditMode int

const (
	editView policyEditMode = iota
	editAddAllow
	editAddSens
)

type policyRowKind int

const (
	rowCorr policyRowKind = iota
	rowAllow
	rowSens
	rowAddAllow
	rowAddSens
	rowInfo
)

// policyRow is one rendered/navigable line in the editor.
type policyRow struct {
	kind      policyRowKind
	header    string // section header rendered above this row when non-empty
	label     string
	value     string
	corrField string // rowCorr: "warn"|"block"|"quarantine"|"critical"
	ruleIdx   int    // rowAllow/rowSens: index into pf.Rules
}

// PolicyPanel implements PanelContent for the policy & thresholds overlay.
//
// Unlike the retired prototype, this panel edits the REAL, enforced policy file
// (policyloader.ManagedPolicyName in ~/.beekeeper/policies — the same directory
// beekeeper check reads). Every edit is validated by policyloader.SavePolicyFile
// BEFORE it is written, so an invalid edit is rejected in the TUI and never
// persisted: the dashboard is the user's source of truth and the last gate.
//
// Editable (the rule types the backend actually enforces from policy files):
//   - corroboration_threshold (warn/block/quarantine/critical) via +/-
//   - package_allowlist (trust overrides) via add/remove
//   - sensitive_path (deny) via add/remove
//
// Toggle/edit keys are gated on adminMode (mirrors QuarantinePanel's r/p gate).
type PolicyPanel struct {
	adminMode   bool
	policiesDir string
	pf          policyloader.PolicyFile
	rows        []policyRow
	selIdx      int
	editMode    policyEditMode
	buf         string
}

// NewPolicyPanel creates a PolicyPanel bound to the managed policy file in the
// shared policies directory. It seeds the file with the engine default
// thresholds on first run (via LoadOrSeedManagedPolicy).
func NewPolicyPanel(adminMode bool) *PolicyPanel {
	stateDir, err := platform.StateDir()
	if err != nil {
		stateDir = "."
	}
	p := &PolicyPanel{
		adminMode:   adminMode,
		policiesDir: filepath.Join(stateDir, "policies"),
		editMode:    editView,
	}
	p.reload()
	return p
}

// reload loads (or seeds) the managed policy file and rebuilds the row model.
// On a load error (e.g. a corrupt managed file) it keeps the current in-memory
// file — fail-soft, never panics.
func (p *PolicyPanel) reload() {
	pf, errs := policyloader.LoadOrSeedManagedPolicy(p.policiesDir)
	if len(errs) == 0 {
		p.pf = pf
	}
	p.rows = buildPolicyRows(p.pf)
	p.clampSel()
}

func (p *PolicyPanel) clampSel() {
	if p.selIdx >= len(p.rows) {
		p.selIdx = len(p.rows) - 1
	}
	if p.selIdx < 0 {
		p.selIdx = 0
	}
}

// buildPolicyRows derives the navigable/rendered rows from a policy file. The
// corroboration values shown are the EFFECTIVE thresholds (what beekeeper check
// would compute), so display always matches enforcement.
func buildPolicyRows(pf policyloader.PolicyFile) []policyRow {
	th := policyloader.ThresholdsFromPolicyFiles([]policyloader.PolicyFile{pf})
	critical := 0
	if ov, ok := th.SeverityOverrides["critical"]; ok {
		critical = ov.BlockAt
	}

	rows := []policyRow{
		{kind: rowCorr, header: "Corroboration thresholds  (signed sources required)", label: "warn at", value: strconv.Itoa(th.WarnAt), corrField: "warn"},
		{kind: rowCorr, label: "block at", value: strconv.Itoa(th.BlockAt), corrField: "block"},
		{kind: rowCorr, label: "quarantine at", value: strconv.Itoa(th.QuarantineAt), corrField: "quarantine"},
		{kind: rowCorr, label: "critical block at", value: strconv.Itoa(critical), corrField: "critical"},
	}

	const allowHdr = "Package allowlist  (trusted — overrides a block)"
	firstAllow := true
	for i, r := range pf.Rules {
		if r.RuleType != "package_allowlist" {
			continue
		}
		hdr := ""
		if firstAllow {
			hdr, firstAllow = allowHdr, false
		}
		rows = append(rows, policyRow{kind: rowAllow, header: hdr, label: r.Ecosystem, value: strings.Join(r.Packages, ", "), ruleIdx: i})
	}
	addAllowHdr := ""
	if firstAllow {
		addAllowHdr = allowHdr
	}
	rows = append(rows, policyRow{kind: rowAddAllow, header: addAllowHdr, label: "+ add allowlist entry"})

	const sensHdr = "Sensitive paths  (denied)"
	firstSens := true
	for i, r := range pf.Rules {
		if r.RuleType != "sensitive_path" {
			continue
		}
		hdr := ""
		if firstSens {
			hdr, firstSens = sensHdr, false
		}
		action := r.Action
		if action == "" {
			action = "block"
		}
		rows = append(rows, policyRow{kind: rowSens, header: hdr, label: strings.Join(r.PathPatterns, ", "), value: action, ruleIdx: i})
	}
	addSensHdr := ""
	if firstSens {
		addSensHdr = sensHdr
	}
	rows = append(rows, policyRow{kind: rowAddSens, header: addSensHdr, label: "+ add sensitive path"})

	// Honest read-only rows: things this panel does NOT control.
	rows = append(rows,
		policyRow{kind: rowInfo, header: "Not editable here", label: "release-age · lifecycle", value: "catalog/config-driven"},
		policyRow{kind: rowInfo, label: "llamafirewall · sentry baseline", value: "config.json"},
	)
	return rows
}

// Update implements PanelContent. Navigation always works; edits are admin-gated.
func (p *PolicyPanel) Update(msg tea.Msg) (PanelContent, tea.Cmd) {
	switch msg := msg.(type) {
	case stateTick:
		// Reload so external edits surface — but never while a text entry is in
		// progress, or the 5s tick would clobber the user's in-flight input.
		if p.editMode == editView {
			p.reload()
		}
	case tea.KeyPressMsg:
		return p, p.handleKey(msg.String())
	}
	return p, nil
}

// handleKey is the version-independent key dispatch (keyed on the string form so
// it is unit-testable without constructing a tea.KeyPressMsg). Returns a tea.Cmd
// carrying a toast message when an edit succeeds or is rejected.
func (p *PolicyPanel) handleKey(k string) tea.Cmd {
	// Text-input sub-mode captures keys for add-entry flows.
	if p.editMode != editView {
		switch k {
		case "esc":
			p.editMode = editView
			p.buf = ""
		case "enter":
			return p.commitInput()
		case "backspace":
			if len(p.buf) > 0 {
				p.buf = p.buf[:len(p.buf)-1]
			}
		case "space":
			p.buf += " "
		default:
			if len(k) == 1 {
				p.buf += k
			}
		}
		return nil
	}

	// Navigation (available to all users).
	switch k {
	case "j", "down":
		if p.selIdx < len(p.rows)-1 {
			p.selIdx++
		}
		return nil
	case "k", "up":
		if p.selIdx > 0 {
			p.selIdx--
		}
		return nil
	}

	// Edits require admin mode.
	if !p.adminMode {
		return nil
	}

	switch k {
	case "+", "=":
		if r := p.curRow(); r != nil && r.kind == rowCorr {
			return p.adjustCorr(r.corrField, 1)
		}
	case "-", "_":
		if r := p.curRow(); r != nil && r.kind == rowCorr {
			return p.adjustCorr(r.corrField, -1)
		}
	case "a", "A", "enter":
		if r := p.curRow(); r != nil {
			switch r.kind {
			case rowAddAllow:
				p.editMode = editAddAllow
				p.buf = ""
			case rowAddSens:
				p.editMode = editAddSens
				p.buf = ""
			}
		}
	case "d", "D", "x", "X":
		return p.deleteSelected()
	}
	return nil
}

func (p *PolicyPanel) curRow() *policyRow {
	if p.selIdx < 0 || p.selIdx >= len(p.rows) {
		return nil
	}
	return &p.rows[p.selIdx]
}

// adjustCorr changes a corroboration threshold field by delta (floored at 1),
// then validates+persists. It edits the EFFECTIVE value so display and edit stay
// consistent regardless of which fields the rule sets explicitly.
func (p *PolicyPanel) adjustCorr(field string, delta int) tea.Cmd {
	cand := clonePolicyFile(p.pf)
	idx := corrRuleIndex(cand)
	if idx < 0 {
		cand.Rules = append(cand.Rules, policyloader.DefaultManagedPolicy().Rules[0])
		idx = len(cand.Rules) - 1
	}
	th := policyloader.ThresholdsFromPolicyFiles([]policyloader.PolicyFile{cand})
	cur := 0
	switch field {
	case "warn":
		cur = th.WarnAt
	case "block":
		cur = th.BlockAt
	case "quarantine":
		cur = th.QuarantineAt
	case "critical":
		if ov, ok := th.SeverityOverrides["critical"]; ok {
			cur = ov.BlockAt
		}
	}
	newVal := cur + delta
	if newVal < 1 {
		newVal = 1
	}
	r := &cand.Rules[idx]
	switch field {
	case "warn":
		r.WarnAt = newVal
	case "block":
		r.BlockAt = newVal
	case "quarantine":
		r.QuarantineAt = newVal
	case "critical":
		r.CriticalBlockAt = newVal
	}
	return p.persist(cand, "") // silent on success to avoid toast spam on +/-
}

// commitInput finalizes a text-entry add flow.
func (p *PolicyPanel) commitInput() tea.Cmd {
	buf := strings.TrimSpace(p.buf)
	mode := p.editMode
	p.editMode = editView
	p.buf = ""
	if buf == "" {
		return nil
	}
	cand := clonePolicyFile(p.pf)
	switch mode {
	case editAddAllow:
		eco, pkg, ok := splitEcoPkg(buf)
		if !ok {
			return func() tea.Msg {
				return policyEditErrMsg{msg: "allowlist entry must be ecosystem:package (e.g. npm:react)"}
			}
		}
		cand.Rules = append(cand.Rules, policyloader.PolicyRule{
			ID:        uniquePolicyRuleID(cand, "tui-allow-"+eco+"-"+pkg),
			RuleType:  "package_allowlist",
			Ecosystem: eco,
			Packages:  []string{pkg},
			Action:    "allow",
		})
		return p.persist(cand, "allowlist entry added")
	case editAddSens:
		cand.Rules = append(cand.Rules, policyloader.PolicyRule{
			ID:           uniquePolicyRuleID(cand, "tui-spath-"+buf),
			RuleType:     "sensitive_path",
			PathPatterns: []string{buf},
			Action:       "block",
		})
		return p.persist(cand, "sensitive path added")
	}
	return nil
}

// deleteSelected removes the allowlist/sensitive_path entry under the cursor.
func (p *PolicyPanel) deleteSelected() tea.Cmd {
	r := p.curRow()
	if r == nil || (r.kind != rowAllow && r.kind != rowSens) {
		return nil
	}
	cand := clonePolicyFile(p.pf)
	if r.ruleIdx < 0 || r.ruleIdx >= len(cand.Rules) {
		return nil
	}
	cand.Rules = append(cand.Rules[:r.ruleIdx], cand.Rules[r.ruleIdx+1:]...)
	return p.persist(cand, "entry removed")
}

// persist is the single write path / last gate: it validates+writes via
// policyloader.SavePolicyFile. On rejection it emits policyEditErrMsg and leaves
// disk unchanged; on success it reloads and (optionally) emits policySavedMsg.
func (p *PolicyPanel) persist(cand policyloader.PolicyFile, okMsg string) tea.Cmd {
	if errs := policyloader.SavePolicyFile(policyloader.ManagedPolicyPath(p.policiesDir), cand); len(errs) > 0 {
		m := errs[0].Error()
		return func() tea.Msg { return policyEditErrMsg{msg: m} }
	}
	p.reload()
	if okMsg != "" {
		return func() tea.Msg { return policySavedMsg{msg: okMsg} }
	}
	return nil
}

// Title implements PanelContent.
func (p *PolicyPanel) Title() string { return "Policy & thresholds" }

// Count implements PanelContent.
func (p *PolicyPanel) Count() string {
	allow, sens := 0, 0
	for _, r := range p.pf.Rules {
		switch r.RuleType {
		case "package_allowlist":
			allow++
		case "sensitive_path":
			sens++
		}
	}
	return fmt.Sprintf("%d allow · %d paths", allow, sens)
}

// Padded implements PanelContent.
func (p *PolicyPanel) Padded() bool { return true }

// Critical implements PanelContent.
func (p *PolicyPanel) Critical() bool { return false }

// Body implements PanelContent.
func (p *PolicyPanel) Body(width, height int) string {
	lines := []string{""}
	for i, row := range p.rows {
		if row.header != "" {
			if i != 0 {
				lines = append(lines, "")
			}
			lines = append(lines, "  "+styleDimmer.Render(row.header))
		}
		lines = append(lines, p.renderRow(i, row))
	}

	if p.editMode != editView {
		prompt := "new allowlist entry (ecosystem:package)"
		if p.editMode == editAddSens {
			prompt = "new sensitive path pattern"
		}
		lines = append(lines, "", "  "+styleTeal.Render(prompt+": ")+styleWhite.Render(p.buf)+styleTeal.Render("▏"))
	}

	lines = append(lines, "", "  "+styleDimmer.Render("Validated against the live policy schema before saving · enforced by beekeeper check"))
	return strings.Join(lines, "\n")
}

func (p *PolicyPanel) renderRow(i int, row policyRow) string {
	var text string
	switch row.kind {
	case rowCorr:
		text = "  " + styleDim.Render(fmt.Sprintf("%-20s", row.label)) + styleTeal.Render(row.value)
	case rowAllow:
		text = "  " + styleGreen.Render("allow ") + styleDim.Render(fmt.Sprintf("%-10s", row.label)) + styleWhite.Render(row.value)
	case rowSens:
		text = "  " + styleRed.Render("deny  ") + styleWhite.Render(row.label) + styleDim.Render("  ("+row.value+")")
	case rowAddAllow, rowAddSens:
		text = "  " + styleTeal.Render(row.label)
	case rowInfo:
		text = "  " + styleDimmer.Render(fmt.Sprintf("%-34s", row.label)+row.value)
	}
	if i == p.selIdx && p.editMode == editView {
		text = styleSelRow.Render(strings.TrimRight(text, " "))
	}
	return text
}

// Footer implements PanelContent.
func (p *PolicyPanel) Footer() string {
	if p.editMode != editView {
		return styleTeal.Render("type") + styleDim.Render(" entry · ") +
			styleTeal.Render("enter") + styleDim.Render(" save · ") +
			styleTeal.Render("esc") + styleDim.Render(" cancel")
	}
	if p.adminMode {
		return styleTeal.Render("+/-") + styleDim.Render(" adjust · ") +
			styleTeal.Render("a") + styleDim.Render(" add · ") +
			styleTeal.Render("d") + styleDim.Render(" delete · ") +
			styleTeal.Render("↑↓") + styleDim.Render(" select · ") +
			styleTeal.Render("esc") + styleDim.Render(" close")
	}
	return styleTeal.Render("↑↓") + styleDim.Render(" select · ") +
		styleTeal.Render("esc") + styleDim.Render(" close · ") +
		styleDimmer.Render("--admin to edit")
}

// --- helpers ---

func clonePolicyFile(pf policyloader.PolicyFile) policyloader.PolicyFile {
	out := pf
	out.Rules = make([]policyloader.PolicyRule, len(pf.Rules))
	for i, r := range pf.Rules {
		nr := r
		nr.Ecosystems = append([]string(nil), r.Ecosystems...)
		nr.Packages = append([]string(nil), r.Packages...)
		nr.PathPatterns = append([]string(nil), r.PathPatterns...)
		out.Rules[i] = nr
	}
	return out
}

func corrRuleIndex(pf policyloader.PolicyFile) int {
	for i, r := range pf.Rules {
		if r.RuleType == "corroboration_threshold" {
			return i
		}
	}
	return -1
}

func splitEcoPkg(s string) (eco, pkg string, ok bool) {
	i := strings.Index(s, ":")
	if i <= 0 || i >= len(s)-1 {
		return "", "", false
	}
	eco = strings.ToLower(strings.TrimSpace(s[:i]))
	pkg = strings.TrimSpace(s[i+1:])
	if eco == "" || pkg == "" {
		return "", "", false
	}
	return eco, pkg, true
}

func uniquePolicyRuleID(pf policyloader.PolicyFile, base string) string {
	base = policySlug(base)
	id := base
	for n := 2; policyHasRuleID(pf, id); n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	return id
}

func policyHasRuleID(pf policyloader.PolicyFile, id string) bool {
	for _, r := range pf.Rules {
		if r.ID == id {
			return true
		}
	}
	return false
}

func policySlug(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		case !prevDash:
			b.WriteByte('-')
			prevDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "entry"
	}
	return out
}
