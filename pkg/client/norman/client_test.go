package norman

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
