package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ersinkoc/tentaserve/internal/openapi"
	"github.com/ersinkoc/tentaserve/internal/schema"
)

// TestNewToolRegistry tests registry creation.
func TestNewToolRegistry(t *testing.T) {
	r := NewToolRegistry(nil)
	if r == nil {
		t.Fatal("Expected non-nil registry")
	}
	if r.tools == nil {
		t.Error("Expected tools map to be initialized")
	}
	if r.nameGen == nil {
		t.Error("Expected name generator to be initialized")
	}
}

// TestToolRegistryRegister tests tool registration.
func TestToolRegistryRegister(t *testing.T) {
	r := NewToolRegistry(nil)

	tool := &Tool{
		Name:        "test_tool",
		Description: "Test tool",
		Upstream:    "test-api",
	}

	if err := r.Register(tool); err != nil {
		t.Fatalf("Failed to register: %v", err)
	}

	if r.Count() != 1 {
		t.Errorf("Expected 1 tool, got %d", r.Count())
	}

	got := r.Get("test_tool")
	if got == nil {
		t.Fatal("Expected to get tool")
	}
	if got.Name != "test_tool" {
		t.Errorf("Expected name test_tool, got %s", got.Name)
	}
}

// TestToolRegistryRegisterNil tests nil tool registration.
func TestToolRegistryRegisterNil(t *testing.T) {
	r := NewToolRegistry(nil)
	err := r.Register(nil)
	if err == nil {
		t.Error("Expected error for nil tool")
	}
}

// TestToolRegistryRegisterEmptyName tests empty name registration.
func TestToolRegistryRegisterEmptyName(t *testing.T) {
	r := NewToolRegistry(nil)
	err := r.Register(&Tool{Name: ""})
	if err == nil {
		t.Error("Expected error for empty name")
	}
}

// TestToolRegistryUnregister tests tool unregistration.
func TestToolRegistryUnregister(t *testing.T) {
	r := NewToolRegistry(nil)
	r.Register(&Tool{Name: "test_tool"})

	r.Unregister("test_tool")

	if r.Get("test_tool") != nil {
		t.Error("Expected tool to be unregistered")
	}
}

// TestToolRegistryList tests listing tools.
func TestToolRegistryList(t *testing.T) {
	r := NewToolRegistry(nil)
	r.Register(&Tool{Name: "tool_b"})
	r.Register(&Tool{Name: "tool_a"})
	r.Register(&Tool{Name: "tool_c"})

	tools := r.List()

	if len(tools) != 3 {
		t.Fatalf("Expected 3 tools, got %d", len(tools))
	}

	// Should be sorted alphabetically
	if tools[0].Name != "tool_a" {
		t.Errorf("Expected first tool_a, got %s", tools[0].Name)
	}
	if tools[1].Name != "tool_b" {
		t.Errorf("Expected second tool_b, got %s", tools[1].Name)
	}
	if tools[2].Name != "tool_c" {
		t.Errorf("Expected third tool_c, got %s", tools[2].Name)
	}
}

// TestToolRegistryClear tests clearing registry.
func TestToolRegistryClear(t *testing.T) {
	r := NewToolRegistry(nil)
	r.Register(&Tool{Name: "tool_a"})
	r.Register(&Tool{Name: "tool_b"})

	r.Clear()

	if r.Count() != 0 {
		t.Errorf("Expected 0 tools, got %d", r.Count())
	}
}

// TestToolRegistryExcludePatterns tests exclude patterns.
func TestToolRegistryExcludePatterns(t *testing.T) {
	r := NewToolRegistry(nil)
	r.SetExcludePatterns([]string{"internal"})

	// This should be excluded
	if !r.shouldExclude("my_internal_tool") {
		t.Error("Expected internal tool to be excluded")
	}

	// This should not be excluded
	if r.shouldExclude("my_public_tool") {
		t.Error("Expected public tool not to be excluded")
	}
}

// TestToolRegistryBuildFromSchema tests building from unified schema.
func TestToolRegistryBuildFromSchema(t *testing.T) {
	r := NewToolRegistry(nil)

	// Create a simple schema
	schemaDef := schema.NewSchemaDefinition()
	schemaDef.Query = &schema.OperationDef{
		Name: "Query",
		Type: "query",
		Fields: []*schema.FieldDef{
			{
				Name:        "getUser",
				Description: "Get a user by ID",
				Type:        schema.NamedType("User"),
				Arguments: []*schema.ArgumentDef{
					{
						Name:         "id",
						Description:  "User ID",
						Type:         schema.IDType(),
						Required:     true,
						DefaultValue: nil,
					},
				},
			},
		},
	}

	err := r.BuildFromSchema(schemaDef, "users-api")
	if err != nil {
		t.Fatalf("Failed to build tools: %v", err)
	}

	if r.Count() != 1 {
		t.Errorf("Expected 1 tool, got %d", r.Count())
	}

	tool := r.Get("users_api_get_user")
	if tool == nil {
		t.Fatal("Expected to find tool")
	}
	if tool.Upstream != "users-api" {
		t.Errorf("Expected upstream users-api, got %s", tool.Upstream)
	}
}

