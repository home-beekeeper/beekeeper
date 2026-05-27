package sentry

import (
	"path/filepath"
	"strings"
	"time"
)

// defaultSensitivePaths is the set of path substrings that the credential-access
// rule (SENTRY-001) considers sensitive. This list targets the most commonly
// harvested credential stores observed in real agent-compromise incidents.
var defaultSensitivePaths = []string{
	".ssh/", ".aws/", ".gnupg/", ".config/Claude/", ".config/op/",
	".config/gh/", ".netrc", ".npmrc", ".pypirc",
	".cargo/credentials", ".env",
}

// credentialCLIs is the set of binary names (base name only, no path) that the
// credential-CLI spawn rule (SENTRY-002) tracks. These tools can exfiltrate or
// rotate credentials when run from a compromised agent subprocess.
var credentialCLIs = map[string]bool{
	"gh":       true,
	"aws":      true,
	"op":       true,
	"vault":    true,
	"npm":      true,
	"gcloud":   true,
	"az":       true,
	"heroku":   true,
	"fly":      true,
	"vercel":   true,
	"netlify":  true,
	"supabase": true,
}

// editorExes is the set of editor executable base-names that the ancestor check
// uses to identify editor-descended processes.
var editorExes = map[string]bool{
	"code":          true,
	"code-insiders": true,
	"cursor":        true,
	"windsurf":      true,
	"codium":        true,
}

// isSensitivePath reports whether path contains any of the well-known sensitive
// credential-store substrings.
func isSensitivePath(path string) bool {
	for _, s := range defaultSensitivePaths {
		if strings.Contains(path, s) {
			return true
		}
	}
	return false
}

// isCredentialCLI reports whether the base name of exe is a known
// credential-management CLI.
func isCredentialCLI(exe string) bool {
	return credentialCLIs[filepath.Base(exe)]
}

// isEditorDescendant reports whether pid is a direct child of an editor process
// or is itself an editor process, by walking the PPID chain up to 32 hops. The
// function returns false if pid is not present in tree.
func isEditorDescendant(pid uint32, tree map[uint32]ProcessNode) bool {
	const maxDepth = 32
	current := pid
	for i := 0; i < maxDepth; i++ {
		node, ok := tree[current]
		if !ok {
			return false
		}
		if editorExes[filepath.Base(node.Exe)] {
			return true
		}
		if node.PPID == 0 || node.PPID == current {
			// Reached init/systemd or a cycle — stop.
			return false
		}
		current = node.PPID
	}
	return false
}

// expireWindow returns a new slice containing only entries whose SeenAt is at
// or after cutoff. It allocates a new slice to avoid mutating the original.
func expireWindow(entries []RuleWindowEntry, cutoff time.Time) []RuleWindowEntry {
	out := entries[:0:0] // zero-length, zero-cap — avoids aliasing
	for _, e := range entries {
		if !e.SeenAt.Before(cutoff) {
			out = append(out, e)
		}
	}
	return out
}

// applyDefaults fills zero-valued fields in cfg with the documented defaults.
func applyDefaults(cfg RuleConfig) RuleConfig {
	if cfg.CredAccessThreshold == 0 {
		cfg.CredAccessThreshold = 2
	}
	if cfg.CredCLIThreshold == 0 {
		cfg.CredCLIThreshold = 2
	}
	if cfg.CredAccessWindowSec == 0 {
		cfg.CredAccessWindowSec = 60 * time.Second
	}
	if cfg.CredCLIWindowSec == 0 {
		cfg.CredCLIWindowSec = 60 * time.Second
	}
	if cfg.PhoneHomeWindowMin == 0 {
		cfg.PhoneHomeWindowMin = 10 * time.Minute
	}
	if cfg.FreshExtWindowMin == 0 {
		cfg.FreshExtWindowMin = 30 * time.Minute
	}
	if cfg.ExfilFusionWindowMin == 0 {
		cfg.ExfilFusionWindowMin = 5 * time.Minute
	}
	return cfg
}

// buildParentChain walks tree from startPPID and collects up to 8 ancestor Exe
// values, stopping at PID 0 or if a node is missing.
func buildParentChain(startPPID uint32, tree map[uint32]ProcessNode) []string {
	const maxAncestors = 8
	var chain []string
	current := startPPID
	for i := 0; i < maxAncestors; i++ {
		node, ok := tree[current]
		if !ok || current == 0 {
			break
		}
		chain = append(chain, node.Exe)
		if node.PPID == 0 || node.PPID == current {
			break
		}
		current = node.PPID
	}
	return chain
}

// recordRecentAlert appends a dedup entry to state.RecentAlerts.
func recordRecentAlert(state *RuleState, ruleID string, pid uint32, now time.Time) {
	state.RecentAlerts = append(state.RecentAlerts, recentAlert{
		RuleID:  ruleID,
		PID:     pid,
		FiredAt: now,
	})
}

