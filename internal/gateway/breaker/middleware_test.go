package breaker

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestNewMiddleware tests middleware creation.
func TestNewMiddleware(t *testing.T) {
	store := NewStore(DefaultConfig())
	m := NewMiddleware(store, nil)

	if m == nil {
		t.Fatal("Expected non-nil middleware")
	}
	if m.store != store {
		t.Error("Expected store to be set")
	}
	if m.logger == nil {
		t.Error("Expected logger to be set")
	}
}

// TestDefaultMiddleware tests default middleware creation.
func TestDefaultMiddleware(t *testing.T) {
	m := DefaultMiddleware(nil)
	if m == nil {
		t.Fatal("Expected non-nil middleware")
	}
}

// TestMiddlewareAllowsWhenClosed tests requests pass through when closed.
func TestMiddlewareAllowsWhenClosed(t *testing.T) {
	config := &Config{
		Enabled:          true,
		FailureThreshold: 5,
	}
	store := NewStore(config)
	m := NewMiddleware(store, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	wrapped := m.Wrap(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if string(rec.Body.Bytes()) != "success" {
		t.Errorf("Expected body 'success', got %s", rec.Body.Bytes())
	}
}

// TestMiddlewareBlocksWhenOpen tests requests are rejected when open.
func TestMiddlewareBlocksWhenOpen(t *testing.T) {
	config := &Config{
		Enabled:          true,
		FailureThreshold: 2,
		Timeout:          time.Hour, // Long timeout to keep it open
	}
	store := NewStore(config)
	m := NewMiddleware(store, nil)

	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	})

	wrapped := m.Wrap(handler)

	// First 2 requests fail and open the circuit
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}

	// Circuit should be open now
	breaker := store.Get("/test")
	if breaker.State() != StateOpen {
		t.Fatalf("Expected circuit to be open, got %s", breaker.State().String())
	}

	// Third request should be rejected
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
	if rec.Header().Get("X-Circuit-Breaker") != "open" {
		t.Error("Expected X-Circuit-Breaker header")
	}
	if callCount != 2 {
		t.Errorf("Expected handler called 2 times, got %d", callCount)
	}
}

// TestMiddlewareRecordsSuccess tests successful responses.
func TestMiddlewareRecordsSuccess(t *testing.T) {
	config := &Config{
		Enabled:          true,
		FailureThreshold: 5,
	}
	store := NewStore(config)
	m := NewMiddleware(store, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := m.Wrap(handler)

	// Make several successful requests
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}

	breaker := store.Get("/test")
	stats := breaker.Stats()
	if stats.Failures != 0 {
		t.Errorf("Expected 0 failures, got %d", stats.Failures)
	}
}

// TestMiddlewareClientErrorsNotFailures tests 4xx errors don't count as failures.
func TestMiddlewareClientErrorsNotFailures(t *testing.T) {
	config := &Config{
		Enabled:          true,
		FailureThreshold: 2,
	}
	store := NewStore(config)
	m := NewMiddleware(store, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	wrapped := m.Wrap(handler)

	// Make several 404 requests
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}

	breaker := store.Get("/test")
	if breaker.State() != StateClosed {
		t.Errorf("Expected circuit to remain closed, got %s", breaker.State().String())
	}
}

// TestMiddlewareServerErrorsAreFailures tests 5xx errors count as failures.
func TestMiddlewareServerErrorsAreFailures(t *testing.T) {
	config := &Config{
		Enabled:          true,
		FailureThreshold: 3,
	}
	store := NewStore(config)
	m := NewMiddleware(store, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	wrapped := m.Wrap(handler)

	// Make requests that fail
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}

	breaker := store.Get("/test")
	if breaker.State() != StateOpen {
		t.Errorf("Expected circuit to be open, got %s", breaker.State().String())
	}
}

// TestWithUpstreamKey tests context key injection.
func TestWithUpstreamKey(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	newReq := WithUpstreamKey(req, "api-backend")

	// Check the key was added
	if key, ok := newReq.Context().Value(upstreamKey{}).(string); !ok || key != "api-backend" {
		t.Error("Expected upstream key to be set in context")
	}
}

// TestResponseWriterCapture tests status code capture.
func TestResponseWriterCapture(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	rw.WriteHeader(http.StatusCreated)
	if rw.statusCode != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, rw.statusCode)
	}

	rw.Write([]byte("test"))
	if rec.Body.String() != "test" {
		t.Errorf("Expected body 'test', got %s", rec.Body.String())
	}
}

// TestResponseWriterAutoStatus tests automatic status code.
func TestResponseWriterAutoStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	// Write without explicit WriteHeader
	rw.Write([]byte("test"))

	if rw.statusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rw.statusCode)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("Expected recorder status %d, got %d", http.StatusOK, rec.Code)
	}
}
