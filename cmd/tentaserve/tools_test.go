package main

import (
	"testing"

	"github.com/ersinkoc/tentaserve/internal/config"
	"github.com/ersinkoc/tentaserve/internal/mcp"
)

// TestPrintToolsNames tests the printToolsNames function.
func TestPrintToolsNames(t *testing.T) {
	tools := []*mcp.Tool{
		{Name: "tool_a"},
		{Name: "tool_b"},
	}

	// Should not panic
	printToolsNames(tools)
}

// TestPrintToolsNamesEmpty tests printToolsNames with empty list.
func TestPrintToolsNamesEmpty(t *testing.T) {
	// Should not panic with empty list
	printToolsNames([]*mcp.Tool{})
}

// TestPrintToolsTable tests the printToolsTable function.
func TestPrintToolsTable(t *testing.T) {
	tools := []*mcp.Tool{
		{
			Name:        "users_api_get_user",
			Description: "Get user by ID",
			Upstream:    "users-api",
			Operation:   "GET /users/{id}",
		},
	}

	// Should not panic
	printToolsTable(tools)
}

// TestPrintToolsTableEmpty tests printToolsTable with empty list.
func TestPrintToolsTableEmpty(t *testing.T) {
	// Should not panic with empty list
	printToolsTable([]*mcp.Tool{})
}

// TestPrintToolsJSON tests the printToolsJSON function.
func TestPrintToolsJSON(t *testing.T) {
	tools := []*mcp.Tool{
		{
			Name:        "users_api_get_user",
			Description: "Get user by ID",
			Upstream:    "users-api",
			Operation:   "GET /users/{id}",
			InputSchema: []byte(`{"type":"object"}`),
		},
	}

	// Should not panic and return valid JSON
	err := printToolsJSON(tools)
	if err != nil {
		t.Errorf("printToolsJSON failed: %v", err)
	}
}

// TestPrintToolsJSONEmpty tests printToolsJSON with empty list.
func TestPrintToolsJSONEmpty(t *testing.T) {
	// Should not panic with empty list
	err := printToolsJSON([]*mcp.Tool{})
	if err != nil {
		t.Errorf("printToolsJSON failed: %v", err)
	}
}

// TestBuildToolsFromUpstreamUnknownType tests building tools from unknown upstream type.
func TestBuildToolsFromUpstreamUnknownType(t *testing.T) {
	registry := mcp.NewToolRegistry(nil)

	upstream := config.UpstreamConfig{
		Name: "test",
		Type: "unknown",
	}

	// Should not error, just skip
	err := buildToolsFromUpstream(registry, upstream)
	if err != nil {
		t.Errorf("buildToolsFromUpstream failed: %v", err)
	}
}
