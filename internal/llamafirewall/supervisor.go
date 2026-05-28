// Package llamafirewall implements the LlamaFirewall sidecar supervisor.
// The supervisor manages the Python sidecar process lifecycle: starting it,
// dialling the Unix socket (or named pipe on Windows), health-monitoring via
// process wait, and restarting up to MaxRetries times with exponential backoff
// before entering degraded mode.
package llamafirewall

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// ErrSidecarUnavailable is returned by Scan when the sidecar has failed and the
// configured fail-mode is "closed" or unset.
var ErrSidecarUnavailable = errors.New("llamafirewall: sidecar unavailable")

// LlamaFirewallConfig holds configuration for the LlamaFirewall sidecar.
// This mirrors the structure in internal/config (defined here for package
// independence; the CLI wires the two via config.LlamaFirewall).
type LlamaFirewallConfig struct {
	// Enabled controls whether LlamaFirewall scanning is active.
	Enabled bool
	// SampleRate is the fraction of requests actually sent to LlamaFirewall
	// (0.0–1.0). Zero or negative is treated as 1.0 (scan everything).
	SampleRate float64
	// FailMode determines behaviour when the sidecar is unavailable:
	//   "closed" (default) — return ErrSidecarUnavailable (fail-closed).
	//   "open"             — return ResultClean silently.
	//   "warn"             — return ResultClean with a reason annotation.
	FailMode string
	// CodeShield enables code-safety scanning (ScanCode kind).
	CodeShield bool
	// AlignmentCheck enables goal-hijacking scanning (ScanAlignment kind).
	AlignmentCheck bool
	// CodeShieldAction controls what happens on a code-safety hit: "warn" or "block".
	CodeShieldAction string
	// PythonPath is the Python interpreter to use (default: "python3" on Unix,
	// "python" on Windows).
	PythonPath string
}

// Status is a snapshot of supervisor state for the CLI status command.
type Status struct {
	PID          int
	StartedAt    time.Time
	Degraded     bool
	SampleRate   float64
	FailMode     string
	P95LatencyMS int64
}

// Supervisor manages the LlamaFirewall Python sidecar process. It starts the
// sidecar, dials the IPC socket, monitors the process for unexpected exit, and
// restarts it up to MaxRetries times before entering degraded mode.
//
// Supervisor is safe for concurrent use. Only one goroutine should call Start;
// Scan may be called concurrently.
type Supervisor struct {
	// PythonPath is the Python interpreter binary to use.
	PythonPath string
	// SidecarPath is the absolute path to llamafirewall_sidecar.py.
	SidecarPath string
	// SockPath is the Unix socket path (or named pipe path on Windows).
	SockPath string
	// MaxRetries is the maximum number of restart attempts before degraded mode.
	MaxRetries int

	cfg       LlamaFirewallConfig
	mu        sync.Mutex
	proc      *os.Process
	startedAt time.Time
	retries   int
	degraded  bool
	client    *Client
	latency   LatencyTracker
}

// NewSupervisor creates a Supervisor from the given config, sock path, and
// sidecar script path. It does not start the sidecar — call Start.
func NewSupervisor(cfg LlamaFirewallConfig, sockPath, sidecarPath string) *Supervisor {
	pythonPath := cfg.PythonPath
	if pythonPath == "" {
		if runtime.GOOS == "windows" {
			pythonPath = "python"
		} else {
			pythonPath = "python3"
		}
	}
	return &Supervisor{
		PythonPath:  pythonPath,
		SidecarPath: sidecarPath,
		SockPath:    sockPath,
		MaxRetries:  3,
		cfg:         cfg,
	}
}

// Start launches the Python sidecar, waits up to 2 seconds for its socket to
// appear, dials it, persists the PID to state.json, and starts the watch goroutine
// that restarts the process on unexpected exit.
func (s *Supervisor) Start(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, s.PythonPath, s.SidecarPath)
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start llamafirewall sidecar: %w", err)
	}

	s.mu.Lock()
	s.proc = cmd.Process
	s.startedAt = time.Now()
	s.mu.Unlock()

	// Wait up to 2 s for the socket file to appear.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(s.SockPath); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if _, err := os.Stat(s.SockPath); err != nil {
		// Kill the process — socket never appeared.
		_ = cmd.Process.Kill()
		return fmt.Errorf("llamafirewall sidecar socket %s did not appear within 2s", s.SockPath)
	}

	c, err := Dial(s.SockPath, 5*time.Second)
	if err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("dial llamafirewall sidecar: %w", err)
	}

	s.mu.Lock()
	s.client = c
	s.mu.Unlock()

	s.persistState(cmd.Process.Pid)
	go s.watchProcess(ctx, cmd)
	return nil
}

// watchProcess blocks until the sidecar process exits. On unexpected exit it
// restarts the sidecar (up to MaxRetries times with exponential backoff). When
// MaxRetries is exceeded it sets degraded=true.
func (s *Supervisor) watchProcess(ctx context.Context, cmd *exec.Cmd) {
	_ = cmd.Wait() // blocks until process exits

	s.mu.Lock()
	if ctx.Err() != nil {
		// Supervised shutdown — do not restart.
		s.mu.Unlock()
		return
	}
	if s.retries >= s.MaxRetries {
		s.degraded = true
		s.mu.Unlock()
		return
	}
	s.retries++
	retries := s.retries
	s.mu.Unlock()

	backoff := math.Min(math.Pow(2, float64(retries)), 30)
	time.Sleep(time.Duration(backoff) * time.Second)

	s.relaunch(ctx)
}

