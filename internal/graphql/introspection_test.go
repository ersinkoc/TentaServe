package graphql

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewIntrospectionClient(t *testing.T) {
	client := NewIntrospectionClient(IntrospectionClientOptions{
		URL: "http://example.com/graphql",
	})

	if client == nil {
		t.Fatal("NewIntrospectionClient returned nil")
	}
	if client.url != "http://example.com/graphql" {
		t.Errorf("Expected URL 'http://example.com/graphql', got '%s'", client.url)
	}
	if client.httpClient == nil {
		t.Error("Expected httpClient to be initialized")
	}
}

func TestIntrospect(t *testing.T) {
	// Create a mock GraphQL server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		// Parse request
		var req IntrospectionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
			return
		}

		// Verify it's the introspection query
		if req.Query == "" {
			t.Error("Expected query in request")
		}

		// Return mock response
		response := IntrospectionResponse{
			Data: &SchemaData{
				Schema: &Schema{
					QueryType: &TypeRef{
						Name: "Query",
					},
					Types: []IntrospectionType{
						{
							Kind: "OBJECT",
							Name: "Query",
							Fields: []IntrospectionField{
								{
									Name:        "user",
									Description: "Get a user",
									Type: TypeRef{
										Kind: "OBJECT",
										Name: "User",
									},
									Args: []InputValue{
										{
											Name: "id",
											Type: TypeRef{
												Kind: "NON_NULL",
												OfType: &TypeRef{
													Kind: "SCALAR",
													Name: "ID",
												},
											},
										},
									},
								},
							},
						},
							{
								Kind: "OBJECT",
								Name: "User",
								Fields: []IntrospectionField{
									{
										Name: "id",
										Type: TypeRef{
											Kind: "NON_NULL",
											OfType: &TypeRef{
												Kind: "SCALAR",
												Name: "ID",
											},
										},
									},
									{
										Name: "name",
										Type: TypeRef{
											Kind: "SCALAR",
											Name: "String",
										},
									},
								},
							},
							{
								Kind: "SCALAR",
								Name: "ID",
							},
							{
								Kind: "SCALAR",
								Name: "String",
							},
						},
					},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewIntrospectionClient(IntrospectionClientOptions{
		URL: server.URL,
	})

	schema, err := client.Introspect(context.Background())
	if err != nil {
		t.Fatalf("Introspect failed: %v", err)
	}

	if schema == nil {
		t.Fatal("Expected schema, got nil")
	}

	if schema.QueryType == nil || schema.QueryType.Name != "Query" {
		t.Error("Expected Query type")
	}

	if len(schema.Types) == 0 {
		t.Error("Expected types in schema")
	}

	// Check we can find the User type
	userType := schema.GetType("User")
	if userType == nil {
		t.Fatal("Expected to find User type")
	}
	if userType.Kind != "OBJECT" {
		t.Errorf("Expected User to be OBJECT, got %s", userType.Kind)
	}
	if len(userType.Fields) != 2 {
		t.Errorf("Expected 2 fields on User, got %d", len(userType.Fields))
	}
}

func TestIntrospect_Error(t *testing.T) {
	// Create a server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"errors":[{"message":"Introspection is disabled"}]}`))
	}))
	defer server.Close()

	client := NewIntrospectionClient(IntrospectionClientOptions{
		URL: server.URL,
	})

	_, err := client.Introspect(context.Background())
	if err == nil {
		t.Error("Expected error for GraphQL errors in response")
	}
}

func TestIntrospect_HTTPError(t *testing.T) {
	// Create a server that returns non-200
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewIntrospectionClient(IntrospectionClientOptions{
		URL: server.URL,
	})

	_, err := client.Introspect(context.Background())
	if err == nil {
		t.Error("Expected error for HTTP error")
	}
}

func TestGetType(t *testing.T) {
	schema := &Schema{
		Types: []IntrospectionType{
			{Kind: "OBJECT", Name: "User"},
			{Kind: "OBJECT", Name: "Post"},
			{Kind: "SCALAR", Name: "String"},
		},
	}

	user := schema.GetType("User")
	if user == nil {
		t.Fatal("Expected to find User type")
	}
	if user.Name != "User" {
		t.Errorf("Expected name 'User', got '%s'", user.Name)
	}

	post := schema.GetType("Post")
	if post == nil {
		t.Fatal("Expected to find Post type")
	}

	notFound := schema.GetType("NonExistent")
	if notFound != nil {
		t.Error("Expected nil for non-existent type")
	}
}

