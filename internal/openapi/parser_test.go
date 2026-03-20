package openapi

import (
	"fmt"
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
							"type":     "string",
							"format":   "email",
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

// --- Additional parser tests for coverage ---

func TestParse_NilSpec(t *testing.T) {
	_, err := Parse(nil)
	if err == nil {
		t.Fatal("expected error for nil spec")
	}
}

func TestParse_NoPaths(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
	}

	spec, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if spec.Paths == nil {
		t.Error("Paths should be initialized even when missing from data")
	}
}

func TestParse_InfoWithContactAndLicense(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":          "Test API",
			"version":        "1.0.0",
			"description":    "A test API",
			"termsOfService": "https://example.com/tos",
			"contact": map[string]any{
				"name":  "Support",
				"url":   "https://example.com/support",
				"email": "support@example.com",
			},
			"license": map[string]any{
				"name": "MIT",
				"url":  "https://opensource.org/licenses/MIT",
			},
		},
		"paths": map[string]any{},
	}

	spec := mustParse(t, data)

	if spec.Info.Description != "A test API" {
		t.Errorf("expected description 'A test API', got %s", spec.Info.Description)
	}
	if spec.Info.TermsOfService != "https://example.com/tos" {
		t.Errorf("expected termsOfService, got %s", spec.Info.TermsOfService)
	}
	if spec.Info.Contact == nil {
		t.Fatal("expected contact")
	}
	if spec.Info.Contact.Name != "Support" {
		t.Errorf("expected contact name 'Support', got %s", spec.Info.Contact.Name)
	}
	if spec.Info.Contact.URL != "https://example.com/support" {
		t.Errorf("expected contact URL, got %s", spec.Info.Contact.URL)
	}
	if spec.Info.Contact.Email != "support@example.com" {
		t.Errorf("expected contact email, got %s", spec.Info.Contact.Email)
	}
	if spec.Info.License == nil {
		t.Fatal("expected license")
	}
	if spec.Info.License.Name != "MIT" {
		t.Errorf("expected license name 'MIT', got %s", spec.Info.License.Name)
	}
}

func TestParse_LicenseMissingName(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
			"license": map[string]any{
				"url": "https://example.com",
			},
		},
		"paths": map[string]any{},
	}

	_, err := Parse(data)
	if err == nil {
		t.Fatal("expected error for license missing name")
	}
}

func TestParse_OperationWithTags(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{
			"/users": map[string]any{
				"get": map[string]any{
					"tags":        []any{"users", "admin"},
					"operationId": "listUsers",
					"deprecated":  true,
					"responses":   map[string]any{"200": map[string]any{"description": "OK"}},
				},
			},
		},
	}

	spec := mustParse(t, data)
	op := spec.Paths["/users"].Get

	if len(op.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(op.Tags))
	}
	if !op.Deprecated {
		t.Error("expected deprecated=true")
	}
}

func TestParse_ParameterWithRef(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{
			"/users/{id}": map[string]any{
				"get": map[string]any{
					"parameters": []any{
						map[string]any{
							"$ref": "#/components/parameters/UserId",
						},
					},
					"responses": map[string]any{"200": map[string]any{"description": "OK"}},
				},
			},
		},
	}

	spec := mustParse(t, data)
	param := spec.Paths["/users/{id}"].Get.Parameters[0]
	if param.Ref != "#/components/parameters/UserId" {
		t.Errorf("expected $ref, got %s", param.Ref)
	}
}

func TestParse_RequestBodyWithRef(t *testing.T) {
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
						"$ref": "#/components/requestBodies/CreateUser",
					},
					"responses": map[string]any{"201": map[string]any{"description": "Created"}},
				},
			},
		},
	}

	spec := mustParse(t, data)
	body := spec.Paths["/users"].Post.RequestBody
	if body.Ref != "#/components/requestBodies/CreateUser" {
		t.Errorf("expected $ref on request body, got %s", body.Ref)
	}
}

func TestParse_ResponseWithRef(t *testing.T) {
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
							"$ref": "#/components/responses/UserList",
						},
					},
				},
			},
		},
	}

	spec := mustParse(t, data)
	resp := spec.Paths["/users"].Get.Responses["200"]
	if resp.Ref != "#/components/responses/UserList" {
		t.Errorf("expected $ref on response, got %s", resp.Ref)
	}
}

func TestParse_SchemaWithRef(t *testing.T) {
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
					"$ref": "#/components/schemas/Animal",
				},
			},
		},
	}

	spec := mustParse(t, data)
	if spec.Components.Schemas["Pet"].Ref != "#/components/schemas/Animal" {
		t.Error("expected $ref on schema")
	}
}

