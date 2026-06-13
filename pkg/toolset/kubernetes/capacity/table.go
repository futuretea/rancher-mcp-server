package capacity

import (
	"fmt"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/toolset/kubernetes/internal/formatutil"
)

const (
	milliCPUBase = 1000
	bytesPerKi   = 1024
	bytesPerMi   = 1024 * 1024
	bytesPerGi   = 1024 * 1024 * 1024
)

// FormatAsTable formats capacity result as a human-readable table
func FormatAsTable(result Result, showAvailable bool) string {
	var b strings.Builder

	writeNodeSummary(&b, result, showAvailable)

	if result.ShowUtil {
		writeUtilizationSection(&b, result.Nodes)
	}

	if result.ShowPods {
		writePodsSection(&b, result, showAvailable)
	}

	return b.String()
}

// writeNodeSummary writes the node and cluster summary table
func writeNodeSummary(b *strings.Builder, result Result, showAvailable bool) {
	tb := formatutil.NewTableBuilder("%-25s", "NAME")

	if !result.HideRequests {
		tb.AddColumn("%-12s", "CPU REQUEST", "MEM REQUEST")
	}
	if !result.HideLimits {
		tb.AddColumn("%-12s", "CPU LIMIT", "MEM LIMIT")
	}
	if result.ShowPodCount {
		tb.AddColumn("%-6s", "PODS")
	}
	if result.ShowLabels {
		tb.AddColumn("%-s", "LABELS")
	}

	fmt.Fprintf(b, "NODE\n")
	tb.WriteHeader(b)
	tb.WriteSeparator(b)

	for _, node := range result.Nodes {
		row := []interface{}{truncate(node.Name, 25)}
		if !result.HideRequests {
			row = append(row, formatCPU(node.CPU.Requested, showAvailable), formatMemory(node.Memory.Requested, showAvailable))
		}
		if !result.HideLimits {
			row = append(row, formatCPU(node.CPU.Limited, showAvailable), formatMemory(node.Memory.Limited, showAvailable))
		}
		if result.ShowPodCount {
			row = append(row, fmt.Sprintf("%d/%d", node.PodCount.Requested, node.PodCount.Allocatable))
		}
		if result.ShowLabels {
			row = append(row, formatLabels(node.Labels))
		}
		tb.WriteRow(b, row)
	}

	fmt.Fprintf(b, "\nCLUSTER\n")
	tb.WriteHeader(b)
	tb.WriteSeparator(b)

	row := []interface{}{result.Cluster.Name}
	if !result.HideRequests {
		row = append(row, formatCPU(result.Cluster.CPU.Requested, showAvailable), formatMemory(result.Cluster.Memory.Requested, showAvailable))
	}
	if !result.HideLimits {
		row = append(row, formatCPU(result.Cluster.CPU.Limited, showAvailable), formatMemory(result.Cluster.Memory.Limited, showAvailable))
	}
	if result.ShowPodCount {
		row = append(row, fmt.Sprintf("%d/%d", result.Cluster.PodCount.Requested, result.Cluster.PodCount.Allocatable))
	}
	if result.ShowLabels {
		row = append(row, "")
	}
	tb.WriteRow(b, row)
}

// writeUtilizationSection writes the utilization section
func writeUtilizationSection(b *strings.Builder, nodes []NodeInfo) {
	fmt.Fprintf(b, "\nNODE UTILIZATION\n")
	fmt.Fprintf(b, "%-25s %-12s %-12s %-12s %-12s\n", "NAME", "CPU CAP", "CPU UTIL%", "MEM CAP", "MEM UTIL%")
	fmt.Fprintf(b, "%-25s %-12s %-12s %-12s %-12s\n", "----", "-------", "---------", "-------", "---------")

	for _, node := range nodes {
		fmt.Fprintf(b, "%-25s %-12s %-11.1f%% %-12s %-11.1f%%\n",
			truncate(node.Name, 25),
			formatCPU(node.CPU.Allocatable, true),
			formatutil.CalcPercentage(node.CPU.Utilized, node.CPU.Allocatable),
			formatMemory(node.Memory.Allocatable, true),
			formatutil.CalcPercentage(node.Memory.Utilized, node.Memory.Allocatable),
		)
	}
}

