package capacity

import (
	"sort"
	"strings"
)

// SortNodes sorts nodes by the specified field
func SortNodes(nodes []NodeInfo, sortBy string) {
	sort.Slice(nodes, func(i, j int) bool {
		a, b := nodes[i], nodes[j]

		switch sortBy {
		case "cpu.util":
			return a.CPU.Utilized > b.CPU.Utilized
		case "mem.util", "memory.util":
			return a.Memory.Utilized > b.Memory.Utilized
		case "cpu.request":
			return a.CPU.Requested > b.CPU.Requested
		case "mem.request", "memory.request":
			return a.Memory.Requested > b.Memory.Requested
		case "cpu.limit":
			return a.CPU.Limited > b.CPU.Limited
		case "mem.limit", "memory.limit":
			return a.Memory.Limited > b.Memory.Limited
		case "cpu.util.percentage":
			return calcPercentage(a.CPU.Utilized, a.CPU.Allocatable) > calcPercentage(b.CPU.Utilized, b.CPU.Allocatable)
		case "mem.util.percentage", "memory.util.percentage":
			return calcPercentage(a.Memory.Utilized, a.Memory.Allocatable) > calcPercentage(b.Memory.Utilized, b.Memory.Allocatable)
		case "cpu.request.percentage":
			return calcPercentage(a.CPU.Requested, a.CPU.Allocatable) > calcPercentage(b.CPU.Requested, b.CPU.Allocatable)
		case "mem.request.percentage", "memory.request.percentage":
			return calcPercentage(a.Memory.Requested, a.Memory.Allocatable) > calcPercentage(b.Memory.Requested, b.Memory.Allocatable)
		case "cpu.limit.percentage":
			return calcPercentage(a.CPU.Limited, a.CPU.Allocatable) > calcPercentage(b.CPU.Limited, b.CPU.Allocatable)
		case "mem.limit.percentage", "memory.limit.percentage":
			return calcPercentage(a.Memory.Limited, a.Memory.Allocatable) > calcPercentage(b.Memory.Limited, b.Memory.Allocatable)
		case "pod.count":
			return a.PodCount.Requested > b.PodCount.Requested
		case "name":
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		default:
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
	})
}

// calcPercentage calculates percentage with zero check
func calcPercentage(value, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(value) / float64(total) * 100
}
