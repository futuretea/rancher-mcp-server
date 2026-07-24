package mcp

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/futuretea/rancher-mcp-server/pkg/client/norman"
	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/core/config"
)

const (
	testOAuthIssuer   = "https://issuer.example.test"
	testOAuthAudience = "rancher-mcp"
)

func TestNewServer_OAuthRejectsUnusableJWKS(t *testing.T) {
	rs512Key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() failed: %v", err)
	}

	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{
			name: "non-success response",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "unavailable", http.StatusServiceUnavailable)
			},
		},
		{
			name: "malformed JWKS document",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"keys":[{"kty":"RSA","kid":"missing-modulus"}]}`))
			},
		},
		{
			name: "empty JWKS document",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"keys":[]}`))
			},
		},
		{
			name: "RS512-only JWKS document",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				writeOAuthJWKS(t, w, rs512Key, "RS512")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jwks := httptest.NewServer(tt.handler)
			t.Cleanup(jwks.Close)

			_, err := NewServer(Configuration{StaticConfig: oauthTestConfig(jwks.URL)})
			if err == nil {
				t.Fatal("expected OAuth server construction to fail for an unusable JWKS response")
			}
		})
	}
}

func TestNewServer_OAuthRejectsUnreachableJWKS(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() failed: %v", err)
	}
	endpoint := "http://" + listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	_, err = NewServer(Configuration{StaticConfig: oauthTestConfig(endpoint)})
	if err == nil {
		t.Fatal("expected OAuth server construction to fail when JWKS is unreachable")
	}
}

func TestNewServer_OAuthRejectsStalledJWKSBeforeConstructionDeadline(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var startedOnce sync.Once
	var releaseOnce sync.Once
	releaseHandler := func() { releaseOnce.Do(func() { close(release) }) }

	jwks := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		startedOnce.Do(func() { close(started) })
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(func() {
		releaseHandler()
		jwks.Close()
	})

	withOAuthJWKSConstructionTimeout(t, 25*time.Millisecond)
	result := make(chan error, 1)
	go func() {
		_, err := NewServer(Configuration{StaticConfig: oauthTestConfig(jwks.URL)})
		result <- err
	}()

	select {
	case <-started:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected server construction to request the JWKS")
	}

	select {
	case err := <-result:
		if err == nil {
			t.Fatal("expected OAuth server construction to fail when JWKS stalls")
		}
	case <-time.After(500 * time.Millisecond):
		releaseHandler()
		t.Fatal("expected server construction to stop at the shortened JWKS deadline")
	}
}

func TestNewServer_OAuthRejectsUnreadableJWKSBody(t *testing.T) {
	var bodyClosed atomic.Bool
	var requests atomic.Int64
	withOAuthJWKSHTTPClient(t, &http.Client{
		Transport: oauthJWKSRoundTripper(func(r *http.Request) (*http.Response, error) {
			requests.Add(1)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       &unreadableJWKSBody{closed: &bodyClosed},
				Request:    r,
			}, nil
		}),
	})

	_, err := NewServer(Configuration{StaticConfig: oauthTestConfig("https://jwks.example.test/keys")})
	if err == nil {
		t.Fatal("expected OAuth server construction to fail when the JWKS body is unreadable")
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("expected one JWKS request, got %d", got)
	}
	if !bodyClosed.Load() {
		t.Fatal("expected the unreadable JWKS response body to be closed")
	}
}

func TestNewServer_OAuthLoadsJWKSExactlyOnceAtConstruction(t *testing.T) {
	fixture := newOAuthJWKSFixture(t)

	server, err := NewServer(Configuration{StaticConfig: oauthTestConfig(fixture.URL())})
	if err != nil {
		t.Fatalf("NewServer() failed with a usable JWKS response: %v", err)
	}
	if server == nil {
		t.Fatal("NewServer() returned a nil server")
	}
	if got := fixture.Requests(); got != 1 {
		t.Fatalf("expected exactly one JWKS request during server construction, got %d", got)
	}
}

func TestNewServer_OAuthJWKSUsesConfiguredInsecureTLS(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() failed: %v", err)
	}

	jwks := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeOAuthJWKS(t, w, privateKey, oauthSigningMethod)
	}))
	t.Cleanup(jwks.Close)

	staticConfig := oauthTestConfig(jwks.URL)
	if server, err := NewServer(Configuration{StaticConfig: staticConfig}); err == nil {
		server.Close()
		t.Fatal("expected self-signed JWKS to fail without rancher_tls_insecure")
	}

	staticConfig.RancherTLSInsecure = true
	server, err := NewServer(Configuration{StaticConfig: staticConfig})
	if err != nil {
		t.Fatalf("NewServer() failed with rancher_tls_insecure: %v", err)
	}
	t.Cleanup(server.Close)
}

