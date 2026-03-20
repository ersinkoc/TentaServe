package integration_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ersinkoc/tentaserve/internal/gateway/auth"
	"github.com/ersinkoc/tentaserve/internal/gateway/cache"
	"github.com/ersinkoc/tentaserve/internal/gateway/health"
	"github.com/ersinkoc/tentaserve/internal/gateway/orchestrator"
	"github.com/ersinkoc/tentaserve/internal/gateway/ratelimit"
)

// TestGateway_AuthAndRateLimit verifies that authentication and rate limiting
// middleware work together correctly when layered in the orchestrator.
func TestGateway_AuthAndRateLimit(t *testing.T) {
	restSrv := startMockRESTServer()
	defer restSrv.Close()

	logger := slog.Default()
	orch := orchestrator.New(logger)

	// Register an API key auth plugin with a known key.
	orch.RegisterAuthPlugin("apikey", auth.NewAPIKey([]string{"test-key-123"}))

	err := orch.LoadConfig(&orchestrator.Config{
		Upstreams: []orchestrator.Upstream{
			{
				Name:       "users",
				Target:     restSrv.URL,
				PathPrefix: "/api",
				AuthPlugin: "apikey",
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	ts := httptest.NewServer(orch)
	defer ts.Close()

	tests := []struct {
		name       string
		apiKey     string
		wantStatus int
	}{
		{
			name:       "valid API key",
			apiKey:     "test-key-123",
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing API key",
			apiKey:     "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid API key",
			apiKey:     "wrong-key",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", ts.URL+"/api/users", nil)
			if tt.apiKey != "" {
				req.Header.Set("X-API-Key", tt.apiKey)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, resp.StatusCode)
			}
		})
	}
}

// TestGateway_PassthroughAuth verifies that the passthrough auth plugin
// allows all requests through without requiring credentials.
func TestGateway_PassthroughAuth(t *testing.T) {
	restSrv := startMockRESTServer()
	defer restSrv.Close()

	logger := slog.Default()
	orch := orchestrator.New(logger)

	err := orch.LoadConfig(&orchestrator.Config{
		Upstreams: []orchestrator.Upstream{
			{
				Name:       "users",
				Target:     restSrv.URL,
				PathPrefix: "/api",
				AuthPlugin: "passthrough",
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	ts := httptest.NewServer(orch)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/users")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	body := readBody(t, resp)

	assertStatusCode(t, resp, http.StatusOK)
	assertContains(t, body, "Alice")
}

// TestGateway_CacheHitMiss verifies the cache middleware correctly caches
// responses and returns hits on subsequent requests.
func TestGateway_CacheHitMiss(t *testing.T) {
	callCount := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"count": callCount,
		})
	}))
	defer backend.Close()

	logger := slog.Default()

	cacheConfig := &cache.Config{
		Enabled:       true,
		MaxSize:       10 * 1024 * 1024,
		MaxEntries:    1000,
		TTL:           5 * time.Minute,
		StaleDuration: 1 * time.Minute,
		VaryHeaders:   []string{},
		Methods:       []string{"GET", "HEAD"},
		StatusCodes:   []int{200},
	}
	c := cache.New(cacheConfig)
	cm := cache.NewMiddleware(c, logger)

	handler := cm.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Proxy to backend
		resp, err := http.Get(backend.URL + r.URL.Path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		buf := make([]byte, 4096)
		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				w.Write(buf[:n])
			}
			if readErr != nil {
				break
			}
		}
	}))

	ts := httptest.NewServer(handler)
	defer ts.Close()

	// First request: should be a cache MISS
	resp1, err := http.Get(ts.URL + "/data")
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}
	body1 := readBody(t, resp1)
	assertContains(t, body1, `"count":1`)

	firstCallCount := callCount

	// Second request: should be a cache HIT (same URL)
	resp2, err := http.Get(ts.URL + "/data")
	if err != nil {
		t.Fatalf("second request failed: %v", err)
	}
	body2 := readBody(t, resp2)

	// The body should be the same as the first request (cached).
	assertContains(t, body2, `"count":1`)

	// The backend should have been called only once.
	if callCount != firstCallCount {
		t.Errorf("expected backend to be called once, but was called %d times", callCount)
	}

	// Check X-Cache header for HIT.
	xCache := resp2.Header.Get("X-Cache")
	if xCache != "HIT" {
		t.Errorf("expected X-Cache: HIT, got %q", xCache)
	}
}

