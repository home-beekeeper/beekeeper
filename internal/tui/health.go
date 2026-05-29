package tui

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	gateway "github.com/mzansi-agentive/beekeeper/internal/gateway"
	ipc "github.com/mzansi-agentive/beekeeper/internal/ipc"
	platform "github.com/mzansi-agentive/beekeeper/internal/platform"
)

const healthProbeTimeout = 200 * time.Millisecond

// refreshHealthState computes the current HealthState by probing each component.
// All probes degrade gracefully — errors return false/degraded state, never panic.
func refreshHealthState(stateDir string) HealthState {
	return HealthState{
		HooksOK:    probeHooks(),
		GatewayOK:  probeGateway(stateDir),
		SentryOK:   probeSentry(stateDir),
		CatalogsOK: probeCatalogs(),
		LastBlock:  probeLastBlock(),
	}
}

// probeHooks checks whether ~/.claude/settings.json has a beekeeper hook installed.
func probeHooks() bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return false
	}
	// Look for beekeeper reference in settings
	return strings.Contains(string(data), "beekeeper")
}

// probeGateway checks gateway state.json + HTTP health endpoint.
func probeGateway(stateDir string) bool {
	statePath := filepath.Join(stateDir, "state.json")
	gw, err := gateway.LoadGatewayState(statePath)
	if err != nil || gw.BoundPort == 0 {
		return false
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/health", gw.BoundPort)
	client := &http.Client{Timeout: healthProbeTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// probeSentry dials the IPC socket with a short timeout.
func probeSentry(stateDir string) bool {
	sockPath := filepath.Join(stateDir, "sentry.sock")
	conn, err := ipc.Connect(sockPath, healthProbeTimeout)
	if err != nil {
		return false
	}
	defer conn.Close()
	if err := ipc.SendCommand(conn, ipc.IPCCommand{Kind: ipc.CmdStatusRequest}, healthProbeTimeout); err != nil {
		return false
	}
	resp, err := ipc.ReadResponse(conn, healthProbeTimeout)
	return err == nil && resp.Error == ""
}

// probeCatalogs checks bumblebee.idx mtime < 25 hours.
func probeCatalogs() bool {
	catalogDir, err := platform.CatalogDir()
	if err != nil {
		return false
	}
	idxPath := filepath.Join(catalogDir, "bumblebee.idx")
	info, err := os.Stat(idxPath)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) < 25*time.Hour
}

// probeLastBlock reads the most recent block decision from the audit log tail.
// Returns a human-readable string like "last block 6m ago" or "no blocks yet".
func probeLastBlock() string {
	auditDir, err := platform.AuditDir()
	if err != nil {
		return "last block unknown"
	}
	auditPath := filepath.Join(auditDir, "beekeeper.ndjson")
	recs, _ := tailFrom(auditPath, 0)
	// Find most recent block decision
	var lastBlockTime time.Time
	for _, rec := range recs {
		if rec.Decision == "block" {
			t, err := time.Parse(time.RFC3339, rec.Timestamp)
			if err == nil && t.After(lastBlockTime) {
				lastBlockTime = t
			}
		}
	}
	if lastBlockTime.IsZero() {
		return "no blocks yet"
	}
	age := time.Since(lastBlockTime)
	switch {
	case age < time.Minute:
		return "last block just now"
	case age < time.Hour:
		return fmt.Sprintf("last block %dm ago", int(age.Minutes()))
	default:
		return fmt.Sprintf("last block %dh ago", int(age.Hours()))
	}
}
