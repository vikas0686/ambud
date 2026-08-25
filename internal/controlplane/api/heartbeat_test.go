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

func doHeartbeat(t *testing.T, srv http.Handler, nodeID, credential string, req apitypes.HeartbeatRequest) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/nodes/"+nodeID+"/heartbeat", bytes.NewReader(body))
	httpReq.Header.Set("Authorization", "Bearer "+credential)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httpReq)
	return rec
}

func TestHeartbeat_UpdatesResourcesAndReturnsDesiredState(t *testing.T) {
	st := newFakeStore()
	srv := testServer(t, st)
	token := createJoinToken(t, srv)
	reg := registerNode(t, srv, token, "node-1")

	// Deploy a workload to the node before it ever heartbeats, so the
	// first heartbeat's response should include it as desired state.
	deployBody, _ := json.Marshal(apitypes.CreateWorkloadRequest{Name: "web", Image: "nginx:alpine"})
	deployReq := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/workloads", bytes.NewReader(deployBody))
	deployRec := httptest.NewRecorder()
	srv.ServeHTTP(deployRec, deployReq)
	if deployRec.Code != http.StatusCreated {
		t.Fatalf("create workload: status = %d, want 201 (body: %s)", deployRec.Code, deployRec.Body.String())
	}

	rec := doHeartbeat(t, srv, reg.NodeID, reg.Credential, apitypes.HeartbeatRequest{
		Resources: apitypes.Resources{CPUCores: 4, MemTotalBytes: 1024},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("heartbeat: status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var resp apitypes.HeartbeatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode heartbeat response: %v", err)
	}
	if len(resp.Workloads) != 1 || resp.Workloads[0].Name != "web" {
		t.Errorf("HeartbeatResponse.Workloads = %+v, want one workload named web", resp.Workloads)
	}

	// Resources should now show up in the node list.
	listReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/nodes", nil)
	listRec := httptest.NewRecorder()
	srv.ServeHTTP(listRec, listReq)
	var nodes apitypes.ListNodesResponse
	_ = json.Unmarshal(listRec.Body.Bytes(), &nodes)
	if len(nodes.Nodes) != 1 || nodes.Nodes[0].Resources.CPUCores != 4 {
		t.Errorf("ListNodes = %+v, want CPUCores=4 after heartbeat", nodes.Nodes)
	}
}

func TestHeartbeat_ReportedContainerStatusUpdatesWorkload(t *testing.T) {
	srv := testServer(t, newFakeStore())
	token := createJoinToken(t, srv)
	reg := registerNode(t, srv, token, "node-1")

	deployBody, _ := json.Marshal(apitypes.CreateWorkloadRequest{Name: "web", Image: "nginx:alpine"})
	deployReq := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/workloads", bytes.NewReader(deployBody))
	deployRec := httptest.NewRecorder()
	srv.ServeHTTP(deployRec, deployReq)
	if deployRec.Code != http.StatusCreated {
		t.Fatalf("create workload: status = %d (body: %s)", deployRec.Code, deployRec.Body.String())
	}

	rec := doHeartbeat(t, srv, reg.NodeID, reg.Credential, apitypes.HeartbeatRequest{
		Containers: []apitypes.ContainerStatus{{Name: "web", Image: "nginx:alpine", State: "running", PID: 4242}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("heartbeat: status = %d (body: %s)", rec.Code, rec.Body.String())
	}

	listReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/workloads", nil)
	listRec := httptest.NewRecorder()
	srv.ServeHTTP(listRec, listReq)
	var workloads apitypes.ListWorkloadsResponse
	_ = json.Unmarshal(listRec.Body.Bytes(), &workloads)
	if len(workloads.Workloads) != 1 || workloads.Workloads[0].State != "running" || workloads.Workloads[0].PID != 4242 {
		t.Errorf("ListWorkloads = %+v, want web running with pid 4242", workloads.Workloads)
	}
}

func TestHeartbeat_InvalidCredentialReturnsUnauthorized(t *testing.T) {
	srv := testServer(t, newFakeStore())
	token := createJoinToken(t, srv)
	reg := registerNode(t, srv, token, "node-1")

	rec := doHeartbeat(t, srv, reg.NodeID, "wrong-credential", apitypes.HeartbeatRequest{})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestHeartbeat_MissingBearerTokenReturnsUnauthorized(t *testing.T) {
	srv := testServer(t, newFakeStore())
	token := createJoinToken(t, srv)
	reg := registerNode(t, srv, token, "node-1")

	body, _ := json.Marshal(apitypes.HeartbeatRequest{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/nodes/"+reg.NodeID+"/heartbeat", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestHeartbeat_CredentialNodeIDMismatchReturnsForbidden(t *testing.T) {
	srv := testServer(t, newFakeStore())
	token1 := createJoinToken(t, srv)
	reg1 := registerNode(t, srv, token1, "node-1")
	token2 := createJoinToken(t, srv)
	reg2 := registerNode(t, srv, token2, "node-2")

	// Valid credential (node-1's), but path says node-2.
	rec := doHeartbeat(t, srv, reg2.NodeID, reg1.Credential, apitypes.HeartbeatRequest{})
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (body: %s)", rec.Code, rec.Body.String())
	}
}