func TestOAuthTokenVerifier_ValidRS256TokenUsesOneStartupJWKSFetch(t *testing.T) {
	fixture := newOAuthJWKSFixture(t)
	verifier, err := newOAuthTokenVerifier(oauthTestConfig(fixture.URL()))
	if err != nil {
		t.Fatalf("newOAuthTokenVerifier() failed: %v", err)
	}

	token := fixture.sign(t, validOAuthClaims())
	for range 2 {
		verified, err := verifier.verifyAuthorization("Bearer " + token)
		if err != nil {
			t.Fatalf("verifyAuthorization() rejected a valid RS256 token: %v", err)
		}
		if verified != token {
			t.Fatal("verifyAuthorization() did not preserve the verified token")
		}
	}

	if got := fixture.Requests(); got != 1 {
		t.Fatalf("expected exactly one JWKS request at construction, got %d", got)
	}
}

func TestOAuthTokenVerifier_AcceptsReferenceCompatibleClaims(t *testing.T) {
	fixture := newOAuthJWKSFixture(t)
	verifier, err := newOAuthTokenVerifier(oauthTestConfig(fixture.URL()))
	if err != nil {
		t.Fatalf("newOAuthTokenVerifier() failed: %v", err)
	}

	now := time.Now()
	claimsWithoutAudience := validOAuthClaims()
	delete(claimsWithoutAudience, "aud")
	claimsWithoutExpiration := validOAuthClaims()
	delete(claimsWithoutExpiration, "exp")
	tests := []struct {
		name   string
		claims map[string]any
	}{
		{
			name:   "missing audience",
			claims: claimsWithoutAudience,
		},
		{
			name: "arbitrary audience",
			claims: oauthClaims(map[string]any{
				"aud": "other-audience",
			}),
		},
		{
			name:   "missing expiration",
			claims: claimsWithoutExpiration,
		},
		{
			name: "expiration within ten-second leeway",
			claims: oauthClaims(map[string]any{
				"exp": now.Add(-9 * time.Second).Unix(),
			}),
		},
		{
			name: "not before within ten-second leeway",
			claims: oauthClaims(map[string]any{
				"nbf": now.Add(9 * time.Second).Unix(),
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := verifier.verifyAuthorization("Bearer " + fixture.sign(t, tt.claims)); err != nil {
				t.Fatalf("verifyAuthorization() rejected a reference-compatible token: %v", err)
			}
		})
	}
}

func TestOAuthTokenVerifier_RejectsInvalidAuthorization(t *testing.T) {
	fixture := newOAuthJWKSFixture(t)
	verifier, err := newOAuthTokenVerifier(oauthTestConfig(fixture.URL()))
	if err != nil {
		t.Fatalf("newOAuthTokenVerifier() failed: %v", err)
	}

	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() failed: %v", err)
	}

	now := time.Now()
	tests := []struct {
		name          string
		authorization string
	}{
		{
			name:          "missing Authorization header",
			authorization: "",
		},
		{
			name:          "malformed JWT",
			authorization: "Bearer not-a-jwt",
		},
		{
			name:          "invalid signature",
			authorization: "Bearer " + signOAuthToken(t, otherKey, validOAuthClaims()),
		},
		{
			name:          "non RS256 algorithm",
			authorization: "Bearer " + signOAuthTokenWithAlgorithm(t, fixture.privateKey, validOAuthClaims(), "RS512"),
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
			name: "scope array contains a non-string member",
			authorization: "Bearer " + fixture.sign(t, oauthClaims(map[string]any{
				"scope": []any{"offline_access", 42},
			})),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := verifier.verifyAuthorization(tt.authorization); err == nil {
				t.Fatal("expected authorization to be rejected")
			}
		})
	}
}

func TestOAuthTokenResolver_UsesOnlyTypedVerifiedToken(t *testing.T) {
	const verifiedToken = "verified-token"
	var receivedTokens []string

	resolver := &oauthTokenResolver{
		serverURL: "https://rancher.example.test",
		steveFactory: func(_ string, token string, _ bool) *steve.Client {
			receivedTokens = append(receivedTokens, token)
			return &steve.Client{}
		},
		normanFactory: func(_ string, token string, _ bool) (*norman.Client, error) {
			receivedTokens = append(receivedTokens, token)
			return &norman.Client{}, nil
		},
	}

	ctx := context.WithValue(context.Background(), authorizationKey, "Bearer raw-authorization-token")
	client, err := resolver.Resolve(oauthTokenContext(ctx, verifiedToken))
	if err != nil {
		t.Fatalf("Resolve() failed with a verified token: %v", err)
	}
	if client == nil {
		t.Fatal("Resolve() returned a nil client")
	}
	if len(receivedTokens) != 2 {
		t.Fatalf("expected both Rancher factories to receive a token, got %d calls", len(receivedTokens))
	}
	for _, got := range receivedTokens {
		if got != verifiedToken {
			t.Fatal("OAuth resolver used a raw Authorization value instead of the typed verified token")
		}
	}

	receivedTokens = nil
	rawOnly := context.WithValue(context.Background(), authorizationKey, "Bearer raw-authorization-token")
	if _, err := resolver.Resolve(rawOnly); err == nil {
		t.Fatal("expected resolver to reject a context without a verified token")
	}
	if len(receivedTokens) != 0 {
		t.Fatal("OAuth resolver fell back to the raw Authorization header")
	}
}

