package dep

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	unstructuredv1 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// FormatTree renders the dependency result as a kube-lineage-style tree string.
func FormatTree(result *Result, depsIsDependencies bool) string {
	if result == nil || len(result.NodeMap) == 0 {
		return "No dependency data found"
	}

	rootNode := result.NodeMap[result.RootUID]
	if rootNode == nil {
		return "Root node not found"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%-12s %-50s %-8s %-12s %-6s %s\n",
		"NAMESPACE", "NAME", "READY", "STATUS", "AGE", "RELATIONSHIPS")

	printTreeNode(&b, result.NodeMap, rootNode, result.RootUID, depsIsDependencies, "", true, true, nil, map[types.UID]bool{})

	return b.String()
}

// printTreeNode recursively prints a node and its children in tree format.
func printTreeNode(b *strings.Builder, nodeMap NodeMap, node *Node, uid types.UID, depsIsDependencies bool, prefix string, isRoot, isLast bool, rels RelationshipSet, visited map[types.UID]bool) {
	if node == nil {
		return
	}

	ns := node.Namespace
	if ns == "" {
		ns = "-"
	}
	name := fmt.Sprintf("%s/%s", node.Kind, node.Name)

	relStr := "[]"
	if len(rels) > 0 {
		relStr = fmt.Sprintf("[%s]", strings.Join(rels.List(), " "))
	}

	connector := ""
	if !isRoot {
		if isLast {
			connector = "└── "
		} else {
			connector = "├── "
		}
	}

	fmt.Fprintf(b, "%-12s %s%-46s %-8s %-12s %-6s %s\n",
		truncateStr(ns, 12),
		prefix+connector,
		truncateStr(name, 46-len(prefix)-len(connector)),
		truncateStr(getNodeReady(node), 8),
		truncateStr(getNodeStatus(node), 12),
		truncateStr(getNodeAge(node), 6),
		relStr,
	)

	// Stop recursion on cycle
	if visited[uid] {
		return
	}
	visited[uid] = true

	deps := node.GetDeps(depsIsDependencies)
	children := sortedChildren(nodeMap, deps, uid)
	for i, child := range children {
		childIsLast := i == len(children)-1
		childPrefix := prefix
		if !isRoot {
			if isLast {
				childPrefix += "    "
			} else {
				childPrefix += "│   "
			}
		}
		printTreeNode(b, nodeMap, child, child.UID, depsIsDependencies, childPrefix, false, childIsLast, deps[child.UID], visited)
	}
}

// JSONNode represents a node in JSON output format.
type JSONNode struct {
	Kind          string      `json:"kind"`
	Namespace     string      `json:"namespace,omitempty"`
	Name          string      `json:"name"`
	Ready         string      `json:"ready,omitempty"`
	Status        string      `json:"status,omitempty"`
	Age           string      `json:"age,omitempty"`
	Relationships []string    `json:"relationships,omitempty"`
	Children      []*JSONNode `json:"children,omitempty"`
}

