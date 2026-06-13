// Package formatutil provides shared formatting helpers for the kubernetes toolset.
package formatutil

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"k8s.io/apimachinery/pkg/api/resource"
)

// TableBuilder helps build formatted text tables column by column.
type TableBuilder struct {
	formats []string
	headers []string
}

// NewTableBuilder creates a TableBuilder with the first column format and header.
func NewTableBuilder(format, header string) *TableBuilder {
	return &TableBuilder{
		formats: []string{format},
		headers: []string{header},
	}
}

// AddColumn appends one or more columns with the same format string.
func (tb *TableBuilder) AddColumn(format string, headers ...string) {
	for range headers {
		tb.formats = append(tb.formats, format)
	}
	tb.headers = append(tb.headers, headers...)
}

// WriteHeader writes the table header row into b.
func (tb *TableBuilder) WriteHeader(b *strings.Builder) {
	fmt.Fprintf(b, strings.Join(tb.formats, " ")+"\n", ToAnySlice(tb.headers)...)
}

// WriteSeparator writes a separator line matching the header widths into b.
func (tb *TableBuilder) WriteSeparator(b *strings.Builder) {
	separators := make([]string, len(tb.headers))
	for i, h := range tb.headers {
		separators[i] = strings.Repeat("-", len(h))
	}
	fmt.Fprintf(b, strings.Join(tb.formats, " ")+"\n", ToAnySlice(separators)...)
}

// WriteRow writes a data row into b.
func (tb *TableBuilder) WriteRow(b *strings.Builder, values []interface{}) {
	fmt.Fprintf(b, strings.Join(tb.formats, " ")+"\n", values...)
}

// ToAnySlice converts a string slice to an any slice for fmt formatting.
func ToAnySlice(ss []string) []any {
	result := make([]any, len(ss))
	for i, s := range ss {
		result[i] = s
	}
	return result
}

// Truncate truncates s to maxLen runes, appending "..." when truncated.
// Returns s unchanged if it is already within the limit.
func Truncate(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

// ResourceQuantityToMilli parses a resource quantity string and returns millivalue.
// Invalid or empty quantities are treated as zero instead of panicking.
func ResourceQuantityToMilli(q string) int64 {
	if q == "" {
		return 0
	}
	qty, err := resource.ParseQuantity(q)
	if err != nil {
		return 0
	}
	return qty.MilliValue()
}

// ResourceQuantityToBytes parses a resource quantity string and returns bytes.
// Invalid or empty quantities are treated as zero instead of panicking.
func ResourceQuantityToBytes(q string) int64 {
	if q == "" {
		return 0
	}
	qty, err := resource.ParseQuantity(q)
	if err != nil {
		return 0
	}
	return qty.Value()
}

// CalcPercentage calculates the utilization percentage, returning 0 for non-positive totals.
func CalcPercentage(value, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(value) / float64(total) * 100
}
