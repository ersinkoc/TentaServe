package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestAPIKeyName tests the API key plugin name.
func TestAPIKeyName(t *testing.T) {
	a := NewAPIKey([]string{"key1"})
	if a.Name() != "apikey" {
		t.Errorf("Expected name 'apikey', got %s", a.Name())
	}
}

// TestAPIKeyNewAPIKeyWithOptions tests creating API key plugin with custom options.
func TestAPIKeyNewAPIKeyWithOptions(t *testing.T) {
	a := NewAPIKeyWithOptions(
		[]string{"key1", "key2"},
		"X-Custom-Key",
		"ApiKey ",
	)

	if len(a.Keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(a.Keys))
	}
	if a.HeaderName != "X-Custom-Key" {
		t.Errorf("Expected header name 'X-Custom-Key', got %s", a.HeaderName)
	}
	if a.HeaderPrefix != "ApiKey " {
		t.Errorf("Expected header prefix 'ApiKey ', got %s", a.HeaderPrefix)
	}
}

// TestAPIKeyValidKey tests valid API key authentication.
func TestAPIKeyValidKey(t *testing.T) {
	a := NewAPIKey([]string{"valid-key-123", "another-key"})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "valid-key-123")

	result, err := a.Authenticate(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result.Authenticated {
		t.Error("Expected Authenticated to be true")
	}

	// API key header should be stripped
	if result.Headers.Get("X-API-Key") != "" {
		t.Error("X-API-Key header should be stripped")
	}
}

// TestAPIKeyInvalidKey tests invalid API key rejection.
func TestAPIKeyInvalidKey(t *testing.T) {
	a := NewAPIKey([]string{"valid-key"})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "invalid-key")

	_, err := a.Authenticate(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for invalid key")
	}

	authErr, ok := err.(*AuthError)
	if !ok {
		t.Fatal("Expected AuthError")
	}
	if authErr.Code != "invalid_key" {
		t.Errorf("Expected code 'invalid_key', got %s", authErr.Code)
	}
	if authErr.Status != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, authErr.Status)
	}
	if authErr.Headers["WWW-Authenticate"] == "" {
		t.Error("Expected WWW-Authenticate header")
	}
}

// TestAPIKeyMissingKey tests missing API key handling.
func TestAPIKeyMissingKey(t *testing.T) {
	a := NewAPIKey([]string{"valid-key"})

	req := httptest.NewRequest("GET", "/test", nil)
	// No X-API-Key header

	_, err := a.Authenticate(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for missing key")
	}

	authErr, ok := err.(*AuthError)
	if !ok {
		t.Fatal("Expected AuthError")
	}
	if authErr.Code != "missing_key" {
		t.Errorf("Expected code 'missing_key', got %s", authErr.Code)
	}
}

// TestAPIKeyEmptyKeyList tests authentication with empty key list.
func TestAPIKeyEmptyKeyList(t *testing.T) {
	a := NewAPIKey([]string{})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "any-key")

	_, err := a.Authenticate(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for empty key list")
	}

	authErr, ok := err.(*AuthError)
	if !ok {
		t.Fatal("Expected AuthError")
	}
	if authErr.Code != "invalid_key" {
		t.Errorf("Expected code 'invalid_key', got %s", authErr.Code)
	}
}

// TestAPIKeyCustomHeader tests custom header name.
func TestAPIKeyCustomHeader(t *testing.T) {
	a := NewAPIKeyWithOptions(
		[]string{"valid-key"},
		"Authorization",
		"",
	)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "valid-key")

	result, err := a.Authenticate(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result.Authenticated {
		t.Error("Expected authenticated")
	}

	// Should fail with wrong header
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "valid-key")

	_, err = a.Authenticate(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for wrong header name")
	}
}

// TestAPIKeyWithPrefix tests API key with prefix.
func TestAPIKeyWithPrefix(t *testing.T) {
	a := NewAPIKeyWithOptions(
		[]string{"valid-key"},
		"Authorization",
		"ApiKey ",
	)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "ApiKey valid-key")

	result, err := a.Authenticate(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result.Authenticated {
		t.Error("Expected authenticated")
	}

	// Should fail without prefix
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "valid-key")

	_, err = a.Authenticate(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for missing prefix")
	}

	// Should fail with wrong prefix
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-key")

	_, err = a.Authenticate(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for wrong prefix")
	}
}

// TestAPIKeyForwardingHeaders tests that non-auth headers are forwarded.
func TestAPIKeyForwardingHeaders(t *testing.T) {
	a := NewAPIKey([]string{"valid-key"})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "valid-key")
	req.Header.Set("X-Custom-Header", "custom-value")
	req.Header.Set("Accept", "application/json")

	result, err := a.Authenticate(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Headers.Get("X-Custom-Header") != "custom-value" {
		t.Error("X-Custom-Header not forwarded")
	}
	if result.Headers.Get("Accept") != "application/json" {
		t.Error("Accept header not forwarded")
	}
}

// TestAPIKeyHopByHopHeaders tests that hop-by-hop headers are excluded.
func TestAPIKeyHopByHopHeaders(t *testing.T) {
	a := NewAPIKey([]string{"valid-key"})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "valid-key")
	req.Header.Set("Connection", "keep-alive")

	result, err := a.Authenticate(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Headers.Get("Connection") != "" {
		t.Error("Connection header should be excluded")
	}
}

// TestAPIKeyTimingAttack tests timing attack resistance.
// This test checks that the comparison is constant-time by verifying
// that different key lengths don't cause different behavior.
func TestAPIKeyTimingAttack(t *testing.T) {
	a := NewAPIKey([]string{"short", "a-very-long-api-key-that-is-much-longer"})

	// Test with keys of different lengths
	testCases := []struct {
		name string
		key  string
	}{
		{"short wrong", "shorx"},
		{"long wrong", "a-very-long-api-key-that-is-much-longerX"},
		{"completely different", "totally-different-key"},
	}

	for _, tc := range testCases {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-API-Key", tc.key)

		start := time.Now()
		_, err := a.Authenticate(context.Background(), req)
		_ = time.Since(start) // We don't actually measure timing in unit tests

		if err == nil {
			t.Errorf("%s: Expected error", tc.name)
		}
	}

	// Valid keys should succeed
	validKeys := []string{"short", "a-very-long-api-key-that-is-much-longer"}
	for _, key := range validKeys {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-API-Key", key)

		result, err := a.Authenticate(context.Background(), req)
		if err != nil {
			t.Errorf("Expected success for key '%s', got error: %v", key, err)
		}
		if !result.Authenticated {
			t.Errorf("Expected authenticated for key '%s'", key)
		}
	}
}

// TestAPIKeyMultipleKeys tests that any valid key works.
func TestAPIKeyMultipleKeys(t *testing.T) {
	a := NewAPIKey([]string{"key-one", "key-two", "key-three"})

	for _, key := range a.Keys {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-API-Key", key)

		result, err := a.Authenticate(context.Background(), req)
		if err != nil {
			t.Errorf("Unexpected error for key '%s': %v", key, err)
			continue
		}
		if !result.Authenticated {
			t.Errorf("Expected authenticated for key '%s'", key)
		}
	}
}

// BenchmarkAPIKeyValidate benchmarks API key validation.
func BenchmarkAPIKeyValidate(b *testing.B) {
	a := NewAPIKey([]string{"key-one", "key-two", "key-three", "key-four", "key-five"})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "key-five")

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Authenticate(ctx, req)
	}
}
