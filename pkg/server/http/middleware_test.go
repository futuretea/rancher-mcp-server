package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test"))
	})

	middleware := RequestMiddleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	if rec.Body.String() != "test" {
		t.Errorf("expected body 'test', got '%s'", rec.Body.String())
	}
}

func TestRequestMiddlewareHealthCheck(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequestMiddleware(handler)

	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestLoggingResponseWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	lrw := &loggingResponseWriter{
		ResponseWriter: rec,
		statusCode:     http.StatusOK,
	}

	// Test WriteHeader
	lrw.WriteHeader(http.StatusNotFound)
	if lrw.statusCode != http.StatusNotFound {
		t.Errorf("expected status code 404, got %d", lrw.statusCode)
	}

	// Test multiple WriteHeader calls (should only write once)
	lrw.WriteHeader(http.StatusInternalServerError)
	if lrw.statusCode != http.StatusNotFound {
		t.Errorf("expected status code to remain 404, got %d", lrw.statusCode)
	}
}

func TestLoggingResponseWriterWrite(t *testing.T) {
	rec := httptest.NewRecorder()
	lrw := &loggingResponseWriter{
		ResponseWriter: rec,
		statusCode:     http.StatusOK,
	}

	// Test Write
	data := []byte("test data")
	n, err := lrw.Write(data)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != len(data) {
		t.Errorf("expected %d bytes written, got %d", len(data), n)
	}
	if lrw.statusCode != http.StatusOK {
		t.Errorf("expected status code 200, got %d", lrw.statusCode)
	}
}
