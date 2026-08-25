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
	"time"

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

	return withLogging(logger, mux)
}

type handlers struct {
	rt        runtime.Runtime
	collector *ResourceCollector
}

// withLogging logs one line per request: method, path, resulting
// status code, and duration. It wraps http.ResponseWriter to capture
// the status code, since the standard library doesn't expose it after
// the fact.
func withLogging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
