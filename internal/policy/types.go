// Package policy is the single, pure policy-evaluation library for Beekeeper.
//
// Evaluate is a pure function: given a decoded tool call and a catalog lookup it
// returns a Decision with no I/O, no goroutines, no globals, and no wall-clock
// side effects. This is load-bearing: the hook handler (Phase 1), the MCP
// gateway (Phase 4), and Sentry correlation (Phase 5+) all call this one
// implementation, so keeping it pure prevents policy drift across consumers.
//
// Phase 2 scope: corroboration-based block enforcement (PLCY-01) — a package
// matched by two or more independent signed sources yields a block decision.
// Three or more signed sources additionally sets Quarantine true. The
// MultiCatalogLookup interface is the contract that Wave 2 catalog adapters
// and Wave 3 integration consume.
package policy

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
//
// Phase 1 fields (unchanged): CatalogSource, EntryID, Ecosystem, Package,
// Version, Severity, Signed.
//
// Phase 2 additions (CTLG-09):
//   - Corroborated: true when this source contributed to a block or quarantine decision.
//   - Dissented: true when this source disagreed with the majority (reserved; always
//     false in Phase 2 — a source that returns no match simply does not appear).
//   - CatalogVersion: hash or timestamp of the catalog snapshot at evaluation time,
//     supplied by the adapter; "" when unknown.
type CatalogMatch struct {
	CatalogSource  string // e.g. "bumblebee", "osv", "socket"
	EntryID        string
	Ecosystem      string
	Package        string
	Version        string // extracted tool-call version ("" if unspecified)
	Severity       string
	Signed         bool   // true iff the catalog entry carries a signature (CTLG-07)
	Corroborated   bool   // Phase 2: true when this source contributed to a block/quarantine
	Dissented      bool   // Phase 2: true when this source disagreed with the majority (reserved)
	CatalogVersion string // Phase 2: hash or timestamp of the catalog at evaluation time
}

// Decision is the result of evaluating a tool call.
//
//	Allow == true  => permit (hook handler exits 0)
//	Allow == false => block  (hook handler exits non-zero)
//
// Level is one of "allow" | "warn" | "block".
//
// Phase 2 additions (CTLG-09):
//   - CorroborationCount: number of independent SIGNED sources that matched.
//   - SourcesAgreed: distinct catalog_source values that agreed (matched), e.g. ["bumblebee","osv"].
//   - SourcesDissented: distinct catalog_source values that dissented (reserved; always empty in Phase 2).
//   - Quarantine: true when the quarantine threshold (3+ signed sources) is met.
type Decision struct {
	Allow              bool
	Level              string
	Reason             string
	RuleIDs            []string
	CatalogMatches     []CatalogMatch
	CorroborationCount int      // Phase 2: number of independent signed sources that matched
	SourcesAgreed      []string // Phase 2: e.g. ["bumblebee", "osv"]
	SourcesDissented   []string // Phase 2: reserved; always empty in Phase 2
	Quarantine         bool     // Phase 2: true when 3+ signed sources agree
}

// MultiCatalogLookup is the Phase 2 multi-source catalog interface. It returns
// matches from ALL configured catalog sources with full provenance. The method
// is pure: implementations must not perform I/O during the call — all I/O
// (network, disk) must be resolved by the concrete adapter before Evaluate is
// called. The concrete multi-source adapter (Plan 04/05 wiring, Plan 08) and
// the test fakeMultiCatalog satisfy this interface.
type MultiCatalogLookup interface {
	LookupAll(ecosystem, pkg string) []CatalogMatch
}

// CorroborationThresholds controls when corroboration escalates the decision
// level. Thresholds are per-ecosystem configurable; the defaults (WarnAt 1,
// BlockAt 2, QuarantineAt 3) match the PLCY-01 specification.
//
// Only SIGNED sources count toward thresholds. Unsigned sources contribute
// warn-only weight (0.5) and can never alone reach BlockAt.
type CorroborationThresholds struct {
	WarnAt      int // minimum signed-source count for warn level (default 1)
	BlockAt     int // minimum signed-source count for block level (default 2)
	QuarantineAt int // minimum signed-source count for block+quarantine (default 3)
}

// DefaultCorroborationThresholds returns the PLCY-01 default thresholds:
// warn at 1 signed source, block at 2, quarantine at 3. No I/O.
func DefaultCorroborationThresholds() CorroborationThresholds {
	return CorroborationThresholds{
		WarnAt:      1,
		BlockAt:     2,
		QuarantineAt: 3,
	}
}
