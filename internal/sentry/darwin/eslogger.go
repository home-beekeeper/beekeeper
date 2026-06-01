//go:build darwin

package darwin

import (
	"bufio"
	"context"
	"io"
	"os/exec"
	"sync/atomic"

	"github.com/bantuson/beekeeper/internal/sentry"
)

// EventsDropped is incremented atomically when the event channel is full.
var EventsDropped uint64

// DefaultEsloggerEvents is the eslogger subscription list.
// Both "open" and "create" are required to detect credential reads.
var DefaultEsloggerEvents = []string{"exec", "open", "create", "network_flow", "fork"}

// EsloggerCommand returns an *exec.Cmd for eslogger with the given event types.
// The caller must attach stdout/stderr pipes before calling cmd.Start().
func EsloggerCommand(ctx context.Context, events []string) *exec.Cmd {
	args := make([]string, len(events))
	copy(args, events)
	return exec.CommandContext(ctx, "eslogger", args...)
}

// drainEslogger reads NDJSON lines from r, parses each line into a SentryEvent,
// and sends it to out. The send is non-blocking: if out is full, EventsDropped
// is incremented and the event is discarded.
func drainEslogger(r io.Reader, out chan<- sentry.SentryEvent) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		ev, err := parseEsloggerLine(line)
		if err != nil {
			continue
		}
		select {
		case out <- ev:
		default:
			atomic.AddUint64(&EventsDropped, 1)
		}
	}
	return scanner.Err()
}
