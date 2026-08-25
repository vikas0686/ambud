// SPDX-License-Identifier: Apache-2.0

package cpclient

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

// newTestServer starts a real internal/controlplane/api server against
// real Postgres, same pattern as internal/agent's and
// internal/apiclient's own integration tests — proving the wire
// contract end to end, not just that both sides compile against
// apitypes. Skips (doesn't fail) if no test Postgres is reachable.
func newTestServer(t *testing.T) *httptest.Server {
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
	return srv
}

func uniqueName(prefix string) string {
	return prefix + "-" + uuid.NewString()
}

func TestClient_CreateJoinTokenAndListNodes(t *testing.T) {
	srv := newTestServer(t)
	client := New(srv.URL)
	ctx := context.Background()

	token, err := client.CreateJoinToken(ctx)
	if err != nil {
		t.Fatalf("CreateJoinToken() error = %v, want nil", err)
	}
	if token == "" {
		t.Fatal("CreateJoinToken() returned an empty token")
	}

	nodesBefore, err := client.ListNodes(ctx)
	if err != nil {
		t.Fatalf("ListNodes() error = %v, want nil", err)
	}

	// Register a node directly against the store (cpclient has no
	// Register method — that's the agent's job, see internal/agent) so
	// there's something for ListNodes to find.
	registerNodeForTest(t, srv.URL, token, uniqueName("node"))

	nodesAfter, err := client.ListNodes(ctx)
	if err != nil {
		t.Fatalf("ListNodes() error = %v, want nil", err)
	}
	if len(nodesAfter) != len(nodesBefore)+1 {
		t.Errorf("ListNodes() returned %d nodes, want %d (one more than before)", len(nodesAfter), len(nodesBefore)+1)
	}
}

func TestClient_CreateAndListWorkloads(t *testing.T) {
	srv := newTestServer(t)
	client := New(srv.URL)
	ctx := context.Background()

	token, err := client.CreateJoinToken(ctx)
	if err != nil {
		t.Fatalf("CreateJoinToken() error = %v", err)
	}
	nodeID := registerNodeForTest(t, srv.URL, token, uniqueName("node"))

	name := uniqueName("web")
	created, err := client.CreateWorkload(ctx, apitypes.CreateWorkloadRequest{
		Name: name, Image: "nginx:alpine", NodeID: nodeID,
	})
	if err != nil {
		t.Fatalf("CreateWorkload() error = %v, want nil", err)
	}
	if created.Name != name || created.NodeID != nodeID {
		t.Errorf("CreateWorkload() = %+v, want name=%s node=%s", created, name, nodeID)
	}

	workloads, err := client.ListWorkloads(ctx)
	if err != nil {
		t.Fatalf("ListWorkloads() error = %v, want nil", err)
	}
	found := false
	for _, w := range workloads {
		if w.Name == name {
			found = true
		}
	}
	if !found {
		t.Errorf("ListWorkloads() = %+v, want it to contain %s", workloads, name)
	}
}

func TestClient_CreateWorkloadWithAmbiguousNodeFails(t *testing.T) {
	// These tests share one un-truncated database across packages and
	// runs (see newTestServer), so "zero nodes registered" can't be
	// asserted reliably — some other test may have already registered
	// one. "At least two nodes registered" can: forcing two more here
	// guarantees the auto-assign is ambiguous regardless of whatever
	// else is in the database.
	srv := newTestServer(t)
	client := New(srv.URL)
	ctx := context.Background()

	token1, err := client.CreateJoinToken(ctx)
	if err != nil {
		t.Fatalf("CreateJoinToken() error = %v", err)
	}
	registerNodeForTest(t, srv.URL, token1, uniqueName("node"))
	token2, err := client.CreateJoinToken(ctx)
	if err != nil {
		t.Fatalf("CreateJoinToken() error = %v", err)
	}
	registerNodeForTest(t, srv.URL, token2, uniqueName("node"))

	_, err = client.CreateWorkload(ctx, apitypes.CreateWorkloadRequest{
		Name: uniqueName("orphan"), Image: "nginx:alpine",
	})
	if err == nil {
		t.Error("CreateWorkload() error = nil, want an error when node_id is omitted and multiple nodes exist")
	}
}

// registerNodeForTest performs a raw HTTP registration (cpclient
// itself has no Register method) and returns the new node's ID.
func registerNodeForTest(t *testing.T, baseURL, joinToken, name string) string {
	t.Helper()
	client := New(baseURL)
	var resp apitypes.RegisterNodeResponse
	if err := client.doJSON(context.Background(), "POST", "/v1/nodes/register",
		apitypes.RegisterNodeRequest{JoinToken: joinToken, Name: name}, &resp); err != nil {
		t.Fatalf("register test node: %v", err)
	}
	return resp.NodeID
}
