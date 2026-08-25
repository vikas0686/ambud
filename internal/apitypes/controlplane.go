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
	Name  string `json:"name"`
	Image string `json:"image"`
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
// returned by GET /v1/nodes. There's no online/offline field yet —
// heartbeat-timeout classification is Phase 4 (docs/ROADMAP.md);
// LastHeartbeatAt is the raw fact callers have today.
type NodeStatus struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	CreatedAt       time.Time  `json:"created_at"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at,omitempty"`
	Resources       Resources  `json:"resources"`
}

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
	Name   string `json:"name"`
	Image  string `json:"image"`
	NodeID string `json:"node_id,omitempty"`
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
}

// ListWorkloadsResponse is the JSON body for GET /v1/workloads.
type ListWorkloadsResponse struct {
	Workloads []WorkloadStatus `json:"workloads"`
}
