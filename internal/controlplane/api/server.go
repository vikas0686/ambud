// SPDX-License-Identifier: Apache-2.0

package api

import (
	"log/slog"
	"net/http"

	"github.com/vikas0686/ambud/internal/httputil"
)

// NewServer builds the control plane's HTTP handler.
func NewServer(st Store, logger *slog.Logger) http.Handler {
	h := &handlers{store: st}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/join-tokens", h.createJoinToken)
	mux.HandleFunc("POST /v1/nodes/register", h.registerNode)
	mux.HandleFunc("POST /v1/nodes/{id}/heartbeat", h.heartbeat)
	mux.HandleFunc("GET /v1/nodes", h.listNodes)
	mux.HandleFunc("POST /v1/workloads", h.createWorkload)
	mux.HandleFunc("GET /v1/workloads", h.listWorkloads)

	// withCORS wraps outermost, ahead of the mux: a browser's preflight
	// OPTIONS request must be answered before routing (the mux only
	// registers GET/POST handlers per path, so OPTIONS would otherwise
	// 405 there instead of getting CORS headers).
	return withCORS(httputil.WithLogging(logger, mux))
}

type handlers struct {
	store Store
}
