package orchestrator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestNew tests orchestrator creation.
func TestNew(t *testing.T) {
	o := New(nil)
	if o == nil {
		t.Fatal("Expected non-nil orchestrator")
	}
	if o.handlers == nil {
		t.Error("Expected handlers map to be initialized")
	}
	if o.authPlugins == nil {
		t.Error("Expected auth plugins map to be initialized")
	}
	if o.client == nil {
		t.Error("Expected HTTP client to be initialized")
	}
}

// TestLoadConfig tests config loading.
func TestLoadConfig(t *testing.T) {
	o := New(nil)

	config := &Config{
		Upstreams: []Upstream{
			{
				Name:       "api",
				Target:     "http://localhost:8081",
				PathPrefix: "/api",
			},
		},
	}

	if err := o.LoadConfig(config); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if o.config == nil {
		t.Error("Expected config to be set")
	}
	if len(o.handlers) != 1 {
		t.Errorf("Expected 1 handler, got %d", len(o.handlers))
	}
}

// TestLoadConfigNil tests nil config.
func TestLoadConfigNil(t *testing.T) {
	o := New(nil)

	err := o.LoadConfig(nil)
	if err == nil {
		t.Error("Expected error for nil config")
	}
}

// TestFindHandler tests handler lookup.
func TestFindHandler(t *testing.T) {
	o := New(nil)
	config := &Config{
		Upstreams: []Upstream{
			{
				Name:       "api",
				Target:     "http://localhost:8081",
				PathPrefix: "/api",
			},
			{
				Name:       "admin",
				Target:     "http://localhost:8082",
				PathPrefix: "/api/admin",
			},
		},
	}

	if err := o.LoadConfig(config); err != nil {
		t.Fatal(err)
	}

	// Test exact match
	handler, prefix := o.findHandler("/api/users")
	if handler == nil {
		t.Error("Expected handler for /api/users")
	}
	if prefix != "/api" {
		t.Errorf("Expected prefix /api, got %s", prefix)
	}

	// Test longer match
	handler, prefix = o.findHandler("/api/admin/users")
	if handler == nil {
		t.Error("Expected handler for /api/admin/users")
	}
	if prefix != "/api/admin" {
		t.Errorf("Expected prefix /api/admin, got %s", prefix)
	}

	// Test no match
	handler, prefix = o.findHandler("/other")
	if handler != nil {
		t.Error("Expected no handler for /other")
	}
}

