// SPDX-License-Identifier: Apache-2.0

// Package agent implements the ambud-agent HTTP API: container
// lifecycle endpoints backed by an internal/runtime.Runtime, and a
// host resource-usage endpoint backed by a background ResourceCollector.
// See docs/ROADMAP.md's Phase 2 and docs/ARCHITECTURE.md's Node Agent
// section — this package is the "local REST API" half of the agent;
// cmd/ambud-agent wires it to a real containerd-backed runtime and an
// HTTP server with graceful shutdown.
package agent

import (
	"log/slog"
	"net/http"

	"github.com/vikas0686/ambud/internal/httputil"
	"github.com/vikas0686/ambud/internal/runtime"
)

// NewServer builds the agent's HTTP handler: container lifecycle
// endpoints backed by rt, and a resources endpoint backed by collector.
func NewServer(rt runtime.Runtime, collector *ResourceCollector, logger *slog.Logger) http.Handler {
	h := &handlers{rt: rt, collector: collector}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/containers", h.createContainer)
	mux.HandleFunc("GET /v1/containers", h.listContainers)
	mux.HandleFunc("GET /v1/containers/{name}", h.getContainer)
	mux.HandleFunc("POST /v1/containers/{name}/stop", h.stopContainer)
	mux.HandleFunc("POST /v1/containers/{name}/restart", h.restartContainer)
	mux.HandleFunc("POST /v1/images/pull", h.pullImage)
	mux.HandleFunc("GET /v1/resources", h.getResources)

	return httputil.WithLogging(logger, mux)
}

type handlers struct {
	rt        runtime.Runtime
	collector *ResourceCollector
}
