// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"encoding/json"
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
	specs "github.com/opencontainers/runtime-spec/specs-go"
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

	// portsLabel stores a container's port mappings as JSON, so List
	// can report them back without Ambud needing a separate store of
	// its own — containerd's container labels are already exactly the
	// right place for this kind of small, container-scoped metadata.
	portsLabel = "io.ambud.ports"
)

// Containerd is a Runtime backed by containerd's native Go client
// (not the CRI shim — see docs/ADR/0001-initial-architecture.md for
// why). It satisfies the Runtime interface defined in runtime.go.
type Containerd struct {
	client  *containerd.Client
	logDir  string
	network *network
}

var _ Runtime = (*Containerd)(nil)

// New connects to containerd over its local Unix socket at socketPath
// and prepares Ambud's CNI network (see network.go) for giving
// containers real connectivity.
func New(socketPath string) (*Containerd, error) {
	client, err := containerd.New(socketPath)
	if err != nil {
		return nil, fmt.Errorf("connect to containerd at %s: %w", socketPath, err)
	}

	net, err := newNetwork()
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("init networking: %w", err)
	}

	return &Containerd{client: client, logDir: defaultLogDir, network: net}, nil
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
func (c *Containerd) Run(ctx context.Context, name, image string, ports []PortMapping) error {
	ctx = c.namespaced(ctx)

	img, err := c.client.Pull(ctx, image, containerd.WithPullUnpack)
	if err != nil {
		return fmt.Errorf("pull image %s: %w", image, err)
	}

	nsPath, err := createNetNS(ctx, name)
	if err != nil {
		return fmt.Errorf("create network namespace for %s: %w", name, err)
	}

	portsJSON, err := json.Marshal(ports)
	if err != nil {
		_ = deleteNetNS(ctx, name)
		return fmt.Errorf("encode port mappings for %s: %w", name, err)
	}

	container, err := c.client.NewContainer(
		ctx,
		name,
		containerd.WithNewSnapshot(name+"-snapshot", img),
		containerd.WithContainerLabels(map[string]string{portsLabel: string(portsJSON)}),
		containerd.WithNewSpec(
			oci.WithImageConfig(img),
			oci.WithLinuxNamespace(specs.LinuxNamespace{Type: specs.NetworkNamespace, Path: nsPath}),
		),
	)
	if err != nil {
		_ = deleteNetNS(ctx, name)
		if errdefs.IsAlreadyExists(err) {
			return fmt.Errorf("create container %s: %w: %w", name, ErrAlreadyExists, err)
		}
		return fmt.Errorf("create container %s: %w", name, err)
	}

	// CNI setup happens before the task starts, not after, so there's
	// no window where the app is already running but unreachable.
	if _, setupErr := c.network.setup(ctx, name, nsPath, ports); setupErr != nil {
		_ = container.Delete(ctx, containerd.WithSnapshotCleanup)
		_ = deleteNetNS(ctx, name)
		return fmt.Errorf("set up networking for %s: %w", name, setupErr)
	}

	if mkdirErr := os.MkdirAll(c.logDir, 0o750); mkdirErr != nil {
		_ = c.network.teardown(ctx, name, nsPath)
		_ = container.Delete(ctx, containerd.WithSnapshotCleanup)
		_ = deleteNetNS(ctx, name)
		return fmt.Errorf("create log directory %s: %w", c.logDir, mkdirErr)
	}
	logPath := filepath.Join(c.logDir, name+".log")

	task, err := container.NewTask(ctx, cio.LogFile(logPath))
	if err != nil {
		_ = c.network.teardown(ctx, name, nsPath)
		_ = container.Delete(ctx, containerd.WithSnapshotCleanup)
		_ = deleteNetNS(ctx, name)
		return fmt.Errorf("create task for %s: %w", name, err)
	}

	if err := task.Start(ctx); err != nil {
		_, _ = task.Delete(ctx)
		_ = c.network.teardown(ctx, name, nsPath)
		_ = container.Delete(ctx, containerd.WithSnapshotCleanup)
		_ = deleteNetNS(ctx, name)
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
			// stop, just clean up networking and the container record.
			return c.cleanupContainer(ctx, name, container)
		}
		return fmt.Errorf("load task for %s: %w", name, err)
	}

	if err := stopTask(ctx, task); err != nil {
		return fmt.Errorf("stop task for %s: %w", name, err)
	}

	return c.cleanupContainer(ctx, name, container)
}

// cleanupContainer tears down networking (idempotent: safe even if
// setup never completed, or already ran once — see network.go) and
// removes the container record. The network namespace is named after
// name, not derived from the task's PID, specifically so this works
// the same way whether the task exited cleanly, crashed, or never
// started at all.
func (c *Containerd) cleanupContainer(ctx context.Context, name string, container containerd.Container) error {
	if err := c.network.teardown(ctx, name, netnsPath(name)); err != nil {
		return fmt.Errorf("tear down networking for %s: %w", name, err)
	}
	if err := deleteNetNS(ctx, name); err != nil {
		return fmt.Errorf("delete network namespace for %s: %w", name, err)
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
		Ports: decodePorts(info.Labels[portsLabel]),
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

// decodePorts parses the JSON written by Run into the ports label. A
// missing or malformed label (e.g. a container created before Phase 6,
// or by something other than Ambud) decodes to nil rather than an
// error — port info is a nice-to-have on List, not something worth
// failing the whole call over.
func decodePorts(label string) []PortMapping {
	if label == "" {
		return nil
	}
	var ports []PortMapping
	if err := json.Unmarshal([]byte(label), &ports); err != nil {
		return nil
	}
	return ports
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
