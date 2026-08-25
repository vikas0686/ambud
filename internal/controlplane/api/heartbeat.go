// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"net/http"

	"github.com/vikas0686/ambud/internal/apitypes"
	"github.com/vikas0686/ambud/internal/controlplane/store"
	"github.com/vikas0686/ambud/internal/httputil"
)

// heartbeat authenticates the calling node by its bearer credential
// (not the {id} path segment — that's just for a nicer URL and is
// cross-checked against the authenticated identity below), records
// what it reported, and replies with that node's full desired-state
// workload list. See docs/ROADMAP.md's Phase 3 note on this being the
// agent's shift from "dumb executor" to "local controller reconciling
// toward desired state."
func (h *handlers) heartbeat(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerToken(r)
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "missing bearer token")
		return
	}

	node, err := h.store.AuthenticateNode(r.Context(), token)
	if err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid credential")
		return
	}
	if r.PathValue("id") != node.ID.String() {
		httputil.WriteError(w, http.StatusForbidden, "credential does not match node id in path")
		return
	}

	var req apitypes.HeartbeatRequest
	if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body: "+decodeErr.Error())
		return
	}

	if updateErr := h.store.UpdateNodeHeartbeat(r.Context(), node.ID, store.NodeResources{
		CPUCores:       req.Resources.CPUCores,
		CPUUsedPercent: req.Resources.CPUUsedPercent,
		MemTotalBytes:  req.Resources.MemTotalBytes,
		MemUsedBytes:   req.Resources.MemUsedBytes,
		DiskTotalBytes: req.Resources.DiskTotalBytes,
		DiskUsedBytes:  req.Resources.DiskUsedBytes,
	}, remoteHost(r)); updateErr != nil {
		httputil.WriteError(w, http.StatusInternalServerError, updateErr.Error())
		return
	}

	for _, c := range req.Containers {
		if statusErr := h.store.UpdateWorkloadStatus(r.Context(), node.ID, c.Name, c.State, int(c.PID)); statusErr != nil {
			httputil.WriteError(w, http.StatusInternalServerError, statusErr.Error())
			return
		}
	}

	workloads, err := h.store.ListWorkloadsForNode(r.Context(), node.ID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := apitypes.HeartbeatResponse{Workloads: make([]apitypes.WorkloadSpec, 0, len(workloads))}
	for _, wl := range workloads {
		resp.Workloads = append(resp.Workloads, apitypes.WorkloadSpec{Name: wl.Name, Image: wl.Image, Ports: toAPIPorts(wl.Ports)})
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

// toAPIPorts converts the store's PortMapping into apitypes' wire
// type — kept separate so the wire contract can't accidentally change
// just because the store package's internals do.
func toAPIPorts(ports []store.PortMapping) []apitypes.PortMapping {
	if len(ports) == 0 {
		return nil
	}
	out := make([]apitypes.PortMapping, len(ports))
	for i, p := range ports {
		out[i] = apitypes.PortMapping{
			ContainerPort: uint16(p.ContainerPort), //nolint:gosec // ports are validated 1-65535 at store insert time (see cmd/ambudctl's parsePortNumber and the DB layer that wrote them)
			HostPort:      uint16(p.HostPort),      //nolint:gosec // same as above
			Protocol:      p.Protocol,
		}
	}
	return out
}

func fromAPIPorts(ports []apitypes.PortMapping) []store.PortMapping {
	if len(ports) == 0 {
		return nil
	}
	out := make([]store.PortMapping, len(ports))
	for i, p := range ports {
		out[i] = store.PortMapping{ContainerPort: int(p.ContainerPort), HostPort: int(p.HostPort), Protocol: p.Protocol}
	}
	return out
}
