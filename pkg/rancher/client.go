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
	Cluster          = managementClient.Cluster
	Project          = managementClient.Project
	User             = managementClient.User
	Node             = managementClient.Node
	Pod              = projectClient.Pod
	Workload         = projectClient.Workload
	Namespace        = clusterClient.Namespace
	ConfigMap        = projectClient.ConfigMap
	NamespacedSecret = projectClient.NamespacedSecret
	Service          = projectClient.Service
	Ingress          = projectClient.Ingress
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
		Insecure:  c.management.Opts.Insecure,
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
		Insecure:  c.management.Opts.Insecure,
	}

	clusterClient, err := clusterClient.NewClient(clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster client: %w", err)
	}

	c.clusterClients[clusterID] = clusterClient
	return clusterClient, nil
}

// ListConfigMaps returns all configmaps for a cluster and project
func (c *Client) ListConfigMaps(ctx context.Context, clusterID, projectID string) ([]projectClient.ConfigMap, error) {
	if !c.configured {
		return nil, fmt.Errorf("Rancher client not configured")
	}

	// Get or create project client
	projectClient, err := c.getProjectClient(clusterID, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project client for cluster %s, project %s: %w", clusterID, projectID, err)
	}

	configMapList, err := projectClient.ConfigMap.List(&types.ListOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to list configmaps for cluster %s, project %s: %w", clusterID, projectID, err)
	}

	return configMapList.Data, nil
}

// ListSecrets returns all namespaced secrets for a cluster and project
func (c *Client) ListSecrets(ctx context.Context, clusterID, projectID string) ([]projectClient.NamespacedSecret, error) {
	if !c.configured {
		return nil, fmt.Errorf("Rancher client not configured")
	}

	// Get or create project client
	projectClient, err := c.getProjectClient(clusterID, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project client for cluster %s, project %s: %w", clusterID, projectID, err)
	}

	// Use NamespacedSecret instead of Secret for project-scoped secrets
	secretList, err := projectClient.NamespacedSecret.List(&types.ListOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets for cluster %s, project %s: %w", clusterID, projectID, err)
	}

	return secretList.Data, nil
}

// ListServices returns all services for a cluster and project
func (c *Client) ListServices(ctx context.Context, clusterID, projectID string) ([]projectClient.Service, error) {
	if !c.configured {
		return nil, fmt.Errorf("Rancher client not configured")
	}

	// Get or create project client
	projectClient, err := c.getProjectClient(clusterID, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project client for cluster %s, project %s: %w", clusterID, projectID, err)
	}

	serviceList, err := projectClient.Service.List(&types.ListOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to list services for cluster %s, project %s: %w", clusterID, projectID, err)
	}

	return serviceList.Data, nil
}

// GetIngress gets a single ingress for a cluster, project, and namespace by name
func (c *Client) GetIngress(ctx context.Context, clusterID, projectID, namespace, name string) (*projectClient.Ingress, error) {
	if !c.configured {
		return nil, fmt.Errorf("Rancher client not configured")
	}

	// Get or create project client
	projectClient, err := c.getProjectClient(clusterID, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project client for cluster %s, project %s: %w", clusterID, projectID, err)
	}

	// Get the ingress by ID
	ingressID := fmt.Sprintf("%s:%s", namespace, name)
	ingress, err := projectClient.Ingress.ByID(ingressID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ingress %s in namespace %s for cluster %s, project %s: %w", name, namespace, clusterID, projectID, err)
	}

	return ingress, nil
}

// ListIngresses returns all ingresses for a cluster and project
func (c *Client) ListIngresses(ctx context.Context, clusterID, projectID string) ([]projectClient.Ingress, error) {
	if !c.configured {
		return nil, fmt.Errorf("Rancher client not configured")
	}

	// Get or create project client
	projectClient, err := c.getProjectClient(clusterID, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project client for cluster %s, project %s: %w", clusterID, projectID, err)
	}

	ingressList, err := projectClient.Ingress.List(&types.ListOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to list ingresses for cluster %s, project %s: %w", clusterID, projectID, err)
	}

	return ingressList.Data, nil
}

