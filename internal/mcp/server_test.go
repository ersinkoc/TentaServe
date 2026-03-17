package mcp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestNewServer tests server creation.
func TestNewServer(t *testing.T) {
	s := NewServer(nil, nil)
	if s == nil {
		t.Fatal("Expected non-nil server")
	}
	if s.handlers == nil {
		t.Error("Expected handlers map to be initialized")
	}
	if s.logger == nil {
		t.Error("Expected logger to be initialized")
	}
}

// TestServerSetServerInfo tests setting server info.
func TestServerSetServerInfo(t *testing.T) {
	s := NewServer(nil, nil)
	s.SetServerInfo("my-server", "1.0.0")

	info := s.ServerInfo()
	if info.Name != "my-server" {
		t.Errorf("Expected name my-server, got %s", info.Name)
	}
	if info.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", info.Version)
	}
}

// TestServerRegister tests handler registration.
func TestServerRegister(t *testing.T) {
	s := NewServer(nil, nil)

	called := false
	handler := func(req *Request) (any, *Error) {
		called = true
		return "ok", nil
	}

	s.Register("test", handler)

	// Check handler was registered
	if s.GetHandler("test") == nil {
		t.Error("Expected handler to be registered")
	}

	// Call the handler
	req := &Request{JSONRPC: "2.0", Method: "test", ID: NewRequestID(1)}
	s.Handle(req)

	if !called {
		t.Error("Expected handler to be called")
	}
}

// TestServerUnregister tests handler unregistration.
func TestServerUnregister(t *testing.T) {
	s := NewServer(nil, nil)
	s.Register("test", func(req *Request) (any, *Error) {
		return nil, nil
	})

	s.Unregister("test")

	if s.GetHandler("test") != nil {
		t.Error("Expected handler to be unregistered")
	}
}

// TestServerHandle tests request handling.
func TestServerHandle(t *testing.T) {
	s := NewServer(nil, nil)
	s.Register("echo", func(req *Request) (any, *Error) {
		return "echo", nil
	})

	req := &Request{
		JSONRPC: "2.0",
		Method:  "echo",
		ID:      NewRequestID(1),
	}

	resp := s.Handle(req)

	if resp == nil {
		t.Fatal("Expected response")
	}
	if resp.JSONRPC != "2.0" {
		t.Error("Expected jsonrpc 2.0")
	}
	if resp.Error != nil {
		t.Errorf("Unexpected error: %v", resp.Error)
	}
	if resp.Result != "echo" {
		t.Errorf("Expected result 'echo', got %v", resp.Result)
	}
}

// TestServerHandleNilRequest tests handling nil request.
func TestServerHandleNilRequest(t *testing.T) {
	s := NewServer(nil, nil)
	resp := s.Handle(nil)

	if resp == nil {
		t.Fatal("Expected error response")
	}
	if resp.Error == nil {
		t.Fatal("Expected error")
	}
	if resp.Error.Code != ErrInvalidRequest {
		t.Errorf("Expected code %d, got %d", ErrInvalidRequest, resp.Error.Code)
	}
}

// TestServerHandleInvalidRequest tests handling invalid request.
func TestServerHandleInvalidRequest(t *testing.T) {
	s := NewServer(nil, nil)

	// Missing method
	req := &Request{JSONRPC: "2.0", ID: NewRequestID(1)}
	resp := s.Handle(req)

	if resp.Error == nil {
		t.Fatal("Expected error")
	}
	if resp.Error.Code != ErrInvalidRequest {
		t.Errorf("Expected code %d, got %d", ErrInvalidRequest, resp.Error.Code)
	}
}

// TestServerHandleMethodNotFound tests method not found.
func TestServerHandleMethodNotFound(t *testing.T) {
	s := NewServer(nil, nil)

	req := &Request{
		JSONRPC: "2.0",
		Method:  "unknown",
		ID:      NewRequestID(1),
	}
	resp := s.Handle(req)

	if resp.Error == nil {
		t.Fatal("Expected error")
	}
	if resp.Error.Code != ErrMethodNotFound {
		t.Errorf("Expected code %d, got %d", ErrMethodNotFound, resp.Error.Code)
	}
}