// TestServeHTTPNotFound tests 404 handling.
func TestServeHTTPNotFound(t *testing.T) {
	o := New(nil)
	config := &Config{
		Upstreams: []Upstream{
			{
				Name:       "api",
				Target:     "http://localhost:8081",
				PathPrefix: "/api",
			},
		},
	}
	if err := o.LoadConfig(config); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/other", nil)
	rec := httptest.NewRecorder()

	o.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

// TestUpstreamConfig structures.
func TestUpstreamConfig(t *testing.T) {
	config := &Upstream{
		Name:       "test",
		Target:     "http://localhost:8080",
		PathPrefix: "/api",
		Headers: map[string]string{
			"X-Custom": "value",
		},
		AuthPlugin: "jwt",
		AuthConfig: map[string]string{
			"secret": "test-secret",
		},
		RateLimit: &RateLimitConfig{
			Enabled: true,
			Rate:    100,
			Burst:   200,
			Scope:   "ip",
		},
		Cache: &CacheConfig{
			Enabled:       true,
			TTL:           300,
			StatusCodes:   []int{200, 301},
			VaryHeaders:   []string{"Accept"},
			StaleDuration: 60,
		},
		Breaker: &BreakerConfig{
			Enabled:          true,
			FailureThreshold: 5,
			SuccessThreshold: 3,
			TimeoutSeconds:   30,
		},
		HealthCheck: &HealthCheck{
			Enabled:  true,
			Path:     "/health",
			Interval: 30,
		},
	}

	// Verify all fields are accessible
	if config.Name != "test" {
		t.Error("Name mismatch")
	}
	if config.RateLimit.Rate != 100 {
		t.Error("Rate limit mismatch")
	}
	if config.Cache.TTL != 300 {
		t.Error("Cache TTL mismatch")
	}
	if config.Breaker.FailureThreshold != 5 {
		t.Error("Breaker threshold mismatch")
	}
}

// TestStats returns stats.
func TestStats(t *testing.T) {
	o := New(nil)
	config := &Config{
		Upstreams: []Upstream{
			{
				Name:       "api",
				Target:     "http://localhost:8081",
				PathPrefix: "/api",
				Cache: &CacheConfig{
					Enabled: true,
					TTL:     60,
				},
				Breaker: &BreakerConfig{
					Enabled: true,
				},
			},
		},
	}

	if err := o.LoadConfig(config); err != nil {
		t.Fatal(err)
	}

	stats := o.Stats()
	if stats["upstreams"] != 1 {
		t.Errorf("Expected 1 upstream, got %v", stats["upstreams"])
	}
	if stats["caches"] == nil {
		t.Error("Expected cache stats")
	}
	if stats["breakers"] == nil {
		t.Error("Expected breaker stats")
	}
}

// TestRegisterAuthPlugin tests plugin registration.
func TestRegisterAuthPlugin(t *testing.T) {
	o := New(nil)

	// Check built-in plugins
	if _, ok := o.authPlugins["passthrough"]; !ok {
		t.Error("Expected passthrough plugin")
	}
	if _, ok := o.authPlugins["jwt"]; !ok {
		t.Error("Expected jwt plugin")
	}
	if _, ok := o.authPlugins["apikey"]; !ok {
		t.Error("Expected apikey plugin")
	}
}

// TestGetUpstream tests upstream lookup.
func TestGetUpstream(t *testing.T) {
	o := New(nil)
	config := &Config{
		Upstreams: []Upstream{
			{
				Name:       "api",
				Target:     "http://localhost:8081",
				PathPrefix: "/api",
			},
		},
	}

	if err := o.LoadConfig(config); err != nil {
		t.Fatal(err)
	}

	upstream, err := o.GetUpstream("api")
	if err != nil {
		t.Fatalf("Expected to find upstream: %v", err)
	}
	if upstream.Name != "api" {
		t.Errorf("Expected name api, got %s", upstream.Name)
	}

	_, err = o.GetUpstream("missing")
	if err != ErrUpstreamNotFound {
		t.Error("Expected ErrUpstreamNotFound")
	}
}

// TestUpstreamFromContext tests context extraction.
func TestUpstreamFromContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), upstreamNameKey{}, "api")
	name := UpstreamFromContext(ctx)
	if name != "api" {
		t.Errorf("Expected api, got %s", name)
	}

	// Empty context
	name = UpstreamFromContext(context.Background())
	if name != "" {
		t.Errorf("Expected empty, got %s", name)
	}
}

