// SPDX-License-Identifier: Apache-2.0

package apiclient

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vikas0686/ambud/internal/agent"
	"github.com/vikas0686/ambud/internal/runtime"
)

// newTestAgent starts a real internal/agent HTTP server (backed by a
// runtime.Fake, not a mock of the client's own expectations) so these
// tests exercise the actual wire contract end to end — request
// encoding, routing, status codes, and error translation — not just
// that Client compiles against apitypes.
func newTestAgent(t *testing.T) (*Client, *runtime.Fake) {
	t.Helper()

	fake := runtime.NewFake()
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	collector := agent.NewResourceCollector(time.Hour, "/")
	srv := httptest.NewServer(agent.NewServer(fake, collector, logger))
	t.Cleanup(srv.Close)

	return New(srv.URL), fake
}

func TestClient_RunListStop(t *testing.T) {
	client, fake := newTestAgent(t)
	ctx := context.Background()

	if err := client.Run(ctx, "web", "nginx:alpine", nil); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	statuses, err := client.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if len(statuses) != 1 || statuses[0].Name != "web" || statuses[0].Image != "nginx:alpine" {
		t.Errorf("List() = %+v, want one container named web", statuses)
	}
	if statuses[0].State != runtime.StateRunning {
		t.Errorf("List()[0].State = %q, want %q", statuses[0].State, runtime.StateRunning)
	}

	if err := client.Stop(ctx, "web"); err != nil {
		t.Fatalf("Stop() error = %v, want nil", err)
	}

	// Cross-check directly against the fake, not just through the
	// client, to confirm the HTTP round trip actually reached it.
	fakeStatuses, _ := fake.List(ctx)
	if len(fakeStatuses) != 0 {
		t.Errorf("fake.List() after Stop = %+v, want empty", fakeStatuses)
	}
}

func TestClient_RunDuplicateNameReturnsAlreadyExists(t *testing.T) {
	client, _ := newTestAgent(t)
	ctx := context.Background()

	if err := client.Run(ctx, "web", "nginx:alpine", nil); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}

	err := client.Run(ctx, "web", "nginx:alpine", nil)
	if !errors.Is(err, runtime.ErrAlreadyExists) {
		t.Errorf("second Run() error = %v, want wrapping runtime.ErrAlreadyExists", err)
	}
}

func TestClient_StopUnknownContainerReturnsNotFound(t *testing.T) {
	client, _ := newTestAgent(t)

	err := client.Stop(context.Background(), "does-not-exist")
	if !errors.Is(err, runtime.ErrNotFound) {
		t.Errorf("Stop() error = %v, want wrapping runtime.ErrNotFound", err)
	}
}

func TestClient_Pull(t *testing.T) {
	client, _ := newTestAgent(t)

	if err := client.Pull(context.Background(), "nginx:alpine"); err != nil {
		t.Errorf("Pull() error = %v, want nil", err)
	}
}

func TestClient_ListEmptyReturnsEmptyNotNilError(t *testing.T) {
	client, _ := newTestAgent(t)

	statuses, err := client.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if len(statuses) != 0 {
		t.Errorf("List() = %+v, want empty", statuses)
	}
}
