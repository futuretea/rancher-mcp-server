package common

import (
	"github.com/futuretea/rancher-mcp-server/pkg/rancher"
)

// ValidateRancherClient validates and returns a configured Rancher client
func ValidateRancherClient(client interface{}) (*rancher.Client, error) {
	rancherClient, ok := client.(*rancher.Client)
	if !ok || rancherClient == nil || !rancherClient.IsConfigured() {
		return nil, ErrRancherNotConfigured
	}
	return rancherClient, nil
}
