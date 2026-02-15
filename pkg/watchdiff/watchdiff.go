package watchdiff

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Printer formats diffs between two unstructured Kubernetes objects.
// It is inspired by kubectl-yadt's DiffPrinter but outputs plain text
// without terminal color codes so it is suitable for MCP tool responses.
type Printer struct {
	lastPrintTime time.Time
	showTimestamp bool
}

// NewPrinter creates a new Printer.
func NewPrinter(showTimestamp bool) *Printer {
	return &Printer{showTimestamp: showTimestamp}
}

// Differ maintains per-session state for computing diffs between
// successive versions of Kubernetes objects.
//
// It caches the last-seen version of each object and uses Printer to
// render git-style diffs between the cached and new versions.
type Differ struct {
	printer      *Printer
	cache        map[string]*unstructured.Unstructured
	ignoreStatus bool
	ignoreMeta   bool
}

// NewDiffer creates a new Differ.
func NewDiffer(showTimestamp bool) *Differ {
	return &Differ{
		printer: NewPrinter(showTimestamp),
		cache:   make(map[string]*unstructured.Unstructured),
	}
}

// SetIgnoreStatus controls whether the status field is excluded from diffs.
func (d *Differ) SetIgnoreStatus(ignore bool) {
	d.ignoreStatus = ignore
}

// SetIgnoreMeta controls whether non-essential metadata is excluded from diffs.
func (d *Differ) SetIgnoreMeta(ignore bool) {
	d.ignoreMeta = ignore
}

// Diff computes the diff between the last cached version of obj and the
// current version. The cache is updated to the new version.
//
// It returns an empty string if there is no effective change.
func (d *Differ) Diff(obj *unstructured.Unstructured) (string, error) {
	if obj == nil {
		return "", nil
	}

	key := getKey(obj)
	oldObj := d.cache[key]

	if oldObj == nil {
		// Try to construct a minimal previous object so that the first
		// diff shows the full resource content.
		oldObj = newEmptyObject(obj)
	}

	// Store a deep copy of the new object in the cache for future diffs.
	d.cache[key] = obj.DeepCopy()

	oldCopy := oldObj.DeepCopy()
	newCopy := obj.DeepCopy()

	if d.ignoreStatus {
		delete(oldCopy.Object, "status")
		delete(newCopy.Object, "status")
	}

	if d.ignoreMeta {
		trimMetadata(oldCopy)
		trimMetadata(newCopy)
	}

	return d.printer.Diff(oldCopy, newCopy)
}

// DiffDelete computes a diff that represents deletion of the object
// previously seen with the same identity. If the object was not seen
// before, it returns an empty string.
func (d *Differ) DiffDelete(obj *unstructured.Unstructured) (string, error) {
	if obj == nil {
		return "", nil
	}

	key := getKey(obj)
	oldObj := d.cache[key]
	if oldObj == nil {
		return "", nil
	}

	// Remove from cache as it is considered deleted.
	delete(d.cache, key)

	oldCopy := oldObj.DeepCopy()
	newCopy := newEmptyObject(obj)

	if d.ignoreStatus {
		delete(oldCopy.Object, "status")
		delete(newCopy.Object, "status")
	}

	if d.ignoreMeta {
		trimMetadata(oldCopy)
		trimMetadata(newCopy)
	}

	return d.printer.Diff(oldCopy, newCopy)
}

