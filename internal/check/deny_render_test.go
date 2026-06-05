package check

import (
	"strings"
	"testing"

	"github.com/bantuson/beekeeper/internal/policy"
)

// TestRenderDeny is the CI-runnable regression gate asserting the exact per-harness
// deny output + exit code for every hook-capable harness. This test was the missing
// release gate that allowed the exit-1 silent-over-allow bug to ship (HPC-06).
//
// Each row covers one (harness, decision) pair. The loop asserts ExitCode, the
// presence of required Stdout substrings, and the presence of required Stderr
// substrings — asserting BOTH the exit code AND the exact JSON shape so a wrong
// shape fails the build.
func TestRenderDeny(t *testing.T) {
	cases := []struct {
		name               string
		harness            HarnessID
		decision           policy.Decision
		wantExit           int
		wantStdoutContains string // "" means assert Stdout is empty/nil
		wantStderrContains string // "" means no stderr assertion
	}{
		// --- Family A: nested hookSpecificOutput (Claude Code, Codex, CodeBuddy, Augment, Qwen) ---
		{
			name:               "claude-code block emits exit 2 + hookSpecificOutput deny",
			harness:            HarnessClaudeCode,
			decision:           policy.Decision{Allow: false, Level: "block", Reason: "credential read"},
			wantExit:           2,
			wantStdoutContains: `"permissionDecision":"deny"`,
			wantStderrContains: "credential read",
		},
		{
			name:               "codex block emits exit 2 + hookSpecificOutput deny",
			harness:            HarnessCodex,
			decision:           policy.Decision{Allow: false, Level: "block", Reason: "malicious package"},
			wantExit:           2,
			wantStdoutContains: `"permissionDecision":"deny"`,
			wantStderrContains: "malicious package",
		},
		{
			name:               "codebuddy block emits exit 2 + hookSpecificOutput deny",
			harness:            HarnessCodeBuddy,
			decision:           policy.Decision{Allow: false, Level: "block", Reason: "policy"},
			wantExit:           2,
			wantStdoutContains: `"permissionDecision":"deny"`,
			wantStderrContains: "policy",
		},
		{
			name:               "augment block emits exit 2 + hookSpecificOutput deny",
			harness:            HarnessAugment,
			decision:           policy.Decision{Allow: false, Level: "block", Reason: "path blocked"},
			wantExit:           2,
			wantStdoutContains: `"permissionDecision":"deny"`,
			wantStderrContains: "path blocked",
		},
		{
			name:               "qwen block emits exit 2 + hookSpecificOutput deny",
			harness:            HarnessQwen,
			decision:           policy.Decision{Allow: false, Level: "block", Reason: "supply chain"},
			wantExit:           2,
			wantStdoutContains: `"permissionDecision":"deny"`,
			wantStderrContains: "supply chain",
		},
		// --- Family B: flat permissionDecision (Copilot) ---
		{
			name:               "copilot block exit 2 + flat permissionDecision deny (no hookSpecificOutput wrapper)",
			harness:            HarnessCopilot,
			decision:           policy.Decision{Allow: false, Level: "block", Reason: "exfil risk"},
			wantExit:           2,
			wantStdoutContains: `"permissionDecision":"deny"`,
			wantStderrContains: "exfil risk",
		},
		// --- Family C: permission field (Cursor) ---
		{
			name:               "cursor block exit 2 + permission deny",
			harness:            HarnessCursor,
			decision:           policy.Decision{Allow: false, Level: "block", Reason: "x"},
			wantExit:           2,
			wantStdoutContains: `"permission":"deny"`,
			wantStderrContains: "x",
		},
		// --- Family D: decision field (Gemini CLI) ---
		{
			name:               "gemini block exit 2 + decision deny",
			harness:            HarnessGemini,
			decision:           policy.Decision{Allow: false, Level: "block", Reason: "threat intel"},
			wantExit:           2,
			wantStdoutContains: `"decision":"deny"`,
			wantStderrContains: "threat intel",
		},
		// --- Family E: dual-defensive (Antigravity) ---
		{
			name:               "antigravity block exit 2 + both decision and permissionDecision deny",
			harness:            HarnessAntigravity,
			decision:           policy.Decision{Allow: false, Level: "block", Reason: "catalog match"},
			wantExit:           2,
			wantStdoutContains: `"decision":"deny"`,
			wantStderrContains: "catalog match",
		},
		// --- Family F: cancel (Cline) ---
		{
			name:               "cline block exit 2 + cancel true",
			harness:            HarnessCline,
			decision:           policy.Decision{Allow: false, Level: "block", Reason: "blocked tool"},
			wantExit:           2,
			wantStdoutContains: `"cancel":true`,
			wantStderrContains: "blocked tool",
		},
		// --- Family G: Hermes fail-open (JSON-only block path) ---
		{
			name:               "hermes block exit is 0 (fail-open harness — JSON is the block)",
			harness:            HarnessHermes,
			decision:           policy.Decision{Allow: false, Level: "block", Reason: "policy"},
			wantExit:           0, // Hermes ignores exit code; JSON carries the block
			wantStdoutContains: `"action":"block"`,
			wantStderrContains: "policy",
		},
		// --- Family H: Windsurf — exit-2-only, no stdout JSON ---
		{
			name:               "windsurf block exit 2 no stdout",
			harness:            HarnessWindsurf,
			decision:           policy.Decision{Allow: false, Level: "block", Reason: "policy"},
			wantExit:           2,
			wantStdoutContains: "", // Windsurf: exit-2 only, no JSON
			wantStderrContains: "policy",
		},
		// --- Family H: OpenCode — exit-2-only, no stdout JSON ---
		{
			name:               "opencode block exit 2 no stdout",
			harness:            HarnessOpenCode,
			decision:           policy.Decision{Allow: false, Level: "block", Reason: "policy"},
			wantExit:           2,
			wantStdoutContains: "", // OpenCode: exit-2 only (plugin throws)
			wantStderrContains: "policy",
		},
		// --- Additional harnesses: Kilo, Trae (MCP-gateway-only — still emit exit 2 + stderr on block) ---
		{
			name:               "kilo block exit 2 no stdout",
			harness:            HarnessKilo,
			decision:           policy.Decision{Allow: false, Level: "block", Reason: "policy"},
			wantExit:           2,
			wantStdoutContains: "",
			wantStderrContains: "policy",
		},
		{
			name:               "trae block exit 2 no stdout",
			harness:            HarnessTrae,
			decision:           policy.Decision{Allow: false, Level: "block", Reason: "policy"},
			wantExit:           2,
			wantStdoutContains: "",
			wantStderrContains: "policy",
		},
		// --- Unknown harness: fail-closed ---
		{
			name:               "unknown harness block fail-closed exit 2 no stdout",
			harness:            HarnessID("unknown-harness"),
			decision:           policy.Decision{Allow: false, Level: "block", Reason: "policy"},
			wantExit:           2,
			wantStdoutContains: "",
			wantStderrContains: "policy",
		},
		// --- Allow path: no harness-specific output on allow/warn ---
		{
			name:               "allow path: no harness output emitted",
			harness:            HarnessClaudeCode,
			decision:           policy.Decision{Allow: true, Level: "allow"},
			wantExit:           0,
			wantStdoutContains: "", // proves non-block never over-allows
			wantStderrContains: "",
		},
		// --- Additional assertions ---
		{
			name:               "antigravity block also emits permissionDecision deny (dual defensive)",
			harness:            HarnessAntigravity,
			decision:           policy.Decision{Allow: false, Level: "block", Reason: "dual check"},
			wantExit:           2,
			wantStdoutContains: `"permissionDecision":"deny"`,
			wantStderrContains: "dual check",
		},
		{
			name:               "hermes block message non-empty when reason provided",
			harness:            HarnessHermes,
			decision:           policy.Decision{Allow: false, Level: "block", Reason: "explicit reason"},
			wantExit:           0,
			wantStdoutContains: `"action":"block"`,
			wantStderrContains: "explicit reason",
		},
		{
			name:               "hermes block message is non-empty when reason is empty",
			harness:            HarnessHermes,
			decision:           policy.Decision{Allow: false, Level: "block", Reason: ""},
			wantExit:           0,
			wantStdoutContains: `"action":"block"`,
			wantStderrContains: "", // no specific stderr expected but message must be non-empty
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := RenderDeny(tc.harness, tc.decision)

			// Assert exit code.
			if out.ExitCode != tc.wantExit {
				t.Errorf("ExitCode = %d, want %d", out.ExitCode, tc.wantExit)
			}

			// Assert Stdout: if wantStdoutContains is non-empty, the substring must
			// be present. If it is empty, Stdout must be nil or empty (proves no
			// accidental allow-form emission).
			if tc.wantStdoutContains != "" {
				if !strings.Contains(string(out.Stdout), tc.wantStdoutContains) {
					t.Errorf("Stdout %q does not contain %q", string(out.Stdout), tc.wantStdoutContains)
				}
			} else {
				if len(out.Stdout) != 0 {
					t.Errorf("expected empty Stdout, got %q", string(out.Stdout))
				}
			}

			// Assert Stderr: if wantStderrContains is non-empty, the substring must
			// be present.
			if tc.wantStderrContains != "" {
				if !strings.Contains(string(out.Stderr), tc.wantStderrContains) {
					t.Errorf("Stderr %q does not contain %q", string(out.Stderr), tc.wantStderrContains)
				}
			}
		})
	}

	// Extra assertion: Hermes with empty reason must have a non-empty message in
	// the JSON (substitutes "blocked by beekeeper policy").
	t.Run("hermes block guarantees non-empty message even with empty reason", func(t *testing.T) {
		out := RenderDeny(HarnessHermes, policy.Decision{Allow: false, Level: "block", Reason: ""})
		if out.ExitCode != 0 {
			t.Errorf("Hermes ExitCode = %d, want 0", out.ExitCode)
		}
		stdout := string(out.Stdout)
		if !strings.Contains(stdout, `"action":"block"`) {
			t.Errorf("Hermes Stdout missing action:block, got %q", stdout)
		}
		// Verify the message is non-empty in the JSON.
		if strings.Contains(stdout, `"message":""`) {
			t.Errorf("Hermes message must be non-empty, got %q", stdout)
		}
	})
}
