// Package check — deny_render.go
//
// RenderDeny is the pure, table-driven deny renderer for the --hook adapter.
// It maps a (HarnessID, policy.Decision) pair to a DenyOutput (Stdout bytes,
// Stderr bytes, ExitCode) without performing any I/O, calling os.Exit, or
// touching package-level state. Callers write Stdout/Stderr to the appropriate
// streams and call os.Exit(out.ExitCode).
//
// The default `beekeeper check` path (exitAllow/exitBlock/exitCodeFor/RunCheck)
// is UNCHANGED. This file adds a parallel path activated only by --hook.
package check

import (
	"encoding/json"
	"fmt"

	"github.com/bantuson/beekeeper/internal/policy"
)

// exitHookBlock is the exit code emitted when --hook <harness> is active and
// the decision is block. EVERY hook-capable harness except Hermes honors exit 2
// as deny. Hermes is fail-open on exit codes; its block path is JSON-only.
const exitHookBlock = 2

// HarnessID is the canonical name passed to --hook (matches installer target names).
// The value EXACTLY matches the string used as the --hook flag value and the
// per-harness installer target name.
type HarnessID string

// Supported harness IDs — one const per harness.
const (
	HarnessClaudeCode  HarnessID = "claude-code"
	HarnessCursor      HarnessID = "cursor"
	HarnessCodex       HarnessID = "codex"
	HarnessAugment     HarnessID = "augment"
	HarnessCodeBuddy   HarnessID = "codebuddy"
	HarnessQwen        HarnessID = "qwen"
	HarnessCopilot     HarnessID = "copilot"
	HarnessGemini      HarnessID = "gemini"
	HarnessAntigravity HarnessID = "antigravity"
	HarnessWindsurf    HarnessID = "windsurf"
	HarnessCline       HarnessID = "cline"
	HarnessHermes      HarnessID = "hermes"
	HarnessOpenCode    HarnessID = "opencode"
	HarnessKilo        HarnessID = "kilo"
	HarnessTrae        HarnessID = "trae"
)

// DenyOutput is the complete deny rendering for a blocked tool call under a
// specific harness contract. Callers write Stdout to os.Stdout, Stderr to
// os.Stderr, then call os.Exit(ExitCode).
type DenyOutput struct {
	Stdout   []byte // harness-specific JSON (nil = nothing to stdout)
	Stderr   []byte // human-readable reason (always non-nil on block)
	ExitCode int    // exitHookBlock (2) for all harnesses; 0 for Hermes (fail-open)
}

// ---- per-family deny structs ------------------------------------------------
//
// Each struct is marshalled to JSON by RenderDeny. Using typed structs (rather
// than map[string]any or hand-built strings) ensures field names are stable and
// testable.

// hookSpecificOutputDeny is Family A: nested hookSpecificOutput schema used by
// Claude Code, Codex, CodeBuddy, Augment, and Qwen.
type hookSpecificOutputDeny struct {
	HookSpecificOutput hookSpecificOutputBody `json:"hookSpecificOutput"`
}

type hookSpecificOutputBody struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason"`
}

// copilotDeny is Family B: flat permissionDecision used by Copilot (NOT nested).
type copilotDeny struct {
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason"`
}

// cursorDeny is Family C: permission + user/agent message used by Cursor.
type cursorDeny struct {
	Permission   string `json:"permission"`
	UserMessage  string `json:"user_message"`
	AgentMessage string `json:"agent_message"`
}

// geminiDeny is Family D: Gemini-native decision field used by Gemini CLI.
type geminiDeny struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

// antigravityDeny is Family E: dual-defensive deny for Antigravity (MED-confidence
// docs — emit both decision and permissionDecision so either field triggers the block).
type antigravityDeny struct {
	Decision           string `json:"decision"`
	PermissionDecision string `json:"permissionDecision"`
	DenyReason         string `json:"denyReason"`
}

// clineDeny is Family F: cancel-based deny used by Cline.
type clineDeny struct {
	Cancel       bool   `json:"cancel"`
	ErrorMessage string `json:"errorMessage"`
}

// hermesDeny is Family G: action-based deny for Hermes. Exit codes are IGNORED
// by Hermes — this JSON is the ONLY block path. The Message field MUST be
// non-empty; if d.Reason is empty we substitute the policy sentinel string.
type hermesDeny struct {
	Action  string `json:"action"`
	Message string `json:"message"`
}

// ---- RenderDeny -------------------------------------------------------------

