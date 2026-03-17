package graphql

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient(ClientOptions{
		Endpoint: "http://localhost:8080/graphql",
		Timeout:  10 * time.Second,
		Headers: map[string]string{
			"X-Custom-Header": "value",
		},
	})

	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	if client.endpoint != "http://localhost:8080/graphql" {
		t.Errorf("Expected endpoint to be set, got %s", client.endpoint)
	}

	if client.httpClient.Timeout != 10*time.Second {
		t.Errorf("Expected timeout to be 10s, got %v", client.httpClient.Timeout)
	}
}

func TestNewClient_DefaultTimeout(t *testing.T) {
	client := NewClient(ClientOptions{
		Endpoint: "http://localhost:8080/graphql",
	})

	if client.httpClient.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout to be 30s, got %v", client.httpClient.Timeout)
	}
}

func TestClient_Execute(t *testing.T) {
	// Create a mock GraphQL server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check method
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		// Check content type
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", ct)
		}

		// Check custom header
		if h := r.Header.Get("X-Custom-Header"); h != "custom-value" {
			t.Errorf("Expected X-Custom-Header to be forwarded, got %s", h)
		}

		// Read request body
		body, _ := io.ReadAll(r.Body)

		var req QueryRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("Failed to unmarshal request: %v", err)
		}

		// Verify query
		if req.Query != "query GetUser($id: ID!) { user(id: $id) { name } }" {
			t.Errorf("Unexpected query: %s", req.Query)
		}

		// Return mock response
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"user": map[string]interface{}{
					"name": "John Doe",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Create client
	client := NewClient(ClientOptions{
		Endpoint: mockServer.URL,
		Headers: map[string]string{
			"X-Custom-Header": "custom-value",
		},
	})

	// Execute query
	req := QueryRequest{
		Query:         "query GetUser($id: ID!) { user(id: $id) { name } }",
		Variables:     map[string]interface{}{"id": "123"},
		OperationName: "GetUser",
	}

	resp, err := client.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify response
	if resp == nil {
		t.Fatal("Expected non-nil response")
	}

	if len(resp.Errors) > 0 {
		t.Errorf("Expected no errors, got %v", resp.Errors)
	}

	// Parse data
	var data struct {
		User struct {
			Name string `json:"name"`
		} `json:"user"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("Failed to unmarshal data: %v", err)
	}

	if data.User.Name != "John Doe" {
		t.Errorf("Expected user name 'John Doe', got %s", data.User.Name)
	}
}

func TestClient_Execute_WithGraphQLError(t *testing.T) {
	// Create a mock GraphQL server that returns errors
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"data": nil,
			"errors": []map[string]interface{}{
				{
					"message": "User not found",
					"path":    []string{"user"},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Create client
	client := NewClient(ClientOptions{
		Endpoint: mockServer.URL,
	})

	// Execute query
	req := QueryRequest{
		Query: "query { user { name } }",
	}

	resp, err := client.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify response contains errors
	if len(resp.Errors) != 1 {
		t.Fatalf("Expected 1 error, got %d", len(resp.Errors))
	}

	if resp.Errors[0].Message != "User not found" {
		t.Errorf("Expected error message 'User not found', got %s", resp.Errors[0].Message)
	}
}

func TestClient_Execute_UpstreamError(t *testing.T) {
	// Create a mock server that returns an error status
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer mockServer.Close()

	// Create client
	client := NewClient(ClientOptions{
		Endpoint: mockServer.URL,
	})

	// Execute query
	req := QueryRequest{
		Query: "query { user { name } }",
	}

	resp, err := client.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for upstream failure")
	}

	if resp != nil {
		t.Error("Expected nil response on error")
	}

	if err.Error() != "upstream error: status=500, body=Internal Server Error" {
		t.Errorf("Unexpected error message: %s", err.Error())
	}
}

func TestClient_Execute_WithContextTimeout(t *testing.T) {
	// Create a slow mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": {}}`))
	}))
	defer mockServer.Close()

	// Create client with short timeout
	client := NewClient(ClientOptions{
		Endpoint: mockServer.URL,
		Timeout:  10 * time.Millisecond,
	})

	// Execute query with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	req := QueryRequest{
		Query: "query { user { name } }",
	}

	_, err := client.Execute(ctx, req)
	if err == nil {
		t.Fatal("Expected timeout error")
	}
}

func TestClient_Introspect(t *testing.T) {
	// Create a mock GraphQL server with introspection
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify introspection query
		body, _ := io.ReadAll(r.Body)
		var req QueryRequest
		json.Unmarshal(body, &req)

		if req.Query != introspectionQuery {
			t.Error("Expected introspection query")
		}

		// Return mock introspection response
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"__schema": map[string]interface{}{
					"queryType": map[string]interface{}{
						"name": "Query",
					},
					"mutationType": map[string]interface{}{
						"name": "Mutation",
					},
					"types": []map[string]interface{}{
						{
							"kind": "OBJECT",
							"name": "Query",
							"fields": []map[string]interface{}{
								{
									"name": "user",
									"type": map[string]interface{}{
										"kind": "OBJECT",
										"name": "User",
									},
								},
							},
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Create client
	client := NewClient(ClientOptions{
		Endpoint: mockServer.URL,
	})

	// Perform introspection
	schema, err := client.Introspect(context.Background())
	if err != nil {
		t.Fatalf("Introspect failed: %v", err)
	}

	// Verify schema
	if schema == nil {
		t.Fatal("Expected non-nil schema")
	}

	if schema.QueryType == nil || schema.QueryType.Name != "Query" {
		t.Error("Expected Query type")
	}

	if schema.MutationType == nil || schema.MutationType.Name != "Mutation" {
		t.Error("Expected Mutation type")
	}

	if len(schema.Types) != 1 {
		t.Errorf("Expected 1 type, got %d", len(schema.Types))
	}
}

func TestGraphQLError_Error(t *testing.T) {
	err := GraphQLError{
		Message: "Test error",
		Path:    []interface{}{"user", "name"},
	}

	if err.Error() != "Test error" {
		t.Errorf("Expected error message 'Test error', got %s", err.Error())
	}
}
