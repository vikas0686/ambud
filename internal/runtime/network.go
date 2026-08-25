// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	gocni "github.com/containerd/go-cni"
)

// Default paths for CNI plugin binaries and config — standard
// locations across containerd/nerdctl/Kubernetes, not an Ambud
// invention. See docs/DEVELOPMENT.md for how to install the plugin
// binaries; this package writes its own network config into
// cniConfDir automatically (see ensureCNIConflist) so there's no
// separate manual config step.
const (
	cniConfDir      = "/etc/cni/net.d"
	cniConflistPath = cniConfDir + "/10-ambud.conflist"
	cniBinDir       = "/opt/cni/bin"

	// netnsDir matches `ip netns add`'s own default location, so
	// standard `ip netns` tooling can inspect what Ambud creates.
	netnsDir = "/var/run/netns"
)

//go:embed cni-conflist.json
var defaultCNIConflist []byte

// PortMapping maps a container port to a host port, matching `docker
// run -p` semantics — see docs/ROADMAP.md's Phase 6.
type PortMapping struct {
	ContainerPort uint16
	HostPort      uint16
	// Protocol is "tcp" or "udp"; empty defaults to "tcp".
	Protocol string
}

// network gives containers real connectivity (a bridge, an IP,
// outbound NAT) and, when ports are given, inbound host-port
// forwarding, via the CNI bridge+portmap plugins — see
// docs/ADR/0001-initial-architecture.md's "why not build our own
// networking" reasoning. Before this, a container created by this
// package had loopback only; see docs/ROADMAP.md's Phase 6 for why
// that was true through Phase 5 and what changes here.
//
// Each container gets its own named, persistent network namespace
// (via `ip netns add`, not derived from the container's process PID)
// so teardown works the same way whether the container was stopped
// normally or crashed — a PID-derived netns path (/proc/<pid>/ns/net)
// stops resolving the moment the process is gone, which is exactly
// when crash cleanup needs it most.
type network struct {
	cni gocni.CNI
}

func newNetwork() (*network, error) {
	if err := ensureCNIConflist(); err != nil {
		return nil, fmt.Errorf("ensure CNI config: %w", err)
	}

	cni, err := gocni.New(
		gocni.WithMinNetworkCount(2), // loopback + the ambud bridge
		gocni.WithPluginConfDir(cniConfDir),
		gocni.WithPluginDir([]string{cniBinDir}),
	)
	if err != nil {
		return nil, fmt.Errorf("init CNI: %w", err)
	}
	if err := cni.Load(gocni.WithLoNetwork, gocni.WithConfListFile(cniConflistPath)); err != nil {
		return nil, fmt.Errorf("load CNI config from %s (is %s installed? see docs/DEVELOPMENT.md): %w",
			cniConflistPath, cniBinDir, err)
	}

	return &network{cni: cni}, nil
}

// ensureCNIConflist writes the embedded default network config to
// cniConflistPath if nothing is there yet. An operator can replace it
// with their own before first starting the agent for different
// bridge/subnet settings; this never overwrites an existing file.
func ensureCNIConflist() error {
	if _, err := os.Stat(cniConflistPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if err := os.MkdirAll(cniConfDir, 0o755); err != nil { //nolint:gosec // standard CNI config dir, world-readable by convention (other CNI-aware tools expect to read it)
		return fmt.Errorf("create %s: %w", cniConfDir, err)
	}
	if err := os.WriteFile(cniConflistPath, defaultCNIConflist, 0o644); err != nil { //nolint:gosec // CNI config is not secret; other tools (ip, CNI plugins themselves) expect to read it
		return fmt.Errorf("write %s: %w", cniConflistPath, err)
	}
	return nil
}

func netnsPath(name string) string {
	return netnsDir + "/ambud-" + name
}

// createNetNS creates a new, empty, named network namespace for name
// and returns its path. Shells out to `ip netns add` (iproute2)
// rather than making raw netns syscalls directly — boring, already
// installed everywhere containerd is, and exactly what `ip netns`
// exists for.
func createNetNS(ctx context.Context, name string) (string, error) {
	cmd := exec.CommandContext(ctx, "ip", "netns", "add", "ambud-"+name) //nolint:gosec // name is an Ambud container name, not attacker-controlled input
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ip netns add ambud-%s: %w (%s)", name, err, out)
	}
	return netnsPath(name), nil
}

// deleteNetNS removes the namespace created by createNetNS. A missing
// namespace is not an error — callers may race with (or repeat after)
// a partial cleanup.
func deleteNetNS(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "ip", "netns", "delete", "ambud-"+name) //nolint:gosec // name is an Ambud container name, not attacker-controlled input
	out, err := cmd.CombinedOutput()
	if err != nil && !isNoSuchNamespace(out) {
		return fmt.Errorf("ip netns delete ambud-%s: %w (%s)", name, err, out)
	}
	return nil
}

func isNoSuchNamespace(out []byte) bool {
	s := string(out)
	return strings.Contains(s, "No such file") || strings.Contains(s, "not found")
}

// setup gives the namespace at path real networking, applying any
// port mappings. id must be unique per call (Ambud uses the container
// name) — CNI uses it to key the state it tracks for later Remove
// calls.
func (n *network) setup(ctx context.Context, id, path string, ports []PortMapping) (*gocni.Result, error) {
	var opts []gocni.NamespaceOpts
	if len(ports) > 0 {
		mappings := make([]gocni.PortMapping, 0, len(ports))
		for _, p := range ports {
			proto := p.Protocol
			if proto == "" {
				proto = "tcp"
			}
			mappings = append(mappings, gocni.PortMapping{
				HostPort:      int32(p.HostPort),
				ContainerPort: int32(p.ContainerPort),
				Protocol:      proto,
			})
		}
		opts = append(opts, gocni.WithCapabilityPortMap(mappings))
	}

	result, err := n.cni.Setup(ctx, id, path, opts...)
	if err != nil {
		return nil, fmt.Errorf("cni setup for %s: %w", id, err)
	}
	return result, nil
}

// teardown removes the networking set up by setup — the veth pair and
// any portmap iptables rules. Safe to call even if path's namespace
// has already been deleted; CNI plugins are specified to tolerate a
// missing target namespace on removal and still clean up host-side
// state.
func (n *network) teardown(ctx context.Context, id, path string) error {
	if err := n.cni.Remove(ctx, id, path); err != nil {
		return fmt.Errorf("cni teardown for %s: %w", id, err)
	}
	return nil
}