func TestParse_SchemaAnyOf(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{},
		"components": map[string]any{
			"schemas": map[string]any{
				"PetOrError": map[string]any{
					"anyOf": []any{
						map[string]any{"type": "object"},
						map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	spec := mustParse(t, data)
	schema := spec.Components.Schemas["PetOrError"]
	if len(schema.AnyOf) != 2 {
		t.Errorf("expected 2 anyOf, got %d", len(schema.AnyOf))
	}
}

func TestParse_SchemaNotField(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{},
		"components": map[string]any{
			"schemas": map[string]any{
				"NotEmpty": map[string]any{
					"not": map[string]any{"type": "null"},
				},
			},
		},
	}

	spec := mustParse(t, data)
	schema := spec.Components.Schemas["NotEmpty"]
	if schema.Not == nil {
		t.Error("expected not field to be parsed")
	}
}

func TestParse_SchemaNumericConstraints(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{},
		"components": map[string]any{
			"schemas": map[string]any{
				"Rating": map[string]any{
					"type":             "number",
					"minimum":          float64(0),
					"maximum":          float64(5),
					"exclusiveMinimum": true,
					"exclusiveMaximum": true,
					"multipleOf":       float64(0.5),
				},
			},
		},
	}

	spec := mustParse(t, data)
	schema := spec.Components.Schemas["Rating"]
	if schema.Minimum != 0 {
		t.Errorf("expected minimum 0, got %f", schema.Minimum)
	}
	if schema.Maximum != 5 {
		t.Errorf("expected maximum 5, got %f", schema.Maximum)
	}
	if !schema.ExclusiveMinimum {
		t.Error("expected exclusiveMinimum=true")
	}
	if !schema.ExclusiveMaximum {
		t.Error("expected exclusiveMaximum=true")
	}
	if schema.MultipleOf != 0.5 {
		t.Errorf("expected multipleOf 0.5, got %f", schema.MultipleOf)
	}
}

func TestParse_SchemaStringConstraints(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{},
		"components": map[string]any{
			"schemas": map[string]any{
				"Username": map[string]any{
					"type":          "string",
					"minLength":     3,
					"maxLength":     50,
					"pattern":       "^[a-z]+$",
					"maxItems":      10,
					"minItems":      1,
					"maxProperties": 20,
					"minProperties": 1,
					"uniqueItems":   true,
					"readOnly":      true,
					"writeOnly":     false,
					"deprecated":    true,
				},
			},
		},
	}

	spec := mustParse(t, data)
	schema := spec.Components.Schemas["Username"]
	if schema.MinLength != 3 {
		t.Errorf("expected minLength 3, got %d", schema.MinLength)
	}
	if schema.MaxLength != 50 {
		t.Errorf("expected maxLength 50, got %d", schema.MaxLength)
	}
	if schema.Pattern != "^[a-z]+$" {
		t.Errorf("expected pattern, got %s", schema.Pattern)
	}
	if !schema.UniqueItems {
		t.Error("expected uniqueItems=true")
	}
	if !schema.ReadOnly {
		t.Error("expected readOnly=true")
	}
	if schema.WriteOnly {
		t.Error("expected writeOnly=false")
	}
	if !schema.Deprecated {
		t.Error("expected deprecated=true")
	}
}

func TestParse_PathItemWithRef(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{
			"/users": map[string]any{
				"$ref": "#/paths/~1other",
			},
		},
	}

	spec := mustParse(t, data)
	if spec.Paths["/users"].Ref != "#/paths/~1other" {
		t.Error("expected $ref on path item")
	}
}

func TestParse_ComponentsWithResponses(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{},
		"components": map[string]any{
			"responses": map[string]any{
				"NotFound": map[string]any{
					"description": "Resource not found",
				},
			},
			"parameters": map[string]any{
				"PageSize": map[string]any{
					"name": "page_size",
					"in":   "query",
					"schema": map[string]any{
						"type": "integer",
					},
				},
			},
			"requestBodies": map[string]any{
				"UserInput": map[string]any{
					"description": "User input body",
					"required":    true,
				},
			},
		},
	}

	spec := mustParse(t, data)
	if spec.Components.Responses["NotFound"] == nil {
		t.Error("expected NotFound response in components")
	}
	if spec.Components.Parameters["PageSize"] == nil {
		t.Error("expected PageSize parameter in components")
	}
	if spec.Components.RequestBodies["UserInput"] == nil {
		t.Error("expected UserInput request body in components")
	}
}

