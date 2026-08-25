// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/vikas0686/ambud/internal/apitypes"
)

// writeJSON encodes v as the response body with the given status code.
// A failure to encode is logged rather than returned: by the time
// encoding fails, headers are already written and there's nothing
// meaningful left to do but note it happened.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write JSON response failed", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apitypes.ErrorResponse{Error: msg})
}
