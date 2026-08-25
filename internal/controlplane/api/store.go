// SPDX-License-Identifier: Apache-2.0

// Package api implements the ambud-controlplane HTTP API: node
// join-token issuance, registration, and heartbeats, plus workload
// creation and listing. See docs/ROADMAP.md's Phase 3 and
// docs/ARCHITECTURE.md's Control Plane / API Server sections.
package api

import (
	"context"

	"github.com/google/uuid"

	"github.com/vikas0686/ambud/internal/controlplane/store"
)

// Store is exactly what the control-plane API handlers need from
// internal/controlplane/store — narrow on purpose (see
// docs/ARCHITECTURE.md's "every component replaceable behind its
// interface" principle) so handler tests can substitute an in-memory
// fake instead of a real Postgres connection. The real *store.Store
// satisfies this structurally; see the compile-time assertion in
// cmd/ambud-controlplane.
type Store interface {
	CreateJoinToken(ctx context.Context) (string, error)
	RegisterNode(ctx context.Context, joinToken, name string) (store.Node, string, error)
	AuthenticateNode(ctx context.Context, credentialToken string) (store.Node, error)
	GetNode(ctx context.Context, id uuid.UUID) (store.Node, error)
	ListNodes(ctx context.Context) ([]store.Node, error)
	UpdateNodeHeartbeat(ctx context.Context, nodeID uuid.UUID, r store.NodeResources) error

	CreateWorkload(ctx context.Context, name, image string, nodeID uuid.UUID) (store.Workload, error)
	ListWorkloads(ctx context.Context) ([]store.Workload, error)
	ListWorkloadsForNode(ctx context.Context, nodeID uuid.UUID) ([]store.Workload, error)
	UpdateWorkloadStatus(ctx context.Context, nodeID uuid.UUID, name, state string, pid int) error
}
