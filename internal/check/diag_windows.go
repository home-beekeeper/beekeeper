//go:build windows

package check

import (
	"sync/atomic"

	winsentry "github.com/home-beekeeper/beekeeper/internal/sentry/windows"
)

// eventsLost returns the number of ETW events dropped by the Sentry consumer
// (internal/sentry/windows.EventsLost) since process start. Uses an atomic
// load to avoid data races with the ETW event callback that increments the
// counter concurrently. Only compiled on Windows (SWIN-04).
func eventsLost() uint64 {
	return atomic.LoadUint64(&winsentry.EventsLost)
}
