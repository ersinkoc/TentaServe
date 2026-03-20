package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestRequestIDString tests string conversion of request IDs.
func TestRequestIDString(t *testing.T) {
	tests := []struct {
		name     string
		id       any
		expected string
	}{
		{"string", "test-id", "test-id"},
		{"int", 42, "42"},
		{"int64", int64(123), "123"},
		{"float64", float64(3.14), "3.14"},
		{"json.Number", json.Number("999"), "999"},
		{"nil", nil, "null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rid := NewRequestID(tt.id)
			result := rid.String()
			if result != tt.expected {
				t.Errorf("String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestRequestIDIsNull tests null check.
func TestRequestIDIsNull(t *testing.T) {
	// Nil ID
	var nilID *RequestID
	if !nilID.IsNull() {
		t.Error("Expected nil ID to be null")
	}

	// Nil value - NewRequestID(nil) returns nil, so we need to handle this differently
	nullID := &RequestID{value: nil}
	if !nullID.IsNull() {
		t.Error("Expected nil value to be null")
	}

	// Non-nil value
	id := NewRequestID("test")
	if id.IsNull() {
		t.Error("Expected non-nil value to not be null")
	}
}

// TestRequestIDMarshalJSON tests JSON marshaling.
func TestRequestIDMarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		id       *RequestID
		expected string
	}{
		{"string", NewRequestID("abc"), `"abc"`},
		{"number", NewRequestID(42), `42`},
		{"null", NewRequestID(nil), "null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.id)
			if err != nil {
				t.Fatalf("MarshalJSON failed: %v", err)
			}
			if string(data) != tt.expected {
				t.Errorf("MarshalJSON() = %q, want %q", string(data), tt.expected)
			}
		})
	}
}

// TestRequestIDUnmarshalJSON tests JSON unmarshaling.
func TestRequestIDUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		isNull   bool
		expected string
	}{
		{"string", `"abc"`, false, "abc"},
		{"number", `42`, false, "42"},
		{"null", `null`, true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rid RequestID
			err := json.Unmarshal([]byte(tt.data), &rid)
			if err != nil {
				t.Fatalf("UnmarshalJSON failed: %v", err)
			}
			if tt.isNull {
				if !rid.IsNull() {
					t.Errorf("UnmarshalJSON() expected null, got %v", rid.Value())
				}
			} else {
				if rid.String() != tt.expected {
					t.Errorf("UnmarshalJSON() got %v, want %v", rid.String(), tt.expected)
				}
			}
		})
	}
}

