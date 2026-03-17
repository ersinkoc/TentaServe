package gql2rest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// MockGraphQLClient is a mock GraphQL client for testing.
type MockGraphQLClient struct {
	ExecuteFunc func(ctx context.Context, query string, variables map[string]interface{}) (*GraphQLResponse, error)
}

func (m *MockGraphQLClient) Execute(ctx context.Context, query string, variables map[string]interface{}) (*GraphQLResponse, error) {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, query, variables)
	}
	return nil, errors.New("no execute function defined")
}

func TestHandler_ServeHTTP_SimpleQuery(t *testing.T) {
	mockClient := &MockGraphQLClient{
		ExecuteFunc: func(ctx context.Context, query string, variables map[string]interface{}) (*GraphQLResponse, error) {
			// Verify the query structure
			if variables["id"] != "123" {
				t.Errorf("Expected variable id=123, got %v", variables["id"])
			}

			return &GraphQLResponse{
				Data: json.RawMessage(`{"getUser":{"id":"123","name":"John Doe","email":"john@example.com"}}`),
			}, nil
		},
	}

	endpoints := []Endpoint{
		{
			Path:        "/api/users/{id}",
			Method:      "GET",
			GraphQLType: "Query",
			Field:       "getUser",
			Arguments: []Argument{
				{Name: "id", Type: "string", Required: true, Location: "path"},
			},
			ReturnType: "User",
		},
	}

	handler := NewHandler(HandlerOptions{
		BasePath:     "/api",
		Endpoints:    endpoints,
		GraphQLClient: mockClient,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users/123", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["id"] != "123" {
		t.Errorf("Expected id=123, got %v", result["id"])
	}
	if result["name"] != "John Doe" {
		t.Errorf("Expected name='John Doe', got %v", result["name"])
	}
}

func TestHandler_ServeHTTP_WithQueryParams(t *testing.T) {
	mockClient := &MockGraphQLClient{
		ExecuteFunc: func(ctx context.Context, query string, variables map[string]interface{}) (*GraphQLResponse, error) {
			if variables["search"] != "john" {
				t.Errorf("Expected variable search='john', got %v", variables["search"])
			}
			if variables["limit"] != int64(10) {
				t.Errorf("Expected variable limit=10 (int64), got %v (%T)", variables["limit"], variables["limit"])
			}

			return &GraphQLResponse{
				Data: json.RawMessage(`{"searchUsers":[{"id":"1","name":"John Doe"}]}`),
			}, nil
		},
	}

	endpoints := []Endpoint{
		{
			Path:        "/api/users",
			Method:      "GET",
			GraphQLType: "Query",
			Field:       "searchUsers",
			Arguments: []Argument{
				{Name: "search", Type: "string", Location: "query"},
				{Name: "limit", Type: "integer", Location: "query"},
			},
			ReturnType: "[User]",
		},
	}

	handler := NewHandler(HandlerOptions{
		BasePath:      "/api",
		Endpoints:     endpoints,
		GraphQLClient: mockClient,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users?search=john&limit=10", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result []interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("Expected 1 user, got %d", len(result))
	}
}

func TestHandler_ServeHTTP_Mutation(t *testing.T) {
	mockClient := &MockGraphQLClient{
		ExecuteFunc: func(ctx context.Context, query string, variables map[string]interface{}) (*GraphQLResponse, error) {
			if variables["name"] != "New User" {
				t.Errorf("Expected variable name='New User', got %v", variables["name"])
			}

			return &GraphQLResponse{
				Data: json.RawMessage(`{"createUser":{"id":"123","name":"New User"}}`),
			}, nil
		},
	}

	endpoints := []Endpoint{
		{
			Path:        "/api/users",
			Method:      "POST",
			GraphQLType: "Mutation",
			Field:       "createUser",
			Arguments: []Argument{
				{Name: "name", Type: "string", Required: true, Location: "body"},
				{Name: "email", Type: "string", Location: "body"},
			},
			ReturnType: "User",
		},
	}

	handler := NewHandler(HandlerOptions{
		BasePath:      "/api",
		Endpoints:     endpoints,
		GraphQLClient: mockClient,
	})

	body := `{"name":"New User","email":"new@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["id"] != "123" {
		t.Errorf("Expected id='123', got %v", result["id"])
	}
}

func TestHandler_ServeHTTP_NotFound(t *testing.T) {
	endpoints := []Endpoint{
		{
			Path:   "/api/users",
			Method: "GET",
			Field:  "getUsers",
		},
	}

	handler := NewHandler(HandlerOptions{
		BasePath:  "/api",
		Endpoints: endpoints,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/nonexistent", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rec.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["code"] != "NOT_FOUND" {
		t.Errorf("Expected error code 'NOT_FOUND', got %v", result["code"])
	}
}

func TestHandler_ServeHTTP_GraphQLError(t *testing.T) {
	mockClient := &MockGraphQLClient{
		ExecuteFunc: func(ctx context.Context, query string, variables map[string]interface{}) (*GraphQLResponse, error) {
			return &GraphQLResponse{
				Data: json.RawMessage(`null`),
				Errors: []GraphQLError{
					{
						Message: "User not found",
						Extensions: map[string]interface{}{
							"code": "NOT_FOUND",
						},
					},
				},
			}, nil
		},
	}

	endpoints := []Endpoint{
		{
			Path:        "/api/users/{id}",
			Method:      "GET",
			GraphQLType: "Query",
			Field:       "getUser",
			Arguments: []Argument{
				{Name: "id", Type: "string", Required: true, Location: "path"},
			},
			ReturnType: "User",
		},
	}

	handler := NewHandler(HandlerOptions{
		BasePath:      "/api",
		Endpoints:     endpoints,
		GraphQLClient: mockClient,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users/999", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rec.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["code"] != "NOT_FOUND" {
		t.Errorf("Expected error code 'NOT_FOUND', got %v", result["code"])
	}
	if result["message"] != "User not found" {
		t.Errorf("Expected error message 'User not found', got %v", result["message"])
	}
}

func TestHandler_ServeHTTP_InvalidJSON(t *testing.T) {
	endpoints := []Endpoint{
		{
			Path:        "/api/users",
			Method:      "POST",
			GraphQLType: "Mutation",
			Field:       "createUser",
		},
	}

	handler := NewHandler(HandlerOptions{
		BasePath:  "/api",
		Endpoints: endpoints,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader([]byte(`{invalid json`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rec.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["code"] != "INVALID_BODY" {
		t.Errorf("Expected error code 'INVALID_BODY', got %v", result["code"])
	}
}

func TestHandler_ServeHTTP_UpstreamError(t *testing.T) {
	mockClient := &MockGraphQLClient{
		ExecuteFunc: func(ctx context.Context, query string, variables map[string]interface{}) (*GraphQLResponse, error) {
			return nil, errors.New("connection refused")
		},
	}

	endpoints := []Endpoint{
		{
			Path:        "/api/users",
			Method:      "GET",
			GraphQLType: "Query",
			Field:       "getUsers",
		},
	}

	handler := NewHandler(HandlerOptions{
		BasePath:      "/api",
		Endpoints:     endpoints,
		GraphQLClient: mockClient,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rec.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["code"] != "UPSTREAM_ERROR" {
		t.Errorf("Expected error code 'UPSTREAM_ERROR', got %v", result["code"])
	}
}

func TestHandler_ServeHTTP_WithFields(t *testing.T) {
	mockClient := &MockGraphQLClient{
		ExecuteFunc: func(ctx context.Context, query string, variables map[string]interface{}) (*GraphQLResponse, error) {
			// Verify the query includes only requested fields
			// Note: we check the selection part after the argument list
			// The query format is: query { getUser(id: $id) { email name } }
			argEnd := strings.Index(query, ")")
			if argEnd == -1 {
				t.Fatalf("Could not find end of arguments in query: %s", query)
			}
			selectionPart := query[argEnd:]

			if !strings.Contains(selectionPart, "name") || !strings.Contains(selectionPart, "email") {
				t.Errorf("Query should include name and email fields in selection: %s", query)
			}
			// id should not appear in the selection part (but it can be in the argument list)
			if strings.Contains(selectionPart, "id") {
				t.Errorf("Query should NOT include id field in selection (not requested): %s", query)
			}

			return &GraphQLResponse{
				Data: json.RawMessage(`{"getUser":{"name":"John","email":"john@example.com"}}`),
			}, nil
		},
	}

	endpoints := []Endpoint{
		{
			Path:        "/api/users/{id}",
			Method:      "GET",
			GraphQLType: "Query",
			Field:       "getUser",
			Arguments: []Argument{
				{Name: "id", Type: "string", Required: true, Location: "path"},
			},
			ReturnType: "User",
		},
	}

	handler := NewHandler(HandlerOptions{
		BasePath:      "/api",
		Endpoints:     endpoints,
		GraphQLClient: mockClient,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users/123?fields=name,email", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["name"] != "John" {
		t.Errorf("Expected name='John', got %v", result["name"])
	}
}

func TestHandler_matchPath(t *testing.T) {
	h := &Handler{}

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
			path:     "/api/users",
			expected: nil,
		},
		{
			pattern:  "/api/users/{id}",
			path:     "/api/users/123/extra",
			expected: nil,
		},
		{
			pattern:  "/api/users/{id}",
			path:     "/api/posts/123",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.path, func(t *testing.T) {
			result := h.matchPath(tt.pattern, tt.path)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil, got %v", result)
				}
				return
			}

			if result == nil {
				t.Errorf("Expected %v, got nil", tt.expected)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
				return
			}

			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("Expected %s=%s, got %s", k, v, result[k])
				}
			}
		})
	}
}

func TestHandler_findEndpoint(t *testing.T) {
	endpoints := []Endpoint{
		{Path: "/api/users", Method: "GET", Field: "getUsers"},
		{Path: "/api/users/{id}", Method: "GET", Field: "getUser"},
		{Path: "/api/users", Method: "POST", Field: "createUser"},
		{Path: "/api/users/{id}", Method: "PUT", Field: "updateUser"},
	}

	h := &Handler{endpoints: endpoints}

	tests := []struct {
		path           string
		method         string
		expectedField  string
		expectedParams map[string]string
	}{
		{"/api/users", "GET", "getUsers", map[string]string{}},
		{"/api/users/123", "GET", "getUser", map[string]string{"id": "123"}},
		{"/api/users", "POST", "createUser", map[string]string{}},
		{"/api/users/456", "PUT", "updateUser", map[string]string{"id": "456"}},
		{"/api/users", "DELETE", "", nil},
		{"/api/nonexistent", "GET", "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.method+"_"+tt.path, func(t *testing.T) {
			endpoint, params := h.findEndpoint(tt.path, tt.method)

			if tt.expectedField == "" {
				if endpoint != nil {
					t.Errorf("Expected nil endpoint, got %v", endpoint)
				}
				return
			}

			if endpoint == nil {
				t.Errorf("Expected endpoint with field %s, got nil", tt.expectedField)
				return
			}

			if endpoint.Field != tt.expectedField {
				t.Errorf("Expected field %s, got %s", tt.expectedField, endpoint.Field)
			}

			for k, v := range tt.expectedParams {
				if params[k] != v {
					t.Errorf("Expected param %s=%s, got %s", k, v, params[k])
				}
			}
		})
	}
}

func TestNewHTTPGraphQLClient(t *testing.T) {
	client := NewHTTPGraphQLClient("http://localhost:4000/graphql")

	if client.BaseURL != "http://localhost:4000/graphql" {
		t.Errorf("Expected BaseURL to be 'http://localhost:4000/graphql', got %s", client.BaseURL)
	}

	if client.HTTPClient == nil {
		t.Error("Expected HTTPClient to be initialized")
	}

	if client.Headers == nil {
		t.Error("Expected Headers to be initialized")
	}
}

func TestHTTPGraphQLClient_Execute(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST method, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got %s", r.Header.Get("Content-Type"))
		}

		// Read and verify request body
		body, _ := io.ReadAll(r.Body)
		var req GraphQLRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("Failed to parse request: %v", err)
		}

		if req.Query != "query { test }" {
			t.Errorf("Expected query 'query { test }', got %s", req.Query)
		}

		// Send response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(GraphQLResponse{
			Data: json.RawMessage(`{"test":"value"}`),
		})
	}))
	defer server.Close()

	client := NewHTTPGraphQLClient(server.URL)
	client.Headers["X-Custom-Header"] = "custom-value"

	resp, err := client.Execute(context.Background(), "query { test }", nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var dataMap map[string]json.RawMessage
	if err := json.Unmarshal(resp.Data, &dataMap); err != nil {
		t.Fatalf("Failed to parse response data: %v", err)
	}

	if string(dataMap["test"]) != `"value"` {
		t.Errorf("Expected test='value', got %s", string(dataMap["test"]))
	}
}

func TestHTTPGraphQLClient_Execute_GraphQLError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(GraphQLResponse{
			Data: json.RawMessage(`null`),
			Errors: []GraphQLError{
				{Message: "Something went wrong"},
			},
		})
	}))
	defer server.Close()

	client := NewHTTPGraphQLClient(server.URL)

	resp, err := client.Execute(context.Background(), "query { test }", nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if len(resp.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(resp.Errors))
	}

	if resp.Errors[0].Message != "Something went wrong" {
		t.Errorf("Expected error message 'Something went wrong', got %s", resp.Errors[0].Message)
	}
}

func TestHTTPGraphQLClient_Execute_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewHTTPGraphQLClient(server.URL)

	resp, err := client.Execute(context.Background(), "query { test }", nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Response should still be parsed even with non-200 status
	// (GraphQL typically returns 200 OK with errors in body)
	if resp == nil {
		t.Error("Expected response even on HTTP error")
	}
}

func TestMapErrorCodeToHTTPStatus(t *testing.T) {
	tests := []struct {
		code     string
		expected int
	}{
		{"UNAUTHORIZED", http.StatusUnauthorized},
		{"FORBIDDEN", http.StatusForbidden},
		{"NOT_FOUND", http.StatusNotFound},
		{"VALIDATION_FAILED", http.StatusBadRequest},
		{"BAD_USER_INPUT", http.StatusBadRequest},
		{"INTERNAL_ERROR", http.StatusInternalServerError},
		{"TIMEOUT", http.StatusGatewayTimeout},
		{"RATE_LIMITED", http.StatusTooManyRequests},
		{"UNKNOWN_ERROR", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			result := mapErrorCodeToHTTPStatus(tt.code)
			if result != tt.expected {
				t.Errorf("mapErrorCodeToHTTPStatus(%q) = %d, want %d", tt.code, result, tt.expected)
			}
		})
	}
}

func TestNewHandler(t *testing.T) {
	mockClient := &MockGraphQLClient{}

	handler := NewHandler(HandlerOptions{
		BasePath:      "/api",
		Endpoints:     []Endpoint{{Path: "/test", Method: "GET"}},
		GraphQLClient: mockClient,
	})

	if handler == nil {
		t.Fatal("Expected handler to be created")
	}

	if handler.basePath != "/api" {
		t.Errorf("Expected basePath '/api', got %s", handler.basePath)
	}

	if len(handler.endpoints) != 1 {
		t.Errorf("Expected 1 endpoint, got %d", len(handler.endpoints))
	}

	if handler.client != mockClient {
		t.Error("Expected client to be set")
	}
}

func TestNewHandler_WithGraphQLURL(t *testing.T) {
	handler := NewHandler(HandlerOptions{
		BasePath:   "/api",
		GraphQLURL: "http://localhost:4000/graphql",
	})

	if handler == nil {
		t.Fatal("Expected handler to be created")
	}

	if handler.client == nil {
		t.Error("Expected client to be created from GraphQLURL")
	}
}

