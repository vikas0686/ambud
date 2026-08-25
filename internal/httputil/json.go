// SPDX-License-Identifier: Apache-2.0

// Package httputil holds small HTTP helpers shared by internal/agent
// and internal/controlplane/api — both write the same
// apitypes.ErrorResponse shape on failure, so that logic lives here
// once instead of being copy-pasted between the two servers.
package httputil

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/vikas0686/ambud/internal/apitypes"
)

// WriteJSON encodes v as the response body with the given status code.
// A failure to encode is logged rather than returned: by the time
// encoding fails, headers are already written and there's nothing
// meaningful left to do but note it happened.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write JSON response failed", "error", err)
	}
}

// WriteError writes an apitypes.ErrorResponse with the given status
// code and message.
func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, apitypes.ErrorResponse{Error: msg})
}