func TestUnwrapType(t *testing.T) {
	tests := []struct {
		name     string
		input    *TypeRef
		expected *TypeRef
	}{
		{
			name: "direct scalar",
			input: &TypeRef{
				Kind: "SCALAR",
				Name: "String",
			},
			expected: &TypeRef{
				Kind: "SCALAR",
				Name: "String",
			},
		},
		{
			name: "non-null wrapper",
			input: &TypeRef{
				Kind: "NON_NULL",
				OfType: &TypeRef{
					Kind: "SCALAR",
					Name: "String",
				},
			},
			expected: &TypeRef{
				Kind: "SCALAR",
				Name: "String",
			},
		},
		{
			name: "list of non-null",
			input: &TypeRef{
				Kind: "LIST",
				OfType: &TypeRef{
					Kind: "NON_NULL",
					OfType: &TypeRef{
						Kind: "OBJECT",
						Name: "User",
					},
				},
			},
			expected: &TypeRef{
				Kind: "OBJECT",
				Name: "User",
			},
		},
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := UnwrapType(tt.input)
			if tt.expected == nil {
				if result != nil {
					t.Error("Expected nil result")
				}
				return
			}
			if result == nil {
				t.Fatal("Expected non-nil result")
			}
			if result.Kind != tt.expected.Kind || result.Name != tt.expected.Name {
				t.Errorf("UnwrapType() = {%s, %s}, want {%s, %s}",
					result.Kind, result.Name, tt.expected.Kind, tt.expected.Name)
			}
		})
	}
}

func TestGetTypeName(t *testing.T) {
	tests := []struct {
		name     string
		input    *TypeRef
		expected string
	}{
		{
			name: "direct type",
			input: &TypeRef{
				Kind: "SCALAR",
				Name: "String",
			},
			expected: "String",
		},
		{
			name: "wrapped type",
			input: &TypeRef{
				Kind: "NON_NULL",
				OfType: &TypeRef{
					Kind: "LIST",
					OfType: &TypeRef{
						Kind: "OBJECT",
						Name: "User",
					},
				},
			},
			expected: "User",
		},
		{
			name:     "nil input",
			input:    nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetTypeName(tt.input)
			if result != tt.expected {
				t.Errorf("GetTypeName() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestIsNonNull(t *testing.T) {
	tests := []struct {
		name     string
		input    *TypeRef
		expected bool
	}{
		{
			name: "non-null wrapper",
			input: &TypeRef{
				Kind: "NON_NULL",
				OfType: &TypeRef{
					Kind: "SCALAR",
					Name: "String",
				},
			},
			expected: true,
		},
		{
			name: "nullable",
			input: &TypeRef{
				Kind: "SCALAR",
				Name: "String",
			},
			expected: false,
		},
		{
			name:     "nil",
			input:    nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNonNull(tt.input)
			if result != tt.expected {
				t.Errorf("IsNonNull() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsList(t *testing.T) {
	tests := []struct {
		name     string
		input    *TypeRef
		expected bool
	}{
		{
			name: "list wrapper",
			input: &TypeRef{
				Kind: "LIST",
				OfType: &TypeRef{
					Kind: "OBJECT",
					Name: "User",
				},
			},
			expected: true,
		},
		{
			name: "non-list",
			input: &TypeRef{
				Kind: "SCALAR",
				Name: "String",
			},
			expected: false,
		},
		{
			name:     "nil",
			input:    nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsList(tt.input)
			if result != tt.expected {
				t.Errorf("IsList() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestTypeKindChecks(t *testing.T) {
	tests := []struct {
		name     string
		typ      *IntrospectionType
		method   string
		expected bool
	}{
		{"scalar is scalar", &IntrospectionType{Kind: "SCALAR"}, "IsScalar", true},
		{"object is not scalar", &IntrospectionType{Kind: "OBJECT"}, "IsScalar", false},
		{"object is object", &IntrospectionType{Kind: "OBJECT"}, "IsObject", true},
		{"enum is enum", &IntrospectionType{Kind: "ENUM"}, "IsEnum", true},
		{"input object is input", &IntrospectionType{Kind: "INPUT_OBJECT"}, "IsInputObject", true},
		{"interface is interface", &IntrospectionType{Kind: "INTERFACE"}, "IsInterface", true},
		{"union is union", &IntrospectionType{Kind: "UNION"}, "IsUnion", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result bool
			switch tt.method {
			case "IsScalar":
				result = tt.typ.IsScalar()
			case "IsObject":
				result = tt.typ.IsObject()
			case "IsEnum":
				result = tt.typ.IsEnum()
			case "IsInputObject":
				result = tt.typ.IsInputObject()
			case "IsInterface":
				result = tt.typ.IsInterface()
			case "IsUnion":
				result = tt.typ.IsUnion()
			}
			if result != tt.expected {
				t.Errorf("%s() = %v, want %v", tt.method, result, tt.expected)
			}
		})
	}
}
