package steve

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

func TestGetDynamicClient_ReusesClientPerCluster(t *testing.T) {
	client := NewClient("https://example.com", "", "", "", false)

	first, err := client.getDynamicClient("cluster-a")
	if err != nil {
		t.Fatalf("getDynamicClient() returned unexpected error: %v", err)
	}
	second, err := client.getDynamicClient("cluster-a")
	if err != nil {
		t.Fatalf("second getDynamicClient() returned unexpected error: %v", err)
	}

	if interfacePointer(first) != interfacePointer(second) {
		t.Fatal("expected dynamic client to be reused for the same cluster")
	}
}

func TestGetDynamicClient_SeparatesClusters(t *testing.T) {
	client := NewClient("https://example.com", "token", "", "", false)

	first, err := client.getDynamicClient("cluster-a")
	if err != nil {
		t.Fatalf("getDynamicClient() returned unexpected error: %v", err)
	}
	second, err := client.getDynamicClient("cluster-b")
	if err != nil {
		t.Fatalf("getDynamicClient() for second cluster returned unexpected error: %v", err)
	}

	if interfacePointer(first) == interfacePointer(second) {
		t.Fatal("expected different clusters to receive different dynamic clients")
	}
}

func TestGetDynamicClient_InitializesZeroValueCaches(t *testing.T) {
	client := &Client{
		serverURL: "https://example.com",
		token:     "token",
	}

	first, err := client.getDynamicClient("cluster-a")
	if err != nil {
		t.Fatalf("getDynamicClient() returned unexpected error: %v", err)
	}
	second, err := client.getDynamicClient("cluster-a")
	if err != nil {
		t.Fatalf("second getDynamicClient() returned unexpected error: %v", err)
	}

	if interfacePointer(first) != interfacePointer(second) {
		t.Fatal("expected zero-value client caches to initialize lazily and reuse the client")
	}
}

func TestGetClientset_ReusesClientsetPerCluster(t *testing.T) {
	client := NewClient("https://example.com", "token", "", "", false)

	first, err := client.getClientset("cluster-a")
	if err != nil {
		t.Fatalf("getClientset() returned unexpected error: %v", err)
	}
	second, err := client.getClientset("cluster-a")
	if err != nil {
		t.Fatalf("second getClientset() returned unexpected error: %v", err)
	}

	if interfacePointer(first) != interfacePointer(second) {
		t.Fatal("expected clientset to be reused for the same cluster")
	}
}

func TestGetClientset_ReusesClientsetAcrossConcurrentCalls(t *testing.T) {
	client := NewClient("https://example.com", "token", "", "", false)

	const workers = 8
	results := make([]uintptr, workers)
	errs := make([]error, workers)

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(index int) {
			defer wg.Done()
			clientset, err := client.getClientset("cluster-a")
			if err != nil {
				errs[index] = err
				return
			}
			results[index] = interfacePointer(clientset)
		}(i)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			t.Fatalf("concurrent getClientset() returned unexpected error: %v", err)
		}
	}
	for i := 1; i < len(results); i++ {
		if results[i] != results[0] {
			t.Fatal("expected concurrent calls to reuse the same clientset instance")
		}
	}
}

func interfacePointer(value interface{}) uintptr {
	return reflect.ValueOf(value).Pointer()
}

func TestNewClientWithToken_BindsToken(t *testing.T) {
	client := NewClientWithToken("https://example.com", "request-token", true)

	if client.serverURL != "https://example.com" {
		t.Errorf("expected server URL https://example.com, got %q", client.serverURL)
	}
	if client.token != "request-token" {
		t.Errorf("expected token request-token, got %q", client.token)
	}
	if client.accessKey != "" || client.secretKey != "" {
		t.Error("expected accessKey and secretKey to be empty")
	}
	if !client.insecure {
		t.Error("expected insecure to be true")
	}
}

func TestCreateRestConfigUsesClientGoTransportDefaults(t *testing.T) {
	client := NewClient("https://example.com", "", "", "", false)

	restConfig, err := client.createRestConfig("cluster-a")
	if err != nil {
		t.Fatalf("createRestConfig() returned unexpected error: %v", err)
	}

	if restConfig.Transport != nil {
		t.Fatalf("expected rest.Config Transport to remain nil, got %T", restConfig.Transport)
	}

	transportValue, err := rest.TransportFor(restConfig)
	if err != nil {
		t.Fatalf("rest.TransportFor() returned unexpected error: %v", err)
	}
	transport, ok := transportValue.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", transportValue)
	}
	if transport.Proxy == nil {
		t.Fatal("expected transport to configure a proxy function")
	}
	if transport.DialContext == nil {
		t.Fatal("expected transport to configure a dialer")
	}
	if transport.TLSHandshakeTimeout != 10*time.Second {
		t.Fatalf("expected a 10 second TLS handshake timeout, got %s", transport.TLSHandshakeTimeout)
	}
	if transport.IdleConnTimeout == 0 {
		t.Fatal("expected transport to configure an idle connection timeout")
	}
}

func TestCreateRestConfigUsesSharedTLSCacheAcrossClusters(t *testing.T) {
	client := NewClient("https://example.com", "", "", "", true)

	first, err := client.createRestConfig("cluster-a")
	if err != nil {
		t.Fatalf("first createRestConfig() returned unexpected error: %v", err)
	}
	second, err := client.createRestConfig("cluster-b")
	if err != nil {
		t.Fatalf("createRestConfig() for second cluster returned unexpected error: %v", err)
	}

	firstTransport, err := rest.TransportFor(first)
	if err != nil {
		t.Fatalf("rest.TransportFor(first) returned unexpected error: %v", err)
	}
	secondTransport, err := rest.TransportFor(second)
	if err != nil {
		t.Fatalf("rest.TransportFor(second) returned unexpected error: %v", err)
	}
	if firstTransport != secondTransport {
		t.Fatal("expected client-go TLS cache to share a transport across clusters")
	}
}

