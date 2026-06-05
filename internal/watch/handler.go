package watch

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/bantuson/beekeeper/internal/audit"
	"github.com/bantuson/beekeeper/internal/catalog"
	"github.com/bantuson/beekeeper/internal/notify"
	"github.com/bantuson/beekeeper/internal/policy"
	"github.com/bantuson/beekeeper/internal/policyloader"
	"github.com/bantuson/beekeeper/internal/quarantine"
)

// Handler evaluates newly-detected extension directories against the Beekeeper
// catalog and release-age policy, quarantining and alerting on hits.
type Handler struct {
	IndexPath     string
	CacheDir      string
	QuarantineDir string
	AuditPath     string
	NotifyConfig  notify.Config
	SocketToken   string
	HTTPClient    *http.Client
	Now           func() time.Time
	WatchedRoots  []string
}

// NewHandler constructs a Handler with all required fields.
func NewHandler(
	indexPath, cacheDir, quarantineDir, auditPath string,
	notifyCfg notify.Config,
	socketToken string,
	httpClient *http.Client,
	now func() time.Time,
	watchedRoots []string,
) *Handler {
	return &Handler{
		IndexPath:     indexPath,
		CacheDir:      cacheDir,
		QuarantineDir: quarantineDir,
		AuditPath:     auditPath,
		NotifyConfig:  notifyCfg,
		SocketToken:   socketToken,
		HTTPClient:    httpClient,
		Now:           now,
		WatchedRoots:  watchedRoots,
	}
}

