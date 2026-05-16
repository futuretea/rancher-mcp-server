package kubernetes

import (
	"reflect"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
)

func TestAggregateToolsRegisteredAsReadOnly(t *testing.T) {
	tools := mapToolsByName((&Toolset{}).GetTools(nil))
	for _, name := range []string{
		"kubernetes_top",
		"kubernetes_workload_health",
		"kubernetes_resource_summary",
		"kubernetes_event_summary",
	} {
		st, ok := tools[name]
		if !ok {
			t.Fatalf("%s is not registered", name)
		}
		if st.Handler == nil {
			t.Fatalf("%s handler is nil", name)
		}
		if st.Annotations.ReadOnlyHint == nil || !*st.Annotations.ReadOnlyHint {
			t.Fatalf("%s ReadOnlyHint = %v, want true", name, st.Annotations.ReadOnlyHint)
		}
		if !reflect.DeepEqual(st.Tool.InputSchema.Required, []string{"cluster"}) {
			t.Fatalf("%s required = %#v, want [cluster]", name, st.Tool.InputSchema.Required)
		}
	}
}

func TestAggregateToolSchemasExposeExpectedEnums(t *testing.T) {
	tools := mapToolsByName((&Toolset{}).GetTools(nil))

	assertEnum(t, tools["kubernetes_top"], "kind", []string{"pod", "node"})
	assertEnum(t, tools["kubernetes_top"], "sortBy", []string{"", "cpu.util", "mem.util", "cpu.request", "mem.request", "cpu.limit", "mem.limit", "cpu.util.percentage", "mem.util.percentage", "restart.count", "pod.count", "name"})
	assertDefault(t, tools["kubernetes_top"], "limit", 50)
	assertDefault(t, tools["kubernetes_top"], "format", "table")

	assertEnum(t, tools["kubernetes_workload_health"], "kind", []string{"deployment", "statefulset", "daemonset", "all"})
	assertEnum(t, tools["kubernetes_workload_health"], "sortBy", []string{"", "unready.count", "ready.ratio", "name"})

	assertEnum(t, tools["kubernetes_resource_summary"], "groupBy", []string{"namespace", "label"})
	assertEnum(t, tools["kubernetes_resource_summary"], "sortBy", []string{"", "cpu.request", "mem.request", "cpu.limit", "mem.limit", "pod.count", "name"})

	assertEnum(t, tools["kubernetes_event_summary"], "type", []string{"", "Warning", "Normal"})
	assertEnum(t, tools["kubernetes_event_summary"], "sortBy", []string{"", "count", "lastSeen", "name"})
}

func mapToolsByName(tools []toolset.ServerTool) map[string]toolset.ServerTool {
	result := make(map[string]toolset.ServerTool, len(tools))
	for _, st := range tools {
		result[st.Tool.Name] = st
	}
	return result
}

func assertEnum(t *testing.T, st toolset.ServerTool, property string, want []string) {
	t.Helper()
	prop := schemaProperty(t, st, property)
	got, ok := prop["enum"].([]string)
	if !ok {
		t.Fatalf("%s.%s enum has type %T, want []string", st.Tool.Name, property, prop["enum"])
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s.%s enum = %#v, want %#v", st.Tool.Name, property, got, want)
	}
}

func assertDefault(t *testing.T, st toolset.ServerTool, property string, want any) {
	t.Helper()
	prop := schemaProperty(t, st, property)
	if !reflect.DeepEqual(prop["default"], want) {
		t.Fatalf("%s.%s default = %#v, want %#v", st.Tool.Name, property, prop["default"], want)
	}
}

func schemaProperty(t *testing.T, st toolset.ServerTool, property string) map[string]any {
	t.Helper()
	prop, ok := st.Tool.InputSchema.Properties[property].(map[string]any)
	if !ok {
		t.Fatalf("%s.%s property has type %T, want map[string]any", st.Tool.Name, property, st.Tool.InputSchema.Properties[property])
	}
	return prop
}
