// SPDX-License-Identifier: Apache-2.0

package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/vikas0686/ambud/internal/httputil"
)

// DefaultHeartbeatTimeout is how long a node can go without
// heartbeating before GET /v1/nodes reports it offline — three missed
// heartbeats at ambud-agent's own default --heartbeat-interval (5s).
const DefaultHeartbeatTimeout = 15 * time.Second

// NewServer builds the control plane's HTTP handler. heartbeatTimeout
// controls online/offline classification in NodeStatus.Status — see
// docs/ROADMAP.md's Phase 4.
func NewServer(st Store, heartbeatTimeout time.Duration, logger *slog.Logger) http.Handler {
	h := &handlers{store: st, heartbeatTimeout: heartbeatTimeout}

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
	store            Store
	heartbeatTimeout time.Duration
}
