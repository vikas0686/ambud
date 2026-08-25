// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vikas0686/ambud/internal/apitypes"
)

func testServer(t *testing.T, st Store) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	return NewServer(st, logger)
}

func createJoinToken(t *testing.T, srv http.Handler) string {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/join-tokens", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create join token: status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}

	var resp apitypes.CreateJoinTokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode join token response: %v", err)
	}
	return resp.Token
}

func registerNode(t *testing.T, srv http.Handler, joinToken, name string) apitypes.RegisterNodeResponse {
	t.Helper()
	body, _ := json.Marshal(apitypes.RegisterNodeRequest{JoinToken: joinToken, Name: name})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/nodes/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register node: status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}

	var resp apitypes.RegisterNodeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	return resp
}

func TestCreateJoinToken(t *testing.T) {
	srv := testServer(t, newFakeStore())
	token := createJoinToken(t, srv)
	if token == "" {
		t.Error("createJoinToken() returned an empty token")
	}
}

func TestRegisterNode(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srv := testServer(t, newFakeStore())
		token := createJoinToken(t, srv)
		resp := registerNode(t, srv, token, "node-1")
		if resp.NodeID == "" || resp.Credential == "" {
			t.Errorf("register response = %+v, want non-empty node_id and credential", resp)
		}
	})

	t.Run("invalid join token", func(t *testing.T) {
		srv := testServer(t, newFakeStore())
		body, _ := json.Marshal(apitypes.RegisterNodeRequest{JoinToken: "bogus", Name: "node-1"})
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/nodes/register", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("join token already used", func(t *testing.T) {
		srv := testServer(t, newFakeStore())
		token := createJoinToken(t, srv)
		registerNode(t, srv, token, "node-1")

		body, _ := json.Marshal(apitypes.RegisterNodeRequest{JoinToken: token, Name: "node-2"})
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/nodes/register", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusConflict {
			t.Errorf("status = %d, want 409 (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("duplicate name", func(t *testing.T) {
		srv := testServer(t, newFakeStore())
		token1 := createJoinToken(t, srv)
		registerNode(t, srv, token1, "node-1")

		token2 := createJoinToken(t, srv)
		body, _ := json.Marshal(apitypes.RegisterNodeRequest{JoinToken: token2, Name: "node-1"})
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/nodes/register", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusConflict {
			t.Errorf("status = %d, want 409 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

func TestListNodes(t *testing.T) {
	srv := testServer(t, newFakeStore())
	token := createJoinToken(t, srv)
	registerNode(t, srv, token, "node-1")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/nodes", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var resp apitypes.ListNodesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Nodes) != 1 || resp.Nodes[0].Name != "node-1" {
		t.Errorf("Nodes = %+v, want one node named node-1", resp.Nodes)
	}
}
