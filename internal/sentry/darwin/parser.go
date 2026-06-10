//go:build darwin

package darwin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"path"
	"strings"
	"time"

	"github.com/bantuson/beekeeper/internal/sentry"
)

var ParseError = errors.New("eslogger: parse error")

type esloggerLine struct {
	EventType string                     `json:"event_type"`
	Time      string                     `json:"time"`
	Process   esloggerProcess            `json:"process"`
	Event     map[string]json.RawMessage `json:"event"`
}

type esloggerProcess struct {
	AuditToken esloggerAuditToken `json:"audit_token"`
	PPID       uint32             `json:"ppid"`
	Executable esloggerExecutable `json:"executable"`
}

type esloggerAuditToken struct {
	PID uint32 `json:"pid"`
	UID uint32 `json:"uid"`
}

type esloggerExecutable struct {
	Path string `json:"path"`
}

type esloggerExecEvent struct {
	Target struct {
		AuditToken esloggerAuditToken `json:"audit_token"`
		PPID       uint32             `json:"ppid"`
		Executable esloggerExecutable `json:"executable"`
		Args       []string           `json:"args"`
	} `json:"target"`
}

type esloggerOpenEvent struct {
	File struct {
		Path string `json:"path"`
	} `json:"file"`
}

// esloggerDestination models the ES destination union used by both create and
// rename events. A NEW file's path is carried in new_path.dir.path + filename,
// NOT existing_file.path — the original parser read only existing_file and so
// DROPPED every new-file create (Phase 20 SENT-07 bug). path() returns the union.
type esloggerDestination struct {
	ExistingFile struct {
		Path string `json:"path"`
	} `json:"existing_file"`
	NewPath struct {
		Dir struct {
			Path string `json:"path"`
		} `json:"dir"`
		Filename string `json:"filename"`
	} `json:"new_path"`
}

// path returns the destination path, preferring existing_file (modify/overwrite)
// and falling back to the new_path.dir+filename union (new-file create).
func (d esloggerDestination) path() string {
	if d.ExistingFile.Path != "" {
		return d.ExistingFile.Path
	}
	if d.NewPath.Dir.Path != "" || d.NewPath.Filename != "" {
		return path.Join(d.NewPath.Dir.Path, d.NewPath.Filename)
	}
	return ""
}

type esloggerCreateEvent struct {
	Destination esloggerDestination `json:"destination"`
}

// esloggerWriteEvent models es_event_write_t (target file written in place).
type esloggerWriteEvent struct {
	Target struct {
		Path string `json:"path"`
	} `json:"target"`
}

// esloggerRenameEvent models es_event_rename_t; the destination is the same
// union as create (editors write-temp-then-rename, so this is the common
// persistence-write path).
type esloggerRenameEvent struct {
	Destination esloggerDestination `json:"destination"`
}

type esloggerNetworkFlowEvent struct {
	Remote struct {
		Address string `json:"address"`
		Port    uint16 `json:"port"`
	} `json:"remote"`
}