func TestParse_ParameterMissingName(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{
			"/test": map[string]any{
				"get": map[string]any{
					"parameters": []any{
						map[string]any{
							"in": "query",
						},
					},
					"responses": map[string]any{"200": map[string]any{"description": "OK"}},
				},
			},
		},
	}

	_, err := Parse(data)
	if err == nil {
		t.Fatal("expected error for parameter missing name")
	}
}

func TestParse_ParameterMissingIn(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{
			"/test": map[string]any{
				"get": map[string]any{
					"parameters": []any{
						map[string]any{
							"name": "id",
						},
					},
					"responses": map[string]any{"200": map[string]any{"description": "OK"}},
				},
			},
		},
	}

	_, err := Parse(data)
	if err == nil {
		t.Fatal("expected error for parameter missing in")
	}
}

func TestSchemaObject_IsArray(t *testing.T) {
	tests := []struct {
		schema   *SchemaObject
		expected bool
	}{
		{&SchemaObject{Type: "array"}, true},
		{&SchemaObject{Type: "object"}, false},
		{&SchemaObject{Type: "string"}, false},
		{nil, false},
	}
	for _, tc := range tests {
		if got := tc.schema.IsArray(); got != tc.expected {
			typeName := ""
			if tc.schema != nil {
				typeName = tc.schema.Type
			}
			t.Errorf("IsArray() = %v, want %v for type %s", got, tc.expected, typeName)
		}
	}
}

func TestSchemaObject_IsObject(t *testing.T) {
	tests := []struct {
		schema   *SchemaObject
		expected bool
	}{
		{&SchemaObject{Type: "object"}, true},
		{&SchemaObject{Type: "array"}, false},
		{nil, false},
	}
	for _, tc := range tests {
		if got := tc.schema.IsObject(); got != tc.expected {
			t.Errorf("IsObject() = %v, want %v", got, tc.expected)
		}
	}
}

func TestSchemaObject_GetEffectiveType(t *testing.T) {
	tests := []struct {
		schema   *SchemaObject
		expected string
	}{
		{&SchemaObject{Type: "string"}, "string"},
		{&SchemaObject{Type: ""}, ""},
		{nil, ""},
	}
	for _, tc := range tests {
		if got := tc.schema.GetEffectiveType(); got != tc.expected {
			t.Errorf("GetEffectiveType() = %q, want %q", got, tc.expected)
		}
	}
}

func TestParseError_WithAndWithoutPath(t *testing.T) {
	e1 := &ParseError{Path: "info.title", Message: "missing"}
	if e1.Error() != "parse error at info.title: missing" {
		t.Errorf("unexpected error string: %s", e1.Error())
	}

	e2 := &ParseError{Message: "general error"}
	if e2.Error() != "parse error: general error" {
		t.Errorf("unexpected error string: %s", e2.Error())
	}

	e3 := &ParseError{Message: "wrapped", Cause: fmt.Errorf("root cause")}
	if e3.Unwrap() == nil {
		t.Error("expected Unwrap to return cause")
	}
}

func TestValidationError_WithAndWithoutPath(t *testing.T) {
	e1 := &ValidationError{Path: "info.title", Message: "required"}
	if e1.Error() != "validation error at info.title: required" {
		t.Errorf("unexpected error string: %s", e1.Error())
	}

	e2 := &ValidationError{Message: "general"}
	if e2.Error() != "validation error: general" {
		t.Errorf("unexpected error string: %s", e2.Error())
	}
}

func TestValidateSpec(t *testing.T) {
	tests := []struct {
		name     string
		spec     *OpenAPISpec
		wantErrs int
	}{
		{
			name: "valid spec",
			spec: &OpenAPISpec{
				OpenAPI: "3.0.0",
				Info:    Info{Title: "Test", Version: "1.0"},
				Paths:   map[string]*PathItem{},
			},
			wantErrs: 0,
		},
		{
			name: "missing version",
			spec: &OpenAPISpec{
				Info:  Info{Title: "Test", Version: "1.0"},
				Paths: map[string]*PathItem{},
			},
			wantErrs: 1,
		},
		{
			name: "unsupported version",
			spec: &OpenAPISpec{
				OpenAPI: "2.0.0",
				Info:    Info{Title: "Test", Version: "1.0"},
				Paths:   map[string]*PathItem{},
			},
			wantErrs: 1,
		},
		{
			name: "missing title",
			spec: &OpenAPISpec{
				OpenAPI: "3.0.0",
				Info:    Info{Version: "1.0"},
				Paths:   map[string]*PathItem{},
			},
			wantErrs: 1,
		},
		{
			name: "missing info version",
			spec: &OpenAPISpec{
				OpenAPI: "3.0.0",
				Info:    Info{Title: "Test"},
				Paths:   map[string]*PathItem{},
			},
			wantErrs: 1,
		},
		{
			name: "duplicate operation IDs",
			spec: &OpenAPISpec{
				OpenAPI: "3.0.0",
				Info:    Info{Title: "Test", Version: "1.0"},
				Paths: map[string]*PathItem{
					"/a": {Get: &Operation{OperationID: "dup"}},
					"/b": {Get: &Operation{OperationID: "dup"}},
				},
			},
			wantErrs: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := ValidateSpec(tc.spec)
			if len(errs) != tc.wantErrs {
				t.Errorf("ValidateSpec() returned %d errors, want %d: %v", len(errs), tc.wantErrs, errs)
			}
		})
	}
}

