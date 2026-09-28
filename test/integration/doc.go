// Package integration holds Docker-backed integration tests that run against
// real Rancher releases. The test files carry the "integration" build tag, so
// the default `go test ./...` run stays fast and Docker-free.
// Namespace allowlist tests exercise the built server over MCP HTTP with both
// Rancher proxy and direct kubeconfig access, including configuration precedence,
// scoped reads, rejected writes, and persisted resource state.
//
// Run them with:
//
//	go test -tags=integration -timeout 45m ./test/integration/...
package integration
