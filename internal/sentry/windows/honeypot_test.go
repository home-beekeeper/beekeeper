//go:build windows

package windows

import (
	"net"
	"testing"
	"time"

	"github.com/home-beekeeper/beekeeper/internal/sentry"
)

// TestHoneypotExfilFusion is the Windows Sentry honeypot E2E (PTEST-05).
//
// It proves that the exfil-signature-fusion rule (SENTRY-005) fires on a
// synthetic Windows scenario: an editor-descended process reads a native
// backslash credential path and then makes an outbound connection while a
// recently installed extension is present in the inventory.
//
// The credentials path is a STRING LITERAL only — no real file is created,
// no real network connection is made. The outbound IP is the RFC 5737
// documentation range (203.0.113.0/24) and is never dialled.
//
// This test also exercises the Task-1 isSensitivePath filepath.ToSlash fix
// end-to-end: the backslash form C:\Users\FAKEUSER\.aws\credentials would
// silently miss without the normalisation, causing SENTRY-005 never to fire.
func TestHoneypotExfilFusion(t *testing.T) {
	now := time.Now().UTC()

	// Step 1: Build a Windows process tree.
	//   cursor.exe (PID 100, PPID 0) — editor root.
	//   cmd.exe    (PID 500, PPID 100) — editor-descended child.
	tree := map[uint32]sentry.ProcessNode{
		100: {
			PID:    100,
			PPID:   0,
			Exe:    `C:\Program Files\cursor\cursor.exe`,
			SeenAt: now,
		},
		500: {
			PID:    500,
			PPID:   100,
			Exe:    `C:\Windows\System32\cmd.exe`,
			SeenAt: now,
		},
	}

	state := sentry.NewRuleState()

	// Step 2: Feed an EventFileAccess event.
	//
	// FilePath is a native Windows backslash credential path — a STRING LITERAL.
	// No file is created on disk. The isSensitivePath fix (filepath.ToSlash)
	// makes this path match ".aws/" in defaultSensitivePaths.
	fileEv := sentry.SentryEvent{
		Kind:     sentry.EventFileAccess,
		PID:      500,
		PPID:     100,
		Exe:      `C:\Windows\System32\cmd.exe`,
		FilePath: `C:\Users\FAKEUSER\.aws\credentials`,
		WallTime: now,
	}
	// This call populates state.CredAccessByPID[500] via evalSENTRY001 internals.
	sentry.EvaluateEvent(fileEv, state, tree, sentry.InventorySnapshot{}, sentry.RuleConfig{}, sentry.BaselineState{}, now)

	// Step 3: Build an InventorySnapshot with a recently installed extension
	// (2 minutes ago — well within the 5-minute ExfilFusionWindowMin default).
	inv := sentry.InventorySnapshot{
		RecentExtensions: map[string]time.Time{
			"evil-ext-id": now.Add(-2 * time.Minute),
		},
	}

	// Step 4: Feed an EventNetworkConnect event.
	//
	// DstAddr is 203.0.113.1, a TEST-NET-3 address from RFC 5737 — never a real
	// host and never dialled. This is the trigger event for SENTRY-005.
	netEv := sentry.SentryEvent{
		Kind:    sentry.EventNetworkConnect,
		PID:     500,
		PPID:    100,
		Exe:     `C:\Windows\System32\cmd.exe`,
		DstAddr: net.ParseIP("203.0.113.1"),
		DstPort: 443,
		WallTime: now,
	}
	alerts := sentry.EvaluateEvent(netEv, state, tree, inv, sentry.RuleConfig{}, sentry.BaselineState{}, now)

	// Step 5: Assert SENTRY-005 fired.
	found := false
	for _, a := range alerts {
		if a.RuleID == "SENTRY-005" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("SENTRY-005 exfil-fusion did not fire on Windows honeypot scenario; alerts returned: %v", alerts)
	}
}
