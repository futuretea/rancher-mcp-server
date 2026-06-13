// Package norman provides the Norman API client for Rancher management operations.
// Norman is Rancher's v3 API used for managing clusters, projects, and users.
package norman

import (
	"context"
	"fmt"
	"net/http"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/types"
	managementClient "github.com/rancher/rancher/pkg/client/generated/management/v3"

	"github.com/futuretea/rancher-mcp-server/pkg/core/config"
	urlutil "github.com/futuretea/rancher-mcp-server/pkg/util/url"
)

// ErrNotConfigured is returned when the Norman client is not properly configured
var ErrNotConfigured = fmt.Errorf("norman client not configured")

// Type aliases for compatibility with existing code
type (
	Cluster = managementClient.Cluster
	Project = managementClient.Project
	User    = managementClient.User
)

// Client wraps the Rancher management client for Norman API operations
type Client struct {
	management *managementClient.Client
}

// IsUsable returns true when the client has an initialized management backend.
func (c *Client) IsUsable() bool {
	return c != nil && c.management != nil
}

// NewClient creates a new Norman API client
func NewClient(cfg *config.StaticConfig) (*Client, error) {
	if !cfg.HasRancherConfig() {
		return &Client{}, nil
	}

	client, err := NewClientWithToken(cfg.RancherServerURL, cfg.RancherToken, cfg.RancherTLSInsecure)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// NewClientWithToken creates a new Norman API client bound to a single request token.
func NewClientWithToken(serverURL, token string, insecure bool) (*Client, error) {
	// Create management client configuration
	// Use GetNormanURL to ensure /v3 suffix regardless of user input
	clientOpts := &clientbase.ClientOpts{
		URL:      urlutil.GetNormanURL(serverURL),
		TokenKey: token,
		Insecure: insecure,
	}

	// Create the management client
	management, err := managementClient.NewClient(clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create management client: %w", err)
	}

	return &Client{
		management: management,
	}, nil
}

// Close releases resources held by the client. After Close the client must not be used.
func (c *Client) Close() {
	if c == nil {
		return
	}
	if c.management != nil {
		if c.management.Ops != nil && c.management.Ops.Client != nil {
			if transport, ok := c.management.Ops.Client.Transport.(*http.Transport); ok {
				transport.CloseIdleConnections()
			}
		}
	}
	c.management = nil
}

// ListClusters returns all clusters
func (c *Client) ListClusters(_ context.Context) ([]managementClient.Cluster, error) {
	if c.management == nil {
		return nil, ErrNotConfigured
	}

	clusterList, err := c.management.Cluster.List(&types.ListOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	return clusterList.Data, nil
}

// GetCluster returns a specific cluster by ID.
// Deprecated: Use LookupCluster instead for better error messages.
func (c *Client) GetCluster(ctx context.Context, clusterID string) (*managementClient.Cluster, error) {
	return c.LookupCluster(ctx, clusterID)
}

// ListProjects returns all projects for a cluster
func (c *Client) ListProjects(_ context.Context, clusterID string) ([]managementClient.Project, error) {
	if c.management == nil {
		return nil, ErrNotConfigured
	}

	projectList, err := c.management.Project.List(&types.ListOpts{
		Filters: map[string]interface{}{
			"clusterId": clusterID,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list projects for cluster %s: %w", clusterID, err)
	}

	return projectList.Data, nil
}

// ListUsers returns all users
func (c *Client) ListUsers(_ context.Context) ([]managementClient.User, error) {
	if c.management == nil {
		return nil, ErrNotConfigured
	}

	userList, err := c.management.User.List(&types.ListOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	return userList.Data, nil
}

// GenerateKubeconfig generates kubeconfig for a cluster
func (c *Client) GenerateKubeconfig(ctx context.Context, clusterID string) (string, error) {
	cluster, err := c.LookupCluster(ctx, clusterID)
	if err != nil {
		return "", err
	}

	config, err := c.management.Cluster.ActionGenerateKubeconfig(cluster)
	if err != nil {
		return "", fmt.Errorf("failed to generate kubeconfig for cluster %s: %w", clusterID, err)
	}

	return config.Config, nil
}

// LookupCluster finds a cluster by ID
// Returns the cluster if found, or a helpful error if not found
func (c *Client) LookupCluster(_ context.Context, clusterID string) (*Cluster, error) {
	if c.management == nil {
		return nil, ErrNotConfigured
	}

	cluster, err := c.management.Cluster.ByID(clusterID)
	if err != nil {
		return nil, fmt.Errorf("cluster not found: '%s' (use cluster_list to get available cluster IDs)", clusterID)
	}
	return cluster, nil
}

// LookupProject finds a project by ID within a cluster
// Returns the project if found, or a helpful error if not found
func (c *Client) LookupProject(_ context.Context, clusterID, projectID string) (*Project, error) {
	if c.management == nil {
		return nil, ErrNotConfigured
	}

	project, err := c.management.Project.ByID(projectID)
	if err != nil || project.ClusterID != clusterID {
		return nil, fmt.Errorf("project not found: '%s' in cluster '%s' (use project_list to get available project IDs)", projectID, clusterID)
	}
	return project, nil
}

// LookupUser finds a user by ID
// Returns the user if found, or a helpful error if not found
func (c *Client) LookupUser(_ context.Context, userID string) (*User, error) {
	if c.management == nil {
		return nil, ErrNotConfigured
	}

	user, err := c.management.User.ByID(userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: '%s' (use user_list to get available user IDs)", userID)
	}
	return user, nil
}
