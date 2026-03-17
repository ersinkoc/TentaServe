package openapi

import (
	"testing"
)

// Test helper to parse a simple spec
func mustParse(t *testing.T, data map[string]any) *OpenAPISpec {
	spec, err := Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}
	return spec
}

func TestParse_MinimalSpec(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{},
	}

	spec, err := Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse minimal spec: %v", err)
	}

	if spec.OpenAPI != "3.0.0" {
		t.Errorf("Expected OpenAPI version 3.0.0, got %s", spec.OpenAPI)
	}
	if spec.Info.Title != "Test API" {
		t.Errorf("Expected title 'Test API', got %s", spec.Info.Title)
	}
	if spec.Info.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", spec.Info.Version)
	}
}

func TestParse_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
		want string
	}{
		{
			name: "missing openapi version",
			data: map[string]any{
				"info": map[string]any{
					"title":   "Test",
					"version": "1.0.0",
				},
				"paths": map[string]any{},
			},
			want: "openapi version is required",
		},
		{
			name: "missing info",
			data: map[string]any{
				"openapi": "3.0.0",
				"paths":   map[string]any{},
			},
			want: "info is required",
		},
		{
			name: "missing info.title",
			data: map[string]any{
				"openapi": "3.0.0",
				"info": map[string]any{
					"version": "1.0.0",
				},
				"paths": map[string]any{},
			},
			want: "title is required",
		},
		{
			name: "missing info.version",
			data: map[string]any{
				"openapi": "3.0.0",
				"info": map[string]any{
					"title": "Test",
				},
				"paths": map[string]any{},
			},
			want: "version is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.data)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !contains(err.Error(), tc.want) {
				t.Errorf("Expected error containing %q, got %q", tc.want, err.Error())
			}
		})
	}
}

func TestParse_Paths(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{
			"/users": map[string]any{
				"get": map[string]any{
					"operationId": "getUsers",
					"summary":     "List users",
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Success",
						},
					},
				},
				"post": map[string]any{
					"operationId": "createUser",
					"summary":     "Create user",
					"responses": map[string]any{
						"201": map[string]any{
							"description": "Created",
						},
					},
				},
			},
			"/users/{id}": map[string]any{
				"get": map[string]any{
					"operationId": "getUser",
					"parameters": []any{
						map[string]any{
							"name":     "id",
							"in":       "path",
							"required": true,
							"schema": map[string]any{
								"type": "string",
							},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Success",
						},
					},
				},
			},
		},
	}

	spec := mustParse(t, data)

	if len(spec.Paths) != 2 {
		t.Errorf("Expected 2 paths, got %d", len(spec.Paths))
	}

	// Check /users path
	usersPath, ok := spec.Paths["/users"]
	if !ok {
		t.Fatal("Missing /users path")
	}

	if usersPath.Get == nil {
		t.Fatal("Missing GET operation on /users")
	}
	if usersPath.Get.OperationID != "getUsers" {
		t.Errorf("Expected operationId 'getUsers', got %s", usersPath.Get.OperationID)
	}

	if usersPath.Post == nil {
		t.Fatal("Missing POST operation on /users")
	}
	if usersPath.Post.OperationID != "createUser" {
		t.Errorf("Expected operationId 'createUser', got %s", usersPath.Post.OperationID)
	}

	// Check /users/{id} path
	userByIDPath, ok := spec.Paths["/users/{id}"]
	if !ok {
		t.Fatal("Missing /users/{id} path")
	}

	if userByIDPath.Get == nil {
		t.Fatal("Missing GET operation on /users/{id}")
	}

	if len(userByIDPath.Get.Parameters) != 1 {
		t.Errorf("Expected 1 parameter, got %d", len(userByIDPath.Get.Parameters))
	}

	param := userByIDPath.Get.Parameters[0]
	if param.Name != "id" {
		t.Errorf("Expected param name 'id', got %s", param.Name)
	}
	if param.In != "path" {
		t.Errorf("Expected param in 'path', got %s", param.In)
	}
	if !param.Required {
		t.Error("Expected param to be required")
	}
}

