// Package router provides request classification and routing for Tentaserve.
//
// The router is responsible for:
//   - Classifying incoming requests (GraphQL, REST, MCP, health, metrics)
//   - Routing requests to the appropriate upstream based on path prefix matching
//   - Extracting path parameters from URL patterns
//
// Routing follows these rules:
//   - POST /graphql and configured GraphQL path → GraphQL request
//   - POST /mcp and configured MCP path → MCP request
//   - GET /-/health → Health check
//   - GET /-/metrics → Metrics endpoint
//   - Paths matching configured REST prefix → REST request (routed to upstream)
//
// Upstream routing uses longest prefix match. For example, with upstreams
// configured for "/api/users" and "/api", a request to "/api/users/123"
// routes to the "/api/users" upstream.
package router

import (
	"net/http"
	"strings"
)

// RequestType indicates the classification of an incoming request.
type RequestType int

const (
	// TypeUnknown indicates an unrecognized request type.
	TypeUnknown RequestType = iota
	// TypeGraphQL indicates a GraphQL query/mutation request.
	TypeGraphQL
	// TypeREST indicates a REST API request.
	TypeREST
	// TypeMCP indicates an MCP (Model Context Protocol) request.
	TypeMCP
	// TypeHealth indicates a health check request.
	TypeHealth
	// TypeMetrics indicates a metrics endpoint request.
	TypeMetrics
	// TypeAdmin indicates an admin dashboard request.
	TypeAdmin
)

// String returns a human-readable name for the request type.
func (t RequestType) String() string {
	switch t {
	case TypeGraphQL:
		return "graphql"
	case TypeREST:
		return "rest"
	case TypeMCP:
		return "mcp"
	case TypeHealth:
		return "health"
	case TypeMetrics:
		return "metrics"
	case TypeAdmin:
		return "admin"
	default:
		return "unknown"
	}
}

// ClassifiedRequest contains the classification result for a request.
type ClassifiedRequest struct {
	// Type is the classified request type.
	Type RequestType
	// Upstream is the name of the upstream to route to (for REST requests).
	Upstream string
	// PathParams contains extracted path parameters (for REST requests).
	PathParams map[string]string
	// RawPath is the original request path.
	RawPath string
	// CleanPath is the path with prefix removed (for upstream routing).
	CleanPath string
}

// Config holds router configuration.
type Config struct {
	// GraphQLPath is the path for GraphQL requests (e.g., "/graphql").
	GraphQLPath string
	// MCPPath is the path for MCP requests (e.g., "/mcp").
	MCPPath string
	// RESTPrefix is the prefix for REST API requests (e.g., "/api").
	RESTPrefix string
	// HealthPath is the path for health checks (e.g., "/-/health").
	HealthPath string
	// MetricsPath is the path for metrics (e.g., "/-/metrics").
	MetricsPath string
	// AdminPath is the path for admin dashboard (e.g., "/-/admin").
	AdminPath string
	// Upstreams maps path prefixes to upstream names.
	// Keys should be sorted by length (longest first) for proper matching.
	Upstreams map[string]string
}

// DefaultConfig returns a router config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		GraphQLPath: "/graphql",
		MCPPath:     "/mcp",
		RESTPrefix:  "/api",
		HealthPath:  "/-/health",
		MetricsPath: "/-/metrics",
		AdminPath:   "/-/admin",
		Upstreams:   make(map[string]string),
	}
}

// Router handles request classification and upstream routing.
type Router struct {
	config *Config
	// sortedPrefixes is a list of path prefixes sorted by length (longest first).
	sortedPrefixes []string
}

// New creates a new Router with the given configuration.
func New(config *Config) *Router {
	if config == nil {
		config = DefaultConfig()
	}

	// Sort upstream prefixes by length (longest first) for proper matching
	sorted := make([]string, 0, len(config.Upstreams))
	for prefix := range config.Upstreams {
		sorted = append(sorted, prefix)
	}

	// Simple bubble sort by length (descending)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if len(sorted[i]) < len(sorted[j]) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return &Router{
		config:         config,
		sortedPrefixes: sorted,
	}
}

// ClassifyRequest classifies an HTTP request and returns its type and routing info.
func (r *Router) ClassifyRequest(req *http.Request) *ClassifiedRequest {
	path := req.URL.Path

	result := &ClassifiedRequest{
		RawPath:    path,
		PathParams: make(map[string]string),
	}

	// Check for GraphQL endpoint
	if path == r.config.GraphQLPath {
		result.Type = TypeGraphQL
		return result
	}

	// Check for MCP endpoint (both POST and SSE)
	if path == r.config.MCPPath {
		result.Type = TypeMCP
		return result
	}

	// Check for health endpoint
	if path == r.config.HealthPath {
		result.Type = TypeHealth
		return result
	}

	// Check for metrics endpoint
	if path == r.config.MetricsPath {
		result.Type = TypeMetrics
		return result
	}

	// Check for admin endpoint
	if r.config.AdminPath != "" && (path == r.config.AdminPath || strings.HasPrefix(path, r.config.AdminPath+"/")) {
		result.Type = TypeAdmin
		return result
	}

	// Check for REST API requests
	if r.config.RESTPrefix != "" && strings.HasPrefix(path, r.config.RESTPrefix) {
		result.Type = TypeREST
		result.CleanPath = path[len(r.config.RESTPrefix):]
		if result.CleanPath == "" {
			result.CleanPath = "/"
		}

		// Find matching upstream using longest prefix match
		for _, prefix := range r.sortedPrefixes {
			if strings.HasPrefix(result.CleanPath, prefix) {
				result.Upstream = r.config.Upstreams[prefix]
				break
			}
		}

		return result
	}

	// Unknown request type
	result.Type = TypeUnknown
	return result
}

// Classify is a convenience function for one-off classification without upstream routing.
func Classify(method, path, graphqlPath, mcpPath, healthPath, metricsPath string) RequestType {
	r := New(&Config{
		GraphQLPath: graphqlPath,
		MCPPath:     mcpPath,
		HealthPath:  healthPath,
		MetricsPath: metricsPath,
		RESTPrefix:  "/api", // Default
		Upstreams:   make(map[string]string),
	})
	req, _ := http.NewRequest(method, path, nil)
	return r.ClassifyRequest(req).Type
}

// MatchUpstream finds the best matching upstream for a given path.
// Returns empty string if no upstream matches.
func (r *Router) MatchUpstream(path string) string {
	// Remove REST prefix if present
	cleanPath := path
	if strings.HasPrefix(path, r.config.RESTPrefix) {
		cleanPath = path[len(r.config.RESTPrefix):]
		if cleanPath == "" {
			cleanPath = "/"
		}
	}

	// Find longest matching prefix
	for _, prefix := range r.sortedPrefixes {
		if strings.HasPrefix(cleanPath, prefix) {
			return r.config.Upstreams[prefix]
		}
	}

	return ""
}
