// Package sentry provides the cross-platform Sentry correlation engine for
// Beekeeper. It defines OS-agnostic event types, alert structures, rule state,
// and evaluation interfaces. Platform-specific collectors (fanotify on Linux,
// eslogger on macOS, ETW on Windows) feed SentryEvent values into EvaluateEvent
// and receive SentryAlert values back.
//
// All types in this file carry no build tags — they compile on every platform.
package sentry

import (
	"net"
	"time"
)

// EventKind classifies a SentryEvent by the OS primitive that produced it.
type EventKind uint8

const (
	// EventProcessCreate is a new-process creation event (execve / CreateProcess).
	EventProcessCreate EventKind = iota
	// EventFileAccess is a file open / read event (fanotify FAN_OPEN / ETW).
	EventFileAccess
	// EventNetworkConnect is an outbound TCP/UDP connection event.
	EventNetworkConnect
	// EventFileWrite is a file create / write / rename event (Phase 20, SENT-05).
	// It is a DISTINCT kind from EventFileAccess so the read-clustering path
	// (SENTRY-001/006) stays uncontaminated; it dispatches to SENTRY-008.
	EventFileWrite
)

// SentryEvent is the normalised, OS-agnostic representation of a single kernel
// event. Platform collectors translate raw kernel structs into SentryEvent
// before handing off to the correlation engine; the engine itself is pure and
// platform-free.
type SentryEvent struct {
	Kind     EventKind `json:"kind"`
	PID      uint32    `json:"pid"`
	PPID     uint32    `json:"ppid"`
	UID      uint32    `json:"uid"`
	Exe      string    `json:"exe"`
	Cmdline  string    `json:"cmdline,omitempty"`
	FilePath string    `json:"file_path,omitempty"`
	DstAddr  net.IP    `json:"dst_addr,omitempty"`
	DstPort  uint16    `json:"dst_port,omitempty"`
	KTimeNS  uint64    `json:"ktime_ns"`
	WallTime time.Time `json:"wall_time"`
}

// ProcessNode is one entry in the live process tree maintained by the Sentry
// collector. It mirrors the fields needed by the correlation rules to walk
// parent chains and identify editor-descended processes.
type ProcessNode struct {
	PID     uint32
	PPID    uint32
	UID     uint32
	Exe     string
	Cmdline string
	SeenAt  time.Time
}

// SentryAlert is a structured finding emitted by the correlation engine when a
// rule fires. Callers convert SentryAlert into an AuditRecord (sentry_alert
// record_type) for the NDJSON audit log and optionally into a desktop
// notification via the notify package.
type SentryAlert struct {
	RuleID              string
	RuleName            string
	Severity            string // "critical" or "high"
	BaselineMode        bool
	ProcessPID          uint32
	ProcessExe          string
	ParentChain         []string
	FilesAccessed       []string
	NetworkDests        []string
	CorrelatedExtension string
	QuarantineRec       bool
	Timestamp           time.Time
}

// RuleWindowEntry is a single data point within a sliding time-window used by
// the stateful correlation rules (SENTRY-001, SENTRY-002, SENTRY-003). The
// Value field carries rule-specific payload (file path, exe name, or IP:port).
type RuleWindowEntry struct {
	PID    uint32
	Value  string
	SeenAt time.Time
}

// RuleState holds the mutable, in-memory windows and dedup lists used by the
// correlation rules. It is not thread-safe; callers must synchronise externally
// (e.g. via a single goroutine processing events serially).
type RuleState struct {
	// CredAccessByPID tracks sensitive file accesses per PID for SENTRY-001.
	CredAccessByPID map[uint32][]RuleWindowEntry
	// CredCLIByPID tracks credential-CLI spawns per PID for SENTRY-002.
	CredCLIByPID map[uint32][]RuleWindowEntry
	// PhoneHomeByPID tracks outbound network connections per PID for SENTRY-003.
	PhoneHomeByPID map[uint32][]RuleWindowEntry
	// PersistWriteByPID tracks persistence-path writes per PID. Populated by
	// SENTRY-008 (plan 20-04, EventFileWrite) and consumed by SENTRY-007's
	// recent-persistence-write fusion input — it is the extension point left by
	// plan 20-03 (empty until 20-04 lands the write-ingestion source).
	PersistWriteByPID map[uint32][]RuleWindowEntry
	// RecentAlerts deduplicates alerts within a short suppression window.
	RecentAlerts []recentAlert
}

// recentAlert is an unexported dedup record stored in RuleState.RecentAlerts.
type recentAlert struct {
	RuleID  string
	PID     uint32
	FiredAt time.Time
}

// RuleConfig carries tuneable thresholds for the correlation rules.  Zero
// values are replaced by engine defaults inside EvaluateEvent; callers may
// populate only the fields they wish to override.
type RuleConfig struct {
	// CredAccessThreshold is the minimum number of sensitive-file accesses
	// within CredAccessWindowSec before SENTRY-001 fires. Default: 2.
	CredAccessThreshold int
	// CredCLIThreshold is the minimum number of credential-CLI spawns within
	// CredCLIWindowSec before SENTRY-002 fires. Default: 2.
	CredCLIThreshold int
	// PhoneHomeWindowMin is the sliding window for SENTRY-003. Default: 10 min.
	PhoneHomeWindowMin time.Duration
	// CredAccessWindowSec is the sliding window for SENTRY-001. Default: 60 s.
	CredAccessWindowSec time.Duration
	// CredCLIWindowSec is the sliding window for SENTRY-002. Default: 60 s.
	CredCLIWindowSec time.Duration
	// FreshExtWindowMin is the look-back window for SENTRY-004. Default: 30 min.
	FreshExtWindowMin time.Duration
	// ExfilFusionWindowMin is the look-back window for SENTRY-005. Default: 5 min.
	ExfilFusionWindowMin time.Duration
}

// InventorySnapshot is an immutable view of the extension/plugin inventory at
// the time of event evaluation. It is provided by the inventory subsystem and
// passed into EvaluateEvent; the correlation engine treats it as read-only.
type InventorySnapshot struct {
	// RecentExtensions maps extension/plugin ID to its install time.
	RecentExtensions map[string]time.Time
}

// NewRuleState allocates a zero-valued RuleState with all maps initialised so
// that callers need not guard against nil map assignments.
func NewRuleState() *RuleState {
	return &RuleState{
		CredAccessByPID:   make(map[uint32][]RuleWindowEntry),
		CredCLIByPID:      make(map[uint32][]RuleWindowEntry),
		PhoneHomeByPID:    make(map[uint32][]RuleWindowEntry),
		PersistWriteByPID: make(map[uint32][]RuleWindowEntry),
	}
}
