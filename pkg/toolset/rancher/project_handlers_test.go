package rancher

import (
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/client/norman"
)

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
