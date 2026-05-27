//go:build linux

package linux

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/cilium/ebpf/ringbuf"
	"golang.org/x/sys/unix"

	"github.com/mzansi-agentive/beekeeper/internal/sentry"
)

// EventsDropped is incremented atomically when the events channel is full and
// an event cannot be delivered without blocking.
var EventsDropped uint64

// processEventLayout mirrors the C struct process_event layout exactly.
// Field order and sizes must remain in sync with exec_tracer.bpf.c.
type processEventLayout struct {
	Pid     uint32
	Ppid    uint32
	Uid     uint32
	Exe     [256]byte
	Cmdline [512]byte
	KtimeNS uint64
}

// networkEventLayout mirrors the C struct network_event layout exactly.
// Field order and sizes must remain in sync with network_tracer.bpf.c.
type networkEventLayout struct {
	Pid     uint32
	Ppid    uint32
	Uid     uint32
	Saddr   uint32
	Daddr   uint32
	Dport   uint16
	Sport   uint16
	IsIPv6  uint8
	Pad     [3]byte
	Saddr6  [16]byte
	Daddr6  [16]byte
	KtimeNS uint64
}

// StartEBPFReaders attaches eBPF probes (tracepoint for exec, kprobe for
// tcp_connect) and starts goroutines that read from the associated ring/perf
// buffers. The chosen buffer type is determined by tier:
//   - Tier0 → ring buffer (kernel ≥ 5.8)
//   - Tier1 → perf event array
//
// The returned closers must be closed when the caller shuts down.
func StartEBPFReaders(ctx context.Context, execObjs *BeekeeperExecObjects, netObjs *BeekeeperNetObjects, tier DegradationTier, events chan<- sentry.SentryEvent) ([]io.Closer, error) {
	var closers []io.Closer

	tp, err := link.Tracepoint("sched", "sched_process_exec",
		execObjs.BeekeeperExecPrograms.TracepointSchedSchedProcessExec, nil)
	if err != nil {
		return nil, err
	}
	closers = append(closers, tp)

	kp, err := link.Kprobe("tcp_connect",
		netObjs.BeekeeperNetPrograms.KprobeTcpConnect, nil)
	if err != nil {
		tp.Close() //nolint:errcheck
		return nil, err
	}
	closers = append(closers, kp)

	if tier == Tier0 {
		go startRingBufReader(ctx, execObjs.BeekeeperExecMaps.Events, events, sentry.EventProcessCreate)
		go startRingBufReader(ctx, netObjs.BeekeeperNetMaps.Events, events, sentry.EventNetworkConnect)
	} else {
		go startPerfReader(ctx, execObjs.BeekeeperExecMaps.Events, events, sentry.EventProcessCreate)
		go startPerfReader(ctx, netObjs.BeekeeperNetMaps.Events, events, sentry.EventNetworkConnect)
	}

	return closers, nil
}

// startRingBufReader reads from an eBPF ring buffer map and forwards parsed
// events onto the events channel. Blocks until ctx is cancelled or the
// reader is closed.
func startRingBufReader(ctx context.Context, m *ebpf.Map, events chan<- sentry.SentryEvent, kind sentry.EventKind) {
	rd, err := ringbuf.NewReader(m)
	if err != nil {
		return
	}
	go func() {
		<-ctx.Done()
		rd.Close() //nolint:errcheck
	}()

	var rec ringbuf.Record
	for {
		if err := rd.ReadInto(&rec); err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return
			}
			continue
		}
		ev := parseEvent(rec.RawSample, kind)
		select {
		case events <- ev:
		default:
			atomic.AddUint64(&EventsDropped, 1)
		}
	}
}

// startPerfReader reads from a perf event array map and forwards parsed events
// onto the events channel. Used in Tier1 degraded mode.
func startPerfReader(ctx context.Context, m *ebpf.Map, events chan<- sentry.SentryEvent, kind sentry.EventKind) {
	rd, err := perf.NewReader(m, 4096)
	if err != nil {
		return
	}
	go func() {
		<-ctx.Done()
		rd.Close() //nolint:errcheck
	}()

	for {
		rec, err := rd.Read()
		if err != nil {
			if errors.Is(err, perf.ErrClosed) {
				return
			}
			continue
		}
		if rec.LostSamples > 0 {
			atomic.AddUint64(&EventsDropped, rec.LostSamples)
		}
		ev := parseEvent(rec.RawSample, kind)
		select {
		case events <- ev:
		default:
			atomic.AddUint64(&EventsDropped, 1)
		}
	}
}

