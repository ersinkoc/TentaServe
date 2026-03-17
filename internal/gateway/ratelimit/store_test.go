package ratelimit

import (
	"net/http/httptest"
	"testing"
	"time"
)

// TestNewStore tests store creation.
func TestNewStore(t *testing.T) {
	config := &Config{
		Enabled: true,
		Rate:    10,
		Burst:   20,
		Scope:   ScopeIP,
	}

	store := NewStore(config)
	if store == nil {
		t.Fatal("Expected non-nil store")
	}
	if store.config != config {
		t.Error("Expected config to be stored")
	}
}

// TestNewStoreDefaultConfig tests store creation with nil config.
func TestNewStoreDefaultConfig(t *testing.T) {
	store := NewStore(nil)
	if store == nil {
		t.Fatal("Expected non-nil store")
	}
	if store.config == nil {
		t.Fatal("Expected default config")
	}
	if store.config.Rate != 100 {
		t.Errorf("Expected default rate 100, got %f", store.config.Rate)
	}
	if store.config.Burst != 150 {
		t.Errorf("Expected default burst 150, got %d", store.config.Burst)
	}
}

// TestDefaultConfig tests default configuration.
func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Enabled {
		t.Error("Expected default Enabled to be false")
	}
	if config.Rate != 100 {
		t.Errorf("Expected default rate 100, got %f", config.Rate)
	}
	if config.Burst != 150 {
		t.Errorf("Expected default burst 150, got %d", config.Burst)
	}
	if config.Scope != ScopeIP {
		t.Errorf("Expected default scope IP, got %s", config.Scope)
	}
}

// TestStoreAllowDisabled tests that requests pass when disabled.
func TestStoreAllowDisabled(t *testing.T) {
	config := &Config{
		Enabled: false,
		Rate:    10,
		Burst:   5,
		Scope:   ScopeIP,
	}
	store := NewStore(config)

	req := httptest.NewRequest("GET", "/test", nil)

	// Should always allow when disabled
	for i := 0; i < 100; i++ {
		allowed, waitTime := store.Allow(req, "")
		if !allowed {
			t.Errorf("Expected allowed on iteration %d", i)
		}
		if waitTime != 0 {
			t.Errorf("Expected wait time 0, got %v", waitTime)
		}
	}
}

// TestStoreAllowGlobalScope tests global rate limiting.
func TestStoreAllowGlobalScope(t *testing.T) {
	config := &Config{
		Enabled: true,
		Rate:    1000, // High rate for testing
		Burst:   10,
		Scope:   ScopeGlobal,
	}
	store := NewStore(config)

	req1 := httptest.NewRequest("GET", "/test1", nil)
	req2 := httptest.NewRequest("GET", "/test2", nil)
	req2.RemoteAddr = "1.2.3.4:1234"

	// Both requests share the same bucket
	allowed := 0
	for i := 0; i < 10; i++ {
		if a, _ := store.Allow(req1, ""); a {
			allowed++
		}
		if a, _ := store.Allow(req2, ""); a {
			allowed++
		}
	}

	if allowed != 10 {
		t.Errorf("Expected 10 allowed total, got %d", allowed)
	}

	// Both should be rejected now
	if a, _ := store.Allow(req1, ""); a {
		t.Error("Expected req1 to be rejected")
	}
	if a, _ := store.Allow(req2, ""); a {
		t.Error("Expected req2 to be rejected")
	}
}

// TestStoreAllowIPScope tests IP-based rate limiting.
func TestStoreAllowIPScope(t *testing.T) {
	config := &Config{
		Enabled: true,
		Rate:    1000, // High rate for testing
		Burst:   5,
		Scope:   ScopeIP,
	}
	store := NewStore(config)

	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "1.2.3.4:1234"

	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "5.6.7.8:5678"

	// Each IP should have its own bucket
	for i := 0; i < 5; i++ {
		if a, _ := store.Allow(req1, ""); !a {
			t.Errorf("Expected req1 to be allowed on iteration %d", i)
		}
		if a, _ := store.Allow(req2, ""); !a {
			t.Errorf("Expected req2 to be allowed on iteration %d", i)
		}
	}

	// Both should be rejected (each bucket exhausted)
	if a, _ := store.Allow(req1, ""); a {
		t.Error("Expected req1 to be rejected")
	}
	if a, _ := store.Allow(req2, ""); a {
		t.Error("Expected req2 to be rejected")
	}
}

