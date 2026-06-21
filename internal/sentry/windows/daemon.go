//go:build windows

package windows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	etw "github.com/tekert/golang-etw/etw"
	"golang.org/x/sys/windows/svc"

	"github.com/home-beekeeper/beekeeper/internal/audit"
	"github.com/home-beekeeper/beekeeper/internal/config"
	"github.com/home-beekeeper/beekeeper/internal/ipc"
	"github.com/home-beekeeper/beekeeper/internal/platform"
	"github.com/home-beekeeper/beekeeper/internal/sentry"
)

// daemonState holds shared state protected by mu.
// eventsProcessed is accessed atomically (no lock needed for that field alone).
type daemonState struct {
	mu              sync.RWMutex
	ruleStates      map[string]bool // ruleID -> enabled
	startedAt       time.Time
	eventsProcessed uint64 // accessed atomically
	tierReason      string // mutated when ETW fallback is engaged
}

// windowsService implements svc.Handler for the Windows Service Control Manager.
type windowsService struct {
	ctx       context.Context
	cfg       *config.Config
	auditPath string
}

// Execute is called by the Windows SCM when the service starts.
// It sends status updates and delegates to runDaemonBody.
func (ws *windowsService) Execute(args []string, r <-chan svc.ChangeRequest, status chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	status <- svc.Status{State: svc.StartPending}

	innerCtx, cancel := context.WithCancel(ws.ctx)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- runDaemonBody(innerCtx, ws.cfg, ws.auditPath) }()

	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for {
		select {
		case req := <-r:
			switch req.Cmd {
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending}
				cancel()
				<-errCh // wait for runDaemonBody to clean up
				return false, 0
			default:
				// Pause/Continue not supported; ignore.
			}
		case err := <-errCh:
			// Daemon body exited unexpectedly.
			if err != nil && !errors.Is(err, context.Canceled) {
				fmt.Fprintf(os.Stderr, "beekeeper sentry: daemon exited: %v\n", err)
				return true, 1
			}
			return false, 0
		}
	}
}

// RunDaemon is the Windows Sentry daemon entry point invoked by "beekeeper sentry".
// It detects whether the process is running as a Windows Service; if so it
// dispatches via svc.Run, otherwise it runs the daemon body inline (foreground/dev mode).
func RunDaemon(ctx context.Context, cfg *config.Config, auditPath string) error {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return fmt.Errorf("svc.IsWindowsService: %w", err)
	}

	if isService {
		return svc.Run(ServiceName, &windowsService{ctx: ctx, cfg: cfg, auditPath: auditPath})
	}
	return runDaemonBody(ctx, cfg, auditPath)
}