// Diff computes a git-style diff between two objects and returns it as a string.
// The original objects should already have had any ignoreStatus/ignoreMeta
// transformations applied.
func (p *Printer) Diff(oldObj, newObj *unstructured.Unstructured) (string, error) {
	if oldObj == nil && newObj == nil {
		return "", nil
	}

	// Prepare shallow maps containing only fields we care about.
	oldFields := make(map[string]interface{})
	newFields := make(map[string]interface{})

	if oldObj != nil {
		if spec, ok := oldObj.Object["spec"]; ok {
			oldFields["spec"] = spec
		}
		if status, ok := oldObj.Object["status"]; ok {
			oldFields["status"] = status
		}
	}
	if newObj != nil {
		if spec, ok := newObj.Object["spec"]; ok {
			newFields["spec"] = spec
		}
		if status, ok := newObj.Object["status"]; ok {
			newFields["status"] = status
		}
	}

	// If there is no change, return empty string.
	if areObjectsEqual(oldFields, newFields) {
		return "", nil
	}

	var buf bytes.Buffer

	// Header line similar to "diff kind.group name" with optional timestamp.
	name := ""
	if newObj != nil {
		name = newObj.GetName()
		if ns := newObj.GetNamespace(); ns != "" {
			name = ns + "/" + name
		}
	} else if oldObj != nil {
		name = oldObj.GetName()
		if ns := oldObj.GetNamespace(); ns != "" {
			name = ns + "/" + name
		}
	}

	resource := ""
	if newObj != nil {
		resource = strings.ToLower(newObj.GetKind())
		if group := newObj.GetAPIVersion(); group != "" {
			resource = fmt.Sprintf("%s.%s", resource, group)
		}
	} else if oldObj != nil {
		resource = strings.ToLower(oldObj.GetKind())
		if group := oldObj.GetAPIVersion(); group != "" {
			resource = fmt.Sprintf("%s.%s", resource, group)
		}
	}

	timestamp := ""
	if p.showTimestamp {
		now := time.Now()
		if now.Sub(p.lastPrintTime) > time.Second {
			p.lastPrintTime = now
			timestamp = now.Format("15:04:05 ")
		}
	}

	fmt.Fprintf(&buf, "%s%s\n", timestamp, fmt.Sprintf("diff %s %s", resource, name))
	fmt.Fprintf(&buf, "%s\n", strings.Repeat("-", 80))

	// For first-time objects (no real old version), show as added sections.
	if oldObj == nil || (oldObj.Object["spec"] == nil && oldObj.Object["status"] == nil) {
		buf.WriteString("+ New Resource\n")
		if spec, ok := newObj.Object["spec"]; ok {
			printSection(&buf, "spec", spec, true)
		}
		if status, ok := newObj.Object["status"]; ok {
			printSection(&buf, "status", status, true)
		}
		buf.WriteString("\n")
		return buf.String(), nil
	}

	// Print structured diff.
	printDiff(&buf, "", oldFields, newFields, "")
	buf.WriteString("\n")
	return buf.String(), nil
}

// trimMetadata keeps only essential metadata fields (name and namespace).
func trimMetadata(obj *unstructured.Unstructured) {
	metaVal, ok := obj.Object["metadata"].(map[string]interface{})
	if !ok {
		return
	}
	cleanMeta := map[string]interface{}{
		"name":      metaVal["name"],
		"namespace": metaVal["namespace"],
	}
	obj.Object["metadata"] = cleanMeta
}

// getKey builds a stable key for an object based on its identity.
func getKey(obj *unstructured.Unstructured) string {
	return fmt.Sprintf("%s/%s/%s/%s",
		obj.GetAPIVersion(),
		obj.GetKind(),
		obj.GetNamespace(),
		obj.GetName(),
	)
}

// newEmptyObject creates a minimal object with only identity fields.
func newEmptyObject(obj *unstructured.Unstructured) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": obj.GetAPIVersion(),
			"kind":       obj.GetKind(),
			"metadata": map[string]interface{}{
				"name":      obj.GetName(),
				"namespace": obj.GetNamespace(),
			},
		},
	}
}

// areObjectsEqual compares two objects recursively.
func areObjectsEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}

	for k, v1 := range a {
		v2, ok := b[k]
		if !ok {
			return false
		}

		switch val1 := v1.(type) {
		case map[string]interface{}:
			val2, ok := v2.(map[string]interface{})
			if !ok || !areObjectsEqual(val1, val2) {
				return false
			}
		case []interface{}:
			val2, ok := v2.([]interface{})
			if !ok || !areSlicesEqual(val1, val2) {
				return false
			}
		default:
			if v1 != v2 {
				return false
			}
		}
	}

	return true
}

// areSlicesEqual compares two slices recursively.
func areSlicesEqual(a, b []interface{}) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		switch v1 := a[i].(type) {
		case map[string]interface{}:
			v2, ok := b[i].(map[string]interface{})
			if !ok || !areObjectsEqual(v1, v2) {
				return false
			}
		case []interface{}:
			v2, ok := b[i].([]interface{})
			if !ok || !areSlicesEqual(v1, v2) {
				return false
			}
		default:
			if a[i] != b[i] {
				return false
			}
		}
	}

	return true
}

