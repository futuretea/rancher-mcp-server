package capacity

import (
	"sort"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/toolset/kubernetes/internal/formatutil"
)

// SortNodes sorts nodes by the specified field.
// Unknown sort keys fall back to sorting by node name ascending.
func SortNodes(nodes []NodeInfo, sortBy string) {
	cmp, ok := sortComparators[sortBy]
	if !ok {
		cmp = sortByName
	}
	sort.Slice(nodes, func(i, j int) bool { return cmp(nodes[i], nodes[j]) })
}

// sortComparators maps sort keys to their comparison functions.
// All comparisons are descending except for the name sort which is ascending.
var sortComparators = map[string]func(a, b NodeInfo) bool{
	"cpu.util":       func(a, b NodeInfo) bool { return a.CPU.Utilized > b.CPU.Utilized },
	"mem.util":       func(a, b NodeInfo) bool { return a.Memory.Utilized > b.Memory.Utilized },
	"memory.util":    func(a, b NodeInfo) bool { return a.Memory.Utilized > b.Memory.Utilized },
	"cpu.request":    func(a, b NodeInfo) bool { return a.CPU.Requested > b.CPU.Requested },
	"mem.request":    func(a, b NodeInfo) bool { return a.Memory.Requested > b.Memory.Requested },
	"memory.request": func(a, b NodeInfo) bool { return a.Memory.Requested > b.Memory.Requested },
	"cpu.limit":      func(a, b NodeInfo) bool { return a.CPU.Limited > b.CPU.Limited },
	"mem.limit":      func(a, b NodeInfo) bool { return a.Memory.Limited > b.Memory.Limited },
	"memory.limit":   func(a, b NodeInfo) bool { return a.Memory.Limited > b.Memory.Limited },
	"cpu.util.percentage": func(a, b NodeInfo) bool {
		return formatutil.CalcPercentage(a.CPU.Utilized, a.CPU.Allocatable) > formatutil.CalcPercentage(b.CPU.Utilized, b.CPU.Allocatable)
	},
	"mem.util.percentage": func(a, b NodeInfo) bool {
		return formatutil.CalcPercentage(a.Memory.Utilized, a.Memory.Allocatable) > formatutil.CalcPercentage(b.Memory.Utilized, b.Memory.Allocatable)
	},
	"memory.util.percentage": func(a, b NodeInfo) bool {
		return formatutil.CalcPercentage(a.Memory.Utilized, a.Memory.Allocatable) > formatutil.CalcPercentage(b.Memory.Utilized, b.Memory.Allocatable)
	},
	"cpu.request.percentage": func(a, b NodeInfo) bool {
		return formatutil.CalcPercentage(a.CPU.Requested, a.CPU.Allocatable) > formatutil.CalcPercentage(b.CPU.Requested, b.CPU.Allocatable)
	},
	"mem.request.percentage": func(a, b NodeInfo) bool {
		return formatutil.CalcPercentage(a.Memory.Requested, a.Memory.Allocatable) > formatutil.CalcPercentage(b.Memory.Requested, b.Memory.Allocatable)
	},
	"memory.request.percentage": func(a, b NodeInfo) bool {
		return formatutil.CalcPercentage(a.Memory.Requested, a.Memory.Allocatable) > formatutil.CalcPercentage(b.Memory.Requested, b.Memory.Allocatable)
	},
	"cpu.limit.percentage": func(a, b NodeInfo) bool {
		return formatutil.CalcPercentage(a.CPU.Limited, a.CPU.Allocatable) > formatutil.CalcPercentage(b.CPU.Limited, b.CPU.Allocatable)
	},
	"mem.limit.percentage": func(a, b NodeInfo) bool {
		return formatutil.CalcPercentage(a.Memory.Limited, a.Memory.Allocatable) > formatutil.CalcPercentage(b.Memory.Limited, b.Memory.Allocatable)
	},
	"memory.limit.percentage": func(a, b NodeInfo) bool {
		return formatutil.CalcPercentage(a.Memory.Limited, a.Memory.Allocatable) > formatutil.CalcPercentage(b.Memory.Limited, b.Memory.Allocatable)
	},
	"pod.count": func(a, b NodeInfo) bool { return a.PodCount.Requested > b.PodCount.Requested },
	"name":      sortByName,
}

func sortByName(a, b NodeInfo) bool {
	return strings.ToLower(a.Name) < strings.ToLower(b.Name)
}
