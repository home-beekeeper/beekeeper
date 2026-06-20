package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// withBase points a package-level registry base var at u for the test duration.
func withBase(t *testing.T, target *string, u string) {
	t.Helper()
	prev := *target
	*target = u
	t.Cleanup(func() { *target = prev })
}

// TestFetchPyPIPublishTime: PyPI JSON-API stub returns urls[0].upload_time_iso_8601.
func TestFetchPyPIPublishTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"urls":[{"upload_time_iso_8601":"2023-05-22T16:12:00.000000Z"}]}`))
	}))
	defer srv.Close()
	withBase(t, &pypiRegistryBase, srv.URL)

	ts, err := fetchPyPIPublishTime(context.Background(), srv.Client(), "requests", "2.31.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ts != "2023-05-22T16:12:00.000000Z" {
		t.Errorf("ts = %q, want the upload_time_iso_8601 value", ts)
	}
}

// TestFetchPyPIPublishTimeMissing: empty urls array → error.
func TestFetchPyPIPublishTimeMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"urls":[]}`))
	}))
	defer srv.Close()
	withBase(t, &pypiRegistryBase, srv.URL)

	if _, err := fetchPyPIPublishTime(context.Background(), srv.Client(), "x", "1.0.0"); err == nil {
		t.Error("want error for empty urls, got nil")
	}
}

// TestFetchCratesPublishTime: crates.io stub returns version.created_at.
func TestFetchCratesPublishTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"version":{"created_at":"2024-02-01T10:00:00.000000+00:00"}}`))
	}))
	defer srv.Close()
	withBase(t, &cratesRegistryBase, srv.URL)

	ts, err := fetchCratesPublishTime(context.Background(), srv.Client(), "serde", "1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ts != "2024-02-01T10:00:00.000000+00:00" {
		t.Errorf("ts = %q, want version.created_at", ts)
	}
}

// TestFetchCratesPublishTimeMissing: absent created_at → error.
func TestFetchCratesPublishTimeMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"version":{}}`))
	}))
	defer srv.Close()
	withBase(t, &cratesRegistryBase, srv.URL)

	if _, err := fetchCratesPublishTime(context.Background(), srv.Client(), "serde", "1.0.0"); err == nil {
		t.Error("want error for missing created_at, got nil")
	}
}

// TestFetchRubyGemsPublishTime: versions array stub → built_at for matching number.
func TestFetchRubyGemsPublishTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"number":"6.0.0","built_at":"2019-08-16T00:00:00.000Z"},{"number":"7.0.0","built_at":"2021-12-05T00:00:00.000Z"}]`))
	}))
	defer srv.Close()
	withBase(t, &rubygemsRegistryBase, srv.URL)

	ts, err := fetchRubyGemsPublishTime(context.Background(), srv.Client(), "rails", "7.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ts != "2021-12-05T00:00:00.000Z" {
		t.Errorf("ts = %q, want the built_at for version 7.0.0", ts)
	}
}

// TestFetchRubyGemsPublishTimeVersionNotFound: requested version absent → error.
func TestFetchRubyGemsPublishTimeVersionNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"number":"6.0.0","built_at":"2019-08-16T00:00:00.000Z"}]`))
	}))
	defer srv.Close()
	withBase(t, &rubygemsRegistryBase, srv.URL)

	if _, err := fetchRubyGemsPublishTime(context.Background(), srv.Client(), "rails", "9.9.9"); err == nil {
		t.Error("want error when version not present, got nil")
	}
}

// TestFetchRubyGemsPublishTimeEmptyBuiltAt: matching version but empty built_at → error.
func TestFetchRubyGemsPublishTimeEmptyBuiltAt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"number":"1.0.0","built_at":""}]`))
	}))
	defer srv.Close()
	withBase(t, &rubygemsRegistryBase, srv.URL)

	if _, err := fetchRubyGemsPublishTime(context.Background(), srv.Client(), "x", "1.0.0"); err == nil {
		t.Error("want error for empty built_at, got nil")
	}
}

// TestFetchGoPublishTime: Go proxy .info stub → .Time.
func TestFetchGoPublishTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"Version":"v1.2.3","Time":"2022-06-01T12:00:00Z"}`))
	}))
	defer srv.Close()
	withBase(t, &goProxyBase, srv.URL)

	ts, err := fetchGoPublishTime(context.Background(), srv.Client(), "github.com/foo/bar", "v1.2.3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ts != "2022-06-01T12:00:00Z" {
		t.Errorf("ts = %q, want .Time", ts)
	}
}

