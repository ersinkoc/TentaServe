package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ersinkoc/tentaserve/internal/config"
	"github.com/ersinkoc/tentaserve/internal/gateway/metrics"
	"github.com/ersinkoc/tentaserve/internal/observability"
)

// TestRESTToGraphQLTranslation tests REST requests translated to GraphQL upstream
func TestRESTToGraphQLTranslation(t *testing.T) {
	// Create a mock GraphQL upstream server
	graphqlUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's a POST request with GraphQL content
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", ct)
		}

		// Parse the GraphQL request
		var gqlReq struct {
			Query     string                 `json:"query"`
			Variables map[string]interface{} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Failed to decode GraphQL request: %v", err)
		}

		// Verify query structure based on path/method
		var response map[string]interface{}

		// Check query type
		if contains(gqlReq.Query, "query Listusers") || contains(gqlReq.Query, "query ListUsers") {
			response = map[string]interface{}{
				"data": map[string]interface{}{
					"users": []map[string]interface{}{
						{"id": "1", "name": "Alice"},
						{"id": "2", "name": "Bob"},
					},
				},
			}
		} else if contains(gqlReq.Query, "query Getusers") || contains(gqlReq.Query, "query GetUser") {
			response = map[string]interface{}{
				"data": map[string]interface{}{
					"users": map[string]interface{}{ // Field name matches resource name
						"id":   gqlReq.Variables["id"],
						"name": "Test User",
					},
				},
			}
		} else if contains(gqlReq.Query, "mutation Createusers") || contains(gqlReq.Query, "mutation CreateUser") {
			response = map[string]interface{}{
				"data": map[string]interface{}{
					"createusers": map[string]interface{}{
						"id": "new-id-123",
					},
				},
			}
		} else if contains(gqlReq.Query, "mutation Updateusers") || contains(gqlReq.Query, "mutation UpdateUser") {
			response = map[string]interface{}{
				"data": map[string]interface{}{
					"updateusers": map[string]interface{}{
						"id": gqlReq.Variables["id"],
					},
				},
			}
		} else if contains(gqlReq.Query, "mutation Deleteusers") || contains(gqlReq.Query, "mutation DeleteUser") {
			response = map[string]interface{}{
				"data": map[string]interface{}{
					"deleteusers": map[string]interface{}{
						"success": true,
					},
				},
			}
		} else {
			t.Logf("Unexpected query: %s", gqlReq.Query)
			response = map[string]interface{}{
				"errors": []map[string]interface{}{
					{"message": "Unknown query"},
				},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer graphqlUpstream.Close()

	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			RESTPrefix:  "/api",
			GraphQLPath: "/graphql",
			MCPPath:     "/mcp",
		},
		Upstreams: []config.UpstreamConfig{
			{
				Name:     "users",
				Type:     "graphql",
				Endpoint: graphqlUpstream.URL + "/graphql",
				BaseURL:  graphqlUpstream.URL,
				Timeout:  30 * time.Second,
			},
		},
	}

	logger := observability.NewLogger(config.LoggingConfig{Level: "info"})
	handler := NewRESTHandler(cfg, logger.Logger)

	t.Run("ListUsers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
			t.Logf("Response: %s", w.Body.String())
		}

		var result map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		data, ok := result["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected data in response, got: %v", result)
		}

		if _, ok := data["users"]; !ok {
			t.Errorf("Expected users in data, got: %v", data)
		}
	})

	t.Run("GetUser", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/users/123", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
			t.Logf("Response: %s", w.Body.String())
		}

		var result map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		data, ok := result["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected data in response, got: %v", result)
		}

		// Field name is "users" (resource name from path)
		users, ok := data["users"].(map[string]interface{})
		if !ok {
			t.Errorf("Expected users in data, got: %v", data)
		}

		if users["id"] != "123" {
			t.Errorf("Expected user id 123, got %v", users["id"])
		}
	})

	t.Run("CreateUser", func(t *testing.T) {
		body := `{"name":"New User","email":"new@example.com"}`
		req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
			t.Logf("Response: %s", w.Body.String())
		}

		var result map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		data, ok := result["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected data in response, got: %v", result)
		}

		// Field name is "createusers" (resource name based)
		if _, ok := data["createusers"]; !ok {
			t.Errorf("Expected createusers in data, got: %v", data)
		}
	})

	t.Run("UpdateUser", func(t *testing.T) {
		body := `{"name":"Updated User"}`
		req := httptest.NewRequest(http.MethodPut, "/api/users/456", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
			t.Logf("Response: %s", w.Body.String())
		}

		var result map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		data, ok := result["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected data in response, got: %v", result)
		}

		// Field name is "updateusers" (resource name based)
		if _, ok := data["updateusers"]; !ok {
			t.Errorf("Expected updateusers in data, got: %v", data)
		}
	})

	t.Run("DeleteUser", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/users/789", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
			t.Logf("Response: %s", w.Body.String())
		}

		var result map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		data, ok := result["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected data in response, got: %v", result)
		}

		// Field name is "deleteusers" (resource name based)
		if _, ok := data["deleteusers"]; !ok {
			t.Errorf("Expected deleteusers in data, got: %v", data)
		}
	})
}

