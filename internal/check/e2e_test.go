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
	"bytes"
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

	"github.com/home-beekeeper/beekeeper/internal/catalog"
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
// HOOK case:   --hook claude-code credential read → exit 2 / deny stdout
//
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
		// beekeeperhomeoverride lets the shipped binary honor BEEKEEPER_HOME for
		// hermetic E2E isolation; production builds omit it so the override cannot
		// repoint the trust root at runtime (remediation 260615, finding #1).
		"-tags", "beekeeperhomeoverride",
		"-o", binPath,
		"github.com/home-beekeeper/beekeeper/cmd/beekeeper",
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
		// Set BEEKEEPER_HOME; inherit the rest of the environment.
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
	// Case 4 (install-posture, IPST/IPOVR): a git/remote-source install at the
	// agent hook.
	//
	// Deterministic with NO network: the git-remote rule is parsed entirely from
	// the command string (pkgparse classifies "git+https://..." as a git source),
	// so no registry/OSV fetch is consulted for a remote install. The seeded
	// catalog is empty -> the only signal is the posture git-remote rule.
	//
	//   4a (default config): a git install WARNS but does NOT block -> exit 0,
	//      audit decision "warn", a git/remote-source reason.
	//   4b (git-remote opted to block via config.json): the SAME git install
	//      BLOCKS -> exit 1, audit decision "block". The block is attributable to
	//      the opt-up alone (4a proves the default warns).
	// -------------------------------------------------------------------------
	const gitInstallJSON = `{"agent_name":"e2e-agent","tool_name":"Bash","tool_input":{"command":"npm install git+https://github.com/evil/pkg.git"}}`

	t.Run("posture_git_remote_default_warn", func(t *testing.T) {
		homeDir, auditPath, catalogsDir, _ := newHome(t)
		seedCatalog(t, catalogsDir, nil) // empty: posture is the only signal

		exitCode, rec := runCase(t, homeDir, auditPath, gitInstallJSON, "policy_decision")

		// A git install WARNS by default -- it does not block (exit 0).
		if exitCode != 0 {
			t.Errorf("posture default: exit code = %d, want 0 (a git install warns, does not block)", exitCode)
		}
		if rec.Decision != "warn" {
			t.Errorf("posture default: audit decision = %q, want %q (git-remote warns by default)", rec.Decision, "warn")
		}
		if !postureReasonMentionsRemote(t, auditPath) {
			t.Errorf("posture default: audit reason should mention the git/remote source")
		}
	})

	t.Run("posture_git_remote_block_mode", func(t *testing.T) {
		homeDir, auditPath, catalogsDir, _ := newHome(t)
		seedCatalog(t, catalogsDir, nil)
		// Opt the git-remote rule UP to block via the hermetic config.json that
		// ConfigPath() resolves under $homeDir/beekeeper/config.json. config key is
		// the JSON tag "remote_source" (config.PostureConfig.RemoteSource).
		writePostureBlockConfig(t, homeDir, `{"posture":{"remote_source":{"action":"block"}}}`)

		exitCode, rec := runCase(t, homeDir, auditPath, gitInstallJSON, "policy_decision")

		// The SAME install now BLOCKS (exit 1) -- attributable to the opt-up.
		if exitCode != 1 {
			t.Errorf("posture block mode: exit code = %d, want 1 (git-remote opted to block must block)", exitCode)
		}
		if rec.Decision != "block" {
			t.Errorf("posture block mode: audit decision = %q, want %q", rec.Decision, "block")
		}
		if !postureReasonMentionsRemote(t, auditPath) {
			t.Errorf("posture block mode: audit reason should name the git/remote source")
		}
	})

	// -------------------------------------------------------------------------
	// Case 5 (SENTRY-009, human-install-observed-not-blocked): the daemon-level
	// per-OS install tap that OBSERVES (never blocks) a human-run install is a
	// CI/Linux-only surface (eBPF on Linux, ETW on Windows, eslogger on macOS) and
	// is not exercisable from a single cross-platform live-binary E2E. The
	// observe-only contract -- record_type sentry_install_observed, decision
	// "observe", QuarantineRec=false, never a block -- is covered deterministically
	// at the unit level by internal/sentry/install_observe_test.go. The hook (this
	// binary) only ever sees AGENT tool calls; a human-run install never reaches
	// it, so there is nothing for the hook to (not) block. This sub-case is
	// recorded as a deliberate CI-only gap (see docs/posture-validation.md) and
	// SKIPS with a structured reason rather than asserting a daemon tap here.
	// -------------------------------------------------------------------------
	t.Run("posture_sentry009_human_install_observe_only", func(t *testing.T) {
		t.Skip("SENTRY-009 daemon install taps are CI/Linux-only (eBPF/ETW/eslogger); the observe-only, never-block contract is covered by internal/sentry/install_observe_test.go and documented as a CI-only gap in docs/posture-validation.md")
	})

	// -------------------------------------------------------------------------
	// Case 3 (VAL-05): Claude Code --hook exit-2 canary block — the documented
	// true-block reference.
	//
	// Phase 10 changed the HOOK contract to exit 2 via `--hook <harness>`; the
	// exit-1 SPATH/CORR cases above are the DEFAULT-mode block (Pitfall 2 — they
	// are NOT the hook contract). This proves the shipped `beekeeper check
	// --hook claude-code` path denies a canary credential read end-to-end:
	// exit 2 + Family-A hookSpecificOutput permissionDecision:deny on stdout +
	// audit decision "block". Both ~/.ssh and ~/.aws canaries are exercised
	// (matching the Phase-10 live proof).
	// -------------------------------------------------------------------------
	t.Run("SPATH_hook_claude_code_exit2", func(t *testing.T) {
		for _, canary := range []string{"~/.ssh/id_rsa", "~/.aws/credentials"} {
			t.Run(canary, func(t *testing.T) {
				homeDir, auditPath, catalogsDir, _ := newHome(t)
				seedCatalog(t, catalogsDir, nil)

				stdinJSON := fmt.Sprintf(`{"agent_name":"e2e-agent","tool_name":"Read","tool_input":{"file_path":%q}}`, canary)
				cmd := exec.Command(binPath, "check", "--hook", "claude-code")
				cmd.Stdin = strings.NewReader(stdinJSON)
				cmd.Env = append(os.Environ(), fmt.Sprintf("BEEKEEPER_HOME=%s", homeDir))
				var stdout, stderr bytes.Buffer
				cmd.Stdout = &stdout
				cmd.Stderr = &stderr

				exitCode := 0
				if err := cmd.Run(); err != nil {
					if exitErr, ok := err.(*exec.ExitError); ok {
						exitCode = exitErr.ProcessState.ExitCode()
					} else {
						t.Fatalf("cmd.Run non-exit error: %v", err)
					}
				}

				// The HOOK deny contract is exit 2 — NOT exit 1 (Pitfall 2).
				if exitCode != 2 {
					t.Errorf("--hook claude-code %s: exit code = %d, want 2 (hook deny contract); stderr=%s", canary, exitCode, stderr.String())
				}
				// Family-A nested hookSpecificOutput deny on stdout.
				if !strings.Contains(stdout.String(), `"permissionDecision":"deny"`) {
					t.Errorf("--hook claude-code %s: stdout = %q, want it to contain \"permissionDecision\":\"deny\"", canary, stdout.String())
				}
				// The block is still recorded in the audit log.
				rec := readE2EAuditRecord(t, auditPath, "policy_decision")
				if rec.Decision != "block" {
					t.Errorf("--hook claude-code %s: audit decision = %q, want \"block\"", canary, rec.Decision)
				}
			})
		}
	})
}

