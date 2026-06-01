//go:build windows

package windows

import (
	"errors"
	"testing"
	"time"

	"github.com/bantuson/beekeeper/internal/sentry"
)

func makeEventSummary(providerGUID string, eventID uint16, pid uint32, data map[string]interface{}) etwEventSummary {
	return etwEventSummary{
		ProviderGUID: providerGUID,
		EventID:      eventID,
		PID:          pid,
		EventData:    data,
		WallTime:     time.Now(),
	}
}

func TestParseProcessStartEvent(t *testing.T) {
	e := makeEventSummary("{22FB2CD6-0E7B-422B-A0C7-2FAD1FD0E716}", 1, 1234, map[string]interface{}{
		"ImageName":       `C:\Windows\System32\cmd.exe`,
		"CommandLine":     "cmd /c whoami",
		"ParentProcessID": uint32(500),
	})
	ev, err := parseETWEventSummary(e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Kind != sentry.EventProcessCreate {
		t.Errorf("Kind = %v, want EventProcessCreate", ev.Kind)
	}
	if ev.PID != 1234 {
		t.Errorf("PID = %d, want 1234", ev.PID)
	}
	if ev.PPID != 500 {
		t.Errorf("PPID = %d, want 500", ev.PPID)
	}
	if ev.Exe != `C:\Windows\System32\cmd.exe` {
		t.Errorf("Exe = %q", ev.Exe)
	}
	if ev.Cmdline != "cmd /c whoami" {
		t.Errorf("Cmdline = %q", ev.Cmdline)
	}
}

func TestParseProcessStopEventReturnsErr(t *testing.T) {
	e := makeEventSummary("{22FB2CD6-0E7B-422B-A0C7-2FAD1FD0E716}", 2, 1234, nil)
	_, err := parseETWEventSummary(e)
	if !errors.Is(err, ErrUnknownEvent) {
		t.Errorf("expected ErrUnknownEvent, got %v", err)
	}
}

func TestParseFileCreateEvent(t *testing.T) {
	e := makeEventSummary("{EDD08927-9CC4-4E65-B970-C2560FB5C289}", 12, 555, map[string]interface{}{
		"FileName": `C:\Users\me\.ssh\id_rsa`,
	})
	ev, err := parseETWEventSummary(e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Kind != sentry.EventFileAccess {
		t.Errorf("Kind = %v, want EventFileAccess", ev.Kind)
	}
	if ev.PID != 555 {
		t.Errorf("PID = %d, want 555", ev.PID)
	}
	if ev.FilePath != `C:\Users\me\.ssh\id_rsa` {
		t.Errorf("FilePath = %q", ev.FilePath)
	}
}

func TestParseNetworkConnectEvent(t *testing.T) {
	e := makeEventSummary("{7DD42A49-5329-4832-8DFD-43D979153A88}", 12, 777, map[string]interface{}{
		"daddr": "52.14.222.1",
		"dport": uint16(443),
	})
	ev, err := parseETWEventSummary(e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Kind != sentry.EventNetworkConnect {
		t.Errorf("Kind = %v, want EventNetworkConnect", ev.Kind)
	}
	if ev.DstAddr == nil || ev.DstAddr.String() != "52.14.222.1" {
		t.Errorf("DstAddr = %v, want 52.14.222.1", ev.DstAddr)
	}
	if ev.DstPort != 443 {
		t.Errorf("DstPort = %d, want 443", ev.DstPort)
	}
}

func TestParseUnknownProviderReturnsErr(t *testing.T) {
	e := makeEventSummary("{DEADBEEF-0000-0000-0000-000000000000}", 1, 1, nil)
	_, err := parseETWEventSummary(e)
	if !errors.Is(err, ErrUnknownEvent) {
		t.Errorf("expected ErrUnknownEvent, got %v", err)
	}
}

func TestParseSystemIdlePIDDropped(t *testing.T) {
	e := makeEventSummary("{22FB2CD6-0E7B-422B-A0C7-2FAD1FD0E716}", 1, 0, map[string]interface{}{
		"ImageName": "System Idle Process",
	})
	_, err := parseETWEventSummary(e)
	if !errors.Is(err, ErrUnknownEvent) {
		t.Errorf("expected ErrUnknownEvent for PID 0, got %v", err)
	}
}

func TestNormalizeGUID(t *testing.T) {
	if normalizeGUID("{ABC-DEF}") != "abc-def" {
		t.Errorf("normalizeGUID({ABC-DEF}) = %q, want abc-def", normalizeGUID("{ABC-DEF}"))
	}
}