// TestToolRegistryBuildFromOpenAPI tests building from OpenAPI.
func TestToolRegistryBuildFromOpenAPI(t *testing.T) {
	r := NewToolRegistry(nil)

	// Create a simple OpenAPI spec
	spec := &openapi.OpenAPISpec{
		OpenAPI: "3.0.0",
		Info: openapi.Info{
			Title:   "Users API",
			Version: "1.0.0",
		},
		Paths: map[string]*openapi.PathItem{
			"/users": {
				Get: &openapi.Operation{
					OperationID: "listUsers",
					Summary:     "List all users",
					Parameters:  []*openapi.Parameter{},
					Responses:   map[string]*openapi.Response{},
				},
				Post: &openapi.Operation{
					OperationID: "createUser",
					Summary:     "Create a new user",
					Parameters:  []*openapi.Parameter{},
					Responses:   map[string]*openapi.Response{},
				},
			},
		},
	}

	err := r.BuildFromOpenAPI(spec, "users-api")
	if err != nil {
		t.Fatalf("Failed to build tools: %v", err)
	}

	if r.Count() != 2 {
		t.Errorf("Expected 2 tools, got %d", r.Count())
	}

	// Check that tools were created with operation IDs
	if r.Get("users_api_list_users") == nil {
		t.Error("Expected list_users tool")
	}
	if r.Get("users_api_create_user") == nil {
		t.Error("Expected create_user tool")
	}
}

// TestToolRegistryBuildFromOpenAPIWithoutOperationID tests building without operationId.
func TestToolRegistryBuildFromOpenAPIWithoutOperationID(t *testing.T) {
	r := NewToolRegistry(nil)

	spec := &openapi.OpenAPISpec{
		OpenAPI: "3.0.0",
		Info: openapi.Info{
			Title:   "Users API",
			Version: "1.0.0",
		},
		Paths: map[string]*openapi.PathItem{
			"/users/{id}": {
				Get: &openapi.Operation{
					Summary:    "Get user",
					Parameters: []*openapi.Parameter{},
					Responses:  map[string]*openapi.Response{},
				},
			},
		},
	}

	err := r.BuildFromOpenAPI(spec, "users-api")
	if err != nil {
		t.Fatalf("Failed to build tools: %v", err)
	}

	if r.Count() != 1 {
		t.Errorf("Expected 1 tool, got %d", r.Count())
	}

	// Tool should be named from path
	tool := r.List()[0]
	if !strings.Contains(tool.Name, "users") {
		t.Errorf("Expected name to contain 'users', got %s", tool.Name)
	}
}

// TestGenerateInputSchema tests input schema generation.
func TestGenerateInputSchema(t *testing.T) {
	args := []*schema.ArgumentDef{
		{
			Name:         "id",
			Description:  "User ID",
			Type:         schema.IDType(),
			Required:     true,
			DefaultValue: nil,
		},
		{
			Name:         "name",
			Description:  "User name",
			Type:         schema.StringType(),
			Required:     false,
			DefaultValue: nil,
		},
	}

	schemaDef := schema.NewSchemaDefinition()
	data, err := GenerateInputSchema(args, schemaDef)
	if err != nil {
		t.Fatalf("Failed to generate schema: %v", err)
	}

	var schema JSONSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("Failed to parse schema: %v", err)
	}

	if schema.Type != "object" {
		t.Errorf("Expected type object, got %s", schema.Type)
	}
	if len(schema.Properties) != 2 {
		t.Errorf("Expected 2 properties, got %d", len(schema.Properties))
	}
	if len(schema.Required) != 1 {
		t.Errorf("Expected 1 required field, got %d", len(schema.Required))
	}
	if schema.Required[0] != "id" {
		t.Errorf("Expected 'id' to be required, got %s", schema.Required[0])
	}
}

// TestGenerateInputSchemaEmpty tests empty input schema.
func TestGenerateInputSchemaEmpty(t *testing.T) {
	schemaDef := schema.NewSchemaDefinition()
	data, err := GenerateInputSchema([]*schema.ArgumentDef{}, schemaDef)
	if err != nil {
		t.Fatalf("Failed to generate schema: %v", err)
	}

	var schema JSONSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("Failed to parse schema: %v", err)
	}

	if schema.Type != "object" {
		t.Errorf("Expected type object, got %s", schema.Type)
	}
}

// TestGenerateInputSchemaFromOpenAPI tests OpenAPI input schema.
func TestGenerateInputSchemaFromOpenAPI(t *testing.T) {
	params := []*openapi.Parameter{
		{
			Name:        "page",
			Description: "Page number",
			Required:    false,
			Schema: &openapi.SchemaObject{
				Type:    "integer",
				Default: 1,
			},
		},
	}

	data, err := GenerateInputSchemaFromOpenAPI(params, nil)
	if err != nil {
		t.Fatalf("Failed to generate schema: %v", err)
	}

	var schema JSONSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("Failed to parse schema: %v", err)
	}

	if len(schema.Properties) != 1 {
		t.Errorf("Expected 1 property, got %d", len(schema.Properties))
	}
}

