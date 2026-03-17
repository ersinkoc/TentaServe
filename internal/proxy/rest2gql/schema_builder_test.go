package rest2gql

import (
	"testing"

	"github.com/ersinkoc/tentaserve/internal/openapi"
	schemapkg "github.com/ersinkoc/tentaserve/internal/schema"
)

func TestNewSchemaBuilder(t *testing.T) {
	opts := SchemaBuilderOptions{TypePrefix: "Test"}
	builder := NewSchemaBuilder(opts)

	if builder == nil {
		t.Fatal("NewSchemaBuilder returned nil")
	}
	if builder.prefix != "Test" {
		t.Errorf("Expected prefix 'Test', got '%s'", builder.prefix)
	}
	if builder.registry == nil {
		t.Error("Expected registry to be initialized")
	}
	if builder.mapper == nil {
		t.Error("Expected mapper to be initialized")
	}
}

func TestSchemaBuilder_Build_EmptySpec(t *testing.T) {
	builder := NewSchemaBuilder(SchemaBuilderOptions{})

	// Test nil spec
	_, err := builder.Build(nil)
	if err == nil {
		t.Error("Expected error for nil spec")
	}

	// Test empty spec
	emptySpec := &openapi.OpenAPISpec{
		OpenAPI: "3.0.0",
		Info:    openapi.Info{Title: "Test API", Version: "1.0.0"},
		Paths:   make(map[string]*openapi.PathItem),
	}

	schemaDef, err := builder.Build(emptySpec)
	if err != nil {
		t.Errorf("Build failed: %v", err)
	}
	if schemaDef == nil {
		t.Fatal("Expected schemaDef, got nil")
	}
	if schemaDef.Query != nil {
		t.Error("Expected no Query type for empty spec")
	}
	if schemaDef.Mutation != nil {
		t.Error("Expected no Mutation type for empty spec")
	}
}

