package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewHandler(t *testing.T) {
	handler := NewHandler(HandlerConfig{})
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
	if handler.executor == nil {
		t.Error("expected non-nil executor")
	}
	if handler.validator == nil {
		t.Error("expected non-nil validator")
	}
}

func TestHandlerMethodNotAllowed(t *testing.T) {
	handler := NewHandler(HandlerConfig{})

	req := httptest.NewRequest(http.MethodGet, "/graphql", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestHandlerMissingQuery(t *testing.T) {
	handler := NewHandler(HandlerConfig{})

	body := `{"variables": {}}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandlerInvalidJSON(t *testing.T) {
	handler := NewHandler(HandlerConfig{})

	body := `{"invalid json`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandlerParseError(t *testing.T) {
	handler := NewHandler(HandlerConfig{})

	body := `{"query": "{ invalid"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var result ExecutionResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(result.Errors) == 0 {
		t.Error("expected errors in response")
	}
}

func TestHandlerDepthLimit(t *testing.T) {
	handler := NewHandler(HandlerConfig{
		MaxDepth: 2,
	})

	// Query with depth 3 should fail
	body := `{"query": "{ a { b { c { d } } } }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var result ExecutionResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(result.Errors) == 0 {
		t.Error("expected depth validation error")
	}
}

func TestHandlerSimpleQuery(t *testing.T) {
	handler := NewHandler(HandlerConfig{})

	// Register a simple resolver
	handler.RegisterResolver("Query", "hello", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return "world", nil
	})

	body := `{"query": "{ hello }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var result ExecutionResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(result.Errors) > 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be a map")
	}

	if data["hello"] != "world" {
		t.Errorf("expected hello='world', got %v", data["hello"])
	}
}

func TestHandlerQueryWithVariables(t *testing.T) {
	handler := NewHandler(HandlerConfig{})

	// Register a resolver that uses arguments
	handler.RegisterResolver("Query", "greet", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		name := "world"
		if n, ok := args["name"].(string); ok {
			name = n
		}
		return "Hello, " + name + "!", nil
	})

	body := `{"query": "query Greet($name: String) { greet(name: $name) }", "variables": {"name": "Alice"}}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var result ExecutionResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(result.Errors) > 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be a map")
	}

	if data["greet"] != "Hello, Alice!" {
		t.Errorf("expected greet='Hello, Alice!', got %v", data["greet"])
	}
}

func TestHandlerNestedQuery(t *testing.T) {
	handler := NewHandler(HandlerConfig{})

	// Register resolvers for nested data
	handler.RegisterResolver("Query", "user", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"id":    "1",
			"name":  "John Doe",
			"email": "john@example.com",
		}, nil
	})

	body := `{"query": "{ user { id name email } }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var result ExecutionResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(result.Errors) > 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be a map")
	}

	user, ok := data["user"].(map[string]interface{})
	if !ok {
		t.Fatal("expected user to be a map")
	}

	if user["name"] != "John Doe" {
		t.Errorf("expected user.name='John Doe', got %v", user["name"])
	}
}

func TestHandlerResolverError(t *testing.T) {
	handler := NewHandler(HandlerConfig{})

	// Register a resolver that returns an error
	handler.RegisterResolver("Query", "error", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return nil, fmt.Errorf("something went wrong")
	})

	body := `{"query": "{ error }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var result ExecutionResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(result.Errors) == 0 {
		t.Error("expected errors in response")
	}
}

// --- Additional handler tests for coverage ---

func TestHandler_Executor(t *testing.T) {
	handler := NewHandler(HandlerConfig{})
	exec := handler.Executor()
	if exec == nil {
		t.Error("Expected non-nil executor")
	}
}

func TestHandler_SetValidator(t *testing.T) {
	handler := NewHandler(HandlerConfig{})
	v := NewValidator(5, 500)
	handler.SetValidator(v)

	// Verify the new validator is active by sending a deep query
	body := `{"query": "{ a { b { c { d { e { f } } } } } }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var result ExecutionResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(result.Errors) == 0 {
		t.Error("Expected depth validation error with maxDepth=5")
	}
}

func TestHandler_WithCustomConfig(t *testing.T) {
	handler := NewHandler(HandlerConfig{
		MaxDepth:      20,
		MaxComplexity: 5000,
	})
	if handler == nil {
		t.Fatal("Expected non-nil handler")
	}
}

func TestHandler_ContentTypeJSON(t *testing.T) {
	handler := NewHandler(HandlerConfig{})
	handler.RegisterResolver("Query", "ping", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return "pong", nil
	})

	body := `{"query": "{ ping }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %s", ct)
	}
}

func TestHandler_ErrorOnlyResponse(t *testing.T) {
	handler := NewHandler(HandlerConfig{})
	// No resolvers registered, query will use default resolver which errors

	body := `{"query": "{ nonexistent }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if result["errors"] == nil {
		t.Error("Expected errors in response")
	}
}
