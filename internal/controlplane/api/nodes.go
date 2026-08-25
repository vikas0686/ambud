// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

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

	node, credential, err := h.store.RegisterNode(r.Context(), req.JoinToken, req.Name, remoteHost(r))
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

	now := time.Now()
	resp := apitypes.ListNodesResponse{Nodes: make([]apitypes.NodeStatus, 0, len(nodes))}
	for _, n := range nodes {
		resp.Nodes = append(resp.Nodes, toNodeStatus(n, h.heartbeatTimeout, now))
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func toNodeStatus(n store.Node, heartbeatTimeout time.Duration, now time.Time) apitypes.NodeStatus {
	return apitypes.NodeStatus{
		ID:              n.ID.String(),
		Name:            n.Name,
		Status:          nodeState(n.LastHeartbeatAt, heartbeatTimeout, now),
		CreatedAt:       n.CreatedAt,
		LastHeartbeatAt: n.LastHeartbeatAt,
		Address:         n.Address,
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

// nodeState classifies a node as online/offline as of now, given when
// it last heartbeated and how long it's allowed to go without one. A
// node that has never heartbeated (lastHeartbeatAt == nil) is offline,
// not some third "unknown" state — from an operator's point of view,
// "never checked in" and "stopped checking in" call for the same
// attention.
func nodeState(lastHeartbeatAt *time.Time, heartbeatTimeout time.Duration, now time.Time) apitypes.NodeState {
	if lastHeartbeatAt == nil {
		return apitypes.NodeOffline
	}
	if now.Sub(*lastHeartbeatAt) > heartbeatTimeout {
		return apitypes.NodeOffline
	}
	return apitypes.NodeOnline
}

// remoteHost returns the calling client's host/IP, without its
// ephemeral source port — the address a node registers or heartbeats
// from is captured this way (server-side, from the actual TCP peer)
// rather than trusted from anything the agent claims about itself, see
// docs/ROADMAP.md's Phase 6. Falls back to the raw RemoteAddr if it
// isn't in host:port form, which shouldn't happen for a real
// net/http request but is harmless either way.
func remoteHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
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
	case errors.Is(err, store.ErrAlreadyExists), errors.Is(err, store.ErrPortConflict):
		httputil.WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, store.ErrNotFound):
		httputil.WriteError(w, http.StatusNotFound, err.Error())
	default:
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
	}
}
