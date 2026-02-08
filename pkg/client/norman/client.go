// Package norman provides the Norman API client for Rancher management operations.
// Norman is Rancher's v3 API used for managing clusters, projects, and users.
package norman

import (
	"context"
	"fmt"

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

// NewClient creates a new Norman API client
func NewClient(cfg *config.StaticConfig) (*Client, error) {
	if !cfg.HasRancherConfig() {
		return &Client{}, nil
	}

	// Create management client configuration
	// Use GetNormanURL to ensure /v3 suffix regardless of user input
	clientOpts := &clientbase.ClientOpts{
		URL:       urlutil.GetNormanURL(cfg.RancherServerURL),
		AccessKey: cfg.RancherAccessKey,
		SecretKey: cfg.RancherSecretKey,
		TokenKey:  cfg.RancherToken,
		Insecure:  cfg.RancherTLSInsecure,
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


// ListClusters returns all clusters
func (c *Client) ListClusters(ctx context.Context) ([]managementClient.Cluster, error) {
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
func (c *Client) ListProjects(ctx context.Context, clusterID string) ([]managementClient.Project, error) {
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
func (c *Client) ListUsers(ctx context.Context) ([]managementClient.User, error) {
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
func (c *Client) LookupCluster(ctx context.Context, clusterID string) (*Cluster, error) {
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
func (c *Client) LookupProject(ctx context.Context, clusterID, projectID string) (*Project, error) {
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
func (c *Client) LookupUser(ctx context.Context, userID string) (*User, error) {
	if c.management == nil {
		return nil, ErrNotConfigured
	}

	user, err := c.management.User.ByID(userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: '%s' (use user_list to get available user IDs)", userID)
	}
	return user, nil
}
