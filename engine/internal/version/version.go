// Package version holds the single source of truth for the engine version.
package version

// Version is stamped at release time with:
// go build -ldflags "-X github.com/runtz-dev/runtz/engine/internal/version.Version=1.0.0-rc1"
var Version = "dev"