// HandleNewExtension is called by the Watch loop when a new directory appears
// under a watched root. It parses the extension manifest, evaluates the
// extension against the catalog and release-age policy, and quarantines
// malicious ones.
func (h *Handler) HandleNewExtension(ctx context.Context, path string) {
	// 1. Symlink escape guard: the parent of the clean path must be one of the
	//    watched roots. This prevents a symlink pointing outside the watched tree
	//    from being evaluated and quarantined.
	parent := filepath.Dir(filepath.Clean(path))
	inRoot := false
	for _, root := range h.WatchedRoots {
		if parent == filepath.Clean(root) {
			inRoot = true
			break
		}
	}
	if !inRoot {
		return
	}

	// 2. Parse the extension manifest.
	m, err := ParseManifest(path)
	if err != nil {
		// ErrNoManifest means this is not an extension directory — silent skip.
		return
	}

	publisher := strings.ToLower(m.Publisher)
	name := strings.ToLower(m.Name)
	version := m.Version
	displayName := m.DisplayName
	pkg := publisher + "." + name

	// 3. Network context capped at 3 s.
	netCtx, netCancel := context.WithTimeout(ctx, 3*time.Second)
	defer netCancel()

	// 4. Open the mmap index.
	bbIdx, err := catalog.OpenIndex(h.IndexPath)
	if err != nil {
		log.Printf("beekeeper watch: catalog index unavailable: %v", err)
		return
	}
	defer bbIdx.Close()

	// 5. Build catalog adapters.
	httpClient := h.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 4 * time.Second}
	}
	var osvAdapter policy.MultiCatalogLookup = &catalog.OSVAdapter{
		Client:   httpClient,
		CacheDir: h.CacheDir,
		Ctx:      netCtx,
	}
	var socketAdapter policy.MultiCatalogLookup
	if h.SocketToken != "" {
		socketAdapter = catalog.SocketAdapter{
			Client:   httpClient,
			CacheDir: h.CacheDir,
			Token:    h.SocketToken,
			Ctx:      netCtx,
		}
	}
	multiIdx := catalog.NewMultiIndex(bbIdx, osvAdapter, socketAdapter)

	// 6. Load policy overlay files and derive corroboration thresholds (INT-WARN-1).
	// Mirrors handler.go pattern: missing dir = no-op; malformed file = skip.
	var policyFiles []policyloader.PolicyFile
	if h.CacheDir != "" {
		policiesDir := filepath.Join(filepath.Dir(h.CacheDir), "policies")
		policyFiles, _ = policyloader.LoadPolicyDir(policiesDir)
		// Errors are ignored (non-fatal for watch): missing/unreadable dir = no overlay.
	}
	thresholds := policyloader.ThresholdsFromPolicyFiles(policyFiles)
	// CORR-02: thread catalog sanity state into thresholds.
	thresholds.CatalogHealthy = resolveCatalogHealthy(h.CacheDir)

	// 7. Catalog evaluation using policy-file-derived thresholds.
	tc := policy.ToolCall{
		ToolName: "watch",
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

	// 8. Release-age evaluation.
	now := h.Now()
	ageMinutes, missing, _ := catalog.FetchMarketplaceAge(netCtx, httpClient, h.CacheDir, publisher, name, version, now)
	ageDecision := policy.EvaluateReleaseAge(policy.ReleaseAgeInput{
		Ecosystem:        "editor-extension",
		Package:          pkg,
		AgeMinutes:       ageMinutes,
		TimestampMissing: missing,
	}, policy.DefaultReleaseAgeConfig())

	// 9. Decision merge: hit if either evaluation blocks.
	hit := !catalogDecision.Allow || !ageDecision.Allow

	// Determine the effective reason and decision for the audit record.
	effectiveDecision := catalogDecision
	if !ageDecision.Allow && catalogDecision.Allow {
		// Age policy drove the block.
		effectiveDecision = ageDecision
	}

	if hit {
		reason := effectiveDecision.Reason

		// Build the audit record.
		rec := audit.FromDecision(tc, effectiveDecision, generateRecordID(), time.Now().UTC().Format(time.RFC3339), policy.AgentContext{})
		rec.RecordType = "sentry_alert"
		// Ensure EDXT-03 is in rule IDs.
		if !containsRuleID(rec.RuleIDs, "EDXT-03") {
			rec.RuleIDs = append([]string{"EDXT-03"}, rec.RuleIDs...)
		}

		// Write audit record — redact before fan-out to disk/remote sinks.
		// Mirrors check/handler.go:493-494 and gateway/proxy.go:514-515 (TM-D-03).
		redactedRec := audit.RedactRecord(rec, audit.DefaultRedactPatterns())
		if w, err := audit.NewWriter(h.AuditPath); err != nil {
			log.Printf("beekeeper watch: audit writer unavailable: %v", err)
		} else {
			if err := w.Write(redactedRec); err != nil {
				log.Printf("beekeeper watch: audit write failed: %v", err)
			}
			w.Close()
		}

		// Best-effort desktop notification.
		notify.Notify(h.NotifyConfig, "Beekeeper: extension quarantined",
			fmt.Sprintf("%s@%s — %s", pkg, version, reason))

		// Map catalog matches for the quarantine manifest.
		var mappedMatches []quarantine.CatalogMatchSummary
		for _, cm := range effectiveDecision.CatalogMatches {
			mappedMatches = append(mappedMatches, quarantine.CatalogMatchSummary{
				CatalogSource: cm.CatalogSource,
				EntryID:       cm.EntryID,
				Severity:      cm.Severity,
			})
		}

		// Quarantine the extension.
		if _, err := quarantine.Move(h.QuarantineDir, path, quarantine.Manifest{
			Publisher:      m.Publisher,
			Name:           m.Name,
			Version:        version,
			DisplayName:    displayName,
			Reason:         reason,
			RuleIDs:        []string{"EDXT-03"},
			AuditRecordID:  rec.RecordID,
			CatalogMatches: mappedMatches,
		}); err != nil {
			log.Printf("beekeeper watch: quarantine failed for %s: %v", pkg, err)
		}
		return
	}

	// 10. Clean — write an allow audit record — redact before fan-out (TM-D-03).
	allowDecision := policy.Decision{
		Allow:   true,
		Level:   "allow",
		Reason:  "no catalog match",
		RuleIDs: []string{"EDXT-02"},
	}
	rec := audit.FromDecision(tc, allowDecision, generateRecordID(), time.Now().UTC().Format(time.RFC3339), policy.AgentContext{})
	redactedAllow := audit.RedactRecord(rec, audit.DefaultRedactPatterns())
	if w, err := audit.NewWriter(h.AuditPath); err != nil {
		log.Printf("beekeeper watch: audit writer unavailable: %v", err)
	} else {
		if err := w.Write(redactedAllow); err != nil {
			log.Printf("beekeeper watch: audit write failed: %v", err)
		}
		w.Close()
	}
}

// generateRecordID returns a random 8-byte hex record ID. On RNG failure it
// falls back to a timestamp-derived ID so auditing is never silently skipped.
func generateRecordID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("ts-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// containsRuleID reports whether ruleIDs contains id.
func containsRuleID(ruleIDs []string, id string) bool {
	for _, r := range ruleIDs {
		if r == id {
			return true
		}
	}
	return false
}
