package kubernetes

import (
	"testing"
	"time"
)

func TestParseLogTimestamp(t *testing.T) {
	tests := []struct {
		name          string
		line          string
		wantTimestamp string
		wantContent   string
		wantZero      bool
	}{
		{
			name:          "rfc3339 nano with content",
			line:          "2024-01-15T10:30:00.123456789Z log message",
			wantTimestamp: "2024-01-15T10:30:00.123456789Z",
			wantContent:   "log message",
		},
		{
			name:          "rfc3339 nano without content",
			line:          "2024-01-15T10:30:00.123456789Z",
			wantTimestamp: "2024-01-15T10:30:00.123456789Z",
			wantContent:   "",
		},
		{
			name:          "rfc3339 with content",
			line:          "2024-01-15T10:30:00Z log message",
			wantTimestamp: "2024-01-15T10:30:00Z",
			wantContent:   "log message",
		},
		{
			name:          "rfc3339 without content",
			line:          "2024-01-15T10:30:00Z",
			wantTimestamp: "2024-01-15T10:30:00Z",
			wantContent:   "",
		},
		{
			name:        "not timestamped",
			line:        "plain log message",
			wantContent: "plain log message",
			wantZero:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTime, gotContent := parseLogTimestamp(tt.line)
			if tt.wantZero && !gotTime.IsZero() {
				t.Fatalf("parseLogTimestamp() time = %v, want zero", gotTime)
			}
			if !tt.wantZero && gotTime.IsZero() {
				t.Fatal("parseLogTimestamp() time is zero, want parsed timestamp")
			}
			if tt.wantTimestamp != "" {
				wantTime, err := time.Parse(time.RFC3339Nano, tt.wantTimestamp)
				if err != nil {
					t.Fatalf("failed to parse expected timestamp: %v", err)
				}
				if !gotTime.Equal(wantTime) {
					t.Fatalf("parseLogTimestamp() time = %v, want %v", gotTime, wantTime)
				}
			}
			if gotContent != tt.wantContent {
				t.Fatalf("parseLogTimestamp() content = %q, want %q", gotContent, tt.wantContent)
			}
		})
	}
}

func TestSortLogsByTime_TimestampOnlyLine(t *testing.T) {
	timestamp := "2024-01-15T10:30:00.123456789Z"
	got := sortLogsByTime(timestamp, true)

	wantTime, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		t.Fatalf("failed to parse test timestamp: %v", err)
	}
	if got != wantTime.Format(time.RFC3339Nano) {
		t.Fatalf("sortLogsByTime() = %q, want timestamp-only output", got)
	}
}

func TestFormatTimestampedContent(t *testing.T) {
	timestamp, err := time.Parse(time.RFC3339Nano, "2024-01-15T10:30:00.123456789Z")
	if err != nil {
		t.Fatalf("failed to parse test timestamp: %v", err)
	}

	if got := formatTimestampedContent(timestamp, "log message"); got != "2024-01-15T10:30:00.123456789Z log message" {
		t.Fatalf("formatTimestampedContent() = %q, want timestamp and content", got)
	}
	if got := formatTimestampedContent(timestamp, ""); got != "2024-01-15T10:30:00.123456789Z" {
		t.Fatalf("formatTimestampedContent() = %q, want timestamp without trailing space", got)
	}
}
