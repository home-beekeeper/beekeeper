//go:build linux

package linux

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/mzansi-agentive/beekeeper/internal/sentry"
)

// InitFanotify opens a fanotify file descriptor with flags appropriate for the
// given degradation tier. On Tier0/Tier1 it requests FAN_REPORT_FID (Linux 5.1+)
// and falls back to basic flags if that fails. On Tier2 it always uses basic flags.
func InitFanotify(tier DegradationTier) (int, error) {
	var flags uint
	switch {
	case tier <= Tier1:
		flags = unix.FAN_CLASS_CONTENT | unix.FAN_NONBLOCK | unix.FAN_CLOEXEC | unix.FAN_REPORT_FID
		fd, err := unix.FanotifyInit(flags, unix.O_RDWR)
		if err == nil {
			return fd, nil
		}
		// Fallback: try without FAN_REPORT_FID (kernel < 5.1).
		flags = unix.FAN_CLASS_CONTENT | unix.FAN_NONBLOCK | unix.FAN_CLOEXEC
		return unix.FanotifyInit(flags, unix.O_RDWR)
	default: // Tier2
		flags = unix.FAN_CLASS_CONTENT | unix.FAN_NONBLOCK | unix.FAN_CLOEXEC
		return unix.FanotifyInit(flags, unix.O_RDWR)
	}
}

// FanotifyMarkPaths adds fanotify marks for each path in paths. Paths that do
// not exist are silently skipped; other errors are returned immediately.
func FanotifyMarkPaths(fanFd int, paths []string) error {
	for _, path := range paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue // skip non-existent paths silently
		}
		if err := unix.FanotifyMark(fanFd, unix.FAN_MARK_ADD,
			unix.FAN_ACCESS|unix.FAN_OPEN_PERM, unix.AT_FDCWD, path); err != nil {
			return fmt.Errorf("fanotify mark %s: %w", path, err)
		}
	}
	return nil
}

// StartFanotifyReader starts a goroutine that reads fanotify events from fanFd
// and sends normalised SentryEvent values to events. The goroutine exits when
// ctx is cancelled. The caller is responsible for closing fanFd after the
// goroutine exits.
//
// For FAN_OPEN_PERM events this function always writes FAN_ALLOW immediately
// after resolving the path — Beekeeper observes but never blocks file access
// at the fanotify layer.
func StartFanotifyReader(ctx context.Context, fanFd int, events chan<- sentry.SentryEvent) {
	epollFd, err := unix.EpollCreate1(unix.EPOLL_CLOEXEC)
	if err != nil {
		return
	}
	defer unix.Close(epollFd)

	if err := unix.EpollCtl(epollFd, unix.EPOLL_CTL_ADD, fanFd,
		&unix.EpollEvent{Events: unix.EPOLLIN, Fd: int32(fanFd)}); err != nil {
		return
	}

	// Cancel goroutine: close epollFd when context is done to unblock EpollWait.
	go func() {
		<-ctx.Done()
		unix.Close(epollFd)
	}()

	buf := make([]byte, 4096)
	epollEvents := make([]unix.EpollEvent, 8)

	for {
		n, err := unix.EpollWait(epollFd, epollEvents, 500)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return
		}

		select {
		case <-ctx.Done():
			return
		default:
		}

		for i := 0; i < n; i++ {
			if epollEvents[i].Events&unix.EPOLLIN == 0 {
				continue
			}
			readFanotifyEvents(fanFd, buf, events)
		}
	}
}

// readFanotifyEvents reads all pending fanotify events from fanFd into buf and
// sends them to events. It always responds to FAN_OPEN_PERM with FAN_ALLOW.
func readFanotifyEvents(fanFd int, buf []byte, events chan<- sentry.SentryEvent) {
	n, err := unix.Read(fanFd, buf)
	if err != nil || n == 0 {
		return
	}

	offset := 0
	for offset < n {
		if offset+int(unix.FAN_EVENT_METADATA_LEN) > n {
			break
		}
		meta := (*unix.FanotifyEventMetadata)(unsafe.Pointer(&buf[offset]))
		if meta.Event_len < unix.FAN_EVENT_METADATA_LEN {
			break
		}

		// Save original fd value before any close.
		origFd := meta.Fd

		// Resolve the path through /proc/self/fd before closing.
		var filePath string
		if meta.Fd >= 0 {
			filePath, _ = os.Readlink(fmt.Sprintf("/proc/self/fd/%d", meta.Fd))
			unix.Close(int(meta.Fd)) // CRITICAL: close immediately after readlink
		}

		// Respond to FAN_OPEN_PERM: always write FAN_ALLOW (never block the
		// accessing process — Beekeeper is an observer, not a blocker at this layer).
		if meta.Mask&unix.FAN_OPEN_PERM != 0 {
			var resp [8]byte
			binary.LittleEndian.PutUint32(resp[0:4], uint32(origFd))
			binary.LittleEndian.PutUint32(resp[4:8], unix.FAN_ALLOW)
			unix.Write(fanFd, resp[:]) //nolint:errcheck // best-effort response
		}

		// Non-blocking send — drop if channel full; fd already closed and
		// FAN_ALLOW already sent so there is no kernel-side resource leak.
		ev := sentry.SentryEvent{
			Kind:     sentry.EventFileAccess,
			PID:      uint32(meta.Pid),
			FilePath: filePath,
			WallTime: time.Now().UTC(),
		}
		select {
		case events <- ev:
		default:
		}

		offset += int(meta.Event_len)
	}
}
