// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/vikas0686/ambud/internal/controlplane/store"
)

// fakeStore is an in-memory Store for handler tests — no Postgres
// needed. It mirrors the real store's semantics closely enough for
// handler-level testing (unique names, single-use join tokens,
// credential hashing) without being a full reimplementation; the real
// semantics are proven separately by internal/controlplane/store's own
// tests against real Postgres.
type fakeStore struct {
	mu          sync.Mutex
	nodes       map[uuid.UUID]store.Node
	credentials map[uuid.UUID]string // node ID -> credential hash
	joinTokens  map[string]bool      // hash -> used
	workloads   map[uuid.UUID]store.Workload
}

var _ Store = (*fakeStore)(nil)

func newFakeStore() *fakeStore {
	return &fakeStore{
		nodes:       make(map[uuid.UUID]store.Node),
		credentials: make(map[uuid.UUID]string),
		joinTokens:  make(map[string]bool),
		workloads:   make(map[uuid.UUID]store.Workload),
	}
}

func randomToken() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func hashOf(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (f *fakeStore) CreateJoinToken(_ context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	token := randomToken()
	f.joinTokens[hashOf(token)] = false
	return token, nil
}

func (f *fakeStore) RegisterNode(_ context.Context, joinToken, name, address string) (store.Node, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	used, exists := f.joinTokens[hashOf(joinToken)]
	if !exists {
		return store.Node{}, "", fmt.Errorf("invalid join token: %w", store.ErrNotFound)
	}
	if used {
		return store.Node{}, "", fmt.Errorf("join token already used: %w", store.ErrAlreadyExists)
	}
	for _, n := range f.nodes {
		if n.Name == name {
			return store.Node{}, "", fmt.Errorf("name %q already taken: %w", name, store.ErrAlreadyExists)
		}
	}

	credential := randomToken()
	node := store.Node{ID: uuid.New(), Name: name, CreatedAt: time.Now(), Address: address}
	f.nodes[node.ID] = node
	f.joinTokens[hashOf(joinToken)] = true
	f.credentials[node.ID] = hashOf(credential)

	return node, credential, nil
}

func (f *fakeStore) AuthenticateNode(_ context.Context, credentialToken string) (store.Node, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	hash := hashOf(credentialToken)
	for id, h := range f.credentials {
		if h == hash {
			return f.nodes[id], nil
		}
	}
	return store.Node{}, fmt.Errorf("authenticate node: %w", store.ErrNotFound)
}

func (f *fakeStore) GetNode(_ context.Context, id uuid.UUID) (store.Node, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	n, ok := f.nodes[id]
	if !ok {
		return store.Node{}, fmt.Errorf("get node %s: %w", id, store.ErrNotFound)
	}
	return n, nil
}

func (f *fakeStore) ListNodes(_ context.Context) ([]store.Node, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	nodes := make([]store.Node, 0, len(f.nodes))
	for _, n := range f.nodes {
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func (f *fakeStore) UpdateNodeHeartbeat(_ context.Context, nodeID uuid.UUID, r store.NodeResources, address string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	n, ok := f.nodes[nodeID]
	if !ok {
		return fmt.Errorf("update heartbeat for %s: %w", nodeID, store.ErrNotFound)
	}
	n.CPUCores = r.CPUCores
	n.CPUUsedPercent = r.CPUUsedPercent
	n.MemTotalBytes = r.MemTotalBytes
	n.MemUsedBytes = r.MemUsedBytes
	n.DiskTotalBytes = r.DiskTotalBytes
	n.DiskUsedBytes = r.DiskUsedBytes
	n.Address = address
	now := time.Now()
	n.LastHeartbeatAt = &now
	f.nodes[nodeID] = n
	return nil
}

func (f *fakeStore) CreateWorkload(_ context.Context, name, image string, nodeID uuid.UUID, ports []store.PortMapping) (store.Workload, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.nodes[nodeID]; !ok {
		return store.Workload{}, fmt.Errorf("node %s: %w", nodeID, store.ErrNotFound)
	}
	for _, w := range f.workloads {
		if w.Name == name {
			return store.Workload{}, fmt.Errorf("name %q already taken: %w", name, store.ErrAlreadyExists)
		}
	}
	for _, p := range ports {
		for _, w := range f.workloads {
			if w.NodeID != nodeID {
				continue
			}
			for _, existing := range w.Ports {
				if existing.HostPort == p.HostPort && existing.Protocol == p.Protocol {
					return store.Workload{}, fmt.Errorf("host port %d/%s on node %s: %w", p.HostPort, p.Protocol, nodeID, store.ErrPortConflict)
				}
			}
		}
	}

	now := time.Now()
	w := store.Workload{
		ID: uuid.New(), Name: name, Image: image, NodeID: nodeID,
		State: "pending", CreatedAt: now, UpdatedAt: now, Ports: ports,
	}
	f.workloads[w.ID] = w
	return w, nil
}

func (f *fakeStore) ListWorkloads(_ context.Context) ([]store.Workload, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	workloads := make([]store.Workload, 0, len(f.workloads))
	for _, w := range f.workloads {
		workloads = append(workloads, w)
	}
	return workloads, nil
}

func (f *fakeStore) ListWorkloadsForNode(_ context.Context, nodeID uuid.UUID) ([]store.Workload, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var workloads []store.Workload
	for _, w := range f.workloads {
		if w.NodeID == nodeID {
			workloads = append(workloads, w)
		}
	}
	return workloads, nil
}

func (f *fakeStore) UpdateWorkloadStatus(_ context.Context, nodeID uuid.UUID, name, state string, pid int) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for id, w := range f.workloads {
		if w.NodeID == nodeID && w.Name == name {
			w.State = state
			w.PID = pid
			w.UpdatedAt = time.Now()
			f.workloads[id] = w
			return nil
		}
	}
	return nil // matches the real store: unknown workload is a no-op, not an error
}
