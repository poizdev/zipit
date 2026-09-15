// Package buildinfo contains values replaced by linker flags in release builds.
package buildinfo

import "runtime"

var (
	Version    = "dev"
	Commit     = "none"
	BuildDate  = "unknown"
	Prerelease = ""
)

// FullVersion returns the version string with optional prerelease suffix.
func FullVersion() string {
	if Prerelease != "" {
		return Version + "-" + Prerelease
	}
	return Version
}

// GoVersion returns the Go version used to build this binary.
func GoVersion() string {
	return runtime.Version()
}

// Platform returns the OS/architecture this binary was built for.
func Platform() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}
