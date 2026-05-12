package aggregate

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDeriveWorkloadStatus(t *testing.T) {
	tests := []struct {
		name   string
		item   WorkloadItem
		want   string
	}{
		{
			name: "healthy",
			item: WorkloadItem{Ready: 5, Desired: 5, Unavailable: 0},
			want: "Healthy",
		},
		{
			name: "progressing",
			item: WorkloadItem{Ready: 8, Desired: 10, Unavailable: 2},
			want: "Progressing",
		},
		{
			name: "degraded",
			item: WorkloadItem{Ready: 3, Desired: 5, Unavailable: 0},
			want: "Degraded",
		},
		{
			name: "scaled_to_zero",
			item: WorkloadItem{Ready: 0, Desired: 0, Unavailable: 0},
			want: "ScaledToZero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveWorkloadStatus(tt.item)
			if got != tt.want {
				t.Errorf("deriveWorkloadStatus() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestExtractWorkloadItem(t *testing.T) {
	dep := unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "test-dep",
				"namespace": "default",
			},
			"status": map[string]interface{}{
				"replicas":            int64(10),
				"readyReplicas":       int64(8),
				"updatedReplicas":     int64(10),
				"unavailableReplicas": int64(2),
			},
		},
	}

	item := extractWorkloadItem(dep, "deployment")
	if item.Name != "test-dep" {
		t.Errorf("expected name test-dep, got %s", item.Name)
	}
	if item.Kind != "Deployment" {
		t.Errorf("expected kind Deployment, got %s", item.Kind)
	}
	if item.Desired != 10 {
		t.Errorf("expected Desired 10, got %d", item.Desired)
	}
	if item.Ready != 8 {
		t.Errorf("expected Ready 8, got %d", item.Ready)
	}
	if item.Unavailable != 2 {
		t.Errorf("expected Unavailable 2, got %d", item.Unavailable)
	}
	if item.Updated != 10 {
		t.Errorf("expected Updated 10, got %d", item.Updated)
	}
	if item.Status != "Progressing" {
		t.Errorf("expected Status Progressing, got %s", item.Status)
	}
}

func TestSortWorkloadItems_ByUnreadyCount(t *testing.T) {
	items := []WorkloadItem{
		{Name: "dep-a", Ready: 5, Desired: 5},   // unready = 0
		{Name: "dep-b", Ready: 3, Desired: 10},  // unready = 7
		{Name: "dep-c", Ready: 8, Desired: 10},  // unready = 2
	}
	sortWorkloadItems(items, "unready.count")
	if items[0].Name != "dep-b" {
		t.Errorf("expected first item to be dep-b (7 unready), got %s", items[0].Name)
	}
	if items[1].Name != "dep-c" {
		t.Errorf("expected second item to be dep-c (2 unready), got %s", items[1].Name)
	}
	if items[2].Name != "dep-a" {
		t.Errorf("expected third item to be dep-a (0 unready), got %s", items[2].Name)
	}
}

func TestSortWorkloadItems_ByReadyRatio(t *testing.T) {
	items := []WorkloadItem{
		{Name: "dep-a", Ready: 5, Desired: 5},   // 100%
		{Name: "dep-b", Ready: 3, Desired: 10},  // 30%
		{Name: "dep-c", Ready: 8, Desired: 10},  // 80%
	}
	sortWorkloadItems(items, "ready.ratio")
	// Lower ready ratio first (worst first)
	if items[0].Name != "dep-b" {
		t.Errorf("expected first item to be dep-b (30%%), got %s", items[0].Name)
	}
	if items[1].Name != "dep-c" {
		t.Errorf("expected second item to be dep-c (80%%), got %s", items[1].Name)
	}
	if items[2].Name != "dep-a" {
		t.Errorf("expected third item to be dep-a (100%%), got %s", items[2].Name)
	}
}

func TestSortWorkloadItems_ByName(t *testing.T) {
	items := []WorkloadItem{
		{Name: "dep-c"},
		{Name: "dep-a"},
		{Name: "dep-b"},
	}
	sortWorkloadItems(items, "name")
	if items[0].Name != "dep-a" {
		t.Errorf("expected first item to be dep-a, got %s", items[0].Name)
	}
}

func TestCalcRatio(t *testing.T) {
	tests := []struct {
		part, total int32
		want        float64
	}{
		{5, 10, 0.5},
		{0, 10, 0},
		{5, 0, 0},
		{0, 0, 0},
		{10, 10, 1},
	}
	for _, tt := range tests {
		got := calcRatio(tt.part, tt.total)
		if got != tt.want {
			t.Errorf("calcRatio(%d, %d) = %f, want %f", tt.part, tt.total, got, tt.want)
		}
	}
}

func TestCapitalize(t *testing.T) {
	if got := capitalize("deployment"); got != "Deployment" {
		t.Errorf("capitalize('deployment') = %s, want Deployment", got)
	}
	if got := capitalize(""); got != "" {
		t.Errorf("capitalize('') = %s, want empty string", got)
	}
}