// TestStoreAllowHeaderScope tests header-based rate limiting.
func TestStoreAllowHeaderScope(t *testing.T) {
	config := &Config{
		Enabled:    true,
		Rate:       1000,
		Burst:      5,
		Scope:      ScopeHeader,
		HeaderName: "X-API-Key",
	}
	store := NewStore(config)

	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.Header.Set("X-API-Key", "key1")

	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("X-API-Key", "key2")

	// Each API key should have its own bucket
	for i := 0; i < 5; i++ {
		if a, _ := store.Allow(req1, ""); !a {
			t.Errorf("Expected req1 to be allowed on iteration %d", i)
		}
		if a, _ := store.Allow(req2, ""); !a {
			t.Errorf("Expected req2 to be allowed on iteration %d", i)
		}
	}

	// Both should be rejected (each bucket exhausted)
	if a, _ := store.Allow(req1, ""); a {
		t.Error("Expected req1 to be rejected")
	}
	if a, _ := store.Allow(req2, ""); a {
		t.Error("Expected req2 to be rejected")
	}
}

// TestStoreAllowPathScope tests path-based rate limiting.
func TestStoreAllowPathScope(t *testing.T) {
	config := &Config{
		Enabled: true,
		Rate:    1000,
		Burst:   5,
		Scope:   ScopePath,
	}
	store := NewStore(config)

	req1 := httptest.NewRequest("GET", "/path1", nil)
	req2 := httptest.NewRequest("GET", "/path2", nil)

	// Each path should have its own bucket
	for i := 0; i < 5; i++ {
		if a, _ := store.Allow(req1, ""); !a {
			t.Errorf("Expected req1 to be allowed on iteration %d", i)
		}
		if a, _ := store.Allow(req2, ""); !a {
			t.Errorf("Expected req2 to be allowed on iteration %d", i)
		}
	}

	// Both should be rejected (each bucket exhausted)
	if a, _ := store.Allow(req1, ""); a {
		t.Error("Expected req1 to be rejected")
	}
	if a, _ := store.Allow(req2, ""); a {
		t.Error("Expected req2 to be rejected")
	}
}

