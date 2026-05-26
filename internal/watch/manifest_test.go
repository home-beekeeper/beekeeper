package watch

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestParseManifest(t *testing.T) {
	dir := filepath.Join("testdata", "valid-extension")
	m, err := ParseManifest(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Publisher != "ms-python" {
		t.Errorf("Publisher = %q, want %q", m.Publisher, "ms-python")
	}
	if m.Name != "python" {
		t.Errorf("Name = %q, want %q", m.Name, "python")
	}
	if m.Version != "2026.4.0" {
		t.Errorf("Version = %q, want %q", m.Version, "2026.4.0")
	}
}

func TestParseManifestNonExtension(t *testing.T) {
	t.Run("no package.json", func(t *testing.T) {
		dir := t.TempDir()
		_, err := ParseManifest(dir)
		if !errors.Is(err, ErrNoManifest) {
			t.Fatalf("want ErrNoManifest, got %v", err)
		}
	})

	t.Run("empty publisher", func(t *testing.T) {
		dir := t.TempDir()
		content := []byte(`{"publisher":"","name":"some-ext","version":"1.0.0"}`)
		if err := os.WriteFile(filepath.Join(dir, "package.json"), content, 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := ParseManifest(dir)
		if !errors.Is(err, ErrNoManifest) {
			t.Fatalf("want ErrNoManifest for empty publisher, got %v", err)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		dir := t.TempDir()
		content := []byte(`{"publisher":"ms-python","name":"","version":"1.0.0"}`)
		if err := os.WriteFile(filepath.Join(dir, "package.json"), content, 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := ParseManifest(dir)
		if !errors.Is(err, ErrNoManifest) {
			t.Fatalf("want ErrNoManifest for empty name, got %v", err)
		}
	})
}
