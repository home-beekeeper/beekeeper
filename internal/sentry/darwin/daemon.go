//go:build darwin

package darwin

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/home-beekeeper/beekeeper/internal/audit"
	"github.com/home-beekeeper/beekeeper/internal/config"
	"github.com/home-beekeeper/beekeeper/internal/ipc"
	"github.com/home-beekeeper/beekeeper/internal/platform"
	"github.com/home-beekeeper/beekeeper/internal/sentry"
)

// daemonState holds shared state protected by mu.
type daemonState struct {
	mu              sync.RWMutex
	ruleStates      map[string]bool // ruleID -> enabled
	startedAt       time.Time
	eventsProcessed uint64 // accessed atomically
}

// RunDaemon is the macOS Sentry daemon entry point invoked by "beekeeper sentry".
// It starts eslogger, drains events, runs the IPC server, loads the baseline,
// and runs the correlation engine loop until ctx is cancelled.
func RunDaemon(ctx context.Context, cfg *config.Config, auditPath string) error {
	state := &daemonState{
		ruleStates: map[string]bool{
			"SENTRY-001": true,
			"SENTRY-002": true,
			"SENTRY-003": true,
			"SENTRY-004": true,
			"SENTRY-005": true,
		},
		startedAt: time.Now().UTC(),
	}

	// 1. Open audit writer.
	auditWriter, err := audit.NewWriter(auditPath)
	if err != nil {
		return fmt.Errorf("audit writer: %w", err)
	}
	defer auditWriter.Close() //nolint:errcheck

	// 2. Start eslogger subprocess.
	cmd := EsloggerCommand(ctx, DefaultEsloggerEvents)
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("eslogger stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start eslogger: %w", err)
	}

	// 3. Drain eslogger stdout into the events channel.
	events := make(chan sentry.SentryEvent, 10000)
	drainDone := make(chan error, 1)
	go func() { drainDone <- drainEslogger(stdout, events) }()

	// 4. IPC server.
	stateDir, err := platform.StateDir()
	if err != nil {
		return fmt.Errorf("state dir: %w", err)
	}
	sockPath := filepath.Join(stateDir, "sentry.sock")
	ipcSrv, err := ipc.NewServer(sockPath, uint32(os.Getuid()))
	if err != nil {
		return fmt.Errorf("ipc server: %w", err)
	}
	defer ipcSrv.Close()

	// 5. Build live extension inventory (TM-RS-01): seed from watch directories
	// at startup, refresh every 30 s so SENTRY-004/005 can fire in production.
	invStore := sentry.NewInventoryStore()
	watchDirs := sentry.ExpandedWatchDirs(cfg.WatchDirectories())
	invStore.ScanDirs(watchDirs, time.Now().UTC())
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				invStore.ScanDirs(watchDirs, time.Now().UTC())
			}
		}
	}()

	// 6. Load baseline and start correlation engine goroutine.
	baselinePath := filepath.Join(stateDir, "sentry-baseline.json")
	go correlationEngineLoop(ctx, events, auditWriter, baselinePath, state, invStore)

	// 7. IPC server goroutine.
	go func() {
		_ = ipcSrv.Serve(ctx, func(conn net.Conn) {
			handleIPCConn(conn, state, stateDir)
		})
	}()

	// 8. Block until context is cancelled or eslogger exits.
	select {
	case <-ctx.Done():
		return nil
	case err := <-drainDone:
		if err != nil {
			return fmt.Errorf("eslogger drain: %w", err)
		}
		return nil
	}
}

