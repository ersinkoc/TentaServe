package gql2rest

import (
	"testing"

	"github.com/ersinkoc/tentaserve/internal/graphql"
)

func TestParseFieldsQuery(t *testing.T) {
	tests := []struct {
		name     string
		fields   string
		expected []FieldSelector
	}{
		{
			name:   "simple fields",
			fields: "name,email",
			expected: []FieldSelector{
				{Name: "name"},
				{Name: "email"},
			},
		},
		{
			name:   "nested fields",
			fields: "name,posts.title",
			expected: []FieldSelector{
				{Name: "name"},
				{Name: "posts", SubFields: []FieldSelector{
					{Name: "title"},
				}},
			},
		},
		{
			name:   "deeply nested",
			fields: "name,posts.author.name",
			expected: []FieldSelector{
				{Name: "name"},
				{Name: "posts", SubFields: []FieldSelector{
					{Name: "author", SubFields: []FieldSelector{
						{Name: "name"},
					}},
				}},
			},
		},
		{
			name:     "empty",
			fields:   "",
			expected: nil,
		},
		{
			name:   "with spaces",
			fields: "name , email , posts.title",
			expected: []FieldSelector{
				{Name: "name"},
				{Name: "email"},
				{Name: "posts", SubFields: []FieldSelector{
					{Name: "title"},
				}},
			},
		},
		{
			name:   "multiple nested on same parent",
			fields: "posts.title,posts.content",
			expected: []FieldSelector{
				{Name: "posts", SubFields: []FieldSelector{
					{Name: "title"},
					{Name: "content"},
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseFieldsQuery(tt.fields)
			if !fieldSelectorsEqual(result, tt.expected) {
				t.Errorf("ParseFieldsQuery(%q) = %v, want %v", tt.fields, result, tt.expected)
			}
		})
	}
}

func TestBuildSelectionSet(t *testing.T) {
	tests := []struct {
		name      string
		selectors []FieldSelector
		expected  string
	}{
		{
			name:      "empty",
			selectors: nil,
			expected:  "",
		},
		{
			name: "simple fields",
			selectors: []FieldSelector{
				{Name: "id"},
				{Name: "name"},
				{Name: "email"},
			},
			expected: "{ id name email }",
		},
		{
			name: "nested fields",
			selectors: []FieldSelector{
				{Name: "id"},
				{Name: "posts", SubFields: []FieldSelector{
					{Name: "title"},
					{Name: "content"},
				}},
			},
			expected: "{ id posts { title content } }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildSelectionSet(tt.selectors)
			if result != tt.expected {
				t.Errorf("BuildSelectionSet() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTranslator_Translate(t *testing.T) {
	schema := &graphql.Schema{
		QueryType: &graphql.TypeRef{Name: "Query"},
		Types: []graphql.IntrospectionType{
			{
				Kind: "OBJECT",
				Name: "Query",
				Fields: []graphql.IntrospectionField{
					{
						Name: "getUser",
						Type: graphql.TypeRef{Kind: "OBJECT", Name: "User"},
						Args: []graphql.InputValue{
							{Name: "id", Type: graphql.TypeRef{Kind: "NON_NULL", OfType: &graphql.TypeRef{Kind: "SCALAR", Name: "ID"}}},
						},
					},
				},
			},
			{
				Kind: "OBJECT",
				Name: "User",
				Fields: []graphql.IntrospectionField{
					{Name: "id", Type: graphql.TypeRef{Kind: "SCALAR", Name: "ID"}},
					{Name: "name", Type: graphql.TypeRef{Kind: "SCALAR", Name: "String"}},
					{Name: "email", Type: graphql.TypeRef{Kind: "SCALAR", Name: "String"}},
				},
			},
		},
	}

	translator := NewTranslator(schema)

	tests := []struct {
		name         string
		restReq      RESTRequest
		endpoint     Endpoint
		expectedQuery string
	}{
		{
			name: "simple query with ID",
			restReq: RESTRequest{
				Method:      "GET",
				Path:        "/api/users/123",
				QueryParams: map[string]string{"id": "123"},
				Fields:      []FieldSelector{{Name: "name"}, {Name: "email"}},
			},
			endpoint: Endpoint{
				Path:        "/api/users/{id}",
				Method:      "GET",
				GraphQLType: "Query",
				Field:       "getUser",
				Arguments: []Argument{
					{Name: "id", Type: "string", Required: true, Location: "path"},
				},
				ReturnType: "User",
			},
			expectedQuery: "query { getUser(id: $id) { name email } }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gqlReq, err := translator.Translate(tt.restReq, tt.endpoint)
			if err != nil {
				t.Fatalf("Translate failed: %v", err)
			}

			if gqlReq.Query != tt.expectedQuery {
				t.Errorf("Query = %q, want %q", gqlReq.Query, tt.expectedQuery)
			}

			if gqlReq.Operation != tt.endpoint.Field {
				t.Errorf("Operation = %q, want %q", gqlReq.Operation, tt.endpoint.Field)
			}
		})
	}
}

func TestTranslator_coerceValue(t *testing.T) {
	translator := NewTranslator(nil)

	tests := []struct {
		value    string
		typ      string
		expected interface{}
	}{
		{"123", "integer", int64(123)},
		{"45.67", "number", 45.67},
		{"true", "boolean", true},
		{"false", "boolean", false},
		{"hello", "string", "hello"},
		{"abc", "integer", "abc"}, // invalid int, returns as string
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			result := translator.coerceValue(tt.value, tt.typ)
			if result != tt.expected {
				t.Errorf("coerceValue(%q, %q) = %v (%T), want %v (%T)",
					tt.value, tt.typ, result, result, tt.expected, tt.expected)
			}
		})
	}
}

func TestParsePathParams(t *testing.T) {
	tests := []struct {
		pattern  string
		path     string
		expected map[string]string
	}{
		{
			pattern:  "/api/users/{id}",
			path:     "/api/users/123",
			expected: map[string]string{"id": "123"},
		},
		{
			pattern:  "/api/users/{userId}/posts/{postId}",
			path:     "/api/users/42/posts/99",
			expected: map[string]string{"userId": "42", "postId": "99"},
		},
		{
			pattern:  "/api/users",
			path:     "/api/users",
			expected: map[string]string{},
		},
		{
			pattern:  "/api/users/{id}",
			path:     "/api/users/123/extra",
			expected: map[string]string{}, // path segment count mismatch
		},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			result := ParsePathParams(tt.pattern, tt.path)
			if len(result) != len(tt.expected) {
				t.Errorf("ParsePathParams(%q, %q) = %v, want %v",
					tt.pattern, tt.path, result, tt.expected)
			}
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("ParsePathParams(%q, %q)[%q] = %q, want %q",
						tt.pattern, tt.path, k, result[k], v)
				}
			}
		})
	}
}

// Helper functions

func fieldSelectorsEqual(a, b []FieldSelector) bool {
	if len(a) != len(b) {
		return false
	}
	// Sort both slices by name for comparison (order doesn't matter for field selection)
	aSorted := make([]FieldSelector, len(a))
	bSorted := make([]FieldSelector, len(b))
	copy(aSorted, a)
	copy(bSorted, b)

	// Simple comparison - check that all items in a exist in b
	for _, av := range a {
		found := false
		for _, bv := range b {
			if fieldSelectorMatches(av, bv) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func fieldSelectorMatches(a, b FieldSelector) bool {
	if a.Name != b.Name {
		return false
	}
	if len(a.SubFields) != len(b.SubFields) {
		return false
	}
	return fieldSelectorsEqual(a.SubFields, b.SubFields)
}
