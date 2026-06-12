package kubernetes

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
)

func int64Ptr(v int64) *int64 { return &v }

type mockMultiPodLogClient struct {
	opts    *steve.PodLogOptions
	results []steve.MultiPodLogResult
	err     error
}

func (m *mockMultiPodLogClient) GetMultiPodLogs(ctx context.Context, clusterID, namespace, labelSelector string, opts *steve.PodLogOptions) ([]steve.MultiPodLogResult, error) {
	m.opts = opts
	return m.results, m.err
}

type mockAllContainerLogClient struct {
	opts *steve.PodLogOptions
	logs map[string]string
	err  error
}

func (m *mockAllContainerLogClient) GetAllContainerLogs(ctx context.Context, clusterID, namespace, podName string, opts *steve.PodLogOptions) (map[string]string, error) {
	m.opts = opts
	return m.logs, m.err
}

func TestGetMultiPodLogs_PropagatesOptions(t *testing.T) {
	since := int64(120)
	client := &mockMultiPodLogClient{
		results: []steve.MultiPodLogResult{
			{Pod: "pod-1", Logs: map[string]string{"app": "hello"}},
		},
	}

	_, err := getMultiPodLogs(context.Background(), client, "c1", "ns", "app=web", "", 50, &since, true, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.opts == nil {
		t.Fatal("expected PodLogOptions to be passed to client")
	}
	if client.opts.SinceSeconds == nil || *client.opts.SinceSeconds != since {
		t.Errorf("SinceSeconds = %v, want %d", client.opts.SinceSeconds, since)
	}
	if !client.opts.Previous {
		t.Error("expected Previous to be true")
	}
}

func TestGetAllContainerLogs_PropagatesOptions(t *testing.T) {
	since := int64(120)
	client := &mockAllContainerLogClient{
		logs: map[string]string{"app": "hello"},
	}

	_, err := getAllContainerLogs(context.Background(), client, "c1", "ns", "pod-1", &steve.PodLogOptions{
		TailLines:    int64Ptr(50),
		SinceSeconds: &since,
		Previous:     true,
	}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.opts == nil {
		t.Fatal("expected PodLogOptions to be passed to client")
	}
	if client.opts.SinceSeconds == nil || *client.opts.SinceSeconds != since {
		t.Errorf("SinceSeconds = %v, want %d", client.opts.SinceSeconds, since)
	}
	if !client.opts.Previous {
		t.Error("expected Previous to be true")
	}
}

func TestGetAllContainerLogs_HappyPath(t *testing.T) {
	client := &mockAllContainerLogClient{
		logs: map[string]string{"app": "line1\nline2"},
	}

	out, err := getAllContainerLogs(context.Background(), client, "c1", "ns", "pod-1", &steve.PodLogOptions{TailLines: int64Ptr(10)}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "[app] line1") {
		t.Errorf("expected output to contain '[app] line1', got %q", out)
	}
	if !strings.Contains(out, "[app] line2") {
		t.Errorf("expected output to contain '[app] line2', got %q", out)
	}
}

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
