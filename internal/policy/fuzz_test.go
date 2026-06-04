//go:build fuzz

package policy

import (
	"encoding/json"
	"testing"
)

// FuzzEvaluate runs the policy engine against fuzz-generated tool-call JSON.
// Contract: Evaluate must never panic and must return a Decision whose Level
// is one of the three defined values (allow|warn|block). A panic on the hot
// path is a fail-open vector (T-02-09-01); an invalid Level breaks audit
// records and downstream SIEM correlations.
//
// The empty fakeMultiCatalog{} returns no matches for all lookups, so the
// fuzzer exercises the extraction and corroboration logic without needing a
// real catalog. The fakeMultiCatalog type is defined in engine_test.go (same
// package); it is compiled alongside this file under -tags fuzz.
func FuzzEvaluate(f *testing.F) {
	// Seed corpus: representative shapes that exercise the extraction paths —
	// command shape, direct shape, scoped npm package, empty input, deeply nested.
	f.Add(`{"tool_name":"Bash","tool_input":{"command":"npm install express@4.18.2"}}`)
	f.Add(`{"tool_name":"Install","tool_input":{"ecosystem":"npm","package":"lodash","version":"4.17.21"}}`)
	f.Add(`{"tool_name":"Bash","tool_input":{"command":"npm install @scope/pkg@1.0.0"}}`)
	f.Add(`{}`)
	f.Add(`{"tool_name":"Bash","tool_input":{"command":"pip install requests","extra":{"a":{"b":{"c":{"d":"e"}}}}}}`)

	f.Fuzz(func(t *testing.T, data string) {
		var tc ToolCall
		// Unmarshal errors are expected and intentional — the fuzz target probes
		// the post-decode path; partial or invalid JSON produces a zero-value
		// ToolCall which the engine must handle without panicking.
		_ = json.Unmarshal([]byte(data), &tc)

		d := Evaluate(tc, fakeMultiCatalog{}, DefaultCorroborationThresholds(), AgentContext{})

		switch d.Level {
		case "allow", "warn", "block":
		default:
			t.Errorf("Evaluate returned invalid Level %q for input %q; must be allow|warn|block", d.Level, data)
		}
	})
}
