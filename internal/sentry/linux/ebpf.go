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

	"github.com/home-beekeeper/beekeeper/internal/sentry"
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

// dnsEventLayout mirrors the C struct dns_event layout exactly (Phase 20,
// SENT-11). Field order and sizes must remain in sync with dns_tracer.bpf.c.
type dnsEventLayout struct {
	Pid     uint32
	Ppid    uint32
	Uid     uint32
	Dport   uint16
	Pad     [2]byte
	KtimeNS uint64
	Qbuf    [256]byte // raw DNS message bytes; QNAME decoded by decodeDNSQName
}

// StartEBPFReaders attaches eBPF probes (tracepoint for exec, kprobe for
// tcp_connect) and starts goroutines that read from the associated ring/perf
// buffers. The chosen buffer type is determined by tier:
//   - Tier0 → ring buffer (kernel ≥ 5.8)
//   - Tier1 → perf event array
//
// The returned closers must be closed when the caller shuts down.
// dnsObjs is OPTIONAL (Phase 20, SENT-11): when nil (DNS bytecode unavailable or
// load failed), DNS ingestion is simply absent and the exec/net readers run
// unchanged. This keeps the DNS stretch fail-safe — a missing DNS source never
// degrades the core sources.
func StartEBPFReaders(ctx context.Context, execObjs *BeekeeperExecObjects, netObjs *BeekeeperNetObjects, dnsObjs *BeekeeperDNSObjects, tier DegradationTier, events chan<- sentry.SentryEvent) ([]io.Closer, error) {
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

	// SENT-11 (OPTIONAL): DNS kprobes on udp_sendmsg/tcp_sendmsg (dport 53).
	if dnsObjs != nil {
		if kpUDP, derr := link.Kprobe("udp_sendmsg", dnsObjs.BeekeeperDNSPrograms.KprobeUdpSendmsg, nil); derr == nil {
			closers = append(closers, kpUDP)
		}
		if kpTCP, derr := link.Kprobe("tcp_sendmsg", dnsObjs.BeekeeperDNSPrograms.KprobeTcpSendmsg, nil); derr == nil {
			closers = append(closers, kpTCP)
		}
		if tier == Tier0 {
			go startRingBufReader(ctx, dnsObjs.BeekeeperDNSMaps.Events, events, sentry.EventDNSQuery)
		} else {
			go startPerfReader(ctx, dnsObjs.BeekeeperDNSMaps.Events, events, sentry.EventDNSQuery)
		}
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
	case sentry.EventDNSQuery:
		var de dnsEventLayout
		if err := binary.Read(r, binary.LittleEndian, &de); err == nil {
			ev.PID = de.Pid
			ev.PPID = de.Ppid
			ev.UID = de.Uid
			ev.KTimeNS = de.KtimeNS
			ev.DstPort = binary.BigEndian.Uint16([]byte{byte(de.Dport >> 8), byte(de.Dport)})
			ev.FilePath = decodeDNSQName(de.Qbuf[:])
		}
	}
	return ev
}

// decodeDNSQName decodes the QNAME from a raw DNS message (length-prefixed wire
// format) into a dotted domain string. buf starts at the DNS message header; the
// 12-byte header is skipped and the question's QNAME labels follow. Compression
// pointers are not expected in a query QNAME and terminate the decode. Phase 20,
// SENT-11.
func decodeDNSQName(buf []byte) string {
	const dnsHeaderLen = 12
	if len(buf) <= dnsHeaderLen {
		return ""
	}
	var sb strings.Builder
	for i := dnsHeaderLen; i < len(buf); {
		l := int(buf[i])
		if l == 0 {
			break // root label: end of QNAME
		}
		if l&0xC0 != 0 {
			break // compression pointer / reserved: not valid in a query QNAME
		}
		i++
		if i+l > len(buf) {
			break
		}
		if sb.Len() > 0 {
			sb.WriteByte('.')
		}
		sb.Write(buf[i : i+l])
		i += l
		if sb.Len() > 253 { // max DNS name length
			break
		}
	}
	return sb.String()
}

// StartProcessTreeBuilder consumes SentryEvent values from the events channel
// and maintains a live in-memory process tree. Every 5 seconds it publishes a
// shallow snapshot onto the tree channel. The tree channel must be buffered
// (capacity ≥ 1); if it is full the new snapshot is dropped rather than
// blocking.
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
			// Non-blocking send; drop the snapshot if the consumer is slow.
			select {
			case tree <- snapshot:
			default:
				// Consumer has not drained the previous snapshot; discard this one.
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
