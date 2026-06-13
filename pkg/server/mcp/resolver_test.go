package mcp

import (
	"context"
	"errors"
	"expvar"
	"strings"
	"sync"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/client/norman"
	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
)

func TestStaticResolver_ReturnsPrebuiltClient(t *testing.T) {
	client := toolset.NewCombinedClient(nil, nil, false)
	r := &staticResolver{client: client}

	resolved, err := r.Resolve(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != client {
		t.Fatal("expected static resolver to return pre-built client")
	}
}

func TestRequestTokenResolver_MissingHeader(t *testing.T) {
	r := &requestTokenResolver{
		serverURL:     "https://rancher.example.com",
		steveFactory:  steve.NewClientWithToken,
		normanFactory: norman.NewClientWithToken,
	}

	_, err := r.Resolve(context.Background())
	if err == nil {
		t.Fatal("expected error for missing Authorization header")
	}
	if !strings.Contains(err.Error(), "missing Authorization header") {
		t.Fatalf("expected clear missing header error, got: %v", err)
	}
}

func TestRequestTokenResolver_MalformedHeader(t *testing.T) {
	r := &requestTokenResolver{
		serverURL:     "https://rancher.example.com",
		steveFactory:  steve.NewClientWithToken,
		normanFactory: norman.NewClientWithToken,
	}

	cases := []struct {
		name           string
		header         string
		expectedErrMsg string
	}{
		{"no space", "Bearer", "malformed Authorization header"},
		{"wrong scheme", "Basic token", "malformed Authorization header"},
		{"empty token", "Bearer ", "malformed Authorization header"},
		{"extra parts", "Bearer token extra", "malformed Authorization header"},
		{"non-string value", "", "missing Authorization header"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ctx context.Context
			if tc.header == "non-string value" {
				ctx = context.WithValue(context.Background(), authorizationKey, 12345)
			} else {
				ctx = context.WithValue(context.Background(), authorizationKey, tc.header)
			}
			_, err := r.Resolve(ctx)
			if err == nil {
				t.Fatal("expected error for malformed header")
			}
			if !strings.Contains(err.Error(), tc.expectedErrMsg) {
				t.Fatalf("expected error to contain %q, got: %v", tc.expectedErrMsg, err)
			}
		})
	}
}

func TestBearerTokenFromContext_CaseInsensitiveScheme(t *testing.T) {
	ctx := context.WithValue(context.Background(), authorizationKey, "bearer lowercase-token")
	token, err := bearerTokenFromContext(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "lowercase-token" {
		t.Fatalf("expected lowercase-token, got %q", token)
	}
}

func TestBearerTokenFromContext_TabSeparatorAccepted(t *testing.T) {
	ctx := context.WithValue(context.Background(), authorizationKey, "Bearer\ttab-token")
	token, err := bearerTokenFromContext(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "tab-token" {
		t.Fatalf("expected tab-token, got %q", token)
	}
}

func TestRequestTokenResolver_MissingHeaderIncrementsErrorMetric(t *testing.T) {
	metrics := NewExpvarMetrics()
	r := &requestTokenResolver{
		serverURL:     "https://rancher.example.com",
		steveFactory:  steve.NewClientWithToken,
		normanFactory: norman.NewClientWithToken,
		metrics:       metrics,
	}

	before := expvar.Get("rancher_request_errors").String()
	_, err := r.Resolve(context.Background())
	if err == nil {
		t.Fatal("expected error for missing Authorization header")
	}
	after := expvar.Get("rancher_request_errors").String()
	if before == after {
		t.Fatalf("expected rancher_request_errors to increment, got %s", after)
	}
}

func TestRequestTokenResolver_NormanFailureFallsBackToSteve(t *testing.T) {
	var steveToken, normanToken string
	metrics := NewExpvarMetrics()
	metrics.(*expvarMetrics).rancherRequestErrors.Set(0)

	r := &requestTokenResolver{
		serverURL: "https://rancher.example.com",
		steveFactory: func(_, token string, _ bool) *steve.Client {
			steveToken = token
			return &steve.Client{}
		},
		normanFactory: func(_, token string, _ bool) (*norman.Client, error) {
			normanToken = token
			return nil, errors.New("failed to create management client")
		},
		metrics: metrics,
	}

	ctx := context.WithValue(context.Background(), authorizationKey, "Bearer secret-token")
	client, err := r.Resolve(ctx)
	if err != nil {
		t.Fatalf("expected resolver to succeed when only Norman fails: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil CombinedClient")
	}
	if client.Steve == nil {
		t.Fatal("expected Steve client to be set")
	}
	if client.Norman != nil {
		t.Fatal("expected Norman client to be nil after creation failure")
	}
	if steveToken != "secret-token" || normanToken != "secret-token" {
		t.Fatalf("expected token to be passed to both factories; steve=%q norman=%q", steveToken, normanToken)
	}
	if errors := expvar.Get("rancher_request_errors").String(); errors != "1" {
		t.Fatalf("expected rancher_request_errors to be 1 after Norman failure, got %s", errors)
	}
}

func TestRequestTokenResolver_DoesNotLeakToken(t *testing.T) {
	// bearerTokenFromContext rejects non-string context values before the token
	// is parsed, so the error must not include the raw value.
	ctx := context.WithValue(context.Background(), authorizationKey, []byte("secret-token"))
	_, err := bearerTokenFromContext(ctx)
	if err == nil {
		t.Fatal("expected error for non-string Authorization value")
	}
	if strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("error message leaked token: %v", err)
	}
}

func TestRequestTokenResolver_ConcurrentTokens_NoLeakage(t *testing.T) {
	var mu sync.Mutex
	seenTokens := make(map[string]int)

	r := &requestTokenResolver{
		serverURL: "https://rancher.example.com",
		steveFactory: func(_, token string, _ bool) *steve.Client {
			mu.Lock()
			seenTokens[token]++
			mu.Unlock()
			return &steve.Client{}
		},
		normanFactory: func(_, token string, _ bool) (*norman.Client, error) {
			mu.Lock()
			seenTokens[token]++
			mu.Unlock()
			return &norman.Client{}, nil
		},
	}

	const iterations = 50
	var wg sync.WaitGroup
	wg.Add(iterations * 2)

	resolve := func(token string) {
		defer wg.Done()
		ctx := context.WithValue(context.Background(), authorizationKey, "Bearer "+token)
		client, err := r.Resolve(ctx)
		if err != nil {
			t.Errorf("unexpected error for token %s: %v", token, err)
			return
		}
		if client == nil {
			t.Errorf("expected non-nil client for token %s", token)
		}
	}

	for i := 0; i < iterations; i++ {
		go resolve("token-a")
		go resolve("token-b")
	}
	wg.Wait()

	if seenTokens["token-a"] != iterations*2 {
		t.Fatalf("expected token-a seen %d times, got %d", iterations*2, seenTokens["token-a"])
	}
	if seenTokens["token-b"] != iterations*2 {
		t.Fatalf("expected token-b seen %d times, got %d", iterations*2, seenTokens["token-b"])
	}
}
