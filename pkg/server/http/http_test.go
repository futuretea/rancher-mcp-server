package http

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/futuretea/rancher-mcp-server/pkg/core/config"
	"github.com/futuretea/rancher-mcp-server/pkg/core/logging"
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

func TestOAuthOnlyRoutes_ServeReferenceMetadata(t *testing.T) {
	staticConfig := newOAuthHTTPTestConfig(t)
	mcpServer := newOAuthHTTPTestMCPServer(t, staticConfig)
	httpServer := newHTTPServer(mcpServer, staticConfig)

	metadataResponse := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(metadataResponse, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil))

	if metadataResponse.Code != http.StatusOK {
		t.Fatalf("expected protected-resource metadata status 200, got %d", metadataResponse.Code)
	}
	if contentType := metadataResponse.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected protected-resource metadata content type application/json, got %q", contentType)
	}

	var metadata oauthProtectedResourceMetadata
	if err := json.NewDecoder(metadataResponse.Body).Decode(&metadata); err != nil {
		t.Fatalf("decode protected-resource metadata: %v", err)
	}
	if metadata.Resource != staticConfig.RancherOAuthResourceURL {
		t.Errorf("expected metadata resource %q, got %q", staticConfig.RancherOAuthResourceURL, metadata.Resource)
	}
	if !slices.Equal(metadata.AuthorizationServers, []string{staticConfig.RancherOAuthAuthorizationServerURL}) {
		t.Errorf("expected metadata authorization_servers [%q], got %q", staticConfig.RancherOAuthAuthorizationServerURL, metadata.AuthorizationServers)
	}
	if !slices.Equal(metadata.ScopesSupported, []string{"offline_access", "rancher:mcp"}) {
		t.Errorf("expected metadata scopes_supported [offline_access rancher:mcp], got %q", metadata.ScopesSupported)
	}

	optionsResponse := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(optionsResponse, httptest.NewRequest(http.MethodOptions, oauthMetadataPath, nil))
	if optionsResponse.Code != http.StatusOK {
		t.Fatalf("expected metadata OPTIONS status 200, got %d", optionsResponse.Code)
	}
	if got := optionsResponse.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("expected metadata CORS allow origin *, got %q", got)
	}
	if got := optionsResponse.Header().Get("Access-Control-Allow-Methods"); got != "GET, OPTIONS" {
		t.Errorf("expected metadata CORS methods GET, OPTIONS, got %q", got)
	}
	if got := optionsResponse.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type" {
		t.Errorf("expected metadata CORS headers Content-Type, got %q", got)
	}
}

func TestOAuthOnlyRoutes_ChallengeUnauthenticatedMCP(t *testing.T) {
	staticConfig := newOAuthHTTPTestConfig(t)
	mcpServer := newOAuthHTTPTestMCPServer(t, staticConfig)
	httpServer := newHTTPServer(mcpServer, staticConfig)

	challengeResponse := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(challengeResponse, httptest.NewRequest(http.MethodPost, mcpEndpoint, nil))

	if challengeResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated MCP request status 401, got %d", challengeResponse.Code)
	}
	wantChallenge := `Bearer resource_metadata="` + staticConfig.RancherOAuthResourceURL + `/.well-known/oauth-protected-resource"`
	if gotChallenge := challengeResponse.Header().Get("WWW-Authenticate"); gotChallenge != wantChallenge {
		t.Errorf("expected credential-free WWW-Authenticate challenge %q, got %q", wantChallenge, gotChallenge)
	}
	if gotAuthorization := challengeResponse.Header().Get("Authorization"); gotAuthorization != "" {
		t.Errorf("expected rejection not to return an Authorization header, got %q", gotAuthorization)
	}
}

func TestOAuthOnlyRoutes_RejectionDoesNotLogBearerToken(t *testing.T) {
	staticConfig := newOAuthHTTPTestConfig(t)
	mcpServer := newOAuthHTTPTestMCPServer(t, staticConfig)
	httpServer := newHTTPServer(mcpServer, staticConfig)

	var logOutput bytes.Buffer
	logging.Initialize(6, &logOutput)
	t.Cleanup(func() { logging.Initialize(0, nil) })

	const token = "raw-bearer-token-that-must-not-be-logged"
	request := httptest.NewRequest(http.MethodPost, mcpEndpoint, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected malformed bearer token to be rejected with 401, got %d", response.Code)
	}
	if strings.Contains(logOutput.String(), token) {
		t.Fatal("OAuth rejection log contained the raw bearer token")
	}
}