// TestRESTToGraphQLErrorHandling tests error handling in REST→GraphQL translation
func TestRESTToGraphQLErrorHandling(t *testing.T) {
	// Create a mock GraphQL upstream that returns errors
	graphqlUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var gqlReq struct {
			Query string `json:"query"`
		}
		json.NewDecoder(r.Body).Decode(&gqlReq)

		// Return error for specific queries
		if contains(gqlReq.Query, "Getinvalid") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []map[string]interface{}{
					{
						"message": "Resource not found",
						"extensions": map[string]interface{}{
							"code": "NOT_FOUND",
						},
					},
				},
			})
			return
		}

		// Simulate server error
		if contains(gqlReq.Query, "Getservererror") {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"errors":[{"message":"Internal server error"}]}`))
			return
		}

		// Default success
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{"test": "ok"},
		})
	}))
	defer graphqlUpstream.Close()

	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			RESTPrefix: "/api",
		},
		Upstreams: []config.UpstreamConfig{
			{
				Name:     "invalid",
				Type:     "graphql",
				Endpoint: graphqlUpstream.URL,
				BaseURL:  graphqlUpstream.URL,
				Timeout:  30 * time.Second,
			},
			{
				Name:     "servererror",
				Type:     "graphql",
				Endpoint: graphqlUpstream.URL,
				BaseURL:  graphqlUpstream.URL,
				Timeout:  30 * time.Second,
			},
			{
				Name:     "users",
				Type:     "graphql",
				Endpoint: graphqlUpstream.URL,
				BaseURL:  graphqlUpstream.URL,
				Timeout:  30 * time.Second,
			},
		},
	}

	logger := observability.NewLogger(config.LoggingConfig{Level: "info"})
	handler := NewRESTHandler(cfg, logger.Logger)

	t.Run("GraphQLErrorResponse", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/invalid/123", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// Should return the GraphQL error response
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 (GraphQL returns 200 with errors), got %d", w.Code)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if _, ok := result["errors"]; !ok {
			t.Errorf("Expected errors in response, got: %v", result)
		}
	})

	t.Run("UpstreamServerError", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/servererror/123", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// Should return the upstream status code
		if w.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", w.Code)
		}
	})

	t.Run("MissingIDForUpdate", func(t *testing.T) {
		body := `{"name":"Test"}`
		req := httptest.NewRequest(http.MethodPut, "/api/users", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}

		if !contains(w.Body.String(), "ID required") {
			t.Errorf("Expected 'ID required' error message, got: %s", w.Body.String())
		}
	})

	t.Run("MissingIDForDelete", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/users", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}

		if !contains(w.Body.String(), "ID required") {
			t.Errorf("Expected 'ID required' error message, got: %s", w.Body.String())
		}
	})
}

// TestFullGatewayIntegration tests the complete gateway with both REST and GraphQL upstreams
func TestFullGatewayIntegration(t *testing.T) {
	// Create mock REST upstream
	restUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if r.URL.Path == "/products" {
				json.NewEncoder(w).Encode([]map[string]interface{}{
					{"id": "1", "name": "Product 1", "price": 10.99},
					{"id": "2", "name": "Product 2", "price": 20.99},
				})
			} else if r.URL.Path == "/products/1" {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"id": "1", "name": "Product 1", "price": 10.99,
				})
			}
		case http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			var data map[string]interface{}
			json.Unmarshal(body, &data)
			data["id"] = "new-product-id"
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(data)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer restUpstream.Close()

	// Create mock GraphQL upstream
	graphqlUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var gqlReq struct {
			Query     string                 `json:"query"`
			Variables map[string]interface{} `json:"variables"`
		}
		json.NewDecoder(r.Body).Decode(&gqlReq)

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"orders": []map[string]interface{}{
					{"id": "ord1", "total": 100.0},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer graphqlUpstream.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		Gateway: config.GatewayConfig{
			RESTPrefix:  "/api",
			GraphQLPath: "/graphql",
			MCPPath:     "/mcp",
		},
		Upstreams: []config.UpstreamConfig{
			{
				Name:    "products",
				Type:    "rest",
				BaseURL: restUpstream.URL,
				Timeout: 30 * time.Second,
			},
			{
				Name:     "orders",
				Type:     "graphql",
				Endpoint: graphqlUpstream.URL,
				BaseURL:  graphqlUpstream.URL,
				Timeout:  30 * time.Second,
			},
		},
		Observability: config.ObservabilityConfig{
			RequestID: config.RequestIDConfig{
				Header:    "X-Request-ID",
				Propagate: true,
			},
		},
	}

	logger := observability.NewLogger(config.LoggingConfig{Level: "info"})
	metricsReg := metrics.NewRegistry()

	// Create the full handler
	handler := createHandler(cfg, logger, nil, metricsReg)

	t.Run("RESTUpstreamDirectProxy", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/products", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
			t.Logf("Response: %s", w.Body.String())
		}

		var result []map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if len(result) != 2 {
			t.Errorf("Expected 2 products, got %d", len(result))
		}
	})

	t.Run("RESTUpstreamCreateProduct", func(t *testing.T) {
		body := `{"name":"New Product","price":15.99}`
		req := httptest.NewRequest(http.MethodPost, "/api/products", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", w.Code)
			t.Logf("Response: %s", w.Body.String())
		}

		var result map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if result["id"] != "new-product-id" {
			t.Errorf("Expected id new-product-id, got %v", result["id"])
		}
	})

	t.Run("GraphQLUpstreamTranslation", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
			t.Logf("Response: %s", w.Body.String())
		}

		var result map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		data, ok := result["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected data in response, got: %v", result)
		}

		if _, ok := data["orders"]; !ok {
			t.Errorf("Expected orders in data, got: %v", data)
		}
	})

	t.Run("GraphQLEndpointQuery", func(t *testing.T) {
		query := `{"query": "{ products_api { id } }"}`
		req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(query))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
			t.Logf("Response: %s", w.Body.String())
		}

		var result map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		// The GraphQL handler should return data for the REST upstream
		if result["data"] == nil && len(result["errors"].([]interface{})) > 0 {
			t.Logf("GraphQL errors: %v", result["errors"])
		}
	})
}

// Helper function
func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