// TestRequestIsValid tests request validation.
func TestRequestIsValid(t *testing.T) {
	tests := []struct {
		name    string
		request Request
		valid   bool
	}{
		{
			name:    "valid",
			request: Request{JSONRPC: "2.0", Method: "test"},
			valid:   true,
		},
		{
			name:    "missing jsonrpc",
			request: Request{Method: "test"},
			valid:   false,
		},
		{
			name:    "missing method",
			request: Request{JSONRPC: "2.0"},
			valid:   false,
		},
		{
			name:    "wrong jsonrpc version",
			request: Request{JSONRPC: "1.0", Method: "test"},
			valid:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.request.IsValid(); got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

// TestRequestIsNotification tests notification detection.
func TestRequestIsNotification(t *testing.T) {
	tests := []struct {
		name     string
		id       *RequestID
		isNotify bool
	}{
		{"no id", nil, true},
		{"null id", NewRequestID(nil), true},
		{"string id", NewRequestID("abc"), false},
		{"number id", NewRequestID(1), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := Request{JSONRPC: "2.0", Method: "test", ID: tt.id}
			if got := req.IsNotification(); got != tt.isNotify {
				t.Errorf("IsNotification() = %v, want %v", got, tt.isNotify)
			}
		})
	}
}

// TestParseRequest tests request parsing.
func TestParseRequest(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name:    "valid",
			json:    `{"jsonrpc":"2.0","method":"test","id":1}`,
			wantErr: false,
		},
		{
			name:    "invalid json",
			json:    `{invalid}`,
			wantErr: true,
		},
		{
			name:    "missing jsonrpc",
			json:    `{"method":"test"}`,
			wantErr: true,
		},
		{
			name:    "missing method",
			json:    `{"jsonrpc":"2.0"}`,
			wantErr: true,
		},
		{
			name:    "wrong jsonrpc",
			json:    `{"jsonrpc":"1.0","method":"test"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := ParseRequest([]byte(tt.json))
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if req.JSONRPC != "2.0" {
				t.Errorf("Expected JSONRPC 2.0, got %s", req.JSONRPC)
			}
			if req.Method != "test" {
				t.Errorf("Expected method test, got %s", req.Method)
			}
		})
	}
}

// TestParseRequestBatch tests batch request parsing.
func TestParseRequestBatch(t *testing.T) {
	tests := []struct {
		name      string
		json      string
		wantCount int
		wantErr   bool
	}{
		{
			name:      "valid batch",
			json:      `[{"jsonrpc":"2.0","method":"test1","id":1},{"jsonrpc":"2.0","method":"test2","id":2}]`,
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "single request",
			json:      `{"jsonrpc":"2.0","method":"test","id":1}`,
			wantCount: 0, // Returns nil for single request
			wantErr:   false,
		},
		{
			name:      "invalid batch",
			json:      `[{"jsonrpc":"2.0"}]`,
			wantCount: 0,
			wantErr:   true,
		},
		{
			name:      "empty batch",
			json:      `[]`,
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqs, err := ParseRequestBatch([]byte(tt.json))
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if len(reqs) != tt.wantCount {
				t.Errorf("Got %d requests, want %d", len(reqs), tt.wantCount)
			}
		})
	}
}

// TestSerializeResponse tests response serialization.
func TestSerializeResponse(t *testing.T) {
	resp := NewSuccessResponse(NewRequestID(1), map[string]string{"result": "ok"})
	data, err := SerializeResponse(resp)
	if err != nil {
		t.Fatalf("SerializeResponse failed: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to parse serialized response: %v", err)
	}

	if parsed["jsonrpc"] != "2.0" {
		t.Errorf("Expected jsonrpc 2.0, got %v", parsed["jsonrpc"])
	}
	if parsed["id"] != float64(1) {
		t.Errorf("Expected id 1, got %v", parsed["id"])
	}
}

// TestSerializeResponseBatch tests batch response serialization.
func TestSerializeResponseBatch(t *testing.T) {
	resps := []*Response{
		NewSuccessResponse(NewRequestID(1), "result1"),
		NewSuccessResponse(NewRequestID(2), "result2"),
	}
	data, err := SerializeResponseBatch(resps)
	if err != nil {
		t.Fatalf("SerializeResponseBatch failed: %v", err)
	}

	var parsed []map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to parse serialized batch: %v", err)
	}

	if len(parsed) != 2 {
		t.Errorf("Got %d responses, want 2", len(parsed))
	}
}

// TestNewSuccessResponse tests creating success responses.
func TestNewSuccessResponse(t *testing.T) {
	id := NewRequestID(123)
	result := map[string]string{"message": "hello"}
	resp := NewSuccessResponse(id, result)

	if resp.JSONRPC != "2.0" {
		t.Error("Expected jsonrpc 2.0")
	}
	if resp.Error != nil {
		t.Error("Expected no error")
	}
	if resp.Result == nil {
		t.Fatal("Expected result")
	}
}

// TestNewErrorResponse tests creating error responses.
func TestNewErrorResponse(t *testing.T) {
	id := NewRequestID(456)
	err := NewError(ErrMethodNotFound, "Method not found")
	resp := NewErrorResponse(id, err)

	if resp.JSONRPC != "2.0" {
		t.Error("Expected jsonrpc 2.0")
	}
	if resp.Result != nil {
		t.Error("Expected no result")
	}
	if resp.Error == nil {
		t.Fatal("Expected error")
	}
	if resp.Error.Code != ErrMethodNotFound {
		t.Errorf("Expected code %d, got %d", ErrMethodNotFound, resp.Error.Code)
	}
}

// TestRequestNewResponse tests request response creation.
func TestRequestNewResponse(t *testing.T) {
	req := &Request{
		JSONRPC: "2.0",
		Method:  "test",
		ID:      NewRequestID(789),
	}

	result := "success"
	resp := req.NewResponse(result)

	if resp.JSONRPC != "2.0" {
		t.Error("Expected jsonrpc 2.0")
	}
	if resp.Result != result {
		t.Errorf("Expected result %v, got %v", result, resp.Result)
	}
	if resp.ID.String() != "789" {
		t.Errorf("Expected id 789, got %s", resp.ID.String())
	}
}

// TestRequestNewErrorResponse tests request error response creation.
func TestRequestNewErrorResponse(t *testing.T) {
	req := &Request{
		JSONRPC: "2.0",
		Method:  "test",
		ID:      NewRequestID("abc"),
	}

	rpcErr := NewError(ErrInvalidParams, "bad params")
	resp := req.NewErrorResponse(rpcErr)

	if resp.Error == nil {
		t.Fatal("Expected error")
	}
	if resp.Error.Message != "bad params" {
		t.Errorf("Expected message 'bad params', got %s", resp.Error.Message)
	}
	if resp.ID.String() != "abc" {
		t.Errorf("Expected id abc, got %s", resp.ID.String())
	}
}

// --- Additional tests for coverage ---

func TestRequestIDValue(t *testing.T) {
	tests := []struct {
		name    string
		id      *RequestID
		wantNil bool
	}{
		{"nil ID", nil, true},
		{"string value", NewRequestID("test"), false},
		{"int value", NewRequestID(42), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := tt.id.Value()
			if tt.wantNil {
				if val != nil {
					t.Errorf("expected nil, got %v", val)
				}
			} else {
				if val == nil {
					t.Error("expected non-nil value")
				}
			}
		})
	}
}

func TestRequestIDString_UnknownType(t *testing.T) {
	rid := &RequestID{value: struct{}{}}
	result := rid.String()
	if result != "" {
		t.Errorf("expected empty string for unknown type, got %q", result)
	}
}

func TestParseRequestFromReader(t *testing.T) {
	t.Run("valid request", func(t *testing.T) {
		reader := strings.NewReader(`{"jsonrpc":"2.0","method":"test","id":1}`)
		req, err := ParseRequestFromReader(reader)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.Method != "test" {
			t.Errorf("expected method test, got %s", req.Method)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		reader := strings.NewReader(`{invalid}`)
		_, err := ParseRequestFromReader(reader)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("missing version", func(t *testing.T) {
		reader := strings.NewReader(`{"method":"test"}`)
		_, err := ParseRequestFromReader(reader)
		if err == nil {
			t.Error("expected error for missing jsonrpc version")
		}
	})
}

func TestParseRequestBatchFromReader(t *testing.T) {
	t.Run("valid batch", func(t *testing.T) {
		reader := strings.NewReader(`[{"jsonrpc":"2.0","method":"test1","id":1},{"jsonrpc":"2.0","method":"test2","id":2}]`)
		reqs, err := ParseRequestBatchFromReader(reader)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(reqs) != 2 {
			t.Errorf("expected 2 requests, got %d", len(reqs))
		}
	})

	t.Run("single request returns nil", func(t *testing.T) {
		reader := strings.NewReader(`{"jsonrpc":"2.0","method":"test","id":1}`)
		reqs, err := ParseRequestBatchFromReader(reader)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if reqs != nil {
			t.Error("expected nil for single request")
		}
	})

	t.Run("empty data", func(t *testing.T) {
		reader := strings.NewReader(``)
		reqs, err := ParseRequestBatchFromReader(reader)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if reqs != nil {
			t.Error("expected nil for empty data")
		}
	})
}
