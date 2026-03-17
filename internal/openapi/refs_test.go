package openapi

import (
	"strings"
	"testing"
	"time"
)

func TestResolver_SimpleRef(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{
			"/users": map[string]any{
				"get": map[string]any{
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Success",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"$ref": "#/components/schemas/User",
									},
								},
							},
						},
					},
				},
			},
		},
		"components": map[string]any{
			"schemas": map[string]any{
				"User": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":   map[string]any{"type": "integer"},
						"name": map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	spec, err := Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Before resolution, the schema has a $ref
	resp := spec.Paths["/users"].Get.Responses["200"]
	jsonContent := resp.Content["application/json"]
	if jsonContent.Schema.Ref != "#/components/schemas/User" {
		t.Fatal("Schema should have $ref before resolution")
	}

	// Resolve refs
	resolver := NewResolver(spec)
	if err := resolver.ResolveAll(); err != nil {
		t.Fatalf("Failed to resolve: %v", err)
	}

	// After resolution, the $ref should be replaced
	if jsonContent.Schema.Ref != "" {
		t.Error("$ref should be empty after resolution")
	}

	if jsonContent.Schema.Type != "object" {
		t.Errorf("Expected type 'object', got %s", jsonContent.Schema.Type)
	}

	if len(jsonContent.Schema.Properties) != 2 {
		t.Errorf("Expected 2 properties, got %d", len(jsonContent.Schema.Properties))
	}
}

func TestResolver_NestedRef(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{},
		"components": map[string]any{
			"schemas": map[string]any{
				"Address": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"street": map[string]any{"type": "string"},
						"city":   map[string]any{"type": "string"},
					},
				},
				"User": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id": map[string]any{"type": "integer"},
						"address": map[string]any{
							"$ref": "#/components/schemas/Address",
						},
					},
				},
			},
		},
	}

	spec, err := Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	resolver := NewResolver(spec)
	if err := resolver.ResolveAll(); err != nil {
		t.Fatalf("Failed to resolve: %v", err)
	}

	userSchema := spec.Components.Schemas["User"]
	addressProp := userSchema.Properties["address"]

	if addressProp.Ref != "" {
		t.Error("$ref should be resolved in nested property")
	}

	if addressProp.Type != "object" {
		t.Errorf("Expected address type 'object', got %s", addressProp.Type)
	}

	if addressProp.Properties["street"] == nil {
		t.Error("Address street property not found")
	}
}

func TestResolver_ArrayItemsRef(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{},
		"components": map[string]any{
			"schemas": map[string]any{
				"User": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":   map[string]any{"type": "integer"},
						"name": map[string]any{"type": "string"},
					},
				},
				"UsersResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"data": map[string]any{
							"type": "array",
							"items": map[string]any{
								"$ref": "#/components/schemas/User",
							},
						},
					},
				},
			},
		},
	}

	spec, err := Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	resolver := NewResolver(spec)
	if err := resolver.ResolveAll(); err != nil {
		t.Fatalf("Failed to resolve: %v", err)
	}

	responseSchema := spec.Components.Schemas["UsersResponse"]
	items := responseSchema.Properties["data"].Items

	if items.Ref != "" {
		t.Error("$ref in array items should be resolved")
	}

	if items.Properties["name"] == nil {
		t.Error("User name property not found in array items")
	}
}

func TestResolver_CircularRef(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{},
		"components": map[string]any{
			"schemas": map[string]any{
				"Node": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"value": map[string]any{"type": "string"},
						"child": map[string]any{
							"$ref": "#/components/schemas/Node",
						},
					},
				},
			},
		},
	}

	spec, err := Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	resolver := NewResolver(spec)

	// Circular refs should be handled gracefully without infinite loop
	// The resolver may return an error or handle it silently - both are acceptable
	// as long as it doesn't hang
	done := make(chan error, 1)
	go func() {
		done <- resolver.ResolveAll()
	}()

	select {
	case err := <-done:
		// If it returns, that's fine - error or nil both acceptable
		_ = err
	case <-time.After(2 * time.Second):
		t.Fatal("Resolver timed out - likely infinite loop on circular ref")
	}
}

