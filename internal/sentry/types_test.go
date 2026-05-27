package sentry

import (
	"encoding/json"
	"net"
	"testing"
	"time"
)

// TestSentryEventRoundTrip verifies that a fully-populated SentryEvent survives
// a JSON marshal → unmarshal round-trip with all fields intact.
func TestSentryEventRoundTrip(t *testing.T) {
	original := SentryEvent{
		Kind:     EventNetworkConnect,
		PID:      1234,
		PPID:     1000,
		UID:      501,
		Exe:      "/usr/bin/curl",
		Cmdline:  "curl https://example.com",
		FilePath: "/home/user/.ssh/id_rsa",
		DstAddr:  net.ParseIP("93.184.216.34"),
		DstPort:  443,
		KTimeNS:  9876543210,
		WallTime: time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got SentryEvent
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Kind != original.Kind {
		t.Errorf("Kind: got %d, want %d", got.Kind, original.Kind)
	}
	if got.PID != original.PID {
		t.Errorf("PID: got %d, want %d", got.PID, original.PID)
	}
	if got.PPID != original.PPID {
		t.Errorf("PPID: got %d, want %d", got.PPID, original.PPID)
	}
	if got.UID != original.UID {
		t.Errorf("UID: got %d, want %d", got.UID, original.UID)
	}
	if got.Exe != original.Exe {
		t.Errorf("Exe: got %q, want %q", got.Exe, original.Exe)
	}
	if got.Cmdline != original.Cmdline {
		t.Errorf("Cmdline: got %q, want %q", got.Cmdline, original.Cmdline)
	}
	if got.FilePath != original.FilePath {
		t.Errorf("FilePath: got %q, want %q", got.FilePath, original.FilePath)
	}
	if !got.DstAddr.Equal(original.DstAddr) {
		t.Errorf("DstAddr: got %v, want %v", got.DstAddr, original.DstAddr)
	}
	if got.DstPort != original.DstPort {
		t.Errorf("DstPort: got %d, want %d", got.DstPort, original.DstPort)
	}
	if got.KTimeNS != original.KTimeNS {
		t.Errorf("KTimeNS: got %d, want %d", got.KTimeNS, original.KTimeNS)
	}
	if !got.WallTime.Equal(original.WallTime) {
		t.Errorf("WallTime: got %v, want %v", got.WallTime, original.WallTime)
	}
}

// TestRuleStateInit verifies that NewRuleState returns a struct with all
// maps initialised (non-nil), preventing nil map assignment panics.
func TestRuleStateInit(t *testing.T) {
	rs := NewRuleState()
	if rs.CredAccessByPID == nil {
		t.Error("CredAccessByPID is nil")
	}
	if rs.CredCLIByPID == nil {
		t.Error("CredCLIByPID is nil")
	}
	if rs.PhoneHomeByPID == nil {
		t.Error("PhoneHomeByPID is nil")
	}

	// Exercise map writes to confirm they don't panic.
	rs.CredAccessByPID[42] = append(rs.CredAccessByPID[42], RuleWindowEntry{PID: 42, Value: "v"})
	rs.CredCLIByPID[42] = append(rs.CredCLIByPID[42], RuleWindowEntry{PID: 42, Value: "v"})
	rs.PhoneHomeByPID[42] = append(rs.PhoneHomeByPID[42], RuleWindowEntry{PID: 42, Value: "v"})
}

// TestInventorySnapshot verifies that InventorySnapshot can be constructed and
// queried without panics.
func TestInventorySnapshot(t *testing.T) {
	now := time.Now().UTC()
	inv := InventorySnapshot{
		RecentExtensions: map[string]time.Time{
			"ext-foo": now.Add(-5 * time.Minute),
			"ext-bar": now.Add(-2 * time.Hour),
		},
	}

	if len(inv.RecentExtensions) != 2 {
		t.Errorf("expected 2 extensions, got %d", len(inv.RecentExtensions))
	}
	if _, ok := inv.RecentExtensions["ext-foo"]; !ok {
		t.Error("ext-foo missing from inventory")
	}
}
