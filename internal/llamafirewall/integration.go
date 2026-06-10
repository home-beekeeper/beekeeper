// Package llamafirewall provides integration helpers that determine which tool
// calls require LlamaFirewall scanning and how to surface detection results.
package llamafirewall

import "encoding/json"

// ShouldScanPrompt returns true for tool names whose outputs flow into agent
// context and should be scanned by PromptGuard 2 (LLMF-02).
func ShouldScanPrompt(toolName string) bool {
	switch toolName {
	case "web_search", "read_file":
		return true
	}
	return false
}

// ShouldScanCode returns true for tool names that write code and should be
// evaluated by CodeShield (LLMF-03).
func ShouldScanCode(toolName string) bool {
	return toolName == "write_file"
}

// BuildWarningPayload returns the JSON warning payload injected into agent
// context when a scan detects injection, unsafe code, or alignment hijacking.
func BuildWarningPayload(resp ScanResponse) []byte {
	alertType := "prompt_injection"
	reason := resp.Reason
	switch resp.Result {
	case ResultUnsafe:
		alertType = "code_unsafe"
		if reason == "" {
			reason = "CodeShield detected insecure code pattern"
		}
	default:
		if reason == "" {
			reason = "PromptGuard 2 detected indirect prompt injection attempt in tool output"
		}
	}
	payload := map[string]any{
		"beekeeper_alert":            true,
		"alert_type":                 alertType,
		"confidence":                 resp.Confidence,
		"reason":                     reason,
		"original_content_redacted": true,
	}
	data, _ := json.Marshal(payload)
	return data
}