// TestStoreClientIP tests client IP extraction.
func TestStoreClientIP(t *testing.T) {
	store := NewStore(DefaultConfig())

	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		want       string
	}{
		{
			name:       "X-Forwarded-For single",
			remoteAddr: "10.0.0.1:1234",
			headers:    map[string]string{"X-Forwarded-For": "192.168.1.1"},
			want:       "192.168.1.1",
		},
		{
			name:       "X-Forwarded-For multiple",
			remoteAddr: "10.0.0.1:1234",
			headers:    map[string]string{"X-Forwarded-For": "192.168.1.1, 10.0.0.2, 10.0.0.3"},
			want:       "192.168.1.1",
		},
		{
			name:       "X-Real-Ip",
			remoteAddr: "10.0.0.1:1234",
			headers:    map[string]string{"X-Real-Ip": "192.168.2.2"},
			want:       "192.168.2.2",
		},
		{
			name:       "X-Forwarded-For takes precedence over X-Real-Ip",
			remoteAddr: "10.0.0.1:1234",
			headers:    map[string]string{
				"X-Forwarded-For": "192.168.1.1",
				"X-Real-Ip":       "192.168.2.2",
			},
			want: "192.168.1.1",
		},
		{
			name:       "RemoteAddr IPv4",
			remoteAddr: "10.0.0.1:1234",
			headers:    map[string]string{},
			want:       "10.0.0.1",
		},
		{
			name:       "RemoteAddr IPv6",
			remoteAddr: "[::1]:1234",
			headers:    map[string]string{},
			want:       "::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			got := store.clientIP(req)
			if got != tt.want {
				t.Errorf("clientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestStorePerUpstream tests per-upstream configuration override.
func TestStorePerUpstream(t *testing.T) {
	config := &Config{
		Enabled: true,
		Rate:    1000,
		Burst:   2,
		Scope:   ScopeIP,
		PerUpstream: map[string]*Config{
			"api": {
				Enabled: true,
				Rate:    1000,
				Burst:   10,
				Scope:   ScopeIP,
			},
		},
	}
	store := NewStore(config)

	// Default upstream should have burst of 2
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "1.1.1.1:1234"
	allowed := 0
	for i := 0; i < 10; i++ {
		if a, _ := store.Allow(req1, "default"); a {
			allowed++
		}
	}
	if allowed != 2 {
		t.Errorf("Expected 2 allowed for default upstream, got %d", allowed)
	}

	// "api" upstream should have burst of 10 (using different IP to get fresh bucket)
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "2.2.2.2:1234"
	allowed = 0
	for i := 0; i < 15; i++ {
		if a, _ := store.Allow(req2, "api"); a {
			allowed++
		}
	}
	if allowed != 10 {
		t.Errorf("Expected 10 allowed for api upstream, got %d", allowed)
	}
}

// TestStoreCleanup tests stale bucket cleanup.
func TestStoreCleanup(t *testing.T) {
	config := &Config{
		Enabled: true,
		Rate:    1000,
		Burst:   5,
		Scope:   ScopeIP,
	}
	store := NewStore(config)

	// Create buckets by making requests
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1." + string(rune('1'+i)) + ":1234"
		store.Allow(req, "")
	}

	if store.Len() != 5 {
		t.Errorf("Expected 5 buckets, got %d", store.Len())
	}

	// Wait a bit, then cleanup with short max age
	time.Sleep(10 * time.Millisecond)
	removed := store.Cleanup(5 * time.Millisecond)
	if removed != 5 {
		t.Errorf("Expected 5 removed, got %d", removed)
	}
	if store.Len() != 0 {
		t.Errorf("Expected 0 buckets, got %d", store.Len())
	}
}

// TestStoreCleanupPartial tests partial cleanup.
func TestStoreCleanupPartial(t *testing.T) {
	config := &Config{
		Enabled: true,
		Rate:    1000,
		Burst:   5,
		Scope:   ScopeIP,
	}
	store := NewStore(config)

	// Create buckets with manual lastAccess manipulation is hard
	// Instead, we'll just test that recent buckets are not removed
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1." + string(rune('1'+i)) + ":1234"
		store.Allow(req, "")
	}

	// Cleanup with 1 hour max age should remove nothing
	removed := store.Cleanup(time.Hour)
	if removed != 0 {
		t.Errorf("Expected 0 removed (buckets are recent), got %d", removed)
	}
	if store.Len() != 3 {
		t.Errorf("Expected 3 buckets, got %d", store.Len())
	}
}

// TestStoreReset tests clearing all buckets.
func TestStoreReset(t *testing.T) {
	config := &Config{
		Enabled: true,
		Rate:    1000,
		Burst:   5,
		Scope:   ScopeIP,
	}
	store := NewStore(config)

	// Create buckets
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1." + string(rune('1'+i)) + ":1234"
		store.Allow(req, "")
	}

	if store.Len() != 3 {
		t.Errorf("Expected 3 buckets, got %d", store.Len())
	}

	store.Reset()

	if store.Len() != 0 {
		t.Errorf("Expected 0 buckets after reset, got %d", store.Len())
	}
}

// TestStoreAllowRetryAfter tests retry-after calculation.
func TestStoreAllowRetryAfter(t *testing.T) {
	config := &Config{
		Enabled: true,
		Rate:    10, // 10 per second = 1 per 100ms
		Burst:   1,
		Scope:   ScopeIP,
	}
	store := NewStore(config)

	req := httptest.NewRequest("GET", "/test", nil)

	// First request should succeed
	allowed, _ := store.Allow(req, "")
	if !allowed {
		t.Fatal("Expected first request to succeed")
	}

	// Second request should fail with retry-after
	allowed, retryAfter := store.Allow(req, "")
	if allowed {
		t.Error("Expected second request to fail")
	}
	if retryAfter < 50*time.Millisecond || retryAfter > 150*time.Millisecond {
		t.Errorf("Expected retry-after ~100ms, got %v", retryAfter)
	}
}

// TestSplitHostPort tests the host:port splitting function.
func TestSplitHostPort(t *testing.T) {
	tests := []struct {
		addr string
		host string
		port string
	}{
		{"127.0.0.1:8080", "127.0.0.1", "8080"},
		{"[::1]:8080", "::1", "8080"},
		{"[2001:db8::1]:8080", "2001:db8::1", "8080"},
		{"localhost:8080", "localhost", "8080"},
		{"127.0.0.1", "127.0.0.1", ""},
	}

	for _, tt := range tests {
		host, port, _ := splitHostPort(tt.addr)
		if host != tt.host {
			t.Errorf("splitHostPort(%q) host = %q, want %q", tt.addr, host, tt.host)
		}
		if port != tt.port {
			t.Errorf("splitHostPort(%q) port = %q, want %q", tt.addr, port, tt.port)
		}
	}
}

// BenchmarkStoreAllow benchmarks store Allow.
func BenchmarkStoreAllow(b *testing.B) {
	config := &Config{
		Enabled: true,
		Rate:    1000000,
		Burst:   b.N + 1000,
		Scope:   ScopeIP,
	}
	store := NewStore(config)

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Allow(req, "")
	}
}
