// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/vikas0686/ambud/internal/apitypes"
	"github.com/vikas0686/ambud/internal/controlplane/store"
	"github.com/vikas0686/ambud/internal/httputil"
)

func (h *handlers) createJoinToken(w http.ResponseWriter, r *http.Request) {
	token, err := h.store.CreateJoinToken(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, apitypes.CreateJoinTokenResponse{Token: token})
}

func (h *handlers) registerNode(w http.ResponseWriter, r *http.Request) {
	var req apitypes.RegisterNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.JoinToken == "" || req.Name == "" {
		httputil.WriteError(w, http.StatusBadRequest, "join_token and name are required")
		return
	}

	node, credential, err := h.store.RegisterNode(r.Context(), req.JoinToken, req.Name)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, apitypes.RegisterNodeResponse{
		NodeID:     node.ID.String(),
		Credential: credential,
	})
}

func (h *handlers) listNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.store.ListNodes(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := apitypes.ListNodesResponse{Nodes: make([]apitypes.NodeStatus, 0, len(nodes))}
	for _, n := range nodes {
		resp.Nodes = append(resp.Nodes, toNodeStatus(n))
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func toNodeStatus(n store.Node) apitypes.NodeStatus {
	return apitypes.NodeStatus{
		ID:              n.ID.String(),
		Name:            n.Name,
		CreatedAt:       n.CreatedAt,
		LastHeartbeatAt: n.LastHeartbeatAt,
		Resources: apitypes.Resources{
			CPUCores:       n.CPUCores,
			CPUUsedPercent: n.CPUUsedPercent,
			MemTotalBytes:  n.MemTotalBytes,
			MemUsedBytes:   n.MemUsedBytes,
			DiskTotalBytes: n.DiskTotalBytes,
			DiskUsedBytes:  n.DiskUsedBytes,
		},
	}
}

// bearerToken extracts the token from an "Authorization: Bearer <token>"
// header.
func bearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, prefix) {
		return "", false
	}
	return strings.TrimPrefix(auth, prefix), true
}

// writeStoreError translates a store package error into the
// appropriate HTTP status, matching the pattern internal/agent uses
// for internal/runtime errors.
func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrAlreadyExists):
		httputil.WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, store.ErrNotFound):
		httputil.WriteError(w, http.StatusNotFound, err.Error())
	default:
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
	}
}
