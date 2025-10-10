package output

import (
	"strings"
	"testing"
)

func TestIsValidFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"table", true},
		{"TABLE", true},
		{"yaml", true},
		{"YAML", true},
		{"json", true},
		{"JSON", true},
		{"yml", false}, // Only "yaml" is supported, not "yml"
		{"unknown", false},
		{"", false},
		{"xml", false},
		{"csv", false},
	}

	for _, test := range tests {
		result := IsValidFormat(test.input)
		if result != test.expected {
			t.Errorf("IsValidFormat('%s') = %v, expected %v", test.input, result, test.expected)
		}
	}
}

func TestFormatter_Format(t *testing.T) {
	formatter := NewFormatter()

	testData := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	// Test JSON format
	jsonResult, err := formatter.Format(testData, "json")
	if err != nil {
		t.Errorf("Format JSON failed: %v", err)
	}
	if !strings.Contains(jsonResult, "key1") || !strings.Contains(jsonResult, "value1") {
		t.Errorf("JSON output should contain test data, got: %s", jsonResult)
	}

	// Test YAML format
	yamlResult, err := formatter.Format(testData, "yaml")
	if err != nil {
		t.Errorf("Format YAML failed: %v", err)
	}
	if !strings.Contains(yamlResult, "key1") || !strings.Contains(yamlResult, "value1") {
		t.Errorf("YAML output should contain test data, got: %s", yamlResult)
	}

	// Test table format (default)
	tableResult, err := formatter.Format(testData, "table")
	if err != nil {
		t.Errorf("Format table failed: %v", err)
	}
	if tableResult == "" {
		t.Errorf("Table output should not be empty")
	}

	// Test unknown format (should default to table)
	defaultResult, err := formatter.Format(testData, "unknown")
	if err != nil {
		t.Errorf("Format unknown failed: %v", err)
	}
	if defaultResult == "" {
		t.Errorf("Default output should not be empty")
	}
}

func TestFormatter_FormatJSON(t *testing.T) {
	formatter := NewFormatter()

	testData := []map[string]string{
		{"name": "test1", "status": "running"},
		{"name": "test2", "status": "pending"},
	}

	result, err := formatter.FormatJSON(testData)
	if err != nil {
		t.Errorf("FormatJSON failed: %v", err)
	}

	// Check JSON structure
	if !strings.Contains(result, "test1") || !strings.Contains(result, "running") {
		t.Errorf("JSON output should contain test data, got: %s", result)
	}
	if !strings.Contains(result, "[") || !strings.Contains(result, "]") {
		t.Errorf("JSON output should be valid JSON array")
	}
}

func TestFormatter_FormatYAML(t *testing.T) {
	formatter := NewFormatter()

	testData := map[string]interface{}{
		"items": []map[string]string{
			{"name": "test1", "status": "running"},
			{"name": "test2", "status": "pending"},
		},
	}

	result, err := formatter.FormatYAML(testData)
	if err != nil {
		t.Errorf("FormatYAML failed: %v", err)
	}

	// Check YAML structure
	if !strings.Contains(result, "test1") || !strings.Contains(result, "running") {
		t.Errorf("YAML output should contain test data, got: %s", result)
	}
}

func TestFormatter_FormatTable(t *testing.T) {
	formatter := NewFormatter()

	testData := []map[string]string{
		{"name": "test1", "status": "running"},
		{"name": "test2", "status": "pending"},
	}

	result, err := formatter.FormatTable(testData)
	if err != nil {
		t.Errorf("FormatTable failed: %v", err)
	}

	// Table output should not be empty
	if result == "" {
		t.Errorf("Table output should not be empty")
	}
}

func TestFormatter_FormatTableWithHeaders(t *testing.T) {
	formatter := NewFormatter()

	data := []map[string]string{
		{"name": "pod-1", "status": "Running", "age": "1d"},
		{"name": "pod-2", "status": "Pending", "age": "2h"},
	}

	headers := []string{"name", "status", "age"}

	result := formatter.FormatTableWithHeaders(data, headers)

	// Check that headers are present
	for _, header := range headers {
		if !strings.Contains(result, header) {
			t.Errorf("Result should contain header '%s', got:\n%s", header, result)
		}
	}

	// Check that data is present
	for _, row := range data {
		for _, value := range row {
			if !strings.Contains(result, value) {
				t.Errorf("Result should contain value '%s', got:\n%s", value, result)
			}
		}
	}

	// Test empty case
	emptyResult := formatter.FormatTableWithHeaders(nil, headers)
	if emptyResult != "No data available" {
		t.Errorf("Empty table should return 'No data available', got: '%s'", emptyResult)
	}

	emptyResult2 := formatter.FormatTableWithHeaders([]map[string]string{}, headers)
	if emptyResult2 != "No data available" {
		t.Errorf("Empty table should return 'No data available', got: '%s'", emptyResult2)
	}
}