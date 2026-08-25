// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vikas0686/ambud/internal/apitypes"
)

func doCreateWorkload(t *testing.T, srv http.Handler, req apitypes.CreateWorkloadRequest) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/workloads", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httpReq)
	return rec
}

func TestCreateWorkload_AutoAssignsSoleNode(t *testing.T) {
	srv := testServer(t, newFakeStore())
	token := createJoinToken(t, srv)
	reg := registerNode(t, srv, token, "node-1")

	rec := doCreateWorkload(t, srv, apitypes.CreateWorkloadRequest{Name: "web", Image: "nginx:alpine"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}

	var got apitypes.WorkloadStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.NodeID != reg.NodeID {
		t.Errorf("WorkloadStatus.NodeID = %q, want %q", got.NodeID, reg.NodeID)
	}
}

func TestCreateWorkload_NoNodesRegistered(t *testing.T) {
	srv := testServer(t, newFakeStore())

	rec := doCreateWorkload(t, srv, apitypes.CreateWorkloadRequest{Name: "web", Image: "nginx:alpine"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestCreateWorkload_MultipleNodesRequireExplicitNodeID(t *testing.T) {
	srv := testServer(t, newFakeStore())
	token1 := createJoinToken(t, srv)
	registerNode(t, srv, token1, "node-1")
	token2 := createJoinToken(t, srv)
	reg2 := registerNode(t, srv, token2, "node-2")

	t.Run("omitted node_id is ambiguous", func(t *testing.T) {
		rec := doCreateWorkload(t, srv, apitypes.CreateWorkloadRequest{Name: "web", Image: "nginx:alpine"})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("explicit node_id works", func(t *testing.T) {
		rec := doCreateWorkload(t, srv, apitypes.CreateWorkloadRequest{Name: "web", Image: "nginx:alpine", NodeID: reg2.NodeID})
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

func TestCreateWorkload_DuplicateNameConflicts(t *testing.T) {
	srv := testServer(t, newFakeStore())
	token := createJoinToken(t, srv)
	registerNode(t, srv, token, "node-1")

	doCreateWorkload(t, srv, apitypes.CreateWorkloadRequest{Name: "web", Image: "nginx:alpine"})
	rec := doCreateWorkload(t, srv, apitypes.CreateWorkloadRequest{Name: "web", Image: "nginx:alpine"})
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestListWorkloads(t *testing.T) {
	srv := testServer(t, newFakeStore())
	token := createJoinToken(t, srv)
	registerNode(t, srv, token, "node-1")
	doCreateWorkload(t, srv, apitypes.CreateWorkloadRequest{Name: "web", Image: "nginx:alpine"})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/workloads", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var resp apitypes.ListWorkloadsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Workloads) != 1 || resp.Workloads[0].NodeName != "node-1" {
		t.Errorf("Workloads = %+v, want one workload with NodeName=node-1", resp.Workloads)
	}
}