// printDiff recursively walks the structures and writes diff lines into buf.
func printDiff(buf *bytes.Buffer, path string, oldVal, newVal interface{}, indent string) {
	switch {
	case oldVal == nil && newVal == nil:
		return
	case oldVal == nil:
		printValue(buf, path, newVal, indent, true)
	case newVal == nil:
		printValue(buf, path, oldVal, indent, false)
	default:
		switch o := oldVal.(type) {
		case map[string]interface{}:
			n, ok := newVal.(map[string]interface{})
			if ok {
				printMapDiff(buf, path, o, n, indent)
			} else {
				printValue(buf, path, oldVal, indent, false)
				printValue(buf, path, newVal, indent, true)
			}
		case []interface{}:
			n, ok := newVal.([]interface{})
			if ok {
				printSliceDiff(buf, path, o, n, indent)
			} else {
				printValue(buf, path, oldVal, indent, false)
				printValue(buf, path, newVal, indent, true)
			}
		default:
			if oldVal != newVal {
				printValue(buf, path, oldVal, indent, false)
				printValue(buf, path, newVal, indent, true)
			}
		}
	}
}

func printMapDiff(buf *bytes.Buffer, path string, oldMap, newMap map[string]interface{}, indent string) {
	keys := make(map[string]bool)
	for k := range oldMap {
		keys[k] = true
	}
	for k := range newMap {
		keys[k] = true
	}

	sortedKeys := make([]string, 0, len(keys))
	for k := range keys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	for _, k := range sortedKeys {
		oldVal, oldOk := oldMap[k]
		newVal, newOk := newMap[k]

		newPath := k
		if path != "" {
			newPath = path + "." + k
		}

		switch {
		case !oldOk:
			printDiff(buf, newPath, nil, newVal, indent)
		case !newOk:
			printDiff(buf, newPath, oldVal, nil, indent)
		default:
			printDiff(buf, newPath, oldVal, newVal, indent)
		}
	}
}

func printSliceDiff(buf *bytes.Buffer, path string, oldSlice, newSlice []interface{}, indent string) {
	maxLen := len(oldSlice)
	if len(newSlice) > maxLen {
		maxLen = len(newSlice)
	}

	for i := 0; i < maxLen; i++ {
		var oldVal, newVal interface{}
		if i < len(oldSlice) {
			oldVal = oldSlice[i]
		}
		if i < len(newSlice) {
			newVal = newSlice[i]
		}

		newPath := fmt.Sprintf("%s[%d]", path, i)
		printDiff(buf, newPath, oldVal, newVal, indent+"  ")
	}
}

func printValue(buf *bytes.Buffer, path string, val interface{}, indent string, isAdd bool) {
	if val == nil {
		return
	}

	prefix := "-"
	if isAdd {
		prefix = "+"
	}

	switch v := val.(type) {
	case map[string]interface{}:
		if path == "" {
			for k, fieldVal := range v {
				printDiff(buf, k, nil, fieldVal, indent)
			}
		} else {
			b, _ := json.MarshalIndent(v, indent, "  ")
			lines := strings.Split(string(b), "\n")
			for _, line := range lines {
				if line == "{" || line == "}" {
					continue
				}
				fmt.Fprintf(buf, "%s%s%s\n", prefix, indent, line)
			}
		}
	case []interface{}:
		b, _ := json.MarshalIndent(v, indent, "  ")
		lines := strings.Split(string(b), "\n")
		for _, line := range lines {
			if line == "[" || line == "]" {
				continue
			}
			fmt.Fprintf(buf, "%s%s%s\n", prefix, indent, line)
		}
	default:
		if path != "" {
			fmt.Fprintf(buf, "%s%s%s: %v\n", prefix, indent, path, v)
		} else {
			fmt.Fprintf(buf, "%s%s%v\n", prefix, indent, v)
		}
	}
}

// printSection prints a whole section as added or removed.
func printSection(buf *bytes.Buffer, name string, val interface{}, isAdd bool) {
	prefix := "+"
	if !isAdd {
		prefix = "-"
	}

	fmt.Fprintf(buf, "%s %s:\n", prefix, name)
	if m, ok := val.(map[string]interface{}); ok {
		b, _ := json.MarshalIndent(m, "  ", "  ")
		lines := strings.Split(string(b), "\n")
		for _, line := range lines {
			if line == "{" || line == "}" {
				continue
			}
			fmt.Fprintf(buf, "%s  %s\n", prefix, line)
		}
	}
}
