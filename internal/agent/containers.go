// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vikas0686/ambud/internal/apitypes"
	"github.com/vikas0686/ambud/internal/runtime"
)

func (h *handlers) createContainer(w http.ResponseWriter, r *http.Request) {
	var req apitypes.CreateContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Name == "" || req.Image == "" {
		writeError(w, http.StatusBadRequest, "name and image are required")
		return
	}

	if err := h.rt.Run(r.Context(), req.Name, req.Image); err != nil {
		writeRuntimeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, apitypes.ContainerStatus{
		Name:  req.Name,
		Image: req.Image,
		State: string(runtime.StateRunning),
	})
}

func (h *handlers) listContainers(w http.ResponseWriter, r *http.Request) {
	statuses, err := h.rt.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apitypes.ListContainersResponse{Containers: toAPIStatuses(statuses)})
}

func (h *handlers) getContainer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	st, found, err := findContainer(r, h.rt, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "container not found: "+name)
		return
	}
	writeJSON(w, http.StatusOK, toAPIStatus(st))
}

func (h *handlers) stopContainer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	if err := h.rt.Stop(r.Context(), name); err != nil {
		writeRuntimeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// restartContainer stops and re-runs a container under the same name
// and image. internal/runtime has no Restart primitive of its own (see
// docs/ROADMAP.md Phase 1) — Stop there fully removes the container, so
// "restart" is composed here as: look up the current image, stop, run
// again.
func (h *handlers) restartContainer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	st, found, err := findContainer(r, h.rt, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "container not found: "+name)
		return
	}

	if err := h.rt.Stop(r.Context(), name); err != nil && !errors.Is(err, runtime.ErrNotFound) {
		writeRuntimeError(w, err)
		return
	}
	if err := h.rt.Run(r.Context(), name, st.Image); err != nil {
		writeRuntimeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, apitypes.ContainerStatus{
		Name:  name,
		Image: st.Image,
		State: string(runtime.StateRunning),
	})
}

func (h *handlers) pullImage(w http.ResponseWriter, r *http.Request) {
	var req apitypes.PullImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Image == "" {
		writeError(w, http.StatusBadRequest, "image is required")
		return
	}

	if err := h.rt.Pull(r.Context(), req.Image); err != nil {
		writeRuntimeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// findContainer looks up name in rt's current container list, since
// Runtime has no single-container lookup of its own.
func findContainer(r *http.Request, rt runtime.Runtime, name string) (runtime.ContainerStatus, bool, error) {
	statuses, err := rt.List(r.Context())
	if err != nil {
		return runtime.ContainerStatus{}, false, err
	}
	for _, st := range statuses {
		if st.Name == name {
			return st, true, nil
		}
	}
	return runtime.ContainerStatus{}, false, nil
}

func writeRuntimeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, runtime.ErrAlreadyExists):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, runtime.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func toAPIStatus(st runtime.ContainerStatus) apitypes.ContainerStatus {
	return apitypes.ContainerStatus{Name: st.Name, Image: st.Image, State: string(st.State), PID: st.PID}
}

func toAPIStatuses(statuses []runtime.ContainerStatus) []apitypes.ContainerStatus {
	out := make([]apitypes.ContainerStatus, 0, len(statuses))
	for _, st := range statuses {
		out = append(out, toAPIStatus(st))
	}
	return out
}