func TestDetectSourceType(t *testing.T) {
	tests := []struct {
		source   string
		expected sourceType
	}{
		{"https://api.example.com/spec.json", sourceURL},
		{"http://localhost/spec.yaml", sourceURL},
		{"spec.json", sourceFile},
		{"spec.yaml", sourceFile},
		{"spec.yml", sourceFile},
		{`{"openapi":"3.0.0"}`, sourceInline},
	}

	for _, tc := range tests {
		t.Run(tc.source, func(t *testing.T) {
			got := detectSourceType(tc.source)
			if got != tc.expected {
				t.Errorf("detectSourceType(%q) = %d, want %d", tc.source, got, tc.expected)
			}
		})
	}
}

func TestDetectFormatFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"spec.json", "json"},
		{"spec.yaml", "yaml"},
		{"spec.yml", "yaml"},
		{"spec.txt", "yaml"},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got := detectFormatFromPath(tc.path)
			if got != tc.expected {
				t.Errorf("detectFormatFromPath(%q) = %q, want %q", tc.path, got, tc.expected)
			}
		})
	}
}

func TestDetectFormatFromContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{"json object", `{"openapi": "3.0.0"}`, "json"},
		{"json array", `[1,2,3]`, "json"},
		{"yaml content", "openapi: 3.0.0\n", "yaml"},
		{"empty", "", "yaml"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := detectFormatFromContent([]byte(tc.content))
			if got != tc.expected {
				t.Errorf("detectFormatFromContent(%q) = %q, want %q", tc.content, got, tc.expected)
			}
		})
	}
}

func TestMediaTypeWithExample(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{
			"/test": map[string]any{
				"get": map[string]any{
					"responses": map[string]any{
						"200": map[string]any{
							"description": "OK",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"type": "string",
									},
									"example": "hello",
								},
							},
						},
					},
				},
			},
		},
	}

	spec := mustParse(t, data)
	mt := spec.Paths["/test"].Get.Responses["200"].Content["application/json"]
	if mt.Example != "hello" {
		t.Errorf("expected example 'hello', got %v", mt.Example)
	}
}

func TestParse_ParameterWithDeprecatedAndAllowEmpty(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{
			"/test": map[string]any{
				"get": map[string]any{
					"parameters": []any{
						map[string]any{
							"name":            "q",
							"in":              "query",
							"deprecated":      true,
							"allowEmptyValue": true,
							"description":     "search query",
						},
					},
					"responses": map[string]any{"200": map[string]any{"description": "OK"}},
				},
			},
		},
	}

	spec := mustParse(t, data)
	param := spec.Paths["/test"].Get.Parameters[0]
	if !param.Deprecated {
		t.Error("expected deprecated=true")
	}
	if !param.AllowEmptyValue {
		t.Error("expected allowEmptyValue=true")
	}
	if param.Description != "search query" {
		t.Errorf("expected description 'search query', got %s", param.Description)
	}
}

func TestParse_ServerMissingURL(t *testing.T) {
	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"servers": []any{
			map[string]any{
				"description": "No URL",
			},
		},
		"paths": map[string]any{},
	}

	_, err := Parse(data)
	if err == nil {
		t.Fatal("expected error for server missing URL")
	}
}

func TestParseContent_JSON(t *testing.T) {
	content := `{
		"openapi": "3.0.0",
		"info": {"title": "Test", "version": "1.0"},
		"paths": {}
	}`
	spec, err := parseContent([]byte(content), "json")
	if err != nil {
		t.Fatalf("parseContent JSON failed: %v", err)
	}
	if spec.OpenAPI != "3.0.0" {
		t.Errorf("expected OpenAPI 3.0.0, got %s", spec.OpenAPI)
	}
}

