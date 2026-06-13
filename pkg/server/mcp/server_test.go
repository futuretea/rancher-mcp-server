package mcp

import (
	"context"
	"encoding/json"
	"expvar"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcptransport "github.com/mark3labs/mcp-go/client/transport"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/rancher/norman/types"

	"github.com/futuretea/rancher-mcp-server/pkg/client/norman"
	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/core/config"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
)

// fakeRancherBackend is a minimal Rancher API double used by the end-to-end
// tests below. It serves the Norman schema discovery endpoint and the Steve
// namespace list endpoint, while recording every Authorization header it sees.
type fakeRancherBackend struct {
	server *httptest.Server

	mu       sync.Mutex
	auths    []string
	schemaOK int64
	listOK   int64
}

func newFakeRancherBackend(tb testing.TB) *fakeRancherBackend {
	f := &fakeRancherBackend{}
	// Use a TLS server so the Steve client treats the downstream URL as secure
	// and includes the bearer token in requests.
	f.server = httptest.NewTLSServer(f)
	tb.Cleanup(f.server.Close)
	return f
}

func (f *fakeRancherBackend) URL() string {
	return f.server.URL
}

func (f *fakeRancherBackend) recordAuth(auth string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.auths = append(f.auths, auth)
}

func (f *fakeRancherBackend) AuthHeaders() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.auths))
	copy(out, f.auths)
	return out
}

func (f *fakeRancherBackend) HasAuth(prefix string) bool {
	for _, h := range f.AuthHeaders() {
		if h == prefix {
			return true
		}
	}
	return false
}

func (f *fakeRancherBackend) SchemaOK() int64 {
	return atomic.LoadInt64(&f.schemaOK)
}

func (f *fakeRancherBackend) ListOK() int64 {
	return atomic.LoadInt64(&f.listOK)
}

func (f *fakeRancherBackend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	auth := r.Header.Get("Authorization")

	// Norman schema discovery: the management client fetches /v3 (or /v3/schemas).
	if strings.HasPrefix(path, "/v3") {
		f.recordAuth(auth)
		atomic.AddInt64(&f.schemaOK, 1)
		f.writeSchemaDiscovery(w, r)
		return
	}

	// Steve namespace list: /k8s/clusters/{clusterID}/api/v1/namespaces
	if strings.HasPrefix(path, "/k8s/clusters/") && strings.HasSuffix(path, "/api/v1/namespaces") {
		f.recordAuth(auth)
		atomic.AddInt64(&f.listOK, 1)
		f.writeNamespaceList(w, r)
		return
	}

	http.NotFound(w, r)
}

func (f *fakeRancherBackend) writeSchemaDiscovery(w http.ResponseWriter, r *http.Request) {
	schemas := types.SchemaCollection{
		Data: []types.Schema{
			{
				ID:      "cluster",
				Type:    "/meta/schemas/schema",
				Links:   map[string]string{},
				Version: types.APIVersion{Path: "/v3", Version: "v3"},
			},
		},
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-API-Schemas", scheme+"://"+r.Host+"/v3/schemas")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(schemas)
}

func (f *fakeRancherBackend) writeNamespaceList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"kind":       "NamespaceList",
		"apiVersion": "v1",
		"metadata":   map[string]any{"resourceVersion": "1"},
		"items": []map[string]any{
			{"metadata": map[string]any{"name": "default"}},
			{"metadata": map[string]any{"name": "kube-system"}},
		},
	})
}

func newDynamicMCPServer(tb testing.TB, backendURL string, toolsets []string) *httptest.Server {
	cfg := Configuration{
		StaticConfig: &config.StaticConfig{
			RancherServerURL:        backendURL,
			RancherRequestTokenAuth: true,
			RancherTLSInsecure:      true,
			Toolsets:                toolsets,
			ListOutput:              "json",
		},
	}

	srv, err := NewServer(cfg)
	if err != nil {
		tb.Fatalf("NewServer() failed: %v", err)
	}

	// Use the HTTP transport handler directly so we can exercise the production
	// contextFunc path without binding to a real address.
	handler := srv.ServeHTTP(nil)
	ts := httptest.NewServer(handler)
	tb.Cleanup(ts.Close)
	return ts
}

