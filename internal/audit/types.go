// Package audit provides the Phase 1 NDJSON audit log: a Bumblebee-compatible
// append-only writer that records one record per policy decision with
// owner-only file permissions.
//
// Every decision the hook handler makes — including fail-closed decisions —
// must be written here. The schema is Bumblebee-compatible (AUDT-01 minimum,
// CTLG-07 signedness provenance); full audit sinks (syslog, OTLP, query,
// export, rotation) are Phase 6 and deliberately out of scope.
package audit

import "github.com/mzansi-agentive/beekeeper/internal/policy"

// AuditRecord is one NDJSON line in the audit log: a single policy decision
// with its provenance. ScannerName is always the literal "beekeeper" and
// Endpoint is "check" in Phase 1 (the only decision surface). RecordID and
// Timestamp are caller-supplied so that FromDecision stays a pure mapping and
// is trivially testable; the hook handler supplies real values at runtime.
type AuditRecord struct {
	RecordType     string              `json:"record_type"` // "policy_decision"
	RecordID       string              `json:"record_id"`
	Timestamp      string              `json:"timestamp"`    // RFC3339
	ScannerName    string              `json:"scanner_name"` // always "beekeeper"
	AgentName      string              `json:"agent_name"`
	ToolName       string              `json:"tool_name"`
	Decision       string              `json:"decision"` // allow|warn|block
	Reason         string              `json:"reason"`
	RuleIDs        []string            `json:"rule_ids"`
	CatalogMatches []CatalogProvenance `json:"catalog_matches"`
	Endpoint       string              `json:"endpoint"` // "check" in Phase 1
}

// CatalogProvenance is the audit-record view of a single catalog hit. It mirrors
// policy.CatalogMatch field-for-field, including the Signed flag (CTLG-07), so
// the audit log records exactly which catalog source, entry, and signedness
// drove a decision.
type CatalogProvenance struct {
	CatalogSource string `json:"catalog_source"`
	EntryID       string `json:"entry_id"`
	Ecosystem     string `json:"ecosystem"`
	Package       string `json:"package"`
	Version       string `json:"version"`
	Severity      string `json:"severity"`
	Signed        bool   `json:"signed"`
}

// FromDecision maps a policy Decision plus the originating tool call and
// caller-supplied metadata into an AuditRecord. It performs no I/O and reads no
// wall clock — recordID and timestamp are passed in verbatim — so it remains a
// pure, side-effect-free mapping that the hook handler and tests both rely on.
func FromDecision(tc policy.ToolCall, d policy.Decision, recordID, timestamp string) AuditRecord {
	matches := make([]CatalogProvenance, 0, len(d.CatalogMatches))
	for _, m := range d.CatalogMatches {
		matches = append(matches, CatalogProvenance{
			CatalogSource: m.CatalogSource,
			EntryID:       m.EntryID,
			Ecosystem:     m.Ecosystem,
			Package:       m.Package,
			Version:       m.Version,
			Severity:      m.Severity,
			Signed:        m.Signed,
		})
	}

	return AuditRecord{
		RecordType:     "policy_decision",
		RecordID:       recordID,
		Timestamp:      timestamp,
		ScannerName:    "beekeeper",
		AgentName:      tc.AgentName,
		ToolName:       tc.ToolName,
		Decision:       d.Level,
		Reason:         d.Reason,
		RuleIDs:        d.RuleIDs,
		CatalogMatches: matches,
		Endpoint:       "check",
	}
}
