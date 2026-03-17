package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ersinkoc/tentaserve/internal/config"
	"github.com/ersinkoc/tentaserve/internal/observability"
)

// TestRESTHandlerRouting tests REST request routing to upstreams.
func TestRESTHandlerRouting(t *testing.T) {
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			RESTPrefix: "/api",
		},
		Upstreams: []config.UpstreamConfig{
			{
				Name:    "users-api",
				Type:    "rest",
				BaseURL: "http://localhost:8081",
				Timeout: 30 * 1000000000, // 30 seconds in nanoseconds
			},
			{
				Name:    "products-api",
				Type:    "rest",
				BaseURL: "http://localhost:8082",
				Timeout: 30 * 1000000000,
			},
		},
	}

	logger := observability.NewLogger(config.LoggingConfig{Level: "info"})

	// Create handler
	handler := NewRESTHandler(cfg, logger.Logger)

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}

	if handler.router == nil {
		t.Error("expected non-nil router")
	}

	if len(handler.upstreams) != 2 {
		t.Errorf("expected 2 upstreams, got %d", len(handler.upstreams))
	}
}

// TestRESTHandlerUnknownUpstream tests requests to unconfigured upstreams.
func TestRESTHandlerUnknownUpstream(t *testing.T) {
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			RESTPrefix: "/api",
		},
		Upstreams: []config.UpstreamConfig{},
	}

	logger := observability.NewLogger(config.LoggingConfig{Level: "info"})

	handler := NewRESTHandler(cfg, logger.Logger)

	// Request to unknown path
	req := httptest.NewRequest(http.MethodGet, "/api/unknown", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should get 404 since no upstream matches
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// TestRESTHandlerClassifyNonREST tests that non-REST requests are rejected.
func TestRESTHandlerClassifyNonREST(t *testing.T) {
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			RESTPrefix:  "/api",
			GraphQLPath: "/graphql",
		},
		Upstreams: []config.UpstreamConfig{
			{
				Name:    "users-api",
				Type:    "rest",
				BaseURL: "http://localhost:8081",
			},
		},
	}

	logger := observability.NewLogger(config.LoggingConfig{Level: "info"})

	handler := NewRESTHandler(cfg, logger.Logger)

	// Request to GraphQL path (should be rejected)
	req := httptest.NewRequest(http.MethodPost, "/graphql", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d for non-REST request, got %d", http.StatusNotFound, w.Code)
	}
}
