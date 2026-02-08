package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/futuretea/rancher-mcp-server/pkg/core/config"
	"github.com/futuretea/rancher-mcp-server/pkg/server/mcp"
)

func TestServeHealthEndpoint(t *testing.T) {
	// Use a dynamic port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	cfg := &config.StaticConfig{
		Port:     port,
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

	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = Serve(ctx, server, cfg)
	}()

	// Wait for server to start
	time.Sleep(500 * time.Millisecond)

	// Test health endpoint
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/healthz", port))
	if err != nil {
		t.Fatalf("failed to call health endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Cancel context to stop server
	cancel()
	time.Sleep(100 * time.Millisecond)
}
