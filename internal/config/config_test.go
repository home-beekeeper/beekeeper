package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileDefaultsClosed(t *testing.T) {
	// A path that does not exist must yield the secure default, not an error.
	path := filepath.Join(t.TempDir(), "does-not-exist.json")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load on missing file returned error: %v", err)
	}
	if cfg.FailMode != FailModeClosed {
		t.Fatalf("FailMode = %q, want %q", cfg.FailMode, FailModeClosed)
	}
	if !cfg.FailClosed() {
		t.Fatal("FailClosed() = false, want true for default config")
	}
}

func TestLoadOpenMode(t *testing.T) {
	path := writeConfig(t, `{"fail_mode":"open"}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.FailMode != FailModeOpen {
		t.Fatalf("FailMode = %q, want %q", cfg.FailMode, FailModeOpen)
	}
	if cfg.FailClosed() {
		t.Fatal("FailClosed() = true, want false for fail_mode=open")
	}
}

func TestLoadWarnMode(t *testing.T) {
	path := writeConfig(t, `{"fail_mode":"warn"}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.FailMode != FailModeWarn {
		t.Fatalf("FailMode = %q, want %q", cfg.FailMode, FailModeWarn)
	}
	if cfg.FailClosed() {
		t.Fatal("FailClosed() = true, want false for fail_mode=warn")
	}
}

func TestLoadInvalidModeErrors(t *testing.T) {
	path := writeConfig(t, `{"fail_mode":"yolo"}`)

	if _, err := Load(path); err == nil {
		t.Fatal("Load with invalid fail_mode returned nil error, want non-nil")
	}
}

func TestEmptyModeDefaultsClosed(t *testing.T) {
	// An empty/omitted fail_mode must default to the secure mode.
	path := writeConfig(t, `{}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.FailMode != FailModeClosed {
		t.Fatalf("FailMode = %q, want %q", cfg.FailMode, FailModeClosed)
	}
	if !cfg.FailClosed() {
		t.Fatal("FailClosed() = false, want true for empty fail_mode")
	}
}

func TestLoadMalformedJSONErrors(t *testing.T) {
	path := writeConfig(t, `{not json}`)

	if _, err := Load(path); err == nil {
		t.Fatal("Load with malformed JSON returned nil error, want non-nil")
	}
}

func TestSocketTokenLoads(t *testing.T) {
	// A config with socket.api_token set must load the token and still default
	// fail_mode to "closed".
	path := writeConfig(t, `{"socket":{"api_token":"tok_abc"}}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got := cfg.SocketAPIToken(); got != "tok_abc" {
		t.Fatalf("SocketAPIToken() = %q, want %q", got, "tok_abc")
	}
	if cfg.FailMode != FailModeClosed {
		t.Fatalf("FailMode = %q, want %q (fail_mode must default to closed when omitted)", cfg.FailMode, FailModeClosed)
	}
	if !cfg.FailClosed() {
		t.Fatal("FailClosed() = false, want true when fail_mode is absent")
	}
}

func TestSocketTokenAbsentIsEmpty(t *testing.T) {
	// A config that only sets fail_mode and omits socket block must return ""
	// from SocketAPIToken() without error.
	path := writeConfig(t, `{"fail_mode":"closed"}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got := cfg.SocketAPIToken(); got != "" {
		t.Fatalf("SocketAPIToken() = %q, want \"\" when socket block absent", got)
	}
}

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
	return path
}
