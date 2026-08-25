// SPDX-License-Identifier: Apache-2.0

// Package runtime wraps a container runtime behind Ambud's own narrow
// interface. The only implementation today (see containerd.go) drives
// containerd directly via its native Go client — nothing outside this
// package should import containerd. See docs/ARCHITECTURE.md's
// "Container Runtime" section and docs/ADR/0001-initial-architecture.md
// for why containerd was chosen and why this interface exists: it lets
// callers (ambudctl today, the node agent's reconcile loop from Phase 2
// onward) be tested against a fake without a real containerd socket.
package runtime

import (
	"context"
	"errors"
)

// State is the lifecycle state of a container as reported by the
// runtime.
type State string

const (
	// StateRunning means the container's init process is executing.
	StateRunning State = "running"
	// StateStopped means the container ran and its init process has
	// exited, but the container record still exists.
	StateStopped State = "stopped"
	// StateUnknown means the runtime could not determine the
	// container's state.
	StateUnknown State = "unknown"
)

// ContainerStatus describes the observed state of one container.
type ContainerStatus struct {
	Name  string
	Image string
	State State
	// PID is the container's init process ID on the host, or 0 if the
	// container has no running task.
	PID uint32
}

// ErrNotFound is returned when a named container does not exist.
var ErrNotFound = errors.New("runtime: container not found")

// ErrAlreadyExists is returned by Run when a container with the given
// name already exists.
var ErrAlreadyExists = errors.New("runtime: container already exists")

// Runtime manages the lifecycle of containers on a single machine. It
// is deliberately narrow: everything an Ambud node agent needs from a
// container runtime, and nothing else.
type Runtime interface {
	// Pull fetches and unpacks image so it's ready to run, without
	// starting a container. Run calls this internally, so most callers
	// don't need to call it directly — it exists for callers that want
	// to pre-warm an image separately from running it.
	Pull(ctx context.Context, image string) error

	// Run pulls image if needed, creates a container named name from
	// it, and starts it. It returns an error wrapping ErrAlreadyExists
	// if a container with that name already exists.
	Run(ctx context.Context, name, image string) error

	// Stop signals the named container to terminate, waits for it to
	// exit (escalating to SIGKILL if it does not exit within a short
	// grace period), and removes it. It returns an error wrapping
	// ErrNotFound if no such container exists.
	Stop(ctx context.Context, name string) error

	// List returns the status of every container the runtime knows
	// about in Ambud's namespace.
	List(ctx context.Context) ([]ContainerStatus, error)

	// Close releases any resources (e.g. the containerd client
	// connection) held by the Runtime.
	Close() error
}
