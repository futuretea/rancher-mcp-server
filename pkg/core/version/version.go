package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the current version of the Rancher MCP Server
	Version = "0.1.0"

	// GitCommit is the git commit hash
	GitCommit = "unknown"

	// BuildDate is the build date
	BuildDate = "unknown"

	// GoVersion is the Go runtime version
	GoVersion = runtime.Version()

	// Platform is the build platform
	Platform = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
)

// BinaryName is the name of the binary
const BinaryName = "rancher-mcp-server"

// GetVersionInfo returns formatted version information
func GetVersionInfo() string {
	return fmt.Sprintf(`%s
Version:    %s
Git commit: %s
Built:      %s
Go version: %s
Platform:   %s`,
		BinaryName, Version, GitCommit, BuildDate, GoVersion, Platform)
}