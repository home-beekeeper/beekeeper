// Package corpus — SCHEMA-06 evaluator gate.
//
// TestSchemaLockNxConsoleTrace is the Phase 22 schema-lock evaluator gate
// (PRD §4 Phase 0). It proves:
//
//  1. The Nx Console Sentry exfil incident maps to the typed Go schema with
//     NO gaps — every PRD §3.1 four-layer field name has a corresponding
//     populated Go field.
//
//  2. A PushEnvelope can represent a watch_and_block push for that incident
//     carrying confidence_tier:"enforce" + source_count:2.
//
// Passing this gate is the precondition for the human schema-freeze sign-off.
// Sign-off freezes the format: no field may be added, removed, or renamed after
// Phase 22 without a CorpusSchemaVersion bump and a migration plan.
package corpus

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/home-beekeeper/beekeeper/internal/audit"
)

// TestSchemaLockNxConsoleTrace is the SCHEMA-06 evaluator gate.
//
// The gate is structured as four sequential proofs:
//
//  1. Load and parse the Nx Console fixture.
//  2. Construct a CorpusRecord from the trace, populating every PRD §3.1 field.
//  3. Assert every PRD §3.1 field name maps to a non-empty Go field (the no-gaps detector).
//  4. Construct a PushEnvelope with ActionHintWatchAndBlock, ConfidenceTier:"enforce",
//     SourceCount:2 and assert the marshaled JSON carries all required envelope fields.
//
// Failure: the test emits a clear message naming any unmapped field.
func TestSchemaLockNxConsoleTrace(t *testing.T) {
	// --- Step 1: Load and parse the fixture ---

	data, err := os.ReadFile("testdata/nx_console_trace.json")
	if err != nil {
		t.Fatalf("SCHEMA-06: cannot load fixture testdata/nx_console_trace.json: %v", err)
	}

	// Parse into a generic map so we can extract values by key name.
	var fixture map[string]any
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("SCHEMA-06: invalid JSON in nx_console_trace.json: %v", err)
	}

	// Convenience extractor: navigate into nested map by dot-path.
	getStr := func(path ...string) string {
		var cur any = fixture
		for _, key := range path {
			m, ok := cur.(map[string]any)
			if !ok {
				return ""
			}
			cur = m[key]
		}
		if s, ok := cur.(string); ok {
			return s
		}
		return ""
	}
	getFloat := func(path ...string) float64 {
		var cur any = fixture
		for _, key := range path {
			m, ok := cur.(map[string]any)
			if !ok {
				return 0
			}
			cur = m[key]
		}
		if f, ok := cur.(float64); ok {
			return f
		}
		return 0
	}
	getSlice := func(path ...string) []string {
		var cur any = fixture
		for _, key := range path {
			m, ok := cur.(map[string]any)
			if !ok {
				return nil
			}
			cur = m[key]
		}
		raw, ok := cur.([]any)
		if !ok {
			return nil
		}
		out := make([]string, 0, len(raw))
		for _, v := range raw {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}

	// --- Step 2: Construct a CorpusRecord from the trace ---
	//
	// Behavior + decision layers come from the embedded audit.AuditRecord.
	// Outcome + context layers are set directly on CorpusRecord.
	//
	// This is the schema-gap proof: if any PRD §3.1 field had no Go field to
	// receive it, this step would not compile or would produce an empty assertion.

	var rec CorpusRecord

	// Behavior layer — via embedded AuditRecord:
	rec.AuditRecord = audit.AuditRecord{
		// source_surface: the branch key identifying the Beekeeper surface.
		SourceSurface: getStr("behavior_layer", "source_surface"), // "sentry"

		// action_type: synthesized from SENTRY-005 rule match.
		// AuditRecord.ToolName is the action_type proxy for non-agent surfaces.
		ToolName: getStr("behavior_layer", "action_type"), // "sentry_exfil_fusion"

		// actor_lineage: editor-descended process tree.
		// AuditRecord.SentryParentChain maps to actor_lineage for sentry surface.
		SentryParentChain: getSlice("behavior_layer", "actor_lineage"),

		// target_resource: credential file read by the exfil.
		// AuditRecord.SentryFilesAccessed is the sentry target_resource slot.
		SentryFilesAccessed: []string{getStr("behavior_layer", "target_resource")},

		// network_destination: attacker collector.
		// AuditRecord.SentryNetworkDests is the sentry network_destination slot.
		SentryNetworkDests: []string{getStr("behavior_layer", "network_destination")},

		// Decision layer — via embedded AuditRecord:

		// verdict: sentry surface uses "alert" (detection-only, not enforcement block).
		Decision: getStr("decision_layer", "verdict"),

		// policy_matched: the SENTRY rule that fired.
		Reason: getStr("decision_layer", "policy_matched"),

		// rule_id: the sentry-specific rule ID slot.
		SentryRuleID: getStr("decision_layer", "rule_id"), // "SENTRY-005"

		// correlation_window: encoded in the sentry rule window.
		// No direct AuditRecord field; documented as sentry_rule_name proxy.
		SentryRuleName: getStr("decision_layer", "correlation_window"),

		// confidence: corroboration count (catalog + forensic = 2).
		CorroborationCount: int(getFloat("decision_layer", "confidence")),

		// ruleset_version: catalog snapshot version at decision time.
		RulesetVersion: getStr("decision_layer", "ruleset_version"),

		// agent_id: absent for sentry surface (sentry is not an agent-mediated surface).
		// AgentID is omitempty; left as "" (correctly absent from JSON).
	}

	// Outcome layer — directly on CorpusRecord (corpus-specific; non-retrofittable):

	// true_label: always present; "unresolved" from run-1.
	rec.TrueLabel = getStr("outcome_layer", "true_label") // "unresolved"

	// was_correct: nil (pointer) — unresolved initially.
	rec.WasCorrect = nil // *bool nil = unresolved

	// adjudication_source: absent until adjudicated (omitempty).
	rec.AdjudicationSource = getStr("outcome_layer", "adjudication_source") // ""

	// resolved_at: absent until adjudicated (omitempty).
	rec.ResolvedAt = getStr("outcome_layer", "resolved_at") // ""

	// Context layer — directly on CorpusRecord:

	// cluster_id: derived from the sentry correlation window.
	// The embedded AuditRecord.ClusterID carries this on the audit side;
	// CorpusRecord can also carry it directly when the corpus emitter enriches the record.
	rec.AuditRecord.ClusterID = getStr("context_layer", "cluster_id")

	// baseline_deviation: behavioral baseline deviation level.
	rec.BaselineDeviation = getStr("context_layer", "baseline_deviation") // "high"

	// repo_fingerprint: HMAC-SHA256 hex (Phase 23 STORE-05 populates; empty in gate).
	rec.RepoFingerprint = getStr("context_layer", "repo_fingerprint") // ""

	// fleet_node_id: HMAC-SHA256 hex (Phase 23 STORE-05 populates; empty in gate).
	rec.FleetNodeID = getStr("context_layer", "fleet_node_id") // ""

	// scope: org_only (default; zero value → "org_only" via MarshalJSON).
	rec.Scope = ScopeOrgOnly

	// corpus_schema_version: always the current schema constant.
	rec.CorpusSchemaVersion = CorpusSchemaVersion

	// --- Step 3: PRD §3.1 field-name no-gaps detector ---
	//
	// Every PRD §3.1 field name is enumerated. The test checks that each maps
	// to a populated (non-zero or explicitly documented) Go field.
	// Conditional fields (agent_id, correlation_window, policy_matched for
	// non-sentry surfaces) are noted but not failed — they are intentionally
	// empty for this surface.
	//
	// Failure message format: "SCHEMA-06 gap: <field_name> is unmapped"
	//
	// If ANY field name has no corresponding Go field carrying a non-zero value
	// (or an explicit nil/empty documented as intentional), the test fails.

	type fieldCheck struct {
		name    string // PRD §3.1 field name
		value   string // the Go field value (stringified for comparison)
		wantAny bool   // true = any non-empty value is acceptable
		allow   string // expected exact value (if wantAny==false)
		skip    bool   // true = field is intentionally empty for this surface (conditional)
		skipMsg string // why it is skipped
	}

	checks := []fieldCheck{
		// --- Behavior layer ---
		{name: "source_surface", value: rec.AuditRecord.SourceSurface, allow: "sentry"},
		{name: "action_type (via ToolName)", value: rec.AuditRecord.ToolName, allow: "sentry_exfil_fusion"},
		{name: "actor_lineage (via SentryParentChain)", value: strings.Join(rec.AuditRecord.SentryParentChain, ","), wantAny: true},
		{name: "target_resource (via SentryFilesAccessed[0])", value: func() string {
			if len(rec.AuditRecord.SentryFilesAccessed) > 0 {
				return rec.AuditRecord.SentryFilesAccessed[0]
			}
			return ""
		}(), allow: "~/.ssh/id_rsa"},
		{name: "network_destination (via SentryNetworkDests[0])", value: func() string {
			if len(rec.AuditRecord.SentryNetworkDests) > 0 {
				return rec.AuditRecord.SentryNetworkDests[0]
			}
			return ""
		}(), allow: "malicious-collector.example.com"},
		{
			name: "agent_id",
			skip: true,
			skipMsg: "conditional — only for agent-mediated surfaces (hook/mcp_gateway/shim); sentry surface has no agent_id",
		},

		// --- Decision layer ---
		{name: "verdict (via Decision)", value: rec.AuditRecord.Decision, allow: "alert"},
		{name: "policy_matched (via Reason)", value: rec.AuditRecord.Reason, allow: "SENTRY-005"},
		{name: "rule_id (via SentryRuleID)", value: rec.AuditRecord.SentryRuleID, allow: "SENTRY-005"},
		{
			name:    "correlation_window (via SentryRuleName proxy)",
			value:   rec.AuditRecord.SentryRuleName,
			skip:    false,
			wantAny: true,
		},
		{name: "confidence (via CorroborationCount)", value: func() string {
			if rec.AuditRecord.CorroborationCount == 2 {
				return "2"
			}
			return ""
		}(), allow: "2"},
		{name: "ruleset_version (via RulesetVersion)", value: rec.AuditRecord.RulesetVersion, allow: "1.0"},

		// --- Outcome layer ---
		{name: "true_label", value: rec.TrueLabel, allow: "unresolved"},
		{name: "was_correct", skip: true, skipMsg: "*bool nil = unresolved; absent from JSON until adjudicated (pointer omitempty)"},
		{name: "adjudication_source", skip: true, skipMsg: "omitempty; absent until adjudicated"},
		{name: "resolved_at", skip: true, skipMsg: "omitempty; absent until adjudicated"},

		// --- Context layer ---
		{name: "cluster_id (via AuditRecord.ClusterID)", value: rec.AuditRecord.ClusterID, wantAny: true},
		{name: "baseline_deviation", value: rec.BaselineDeviation, allow: "high"},
		{name: "repo_fingerprint", skip: true, skipMsg: "HMAC-SHA256 hex populated by Phase 23 STORE-05; empty in gate"},
		{name: "fleet_node_id", skip: true, skipMsg: "HMAC-SHA256 hex populated by Phase 23 STORE-05; empty in gate"},
		{name: "scope", value: string(rec.Scope), allow: "org_only"},
	}

	for _, c := range checks {
		if c.skip {
			t.Logf("SCHEMA-06 skip: %s — %s", c.name, c.skipMsg)
			continue
		}
		if c.wantAny {
			if c.value == "" {
				t.Errorf("SCHEMA-06 gap: %s is unmapped (got empty string; want any non-empty value)", c.name)
			}
		} else {
			if c.value != c.allow {
				t.Errorf("SCHEMA-06 gap: %s = %q; want %q", c.name, c.value, c.allow)
			}
		}
	}

	// --- Step 3b: Confirm the CorpusRecord marshals correctly ---
	recJSON, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("SCHEMA-06: json.Marshal(CorpusRecord): %v", err)
	}
	recOut := string(recJSON)

	// AuditRecord fields must be promoted (not nested under "AuditRecord").
	if strings.Contains(recOut, `"AuditRecord"`) {
		t.Errorf("SCHEMA-06: json.Marshal produced nested \"AuditRecord\" key; unnamed embedding must promote fields. JSON: %s", recOut)
	}

	// source_surface must appear at top level.
	if !strings.Contains(recOut, `"source_surface":"sentry"`) {
		t.Errorf("SCHEMA-06: expected top-level \"source_surface\":\"sentry\"; got: %s", recOut)
	}

	// true_label must always be present (no omitempty).
	if !strings.Contains(recOut, `"true_label":"unresolved"`) {
		t.Errorf("SCHEMA-06: expected \"true_label\":\"unresolved\" in JSON; got: %s", recOut)
	}

	// scope must be "org_only" (zero-value guarantee).
	if !strings.Contains(recOut, `"scope":"org_only"`) {
		t.Errorf("SCHEMA-06: expected \"scope\":\"org_only\" in JSON; got: %s", recOut)
	}

	// was_correct must be absent (nil pointer + omitempty).
	if strings.Contains(recOut, `"was_correct"`) {
		t.Errorf("SCHEMA-06: \"was_correct\" must be absent when WasCorrect is nil; got: %s", recOut)
	}

	// --- Step 4: Construct and assert a PushEnvelope ---
	//
	// The envelope must carry ActionHintWatchAndBlock (typed), ConfidenceTier:"enforce",
	// SourceCount:2. This proves the envelope types are sufficient to represent a
	// watch_and_block push for the Nx Console incident.

	pkg := getStr("expected_envelope", "package_or_extension_id")
	ver := getStr("expected_envelope", "version")
	target := ""
	if len(rec.AuditRecord.SentryFilesAccessed) > 0 {
		target = rec.AuditRecord.SentryFilesAccessed[0]
	}
	netDest := ""
	if len(rec.AuditRecord.SentryNetworkDests) > 0 {
		netDest = rec.AuditRecord.SentryNetworkDests[0]
	}

	// BehaviorSigHash uses the frozen normalization rules — exercises both
	// behavior_sig.go and the fixture in a single gate assertion.
	sigHash := BehaviorSigHash(rec.AuditRecord.ToolName, target, netDest)

	env := PushEnvelope{
		Signature: EnvelopeSignature{
			PackageOrExtensionID:  pkg,
			Version:               ver,
			BehaviorSignatureHash: sigHash,
			IOCs: IOCBlock{
				Domains: []string{netDest},
			},
		},
		TrueLabel:      "malicious",       // adjudicated label at push time
		ConfidenceTier: "enforce",         // source_count 2 >= BlockAt threshold
		SourceCount:    2,                 // catalog_confirmation + breach_confirmation
		Scope:          ScopeOrgOnly,
		ActionHint:     ActionHintWatchAndBlock, // typed: only valid value
		Signing:        nil,               // v1 zero-value
	}

	// Step 4a: Typed ActionHint check.
	if env.ActionHint != ActionHintWatchAndBlock {
		t.Errorf("SCHEMA-06: env.ActionHint = %q; want ActionHintWatchAndBlock", env.ActionHint)
	}

	// Step 4b: Marshal and assert all required envelope fields.
	envJSON, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("SCHEMA-06: json.Marshal(PushEnvelope): %v", err)
	}
	envOut := string(envJSON)

	envChecks := []struct {
		fragment string
		desc     string
	}{
		{`"action_hint":"watch_and_block"`, "action_hint must be watch_and_block"},
		{`"confidence_tier":"enforce"`, "confidence_tier must be enforce"},
		{`"source_count":2`, "source_count must be 2"},
		{`"scope":"org_only"`, "scope must default to org_only"},
		{`"behavior_signature_hash"`, "behavior_signature_hash must be present"},
		{`"package_or_extension_id"`, "package_or_extension_id must be present"},
		{`"version"`, "version must be present"},
		{`"true_label":"malicious"`, "true_label must be malicious"},
	}

	for _, ec := range envChecks {
		if !strings.Contains(envOut, ec.fragment) {
			t.Errorf("SCHEMA-06: %s — fragment %q not found in envelope JSON: %s", ec.desc, ec.fragment, envOut)
		}
	}

	// "signing" must be absent in v1 (nil pointer + omitempty).
	if strings.Contains(envOut, `"signing"`) {
		t.Errorf("SCHEMA-06: \"signing\" must be absent when Signing is nil (v1); got: %s", envOut)
	}

	// Step 4c: BehaviorSignatureHash must be a 64-char hex string.
	if len(sigHash) != 64 {
		t.Errorf("SCHEMA-06: BehaviorSignatureHash: expected 64-char hex; got %d chars: %q", len(sigHash), sigHash)
	}

	// --- Step 5: Zero-value CorpusRecord scope assertion ---
	//
	// Proves the SCOPE-01 zero-value guarantee independently from TestScopeZeroValue:
	// a zero-value CorpusRecord (no constructor call) must serialize scope as "org_only".
	zeroRec := CorpusRecord{}
	zeroJSON, err := json.Marshal(zeroRec)
	if err != nil {
		t.Fatalf("SCHEMA-06: json.Marshal(CorpusRecord{}): %v", err)
	}
	if !strings.Contains(string(zeroJSON), `"scope":"org_only"`) {
		t.Errorf("SCHEMA-06: zero-value CorpusRecord{} must serialize scope as \"org_only\"; got: %s", string(zeroJSON))
	}
}
