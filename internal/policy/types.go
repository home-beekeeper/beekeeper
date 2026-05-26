// Package policy is the single, pure policy-evaluation library for Beekeeper.
//
// Evaluate is a pure function: given a decoded tool call and a catalog lookup it
// returns a Decision with no I/O, no goroutines, no globals, and no wall-clock
// side effects. This is load-bearing: the hook handler (Phase 1), the MCP
// gateway (Phase 4), and Sentry correlation (Phase 5+) all call this one
// implementation, so keeping it pure prevents policy drift across consumers.
//
// Phase 1 scope: the only rule is Bumblebee single-source catalog matching,
// which produces a warn decision (warn does NOT block — Allow stays true).
// Corroboration-based block enforcement (PLCY-01) arrives in Phase 2.
package policy

import "github.com/mzansi-agentive/beekeeper/internal/catalog"

// ToolCall is a decoded, untrusted, attacker-influenceable agent tool call.
// ToolInput holds the raw, tool-specific arguments; package coordinates are
// extracted defensively by the engine.
type ToolCall struct {
	AgentName string         `json:"agent_name"`
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
}

// CatalogMatch records a single threat-intel catalog hit with provenance, for
// surfacing in the decision reason and the NDJSON audit record.
type CatalogMatch struct {
	CatalogSource string // e.g. "bumblebee"
	EntryID       string
	Ecosystem     string
	Package       string
	Version       string // extracted tool-call version ("" if unspecified)
	Severity      string
	Signed        bool // true iff the catalog entry carries a signature (CTLG-07)
}

// Decision is the result of evaluating a tool call.
//
//	Allow == true  => permit (hook handler exits 0)
//	Allow == false => block  (hook handler exits non-zero)
//
// In Phase 1 a catalog match yields Level "warn" with Allow still true, because
// catalog-driven blocking requires corroboration (Phase 2). Level is one of
// "allow" | "warn" | "block".
type Decision struct {
	Allow          bool
	Level          string
	Reason         string
	RuleIDs        []string
	CatalogMatches []CatalogMatch
}

// CatalogLookup is the minimal interface Evaluate depends on, so the policy
// engine does not import the concrete mmap-backed *catalog.Index and stays
// trivially unit-testable with a fake. The concrete *catalog.Index satisfies
// this interface.
type CatalogLookup interface {
	Lookup(ecosystem, pkg string) (catalog.Entry, bool)
}
