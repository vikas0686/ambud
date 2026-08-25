// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Workload is one deployed container's desired image/node plus the
// last state an agent heartbeat reported for it. There is no
// scheduler yet (Phase 5) — NodeID is fixed at creation time by
// whoever calls CreateWorkload.
type Workload struct {
	ID        uuid.UUID
	Name      string
	Image     string
	NodeID    uuid.UUID
	State     string
	PID       int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateWorkload records a new desired workload on nodeID, in the
// "pending" state until the node's next heartbeat picks it up and
// reports back what actually happened.
func (s *Store) CreateWorkload(ctx context.Context, name, image string, nodeID uuid.UUID) (Workload, error) {
	var w Workload
	err := s.pool.QueryRow(ctx,
		`INSERT INTO workloads (id, name, image, node_id)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, name, image, node_id, state, pid, created_at, updated_at`,
		uuid.New(), name, image, nodeID,
	).Scan(&w.ID, &w.Name, &w.Image, &w.NodeID, &w.State, &w.PID, &w.CreatedAt, &w.UpdatedAt)
	switch {
	case isUniqueViolation(err):
		return Workload{}, fmt.Errorf("create workload: name %q already taken: %w", name, ErrAlreadyExists)
	case isForeignKeyViolation(err):
		return Workload{}, fmt.Errorf("create workload: node %s: %w", nodeID, ErrNotFound)
	case err != nil:
		return Workload{}, fmt.Errorf("create workload: %w", err)
	}
	return w, nil
}

// ListWorkloads returns every workload, most recently created first.
func (s *Store) ListWorkloads(ctx context.Context) ([]Workload, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, image, node_id, state, pid, created_at, updated_at
		 FROM workloads ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list workloads: %w", err)
	}
	defer rows.Close()

	var workloads []Workload
	for rows.Next() {
		w, err := scanWorkload(rows)
		if err != nil {
			return nil, fmt.Errorf("scan workload: %w", err)
		}
		workloads = append(workloads, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list workloads: %w", err)
	}
	return workloads, nil
}

// ListWorkloadsForNode returns the workloads assigned to nodeID — the
// "desired state" a heartbeat response tells that node's agent to
// reconcile toward.
func (s *Store) ListWorkloadsForNode(ctx context.Context, nodeID uuid.UUID) ([]Workload, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, image, node_id, state, pid, created_at, updated_at
		 FROM workloads WHERE node_id = $1 ORDER BY created_at`,
		nodeID,
	)
	if err != nil {
		return nil, fmt.Errorf("list workloads for node %s: %w", nodeID, err)
	}
	defer rows.Close()

	var workloads []Workload
	for rows.Next() {
		w, err := scanWorkload(rows)
		if err != nil {
			return nil, fmt.Errorf("scan workload: %w", err)
		}
		workloads = append(workloads, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list workloads for node %s: %w", nodeID, err)
	}
	return workloads, nil
}

// UpdateWorkloadStatus records what a node's heartbeat reported for
// one of its workloads. It's scoped to nodeID as well as name — a
// heartbeat can only update the state of workloads actually assigned
// to the node sending it — and is a no-op (not an error) if no such
// workload exists, since an agent may legitimately report a container
// the control plane doesn't (yet, or ever) know about.
func (s *Store) UpdateWorkloadStatus(ctx context.Context, nodeID uuid.UUID, name, state string, pid int) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE workloads SET state = $3, pid = $4, updated_at = now()
		 WHERE node_id = $1 AND name = $2`,
		nodeID, name, state, pid,
	)
	if err != nil {
		return fmt.Errorf("update workload status for %q: %w", name, err)
	}
	return nil
}

func scanWorkload(row rowScanner) (Workload, error) {
	var w Workload
	err := row.Scan(&w.ID, &w.Name, &w.Image, &w.NodeID, &w.State, &w.PID, &w.CreatedAt, &w.UpdatedAt)
	return w, err
}