// TestTypeRefToJSONSchema tests type conversion.
func TestTypeRefToJSONSchema(t *testing.T) {
	tests := []struct {
		name     string
		typ      *schema.TypeRef
		expected string
	}{
		{"string", schema.StringType(), "string"},
		{"int", schema.IntType(), "integer"},
		{"float", schema.FloatType(), "number"},
		{"bool", schema.BoolType(), "boolean"},
		{"id", schema.IDType(), "string"},
		{"list", schema.ListType(schema.StringType()), "array"},
		{"null", nil, "null"},
	}

	schemaDef := schema.NewSchemaDefinition()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := typeRefToJSONSchema(tt.typ, schemaDef)
			if schema.Type != tt.expected {
				t.Errorf("Expected type %s, got %s", tt.expected, schema.Type)
			}
		})
	}
}

// TestOpenAPISchemaToJSONSchema tests OpenAPI to JSON Schema conversion.
func TestOpenAPISchemaToJSONSchema(t *testing.T) {
	tests := []struct {
		name     string
		schema   *openapi.SchemaObject
		expected string
	}{
		{
			name:     "string",
			schema:   &openapi.SchemaObject{Type: "string"},
			expected: "string",
		},
		{
			name:     "integer",
			schema:   &openapi.SchemaObject{Type: "integer"},
			expected: "integer",
		},
		{
			name:     "number",
			schema:   &openapi.SchemaObject{Type: "number"},
			expected: "number",
		},
		{
			name:     "boolean",
			schema:   &openapi.SchemaObject{Type: "boolean"},
			expected: "boolean",
		},
		{
			name:     "nil",
			schema:   nil,
			expected: "object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := openAPISchemaToJSONSchema(tt.schema)
			if result.Type != tt.expected {
				t.Errorf("Expected type %s, got %s", tt.expected, result.Type)
			}
		})
	}
}

// TestOpenAPISchemaToJSONSchemaEnum tests enum handling.
func TestOpenAPISchemaToJSONSchemaEnum(t *testing.T) {
	schema := &openapi.SchemaObject{
		Type: "string",
		Enum: []interface{}{"active", "inactive", "pending"},
	}

	result := openAPISchemaToJSONSchema(schema)
	if result.Type != "string" {
		t.Errorf("Expected type string, got %s", result.Type)
	}
	if len(result.Enum) != 3 {
		t.Errorf("Expected 3 enum values, got %d", len(result.Enum))
	}
}

// TestOpenAPISchemaToJSONSchemaArray tests array handling.
func TestOpenAPISchemaToJSONSchemaArray(t *testing.T) {
	schema := &openapi.SchemaObject{
		Type: "array",
		Items: &openapi.SchemaObject{
			Type: "string",
		},
	}

	result := openAPISchemaToJSONSchema(schema)
	if result.Type != "array" {
		t.Errorf("Expected type array, got %s", result.Type)
	}
	if result.Items == nil {
		t.Fatal("Expected items schema")
	}
	if result.Items.Type != "string" {
		t.Errorf("Expected items type string, got %s", result.Items.Type)
	}
}

// TestOpenAPISchemaToJSONSchemaObject tests object handling.
func TestOpenAPISchemaToJSONSchemaObject(t *testing.T) {
	schema := &openapi.SchemaObject{
		Type: "object",
		Properties: map[string]*openapi.SchemaObject{
			"name": {Type: "string"},
			"age":  {Type: "integer"},
		},
	}

	result := openAPISchemaToJSONSchema(schema)
	if result.Type != "object" {
		t.Errorf("Expected type object, got %s", result.Type)
	}
	if len(result.Properties) != 2 {
		t.Errorf("Expected 2 properties, got %d", len(result.Properties))
	}
}

// TestGenerateToolDescription tests description generation.
func TestGenerateToolDescription(t *testing.T) {
	tests := []struct {
		upstream  string
		summary   string
		operation string
		expected  string
	}{
		{"api", "Get user", "GET /users", "[api] Get user"},
		{"", "Get user", "GET /users", "Get user"},
		{"api", "", "GET /users", "[api] GET /users"},
		{"api", "", "", "[api]"},
		{"", "", "", ""},
	}

	for _, tt := range tests {
		got := GenerateToolDescription(tt.upstream, tt.summary, tt.operation)
		if got != tt.expected {
			t.Errorf("GenerateToolDescription(%q, %q, %q) = %q, want %q",
				tt.upstream, tt.summary, tt.operation, got, tt.expected)
		}
	}
}

// TestToolStructure tests Tool struct JSON marshaling.
func TestToolStructure(t *testing.T) {
	tool := &Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: []byte(`{"type":"object"}`),
	}

	data, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("Failed to marshal tool: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if result["name"] != "test_tool" {
		t.Errorf("Expected name test_tool, got %v", result["name"])
	}
	if result["description"] != "A test tool" {
		t.Errorf("Expected description 'A test tool', got %v", result["description"])
	}
}
