//go:build linux

package linux

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

	"github.com/bantuson/beekeeper/internal/audit"
	"github.com/bantuson/beekeeper/internal/config"
	"github.com/bantuson/beekeeper/internal/ipc"
	"github.com/bantuson/beekeeper/internal/platform"
	"github.com/bantuson/beekeeper/internal/sentry"
)

// daemonSensitivePaths mirrors sentry.defaultSensitivePaths for use in fanotify
// marking. Kept in sync manually; the canonical list lives in sentry/rules.go.
var daemonSensitivePaths = []string{
	".ssh/", ".aws/", ".gnupg/", ".config/Claude/", ".config/op/",
	".config/gh/", ".netrc", ".npmrc", ".pypirc",
	".cargo/credentials", ".env",
}

// daemonState holds shared state protected by mu.
type daemonState struct {
	mu              sync.RWMutex
	ruleStates      map[string]bool // ruleID -> enabled
	startedAt       time.Time
	tier            DegradationTier
	eventsProcessed uint64
}

// RunDaemon is the Sentry daemon entry point invoked by "beekeeper sentry".
// It probes the degradation tier, loads eBPF objects when possible, drops
// privileges, initialises fanotify, starts the IPC server, and runs the
// correlation engine loop until ctx is cancelled.
func RunDaemon(ctx context.Context, cfg *config.Config, auditPath string) error {
	// 1. Remove memlock limit (non-fatal: older kernels don't need it).
	if err := RemoveMemlock(); err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper sentry: removeMemlock: %v (continuing)\n", err)
	}

	// 2. Probe degradation tier.
	tier := ProbeTier()

	state := &daemonState{
		ruleStates: map[string]bool{
			"SENTRY-001": true,
			"SENTRY-002": true,
			"SENTRY-003": true,
			"SENTRY-004": true,
			"SENTRY-005": true,
		},
		startedAt: time.Now().UTC(),
		tier:      tier,
	}

	// 3. Load eBPF objects if the tier allows it.
	var execObjs BeekeeperExecObjects
	var netObjs BeekeeperNetObjects
	ebpfAvailable := false
	if tier <= Tier1 {
		if err := loadBeekeeperExecObjects(&execObjs, nil); err != nil {
			fmt.Fprintf(os.Stderr, "beekeeper sentry: load exec eBPF: %v (degrading to Tier2)\n", err)
			tier = Tier2
		} else if err := loadBeekeeperNetObjects(&netObjs, nil); err != nil {
			execObjs.Close() //nolint:errcheck
			fmt.Fprintf(os.Stderr, "beekeeper sentry: load net eBPF: %v (degrading to Tier2)\n", err)
			tier = Tier2
		} else {
			ebpfAvailable = true
			defer execObjs.Close() //nolint:errcheck
			defer netObjs.Close()  //nolint:errcheck
		}
	}

	// 4. Drop capabilities after eBPF load.
	if err := DropCapabilities(keepCaps(tier)); err != nil {
		return fmt.Errorf("drop capabilities: %w", err)
	}

	// 5. Apply seccomp filter.
	if err := ApplySeccomp(); err != nil {
		return fmt.Errorf("apply seccomp: %w", err)
	}

	// 6. Initialise fanotify.
	fanFd, err := InitFanotify(tier)
	if err != nil {
		return fmt.Errorf("init fanotify: %w", err)
	}

	sensitivePaths := daemonSensitivePaths
	if err := FanotifyMarkPaths(fanFd, sensitivePaths); err != nil {
		return fmt.Errorf("fanotify mark paths: %w", err)
	}

	// 7. IPC server.
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

	// 8. Audit writer.
	auditWriter, err := audit.NewWriter(auditPath)
	if err != nil {
		return fmt.Errorf("audit writer: %w", err)
	}
	defer auditWriter.Close() //nolint:errcheck

	// 9. Notify systemd.
	if err := sdNotifyReady(); err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper sentry: sd_notify: %v\n", err)
	}

	// 10. Start background goroutines.
	events := make(chan sentry.SentryEvent, 10000)
	treeCh := make(chan map[uint32]sentry.ProcessNode, 1)

	if ebpfAvailable {
		closers, err := StartEBPFReaders(ctx, &execObjs, &netObjs, tier, events)
		if err != nil {
			return fmt.Errorf("start eBPF readers: %w", err)
		}
		defer func() {
			for _, c := range closers {
				c.Close() //nolint:errcheck
			}
		}()
		go StartProcessTreeBuilder(ctx, events, treeCh)
	}

	go StartFanotifyReader(ctx, fanFd, events)

	// TM-RS-01: build live extension inventory so SENTRY-004/005 can fire in
	// production. Seed from the configured watch directories at startup, then
	// refresh every 30 s so newly-installed extensions are picked up.
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

	baselinePath := filepath.Join(stateDir, "sentry-baseline.json")
	go correlationEngineLoop(ctx, events, treeCh, auditWriter, baselinePath, state, invStore)

	go func() {
		_ = ipcSrv.Serve(ctx, func(conn net.Conn) {
			handleIPCConn(conn, state, stateDir)
		})
	}()

	<-ctx.Done()
	return nil
}