// TestServerHandleHandlerError tests handler returning error.
func TestServerHandleHandlerError(t *testing.T) {
	s := NewServer(nil, nil)
	s.Register("fail", func(req *Request) (any, *Error) {
		return nil, NewError(ErrServerError, "something went wrong")
	})

	req := &Request{
		JSONRPC: "2.0",
		Method:  "fail",
		ID:      NewRequestID(1),
	}
	resp := s.Handle(req)

	if resp.Error == nil {
		t.Fatal("Expected error")
	}
	if resp.Error.Code != ErrServerError {
		t.Errorf("Expected code %d, got %d", ErrServerError, resp.Error.Code)
	}
	if resp.Error.Message != "something went wrong" {
		t.Errorf("Expected message 'something went wrong', got %s", resp.Error.Message)
	}
}

// TestServerHandleBatch tests batch request handling.
func TestServerHandleBatch(t *testing.T) {
	s := NewServer(nil, nil)
	s.Register("add", func(req *Request) (any, *Error) {
		var params struct{ A, B int }
		json.Unmarshal(req.Params, &params)
		return params.A + params.B, nil
	})

	reqs := []*Request{
		{JSONRPC: "2.0", Method: "add", ID: NewRequestID(1), Params: []byte(`{"a":1,"b":2}`)},
		{JSONRPC: "2.0", Method: "add", ID: NewRequestID(2), Params: []byte(`{"a":3,"b":4}`)},
	}

	resps := s.HandleBatch(reqs)

	if len(resps) != 2 {
		t.Fatalf("Expected 2 responses, got %d", len(resps))
	}

	// Check first response
	if resps[0].Error != nil {
		t.Errorf("Unexpected error in response 1: %v", resps[0].Error)
	}

	// Check second response
	if resps[1].Error != nil {
		t.Errorf("Unexpected error in response 2: %v", resps[1].Error)
	}
}

// TestServerHandleEmptyBatch tests empty batch handling.
func TestServerHandleEmptyBatch(t *testing.T) {
	s := NewServer(nil, nil)
	resps := s.HandleBatch([]*Request{})

	if len(resps) != 1 {
		t.Fatalf("Expected 1 error response, got %d", len(resps))
	}
	if resps[0].Error == nil || resps[0].Error.Code != ErrInvalidRequest {
		t.Error("Expected invalid request error")
	}
}

// TestServerHandleBatchWithNotification tests batch with notifications.
func TestServerHandleBatchWithNotification(t *testing.T) {
	s := NewServer(nil, nil)
	called := false
	s.Register("notify", func(req *Request) (any, *Error) {
		called = true
		return nil, nil
	})

	reqs := []*Request{
		{JSONRPC: "2.0", Method: "notify"}, // Notification (no ID)
		{JSONRPC: "2.0", Method: "notify", ID: NewRequestID(1)},
	}

	resps := s.HandleBatch(reqs)

	if len(resps) != 1 {
		t.Fatalf("Expected 1 response (notification excluded), got %d", len(resps))
	}
	if !called {
		t.Error("Expected handler to be called for notification")
	}
}

// TestServerHandleBatchWithNull tests batch with null entries.
func TestServerHandleBatchWithNull(t *testing.T) {
	s := NewServer(nil, nil)
	reqs := []*Request{nil}

	resps := s.HandleBatch(reqs)

	if len(resps) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(resps))
	}
	if resps[0].Error == nil || resps[0].Error.Code != ErrInvalidRequest {
		t.Error("Expected invalid request error for null entry")
	}
}

// TestServerServeHTTP tests HTTP handler.
func TestServerServeHTTP(t *testing.T) {
	s := NewServer(nil, nil)
	s.Register("test", func(req *Request) (any, *Error) {
		return "success", nil
	})

	body := []byte(`{"jsonrpc":"2.0","method":"test","id":1}`)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("Unexpected error: %v", resp.Error)
	}
	if resp.Result != "success" {
		t.Errorf("Expected result 'success', got %v", resp.Result)
	}
}

