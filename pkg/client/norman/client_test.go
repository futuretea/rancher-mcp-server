package norman

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rancher/norman/types"

	"github.com/futuretea/rancher-mcp-server/pkg/core/config"
)

func startRancherSchemaServer(t *testing.T, expectedAuthorization string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != expectedAuthorization {
			t.Errorf("expected Authorization header %q on request to %s, got %q", expectedAuthorization, r.URL.Path, auth)
		}

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

		w.Header().Set("X-API-Schemas", "http://"+r.Host+"/v3/schemas")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(schemas)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestNewClientWithToken_BindsToken(t *testing.T) {
	server := startRancherSchemaServer(t, "Bearer request-token")

	client, err := NewClientWithToken(server.URL, "request-token", true)
	if err != nil {
		t.Fatalf("NewClientWithToken() returned unexpected error: %v", err)
	}

	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if !client.IsUsable() {
		t.Fatal("expected client to be usable")
	}
}

func TestNewClient_BindsAccessKeyCredentials(t *testing.T) {
	const (
		accessKey = "access-key"
		secretKey = "secret-key"
	)
	expectedAuthorization := "Basic " + base64.StdEncoding.EncodeToString([]byte(accessKey+":"+secretKey))
	server := startRancherSchemaServer(t, expectedAuthorization)

	client, err := NewClient(&config.StaticConfig{
		RancherServerURL:   server.URL,
		RancherAccessKey:   accessKey,
		RancherSecretKey:   secretKey,
		RancherTLSInsecure: true,
	})
	if err != nil {
		t.Fatalf("NewClient() returned unexpected error: %v", err)
	}
	t.Cleanup(client.Close)
	if !client.IsUsable() {
		t.Fatal("expected client to be usable")
	}
}

func TestNormanClientClose_ClearsCaches(t *testing.T) {
	server := startRancherSchemaServer(t, "Bearer request-token")

	client, err := NewClientWithToken(server.URL, "request-token", true)
	if err != nil {
		t.Fatalf("NewClientWithToken() returned unexpected error: %v", err)
	}

	if !client.IsUsable() {
		t.Fatal("expected client to be usable before Close")
	}

	client.Close()

	if client.IsUsable() {
		t.Fatal("expected Close to clear management client cache")
	}
}

func TestRedactURLCredentials(t *testing.T) {
	cases := []struct {
		name   string
		rawURL string
		err    error
		want   string
	}{
		{
			name:   "userinfo credentials are masked",
			rawURL: "http://user:sup3rsecret@127.0.0.1:18099/v3",
			err:    errors.New(`failed to create management client: Bad response statusCode [401]. Body: [] from [http://user:sup3rsecret@127.0.0.1:18099/v3]`),
			want:   `failed to create management client: Bad response statusCode [401]. Body: [] from [http://***@127.0.0.1:18099/v3]`,
		},
		{
			name:   "username only userinfo is masked",
			rawURL: "https://token-value@rancher.example.com/v3",
			err:    errors.New(`Get "https://token-value@rancher.example.com/v3": connection refused`),
			want:   `Get "https://***@rancher.example.com/v3": connection refused`,
		},
		{
			name:   "unescaped at sign in the password is masked",
			rawURL: "http://user:p@ssw0rd@127.0.0.1:18099/v3",
			err:    errors.New(`from [http://user:p@ssw0rd@127.0.0.1:18099/v3]`),
			want:   `from [http://***@127.0.0.1:18099/v3]`,
		},
		{
			name:   "unparseable url with credentials is masked",
			rawURL: "http://user:pa/ss@127.0.0.1:18099/v3",
			err:    errors.New(`parse "http://user:pa/ss@127.0.0.1:18099/v3": invalid port ":pa" after host`),
			want:   `parse "http://***@127.0.0.1:18099/v3": invalid port ":pa" after host`,
		},
		{
			name:   "re-rendered url is masked by the fallback pattern",
			rawURL: "http://user:p@ss@127.0.0.1:18099/v3",
			err:    errors.New(`Get "http://user:p%40ss@127.0.0.1:18099/v3": connection refused`),
			want:   `Get "http://***@127.0.0.1:18099/v3": connection refused`,
		},
		{
			name:   "plain url is unchanged",
			rawURL: "https://rancher.example.com/v3",
			err:    errors.New(`Get "https://rancher.example.com/v3": connection refused`),
			want:   `Get "https://rancher.example.com/v3": connection refused`,
		},
		{
			name:   "at sign inside a path is unchanged",
			rawURL: "https://rancher.example.com/path@name",
			err:    errors.New(`Get "https://rancher.example.com/path@name": connection refused`),
			want:   `Get "https://rancher.example.com/path@name": connection refused`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := redactURLCredentials(tc.err, tc.rawURL).Error(); got != tc.want {
				t.Fatalf("redactURLCredentials() = %q, want %q", got, tc.want)
			}
		})
	}

	if redactURLCredentials(nil, "") != nil {
		t.Fatal("expected a nil error to stay nil")
	}
}

func TestNewClientWithToken_RedactsCredentialsInErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no schema here", http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	serverURL := strings.Replace(server.URL, "http://", "http://user:s3cretpw@", 1)
	_, err := NewClientWithToken(serverURL, "request-token", true)
	if err == nil {
		t.Fatal("expected NewClientWithToken() to fail against a non-schema endpoint")
	}
	if strings.Contains(err.Error(), "s3cretpw") {
		t.Fatalf("expected credentials to be redacted, got %v", err)
	}
	if !strings.Contains(err.Error(), "***@") {
		t.Fatalf("expected the redacted URL in the error, got %v", err)
	}
}