func TestParse_AllHTTPMethods(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{
			"/test": map[string]any{
				"get": map[string]any{
					"operationId": "testGet",
					"responses":   map[string]any{"200": map[string]any{"description": "OK"}},
				},
				"put": map[string]any{
					"operationId": "testPut",
					"responses":   map[string]any{"200": map[string]any{"description": "OK"}},
				},
				"post": map[string]any{
					"operationId": "testPost",
					"responses":   map[string]any{"200": map[string]any{"description": "OK"}},
				},
				"delete": map[string]any{
					"operationId": "testDelete",
					"responses":   map[string]any{"200": map[string]any{"description": "OK"}},
				},
				"patch": map[string]any{
					"operationId": "testPatch",
					"responses":   map[string]any{"200": map[string]any{"description": "OK"}},
				},
				"head": map[string]any{
					"operationId": "testHead",
					"responses":   map[string]any{"200": map[string]any{"description": "OK"}},
				},
				"options": map[string]any{
					"operationId": "testOptions",
					"responses":   map[string]any{"200": map[string]any{"description": "OK"}},
				},
				"trace": map[string]any{
					"operationId": "testTrace",
					"responses":   map[string]any{"200": map[string]any{"description": "OK"}},
				},
			},
		},
	}

	spec := mustParse(t, data)

	testPath := spec.Paths["/test"]
	if testPath.Get == nil || testPath.Get.OperationID != "testGet" {
		t.Error("GET not parsed correctly")
	}
	if testPath.Put == nil || testPath.Put.OperationID != "testPut" {
		t.Error("PUT not parsed correctly")
	}
	if testPath.Post == nil || testPath.Post.OperationID != "testPost" {
		t.Error("POST not parsed correctly")
	}
	if testPath.Delete == nil || testPath.Delete.OperationID != "testDelete" {
		t.Error("DELETE not parsed correctly")
	}
	if testPath.Patch == nil || testPath.Patch.OperationID != "testPatch" {
		t.Error("PATCH not parsed correctly")
	}
	if testPath.Head == nil || testPath.Head.OperationID != "testHead" {
		t.Error("HEAD not parsed correctly")
	}
	if testPath.Options == nil || testPath.Options.OperationID != "testOptions" {
		t.Error("OPTIONS not parsed correctly")
	}
	if testPath.Trace == nil || testPath.Trace.OperationID != "testTrace" {
		t.Error("TRACE not parsed correctly")
	}
}

func TestParse_Schemas(t *testing.T) {
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
					"type":        "object",
					"description": "A user",
					"required":    []any{"id", "name"},
					"properties": map[string]any{
						"id": map[string]any{
							"type":   "integer",
							"format": "int64",
						},
						"name": map[string]any{
							"type":      "string",
							"minLength": 1,
							"maxLength": 100,
						},
						"email": map[string]any{
							"type":    "string",
							"format":  "email",
							"nullable": true,
						},
						"tags": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
						},
					},
				},
				"Status": map[string]any{
					"type": "string",
					"enum": []any{"active", "inactive", "pending"},
				},
			},
		},
	}

	spec := mustParse(t, data)

	if spec.Components == nil {
		t.Fatal("Missing components")
	}

	if len(spec.Components.Schemas) != 2 {
		t.Errorf("Expected 2 schemas, got %d", len(spec.Components.Schemas))
	}

	// Check User schema
	userSchema, ok := spec.Components.Schemas["User"]
	if !ok {
		t.Fatal("Missing User schema")
	}

	if userSchema.Type != "object" {
		t.Errorf("Expected type 'object', got %s", userSchema.Type)
	}

	if len(userSchema.Required) != 2 {
		t.Errorf("Expected 2 required fields, got %d", len(userSchema.Required))
	}

	if len(userSchema.Properties) != 4 {
		t.Errorf("Expected 4 properties, got %d", len(userSchema.Properties))
	}

	// Check id property
	idProp := userSchema.Properties["id"]
	if idProp == nil || idProp.Type != "integer" {
		t.Error("id property not parsed correctly")
	}

	// Check tags array property
	tagsProp := userSchema.Properties["tags"]
	if tagsProp == nil || !tagsProp.IsArray() {
		t.Error("tags property should be an array")
	}
	if tagsProp.Items == nil || tagsProp.Items.Type != "string" {
		t.Error("tags items should be string")
	}

	// Check Status enum schema
	statusSchema, ok := spec.Components.Schemas["Status"]
	if !ok {
		t.Fatal("Missing Status schema")
	}

	if len(statusSchema.Enum) != 3 {
		t.Errorf("Expected 3 enum values, got %d", len(statusSchema.Enum))
	}
}

func TestParse_RequestBody(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{
			"/users": map[string]any{
				"post": map[string]any{
					"requestBody": map[string]any{
						"required": true,
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"name": map[string]any{
											"type": "string",
										},
									},
								},
							},
						},
					},
					"responses": map[string]any{
						"201": map[string]any{
							"description": "Created",
						},
					},
				},
			},
		},
	}

	spec := mustParse(t, data)

	op := spec.Paths["/users"].Post
	if op.RequestBody == nil {
		t.Fatal("Missing request body")
	}

	if !op.RequestBody.Required {
		t.Error("Request body should be required")
	}

	jsonContent, ok := op.RequestBody.Content["application/json"]
	if !ok {
		t.Fatal("Missing application/json content")
	}

	if jsonContent.Schema == nil {
		t.Fatal("Missing schema in content")
	}

	if jsonContent.Schema.Type != "object" {
		t.Errorf("Expected object schema, got %s", jsonContent.Schema.Type)
	}
}