// writePodsSection writes the pods section with optional container details
func writePodsSection(b *strings.Builder, result Result, showAvailable bool) {
	fmt.Fprintf(b, "\nPODS\n")

	for _, node := range result.Nodes {
		if len(node.Pods) == 0 {
			continue
		}

		fmt.Fprintf(b, "\n%s (%d pods)\n", node.Name, len(node.Pods))

		tb := formatutil.NewTableBuilder("  %-40s", "POD")
		tb.AddColumn("%-10s", "NAMESPACE")
		if !result.HideRequests {
			tb.AddColumn("%-12s", "CPU REQUEST", "MEM REQUEST")
		}
		if !result.HideLimits {
			tb.AddColumn("%-12s", "CPU LIMIT", "MEM LIMIT")
		}

		tb.WriteHeader(b)
		tb.WriteSeparator(b)

		for _, pod := range node.Pods {
			row := []interface{}{truncate(pod.Name, 40), truncate(pod.Namespace, 10)}
			if !result.HideRequests {
				row = append(row, formatCPU(pod.CPU.Requested, showAvailable), formatMemory(pod.Memory.Requested, showAvailable))
			}
			if !result.HideLimits {
				row = append(row, formatCPU(pod.CPU.Limited, showAvailable), formatMemory(pod.Memory.Limited, showAvailable))
			}
			tb.WriteRow(b, row)

			if result.ShowContainers {
				writeContainers(b, pod.Containers, result, showAvailable)
			}
		}
	}
}

// writeContainers writes container details for a pod
func writeContainers(b *strings.Builder, containers []ContainerInfo, result Result, showAvailable bool) {
	if len(containers) == 0 {
		return
	}

	tb := formatutil.NewTableBuilder("  %-38s", "[C]")
	if !result.HideRequests {
		tb.AddColumn("%-12s", "CPU REQUEST", "MEM REQUEST")
	}
	if !result.HideLimits {
		tb.AddColumn("%-12s", "CPU LIMIT", "MEM LIMIT")
	}

	for _, c := range containers {
		prefix := "[C]"
		if c.Init {
			prefix = "[I]"
		}
		row := []interface{}{prefix + " " + truncate(c.Name, 35)}
		if !result.HideRequests {
			row = append(row, formatCPU(c.CPU.Requested, showAvailable), formatMemory(c.Memory.Requested, showAvailable))
		}
		if !result.HideLimits {
			row = append(row, formatCPU(c.CPU.Limited, showAvailable), formatMemory(c.Memory.Limited, showAvailable))
		}
		tb.WriteRow(b, row)
	}
}

// formatLabels formats node labels as a string
func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	parts := make([]string, 0, len(labels))
	for k, v := range labels {
		if v == "" {
			parts = append(parts, k)
		} else {
			parts = append(parts, k+"="+v)
		}
	}
	return truncate(strings.Join(parts, ","), 60)
}

// formatCPU formats CPU value (millicores) to string
func formatCPU(val int64, showRaw bool) string {
	cores := float64(val) / milliCPUBase
	if showRaw && val < milliCPUBase {
		return fmt.Sprintf("%dm", val)
	}
	return fmt.Sprintf("%.2fc", cores)
}

// formatMemory formats memory value (bytes) to string
func formatMemory(val int64, showRaw bool) string {
	if showRaw {
		switch {
		case val >= bytesPerGi:
			return fmt.Sprintf("%dGi", val/bytesPerGi)
		case val >= bytesPerMi:
			return fmt.Sprintf("%dMi", val/bytesPerMi)
		case val >= bytesPerKi:
			return fmt.Sprintf("%dKi", val/bytesPerKi)
		default:
			return fmt.Sprintf("%d", val)
		}
	}
	return fmt.Sprintf("%.2fGi", float64(val)/bytesPerGi)
}

// truncate truncates a string to the specified length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
