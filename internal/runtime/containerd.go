// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"
)

const (
	// ambudNamespace isolates Ambud's containers from anything else
	// running on the same containerd daemon (nerdctl, Kubernetes CRI,
	// etc.) — see docs/ARCHITECTURE.md.
	ambudNamespace = "ambud"

	// stopGracePeriod is how long Stop waits for a container to exit
	// after SIGTERM before escalating to SIGKILL.
	stopGracePeriod = 10 * time.Second

	// DefaultSocketPath is containerd's default local Unix socket.
	DefaultSocketPath = "/run/containerd/containerd.sock"

	// defaultLogDir is where container stdout/stderr is written, since
	// Run detaches rather than attaching to the caller's own stdio.
	defaultLogDir = "/var/log/ambud/containers"
)

// Containerd is a Runtime backed by containerd's native Go client
// (not the CRI shim — see docs/ADR/0001-initial-architecture.md for
// why). It satisfies the Runtime interface defined in runtime.go.
type Containerd struct {
	client *containerd.Client
	logDir string
}

var _ Runtime = (*Containerd)(nil)

// New connects to containerd over its local Unix socket at socketPath.
func New(socketPath string) (*Containerd, error) {
	client, err := containerd.New(socketPath)
	if err != nil {
		return nil, fmt.Errorf("connect to containerd at %s: %w", socketPath, err)
	}
	return &Containerd{client: client, logDir: defaultLogDir}, nil
}

// Close releases the underlying containerd client connection.
func (c *Containerd) Close() error {
	return c.client.Close()
}

func (c *Containerd) namespaced(ctx context.Context) context.Context {
	return namespaces.WithNamespace(ctx, ambudNamespace)
}

// Pull implements Runtime.
func (c *Containerd) Pull(ctx context.Context, image string) error {
	ctx = c.namespaced(ctx)

	if _, err := c.client.Pull(ctx, image, containerd.WithPullUnpack); err != nil {
		return fmt.Errorf("pull image %s: %w", image, err)
	}
	return nil
}

// Run implements Runtime.
func (c *Containerd) Run(ctx context.Context, name, image string) error {
	ctx = c.namespaced(ctx)

	img, err := c.client.Pull(ctx, image, containerd.WithPullUnpack)
	if err != nil {
		return fmt.Errorf("pull image %s: %w", image, err)
	}

	container, err := c.client.NewContainer(
		ctx,
		name,
		containerd.WithNewSnapshot(name+"-snapshot", img),
		containerd.WithNewSpec(oci.WithImageConfig(img)),
	)
	if err != nil {
		if errdefs.IsAlreadyExists(err) {
			return fmt.Errorf("create container %s: %w: %w", name, ErrAlreadyExists, err)
		}
		return fmt.Errorf("create container %s: %w", name, err)
	}

	if mkdirErr := os.MkdirAll(c.logDir, 0o750); mkdirErr != nil {
		_ = container.Delete(ctx, containerd.WithSnapshotCleanup)
		return fmt.Errorf("create log directory %s: %w", c.logDir, mkdirErr)
	}
	logPath := filepath.Join(c.logDir, name+".log")

	task, err := container.NewTask(ctx, cio.LogFile(logPath))
	if err != nil {
		_ = container.Delete(ctx, containerd.WithSnapshotCleanup)
		return fmt.Errorf("create task for %s: %w", name, err)
	}

	if err := task.Start(ctx); err != nil {
		_, _ = task.Delete(ctx)
		_ = container.Delete(ctx, containerd.WithSnapshotCleanup)
		return fmt.Errorf("start task for %s: %w", name, err)
	}

	return nil
}

// Stop implements Runtime.
func (c *Containerd) Stop(ctx context.Context, name string) error {
	ctx = c.namespaced(ctx)

	container, err := c.client.LoadContainer(ctx, name)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return fmt.Errorf("stop %s: %w: %w", name, ErrNotFound, err)
		}
		return fmt.Errorf("load container %s: %w", name, err)
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			// Container exists but has no running task: nothing to
			// stop, just remove the (already-stopped) container record.
			return containerDelete(ctx, container)
		}
		return fmt.Errorf("load task for %s: %w", name, err)
	}

	if err := stopTask(ctx, task); err != nil {
		return fmt.Errorf("stop task for %s: %w", name, err)
	}

	return containerDelete(ctx, container)
}

// stopTask signals a task to terminate and waits for it to exit,
// escalating to SIGKILL if it doesn't exit within stopGracePeriod. A
// task that has already exited is treated as success, not an error.
func stopTask(ctx context.Context, task containerd.Task) error {
	status, err := task.Status(ctx)
	if err != nil {
		return fmt.Errorf("get task status: %w", err)
	}
	if status.Status == containerd.Stopped {
		_, deleteErr := task.Delete(ctx)
		return deleteErr
	}

	exitCh, err := task.Wait(ctx)
	if err != nil {
		return fmt.Errorf("wait on task: %w", err)
	}

	if err := task.Kill(ctx, syscall.SIGTERM); err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("send SIGTERM: %w", err)
	}

	select {
	case <-exitCh:
	case <-time.After(stopGracePeriod):
		if err := task.Kill(ctx, syscall.SIGKILL); err != nil && !errdefs.IsNotFound(err) {
			return fmt.Errorf("send SIGKILL: %w", err)
		}
		<-exitCh
	}

	if _, err := task.Delete(ctx); err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	return nil
}

func containerDelete(ctx context.Context, container containerd.Container) error {
	if err := container.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
		return fmt.Errorf("delete container %s: %w", container.ID(), err)
	}
	return nil
}

// List implements Runtime.
func (c *Containerd) List(ctx context.Context) ([]ContainerStatus, error) {
	ctx = c.namespaced(ctx)

	containers, err := c.client.Containers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	statuses := make([]ContainerStatus, 0, len(containers))
	for _, ctr := range containers {
		st, err := containerStatus(ctx, ctr)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, st)
	}
	return statuses, nil
}

func containerStatus(ctx context.Context, ctr containerd.Container) (ContainerStatus, error) {
	info, err := ctr.Info(ctx)
	if err != nil {
		return ContainerStatus{}, fmt.Errorf("get info for %s: %w", ctr.ID(), err)
	}

	st := ContainerStatus{
		Name:  ctr.ID(),
		Image: info.Image,
		State: StateUnknown,
	}

	task, err := ctr.Task(ctx, nil)
	switch {
	case err == nil:
		taskStatus, statusErr := task.Status(ctx)
		if statusErr != nil {
			return ContainerStatus{}, fmt.Errorf("get task status for %s: %w", ctr.ID(), statusErr)
		}
		st.PID = task.Pid()
		st.State = mapProcessStatus(taskStatus.Status)
	case errdefs.IsNotFound(err):
		st.State = StateStopped
	default:
		return ContainerStatus{}, fmt.Errorf("get task for %s: %w", ctr.ID(), err)
	}

	return st, nil
}

func mapProcessStatus(s containerd.ProcessStatus) State {
	switch s {
	case containerd.Running:
		return StateRunning
	case containerd.Stopped:
		return StateStopped
	default:
		return StateUnknown
	}
}
