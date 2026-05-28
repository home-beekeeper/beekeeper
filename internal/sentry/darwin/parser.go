//go:build darwin

package darwin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/mzansi-agentive/beekeeper/internal/sentry"
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

type esloggerCreateEvent struct {
	Destination struct {
		ExistingFile struct {
			Path string `json:"path"`
		} `json:"existing_file"`
	} `json:"destination"`
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
			FilePath: createEv.Destination.ExistingFile.Path,
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