// TestConfigJSON marshaling.
func TestConfigJSON(t *testing.T) {
	config := &Config{
		Upstreams: []Upstream{
			{
				Name:       "api",
				Target:     "http://localhost:8080",
				PathPrefix: "/api",
			},
		},
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded Config
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if len(decoded.Upstreams) != 1 {
		t.Error("Expected 1 upstream")
	}
	if decoded.Upstreams[0].Name != "api" {
		t.Error("Name mismatch")
	}
}

// TestConcurrentAccess tests thread safety.
func TestConcurrentAccess(t *testing.T) {
	o := New(nil)

	// Load initial config
	config := &Config{
		Upstreams: []Upstream{
			{
				Name:       "api",
				Target:     "http://localhost:8081",
				PathPrefix: "/api",
			},
		},
	}
	if err := o.LoadConfig(config); err != nil {
		t.Fatal(err)
	}

	// Concurrent reads
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			o.findHandler("/api/test")
			done <- true
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}

// TestStopWithoutStart tests stopping without starting.
func TestStopWithoutStart(t *testing.T) {
	o := New(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := o.Stop(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

// --- Additional orchestrator tests for coverage ---

func TestServeHTTP_ProxyToUpstream(t *testing.T) {
	// Create a test upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Upstream", "true")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer upstream.Close()

	o := New(nil)
	config := &Config{
		Upstreams: []Upstream{
			{
				Name:       "test-api",
				Target:     upstream.URL,
				PathPrefix: "/api",
			},
		},
	}
	if err := o.LoadConfig(config); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()

	o.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("X-Upstream") != "true" {
		t.Error("Expected X-Upstream header from upstream")
	}
	if rec.Body.String() != `{"status":"ok"}` {
		t.Errorf("Expected body, got %s", rec.Body.String())
	}
}

func TestServeHTTP_ProxyWithCustomHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "custom-value" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("missing custom header"))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	o := New(nil)
	config := &Config{
		Upstreams: []Upstream{
			{
				Name:       "test-api",
				Target:     upstream.URL,
				PathPrefix: "/api",
				Headers: map[string]string{
					"X-Custom": "custom-value",
				},
			},
		},
	}
	if err := o.LoadConfig(config); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()

	o.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d. Body: %s", rec.Code, rec.Body.String())
	}
}

func TestServeHTTP_ProxyWithQueryString(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "foo=bar" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("missing query string"))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	o := New(nil)
	config := &Config{
		Upstreams: []Upstream{
			{
				Name:       "test-api",
				Target:     upstream.URL,
				PathPrefix: "/api",
			},
		},
	}
	if err := o.LoadConfig(config); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/test?foo=bar", nil)
	rec := httptest.NewRecorder()

	o.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d. Body: %s", rec.Code, rec.Body.String())
	}
}

func TestServeHTTP_UpstreamDown(t *testing.T) {
	o := New(nil)
	config := &Config{
		Upstreams: []Upstream{
			{
				Name:       "dead-api",
				Target:     "http://127.0.0.1:1", // port 1 should refuse
				PathPrefix: "/api",
			},
		},
	}
	if err := o.LoadConfig(config); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()

	o.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("Expected 502 Bad Gateway, got %d", rec.Code)
	}
}

func TestLoadConfig_MultipleUpstreams(t *testing.T) {
	o := New(nil)
	config := &Config{
		Upstreams: []Upstream{
			{Name: "api1", Target: "http://localhost:8081", PathPrefix: "/api1"},
			{Name: "api2", Target: "http://localhost:8082", PathPrefix: "/api2"},
			{Name: "api3", Target: "http://localhost:8083", PathPrefix: "/api3"},
		},
	}
	if err := o.LoadConfig(config); err != nil {
		t.Fatal(err)
	}
	if len(o.handlers) != 3 {
		t.Errorf("Expected 3 handlers, got %d", len(o.handlers))
	}
}

func TestLoadConfig_WithBreaker(t *testing.T) {
	o := New(nil)
	config := &Config{
		Upstreams: []Upstream{
			{
				Name:       "api",
				Target:     "http://localhost:8081",
				PathPrefix: "/api",
				Breaker: &BreakerConfig{
					Enabled:          true,
					FailureThreshold: 5,
					SuccessThreshold: 3,
					TimeoutSeconds:   30,
				},
			},
		},
	}
	if err := o.LoadConfig(config); err != nil {
		t.Fatal(err)
	}
	if len(o.breakers) != 1 {
		t.Errorf("Expected 1 breaker, got %d", len(o.breakers))
	}
}

func TestLoadConfig_WithCacheAndBreaker(t *testing.T) {
	o := New(nil)
	config := &Config{
		Upstreams: []Upstream{
			{
				Name:       "api",
				Target:     "http://localhost:8081",
				PathPrefix: "/api",
				Cache: &CacheConfig{
					Enabled: true,
					TTL:     60,
				},
				Breaker: &BreakerConfig{
					Enabled:          true,
					FailureThreshold: 5,
					SuccessThreshold: 3,
					TimeoutSeconds:   30,
				},
			},
		},
	}
	if err := o.LoadConfig(config); err != nil {
		t.Fatal(err)
	}
	if len(o.caches) != 1 {
		t.Errorf("Expected 1 cache, got %d", len(o.caches))
	}
	if len(o.breakers) != 1 {
		t.Errorf("Expected 1 breaker, got %d", len(o.breakers))
	}
}

