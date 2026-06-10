//go:build windows

package windows

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	etw "github.com/tekert/golang-etw/etw"

	"github.com/bantuson/beekeeper/internal/sentry"
)

var (
	guidKernelProcess = "22fb2cd6-0e7b-422b-a0c7-2fad1fd0e716"
	guidKernelFile    = "edd08927-9cc4-4e65-b970-c2560fb5c289"
	guidKernelNetwork = "7dd42a49-5329-4832-8dfd-43d979153a88"
)

// ErrUnknownEvent is returned by parseETWEvent when the provider or event ID
// is not handled by Beekeeper Sentry.
var ErrUnknownEvent = errors.New("etw: unknown provider or event id")

// etwEventSummary is a plain-struct view of an etw.Event used for testability.
// The adapter parseETWEvent maps the library type to this struct; tests use
// parseETWEventSummary directly so they do not depend on unexported library fields.
type etwEventSummary struct {
	ProviderGUID string
	EventID      uint16
	PID          uint32
	EventData    map[string]interface{}
	WallTime     time.Time
}

// normalizeGUID strips braces and lowercases a GUID string so comparisons are
// format-independent (e.g., "{22FB2CD6-...}" == "22fb2cd6-...").
func normalizeGUID(g string) string {
	g = strings.TrimPrefix(g, "{")
	g = strings.TrimSuffix(g, "}")
	return strings.ToLower(g)
}

func toUint32(v interface{}) uint32 {
	switch x := v.(type) {
	case uint32:
		return x
	case uint64:
		return uint32(x)
	case float64:
		return uint32(x)
	case int:
		return uint32(x)
	case int64:
		return uint32(x)
	}
	return 0
}

func toUint16(v interface{}) uint16 {
	switch x := v.(type) {
	case uint16:
		return x
	case uint32:
		return uint16(x)
	case uint64:
		return uint16(x)
	case float64:
		return uint16(x)
	case int:
		return uint16(x)
	}
	return 0
}

// parseETWEvent adapts an *etw.Event from the library to parseETWEventSummary.
// The GUID field on the library Event is type etw.GUID; we call .String() which
// returns "{lowercase-guid}" format.
func parseETWEvent(e *etw.Event) (sentry.SentryEvent, error) {
	if e == nil {
		return sentry.SentryEvent{}, ErrUnknownEvent
	}
	return parseETWEventSummary(etwEventSummary{
		ProviderGUID: e.System.Provider.Guid.String(),
		EventID:      e.System.EventID,
		PID:          e.System.Execution.ProcessID,
		EventData:    e.EventData,
		WallTime:     e.System.TimeCreated.SystemTime,
	})
}

// parseETWEventSummary normalises a Windows ETW event into a platform-agnostic
// sentry.SentryEvent. It handles three kernel provider GUIDs:
//
//   - Microsoft-Windows-Kernel-Process (event ID 1 = process start)
//   - Microsoft-Windows-Kernel-File    (12=Create/open + 15=Read -> file access;
//     16=Write/30=CreateNewFile/27=RenamePath/19=Rename -> file write;
//     14=Close is ignored. Phase 20 SENT-08 corrected the prior mislabel that
//     lumped 12/14/15 together as "create/name".)
//   - Microsoft-Windows-Kernel-Network (event IDs 10, 11, 12 = TCP connect)
func parseETWEventSummary(e etwEventSummary) (sentry.SentryEvent, error) {
	if e.PID == 0 {
		return sentry.SentryEvent{}, fmt.Errorf("%w: system idle process (PID 0)", ErrUnknownEvent)
	}

	wallTime := e.WallTime
	if wallTime.IsZero() {
		wallTime = time.Now().UTC()
	}

	guid := normalizeGUID(e.ProviderGUID)

	switch guid {
	case guidKernelProcess:
		if e.EventID != 1 {
			return sentry.SentryEvent{}, fmt.Errorf("%w: kernel-process event %d", ErrUnknownEvent, e.EventID)
		}
		imageName, _ := e.EventData["ImageName"].(string)
		if imageName == "" {
			imageName, _ = e.EventData["ImagePath"].(string)
		}
		if imageName == "" {
			imageName, _ = e.EventData["ImageFileName"].(string)
		}
		cmdline, _ := e.EventData["CommandLine"].(string)
		ppid := toUint32(e.EventData["ParentProcessID"])
		return sentry.SentryEvent{
			Kind:     sentry.EventProcessCreate,
			PID:      e.PID,
			PPID:     ppid,
			Exe:      imageName,
			Cmdline:  cmdline,
			WallTime: wallTime,
		}, nil

	case guidKernelFile:
		// Phase 20 (SENT-08): split the Kernel-File branch by the CORRECT event
		// IDs. The write/rename EventData path key is read with the same
		// FileName -> FilePath fallback as reads; the exact key for write/rename
		// templates is flagged for live golang-etw verification (see SUMMARY /
		// CLAUDE.md Phase-7 ETW field-name research flag).
		filePath, _ := e.EventData["FileName"].(string)
		if filePath == "" {
			filePath, _ = e.EventData["FilePath"].(string)
		}
		switch e.EventID {
		case 12, 15:
			// 12 = Create (open handle), 15 = Read — the process touched the file.
			return sentry.SentryEvent{
				Kind:     sentry.EventFileAccess,
				PID:      e.PID,
				FilePath: filePath,
				WallTime: wallTime,
			}, nil
		case 16, 30, 27, 19:
			// 16 = Write, 30 = CreateNewFile, 27 = RenamePath, 19 = Rename.
			return sentry.SentryEvent{
				Kind:     sentry.EventFileWrite,
				PID:      e.PID,
				FilePath: filePath,
				WallTime: wallTime,
			}, nil
		default:
			// 14 = Close and all other Kernel-File IDs are not alertable events.
			return sentry.SentryEvent{}, fmt.Errorf("%w: kernel-file event %d", ErrUnknownEvent, e.EventID)
		}

	case guidKernelNetwork:
		if e.EventID != 10 && e.EventID != 11 && e.EventID != 12 {
			return sentry.SentryEvent{}, fmt.Errorf("%w: kernel-network event %d", ErrUnknownEvent, e.EventID)
		}
		daddrStr, _ := e.EventData["daddr"].(string)
		if daddrStr == "" {
			daddrStr, _ = e.EventData["DestinationAddress"].(string)
		}
		portRaw := e.EventData["dport"]
		if portRaw == nil {
			portRaw = e.EventData["DestinationPort"]
		}
		return sentry.SentryEvent{
			Kind:     sentry.EventNetworkConnect,
			PID:      e.PID,
			DstAddr:  net.ParseIP(daddrStr),
			DstPort:  toUint16(portRaw),
			WallTime: wallTime,
		}, nil

	default:
		return sentry.SentryEvent{}, fmt.Errorf("%w: provider %s", ErrUnknownEvent, guid)
	}
}
