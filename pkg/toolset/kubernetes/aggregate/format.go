package aggregate

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

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

// --- Table formatting helpers ---

type tableBuilder struct {
	formats []string
	headers []string
}

func newTableBuilder(format, header string) *tableBuilder {
	return &tableBuilder{
		formats: []string{format},
		headers: []string{header},
	}
}

func (tb *tableBuilder) addColumn(format string, headers ...string) {
	for range headers {
		tb.formats = append(tb.formats, format)
	}
	tb.headers = append(tb.headers, headers...)
}

func (tb *tableBuilder) writeHeader(b *strings.Builder) {
	fmt.Fprintf(b, strings.Join(tb.formats, " ")+"\n", toAnySlice(tb.headers)...)
}

func (tb *tableBuilder) writeSeparator(b *strings.Builder) {
	separators := make([]string, len(tb.headers))
	for i, h := range tb.headers {
		separators[i] = strings.Repeat("-", len(h))
	}
	fmt.Fprintf(b, strings.Join(tb.formats, " ")+"\n", toAnySlice(separators)...)
}

func (tb *tableBuilder) writeRow(b *strings.Builder, values []interface{}) {
	fmt.Fprintf(b, strings.Join(tb.formats, " ")+"\n", values...)
}

func toAnySlice(ss []string) []any {
	result := make([]any, len(ss))
	for i, s := range ss {
		result[i] = s
	}
	return result
}

func truncate(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
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

	tb := newTableBuilder("%-40s", "NAME")
	tb.addColumn("%-15s", "NAMESPACE")
	tb.addColumn("%-12s", "CPU.REQ")
	tb.addColumn("%-12s", "CPU.UTIL")
	tb.addColumn("%-12s", "MEM.REQ")
	tb.addColumn("%-12s", "MEM.UTIL")
	tb.addColumn("%-10s", "RESTARTS")

	tb.writeHeader(&b)
	tb.writeSeparator(&b)

	for _, item := range r.Items {
		row := []interface{}{
			truncate(item.Name, 40),
			truncate(item.Namespace, 15),
			formatCPU(item.CPUReq),
			formatCPU(item.CPUUtil),
			formatMemory(item.MemReq),
			formatMemory(item.MemUtil),
			fmt.Sprintf("%d", item.Restarts),
		}
		tb.writeRow(&b, row)
	}

	return b.String()
}

// --- Workload table ---

func formatWorkloadAsTable(r *WorkloadResult) string {
	if len(r.Items) == 0 {
		return "No workloads found"
	}
	var b strings.Builder

	tb := newTableBuilder("%-40s", "NAME")
	tb.addColumn("%-15s", "NAMESPACE")
	tb.addColumn("%-15s", "KIND")
	tb.addColumn("%-10s", "READY")
	tb.addColumn("%-12s", "UNAVAILABLE")
	tb.addColumn("%-10s", "UPDATED")
	tb.addColumn("%-10s", "AGE")
	tb.addColumn("%-10s", "STATUS")

	tb.writeHeader(&b)
	tb.writeSeparator(&b)

	for _, item := range r.Items {
		row := []interface{}{
			truncate(item.Name, 40),
			truncate(item.Namespace, 15),
			truncate(item.Kind, 15),
			fmt.Sprintf("%d/%d", item.Ready, item.Desired),
			fmt.Sprintf("%d", item.Unavailable),
			fmt.Sprintf("%d", item.Updated),
			item.Age,
			item.Status,
		}
		tb.writeRow(&b, row)
	}

	return b.String()
}

// --- Summary table ---

func formatSummaryAsTable(r *SummaryResult) string {
	if len(r.Items) == 0 {
		return "No resources found"
	}
	var b strings.Builder

	tb := newTableBuilder("%-25s", "GROUP")
	tb.addColumn("%-8s", "PODS")
	tb.addColumn("%-12s", "CPU.REQ")
	tb.addColumn("%-12s", "CPU.LIM")
	tb.addColumn("%-12s", "MEM.REQ")
	tb.addColumn("%-12s", "MEM.LIM")

	tb.writeHeader(&b)
	tb.writeSeparator(&b)

	for _, item := range r.Items {
		row := []interface{}{
			truncate(item.Group, 25),
			fmt.Sprintf("%d", item.PodCount),
			formatCPU(item.CPUReq),
			formatCPU(item.CPULimit),
			formatMemory(item.MemReq),
			formatMemory(item.MemLimit),
		}
		tb.writeRow(&b, row)
	}

	return b.String()
}

// --- Event table ---

func formatEventAsTable(r *EventResult) string {
	if len(r.Items) == 0 {
		return "No events found"
	}
	var b strings.Builder

	tb := newTableBuilder("%-25s", "REASON")
	tb.addColumn("%-15s", "KIND")
	tb.addColumn("%-15s", "NAMESPACE")
	tb.addColumn("%-8s", "COUNT")
	tb.addColumn("%-15s", "LAST_SEEN")

	tb.writeHeader(&b)
	tb.writeSeparator(&b)

	for _, item := range r.Items {
		row := []interface{}{
			truncate(item.Reason, 25),
			truncate(item.Kind, 15),
			truncate(item.Namespace, 15),
			fmt.Sprintf("%d", item.Count),
			formatAge(item.LastSeen),
		}
		tb.writeRow(&b, row)
	}

	return b.String()
}
