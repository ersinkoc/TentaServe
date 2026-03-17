package mcp

import (
	"encoding/json"
	"fmt"
	"testing"
)

// TestRegisterToolsHandlers tests the tools/list handler registration.
func TestRegisterToolsHandlers(t *testing.T) {
	server := NewServer(nil, nil)
	registry := NewToolRegistry(nil)

	// Register a test tool
	tool := &Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: []byte(`{"type":"object"}`),
	}
	registry.Register(tool)

	// Register handlers
	server.RegisterToolsHandlers(registry)

	// Test tools/list
	req := &Request{
		JSONRPC: "2.0",
		Method:  "tools/list",
		ID:      NewRequestID("1"),
	}

	resp := server.Handle(req)
	if resp == nil {
		t.Fatal("Expected response, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Parse result
	resultData, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}

	var result ListToolsResult
	if err := json.Unmarshal(resultData, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if len(result.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(result.Tools))
	}

	if result.Tools[0].Name != "test_tool" {
		t.Errorf("Expected tool name 'test_tool', got %s", result.Tools[0].Name)
	}
}

// TestToolsListEmpty tests tools/list with no tools.
func TestToolsListEmpty(t *testing.T) {
	server := NewServer(nil, nil)
	registry := NewToolRegistry(nil)

	server.RegisterToolsHandlers(registry)

	req := &Request{
		JSONRPC: "2.0",
		Method:  "tools/list",
		ID:      NewRequestID("1"),
	}

	resp := server.Handle(req)
	if resp == nil {
		t.Fatal("Expected response, got nil")
	}

	resultData, _ := json.Marshal(resp.Result)
	var result ListToolsResult
	json.Unmarshal(resultData, &result)

	if len(result.Tools) != 0 {
		t.Errorf("Expected 0 tools, got %d", len(result.Tools))
	}
}

// TestToolsListWithCursor tests tools/list with cursor parameter.
func TestToolsListWithCursor(t *testing.T) {
	server := NewServer(nil, nil)
	registry := NewToolRegistry(nil)

	// Register multiple tools
	for i := 0; i < 3; i++ {
		registry.Register(&Tool{
			Name:        fmt.Sprintf("tool_%d", i),
			Description: fmt.Sprintf("Tool %d", i),
			InputSchema: []byte(`{"type":"object"}`),
		})
	}

	server.RegisterToolsHandlers(registry)

	// Request with cursor
	params, _ := json.Marshal(ListToolsRequest{Cursor: ""})
	req := &Request{
		JSONRPC: "2.0",
		Method:  "tools/list",
		Params:  params,
		ID:      NewRequestID("1"),
	}

	resp := server.Handle(req)
	if resp == nil {
		t.Fatal("Expected response, got nil")
	}

	resultData, _ := json.Marshal(resp.Result)
	var result ListToolsResult
	json.Unmarshal(resultData, &result)

	if len(result.Tools) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(result.Tools))
	}
}

// TestToolsListInvalidParams tests tools/list with invalid parameters.
func TestToolsListInvalidParams(t *testing.T) {
	server := NewServer(nil, nil)
	registry := NewToolRegistry(nil)

	server.RegisterToolsHandlers(registry)

	// Request with invalid JSON params
	req := &Request{
		JSONRPC: "2.0",
		Method:  "tools/list",
		Params:  []byte(`{invalid json`),
		ID:      NewRequestID("1"),
	}

	resp := server.Handle(req)
	if resp == nil {
		t.Fatal("Expected response, got nil")
	}
	if resp.Error == nil {
		t.Fatal("Expected error for invalid params")
	}
	if resp.Error.Code != ErrInvalidParams {
		t.Errorf("Expected error code %d, got %d", ErrInvalidParams, resp.Error.Code)
	}
}

