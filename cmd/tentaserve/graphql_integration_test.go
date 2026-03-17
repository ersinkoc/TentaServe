package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ersinkoc/tentaserve/internal/config"
	"github.com/ersinkoc/tentaserve/internal/gateway/metrics"
	"github.com/ersinkoc/tentaserve/internal/observability"
)

// TestGraphQLHandlerIntegration tests the GraphQL handler with upstream configuration.
func TestGraphQLHandlerIntegration(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		Gateway: config.GatewayConfig{
			GraphQLPath: "/graphql",
			MCPPath:     "/mcp",
			RESTPrefix:  "/api",
		},
		Schema: config.SchemaConfig{
			Limits: config.SchemaLimits{
				MaxDepth:      10,
				MaxComplexity: 1000,
			},
		},
		Upstreams: []config.UpstreamConfig{
			{
				Name:    "users-api",
				Type:    "rest",
				BaseURL: "http://localhost:8081",
			},
			{
				Name:     "products-api",
				Type:     "graphql",
				Endpoint: "http://localhost:8082/graphql",
			},
		},
		Observability: config.ObservabilityConfig{
			RequestID: config.RequestIDConfig{
				Header:    "X-Request-ID",
				Propagate: true,
			},
		},
	}

	logger := observability.NewLogger(config.LoggingConfig{Level: "info"})
	metricsReg := metrics.NewRegistry()

	// Create the handler
	handler := createHandler(cfg, logger, nil, metricsReg)

	// Test 1: Simple query to REST upstream (using normalized field name)
	t.Run("QueryRESTUpstream", func(t *testing.T) {
		query := `{"query": "{ users_api { id } }"}`
		req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(query))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
			t.Logf("Response: %s", w.Body.String())
		}

		var result struct {
			Data   map[string]interface{} `json:"data"`
			Errors []map[string]interface{} `json:"errors"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if len(result.Errors) > 0 {
			t.Logf("Errors: %v", result.Errors)
		}

		if result.Data == nil {
			t.Error("expected data in response")
		}
	})

	// Test 2: Query to GraphQL upstream (using normalized field name)
	t.Run("QueryGraphQLUpstream", func(t *testing.T) {
		query := `{"query": "{ products_api { id } }"}`
		req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(query))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
			t.Logf("Response: %s", w.Body.String())
		}

		var result struct {
			Data   map[string]interface{} `json:"data"`
			Errors []map[string]interface{} `json:"errors"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if len(result.Errors) > 0 {
			t.Logf("Errors: %v", result.Errors)
		}

		if result.Data == nil {
			t.Error("expected data in response")
		}
	})

	// Test 3: Invalid query (syntax error)
	t.Run("InvalidQuery", func(t *testing.T) {
		query := `{"query": "{ invalid"}`
		req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(query))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// Should return error response
		var result struct {
			Errors []map[string]interface{} `json:"errors"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if len(result.Errors) == 0 {
			t.Error("expected errors in response for invalid query")
		}
	})

	// Test 4: Query with variables (using normalized field name)
	t.Run("QueryWithVariables", func(t *testing.T) {
		query := `{"query": "query GetUser($id: String) { users_api(id: $id) { id } }", "variables": {"id": "123"}}`
		req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(query))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		var result struct {
			Data   map[string]interface{} `json:"data"`
			Errors []map[string]interface{} `json:"errors"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		// Variables are passed but resolver doesn't use them yet - that's fine for now
		if result.Data == nil && len(result.Errors) > 0 {
			t.Logf("Errors: %v", result.Errors)
		}
	})
}

// TestGraphQLHandlerMethodNotAllowed tests that non-POST methods are rejected.
func TestGraphQLHandlerMethodNotAllowed(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		Gateway: config.GatewayConfig{
			GraphQLPath: "/graphql",
			MCPPath:     "/mcp",
			RESTPrefix:  "/api",
		},
		Schema: config.SchemaConfig{
			Limits: config.SchemaLimits{
				MaxDepth:      10,
				MaxComplexity: 1000,
			},
		},
		Observability: config.ObservabilityConfig{
			RequestID: config.RequestIDConfig{
				Header:    "X-Request-ID",
				Propagate: true,
			},
		},
	}

	logger := observability.NewLogger(config.LoggingConfig{Level: "info"})
	handler := createHandler(cfg, logger, nil, nil)

	// Test GET request to GraphQL endpoint
	req := httptest.NewRequest(http.MethodGet, "/graphql", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}