func newDynamicSSEMCPServer(tb testing.TB, backendURL string, toolsets []string) *httptest.Server {
	cfg := Configuration{
		StaticConfig: &config.StaticConfig{
			RancherServerURL:        backendURL,
			RancherRequestTokenAuth: true,
			RancherTLSInsecure:      true,
			Toolsets:                toolsets,
			ListOutput:              "json",
		},
	}

	srv, err := NewServer(cfg)
	if err != nil {
		tb.Fatalf("NewServer() failed: %v", err)
	}

	sseServer := srv.ServeSse("", nil)
	ts := httptest.NewServer(sseServer)
	tb.Cleanup(ts.Close)
	return ts
}

func newStreamableMCPClient(tb testing.TB, serverURL, token string) *mcpclient.Client {
	baseURL := serverURL + "/mcp"
	headers := map[string]string{"Authorization": "Bearer " + token}
	client, err := mcpclient.NewStreamableHttpClient(baseURL, mcptransport.WithHTTPHeaders(headers))
	if err != nil {
		tb.Fatalf("NewStreamableHttpClient(%q) failed: %v", baseURL, err)
	}
	tb.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		tb.Fatalf("client.Start() failed: %v", err)
	}

	_, err = client.Initialize(ctx, mcpgo.InitializeRequest{
		Params: mcpgo.InitializeParams{
			ProtocolVersion: mcpgo.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcpgo.Implementation{
				Name:    "rancher-mcp-server-test",
				Version: "0.0.0",
			},
			Capabilities: mcpgo.ClientCapabilities{},
		},
	})
	if err != nil {
		tb.Fatalf("client.Initialize() failed: %v", err)
	}
	return client
}

func callListNamespaces(ctx context.Context, tb testing.TB, client *mcpclient.Client) *mcpgo.CallToolResult {
	tb.Helper()
	result, err := client.CallTool(ctx, mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name: "kubernetes_list",
			Arguments: map[string]any{
				"cluster": "local",
				"kind":    "namespace",
				"format":  "json",
			},
		},
	})
	if err != nil {
		tb.Fatalf("CallTool(kubernetes_list) failed: %v", err)
	}
	return result
}

// TestDynamicAuthHTTPToolCall verifies the full HTTP/SSE-style request-token
// path: an Authorization header from the MCP client is forwarded to the
// downstream Rancher APIs by the per-request resolver.
func TestDynamicAuthHTTPToolCall(t *testing.T) {
	backend := newFakeRancherBackend(t)
	mcpServer := newDynamicMCPServer(t, backend.URL(), []string{"kubernetes"})

	const token = "integration-token"
	client := newStreamableMCPClient(t, mcpServer.URL, token)

	result := callListNamespaces(context.Background(), t, client)
	if result.IsError {
		t.Fatalf("tool returned error: %v", result.Content)
	}

	text := toolResultText(result)
	if !strings.Contains(text, "default") {
		t.Fatalf("expected namespace list to contain 'default', got: %s", text)
	}
	if !strings.Contains(text, "kube-system") {
		t.Fatalf("expected namespace list to contain 'kube-system', got: %s", text)
	}

	if !backend.HasAuth("Bearer " + token) {
		t.Fatalf("backend did not receive expected Bearer token; observed headers: %v", backend.AuthHeaders())
	}
	if backend.SchemaOK() == 0 {
		t.Fatal("backend did not receive any Norman schema discovery requests")
	}
	if backend.ListOK() == 0 {
		t.Fatal("backend did not receive any Steve namespace list requests")
	}
	for _, h := range backend.AuthHeaders() {
		if h != "Bearer "+token {
			t.Fatalf("backend observed unexpected Authorization header %q; all requests should carry the per-request token", h)
		}
	}
}

