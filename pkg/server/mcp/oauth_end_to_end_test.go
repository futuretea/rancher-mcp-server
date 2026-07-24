package mcp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// newOAuthDynamicMCPServer exercises the OAuth-only HTTP handler with the
// existing fake Rancher backend. The fixture provides a local, generated JWKS.
func newOAuthDynamicMCPServer(
	tb testing.TB,
	backendURL string,
	fixture *oauthJWKSFixture,
	toolsets []string,
) *httptest.Server {
	tb.Helper()

	staticConfig := oauthTestConfig(fixture.URL())
	staticConfig.RancherServerURL = backendURL
	staticConfig.RancherTLSInsecure = true
	staticConfig.Toolsets = toolsets

	srv, err := NewServer(Configuration{StaticConfig: staticConfig})
	if err != nil {
		tb.Fatalf("NewServer() failed with a local OAuth fixture: %v", err)
	}
	tb.Cleanup(srv.Close)

	ts := httptest.NewServer(srv.ServeOAuthHTTP(nil))
	tb.Cleanup(ts.Close)
	return ts
}

// TestOAuthDynamicAuthHTTPToolCall verifies that an accepted JWT unlocks the
// dynamic capability and is passed through both request-scoped Rancher clients.
func TestOAuthDynamicAuthHTTPToolCall(t *testing.T) {
	fixture := newOAuthJWKSFixture(t)
	backend := newFakeRancherBackend(t)
	mcpServer := newOAuthDynamicMCPServer(t, backend.URL(), fixture, []string{"kubernetes"})

	verifiedToken := fixture.sign(t, validOAuthClaims())
	client := newStreamableMCPClient(t, mcpServer.URL, verifiedToken)

	result, err := callListNamespaces(context.Background(), t, client)
	if err != nil {
		t.Fatal("expected a verified OAuth token to reach the Kubernetes tool")
	}
	if result.IsError {
		t.Fatal("expected a verified OAuth token to enable a successful Kubernetes tool call")
	}
	if backend.SchemaOK() == 0 {
		t.Fatal("expected the verified OAuth token to reach the Norman client path")
	}
	if backend.ListOK() == 0 {
		t.Fatal("expected the verified OAuth token to reach the Steve client path")
	}

	for _, authorization := range backend.AuthHeaders() {
		if authorization != "Bearer "+verifiedToken {
			t.Fatal("backend received a token other than the verified OAuth token")
		}
	}
}

