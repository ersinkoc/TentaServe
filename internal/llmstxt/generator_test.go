package llmstxt

import (
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	t.Run("full config", func(t *testing.T) {
		cfg := Config{
			Version:     "0.1.0",
			Host:        "0.0.0.0",
			Port:        8080,
			GraphQLPath: "/graphql",
			RESTPrefix:  "/api",
			MCPPath:     "/mcp",
			Upstreams: []Upstream{
				{Name: "users-api", Type: "rest", BaseURL: "http://localhost:3000"},
				{Name: "products", Type: "graphql", Endpoint: "http://localhost:4000/graphql"},
			},
			Tools: []Tool{
				{Name: "get_users", Description: "List all users", Upstream: "users-api"},
				{Name: "create_user", Description: "Create a new user", Upstream: "users-api", InputSchema: `{"type":"object"}`},
			},
		}

		result := Generate(cfg)

		// Check header
		if !strings.Contains(result, "# Tentaserve API Gateway") {
			t.Error("expected header")
		}
		if !strings.Contains(result, "Version: 0.1.0") {
			t.Error("expected version")
		}

		// Check endpoints
		if !strings.Contains(result, "POST http://0.0.0.0:8080/graphql") {
			t.Error("expected GraphQL endpoint")
		}
		if !strings.Contains(result, "http://0.0.0.0:8080/api/*") {
			t.Error("expected REST endpoint")
		}
		if !strings.Contains(result, "POST http://0.0.0.0:8080/mcp") {
			t.Error("expected MCP endpoint")
		}
		if !strings.Contains(result, "/-/health") {
			t.Error("expected health endpoint")
		}

		// Check upstreams
		if !strings.Contains(result, "users-api") {
			t.Error("expected users-api upstream")
		}
		if !strings.Contains(result, "products") {
			t.Error("expected products upstream")
		}

		// Check tools
		if !strings.Contains(result, "### get_users") {
			t.Error("expected get_users tool")
		}
		if !strings.Contains(result, "List all users") {
			t.Error("expected tool description")
		}
		if !strings.Contains(result, `{"type":"object"}`) {
			t.Error("expected input schema")
		}

		// Check usage examples
		if !strings.Contains(result, "curl -X POST") {
			t.Error("expected curl examples")
		}
	})

	t.Run("minimal config", func(t *testing.T) {
		cfg := Config{
			Version: "0.1.0",
			Port:    8080,
		}

		result := Generate(cfg)

		if !strings.Contains(result, "# Tentaserve API Gateway") {
			t.Error("expected header")
		}
		if !strings.Contains(result, "localhost:8080") {
			t.Error("expected localhost when host is empty")
		}
		// Should not contain upstream or tools sections
		if strings.Contains(result, "## Upstreams") {
			t.Error("should not contain upstreams section when empty")
		}
	})

	t.Run("no tools", func(t *testing.T) {
		cfg := Config{
			Version:     "0.1.0",
			Port:        8080,
			GraphQLPath: "/graphql",
		}

		result := Generate(cfg)

		if strings.Contains(result, "## MCP Tools") {
			t.Error("should not contain MCP Tools section when no tools")
		}
	})
}

func TestGenerateHandler(t *testing.T) {
	cfg := Config{
		Version: "0.1.0",
		Port:    8080,
	}

	handler := GenerateHandler(cfg)

	var buf strings.Builder
	handler(&buf)

	if !strings.Contains(buf.String(), "Tentaserve") {
		t.Error("expected handler to write llms.txt content")
	}
}