func TestParseContent_InvalidJSON(t *testing.T) {
	_, err := parseContent([]byte(`{invalid`), "json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseContent_UnknownFormat(t *testing.T) {
	_, err := parseContent([]byte(`test`), "xml")
	if err == nil {
		t.Error("expected error for unknown format")
	}
}

func TestParseContent_YAML(t *testing.T) {
	// Note: paths: {} is flow syntax not supported by our custom parser,
	// but an empty paths block should work as omission
	content := `
openapi: "3.0.0"
info:
  title: "Test"
  version: "1.0.0"
paths:
  /test:
    get:
      operationId: "getTest"
      responses:
        "200":
          description: "OK"
`
	spec, err := parseContent([]byte(content), "yaml")
	if err != nil {
		t.Fatalf("parseContent YAML failed: %v", err)
	}
	if spec.OpenAPI != "3.0.0" {
		t.Errorf("expected OpenAPI 3.0.0, got %s", spec.OpenAPI)
	}
}

func TestDefaultLoadOptions(t *testing.T) {
	opts := DefaultLoadOptions()
	if !opts.ResolveRefs {
		t.Error("expected ResolveRefs=true")
	}
	if !opts.AllowFileLoading {
		t.Error("expected AllowFileLoading=true")
	}
	if !opts.AllowURLLoading {
		t.Error("expected AllowURLLoading=true")
	}
}

func TestLoadOpenAPISpec_InlineJSON(t *testing.T) {
	content := `{"openapi":"3.0.0","info":{"title":"Test","version":"1.0"},"paths":{}}`
	spec, err := LoadFromInlineJSON(content)
	if err != nil {
		t.Fatalf("LoadFromInlineJSON failed: %v", err)
	}
	if spec.Info.Title != "Test" {
		t.Errorf("expected title 'Test', got %s", spec.Info.Title)
	}
}

func TestLoadOpenAPISpecWithOptions_URLDisabled(t *testing.T) {
	opts := &LoadOptions{
		AllowURLLoading: false,
	}
	_, err := LoadOpenAPISpecWithOptions("https://example.com/spec.json", opts)
	if err == nil {
		t.Error("expected error for disabled URL loading")
	}
}

func TestLoadOpenAPISpecWithOptions_FileDisabled(t *testing.T) {
	opts := &LoadOptions{
		AllowFileLoading: false,
	}
	_, err := LoadOpenAPISpecWithOptions("spec.json", opts)
	if err == nil {
		t.Error("expected error for disabled file loading")
	}
}

func TestDetectFormatFromURL(t *testing.T) {
	got := detectFormatFromURL("https://example.com/spec.json")
	if got != "json" {
		t.Errorf("expected json, got %s", got)
	}
	got = detectFormatFromURL("https://example.com/spec.yaml")
	if got != "yaml" {
		t.Errorf("expected yaml, got %s", got)
	}
}

func TestHelperFunctions_EdgeCases(t *testing.T) {
	// getString with non-string value
	data := map[string]any{"key": 42}
	_, ok := getString(data, "key")
	if ok {
		t.Error("getString should return false for non-string value")
	}

	// getBool with non-bool value
	_, ok = getBool(data, "key")
	if ok {
		t.Error("getBool should return false for non-bool value")
	}

	// getInt with string value
	data2 := map[string]any{"key": "123"}
	v, ok := getInt(data2, "key")
	if !ok || v != 123 {
		t.Errorf("getInt with string should work, got %d, ok=%v", v, ok)
	}

	// getFloat with string value
	data3 := map[string]any{"key": "3.14"}
	f, ok := getFloat(data3, "key")
	if !ok || f != 3.14 {
		t.Errorf("getFloat with string should work, got %f, ok=%v", f, ok)
	}

	// getInt with invalid string
	data4 := map[string]any{"key": "notanum"}
	_, ok = getInt(data4, "key")
	if ok {
		t.Error("getInt should return false for non-numeric string")
	}

	// getFloat with invalid string
	_, ok = getFloat(data4, "key")
	if ok {
		t.Error("getFloat should return false for non-numeric string")
	}

	// getMap with non-map value
	_, ok = getMap(data, "key")
	if ok {
		t.Error("getMap should return false for non-map value")
	}

	// getSlice with non-slice value
	_, ok = getSlice(data, "key")
	if ok {
		t.Error("getSlice should return false for non-slice value")
	}

	// missing keys
	empty := map[string]any{}
	_, ok = getString(empty, "missing")
	if ok {
		t.Error("expected false for missing key")
	}
	_, ok = getBool(empty, "missing")
	if ok {
		t.Error("expected false for missing key")
	}
	_, ok = getInt(empty, "missing")
	if ok {
		t.Error("expected false for missing key")
	}
}
