package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/home-beekeeper/beekeeper/internal/catalog"
	"github.com/home-beekeeper/beekeeper/internal/policy"
)

const (
	defaultPort     = 7837
	defaultBindAddr = "127.0.0.1"
)

// IsLoopbackAddr reports whether addr is a loopback address that is safe to
// bind without the --allow-remote opt-in. The following are considered loopback:
//   - empty string (Start will substitute defaultBindAddr = "127.0.0.1")
//   - "localhost" (DNS name that conventionally resolves to loopback)
//   - any address in 127.0.0.0/8 (IPv4 loopback range)
//   - "::1" (IPv6 loopback)
//
// Everything else — "0.0.0.0", "::", a LAN IP, an external hostname — is
// treated as non-loopback and requires AllowRemote: true (TM-A-01).
func IsLoopbackAddr(addr string) bool {
	if addr == "" || strings.EqualFold(addr, "localhost") {
		return true
	}
	ip := net.ParseIP(addr)
	if ip == nil {
		// Unresolvable or hostname other than "localhost" — treat as non-loopback.
		return false
	}
	return ip.IsLoopback()
}

// Server holds the gateway configuration for the MCP proxy daemon.
type Server struct {
	cfg Config
}

// New constructs a Server with the given configuration.
func New(cfg Config) *Server {
	return &Server{cfg: cfg}
}

// Start runs the MCP gateway daemon. It:
//  1. Generates a per-session 64-char hex token from crypto/rand.
//  2. Opens the Bumblebee catalog index (mmap, never cold-loaded per request).
//  3. Writes the token, port, and PID to state.json (0o600 permissions).
//  4. Binds a TCP listener on BindAddr:Port (defaults: 127.0.0.1:7837).
//     If the port is busy, Start returns an error; callers may retry with :0.
//  5. Serves HTTP requests until ctx is cancelled.
//  6. On shutdown: clears the gateway state from state.json.
//
// Start blocks until ctx is cancelled or the server encounters a fatal error.
// A clean shutdown (ctx cancellation) returns nil. Only genuine errors return
// non-nil.
//
// Fail-closed contract: the per-request handler (gatewayHandler.ServeHTTP)
// enforces the fail-closed invariant independently; Start's responsibility is
// lifecycle, token generation, and state persistence.
func Start(ctx context.Context, cfg Config) error {
	// Apply defaults.
	if cfg.BindAddr == "" {
		cfg.BindAddr = defaultBindAddr
	}
	if cfg.Port == 0 {
		cfg.Port = defaultPort
	}

	// TM-A-01: refuse to bind a non-loopback address unless the operator has
	// explicitly opted in with AllowRemote: true (--allow-remote CLI flag).
	// The gateway is plain HTTP — binding 0.0.0.0 or any LAN address sends the
	// bearer token in cleartext across the network. Fail closed.
	if !IsLoopbackAddr(cfg.BindAddr) {
		if !cfg.AllowRemote {
			return fmt.Errorf(
				"beekeeper gateway: refusing to bind non-loopback address %q — "+
					"the gateway is plain HTTP; binding a non-loopback address "+
					"would expose the bearer token in cleartext over the network. "+
					"To opt in, re-run with --allow-remote and place the gateway "+
					"behind a TLS-terminating reverse proxy (TM-A-01)",
				cfg.BindAddr,
			)
		}
		// Opt-in set: proceed, but warn loudly about cleartext exposure.
		fmt.Fprintf(os.Stderr,
			"WARNING: beekeeper gateway is bound to %s (non-loopback) with plain HTTP — "+
				"the bearer token is transmitted in cleartext. "+
				"Place this gateway behind a TLS-terminating reverse proxy.\n",
			cfg.BindAddr,
		)
	}

	// Step 1: generate per-session token (T-04-03-01: never in args/config.json).
	token, err := generateToken()
	if err != nil {
		return fmt.Errorf("generate gateway token: %w", err)
	}

	// Step 2: open catalog index (mmap — never cold-load per request).
	bbIdx, err := catalog.OpenIndex(cfg.IndexPath)
	if err != nil {
		return fmt.Errorf("open catalog index %q: %w", cfg.IndexPath, err)
	}
	defer bbIdx.Close()

	// Build the OSV adapter. Uses a background context with per-request
	// sub-contexts applied via the gatewayHandler's policy evaluation goroutine.
	// On OSV error, LookupAll returns nil — source degrades to no-match.
	httpClient := &http.Client{Timeout: 4 * time.Second}
	var osvAdapter policy.MultiCatalogLookup = &catalog.OSVAdapter{
		Client:   httpClient,
		CacheDir: cfg.CacheDir,
		Ctx:      context.Background(), // handler goroutine uses its own deadline
	}

	// Build the Socket adapter. Empty token → Socket disabled (not an error).
	var socketAdapter policy.MultiCatalogLookup
	if cfg.SocketToken != "" {
		socketAdapter = catalog.SocketAdapter{
			Client:   httpClient,
			CacheDir: cfg.CacheDir,
			Token:    cfg.SocketToken,
			Ctx:      context.Background(),
		}
	}

	// Aggregate all three sources into a MultiIndex. Mirrors handler.go:194-213.
	// Nil adapters are skipped. Closes INT-BLOCK-3: gateway now performs
	// multi-source corroboration identical to the hook handler.
	multiIdx := catalog.NewMultiIndex(bbIdx, osvAdapter, socketAdapter)

	// Step 3: write gateway state (token + port + PID) to state.json (0o600).
	st := GatewayState{
		GatewayToken: token,
		BoundAddr:    cfg.BindAddr,
		BoundPort:    cfg.Port,
		StartedAt:    time.Now().UTC().Format(time.RFC3339),
		PID:          os.Getpid(),
	}
	if err := SaveGatewayState(cfg.StateFile, st); err != nil {
		return fmt.Errorf("save gateway state: %w", err)
	}

	// Step 4: bind TCP listener on BindAddr:Port (T-04-03-08: default is 127.0.0.1).
	addr := fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		// Clean up state on bind failure.
		_ = ClearGatewayState(cfg.StateFile)
		return fmt.Errorf("bind gateway at %s: %w", addr, err)
	}
	defer listener.Close()

	// Update state with the actual bound port (relevant when Port was 0 for random).
	actualPort := listener.Addr().(*net.TCPAddr).Port
	if actualPort != cfg.Port {
		st.BoundPort = actualPort
		_ = SaveGatewayState(cfg.StateFile, st)
	}

	// Step 5: create HTTP server with the gateway handler and standard timeouts.
	handler := newGatewayHandler(cfg, token, multiIdx)
	srv := &http.Server{
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 35 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start the periodic drift check scheduler (§10-15, Open Q2 resolved).
	// NEVER on the request path — runs in a dedicated goroutine.
	startDriftScheduler(ctx, handler)

	// Step 6: graceful shutdown goroutine listening for context cancellation.
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			fmt.Fprintf(os.Stderr, "beekeeper gateway: shutdown error: %v\n", err)
		}
	}()

	// Serve blocks until shutdown.
	if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("gateway serve error: %w", err)
	}

	// Step 7: clean shutdown — clear gateway state from state.json.
	_ = ClearGatewayState(cfg.StateFile)

	return nil
}

// generateToken creates a 64-char hex token from 32 random bytes (256 bits of
// entropy from crypto/rand). This matches the newRecordID pattern in
// internal/check/handler.go (T-04-03-01: same approach, different use).
func generateToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("crypto/rand failed: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}