// parseEvent deserialises a raw kernel event byte slice into a SentryEvent.
// The raw bytes must match the corresponding C struct layout exactly.
func parseEvent(raw []byte, kind sentry.EventKind) sentry.SentryEvent {
	ev := sentry.SentryEvent{Kind: kind, WallTime: time.Now().UTC()}
	r := bytes.NewReader(raw)

	switch kind {
	case sentry.EventProcessCreate:
		var pe processEventLayout
		if err := binary.Read(r, binary.LittleEndian, &pe); err == nil {
			ev.PID = pe.Pid
			ev.PPID = pe.Ppid
			ev.UID = pe.Uid
			ev.Exe = strings.TrimRight(string(pe.Exe[:]), "\x00")
			ev.Cmdline = strings.TrimRight(string(pe.Cmdline[:]), "\x00")
			ev.KTimeNS = pe.KtimeNS
		}
	case sentry.EventNetworkConnect:
		var ne networkEventLayout
		if err := binary.Read(r, binary.LittleEndian, &ne); err == nil {
			ev.PID = ne.Pid
			ev.PPID = ne.Ppid
			ev.UID = ne.Uid
			ev.KTimeNS = ne.KtimeNS
			// dport arrives in network byte order (big-endian) from the kernel.
			ev.DstPort = binary.BigEndian.Uint16([]byte{byte(ne.Dport >> 8), byte(ne.Dport)})
			if ne.IsIPv6 == 0 {
				b := make([]byte, 4)
				binary.LittleEndian.PutUint32(b, ne.Daddr)
				ev.DstAddr = net.IP(b)
			} else {
				addr := make([]byte, 16)
				copy(addr, ne.Daddr6[:])
				ev.DstAddr = net.IP(addr)
			}
		}
	}
	return ev
}

// StartProcessTreeBuilder consumes SentryEvent values from the events channel
// and maintains a live in-memory process tree. Every 5 seconds it publishes a
// shallow snapshot onto the tree channel, replacing any unconsumed snapshot.
// Entries older than 10 minutes are GC'd to bound memory usage.
func StartProcessTreeBuilder(ctx context.Context, events <-chan sentry.SentryEvent, tree chan<- map[uint32]sentry.ProcessNode) {
	local := make(map[uint32]sentry.ProcessNode)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	const gcCutoff = 10 * time.Minute

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			if ev.Kind == sentry.EventProcessCreate {
				local[ev.PID] = sentry.ProcessNode{
					PID:     ev.PID,
					PPID:    ev.PPID,
					UID:     ev.UID,
					Exe:     ev.Exe,
					Cmdline: ev.Cmdline,
					SeenAt:  ev.WallTime,
				}
				// GC stale entries to prevent unbounded growth.
				now := time.Now()
				for pid, node := range local {
					if now.Sub(node.SeenAt) > gcCutoff {
						delete(local, pid)
					}
				}
			}
		case <-ticker.C:
			// Publish a shallow copy of the current tree.
			snapshot := make(map[uint32]sentry.ProcessNode, len(local))
			for k, v := range local {
				snapshot[k] = v
			}
			// Non-blocking send; if channel is full, replace the stale snapshot.
			select {
			case tree <- snapshot:
			default:
				select {
				case <-tree:
				default:
				}
				tree <- snapshot
			}
		}
	}
}

// RemoveMemlock removes the kernel's RLIMIT_MEMLOCK limit so that eBPF maps
// can be allocated. Must be called before loading eBPF objects.
func RemoveMemlock() error {
	return unix.Setrlimit(unix.RLIMIT_MEMLOCK, &unix.Rlimit{
		Cur: unix.RLIM_INFINITY,
		Max: unix.RLIM_INFINITY,
	})
}
