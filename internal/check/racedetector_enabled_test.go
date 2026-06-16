//go:build race

package check

// raceDetectorEnabled is true when the test binary is built with -race. Wall-clock
// latency gates (TestBenchmarkRunCheckGate) are unreliable under the race detector
// because its instrumentation inflates timings well past the budget, so they skip.
const raceDetectorEnabled = true
