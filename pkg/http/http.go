package http

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/futuretea/rancher-mcp-server/pkg/config"
	"github.com/futuretea/rancher-mcp-server/pkg/logging"
	"github.com/futuretea/rancher-mcp-server/pkg/mcp"
)

const (
	healthEndpoint     = "/healthz"
	mcpEndpoint        = "/mcp"
	sseEndpoint        = "/sse"
	sseMessageEndpoint = "/message"
)

func Serve(ctx context.Context, mcpServer *mcp.Server, staticConfig *config.StaticConfig) error {
	mux := http.NewServeMux()

	wrappedMux := RequestMiddleware(mux)

	httpServer := &http.Server{
		Addr:    staticConfig.GetPortString(),
		Handler: wrappedMux,
	}

	sseServer := mcpServer.ServeSse(staticConfig.SSEBaseURL, httpServer)
	streamableHttpServer := mcpServer.ServeHTTP(httpServer)
	mux.Handle(sseEndpoint, sseServer)
	mux.Handle(sseMessageEndpoint, sseServer)
	mux.Handle(mcpEndpoint, streamableHttpServer)
	mux.HandleFunc(healthEndpoint, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM)

	serverErr := make(chan error, 1)
	go func() {
		logging.Info("Streaming and SSE HTTP servers starting on port %s and paths /mcp, /sse, /message", staticConfig.GetPortString())
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case sig := <-sigChan:
		logging.Info("Received signal %v, initiating graceful shutdown", sig)
		cancel()
	case <-ctx.Done():
		logging.Info("Context cancelled, initiating graceful shutdown")
	case err := <-serverErr:
		logging.Error("HTTP server error: %v", err)
		return err
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	logging.Info("Shutting down HTTP server gracefully...")
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logging.Error("HTTP server shutdown error: %v", err)
		return err
	}

	logging.Info("HTTP server shutdown complete")
	return nil
}
