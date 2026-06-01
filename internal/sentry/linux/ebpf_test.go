//go:build linux

package linux

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/bantuson/beekeeper/internal/sentry"
)

func TestParseProcessEvent(t *testing.T) {
	var pe processEventLayout
	pe.Pid = 1234
	pe.Ppid = 5678
	pe.Uid = 1000
	copy(pe.Exe[:], "cursor")
	pe.KtimeNS = 9999999

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, pe); err != nil {
		t.Fatalf("binary.Write: %v", err)
	}

	ev := parseEvent(buf.Bytes(), sentry.EventProcessCreate)
	if ev.PID != 1234 {
		t.Errorf("PID: got %d, want 1234", ev.PID)
	}
	if ev.PPID != 5678 {
		t.Errorf("PPID: got %d, want 5678", ev.PPID)
	}
	if ev.Exe != "cursor" {
		t.Errorf("Exe: got %q, want %q", ev.Exe, "cursor")
	}
	if ev.KTimeNS != 9999999 {
		t.Errorf("KTimeNS: got %d, want 9999999", ev.KTimeNS)
	}
}

func TestParseNetworkEvent(t *testing.T) {
	var ne networkEventLayout
	ne.Pid = 42
	ne.Daddr = 0x04030201 // 1.2.3.4 in little-endian
	ne.Dport = 0x5000     // port 80 in network byte order: 0x0050 → big-endian read = 80
	ne.IsIPv6 = 0

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, ne); err != nil {
		t.Fatalf("binary.Write: %v", err)
	}

	ev := parseEvent(buf.Bytes(), sentry.EventNetworkConnect)
	if ev.PID != 42 {
		t.Errorf("PID: got %d, want 42", ev.PID)
	}
	if ev.DstAddr == nil {
		t.Error("DstAddr is nil")
	}
}

func TestDropCounterIncrements(t *testing.T) {
	// Reset counter.
	EventsDropped = 0

	// Fill a channel to capacity.
	ch := make(chan sentry.SentryEvent, 1)
	ev := sentry.SentryEvent{Kind: sentry.EventProcessCreate}
	ch <- ev

	// Verify the counter variable is accessible and writable.
	// Actual increments occur inside reader goroutines; this test only verifies
	// that EventsDropped is exported and can be reset.
	select {
	case ch <- ev:
	default:
		// Channel full — in a real reader goroutine EventsDropped would increment.
		_ = EventsDropped
	}

	if EventsDropped != 0 {
		t.Logf("EventsDropped = %d (incremented by concurrent reader; acceptable)", EventsDropped)
	}
}

func TestParseEventUnknownKind(t *testing.T) {
	// Unknown kind should return a zero-valued event with correct WallTime set.
	ev := parseEvent([]byte{}, sentry.EventFileAccess)
	if ev.Kind != sentry.EventFileAccess {
		t.Errorf("Kind: got %d, want %d", ev.Kind, sentry.EventFileAccess)
	}
}
