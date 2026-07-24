package rancher

import (
	"context"
	"errors"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/client/norman"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
)

type fakeProjectClient struct {
	clusters        []norman.Cluster
	cluster         *norman.Cluster
	listClustersErr error
	lookupErr       error
	projectErr      map[string]error
}

func (c *fakeProjectClient) ListClusters(context.Context) ([]norman.Cluster, error) {
	return c.clusters, c.listClustersErr
}

func (c *fakeProjectClient) ListProjects(_ context.Context, clusterID string) ([]norman.Project, error) {
	if err := c.projectErr[clusterID]; err != nil {
		return nil, err
	}
	return nil, nil
}

func (c *fakeProjectClient) LookupCluster(context.Context, string) (*norman.Cluster, error) {
	return c.cluster, c.lookupErr
}

func TestProjectToMap(t *testing.T) {
	p := norman.Project{
		Name:        "test-project",
		ClusterID:   "c-abc123",
		State:       "active",
		Description: "A test project",
	}
	p.ID = "p-xyz789"
	p.Created = "2024-01-15T10:30:00Z"

	result := projectToMap(p)

	if result["id"] != "p-xyz789" {
		t.Errorf("expected id 'p-xyz789', got %q", result["id"])
	}
	if result["name"] != "test-project" {
		t.Errorf("expected name 'test-project', got %q", result["name"])
	}
	if result["cluster"] != "c-abc123" {
		t.Errorf("expected cluster 'c-abc123', got %q", result["cluster"])
	}
	if result["state"] != "active" {
		t.Errorf("expected state 'active', got %q", result["state"])
	}
	if result["created"] != "2024-01-15T10:30:00Z" {
		t.Errorf("expected created timestamp, got %q", result["created"])
	}
	if result["description"] != "A test project" {
		t.Errorf("expected description, got %q", result["description"])
	}
}

func TestProjectToMap_EmptyFields(t *testing.T) {
	p := norman.Project{}
	result := projectToMap(p)

	if result["created"] != "-" {
		t.Errorf("expected '-' for empty created, got %q", result["created"])
	}
	if result["description"] != "" {
		t.Errorf("expected empty description, got %q", result["description"])
	}
}

func TestFilterProjectsByName(t *testing.T) {
	projects := []norman.Project{
		{Name: "System"},
		{Name: "Default"},
		{Name: "my-app"},
	}

	t.Run("empty filter returns all", func(t *testing.T) {
		result := filterProjectsByName(projects, "")
		if len(result) != 3 {
			t.Fatalf("expected 3, got %d", len(result))
		}
	})

	t.Run("case insensitive partial match", func(t *testing.T) {
		result := filterProjectsByName(projects, "system")
		if len(result) != 1 || result[0].Name != "System" {
			t.Fatalf("expected [System], got %d items", len(result))
		}
	})

	t.Run("no match", func(t *testing.T) {
		result := filterProjectsByName(projects, "nonexistent")
		if len(result) != 0 {
			t.Fatalf("expected 0, got %d", len(result))
		}
	})
}

func TestFetchProjects_PropagatesClusterProjectError(t *testing.T) {
	wantErr := errors.New("permission denied")
	clusters := []norman.Cluster{{}, {}}
	clusters[0].ID = "c1"
	clusters[1].ID = "c2"
	client := &fakeProjectClient{
		clusters:   clusters,
		projectErr: map[string]error{"c2": wantErr},
	}

	_, err := fetchProjects(context.Background(), client, "")
	if !errors.Is(err, wantErr) {
		t.Fatalf("fetchProjects() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestResolveOptionalProjectCluster_PropagatesLookupError(t *testing.T) {
	wantErr := errors.New("cluster not found")
	client := &fakeProjectClient{lookupErr: wantErr}

	_, err := resolveOptionalProjectCluster(context.Background(), client, map[string]interface{}{paramutil.ParamCluster: "missing"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("resolveOptionalProjectCluster() error = %v, want wrapped %v", err, wantErr)
	}
}