func TestLoadConfig_WithAuthPlugin(t *testing.T) {
	o := New(nil)
	config := &Config{
		Upstreams: []Upstream{
			{
				Name:       "api",
				Target:     "http://localhost:8081",
				PathPrefix: "/api",
				AuthPlugin: "passthrough",
			},
		},
	}
	if err := o.LoadConfig(config); err != nil {
		t.Fatal(err)
	}
	if len(o.handlers) != 1 {
		t.Errorf("Expected 1 handler, got %d", len(o.handlers))
	}
}

func TestLoadConfig_ReloadConfig(t *testing.T) {
	o := New(nil)

	config1 := &Config{
		Upstreams: []Upstream{
			{Name: "api1", Target: "http://localhost:8081", PathPrefix: "/api1"},
		},
	}
	if err := o.LoadConfig(config1); err != nil {
		t.Fatal(err)
	}
	if len(o.handlers) != 1 {
		t.Errorf("Expected 1 handler after first load, got %d", len(o.handlers))
	}

	config2 := &Config{
		Upstreams: []Upstream{
			{Name: "api2", Target: "http://localhost:8082", PathPrefix: "/api2"},
			{Name: "api3", Target: "http://localhost:8083", PathPrefix: "/api3"},
		},
	}
	if err := o.LoadConfig(config2); err != nil {
		t.Fatal(err)
	}
	if len(o.handlers) != 2 {
		t.Errorf("Expected 2 handlers after reload, got %d", len(o.handlers))
	}
}

func TestGetUpstream_Multiple(t *testing.T) {
	o := New(nil)
	config := &Config{
		Upstreams: []Upstream{
			{Name: "api1", Target: "http://localhost:8081", PathPrefix: "/api1"},
			{Name: "api2", Target: "http://localhost:8082", PathPrefix: "/api2"},
		},
	}
	if err := o.LoadConfig(config); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		wantErr bool
	}{
		{"api1", false},
		{"api2", false},
		{"api3", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := o.GetUpstream(tc.name)
			if (err != nil) != tc.wantErr {
				t.Errorf("GetUpstream(%q) error = %v, wantErr %v", tc.name, err, tc.wantErr)
			}
		})
	}
}

func TestStats_NoCacheOrBreaker(t *testing.T) {
	o := New(nil)
	config := &Config{
		Upstreams: []Upstream{
			{Name: "api", Target: "http://localhost:8081", PathPrefix: "/api"},
		},
	}
	if err := o.LoadConfig(config); err != nil {
		t.Fatal(err)
	}

	stats := o.Stats()
	if stats["upstreams"] != 1 {
		t.Errorf("Expected 1 upstream, got %v", stats["upstreams"])
	}
	if stats["caches"] != nil {
		t.Error("Expected no cache stats for upstream without cache")
	}
	if stats["breakers"] != nil {
		t.Error("Expected no breaker stats for upstream without breaker")
	}
}

func TestFindHandler_EmptyHandlers(t *testing.T) {
	o := New(nil)
	config := &Config{Upstreams: []Upstream{}}
	if err := o.LoadConfig(config); err != nil {
		t.Fatal(err)
	}

	handler, prefix := o.findHandler("/anything")
	if handler != nil {
		t.Error("Expected nil handler for empty handlers")
	}
	if prefix != "" {
		t.Errorf("Expected empty prefix, got %s", prefix)
	}
}

func TestUpstreamFromContext_TypeMismatch(t *testing.T) {
	// Put a non-string value in context with the same key
	ctx := context.WithValue(context.Background(), upstreamNameKey{}, 42)
	name := UpstreamFromContext(ctx)
	if name != "" {
		t.Errorf("Expected empty string for non-string context value, got %s", name)
	}
}
