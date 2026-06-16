// Package audit provides the Phase 1 NDJSON audit log: a Bumblebee-compatible
// append-only writer that records one record per policy decision with
// owner-only file permissions.
//
// Every decision the hook handler makes — including fail-closed decisions —
// must be written here. The schema is Bumblebee-compatible (AUDT-01 minimum,
// CTLG-07 signedness provenance); full audit sinks (syslog, OTLP, query,
// export, rotation) are Phase 6 and deliberately out of scope.
package audit

import "github.com/home-beekeeper/beekeeper/internal/policy"

// AuditRecord is one NDJSON line in the audit log: a single policy decision
// with its provenance. ScannerName is always the literal "beekeeper" and
// Endpoint is "check" in Phase 1 (the only decision surface). RecordID and
// Timestamp are caller-supplied so that FromDecision stays a pure mapping and
// is trivially testable; the hook handler supplies real values at runtime.
//
// Phase 2 additions (CTLG-09): CorroborationCount, SourcesAgreed, SourcesDissented,
// and Quarantine carry the full corroboration provenance so operators know exactly
// which sources agreed/dissented on every decision.
type AuditRecord struct {
	RecordType     string              `json:"record_type"` // "policy_decision"
	RecordID       string              `json:"record_id"`
	Timestamp      string              `json:"timestamp"`    // RFC3339
	ScannerName    string              `json:"scanner_name"` // always "beekeeper"
	AgentName      string              `json:"agent_name"`
	ToolName       string              `json:"tool_name"`
	Decision       string              `json:"decision"` // allow|warn|block|alert (alert = Sentry detection-only, set by the corpus emitter in Phase 23)
	Reason         string              `json:"reason"`
	RuleIDs        []string            `json:"rule_ids"`
	CatalogMatches []CatalogProvenance `json:"catalog_matches"`
	Endpoint       string              `json:"endpoint"` // "check" in Phase 1
	// Phase 2 additions (CTLG-09):
	CorroborationCount int      `json:"corroboration_count"`
	SourcesAgreed      []string `json:"sources_agreed"`
	SourcesDissented   []string `json:"sources_dissented"`
	Quarantine         bool     `json:"quarantine,omitempty"`
	// Phase 4 additions (INTG-07): multi-agent lineage for forensic audit trail.
	AgentID       string   `json:"agent_id,omitempty"`
	ParentAgentID string   `json:"parent_agent_id,omitempty"`
	AgentDepth    int      `json:"agent_depth,omitempty"`
	AgentLineage  []string `json:"agent_lineage,omitempty"`
	// Phase 5 additions: sentry_alert record type (SLNX-08)
	SentryRuleID        string   `json:"sentry_rule_id,omitempty"`
	SentryRuleName      string   `json:"sentry_rule_name,omitempty"`
	SentrySeverity      string   `json:"sentry_severity,omitempty"`
	SentryBaselineMode  bool     `json:"sentry_baseline_mode,omitempty"`
	SentryProcessPID    uint32   `json:"sentry_process_pid,omitempty"`
	SentryProcessExe    string   `json:"sentry_process_exe,omitempty"`
	SentryParentChain   []string `json:"sentry_parent_chain,omitempty"`
	SentryFilesAccessed []string `json:"sentry_files_accessed,omitempty"`
	SentryNetworkDests  []string `json:"sentry_network_dests,omitempty"`
	SentryCorrelatedExt string   `json:"sentry_correlated_ext,omitempty"`
	SentryQuarantineRec bool     `json:"sentry_quarantine_recommended,omitempty"`
	// Phase 6 additions (LLMF-02, LLMF-03, LLMF-04)
	LLMFScanned    bool    `json:"llmf_scanned,omitempty"`
	LLMFScanKind   string  `json:"llmf_scan_kind,omitempty"`   // prompt|code|alignment
	LLMFResult     string  `json:"llmf_result,omitempty"`      // clean|injection|unsafe|hijacked
	LLMFConfidence float64 `json:"llmf_confidence,omitempty"`
	LLMFLatencyMS  int64   `json:"llmf_latency_ms,omitempty"`
	// Phase 8 additions (NUDGE-06): package-manager nudge provenance.
	//
	// These fields carry the nudge decision provenance required by PRD §9.
	// The existing Decision field (json:"decision") retains the repo's
	// allow|warn|block vocabulary and MUST NOT be changed — it is the catalog
	// and policy decision level used by all existing consumers.
	//
	// NudgeAction carries the PRD §9 "decision" vocabulary (advise|proceed|
	// rewrite|block), which is distinct from the repo Level enum. A soft
	// advisory therefore has Decision:"warn" (Level) AND NudgeAction:"advise"
	// (§9), resolving the §9 vs repo enum mismatch without disturbing existing
	// consumers.
	//
	// record_type "nudge" and "version_drift" join the existing set
	// ("policy_decision", "tool_result", "llmf_alert", "sentry_alert").
	// Callers set RecordType explicitly — FromDecision is a pure mapping that
	// continues to produce "policy_decision" records; callers set RecordType
	// and the nudge fields after, as handler.go does at the tool_result site.
	OriginalCommand  string `json:"original_command,omitempty"`
	RewrittenCommand string `json:"rewritten_command,omitempty"`
	ReasonCode       string `json:"reason_code,omitempty"`
	PMState          string `json:"pm_state,omitempty"` // flattened JSON-string view per §9
	NudgeAction      string `json:"nudge_action,omitempty"` // closed §9 enum: advise|proceed|rewrite|block

	// Phase 22 corpus-schema additions (SCHEMA-01/02/05):
	// These three fields are additive omitempty — existing audit consumers are
	// unaffected (records written before Phase 22 serialize without these keys).
	// They are set by the corpus emitter (Phase 23) or directly by the Sentry
	// surface, not by FromDecision (which continues to produce policy_decision
	// records unchanged).

	// SourceSurface is the branch key identifying which Beekeeper surface produced
	// this record. Valid values: hook|mcp_gateway|shim|file_watcher|sentry|scan.
	// Populated by the corpus emitter (Phase 23) from context, or by the Sentry
	// surface directly. (SCHEMA-01)
	SourceSurface string `json:"source_surface,omitempty"`

	// ClusterID binds correlated non-agent events (e.g. a Sentry correlation
	// window). Agent-mediated surfaces (hook/mcp_gateway/shim) adjudicate per
	// event; non-agent surfaces (file_watcher/sentry/scan) adjudicate per cluster.
	// Sentry surface may set this directly; the corpus emitter reads it. (SCHEMA-02)
	ClusterID string `json:"cluster_id,omitempty"`

	// RulesetVersion is the catalog snapshot version at decision time. Populated
	// by the policy loader. Recorded so schema/rule evolution is detectable. (SCHEMA-05)
	RulesetVersion string `json:"ruleset_version,omitempty"`
}

