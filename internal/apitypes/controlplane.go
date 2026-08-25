// SPDX-License-Identifier: Apache-2.0

package apitypes

import "time"

// CreateJoinTokenResponse is the JSON body for POST /v1/join-tokens.
// Token is only ever returned here — the control plane stores just its
// hash — so this response must be captured by the operator immediately.
type CreateJoinTokenResponse struct {
	Token string `json:"token"`
}

// RegisterNodeRequest is the JSON body for POST /v1/nodes/register.
type RegisterNodeRequest struct {
	JoinToken string `json:"join_token"`
	Name      string `json:"name"`
}

// RegisterNodeResponse is the JSON body for a successful
// POST /v1/nodes/register. Credential must be persisted by the agent
// (see internal/agent's credential store) and sent as a bearer token
// on every subsequent heartbeat — like the join token, the control
// plane only ever stores its hash, so this is the one time it's
// visible.
type RegisterNodeResponse struct {
	NodeID     string `json:"node_id"`
	Credential string `json:"credential"`
}

// HeartbeatRequest is the JSON body for POST /v1/nodes/{id}/heartbeat:
// what the agent currently knows about itself and what it's running.
type HeartbeatRequest struct {
	Resources  Resources         `json:"resources"`
	Containers []ContainerStatus `json:"containers"`
}

// WorkloadSpec is one entry in a HeartbeatResponse: a workload the
// receiving node should be running. It's deliberately minimal (just
// enough to call Runtime.Run) — anything richer belongs to later
// phases (resource requests in Phase 5, volumes in Phase 7, ...).
type WorkloadSpec struct {
	Name  string        `json:"name"`
	Image string        `json:"image"`
	Ports []PortMapping `json:"ports,omitempty"`
}

// HeartbeatResponse is the JSON body of a heartbeat reply: the full
// desired-state list for the reporting node. The agent reconciles its
// local containerd state toward exactly this list — see
// docs/ROADMAP.md's Phase 3 note on the agent shifting from "dumb
// executor" to "local controller."
type HeartbeatResponse struct {
	Workloads []WorkloadSpec `json:"workloads"`
}

// NodeStatus is the wire representation of a registered node, as
// returned by GET /v1/nodes. Status is computed by the control plane
// at request time from LastHeartbeatAt and its --heartbeat-timeout
// (see docs/ROADMAP.md's Phase 4) — never stored, so it's always
// current as of the response, not as of whenever it was last written.
type NodeStatus struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Status          NodeState  `json:"status"`
	CreatedAt       time.Time  `json:"created_at"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at,omitempty"`
	Resources       Resources  `json:"resources"`
	// Address is the host/IP the node last registered or heartbeated
	// from, captured server-side (not self-reported) — see
	// docs/ROADMAP.md's Phase 6. Empty if the node has never
	// registered/heartbeated, which shouldn't happen in practice since
	// registration always sets it.
	Address string `json:"address,omitempty"`
}

// NodeState is a node's computed reachability, from the control
// plane's point of view.
type NodeState string

const (
	// NodeOnline means the node heartbeated within the configured
	// --heartbeat-timeout.
	NodeOnline NodeState = "online"
	// NodeOffline means it hasn't — including a node that has never
	// heartbeated at all.
	NodeOffline NodeState = "offline"
)

// ListNodesResponse is the JSON body for GET /v1/nodes.
type ListNodesResponse struct {
	Nodes []NodeStatus `json:"nodes"`
}

// CreateWorkloadRequest is the JSON body for POST /v1/workloads.
// NodeID is optional: if omitted, the control plane assigns the only
// registered node when there's exactly one, and returns an error
// otherwise — there's no scheduler yet (Phase 5), so ambiguity is
// rejected rather than guessed at.
type CreateWorkloadRequest struct {
	Name   string        `json:"name"`
	Image  string        `json:"image"`
	NodeID string        `json:"node_id,omitempty"`
	Ports  []PortMapping `json:"ports,omitempty"`
}

// WorkloadStatus is the wire representation of a workload, as returned
// by GET /v1/workloads and POST /v1/workloads.
type WorkloadStatus struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Image     string    `json:"image"`
	NodeID    string    `json:"node_id"`
	NodeName  string    `json:"node_name"`
	State     string    `json:"state"`
	PID       uint32    `json:"pid,omitempty"`
	CreatedAt time.Time `json:"created_at"`

	Ports []PortMapping `json:"ports,omitempty"`
	// NodeAddress is the node's Address (see NodeStatus) at the time of
	// the response — combined with a PortMapping's HostPort, this is
	// the reachable "nodeIP:hostPort" address for the workload (see
	// docs/ROADMAP.md's Phase 6). Empty if the node has no known
	// address yet.
	NodeAddress string `json:"node_address,omitempty"`
}

// ListWorkloadsResponse is the JSON body for GET /v1/workloads.
type ListWorkloadsResponse struct {
	Workloads []WorkloadStatus `json:"workloads"`
}