// makeAlert constructs a SentryAlert from common fields.
func makeAlert(ruleID, ruleName, severity string, event SentryEvent, tree map[uint32]ProcessNode, baseline BaselineState, now time.Time) SentryAlert {
	baselineMode := IsBaselineActive(baseline, now)
	return SentryAlert{
		RuleID:      ruleID,
		RuleName:    ruleName,
		Severity:    severity,
		BaselineMode: baselineMode,
		ProcessPID:   event.PID,
		ProcessExe:   event.Exe,
		ParentChain:  buildParentChain(event.PPID, tree),
		QuarantineRec: !baselineMode && severity == "critical",
		Timestamp:    now,
	}
}

// checkSENTRY004 checks whether any recently installed extension falls within
// the FreshExtWindowMin look-back. If so it returns a non-empty extension ID.
func checkSENTRY004(inventory InventorySnapshot, cfg RuleConfig, now time.Time) string {
	cutoff := now.Add(-cfg.FreshExtWindowMin)
	for extID, installTime := range inventory.RecentExtensions {
		if installTime.After(cutoff) || installTime.Equal(cutoff) {
			return extID
		}
	}
	return ""
}

// EvaluateEvent is the main entry point for the correlation engine. It accepts
// a single normalised SentryEvent, the mutable RuleState, a snapshot of the
// live process tree, an extension inventory snapshot, rule configuration, and
// the current baseline state. It returns zero or more SentryAlert values and
// mutates state in place.
//
// EvaluateEvent is a pure function with respect to side effects (no I/O, no
// goroutines) — callers serialise events onto a single goroutine.
func EvaluateEvent(
	event SentryEvent,
	state *RuleState,
	tree map[uint32]ProcessNode,
	inventory InventorySnapshot,
	cfg RuleConfig,
	baseline BaselineState,
	now time.Time,
) []SentryAlert {
	cfg = applyDefaults(cfg)

	var alerts []SentryAlert

	switch event.Kind {
	case EventFileAccess:
		alerts = append(alerts, evalSENTRY001(event, state, tree, inventory, cfg, baseline, now)...)

	case EventProcessCreate:
		alerts = append(alerts, evalSENTRY002(event, state, tree, inventory, cfg, baseline, now)...)

	case EventNetworkConnect:
		alerts = append(alerts, evalSENTRY003(event, state, tree, inventory, cfg, baseline, now)...)
		alerts = append(alerts, evalSENTRY005(event, state, tree, inventory, cfg, baseline, now)...)
	}

	return alerts
}

// evalSENTRY001 implements the credential-file-access clustering rule.
// Fires when an editor-descended process accesses ≥ CredAccessThreshold
// sensitive paths within CredAccessWindowSec.
func evalSENTRY001(
	event SentryEvent,
	state *RuleState,
	tree map[uint32]ProcessNode,
	inventory InventorySnapshot,
	cfg RuleConfig,
	baseline BaselineState,
	now time.Time,
) []SentryAlert {
	if !isEditorDescendant(event.PID, tree) {
		return nil
	}
	if !isSensitivePath(event.FilePath) {
		return nil
	}

	// Append the new entry.
	state.CredAccessByPID[event.PID] = append(state.CredAccessByPID[event.PID], RuleWindowEntry{
		PID:    event.PID,
		Value:  event.FilePath,
		SeenAt: now,
	})

	// Expire old entries.
	cutoff := now.Add(-cfg.CredAccessWindowSec)
	state.CredAccessByPID[event.PID] = expireWindow(state.CredAccessByPID[event.PID], cutoff)

	if len(state.CredAccessByPID[event.PID]) < cfg.CredAccessThreshold {
		return nil
	}

	// Collect file paths for the alert.
	var files []string
	for _, e := range state.CredAccessByPID[event.PID] {
		files = append(files, e.Value)
	}

	alert := makeAlert("SENTRY-001", "Credential File Access Cluster", "critical", event, tree, baseline, now)
	alert.FilesAccessed = files
	recordRecentAlert(state, "SENTRY-001", event.PID, now)

	var followup []SentryAlert
	// SENTRY-004 post-alert correlation.
	followup = append(followup, evalSENTRY004(event, inventory, cfg, baseline, now)...)

	return append([]SentryAlert{alert}, followup...)
}

// evalSENTRY002 implements the credential-CLI spawn clustering rule.
// Fires when an editor-descended process spawns ≥ CredCLIThreshold credential
// CLIs within CredCLIWindowSec.
func evalSENTRY002(
	event SentryEvent,
	state *RuleState,
	tree map[uint32]ProcessNode,
	inventory InventorySnapshot,
	cfg RuleConfig,
	baseline BaselineState,
	now time.Time,
) []SentryAlert {
	if !isEditorDescendant(event.PID, tree) {
		return nil
	}
	if !isCredentialCLI(event.Exe) {
		return nil
	}

	state.CredCLIByPID[event.PID] = append(state.CredCLIByPID[event.PID], RuleWindowEntry{
		PID:    event.PID,
		Value:  event.Exe,
		SeenAt: now,
	})

	cutoff := now.Add(-cfg.CredCLIWindowSec)
	state.CredCLIByPID[event.PID] = expireWindow(state.CredCLIByPID[event.PID], cutoff)

	if len(state.CredCLIByPID[event.PID]) < cfg.CredCLIThreshold {
		return nil
	}

	var clis []string
	for _, e := range state.CredCLIByPID[event.PID] {
		clis = append(clis, e.Value)
	}

	alert := makeAlert("SENTRY-002", "Credential CLI Spawn Cluster", "critical", event, tree, baseline, now)
	_ = clis // available for future enrichment
	recordRecentAlert(state, "SENTRY-002", event.PID, now)

	var followup []SentryAlert
	followup = append(followup, evalSENTRY004(event, inventory, cfg, baseline, now)...)

	return append([]SentryAlert{alert}, followup...)
}

