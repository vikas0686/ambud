// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vikas0686/ambud/internal/apitypes"
	"github.com/vikas0686/ambud/internal/runtime"
)

func testServer(t *testing.T, fake *runtime.Fake) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	collector := NewResourceCollector(time.Hour, "/")
	return NewServer(fake, collector, logger)
}

func TestCreateContainer(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		seed       func(*runtime.Fake)
		wantStatus int
	}{
		{
			name:       "success",
			body:       `{"name":"web","image":"nginx:alpine"}`,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "missing name",
			body:       `{"image":"nginx:alpine"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid JSON",
			body:       `{not json`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "name collision",
			body: `{"name":"web","image":"nginx:alpine"}`,
			seed: func(f *runtime.Fake) {
				_ = f.Run(context.Background(), "web", "nginx:alpine")
			},
			wantStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := runtime.NewFake()
			if tt.seed != nil {
				tt.seed(fake)
			}
			srv := testServer(t, fake)

			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/containers", bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}

			if tt.wantStatus == http.StatusCreated {
				var got apitypes.ContainerStatus
				if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if got.Name != "web" || got.State != string(runtime.StateRunning) {
					t.Errorf("response = %+v, want name=web state=running", got)
				}
			}
		})
	}
}

func TestListContainers(t *testing.T) {
	fake := runtime.NewFake()
	if err := fake.Run(context.Background(), "web", "nginx:alpine"); err != nil {
		t.Fatalf("seeding fake failed: %v", err)
	}
	srv := testServer(t, fake)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/containers", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var got apitypes.ListContainersResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Containers) != 1 || got.Containers[0].Name != "web" {
		t.Errorf("Containers = %+v, want one container named web", got.Containers)
	}
}

func TestGetContainer(t *testing.T) {
	fake := runtime.NewFake()
	if err := fake.Run(context.Background(), "web", "nginx:alpine"); err != nil {
		t.Fatalf("seeding fake failed: %v", err)
	}
	srv := testServer(t, fake)

	t.Run("found", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/containers/web", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/containers/does-not-exist", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

func TestStopContainer(t *testing.T) {
	fake := runtime.NewFake()
	if err := fake.Run(context.Background(), "web", "nginx:alpine"); err != nil {
		t.Fatalf("seeding fake failed: %v", err)
	}
	srv := testServer(t, fake)

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/containers/web/stop", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("unknown container", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/containers/does-not-exist/stop", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

func TestRestartContainer(t *testing.T) {
	fake := runtime.NewFake()
	if err := fake.Run(context.Background(), "web", "nginx:alpine"); err != nil {
		t.Fatalf("seeding fake failed: %v", err)
	}
	srv := testServer(t, fake)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/containers/web/restart", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	statuses, err := fake.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(statuses) != 1 || statuses[0].Name != "web" || statuses[0].State != runtime.StateRunning {
		t.Errorf("List() = %+v, want web running after restart", statuses)
	}
}

func TestPullImage(t *testing.T) {
	fake := runtime.NewFake()
	srv := testServer(t, fake)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/images/pull", bytes.NewBufferString(`{"image":"nginx:alpine"}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestGetResources(t *testing.T) {
	fake := runtime.NewFake()
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	collector := NewResourceCollector(time.Hour, "/")
	collector.sample(context.Background()) // one synchronous sample; Run() would block forever on its ticker loop
	srv := NewServer(fake, collector, logger)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/resources", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
}
