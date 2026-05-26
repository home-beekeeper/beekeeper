package catalog

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestFetchNPMPublishTime: npm full-package-document stub returns a .time map
// with a known version → parsed timestamp string returned.
func TestFetchNPMPublishTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// npm full-package-document format: .time is a map of version → timestamp.
		_, _ = w.Write([]byte(`{
			"name": "lodash",
			"time": {
				"modified": "2024-01-01T00:00:00.000Z",
				"created": "2012-04-23T18:25:43.511Z",
				"4.17.21": "2021-02-20T15:42:16.891Z"
			}
		}`))
	}))
	defer srv.Close()

	// Override base URL to point to the test server.
	origBase := npmRegistryBase
	npmRegistryBase = srv.URL
	defer func() { npmRegistryBase = origBase }()

	ts, err := fetchNPMPublishTime(context.Background(), srv.Client(), "lodash", "4.17.21")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ts != "2021-02-20T15:42:16.891Z" {
		t.Errorf("timestamp = %q, want %q", ts, "2021-02-20T15:42:16.891Z")
	}
}

// TestFetchNPMLifecycleScripts: npm version-doc stub with a postinstall script
// → returns ["postinstall"].
func TestFetchNPMLifecycleScripts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// npm version document with scripts field.
		_, _ = w.Write([]byte(`{
			"name": "some-pkg",
			"version": "1.0.0",
			"scripts": {
				"test": "jest",
				"postinstall": "node setup.js",
				"build": "webpack"
			}
		}`))
	}))
	defer srv.Close()

	origBase := npmRegistryBase
	npmRegistryBase = srv.URL
	defer func() { npmRegistryBase = origBase }()

	scripts, err := fetchNPMLifecycleScripts(context.Background(), srv.Client(), "some-pkg", "1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scripts) != 1 || scripts[0] != "postinstall" {
		t.Errorf("scripts = %v, want [postinstall]", scripts)
	}
}

// TestFetchPublishTimeUnsupportedEcosystem: unknown ecosystem → non-nil error.
func TestFetchPublishTimeUnsupportedEcosystem(t *testing.T) {
	_, err := fetchPublishTime(context.Background(), http.DefaultClient, "unknown-eco", "pkg", "1.0.0")
	if err == nil {
		t.Errorf("want error for unsupported ecosystem, got nil")
	}
}

// TestFetchNon200IsError: any non-200 HTTP response → non-nil error (caller
// will treat as missing timestamp and fail closed).
func TestFetchNon200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	origBase := npmRegistryBase
	npmRegistryBase = srv.URL
	defer func() { npmRegistryBase = origBase }()

	_, err := fetchNPMPublishTime(context.Background(), srv.Client(), "missing-pkg", "9.9.9")
	if err == nil {
		t.Errorf("want non-nil error on 404, got nil")
	}
}

// TestFetchLifecycleScriptsNonNpmReturnsUnsupported: cargo ecosystem →
// ErrEcosystemLifecycleUnsupported (not nil, not an empty-nil distinction —
// the caller sets RegistryCheckFailed:true → EvaluateLifecycle blocks).
func TestFetchLifecycleScriptsNonNpmReturnsUnsupported(t *testing.T) {
	_, err := fetchLifecycleScripts(context.Background(), http.DefaultClient, "cargo", "serde", "1.0.0")
	if err == nil {
		t.Fatalf("want error for cargo lifecycle scripts, got nil")
	}
	if !errors.Is(err, ErrEcosystemLifecycleUnsupported) {
		t.Errorf("want ErrEcosystemLifecycleUnsupported, got %v", err)
	}
}

// TestFetchNPMPublishTimeMissingVersion: npm stub .time map does not contain
// the requested version → non-nil error.
func TestFetchNPMPublishTimeMissingVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"pkg","time":{"1.0.0":"2024-01-01T00:00:00Z"}}`))
	}))
	defer srv.Close()

	origBase := npmRegistryBase
	npmRegistryBase = srv.URL
	defer func() { npmRegistryBase = origBase }()

	_, err := fetchNPMPublishTime(context.Background(), srv.Client(), "pkg", "9.9.9")
	if err == nil {
		t.Errorf("want error when version not in .time map, got nil")
	}
}

// TestFetchNPMLifecycleNoScripts: npm version doc with no lifecycle script keys
// → empty slice returned, nil error.
func TestFetchNPMLifecycleNoScripts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Only "test" and "build" — not lifecycle scripts.
		_, _ = w.Write([]byte(`{"name":"safe-pkg","version":"1.0.0","scripts":{"test":"jest","build":"tsc"}}`))
	}))
	defer srv.Close()

	origBase := npmRegistryBase
	npmRegistryBase = srv.URL
	defer func() { npmRegistryBase = origBase }()

	scripts, err := fetchNPMLifecycleScripts(context.Background(), srv.Client(), "safe-pkg", "1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scripts) != 0 {
		t.Errorf("scripts = %v, want empty (no lifecycle scripts)", scripts)
	}
}

// TestFetchLifecycleScriptsPyPIUnsupported: pypi → ErrEcosystemLifecycleUnsupported.
func TestFetchLifecycleScriptsPyPIUnsupported(t *testing.T) {
	_, err := fetchLifecycleScripts(context.Background(), http.DefaultClient, "pypi", "requests", "2.31.0")
	if err == nil {
		t.Fatalf("want error for pypi lifecycle scripts, got nil")
	}
	if !errors.Is(err, ErrEcosystemLifecycleUnsupported) {
		t.Errorf("want ErrEcosystemLifecycleUnsupported, got %v", err)
	}
}