// evalSENTRY003 implements the phone-home detection rule.
// Fires on the first outbound network connection by an editor-descended process
// within the PhoneHomeWindowMin sliding window.
func evalSENTRY003(
	event SentryEvent,
	state *RuleState,
	tree map[uint32]ProcessNode,
	inventory InventorySnapshot,
	cfg RuleConfig,
	baseline BaselineState,
	now time.Time,
) []SentryAlert {
	if !isEditorDescendant(event.PID, tree) {
		return nil
	}

	// Expire old entries first.
	cutoff := now.Add(-cfg.PhoneHomeWindowMin)
	state.PhoneHomeByPID[event.PID] = expireWindow(state.PhoneHomeByPID[event.PID], cutoff)

	dest := event.DstAddr.String()

	// Record the new entry.
	state.PhoneHomeByPID[event.PID] = append(state.PhoneHomeByPID[event.PID], RuleWindowEntry{
		PID:    event.PID,
		Value:  dest,
		SeenAt: now,
	})

	// Fire on the first qualifying connection per window (count becomes 1 after append).
	if len(state.PhoneHomeByPID[event.PID]) != 1 {
		return nil
	}

	var dests []string
	for _, e := range state.PhoneHomeByPID[event.PID] {
		dests = append(dests, e.Value)
	}

	alert := makeAlert("SENTRY-003", "Unexpected Outbound Connection", "high", event, tree, baseline, now)
	alert.NetworkDests = dests
	recordRecentAlert(state, "SENTRY-003", event.PID, now)

	var followup []SentryAlert
	followup = append(followup, evalSENTRY004(event, inventory, cfg, baseline, now)...)

	return append([]SentryAlert{alert}, followup...)
}

// evalSENTRY004 is called after any SENTRY-001/002/003 alert fires. It checks
// whether any extension was installed within FreshExtWindowMin and, if so,
// emits a SENTRY-004 correlation alert.
func evalSENTRY004(
	event SentryEvent,
	inventory InventorySnapshot,
	cfg RuleConfig,
	baseline BaselineState,
	now time.Time,
) []SentryAlert {
	extID := checkSENTRY004(inventory, cfg, now)
	if extID == "" {
		return nil
	}

	baselineMode := IsBaselineActive(baseline, now)
	alert := SentryAlert{
		RuleID:              "SENTRY-004",
		RuleName:            "Fresh Extension Correlation",
		Severity:            "high",
		BaselineMode:        baselineMode,
		ProcessPID:          event.PID,
		ProcessExe:          event.Exe,
		CorrelatedExtension: extID,
		QuarantineRec:       false, // SENTRY-004 severity is "high", not "critical"
		Timestamp:           now,
	}
	return []SentryAlert{alert}
}

// evalSENTRY005 implements the exfiltration-fusion rule.
// Fires on EventNetworkConnect when an editor-descended PID has both a recent
// sensitive-file read AND a recently installed extension within ExfilFusionWindowMin.
func evalSENTRY005(
	event SentryEvent,
	state *RuleState,
	tree map[uint32]ProcessNode,
	inventory InventorySnapshot,
	cfg RuleConfig,
	baseline BaselineState,
	now time.Time,
) []SentryAlert {
	if !isEditorDescendant(event.PID, tree) {
		return nil
	}

	cutoff := now.Add(-cfg.ExfilFusionWindowMin)

	// Check for a recent sensitive-file access by this PID.
	hasRecentCredAccess := false
	for _, e := range state.CredAccessByPID[event.PID] {
		if !e.SeenAt.Before(cutoff) {
			hasRecentCredAccess = true
			break
		}
	}
	if !hasRecentCredAccess {
		return nil
	}

	// Check for a recently installed extension.
	extID := ""
	for id, installTime := range inventory.RecentExtensions {
		if !installTime.Before(cutoff) {
			extID = id
			break
		}
	}
	if extID == "" {
		return nil
	}

	baselineMode := IsBaselineActive(baseline, now)
	alert := SentryAlert{
		RuleID:              "SENTRY-005",
		RuleName:            "Exfiltration Fusion",
		Severity:            "critical",
		BaselineMode:        baselineMode,
		ProcessPID:          event.PID,
		ProcessExe:          event.Exe,
		ParentChain:         buildParentChain(event.PPID, tree),
		CorrelatedExtension: extID,
		QuarantineRec:       !baselineMode,
		Timestamp:           now,
	}
	recordRecentAlert(state, "SENTRY-005", event.PID, now)
	return []SentryAlert{alert}
}
