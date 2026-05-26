package editorinit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestPatchEditorSettings verifies that DisableExtensionAutoUpdate adds the
// extensions.autoUpdate key while preserving existing keys.
func TestPatchEditorSettings(t *testing.T) {
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "settings.json")

	// Seed with an existing setting.
	seed := `{"editor.fontSize":14}`
	if err := os.WriteFile(tmpPath, []byte(seed), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if err := DisableExtensionAutoUpdate(tmpPath); err != nil {
		t.Fatalf("DisableExtensionAutoUpdate: %v", err)
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	// Both keys must be present.
	if v, ok := result["editor.fontSize"]; !ok || v.(float64) != 14 {
		t.Errorf("editor.fontSize: expected 14, got %v (present=%v)", v, ok)
	}
	if v, ok := result["extensions.autoUpdate"]; !ok || v.(bool) != false {
		t.Errorf("extensions.autoUpdate: expected false, got %v (present=%v)", v, ok)
	}
}

// TestPatchEditorSettingsIdempotent verifies that calling DisableExtensionAutoUpdate
// twice does not duplicate the key.
func TestPatchEditorSettingsIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "settings.json")

	// First call creates the file.
	if err := DisableExtensionAutoUpdate(tmpPath); err != nil {
		t.Fatalf("first call: %v", err)
	}
	// Second call should be a no-op structurally.
	if err := DisableExtensionAutoUpdate(tmpPath); err != nil {
		t.Fatalf("second call: %v", err)
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	// extensions.autoUpdate must appear exactly once with value false.
	v, ok := result["extensions.autoUpdate"]
	if !ok {
		t.Errorf("extensions.autoUpdate missing after two calls")
	} else if v.(bool) != false {
		t.Errorf("extensions.autoUpdate: expected false, got %v", v)
	}

	// JSON map semantics guarantee uniqueness; re-encode and check raw JSON too.
	raw, _ := json.Marshal(result)
	count := 0
	for i := 0; i < len(raw)-len("extensions.autoUpdate"); i++ {
		if string(raw[i:i+len("extensions.autoUpdate")]) == "extensions.autoUpdate" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("extensions.autoUpdate appears %d times in JSON, expected 1", count)
	}
}

// TestPatchEditorSettingsJSONC verifies that a JSONC file (with comments) is
// handled gracefully: the result must be valid JSON with both expected keys,
// even though comments are stripped.
func TestPatchEditorSettingsJSONC(t *testing.T) {
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "settings.json")

	// Seed with JSONC-style comment.
	jsonc := "// a comment\n{\"editor.fontSize\":14}"
	if err := os.WriteFile(tmpPath, []byte(jsonc), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if err := DisableExtensionAutoUpdate(tmpPath); err != nil {
		t.Fatalf("DisableExtensionAutoUpdate: %v", err)
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}

	// Result must be valid JSON (not JSONC).
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("result is not valid JSON: %v\n%s", err, data)
	}

	// Both keys must be present (comment loss is acceptable).
	if v, ok := result["editor.fontSize"]; !ok || v.(float64) != 14 {
		t.Errorf("editor.fontSize: expected 14, got %v (present=%v)", v, ok)
	}
	if v, ok := result["extensions.autoUpdate"]; !ok || v.(bool) != false {
		t.Errorf("extensions.autoUpdate: expected false, got %v (present=%v)", v, ok)
	}
}

// TestPatchEditorSettingsCreateFile verifies that PatchSettings creates the
// file (and any missing parent directories) when it does not yet exist.
func TestPatchEditorSettingsCreateFile(t *testing.T) {
	tmpDir := t.TempDir()
	// Use a nested path that does not exist yet.
	tmpPath := filepath.Join(tmpDir, "nonexistent", "subdir", "settings.json")

	if err := DisableExtensionAutoUpdate(tmpPath); err != nil {
		t.Fatalf("DisableExtensionAutoUpdate on non-existent path: %v", err)
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("created file is not valid JSON: %v\n%s", err, data)
	}

	// Only extensions.autoUpdate should be present (no other keys seeded).
	if v, ok := result["extensions.autoUpdate"]; !ok || v.(bool) != false {
		t.Errorf("extensions.autoUpdate: expected false, got %v (present=%v)", v, ok)
	}
}