// handleIPCConn decodes a single IPCCommand from conn, dispatches it, and
// encodes the IPCResponse back. Called per-connection by the IPC server.
// stateDir is the platform state directory used to locate sentry-baseline.json.
func handleIPCConn(conn net.Conn, state *daemonState, stateDir string) {
	var cmd ipc.IPCCommand
	if err := ipc.Decode(conn, &cmd); err != nil {
		return
	}

	var resp ipc.IPCResponse
	switch cmd.Kind {
	case ipc.CmdStatusRequest:
		state.mu.RLock()
		rulesActive := 0
		for _, enabled := range state.ruleStates {
			if enabled {
				rulesActive++
			}
		}
		uptime := time.Since(state.startedAt).Truncate(time.Second).String()
		ep := atomic.LoadUint64(&state.eventsProcessed)
		dropped := atomic.LoadUint64(&EventsDropped)
		state.mu.RUnlock()

		// TM-RS-04: read the canonical stateDir path, not a sockPath-derived path.
		// Windows handleIPCConn already uses this pattern correctly.
		baselinePath := filepath.Join(stateDir, "sentry-baseline.json")
		baseline, _ := sentry.LoadBaseline(baselinePath)
		now := time.Now().UTC()
		permanent := baseline.DurationDays < 0
		daysLeft := 0
		if sentry.IsBaselineActive(baseline, now) && !permanent {
			// Only compute remaining days for finite-duration baselines (DurationDays > 0).
			// For permanent baselines (DurationDays < 0) the duration arithmetic is
			// nonsensical — set daysLeft=0 and surface BaselinePermanent=true (TM-RS-03).
			remaining := time.Until(baseline.StartedAt.Add(
				time.Duration(baseline.DurationDays) * 24 * time.Hour,
			))
			daysLeft = int(remaining.Hours() / 24)
		}

		sr := ipc.StatusResponse{
			DaemonPID:         os.Getpid(),
			Uptime:            uptime,
			Tier:              0,
			TierReason:        "macOS eslogger (no entitlement)",
			RulesActive:       rulesActive,
			EventsProcessed:   ep,
			EventsDropped:     dropped,
			BaselineActive:    sentry.IsBaselineActive(baseline, now),
			BaselineDaysLeft:  daysLeft,
			BaselinePermanent: permanent,
			SockPath:          filepath.Join(stateDir, "sentry.sock"),
		}
		payload, _ := json.Marshal(sr)
		resp = ipc.IPCResponse{Kind: "status_response", Payload: payload}

	case ipc.CmdRulesListRequest:
		state.mu.RLock()
		rules := make([]ipc.RuleInfo, 0, len(state.ruleStates))
		for id, enabled := range state.ruleStates {
			rules = append(rules, ipc.RuleInfo{ID: id, Name: id, Enabled: enabled, Severity: "critical"})
		}
		state.mu.RUnlock()
		payload, _ := json.Marshal(ipc.RulesListResponse{Rules: rules})
		resp = ipc.IPCResponse{Kind: "rules_list_response", Payload: payload}

	case ipc.CmdRulesEnableRequest:
		state.mu.Lock()
		state.ruleStates[cmd.RuleID] = true
		state.mu.Unlock()
		resp = ipc.IPCResponse{Kind: "ok"}

	case ipc.CmdRulesDisableRequest:
		state.mu.Lock()
		state.ruleStates[cmd.RuleID] = false
		state.mu.Unlock()
		resp = ipc.IPCResponse{Kind: "ok"}

	default:
		resp = ipc.IPCResponse{Error: "unknown command"}
	}

	_ = ipc.Encode(conn, resp)
}

