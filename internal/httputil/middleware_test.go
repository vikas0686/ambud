// SPDX-License-Identifier: Apache-2.0

package httputil

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWithLogging(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	wrapped := WithLogging(logger, inner)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/example", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d (WithLogging must not alter the response)", rec.Code, http.StatusTeapot)
	}

	logged := logs.String()
	for _, want := range []string{"GET", "/v1/example", "418"} {
		if !strings.Contains(logged, want) {
			t.Errorf("log output missing %q; got: %s", want, logged)
		}
	}
}
