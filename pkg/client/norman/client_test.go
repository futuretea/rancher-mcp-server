package norman

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rancher/norman/types"
)

func startRancherSchemaServer(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			t.Errorf("expected Authorization header on request to %s", r.URL.Path)
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
	server := startRancherSchemaServer(t)

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

func TestNormanClientClose_ClearsCaches(t *testing.T) {
	server := startRancherSchemaServer(t)

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
