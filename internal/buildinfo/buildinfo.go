// Package buildinfo carries the identity of the binary that is running.
//
// The values are injected at link time with -ldflags -X. They are variables
// rather than constants for exactly that reason, and nothing else in the
// program may assign to them.
//
// The defaults matter as much as the injected values. A binary built with
// `go build` and no flags — which is what every developer runs, and what
// `go test` builds — reports "dev" and "unknown" rather than an empty string.
// An empty version renders as `"version": ""` in a health response and as a
// blank column in a dashboard, where it reads as a bug in the reporting rather
// than as a build that was never stamped.
package buildinfo

// Injected by the linker. See the LDFLAGS variable in the Makefile and the
// build stage in the Dockerfile; the two must agree on these paths, because a
// typo in an -X path is silently ignored by the linker.
var (
	// Version is the release identity: a semver tag in CI, "dev" otherwise.
	Version = "dev"

	// Commit is the full git SHA the binary was built from. It is the only
	// value here that can identify a build unambiguously, because two
	// different commits can carry the same version during development.
	Commit = "unknown"

	// BuildTime is an RFC 3339 timestamp. It answers "is this pod running the
	// image I pushed twenty minutes ago?", which the SHA alone cannot when a
	// tag has been rebuilt.
	BuildTime = "unknown"
)
