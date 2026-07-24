package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/futuretea/rancher-mcp-server/pkg/core/config"
	"github.com/futuretea/rancher-mcp-server/pkg/server/mcp"
)

func TestServeHealthEndpoint(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
	})

	cfg := &config.StaticConfig{
		Port:     listener.Addr().(*net.TCPAddr).Port,
		LogLevel: 0,
	}

	mcpConfig := mcp.Configuration{
		StaticConfig: cfg,
	}

	server, err := mcp.NewServer(mcpConfig)
	if err != nil {
		t.Fatalf("failed to create MCP server: %v", err)
	}
	defer server.Close()

	httpServer := newHTTPServer(server, cfg)
	t.Cleanup(func() {
		_ = httpServer.Shutdown(context.Background())
	})

	serverErr := make(chan error, 1)
	go func() {
		if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// Test health endpoint
	resp, err := waitForHTTPGet(fmt.Sprintf("http://%s/healthz", listener.Addr().String()), 2*time.Second)
	if err != nil {
		t.Fatalf("failed to call health endpoint: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var payload mcp.HealthStatus
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode health payload: %v", err)
	}
	if payload.Status != "ok" {
		t.Fatalf("expected health status ok, got %s", payload.Status)
	}
	if payload.Capabilities["rancher"].Available {
		t.Fatalf("expected rancher capability to be unavailable without config")
	}
	if payload.Capabilities["kubernetes"].Available {
		t.Fatalf("expected kubernetes capability to be unavailable without config")
	}

	select {
	case err := <-serverErr:
		t.Fatalf("server returned unexpected error: %v", err)
	default:
	}
}

func TestMetricsEndpointExposesExpvarMetrics(t *testing.T) {
	cfg := &config.StaticConfig{LogLevel: 0}
	server, err := mcp.NewServer(mcp.Configuration{StaticConfig: cfg})
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}
	defer server.Close()

	httpServer := newHTTPServer(server, cfg)
	response := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, metricsEndpoint, nil))

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}

	var metrics map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &metrics); err != nil {
		t.Fatalf("failed to decode metrics response: %v", err)
	}
	expectedMetricNames := map[string]struct{}{
		"client_resolve_duration":          {},
		"client_resolve_memory_bytes":      {},
		"active_client_count":              {},
		"rancher_request_errors":           {},
		"client_resolve_duration_count":    {},
		"client_resolve_duration_total_ms": {},
	}
	if len(metrics) != len(expectedMetricNames) {
		t.Errorf("expected exactly %d metrics, got %d", len(expectedMetricNames), len(metrics))
	}
	for name := range expectedMetricNames {
		if _, ok := metrics[name]; !ok {
			t.Errorf("expected metric %q to be published", name)
		}
	}
	for name := range metrics {
		if _, ok := expectedMetricNames[name]; !ok {
			t.Errorf("unexpected metric %q exposed", name)
		}
	}
	if _, ok := metrics["cmdline"]; ok {
		t.Fatal("metrics endpoint must not expose process command-line arguments")
	}
}

func waitForHTTPGet(url string, timeout time.Duration) (*http.Response, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		time.Sleep(20 * time.Millisecond)
	}
	return nil, lastErr
}
