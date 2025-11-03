package rancher

import (
	"context"
	"fmt"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/types"
	clusterClient "github.com/rancher/rancher/pkg/client/generated/cluster/v3"
	managementClient "github.com/rancher/rancher/pkg/client/generated/management/v3"
	projectClient "github.com/rancher/rancher/pkg/client/generated/project/v3"

	"github.com/futuretea/rancher-mcp-server/pkg/config"
)

// Type aliases for compatibility with existing code
type (
	Cluster   = managementClient.Cluster
	Project   = managementClient.Project
	User      = managementClient.User
	Node      = managementClient.Node
	Pod       = projectClient.Pod
	Workload  = projectClient.Workload
	Namespace = clusterClient.Namespace
)

// Client wraps the Rancher generated clients
type Client struct {
	management            *managementClient.Client
	projectClients        map[string]*projectClient.Client
	clusterClients        map[string]*clusterClient.Client
	configured            bool
}

// NewClient creates a new Rancher client using the generated management client
func NewClient(cfg *config.StaticConfig) (*Client, error) {
	if !cfg.HasRancherConfig() {
		return &Client{configured: false}, nil
	}

	// Create management client configuration
	clientOpts := &clientbase.ClientOpts{
		URL:       cfg.RancherServerURL,
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
		management:     management,
		projectClients: make(map[string]*projectClient.Client),
		clusterClients: make(map[string]*clusterClient.Client),
		configured:     true,
	}, nil
}

// IsConfigured returns whether the client is properly configured
func (c *Client) IsConfigured() bool {
	return c.configured
}

