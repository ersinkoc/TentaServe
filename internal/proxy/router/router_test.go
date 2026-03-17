package router

import (
	"net/http"
	"testing"
)

func TestRequestTypeString(t *testing.T) {
	tests := []struct {
		reqType  RequestType
		expected string
	}{
		{TypeGraphQL, "graphql"},
		{TypeREST, "rest"},
		{TypeMCP, "mcp"},
		{TypeHealth, "health"},
		{TypeMetrics, "metrics"},
		{TypeAdmin, "admin"},
		{TypeUnknown, "unknown"},
		{RequestType(999), "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			if got := tc.reqType.String(); got != tc.expected {
				t.Errorf("RequestType.String() = %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestClassifyRequest_GraphQL(t *testing.T) {
	config := DefaultConfig()
	r := New(config)

	req, _ := http.NewRequest("POST", "/graphql", nil)
	result := r.ClassifyRequest(req)

	if result.Type != TypeGraphQL {
		t.Errorf("expected TypeGraphQL, got %s", result.Type)
	}
	if result.RawPath != "/graphql" {
		t.Errorf("expected RawPath=/graphql, got %s", result.RawPath)
	}
}

func TestClassifyRequest_MCP(t *testing.T) {
	config := DefaultConfig()
	r := New(config)

	// Test POST to MCP endpoint
	req, _ := http.NewRequest("POST", "/mcp", nil)
	result := r.ClassifyRequest(req)

	if result.Type != TypeMCP {
		t.Errorf("expected TypeMCP, got %s", result.Type)
	}
}

func TestClassifyRequest_Health(t *testing.T) {
	config := DefaultConfig()
	r := New(config)

	req, _ := http.NewRequest("GET", "/-/health", nil)
	result := r.ClassifyRequest(req)

	if result.Type != TypeHealth {
		t.Errorf("expected TypeHealth, got %s", result.Type)
	}
}

func TestClassifyRequest_Metrics(t *testing.T) {
	config := DefaultConfig()
	r := New(config)

	req, _ := http.NewRequest("GET", "/-/metrics", nil)
	result := r.ClassifyRequest(req)

	if result.Type != TypeMetrics {
		t.Errorf("expected TypeMetrics, got %s", result.Type)
	}
}

func TestClassifyRequest_Admin(t *testing.T) {
	config := DefaultConfig()
	r := New(config)

	tests := []struct {
		path     string
		expected RequestType
	}{
		{"/-admin", TypeUnknown}, // wrong prefix
		{"/-/admin", TypeAdmin},
		{"/-/admin/", TypeAdmin},
		{"/-/admin/dashboard", TypeAdmin},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			req, _ := http.NewRequest("GET", tc.path, nil)
			result := r.ClassifyRequest(req)
			if result.Type != tc.expected {
				t.Errorf("path %s: expected %s, got %s", tc.path, tc.expected, result.Type)
			}
		})
	}
}

func TestClassifyRequest_REST(t *testing.T) {
	config := &Config{
		GraphQLPath: "/graphql",
		MCPPath:     "/mcp",
		RESTPrefix:  "/api",
		HealthPath:  "/-/health",
		MetricsPath: "/-/metrics",
		Upstreams:   make(map[string]string),
	}
	r := New(config)

	tests := []struct {
		path      string
		wantType  RequestType
		wantClean string
	}{
		{"/api/users", TypeREST, "/users"},
		{"/api/users/123", TypeREST, "/users/123"},
		{"/api/users/123/posts", TypeREST, "/users/123/posts"},
		{"/api", TypeREST, "/"},
		{"/api/", TypeREST, "/"},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			req, _ := http.NewRequest("GET", tc.path, nil)
			result := r.ClassifyRequest(req)

			if result.Type != tc.wantType {
				t.Errorf("expected type %s, got %s", tc.wantType, result.Type)
			}
			if result.CleanPath != tc.wantClean {
				t.Errorf("expected CleanPath=%s, got %s", tc.wantClean, result.CleanPath)
			}
		})
	}
}

