package main

import "testing"

// TestBuildShimToolCall proves the shim wiring fix: a shim invocation
// (beekeeper check --tool npm --args install left-pad) is reconstructed into a
// BASH tool call carrying the FULL command, so the catalog engine and install
// posture actually evaluate it. Before this fix the shape was
// {tool_name:"execute", command:"npm", args:[...]} and nothing parsed the install,
// so beekeeper check allowed every shim-intercepted install (a no-op).
func TestBuildShimToolCall(t *testing.T) {
	tc := buildShimToolCall("npm", []string{"install", "left-pad"})

	if tc["tool_name"] != "Bash" {
		t.Errorf("tool_name = %v, want Bash (so evaluatePosture and extract fire)", tc["tool_name"])
	}
	if tc["agent_name"] != "shim" {
		t.Errorf("agent_name = %v, want shim", tc["agent_name"])
	}
	ti, ok := tc["tool_input"].(map[string]any)
	if !ok {
		t.Fatalf("tool_input is not a map: %T", tc["tool_input"])
	}
	if ti["command"] != "npm install left-pad" {
		t.Errorf("command = %q, want %q (full reconstructed install command)", ti["command"], "npm install left-pad")
	}
	// The bare-tool (no args) case must not append a trailing space.
	bare := buildShimToolCall("npm", nil)["tool_input"].(map[string]any)
	if bare["command"] != "npm" {
		t.Errorf("no-arg command = %q, want %q", bare["command"], "npm")
	}
}
