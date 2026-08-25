// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/vikas0686/ambud/internal/runtime"
)

func TestRun_ReturnsOnContextCancellation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	fake := runtime.NewFake()
	cfg := config{listenAddr: "127.0.0.1:0", resourceInterval: time.Hour, diskPath: "/"}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, fake, cfg, logger)
	}()

	// No need to wait for the server to actually start listening first:
	// http.Server.Shutdown is safe to race against ListenAndServe — if
	// Shutdown runs first, the subsequent ListenAndServe call returns
	// http.ErrServerClosed immediately instead of serving.
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("run() = %v, want nil after graceful shutdown", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run() did not return within 2s of context cancellation")
	}
}

func TestRun_ReturnsErrorWhenAddressUnavailable(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	fake := runtime.NewFake()

	// Occupy a port so the HTTP server inside run() fails to bind.
	var lc net.ListenConfig
	l, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	cfg := config{listenAddr: l.Addr().String(), resourceInterval: time.Hour, diskPath: "/"}

	done := make(chan error, 1)
	go func() {
		done <- run(context.Background(), fake, cfg, logger)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Error("run() error = nil, want an error for an address already in use")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run() did not return within 2s for an unbindable address")
	}
}