// TestDynamicAuthSSEToolCall verifies the SSE request-token path: the
// Authorization header is propagated through the SSE handshake and each
// subsequent tool call message.
func TestDynamicAuthSSEToolCall(t *testing.T) {
	backend := newFakeRancherBackend(t)
	mcpServer := newDynamicSSEMCPServer(t, backend.URL(), []string{"kubernetes"})

	const token = "sse-integration-token"
	headers := map[string]string{"Authorization": "Bearer " + token}
	client, err := mcpclient.NewSSEMCPClient(mcpServer.URL+"/sse", mcptransport.WithHeaders(headers))
	if err != nil {
		t.Fatalf("NewSSEMCPClient(%q) failed: %v", mcpServer.URL, err)
	}
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("client.Start() failed: %v", err)
	}
	_, err = client.Initialize(ctx, mcpgo.InitializeRequest{
		Params: mcpgo.InitializeParams{
			ProtocolVersion: mcpgo.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcpgo.Implementation{
				Name:    "rancher-mcp-server-test",
				Version: "0.0.0",
			},
			Capabilities: mcpgo.ClientCapabilities{},
		},
	})
	if err != nil {
		t.Fatalf("client.Initialize() failed: %v", err)
	}

	result := callListNamespaces(ctx, t, client)
	if result.IsError {
		t.Fatalf("tool returned error: %v", result.Content)
	}

	text := toolResultText(result)
	if !strings.Contains(text, "default") {
		t.Fatalf("expected namespace list to contain 'default', got: %s", text)
	}
	if !backend.HasAuth("Bearer " + token) {
		t.Fatalf("backend did not receive expected Bearer token; observed headers: %v", backend.AuthHeaders())
	}
}

// TestDynamicAuthHTTPMissingHeader verifies fail-closed behavior when the
// upstream request does not carry an Authorization header.
func TestDynamicAuthHTTPMissingHeader(t *testing.T) {
	backend := newFakeRancherBackend(t)
	mcpServer := newDynamicMCPServer(t, backend.URL(), []string{"kubernetes"})

	baseURL := mcpServer.URL + "/mcp"
	client, err := mcpclient.NewStreamableHttpClient(baseURL)
	if err != nil {
		t.Fatalf("NewStreamableHttpClient(%q) failed: %v", baseURL, err)
	}
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("client.Start() failed: %v", err)
	}
	_, err = client.Initialize(ctx, mcpgo.InitializeRequest{
		Params: mcpgo.InitializeParams{
			ProtocolVersion: mcpgo.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcpgo.Implementation{
				Name:    "rancher-mcp-server-test",
				Version: "0.0.0",
			},
			Capabilities: mcpgo.ClientCapabilities{},
		},
	})
	if err != nil {
		t.Fatalf("client.Initialize() failed: %v", err)
	}

	result, err := client.CallTool(ctx, mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name: "kubernetes_list",
			Arguments: map[string]any{
				"cluster": "local",
				"kind":    "namespace",
				"format":  "json",
			},
		},
	})
	if err != nil {
		t.Fatalf("CallTool() transport failed: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected tool error for missing Authorization header, got success")
	}
	text := toolResultText(result)
	if !strings.Contains(text, "Authorization") && !strings.Contains(text, "per-request Rancher token") {
		t.Fatalf("expected clear missing-token error, got: %s", text)
	}

	for _, h := range backend.AuthHeaders() {
		if h != "" {
			t.Fatalf("backend received request without token: %q", h)
		}
	}
}

