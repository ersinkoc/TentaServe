package rest2gql

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ersinkoc/tentaserve/internal/openapi"
	"github.com/ersinkoc/tentaserve/internal/upstream"
)

func TestNewTranslator(t *testing.T) {
	registry := NewResolverRegistry()
	translator := NewTranslator(DefaultTranslatorOptions(), registry)

	if translator == nil {
		t.Fatal("NewTranslator returned nil")
	}
	if translator.validator == nil {
		t.Error("Expected validator to be initialized")
	}
	if translator.executor == nil {
		t.Error("Expected executor to be initialized")
	}
}

func TestTranslator_Translate(t *testing.T) {
	// Create a translator with no resolvers
	registry := NewResolverRegistry()
	translator := NewTranslator(DefaultTranslatorOptions(), registry)

	// Test parsing error
	resp := translator.Translate(nil, &GraphQLRequest{
		Query: "invalid {",
	})
	if len(resp.Errors) == 0 {
		t.Error("Expected parse error")
	}

	// Test validation - query too deep
	deepQuery := "query { a { b { c { d { e { f { g { h { i { j { k } } } } } } } } } } }"
	resp = translator.Translate(nil, &GraphQLRequest{
		Query: deepQuery,
	})
	if len(resp.Errors) == 0 {
		t.Error("Expected validation error for deep query")
	}
}

func TestTranslator_Translate_SimpleQuery(t *testing.T) {
	// This test requires a mock resolver
	// Create a simple resolver that returns data
	registry := NewResolverRegistry()

	client, _ := upstream.NewClient(upstream.ClientOptions{BaseURL: "http://example.com"})
	builder := NewResolverBuilder(client, "http://example.com")

	// Build a simple resolver
	op := &openapi.Operation{
		Summary: "Get user",
	}
	resolver := builder.BuildRESTResolver("/users", "GET", nil, op)
	registry.Register("Query.users", resolver)

	translator := NewTranslator(DefaultTranslatorOptions(), registry)

	// This will fail because there's no actual upstream
	// But it should parse and validate successfully
	resp := translator.Translate(nil, &GraphQLRequest{
		Query: "query { users }",
	})

	// Should have an execution error (no upstream), but should parse and validate
	if resp.Data != nil {
		t.Logf("Got data: %v", resp.Data)
	}
}

func TestTranslator_HandleHTTP(t *testing.T) {
	registry := NewResolverRegistry()
	translator := NewTranslator(DefaultTranslatorOptions(), registry)

	// Test GET request (should fail)
	req := httptest.NewRequest(http.MethodGet, "/graphql", nil)
	w := httptest.NewRecorder()
	translator.HandleHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", w.Code)
	}

	// Test invalid JSON
	req = httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader([]byte("invalid")))
	w = httptest.NewRecorder()
	translator.HandleHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}

	// Test valid request
	body, _ := json.Marshal(GraphQLRequest{
		Query: "{ __typename }",
	})
	req = httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	translator.HandleHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	var resp GraphQLResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should have an error (no resolver for __typename)
	if len(resp.Errors) == 0 {
		t.Error("Expected error for unregistered field")
	}
}

func TestParseFieldPath(t *testing.T) {
	tests := []struct {
		path         string
		wantType     string
		wantField    string
	}{
		{"Query.users", "Query", "users"},
		{"Mutation.createUser", "Mutation", "createUser"},
		{"Subscription.userUpdates", "Subscription", "userUpdates"},
		{"Query", "", ""},
		{"", "", ""},
		{"NoDotHere", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			gotType, gotField := parseFieldPath(tt.path)
			if gotType != tt.wantType {
				t.Errorf("parseFieldPath(%q) type = %q, want %q", tt.path, gotType, tt.wantType)
			}
			if gotField != tt.wantField {
				t.Errorf("parseFieldPath(%q) field = %q, want %q", tt.path, gotField, tt.wantField)
			}
		})
	}
}

func TestGraphQLRequest(t *testing.T) {
	req := GraphQLRequest{
		Query:         "query { users { id name } }",
		Variables:     map[string]interface{}{"limit": 10},
		OperationName: "GetUsers",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	var decoded GraphQLRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}

	if decoded.Query != req.Query {
		t.Errorf("Query mismatch: got %q, want %q", decoded.Query, req.Query)
	}

	if decoded.OperationName != req.OperationName {
		t.Errorf("OperationName mismatch: got %q, want %q", decoded.OperationName, req.OperationName)
	}
}

func TestGraphQLResponse(t *testing.T) {
	resp := GraphQLResponse{
		Data: map[string]interface{}{
			"users": []interface{}{
				map[string]interface{}{"id": "1", "name": "John"},
			},
		},
		Errors: []GraphQLError{
			{
				Message: "Field not found",
				Path:    []string{"users", "email"},
				Extensions: map[string]interface{}{
					"code": "FIELD_NOT_FOUND",
				},
			},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	var decoded GraphQLResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(decoded.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(decoded.Errors))
	}

	if decoded.Errors[0].Message != "Field not found" {
		t.Errorf("Error message mismatch: got %q", decoded.Errors[0].Message)
	}
}

func TestTranslator_RefreshResolvers(t *testing.T) {
	registry := NewResolverRegistry()
	translator := NewTranslator(DefaultTranslatorOptions(), registry)

	// Add a resolver after translator creation
	client, _ := upstream.NewClient(upstream.ClientOptions{BaseURL: "http://example.com"})
	builder := NewResolverBuilder(client, "http://example.com")
	op := &openapi.Operation{Summary: "Test"}
	resolver := builder.BuildRESTResolver("/test", "GET", nil, op)
	registry.Register("Query.test", resolver)

	// Refresh resolvers
	translator.RefreshResolvers()

	// The resolver should now be registered with the executor
	// We can verify by trying to execute a query
	resp := translator.Translate(nil, &GraphQLRequest{
		Query: "query { test }",
	})

	// Will have execution error (no actual upstream), but should parse/validate
	t.Logf("Response: %+v", resp)
}

func TestDefaultTranslatorOptions(t *testing.T) {
	opts := DefaultTranslatorOptions()

	if opts.MaxDepth != 10 {
		t.Errorf("Expected MaxDepth=10, got %d", opts.MaxDepth)
	}

	if opts.MaxComplexity != 1000 {
		t.Errorf("Expected MaxComplexity=1000, got %d", opts.MaxComplexity)
	}
}