// GetNode gets a single node for a cluster by ID
func (c *Client) GetNode(ctx context.Context, clusterID, nodeID string) (*managementClient.Node, error) {
	if !c.configured {
		return nil, fmt.Errorf("Rancher client not configured")
	}

	node, err := c.management.Node.ByID(nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node %s: %w", nodeID, err)
	}

	// Verify the node belongs to the specified cluster
	if node.ClusterID != clusterID {
		return nil, fmt.Errorf("node %s does not belong to cluster %s", nodeID, clusterID)
	}

	return node, nil
}

// GetWorkload gets a single workload for a cluster and project by name and namespace
func (c *Client) GetWorkload(ctx context.Context, clusterID, projectID, namespace, name string) (*projectClient.Workload, error) {
	if !c.configured {
		return nil, fmt.Errorf("Rancher client not configured")
	}

	// Get or create project client
	projectClient, err := c.getProjectClient(clusterID, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project client for cluster %s, project %s: %w", clusterID, projectID, err)
	}

	// Construct workload ID: namespace:name
	workloadID := fmt.Sprintf("%s:%s", namespace, name)
	workload, err := projectClient.Workload.ByID(workloadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workload %s in namespace %s for cluster %s, project %s: %w", name, namespace, clusterID, projectID, err)
	}

	return workload, nil
}

// GetNamespace gets a single namespace for a cluster by name
func (c *Client) GetNamespace(ctx context.Context, clusterID, name string) (*clusterClient.Namespace, error) {
	if !c.configured {
		return nil, fmt.Errorf("Rancher client not configured")
	}

	// Get or create cluster client
	clusterClient, err := c.getClusterClient(clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster client for cluster %s: %w", clusterID, err)
	}

	// Construct namespace ID: name
	// For namespaces, the ID is just the name
	namespace, err := clusterClient.Namespace.ByID(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace %s for cluster %s: %w", name, clusterID, err)
	}

	return namespace, nil
}

// GetConfigMap gets a single configmap for a cluster and project by name and namespace
func (c *Client) GetConfigMap(ctx context.Context, clusterID, projectID, namespace, name string) (*projectClient.ConfigMap, error) {
	if !c.configured {
		return nil, fmt.Errorf("Rancher client not configured")
	}

	// Get or create project client
	projectClient, err := c.getProjectClient(clusterID, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project client for cluster %s, project %s: %w", clusterID, projectID, err)
	}

	// Construct configmap ID: namespace:name
	configMapID := fmt.Sprintf("%s:%s", namespace, name)
	configMap, err := projectClient.ConfigMap.ByID(configMapID)
	if err != nil {
		return nil, fmt.Errorf("failed to get configmap %s in namespace %s for cluster %s, project %s: %w", name, namespace, clusterID, projectID, err)
	}

	return configMap, nil
}

// GetSecret gets a single namespaced secret for a cluster and project by name and namespace
func (c *Client) GetSecret(ctx context.Context, clusterID, projectID, namespace, name string) (*projectClient.NamespacedSecret, error) {
	if !c.configured {
		return nil, fmt.Errorf("Rancher client not configured")
	}

	// Get or create project client
	projectClient, err := c.getProjectClient(clusterID, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project client for cluster %s, project %s: %w", clusterID, projectID, err)
	}

	// Construct secret ID: namespace:name
	secretID := fmt.Sprintf("%s:%s", namespace, name)
	secret, err := projectClient.NamespacedSecret.ByID(secretID)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s in namespace %s for cluster %s, project %s: %w", name, namespace, clusterID, projectID, err)
	}

	return secret, nil
}

// GetService gets a single service for a cluster, project, and namespace by name
func (c *Client) GetService(ctx context.Context, clusterID, projectID, namespace, name string) (*projectClient.Service, error) {
	if !c.configured {
		return nil, fmt.Errorf("Rancher client not configured")
	}

	projectClient, err := c.getProjectClient(clusterID, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project client for cluster %s, project %s: %w", clusterID, projectID, err)
	}

	// Construct service ID: namespace:name
	serviceID := fmt.Sprintf("%s:%s", namespace, name)
	service, err := projectClient.Service.ByID(serviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get service %s in namespace %s for cluster %s, project %s: %w", name, namespace, clusterID, projectID, err)
	}

	return service, nil
}
