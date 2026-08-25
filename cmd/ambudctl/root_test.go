// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCmd_VersionSubcommand(t *testing.T) {
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if got := out.String(); !strings.Contains(got, "dev") {
		t.Errorf("output = %q, want it to contain the default dev version", got)
	}
}

func TestRootCmd_UnknownCommandFails(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"does-not-exist"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute() error = nil, want an error for an unknown subcommand")
	}
}

func TestRootCmd_RunRequiresExactlyOneArg(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"run"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute() error = nil, want an error when IMAGE is missing")
	}
}
