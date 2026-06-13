package steve

import (
	"reflect"
	"sync"
	"testing"
)

func TestGetDynamicClient_ReusesClientPerCluster(t *testing.T) {
	client := NewClient("https://example.com", "token", "", "", false)

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