func parseEsloggerLine(data []byte) (sentry.SentryEvent, error) {
	var line esloggerLine
	if err := json.Unmarshal(data, &line); err != nil {
		return sentry.SentryEvent{}, fmt.Errorf("%w: %v", ParseError, err)
	}

	wallTime := time.Now().UTC()
	if line.Time != "" {
		if t, err := time.Parse(time.RFC3339Nano, line.Time); err == nil {
			wallTime = t
		}
	}

	switch line.EventType {
	case "exec":
		raw, ok := line.Event["exec"]
		if !ok {
			return sentry.SentryEvent{}, fmt.Errorf("%w: exec event missing exec key", ParseError)
		}
		var execEv esloggerExecEvent
		if err := json.Unmarshal(raw, &execEv); err != nil {
			return sentry.SentryEvent{}, fmt.Errorf("%w: %v", ParseError, err)
		}
		return sentry.SentryEvent{
			Kind:     sentry.EventProcessCreate,
			PID:      execEv.Target.AuditToken.PID,
			PPID:     execEv.Target.PPID,
			UID:      execEv.Target.AuditToken.UID,
			Exe:      execEv.Target.Executable.Path,
			Cmdline:  strings.Join(execEv.Target.Args, " "),
			WallTime: wallTime,
		}, nil

	case "open":
		raw, ok := line.Event["open"]
		if !ok {
			return sentry.SentryEvent{}, fmt.Errorf("%w: open event missing open key", ParseError)
		}
		var openEv esloggerOpenEvent
		if err := json.Unmarshal(raw, &openEv); err != nil {
			return sentry.SentryEvent{}, fmt.Errorf("%w: %v", ParseError, err)
		}
		return sentry.SentryEvent{
			Kind:     sentry.EventFileAccess,
			PID:      line.Process.AuditToken.PID,
			PPID:     line.Process.PPID,
			UID:      line.Process.AuditToken.UID,
			Exe:      line.Process.Executable.Path,
			FilePath: openEv.File.Path,
			WallTime: wallTime,
		}, nil

	case "create":
		raw, ok := line.Event["create"]
		if !ok {
			return sentry.SentryEvent{}, fmt.Errorf("%w: create event missing create key", ParseError)
		}
		var createEv esloggerCreateEvent
		if err := json.Unmarshal(raw, &createEv); err != nil {
			return sentry.SentryEvent{}, fmt.Errorf("%w: %v", ParseError, err)
		}
		return sentry.SentryEvent{
			Kind:     sentry.EventFileAccess,
			PID:      line.Process.AuditToken.PID,
			PPID:     line.Process.PPID,
			UID:      line.Process.AuditToken.UID,
			Exe:      line.Process.Executable.Path,
			FilePath: createEv.Destination.path(), // union fix: new-file creates no longer dropped
			WallTime: wallTime,
		}, nil

	case "write":
		raw, ok := line.Event["write"]
		if !ok {
			return sentry.SentryEvent{}, fmt.Errorf("%w: write event missing write key", ParseError)
		}
		var writeEv esloggerWriteEvent
		if err := json.Unmarshal(raw, &writeEv); err != nil {
			return sentry.SentryEvent{}, fmt.Errorf("%w: %v", ParseError, err)
		}
		return sentry.SentryEvent{
			Kind:     sentry.EventFileWrite,
			PID:      line.Process.AuditToken.PID,
			PPID:     line.Process.PPID,
			UID:      line.Process.AuditToken.UID,
			Exe:      line.Process.Executable.Path,
			FilePath: writeEv.Target.Path,
			WallTime: wallTime,
		}, nil

	case "rename":
		raw, ok := line.Event["rename"]
		if !ok {
			return sentry.SentryEvent{}, fmt.Errorf("%w: rename event missing rename key", ParseError)
		}
		var renameEv esloggerRenameEvent
		if err := json.Unmarshal(raw, &renameEv); err != nil {
			return sentry.SentryEvent{}, fmt.Errorf("%w: %v", ParseError, err)
		}
		return sentry.SentryEvent{
			Kind:     sentry.EventFileWrite,
			PID:      line.Process.AuditToken.PID,
			PPID:     line.Process.PPID,
			UID:      line.Process.AuditToken.UID,
			Exe:      line.Process.Executable.Path,
			FilePath: renameEv.Destination.path(), // rename -> destination path
			WallTime: wallTime,
		}, nil

	case "network_flow":
		raw, ok := line.Event["network_flow"]
		if !ok {
			return sentry.SentryEvent{}, fmt.Errorf("%w: network_flow event missing network_flow key", ParseError)
		}
		var netEv esloggerNetworkFlowEvent
		if err := json.Unmarshal(raw, &netEv); err != nil {
			return sentry.SentryEvent{}, fmt.Errorf("%w: %v", ParseError, err)
		}
		return sentry.SentryEvent{
			Kind:     sentry.EventNetworkConnect,
			PID:      line.Process.AuditToken.PID,
			PPID:     line.Process.PPID,
			UID:      line.Process.AuditToken.UID,
			Exe:      line.Process.Executable.Path,
			DstAddr:  net.ParseIP(netEv.Remote.Address),
			DstPort:  netEv.Remote.Port,
			WallTime: wallTime,
		}, nil

	default:
		return sentry.SentryEvent{}, fmt.Errorf("%w: unknown event_type %q", ParseError, line.EventType)
	}
}
