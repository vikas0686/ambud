// SPDX-License-Identifier: Apache-2.0

package httputil

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, 201, map[string]string{"hello": "world"})

	if rec.Code != 201 {
		t.Errorf("status = %d, want 201", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["hello"] != "world" {
		t.Errorf("body = %v, want {hello: world}", body)
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, 404, "not found")

	if rec.Code != 404 {
		t.Errorf("status = %d, want 404", rec.Code)
	}

	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error != "not found" {
		t.Errorf("body.Error = %q, want %q", body.Error, "not found")
	}
}
