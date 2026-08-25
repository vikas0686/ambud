// SPDX-License-Identifier: Apache-2.0

// Package apitypes holds the JSON request/response shapes for the
// ambud-agent HTTP API (and, from Phase 3 onward, the control plane's
// API). It exists as its own package — rather than living inside
// internal/agent or internal/runtime — so that both server-side
// packages (internal/agent) and client-side packages (internal/apiclient,
// and eventually the control plane) depend on one shared, stable wire
// contract instead of each other's internals. See
// docs/PROJECT_STRUCTURE.md.
package apitypes

// PortMapping maps a container port to a host port, matching `docker
// run -p` semantics — see docs/ROADMAP.md's Phase 6. It mirrors
// internal/runtime.PortMapping field for field but is defined
// separately as this package's own wire contract, same as
// ContainerStatus below.
type PortMapping struct {
	ContainerPort uint16 `json:"container_port"`
	HostPort      uint16 `json:"host_port"`
	// Protocol is "tcp" or "udp"; empty defaults to "tcp".
	Protocol string `json:"protocol,omitempty"`
}

// CreateContainerRequest is the JSON body for POST /v1/containers.
type CreateContainerRequest struct {
	Name  string        `json:"name"`
	Image string        `json:"image"`
	Ports []PortMapping `json:"ports,omitempty"`
}

// PullImageRequest is the JSON body for POST /v1/images/pull.
type PullImageRequest struct {
	Image string `json:"image"`
}

// ContainerStatus is the wire representation of a container's observed
// state. It deliberately mirrors internal/runtime.ContainerStatus field
// for field but is defined separately: this is the API's public
// contract, and shouldn't change just because the runtime package's
// internals do.
type ContainerStatus struct {
	Name  string        `json:"name"`
	Image string        `json:"image"`
	State string        `json:"state"`
	PID   uint32        `json:"pid,omitempty"`
	Ports []PortMapping `json:"ports,omitempty"`
}

// ListContainersResponse is the JSON body for GET /v1/containers.
type ListContainersResponse struct {
	Containers []ContainerStatus `json:"containers"`
}

// Resources is the JSON body for GET /v1/resources: a snapshot of host
// CPU/RAM/disk usage, as last sampled by the agent's background
// collector (see internal/agent.ResourceCollector).
type Resources struct {
	CPUCores       int     `json:"cpu_cores"`
	CPUUsedPercent float64 `json:"cpu_used_percent"`

	MemTotalBytes  uint64  `json:"mem_total_bytes"`
	MemUsedBytes   uint64  `json:"mem_used_bytes"`
	MemUsedPercent float64 `json:"mem_used_percent"`

	DiskTotalBytes  uint64  `json:"disk_total_bytes"`
	DiskUsedBytes   uint64  `json:"disk_used_bytes"`
	DiskUsedPercent float64 `json:"disk_used_percent"`
}

// ErrorResponse is the JSON body of any non-2xx response.
type ErrorResponse struct {
	Error string `json:"error"`
}
