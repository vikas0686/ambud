// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS_PreflightIsAnswered(t *testing.T) {
	srv := testServer(t, newFakeStore())

	req := httptest.NewRequestWithContext(context.Background(), http.MethodOptions, "/v1/nodes", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want \"*\"", got)
	}
}

func TestCORS_HeaderPresentOnRealRequests(t *testing.T) {
	srv := testServer(t, newFakeStore())

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/nodes", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want \"*\" on a normal request too", got)
	}
}