// TestServerServeHTTPWrongMethod tests HTTP handler with wrong method.
func TestServerServeHTTPWrongMethod(t *testing.T) {
	s := NewServer(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rec := httptest.NewRecorder()

	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

// TestServerServeHTTPParseError tests HTTP handler with invalid JSON.
func TestServerServeHTTPParseError(t *testing.T) {
	s := NewServer(nil, nil)
	body := []byte(`{invalid json}`)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("Expected error")
	}
	if resp.Error.Code != ErrParseError {
		t.Errorf("Expected code %d, got %d", ErrParseError, resp.Error.Code)
	}
}

// TestServerServeHTTPNotification tests HTTP handler with notification.
func TestServerServeHTTPNotification(t *testing.T) {
	s := NewServer(nil, nil)
	called := false
	s.Register("notify", func(req *Request) (any, *Error) {
		called = true
		return nil, nil
	})

	body := []byte(`{"jsonrpc":"2.0","method":"notify"}`)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if !called {
		t.Error("Expected handler to be called")
	}
}

// TestServerServeHTTPBatch tests HTTP handler with batch request.
func TestServerServeHTTPBatch(t *testing.T) {
	s := NewServer(nil, nil)
	s.Register("test", func(req *Request) (any, *Error) {
		return "ok", nil
	})

	body := []byte(`[
		{"jsonrpc":"2.0","method":"test","id":1},
		{"jsonrpc":"2.0","method":"test","id":2}
	]`)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resps []Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resps); err != nil {
		t.Fatalf("Failed to parse batch response: %v", err)
	}

	if len(resps) != 2 {
		t.Errorf("Expected 2 responses, got %d", len(resps))
	}
}

// TestServerInitialize tests the built-in initialize handler.
func TestServerInitialize(t *testing.T) {
	s := NewServer(nil, nil)
	s.SetServerInfo("test-server", "0.1.0")

	req := &Request{
		JSONRPC: "2.0",
		Method:  "initialize",
		ID:      NewRequestID(1),
		Params:  []byte(`{"protocolVersion":"2024-11-05"}`),
	}

	resp := s.Handle(req)

	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("Expected map result, got %T", resp.Result)
	}

	// Check protocol version
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("Unexpected protocol version: %v", result["protocolVersion"])
	}

	// Check server info - it's a map[string]string internally
	serverInfoRaw, ok := result["serverInfo"]
	if !ok {
		t.Fatal("Expected serverInfo in result")
	}

	// The serverInfo is created as map[string]string
	serverInfo, ok := serverInfoRaw.(map[string]string)
	if !ok {
		t.Fatalf("Expected serverInfo to be map[string]string, got %T", serverInfoRaw)
	}
	if serverInfo["name"] != "test-server" {
		t.Errorf("Expected name 'test-server', got %v", serverInfo["name"])
	}
	if serverInfo["version"] != "0.1.0" {
		t.Errorf("Expected version '0.1.0', got %v", serverInfo["version"])
	}
}

// TestServerInitializeInvalidParams tests initialize with invalid params.
func TestServerInitializeInvalidParams(t *testing.T) {
	s := NewServer(nil, nil)

	req := &Request{
		JSONRPC: "2.0",
		Method:  "initialize",
		ID:      NewRequestID(1),
		Params:  []byte(`{invalid json}`),
	}

	resp := s.Handle(req)

	if resp.Error == nil {
		t.Fatal("Expected error for invalid params")
	}
	if resp.Error.Code != ErrInvalidParams {
		t.Errorf("Expected code %d, got %d", ErrInvalidParams, resp.Error.Code)
	}
}

// TestHandlerNames tests getting handler names.
func TestHandlerNames(t *testing.T) {
	s := NewServer(nil, nil)
	names := s.HandlerNames()

	// Should have at least "initialize"
	found := false
	for _, name := range names {
		if name == "initialize" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'initialize' handler to be registered")
	}
}
