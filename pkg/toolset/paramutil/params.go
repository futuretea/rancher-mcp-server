package paramutil

import "fmt"

// ExtractRequiredString extracts a required string parameter from params map.
// Returns ErrMissingParameter if the parameter is missing or empty.
func ExtractRequiredString(params map[string]interface{}, key string) (string, error) {
	if v, ok := params[key].(string); ok && v != "" {
		return v, nil
	}
	return "", fmt.Errorf("%w: %s", ErrMissingParameter, key)
}

// ExtractOptionalString extracts an optional string parameter.
// Returns empty string if the parameter is missing or empty.
func ExtractOptionalString(params map[string]interface{}, key string) string {
	if v, ok := params[key].(string); ok {
		return v
	}
	return ""
}

// ExtractOptionalStringWithDefault extracts an optional string parameter with a default value.
// Returns defaultValue if the parameter is missing or empty.
func ExtractOptionalStringWithDefault(params map[string]interface{}, key, defaultValue string) string {
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

// ExtractFormat extracts the format parameter with "json" as default.
func ExtractFormat(params map[string]interface{}) string {
	return ExtractOptionalStringWithDefault(params, ParamFormat, FormatJSON)
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

// ExtractOptionalInt64 extracts an optional int64 parameter
func ExtractOptionalInt64(params map[string]interface{}, key string) *int64 {
	if v, ok := params[key].(float64); ok {
		val := int64(v)
		return &val
	}
	if v, ok := params[key].(int64); ok {
		return &v
	}
	if v, ok := params[key].(int); ok {
		val := int64(v)
		return &val
	}
	return nil
}

// ExtractInt64 extracts an int64 parameter with a default value
func ExtractInt64(params map[string]interface{}, key string, defaultValue int64) int64 {
	if v, ok := params[key].(float64); ok {
		return int64(v)
	}
	if v, ok := params[key].(int64); ok {
		return v
	}
	if v, ok := params[key].(int); ok {
		return int64(v)
	}
	return defaultValue
}

// ExtractAndValidateFormat extracts format parameter and validates it.
// Returns validated format or error if format is invalid.
func ExtractAndValidateFormat(params map[string]interface{}) (string, error) {
	format := ExtractFormat(params)
	if err := ValidateFormat(format); err != nil {
		return "", err
	}
	return format, nil
}

// ApplyPagination applies limit and page to a slice, returns paginated slice and total count.
func ApplyPagination[T any](items []T, limit, page int64) ([]T, int64) {
	total := int64(len(items))
	if limit <= 0 {
		return items, total
	}
	if page <= 0 {
		page = 1
	}
	start := (page - 1) * limit
	if start >= total {
		return []T{}, total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return items[start:end], total
}
