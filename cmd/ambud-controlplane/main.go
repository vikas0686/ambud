// SPDX-License-Identifier: Apache-2.0

// Command ambud-controlplane is the Ambud control plane: a REST API,
// backed by PostgreSQL, that knows about registered nodes and the
// workloads assigned to them. See docs/ROADMAP.md's Phase 3 and
// docs/ARCHITECTURE.md's Control Plane section.
//
// There is no scheduler yet (Phase 5) and no user authentication yet
// (Phase 9) — this binary trusts anything that can reach it on the
// network, same caveat as ambud-agent at this stage.
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

	"github.com/vikas0686/ambud/internal/controlplane/api"
	"github.com/vikas0686/ambud/internal/controlplane/store"
	"github.com/vikas0686/ambud/internal/version"
)

const shutdownTimeout = 10 * time.Second

func main() {
	listenAddr := flag.String("listen", ":8081", "address to listen on")
	dbDSN := flag.String("db-dsn", "postgres://ambud:devpassword@localhost:5432/ambud?sslmode=disable", "PostgreSQL connection string")
	heartbeatTimeout := flag.Duration("heartbeat-timeout", api.DefaultHeartbeatTimeout,
		"how long a node can go without heartbeating before it's reported offline")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := mainErr(*listenAddr, *dbDSN, *heartbeatTimeout, logger); err != nil {
		logger.Error("ambud-controlplane exited with error", "error", err)
		os.Exit(1)
	}
}

func mainErr(listenAddr, dbDSN string, heartbeatTimeout time.Duration, logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, dbDSN)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	return run(ctx, st, listenAddr, heartbeatTimeout, logger)
}

// run starts the HTTP server and blocks until ctx is cancelled
// (graceful shutdown) or the server fails outright. It takes an
// api.Store rather than opening one itself so it can be tested against
// a fake store — same pattern as cmd/ambud-agent's run.
func run(ctx context.Context, st api.Store, listenAddr string, heartbeatTimeout time.Duration, logger *slog.Logger) error {
	httpServer := &http.Server{
		Addr:              listenAddr,
		Handler:           api.NewServer(st, heartbeatTimeout, logger),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("ambud-controlplane listening", "addr", listenAddr, "version", version.String())
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
