package handler

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const maskedValue = "***"

// SensitiveRule defines which fields to mask for a given resource kind.
type SensitiveRule struct {
	// Kind is the lowercase resource kind (e.g., "secret").
	Kind string
	// Fields are the top-level fields whose values should be masked (e.g., ["data", "stringData"]).
	Fields []string
}

// SensitiveDataFilter masks values in sensitive fields based on resource kind.
// It preserves map keys but replaces all values with a masked placeholder.
type SensitiveDataFilter struct {
	rules []SensitiveRule
}

// NewSensitiveDataFilter creates a new SensitiveDataFilter with the specified rules.
func NewSensitiveDataFilter(rules []SensitiveRule) *SensitiveDataFilter {
	return &SensitiveDataFilter{
		rules: rules,
	}
}

// DefaultSensitiveRules returns the default set of sensitive rules.
// Currently masks Secret data and stringData fields.
func DefaultSensitiveRules() []SensitiveRule {
	return []SensitiveRule{
		{
			Kind:   "secret",
			Fields: []string{"data", "stringData"},
		},
	}
}

// NewSensitiveDataFilterFromParams creates a SensitiveDataFilter from handler params.
// Returns nil if showSensitiveData is true (i.e., no masking needed).
func NewSensitiveDataFilterFromParams(params map[string]interface{}) *SensitiveDataFilter {
	if ExtractBool(params, ParamShowSensitiveData, false) {
		return nil
	}
	return NewSensitiveDataFilter(DefaultSensitiveRules())
}

// Filter masks sensitive field values in a resource and returns a cleaned copy.
// The original resource is not modified.
// If the resource kind does not match any rule, it is returned unchanged.
func (f *SensitiveDataFilter) Filter(obj *unstructured.Unstructured) *unstructured.Unstructured {
	if obj == nil || len(f.rules) == 0 {
		return obj
	}

	rule := f.findRule(obj.GetKind())
	if rule == nil {
		return obj
	}

	// Deep copy to avoid modifying the original
	result := obj.DeepCopy()

	for _, field := range rule.Fields {
		f.maskField(result.Object, field)
	}

	return result
}

// FilterList applies sensitive masking to all resources in a list.
func (f *SensitiveDataFilter) FilterList(list *unstructured.UnstructuredList) *unstructured.UnstructuredList {
	if list == nil || len(f.rules) == 0 {
		return list
	}

	result := &unstructured.UnstructuredList{
		Object: list.Object,
		Items:  make([]unstructured.Unstructured, len(list.Items)),
	}

	for i, item := range list.Items {
		filtered := f.Filter(&item)
		if filtered != nil {
			result.Items[i] = *filtered
		}
	}

	return result
}

// findRule returns the matching rule for the given kind, or nil if none matches.
func (f *SensitiveDataFilter) findRule(kind string) *SensitiveRule {
	kindLower := strings.ToLower(kind)
	for i := range f.rules {
		if f.rules[i].Kind == kindLower {
			return &f.rules[i]
		}
	}
	return nil
}

// maskField replaces all values in a top-level map field with the masked placeholder.
// If the field does not exist or is not a map, it is left unchanged.
// Safe to modify in-place because Filter() always deep-copies first.
func (f *SensitiveDataFilter) maskField(obj map[string]interface{}, field string) {
	raw, ok := obj[field]
	if !ok {
		return
	}

	dataMap, ok := raw.(map[string]interface{})
	if !ok {
		return
	}

	for key := range dataMap {
		dataMap[key] = maskedValue
	}
}
