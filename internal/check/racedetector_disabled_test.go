//go:build !race

package check

// raceDetectorEnabled is false in normal (non -race) test builds, so wall-clock
// latency gates run as usual. See racedetector_enabled_test.go for the -race case.
const raceDetectorEnabled = false
