//go:build linux

package linux

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/home-beekeeper/beekeeper/internal/sentry"
)

// Phase 20 (SENT-06): file-WRITE ingestion via a SEPARATE fanotify group.
//
// FAN_REPORT_DFID_NAME (kernel 5.9+) is REQUIRED to recover the written path
// from create/move events, but it is INCOMPATIBLE with the existing
// FAN_CLASS_CONTENT permission group (FanotifyInit returns EINVAL). So write
// watching runs in its own FAN_CLASS_NOTIF group with FAN_REPORT_DFID_NAME,
// marking the PARENT directories of persistence surfaces with
// FAN_CREATE|FAN_MOVED_TO|FAN_ONDIR (MOVED_TO/CREATE rather than the chatty
// MODIFY — editors write-temp-then-rename). Events carry a directory file
// handle plus the entry name; the reader resolves the handle via
// open_by_handle_at and joins the name into a full path -> EventFileWrite.
//
// Real capture is CI-validated (Ubuntu kernel matrix); locally this is
// build-check-only (Windows dev machine).

// kernelAtLeast reports whether the running kernel is >= major.minor. It is the
// >=5.9 gate for FAN_REPORT_DFID_NAME; on any parse failure it fails safe
// (returns false → degrade, do not open the 2nd group).
func kernelAtLeast(major, minor int) bool {
	var uts unix.Utsname
	if err := unix.Uname(&uts); err != nil {
		return false
	}
	rel := string(uts.Release[:])
	if i := strings.IndexByte(rel, 0); i >= 0 {
		rel = rel[:i]
	}
	parts := strings.SplitN(rel, ".", 3)
	if len(parts) < 2 {
		return false
	}
	maj, err1 := strconv.Atoi(parts[0])
	min, err2 := strconv.Atoi(strings.TrimRight(parts[1], "abcdefghijklmnopqrstuvwxyz-+"))
	if err1 != nil || err2 != nil {
		return false
	}
	if maj != major {
		return maj > major
	}
	return min >= minor
}

// InitFanotifyWrite opens the SECOND fanotify group used for write detection.
// It returns an error (and -1) on kernels older than 5.9 so the daemon degrades
// gracefully (no EINVAL, no crash) rather than losing the read-watch group too.
func InitFanotifyWrite() (int, error) {
	if !kernelAtLeast(5, 9) {
		return -1, fmt.Errorf("write-watch requires kernel >= 5.9 for FAN_REPORT_DFID_NAME; degrading (read-watch unaffected)")
	}
	flags := uint(unix.FAN_CLASS_NOTIF | unix.FAN_REPORT_DFID_NAME | unix.FAN_NONBLOCK | unix.FAN_CLOEXEC)
	return unix.FanotifyInit(flags, unix.O_RDONLY)
}

// FanotifyMarkWriteDirs marks each persistence PARENT directory for create/move
// events on the write group. Non-existent dirs are skipped silently.
func FanotifyMarkWriteDirs(fanFd int, dirs []string) error {
	const mask = unix.FAN_CREATE | unix.FAN_MOVED_TO | unix.FAN_ONDIR
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		if err := unix.FanotifyMark(fanFd, unix.FAN_MARK_ADD, mask, unix.AT_FDCWD, dir); err != nil {
			return fmt.Errorf("fanotify write-mark %s: %w", dir, err)
		}
	}
	return nil
}

// StartWriteWatch sets up the file-write watch group end to end: init the
// 5.9+ group, mark the persistence directories (resolved against $HOME), open
// the $HOME mount reference for open_by_handle_at, and start the reader
// goroutine. It returns a closer to defer. On an unsupported kernel (or any
// setup error) it returns a nil closer and an error the daemon logs as a
// graceful degrade — the read-watch group is unaffected.
func StartWriteWatch(ctx context.Context, events chan<- sentry.SentryEvent) (func(), error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home: %w", err)
	}
	fanFd, err := InitFanotifyWrite()
	if err != nil {
		return nil, err
	}
	absDirs := make([]string, 0, len(daemonPersistenceDirs))
	for _, d := range daemonPersistenceDirs {
		absDirs = append(absDirs, filepath.Join(home, d))
	}
	if err := FanotifyMarkWriteDirs(fanFd, absDirs); err != nil {
		unix.Close(fanFd)
		return nil, err
	}
	mountRef, err := os.OpenFile(home, os.O_RDONLY, 0)
	if err != nil {
		unix.Close(fanFd)
		return nil, fmt.Errorf("open mount reference: %w", err)
	}
	go StartFanotifyWriteReader(ctx, fanFd, mountRef, events)
	return func() {
		unix.Close(fanFd)
		_ = mountRef.Close()
	}, nil
}

