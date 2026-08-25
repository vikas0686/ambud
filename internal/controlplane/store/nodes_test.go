// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"errors"
	"testing"
)

func TestRegisterNode(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		token, err := s.CreateJoinToken(ctx)
		if err != nil {
			t.Fatalf("CreateJoinToken() error = %v", err)
		}

		node, credential, err := s.RegisterNode(ctx, token, "node-1")
		if err != nil {
			t.Fatalf("RegisterNode() error = %v, want nil", err)
		}
		if node.Name != "node-1" || credential == "" {
			t.Errorf("RegisterNode() = (%+v, %q), want name=node-1 and a non-empty credential", node, credential)
		}

		// The credential must actually authenticate.
		authed, err := s.AuthenticateNode(ctx, credential)
		if err != nil {
			t.Fatalf("AuthenticateNode() error = %v, want nil", err)
		}
		if authed.ID != node.ID {
			t.Errorf("AuthenticateNode() = %+v, want node %s", authed, node.ID)
		}
	})

	t.Run("invalid join token", func(t *testing.T) {
		_, _, err := s.RegisterNode(ctx, "not-a-real-token", "node-x")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("RegisterNode() error = %v, want wrapping ErrNotFound", err)
		}
	})

	t.Run("join token already used", func(t *testing.T) {
		token, err := s.CreateJoinToken(ctx)
		if err != nil {
			t.Fatalf("CreateJoinToken() error = %v", err)
		}
		if _, _, registerErr := s.RegisterNode(ctx, token, "node-a"); registerErr != nil {
			t.Fatalf("first RegisterNode() error = %v", registerErr)
		}

		_, _, err = s.RegisterNode(ctx, token, "node-b")
		if !errors.Is(err, ErrAlreadyExists) {
			t.Errorf("second RegisterNode() error = %v, want wrapping ErrAlreadyExists", err)
		}
	})

	t.Run("duplicate name", func(t *testing.T) {
		token1, _ := s.CreateJoinToken(ctx)
		if _, _, err := s.RegisterNode(ctx, token1, "duplicate-name"); err != nil {
			t.Fatalf("first RegisterNode() error = %v", err)
		}

		token2, _ := s.CreateJoinToken(ctx)
		_, _, err := s.RegisterNode(ctx, token2, "duplicate-name")
		if !errors.Is(err, ErrAlreadyExists) {
			t.Errorf("RegisterNode() with duplicate name error = %v, want wrapping ErrAlreadyExists", err)
		}
	})
}

func TestAuthenticateNode_UnknownCredential(t *testing.T) {
	s := newTestStore(t)
	_, err := s.AuthenticateNode(context.Background(), "no-such-credential")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("AuthenticateNode() error = %v, want wrapping ErrNotFound", err)
	}
}

func TestUpdateNodeHeartbeat(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	token, _ := s.CreateJoinToken(ctx)
	node, _, err := s.RegisterNode(ctx, token, "node-1")
	if err != nil {
		t.Fatalf("RegisterNode() error = %v", err)
	}
	if node.LastHeartbeatAt != nil {
		t.Errorf("newly registered node LastHeartbeatAt = %v, want nil", node.LastHeartbeatAt)
	}

	err = s.UpdateNodeHeartbeat(ctx, node.ID, NodeResources{
		CPUCores: 8, CPUUsedPercent: 12.5,
		MemTotalBytes: 1000, MemUsedBytes: 400,
		DiskTotalBytes: 2000, DiskUsedBytes: 900,
	})
	if err != nil {
		t.Fatalf("UpdateNodeHeartbeat() error = %v, want nil", err)
	}

	got, err := s.GetNode(ctx, node.ID)
	if err != nil {
		t.Fatalf("GetNode() error = %v", err)
	}
	if got.CPUCores != 8 || got.MemUsedBytes != 400 || got.LastHeartbeatAt == nil {
		t.Errorf("GetNode() after heartbeat = %+v, want CPUCores=8 MemUsedBytes=400 and a non-nil LastHeartbeatAt", got)
	}
}

func TestUpdateNodeHeartbeat_UnknownNode(t *testing.T) {
	s := newTestStore(t)
	err := s.UpdateNodeHeartbeat(context.Background(), randomUUID(t), NodeResources{})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdateNodeHeartbeat() error = %v, want wrapping ErrNotFound", err)
	}
}

func TestListNodes(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	token1, _ := s.CreateJoinToken(ctx)
	if _, _, err := s.RegisterNode(ctx, token1, "node-1"); err != nil {
		t.Fatalf("RegisterNode(node-1) error = %v", err)
	}
	token2, _ := s.CreateJoinToken(ctx)
	if _, _, err := s.RegisterNode(ctx, token2, "node-2"); err != nil {
		t.Fatalf("RegisterNode(node-2) error = %v", err)
	}

	nodes, err := s.ListNodes(ctx)
	if err != nil {
		t.Fatalf("ListNodes() error = %v", err)
	}
	if len(nodes) != 2 {
		t.Errorf("ListNodes() returned %d nodes, want 2", len(nodes))
	}
}
