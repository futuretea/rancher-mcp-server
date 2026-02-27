package rancher

import (
	"context"
	"fmt"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/client/norman"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
)

// fetchProjects retrieves projects based on optional cluster filter.
// If clusterID is empty, returns projects from all clusters.
func fetchProjects(ctx context.Context, client *norman.Client, clusterID string) ([]norman.Project, error) {
	if clusterID != "" {
		return client.ListProjects(ctx, clusterID)
	}

	clusters, err := client.ListClusters(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	var allProjects []norman.Project
	for _, c := range clusters {
		projects, err := client.ListProjects(ctx, c.ID)
		if err != nil {
			continue
		}
		allProjects = append(allProjects, projects...)
	}
	return allProjects, nil
}

// projectToMap converts a project to a string map for output formatting.
func projectToMap(p norman.Project) map[string]string {
	return map[string]string{
		"id":          p.ID,
		"name":        p.Name,
		"cluster":     p.ClusterID,
		"state":       p.State,
		"created":     paramutil.FormatTime(p.Created),
		"description": p.Description,
	}
}

// projectListHandler handles the project_list tool.
// Supports fuzzy matching for cluster identifier.
func projectListHandler(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
	normanClient, err := toolset.ValidateNormanClient(client)
	if err != nil {
		return "", err
	}

	format, err := paramutil.ExtractAndValidateFormat(params)
	if err != nil {
		return "", err
	}

	// Extract query and pagination parameters
	nameFilter := paramutil.ExtractOptionalString(params, paramutil.ParamName)
	limit := paramutil.ExtractInt64(params, paramutil.ParamLimit, 100)
	page := paramutil.ExtractInt64(params, paramutil.ParamPage, 1)

	clusterID, _ := paramutil.ResolveOptionalCluster(ctx, normanClient, params)

	allProjects, err := fetchProjects(ctx, normanClient, clusterID)
	if err != nil {
		return "", err
	}

	// Apply name filter
	filtered := filterProjectsByName(allProjects, nameFilter)

	// Apply pagination
	paginated, _ := paramutil.ApplyPagination(filtered, limit, page)

	projectMaps := make([]map[string]string, len(paginated))
	for i, p := range paginated {
		projectMaps[i] = projectToMap(p)
	}

	return paramutil.FormatOutput(projectMaps, format, []string{"id", "name", "cluster", "state", "created", "description"}, nil)
}

// filterProjectsByName filters projects by name (partial match, case-insensitive).
func filterProjectsByName(projects []norman.Project, name string) []norman.Project {
	if name == "" {
		return projects
	}
	var result []norman.Project
	for _, p := range projects {
		if strings.Contains(strings.ToLower(p.Name), strings.ToLower(name)) {
			result = append(result, p)
		}
	}
	return result
}