// TestCallToolRequestValidation tests CallToolRequest validation.
func TestCallToolRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		req     CallToolRequest
		wantErr bool
	}{
		{
			name:    "valid",
			req:     CallToolRequest{Name: "test_tool"},
			wantErr: false,
		},
		{
			name:    "empty name",
			req:     CallToolRequest{Name: ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

// TestNewTextContent tests creating text content.
func TestNewTextContent(t *testing.T) {
	content := NewTextContent("Hello, World!")
	if content.Type != "text" {
		t.Errorf("Expected type 'text', got %s", content.Type)
	}
	if content.Text != "Hello, World!" {
		t.Errorf("Expected text 'Hello, World!', got %s", content.Text)
	}
}

// TestNewErrorContent tests creating error content.
func TestNewErrorContent(t *testing.T) {
	content := NewErrorContent("Something went wrong")
	if content.Type != "text" {
		t.Errorf("Expected type 'text', got %s", content.Type)
	}
	if content.Text != "Something went wrong" {
		t.Errorf("Expected error message, got %s", content.Text)
	}
}

// TestToolsCallHandler tests the tools/call handler.
func TestToolsCallHandler(t *testing.T) {
	server := NewServer(nil, nil)
	registry := NewToolRegistry(nil)

	// Register a test tool
	tool := &Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: []byte(`{"type":"object"}`),
		Upstream:    "test-api",
		Operation:   "GET /test",
		Method:      "GET",
		Path:        "/test",
	}
	registry.Register(tool)

	// Register handlers
	server.RegisterToolsHandlers(registry)

	// Test calling the tool
	params, _ := json.Marshal(CallToolRequest{
		Name:      "test_tool",
		Arguments: []byte(`{"key":"value"}`),
	})
	req := &Request{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params:  params,
		ID:      NewRequestID("1"),
	}

	resp := server.Handle(req)
	if resp == nil {
		t.Fatal("Expected response, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Parse result
	resultData, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}

	var result CallToolResult
	if err := json.Unmarshal(resultData, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if len(result.Content) == 0 {
		t.Error("Expected content in result")
	}
	if result.IsError {
		t.Error("Expected no error in result")
	}
}

// TestToolsCallHandlerNotFound tests calling a non-existent tool.
func TestToolsCallHandlerNotFound(t *testing.T) {
	server := NewServer(nil, nil)
	registry := NewToolRegistry(nil)

	server.RegisterToolsHandlers(registry)

	params, _ := json.Marshal(CallToolRequest{
		Name: "nonexistent_tool",
	})
	req := &Request{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params:  params,
		ID:      NewRequestID("1"),
	}

	resp := server.Handle(req)
	if resp == nil {
		t.Fatal("Expected response, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("Expected no RPC error, got: %v", resp.Error)
	}

	// Parse result
	resultData, _ := json.Marshal(resp.Result)
	var result CallToolResult
	json.Unmarshal(resultData, &result)

	if !result.IsError {
		t.Error("Expected IsError to be true")
	}
	if len(result.Content) == 0 || result.Content[0].Text == "" {
		t.Error("Expected error message in content")
	}
}

// TestToolsCallHandlerInvalidParams tests tools/call with invalid parameters.
func TestToolsCallHandlerInvalidParams(t *testing.T) {
	server := NewServer(nil, nil)
	registry := NewToolRegistry(nil)

	server.RegisterToolsHandlers(registry)

	// Request with invalid JSON params
	req := &Request{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params:  []byte(`{invalid json`),
		ID:      NewRequestID("1"),
	}

	resp := server.Handle(req)
	if resp == nil {
		t.Fatal("Expected response, got nil")
	}
	if resp.Error == nil {
		t.Fatal("Expected error for invalid params")
	}
	if resp.Error.Code != ErrInvalidParams {
		t.Errorf("Expected error code %d, got %d", ErrInvalidParams, resp.Error.Code)
	}
}

// TestToolsCallHandlerMissingName tests tools/call with missing tool name.
func TestToolsCallHandlerMissingName(t *testing.T) {
	server := NewServer(nil, nil)
	registry := NewToolRegistry(nil)

	server.RegisterToolsHandlers(registry)

	params, _ := json.Marshal(CallToolRequest{
		Name: "",
	})
	req := &Request{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params:  params,
		ID:      NewRequestID("1"),
	}

	resp := server.Handle(req)
	if resp == nil {
		t.Fatal("Expected response, got nil")
	}
	if resp.Error == nil {
		t.Fatal("Expected error for missing tool name")
	}
	if resp.Error.Code != ErrInvalidParams {
		t.Errorf("Expected error code %d, got %d", ErrInvalidParams, resp.Error.Code)
	}
}

// TestExecuteTool tests the executeTool function.
func TestExecuteTool(t *testing.T) {
	tool := &Tool{
		Name:        "test_tool",
		Description: "Test tool",
		Upstream:    "test-api",
		Operation:   "query.getUser",
		ArgMapping: map[string]string{
			"id": "userId",
		},
	}

	args := []byte(`{"id":"123","name":"John"}`)
	result, err := executeTool(tool, args)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}
	if len(result.Content) == 0 {
		t.Error("Expected content in result")
	}
	if result.IsError {
		t.Error("Expected no error")
	}

	// Content should contain execution info
	content := result.Content[0]
	if content.Type != "text" {
		t.Errorf("Expected type 'text', got %s", content.Type)
	}
	if content.Text == "" {
		t.Error("Expected non-empty text content")
	}
}

// TestExecuteToolInvalidArgs tests executeTool with invalid arguments.
func TestExecuteToolInvalidArgs(t *testing.T) {
	tool := &Tool{
		Name:     "test_tool",
		Upstream: "test-api",
	}

	// Invalid JSON
	args := []byte(`{invalid`)
	_, err := executeTool(tool, args)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

// TestRegisterResourcesHandlers tests the resources handlers registration.
func TestRegisterResourcesHandlers(t *testing.T) {
	server := NewServer(nil, nil)

	resources := []*Resource{
		{
			URI:         "test://resource1",
			Name:        "Resource 1",
			Description: "First test resource",
			MIMEType:    "text/plain",
		},
		{
			URI:         "test://resource2",
			Name:        "Resource 2",
			Description: "Second test resource",
			MIMEType:    "application/json",
		},
	}

	server.RegisterResourcesHandlers(resources)

	// Test resources/list
	req := &Request{
		JSONRPC: "2.0",
		Method:  "resources/list",
		ID:      NewRequestID("1"),
	}

	resp := server.Handle(req)
	if resp == nil {
		t.Fatal("Expected response, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	resultData, _ := json.Marshal(resp.Result)
	var result ListResourcesResult
	if err := json.Unmarshal(resultData, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if len(result.Resources) != 2 {
		t.Errorf("Expected 2 resources, got %d", len(result.Resources))
	}
}

// TestResourcesReadHandler tests the resources/read handler.
func TestResourcesReadHandler(t *testing.T) {
	server := NewServer(nil, nil)

	resources := []*Resource{
		{
			URI:         "test://resource1",
			Name:        "Resource 1",
			Description: "First test resource",
			MIMEType:    "text/plain",
		},
	}

	server.RegisterResourcesHandlers(resources)

	params, _ := json.Marshal(ReadResourceRequest{URI: "test://resource1"})
	req := &Request{
		JSONRPC: "2.0",
		Method:  "resources/read",
		Params:  params,
		ID:      NewRequestID("1"),
	}

	resp := server.Handle(req)
	if resp == nil {
		t.Fatal("Expected response, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	resultData, _ := json.Marshal(resp.Result)
	var result ReadResourceResult
	if err := json.Unmarshal(resultData, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if len(result.Contents) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(result.Contents))
	}
	if result.Contents[0].URI != "test://resource1" {
		t.Errorf("Expected URI 'test://resource1', got %s", result.Contents[0].URI)
	}
}

// TestResourcesReadHandlerNotFound tests reading a non-existent resource.
func TestResourcesReadHandlerNotFound(t *testing.T) {
	server := NewServer(nil, nil)

	resources := []*Resource{}
	server.RegisterResourcesHandlers(resources)

	params, _ := json.Marshal(ReadResourceRequest{URI: "test://nonexistent"})
	req := &Request{
		JSONRPC: "2.0",
		Method:  "resources/read",
		Params:  params,
		ID:      NewRequestID("1"),
	}

	resp := server.Handle(req)
	if resp == nil {
		t.Fatal("Expected response, got nil")
	}
	if resp.Error == nil {
		t.Fatal("Expected error for non-existent resource")
	}
	if resp.Error.Code != ErrInvalidParams {
		t.Errorf("Expected error code %d, got %d", ErrInvalidParams, resp.Error.Code)
	}
}

// TestResourcesReadHandlerMissingURI tests resources/read with missing URI.
func TestResourcesReadHandlerMissingURI(t *testing.T) {
	server := NewServer(nil, nil)

	resources := []*Resource{}
	server.RegisterResourcesHandlers(resources)

	params, _ := json.Marshal(ReadResourceRequest{URI: ""})
	req := &Request{
		JSONRPC: "2.0",
		Method:  "resources/read",
		Params:  params,
		ID:      NewRequestID("1"),
	}

	resp := server.Handle(req)
	if resp == nil {
		t.Fatal("Expected response, got nil")
	}
	if resp.Error == nil {
		t.Fatal("Expected error for missing URI")
	}
}

// TestResourcesListEmpty tests resources/list with no resources.
func TestResourcesListEmpty(t *testing.T) {
	server := NewServer(nil, nil)

	server.RegisterResourcesHandlers([]*Resource{})

	req := &Request{
		JSONRPC: "2.0",
		Method:  "resources/list",
		ID:      NewRequestID("1"),
	}

	resp := server.Handle(req)
	if resp == nil {
		t.Fatal("Expected response, got nil")
	}

	resultData, _ := json.Marshal(resp.Result)
	var result ListResourcesResult
	json.Unmarshal(resultData, &result)

	if len(result.Resources) != 0 {
		t.Errorf("Expected 0 resources, got %d", len(result.Resources))
	}
}