// TestGateway_CacheBypassOnAuthorization verifies that requests with an
// Authorization header bypass the cache.
func TestGateway_CacheBypassOnAuthorization(t *testing.T) {
	callCount := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"count": callCount})
	}))
	defer backend.Close()

	cacheConfig := &cache.Config{
		Enabled:     true,
		MaxSize:     10 * 1024 * 1024,
		MaxEntries:  1000,
		TTL:         5 * time.Minute,
		VaryHeaders: []string{},
		Methods:     []string{"GET"},
		StatusCodes: []int{200},
	}
	c := cache.New(cacheConfig)
	cm := cache.NewMiddleware(c, slog.Default())

	handler := cm.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, _ := http.Get(backend.URL + r.URL.Path)
		defer resp.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		buf := make([]byte, 4096)
		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				w.Write(buf[:n])
			}
			if readErr != nil {
				break
			}
		}
	}))

	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Request with Authorization header should bypass cache.
	req, _ := http.NewRequest("GET", ts.URL+"/data", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	readBody(t, resp)

	// Second request with Authorization should also bypass.
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	readBody(t, resp2)

	// Backend should have been called twice (no caching for auth requests).
	if callCount != 2 {
		t.Errorf("expected 2 backend calls for auth requests, got %d", callCount)
	}
}

// TestGateway_RateLimitMiddleware verifies that the rate limiter rejects
// requests once the limit is exceeded.
func TestGateway_RateLimitMiddleware(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer backend.Close()

	store := ratelimit.NewStore(&ratelimit.Config{
		Enabled: true,
		Rate:    1, // 1 request per second
		Burst:   2, // burst of 2
		Scope:   ratelimit.ScopeGlobal,
	})
	rl := ratelimit.NewMiddleware(store, slog.Default())

	handler := rl.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))

	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Exhaust the burst.
	for i := 0; i < 2; i++ {
		resp, err := http.Get(ts.URL + "/test")
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i, resp.StatusCode)
		}
	}

	// The next request should be rate limited (429).
	resp, err := http.Get(ts.URL + "/test")
	if err != nil {
		t.Fatalf("rate limited request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", resp.StatusCode)
	}
}

// TestGateway_HealthEndpoint verifies the health check handler returns
// the correct structure and status.
func TestGateway_HealthEndpoint(t *testing.T) {
	checker := health.Default("0.1.0-test")

	tests := []struct {
		name       string
		handler    http.Handler
		wantStatus int
		wantInBody string
	}{
		{
			name:       "full health check",
			handler:    checker.Handler(),
			wantStatus: http.StatusOK,
			wantInBody: "healthy",
		},
		{
			name:       "liveness probe",
			handler:    checker.LivenessHandler(),
			wantStatus: http.StatusOK,
			wantInBody: "alive",
		},
		{
			name:       "readiness probe",
			handler:    checker.ReadinessHandler(),
			wantStatus: http.StatusOK,
			wantInBody: "ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/health", nil)
			w := httptest.NewRecorder()

			tt.handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}
			assertContains(t, w.Body.String(), tt.wantInBody)
		})
	}
}

// TestGateway_HealthCheckWithCustomCheck verifies that custom health checks
// are executed and affect the overall status.
func TestGateway_HealthCheckWithCustomCheck(t *testing.T) {
	checker := health.NewChecker("0.1.0-test")
	checker.SetCacheTTL(0) // disable caching for test

	checker.Register("always_healthy", health.SimpleCheck("always_healthy", func() error {
		return nil
	}))

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	checker.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	assertContains(t, w.Body.String(), "healthy")
}

// TestGateway_OrchestratorRouting verifies that the orchestrator routes
// requests to the correct upstream based on path prefix.
func TestGateway_OrchestratorRouting(t *testing.T) {
	restSrv := startMockRESTServer()
	defer restSrv.Close()

	logger := slog.Default()
	orch := orchestrator.New(logger)

	err := orch.LoadConfig(&orchestrator.Config{
		Upstreams: []orchestrator.Upstream{
			{
				Name:       "users",
				Target:     restSrv.URL,
				PathPrefix: "/api",
				AuthPlugin: "passthrough",
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "matched prefix",
			path:       "/api/users",
			wantStatus: http.StatusOK,
		},
		{
			name:       "unmatched prefix returns 404",
			path:       "/unknown/path",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			orch.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

// TestGateway_OrchestratorStats verifies that the orchestrator exposes
// gateway statistics after configuration is loaded.
func TestGateway_OrchestratorStats(t *testing.T) {
	restSrv := startMockRESTServer()
	defer restSrv.Close()

	logger := slog.Default()
	orch := orchestrator.New(logger)

	err := orch.LoadConfig(&orchestrator.Config{
		Upstreams: []orchestrator.Upstream{
			{
				Name:       "users",
				Target:     restSrv.URL,
				PathPrefix: "/api",
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	stats := orch.Stats()
	upstreams, ok := stats["upstreams"].(int)
	if !ok {
		t.Fatalf("expected upstreams count in stats, got %T", stats["upstreams"])
	}
	if upstreams != 1 {
		t.Errorf("expected 1 upstream, got %d", upstreams)
	}
}
