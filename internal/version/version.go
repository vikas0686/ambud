// SPDX-License-Identifier: Apache-2.0

// Package version holds build-time metadata injected via -ldflags at
// compile time. See the Makefile's build target for how Version, Commit,
// and BuildDate are set from git; the zero values below are what a plain
// "go run" or "go build" without ldflags produces.
package version

var (
	// Version is the semantic version of this build, e.g. "v0.1.0".
	Version = "dev"
	// Commit is the short git commit SHA this build was produced from.
	Commit = "unknown"
	// BuildDate is the UTC build timestamp in RFC 3339 format.
	BuildDate = "unknown"
)

// String returns a single-line, human-readable version string suitable
// for a --version flag.
func String() string {
	return Version + " (commit " + Commit + ", built " + BuildDate + ")"
}