func TestResolver_CacheReuse(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{
			"/users": map[string]any{
				"get": map[string]any{
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Success",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"$ref": "#/components/schemas/User",
									},
								},
							},
						},
					},
				},
			},
			"/users/{id}": map[string]any{
				"get": map[string]any{
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Success",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"$ref": "#/components/schemas/User",
									},
								},
							},
						},
					},
				},
			},
		},
		"components": map[string]any{
			"schemas": map[string]any{
				"User": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":   map[string]any{"type": "integer"},
						"name": map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	spec, err := Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	resolver := NewResolver(spec)
	if err := resolver.ResolveAll(); err != nil {
		t.Fatalf("Failed to resolve: %v", err)
	}

	// Both paths should have resolved to the same schema
	resp1 := spec.Paths["/users"].Get.Responses["200"]
	resp2 := spec.Paths["/users/{id}"].Get.Responses["200"]

	if resp1.Content["application/json"].Schema.Type != "object" {
		t.Error("First response schema not resolved")
	}

	if resp2.Content["application/json"].Schema.Type != "object" {
		t.Error("Second response schema not resolved")
	}
}

func TestResolver_ParameterRef(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{
			"/users/{id}": map[string]any{
				"parameters": []any{
					map[string]any{
						"$ref": "#/components/parameters/UserID",
					},
				},
				"get": map[string]any{
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Success",
						},
					},
				},
			},
		},
		"components": map[string]any{
			"parameters": map[string]any{
				"UserID": map[string]any{
					"name":     "id",
					"in":       "path",
					"required": true,
					"schema": map[string]any{
						"type": "string",
					},
				},
			},
		},
	}

	spec, err := Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	resolver := NewResolver(spec)
	if err := resolver.ResolveAll(); err != nil {
		t.Fatalf("Failed to resolve: %v", err)
	}

	params := spec.Paths["/users/{id}"].Parameters
	if len(params) != 1 {
		t.Fatalf("Expected 1 parameter, got %d", len(params))
	}

	param := params[0]
	if param.Ref != "" {
		t.Error("$ref should be resolved in parameter")
	}

	if param.Name != "id" {
		t.Errorf("Expected param name 'id', got %s", param.Name)
	}

	if param.In != "path" {
		t.Errorf("Expected param in 'path', got %s", param.In)
	}
}

func TestResolver_ResponseRef(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{
			"/users": map[string]any{
				"get": map[string]any{
					"responses": map[string]any{
						"200": map[string]any{
							"$ref": "#/components/responses/UsersList",
						},
					},
				},
			},
		},
		"components": map[string]any{
			"responses": map[string]any{
				"UsersList": map[string]any{
					"description": "List of users",
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{
								"type": "array",
								"items": map[string]any{
									"type": "object",
								},
							},
						},
					},
				},
			},
		},
	}

	spec, err := Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	resolver := NewResolver(spec)
	if err := resolver.ResolveAll(); err != nil {
		t.Fatalf("Failed to resolve: %v", err)
	}

	resp := spec.Paths["/users"].Get.Responses["200"]
	if resp.Ref != "" {
		t.Error("$ref should be resolved in response")
	}

	if resp.Description != "List of users" {
		t.Errorf("Expected description 'List of users', got %s", resp.Description)
	}

	jsonContent := resp.Content["application/json"]
	if jsonContent == nil {
		t.Fatal("Missing application/json content")
	}

	if jsonContent.Schema.Type != "array" {
		t.Errorf("Expected array schema, got %s", jsonContent.Schema.Type)
	}
}

func TestDecodeJSONPointer(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"path/to/item", "path/to/item"},
		{"path~1to~1item", "path/to/item"},
		{"path~0to~0item", "path~to~item"},
		{"path~1~0item", "path/~item"},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := decodeJSONPointer(tc.input)
			if result != tc.expected {
				t.Errorf("decodeJSONPointer(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestResolver_InvalidRef(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{
			"/users": map[string]any{
				"get": map[string]any{
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Success",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"$ref": "#/components/schemas/NonExistent",
									},
								},
							},
						},
					},
				},
			},
		},
		"components": map[string]any{
			"schemas": map[string]any{},
		},
	}

	spec, err := Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	resolver := NewResolver(spec)
	err = resolver.ResolveAll()

	if err == nil {
		t.Fatal("Expected error for invalid ref, got nil")
	}

	if !strings.Contains(err.Error(), "cannot resolve reference") {
		t.Errorf("Expected 'cannot resolve reference' error, got: %v", err)
	}
}

func TestResolver_ExternalRefNotSupported(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{
			"/users": map[string]any{
				"get": map[string]any{
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Success",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"$ref": "./external.yaml#/components/schemas/User",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	spec, err := Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	resolver := NewResolver(spec)
	err = resolver.ResolveAll()

	if err == nil {
		t.Fatal("Expected error for external ref, got nil")
	}

	if !strings.Contains(err.Error(), "external references not supported") {
		t.Errorf("Expected 'external references not supported' error, got: %v", err)
	}
}
