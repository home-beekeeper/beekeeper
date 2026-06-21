package sentry

import (
	"strings"
	"time"
)

// install_observe.go - SENTRY-009: detection-only observation of install-class
// process spawns (IPST-06). DETECTION, NEVER PREVENTION.
//
// -- What this does -------------------------------------------------------------
// When a monitored-descendant process (editor- or agent-descended, the SAME
// ancestry gate the other Sentry rules use) is observed spawning a package-manager
// install (npm/pnpm/bun/yarn install|add, pip install, cargo add|install, etc.),
// Sentry emits a detection-only audit record attributing the install: the
// package-manager process PID/PPID, exe, and cmdline. This covers installs the
// pre-exec hook cannot see - a human running `npm install` directly in an editor
// terminal, or a harness with no pre-exec hook - closing the boundary the hook
// leaves open (see posture.BoundaryStatement / IPBND-01).
//
// -- HONEST SCOPE (do NOT overstate) --------------------------------------------
// Sentry records THAT an install happened (process attribution only). It does
// NOT fetch the registry to compute release-age or lifecycle-script posture in
// the privileged daemon - we deliberately add NO network I/O to the privileged
// tier. Posture MATCHING is the hook's job (internal/check/posture_adapter.go).
// This rule must never imply Sentry evaluates posture; it only observes the spawn.
//
// -- Detection-only invariant ---------------------------------------------------
// The emitted alert carries Severity "info" and QuarantineRec=false, so the
// daemons' alertToAuditRecord maps it to an "observe" decision and the
// sentry_install_observed record type. Sentry takes NO block/kill/quarantine
// action here (or anywhere - it is detection-only by design).

// installManagers maps a process base-name to its ecosystem-agnostic manager key.
// These are the SAME managers pkgparse recognizes at the hook; kept as a local
// pure table so the sentry package stays free of a pkgparse dependency and import
// purity (the engine imports only net/strings/path/filepath/time).
var installManagers = map[string]bool{
	"npm": true, "pnpm": true, "bun": true, "yarn": true,
	"pip": true, "pip3": true, "cargo": true, "gem": true,
	"composer": true, "go": true,
}

// installVerbs are the install-class sub-commands. A manager process whose
// cmdline contains one of these verbs as a token is treated as an install spawn.
// "require" covers `composer require`; "get" covers `go get`.
var installVerbs = map[string]bool{
	"install": true, "add": true, "i": true,
	"get": true, "require": true,
}

// isInstallProcess reports whether a process (identified by its exe base-name and
// full cmdline) is an install-class package-manager invocation. It is a pure,
// conservative token scan - no shell parsing, no I/O - intended only to LABEL an
// observed spawn, not to gate enforcement (the hook owns enforcement).
//
// It requires BOTH a known manager base-name AND an install verb token in the
// cmdline so that benign manager invocations (`npm run build`, `go test`,
// `pip list`) do not produce a spurious install observation.
func isInstallProcess(exe, cmdline string) bool {
	if !installManagers[exeBaseName(exe)] {
		return false
	}
	for _, tok := range strings.Fields(strings.ToLower(cmdline)) {
		if installVerbs[tok] {
			return true
		}
	}
	return false
}

// evalSENTRY009 emits a detection-only install-observation alert when a
// monitored-descendant process spawns an install-class package manager.
//
// It reuses isMonitoredDescendant (editor OR agent ancestry) so it covers both
// editor-terminal human installs and standalone-agent installs. It is
// detection-only: Severity "info", QuarantineRec false, no kill/quarantine.
//
// No window/state is kept - every observed install spawn produces exactly one
// observation record (process spawns are discrete, unlike the clustering rules).
func evalSENTRY009(
	event SentryEvent,
	tree map[uint32]ProcessNode,
	baseline BaselineState,
	now time.Time,
) []SentryAlert {
	if !isMonitoredDescendant(event.PID, tree) {
		return nil
	}
	if !isInstallProcess(event.Exe, event.Cmdline) {
		return nil
	}

	alert := makeAlert("SENTRY-009", "Install Observed", "info", event, tree, baseline, now)
	// QuarantineRec is already false for a non-critical severity (makeAlert only
	// sets it true for critical), but make the detection-only intent explicit and
	// defensive against a future makeAlert change.
	alert.QuarantineRec = false
	// Carry the install attribution: the manager process and its argv. FilesAccessed
	// is reused as the attribution slot (it flows to the audit record's
	// sentry_files_accessed field, redacted like every other Sentry field).
	if event.Cmdline != "" {
		alert.FilesAccessed = []string{event.Cmdline}
	}
	return []SentryAlert{alert}
}
