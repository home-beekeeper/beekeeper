// Package check: diag.go assembles the DiagReport that beekeeper diag displays.
//
// CollectDiag gathers four health signals:
//   - Hook latency p95/p99 from the persisted ring file (accumulated across
//     one-shot beekeeper check invocations)
//   - LlamaFirewall sidecar inference latency p95 from the in-process tracker
//   - Catalog freshness per source from state.json (catalog.LoadState)
//   - ETW EventsLost (real counter on Windows, always 0 on other platforms via
//     the platform-dispatched eventsLost() function)
//
// diag.go is platform-agnostic. The platform-specific eventsLost() symbol is
// provided by diag_windows.go (//go:build windows) and diag_other.go
// (//go:build !windows).
package check

import (
	"sort"

	"github.com/mzansi-agentive/beekeeper/internal/catalog"
	"github.com/mzansi-agentive/beekeeper/internal/llamafirewall"
)

// DiagReport is the complete health report assembled by CollectDiag.
// It is returned to the caller (beekeeper diag CLI, Plan 05) for formatting.
type DiagReport struct {
	// HookLatencyP95MS is the 95th-percentile hook handler latency in
	// milliseconds, computed over the last 100 persisted samples.
	HookLatencyP95MS int64

	// HookLatencyP99MS is the 99th-percentile hook handler latency in
	// milliseconds, computed over the last 100 persisted samples.
	HookLatencyP99MS int64

	// SidecarLatencyP95MS is the 95th-percentile LlamaFirewall sidecar
	// inference latency in milliseconds from the in-process tracker.
	SidecarLatencyP95MS int64

	// CatalogSources is the per-source freshness snapshot, sorted by name.
	// It includes all sources present in state.json, including beekeeper-self
	// when the self-catalog has been synced at least once.
	CatalogSources []CatalogSourceStatus

	// ETWEventsLost is the number of ETW events dropped by the Sentry consumer
	// since the process started. Always 0 on non-Windows platforms (ETW is a
	// Windows-only tracing mechanism).
	ETWEventsLost uint64
}

// CatalogSourceStatus is one row in DiagReport.CatalogSources, representing
// the last-known state of a single catalog source.
type CatalogSourceStatus struct {
	// Name is the catalog source identifier (e.g. "bumblebee", "osv", "socket",
	// "beekeeper-self").
	Name string

	// Degraded is true when the source has been flagged by a sanity check and
	// its matches count at most 0.5 toward corroboration (CTLG-08).
	Degraded bool

	// Count is the number of entries in the last-seen catalog snapshot.
	Count int

	// Hash is the content hash of the last-seen catalog snapshot. This acts as
	// a last-sync identity token (not sensitive — it is a content digest).
	Hash string
}

// CollectDiag assembles a DiagReport from all available data sources.
//
//   - stateFile is the path to ~/.beekeeper/state.json (catalog freshness source).
//   - hookLatencyRingPath is the path to the hook-latency.json ring file written
//     by runCheck (typically ~/.beekeeper/hook-latency.json).
//
// CollectDiag is read-only: it never mutates enforcement state. A missing or
// corrupt stateFile or ring file results in zero-value fields rather than an
// error (missing-file-is-OK pattern).
func CollectDiag(stateFile, hookLatencyRingPath string) DiagReport {
	var report DiagReport

	// Hook latency p95/p99 from the persisted ring.
	samples := loadHookLatency(hookLatencyRingPath)
	if len(samples) > 0 {
		var lt llamafirewall.LatencyTracker
		for _, ms := range samples {
			lt.Record(ms)
		}
		report.HookLatencyP95MS = lt.P95()
		report.HookLatencyP99MS = lt.P99()
	}

	// LlamaFirewall sidecar p95 from the in-process global tracker.
	report.SidecarLatencyP95MS = llamafirewall.GlobalLatencyTracker.P95()

	// Catalog freshness per source from state.json.
	// LoadState treats a missing file as an empty WatchState (missing-file-is-OK).
	ws, err := catalog.LoadState(stateFile)
	if err == nil && len(ws.Sources) > 0 {
		// Collect source names and sort them for deterministic output.
		names := make([]string, 0, len(ws.Sources))
		for name := range ws.Sources {
			names = append(names, name)
		}
		sort.Strings(names)

		report.CatalogSources = make([]CatalogSourceStatus, 0, len(names))
		for _, name := range names {
			ss := ws.Sources[name]
			report.CatalogSources = append(report.CatalogSources, CatalogSourceStatus{
				Name:     name,
				Degraded: ss.Degraded,
				Count:    ss.Count,
				Hash:     ss.Hash,
			})
		}
	}

	// ETW EventsLost: platform-dispatched (real counter on Windows, 0 elsewhere).
	report.ETWEventsLost = eventsLost()

	return report
}