// RenderDeny is a pure, table-driven function: no I/O, no side effects, no
// os.Exit. It maps (HarnessID, policy.Decision) → DenyOutput so callers can
// write the output and exit independently.
//
// On allow (d.Allow == true) RenderDeny returns DenyOutput{ExitCode: 0} with
// nil Stdout and nil Stderr. The harness's own permission flow handles
// allow/warn — beekeeper NEVER emits permissionDecision:"allow" (CONTEXT
// decision 3).
//
// On block (d.Allow == false):
//   - Stderr is ALWAYS the non-empty d.Reason (universal stderr baseline).
//   - ExitCode is exitHookBlock (2) for all harnesses EXCEPT Hermes (ExitCode=0).
//   - Stdout is harness-specific JSON marshalled from the per-family struct, or
//     nil for exit-2-only harnesses (Windsurf, OpenCode, Kilo, Trae).
//   - For an unknown/empty HarnessID on block, fail CLOSED: exit 2 + stderr
//     reason, nil Stdout (never silently allow).
func RenderDeny(h HarnessID, d policy.Decision) DenyOutput {
	// Allow/warn: return zero output so the harness's own flow handles it.
	// NEVER emit permissionDecision:"allow" — that bypasses harness approval flow.
	if d.Allow {
		return DenyOutput{ExitCode: exitAllow}
	}

	// Block path: stderr baseline is always the reason.
	stderr := []byte(d.Reason)

	switch h {
	// --- Family A: nested hookSpecificOutput ---
	case HarnessClaudeCode, HarnessCodex, HarnessCodeBuddy, HarnessAugment, HarnessQwen:
		payload := hookSpecificOutputDeny{
			HookSpecificOutput: hookSpecificOutputBody{
				HookEventName:            "PreToolUse",
				PermissionDecision:       "deny",
				PermissionDecisionReason: d.Reason,
			},
		}
		b, err := json.Marshal(payload)
		if err != nil {
			// json.Marshal on typed structs only fails on invalid UTF-8 or cyclic
			// references — neither applies here. Fail closed: no stdout, exit 2.
			return DenyOutput{Stderr: stderr, ExitCode: exitHookBlock}
		}
		return DenyOutput{Stdout: b, Stderr: stderr, ExitCode: exitHookBlock}

	// --- Family B: flat permissionDecision (Copilot) ---
	case HarnessCopilot:
		payload := copilotDeny{
			PermissionDecision:       "deny",
			PermissionDecisionReason: d.Reason,
		}
		b, err := json.Marshal(payload)
		if err != nil {
			return DenyOutput{Stderr: stderr, ExitCode: exitHookBlock}
		}
		return DenyOutput{Stdout: b, Stderr: stderr, ExitCode: exitHookBlock}

	// --- Family C: permission field (Cursor) ---
	case HarnessCursor:
		payload := cursorDeny{
			Permission:   "deny",
			UserMessage:  d.Reason,
			AgentMessage: d.Reason,
		}
		b, err := json.Marshal(payload)
		if err != nil {
			return DenyOutput{Stderr: stderr, ExitCode: exitHookBlock}
		}
		return DenyOutput{Stdout: b, Stderr: stderr, ExitCode: exitHookBlock}

	// --- Family D: decision field (Gemini CLI) ---
	case HarnessGemini:
		payload := geminiDeny{
			Decision: "deny",
			Reason:   d.Reason,
		}
		b, err := json.Marshal(payload)
		if err != nil {
			return DenyOutput{Stderr: stderr, ExitCode: exitHookBlock}
		}
		return DenyOutput{Stdout: b, Stderr: stderr, ExitCode: exitHookBlock}

	// --- Family E: dual-defensive (Antigravity) ---
	case HarnessAntigravity:
		payload := antigravityDeny{
			Decision:           "deny",
			PermissionDecision: "deny",
			DenyReason:         d.Reason,
		}
		b, err := json.Marshal(payload)
		if err != nil {
			return DenyOutput{Stderr: stderr, ExitCode: exitHookBlock}
		}
		return DenyOutput{Stdout: b, Stderr: stderr, ExitCode: exitHookBlock}

	// --- Family F: cancel (Cline) ---
	case HarnessCline:
		payload := clineDeny{
			Cancel:       true,
			ErrorMessage: d.Reason,
		}
		b, err := json.Marshal(payload)
		if err != nil {
			return DenyOutput{Stderr: stderr, ExitCode: exitHookBlock}
		}
		return DenyOutput{Stdout: b, Stderr: stderr, ExitCode: exitHookBlock}

	// --- Family G: Hermes fail-open (JSON is the ONLY block path) ---
	case HarnessHermes:
		// Hermes ignores non-zero exit codes — the block is carried ONLY by the
		// JSON. ExitCode=0 so Hermes does not treat the invocation as an error.
		// The Message field MUST be non-empty.
		msg := d.Reason
		if msg == "" {
			msg = "blocked by beekeeper policy"
		}
		payload := hermesDeny{
			Action:  "block",
			Message: msg,
		}
		b, err := json.Marshal(payload)
		if err != nil {
			// Fallback: use fmt to build a minimal JSON; still exit 0 for Hermes.
			b = []byte(fmt.Sprintf(`{"action":"block","message":%q}`, msg))
		}
		return DenyOutput{Stdout: b, Stderr: stderr, ExitCode: 0}

	// --- Family H: exit-2-only, no stdout JSON ---
	// Windsurf: exit 2 only (fail-open on non-2 exit; no stdout-JSON deny form).
	// OpenCode: exit 2; deny is carried by the JS plugin throwing.
	// Kilo, Trae: MCP-gateway-only; native tools unguarded; exit 2 + stderr.
	case HarnessWindsurf, HarnessOpenCode, HarnessKilo, HarnessTrae:
		return DenyOutput{Stdout: nil, Stderr: stderr, ExitCode: exitHookBlock}

	// --- Unknown/empty HarnessID: fail CLOSED ---
	default:
		// Never silently allow an unknown harness. Exit 2 + stderr, nil Stdout.
		return DenyOutput{Stdout: nil, Stderr: stderr, ExitCode: exitHookBlock}
	}
}