// TestOAuthDynamicAuthRejectedRequestsDoNotReachRancher verifies that the
// OAuth middleware rejects invalid traffic before any dynamic client is built.
func TestOAuthDynamicAuthRejectedRequestsDoNotReachRancher(t *testing.T) {
	fixture := newOAuthJWKSFixture(t)
	backend := newFakeRancherBackend(t)
	mcpServer := newOAuthDynamicMCPServer(t, backend.URL(), fixture, []string{"kubernetes"})

	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() failed: %v", err)
	}

	now := time.Now()
	for _, tt := range []struct {
		name          string
		authorization string
		rancherToken  string
	}{
		{name: "missing token"},
		{name: "R_token without Authorization", rancherToken: "direct-token"},
		{name: "malformed token", authorization: "Bearer not-a-jwt"},
		{
			name:          "invalid signature",
			authorization: "Bearer " + signOAuthToken(t, otherKey, validOAuthClaims()),
		},
		{
			name: "invalid issuer",
			authorization: "Bearer " + fixture.sign(t, oauthClaims(map[string]any{
				"iss": "https://other-issuer.example.test",
			})),
		},
		{
			name: "expiration beyond ten-second leeway",
			authorization: "Bearer " + fixture.sign(t, oauthClaims(map[string]any{
				"exp": now.Add(-11 * time.Second).Unix(),
			})),
		},
		{
			name: "not before beyond ten-second leeway",
			authorization: "Bearer " + fixture.sign(t, oauthClaims(map[string]any{
				"nbf": now.Add(11 * time.Second).Unix(),
			})),
		},
		{
			name: "missing required scope",
			authorization: "Bearer " + fixture.sign(t, oauthClaims(map[string]any{
				"scope": []string{"offline_access"},
			})),
		},
		{
			name: "malformed scope",
			authorization: "Bearer " + fixture.sign(t, oauthClaims(map[string]any{
				"scope": []any{"offline_access", 42},
			})),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			if tt.authorization != "" {
				request.Header.Set("Authorization", tt.authorization)
			}
			if tt.rancherToken != "" {
				request.Header.Set("R_token", tt.rancherToken)
			}
			response := httptest.NewRecorder()
			mcpServer.Config.Handler.ServeHTTP(response, request)

			if response.Code != http.StatusUnauthorized {
				t.Fatalf("expected OAuth rejection status 401, got %d", response.Code)
			}
			const expectedChallenge = "Bearer resource_metadata=\"https://mcp.example.test/.well-known/oauth-protected-resource\""
			if got := response.Header().Get("WWW-Authenticate"); got != expectedChallenge {
				t.Fatalf("expected exact OAuth challenge %q, got %q", expectedChallenge, got)
			}
			for _, sensitiveValue := range []string{tt.authorization, strings.TrimPrefix(tt.authorization, "Bearer ")} {
				if sensitiveValue == "" {
					continue
				}
				for _, values := range response.Header() {
					for _, value := range values {
						if strings.Contains(value, sensitiveValue) {
							t.Fatal("OAuth rejection header echoed credential material")
						}
					}
				}
				if strings.Contains(response.Body.String(), sensitiveValue) {
					t.Fatal("OAuth rejection body echoed credential material")
				}
			}
			if len(backend.AuthHeaders()) != 0 {
				t.Fatal("rejected OAuth request reached the Rancher backend")
			}
		})
	}
}

// TestOAuthDynamicAuthConcurrentToolCallsDoNotCrossIdentities verifies that
// concurrent verified identities retain their own request-scoped token.
func TestOAuthDynamicAuthConcurrentToolCallsDoNotCrossIdentities(t *testing.T) {
	fixture := newOAuthJWKSFixture(t)
	backend := newFakeRancherBackend(t)
	mcpServer := newOAuthDynamicMCPServer(t, backend.URL(), fixture, []string{"kubernetes"})

	const rounds = 12
	tokenA := fixture.sign(t, oauthClaims(map[string]any{"sub": "oauth-identity-a"}))
	tokenB := fixture.sign(t, oauthClaims(map[string]any{"sub": "oauth-identity-b"}))
	clientA := newStreamableMCPClient(t, mcpServer.URL, tokenA)
	clientB := newStreamableMCPClient(t, mcpServer.URL, tokenB)

	var callers sync.WaitGroup
	callers.Add(2)
	// Keep the calls on distinct MCP clients so their HTTP headers model two
	// independent authenticated identities.
	go func() {
		defer callers.Done()
		for i := 0; i < rounds; i++ {
			result, err := callListNamespacesForCluster(context.Background(), t, clientA, "cluster-a")
			if err != nil || result.IsError {
				t.Error("OAuth identity A could not complete a Kubernetes tool call")
				return
			}
		}
	}()
	go func() {
		defer callers.Done()
		for i := 0; i < rounds; i++ {
			result, err := callListNamespacesForCluster(context.Background(), t, clientB, "cluster-b")
			if err != nil || result.IsError {
				t.Error("OAuth identity B could not complete a Kubernetes tool call")
				return
			}
		}
	}()
	callers.Wait()

	seenA := false
	seenB := false
	for _, request := range backend.Requests() {
		switch {
		case strings.Contains(request.path, "/k8s/clusters/cluster-a/"):
			seenA = true
			if request.authorization != "Bearer "+tokenA {
				t.Fatal("OAuth identity A reached Rancher with another identity's token")
			}
		case strings.Contains(request.path, "/k8s/clusters/cluster-b/"):
			seenB = true
			if request.authorization != "Bearer "+tokenB {
				t.Fatal("OAuth identity B reached Rancher with another identity's token")
			}
		}
	}
	if !seenA || !seenB {
		t.Fatal("both OAuth cluster identities must reach the Rancher backend")
	}
}
