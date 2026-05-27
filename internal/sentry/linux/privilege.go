//go:build linux

package linux

import (
	"fmt"
	"net"
	"os"

	seccomp "github.com/elastic/go-seccomp-bpf"
	"golang.org/x/sys/unix"
)

// DropCapabilities zeros all capabilities on the current process and then
// re-enables only the capabilities listed in keep. This must be called after
// all eBPF objects have been loaded and all privileged setup is complete.
//
// Uses the two-struct Capget pattern required by LINUX_CAPABILITY_VERSION_3
// (golang/go#44312 — a single unix.CapUserData causes a kernel ABI mismatch).
func DropCapabilities(keep []uintptr) error {
	hdr := unix.CapUserHeader{
		Version: unix.LINUX_CAPABILITY_VERSION_3,
		Pid:     0,
	}
	var data [2]unix.CapUserData // MUST be array of 2 (golang/go#44312)

	if err := unix.Capget(&hdr, &data[0]); err != nil {
		return fmt.Errorf("capget: %w", err)
	}

	// Zero all capabilities.
	data[0].Effective = 0
	data[0].Permitted = 0
	data[0].Inheritable = 0
	data[1].Effective = 0
	data[1].Permitted = 0
	data[1].Inheritable = 0

	// Re-enable only the kept capabilities.
	for _, cap := range keep {
		idx := int(cap / 32)
		bit := uint32(1 << (cap % 32))
		if idx == 0 {
			data[0].Effective |= bit
			data[0].Permitted |= bit
		} else if idx == 1 {
			data[1].Effective |= bit
			data[1].Permitted |= bit
		}
	}

	return unix.Capset(&hdr, &data[0])
}

// keepCaps returns the capabilities to retain after eBPF objects are loaded
// and privilege separation is applied. The tier parameter is reserved for
// future tier-specific capability profiles; currently all tiers keep the same
// set post-load.
func keepCaps(tier DegradationTier) []uintptr {
	_ = tier // same caps for all tiers post-eBPF-load
	return []uintptr{
		unix.CAP_NET_ADMIN,       // needed for fanotify on network sockets
		unix.CAP_DAC_READ_SEARCH, // needed for /proc/pid/fd/ symlink resolution
	}
}

// ApplySeccomp installs a seccomp-BPF filter that kills the calling process if
// it attempts any of the privileged syscalls listed below. The default action is
// ActionAllow so that all other syscalls pass through unimpeded.
//
// NoNewPrivs is set to true, which prevents any setuid/setgid escalation from
// child processes spawned after this call.
func ApplySeccomp() error {
	filter := seccomp.Filter{
		NoNewPrivs: true,
		Flag:       seccomp.FilterFlagTSync,
		Policy: seccomp.Policy{
			DefaultAction: seccomp.ActionAllow,
			Syscalls: []seccomp.SyscallGroup{
				{
					Action: seccomp.ActionKillProcess,
					Names: []string{
						"execve", "execveat",
						"ptrace",
						"kexec_load",
						"reboot",
						"swapon", "swapoff",
						"mount", "umount2",
					},
				},
			},
		},
	}
	return seccomp.LoadFilter(filter)
}

// sdNotifyReady sends the systemd sd_notify READY=1 message to the socket
// indicated by the NOTIFY_SOCKET environment variable. If the variable is
// unset (non-systemd deployment) this is a no-op.
func sdNotifyReady() error {
	socketPath := os.Getenv("NOTIFY_SOCKET")
	if socketPath == "" {
		return nil
	}
	conn, err := net.Dial("unixgram", socketPath)
	if err != nil {
		return fmt.Errorf("sd_notify dial: %w", err)
	}
	defer conn.Close()
	_, err = conn.Write([]byte("READY=1"))
	return err
}
