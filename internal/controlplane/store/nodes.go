// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Node is a registered machine.
type Node struct {
	ID              uuid.UUID
	Name            string
	CPUCores        int
	CPUUsedPercent  float64
	MemTotalBytes   uint64
	MemUsedBytes    uint64
	DiskTotalBytes  uint64
	DiskUsedBytes   uint64
	CreatedAt       time.Time
	LastHeartbeatAt *time.Time
	// Address is the host/IP this node last registered or heartbeated
	// from — captured server-side from the request, not self-reported
	// by the agent (see docs/ROADMAP.md's Phase 6). Empty until the
	// node's first successful registration.
	Address string
}

// NodeResources is the subset of a heartbeat that updates a node's
// resource columns.
type NodeResources struct {
	CPUCores       int
	CPUUsedPercent float64
	MemTotalBytes  uint64
	MemUsedBytes   uint64
	DiskTotalBytes uint64
	DiskUsedBytes  uint64
}

// newCredential returns a random 32-byte hex-encoded token and its
// SHA-256 hash. Only the hash is ever persisted — see the nodes table
// comment in migrations/0001_initial.up.sql.
func newCredential() (token, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate credential: %w", err)
	}
	token = hex.EncodeToString(buf)
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// RegisterNode consumes joinToken — which must be a valid, unused
// token previously returned by CreateJoinToken — and creates a new
// node named name with a freshly generated credential. It returns the
// node and the credential's plaintext token, the only time the
// plaintext is ever available; only its hash is stored, matching how
// joinToken itself was handled.
//
// Consuming the join token and creating the node happen in one
// transaction so a crash mid-registration can't leave a token marked
// used with no corresponding node, or vice versa.
func (s *Store) RegisterNode(ctx context.Context, joinToken, name, address string) (Node, string, error) {
	credToken, credHash, err := newCredential()
	if err != nil {
		return Node{}, "", err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Node{}, "", fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var usedAt *time.Time
	err = tx.QueryRow(ctx,
		`SELECT used_at FROM join_tokens WHERE token_hash = $1 FOR UPDATE`,
		hashToken(joinToken),
	).Scan(&usedAt)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Node{}, "", fmt.Errorf("register node: invalid join token: %w", ErrNotFound)
	case err != nil:
		return Node{}, "", fmt.Errorf("look up join token: %w", err)
	case usedAt != nil:
		return Node{}, "", fmt.Errorf("register node: join token already used: %w", ErrAlreadyExists)
	}

	id := uuid.New()
	var node Node
	err = tx.QueryRow(ctx,
		`INSERT INTO nodes (id, name, credential_hash, address)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, name, created_at, address`,
		id, name, credHash, address,
	).Scan(&node.ID, &node.Name, &node.CreatedAt, &node.Address)
	switch {
	case isUniqueViolation(err):
		return Node{}, "", fmt.Errorf("register node: name %q already taken: %w", name, ErrAlreadyExists)
	case err != nil:
		return Node{}, "", fmt.Errorf("insert node: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE join_tokens SET used_at = now(), used_by_node = $1 WHERE token_hash = $2`,
		id, hashToken(joinToken),
	); err != nil {
		return Node{}, "", fmt.Errorf("mark join token used: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Node{}, "", fmt.Errorf("commit transaction: %w", err)
	}
	return node, credToken, nil
}

// AuthenticateNode looks up the node whose stored credential hash
// matches credentialToken. It returns ErrNotFound for an unrecognized
// token — deliberately the same error a missing node ID would produce,
// so callers can't distinguish "bad token" from "no such node" through
// the error alone.
func (s *Store) AuthenticateNode(ctx context.Context, credentialToken string) (Node, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, name, created_at, last_heartbeat_at,
		        cpu_cores, cpu_used_percent,
		        mem_total_bytes, mem_used_bytes,
		        disk_total_bytes, disk_used_bytes, address
		 FROM nodes WHERE credential_hash = $1`,
		hashToken(credentialToken),
	)
	node, err := scanNode(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Node{}, fmt.Errorf("authenticate node: %w", ErrNotFound)
	}
	if err != nil {
		return Node{}, fmt.Errorf("authenticate node: %w", err)
	}
	return node, nil
}

// UpdateNodeHeartbeat records a fresh resource sample, refreshes the
// node's last-known address (see Node.Address), and bumps
// last_heartbeat_at to now.
func (s *Store) UpdateNodeHeartbeat(ctx context.Context, nodeID uuid.UUID, r NodeResources, address string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE nodes SET
		   cpu_cores = $2, cpu_used_percent = $3,
		   mem_total_bytes = $4, mem_used_bytes = $5,
		   disk_total_bytes = $6, disk_used_bytes = $7,
		   address = $8,
		   last_heartbeat_at = now()
		 WHERE id = $1`,
		nodeID, r.CPUCores, r.CPUUsedPercent,
		r.MemTotalBytes, r.MemUsedBytes,
		r.DiskTotalBytes, r.DiskUsedBytes,
		address,
	)
	if err != nil {
		return fmt.Errorf("update node heartbeat: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update node heartbeat: %w", ErrNotFound)
	}
	return nil
}

// ListNodes returns every registered node, most recently created
// first.
func (s *Store) ListNodes(ctx context.Context) ([]Node, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, created_at, last_heartbeat_at,
		        cpu_cores, cpu_used_percent,
		        mem_total_bytes, mem_used_bytes,
		        disk_total_bytes, disk_used_bytes, address
		 FROM nodes ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		node, err := scanNode(rows)
		if err != nil {
			return nil, fmt.Errorf("scan node: %w", err)
		}
		nodes = append(nodes, node)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	return nodes, nil
}

// GetNode returns one node by ID.
func (s *Store) GetNode(ctx context.Context, id uuid.UUID) (Node, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, name, created_at, last_heartbeat_at,
		        cpu_cores, cpu_used_percent,
		        mem_total_bytes, mem_used_bytes,
		        disk_total_bytes, disk_used_bytes, address
		 FROM nodes WHERE id = $1`,
		id,
	)
	node, err := scanNode(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Node{}, fmt.Errorf("get node %s: %w", id, ErrNotFound)
	}
	if err != nil {
		return Node{}, fmt.Errorf("get node %s: %w", id, err)
	}
	return node, nil
}

// rowScanner is satisfied by both pgx.Row (QueryRow) and pgx.Rows
// (Query, one row at a time via Next) — letting scanNode serve both
// single-row and multi-row callers without duplicating the column list.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanNode(row rowScanner) (Node, error) {
	var n Node
	err := row.Scan(
		&n.ID, &n.Name, &n.CreatedAt, &n.LastHeartbeatAt,
		&n.CPUCores, &n.CPUUsedPercent,
		&n.MemTotalBytes, &n.MemUsedBytes,
		&n.DiskTotalBytes, &n.DiskUsedBytes, &n.Address,
	)
	return n, err
}
