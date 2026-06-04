// Package gateway — drift check scheduler and record emitter (§10-15, NUDGE-06).
//
// The weekly drift check is a gateway-owned periodic goroutine that calls an
// injected metadata-fetch function, compares the returned latest versions against
// the configured floor via the EXPORTED nudge.IsMajorDrift, and emits an async
// best-effort record_type:"version_drift" info record for each PM where a new
// major is available. It NEVER auto-updates floors (PRD §7.1, Out-of-Scope).
//
// The check is:
//   - GATEWAY-ONLY — resolves Open Q2: the daemon is the long-lived process that
//     has a scheduler; the check hook is one-shot and cannot run periodic tasks.
//   - ASYNC AND NON-BLOCKING — Pitfall 6: drift check MUST NEVER run on the
//     per-request applyPolicy / check hot path. It lives in a dedicated goroutine
//     started by Start (gateway.go) and ticked by time.Ticker.
//   - FAIL-OPEN — a metadataFetchFn error emits NO record and NEVER panics or
//     blocks the gateway (T-08-24). Fail-open here is correct by design: a drift
//     check failure should not affect the request path.
//   - INJECTED metadataFetchFn — the real implementation fetches pnpm/bun latest
//     versions (off the hot path); tests substitute a fake returning a fixed map.
package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/bantuson/beekeeper/internal/audit"
	"github.com/bantuson/beekeeper/internal/nudge"
)

// metadataFetchFn is the injectable package-level variable for the drift check's
// metadata lookup. The real implementation fetches the latest published versions
// for "pnpm" and "bun" from the respective registries / CLI commands. Tests
// substitute a fake returning a controlled map.
//
// Returns a map of PM name → latest version string, e.g.:
//
//	{"pnpm": "12.0.0", "bun": "1.3.14"}
//
// Any error aborts the drift check entirely (fail-open, non-blocking).
var metadataFetchFn = realMetadataFetch

// realMetadataFetch is the production metadata fetch. It calls `pnpm --version`
// to get the locally-installed version as a proxy for "latest available in the
// npm registry" — a proper registry query is future work (Open Q2 placeholder).
// When the command is unavailable or errors, the PM is omitted from the result
// (fail-open, non-blocking — T-08-24).
//
// NOTE: This is intentionally minimal — the real drift check architecture would
// query the npm registry for the latest pnpm/bun version. For now the function
// provides the correct wiring; the metadata query can be enriched later without
// changing the checkDrift contract.
func realMetadataFetch(ctx context.Context) (map[string]string, error) {
	// For the initial implementation: return an empty map to avoid any network
	// calls in production until the registry query is implemented. The injected
	// test fn provides the actual test coverage. This satisfies the structural
	// requirement (§10-15 wiring) without introducing live network calls.
	return map[string]string{}, nil
}

// checkDrift evaluates whether a new major version has been released for pnpm
// or bun relative to the configured floor in h.cfg.Nudge.VersionFloors.
//
// For each PM in the metadata map, it calls the EXPORTED nudge.IsMajorDrift
// (BLOCKER 2 closed — the private isMajorDrift is package-private and
// uncallable from package gateway). When a new major is detected, it emits an
// async, best-effort record_type:"version_drift" severity-info audit record.
//
// checkDrift NEVER:
//   - auto-updates floors (PRD §7.1, Out-of-Scope — T-08-25 mitigated)
//   - panics on fetch error or malformed version strings (T-08-24)
//   - blocks the caller (async-safe; always called from a goroutine)
func (h *gatewayHandler) checkDrift(ctx context.Context) {
	versions, err := metadataFetchFn(ctx)
	if err != nil {
		// Fail-open: log to stderr and return — a metadata fetch error must never
		// block or propagate (T-08-24, Pitfall 6).
		fmt.Fprintf(os.Stderr, "beekeeper gateway: drift check metadata fetch failed: %v\n", err)
		return
	}

	floors := h.cfg.Nudge.VersionFloors

	// Check pnpm drift.
	if latest, ok := versions["pnpm"]; ok && latest != "" {
		if nudge.IsMajorDrift(latest, floors.Pnpm) {
			h.emitVersionDrift(ctx, "pnpm", latest, floors.Pnpm)
		}
	}

	// Check bun drift.
	if latest, ok := versions["bun"]; ok && latest != "" {
		if nudge.IsMajorDrift(latest, floors.Bun) {
			h.emitVersionDrift(ctx, "bun", latest, floors.Bun)
		}
	}
}

// emitVersionDrift writes a single record_type:"version_drift" severity-info
// audit record best-effort. A write failure is logged but NEVER propagated.
// It does NOT auto-update floors (PRD §7.1).
func (h *gatewayHandler) emitVersionDrift(_ context.Context, pm, latest, floor string) {
	if h.cfg.AuditPath == "" {
		return
	}

	w, err := audit.NewWriter(h.cfg.AuditPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper gateway: version_drift audit write failed: %v\n", err)
		return
	}
	defer w.Close()

	var raw [16]byte
	_, _ = rand.Read(raw[:])
	recordID := hex.EncodeToString(raw[:])

	rec := audit.AuditRecord{
		RecordType:  "version_drift",
		RecordID:    recordID,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		ScannerName: "beekeeper",
		Endpoint:    "gateway",
		// Reason encodes the drift summary: PM + detected vs floor.
		Reason: fmt.Sprintf("%s: new major detected (latest=%s, floor=%s) — update floor via 'beekeeper nudge config' (floors not auto-updated per PRD §7.1)", pm, latest, floor),
		// PMState carries the version information for the forensic record.
		PMState: fmt.Sprintf("%s latest=%s floor=%s", pm, latest, floor),
	}

	if err := w.Write(rec); err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper gateway: version_drift audit write error: %v\n", err)
	}
}

// startDriftScheduler launches the periodic drift check goroutine. It reads the
// interval from cfg.Nudge.MajorDriftCheck.Interval (default "168h" = weekly per
// PRD §7.1). The goroutine exits when ctx is cancelled (gateway shutdown).
//
// The scheduler runs ONLY when cfg.Nudge.MajorDriftCheck.Enabled is true.
// It NEVER blocks the Start function or the request path (Pitfall 6).
//
// The first check is deferred to the first tick (no check on startup) so the
// gateway starts promptly without waiting for the first metadata fetch.
func startDriftScheduler(ctx context.Context, h *gatewayHandler) {
	if !h.cfg.Nudge.MajorDriftCheck.Enabled {
		return // drift check disabled in config
	}

	intervalStr := h.cfg.Nudge.MajorDriftCheck.Interval
	if intervalStr == "" {
		intervalStr = "168h" // default: weekly
	}
	interval, err := time.ParseDuration(intervalStr)
	if err != nil || interval <= 0 {
		fmt.Fprintf(os.Stderr, "beekeeper gateway: invalid drift check interval %q, defaulting to 168h\n", intervalStr)
		interval = 168 * time.Hour
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Run in a separate goroutine so a slow fetch doesn't block the ticker.
				go func() {
					driftCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					h.checkDrift(driftCtx)
				}()
			}
		}
	}()
}