// handleIPCConn decodes a single IPCCommand from conn, dispatches it, and
// encodes the IPCResponse back. Called per-connection by the IPC server.
// stateDir is the platform state directory used to locate sentry-baseline.json
// and to report the socket path — mirrors Windows handleIPCConn (TM-RS-04).
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
		tier := state.tier
		ep := atomic.LoadUint64(&state.eventsProcessed)
		dropped := atomic.LoadUint64(&EventsDropped)
		state.mu.RUnlock()

		// TM-RS-04: read the canonical stateDir path, matching the engine's write
		// path at correlationEngineLoop (baselinePath = stateDir/sentry-baseline.json).
		// Windows handleIPCConn already uses this pattern correctly.
		baselinePath := filepath.Join(stateDir, "sentry-baseline.json")
		baseline, _ := sentry.LoadBaseline(baselinePath)
		now := time.Now().UTC()
		daysLeft := 0
		if sentry.IsBaselineActive(baseline, now) {
			remaining := time.Until(baseline.StartedAt.Add(
				time.Duration(baseline.DurationDays) * 24 * time.Hour,
			))
			daysLeft = int(remaining.Hours() / 24)
		}

		sr := ipc.StatusResponse{
			DaemonPID:        os.Getpid(),
			Uptime:           uptime,
			Tier:             int(tier),
			TierReason:       TierString(tier),
			RulesActive:      rulesActive,
			EventsProcessed:  ep,
			EventsDropped:    dropped,
			BaselineActive:   sentry.IsBaselineActive(baseline, now),
			BaselineDaysLeft: daysLeft,
			SockPath:         filepath.Join(stateDir, "sentry.sock"),
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
// It also maintains a local process-tree copy from treeCh snapshots.
//
// invStore is the live extension inventory maintained by the daemon; its
// Snapshot() is taken on each event so SENTRY-004/005 can fire in production
// (TM-RS-01 fix — previously an empty InventorySnapshot{} was always passed).
func correlationEngineLoop(
	ctx context.Context,
	events <-chan sentry.SentryEvent,
	treeCh <-chan map[uint32]sentry.ProcessNode,
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
		case newTree, ok := <-treeCh:
			if ok {
				tree = newTree
			}
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
				_ = auditWriter.Write(rec)
			}
		}
	}
}

// alertToAuditRecord converts a SentryAlert to an AuditRecord ready for the
// NDJSON audit log (sentry_alert or sentry_alert_baseline record_type).
func alertToAuditRecord(alert sentry.SentryAlert) audit.AuditRecord {
	recordType := "sentry_alert"
	decision := "block"
	if alert.BaselineMode {
		recordType = "sentry_alert_baseline"
		decision = "warn"
	} else if !alert.QuarantineRec {
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
