package aggregate

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/futuretea/rancher-mcp-server/pkg/toolset/kubernetes/internal/formatutil"
	"gopkg.in/yaml.v3"
)

// FormatResult formats a result based on the format string
func FormatResult(v interface{}, format string) (string, error) {
	switch format {
	case "yaml":
		data, err := yaml.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("failed to format as YAML: %w", err)
		}
		return string(data), nil
	case "table":
		switch r := v.(type) {
		case *TopResult:
			return formatTopAsTable(r), nil
		case *WorkloadResult:
			return formatWorkloadAsTable(r), nil
		case *SummaryResult:
			return formatSummaryAsTable(r), nil
		case *EventResult:
			return formatEventAsTable(r), nil
		default:
			return "", fmt.Errorf("unsupported result type for table format: %T", v)
		}
	default: // json
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to format as JSON: %w", err)
		}
		return string(data), nil
	}
}

// formatCPU formats CPU value (millicores) to string
func formatCPU(val int64) string {
	if val < 1000 {
		return fmt.Sprintf("%dm", val)
	}
	return fmt.Sprintf("%.2fc", float64(val)/1000)
}

// formatMemory formats memory value (bytes) to string
func formatMemory(val int64) string {
	const (
		Ki = 1024
		Mi = 1024 * Ki
		Gi = 1024 * Mi
	)
	switch {
	case val >= Gi:
		return fmt.Sprintf("%.2fGi", float64(val)/Gi)
	case val >= Mi:
		return fmt.Sprintf("%.2fMi", float64(val)/Mi)
	case val >= Ki:
		return fmt.Sprintf("%.2fKi", float64(val)/Ki)
	default:
		return fmt.Sprintf("%d", val)
	}
}

func formatAge(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	if d < 0 {
		d = -d
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// --- Top table ---

func formatTopAsTable(r *TopResult) string {
	if len(r.Items) == 0 {
		return "No resources found"
	}
	var b strings.Builder

	tb := formatutil.NewTableBuilder("%-40s", "NAME")
	tb.AddColumn("%-15s", "NAMESPACE")
	tb.AddColumn("%-12s", "CPU.REQ")
	tb.AddColumn("%-12s", "CPU.UTIL")
	tb.AddColumn("%-12s", "MEM.REQ")
	tb.AddColumn("%-12s", "MEM.UTIL")
	tb.AddColumn("%-10s", "RESTARTS")

	tb.WriteHeader(&b)
	tb.WriteSeparator(&b)

	for _, item := range r.Items {
		row := []interface{}{
			formatutil.Truncate(item.Name, 40),
			formatutil.Truncate(item.Namespace, 15),
			formatCPU(item.CPUReq),
			formatCPU(item.CPUUtil),
			formatMemory(item.MemReq),
			formatMemory(item.MemUtil),
			fmt.Sprintf("%d", item.Restarts),
		}
		tb.WriteRow(&b, row)
	}

	return b.String()
}

// --- Workload table ---

func formatWorkloadAsTable(r *WorkloadResult) string {
	if len(r.Items) == 0 {
		return "No workloads found"
	}
	var b strings.Builder

	tb := formatutil.NewTableBuilder("%-40s", "NAME")
	tb.AddColumn("%-15s", "NAMESPACE")
	tb.AddColumn("%-15s", "KIND")
	tb.AddColumn("%-10s", "READY")
	tb.AddColumn("%-12s", "UNAVAILABLE")
	tb.AddColumn("%-10s", "UPDATED")
	tb.AddColumn("%-10s", "AGE")
	tb.AddColumn("%-10s", "STATUS")

	tb.WriteHeader(&b)
	tb.WriteSeparator(&b)

	for _, item := range r.Items {
		row := []interface{}{
			formatutil.Truncate(item.Name, 40),
			formatutil.Truncate(item.Namespace, 15),
			formatutil.Truncate(item.Kind, 15),
			fmt.Sprintf("%d/%d", item.Ready, item.Desired),
			fmt.Sprintf("%d", item.Unavailable),
			fmt.Sprintf("%d", item.Updated),
			item.Age,
			item.Status,
		}
		tb.WriteRow(&b, row)
	}

	return b.String()
}

// --- Summary table ---

func formatSummaryAsTable(r *SummaryResult) string {
	if len(r.Items) == 0 {
		return "No resources found"
	}
	var b strings.Builder

	tb := formatutil.NewTableBuilder("%-25s", "GROUP")
	tb.AddColumn("%-8s", "PODS")
	tb.AddColumn("%-12s", "CPU.REQ")
	tb.AddColumn("%-12s", "CPU.LIM")
	tb.AddColumn("%-12s", "MEM.REQ")
	tb.AddColumn("%-12s", "MEM.LIM")

	tb.WriteHeader(&b)
	tb.WriteSeparator(&b)

	for _, item := range r.Items {
		row := []interface{}{
			formatutil.Truncate(item.Group, 25),
			fmt.Sprintf("%d", item.PodCount),
			formatCPU(item.CPUReq),
			formatCPU(item.CPULimit),
			formatMemory(item.MemReq),
			formatMemory(item.MemLimit),
		}
		tb.WriteRow(&b, row)
	}

	return b.String()
}

// --- Event table ---

func formatEventAsTable(r *EventResult) string {
	if len(r.Items) == 0 {
		return "No events found"
	}
	var b strings.Builder

	tb := formatutil.NewTableBuilder("%-25s", "REASON")
	tb.AddColumn("%-15s", "KIND")
	tb.AddColumn("%-15s", "NAMESPACE")
	tb.AddColumn("%-8s", "COUNT")
	tb.AddColumn("%-15s", "LAST_SEEN")

	tb.WriteHeader(&b)
	tb.WriteSeparator(&b)

	for _, item := range r.Items {
		row := []interface{}{
			formatutil.Truncate(item.Reason, 25),
			formatutil.Truncate(item.Kind, 15),
			formatutil.Truncate(item.Namespace, 15),
			fmt.Sprintf("%d", item.Count),
			formatAge(item.LastSeen),
		}
		tb.WriteRow(&b, row)
	}

	return b.String()
}
