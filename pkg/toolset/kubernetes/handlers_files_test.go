package kubernetes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
)

// TestHandleDownloadFile_MissingRequiredParams tests that handleDownloadFile returns
// errors when required parameters are missing.
func TestHandleDownloadFile_MissingRequiredParams(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]interface{}
		wantErrPart string
	}{
		{
			name:        "missing cluster",
			params:      map[string]interface{}{"namespace": "ns", "name": "pod", "filePath": "/path"},
			wantErrPart: "cluster",
		},
		{
			name:        "missing namespace",
			params:      map[string]interface{}{"cluster": "c1", "name": "pod", "filePath": "/path"},
			wantErrPart: "namespace",
		},
		{
			name:        "missing name",
			params:      map[string]interface{}{"cluster": "c1", "namespace": "ns", "filePath": "/path"},
			wantErrPart: "name",
		},
		{
			name:        "missing filePath",
			params:      map[string]interface{}{"cluster": "c1", "namespace": "ns", "name": "pod"},
			wantErrPart: "filePath",
		},
		{
			name:        "empty cluster",
			params:      map[string]interface{}{"cluster": "", "namespace": "ns", "name": "pod", "filePath": "/path"},
			wantErrPart: "cluster",
		},
		{
			name:        "empty namespace",
			params:      map[string]interface{}{"cluster": "c1", "namespace": "", "name": "pod", "filePath": "/path"},
			wantErrPart: "namespace",
		},
		{
			name:        "empty name",
			params:      map[string]interface{}{"cluster": "c1", "namespace": "ns", "name": "", "filePath": "/path"},
			wantErrPart: "name",
		},
		{
			name:        "empty filePath",
			params:      map[string]interface{}{"cluster": "c1", "namespace": "ns", "name": "pod", "filePath": ""},
			wantErrPart: "filePath",
		},
	}

	// Use a mock steve client to bypass client validation
	mockClient := steve.NewClient("https://example.com", "token", "", "", false)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handleDownloadFile(context.Background(), mockClient, tt.params)
			if err == nil {
				t.Errorf("handleDownloadFile() expected error for %s, got nil", tt.wantErrPart)
				return
			}
			if !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Errorf("handleDownloadFile() error = %v, want error containing %q", err, tt.wantErrPart)
			}
		})
	}
}

// TestHandleDownloadFile_InvalidClientType tests that handleDownloadFile returns
// an error when the client type is invalid.
func TestHandleDownloadFile_InvalidClientType(t *testing.T) {
	tests := []struct {
		name   string
		client interface{}
	}{
		{
			name:   "nil client",
			client: nil,
		},
		{
			name:   "wrong type string",
			client: "not a client",
		},
		{
			name:   "wrong type int",
			client: 42,
		},
		{
			name:   "wrong type struct",
			client: struct{ Name string }{"test"},
		},
	}

	params := map[string]interface{}{
		"cluster":   "c1",
		"namespace": "ns",
		"name":      "pod",
		"filePath":  "/tmp/test.txt",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handleDownloadFile(context.Background(), tt.client, params)
			if err == nil {
				t.Error("handleDownloadFile() expected error for invalid client type, got nil")
				return
			}
			// Should be ErrSteveNotConfigured or similar
			if err != paramutil.ErrSteveNotConfigured {
				// Also check if it wraps the error
				if !strings.Contains(err.Error(), "not configured") && !strings.Contains(err.Error(), "client") {
					t.Errorf("handleDownloadFile() error = %v, want client configuration error", err)
				}
			}
		})
	}
}

// TestHandleDownloadFile_InvalidMaxFileSize tests that handleDownloadFile returns
// an error when maxFileSize is invalid.
func TestHandleDownloadFile_InvalidMaxFileSize(t *testing.T) {
	mockClient := steve.NewClient("https://example.com", "token", "", "", false)

	params := map[string]interface{}{
		"cluster":     "c1",
		"namespace":   "ns",
		"name":        "pod",
		"filePath":    "/tmp/test.txt",
		"maxFileSize": "invalid-size",
	}

	_, err := handleDownloadFile(context.Background(), mockClient, params)
	if err == nil {
		t.Error("handleDownloadFile() expected error for invalid maxFileSize, got nil")
		return
	}
	if !strings.Contains(err.Error(), "maxFileSize") {
		t.Errorf("handleDownloadFile() error = %v, want error about maxFileSize", err)
	}
}