// relaunch re-starts the sidecar process. On failure it increments the retry
// counter and, if MaxRetries is exceeded, sets degraded=true.
func (s *Supervisor) relaunch(ctx context.Context) {
	cmd := exec.CommandContext(ctx, s.PythonPath, s.SidecarPath)
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		s.mu.Lock()
		s.retries++
		if s.retries >= s.MaxRetries {
			s.degraded = true
		}
		s.mu.Unlock()
		return
	}

	// Wait up to 2 s for the socket file to appear.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(s.SockPath); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if _, err := os.Stat(s.SockPath); err != nil {
		_ = cmd.Process.Kill()
		s.mu.Lock()
		s.retries++
		if s.retries >= s.MaxRetries {
			s.degraded = true
		}
		s.mu.Unlock()
		return
	}

	c, err := Dial(s.SockPath, 5*time.Second)
	if err != nil {
		_ = cmd.Process.Kill()
		s.mu.Lock()
		s.retries++
		if s.retries >= s.MaxRetries {
			s.degraded = true
		}
		s.mu.Unlock()
		return
	}

	s.mu.Lock()
	s.proc = cmd.Process
	s.startedAt = time.Now()
	if s.client != nil {
		_ = s.client.Close()
	}
	s.client = c
	s.mu.Unlock()

	s.persistState(cmd.Process.Pid)
	go s.watchProcess(ctx, cmd)
}

// Scan sends req to the LlamaFirewall sidecar and returns the response.
//
// If the supervisor is degraded it returns ErrSidecarUnavailable (fail-closed)
// or ResultClean (fail-open / warn). Sample-rate gating is applied before the
// IPC call so that a configured rate < 1.0 reduces the fraction of requests
// forwarded to the sidecar.
func (s *Supervisor) Scan(ctx context.Context, req ScanRequest) (ScanResponse, error) {
	s.mu.Lock()
	degraded := s.degraded
	failMode := s.cfg.FailMode
	sampleRate := s.cfg.SampleRate
	s.mu.Unlock()

	if degraded {
		if failMode == "open" || failMode == "warn" {
			return ScanResponse{
				RequestID: req.RequestID,
				Result:    ResultClean,
				Reason:    "sidecar unavailable (fail-open)",
			}, nil
		}
		// "closed" or "" — fail closed.
		return ScanResponse{}, ErrSidecarUnavailable
	}

	if sampleRate <= 0 {
		sampleRate = 1.0
	}
	if rand.Float64() >= sampleRate {
		return ScanResponse{
			RequestID: req.RequestID,
			Result:    ResultClean,
			Reason:    "not sampled",
		}, nil
	}

	s.mu.Lock()
	c := s.client
	s.mu.Unlock()

	if c == nil {
		if failMode == "open" || failMode == "warn" {
			return ScanResponse{
				RequestID: req.RequestID,
				Result:    ResultClean,
				Reason:    "sidecar unavailable (fail-open)",
			}, nil
		}
		return ScanResponse{}, ErrSidecarUnavailable
	}

	resp, err := c.Scan(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "llamafirewall: scan error: %v\n", err)
		return ScanResponse{}, err
	}

	s.latency.Record(resp.LatencyMS)
	return resp, nil
}

// Stop sends an interrupt signal to the sidecar process and closes the client.
func (s *Supervisor) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.proc != nil {
		_ = s.proc.Signal(os.Interrupt)
	}
	if s.client != nil {
		_ = s.client.Close()
	}
	return nil
}

// IsDegraded reports whether the supervisor has exhausted its restart budget
// and the sidecar is considered permanently unavailable.
func (s *Supervisor) IsDegraded() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.degraded
}

// StatusInfo returns a snapshot of supervisor state for use by the CLI status command.
func (s *Supervisor) StatusInfo() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	pid := 0
	if s.proc != nil {
		pid = s.proc.Pid
	}
	return Status{
		PID:          pid,
		StartedAt:    s.startedAt,
		Degraded:     s.degraded,
		SampleRate:   s.cfg.SampleRate,
		FailMode:     s.cfg.FailMode,
		P95LatencyMS: s.latency.P95(),
	}
}

// persistState writes the sidecar PID and start time to ~/.beekeeper/state.json
// so that the CLI status command can report on a running sidecar without holding
// a reference to the Supervisor object.
func (s *Supervisor) persistState(pid int) {
	stateDir := os.ExpandEnv("$HOME/.beekeeper")
	statePath := filepath.Join(stateDir, "state.json")
	data, _ := os.ReadFile(statePath)
	var state map[string]any
	if json.Unmarshal(data, &state) != nil {
		state = make(map[string]any)
	}
	state["llamafirewall"] = map[string]any{
		"pid":        pid,
		"started_at": time.Now().UTC().Format(time.RFC3339),
	}
	out, _ := json.MarshalIndent(state, "", "  ")
	_ = os.WriteFile(statePath, append(out, '\n'), 0600)
}