// TestDynamicAuthConcurrentToolCallsNoTokenLeakage spawns concurrent callers
// with two distinct tokens and asserts that every downstream request carries
// one of the two expected tokens and that both tokens are observed.
func TestDynamicAuthConcurrentToolCallsNoTokenLeakage(t *testing.T) {
	backend := newFakeRancherBackend(t)
	mcpServer := newDynamicMCPServer(t, backend.URL(), []string{"kubernetes"})

	const (
		tokenA = "concurrent-token-a"
		tokenB = "concurrent-token-b"
		rounds = 20
	)

	clientA := newStreamableMCPClient(t, mcpServer.URL, tokenA)
	clientB := newStreamableMCPClient(t, mcpServer.URL, tokenB)

	var wg sync.WaitGroup
	wg.Add(2)

	caller := func(client *mcpclient.Client) {
		defer wg.Done()
		ctx := context.Background()
		for i := 0; i < rounds; i++ {
			callListNamespaces(ctx, t, client)
		}
	}

	go caller(clientA)
	go caller(clientB)
	wg.Wait()

	seenA := false
	seenB := false
	for _, h := range backend.AuthHeaders() {
		switch h {
		case "Bearer " + tokenA:
			seenA = true
		case "Bearer " + tokenB:
			seenB = true
		default:
			t.Fatalf("observed unexpected Authorization header %q", h)
		}
	}
	if !seenA {
		t.Fatalf("backend never received token A")
	}
	if !seenB {
		t.Fatalf("backend never received token B")
	}
}

// newBenchmarkResolver returns a requestTokenResolver with no-op factories for
// use in benchmarks. It isolates resolver overhead from network latency.
func newBenchmarkResolver(b *testing.B) *requestTokenResolver {
	b.Helper()
	return &requestTokenResolver{
		serverURL: "https://rancher.example.com",
		insecure:  true,
		steveFactory: func(_, _ string, _ bool) *steve.Client {
			return &steve.Client{}
		},
		normanFactory: func(_, _ string, _ bool) (*norman.Client, error) {
			return &norman.Client{}, nil
		},
		metrics: NewExpvarMetrics(),
	}
}

// BenchmarkRequestTokenResolver measures the per-request client construction
// overhead of requestTokenResolver. The factories are no-op so the benchmark
// isolates token parsing, client allocation, and metrics recording from network
// latency.
func BenchmarkRequestTokenResolver(b *testing.B) {
	r := newBenchmarkResolver(b)
	ctx := context.WithValue(context.Background(), authorizationKey, "Bearer benchmark-token")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client, err := r.Resolve(ctx)
		if err != nil {
			b.Fatalf("Resolve() failed: %v", err)
		}
		client.Close()
	}
	b.StopTimer()
	reportMetrics(b)
}

// BenchmarkRequestTokenResolverParallel is the concurrent variant of
// BenchmarkRequestTokenResolver. It exercises the resolver with default
// RunParallel goroutines to surface any lock contention on metrics.
func BenchmarkRequestTokenResolverParallel(b *testing.B) {
	r := newBenchmarkResolver(b)
	ctx := context.WithValue(context.Background(), authorizationKey, "Bearer benchmark-token")

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			client, err := r.Resolve(ctx)
			if err != nil {
				b.Fatalf("Resolve() failed: %v", err)
			}
			client.Close()
		}
	})
	b.StopTimer()
	reportMetrics(b)
}

// BenchmarkCombinedClientClose measures the cost of releasing a request-scoped
// CombinedClient. Static clients must be no-ops; request-scoped clients clear
// internal caches.
func BenchmarkCombinedClientClose(b *testing.B) {
	client := toolset.NewCombinedClient(&norman.Client{}, &steve.Client{}, true)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.Close()
	}
}

func toolResultText(result *mcpgo.CallToolResult) string {
	var parts []string
	for _, c := range result.Content {
		if text, ok := c.(mcpgo.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}
	return strings.Join(parts, "")
}

func reportMetrics(b *testing.B) {
	b.Helper()
	duration := expvar.Get("client_resolve_duration")
	mem := expvar.Get("client_resolve_memory_bytes")
	active := expvar.Get("active_client_count")
	errors := expvar.Get("rancher_request_errors")
	if duration != nil {
		b.Logf("client_resolve_duration_ms=%s", duration.String())
	}
	if mem != nil {
		b.Logf("client_resolve_memory_bytes=%s", mem.String())
	}
	if active != nil {
		b.Logf("active_client_count=%s", active.String())
	}
	if errors != nil {
		b.Logf("rancher_request_errors=%s", errors.String())
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	b.Logf("heap_alloc_bytes=%d goroutines=%d", m.Alloc, runtime.NumGoroutine())
}