// TestHandleUploadFile_MissingRequiredParams tests that handleUploadFile returns
// errors when required parameters are missing.
func TestHandleUploadFile_MissingRequiredParams(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]interface{}
		wantErrPart string
	}{
		{
			name:        "missing cluster",
			params:      map[string]interface{}{"namespace": "ns", "name": "pod", "filePath": "/path", "content": "Y29udGVudA=="},
			wantErrPart: "cluster",
		},
		{
			name:        "missing namespace",
			params:      map[string]interface{}{"cluster": "c1", "name": "pod", "filePath": "/path", "content": "Y29udGVudA=="},
			wantErrPart: "namespace",
		},
		{
			name:        "missing name",
			params:      map[string]interface{}{"cluster": "c1", "namespace": "ns", "filePath": "/path", "content": "Y29udGVudA=="},
			wantErrPart: "name",
		},
		{
			name:        "missing filePath",
			params:      map[string]interface{}{"cluster": "c1", "namespace": "ns", "name": "pod", "content": "Y29udGVudA=="},
			wantErrPart: "filePath",
		},
		{
			name:        "missing content",
			params:      map[string]interface{}{"cluster": "c1", "namespace": "ns", "name": "pod", "filePath": "/path"},
			wantErrPart: "content",
		},
		{
			name:        "empty cluster",
			params:      map[string]interface{}{"cluster": "", "namespace": "ns", "name": "pod", "filePath": "/path", "content": "Y29udGVudA=="},
			wantErrPart: "cluster",
		},
		{
			name:        "empty content",
			params:      map[string]interface{}{"cluster": "c1", "namespace": "ns", "name": "pod", "filePath": "/path", "content": ""},
			wantErrPart: "content",
		},
	}

	// Use a mock steve client to bypass client validation
	mockClient := steve.NewClient("https://example.com", "token", "", "", false)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handleUploadFile(context.Background(), mockClient, tt.params)
			if err == nil {
				t.Errorf("handleUploadFile() expected error for %s, got nil", tt.wantErrPart)
				return
			}
			if !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Errorf("handleUploadFile() error = %v, want error containing %q", err, tt.wantErrPart)
			}
		})
	}
}

// TestHandleUploadFile_ReadOnlyMode tests that handleUploadFile returns
// ErrReadOnlyMode when readOnly is true.
func TestHandleUploadFile_ReadOnlyMode(t *testing.T) {
	params := map[string]interface{}{
		"cluster":   "c1",
		"namespace": "ns",
		"name":      "pod",
		"filePath":  "/tmp/test.txt",
		"content":   "Y29udGVudA==",
		"readOnly":  true,
	}

	// Client doesn't matter for this test as readOnly check happens first
	_, err := handleUploadFile(context.Background(), nil, params)
	if err == nil {
		t.Error("handleUploadFile() expected error for readOnly mode, got nil")
		return
	}
	if err != paramutil.ErrReadOnlyMode {
		t.Errorf("handleUploadFile() error = %v, want %v", err, paramutil.ErrReadOnlyMode)
	}
}

// TestHandleUploadFile_InvalidClientType tests that handleUploadFile returns
// an error when the client type is invalid.
func TestHandleUploadFile_InvalidClientType(t *testing.T) {
	tests := []struct {
		name   string
		client interface{}
	}{
		{
			name:   "nil client",
			client: nil,
		},
		{
			name:   "wrong type string",
			client: "not a client",
		},
		{
			name:   "wrong type int",
			client: 42,
		},
	}

	params := map[string]interface{}{
		"cluster":   "c1",
		"namespace": "ns",
		"name":      "pod",
		"filePath":  "/tmp/test.txt",
		"content":   "Y29udGVudA==",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handleUploadFile(context.Background(), tt.client, params)
			if err == nil {
				t.Error("handleUploadFile() expected error for invalid client type, got nil")
				return
			}
			if err != paramutil.ErrSteveNotConfigured {
				if !strings.Contains(err.Error(), "not configured") && !strings.Contains(err.Error(), "client") {
					t.Errorf("handleUploadFile() error = %v, want client configuration error", err)
				}
			}
		})
	}
}

// TestHandleUploadFile_InvalidBase64Content tests that handleUploadFile returns
// an error when the content is not valid base64.
func TestHandleUploadFile_InvalidBase64Content(t *testing.T) {
	mockClient := steve.NewClient("https://example.com", "token", "", "", false)

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "invalid characters",
			content: "!!!not-base64!!!",
		},
		{
			name:    "invalid padding",
			content: "YWJj=====",
		},
		{
			name:    "truncated content",
			content: "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]interface{}{
				"cluster":   "c1",
				"namespace": "ns",
				"name":      "pod",
				"filePath":  "/tmp/test.txt",
				"content":   tt.content,
			}

			_, err := handleUploadFile(context.Background(), mockClient, params)
			if err == nil {
				t.Error("handleUploadFile() expected error for invalid base64, got nil")
				return
			}
			if !strings.Contains(err.Error(), "base64") {
				t.Errorf("handleUploadFile() error = %v, want error about base64 decoding", err)
			}
		})
	}
}

