//go:build linux

// Package linux provides the Linux-specific Sentry collectors: fanotify file-access
// monitoring, eBPF exec/network tracing, and privilege separation.
package linux

import (
	"errors"
	"fmt"

	ebpf "github.com/cilium/ebpf"
	"github.com/cilium/ebpf/features"
	"golang.org/x/sys/unix"
)

// DegradationTier describes the capability level available on the running kernel.
// Higher tiers indicate reduced telemetry coverage due to missing kernel features
// or insufficient privileges.
type DegradationTier int

const (
	// Tier0 is full capability: ring buffer + FAN_REPORT_FID + eBPF.
	Tier0 DegradationTier = iota
	// Tier1 is degraded: perf buffer instead of ring buffer, no FAN_REPORT_FID.
	Tier1
	// Tier2 is minimal: fanotify only, no eBPF (insufficient privileges or too old kernel).
	Tier2
)

// ProbeTier detects the runtime capability level and returns the appropriate
// DegradationTier. It is safe to call multiple times; each call re-probes.
func ProbeTier() DegradationTier {
	return probeTier()
}

// TierString returns a human-readable description of the degradation tier for
// use in `beekeeper protect status` output.
func TierString(t DegradationTier) string {
	switch t {
	case Tier0:
		return "Tier0 (full: eBPF ring buffer + FAN_REPORT_FID)"
	case Tier1:
		return "Tier1 (degraded: eBPF perf buffer, no FAN_REPORT_FID)"
	case Tier2:
		return "Tier2 (minimal: fanotify only, eBPF unavailable)"
	default:
		return fmt.Sprintf("unknown tier %d", int(t))
	}
}

// probeTier probes available kernel capabilities and returns the degradation tier.
func probeTier() DegradationTier {
	// Check CAP_BPF (bit 39) and CAP_SYS_ADMIN (bit 21) using the two-struct
	// pattern required by the Linux kernel ABI (golang/go#44312).
	hdr := unix.CapUserHeader{
		Version: unix.LINUX_CAPABILITY_VERSION_3,
		Pid:     0,
	}
	var data [2]unix.CapUserData
	if err := unix.Capget(&hdr, &data[0]); err != nil {
		// Cannot determine capabilities — assume unprivileged.
		return Tier2
	}

	capBPF := (data[1].Effective >> (39 % 32)) & 1
	capSysAdmin := (data[0].Effective >> (21 % 32)) & 1
	hasPriv := capBPF != 0 || capSysAdmin != 0

	if !hasPriv {
		return Tier2
	}

	// Probe ring buffer availability (Linux 5.8+).
	if err := features.HaveMapType(ebpf.RingBuf); err != nil {
		if errors.Is(err, ebpf.ErrNotSupported) {
			return Tier1
		}
		// Unknown error — degrade conservatively.
		return Tier1
	}

	// Probe FAN_REPORT_FID (Linux 5.1+).
	fd, err := unix.FanotifyInit(
		unix.FAN_CLASS_CONTENT|unix.FAN_NONBLOCK|unix.FAN_REPORT_FID, 0)
	if err != nil {
		// EINVAL means FAN_REPORT_FID not supported — degrade to Tier1.
		return Tier1
	}
	unix.Close(fd)

	return Tier0
}