// TestFetchGoPublishTimeMissing: absent .Time → error.
func TestFetchGoPublishTimeMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"Version":"v1.2.3"}`))
	}))
	defer srv.Close()
	withBase(t, &goProxyBase, srv.URL)

	if _, err := fetchGoPublishTime(context.Background(), srv.Client(), "m", "v1.2.3"); err == nil {
		t.Error("want error for missing .Time, got nil")
	}
}

// TestFetchPackagistPublishTime: p2 packages map → .time for matching version.
func TestFetchPackagistPublishTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"packages":{"monolog/monolog":[{"version":"3.0.0","time":"2022-08-01T00:00:00+00:00"},{"version":"2.0.0","time":"2019-01-01T00:00:00+00:00"}]}}`))
	}))
	defer srv.Close()
	withBase(t, &packagistRegistryBase, srv.URL)

	ts, err := fetchPackagistPublishTime(context.Background(), srv.Client(), "monolog/monolog", "3.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ts != "2022-08-01T00:00:00+00:00" {
		t.Errorf("ts = %q, want the .time for 3.0.0", ts)
	}
}

// TestFetchPackagistPublishTimePackageMissing: package key absent in map → error.
func TestFetchPackagistPublishTimePackageMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"packages":{}}`))
	}))
	defer srv.Close()
	withBase(t, &packagistRegistryBase, srv.URL)

	if _, err := fetchPackagistPublishTime(context.Background(), srv.Client(), "vendor/pkg", "1.0.0"); err == nil {
		t.Error("want error when package not in response, got nil")
	}
}

// TestFetchPackagistPublishTimeVersionMissing: package present but version absent → error.
func TestFetchPackagistPublishTimeVersionMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"packages":{"vendor/pkg":[{"version":"1.0.0","time":"2020-01-01T00:00:00+00:00"}]}}`))
	}))
	defer srv.Close()
	withBase(t, &packagistRegistryBase, srv.URL)

	if _, err := fetchPackagistPublishTime(context.Background(), srv.Client(), "vendor/pkg", "9.9.9"); err == nil {
		t.Error("want error when version not found, got nil")
	}
}

// TestFetchPublishTimeDispatch routes each supported ecosystem to its fetcher and
// returns the stubbed timestamp. Each ecosystem points its base var at a server
// that always replies with the shape the matching fetcher expects.
func TestFetchPublishTimeDispatch(t *testing.T) {
	cases := []struct {
		eco      string
		base     *string
		body     string
		pkg, ver string
		want     string
	}{
		{"pypi", &pypiRegistryBase, `{"urls":[{"upload_time_iso_8601":"2023-01-01T00:00:00Z"}]}`, "p", "1", "2023-01-01T00:00:00Z"},
		{"cargo", &cratesRegistryBase, `{"version":{"created_at":"2023-02-02T00:00:00Z"}}`, "c", "1", "2023-02-02T00:00:00Z"},
		{"rubygems", &rubygemsRegistryBase, `[{"number":"1","built_at":"2023-03-03T00:00:00Z"}]`, "g", "1", "2023-03-03T00:00:00Z"},
		{"go", &goProxyBase, `{"Time":"2023-04-04T00:00:00Z"}`, "m", "v1", "2023-04-04T00:00:00Z"},
		{"packagist", &packagistRegistryBase, `{"packages":{"v/p":[{"version":"1","time":"2023-05-05T00:00:00Z"}]}}`, "v/p", "1", "2023-05-05T00:00:00Z"},
	}
	for _, tc := range cases {
		t.Run(tc.eco, func(t *testing.T) {
			body := tc.body
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()
			withBase(t, tc.base, srv.URL)

			got, err := fetchPublishTime(context.Background(), srv.Client(), tc.eco, tc.pkg, tc.ver)
			if err != nil {
				t.Fatalf("fetchPublishTime(%s): %v", tc.eco, err)
			}
			if got != tc.want {
				t.Errorf("fetchPublishTime(%s) = %q, want %q", tc.eco, got, tc.want)
			}
		})
	}
}

// TestFetchPublishTimeNonNpmErrorsPropagate: a non-200 from any non-npm ecosystem
// fetcher surfaces an error (caller fails closed).
func TestFetchPublishTimeNonNpmErrorsPropagate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	withBase(t, &pypiRegistryBase, srv.URL)

	if _, err := fetchPublishTime(context.Background(), srv.Client(), "pypi", "x", "1.0.0"); err == nil {
		t.Error("want error on 503, got nil")
	}
}

// TestFetchRegistryJSONBadBody: 200 with non-JSON body → decode error.
func TestFetchRegistryJSONBadBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json at all`))
	}))
	defer srv.Close()

	var dst struct {
		X string `json:"x"`
	}
	if err := fetchRegistryJSON(context.Background(), srv.Client(), srv.URL, &dst); err == nil {
		t.Error("want decode error for non-JSON body, got nil")
	}
}