func TestParse_CompositeSchemas(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{},
		"components": map[string]any{
			"schemas": map[string]any{
				"Pet": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{"type": "string"},
					},
				},
				"Dog": map[string]any{
					"allOf": []any{
						map[string]any{"$ref": "#/components/schemas/Pet"},
						map[string]any{
							"type": "object",
							"properties": map[string]any{
								"breed": map[string]any{"type": "string"},
							},
						},
					},
				},
				"Animal": map[string]any{
					"oneOf": []any{
						map[string]any{"$ref": "#/components/schemas/Pet"},
						map[string]any{"$ref": "#/components/schemas/Dog"},
					},
				},
			},
		},
	}

	spec := mustParse(t, data)

	dogSchema := spec.Components.Schemas["Dog"]
	if len(dogSchema.AllOf) != 2 {
		t.Errorf("Expected 2 allOf schemas, got %d", len(dogSchema.AllOf))
	}

	animalSchema := spec.Components.Schemas["Animal"]
	if len(animalSchema.OneOf) != 2 {
		t.Errorf("Expected 2 oneOf schemas, got %d", len(animalSchema.OneOf))
	}
}

func TestSchemaObject_IsPrimitive(t *testing.T) {
	tests := []struct {
		schema   *SchemaObject
		expected bool
	}{
		{&SchemaObject{Type: "string"}, true},
		{&SchemaObject{Type: "integer"}, true},
		{&SchemaObject{Type: "number"}, true},
		{&SchemaObject{Type: "boolean"}, true},
		{&SchemaObject{Type: "object"}, false},
		{&SchemaObject{Type: "array"}, false},
		{nil, false},
	}

	for _, tc := range tests {
		if got := tc.schema.IsPrimitive(); got != tc.expected {
			t.Errorf("IsPrimitive() = %v, want %v for type %s", got, tc.expected, tc.schema.Type)
		}
	}
}

func TestPathItem_GetOperations(t *testing.T) {
	pathItem := &PathItem{
		Get:  &Operation{OperationID: "get"},
		Post: &Operation{OperationID: "post"},
	}

	ops := pathItem.GetOperations()

	if len(ops) != 2 {
		t.Errorf("Expected 2 operations, got %d", len(ops))
	}

	if ops[MethodGet] == nil || ops[MethodGet].OperationID != "get" {
		t.Error("GET operation not found")
	}

	if ops[MethodPost] == nil || ops[MethodPost].OperationID != "post" {
		t.Error("POST operation not found")
	}

	if ops[MethodPut] != nil {
		t.Error("PUT should not be present")
	}
}

func TestParse_Servers(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"servers": []any{
			map[string]any{
				"url":         "https://api.example.com/v1",
				"description": "Production server",
			},
			map[string]any{
				"url": "https://{environment}.example.com:{port}/v1",
				"variables": map[string]any{
					"environment": map[string]any{
						"default": "api",
						"enum":    []any{"api", "staging"},
					},
					"port": map[string]any{
						"default": "443",
					},
				},
			},
		},
		"paths": map[string]any{},
	}

	spec := mustParse(t, data)

	if len(spec.Servers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(spec.Servers))
	}

	// Check first server
	if spec.Servers[0].URL != "https://api.example.com/v1" {
		t.Errorf("Expected URL 'https://api.example.com/v1', got %s", spec.Servers[0].URL)
	}

	// Check second server with variables
	if spec.Servers[1].Variables == nil {
		t.Fatal("Missing server variables")
	}

	envVar := spec.Servers[1].Variables["environment"]
	if envVar == nil || envVar.Default != "api" {
		t.Error("Environment variable not parsed correctly")
	}

	if len(envVar.Enum) != 2 {
		t.Errorf("Expected 2 enum values, got %d", len(envVar.Enum))
	}
}

func TestParse_Tags(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{},
		"tags": []any{
			map[string]any{
				"name":        "users",
				"description": "User operations",
			},
			map[string]any{
				"name": "products",
			},
		},
	}

	spec := mustParse(t, data)

	if len(spec.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(spec.Tags))
	}

	if spec.Tags[0].Name != "users" {
		t.Errorf("Expected tag name 'users', got %s", spec.Tags[0].Name)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsInternal(s, substr))
}

func containsInternal(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
