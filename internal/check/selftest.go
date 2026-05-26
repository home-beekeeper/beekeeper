package check

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "embed"

	"github.com/mzansi-agentive/beekeeper/internal/catalog"
	"github.com/mzansi-agentive/beekeeper/internal/config"
	"github.com/mzansi-agentive/beekeeper/internal/policy"
)

// corpusJSON is the adversarial corpus embedded directly in the binary so
// `beekeeper selftest` has no runtime file dependency and runs identically on
// all three platforms.
//
//go:embed corpus/fixtures.json
var corpusJSON []byte

// selftestEntries are the known threat-intel entries the selftest evaluates
// against. They mirror live Bumblebee corpus cases (the May-2026 Nx Console
// compromise and a shai-hulud npm worm package) so the fixtures have something
// to match. Keeping them here (not fetched) makes the selftest hermetic.
var selftestEntries = []catalog.Entry{
	{
		ID:            "stepsecurity-2026-05-18-vscode-nrwl-angular-console-compromised",
		Name:          "nrwl.angular-console (Nx Console VS Code extension 2026-05-18 compromise)",
		Ecosystem:     "editor-extension",
		Package:       "nrwl.angular-console",
		Versions:      []string{"18.95.0"},
		Severity:      "critical",
		CatalogSource: "bumblebee",
	},
	{
		ID:            "mini-shai-hulud-npm-shai-hulud",
		Name:          "shai-hulud (npm worm package)",
		Ecosystem:     "npm",
		Package:       "shai-hulud",
		Versions:      []string{"1.0.0"},
		Severity:      "critical",
		CatalogSource: "bumblebee",
	},
}

// fixture is one embedded selftest case: a tool call and its expected decision.
type fixture struct {
	Name             string          `json:"name"`
	ToolCall         policy.ToolCall `json:"tool_call"`
	ExpectLevel      string          `json:"expect_level"`
	ExpectAllow      bool            `json:"expect_allow"`
	ExpectCatalogHit bool            `json:"expect_catalog_match"`
}

// RunSelftest evaluates the embedded adversarial corpus against an in-memory
// catalog index built from selftestEntries, plus a malformed-JSON fail-closed
// case routed through RunCheck. It returns the pass/fail counts; err is non-nil
// only on setup failure (e.g. corpus could not be decoded or the index could
// not be built), not on a fixture mismatch.
func RunSelftest() (passed, failed int, err error) {
	var fixtures []fixture
	if e := json.Unmarshal(corpusJSON, &fixtures); e != nil {
		return 0, 0, fmt.Errorf("decode embedded corpus: %w", e)
	}

	// Build a hermetic mmap index in a temp dir from the known entries.
	tmpDir, e := os.MkdirTemp("", "beekeeper-selftest-*")
	if e != nil {
		return 0, 0, fmt.Errorf("create selftest temp dir: %w", e)
	}
	defer os.RemoveAll(tmpDir)

	indexPath := filepath.Join(tmpDir, "selftest.idx")
	if e := catalog.BuildIndex(indexPath, selftestEntries); e != nil {
		return 0, 0, fmt.Errorf("build selftest index: %w", e)
	}
	idx, e := catalog.OpenIndex(indexPath)
	if e != nil {
		return 0, 0, fmt.Errorf("open selftest index: %w", e)
	}
	defer idx.Close()

	// Catalog-match (warn) and allow fixtures: evaluate via the pure engine.
	for _, f := range fixtures {
		d := policy.Evaluate(f.ToolCall, idx)
		if fixtureMatches(f, d) {
			passed++
		} else {
			failed++
			fmt.Printf("selftest FAIL [%s]: got level=%q allow=%v matches=%d, want level=%q allow=%v hit=%v\n",
				f.Name, d.Level, d.Allow, len(d.CatalogMatches), f.ExpectLevel, f.ExpectAllow, f.ExpectCatalogHit)
		}
	}

	// Fail-closed case: malformed JSON on stdin must produce a block through the
	// full RunCheck path (not just Evaluate). Use a default fail-closed config
	// and a throwaway audit log under the temp dir.
	auditPath := filepath.Join(tmpDir, "selftest.ndjson")
	res := RunCheck(context.Background(), strings.NewReader("{bad json}"), config.Config{FailMode: config.FailModeClosed}, indexPath, auditPath)
	if !res.Decision.Allow && res.ExitCode != exitAllow {
		passed++
	} else {
		failed++
		fmt.Printf("selftest FAIL [malformed JSON fail-closed]: got allow=%v exit=%d, want block\n",
			res.Decision.Allow, res.ExitCode)
	}

	return passed, failed, nil
}

// fixtureMatches reports whether decision d satisfies fixture f's expectations.
func fixtureMatches(f fixture, d policy.Decision) bool {
	if d.Level != f.ExpectLevel {
		return false
	}
	if d.Allow != f.ExpectAllow {
		return false
	}
	hit := len(d.CatalogMatches) > 0
	return hit == f.ExpectCatalogHit
}