func TestCreateRestConfigReusesTransportAcrossConcurrentCalls(t *testing.T) {
	client := NewClient("https://example.com", "", "", "", true)

	const workers = 8
	configs := make([]*rest.Config, workers)
	errs := make([]error, workers)
	start := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(index int) {
			defer wg.Done()
			<-start
			configs[index], errs[index] = client.createRestConfig("cluster-a")
		}(i)
	}
	close(start)
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			t.Fatalf("concurrent createRestConfig() returned unexpected error: %v", err)
		}
	}
	transports := make([]http.RoundTripper, len(configs))
	for i := 1; i < len(configs); i++ {
		transport, err := rest.TransportFor(configs[i])
		if err != nil {
			t.Fatalf("rest.TransportFor() returned unexpected error: %v", err)
		}
		transports[i] = transport
	}
	transport, err := rest.TransportFor(configs[0])
	if err != nil {
		t.Fatalf("rest.TransportFor() returned unexpected error: %v", err)
	}
	transports[0] = transport
	for i := 1; i < len(transports); i++ {
		if transports[i] != transports[0] {
			t.Fatal("expected concurrent REST configs to share client-go's TLS cache")
		}
	}
}

func TestRancherTLSSettingAppliesToRESTAndFileTransport(t *testing.T) {
	for _, test := range []struct {
		name           string
		insecure       bool
		wantServerCall bool
	}{
		{name: "verification enabled", insecure: false, wantServerCall: false},
		{name: "verification disabled", insecure: true, wantServerCall: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var restCalls, websocketCalls, spdyCalls atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/pods"):
					restCalls.Add(1)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"PodList","items":[]}`))
				case strings.HasSuffix(r.URL.Path, "/exec") && r.Method == http.MethodGet:
					websocketCalls.Add(1)
					http.Error(w, "fixture endpoint does not implement WebSocket upgrades", http.StatusInternalServerError)
				case strings.HasSuffix(r.URL.Path, "/exec") && r.Method == http.MethodPost:
					spdyCalls.Add(1)
					http.Error(w, "fixture endpoint does not implement SPDY upgrades", http.StatusInternalServerError)
				default:
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(server.Close)

			client := NewClient(server.URL, "", "", "", test.insecure)
			dynamicClient, err := client.getDynamicClient("cluster-a")
			if err != nil {
				t.Fatalf("getDynamicClient() error = %v", err)
			}
			_, err = dynamicClient.Resource(schema.GroupVersionResource{Version: "v1", Resource: "pods"}).Namespace("default").List(context.Background(), metav1.ListOptions{})
			if test.insecure && err != nil {
				t.Fatalf("REST request error = %v, want successful self-signed request", err)
			}
			if !test.insecure && (err == nil || !strings.Contains(err.Error(), "certificate")) {
				t.Fatalf("REST request error = %v, want certificate verification failure", err)
			}

			_, _, err = client.CheckFileInfo(context.Background(), "cluster-a", "default", "fixture-pod", "fixture-container", "/tmp/fixture")
			if err == nil {
				t.Fatal("CheckFileInfo() error = nil, want remote command failure")
			}
			if got := restCalls.Load() > 0; got != test.wantServerCall {
				t.Fatalf("REST server called = %t, want %t", got, test.wantServerCall)
			}
			if got := websocketCalls.Load() > 0; got != test.wantServerCall {
				t.Fatalf("WebSocket server called = %t, want %t (error: %v)", got, test.wantServerCall, err)
			}
			if got := spdyCalls.Load() > 0; got != test.wantServerCall {
				t.Fatalf("SPDY server called = %t, want %t (error: %v)", got, test.wantServerCall, err)
			}
			if !test.insecure && !strings.Contains(err.Error(), "certificate") {
				t.Fatalf("CheckFileInfo() error = %v, want certificate verification failure", err)
			}
		})
	}
}

func TestCheckFileInfo_InvalidClusterReferencePreservesTypedError(t *testing.T) {
	var remoteCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		remoteCalls.Add(1)
	}))
	t.Cleanup(server.Close)

	client, err := NewClientWithKubeconfigPaths("", "", "", "", false, []string{writeKubeconfig(t, "direct", server.URL, "")})
	if err != nil {
		t.Fatalf("NewClientWithKubeconfigPaths() error = %v", err)
	}

	_, _, err = client.CheckFileInfo(context.Background(), "kubeconfig:../escape", "default", "fixture-pod", "fixture-container", "/tmp/fixture")
	var referenceErr *ClusterReferenceError
	if !errors.As(err, &referenceErr) {
		t.Fatalf("CheckFileInfo() error = %v, want ClusterReferenceError", err)
	}
	if remoteCalls.Load() != 0 {
		t.Fatalf("remote calls = %d, want 0 before invalid reference is rejected", remoteCalls.Load())
	}
}

func TestSteveClientClose_ClearsCaches(t *testing.T) {
	client := NewClient("https://example.com", "token", "", "", false)

	first, err := client.getDynamicClient("cluster-a")
	if err != nil {
		t.Fatalf("getDynamicClient() returned unexpected error: %v", err)
	}

	client.Close()

	second, err := client.getDynamicClient("cluster-a")
	if err != nil {
		t.Fatalf("getDynamicClient() after Close returned unexpected error: %v", err)
	}

	if interfacePointer(first) == interfacePointer(second) {
		t.Fatal("expected Close to clear dynamic client cache")
	}
}
