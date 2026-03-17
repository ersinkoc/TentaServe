package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/ersinkoc/tentaserve/internal/config"
	"github.com/ersinkoc/tentaserve/internal/gateway/metrics"
	"github.com/ersinkoc/tentaserve/internal/mcp"
)

// MCPServer holds the MCP server components.
type MCPServer struct {
	Server    *mcp.Server
	Transport *mcp.SSETransport
	Registry  *mcp.ToolRegistry
	Metrics   *mcp.Metrics
	Logger    *slog.Logger
}

// NewMCPServer creates a new MCP server with all components.
func NewMCPServer(cfg *config.Config, logger *slog.Logger, metricsReg *metrics.Registry) (*MCPServer, error) {
	if !cfg.MCP.Enabled {
		return nil, nil
	}

	// Create MCP metrics
	mcpMetrics := mcp.NewMetrics(metricsReg)

	// Create MCP server
	server := mcp.NewServer(logger, mcpMetrics)

	// Create tool registry
	registry := mcp.NewToolRegistry(logger)

	// Build tools from upstreams
	for _, upstream := range cfg.Upstreams {
		if err := buildToolsFromUpstream(registry, upstream); err != nil {
			logger.Warn("failed to build tools from upstream",
				slog.String("upstream", upstream.Name),
				slog.Any("error", err),
			)
			// Continue with other upstreams
		}
	}

	// Register tool handlers
	server.RegisterToolsHandlers(registry)

	// Build and register resources from upstreams
	resources := buildResourcesFromUpstreams(cfg.Upstreams, logger)
	if len(resources) > 0 {
		server.RegisterResourcesHandlers(resources)
	}

	// Update metrics with initial counts
	if mcpMetrics != nil {
		tools := registry.List()
		mcpMetrics.UpdateToolCount(len(tools))
		mcpMetrics.UpdateResourceCount(len(resources))

		logger.Info("MCP server initialized",
			slog.Int("tools", len(tools)),
			slog.Int("resources", len(resources)),
		)
	}

	// Create SSE transport
	transport := mcp.NewSSETransport(server, logger, cfg.Gateway.MCPPath)

	return &MCPServer{
		Server:    server,
		Transport: transport,
		Registry:  registry,
		Metrics:   mcpMetrics,
		Logger:    logger,
	}, nil
}

// RegisterHandlers registers MCP endpoints with the HTTP mux.
func (m *MCPServer) RegisterHandlers(mux *http.ServeMux, mcpPath string) {
	if m == nil {
		return
	}

	// SSE endpoint for MCP
	mux.HandleFunc("GET "+mcpPath, func(w http.ResponseWriter, r *http.Request) {
		m.Transport.ServeHTTP(w, r)
	})

	// POST endpoint for MCP messages
	mux.HandleFunc("POST "+mcpPath, func(w http.ResponseWriter, r *http.Request) {
		m.Transport.ServeHTTP(w, r)
	})

	m.Logger.Info("MCP endpoints registered",
		slog.String("path", mcpPath),
	)
}

// buildResourcesFromUpstreams creates MCP resources from upstream configurations.
func buildResourcesFromUpstreams(upstreams []config.UpstreamConfig, logger *slog.Logger) []*mcp.Resource {
	var resources []*mcp.Resource

	for _, upstream := range upstreams {
		// Only auto-expose resources if enabled in config
		// For now, we create a resource for each upstream
		resource := &mcp.Resource{
			URI:         fmt.Sprintf("upstream://%s", upstream.Name),
			Name:        upstream.Name,
			Description: fmt.Sprintf("Upstream service: %s (%s)", upstream.Name, upstream.Type),
			MIMEType:    "application/json",
		}
		resources = append(resources, resource)

		// If OpenAPI is configured, add a resource for the spec
		if upstream.OpenAPI != nil && upstream.OpenAPI.Source != "" {
			specResource := &mcp.Resource{
				URI:         fmt.Sprintf("openapi://%s", upstream.Name),
				Name:        fmt.Sprintf("%s-openapi", upstream.Name),
				Description: fmt.Sprintf("OpenAPI specification for %s", upstream.Name),
				MIMEType:    "application/json",
			}
			resources = append(resources, specResource)
		}
	}

	return resources
}

// Shutdown gracefully shuts down the MCP server.
func (m *MCPServer) Shutdown(ctx context.Context) error {
	if m == nil {
		return nil
	}

	m.Logger.Info("shutting down MCP server")

	// Close all SSE connections
	// The transport will clean up sessions when connections close

	return nil
}
