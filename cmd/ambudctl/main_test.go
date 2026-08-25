// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantSub string
		wantErr bool
	}{
		{
			name:    "version flag prints the version string",
			args:    []string{"--version"},
			wantSub: "dev",
		},
		{
			name:    "no args prints the not-yet-implemented placeholder",
			args:    nil,
			wantSub: "no commands implemented yet",
		},
		{
			name:    "unknown flag returns an error",
			args:    []string{"--nope"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := run(tt.args, &out)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("run(%v) = nil error, want an error", tt.args)
				}
				return
			}
			if err != nil {
				t.Fatalf("run(%v) returned unexpected error: %v", tt.args, err)
			}
			if !strings.Contains(out.String(), tt.wantSub) {
				t.Errorf("run(%v) output = %q, want substring %q", tt.args, out.String(), tt.wantSub)
			}
		})
	}
}
