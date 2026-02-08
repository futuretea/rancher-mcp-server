package handler

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// FormatAsTable formats data as a table with headers
func FormatAsTable(data []map[string]string, headers []string) string {
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

// FormatAsYAML formats data as YAML
func FormatAsYAML(data interface{}) (string, error) {
	yamlBytes, err := yaml.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal YAML: %w", err)
	}
	return string(yamlBytes), nil
}

// FormatAsJSON formats data as JSON
func FormatAsJSON(data interface{}) (string, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(jsonBytes), nil
}

// BoolPtr returns a pointer to a boolean value
func BoolPtr(b bool) *bool {
	return &b
}

// GetStringValue extracts string value from interface safely.
// Returns "-" for nil values.
func GetStringValue(v interface{}) string {
	if v == nil {
		return "-"
	}
	if str, ok := v.(string); ok {
		return str
	}
	return fmt.Sprintf("%v", v)
}

// FormatTime formats time for display.
// Returns "-" for empty timestamps.
func FormatTime(timestamp string) string {
	if timestamp == "" {
		return "-"
	}
	return timestamp
}

// FormatEmptyResult formats an empty result based on the output format.
func FormatEmptyResult(format string) (string, error) {
	switch format {
	case FormatYAML:
		return FormatAsYAML([]interface{}{})
	case FormatJSON:
		return FormatAsJSON([]interface{}{})
	default:
		return "", nil
	}
}

// FilterFields filters map data to include only specified fields
// If fields is nil or empty, returns data unchanged
func FilterFields(data []map[string]string, fields []string) []map[string]string {
	if len(fields) == 0 {
		return data
	}

	filtered := make([]map[string]string, len(data))
	for i, row := range data {
		filteredRow := make(map[string]string)
		for _, field := range fields {
			if val, exists := row[field]; exists {
				filteredRow[field] = val
			} else {
				// Include field even if missing, with empty value
				filteredRow[field] = ""
			}
		}
		filtered[i] = filteredRow
	}
	return filtered
}

// FormatWithFields formats data with specific fields only
// If fields is empty, includes all fields
func FormatWithFields(data []map[string]string, fields []string, format string) (string, error) {
	filteredData := FilterFields(data, fields)

	switch format {
	case FormatYAML:
		return FormatAsYAML(filteredData)
	case FormatJSON:
		return FormatAsJSON(filteredData)
	case FormatTable:
		// Use specified fields as headers, or derive from first row
		headers := fields
		if len(headers) == 0 && len(filteredData) > 0 {
			// Derive headers from first row keys
			headers = make([]string, 0, len(filteredData[0]))
			for key := range filteredData[0] {
				headers = append(headers, key)
			}
		}
		return FormatAsTable(filteredData, headers), nil
	default:
		return "", fmt.Errorf("%w: %s", ErrInvalidFormat, format)
	}
}

// FormatOutput is a generic helper for formatting slice data in different output formats.
// It reduces code duplication across handlers by consolidating the format switch logic.
// If fields is provided, only those fields will be included in the output.
func FormatOutput(data []map[string]string, format string, headers []string, fields []string) (string, error) {
	if len(data) == 0 {
		return FormatEmptyResult(format)
	}

	// Apply field filtering if requested
	if len(fields) > 0 {
		return FormatWithFields(data, fields, format)
	}

	switch format {
	case FormatYAML:
		return FormatAsYAML(data)
	case FormatJSON:
		return FormatAsJSON(data)
	case FormatTable:
		return FormatAsTable(data, headers), nil
	default:
		return "", fmt.Errorf("%w: %s", ErrInvalidFormat, format)
	}
}

// FormatSingleResult formats a single result object (map[string]interface{}) in the specified format.
// This is useful for get handlers that return a single resource.
// tableHeaders is optional - if provided and format is "table", it renders a table with those fields.
func FormatSingleResult(data map[string]interface{}, format string, tableHeaders ...string) (string, error) {
	switch format {
	case FormatYAML:
		return FormatAsYAML(data)
	case FormatJSON:
		return FormatAsJSON(data)
	case FormatTable:
		if len(tableHeaders) == 0 {
			return "", fmt.Errorf("%w: table format requires headers", ErrInvalidFormat)
		}
		row := make(map[string]string)
		for _, header := range tableHeaders {
			row[header] = GetStringValue(data[header])
		}
		return FormatAsTable([]map[string]string{row}, tableHeaders), nil
	default:
		return "", fmt.Errorf("%w: %s", ErrInvalidFormat, format)
	}
}
