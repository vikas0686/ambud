// SPDX-License-Identifier: Apache-2.0

package httputil

import (
	"log/slog"
	"net/http"
	"time"
)

// WithLogging logs one line per request: method, path, resulting
// status code, and duration. Shared by internal/agent and
// internal/controlplane/api so both servers log identically.
func WithLogging(logger *slog.Logger, next http.Handler) http.Handler {
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

// statusRecorder wraps http.ResponseWriter to capture the status code,
// since the standard library doesn't expose it after the fact.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
