// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"fmt"
	"sync"
)

// Fake is an in-memory Runtime implementation for tests that don't have
// (and shouldn't need) a live containerd socket — see
// docs/GO_LEARNING_PATH.md #9 on testing HTTP handlers and business
// logic against a fake instead of a real dependency. It's exported so
// the future node agent's reconcile-loop and HTTP handler tests (from
// Phase 2 onward) can reuse it instead of each writing their own.
type Fake struct {
	mu         sync.Mutex
	containers map[string]ContainerStatus
	pulled     map[string]bool

	// PullErr, if set, is returned by Pull instead of succeeding.
	PullErr error
}

var _ Runtime = (*Fake)(nil)

// NewFake returns an empty Fake runtime.
func NewFake() *Fake {
	return &Fake{
		containers: make(map[string]ContainerStatus),
		pulled:     make(map[string]bool),
	}
}

// Pull implements Runtime.
func (f *Fake) Pull(_ context.Context, image string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.PullErr != nil {
		return f.PullErr
	}
	f.pulled[image] = true
	return nil
}

// Run implements Runtime.
func (f *Fake) Run(ctx context.Context, name, image string, ports []PortMapping) error {
	if err := f.Pull(ctx, image); err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.containers[name]; exists {
		return fmt.Errorf("run %s: %w", name, ErrAlreadyExists)
	}
	f.containers[name] = ContainerStatus{
		Name:  name,
		Image: image,
		State: StateRunning,
		PID:   1,
		Ports: ports,
	}
	return nil
}

// Stop implements Runtime.
func (f *Fake) Stop(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.containers[name]; !exists {
		return fmt.Errorf("stop %s: %w", name, ErrNotFound)
	}
	// Matches Containerd.Stop: the container is fully removed, not just
	// marked stopped — see the Runtime.Stop doc comment ("... and
	// removes it"). A container reappearing as StateStopped in List()
	// is what a crash looks like instead (task exits on its own,
	// nobody called Stop) — see SimulateCrash for modeling that in
	// tests.
	delete(f.containers, name)
	return nil
}

// List implements Runtime.
func (f *Fake) List(_ context.Context) ([]ContainerStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	statuses := make([]ContainerStatus, 0, len(f.containers))
	for _, st := range f.containers {
		statuses = append(statuses, st)
	}
	return statuses, nil
}

// Close implements Runtime. It's a no-op: Fake holds no real resources.
func (f *Fake) Close() error {
	return nil
}

// SimulateCrash flips an existing container's state to Stopped without
// removing it — modeling a container whose process exited on its own
// (containerd keeps the record around until something calls Stop/Delete
// on it), as distinct from Stop, which removes it entirely. It's a
// no-op if name doesn't exist. Test-only: nothing in production code
// calls this — a real crash is something containerd reports, not
// something Ambud causes.
func (f *Fake) SimulateCrash(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	c, exists := f.containers[name]
	if !exists {
		return
	}
	c.State = StateStopped
	c.PID = 0
	f.containers[name] = c
}