func TestOAuthOnlyRoutes_OmitSSEEndpoints(t *testing.T) {
	staticConfig := newOAuthHTTPTestConfig(t)
	mcpServer := newOAuthHTTPTestMCPServer(t, staticConfig)
	httpServer := newHTTPServer(mcpServer, staticConfig)

	for _, route := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: sseEndpoint},
		{method: http.MethodPost, path: sseMessageEndpoint},
	} {
		t.Run(route.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			httpServer.Handler.ServeHTTP(response, newHTTPRouteTestRequest(t, route.method, route.path))

			if response.Code != http.StatusNotFound {
				t.Fatalf("expected OAuth-only %s %s to use the ordinary mux 404, got %d", route.method, route.path, response.Code)
			}
			if challenge := response.Header().Get("WWW-Authenticate"); challenge != "" {
				t.Errorf("expected unregistered OAuth-only route to avoid an auth challenge, got %q", challenge)
			}
		})
	}
}

func TestDirectAndStaticRoutes_KeepMCPAndSSEEndpoints(t *testing.T) {
	rancherServer := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(rancherServer.Close)

	configs := []struct {
		name         string
		staticConfig *config.StaticConfig
	}{
		{
			name: "direct Bearer",
			staticConfig: &config.StaticConfig{
				RancherServerURL:        rancherServer.URL,
				RancherRequestTokenAuth: true,
			},
		},
		{
			name: "static credentials",
			staticConfig: &config.StaticConfig{
				RancherServerURL: rancherServer.URL,
				RancherToken:     "test-static-token",
			},
		},
	}

	for _, tt := range configs {
		t.Run(tt.name, func(t *testing.T) {
			mcpServer, err := mcp.NewServer(mcp.Configuration{StaticConfig: tt.staticConfig})
			if err != nil {
				t.Fatalf("NewServer() failed: %v", err)
			}
			t.Cleanup(mcpServer.Close)

			httpServer := newHTTPServer(mcpServer, tt.staticConfig)
			for _, route := range []struct {
				method string
				path   string
			}{
				{method: http.MethodGet, path: mcpEndpoint},
				{method: http.MethodGet, path: sseEndpoint},
				{method: http.MethodPost, path: sseMessageEndpoint},
			} {
				t.Run(route.path, func(t *testing.T) {
					response := httptest.NewRecorder()
					httpServer.Handler.ServeHTTP(response, newHTTPRouteTestRequest(t, route.method, route.path))

					if response.Code == http.StatusNotFound {
						t.Fatalf("expected %s %s to remain registered outside OAuth-only mode", route.method, route.path)
					}
				})
			}
		})
	}
}

type oauthProtectedResourceMetadata struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers"`
	ScopesSupported      []string `json:"scopes_supported"`
}

func newOAuthHTTPTestConfig(t *testing.T) *config.StaticConfig {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() failed: %v", err)
	}

	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]string{
				{
					"kty": "RSA",
					"kid": "http-route-test-key",
					"alg": "RS256",
					"use": "sig",
					"n":   base64.RawURLEncoding.EncodeToString(privateKey.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.PublicKey.E)).Bytes()),
				},
			},
		}); err != nil {
			t.Errorf("Encode() JWKS failed: %v", err)
		}
	}))
	t.Cleanup(jwksServer.Close)

	return &config.StaticConfig{
		RancherServerURL:                   "https://rancher.example.test",
		RancherOAuthTokenAuth:              true,
		RancherOAuthAuthorizationServerURL: "https://auth.example.test",
		RancherOAuthJWKSURL:                jwksServer.URL,
		RancherOAuthResourceURL:            "https://mcp.example.test",
	}
}

func newOAuthHTTPTestMCPServer(t *testing.T, staticConfig *config.StaticConfig) *mcp.Server {
	t.Helper()

	mcpServer, err := mcp.NewServer(mcp.Configuration{StaticConfig: staticConfig})
	if err != nil {
		t.Fatalf("NewServer() failed with local JWKS fixture: %v", err)
	}
	t.Cleanup(mcpServer.Close)
	return mcpServer
}

func newHTTPRouteTestRequest(t *testing.T, method string, path string) *http.Request {
	t.Helper()

	request := httptest.NewRequest(method, path, nil)
	ctx, cancel := context.WithCancel(request.Context())
	cancel()
	return request.WithContext(ctx)
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
