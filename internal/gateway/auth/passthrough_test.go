package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestPassthroughName tests the passthrough plugin name.
func TestPassthroughName(t *testing.T) {
	p := NewPassthrough()
	if p.Name() != "passthrough" {
		t.Errorf("Expected name 'passthrough', got %s", p.Name())
	}
}

// TestPassthroughAuthenticate tests basic passthrough authentication.
func TestPassthroughAuthenticate(t *testing.T) {
	p := NewPassthrough()

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("X-Custom-Header", "custom-value")

	result, err := p.Authenticate(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result.Authenticated {
		t.Error("Expected Authenticated to be true")
	}

	// Check headers were forwarded
	if result.Headers.Get("Authorization") != "Bearer token123" {
		t.Error("Authorization header not forwarded")
	}

	if result.Headers.Get("X-Custom-Header") != "custom-value" {
		t.Error("Custom header not forwarded")
	}
}

// TestPassthroughExcludeHeaders tests that excluded headers are not forwarded.
func TestPassthroughExcludeHeaders(t *testing.T) {
	p := NewPassthrough()

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Length", "100")
	req.Header.Set("Connection", "keep-alive")

	result, err := p.Authenticate(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Authorization should be forwarded
	if result.Headers.Get("Authorization") == "" {
		t.Error("Authorization header should be forwarded")
	}

	// Content-Length should be excluded
	if result.Headers.Get("Content-Length") != "" {
		t.Error("Content-Length header should be excluded")
	}

	// Connection should be excluded
	if result.Headers.Get("Connection") != "" {
		t.Error("Connection header should be excluded")
	}
}

// TestPassthroughWithOptions tests custom passthrough options.
func TestPassthroughWithOptions(t *testing.T) {
	p := NewPassthroughWithOptions([]string{"X-Exclude"}, "X-")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Include", "value1")
	req.Header.Set("X-Exclude", "value2")
	req.Header.Set("Other-Header", "value3")

	result, err := p.Authenticate(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// X-Include should be forwarded (matches prefix, not excluded)
	if result.Headers.Get("X-Include") != "value1" {
		t.Error("X-Include header should be forwarded")
	}

	// X-Exclude should be excluded (matches prefix and is in exclude list)
	// Actually, we only check exclusion by name, not prefix+name
	// So this tests that the header with name "X-Exclude" is excluded
	if result.Headers.Get("X-Exclude") != "" {
		t.Error("X-Exclude header should be excluded")
	}

	// Other-Header should NOT be forwarded (doesn't match prefix)
	if result.Headers.Get("Other-Header") != "" {
		t.Error("Other-Header should not be forwarded (doesn't match prefix)")
	}
}

// TestResultContext tests storing and retrieving auth result from context.
func TestResultContext(t *testing.T) {
	ctx := context.Background()

	// No result should return nil
	if ResultFromContext(ctx) != nil {
		t.Error("Expected nil result from empty context")
	}

	// Store a result
	result := &Result{
		Authenticated: true,
		Subject:       "user123",
		Claims: map[string]interface{}{
			"role": "admin",
		},
	}

	ctx = WithResult(ctx, result)

	// Retrieve the result
	retrieved := ResultFromContext(ctx)
	if retrieved == nil {
		t.Fatal("Expected non-nil result")
	}

	if !retrieved.Authenticated {
		t.Error("Expected Authenticated to be true")
	}

	if retrieved.Subject != "user123" {
		t.Errorf("Expected subject 'user123', got %s", retrieved.Subject)
	}
}

// TestIsAuthenticated tests the IsAuthenticated helper.
func TestIsAuthenticated(t *testing.T) {
	ctx := context.Background()

	// No result should return false
	if IsAuthenticated(ctx) {
		t.Error("Expected not authenticated from empty context")
	}

	// Store unauthenticated result
	ctx = WithResult(ctx, &Result{Authenticated: false})
	if IsAuthenticated(ctx) {
		t.Error("Expected not authenticated")
	}

	// Store authenticated result
	ctx = WithResult(ctx, &Result{Authenticated: true})
	if !IsAuthenticated(ctx) {
		t.Error("Expected authenticated")
	}
}

// TestSubjectFromContext tests the SubjectFromContext helper.
func TestSubjectFromContext(t *testing.T) {
	ctx := context.Background()

	// No result should return empty string
	if SubjectFromContext(ctx) != "" {
		t.Error("Expected empty subject from empty context")
	}

	// Store result with subject
	ctx = WithResult(ctx, &Result{
		Authenticated: true,
		Subject:       "user456",
	})

	if SubjectFromContext(ctx) != "user456" {
		t.Errorf("Expected subject 'user456', got %s", SubjectFromContext(ctx))
	}
}

// TestMiddleware tests the authentication middleware.
func TestMiddleware(t *testing.T) {
	plugin := NewPassthrough()
	middleware := NewMiddleware(plugin, nil)

	// Create a handler that checks the auth result
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := ResultFromContext(r.Context())
		if result == nil {
			t.Error("Expected auth result in context")
		}
		if !result.Authenticated {
			t.Error("Expected authenticated")
		}
		w.WriteHeader(http.StatusOK)
	})

	wrapped := middleware.Wrap(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Check that headers were forwarded to response
	if rec.Header().Get("Authorization") != "Bearer token" {
		t.Error("Authorization header not forwarded to response")
	}
}

// TestMiddlewarePlugin tests retrieving the underlying plugin.
func TestMiddlewarePlugin(t *testing.T) {
	plugin := NewPassthrough()
	middleware := NewMiddleware(plugin, nil)

	if middleware.Plugin() != plugin {
		t.Error("Expected plugin to be returned")
	}
}

// TestDefaultMiddleware tests creating a default middleware.
func TestDefaultMiddleware(t *testing.T) {
	middleware := DefaultMiddleware(nil)

	if middleware == nil {
		t.Fatal("Expected non-nil middleware")
	}

	if middleware.Plugin().Name() != "passthrough" {
		t.Error("Expected passthrough plugin")
	}
}

// TestEqualFold tests case-insensitive string comparison.
func TestEqualFold(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"hello", "hello", true},
		{"hello", "HELLO", true},
		{"Hello", "hello", true},
		{"hello", "world", false},
		{"hello", "helloo", false},
	}

	for _, tt := range tests {
		got := equalFold(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("equalFold(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

// TestHasPrefix tests case-insensitive prefix check.
func TestHasPrefix(t *testing.T) {
	tests := []struct {
		s, prefix string
		want     bool
	}{
		{"Authorization", "Auth", true},
		{"authorization", "AUTH", true},
		{"Content-Type", "Content", true},
		{"X-Custom", "Y", false},
		{"Short", "LongerPrefix", false},
	}

	for _, tt := range tests {
		got := hasPrefix(tt.s, tt.prefix)
		if got != tt.want {
			t.Errorf("hasPrefix(%q, %q) = %v, want %v", tt.s, tt.prefix, got, tt.want)
		}
	}
}

// BenchmarkPassthrough benchmarks the passthrough authentication.
func BenchmarkPassthrough(b *testing.B) {
	p := NewPassthrough()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer token")

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Authenticate(ctx, req)
	}
}
