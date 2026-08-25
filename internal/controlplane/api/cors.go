// SPDX-License-Identifier: Apache-2.0

package api

import "net/http"

// withCORS allows the Web UI (served from Vite's dev server on a
// different origin/port than the control plane, and from a different
// origin than the control plane in production too — see
// docs/ARCHITECTURE.md) to call this API from a browser. Without it,
// every browser fetch() call here would be blocked by CORS before ever
// reaching a handler — curl and ambudctl have no same-origin policy, so
// this gap is invisible to non-browser testing.
//
// Wide open ("*") for now: every endpoint here except heartbeat is
// already unauthenticated (Phase 9 adds user auth), so this doesn't
// weaken anything that isn't already wide open. Revisit alongside
// Phase 9 — a real origin allowlist belongs there, not bolted on here
// ahead of the auth model that should drive it.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
