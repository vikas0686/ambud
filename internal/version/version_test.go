// SPDX-License-Identifier: Apache-2.0

package version

import (
	"strings"
	"testing"
)

func TestString(t *testing.T) {
	t.Cleanup(func() {
		Version = "dev"
		Commit = "unknown"
		BuildDate = "unknown"
	})

	Version = "v0.1.0"
	Commit = "abc1234"
	BuildDate = "2026-08-25T00:00:00Z"

	got := String()

	for _, want := range []string{Version, Commit, BuildDate} {
		if !strings.Contains(got, want) {
			t.Errorf("String() = %q, want it to contain %q", got, want)
		}
	}
}