func TestClassifyRequest_Unknown(t *testing.T) {
	config := DefaultConfig()
	r := New(config)

	tests := []string{
		"/unknown",
		"/random/path",
		"//double-slash",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			req, _ := http.NewRequest("GET", path, nil)
			result := r.ClassifyRequest(req)
			if result.Type != TypeUnknown {
				t.Errorf("path %s: expected TypeUnknown, got %s", path, result.Type)
			}
		})
	}
}

func TestClassifyRequest_UpstreamRouting(t *testing.T) {
	config := &Config{
		GraphQLPath: "/graphql",
		MCPPath:     "/mcp",
		RESTPrefix:  "/api",
		HealthPath:  "/-/health",
		MetricsPath: "/-/metrics",
		Upstreams: map[string]string{
			"/users":       "users-service",
			"/users/admin": "admin-service",
			"/products":    "products-service",
			"/":            "default-service",
		},
	}
	r := New(config)

	tests := []struct {
		path             string
		expectedUpstream string
	}{
		{"/api/users", "users-service"},
		{"/api/users/123", "users-service"},
		{"/api/users/admin", "admin-service"}, // Longest prefix match
		{"/api/users/admin/settings", "admin-service"},
		{"/api/products", "products-service"},
		{"/api/products/456", "products-service"},
		{"/api/other", "default-service"},
		{"/api/", "default-service"},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			req, _ := http.NewRequest("GET", tc.path, nil)
			result := r.ClassifyRequest(req)

			if result.Type != TypeREST {
				t.Errorf("expected TypeREST, got %s", result.Type)
			}
			if result.Upstream != tc.expectedUpstream {
				t.Errorf("expected upstream %q, got %q", tc.expectedUpstream, result.Upstream)
			}
		})
	}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		method      string
		path        string
		expected    RequestType
	}{
		{"POST", "/graphql", TypeGraphQL},
		{"POST", "/mcp", TypeMCP},
		{"GET", "/-/health", TypeHealth},
		{"GET", "/-/metrics", TypeMetrics},
		{"GET", "/api/users", TypeREST},
		{"GET", "/unknown", TypeUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			result := Classify(tc.method, tc.path, "/graphql", "/mcp", "/-/health", "/-/metrics")
			if result != tc.expected {
				t.Errorf("Classify(%s, %s) = %s, want %s", tc.method, tc.path, result, tc.expected)
			}
		})
	}
}

func TestMatchUpstream(t *testing.T) {
	config := &Config{
		RESTPrefix: "/api",
		Upstreams: map[string]string{
			"/users":    "users-service",
			"/products": "products-service",
		},
	}
	r := New(config)

	tests := []struct {
		path     string
		expected string
	}{
		{"/api/users", "users-service"},
		{"/api/users/123", "users-service"},
		{"/api/products", "products-service"},
		{"/api/other", ""},
		{"/graphql", ""},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			result := r.MatchUpstream(tc.path)
			if result != tc.expected {
				t.Errorf("MatchUpstream(%s) = %q, want %q", tc.path, result, tc.expected)
			}
		})
	}
}

func TestNew_NilConfig(t *testing.T) {
	r := New(nil)
	if r == nil {
		t.Fatal("expected non-nil router")
	}
	if r.config == nil {
		t.Fatal("expected default config")
	}
	if r.config.GraphQLPath != "/graphql" {
		t.Errorf("expected default GraphQLPath, got %s", r.config.GraphQLPath)
	}
}

func TestClassifiedRequest_PathParams(t *testing.T) {
	// Ensure PathParams map is initialized
	config := DefaultConfig()
	r := New(config)

	req, _ := http.NewRequest("GET", "/api/users/123", nil)
	result := r.ClassifyRequest(req)

	if result.PathParams == nil {
		t.Fatal("expected PathParams to be initialized")
	}

	// Currently no path param extraction, but map should be usable
	result.PathParams["id"] = "123"
	if result.PathParams["id"] != "123" {
		t.Error("PathParams map should be usable")
	}
}
