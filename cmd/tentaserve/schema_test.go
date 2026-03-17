package main

import (
	"strings"
	"testing"

	"github.com/ersinkoc/tentaserve/internal/config"
	"github.com/ersinkoc/tentaserve/internal/schema"
)

// TestSchemaCmd tests the schema command.
func TestSchemaCmd(t *testing.T) {
	// Create a minimal config
	cfg := &config.Config{
		Upstreams: []config.UpstreamConfig{
			{
				Name:    "users-api",
				Type:    "rest",
				BaseURL: "http://localhost:8080",
			},
		},
	}

	// Build schema
	s, err := buildSchemaFromConfig(cfg, "", nil)
	if err != nil {
		t.Fatalf("Failed to build schema: %v", err)
	}

	if s == nil {
		t.Fatal("Expected non-nil schema")
	}

	// Check that Query type exists
	if s.Query == nil {
		t.Error("Expected Query operation to be set")
	}

	// Check that upstream type was added
	if _, ok := s.GetType("users-apiQuery"); !ok {
		t.Error("Expected users-apiQuery type to be added")
	}
}

// TestSchemaCmdWithUpstreamFilter tests the schema command with upstream filter.
func TestSchemaCmdWithUpstreamFilter(t *testing.T) {
	cfg := &config.Config{
		Upstreams: []config.UpstreamConfig{
			{
				Name:    "users-api",
				Type:    "rest",
				BaseURL: "http://localhost:8080",
			},
			{
				Name:     "products-api",
				Type:     "graphql",
				Endpoint: "http://localhost:8081/graphql",
			},
		},
	}

	// Build schema with filter
	s, err := buildSchemaFromConfig(cfg, "users-api", nil)
	if err != nil {
		t.Fatalf("Failed to build schema: %v", err)
	}

	// Should only have users-api
	if _, ok := s.GetType("users-apiQuery"); !ok {
		t.Error("Expected users-apiQuery type to be added")
	}

	if _, ok := s.GetType("products-apiQuery"); ok {
		t.Error("Expected products-apiQuery type to NOT be added when filtered")
	}
}

// TestSchemaToJSON tests the JSON conversion.
func TestSchemaToJSON(t *testing.T) {
	cfg := &config.Config{
		Upstreams: []config.UpstreamConfig{
			{
				Name:    "users-api",
				Type:    "rest",
				BaseURL: "http://localhost:8080",
			},
		},
	}

	s, err := buildSchemaFromConfig(cfg, "", nil)
	if err != nil {
		t.Fatalf("Failed to build schema: %v", err)
	}

	json, err := schemaToJSON(s)
	if err != nil {
		t.Fatalf("Failed to convert to JSON: %v", err)
	}

	// Check basic structure
	if !strings.Contains(json, "openapi") {
		t.Error("Expected JSON to contain 'openapi'")
	}

	if !strings.Contains(json, "Tentaserve API") {
		t.Error("Expected JSON to contain API title")
	}
}

// TestBuildRESTUpstreamSchema tests REST upstream schema building.
func TestBuildRESTUpstreamSchema(t *testing.T) {
	s := buildEmptySchema()
	upstream := config.UpstreamConfig{
		Name:    "test-api",
		Type:    "rest",
		BaseURL: "http://localhost:8080",
	}

	err := buildRESTUpstreamSchema(s, upstream, nil)
	if err != nil {
		t.Fatalf("Failed to build REST schema: %v", err)
	}

	// Check that type was added
	if _, ok := s.GetType("test-apiQuery"); !ok {
		t.Error("Expected test-apiQuery type to be added")
	}

	// Check that Query type has the field
	queryType, ok := s.GetType("Query")
	if !ok {
		t.Fatal("Expected Query type to exist")
	}

	found := false
	for _, field := range queryType.Fields {
		if field.Name == "test-api" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected Query type to have test-api field")
	}
}

// TestBuildGraphQLUpstreamSchema tests GraphQL upstream schema building.
func TestBuildGraphQLUpstreamSchema(t *testing.T) {
	s := buildEmptySchema()
	upstream := config.UpstreamConfig{
		Name:     "graphql-api",
		Type:     "graphql",
		Endpoint: "http://localhost:8081/graphql",
	}

	err := buildGraphQLUpstreamSchema(s, upstream)
	if err != nil {
		t.Fatalf("Failed to build GraphQL schema: %v", err)
	}

	// Check that type was added
	if _, ok := s.GetType("graphql-apiQuery"); !ok {
		t.Error("Expected graphql-apiQuery type to be added")
	}
}

// Helper function
func buildEmptySchema() *schema.SchemaDefinition {
	sd := schema.NewSchemaDefinition()
	// Add built-in scalars
	for _, t := range schema.BuiltinScalars() {
		sd.AddType(t)
	}
	return sd
}
