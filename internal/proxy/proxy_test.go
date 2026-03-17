package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ersinkoc/tentaserve/internal/openapi"
	"github.com/ersinkoc/tentaserve/internal/upstream"
)

func TestNewProxy(t *testing.T) {
	p := NewProxy(ProxyOptions{
		BaseURL: "http://example.com",
	})

	if p == nil {
		t.Fatal("NewProxy returned nil")
	}
	if p.executor == nil {
		t.Error("Expected executor to be initialized")
	}
	if p.resolvers == nil {
		t.Error("Expected resolvers to be initialized")
	}
	if p.upstreams == nil {
		t.Error("Expected upstreams to be initialized")
	}
}

func TestProxy_RegisterUpstream(t *testing.T) {
	p := NewProxy(ProxyOptions{BaseURL: "http://api.example.com"})

	client, err := upstream.NewClient(upstream.ClientOptions{BaseURL: "http://api.example.com"})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	spec := &openapi.OpenAPISpec{
		OpenAPI: "3.0.0",
		Info:    openapi.Info{Title: "Test API", Version: "1.0.0"},
		Paths: map[string]*openapi.PathItem{
			"/users": {
				Get: &openapi.Operation{Summary: "List users"},
			},
			"/users/{id}": {
				Get: &openapi.Operation{Summary: "Get user"},
			},
		},
	}

	err = p.RegisterUpstream("test-api", client, spec)
	if err != nil {
		t.Fatalf("RegisterUpstream failed: %v", err)
	}

	// Check that upstream was registered
	if p.UpstreamCount() != 1 {
		t.Errorf("Expected 1 upstream, got %d", p.UpstreamCount())
	}

	// Check that resolvers were created
	if p.ResolverCount() != 2 {
		t.Errorf("Expected 2 resolvers, got %d", p.ResolverCount())
	}

	// Check we can get the upstream
	gotClient, ok := p.GetUpstream("test-api")
	if !ok {
		t.Error("Expected to find registered upstream")
	}
	if gotClient != client {
		t.Error("Got different client than registered")
	}
}

func TestProxy_RegisterUpstream_NoPaths(t *testing.T) {
	p := NewProxy(ProxyOptions{BaseURL: "http://api.example.com"})

	client, _ := upstream.NewClient(upstream.ClientOptions{BaseURL: "http://api.example.com"})

	spec := &openapi.OpenAPISpec{
		OpenAPI: "3.0.0",
		Info:    openapi.Info{Title: "Test API", Version: "1.0.0"},
		Paths:   map[string]*openapi.PathItem{},
	}

	err := p.RegisterUpstream("test-api", client, spec)
	if err == nil {
		t.Error("Expected error for empty paths")
	}
}

func TestProxy_ExecuteQuery(t *testing.T) {
	p := NewProxy(ProxyOptions{BaseURL: "http://api.example.com"})

	// Register a resolver directly on the executor
	p.executor.RegisterResolver("Query", "hello", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return "world", nil
	})

	result := p.ExecuteQuery(context.Background(), "query { hello }", nil)

	if result == nil {
		t.Fatal("ExecuteQuery returned nil")
	}

	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", result.Errors)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map data, got %T", result.Data)
	}

	if data["hello"] != "world" {
		t.Errorf("Expected hello='world', got %v", data["hello"])
	}
}

func TestProxy_ExecuteQuery_ParseError(t *testing.T) {
	p := NewProxy(ProxyOptions{})

	result := p.ExecuteQuery(context.Background(), "invalid { query", nil)

	if len(result.Errors) == 0 {
		t.Error("Expected parse error")
	}
}

