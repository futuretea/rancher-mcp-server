package formatutil

import (
	"strings"
	"testing"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{"short", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"long ascii", "hello world", 8, "hello..."},
		{"short max", "hello world", 3, "hel"},
		{"unicode fit", "你好世界", 4, "你好世界"},
		{"unicode truncate", "你好世界", 3, "你好世"},
		{"unicode short max", "你好世界", 2, "你好"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Truncate(tt.s, tt.maxLen); got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestToAnySlice(t *testing.T) {
	in := []string{"a", "b", "c"}
	out := ToAnySlice(in)
	if len(out) != len(in) {
		t.Fatalf("len %d, want %d", len(out), len(in))
	}
	for i, v := range in {
		if out[i] != v {
			t.Errorf("index %d: %v, want %v", i, out[i], v)
		}
	}
}

func TestTableBuilder(t *testing.T) {
	var b strings.Builder
	tb := NewTableBuilder("%-10s", "NAME")
	tb.AddColumn("%-5s", "COUNT")
	tb.WriteHeader(&b)
	tb.WriteSeparator(&b)
	tb.WriteRow(&b, []interface{}{"foo", "1"})

	got := b.String()
	if !strings.Contains(got, "NAME") {
		t.Errorf("header missing NAME: %q", got)
	}
	if !strings.Contains(got, "foo") {
		t.Errorf("row missing foo: %q", got)
	}
}

func TestResourceQuantityToMilli(t *testing.T) {
	tests := []struct {
		q    string
		want int64
	}{
		{"", 0},
		{"100m", 100},
		{"1", 1000},
		{"invalid", 0},
	}
	for _, tt := range tests {
		t.Run(tt.q, func(t *testing.T) {
			if got := ResourceQuantityToMilli(tt.q); got != tt.want {
				t.Errorf("ResourceQuantityToMilli(%q) = %d, want %d", tt.q, got, tt.want)
			}
		})
	}
}

func TestResourceQuantityToBytes(t *testing.T) {
	tests := []struct {
		q    string
		want int64
	}{
		{"", 0},
		{"1Ki", 1024},
		{"1Mi", 1024 * 1024},
		{"invalid", 0},
	}
	for _, tt := range tests {
		t.Run(tt.q, func(t *testing.T) {
			if got := ResourceQuantityToBytes(tt.q); got != tt.want {
				t.Errorf("ResourceQuantityToBytes(%q) = %d, want %d", tt.q, got, tt.want)
			}
		})
	}
}

func TestCalcPercentage(t *testing.T) {
	if got := CalcPercentage(50, 100); got != 50 {
		t.Errorf("CalcPercentage(50, 100) = %f, want 50", got)
	}
	if got := CalcPercentage(50, 0); got != 0 {
		t.Errorf("CalcPercentage(50, 0) = %f, want 0", got)
	}
}
