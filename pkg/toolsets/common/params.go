package common

import "fmt"

// ExtractRequiredString extracts a required string parameter from params map
func ExtractRequiredString(params map[string]interface{}, key string) (string, error) {
	value := ""
	if v, ok := params[key].(string); ok {
		value = v
	}
	if value == "" {
		return "", fmt.Errorf("%w: %s", ErrMissingParameter, key)
	}
	return value, nil
}

// ExtractOptionalString extracts an optional string parameter with a default value
func ExtractOptionalString(params map[string]interface{}, key string, defaultValue string) string {
	if v, ok := params[key].(string); ok && v != "" {
		return v
	}
	return defaultValue
}

// ExtractBool extracts a boolean parameter with a default value
func ExtractBool(params map[string]interface{}, key string, defaultValue bool) bool {
	if v, ok := params[key].(bool); ok {
		return v
	}
	return defaultValue
}

// ExtractFormat extracts the format parameter with "json" as default
func ExtractFormat(params map[string]interface{}) string {
	return ExtractOptionalString(params, ParamFormat, FormatJSON)
}

// ValidateFormat validates that the format is one of the supported formats
func ValidateFormat(format string) error {
	switch format {
	case FormatJSON, FormatYAML, FormatTable:
		return nil
	default:
		return fmt.Errorf("%w: %s (supported: json, yaml, table)", ErrInvalidFormat, format)
	}
}
