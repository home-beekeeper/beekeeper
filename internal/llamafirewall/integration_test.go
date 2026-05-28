package llamafirewall

import (
	"encoding/json"
	"testing"
)

func TestShouldScanPrompt(t *testing.T) {
	trueCases := []string{"web_search", "read_file"}
	for _, tc := range trueCases {
		if !ShouldScanPrompt(tc) {
			t.Errorf("ShouldScanPrompt(%q) = false, want true", tc)
		}
	}
	falseCases := []string{"execute_bash", "write_file", ""}
	for _, tc := range falseCases {
		if ShouldScanPrompt(tc) {
			t.Errorf("ShouldScanPrompt(%q) = true, want false", tc)
		}
	}
}

func TestShouldScanCode(t *testing.T) {
	if !ShouldScanCode("write_file") {
		t.Error("ShouldScanCode(\"write_file\") = false, want true")
	}
	falseCases := []string{"read_file", "web_search", ""}
	for _, tc := range falseCases {
		if ShouldScanCode(tc) {
			t.Errorf("ShouldScanCode(%q) = true, want false", tc)
		}
	}
}

func TestBuildWarningPayload(t *testing.T) {
	resp := ScanResponse{Result: ResultInjection, Confidence: 0.97, Reason: "test"}
	data := BuildWarningPayload(resp)
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if m["beekeeper_alert"] != true {
		t.Errorf("beekeeper_alert = %v, want true", m["beekeeper_alert"])
	}
	if m["alert_type"] != "prompt_injection" {
		t.Errorf("alert_type = %v, want prompt_injection", m["alert_type"])
	}
	if m["confidence"] != 0.97 {
		t.Errorf("confidence = %v, want 0.97", m["confidence"])
	}
	if m["original_content_redacted"] != true {
		t.Errorf("original_content_redacted = %v, want true", m["original_content_redacted"])
	}
}

func TestBuildWarningPayloadCodeShield(t *testing.T) {
	resp := ScanResponse{Result: ResultUnsafe, Confidence: 0.85}
	data := BuildWarningPayload(resp)
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if m["alert_type"] != "code_unsafe" {
		t.Errorf("alert_type = %v, want code_unsafe", m["alert_type"])
	}
}

func TestBuildWarningPayloadAlignment(t *testing.T) {
	resp := ScanResponse{Result: ResultHijacked, Confidence: 0.91}
	data := BuildWarningPayload(resp)
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if m["alert_type"] != "alignment_hijack" {
		t.Errorf("alert_type = %v, want alignment_hijack", m["alert_type"])
	}
}
