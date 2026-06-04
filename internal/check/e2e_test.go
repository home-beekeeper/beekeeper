//go:build e2e

// RELEASE GATE: This file is required for Beekeeper v1.2.0 release.
// TestE2ELiveBinary must pass (all sub-cases green) before any v1.2.0 tag is cut.
// Run: go test -tags e2e -run=TestE2ELiveBinary ./internal/check/...
// It builds the real beekeeper binary and exercises it with raw stdin JSON against
// a hermetic BEEKEEPER_HOME temp directory, proving the shipped binary behaves
// correctly for SPATH+CORR+NUDGE scenarios (BTEST-03).

package check

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/bantuson/beekeeper/internal/catalog"
)

// TestE2ELiveBinary is the v1.2.0 release-gate test. It:
//  1. Compiles the real beekeeper binary into a temp dir.
//  2. Creates a hermetic BEEKEEPER_HOME temp dir (Plan 05 / BTEST-03 A2).
//  3. Seeds a minimal catalog index with the entries needed for each case.
//  4. Drives each case through `beekeeper check` with raw stdin JSON.
//  5. Asserts exit codes AND reads the NDJSON audit record for each case.
//
// SPATH case:  credential read → exit 1 / decision "block"
// CORR case:   ai-figure critical install → exit 1 / decision "block"
// NUDGE case:  pnpm add chalk → exit 0 / record_type "nudge" / decision non-block
// BUN case:    bun add chalk → skipped when bun is absent (exec.LookPath fails)
//
// The E2E uses the REAL pnpm binary on PATH (no DetectStateFn swap — child process).
// State/audit/catalogs all resolve under BEEKEEPER_HOME so the developer's real
// ~/.beekeeper is never touched (T-08-23).
func TestE2ELiveBinary(t *testing.T) {
	// --- Build the beekeeper binary ---
	binName := "beekeeper"
	if runtime.GOOS == "windows" {
		binName = "beekeeper.exe"
	}
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, binName)

	buildCtx, buildCancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer buildCancel()

	buildOut, buildErr := exec.CommandContext(buildCtx,
		"go", "build",
		"-o", binPath,
		"github.com/bantuson/beekeeper/cmd/beekeeper",
	).CombinedOutput()
	if buildErr != nil {
		t.Fatalf("go build failed: %v\n%s", buildErr, buildOut)
	}

	// --- Create hermetic state/audit directory (Plan 05 BEEKEEPER_HOME override) ---
	// Each sub-case gets its OWN homeDir so audit files don't cross-contaminate.
	// The hermetic root is $homeDir; platform.StateDir() returns $homeDir/beekeeper.
	newHome := func(t *testing.T) (homeDir, auditPath, catalogsDir, policiesDir string) {
		t.Helper()
		homeDir = t.TempDir()
		stateDir := filepath.Join(homeDir, "beekeeper")
		auditPath = filepath.Join(stateDir, "audit", "beekeeper.ndjson")
		catalogsDir = filepath.Join(stateDir, "catalogs")
		policiesDir = filepath.Join(stateDir, "policies")
		for _, d := range []string{
			filepath.Join(stateDir, "audit"),
			catalogsDir,
			policiesDir,
		} {
			if err := os.MkdirAll(d, 0o700); err != nil {
				t.Fatalf("mkdir %s: %v", d, err)
			}
		}
		return homeDir, auditPath, catalogsDir, policiesDir
	}

	// seedCatalog writes a minimal Bumblebee index to catalogsDir/bumblebee.idx
	// with the given entries and returns the index path.
	seedCatalog := func(t *testing.T, catalogsDir string, entries []catalog.Entry) string {
		t.Helper()
		idxPath := filepath.Join(catalogsDir, "bumblebee.idx")
		if err := catalog.BuildIndex(idxPath, entries); err != nil {
			t.Fatalf("BuildIndex: %v", err)
		}
		return idxPath
	}

	// runCase runs `beekeeper check` with stdinJSON against the given BEEKEEPER_HOME
	// and returns (exitCode, audit-record-for-wantType or zero).
	runCase := func(t *testing.T, homeDir, auditPath, stdinJSON, wantRecordType string) (exitCode int, rec e2eAuditRecord) {
		t.Helper()
		cmd := exec.Command(binPath, "check")
		cmd.Stdin = strings.NewReader(stdinJSON)
		// Set BEEKEEPER_HOME; inherit the rest of the environment (needed for PATH
		// so the binary can find pnpm/bun/node for nudge detection).
		cmd.Env = append(os.Environ(), fmt.Sprintf("BEEKEEPER_HOME=%s", homeDir))

		if err := cmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ProcessState.ExitCode()
			} else {
				t.Logf("cmd.Run non-exit error: %v", err)
			}
		} else {
			exitCode = 0
		}

		// Read the last record of wantRecordType from the audit file.
		rec = readE2EAuditRecord(t, auditPath, wantRecordType)
		return exitCode, rec
	}

	// -------------------------------------------------------------------------
	// Case 1: SPATH — credential read must block (exit 1, decision "block").
	// -------------------------------------------------------------------------
	t.Run("SPATH_credential_block", func(t *testing.T) {
		homeDir, auditPath, catalogsDir, _ := newHome(t)
		// Minimal catalog (empty — SPATH block does not need catalog entries).
		seedCatalog(t, catalogsDir, nil)

		stdinJSON := `{"agent_name":"e2e-agent","tool_name":"Read","tool_input":{"file_path":"~/.aws/credentials"}}`
		exitCode, rec := runCase(t, homeDir, auditPath, stdinJSON, "policy_decision")

		if exitCode != 1 {
			t.Errorf("SPATH: exit code = %d, want 1 (credential read must block)", exitCode)
		}
		if rec.Decision != "block" {
			t.Errorf("SPATH: audit decision = %q, want %q", rec.Decision, "block")
		}
	})

	// -------------------------------------------------------------------------
	// Case 2: CORR — ai-figure critical install must block (exit 1, decision "block").
	//
	// CLEAN-01: This sub-case is hermetic — the block fires from the LOCAL signed
	// catalog fixture alone, with no OSV network call required. The seeded entry is:
	//   - SIGNED (CatalogSignature non-empty → Signed:true in the bumblebee adapter)
	//   - NON-WILDCARD (Versions:["1.0.0"]) so the all-versions wildcard guard in
	//     findSeverityOverride does NOT suppress escalation
	//   - Severity "critical" matching SeverityOverrides["critical"].BlockAt=1
	// With one signed critical source + CatalogHealthy:true (no state.json degradation),
	// effectiveBlockAt=1 → signedCount(1) >= 1 → decision "block".
	// This mirrors the hermetic unit test TestRunCheckAiFigureBlocks exactly.
	// The stdin command uses the matching version ("npm install ai-figure@1.0.0") so the
	// non-wildcard version entry matches. OSV is unreachable → case still blocks.
	// -------------------------------------------------------------------------
	t.Run("CORR_aifigure_critical_block", func(t *testing.T) {
		homeDir, auditPath, catalogsDir, _ := newHome(t)
		seedCatalog(t, catalogsDir, []catalog.Entry{
			{
				ID:               "e2e-ai-figure-critical-signed",
				Name:             "ai-figure critical supply-chain compromise",
				Ecosystem:        "npm",
				Package:          "ai-figure",
				Versions:         []string{"1.0.0"},
				Severity:         "critical",
				CatalogSource:    "bumblebee",
				CatalogSignature: "sha256:e2e-corr-test-sig", // non-empty → Signed:true in adapter
			},
		})

		stdinJSON := `{"agent_name":"e2e-agent","tool_name":"Bash","tool_input":{"command":"npm install ai-figure@1.0.0"}}`
		exitCode, rec := runCase(t, homeDir, auditPath, stdinJSON, "policy_decision")

		if exitCode != 1 {
			t.Errorf("CORR: exit code = %d, want 1 (ai-figure critical must block)", exitCode)
		}
		if rec.Decision != "block" {
			t.Errorf("CORR: audit decision = %q, want %q", rec.Decision, "block")
		}
	})

	// -------------------------------------------------------------------------
	// Case 3: NUDGE — pnpm add chalk must parse + emit a record_type "nudge" record.
	//
	// This case uses the REAL pnpm binary on PATH (no DetectStateFn swap — it is a
	// child process). pnpm 11.x is installed on this dev box so the advisory fires.
	// Asserts: exit 0 (soft advisory, non-block) + record_type "nudge" present.
	// -------------------------------------------------------------------------
	t.Run("NUDGE_pnpm_add_chalk", func(t *testing.T) {
		homeDir, auditPath, catalogsDir, _ := newHome(t)
		// Empty catalog — chalk is not malicious; catalog match does not fire.
		seedCatalog(t, catalogsDir, nil)

		stdinJSON := `{"agent_name":"e2e-agent","tool_name":"Bash","tool_input":{"command":"pnpm add chalk"}}`
		exitCode, rec := runCase(t, homeDir, auditPath, stdinJSON, "nudge")

		// Soft advisory: must not block (exit 0).
		if exitCode != 0 {
			t.Errorf("NUDGE: exit code = %d, want 0 (soft pnpm advisory must not block)", exitCode)
		}
		if rec.RecordType != "nudge" {
			t.Errorf("NUDGE: no record_type=nudge audit record found; nudge wiring must emit §9 record for install commands (BTEST-03)")
		}
		// Nudge decision must be non-block in soft mode.
		if rec.Decision == "block" {
			t.Errorf("NUDGE: audit decision = %q, want non-block (soft advisory)", rec.Decision)
		}
	})

	// -------------------------------------------------------------------------
	// Case 4: NUDGE-bun — bun add chalk (skip when bun absent).
	// bun is not installed on this dev box; the case skips gracefully.
	// -------------------------------------------------------------------------
	t.Run("NUDGE_bun_add_chalk", func(t *testing.T) {
		if _, err := exec.LookPath("bun"); err != nil {
			t.Skip("bun not installed — skipping bun NUDGE E2E case")
		}

		homeDir, auditPath, catalogsDir, _ := newHome(t)
		seedCatalog(t, catalogsDir, nil)

		stdinJSON := `{"agent_name":"e2e-agent","tool_name":"Bash","tool_input":{"command":"bun add chalk"}}`
		exitCode, rec := runCase(t, homeDir, auditPath, stdinJSON, "nudge")

		if exitCode != 0 {
			t.Errorf("NUDGE-bun: exit code = %d, want 0 (soft bun advisory must not block)", exitCode)
		}
		if rec.RecordType != "nudge" {
			t.Errorf("NUDGE-bun: no record_type=nudge audit record found")
		}
		if rec.Decision == "block" {
			t.Errorf("NUDGE-bun: audit decision = %q, want non-block", rec.Decision)
		}
	})
}

