// Package watchdiff provides resource watching with git-style diff output,
// comparing Kubernetes resource versions over time and reporting changes.
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

	return d.printer.Diff(d.prepareForDiff(oldObj), d.prepareForDiff(obj))
}

// prepareForDiff returns a deep copy of obj with the configured ignore rules
// applied. The caller must ensure obj is non-nil.
func (d *Differ) prepareForDiff(obj *unstructured.Unstructured) *unstructured.Unstructured {
	out := obj.DeepCopy()

	if d.ignoreStatus {
		delete(out.Object, "status")
	}

	if d.ignoreMeta {
		trimMetadata(out)
	}

	return out
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

	return d.printer.Diff(d.prepareForDiff(oldObj), d.prepareForDiff(newEmptyObject(obj)))
}

// Diff computes a git-style diff between two objects and returns it as a string.
// The original objects should already have had any ignoreStatus/ignoreMeta
// transformations applied.
func (p *Printer) Diff(oldObj, newObj *unstructured.Unstructured) (string, error) {
	if oldObj == nil && newObj == nil {
		return "", nil
	}

	oldFields := extractDiffFields(oldObj)
	newFields := extractDiffFields(newObj)

	if areObjectsEqual(oldFields, newFields) {
		return "", nil
	}

	var buf bytes.Buffer

	obj := pickObject(oldObj, newObj)
	name := resourceName(obj)
	resource := resourceType(obj)
	p.writeHeader(&buf, resource, name)

	if isNewResource(oldObj) {
		writeNewResourceDiff(&buf, newObj)
		return buf.String(), nil
	}

	writeStructuredDiff(&buf, oldFields, newFields)
	return buf.String(), nil
}

// extractDiffFields returns a shallow map with only the fields that participate
// in the diff: spec and status.
func extractDiffFields(obj *unstructured.Unstructured) map[string]interface{} {
	fields := make(map[string]interface{})
	if obj == nil {
		return fields
	}
	if spec, ok := obj.Object["spec"]; ok {
		fields["spec"] = spec
	}
	if status, ok := obj.Object["status"]; ok {
		fields["status"] = status
	}
	return fields
}

// pickObject returns newObj when available, otherwise oldObj. It assumes at
// least one argument is non-nil.
func pickObject(oldObj, newObj *unstructured.Unstructured) *unstructured.Unstructured {
	if newObj != nil {
		return newObj
	}
	return oldObj
}

// resourceName returns the display name for an object, optionally prefixed by
// its namespace.
func resourceName(obj *unstructured.Unstructured) string {
	if obj == nil {
		return ""
	}
	name := obj.GetName()
	if ns := obj.GetNamespace(); ns != "" {
		name = ns + "/" + name
	}
	return name
}

// resourceType returns the display resource type as "kind.group" or just "kind"
// when no API group is present.
func resourceType(obj *unstructured.Unstructured) string {
	if obj == nil {
		return ""
	}
	resource := strings.ToLower(obj.GetKind())
	if group := obj.GetAPIVersion(); group != "" {
		resource = fmt.Sprintf("%s.%s", resource, group)
	}
	return resource
}

// writeHeader writes the "diff resource name" header and separator into buf.
// It updates the printer's last-print timestamp when timestamps are enabled.
func (p *Printer) writeHeader(buf *bytes.Buffer, resource, name string) {
	timestamp := ""
	if p.showTimestamp {
		now := time.Now()
		if now.Sub(p.lastPrintTime) > time.Second {
			p.lastPrintTime = now
			timestamp = now.Format("15:04:05 ")
		}
	}

	fmt.Fprintf(buf, "%sdiff %s %s\n", timestamp, resource, name)
	fmt.Fprintln(buf, strings.Repeat("-", 80))
}

// isNewResource reports whether oldObj represents a resource that has no real
// prior version, in which case the diff is rendered as added sections.
func isNewResource(oldObj *unstructured.Unstructured) bool {
	return oldObj == nil || (oldObj.Object["spec"] == nil && oldObj.Object["status"] == nil)
}

// writeNewResourceDiff renders a diff for a resource that is being seen for the
// first time.
func writeNewResourceDiff(buf *bytes.Buffer, newObj *unstructured.Unstructured) {
	buf.WriteString("+ New Resource\n")
	if spec, ok := newObj.Object["spec"]; ok {
		printSection(buf, "spec", spec, true)
	}
	if status, ok := newObj.Object["status"]; ok {
		printSection(buf, "status", status, true)
	}
	buf.WriteString("\n")
}

// writeStructuredDiff renders a field-by-field diff between oldFields and
// newFields.
func writeStructuredDiff(buf *bytes.Buffer, oldFields, newFields map[string]interface{}) {
	if len(oldFields) > 0 && len(newFields) == 0 {
		buf.WriteString("- Deleted Resource\n")
	}
	printDiff(buf, "", oldFields, newFields, "")
	buf.WriteString("\n")
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
	case map[string]interface{}, []interface{}:
		writeJSONLines(buf, prefix, indent, v)
	default:
		if path != "" {
			fmt.Fprintf(buf, "%s%s%s: %v\n", prefix, indent, path, v)
		} else {
			fmt.Fprintf(buf, "%s%s%v\n", prefix, indent, v)
		}
	}
}

// writeJSONLines marshals val as indented JSON and writes each non-bracket
// line to buf with the given prefix and indent.
func writeJSONLines(buf *bytes.Buffer, prefix, indent string, val interface{}) {
	b, _ := json.MarshalIndent(val, indent, "  ")
	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		if line == "{" || line == "}" || line == "[" || line == "]" {
			continue
		}
		fmt.Fprintf(buf, "%s%s%s\n", prefix, indent, line)
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
		writeJSONLines(buf, prefix, "  ", m)
	}
}
