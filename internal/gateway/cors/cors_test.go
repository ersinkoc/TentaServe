package cors

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestDefaultConfig tests default configuration.
func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if !config.Enabled {
		t.Error("Expected default enabled to be true")
	}
	if len(config.AllowedOrigins) != 1 || config.AllowedOrigins[0] != "*" {
		t.Errorf("Expected default origins [*], got %v", config.AllowedOrigins)
	}
	if len(config.AllowedMethods) != 7 {
		t.Errorf("Expected 7 default methods, got %d", len(config.AllowedMethods))
	}
	if config.MaxAge != 86400 {
		t.Errorf("Expected default max age 86400, got %d", config.MaxAge)
	}
}

// TestNew tests CORS creation.
func TestNew(t *testing.T) {
	c := New(nil)
	if c == nil {
		t.Fatal("Expected non-nil CORS")
	}
	if c.config == nil {
		t.Error("Expected config to be set")
	}
}

// TestDefault tests default CORS.
func TestDefault(t *testing.T) {
	c := Default()
	if c == nil {
		t.Fatal("Expected non-nil CORS")
	}
}

// TestWithOrigins tests origin-specific CORS.
func TestWithOrigins(t *testing.T) {
	c := WithOrigins("https://example.com", "https://app.example.com")
	if c == nil {
		t.Fatal("Expected non-nil CORS")
	}
	if len(c.config.AllowedOrigins) != 2 {
		t.Errorf("Expected 2 origins, got %d", len(c.config.AllowedOrigins))
	}
}

// TestMiddlewareDisabled tests disabled CORS.
func TestMiddlewareDisabled(t *testing.T) {
	config := &Config{Enabled: false}
	c := New(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := c.Middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("Expected no CORS headers when disabled")
	}
}

// TestPreflightAllowedOrigin tests preflight with allowed origin.
func TestPreflightAllowedOrigin(t *testing.T) {
	c := Default()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called for preflight")
	})

	wrapped := c.Middleware(handler)

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("Expected origin header, got %s", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

// TestPreflightWildcardOrigin tests preflight with wildcard origin.
func TestPreflightWildcardOrigin(t *testing.T) {
	c := Default()

	wrapped := c.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://any-domain.com")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "https://any-domain.com" {
		t.Errorf("Expected origin header, got %s", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

// TestPreflightNoOrigin tests preflight without origin.
func TestPreflightNoOrigin(t *testing.T) {
	c := Default()

	wrapped := c.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	// No Origin header
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("Expected no CORS headers without origin")
	}
}

// TestPreflightDisallowedOrigin tests preflight with disallowed origin.
func TestPreflightDisallowedOrigin(t *testing.T) {
	c := WithOrigins("https://trusted.com")

	wrapped := c.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://untrusted.com")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("Expected no CORS headers for disallowed origin")
	}
}

// TestPreflightMethods tests preflight method headers.
func TestPreflightMethods(t *testing.T) {
	config := &Config{
		Enabled:        true,
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "DELETE"},
		MaxAge:         3600,
	}
	c := New(config)

	wrapped := c.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	methods := rec.Header().Get("Access-Control-Allow-Methods")
	if methods != "GET, POST, DELETE" {
		t.Errorf("Expected methods header, got %s", methods)
	}
	if rec.Header().Get("Access-Control-Max-Age") != "3600" {
		t.Errorf("Expected max-age 3600, got %s", rec.Header().Get("Access-Control-Max-Age"))
	}
}

// TestPreflightRequestedHeaders tests preflight with requested headers.
func TestPreflightRequestedHeaders(t *testing.T) {
	config := &Config{
		Enabled:        true,
		AllowedOrigins: []string{"*"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	}
	c := New(config)

	wrapped := c.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type, Authorization, X-Custom")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	headers := rec.Header().Get("Access-Control-Allow-Headers")
	if headers != "Content-Type, Authorization" {
		t.Errorf("Expected allowed headers, got %s", headers)
	}
}

// TestPreflightWildcardHeaders tests preflight with wildcard headers.
func TestPreflightWildcardHeaders(t *testing.T) {
	config := &Config{
		Enabled:        true,
		AllowedOrigins: []string{"*"},
		AllowedHeaders: []string{"*"},
	}
	c := New(config)

	wrapped := c.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Headers", "X-Custom-Header, X-Another")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	headers := rec.Header().Get("Access-Control-Allow-Headers")
	if headers != "X-Custom-Header, X-Another" {
		t.Errorf("Expected all requested headers, got %s", headers)
	}
}

// TestActualRequest tests actual request CORS headers.
func TestActualRequest(t *testing.T) {
	c := Default()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response"))
	})

	wrapped := c.Middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("Expected origin header, got %s", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if string(rec.Body.Bytes()) != "response" {
		t.Errorf("Expected body 'response', got %s", rec.Body.Bytes())
	}
}

// TestActualRequestCredentials tests credentials header.
func TestActualRequestCredentials(t *testing.T) {
	config := &Config{
		Enabled:          true,
		AllowedOrigins:   []string{"https://example.com"},
		AllowCredentials: true,
	}
	c := New(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := c.Middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("Expected credentials header, got %s", rec.Header().Get("Access-Control-Allow-Credentials"))
	}
}

// TestExposedHeaders tests exposed headers.
func TestExposedHeaders(t *testing.T) {
	config := &Config{
		Enabled:        true,
		AllowedOrigins: []string{"*"},
		ExposedHeaders: []string{"X-Request-ID", "X-Rate-Limit"},
	}
	c := New(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := c.Middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	exposed := rec.Header().Get("Access-Control-Expose-Headers")
	if exposed != "X-Request-ID, X-Rate-Limit" {
		t.Errorf("Expected exposed headers, got %s", exposed)
	}
}

// TestPrivateNetwork tests private network header.
func TestPrivateNetwork(t *testing.T) {
	config := &Config{
		Enabled:             true,
		AllowedOrigins:      []string{"*"},
		AllowPrivateNetwork: true,
	}
	c := New(config)

	wrapped := c.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Private-Network", "true")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Private-Network") != "true" {
		t.Errorf("Expected private network header, got %s", rec.Header().Get("Access-Control-Allow-Private-Network"))
	}
}

// TestVaryHeader tests Vary header is set.
func TestVaryHeader(t *testing.T) {
	c := Default()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := c.Middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	vary := rec.Header()["Vary"]
	found := false
	for _, v := range vary {
		if v == "Origin" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected Vary: Origin header, got %v", vary)
	}
}
