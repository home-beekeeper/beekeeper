package editorinit

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
)

// TestDetectEditors verifies that DetectEditors correctly filters editors based
// on injectable lookPath and statFunc stubs.
func TestDetectEditors(t *testing.T) {
	// Determine the expected extension dirs from the static table so the test
	// is not duplicating hardcoded paths.
	descriptors := knownEditors()
	var cursorExtDir, windsurfExtDir string
	for _, d := range descriptors {
		switch d.name {
		case "Cursor":
			cursorExtDir = d.extensionDir
		case "Windsurf":
			windsurfExtDir = d.extensionDir
		}
	}
	if cursorExtDir == "" || windsurfExtDir == "" {
		t.Fatal("expected cursor and windsurf descriptors in knownEditors()")
	}

	// Stub: "code" found on PATH; all other executables not found.
	origLook := lookPath
	lookPath = func(name string) (string, error) {
		if name == "code" {
			return "/usr/bin/code", nil
		}
		return "", errors.New("not found")
	}
	defer func() { lookPath = origLook }()

	// Stub: cursor extension dir exists; windsurf extension dir does not.
	origStat := statFunc
	statFunc = func(path string) (fs.FileInfo, error) {
		if path == cursorExtDir {
			return nil, nil // exists (nil error)
		}
		return nil, errors.New("not found")
	}
	defer func() { statFunc = origStat }()

	editors, err := DetectEditors()
	if err != nil {
		t.Fatalf("DetectEditors() returned unexpected error: %v", err)
	}

	// Expect exactly two editors: VS Code and Cursor.
	if len(editors) != 2 {
		t.Fatalf("expected 2 editors, got %d: %v", len(editors), editorNames(editors))
	}

	// Verify VS Code.
	vscode := findEditor(editors, "VS Code")
	if vscode == nil {
		t.Fatalf("VS Code not found in results: %v", editorNames(editors))
	}
	if !vscode.ExecutableFound {
		t.Errorf("VS Code: expected ExecutableFound=true")
	}
	if vscode.Executable == "" {
		t.Errorf("VS Code: expected non-empty Executable")
	}
	assertPathUsesSeparator(t, "VS Code ExtensionDir", vscode.ExtensionDir)
	assertPathUsesSeparator(t, "VS Code SettingsPath", vscode.SettingsPath)
	if vscode.ExtensionDir == "" {
		t.Errorf("VS Code: expected non-empty ExtensionDir")
	}
	if vscode.SettingsPath == "" {
		t.Errorf("VS Code: expected non-empty SettingsPath")
	}

	// Verify Cursor.
	cursor := findEditor(editors, "Cursor")
	if cursor == nil {
		t.Fatalf("Cursor not found in results: %v", editorNames(editors))
	}
	if cursor.ExecutableFound {
		t.Errorf("Cursor: expected ExecutableFound=false (executable not on PATH)")
	}
	if cursor.ExtensionDir == "" {
		t.Errorf("Cursor: expected non-empty ExtensionDir")
	}
	if cursor.SettingsPath == "" {
		t.Errorf("Cursor: expected non-empty SettingsPath")
	}
	assertPathUsesSeparator(t, "Cursor ExtensionDir", cursor.ExtensionDir)
	assertPathUsesSeparator(t, "Cursor SettingsPath", cursor.SettingsPath)

	// Windsurf must NOT appear.
	if windsurf := findEditor(editors, "Windsurf"); windsurf != nil {
		t.Errorf("Windsurf should be absent (neither executable nor dir found)")
	}
}

// assertPathUsesSeparator checks that a path contains the platform separator
// rather than a hardcoded forward slash only (catches filepath.Join regressions).
func assertPathUsesSeparator(t *testing.T, label, path string) {
	t.Helper()
	// filepath.Join always uses os.PathSeparator; the resulting path must
	// contain at least one separator character on any platform.
	sep := string(filepath.Separator)
	if sep == "/" {
		// On Unix, "/" is valid and expected — just ensure path is non-empty.
		if path == "" {
			t.Errorf("%s: empty path", label)
		}
		return
	}
	// On Windows, backslash must appear.
	for _, ch := range path {
		if string(ch) == sep {
			return
		}
	}
	t.Errorf("%s: path %q does not contain platform separator %q", label, path, sep)
}

// findEditor returns the first Editor whose Name matches, or nil.
func findEditor(editors []Editor, name string) *Editor {
	for i := range editors {
		if editors[i].Name == name {
			return &editors[i]
		}
	}
	return nil
}

// editorNames returns a slice of editor names for debug output.
func editorNames(editors []Editor) []string {
	names := make([]string, len(editors))
	for i, e := range editors {
		names[i] = e.Name
	}
	return names
}
