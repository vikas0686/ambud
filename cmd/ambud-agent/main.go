// SPDX-License-Identifier: Apache-2.0

// Command ambud-agent is the Ambud node agent: a long-running daemon
// that exposes a local HTTP API wrapping containerd, plus periodic
// host resource sampling (CPU/RAM/disk). See docs/ROADMAP.md's Phase 2
// and docs/ARCHITECTURE.md's Node Agent section.
//
// There is no control plane yet (Phase 3) — this agent doesn't
// register or heartbeat anywhere. It's a standalone service, reachable
// directly by ambudctl over HTTP instead of Phase 1's direct
// containerd socket calls.
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

func main() {
	listenAddr := flag.String("listen", ":8080", "address to listen on")
	socketPath := flag.String("socket", runtime.DefaultSocketPath, "containerd socket path")
	resourceInterval := flag.Duration("resource-interval", 5*time.Second, "how often to sample host CPU/RAM/disk usage")
	diskPath := flag.String("disk-path", "/", "filesystem path to report disk usage for")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// mainErr does the real work in its own function, rather than
	// inline here, so every deferred cleanup (closing rt, releasing the
	// signal handler) always runs before the single os.Exit below —
	// os.Exit skips pending defers in whatever function calls it.
	if err := mainErr(*listenAddr, *socketPath, *resourceInterval, *diskPath, logger); err != nil {
		logger.Error("ambud-agent exited with error", "error", err)
		os.Exit(1)
	}
}

func mainErr(listenAddr, socketPath string, resourceInterval time.Duration, diskPath string, logger *slog.Logger) error {
	rt, err := runtime.New(socketPath)
	if err != nil {
		return fmt.Errorf("connect to containerd at %s: %w", socketPath, err)
	}
	defer func() { _ = rt.Close() }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return run(ctx, rt, listenAddr, resourceInterval, diskPath, logger)
}

// run starts the resource collector and HTTP server and blocks until
// ctx is cancelled (graceful shutdown) or the server fails outright.
// It takes an already-connected runtime.Runtime — rather than a
// socket path to connect itself — precisely so it can be tested
// against a runtime.Fake without a live containerd daemon; see
// docs/GO_LEARNING_PATH.md #9.
func run(ctx context.Context, rt runtime.Runtime, listenAddr string, resourceInterval time.Duration, diskPath string, logger *slog.Logger) error {
	collector := agent.NewResourceCollector(resourceInterval, diskPath)
	go collector.Run(ctx)

	httpServer := &http.Server{
		Addr:              listenAddr,
		Handler:           agent.NewServer(rt, collector, logger),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("ambud-agent listening", "addr", listenAddr, "version", version.String())
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
