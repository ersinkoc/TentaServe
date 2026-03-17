package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ersinkoc/tentaserve/internal/config"
	"github.com/ersinkoc/tentaserve/internal/gateway/metrics"
	"github.com/ersinkoc/tentaserve/internal/observability"
)

// TestNewMCPServer tests MCP server creation.
func TestNewMCPServer(t *testing.T) {
	cfg := &config.Config{
		MCP: config.MCPConfig{
			Enabled: true,
			Tools: config.MCPToolsConfig{
				AutoDiscover: true,
			},
			Resources: config.MCPResourceConfig{
				AutoExpose: true,
			},
		},
		Gateway: config.GatewayConfig{
			MCPPath: "/mcp",
		},
		Upstreams: []config.UpstreamConfig{
			{
				Name:    "test-api",
				Type:    "rest",
				BaseURL: "http://localhost:8080",
			},
		},
	}

	logger := observability.NewLogger(config.LoggingConfig{Level: "info"})
	metricsReg := metrics.NewRegistry()

	mcpServer, err := NewMCPServer(cfg, logger.Logger, metricsReg)
	if err != nil {
		t.Fatalf("Failed to create MCP server: %v", err)
	}

	if mcpServer == nil {
		t.Fatal("Expected non-nil MCP server")
	}

	if mcpServer.Server == nil {
		t.Error("Expected non-nil Server")
	}

	if mcpServer.Transport == nil {
		t.Error("Expected non-nil Transport")
	}

	if mcpServer.Registry == nil {
		t.Error("Expected non-nil Registry")
	}
}

// TestNewMCPServerDisabled tests MCP server creation when disabled.
func TestNewMCPServerDisabled(t *testing.T) {
	cfg := &config.Config{
		MCP: config.MCPConfig{
			Enabled: false,
		},
	}

	logger := observability.NewLogger(config.LoggingConfig{Level: "info"})

	mcpServer, err := NewMCPServer(cfg, logger.Logger, nil)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	if mcpServer != nil {
		t.Error("Expected nil MCP server when disabled")
	}
}

// TestMCPServerRegisterHandlers tests handler registration.
func TestMCPServerRegisterHandlers(t *testing.T) {
	cfg := &config.Config{
		MCP: config.MCPConfig{
			Enabled: true,
		},
		Gateway: config.GatewayConfig{
			MCPPath: "/mcp",
		},
	}

	logger := observability.NewLogger(config.LoggingConfig{Level: "info"})
	metricsReg := metrics.NewRegistry()

	mcpServer, err := NewMCPServer(cfg, logger.Logger, metricsReg)
	if err != nil {
		t.Fatalf("Failed to create MCP server: %v", err)
	}

	mux := http.NewServeMux()
	mcpServer.RegisterHandlers(mux, "/mcp")

	// Test that handlers are registered by checking if POST doesn't return 404
	// POST without proper body will return error but not 404
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Should not be 404 (handler registered)
	if w.Code == http.StatusNotFound {
		t.Error("POST /mcp endpoint not registered")
	}
}

// TestMCPServerNilSafe tests nil-safe methods.
func TestMCPServerNilSafe(t *testing.T) {
	var mcpServer *MCPServer

	// Should not panic
	mux := http.NewServeMux()
	mcpServer.RegisterHandlers(mux, "/mcp")

	// Should not panic on shutdown
	ctx := context.Background()
	err := mcpServer.Shutdown(ctx)
	if err != nil {
		t.Errorf("Expected no error for nil server shutdown, got: %v", err)
	}
}

// TestBuildResourcesFromUpstreams tests resource building.
func TestBuildResourcesFromUpstreams(t *testing.T) {
	upstreams := []config.UpstreamConfig{
		{
			Name:    "users-api",
			Type:    "rest",
			BaseURL: "http://localhost:8080",
			OpenAPI: &config.OpenAPIConfig{
				Source: "http://localhost:8080/openapi.json",
			},
		},
		{
			Name:     "graphql-api",
			Type:     "graphql",
			Endpoint: "http://localhost:8081/graphql",
		},
	}

	logger := observability.NewLogger(config.LoggingConfig{Level: "info"})
	resources := buildResourcesFromUpstreams(upstreams, logger.Logger)

	// Should have 3 resources: 2 upstreams + 1 OpenAPI spec
	if len(resources) != 3 {
		t.Errorf("Expected 3 resources, got %d", len(resources))
	}

	// Check upstream resources
	foundUpstream := false
	foundOpenAPI := false
	for _, r := range resources {
		if r.URI == "upstream://users-api" {
			foundUpstream = true
		}
		if r.URI == "openapi://users-api" {
			foundOpenAPI = true
		}
	}

	if !foundUpstream {
		t.Error("Expected upstream resource for users-api")
	}
	if !foundOpenAPI {
		t.Error("Expected OpenAPI resource for users-api")
	}
}

// TestBuildResourcesEmpty tests resource building with empty upstreams.
func TestBuildResourcesEmpty(t *testing.T) {
	logger := observability.NewLogger(config.LoggingConfig{Level: "info"})
	resources := buildResourcesFromUpstreams([]config.UpstreamConfig{}, logger.Logger)

	if len(resources) != 0 {
		t.Errorf("Expected 0 resources, got %d", len(resources))
	}
}