// e2eAuditRecord is a minimal view of the NDJSON audit record for E2E assertions.
// Only the fields we assert on are decoded; extra fields are ignored.
type e2eAuditRecord struct {
	RecordType string `json:"record_type"`
	Decision   string `json:"decision"`
	NudgeAction string `json:"nudge_action,omitempty"`
	ReasonCode  string `json:"reason_code,omitempty"`
}

// readE2EAuditRecord reads all NDJSON lines from auditPath and returns the last
// record whose record_type matches wantType. Returns a zero e2eAuditRecord
// (RecordType=="") when no matching record is found or the file does not exist.
func readE2EAuditRecord(t *testing.T, auditPath, wantType string) e2eAuditRecord {
	t.Helper()
	f, err := os.Open(auditPath)
	if err != nil {
		// Audit file may not exist if the binary exited before writing.
		t.Logf("readE2EAuditRecord: open %s: %v", auditPath, err)
		return e2eAuditRecord{}
	}
	defer f.Close()

	var matched e2eAuditRecord
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		var rec e2eAuditRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Logf("readE2EAuditRecord: parse line %q: %v", line, err)
			continue
		}
		if rec.RecordType == wantType {
			matched = rec
		}
	}
	if err := sc.Err(); err != nil {
		t.Logf("readE2EAuditRecord: scan %s: %v", auditPath, err)
	}
	return matched
}
