// Package scan implements the Beekeeper scan orchestrator (EDXT-04).
// It invokes the Pollen CLI when present, reads its NDJSON stdout, and merges
// it with Beekeeper-own per-extension catalog/release-age findings into one NDJSON
// stream. When Pollen is unavailable, a scan_status record is emitted and the
// Beekeeper-own scan continues uninterrupted.
package scan

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bantuson/beekeeper/internal/audit"
	"github.com/bantuson/beekeeper/internal/catalog"
	"github.com/bantuson/beekeeper/internal/policy"
	"github.com/bantuson/beekeeper/internal/policyloader"
	"github.com/bantuson/beekeeper/internal/watch"
)

// Config holds parameters for a Scan call.
type Config struct {
	Deep          bool
	ExtensionDirs []string
	IndexPath     string
	CacheDir      string
	AuditPath     string
	SocketToken   string
	HTTPClient    *http.Client
	Now           func() time.Time
}

// FindingRecord is one beekeeper-own extension-scan result emitted to the NDJSON stream.
type FindingRecord struct {
	RecordType  string   `json:"record_type"`
	ScannerName string   `json:"scanner_name"`
	Publisher   string   `json:"publisher,omitempty"`
	Name        string   `json:"name,omitempty"`
	Version     string   `json:"version,omitempty"`
	DisplayName string   `json:"display_name,omitempty"`
	Decision    string   `json:"decision"`
	Reason      string   `json:"reason"`
	RuleIDs     []string `json:"rule_ids"`
}

// Package-level injectable vars for tests.
var lookPollenFn = func() (string, error) { return exec.LookPath("pollen") }

// runPollenFn is replaced in tests to yield canned lines without spawning a real process.
// Returns (channel, true) when pollen is available, (nil, false) otherwise.
var runPollenFn = func(ctx context.Context, deep bool) (<-chan []byte, bool) {
	return defaultRunPollen(ctx, deep)
}

// defaultRunPollen invokes pollen and streams its stdout NDJSON lines over the
// returned channel. Returns (nil, false) if pollen is not in PATH or fails to start.
// NOTE: no --format flag is passed — NDJSON is pollen's default output format.
func defaultRunPollen(ctx context.Context, deep bool) (<-chan []byte, bool) {
	bin, err := lookPollenFn()
	if err != nil {
		return nil, false
	}
	args := []string{"scan"}
	if deep {
		args = append(args, "--profile", "deep")
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, false
	}
	if err := cmd.Start(); err != nil {
		return nil, false
	}
	ch := make(chan []byte, 64)
	go func() {
		defer close(ch)
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			line := sc.Bytes()
			out := make([]byte, len(line))
			copy(out, line)
			ch <- out
		}
		_ = cmd.Wait()
	}()
	return ch, true
}

// Scan orchestrates the Pollen CLI (when available) and the Beekeeper-own
// per-extension catalog/release-age scan, writing merged NDJSON results to out.
func Scan(ctx context.Context, cfg Config, out io.Writer) error {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 4 * time.Second}
	}
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}

	ch, ok := runPollenFn(ctx, cfg.Deep)
	if ok {
		for line := range ch {
			if len(line) == 0 {
				continue
			}
			// Validate JSON — fail-closed on malformed subprocess output.
			var probe json.RawMessage
			if err := json.Unmarshal(line, &probe); err != nil {
				warn := map[string]any{
					"record_type":  "scan_error",
					"scanner_name": "beekeeper",
					"source":       "pollen",
					"error":        "malformed NDJSON from pollen subprocess",
				}
				_ = writeJSONLine(out, warn)
				continue
			}
			// Pass through unknown record_types unmodified.
			_, _ = fmt.Fprintf(out, "%s\n", line)
			if cfg.AuditPath != "" {
				_ = appendRawAuditLine(cfg.AuditPath, line)
			}
		}
	} else {
		// Pollen not in PATH or failed to start — degrade gracefully.
		status := map[string]any{
			"record_type":        "scan_status",
			"pollen_unavailable": true,
			"scanner_name":       "beekeeper",
		}
		if err := writeJSONLine(out, status); err != nil {
			return fmt.Errorf("write pollen_unavailable status: %w", err)
		}
		if cfg.AuditPath != "" {
			if b, err := json.Marshal(status); err == nil {
				_ = appendRawAuditLine(cfg.AuditPath, b)
			}
		}
	}

	return beekeeperScan(ctx, cfg, out)
}

// beekeeperScan walks cfg.ExtensionDirs, evaluates each extension via catalog
// and release-age policy, and emits a FindingRecord per extension to out.
func beekeeperScan(ctx context.Context, cfg Config, out io.Writer) error {
	if len(cfg.ExtensionDirs) == 0 {
		return nil
	}

	// Open the mmap index; if unavailable, proceed with nil (OSV/Socket only).
	var bbIdx *catalog.Index
	if cfg.IndexPath != "" {
		if idx, err := catalog.OpenIndex(cfg.IndexPath); err == nil {
			bbIdx = idx
			defer bbIdx.Close()
		} else {
			_ = writeJSONLine(out, map[string]any{
				"record_type":  "scan_error",
				"scanner_name": "beekeeper",
				"error":        fmt.Sprintf("catalog index unavailable: %v", err),
			})
		}
	}

	for _, dir := range cfg.ExtensionDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			extPath := filepath.Join(dir, entry.Name())
			manifest, err := watch.ParseManifest(extPath)
			if err != nil {
				continue // ErrNoManifest: not an extension
			}
			if err := evaluateExtension(ctx, cfg, bbIdx, manifest, extPath, out); err != nil {
				return err
			}
		}
	}
	return nil
}

