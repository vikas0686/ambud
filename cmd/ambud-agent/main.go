// SPDX-License-Identifier: Apache-2.0

// Command ambud-agent is the Ambud node agent: a long-running daemon
// that exposes a local HTTP API wrapping containerd, plus periodic
// host resource sampling (CPU/RAM/disk). See docs/ROADMAP.md's Phase 2
// and docs/ARCHITECTURE.md's Node Agent section.
//
// As of Phase 3, the agent optionally registers with a control plane
// (--controlplane) and heartbeats on an interval, reconciling local
// containerd state toward whatever the control plane says should run.
// --controlplane is still optional — with it unset, the agent runs
// exactly as it did in Phase 2: standalone, reachable directly by
// ambudctl, registering and heartbeating nowhere. That's a deliberate,
// still-supported mode (see docs/ARCHITECTURE.md), not a fallback path.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vikas0686/ambud/internal/agent"
	"github.com/vikas0686/ambud/internal/runtime"
	"github.com/vikas0686/ambud/internal/version"
)

// shutdownTimeout bounds how long the HTTP server waits for in-flight
// requests to finish during graceful shutdown before giving up.
const shutdownTimeout = 10 * time.Second

// config holds ambud-agent's startup flags, bundled into one struct
// once the flag count grew past a handful of positional parameters.
type config struct {
	listenAddr        string
	socketPath        string
	resourceInterval  time.Duration
	diskPath          string
	controlPlaneURL   string
	joinToken         string
	nodeName          string
	credentialsPath   string
	heartbeatInterval time.Duration
}

func main() {
	hostname, _ := os.Hostname() // empty is fine; --node-name lets an operator override either way

	var cfg config
	flag.StringVar(&cfg.listenAddr, "listen", ":8080", "address to listen on")
	flag.StringVar(&cfg.socketPath, "socket", runtime.DefaultSocketPath, "containerd socket path")
	flag.DurationVar(&cfg.resourceInterval, "resource-interval", 5*time.Second, "how often to sample host CPU/RAM/disk usage")
	flag.StringVar(&cfg.diskPath, "disk-path", "/", "filesystem path to report disk usage for")
	flag.StringVar(&cfg.controlPlaneURL, "controlplane", "", "control plane URL (omit to run standalone, as in Phase 2)")
	flag.StringVar(&cfg.joinToken, "join-token", "", "one-time join token (only used on first registration)")
	flag.StringVar(&cfg.nodeName, "node-name", hostname, "name to register as (default: hostname)")
	flag.StringVar(&cfg.credentialsPath, "credentials-path", agent.DefaultCredentialsPath, "where to persist this node's control-plane credential")
	flag.DurationVar(&cfg.heartbeatInterval, "heartbeat-interval", 5*time.Second, "how often to heartbeat the control plane")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// mainErr does the real work in its own function, rather than
	// inline here, so every deferred cleanup (closing rt, releasing the
	// signal handler) always runs before the single os.Exit below —
	// os.Exit skips pending defers in whatever function calls it.
	if err := mainErr(cfg, logger); err != nil {
		logger.Error("ambud-agent exited with error", "error", err)
		os.Exit(1)
	}
}

func mainErr(cfg config, logger *slog.Logger) error {
	rt, err := runtime.New(cfg.socketPath)
	if err != nil {
		return fmt.Errorf("connect to containerd at %s: %w", cfg.socketPath, err)
	}
	defer func() { _ = rt.Close() }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return run(ctx, rt, cfg, logger)
}

// run starts the resource collector, the HTTP server, and (if
// cfg.controlPlaneURL is set) the control-plane reconciler, and blocks
// until ctx is cancelled (graceful shutdown) or the server fails
// outright. It takes an already-connected runtime.Runtime — rather
// than a socket path to connect itself — precisely so it can be tested
// against a runtime.Fake without a live containerd daemon; see
// docs/GO_LEARNING_PATH.md #9.
func run(ctx context.Context, rt runtime.Runtime, cfg config, logger *slog.Logger) error {
	collector := agent.NewResourceCollector(cfg.resourceInterval, cfg.diskPath)
	go collector.Run(ctx)

	if cfg.controlPlaneURL != "" {
		cp := agent.NewControlPlaneClient(cfg.controlPlaneURL)
		reconciler := agent.NewReconciler(cp, rt, collector,
			cfg.credentialsPath, cfg.joinToken, cfg.nodeName, cfg.heartbeatInterval, logger)
		go func() {
			if err := reconciler.Run(ctx); err != nil {
				logger.Error("control plane reconciler stopped", "error", err)
			}
		}()
	}

	httpServer := &http.Server{
		Addr:              cfg.listenAddr,
		Handler:           agent.NewServer(rt, collector, logger),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("ambud-agent listening", "addr", cfg.listenAddr, "version", version.String(), "controlplane", cfg.controlPlaneURL)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining in-flight requests")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	case err := <-serveErr:
		return fmt.Errorf("HTTP server: %w", err)
	}
}