func TestProxy_HandleGraphQL(t *testing.T) {
	p := NewProxy(ProxyOptions{})

	// Register a simple resolver
	p.executor.RegisterResolver("Query", "test", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return "success", nil
	})

	// Test GET request (should be rejected)
	req := httptest.NewRequest(http.MethodGet, "/graphql", nil)
	w := httptest.NewRecorder()
	p.HandleGraphQL(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", w.Code)
	}

	// Test valid POST request
	body, _ := json.Marshal(map[string]interface{}{
		"query": "{ test }",
	})

	req = httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w = httptest.NewRecorder()
	p.HandleGraphQL(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestProxy_GetSchema(t *testing.T) {
	p := NewProxy(ProxyOptions{})

	// Initially no schema
	if p.GetSchema() != nil {
		t.Error("Expected nil schema initially")
	}
}

func TestProxy_BuildSchema(t *testing.T) {
	p := NewProxy(ProxyOptions{})

	spec := &openapi.OpenAPISpec{
		OpenAPI: "3.0.0",
		Info: openapi.Info{
			Title:   "Test API",
			Version: "1.0.0",
		},
		Paths: map[string]*openapi.PathItem{
			"/users": {
				Get: &openapi.Operation{
					Summary: "List users",
					Responses: map[string]*openapi.Response{
						"200": {
							Description: "Success",
							Content: map[string]*openapi.MediaType{
								"application/json": {
									Schema: &openapi.SchemaObject{
										Type: "array",
										Items: &openapi.SchemaObject{
											Type: "object",
											Properties: map[string]*openapi.SchemaObject{
												"id":   {Type: "string"},
												"name": {Type: "string"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	schema, err := p.BuildSchema(spec)
	if err != nil {
		t.Fatalf("BuildSchema failed: %v", err)
	}

	if schema == nil {
		t.Fatal("Expected schema, got nil")
	}

	// Schema should now be available
	if p.GetSchema() != schema {
		t.Error("GetSchema should return the built schema")
	}
}

func TestProxy_GetExecutor(t *testing.T) {
	p := NewProxy(ProxyOptions{})
	exec := p.GetExecutor()

	if exec == nil {
		t.Error("Expected non-nil executor")
	}
}

func TestProxy_GetResolvers(t *testing.T) {
	p := NewProxy(ProxyOptions{})
	resolvers := p.GetResolvers()

	if resolvers == nil {
		t.Error("Expected non-nil resolvers")
	}
}

func TestProxy_UpstreamCount(t *testing.T) {
	p := NewProxy(ProxyOptions{})

	if p.UpstreamCount() != 0 {
		t.Errorf("Expected 0 upstreams, got %d", p.UpstreamCount())
	}

	// Register an upstream
	client, _ := upstream.NewClient(upstream.ClientOptions{BaseURL: "http://example.com"})
	spec := &openapi.OpenAPISpec{
		OpenAPI: "3.0.0",
		Info:    openapi.Info{Title: "Test", Version: "1.0"},
		Paths: map[string]*openapi.PathItem{
			"/test": {Get: &openapi.Operation{}},
		},
	}
	p.RegisterUpstream("test", client, spec)

	if p.UpstreamCount() != 1 {
		t.Errorf("Expected 1 upstream, got %d", p.UpstreamCount())
	}
}

func TestProxy_ResolverCount(t *testing.T) {
	p := NewProxy(ProxyOptions{})

	if p.ResolverCount() != 0 {
		t.Errorf("Expected 0 resolvers, got %d", p.ResolverCount())
	}
}

func TestParseFieldPath_Proxy(t *testing.T) {
	tests := []struct {
		path      string
		wantType  string
		wantField string
	}{
		{"Query.users", "Query", "users"},
		{"Mutation.createUser", "Mutation", "createUser"},
		{"Query", "", ""},
		{"", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			gotType, gotField := parseFieldPath(tt.path)
			if gotType != tt.wantType || gotField != tt.wantField {
				t.Errorf("parseFieldPath(%q) = (%q, %q), want (%q, %q)",
					tt.path, gotType, gotField, tt.wantType, tt.wantField)
			}
		})
	}
}
