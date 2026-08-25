// SPDX-License-Identifier: Apache-2.0

// TestControlPlaneClient_ against a real internal/controlplane/api
// server backed by real Postgres — the deepest validation available
// for the agent<->control-plane wire contract, one level beyond
// reconciler_test.go's fakeControlPlane. Skips (doesn't fail) if no
// test Postgres is reachable; see internal/controlplane/store's own
// tests for how to start one locally.
package agent

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/vikas0686/ambud/internal/apitypes"
	"github.com/vikas0686/ambud/internal/controlplane/api"
	"github.com/vikas0686/ambud/internal/controlplane/store"
)

const defaultTestDSN = "postgres://ambud:devpassword@localhost:15432/ambud_test?sslmode=disable"

// newTestControlPlaneServer starts a real internal/controlplane/api
// server against real Postgres. It doesn't truncate tables (store.Store
// exposes no such thing outside its own package, deliberately — that's
// a test-only concern) so callers must use unique names, e.g. via
// uniqueName below, rather than assuming a clean slate.
func newTestControlPlaneServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()

	dsn := os.Getenv("AMBUD_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = defaultTestDSN
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	st, err := store.Open(ctx, dsn)
	if err != nil {
		t.Skipf("skipping: no test Postgres reachable at %s: %v", dsn, err)
	}
	t.Cleanup(st.Close)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	srv := httptest.NewServer(api.NewServer(st, logger))
	t.Cleanup(srv.Close)

	return srv, st
}

// uniqueName returns prefix plus a random suffix, so tests sharing an
// un-truncated database don't collide with each other or with previous
// runs.
func uniqueName(t *testing.T, prefix string) string {
	t.Helper()
	return prefix + "-" + uuid.NewString()
}

func TestControlPlaneClient_RegisterAndHeartbeat(t *testing.T) {
	srv, st := newTestControlPlaneServer(t)
	client := NewControlPlaneClient(srv.URL)
	ctx := context.Background()

	token, err := st.CreateJoinToken(ctx)
	if err != nil {
		t.Fatalf("CreateJoinToken() error = %v", err)
	}

	nodeID, credential, err := client.Register(ctx, token, uniqueName(t, "node"))
	if err != nil {
		t.Fatalf("Register() error = %v, want nil", err)
	}
	if nodeID == "" || credential == "" {
		t.Fatalf("Register() = (%q, %q), want both non-empty", nodeID, credential)
	}

	resp, err := client.Heartbeat(ctx, nodeID, credential, apitypes.HeartbeatRequest{
		Resources: apitypes.Resources{CPUCores: 4},
	})
	if err != nil {
		t.Fatalf("Heartbeat() error = %v, want nil", err)
	}
	if len(resp.Workloads) != 0 {
		t.Errorf("Heartbeat() workloads = %+v, want none (nothing deployed yet)", resp.Workloads)
	}
}

func TestControlPlaneClient_HeartbeatWithBadCredentialFails(t *testing.T) {
	srv, _ := newTestControlPlaneServer(t)
	client := NewControlPlaneClient(srv.URL)

	_, err := client.Heartbeat(context.Background(), "any-node-id", "wrong-credential", apitypes.HeartbeatRequest{})
	if err == nil {
		t.Error("Heartbeat() error = nil, want an error for an invalid credential")
	}
}