// runDaemonBody initialises and runs the full Sentry daemon loop:
//  1. Opens audit writer
//  2. Creates ETW session and enables providers (with ACCESS_DENIED fallback)
//  3. Starts the ETW consumer goroutine
//  4. Opens the IPC named pipe server
//  5. Starts the correlation engine goroutine
//  6. Blocks until ctx is cancelled or consumer exits
func runDaemonBody(ctx context.Context, cfg *config.Config, auditPath string) error {
	state := &daemonState{
		ruleStates: map[string]bool{
			"SENTRY-001": true,
			"SENTRY-002": true,
			"SENTRY-003": true,
			"SENTRY-004": true,
			"SENTRY-005": true,
		},
		startedAt:  time.Now().UTC(),
		tierReason: "Windows ETW (LocalService)",
	}

	// 1. Open audit writer.
	auditWriter, err := audit.NewWriter(auditPath)
	if err != nil {
		return fmt.Errorf("open audit writer: %w", err)
	}
	defer auditWriter.Close() //nolint:errcheck

	// 2. Create ETW session and enable providers.
	//    ETW sessions must be started before the consumer can attach by name.
	sess := etw.NewRealTimeSession(SessionName)
	if startErr := sess.Start(); startErr != nil {
		return fmt.Errorf("etw session start: %w", startErr)
	}
	defer sess.Stop() //nolint:errcheck

	// Enable providers with ACCESS_DENIED fallback (RESEARCH Pitfall 4).
	// Microsoft-Windows-Security-Auditing requires the SeSecurityPrivilege
	// which LocalService does not hold. The daemon gracefully degrades to
	// Kernel-File + Kernel-Network when Security-Auditing is denied.
	requested := []struct {
		name string
		guid string
	}{
		{"Microsoft-Windows-Kernel-Process", ProviderGUIDs["Microsoft-Windows-Kernel-Process"]},
		{"Microsoft-Windows-Security-Auditing", ProviderGUIDs["Microsoft-Windows-Security-Auditing"]},
		{"Microsoft-Windows-Kernel-File", ProviderGUIDs["Microsoft-Windows-Kernel-File"]},
		{"Microsoft-Windows-Kernel-Network", ProviderGUIDs["Microsoft-Windows-Kernel-Network"]},
		// SENT-11 (OPTIONAL): DNS-Client is a manifest provider. If golang-etw
		// cannot enable it on this session, the same access-denied-continue
		// fallback below leaves DNS ingestion absent without failing the daemon.
		{"Microsoft-Windows-DNS-Client", ProviderGUIDs["Microsoft-Windows-DNS-Client"]},
	}

	var enabledCount int
	for _, p := range requested {
		guid := etw.MustParseGUID(p.guid)
		prov := etw.Provider{
			GUID:        *guid,
			EnableLevel: 0xFF, // capture all levels
		}
		if provErr := sess.EnableProvider(prov); provErr != nil {
			if errors.Is(provErr, etw.ERROR_ACCESS_DENIED) {
				state.mu.Lock()
				state.tierReason = fmt.Sprintf("Windows ETW (LocalService): provider %s not accessible (access denied) — using fallback set", p.name)
				state.mu.Unlock()
				fmt.Fprintf(os.Stderr, "beekeeper sentry: ETW EnableProvider %s: access denied; continuing without this provider\n", p.name)
				continue
			}
			return fmt.Errorf("etw EnableProvider %s: %w", p.name, provErr)
		}
		enabledCount++
	}

	if enabledCount == 0 {
		return fmt.Errorf("etw: no providers could be enabled — service privileges insufficient")
	}

	// 3. Start ETW consumer goroutine.
	events := make(chan sentry.SentryEvent, 10000)
	consumerDone := make(chan error, 1)
	go func() { consumerDone <- StartETWConsumer(ctx, SessionName, events) }()

	// 4. IPC named pipe server.
	stateDir, err := platform.StateDir()
	if err != nil {
		return fmt.Errorf("state dir: %w", err)
	}
	sockPath := filepath.Join(stateDir, "sentry.sock") // logically unused on Windows; passed for API parity
	ipcSrv, err := ipc.NewServer(sockPath, 0)
	if err != nil {
		return fmt.Errorf("ipc server: %w", err)
	}
	defer ipcSrv.Close()

	go func() {
		_ = ipcSrv.Serve(ctx, func(conn net.Conn) {
			handleIPCConn(conn, state)
		})
	}()

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

	// 6. Start correlation engine goroutine.
	baselinePath := filepath.Join(stateDir, "sentry-baseline.json")
	go correlationEngineLoop(ctx, events, auditWriter, baselinePath, state, invStore)

	// 6. Block until ctx cancelled or consumer exits.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-consumerDone:
		if err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("etw consumer: %w", err)
		}
		return nil
	}
}

// handleIPCConn decodes a single IPCCommand from conn, dispatches it, and
// encodes the IPCResponse back. Called per-connection by the IPC server.
// This mirrors darwin/daemon.go handleIPCConn exactly, with Windows-specific
// TierReason (ETW fallback degradation) and EventsDropped (EventsLost counter).
func handleIPCConn(conn net.Conn, state *daemonState) {
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
		tierReason := state.tierReason
		state.mu.RUnlock()

		// EventsLost is the Windows-specific counter from etw.go.
		dropped := atomic.LoadUint64(&EventsLost)

		stateDir, _ := platform.StateDir()
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
			TierReason:        tierReason,
			RulesActive:       rulesActive,
			EventsProcessed:   ep,
			EventsDropped:     dropped,
			BaselineActive:    sentry.IsBaselineActive(baseline, now),
			BaselineDaysLeft:  daysLeft,
			BaselinePermanent: permanent,
			SockPath:          ipc.PipePath,
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
// Copied VERBATIM from darwin/daemon.go (which mirrors linux/daemon.go
// field-for-field): the audit schema is cross-platform invariant.
func alertToAuditRecord(alert sentry.SentryAlert) audit.AuditRecord {
	recordType := "sentry_alert"
	decision := "block"
	switch {
	case alert.Severity == "info":
		// SENTRY-009 install observation (IPST-06): DETECTION ONLY. Sentry never
		// blocks/warns/quarantines an observed install — it records THAT one
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
