package toolset

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/futuretea/rancher-mcp-server/pkg/client/norman"
	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/rancher/norman/types"
)

func TestNewCombinedClient(t *testing.T) {
	s := &steve.Client{}
	n := &norman.Client{}

	cc := NewCombinedClient(n, s, true)
	if cc.Norman != n {
		t.Error("expected Norman client to be set")
	}
	if cc.Steve != s {
		t.Error("expected Steve client to be set")
	}
}

func TestCombinedClientClose_StaticNoop(t *testing.T) {
	s := &steve.Client{}
	n := &norman.Client{}

	cc := NewCombinedClient(n, s, false)
	// Close on a static client must not panic or affect the underlying clients.
	cc.Close()

	if cc.Norman != n || cc.Steve != s {
		t.Error("expected static Close to be a no-op")
	}
}

func startRancherSchemaServer(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestCombinedClientClose_ReleasesCaches(t *testing.T) {
	server := startRancherSchemaServer(t)

	s := steve.NewClientWithToken(server.URL, "token", false)
	n, err := norman.NewClientWithToken(server.URL, "token", false)
	if err != nil {
		t.Fatalf("failed to create Norman client: %v", err)
	}

	cc := NewCombinedClient(n, s, true)
	cc.Close()

	if n.IsUsable() {
		t.Error("expected Norman client to be closed")
	}
}

func TestCombinedClientClose_StableGoroutinesAndConnections(t *testing.T) {
	server := startRancherSchemaServer(t)

	beforeGoroutines := runtime.NumGoroutine()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	const iterations = 100
	for i := 0; i < iterations; i++ {
		s := steve.NewClientWithToken(server.URL, "token", false)
		n, err := norman.NewClientWithToken(server.URL, "token", false)
		if err != nil {
			t.Fatalf("failed to create Norman client: %v", err)
		}
		cc := NewCombinedClient(n, s, true)
		cc.Close()
	}

	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	afterGoroutines := runtime.NumGoroutine()
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	// Allow a small noise window for background goroutines.
	if afterGoroutines > beforeGoroutines+5 {
		t.Fatalf("goroutine leak detected: before=%d after=%d", beforeGoroutines, afterGoroutines)
	}

	t.Logf("goroutines before=%d after=%d heap_alloc_before=%d heap_alloc_after=%d",
		beforeGoroutines, afterGoroutines, memBefore.Alloc, memAfter.Alloc)
}

func TestValidateSteveClient(t *testing.T) {
	t.Run("CombinedClient with Steve", func(t *testing.T) {
		s := &steve.Client{}
		cc := &CombinedClient{Steve: s}
		got, err := ValidateSteveClient(cc)
		if err != nil || got != s {
			t.Fatal("expected CombinedClient.Steve to be returned")
		}
	})

	t.Run("CombinedClient with nil Steve", func(t *testing.T) {
		cc := &CombinedClient{}
		_, err := ValidateSteveClient(cc)
		if err == nil {
			t.Fatal("expected error for nil Steve in CombinedClient")
		}
	})

	t.Run("direct Steve client", func(t *testing.T) {
		s := &steve.Client{}
		got, err := ValidateSteveClient(s)
		if err != nil || got != s {
			t.Fatal("expected direct Steve client to be returned")
		}
	})

	t.Run("nil client", func(t *testing.T) {
		_, err := ValidateSteveClient(nil)
		if err == nil {
			t.Fatal("expected error for nil client")
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		_, err := ValidateSteveClient("not-a-client")
		if err == nil {
			t.Fatal("expected error for wrong type")
		}
	})
}

func TestValidateNormanClient(t *testing.T) {
	t.Run("CombinedClient with unusable Norman", func(t *testing.T) {
		cc := &CombinedClient{Norman: &norman.Client{}}
		_, err := ValidateNormanClient(cc)
		if err == nil {
			t.Fatal("expected error for unusable Norman client")
		}
	})

	t.Run("direct unusable Norman client", func(t *testing.T) {
		_, err := ValidateNormanClient(&norman.Client{})
		if err == nil {
			t.Fatal("expected error for unusable Norman client")
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		_, err := ValidateNormanClient(42)
		if err == nil {
			t.Fatal("expected error for wrong type")
		}
	})
}
