// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/vikas0686/ambud/internal/controlplane/store"
)

// fakeStore is a minimal api.Store implementation — just enough for
// run()'s lifecycle tests below, which don't exercise any actual
// endpoint logic (that's internal/controlplane/api's own test suite).
type fakeStore struct{}

func (fakeStore) CreateJoinToken(context.Context) (string, error) { return "", nil }
func (fakeStore) RegisterNode(context.Context, string, string, string) (store.Node, string, error) {
	return store.Node{}, "", nil
}
func (fakeStore) AuthenticateNode(context.Context, string) (store.Node, error) {
	return store.Node{}, nil
}
func (fakeStore) GetNode(context.Context, uuid.UUID) (store.Node, error) { return store.Node{}, nil }
func (fakeStore) ListNodes(context.Context) ([]store.Node, error)        { return nil, nil }
func (fakeStore) UpdateNodeHeartbeat(context.Context, uuid.UUID, store.NodeResources, string) error {
	return nil
}
func (fakeStore) CreateWorkload(context.Context, string, string, uuid.UUID, []store.PortMapping) (store.Workload, error) {
	return store.Workload{}, nil
}
func (fakeStore) ListWorkloads(context.Context) ([]store.Workload, error) { return nil, nil }
func (fakeStore) ListWorkloadsForNode(context.Context, uuid.UUID) ([]store.Workload, error) {
	return nil, nil
}
func (fakeStore) UpdateWorkloadStatus(context.Context, uuid.UUID, string, string, int) error {
	return nil
}

func TestRun_ReturnsOnContextCancellation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, fakeStore{}, "127.0.0.1:0", time.Second, logger)
	}()

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

	var lc net.ListenConfig
	l, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	done := make(chan error, 1)
	go func() {
		done <- run(context.Background(), fakeStore{}, l.Addr().String(), time.Second, logger)
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