func TestSchemaBuilder_Build_QueryFields(t *testing.T) {
	builder := NewSchemaBuilder(SchemaBuilderOptions{})

	spec := &openapi.OpenAPISpec{
		OpenAPI: "3.0.0",
		Info:    openapi.Info{Title: "Test API", Version: "1.0.0"},
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
											Ref: "#/components/schemas/User",
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

	schemaDef, err := builder.Build(spec)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if schemaDef.Query == nil {
		t.Fatal("Expected Query type")
	}
	if len(schemaDef.Query.Fields) != 1 {
		t.Errorf("Expected 1 query field, got %d", len(schemaDef.Query.Fields))
	}

	field := schemaDef.Query.Fields[0]
	if field.Name != "users" {
		t.Errorf("Expected field name 'users', got '%s'", field.Name)
	}
}

func TestSchemaBuilder_Build_MutationFields(t *testing.T) {
	builder := NewSchemaBuilder(SchemaBuilderOptions{})

	spec := &openapi.OpenAPISpec{
		OpenAPI: "3.0.0",
		Info:    openapi.Info{Title: "Test API", Version: "1.0.0"},
		Paths: map[string]*openapi.PathItem{
			"/users": {
				Post: &openapi.Operation{
					Summary:     "Create user",
					RequestBody: &openapi.RequestBody{
						Required: true,
						Content: map[string]*openapi.MediaType{
							"application/json": {
								Schema: &openapi.SchemaObject{
									Ref: "#/components/schemas/UserInput",
								},
							},
						},
					},
					Responses: map[string]*openapi.Response{
						"201": {
							Description: "Created",
							Content: map[string]*openapi.MediaType{
								"application/json": {
									Schema: &openapi.SchemaObject{
										Ref: "#/components/schemas/User",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	schemaDef, err := builder.Build(spec)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if schemaDef.Mutation == nil {
		t.Fatal("Expected Mutation type")
	}
	if len(schemaDef.Mutation.Fields) != 1 {
		t.Errorf("Expected 1 mutation field, got %d", len(schemaDef.Mutation.Fields))
	}

	field := schemaDef.Mutation.Fields[0]
	if field.Name != "createUsers" {
		t.Errorf("Expected field name 'createUsers', got '%s'", field.Name)
	}
}

func TestSchemaBuilder_Build_ObjectType(t *testing.T) {
	builder := NewSchemaBuilder(SchemaBuilderOptions{})

	spec := &openapi.OpenAPISpec{
		OpenAPI: "3.0.0",
		Info:    openapi.Info{Title: "Test API", Version: "1.0.0"},
		Components: &openapi.Components{
			Schemas: map[string]*openapi.SchemaObject{
				"User": {
					Type:        "object",
					Description: "A user",
					Properties: map[string]*openapi.SchemaObject{
						"id": {
							Type:   "string",
							Format: "uuid",
						},
						"name": {
							Type: "string",
						},
						"age": {
							Type: "integer",
						},
					},
					Required: []string{"id", "name"},
				},
			},
		},
		Paths: make(map[string]*openapi.PathItem),
	}

	schemaDef, err := builder.Build(spec)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Check if User type was registered
	userType, ok := schemaDef.GetType("User")
	if !ok {
		t.Fatal("Expected User type to be registered")
	}
	if userType.Kind != schemapkg.TypeKindObject {
		t.Errorf("Expected User to be Object kind, got %s", userType.Kind)
	}
	if len(userType.Fields) != 3 {
		t.Errorf("Expected 3 fields, got %d", len(userType.Fields))
	}
}

func TestSchemaBuilder_Build_EnumType(t *testing.T) {
	builder := NewSchemaBuilder(SchemaBuilderOptions{})

	spec := &openapi.OpenAPISpec{
		OpenAPI: "3.0.0",
		Info:    openapi.Info{Title: "Test API", Version: "1.0.0"},
		Components: &openapi.Components{
			Schemas: map[string]*openapi.SchemaObject{
				"Status": {
					Type:        "string",
					Description: "User status",
					Enum:        []interface{}{"active", "inactive", "pending"},
				},
			},
		},
		Paths: make(map[string]*openapi.PathItem),
	}

	schemaDef, err := builder.Build(spec)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Check if Status type was registered
	statusType, ok := schemaDef.GetType("Status")
	if !ok {
		t.Fatal("Expected Status type to be registered")
	}
	if statusType.Kind != schemapkg.TypeKindEnum {
		t.Errorf("Expected Status to be Enum kind, got %s", statusType.Kind)
	}
	if len(statusType.EnumValues) != 3 {
		t.Errorf("Expected 3 enum values, got %d", len(statusType.EnumValues))
	}
}

func TestSchemaBuilder_Build_WithPrefix(t *testing.T) {
	builder := NewSchemaBuilder(SchemaBuilderOptions{TypePrefix: "UsersApi"})

	spec := &openapi.OpenAPISpec{
		OpenAPI: "3.0.0",
		Info:    openapi.Info{Title: "Test API", Version: "1.0.0"},
		Components: &openapi.Components{
			Schemas: map[string]*openapi.SchemaObject{
				"User": {
					Type: "object",
					Properties: map[string]*openapi.SchemaObject{
						"id": {Type: "string"},
					},
				},
			},
		},
		Paths: make(map[string]*openapi.PathItem),
	}

	schemaDef, err := builder.Build(spec)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Check if type has prefix
	if _, ok := schemaDef.GetType("UsersApiUser"); !ok {
		t.Error("Expected UsersApiUser type to be registered with prefix")
	}
}

func TestSchemaBuilder_buildField_Required(t *testing.T) {
	builder := NewSchemaBuilder(SchemaBuilderOptions{})

	// Test required field
	prop := &openapi.SchemaObject{
		Type:        "string",
		Description: "User name",
	}
	required := []string{"name"}

	field, err := builder.buildField("name", prop, required)
	if err != nil {
		t.Fatalf("buildField failed: %v", err)
	}

	if field.Name != "name" {
		t.Errorf("Expected field name 'name', got '%s'", field.Name)
	}
	if !field.Type.IsNonNull() {
		t.Error("Expected required field to be NonNull type")
	}
}

func TestSchemaBuilder_buildField_Optional(t *testing.T) {
	builder := NewSchemaBuilder(SchemaBuilderOptions{})

	// Test optional field
	prop := &openapi.SchemaObject{
		Type:        "string",
		Description: "User bio",
	}
	required := []string{"name"} // bio not in required

	field, err := builder.buildField("bio", prop, required)
	if err != nil {
		t.Fatalf("buildField failed: %v", err)
	}

	if field.Type.IsNonNull() {
		t.Error("Expected optional field to not be NonNull type")
	}
}

func TestPathToFieldName(t *testing.T) {
	builder := NewSchemaBuilder(SchemaBuilderOptions{})

	tests := []struct {
		path          string
		operationType string
		expected      string
	}{
		{"/users", "query", "users"},
		{"/users/{id}", "query", "usersId"},
		{"/user-profiles", "query", "userProfiles"},
		{"/users/{id}/posts", "query", "usersIdPosts"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := builder.pathToFieldName(tt.path, nil, tt.operationType)
			if got != tt.expected {
				t.Errorf("pathToFieldName(%q, %q) = %q, want %q",
					tt.path, tt.operationType, got, tt.expected)
			}
		})
	}
}

func TestBuildArgument(t *testing.T) {
	builder := NewSchemaBuilder(SchemaBuilderOptions{})

	// Path parameter (required)
	pathParam := &openapi.Parameter{
		Name:     "id",
		In:       "path",
		Required: true,
		Schema: &openapi.SchemaObject{
			Type: "string",
		},
	}

	arg := builder.buildArgument(pathParam)
	if arg == nil {
		t.Fatal("Expected argument for path param")
	}
	if arg.Name != "id" {
		t.Errorf("Expected arg name 'id', got '%s'", arg.Name)
	}
	if !arg.Type.IsNonNull() {
		t.Error("Expected path param to be NonNull")
	}

	// Query parameter (optional)
	queryParam := &openapi.Parameter{
		Name: "limit",
		In:   "query",
		Schema: &openapi.SchemaObject{
			Type: "integer",
		},
	}

	arg = builder.buildArgument(queryParam)
	if arg == nil {
		t.Fatal("Expected argument for query param")
	}
	if arg.Name != "limit" {
		t.Errorf("Expected arg name 'limit', got '%s'", arg.Name)
	}
	if arg.Type.IsNonNull() {
		t.Error("Expected query param to not be NonNull")
	}
}
