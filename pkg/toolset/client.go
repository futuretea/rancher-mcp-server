// Package toolset defines the framework for MCP tool registration, including
// combined client access, annotations, and tool grouping.
package toolset

import (
	"context"

	"github.com/futuretea/rancher-mcp-server/pkg/client/norman"
	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
)

// CombinedClient holds both Norman and Steve clients.
type CombinedClient struct {
	Norman    *norman.Client
	Steve     *steve.Client
	closeable bool
}

// NewCombinedClient creates a CombinedClient. When closeable is false, Close is a no-op.
func NewCombinedClient(normanClient *norman.Client, steveClient *steve.Client, closeable bool) *CombinedClient {
	return &CombinedClient{
		Norman:    normanClient,
		Steve:     steveClient,
		closeable: closeable,
	}
}

// Close releases request-scoped resources when the client is closeable.
// For static clients it is a no-op so shared connection pools are not closed.
func (c *CombinedClient) Close() {
	if c == nil || !c.closeable {
		return
	}
	if c.Norman != nil {
		c.Norman.Close()
	}
	if c.Steve != nil {
		c.Steve.Close()
	}
}

// IsCloseable reports whether Close() will release request-scoped resources.
func (c *CombinedClient) IsCloseable() bool {
	return c != nil && c.closeable
}

// ClientResolver resolves a CombinedClient for the current request.
type ClientResolver interface {
	Resolve(ctx context.Context) (*CombinedClient, error)
}

// ValidateNormanClient validates and returns a configured Norman client.
// Returns ErrRancherNotConfigured if the client is nil or not configured.
func ValidateNormanClient(client interface{}) (*norman.Client, error) {
	// Check if it's a CombinedClient first
	if combined, ok := client.(*CombinedClient); ok {
		if combined.Norman == nil || !combined.Norman.IsUsable() {
			return nil, paramutil.ErrRancherNotConfigured
		}
		return combined.Norman, nil
	}

	// Legacy: direct Norman client
	normanClient, ok := client.(*norman.Client)
	if !ok || normanClient == nil || !normanClient.IsUsable() {
		return nil, paramutil.ErrRancherNotConfigured
	}
	return normanClient, nil
}

// ValidateSteveClient validates and returns a configured Steve client.
// Returns ErrSteveNotConfigured if the client is nil.
func ValidateSteveClient(client interface{}) (*steve.Client, error) {
	// Check if it's a CombinedClient first
	if combined, ok := client.(*CombinedClient); ok {
		if combined.Steve == nil {
			return nil, paramutil.ErrSteveNotConfigured
		}
		return combined.Steve, nil
	}

	// Direct Steve client
	steveClient, ok := client.(*steve.Client)
	if !ok || steveClient == nil {
		return nil, paramutil.ErrSteveNotConfigured
	}
	return steveClient, nil
}
