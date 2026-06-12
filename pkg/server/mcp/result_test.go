package mcp

import (
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestNewTextResult(t *testing.T) {
	// Test success case
	result := NewTextResult("success message", nil)
	if result.IsError {
		t.Error("Result should not be an error")
	}

	if len(result.Content) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(result.Content))
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("Content should be TextContent")
	}

	if textContent.Text != "success message" {
		t.Errorf("Expected 'success message', got '%s'", textContent.Text)
	}

	// Test error case
	err := fmt.Errorf("test error")
	result = NewTextResult("", err)
	if !result.IsError {
		t.Error("Result should be an error")
	}

	if len(result.Content) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(result.Content))
	}

	textContent, ok = result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("Content should be TextContent")
	}

	if textContent.Text != "test error" {
		t.Errorf("Expected 'test error', got '%s'", textContent.Text)
	}
}