// e2eAuditRecord is a minimal view of the NDJSON audit record for E2E assertions.
// Only the fields we assert on are decoded; extra fields are ignored.
type e2eAuditRecord struct {
	RecordType string `json:"record_type"`
	Decision   string `json:"decision"`
	Reason     string `json:"reason"`
}

// writePostureBlockConfig writes a hermetic config.json under
// $homeDir/beekeeper/config.json -- the path the shipped binary resolves via
// platform.ConfigPath() when BEEKEEPER_HOME is honored (beekeeperhomeoverride
// build tag). Used to opt a posture rule UP to block for the live-binary E2E.
func writePostureBlockConfig(t *testing.T, homeDir, jsonBody string) {
	t.Helper()
	cfgPath := filepath.Join(homeDir, "beekeeper", "config.json")
	if err := os.WriteFile(cfgPath, []byte(jsonBody), 0o600); err != nil {
		t.Fatalf("write hermetic config.json: %v", err)
	}
}

// postureReasonMentionsRemote reports whether the last policy_decision record's
// reason names the git/remote source (proving the git-remote rule is what fired,
// not some unrelated decision).
func postureReasonMentionsRemote(t *testing.T, auditPath string) bool {
	t.Helper()
	rec := readE2EAuditRecord(t, auditPath, "policy_decision")
	r := strings.ToLower(rec.Reason)
	return strings.Contains(r, "git") || strings.Contains(r, "source") || strings.Contains(r, "remote")
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
