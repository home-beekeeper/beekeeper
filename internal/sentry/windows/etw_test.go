//go:build windows

package windows

import (
	"strings"
	"sync/atomic"
	"testing"
)

func TestProviderGUIDsKernelProcess(t *testing.T) {
	if ProviderGUIDs["Microsoft-Windows-Kernel-Process"] != "{22FB2CD6-0E7B-422B-A0C7-2FAD1FD0E716}" {
		t.Errorf("got %q", ProviderGUIDs["Microsoft-Windows-Kernel-Process"])
	}
}

func TestProviderGUIDsKernelFile(t *testing.T) {
	if ProviderGUIDs["Microsoft-Windows-Kernel-File"] != "{EDD08927-9CC4-4E65-B970-C2560FB5C289}" {
		t.Errorf("got %q", ProviderGUIDs["Microsoft-Windows-Kernel-File"])
	}
}

func TestProviderGUIDsKernelNetwork(t *testing.T) {
	if ProviderGUIDs["Microsoft-Windows-Kernel-Network"] != "{7DD42A49-5329-4832-8DFD-43D979153A88}" {
		t.Errorf("got %q", ProviderGUIDs["Microsoft-Windows-Kernel-Network"])
	}
}

func TestProviderGUIDsSecurityAuditing(t *testing.T) {
	if ProviderGUIDs["Microsoft-Windows-Security-Auditing"] != "{54849625-5478-4994-A5BA-3E3B0328C30D}" {
		t.Errorf("got %q", ProviderGUIDs["Microsoft-Windows-Security-Auditing"])
	}
}

func TestDefaultKernelProvidersIncludesProcess(t *testing.T) {
	providers := DefaultKernelProviders()
	found := false
	for _, p := range providers {
		if strings.Contains(strings.ToUpper(p), "22FB2CD6") {
			found = true
		}
	}
	if !found {
		t.Errorf("DefaultKernelProviders missing Kernel-Process GUID, got: %v", providers)
	}
}

func TestEventsLostIsAtomicCounter(t *testing.T) {
	before := atomic.LoadUint64(&EventsLost)
	atomic.AddUint64(&EventsLost, 1)
	after := atomic.LoadUint64(&EventsLost)
	if after != before+1 {
		t.Errorf("expected %d, got %d", before+1, after)
	}
	// Restore original value so parallel tests are not affected.
	atomic.StoreUint64(&EventsLost, before)
}

func TestSessionNameIsBeekeeperSentry(t *testing.T) {
	if SessionName != "BeekeeperSentry" {
		t.Errorf("SessionName = %q, want BeekeeperSentry", SessionName)
	}
}