// FormatJSON renders the dependency result as a nested JSON structure.
func FormatJSON(result *Result, depsIsDependencies bool) (string, error) {
	if result == nil || len(result.NodeMap) == 0 {
		return "[]", nil
	}

	rootNode := result.NodeMap[result.RootUID]
	if rootNode == nil {
		return "[]", nil
	}

	jsonTree := buildJSONTree(result.NodeMap, rootNode, result.RootUID, depsIsDependencies, nil, map[types.UID]bool{})

	data, err := json.MarshalIndent(jsonTree, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(data), nil
}

// buildJSONTree recursively builds the JSON tree structure.
func buildJSONTree(nodeMap NodeMap, node *Node, uid types.UID, depsIsDependencies bool, rels RelationshipSet, visited map[types.UID]bool) *JSONNode {
	if node == nil {
		return nil
	}

	if visited[uid] {
		return &JSONNode{
			Kind:      node.Kind,
			Namespace: node.Namespace,
			Name:      node.Name,
		}
	}
	visited[uid] = true

	jn := &JSONNode{
		Kind:      node.Kind,
		Namespace: node.Namespace,
		Name:      node.Name,
		Ready:     getNodeReady(node),
		Status:    getNodeStatus(node),
		Age:       getNodeAge(node),
	}

	if len(rels) > 0 {
		jn.Relationships = rels.List()
	}

	deps := node.GetDeps(depsIsDependencies)
	for _, child := range sortedChildren(nodeMap, deps, uid) {
		if childJSON := buildJSONTree(nodeMap, child, child.UID, depsIsDependencies, deps[child.UID], visited); childJSON != nil {
			jn.Children = append(jn.Children, childJSON)
		}
	}

	return jn
}

// getNodeReady extracts the ready status from a node.
func getNodeReady(n *Node) string {
	content := n.UnstructuredContent()

	// Kind-specific ready display
	switch n.Kind {
	case "Deployment", "ReplicaSet", "StatefulSet":
		replicas := getNestedInt64(content, "status", "replicas")
		readyReplicas := getNestedInt64(content, "status", "readyReplicas")
		return fmt.Sprintf("%d/%d", readyReplicas, replicas)
	case "Pod":
		containerStatuses, found, _ := unstructuredv1.NestedSlice(content, "status", "containerStatuses")
		if found {
			total := len(containerStatuses)
			ready := 0
			for _, cs := range containerStatuses {
				csMap, ok := cs.(map[string]interface{})
				if !ok {
					continue
				}
				if r, _ := csMap["ready"].(bool); r {
					ready++
				}
			}
			return fmt.Sprintf("%d/%d", ready, total)
		}
	}

	// Fallback: check Ready condition
	conditions, found, _ := unstructuredv1.NestedSlice(content, "status", "conditions")
	if found {
		for _, c := range conditions {
			cMap, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			if cType, _ := cMap["type"].(string); cType == "Ready" {
				if status, _ := cMap["status"].(string); status != "" {
					return status
				}
			}
		}
	}

	return "-"
}

// getNodeStatus extracts the status string from a node.
func getNodeStatus(n *Node) string {
	content := n.UnstructuredContent()

	switch n.Kind {
	case "Pod":
		phase, found, _ := unstructuredv1.NestedString(content, "status", "phase")
		if found {
			return phase
		}
	case "Node":
		conditions, found, _ := unstructuredv1.NestedSlice(content, "status", "conditions")
		if found {
			for _, c := range conditions {
				cMap, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				if cType, _ := cMap["type"].(string); cType == "Ready" {
					if reason, _ := cMap["reason"].(string); reason != "" {
						return reason
					}
				}
			}
		}
	}

	return ""
}

// getNodeAge returns a human-readable age string for the node.
func getNodeAge(n *Node) string {
	ts := n.GetCreationTimestamp()
	if ts.IsZero() {
		return "-"
	}
	d := time.Since(ts.Time)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// truncateStr truncates a string to maxLen.
func truncateStr(s string, maxLen int) string {
	if maxLen <= 0 {
		return s
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// sortedChildren returns child nodes from deps, excluding self, sorted by namespace/kind/name.
func sortedChildren(nodeMap NodeMap, deps map[types.UID]RelationshipSet, selfUID types.UID) NodeList {
	children := make(NodeList, 0, len(deps))
	for uid := range deps {
		if depNode, ok := nodeMap[uid]; ok && uid != selfUID {
			children = append(children, depNode)
		}
	}
	sort.Sort(children)
	return children
}

// Helper functions for unstructured access.

func getNestedInt64(obj map[string]interface{}, fields ...string) int64 {
	current := obj
	for i := 0; i < len(fields)-1; i++ {
		next, ok := current[fields[i]]
		if !ok {
			return 0
		}
		nextMap, ok := next.(map[string]interface{})
		if !ok {
			return 0
		}
		current = nextMap
	}
	val, ok := current[fields[len(fields)-1]]
	if !ok {
		return 0
	}
	switch v := val.(type) {
	case int64:
		return v
	case float64:
		return int64(v)
	case int:
		return int64(v)
	default:
		return 0
	}
}
