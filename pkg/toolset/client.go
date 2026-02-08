package toolset

import (
	"github.com/futuretea/rancher-mcp-server/pkg/client/norman"
	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/handler"
)

// CombinedClient holds both Norman and Steve clients.
type CombinedClient struct {
	Norman *norman.Client
	Steve  *steve.Client
}

// ValidateNormanClient validates and returns a configured Norman client.
// Returns ErrRancherNotConfigured if the client is nil or not configured.
func ValidateNormanClient(client interface{}) (*norman.Client, error) {
	// Check if it's a CombinedClient first
	if combined, ok := client.(*CombinedClient); ok {
		if combined.Norman == nil {
			return nil, handler.ErrRancherNotConfigured
		}
		return combined.Norman, nil
	}

	// Legacy: direct Norman client
	normanClient, ok := client.(*norman.Client)
	if !ok || normanClient == nil {
		return nil, handler.ErrRancherNotConfigured
	}
	return normanClient, nil
}

// ValidateSteveClient validates and returns a configured Steve client.
// Returns ErrSteveNotConfigured if the client is nil.
func ValidateSteveClient(client interface{}) (*steve.Client, error) {
	// Check if it's a CombinedClient first
	if combined, ok := client.(*CombinedClient); ok {
		if combined.Steve == nil {
			return nil, handler.ErrSteveNotConfigured
		}
		return combined.Steve, nil
	}

	// Direct Steve client
	steveClient, ok := client.(*steve.Client)
	if !ok || steveClient == nil {
		return nil, handler.ErrSteveNotConfigured
	}
	return steveClient, nil
}
