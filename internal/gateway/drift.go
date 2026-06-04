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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/bantuson/beekeeper/internal/audit"
	"github.com/bantuson/beekeeper/internal/nudge"
)

// npmDriftRegistryBase is the npm registry base URL used by realMetadataFetch to
// query dist-tags for pnpm and bun. It is a package-level variable so tests can
// redirect requests to an httptest server without real network access.
//
// NOTE: This is intentionally NOT imported from internal/catalog to keep packages
// decoupled. The two vars serve different consumers (registry.go serves the
// lifecycle/publish-time adapters; this var serves the drift check adapter).
var npmDriftRegistryBase = "https://registry.npmjs.org"

// metadataFetchFn is the injectable package-level variable for the drift check's
// metadata lookup. Tests substitute a fake returning a controlled map; production
// uses realMetadataFetch which queries the npm registry dist-tags endpoint.
//
// Returns a map of PM name → latest version string, e.g.:
//
//	{"pnpm": "12.0.0", "bun": "1.3.14"}
//
// realMetadataFetch returns nil error even when individual PM fetches fail
// (per-PM fail-open); only a completely unrecoverable error triggers non-nil.
// checkDrift treats any non-nil error as fail-open (no record, no panic).
var metadataFetchFn = realMetadataFetch

// driftDistTagsResponse is the JSON shape returned by the npm dist-tags endpoint:
//
//	GET https://registry.npmjs.org/-/package/<pm>/dist-tags
//	→ {"latest":"12.0.0","next":"..."}
type driftDistTagsResponse struct {
	Latest string `json:"latest"`
}

// realMetadataFetch queries the npm registry dist-tags endpoint for "pnpm" and
// "bun" and returns a map of PM name → latest version string (DRIFT-01).
//
// Design decisions:
//   - Per-PM fail-open: if one PM's fetch fails (non-200, network error, parse
//     error, or empty latest), that PM is OMITTED from the returned map and
//     the function continues to the next PM. The function returns nil error
//     (even when some PMs were skipped) because checkDrift uses an empty/partial
//     map gracefully — it simply has no drift to report for the missing PM.
//     This is correct: a transient registry outage for one PM must not suppress
//     drift detection for the other.
//   - 5s HTTP client timeout (smaller than the scheduler's 30s per-check ctx)
//     so a slow response doesn't consume the full check budget.
//   - 256KB io.LimitReader: dist-tags responses are tiny (~100 bytes); the cap
//     defends against a runaway/malicious response (T-09-10).
//   - npmDriftRegistryBase is overridable so tests redirect to httptest (T-09-11:
//     the base URL is a hardcoded constant, never derived from agent input).
//   - Floors are NEVER auto-bumped (PRD §7.1, Out-of-Scope — T-09-12).
func realMetadataFetch(ctx context.Context) (map[string]string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	pms := []string{"pnpm", "bun"}
	result := make(map[string]string, len(pms))

	for _, pm := range pms {
		url := npmDriftRegistryBase + "/-/package/" + pm + "/dist-tags"

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "beekeeper gateway: drift fetch build request %s: %v\n", pm, err)
			continue // per-PM fail-open: omit this PM, continue to next
		}
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "beekeeper gateway: drift fetch %s: %v\n", pm, err)
			continue // per-PM fail-open: omit this PM (T-09-10, T-09-13)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			fmt.Fprintf(os.Stderr, "beekeeper gateway: drift fetch %s: HTTP %d\n", pm, resp.StatusCode)
			continue // per-PM fail-open: omit this PM
		}

		// Cap to 256KB — dist-tags payload is ~100 bytes; this defends against
		// runaway responses without allocating the full cap upfront (T-09-10).
		limited := io.LimitReader(resp.Body, 256<<10)
		var tags driftDistTagsResponse
		decodeErr := json.NewDecoder(limited).Decode(&tags)
		resp.Body.Close()
		if decodeErr != nil {
			fmt.Fprintf(os.Stderr, "beekeeper gateway: drift fetch %s parse: %v\n", pm, decodeErr)
			continue // per-PM fail-open: omit this PM
		}

		if tags.Latest == "" {
			fmt.Fprintf(os.Stderr, "beekeeper gateway: drift fetch %s: empty latest field\n", pm)
			continue // per-PM fail-open: omit this PM
		}

		result[pm] = tags.Latest
	}

	// Return nil error even when some PMs were omitted. checkDrift handles a
	// partial map correctly (it only iterates keys that exist). The per-PM
	// stderr logs above are the only signal; no fatal error is raised.
	return result, nil
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

		// WR-04: bound concurrency to a single in-flight drift check. Previously
		// every tick spawned a fresh goroutine "so a slow fetch doesn't block the
		// ticker" — but the interval is operator-configurable down to any positive
		// duration, so a checkDrift that routinely outlasts the interval would
		// accumulate goroutines without bound (a config-driven resource leak).
		// The atomic flag drops a tick while a previous check is still running;
		// the next tick after completion picks up again. A dropped drift check is
		// harmless (it is an advisory, best-effort, fail-open task).
		var running atomic.Bool
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Skip this tick if the prior check has not finished. Run in a
				// goroutine so a slow fetch never blocks the ticker, but only
				// ever one at a time.
				if running.CompareAndSwap(false, true) {
					go func() {
						defer running.Store(false)
						driftCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
						defer cancel()
						h.checkDrift(driftCtx)
					}()
				}
			}
		}
	}()
}
