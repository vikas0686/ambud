// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"errors"
	"testing"
)

func registerTestNode(t *testing.T, s *Store, name string) Node {
	t.Helper()
	ctx := context.Background()
	token, err := s.CreateJoinToken(ctx)
	if err != nil {
		t.Fatalf("CreateJoinToken() error = %v", err)
	}
	node, _, err := s.RegisterNode(ctx, token, name)
	if err != nil {
		t.Fatalf("RegisterNode() error = %v", err)
	}
	return node
}

func TestCreateWorkload(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	node := registerTestNode(t, s, "node-1")

	t.Run("success", func(t *testing.T) {
		w, err := s.CreateWorkload(ctx, "web", "nginx:alpine", node.ID)
		if err != nil {
			t.Fatalf("CreateWorkload() error = %v, want nil", err)
		}
		if w.Name != "web" || w.NodeID != node.ID || w.State != "pending" {
			t.Errorf("CreateWorkload() = %+v, want name=web state=pending node=%s", w, node.ID)
		}
	})

	t.Run("duplicate name", func(t *testing.T) {
		_, err := s.CreateWorkload(ctx, "web", "nginx:alpine", node.ID)
		if !errors.Is(err, ErrAlreadyExists) {
			t.Errorf("CreateWorkload() error = %v, want wrapping ErrAlreadyExists", err)
		}
	})

	t.Run("unknown node", func(t *testing.T) {
		_, err := s.CreateWorkload(ctx, "orphan", "nginx:alpine", randomUUID(t))
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("CreateWorkload() error = %v, want wrapping ErrNotFound", err)
		}
	})
}

func TestListWorkloadsForNode(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	node1 := registerTestNode(t, s, "node-1")
	node2 := registerTestNode(t, s, "node-2")

	if _, err := s.CreateWorkload(ctx, "on-node-1", "nginx:alpine", node1.ID); err != nil {
		t.Fatalf("CreateWorkload() error = %v", err)
	}
	if _, err := s.CreateWorkload(ctx, "on-node-2", "redis:alpine", node2.ID); err != nil {
		t.Fatalf("CreateWorkload() error = %v", err)
	}

	got, err := s.ListWorkloadsForNode(ctx, node1.ID)
	if err != nil {
		t.Fatalf("ListWorkloadsForNode() error = %v", err)
	}
	if len(got) != 1 || got[0].Name != "on-node-1" {
		t.Errorf("ListWorkloadsForNode(node1) = %+v, want just on-node-1", got)
	}
}

func TestUpdateWorkloadStatus(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	node := registerTestNode(t, s, "node-1")
	w, err := s.CreateWorkload(ctx, "web", "nginx:alpine", node.ID)
	if err != nil {
		t.Fatalf("CreateWorkload() error = %v", err)
	}

	if updateErr := s.UpdateWorkloadStatus(ctx, node.ID, w.Name, "running", 4242); updateErr != nil {
		t.Fatalf("UpdateWorkloadStatus() error = %v, want nil", updateErr)
	}

	workloads, err := s.ListWorkloads(ctx)
	if err != nil {
		t.Fatalf("ListWorkloads() error = %v", err)
	}
	if len(workloads) != 1 || workloads[0].State != "running" || workloads[0].PID != 4242 {
		t.Errorf("ListWorkloads() = %+v, want web running with pid 4242", workloads)
	}
}

func TestUpdateWorkloadStatus_UnknownWorkloadIsNoOp(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	node := registerTestNode(t, s, "node-1")

	if err := s.UpdateWorkloadStatus(ctx, node.ID, "no-such-workload", "running", 1); err != nil {
		t.Errorf("UpdateWorkloadStatus() for unknown workload error = %v, want nil (no-op)", err)
	}
}

func TestUpdateWorkloadStatus_ScopedToNode(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	node1 := registerTestNode(t, s, "node-1")
	node2 := registerTestNode(t, s, "node-2")
	w, err := s.CreateWorkload(ctx, "web", "nginx:alpine", node1.ID)
	if err != nil {
		t.Fatalf("CreateWorkload() error = %v", err)
	}

	// node2 reporting a container named "web" must not touch the
	// workload actually assigned to node1.
	if updateErr := s.UpdateWorkloadStatus(ctx, node2.ID, w.Name, "running", 999); updateErr != nil {
		t.Fatalf("UpdateWorkloadStatus() error = %v", updateErr)
	}

	workloads, err := s.ListWorkloadsForNode(ctx, node1.ID)
	if err != nil {
		t.Fatalf("ListWorkloadsForNode() error = %v", err)
	}
	if len(workloads) != 1 || workloads[0].State != "pending" {
		t.Errorf("node1's workload = %+v, want still pending (unaffected by node2's report)", workloads)
	}
}