// StartFanotifyWriteReader reads create/move events from the write group and
// emits sentry.EventFileWrite. mountRef is an open O_PATH fd on the same mount
// as the watched dirs (typically $HOME), used as the mount reference for
// open_by_handle_at. The goroutine exits when ctx is cancelled; the caller
// closes fanFd and mountRef afterwards.
func StartFanotifyWriteReader(ctx context.Context, fanFd int, mountRef *os.File, events chan<- sentry.SentryEvent) {
	epollFd, err := unix.EpollCreate1(unix.EPOLL_CLOEXEC)
	if err != nil {
		return
	}
	defer unix.Close(epollFd)

	if err := unix.EpollCtl(epollFd, unix.EPOLL_CTL_ADD, fanFd,
		&unix.EpollEvent{Events: unix.EPOLLIN, Fd: int32(fanFd)}); err != nil {
		return
	}

	go func() {
		<-ctx.Done()
		unix.Close(epollFd)
	}()

	buf := make([]byte, 8192)
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
			readFanotifyWriteEvents(fanFd, int(mountRef.Fd()), buf, events)
		}
	}
}

// readFanotifyWriteEvents parses pending DFID_NAME events and emits EventFileWrite.
func readFanotifyWriteEvents(fanFd, mountFd int, buf []byte, events chan<- sentry.SentryEvent) {
	n, err := unix.Read(fanFd, buf)
	if err != nil || n == 0 {
		return
	}

	offset := 0
	for offset+int(unix.FAN_EVENT_METADATA_LEN) <= n {
		meta := (*unix.FanotifyEventMetadata)(unsafe.Pointer(&buf[offset]))
		evLen := int(meta.Event_len)
		if evLen < int(unix.FAN_EVENT_METADATA_LEN) || offset+evLen > n {
			break
		}

		if path, ok := resolveWritePath(buf[offset:offset+evLen], mountFd); ok {
			ev := sentry.SentryEvent{
				Kind:     sentry.EventFileWrite,
				PID:      uint32(meta.Pid),
				FilePath: path,
				WallTime: time.Now().UTC(),
			}
			select {
			case events <- ev:
			default:
				atomic.AddUint64(&EventsDropped, 1)
			}
		}

		offset += evLen
	}
}

// resolveWritePath walks the info records following the event metadata, finds a
// DFID_NAME record, resolves its directory file handle via open_by_handle_at,
// and joins the entry name to produce the full written path.
func resolveWritePath(ev []byte, mountFd int) (string, bool) {
	metaLen := int(unix.FAN_EVENT_METADATA_LEN)
	rec := metaLen
	for rec+4 <= len(ev) {
		infoType := ev[rec]
		infoLen := int(binary.LittleEndian.Uint16(ev[rec+2 : rec+4]))
		if infoLen < 4 || rec+infoLen > len(ev) {
			break
		}
		if infoType == unix.FAN_EVENT_INFO_TYPE_DFID_NAME {
			// header(4) + __kernel_fsid_t(8) + struct file_handle{ handle_bytes(4)
			// + handle_type(4) + f_handle[handle_bytes] } + name(NUL-terminated).
			p := rec + 4 + 8
			if p+8 > rec+infoLen {
				break
			}
			handleBytes := int(binary.LittleEndian.Uint32(ev[p : p+4]))
			handleType := int32(binary.LittleEndian.Uint32(ev[p+4 : p+8]))
			fhStart := p + 8
			nameStart := fhStart + handleBytes
			if nameStart > rec+infoLen {
				break
			}
			fh := make([]byte, handleBytes)
			copy(fh, ev[fhStart:nameStart])

			nameBytes := ev[nameStart : rec+infoLen]
			if z := bytes.IndexByte(nameBytes, 0); z >= 0 {
				nameBytes = nameBytes[:z]
			}
			name := string(nameBytes)

			handle := unix.NewFileHandle(handleType, fh)
			dfd, err := unix.OpenByHandleAt(mountFd, handle, unix.O_PATH|unix.O_NONBLOCK)
			if err != nil {
				return "", false
			}
			dir, _ := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", dfd))
			unix.Close(dfd)
			if dir == "" {
				return "", false
			}
			if name == "" || name == "." {
				return dir, true
			}
			return filepath.Join(dir, name), true
		}
		rec += infoLen
	}
	return "", false
}
