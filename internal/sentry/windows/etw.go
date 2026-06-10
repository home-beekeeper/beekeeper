//go:build windows

package windows

import (
	"context"
	"sync/atomic"

	etw "github.com/tekert/golang-etw/etw"

	"github.com/bantuson/beekeeper/internal/sentry"
)

// EventsLost is incremented atomically when the SentryEvent channel is full.
// Surfaced in 'beekeeper diag' per SWIN-04.
var EventsLost uint64

// SessionName is the ETW session name used by Beekeeper Sentry.
const SessionName = "BeekeeperSentry"

// ProviderGUIDs maps well-known Windows kernel provider names to their GUIDs.
var ProviderGUIDs = map[string]string{
	"Microsoft-Windows-Kernel-Process":    "{22FB2CD6-0E7B-422B-A0C7-2FAD1FD0E716}",
	"Microsoft-Windows-Kernel-File":       "{EDD08927-9CC4-4E65-B970-C2560FB5C289}",
	"Microsoft-Windows-Kernel-Network":    "{7DD42A49-5329-4832-8DFD-43D979153A88}",
	"Microsoft-Windows-Security-Auditing": "{54849625-5478-4994-A5BA-3E3B0328C30D}",
	// Phase 20 (SENT-11, OPTIONAL): DNS-Client is a MANIFEST provider (not a
	// kernel provider). Event ID 3006 carries the issued QueryName.
	"Microsoft-Windows-DNS-Client": "{1C95126E-7EEA-49A9-A3FE-A378B03DDB4D}",
}

// DefaultKernelProviders returns the GUIDs for the default set of kernel
// providers used by Beekeeper Sentry (process, file, network).
func DefaultKernelProviders() []string {
	return []string{
		ProviderGUIDs["Microsoft-Windows-Kernel-Process"],
		ProviderGUIDs["Microsoft-Windows-Kernel-File"],
		ProviderGUIDs["Microsoft-Windows-Kernel-Network"],
	}
}

// StartETWConsumer attaches to an existing ETW session named sessionName and
// pumps parsed SentryEvents into the events channel. Blocks until ctx is
// cancelled or an error occurs. Uses a non-blocking send with EventsLost
// tracking per SWIN-04.
func StartETWConsumer(ctx context.Context, sessionName string, events chan<- sentry.SentryEvent) error {
	c := etw.NewConsumer(ctx)
	c.FromTraceNames(sessionName)

	// Override the default callback to parse and forward events.
	c.EventCallback = func(e *etw.Event) error {
		ev, err := parseETWEvent(e)
		if err != nil {
			return nil //nolint:nilerr // unknown/uninteresting events are silently dropped
		}
		select {
		case events <- ev:
		default:
			atomic.AddUint64(&EventsLost, 1)
		}
		return nil
	}

	if err := c.Start(); err != nil {
		return err
	}

	// Block until context is cancelled, then stop the consumer.
	<-ctx.Done()
	_ = c.Stop()
	c.Wait()
	return ctx.Err()
}
