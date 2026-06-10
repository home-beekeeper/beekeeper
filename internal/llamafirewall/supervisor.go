// Package llamafirewall implements the LlamaFirewall sidecar supervisor.
// The supervisor manages the Python sidecar process lifecycle: starting it,
// dialling a loopback TCP socket (one transport on every OS), health-monitoring
// via process wait, and restarting up to MaxRetries times with exponential
// backoff before entering degraded mode.
package llamafirewall

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/bantuson/beekeeper/internal/platform"
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

	// port is the loopback TCP port the sidecar binds; token is the per-launch
	// bearer token. Both are chosen once in Start and reused across relaunches.
	port  int
	token string
}

// NewSupervisor creates a Supervisor from the given config and sidecar script
// path. It does not start the sidecar — call Start. The loopback port and bearer
// token are chosen at Start time, not here.
func NewSupervisor(cfg LlamaFirewallConfig, sidecarPath string) *Supervisor {
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
		MaxRetries:  3,
		cfg:         cfg,
	}
}

// addr returns the loopback dial address for the sidecar (127.0.0.1:<port>).
func (s *Supervisor) addr() string {
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(s.port))
}

// childEnv returns the sidecar's environment: the parent environment plus the
// loopback port + per-launch bearer token (the sidecar binds the port and
// authenticates each request against the token) and a pinned HF_HOME so the
// gated model cache lives under the StateDir, not the user's default ~/.cache.
func (s *Supervisor) childEnv() []string {
	env := append(os.Environ(),
		"BEEKEEPER_LLMF_PORT="+strconv.Itoa(s.port),
		"BEEKEEPER_LLMF_TOKEN="+s.token,
	)
	if stateDir, err := platform.StateDir(); err == nil {
		env = append(env, "HF_HOME="+filepath.Join(stateDir, "llamafirewall", "hf"))
	}
	return env
}

// waitReady polls the loopback port until a TCP connection succeeds or the
// deadline passes. This replaces the old os.Stat(socketFile) readiness probe,
// which (a) has no meaning for a TCP socket and (b) raced the sidecar's listen()
// — a successful dial is the only true readiness signal.
func (s *Supervisor) waitReady(deadline time.Time) bool {
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", s.addr(), 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// pickFreeLoopbackPort asks the kernel for an unused 127.0.0.1 port by binding
// :0 and immediately closing. The sidecar (which sets SO_REUSEADDR) binds it
// microseconds later; the brief gap is an accepted TOCTOU on loopback only.
func pickFreeLoopbackPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port, nil
}

// newBearerToken returns a 256-bit cryptographically-random hex token.
func newBearerToken() (string, error) {
	b := make([]byte, 32)
	if _, err := crand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Start chooses a loopback port + bearer token, launches the Python sidecar with
// them (plus HF_HOME) injected into its environment, waits up to 2 seconds for
// the sidecar to accept a TCP connection, dials it, persists the PID/port/token
// to state.json, and starts the watch goroutine that restarts the process on
// unexpected exit.
func (s *Supervisor) Start(ctx context.Context) error {
	port, err := pickFreeLoopbackPort()
	if err != nil {
		return fmt.Errorf("pick llamafirewall loopback port: %w", err)
	}
	token, err := newBearerToken()
	if err != nil {
		return fmt.Errorf("generate llamafirewall bearer token: %w", err)
	}
	s.mu.Lock()
	s.port = port
	s.token = token
	s.mu.Unlock()

	cmd := exec.CommandContext(ctx, s.PythonPath, s.SidecarPath)
	cmd.Env = s.childEnv()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start llamafirewall sidecar: %w", err)
	}

	s.mu.Lock()
	s.proc = cmd.Process
	s.startedAt = time.Now()
	s.mu.Unlock()

	if !s.waitReady(time.Now().Add(2 * time.Second)) {
		// Kill the process — it never started listening.
		_ = cmd.Process.Kill()
		return fmt.Errorf("llamafirewall sidecar did not listen on %s within 2s", s.addr())
	}

	c, err := Dial(s.addr(), s.token, 5*time.Second)
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

// bumpRetry increments the restart counter and trips degraded mode once the
// retry budget is exhausted. Called on every relaunch failure path.
func (s *Supervisor) bumpRetry() {
	s.mu.Lock()
	s.retries++
	if s.retries >= s.MaxRetries {
		s.degraded = true
	}
	s.mu.Unlock()
}

// relaunch re-starts the sidecar process on the same loopback port + token. On
// failure it increments the retry counter and, if MaxRetries is exceeded, sets
// degraded=true.
func (s *Supervisor) relaunch(ctx context.Context) {
	cmd := exec.CommandContext(ctx, s.PythonPath, s.SidecarPath)
	cmd.Env = s.childEnv()

	if err := cmd.Start(); err != nil {
		s.bumpRetry()
		return
	}

	if !s.waitReady(time.Now().Add(2 * time.Second)) {
		_ = cmd.Process.Kill()
		s.bumpRetry()
		return
	}

	c, err := Dial(s.addr(), s.token, 5*time.Second)
	if err != nil {
		_ = cmd.Process.Kill()
		s.bumpRetry()
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

	// Fail-closed (Phase 20, LLMF): a sidecar that returns the error sentinel (a
	// caught Python exception, a missing gated model, an import failure) must
	// NEVER be treated as clean — that was the old silent fail-open. Apply the
	// configured fail-mode: closed => surface an error (block); open/warn => clean.
	if resp.Result == ResultError || resp.Error != "" {
		if failMode == "open" || failMode == "warn" {
			return ScanResponse{
				RequestID: req.RequestID,
				Result:    ResultClean,
				Reason:    "sidecar error (fail-open): " + resp.Error,
			}, nil
		}
		return ScanResponse{}, fmt.Errorf("%w: %s", ErrSidecarUnavailable, resp.Error)
	}

	// Record into the per-instance tracker (for StatusInfo / supervisor-local use)
	// and into the package-level GlobalLatencyTracker so CollectDiag (beekeeper diag)
	// reports a real sidecar p95 after production sidecar calls. Closes INT-BLOCK-4.
	s.latency.Record(resp.LatencyMS)
	GlobalLatencyTracker.Record(resp.LatencyMS)
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

// persistState writes the sidecar PID and start time to the platform state
// directory (honoring %APPDATA% on Windows and BEEKEEPER_HOME on all platforms)
// so that the CLI status command can report on a running sidecar without holding
// a reference to the Supervisor object.
func (s *Supervisor) persistState(pid int) {
	stateDir, err := platform.StateDir()
	if err != nil {
		return // cannot determine state dir — silently skip; informational state only
	}
	statePath := filepath.Join(stateDir, "state.json")
	data, _ := os.ReadFile(statePath)
	var state map[string]any
	if json.Unmarshal(data, &state) != nil {
		state = make(map[string]any)
	}
	state["llamafirewall"] = map[string]any{
		"pid":        pid,
		"started_at": time.Now().UTC().Format(time.RFC3339),
		"port":       s.port,
		"token":      s.token,
	}
	out, _ := json.MarshalIndent(state, "", "  ")
	_ = os.WriteFile(statePath, append(out, '\n'), 0600)
}
