// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/vikas0686/ambud/internal/apitypes"
	"github.com/vikas0686/ambud/internal/controlplane/store"
	"github.com/vikas0686/ambud/internal/httputil"
)

func (h *handlers) createWorkload(w http.ResponseWriter, r *http.Request) {
	var req apitypes.CreateWorkloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Name == "" || req.Image == "" {
		httputil.WriteError(w, http.StatusBadRequest, "name and image are required")
		return
	}

	nodeID, err := h.resolveTargetNode(r, req.NodeID)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	wl, err := h.store.CreateWorkload(r.Context(), req.Name, req.Image, nodeID, fromAPIPorts(req.Ports))
	if err != nil {
		writeStoreError(w, err)
		return
	}

	node, err := h.store.GetNode(r.Context(), nodeID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, toWorkloadStatus(wl, node.Name, node.Address))
}

// resolveTargetNode implements Phase 3/4's "no scheduler yet" rule
// (docs/ROADMAP.md): an explicit node ID is used as given; an omitted
// one is only auto-resolved when there's exactly one registered node.
// Ambiguity (zero or multiple nodes) is a client error, not a guess.
func (h *handlers) resolveTargetNode(r *http.Request, requestedID string) (uuid.UUID, error) {
	if requestedID != "" {
		id, err := uuid.Parse(requestedID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("invalid node_id: %w", err)
		}
		if _, err := h.store.GetNode(r.Context(), id); err != nil {
			return uuid.Nil, fmt.Errorf("node_id %s: %w", requestedID, err)
		}
		return id, nil
	}

	nodes, err := h.store.ListNodes(r.Context())
	if err != nil {
		return uuid.Nil, err
	}
	switch len(nodes) {
	case 0:
		return uuid.Nil, errors.New("no nodes registered — register a node before deploying")
	case 1:
		return nodes[0].ID, nil
	default:
		return uuid.Nil, errors.New("multiple nodes registered and no scheduler yet (Phase 5) — specify node_id explicitly")
	}
}

func (h *handlers) listWorkloads(w http.ResponseWriter, r *http.Request) {
	workloads, err := h.store.ListWorkloads(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	nodes, err := h.store.ListNodes(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	nodes2 := make(map[uuid.UUID]store.Node, len(nodes))
	for _, n := range nodes {
		nodes2[n.ID] = n
	}

	resp := apitypes.ListWorkloadsResponse{Workloads: make([]apitypes.WorkloadStatus, 0, len(workloads))}
	for _, wl := range workloads {
		node := nodes2[wl.NodeID]
		resp.Workloads = append(resp.Workloads, toWorkloadStatus(wl, node.Name, node.Address))
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func toWorkloadStatus(wl store.Workload, nodeName, nodeAddress string) apitypes.WorkloadStatus {
	return apitypes.WorkloadStatus{
		ID:          wl.ID.String(),
		Name:        wl.Name,
		Image:       wl.Image,
		NodeID:      wl.NodeID.String(),
		NodeName:    nodeName,
		State:       wl.State,
		PID:         uint32(wl.PID), //nolint:gosec // PIDs are process IDs, never negative or larger than uint32 in practice
		CreatedAt:   wl.CreatedAt,
		Ports:       toAPIPorts(wl.Ports),
		NodeAddress: nodeAddress,
	}
}
