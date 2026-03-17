package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
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
	if m.store == nil {
		t.Error("Expected store to be set")
	}
}

// TestMiddlewareStore tests retrieving the store.
func TestMiddlewareStore(t *testing.T) {
	store := NewStore(DefaultConfig())
	m := NewMiddleware(store, nil)

	if m.Store() != store {
		t.Error("Expected Store() to return the store")
	}
}

// TestMiddlewareAllow tests successful requests.
func TestMiddlewareAllow(t *testing.T) {
	config := &Config{
		Enabled: false, // Disabled to allow all
		Rate:    10,
		Burst:   5,
		Scope:   ScopeIP,
	}
	store := NewStore(config)
	m := NewMiddleware(store, nil)

	// Create a simple handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	wrapped := m.Wrap(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got %q", rec.Body.String())
	}
}

// TestMiddlewareRateLimit tests rate limiting rejection.
func TestMiddlewareRateLimit(t *testing.T) {
	config := &Config{
		Enabled: true,
		Rate:    1000,
		Burst:   1,
		Scope:   ScopeIP,
	}
	store := NewStore(config)
	m := NewMiddleware(store, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := m.Wrap(handler)

	// First request should succeed
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Expected first request to succeed, got %d", rec.Code)
	}

	// Second request should be rate limited
	req = httptest.NewRequest("GET", "/test", nil)
	rec = httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}

	// Should have Retry-After header
	retryAfter := rec.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("Expected Retry-After header")
	}

	// Should have JSON error body
	if rec.Body.String() == "" {
		t.Error("Expected error body")
	}
}

// TestMiddlewareWithUpstream tests rate limiting with upstream context.
func TestMiddlewareWithUpstream(t *testing.T) {
	config := &Config{
		Enabled: true,
		Rate:    1000,
		Burst:   1,
		Scope:   ScopeIP,
		PerUpstream: map[string]*Config{
			"api": {
				Enabled: true,
				Rate:    1000,
				Burst:   5,
				Scope:   ScopeIP,
			},
		},
	}
	store := NewStore(config)
	m := NewMiddleware(store, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := m.Wrap(handler)

	// Multiple requests with "api" upstream context should succeed (burst 5)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req = req.WithContext(context.WithValue(req.Context(), "upstream", "api"))
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("Request %d: Expected status OK, got %d", i, rec.Code)
		}
	}

	// 6th request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req = req.WithContext(context.WithValue(req.Context(), "upstream", "api"))
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}
}

// TestMiddlewareContentType tests that rate limit response has correct content type.
func TestMiddlewareContentType(t *testing.T) {
	config := &Config{
		Enabled: true,
		Rate:    1000,
		Burst:   0,
		Scope:   ScopeIP,
	}
	store := NewStore(config)
	m := NewMiddleware(store, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := m.Wrap(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %q", contentType)
	}
}

// TestMiddlewareRetryAfterValue tests the retry-after value is reasonable.
func TestMiddlewareRetryAfterValue(t *testing.T) {
	config := &Config{
		Enabled: true,
		Rate:    10, // 10 per second
		Burst:   0,
		Scope:   ScopeIP,
	}
	store := NewStore(config)
	m := NewMiddleware(store, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := m.Wrap(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	retryAfter := rec.Header().Get("Retry-After")
	// Should be around 0 or 1 (since rate is 10/s, wait time is ~100ms)
	if retryAfter != "0" && retryAfter != "1" {
		t.Errorf("Expected Retry-After to be 0 or 1, got %s", retryAfter)
	}
}

// TestMiddlewareConcurrent tests concurrent rate limiting.
func TestMiddlewareConcurrent(t *testing.T) {
	config := &Config{
		Enabled: true,
		Rate:    1, // Very low rate to prevent refill during test
		Burst:   100,
		Scope:   ScopeIP,
	}
	store := NewStore(config)
	m := NewMiddleware(store, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := m.Wrap(handler)

	var successCount int
	var limitedCount int
	var mu sync.Mutex

	// Run 200 concurrent requests
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/test", nil)
			rec := httptest.NewRecorder()
			wrapped.ServeHTTP(rec, req)

			mu.Lock()
			if rec.Code == http.StatusOK {
				successCount++
			} else if rec.Code == http.StatusTooManyRequests {
				limitedCount++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	// Should allow exactly burst (100) requests
	if successCount != 100 {
		t.Errorf("Expected 100 successful requests, got %d", successCount)
	}
	if limitedCount != 100 {
		t.Errorf("Expected 100 rate limited requests, got %d", limitedCount)
	}
}

// BenchmarkMiddleware benchmarks the middleware.
func BenchmarkMiddleware(b *testing.B) {
	config := &Config{
		Enabled: true,
		Rate:    1000000,
		Burst:   b.N + 1000,
		Scope:   ScopeIP,
	}
	store := NewStore(config)
	m := NewMiddleware(store, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := m.Wrap(handler)

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}
}