func oauthTestConfig(jwksURL string) *config.StaticConfig {
	return &config.StaticConfig{
		RancherServerURL:                   "https://rancher.example.test",
		RancherOAuthTokenAuth:              true,
		RancherOAuthAuthorizationServerURL: testOAuthIssuer,
		RancherOAuthJWKSURL:                jwksURL,
		RancherOAuthResourceURL:            "https://mcp.example.test",
		ListOutput:                         "json",
	}
}

type oauthJWKSFixture struct {
	privateKey *rsa.PrivateKey
	server     *httptest.Server
	requests   atomic.Int64
}

func newOAuthJWKSFixture(t *testing.T) *oauthJWKSFixture {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() failed: %v", err)
	}

	fixture := &oauthJWKSFixture{privateKey: privateKey}
	fixture.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fixture.requests.Add(1)
		writeOAuthJWKS(t, w, privateKey, oauthSigningMethod)
	}))
	t.Cleanup(fixture.server.Close)
	return fixture
}

func writeOAuthJWKS(t testing.TB, w http.ResponseWriter, privateKey *rsa.PrivateKey, algorithm string) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"keys": []map[string]string{
			{
				"kty": "RSA",
				"kid": "test-rsa-key",
				"alg": algorithm,
				"use": "sig",
				"n":   base64.RawURLEncoding.EncodeToString(privateKey.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.PublicKey.E)).Bytes()),
			},
		},
	}); err != nil {
		t.Errorf("Encode() failed: %v", err)
	}
}

func (f *oauthJWKSFixture) URL() string {
	return f.server.URL
}

func (f *oauthJWKSFixture) Requests() int64 {
	return f.requests.Load()
}

func (f *oauthJWKSFixture) sign(t *testing.T, claims map[string]any) string {
	t.Helper()
	return signOAuthToken(t, f.privateKey, claims)
}

func validOAuthClaims() map[string]any {
	now := time.Now()
	return map[string]any{
		"iss":   testOAuthIssuer,
		"aud":   testOAuthAudience,
		"exp":   now.Add(time.Hour).Unix(),
		"nbf":   now.Add(-time.Minute).Unix(),
		"iat":   now.Add(-time.Minute).Unix(),
		"scope": []string{"offline_access", "rancher:mcp"},
	}
}

func oauthClaims(overrides map[string]any) map[string]any {
	claims := validOAuthClaims()
	for name, value := range overrides {
		claims[name] = value
	}
	return claims
}

func signOAuthToken(t *testing.T, privateKey *rsa.PrivateKey, claims map[string]any) string {
	return signOAuthTokenWithAlgorithm(t, privateKey, claims, "RS256")
}

func signOAuthTokenWithAlgorithm(t *testing.T, privateKey *rsa.PrivateKey, claims map[string]any, algorithm string) string {
	t.Helper()

	header, err := json.Marshal(map[string]string{
		"alg": algorithm,
		"kid": "test-rsa-key",
		"typ": "JWT",
	})
	if err != nil {
		t.Fatalf("Marshal() header failed: %v", err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("Marshal() claims failed: %v", err)
	}

	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	hash, digest := jwtSigningDigest(t, algorithm, []byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, hash, digest)
	if err != nil {
		t.Fatalf("SignPKCS1v15() failed: %v", err)
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func jwtSigningDigest(t *testing.T, algorithm string, signingInput []byte) (crypto.Hash, []byte) {
	t.Helper()
	switch algorithm {
	case "RS256":
		digest := sha256.Sum256(signingInput)
		return crypto.SHA256, digest[:]
	case "RS512":
		digest := sha512.Sum512(signingInput)
		return crypto.SHA512, digest[:]
	default:
		t.Fatalf("unsupported test JWT algorithm %q", algorithm)
		return 0, nil
	}
}

func withOAuthJWKSConstructionTimeout(t *testing.T, timeout time.Duration) {
	t.Helper()
	previous := oauthJWKSConstructionTimeout
	oauthJWKSConstructionTimeout = timeout
	t.Cleanup(func() { oauthJWKSConstructionTimeout = previous })
}

func withOAuthJWKSHTTPClient(t *testing.T, client *http.Client) {
	t.Helper()
	previous := oauthJWKSHTTPClient
	oauthJWKSHTTPClient = client
	t.Cleanup(func() { oauthJWKSHTTPClient = previous })
}

type oauthJWKSRoundTripper func(*http.Request) (*http.Response, error)

func (fn oauthJWKSRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

type unreadableJWKSBody struct {
	closed *atomic.Bool
}

func (b *unreadableJWKSBody) Read(_ []byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

func (b *unreadableJWKSBody) Close() error {
	b.closed.Store(true)
	return nil
}
