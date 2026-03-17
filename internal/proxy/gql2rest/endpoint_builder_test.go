package gql2rest

import (
	"testing"

	"github.com/ersinkoc/tentaserve/internal/graphql"
)

func TestNewEndpointBuilder(t *testing.T) {
	builder := NewEndpointBuilder(EndpointBuilderOptions{
		BasePath:     "/api/v1",
		MutationsUse: "heuristic",
	})

	if builder == nil {
		t.Fatal("NewEndpointBuilder returned nil")
	}
	if builder.basePath != "/api/v1" {
		t.Errorf("Expected basePath '/api/v1', got '%s'", builder.basePath)
	}
	if builder.mutationsUse != "heuristic" {
		t.Errorf("Expected mutationsUse 'heuristic', got '%s'", builder.mutationsUse)
	}
}

func TestNewEndpointBuilder_Defaults(t *testing.T) {
	builder := NewEndpointBuilder(EndpointBuilderOptions{})

	if builder.basePath != "/api" {
		t.Errorf("Expected default basePath '/api', got '%s'", builder.basePath)
	}
	if builder.mutationsUse != "heuristic" {
		t.Errorf("Expected default mutationsUse 'heuristic', got '%s'", builder.mutationsUse)
	}
}

func TestEndpointBuilder_GenerateEndpoints(t *testing.T) {
	builder := NewEndpointBuilder(EndpointBuilderOptions{
		BasePath:     "/api",
		MutationsUse: "heuristic",
	})

	// Create a mock schema
	schema := &graphql.Schema{
		QueryType: &graphql.TypeRef{Name: "Query"},
		MutationType: &graphql.TypeRef{Name: "Mutation"},
		Types: []graphql.IntrospectionType{
			{
				Kind: "OBJECT",
				Name: "Query",
				Fields: []graphql.IntrospectionField{
					{
						Name:        "getUser",
						Description: "Get a user by ID",
						Type: graphql.TypeRef{
							Kind: "OBJECT",
							Name: "User",
						},
						Args: []graphql.InputValue{
							{
								Name: "id",
								Type: graphql.TypeRef{
									Kind: "NON_NULL",
									OfType: &graphql.TypeRef{
										Kind: "SCALAR",
										Name: "ID",
									},
								},
							},
						},
					},
					{
						Name:        "listUsers",
						Description: "List all users",
						Type: graphql.TypeRef{
							Kind: "LIST",
							OfType: &graphql.TypeRef{
								Kind: "OBJECT",
								Name: "User",
							},
						},
					},
				},
			},
			{
				Kind: "OBJECT",
				Name: "Mutation",
				Fields: []graphql.IntrospectionField{
					{
						Name:        "createUser",
						Description: "Create a new user",
						Type: graphql.TypeRef{
							Kind: "OBJECT",
							Name: "User",
						},
						Args: []graphql.InputValue{
							{
								Name: "name",
								Type: graphql.TypeRef{
									Kind: "NON_NULL",
									OfType: &graphql.TypeRef{
										Kind: "SCALAR",
										Name: "String",
									},
								},
							},
							{
								Name: "email",
								Type: graphql.TypeRef{
									Kind: "SCALAR",
									Name: "String",
								},
							},
						},
					},
					{
						Name:        "updateUser",
						Description: "Update a user",
						Type: graphql.TypeRef{
							Kind: "OBJECT",
							Name: "User",
						},
						Args: []graphql.InputValue{
							{
								Name: "id",
								Type: graphql.TypeRef{
									Kind: "NON_NULL",
									OfType: &graphql.TypeRef{
										Kind: "SCALAR",
										Name: "ID",
									},
								},
							},
							{
								Name: "name",
								Type: graphql.TypeRef{
									Kind: "SCALAR",
									Name: "String",
								},
							},
						},
					},
					{
						Name:        "deleteUser",
						Description: "Delete a user",
						Type: graphql.TypeRef{
							Kind: "OBJECT",
							Name: "User",
						},
						Args: []graphql.InputValue{
							{
								Name: "id",
								Type: graphql.TypeRef{
									Kind: "NON_NULL",
									OfType: &graphql.TypeRef{
										Kind: "SCALAR",
										Name: "ID",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	endpoints := builder.GenerateEndpoints(schema)

	if len(endpoints) != 5 {
		t.Fatalf("Expected 5 endpoints, got %d", len(endpoints))
	}

	// Check query endpoint
	getUser := findEndpoint(endpoints, "/api/get-user/{id}")
	if getUser == nil {
		t.Error("Expected GET /api/get-user/{id} endpoint")
	} else {
		if getUser.Method != "GET" {
			t.Errorf("Expected GET method, got %s", getUser.Method)
		}
		if getUser.GraphQLType != "Query" {
			t.Errorf("Expected Query type, got %s", getUser.GraphQLType)
		}
		if getUser.Field != "getUser" {
			t.Errorf("Expected field 'getUser', got %s", getUser.Field)
		}
	}

	// Check list endpoint
	listUsers := findEndpoint(endpoints, "/api/list-users")
	if listUsers == nil {
		t.Error("Expected GET /api/list-users endpoint")
	}

	// Check create mutation
	createUser := findEndpoint(endpoints, "/api/create-user")
	if createUser == nil {
		t.Error("Expected POST /api/create-user endpoint")
	} else if createUser.Method != "POST" {
		t.Errorf("Expected POST method for createUser, got %s", createUser.Method)
	}

	// Check update mutation
	updateUser := findEndpoint(endpoints, "/api/update-user/{id}")
	if updateUser == nil {
		t.Error("Expected PUT /api/update-user/{id} endpoint")
	} else if updateUser.Method != "PUT" {
		t.Errorf("Expected PUT method for updateUser, got %s", updateUser.Method)
	}

	// Check delete mutation
	deleteUser := findEndpoint(endpoints, "/api/delete-user/{id}")
	if deleteUser == nil {
		t.Error("Expected DELETE /api/delete-user/{id} endpoint")
	} else if deleteUser.Method != "DELETE" {
		t.Errorf("Expected DELETE method for deleteUser, got %s", deleteUser.Method)
	}
}

func TestEndpointBuilder_toKebabCase(t *testing.T) {
	builder := NewEndpointBuilder(EndpointBuilderOptions{})

	tests := []struct {
		input    string
		expected string
	}{
		{"getUser", "get-user"},
		{"getUserProfile", "get-user-profile"},
		{"GetUser", "get-user"},
		{"getURL", "get-url"},
		{"getURLRequest", "get-url-request"},
		{"listAllUsers", "list-all-users"},
		{"user", "user"},
		{"User", "user"},
	}

	for _, tt := range tests {
		result := builder.toKebabCase(tt.input)
		if result != tt.expected {
			t.Errorf("toKebabCase(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestEndpointBuilder_mapGraphQLTypeToJSON(t *testing.T) {
	builder := NewEndpointBuilder(EndpointBuilderOptions{})

	tests := []struct {
		input    string
		expected string
	}{
		{"String", "string"},
		{"ID", "string"},
		{"Int", "integer"},
		{"Float", "number"},
		{"Boolean", "boolean"},
		{"User", "object"},
		{"[String]", "object"},
	}

	for _, tt := range tests {
		result := builder.mapGraphQLTypeToJSON(tt.input)
		if result != tt.expected {
			t.Errorf("mapGraphQLTypeToJSON(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestEndpointBuilder_Arguments(t *testing.T) {
	builder := NewEndpointBuilder(EndpointBuilderOptions{})

	schema := &graphql.Schema{
		QueryType: &graphql.TypeRef{Name: "Query"},
		Types: []graphql.IntrospectionType{
			{
				Kind: "OBJECT",
				Name: "Query",
				Fields: []graphql.IntrospectionField{
					{
						Name:        "getUser",
						Description: "Get user",
						Type: graphql.TypeRef{
							Kind: "OBJECT",
							Name: "User",
						},
						Args: []graphql.InputValue{
							{
								Name:        "id",
								Description: "User ID",
								Type: graphql.TypeRef{
									Kind: "NON_NULL",
									OfType: &graphql.TypeRef{
										Kind: "SCALAR",
										Name: "ID",
									},
								},
							},
							{
								Name: "includeDeleted",
								Type: graphql.TypeRef{
									Kind: "SCALAR",
									Name: "Boolean",
								},
							},
						},
					},
				},
			},
		},
	}

	endpoints := builder.GenerateEndpoints(schema)

	if len(endpoints) != 1 {
		t.Fatalf("Expected 1 endpoint, got %d", len(endpoints))
	}

	ep := endpoints[0]
	if len(ep.Arguments) != 2 {
		t.Fatalf("Expected 2 arguments, got %d", len(ep.Arguments))
	}

	// ID should be a path param
	idArg := findArgument(ep.Arguments, "id")
	if idArg == nil {
		t.Fatal("Expected 'id' argument")
	}
	if idArg.Location != "path" {
		t.Errorf("Expected 'id' to be path param, got %s", idArg.Location)
	}
	if !idArg.Required {
		t.Error("Expected 'id' to be required")
	}
	if idArg.Type != "string" {
		t.Errorf("Expected 'id' type to be 'string', got %s", idArg.Type)
	}

	// includeDeleted should be query param
	deletedArg := findArgument(ep.Arguments, "includeDeleted")
	if deletedArg == nil {
		t.Fatal("Expected 'includeDeleted' argument")
	}
	if deletedArg.Location != "query" {
		t.Errorf("Expected 'includeDeleted' to be query param, got %s", deletedArg.Location)
	}
	if deletedArg.Required {
		t.Error("Expected 'includeDeleted' to be optional")
	}
}

func TestGenerateOpenAPISpec(t *testing.T) {
	builder := NewEndpointBuilder(EndpointBuilderOptions{})

	endpoints := []Endpoint{
		{
			Path:        "/api/users",
			Method:      "GET",
			GraphQLType: "Query",
			Field:       "listUsers",
			Description: "List all users",
			ReturnType:  "User",
		},
		{
			Path:        "/api/users/{id}",
			Method:      "GET",
			GraphQLType: "Query",
			Field:       "getUser",
			Description: "Get a user",
			Arguments: []Argument{
				{Name: "id", Type: "string", Required: true, Location: "path"},
				{Name: "expand", Type: "string", Required: false, Location: "query"},
			},
			ReturnType: "User",
		},
		{
			Path:        "/api/users",
			Method:      "POST",
			GraphQLType: "Mutation",
			Field:       "createUser",
			Description: "Create a user",
			Arguments: []Argument{
				{Name: "name", Type: "string", Required: true, Location: "body"},
				{Name: "email", Type: "string", Required: false, Location: "body"},
			},
			ReturnType: "User",
		},
	}

	spec := builder.GenerateOpenAPISpec(endpoints)

	// Note: /api/users has both GET and POST, so only 2 unique paths
	if len(spec) != 2 {
		t.Fatalf("Expected 2 paths (GET+POST share /api/users), got %d", len(spec))
	}

	// Check GET /api/users
	usersPath := spec["/api/users"]
	if usersPath == nil {
		t.Fatal("Expected /api/users path")
	}

	// Check GET /api/users/{id}
	userByIDPath := spec["/api/users/{id}"]
	if userByIDPath == nil {
		t.Fatal("Expected /api/users/{id} path")
	}
}

// Helper functions

func findEndpoint(endpoints []Endpoint, path string) *Endpoint {
	for _, ep := range endpoints {
		if ep.Path == path {
			return &ep
		}
	}
	return nil
}

func findArgument(args []Argument, name string) *Argument {
	for _, arg := range args {
		if arg.Name == name {
			return &arg
		}
	}
	return nil
}
