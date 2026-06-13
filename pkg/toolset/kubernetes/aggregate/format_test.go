package aggregate

import (
	"strings"
	"testing"
	"time"

	"github.com/futuretea/rancher-mcp-server/pkg/toolset/kubernetes/internal/formatutil"
)

func TestFormatResult_JSON(t *testing.T) {
	result := &TopResult{
		Items: []TopItem{
			{Name: "pod-1", Namespace: "default", CPUReq: 100, MemReq: 128 * 1024 * 1024},
		},
		Total: 1,
	}

	out, err := FormatResult(result, "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `"name": "pod-1"`) {
		t.Errorf("expected JSON output to contain pod name")
	}
}

func TestFormatResult_YAML(t *testing.T) {
	result := &TopResult{
		Items: []TopItem{
			{Name: "pod-1", Namespace: "default", CPUReq: 100},
		},
		Total: 1,
	}

	out, err := FormatResult(result, "yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "name: pod-1") {
		t.Errorf("expected YAML output to contain pod name")
	}
}

func TestFormatResult_TableTop(t *testing.T) {
	result := &TopResult{
		Items: []TopItem{
			{Name: "pod-1", Namespace: "default", CPUReq: 500, CPUUtil: 820, MemReq: 256 * 1024 * 1024, MemUtil: 412 * 1024 * 1024, Restarts: 3},
			{Name: "pod-2", Namespace: "default", CPUReq: 500, CPUUtil: 750, MemReq: 256 * 1024 * 1024, MemUtil: 380 * 1024 * 1024, Restarts: 1},
		},
		Total: 2,
	}

	out, err := FormatResult(result, "table")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "NAME") {
		t.Errorf("expected table header to contain NAME")
	}
	if !strings.Contains(out, "pod-1") {
		t.Errorf("expected table to contain pod-1")
	}
	if !strings.Contains(out, "500m") {
		t.Errorf("expected table to contain formatted CPU request")
	}
}

func TestFormatResult_TableWorkload(t *testing.T) {
	result := &WorkloadResult{
		Items: []WorkloadItem{
			{Name: "api-gateway", Namespace: "prod", Kind: "Deployment", Ready: 8, Desired: 10, Unavailable: 2, Updated: 10, Status: "Progressing"},
			{Name: "order-svc", Namespace: "prod", Kind: "Deployment", Ready: 5, Desired: 5, Unavailable: 0, Updated: 5, Status: "Healthy"},
		},
		Total: 2,
	}

	out, err := FormatResult(result, "table")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "api-gateway") {
		t.Errorf("expected table to contain api-gateway")
	}
	if !strings.Contains(out, "Progressing") {
		t.Errorf("expected table to contain Progressing status")
	}
}

func TestFormatResult_TableSummary(t *testing.T) {
	result := &SummaryResult{
		Items: []SummaryItem{
			{Group: "production", PodCount: 42, CPUReq: 24500, CPULimit: 48000, MemReq: 64 * 1024 * 1024 * 1024, MemLimit: 128 * 1024 * 1024 * 1024},
		},
		Total: 1,
	}

	out, err := FormatResult(result, "table")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "production") {
		t.Errorf("expected table to contain production group")
	}
	if !strings.Contains(out, "42") {
		t.Errorf("expected table to contain pod count")
	}
}

func TestFormatResult_TableEvent(t *testing.T) {
	result := &EventResult{
		Items: []EventItem{
			{Reason: "FailedScheduling", Kind: "Pod", Namespace: "default", Count: 15, LastSeen: time.Now().Add(-2 * time.Minute)},
		},
		Total: 1,
	}

	out, err := FormatResult(result, "table")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "FailedScheduling") {
		t.Errorf("expected table to contain FailedScheduling")
	}
	if !strings.Contains(out, "15") {
		t.Errorf("expected table to contain count 15")
	}
}

func TestFormatResult_UnsupportedTableType(t *testing.T) {
	_, err := FormatResult("not-a-result", "table")
	if err == nil {
		t.Errorf("expected error for unsupported result type")
	}
}

func TestFormatResult_EmptyResults(t *testing.T) {
	tests := []struct {
		name   string
		result interface{}
		want   string
	}{
		{"top", &TopResult{Items: []TopItem{}}, "No resources found"},
		{"workload", &WorkloadResult{Items: []WorkloadItem{}}, "No workloads found"},
		{"summary", &SummaryResult{Items: []SummaryItem{}}, "No resources found"},
		{"event", &EventResult{Items: []EventItem{}}, "No events found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := FormatResult(tt.result, "table")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(out, tt.want) {
				t.Errorf("expected %q in output, got %q", tt.want, out)
			}
		})
	}
}

func TestFormatCPU(t *testing.T) {
	tests := []struct {
		val  int64
		want string
	}{
		{0, "0m"},
		{100, "100m"},
		{999, "999m"},
		{1000, "1.00c"},
		{2500, "2.50c"},
	}
	for _, tt := range tests {
		got := formatCPU(tt.val)
		if got != tt.want {
			t.Errorf("formatCPU(%d) = %s, want %s", tt.val, got, tt.want)
		}
	}
}

func TestFormatMemory(t *testing.T) {
	tests := []struct {
		val  int64
		want string
	}{
		{0, "0"},
		{512, "512"},
		{1024, "1.00Ki"},
		{1024 * 1024, "1.00Mi"},
		{1024 * 1024 * 1024, "1.00Gi"},
		{2 * 1024 * 1024 * 1024, "2.00Gi"},
	}
	for _, tt := range tests {
		got := formatMemory(tt.val)
		if got != tt.want {
			t.Errorf("formatMemory(%d) = %s, want %s", tt.val, got, tt.want)
		}
	}
}

func TestFormatAge(t *testing.T) {
	now := time.Now()
	tests := []struct {
		t    time.Time
		want string
	}{
		{time.Time{}, ""},
		{now.Add(-30 * time.Second), "30s"},
		{now.Add(-5 * time.Minute), "5m"},
		{now.Add(-3 * time.Hour), "3h"},
		{now.Add(-48 * time.Hour), "2d"},
	}
	for _, tt := range tests {
		got := formatAge(tt.t)
		if got != tt.want {
			t.Errorf("formatAge(%v) = %s, want %s", tt.t, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := formatutil.Truncate("hello", 10); got != "hello" {
		t.Errorf("truncate('hello', 10) = %s, want 'hello'", got)
	}
	if got := formatutil.Truncate("hello world this is long", 10); got != "hello w..." {
		t.Errorf("truncate long string = %s, want 'hello w...'", got)
	}
}
