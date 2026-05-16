package http

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	select {
	case err := <-serverErr:
		t.Fatalf("server returned unexpected error: %v", err)
	default:
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