// evaluateExtension runs catalog + release-age evaluation for a single extension
// and writes a FindingRecord to out and an AuditRecord if configured.
func evaluateExtension(
	ctx context.Context,
	cfg Config,
	bbIdx *catalog.Index,
	manifest watch.ExtensionManifest,
	_ string,
	out io.Writer,
) error {
	publisher := strings.ToLower(manifest.Publisher)
	name := strings.ToLower(manifest.Name)
	version := manifest.Version
	pkg := publisher + "." + name

	// Per-extension network timeout.
	netCtx, netCancel := context.WithTimeout(ctx, 3*time.Second)
	defer netCancel()

	var osvAdapter policy.MultiCatalogLookup = &catalog.OSVAdapter{
		Client:   cfg.HTTPClient,
		CacheDir: cfg.CacheDir,
		Ctx:      netCtx,
	}
	var socketAdapter policy.MultiCatalogLookup
	if cfg.SocketToken != "" {
		socketAdapter = catalog.SocketAdapter{
			Client:   cfg.HTTPClient,
			CacheDir: cfg.CacheDir,
			Token:    cfg.SocketToken,
			Ctx:      netCtx,
		}
	}
	multiIdx := catalog.NewMultiIndex(bbIdx, osvAdapter, socketAdapter)

	// Load policy overlay files and derive corroboration thresholds (INT-WARN-1).
	// Missing/unreadable policies dir = no overlay (non-fatal for scan).
	var policyFiles []policyloader.PolicyFile
	if cfg.CacheDir != "" {
		policiesDir := filepath.Join(filepath.Dir(cfg.CacheDir), "policies")
		policyFiles, _ = policyloader.LoadPolicyDir(policiesDir)
	}
	thresholds := policyloader.ThresholdsFromPolicyFiles(policyFiles)

	tc := policy.ToolCall{
		ToolName: "scan",
		ToolInput: map[string]any{
			"ecosystem": "editor-extension",
			"package":   pkg,
			"version":   version,
		},
	}
	catalogDecision := policy.Evaluate(tc, multiIdx, thresholds, policy.AgentContext{})

	// Apply policy overlay (package_allowlist / sensitive_path rules).
	if len(policyFiles) > 0 {
		catalogDecision = policyloader.ApplyPolicyOverlay(policyFiles, tc, catalogDecision)
	}

	now := cfg.Now()
	ageMinutes, missing, _ := catalog.FetchMarketplaceAge(netCtx, cfg.HTTPClient, cfg.CacheDir, publisher, name, version, now)
	ageDecision := policy.EvaluateReleaseAge(policy.ReleaseAgeInput{
		Ecosystem:        "editor-extension",
		Package:          pkg,
		AgeMinutes:       ageMinutes,
		TimestampMissing: missing,
	}, policy.DefaultReleaseAgeConfig())

	hit := !catalogDecision.Allow || !ageDecision.Allow
	effectiveDecision := catalogDecision
	if !ageDecision.Allow && catalogDecision.Allow {
		effectiveDecision = ageDecision
	}

	ruleIDs := []string{"EDXT-04"}
	decision := "allow"
	reason := "no catalog match"
	if hit {
		decision = effectiveDecision.Level
		reason = effectiveDecision.Reason
		ruleIDs = append(ruleIDs, effectiveDecision.RuleIDs...)
	}

	rec := FindingRecord{
		RecordType:  "finding",
		ScannerName: "beekeeper",
		Publisher:   manifest.Publisher,
		Name:        manifest.Name,
		Version:     version,
		DisplayName: manifest.DisplayName,
		Decision:    decision,
		Reason:      reason,
		RuleIDs:     ruleIDs,
	}
	if err := writeJSONLine(out, rec); err != nil {
		return fmt.Errorf("write finding record: %w", err)
	}

	if cfg.AuditPath != "" {
		cleanDecision := policy.Decision{
			Allow:   true,
			Level:   "allow",
			Reason:  "no catalog match",
			RuleIDs: []string{"EDXT-04"},
		}
		auditDecision := effectiveDecision
		if !hit {
			auditDecision = cleanDecision
		}
		auditRec := audit.FromDecision(tc, auditDecision, generateScanID(), time.Now().UTC().Format(time.RFC3339), policy.AgentContext{})
		if w, err := audit.NewWriter(cfg.AuditPath); err == nil {
			_ = w.Write(auditRec)
			w.Close()
		}
	}
	return nil
}

func writeJSONLine(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

func appendRawAuditLine(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}

func generateScanID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("scan-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
