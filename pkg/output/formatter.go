package output

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Formatter provides formatting capabilities for different output formats
type Formatter struct{}

// NewFormatter creates a new formatter
func NewFormatter() *Formatter {
	return &Formatter{}
}

// IsValidFormat checks if the given format is supported
func IsValidFormat(format string) bool {
	switch strings.ToLower(format) {
	case "table", "yaml", "json":
		return true
	default:
		return false
	}
}

// Format formats data in the specified format (table, yaml, json)
func (f *Formatter) Format(data interface{}, format string) (string, error) {
	switch strings.ToLower(format) {
	case "yaml":
		return f.FormatYAML(data)
	case "json":
		return f.FormatJSON(data)
	case "table":
		return f.FormatTable(data)
	default:
		return f.FormatTable(data)
	}
}

// FormatYAML formats data as YAML
func (f *Formatter) FormatYAML(data interface{}) (string, error) {
	yamlBytes, err := yaml.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal YAML: %v", err)
	}
	return string(yamlBytes), nil
}

// FormatJSON formats data as JSON
func (f *Formatter) FormatJSON(data interface{}) (string, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %v", err)
	}
	return string(jsonBytes), nil
}

// FormatTable formats data as a table
func (f *Formatter) FormatTable(data interface{}) (string, error) {
	// For now, return a simple string representation
	// In a real implementation, this would format as a proper table
	return fmt.Sprintf("%+v", data), nil
}

// FormatTableWithHeaders formats tabular data with specific headers
func (f *Formatter) FormatTableWithHeaders(data []map[string]string, headers []string) string {
	if len(data) == 0 {
		return "No data available"
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(header)
	}

	for _, row := range data {
		for i, header := range headers {
			if value, ok := row[header]; ok && len(value) > widths[i] {
				widths[i] = len(value)
			}
		}
	}

	// Build table
	var builder strings.Builder

	// Header
	for i, header := range headers {
		builder.WriteString(fmt.Sprintf("%-*s", widths[i]+2, header))
	}
	builder.WriteString("\n")

	// Separator
	for i, width := range widths {
		builder.WriteString(strings.Repeat("-", width+2))
		if i < len(widths)-1 {
			builder.WriteString(" ")
		}
	}
	builder.WriteString("\n")

	// Rows
	for _, row := range data {
		for i, header := range headers {
			value := ""
			if v, ok := row[header]; ok {
				value = v
			}
			builder.WriteString(fmt.Sprintf("%-*s", widths[i]+2, value))
		}
		builder.WriteString("\n")
	}

	return builder.String()
}