// ListClusters returns all clusters
func (c *Client) ListClusters(ctx context.Context) ([]managementClient.Cluster, error) {
	if !c.configured {
		return nil, fmt.Errorf("Rancher client not configured")
	}

	clusterList, err := c.management.Cluster.List(&types.ListOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	return clusterList.Data, nil
}

// GetCluster returns a specific cluster
func (c *Client) GetCluster(ctx context.Context, clusterID string) (*managementClient.Cluster, error) {
	if !c.configured {
		return nil, fmt.Errorf("Rancher client not configured")
	}

	cluster, err := c.management.Cluster.ByID(clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster %s: %w", clusterID, err)
	}

	return cluster, nil
}

// ListProjects returns all projects for a cluster
func (c *Client) ListProjects(ctx context.Context, clusterID string) ([]managementClient.Project, error) {
	if !c.configured {
		return nil, fmt.Errorf("Rancher client not configured")
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

// GetProject returns a specific project
func (c *Client) GetProject(ctx context.Context, clusterID, projectID string) (*managementClient.Project, error) {
	if !c.configured {
		return nil, fmt.Errorf("Rancher client not configured")
	}

	project, err := c.management.Project.ByID(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project %s: %w", projectID, err)
	}

	// Verify the project belongs to the specified cluster
	if project.ClusterID != clusterID {
		return nil, fmt.Errorf("project %s does not belong to cluster %s", projectID, clusterID)
	}

	return project, nil
}

// ListUsers returns all users
func (c *Client) ListUsers(ctx context.Context) ([]managementClient.User, error) {
	if !c.configured {
		return nil, fmt.Errorf("Rancher client not configured")
	}

	userList, err := c.management.User.List(&types.ListOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	return userList.Data, nil
}

// GetUser returns a specific user
func (c *Client) GetUser(ctx context.Context, userID string) (*managementClient.User, error) {
	if !c.configured {
		return nil, fmt.Errorf("Rancher client not configured")
	}

	user, err := c.management.User.ByID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user %s: %w", userID, err)
	}

	return user, nil
}

// ListNodes returns all nodes for a cluster
func (c *Client) ListNodes(ctx context.Context, clusterID string) ([]managementClient.Node, error) {
	if !c.configured {
		return nil, fmt.Errorf("Rancher client not configured")
	}

	nodeList, err := c.management.Node.List(&types.ListOpts{
		Filters: map[string]interface{}{
			"clusterId": clusterID,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes for cluster %s: %w", clusterID, err)
	}

	return nodeList.Data, nil
}

// GenerateKubeconfig generates kubeconfig for a cluster
func (c *Client) GenerateKubeconfig(ctx context.Context, clusterID string) (string, error) {
	if !c.configured {
		return "", fmt.Errorf("Rancher client not configured")
	}

	cluster, err := c.GetCluster(ctx, clusterID)
	if err != nil {
		return "", fmt.Errorf("failed to get cluster %s: %w", clusterID, err)
	}

	// Generate actual kubeconfig using Rancher API
	config, err := c.management.Cluster.ActionGenerateKubeconfig(cluster)
	if err != nil {
		return "", fmt.Errorf("failed to generate kubeconfig for cluster %s: %w", clusterID, err)
	}

	return config.Config, nil
}

// ListPods returns all pods for a cluster and project
func (c *Client) ListPods(ctx context.Context, clusterID, projectID string) ([]projectClient.Pod, error) {
	if !c.configured {
		return nil, fmt.Errorf("Rancher client not configured")
	}

	// Get or create project client
	projectClient, err := c.getProjectClient(clusterID, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project client for cluster %s, project %s: %w", clusterID, projectID, err)
	}

	podList, err := projectClient.Pod.List(&types.ListOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods for cluster %s, project %s: %w", clusterID, projectID, err)
	}

	return podList.Data, nil
}

// ListWorkloads returns all workloads for a cluster and project
func (c *Client) ListWorkloads(ctx context.Context, clusterID, projectID string) ([]projectClient.Workload, error) {
	if !c.configured {
		return nil, fmt.Errorf("Rancher client not configured")
	}

	// Get or create project client
	projectClient, err := c.getProjectClient(clusterID, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project client for cluster %s, project %s: %w", clusterID, projectID, err)
	}

	workloadList, err := projectClient.Workload.List(&types.ListOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to list workloads for cluster %s, project %s: %w", clusterID, projectID, err)
	}

	return workloadList.Data, nil
}

// ListOrphanPods returns pods without a workloadId for a cluster and project
func (c *Client) ListOrphanPods(ctx context.Context, clusterID, projectID string) ([]projectClient.Pod, error) {
	if !c.configured {
		return nil, fmt.Errorf("Rancher client not configured")
	}

	// Get or create project client
	projectClient, err := c.getProjectClient(clusterID, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project client for cluster %s, project %s: %w", clusterID, projectID, err)
	}

	// List pods with empty workloadId filter
	podList, err := projectClient.Pod.List(&types.ListOpts{
		Filters: map[string]interface{}{
			"workloadId": "",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list orphan pods for cluster %s, project %s: %w", clusterID, projectID, err)
	}

	return podList.Data, nil
}

// ListNamespaces returns all namespaces for a cluster
func (c *Client) ListNamespaces(ctx context.Context, clusterID string) ([]clusterClient.Namespace, error) {
	if !c.configured {
		return nil, fmt.Errorf("Rancher client not configured")
	}

	// Get or create cluster client
	clusterClient, err := c.getClusterClient(clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster client for cluster %s: %w", clusterID, err)
	}

	namespaceList, err := clusterClient.Namespace.List(&types.ListOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces for cluster %s: %w", clusterID, err)
	}

	return namespaceList.Data, nil
}

// getProjectClient gets or creates a project client for the given cluster and project
func (c *Client) getProjectClient(clusterID, projectID string) (*projectClient.Client, error) {
	key := clusterID + "/" + projectID
	if client, exists := c.projectClients[key]; exists {
		return client, nil
	}

	// Create project client configuration with project-specific URL
	// Format: serverURL/projects/projectID
	clientOpts := &clientbase.ClientOpts{
		URL:       c.management.Opts.URL + "/projects/" + projectID,
		AccessKey: c.management.Opts.AccessKey,
		SecretKey: c.management.Opts.SecretKey,
		TokenKey:  c.management.Opts.TokenKey,
	}

	projectClient, err := projectClient.NewClient(clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create project client: %w", err)
	}

	c.projectClients[key] = projectClient
	return projectClient, nil
}

// getClusterClient gets or creates a cluster client for the given cluster
func (c *Client) getClusterClient(clusterID string) (*clusterClient.Client, error) {
	if client, exists := c.clusterClients[clusterID]; exists {
		return client, nil
	}

	// Create cluster client configuration with cluster-specific URL
	clientOpts := &clientbase.ClientOpts{
		URL:       c.management.Opts.URL + "/clusters/" + clusterID,
		AccessKey: c.management.Opts.AccessKey,
		SecretKey: c.management.Opts.SecretKey,
		TokenKey:  c.management.Opts.TokenKey,
	}

	clusterClient, err := clusterClient.NewClient(clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster client: %w", err)
	}

	c.clusterClients[clusterID] = clusterClient
	return clusterClient, nil
}
