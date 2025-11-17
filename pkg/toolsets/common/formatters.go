package common

import (
	"fmt"

	"github.com/futuretea/rancher-mcp-server/pkg/output"
)

// FormatAsTable formats data as a table with headers
func FormatAsTable(data []map[string]string, headers []string) string {
	formatter := output.NewFormatter()
	return formatter.FormatTableWithHeaders(data, headers)
}

// FormatAsYAML formats data as YAML
func FormatAsYAML(data interface{}) (string, error) {
	formatter := output.NewFormatter()
	return formatter.FormatYAML(data)
}

// FormatAsJSON formats data as JSON
func FormatAsJSON(data interface{}) (string, error) {
	formatter := output.NewFormatter()
	return formatter.FormatJSON(data)
}

// BoolPtr returns a pointer to a boolean value
func BoolPtr(b bool) *bool {
	return &b
}

// GetStringValue extracts string value from interface safely
func GetStringValue(v interface{}) string {
	if str, ok := v.(string); ok {
		return str
	}
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%v", v)
}

// FormatTime formats time for display
func FormatTime(timestamp string) string {
	if timestamp == "" {
		return "-"
	}
	return timestamp
}

// FormatEmptyResult formats an empty result based on the output format
func FormatEmptyResult(format string) (string, error) {
	emptyArray := []interface{}{}
	switch format {
	case FormatYAML:
		return FormatAsYAML(emptyArray)
	case FormatJSON:
		return FormatAsJSON(emptyArray)
	case FormatTable:
		return "", nil
	default:
		return "", nil
	}
}