// TestHandleUploadFile_InvalidMaxFileSize tests that handleUploadFile returns
// an error when maxFileSize is invalid.
func TestHandleUploadFile_InvalidMaxFileSize(t *testing.T) {
	mockClient := steve.NewClient("https://example.com", "token", "", "", false)

	params := map[string]interface{}{
		"cluster":     "c1",
		"namespace":   "ns",
		"name":        "pod",
		"filePath":    "/tmp/test.txt",
		"content":     "Y29udGVudA==",
		"maxFileSize": "not-a-quantity",
	}

	_, err := handleUploadFile(context.Background(), mockClient, params)
	if err == nil {
		t.Error("handleUploadFile() expected error for invalid maxFileSize, got nil")
		return
	}
	if !strings.Contains(err.Error(), "maxFileSize") {
		t.Errorf("handleUploadFile() error = %v, want error about maxFileSize", err)
	}
}

// TestHandleUploadFile_ContentExceedsMaxFileSize tests that handleUploadFile returns
// an error when the content exceeds the maximum file size.
func TestHandleUploadFile_ContentExceedsMaxFileSize(t *testing.T) {
	mockClient := steve.NewClient("https://example.com", "token", "", "", false)

	// Create content that is 100 bytes (larger than 10 bytes limit)
	largeContent := make([]byte, 100)
	for i := range largeContent {
		largeContent[i] = 'a'
	}
	encodedContent := base64.StdEncoding.EncodeToString(largeContent)

	params := map[string]interface{}{
		"cluster":     "c1",
		"namespace":   "ns",
		"name":        "pod",
		"filePath":    "/tmp/test.txt",
		"content":     encodedContent,
		"maxFileSize": "10", // 10 bytes
	}

	_, err := handleUploadFile(context.Background(), mockClient, params)
	if err == nil {
		t.Error("handleUploadFile() expected error for content exceeding maxFileSize, got nil")
		return
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("handleUploadFile() error = %v, want error about size exceeding limit", err)
	}
}

// TestHandleUploadFile_ValidBase64Content tests that valid base64 content is properly decoded.
func TestHandleUploadFile_ValidBase64Content(t *testing.T) {
	// This test verifies that the base64 decoding succeeds
	// It will fail on the client call, but that's expected without a real cluster
	mockClient := steve.NewClient("https://example.com", "token", "", "", false)

	// Valid base64 content
	testContent := "Hello, World!"
	encodedContent := base64.StdEncoding.EncodeToString([]byte(testContent))

	params := map[string]interface{}{
		"cluster":   "c1",
		"namespace": "ns",
		"name":      "pod",
		"filePath":  "/tmp/test.txt",
		"content":   encodedContent,
	}

	_, err := handleUploadFile(context.Background(), mockClient, params)
	// We expect an error because we don't have a real cluster
	// But the error should NOT be about base64 decoding
	if err != nil && strings.Contains(err.Error(), "base64") {
		t.Errorf("handleUploadFile() incorrectly reported base64 error for valid content: %v", err)
	}
}

