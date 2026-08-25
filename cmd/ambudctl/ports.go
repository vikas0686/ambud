// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/vikas0686/ambud/internal/apitypes"
	"github.com/vikas0686/ambud/internal/runtime"
)

// parsePortSpecs parses each --port flag value in "hostPort:containerPort[/protocol]"
// form — matching `docker run -p` syntax minus the optional host-IP
// prefix, since Ambud binds every mapping to all interfaces (see
// docs/ROADMAP.md's Phase 6). protocol defaults to "tcp" if omitted.
func parsePortSpecs(specs []string) ([]runtime.PortMapping, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	ports := make([]runtime.PortMapping, 0, len(specs))
	for _, spec := range specs {
		p, err := parsePortSpec(spec)
		if err != nil {
			return nil, err
		}
		ports = append(ports, p)
	}
	return ports, nil
}

func parsePortSpec(spec string) (runtime.PortMapping, error) {
	proto := "tcp"
	rest := spec
	if i := strings.LastIndexByte(spec, '/'); i != -1 {
		proto = strings.ToLower(spec[i+1:])
		rest = spec[:i]
	}
	if proto != "tcp" && proto != "udp" {
		return runtime.PortMapping{}, fmt.Errorf("invalid port mapping %q: protocol must be tcp or udp, got %q", spec, proto)
	}

	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return runtime.PortMapping{}, fmt.Errorf("invalid port mapping %q: expected hostPort:containerPort[/protocol]", spec)
	}

	hostPort, err := parsePortNumber(parts[0])
	if err != nil {
		return runtime.PortMapping{}, fmt.Errorf("invalid host port in %q: %w", spec, err)
	}
	containerPort, err := parsePortNumber(parts[1])
	if err != nil {
		return runtime.PortMapping{}, fmt.Errorf("invalid container port in %q: %w", spec, err)
	}

	return runtime.PortMapping{HostPort: hostPort, ContainerPort: containerPort, Protocol: proto}, nil
}

func parsePortNumber(s string) (uint16, error) {
	n, err := strconv.ParseUint(s, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("must be a number from 1 to 65535: %w", err)
	}
	if n == 0 {
		return 0, fmt.Errorf("must be a number from 1 to 65535")
	}
	return uint16(n), nil
}

// toAPIPortMappings converts parsed --port flags into the control
// plane's wire type for deploy's CreateWorkloadRequest.
func toAPIPortMappings(ports []runtime.PortMapping) []apitypes.PortMapping {
	if len(ports) == 0 {
		return nil
	}
	out := make([]apitypes.PortMapping, len(ports))
	for i, p := range ports {
		out[i] = apitypes.PortMapping{ContainerPort: p.ContainerPort, HostPort: p.HostPort, Protocol: p.Protocol}
	}
	return out
}

// formatPorts renders a workload's port mappings the way `docker ps`
// does: "hostPort:containerPort/protocol", comma-separated. Returns
// "-" when there are none, so table columns stay aligned.
func formatPorts(ports []apitypes.PortMapping) string {
	if len(ports) == 0 {
		return "-"
	}
	parts := make([]string, len(ports))
	for i, p := range ports {
		parts[i] = fmt.Sprintf("%d:%d/%s", p.HostPort, p.ContainerPort, p.Protocol)
	}
	return strings.Join(parts, ",")
}

// formatRuntimePorts is formatPorts for the runtime package's own
// PortMapping — used by ps, which talks to a Runtime directly rather
// than through the control plane's wire types.
func formatRuntimePorts(ports []runtime.PortMapping) string {
	return formatPorts(toAPIPortMappings(ports))
}
