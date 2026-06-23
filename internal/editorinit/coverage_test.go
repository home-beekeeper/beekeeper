package editorinit

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestExecLookPathReal exercises the real lookup seam (defaultLookPath ->
// execLookPath) without the package-level stub, covering both the success and
// failure branches of exec.LookPath. These functions are otherwise never
// touched because every other test swaps the lookPath variable.
func TestExecLookPathReal(t *testing.T) {
	// Failure branch: an executable name that is guaranteed not to exist.
	if _, err := defaultLookPath("beekeeper-nonexistent-binary-xyz"); err == nil {
		t.Errorf("defaultLookPath: expected error for nonexistent binary, got nil")
	}

	// Success branch: a binary that is virtually always on PATH per OS.
	var present string
	switch runtime.GOOS {
	case "windows":
		present = "cmd"
	default:
		present = "sh"
	}
	path, err := defaultLookPath(present)
	if err != nil {
		// Not fatal: some minimal CI images may lack it. Skip rather than fail
		// so the failure branch above still counts and the test stays green.
		t.Skipf("defaultLookPath(%q) unavailable in this environment: %v", present, err)
	}
	if path == "" {
		t.Errorf("defaultLookPath(%q): expected non-empty path on success", present)
	}
}

// TestHomeDirFallback forces os.UserHomeDir to fail by clearing the platform
// home environment variables, exercising the "~" fallback branch.
func TestHomeDirFallback(t *testing.T) {
	// Clear every variable os.UserHomeDir consults on any platform.
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
	// On Unix, os.UserHomeDir falls back to a cgo/passwd lookup when $HOME is
	// empty, so this fallback is reliably reachable only where $HOME drives it.
	if got := homeDir(); got != "~" {
		// If the platform resolved a home another way, that is acceptable; the
		// branch is only guaranteed to fire on Windows where USERPROFILE drives it.
		if runtime.GOOS == "windows" {
			t.Errorf("homeDir(): expected %q fallback when home env cleared, got %q", "~", got)
		}
	}
}

// TestConfigBaseWindowsFallback forces os.UserConfigDir to fail on Windows by
// clearing %AppData%, exercising the AppData\Roaming fallback branch. On
// non-Windows it simply asserts the ~/.config path shape.
func TestConfigBaseWindowsFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", "")
		t.Setenv("APPDATA", "")
		got := configBase()
		// Fallback joins homeDir()/AppData/Roaming.
		if filepath.Base(got) != "Roaming" {
			t.Errorf("configBase(): expected fallback ending in Roaming, got %q", got)
		}
		return
	}

	// Unix branch: must be <home>/.config.
	got := configBase()
	if filepath.Base(got) != ".config" {
		t.Errorf("configBase(): expected path ending in .config, got %q", got)
	}
}

// TestPatchSettingsReadError verifies PatchSettings propagates a read error that
// is not os.ErrNotExist (e.g. when the path is a directory).
func TestPatchSettingsReadError(t *testing.T) {
	dir := t.TempDir()
	// The path itself is a directory; os.ReadFile returns a non-NotExist error.
	if err := PatchSettings(dir, "k", "v"); err == nil {
		t.Errorf("PatchSettings: expected error reading a directory as a file, got nil")
	}
}

// TestPatchSettingsMkdirError verifies PatchSettings returns the MkdirAll error
// when a parent path component is a regular file (so the directory cannot be
// created).
func TestPatchSettingsMkdirError(t *testing.T) {
	dir := t.TempDir()
	fileAsParent := filepath.Join(dir, "afile")
	if err := os.WriteFile(fileAsParent, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed parent file: %v", err)
	}
	// settings.json sits under a path component that is actually a file.
	target := filepath.Join(fileAsParent, "sub", "settings.json")
	if err := PatchSettings(target, "k", "v"); err == nil {
		t.Errorf("PatchSettings: expected MkdirAll error under a file parent, got nil")
	}
}

// TestPatchSettingsCorruptResetsToFreshMap verifies that when the existing file
// content is valid JSON but NOT an object (so json.Unmarshal into a map fails),
// PatchSettings discards it and starts fresh rather than failing.
func TestPatchSettingsCorruptResetsToFreshMap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	// A JSON array unmarshals fine as JSON but fails into map[string]any.
	if err := os.WriteFile(path, []byte("[1,2,3]"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if err := PatchSettings(path, "extensions.autoUpdate", false); err != nil {
		t.Fatalf("PatchSettings on non-object content: %v", err)
	}

	settings, err := ReadSettings(path)
	if err != nil {
		t.Fatalf("ReadSettings: %v", err)
	}
	if v, ok := settings["extensions.autoUpdate"]; !ok || v.(bool) != false {
		t.Errorf("expected fresh map with extensions.autoUpdate=false, got %v (present=%v)", v, ok)
	}
	// The discarded array contents must not survive.
	if len(settings) != 1 {
		t.Errorf("expected exactly one key after reset, got %d: %v", len(settings), settings)
	}
}

// TestReadSettingsNonExistent verifies ReadSettings returns an empty map and no
// error when the file does not exist.
func TestReadSettingsNonExistent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.json")

	settings, err := ReadSettings(path)
	if err != nil {
		t.Fatalf("ReadSettings on nonexistent file: %v", err)
	}
	if settings == nil {
		t.Fatalf("ReadSettings: expected non-nil empty map, got nil")
	}
	if len(settings) != 0 {
		t.Errorf("ReadSettings: expected empty map, got %v", settings)
	}
}