// TestDownloadFileResponse_JSONMarshal tests the downloadFileResponse JSON structure.
func TestDownloadFileResponse_JSONMarshal(t *testing.T) {
	response := downloadFileResponse{
		FileName:  "test.txt",
		SizeBytes: 1024,
		Content:   "SGVsbG8sIFdvcmxkIQ==",
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded["fileName"] != "test.txt" {
		t.Errorf("fileName = %v, want %v", decoded["fileName"], "test.txt")
	}
	if int64(decoded["sizeBytes"].(float64)) != 1024 {
		t.Errorf("sizeBytes = %v, want %v", decoded["sizeBytes"], 1024)
	}
	if decoded["content"] != "SGVsbG8sIFdvcmxkIQ==" {
		t.Errorf("content = %v, want %v", decoded["content"], "SGVsbG8sIFdvcmxkIQ==")
	}
}

// TestUploadFileResponse_JSONMarshal tests the uploadFileResponse JSON structure.
func TestUploadFileResponse_JSONMarshal(t *testing.T) {
	response := uploadFileResponse{
		Status:       "ok",
		FilePath:     "/tmp/test.txt",
		BytesWritten: 2048,
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded["status"] != "ok" {
		t.Errorf("status = %v, want %v", decoded["status"], "ok")
	}
	if decoded["filePath"] != "/tmp/test.txt" {
		t.Errorf("filePath = %v, want %v", decoded["filePath"], "/tmp/test.txt")
	}
	if int(decoded["bytesWritten"].(float64)) != 2048 {
		t.Errorf("bytesWritten = %v, want %v", decoded["bytesWritten"], 2048)
	}
}

// TestHandleDownloadFile_CombinedClient tests that handleDownloadFile works with CombinedClient.
func TestHandleDownloadFile_CombinedClient(t *testing.T) {
	mockSteveClient := steve.NewClient("https://example.com", "token", "", "", false)
	combinedClient := &toolset.CombinedClient{
		Norman: nil,
		Steve:  mockSteveClient,
	}

	params := map[string]interface{}{
		"cluster":   "c1",
		"namespace": "ns",
		"name":      "pod",
		"filePath":  "/tmp/test.txt",
	}

	// The handler should accept CombinedClient
	// It will fail later when trying to connect, but it should pass client validation
	_, err := handleDownloadFile(context.Background(), combinedClient, params)
	if err == paramutil.ErrSteveNotConfigured {
		t.Error("handleDownloadFile() should accept CombinedClient with Steve client")
	}
}

// TestHandleUploadFile_CombinedClient tests that handleUploadFile works with CombinedClient.
func TestHandleUploadFile_CombinedClient(t *testing.T) {
	mockSteveClient := steve.NewClient("https://example.com", "token", "", "", false)
	combinedClient := &toolset.CombinedClient{
		Norman: nil,
		Steve:  mockSteveClient,
	}

	params := map[string]interface{}{
		"cluster":   "c1",
		"namespace": "ns",
		"name":      "pod",
		"filePath":  "/tmp/test.txt",
		"content":   "Y29udGVudA==",
	}

	// The handler should accept CombinedClient
	_, err := handleUploadFile(context.Background(), combinedClient, params)
	if err == paramutil.ErrSteveNotConfigured {
		t.Error("handleUploadFile() should accept CombinedClient with Steve client")
	}
}

// TestHandleDownloadFile_CombinedClientNilSteve tests that handleDownloadFile returns
// error when CombinedClient has nil Steve client.
func TestHandleDownloadFile_CombinedClientNilSteve(t *testing.T) {
	combinedClient := &toolset.CombinedClient{
		Norman: nil,
		Steve:  nil,
	}

	params := map[string]interface{}{
		"cluster":   "c1",
		"namespace": "ns",
		"name":      "pod",
		"filePath":  "/tmp/test.txt",
	}

	_, err := handleDownloadFile(context.Background(), combinedClient, params)
	if err != paramutil.ErrSteveNotConfigured {
		t.Errorf("handleDownloadFile() error = %v, want %v", err, paramutil.ErrSteveNotConfigured)
	}
}

// TestHandleUploadFile_CombinedClientNilSteve tests that handleUploadFile returns
// error when CombinedClient has nil Steve client.
func TestHandleUploadFile_CombinedClientNilSteve(t *testing.T) {
	combinedClient := &toolset.CombinedClient{
		Norman: nil,
		Steve:  nil,
	}

	params := map[string]interface{}{
		"cluster":   "c1",
		"namespace": "ns",
		"name":      "pod",
		"filePath":  "/tmp/test.txt",
		"content":   "Y29udGVudA==",
	}

	_, err := handleUploadFile(context.Background(), combinedClient, params)
	if err != paramutil.ErrSteveNotConfigured {
		t.Errorf("handleUploadFile() error = %v, want %v", err, paramutil.ErrSteveNotConfigured)
	}
}

// TestHandleUploadFile_ReadOnlyFalse tests that handleUploadFile proceeds when readOnly is false.
func TestHandleUploadFile_ReadOnlyFalse(t *testing.T) {
	mockClient := steve.NewClient("https://example.com", "token", "", "", false)

	params := map[string]interface{}{
		"cluster":   "c1",
		"namespace": "ns",
		"name":      "pod",
		"filePath":  "/tmp/test.txt",
		"content":   "Y29udGVudA==",
		"readOnly":  false,
	}

	_, err := handleUploadFile(context.Background(), mockClient, params)
	// Should not be ErrReadOnlyMode
	if err == paramutil.ErrReadOnlyMode {
		t.Error("handleUploadFile() should not return ErrReadOnlyMode when readOnly is false")
	}
}