// correlationEngineLoop receives SentryEvent values from the events channel,
// applies EvaluateEvent, and writes resulting SentryAlerts to auditWriter.
//
// invStore is the live extension inventory maintained by the daemon; its
// Snapshot() is taken on each event so SENTRY-004/005 can fire in production
// (TM-RS-01 fix — previously an empty InventorySnapshot{} was always passed).
func correlationEngineLoop(
	ctx context.Context,
	events <-chan sentry.SentryEvent,
	auditWriter *audit.Writer,
	baselinePath string,
	state *daemonState,
	invStore *sentry.InventoryStore,
) {
	baseline, _ := sentry.LoadBaseline(baselinePath)
	ruleState := sentry.NewRuleState()
	tree := make(map[uint32]sentry.ProcessNode)
	const gcCutoff = 10 * time.Minute

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			// Update local tree on process create events.
			if ev.Kind == sentry.EventProcessCreate {
				tree[ev.PID] = sentry.ProcessNode{
					PID:     ev.PID,
					PPID:    ev.PPID,
					UID:     ev.UID,
					Exe:     ev.Exe,
					Cmdline: ev.Cmdline,
					SeenAt:  ev.WallTime,
				}
				// GC stale entries.
				now := time.Now()
				for pid, node := range tree {
					if now.Sub(node.SeenAt) > gcCutoff {
						delete(tree, pid)
					}
				}
			}

			atomic.AddUint64(&state.eventsProcessed, 1)

			now := time.Now().UTC()
			alerts := sentry.EvaluateEvent(
				ev, ruleState, tree,
				invStore.Snapshot(now),
				sentry.RuleConfig{},
				baseline,
				now,
			)
			for _, alert := range alerts {
				rec := alertToAuditRecord(alert)
				// Finding #5 (HIGH): Sentry daemons are the ONLY writers that
				// populate SentryFilesAccessed / SentryNetworkDests /
				// SentryProcessExe / SentryCorrelatedExt — exactly the fields
				// audit.RedactRecord targets (TM-D-03). A credential in a watched
				// file path or a Bearer/JWT/AKIA token in a network-destination
				// URL must be redacted before it is persisted verbatim. Route the
				// record through the same chokepoint every other audit producer
				// uses (check/handler.go, watch/handler.go, gateway/proxy.go).
				rec = audit.RedactRecord(rec, audit.DefaultRedactPatterns())
				_ = auditWriter.Write(rec)
			}
		}
	}
}

// alertToAuditRecord converts a SentryAlert to an AuditRecord ready for the
// NDJSON audit log (sentry_alert or sentry_alert_baseline record_type).
// This mirrors linux/daemon.go alertToAuditRecord field-for-field: the audit
// schema is cross-platform invariant.
func alertToAuditRecord(alert sentry.SentryAlert) audit.AuditRecord {
	recordType := "sentry_alert"
	decision := "block"
	switch {
	case alert.Severity == "info":
		// SENTRY-009 install observation (IPST-06): DETECTION ONLY. Sentry never
		// blocks/warns/quarantines an observed install, it records THAT one
		// happened. Distinct record_type + "observe" decision so the audit log and
		// the TUI never mistake it for a prevention.
		recordType = "sentry_install_observed"
		decision = "observe"
	case alert.BaselineMode:
		recordType = "sentry_alert_baseline"
		decision = "warn"
	case !alert.QuarantineRec:
		decision = "warn"
	}

	return audit.AuditRecord{
		RecordType:          recordType,
		RecordID:            fmt.Sprintf("sentry-%d", alert.Timestamp.UnixNano()),
		Timestamp:           alert.Timestamp.Format(time.RFC3339),
		ScannerName:         "beekeeper",
		Decision:            decision,
		Reason:              fmt.Sprintf("%s: %s", alert.RuleID, alert.RuleName),
		RuleIDs:             []string{alert.RuleID},
		CatalogMatches:      []audit.CatalogProvenance{},
		SourcesAgreed:       []string{},
		SourcesDissented:    []string{},
		SentryRuleID:        alert.RuleID,
		SentryRuleName:      alert.RuleName,
		SentrySeverity:      alert.Severity,
		SentryBaselineMode:  alert.BaselineMode,
		SentryProcessPID:    alert.ProcessPID,
		SentryProcessExe:    alert.ProcessExe,
		SentryParentChain:   alert.ParentChain,
		SentryFilesAccessed: alert.FilesAccessed,
		SentryNetworkDests:  alert.NetworkDests,
		SentryCorrelatedExt: alert.CorrelatedExtension,
		SentryQuarantineRec: alert.QuarantineRec,
	}
}
