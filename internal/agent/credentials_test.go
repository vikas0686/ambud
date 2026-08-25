// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCredential_MissingFileIsNotAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")

	cred, found, err := LoadCredential(path)
	if err != nil {
		t.Fatalf("LoadCredential() error = %v, want nil", err)
	}
	if found {
		t.Errorf("LoadCredential() found = true, want false for a missing file")
	}
	if cred != (NodeCredential{}) {
		t.Errorf("LoadCredential() = %+v, want the zero value when not found", cred)
	}
}

func TestSaveAndLoadCredential_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "credentials.json")
	want := NodeCredential{NodeID: "node-123", Credential: "secret-token"}

	if err := SaveCredential(path, want); err != nil {
		t.Fatalf("SaveCredential() error = %v, want nil", err)
	}

	got, found, err := LoadCredential(path)
	if err != nil {
		t.Fatalf("LoadCredential() error = %v, want nil", err)
	}
	if !found {
		t.Fatal("LoadCredential() found = false, want true after Save")
	}
	if got != want {
		t.Errorf("LoadCredential() = %+v, want %+v", got, want)
	}
}

func TestSaveCredential_FilePermissionsAreOwnerOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	if err := SaveCredential(path, NodeCredential{NodeID: "n", Credential: "c"}); err != nil {
		t.Fatalf("SaveCredential() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat credential file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("credential file mode = %o, want 0600", perm)
	}
}
