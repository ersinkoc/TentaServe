package integration_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ersinkoc/tentaserve/internal/mcp"
)

// TestMCP_Initialize verifies the MCP initialize handshake. The server
// must respond with protocolVersion, serverInfo, and capabilities.
func TestMCP_Initialize(t *testing.T) {
	server := mcp.NewServer(slog.Default(), nil)

	ts := httptest.NewServer(server)
	defer ts.Close()

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"clientInfo": map[string]string{
				"name":    "test-client",
				"version": "0.1.0",
			},
		},
	}

	body, _ := json.Marshal(reqBody)
	resp, err := http.Post(ts.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	respBody := readBody(t, resp)

	assertStatusCode(t, resp, http.StatusOK)
	assertContains(t, respBody, "protocolVersion")
	assertContains(t, respBody, "serverInfo")
	assertContains(t, respBody, "capabilities")
	assertContains(t, respBody, "tentaserve")
}

// TestMCP_ToolsList verifies that the tools/list method returns the
// registered tools.
func TestMCP_ToolsList(t *testing.T) {
	server := mcp.NewServer(slog.Default(), nil)
	registry := mcp.NewToolRegistry(slog.Default())

	// Register test tools.
	err := registry.Register(&mcp.Tool{
		Name:        "test_get_users",
		Description: "Get all users",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	})
	if err != nil {
		t.Fatalf("failed to register tool: %v", err)
	}

	err = registry.Register(&mcp.Tool{
		Name:        "test_create_user",
		Description: "Create a new user",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`),
	})
	if err != nil {
		t.Fatalf("failed to register tool: %v", err)
	}

	server.RegisterToolsHandlers(registry)

	ts := httptest.NewServer(server)
	defer ts.Close()

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}

	body, _ := json.Marshal(reqBody)
	resp, err := http.Post(ts.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	respBody := readBody(t, resp)

	assertStatusCode(t, resp, http.StatusOK)
	assertContains(t, respBody, "test_get_users")
	assertContains(t, respBody, "test_create_user")
	assertContains(t, respBody, "tools")

	// Parse and validate the JSON-RPC response structure.
	var rpcResp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			Tools []struct {
				Name        string          `json:"name"`
				Description string          `json:"description"`
				InputSchema json.RawMessage `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(respBody), &rpcResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if rpcResp.Error != nil {
		t.Fatalf("unexpected error: %s", rpcResp.Error.Message)
	}
	if len(rpcResp.Result.Tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(rpcResp.Result.Tools))
	}
}

// TestMCP_ToolsCall verifies that the tools/call method executes a tool
// and returns a result.
func TestMCP_ToolsCall(t *testing.T) {
	server := mcp.NewServer(slog.Default(), nil)
	registry := mcp.NewToolRegistry(slog.Default())

	err := registry.Register(&mcp.Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}}}`),
		Upstream:    "test-upstream",
		Operation:   "GET /test",
		Method:      "GET",
		Path:        "/test",
	})
	if err != nil {
		t.Fatalf("failed to register tool: %v", err)
	}

	server.RegisterToolsHandlers(registry)

	ts := httptest.NewServer(server)
	defer ts.Close()

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "test_tool",
			"arguments": map[string]interface{}{"query": "hello"},
		},
	}

	body, _ := json.Marshal(reqBody)
	resp, err := http.Post(ts.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	respBody := readBody(t, resp)

	assertStatusCode(t, resp, http.StatusOK)
	assertContains(t, respBody, "content")
	// The stub implementation returns tool execution info.
	assertContains(t, respBody, "test_tool")
}

// TestMCP_ToolsCallNotFound verifies that calling a nonexistent tool
// returns an error result.
func TestMCP_ToolsCallNotFound(t *testing.T) {
	server := mcp.NewServer(slog.Default(), nil)
	registry := mcp.NewToolRegistry(slog.Default())
	server.RegisterToolsHandlers(registry)

	ts := httptest.NewServer(server)
	defer ts.Close()

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "nonexistent_tool",
		},
	}

	body, _ := json.Marshal(reqBody)
	resp, err := http.Post(ts.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	respBody := readBody(t, resp)

	assertStatusCode(t, resp, http.StatusOK)
	assertContains(t, respBody, "not found")
	assertContains(t, respBody, "isError")
}

// TestMCP_ToolsCallMissingName verifies that calling tools/call without a
// tool name returns an error.
func TestMCP_ToolsCallMissingName(t *testing.T) {
	server := mcp.NewServer(slog.Default(), nil)
	registry := mcp.NewToolRegistry(slog.Default())
	server.RegisterToolsHandlers(registry)

	ts := httptest.NewServer(server)
	defer ts.Close()

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "",
		},
	}

	body, _ := json.Marshal(reqBody)
	resp, err := http.Post(ts.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	respBody := readBody(t, resp)

	assertStatusCode(t, resp, http.StatusOK)
	// Should indicate a validation error about the tool name.
	assertContains(t, respBody, "error")
}

// TestMCP_MethodNotFound verifies that calling an unknown JSON-RPC method
// returns an appropriate error.
func TestMCP_MethodNotFound(t *testing.T) {
	server := mcp.NewServer(slog.Default(), nil)

	ts := httptest.NewServer(server)
	defer ts.Close()

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "unknown/method",
	}

	body, _ := json.Marshal(reqBody)
	resp, err := http.Post(ts.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	respBody := readBody(t, resp)

	assertStatusCode(t, resp, http.StatusOK)
	assertContains(t, respBody, "Method not found")
}

// TestMCP_InvalidJSON verifies that sending invalid JSON returns a parse
// error.
func TestMCP_InvalidJSON(t *testing.T) {
	server := mcp.NewServer(slog.Default(), nil)

	ts := httptest.NewServer(server)
	defer ts.Close()

	resp, err := http.Post(ts.URL, "application/json", bytes.NewReader([]byte(`{not json`)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	respBody := readBody(t, resp)

	assertStatusCode(t, resp, http.StatusOK)
	assertContains(t, respBody, "Parse error")
}

// TestMCP_HTTPMethodNotAllowed verifies that GET requests to the MCP
// endpoint are rejected.
func TestMCP_HTTPMethodNotAllowed(t *testing.T) {
	server := mcp.NewServer(slog.Default(), nil)

	ts := httptest.NewServer(server)
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}

// TestMCP_BatchRequest verifies that the server handles batch JSON-RPC
// requests correctly.
func TestMCP_BatchRequest(t *testing.T) {
	server := mcp.NewServer(slog.Default(), nil)
	registry := mcp.NewToolRegistry(slog.Default())

	err := registry.Register(&mcp.Tool{
		Name:        "batch_tool",
		Description: "Tool for batch test",
		InputSchema: json.RawMessage(`{"type":"object"}`),
		Upstream:    "test",
		Operation:   "GET /batch",
	})
	if err != nil {
		t.Fatalf("failed to register tool: %v", err)
	}

	server.RegisterToolsHandlers(registry)

	ts := httptest.NewServer(server)
	defer ts.Close()

	batchReq := []map[string]interface{}{
		{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]interface{}{
				"protocolVersion": "2024-11-05",
			},
		},
		{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/list",
		},
	}

	body, _ := json.Marshal(batchReq)
	resp, err := http.Post(ts.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	respBody := readBody(t, resp)

	assertStatusCode(t, resp, http.StatusOK)

	// The response should be a JSON array with two items.
	var batchResp []json.RawMessage
	if err := json.Unmarshal([]byte(respBody), &batchResp); err != nil {
		t.Fatalf("failed to parse batch response: %v\nbody: %s", err, respBody)
	}
	if len(batchResp) != 2 {
		t.Errorf("expected 2 responses in batch, got %d", len(batchResp))
	}
}

// TestMCP_TableDriven runs a table of JSON-RPC requests through the MCP
// server and verifies the responses.
func TestMCP_TableDriven(t *testing.T) {
	server := mcp.NewServer(slog.Default(), nil)
	registry := mcp.NewToolRegistry(slog.Default())

	_ = registry.Register(&mcp.Tool{
		Name:        "echo_tool",
		Description: "Echo tool for testing",
		InputSchema: json.RawMessage(`{"type":"object"}`),
		Upstream:    "test",
		Operation:   "GET /echo",
	})

	server.RegisterToolsHandlers(registry)

	ts := httptest.NewServer(server)
	defer ts.Close()

	tests := []struct {
		name       string
		request    map[string]interface{}
		wantInBody string
	}{
		{
			name: "initialize",
			request: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "initialize",
				"params":  map[string]interface{}{"protocolVersion": "2024-11-05"},
			},
			wantInBody: "protocolVersion",
		},
		{
			name: "tools/list",
			request: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      2,
				"method":  "tools/list",
			},
			wantInBody: "echo_tool",
		},
		{
			name: "tools/call existing",
			request: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      3,
				"method":  "tools/call",
				"params":  map[string]interface{}{"name": "echo_tool"},
			},
			wantInBody: "echo_tool",
		},
		{
			name: "tools/call missing",
			request: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      4,
				"method":  "tools/call",
				"params":  map[string]interface{}{"name": "does_not_exist"},
			},
			wantInBody: "not found",
		},
		{
			name: "unknown method",
			request: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      5,
				"method":  "resources/unknown",
			},
			wantInBody: "Method not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			resp, err := http.Post(ts.URL, "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			respBody := readBody(t, resp)

			assertStatusCode(t, resp, http.StatusOK)
			assertContains(t, respBody, tt.wantInBody)
		})
	}
}