// CatalogProvenance is the audit-record view of a single catalog hit. It mirrors
// policy.CatalogMatch field-for-field, including the Signed flag (CTLG-07), so
// the audit log records exactly which catalog source, entry, and signedness
// drove a decision.
//
// Phase 2 additions (CTLG-09): Corroborated, Dissented, and CatalogVersion carry
// per-match provenance so each source's role in the corroboration decision is
// recorded in the forensic trail.
type CatalogProvenance struct {
	CatalogSource  string `json:"catalog_source"`
	EntryID        string `json:"entry_id"`
	Ecosystem      string `json:"ecosystem"`
	Package        string `json:"package"`
	Version        string `json:"version"`
	Severity       string `json:"severity"`
	Signed         bool   `json:"signed"`
	// Phase 2 additions (CTLG-09):
	Corroborated   bool   `json:"corroborated"`
	Dissented      bool   `json:"dissented"`
	CatalogVersion string `json:"catalog_version"`
}

// FromDecision maps a policy Decision plus the originating tool call and
// caller-supplied metadata into an AuditRecord. It performs no I/O and reads no
// wall clock — recordID and timestamp are passed in verbatim — so it remains a
// pure, side-effect-free mapping that the hook handler and tests both rely on.
//
// Phase 2 (CTLG-09): the catalog_matches slice is always present (non-nil), even
// when empty, so non-catalog decisions serialize as `"catalog_matches":[]` rather
// than `"catalog_matches":null`. The new corroboration fields are mapped directly
// from the Decision.
//
// Phase 4 (INTG-07): ac carries agent lineage fields. Zero-value AgentContext{}
// produces no agent fields in the JSON output (all fields are omitempty).
func FromDecision(tc policy.ToolCall, d policy.Decision, recordID, timestamp string, ac policy.AgentContext) AuditRecord {
	// Always allocate the slice (never nil) — CTLG-09 requires the field present
	// even when no catalog matches occurred (non-catalog policy decisions).
	matches := make([]CatalogProvenance, 0, len(d.CatalogMatches))
	for _, m := range d.CatalogMatches {
		matches = append(matches, CatalogProvenance{
			CatalogSource:  m.CatalogSource,
			EntryID:        m.EntryID,
			Ecosystem:      m.Ecosystem,
			Package:        m.Package,
			Version:        m.Version,
			Severity:       m.Severity,
			Signed:         m.Signed,
			Corroborated:   m.Corroborated,
			Dissented:      m.Dissented,
			CatalogVersion: m.CatalogVersion,
		})
	}

	// Ensure SourcesAgreed and SourcesDissented are always non-nil slices so
	// the JSON output is `[]` not `null` (CTLG-09 requires consistent shape).
	sourcesAgreed := d.SourcesAgreed
	if sourcesAgreed == nil {
		sourcesAgreed = []string{}
	}
	sourcesDissented := d.SourcesDissented
	if sourcesDissented == nil {
		sourcesDissented = []string{}
	}

	return AuditRecord{
		RecordType:         "policy_decision",
		RecordID:           recordID,
		Timestamp:          timestamp,
		ScannerName:        "beekeeper",
		AgentName:          tc.AgentName,
		ToolName:           tc.ToolName,
		Decision:           d.Level,
		Reason:             d.Reason,
		RuleIDs:            d.RuleIDs,
		CatalogMatches:     matches,
		Endpoint:           "check",
		CorroborationCount: d.CorroborationCount,
		SourcesAgreed:      sourcesAgreed,
		SourcesDissented:   sourcesDissented,
		Quarantine:         d.Quarantine,
		// Phase 4 (INTG-07): agent lineage — omitempty fields are omitted when zero-value.
		AgentID:       ac.AgentID,
		ParentAgentID: ac.ParentAgentID,
		AgentDepth:    ac.Depth,
		AgentLineage:  ac.Lineage,
	}
}