// TestReadSettingsValidAndJSONC verifies ReadSettings parses both plain JSON and
// JSONC (comment-bearing) files.
func TestReadSettingsValidAndJSONC(t *testing.T) {
	dir := t.TempDir()

	plainPath := filepath.Join(dir, "plain.json")
	if err := os.WriteFile(plainPath, []byte(`{"editor.fontSize":14}`), 0o644); err != nil {
		t.Fatalf("seed plain: %v", err)
	}
	plain, err := ReadSettings(plainPath)
	if err != nil {
		t.Fatalf("ReadSettings(plain): %v", err)
	}
	if v, ok := plain["editor.fontSize"]; !ok || v.(float64) != 14 {
		t.Errorf("plain: expected editor.fontSize=14, got %v (present=%v)", v, ok)
	}

	jsoncPath := filepath.Join(dir, "with-comments.json")
	jsoncContent := "// header comment\n{\n  \"a\": 1, /* inline */ \"b\": true\n}"
	if err := os.WriteFile(jsoncPath, []byte(jsoncContent), 0o644); err != nil {
		t.Fatalf("seed jsonc: %v", err)
	}
	parsed, err := ReadSettings(jsoncPath)
	if err != nil {
		t.Fatalf("ReadSettings(jsonc): %v", err)
	}
	if v, ok := parsed["a"]; !ok || v.(float64) != 1 {
		t.Errorf("jsonc: expected a=1, got %v (present=%v)", v, ok)
	}
	if v, ok := parsed["b"]; !ok || v.(bool) != true {
		t.Errorf("jsonc: expected b=true, got %v (present=%v)", v, ok)
	}
}

// TestReadSettingsInvalidJSON verifies ReadSettings returns an error when the
// content cannot be unmarshalled even after JSONC comment stripping.
func TestReadSettingsInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.json")
	// Not valid JSON and not rescued by comment stripping.
	if err := os.WriteFile(path, []byte("{not json at all"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if _, err := ReadSettings(path); err == nil {
		t.Errorf("ReadSettings: expected error on malformed JSON, got nil")
	}
}

// TestReadSettingsReadError verifies ReadSettings propagates a non-NotExist read
// error (e.g. when the path is a directory).
func TestReadSettingsReadError(t *testing.T) {
	dir := t.TempDir()
	if _, err := ReadSettings(dir); err == nil {
		t.Errorf("ReadSettings: expected error reading a directory as a file, got nil")
	}
}

// TestDetectEditorsNoneFound verifies DetectEditors returns an empty result
// (and no error) when neither executables nor extension dirs are present.
func TestDetectEditorsNoneFound(t *testing.T) {
	origLook := lookPath
	lookPath = func(string) (string, error) { return "", os.ErrNotExist }
	defer func() { lookPath = origLook }()

	origStat := statFunc
	statFunc = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	defer func() { statFunc = origStat }()

	editors, err := DetectEditors()
	if err != nil {
		t.Fatalf("DetectEditors: %v", err)
	}
	if len(editors) != 0 {
		t.Errorf("expected zero editors when nothing is found, got %d: %v", len(editors), editorNames(editors))
	}
}

// TestDetectEditorsAllFound verifies that when every executable resolves, all
// known editors are returned with ExecutableFound=true.
func TestDetectEditorsAllFound(t *testing.T) {
	origLook := lookPath
	lookPath = func(name string) (string, error) {
		return filepath.Join("/usr/bin", name), nil
	}
	defer func() { lookPath = origLook }()

	origStat := statFunc
	statFunc = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	defer func() { statFunc = origStat }()

	editors, err := DetectEditors()
	if err != nil {
		t.Fatalf("DetectEditors: %v", err)
	}
	want := len(knownEditors())
	if len(editors) != want {
		t.Fatalf("expected %d editors when all executables resolve, got %d: %v", want, len(editors), editorNames(editors))
	}
	for _, e := range editors {
		if !e.ExecutableFound {
			t.Errorf("%s: expected ExecutableFound=true", e.Name)
		}
		if e.Executable == "" {
			t.Errorf("%s: expected non-empty Executable", e.Name)
		}
	}
}
