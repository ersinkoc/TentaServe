package mcp

import (
	"errors"
	"testing"
)

// TestNewError tests error creation.
func TestNewError(t *testing.T) {
	err := NewError(ErrMethodNotFound, "Method not found")

	if err.Code != ErrMethodNotFound {
		t.Errorf("Expected code %d, got %d", ErrMethodNotFound, err.Code)
	}
	if err.Message != "Method not found" {
		t.Errorf("Expected message 'Method not found', got %s", err.Message)
	}
	if err.Data != nil {
		t.Error("Expected nil data")
	}
}

// TestNewErrorWithData tests error creation with data.
func TestNewErrorWithData(t *testing.T) {
	data := map[string]string{"detail": "extra info"}
	err := NewErrorWithData(ErrInvalidParams, "Invalid params", data)

	if err.Code != ErrInvalidParams {
		t.Errorf("Expected code %d, got %d", ErrInvalidParams, err.Code)
	}
	if err.Message != "Invalid params" {
		t.Errorf("Expected message 'Invalid params', got %s", err.Message)
	}
	if err.Data == nil {
		t.Fatal("Expected data")
	}
}

// TestErrorError tests error string representation.
func TestErrorError(t *testing.T) {
	tests := []struct {
		name     string
		err      *Error
		expected string
	}{
		{
			name:     "without data",
			err:      NewError(ErrMethodNotFound, "Method not found"),
			expected: "JSON-RPC error -32601: Method not found",
		},
		{
			name:     "with data",
			err:      NewErrorWithData(ErrInvalidParams, "Invalid params", "extra"),
			expected: "JSON-RPC error -32602: Invalid params (data: extra)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestErrorFromError tests converting standard error to JSON-RPC error.
func TestErrorFromError(t *testing.T) {
	// Nil error
	if err := ErrorFromError(nil); err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	// Standard error
	stdErr := errors.New("something failed")
	rpcErr := ErrorFromError(stdErr)
	if rpcErr == nil {
		t.Fatal("Expected non-nil error")
	}
	if rpcErr.Code != ErrInternalError {
		t.Errorf("Expected code %d, got %d", ErrInternalError, rpcErr.Code)
	}
	if rpcErr.Message != "something failed" {
		t.Errorf("Expected message 'something failed', got %s", rpcErr.Message)
	}

	// JSON-RPC error (should pass through)
	original := NewError(ErrMethodNotFound, "Not found")
	passed := ErrorFromError(original)
	if passed != original {
		t.Error("Expected same error instance")
	}
}

// TestPredefinedErrors tests predefined error instances.
func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      *Error
		expected int
	}{
		{"ErrParse", ErrParse, ErrParseError},
		{"ErrInvalidReq", ErrInvalidReq, ErrInvalidRequest},
		{"ErrMethodMissing", ErrMethodMissing, ErrMethodNotFound},
		{"ErrParamsInvalid", ErrParamsInvalid, ErrInvalidParams},
		{"ErrInternal", ErrInternal, ErrInternalError},
		{"ErrServer", ErrServer, ErrServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.expected {
				t.Errorf("Expected code %d, got %d", tt.expected, tt.err.Code)
			}
		})
	}
}

// TestErrorCodes tests error code constants.
func TestErrorCodes(t *testing.T) {
	// Standard JSON-RPC error codes
	if ErrParseError != -32700 {
		t.Errorf("Expected ErrParseError = -32700, got %d", ErrParseError)
	}
	if ErrInvalidRequest != -32600 {
		t.Errorf("Expected ErrInvalidRequest = -32600, got %d", ErrInvalidRequest)
	}
	if ErrMethodNotFound != -32601 {
		t.Errorf("Expected ErrMethodNotFound = -32601, got %d", ErrMethodNotFound)
	}
	if ErrInvalidParams != -32602 {
		t.Errorf("Expected ErrInvalidParams = -32602, got %d", ErrInvalidParams)
	}
	if ErrInternalError != -32603 {
		t.Errorf("Expected ErrInternalError = -32603, got %d", ErrInternalError)
	}
	if ErrServerError != -32000 {
		t.Errorf("Expected ErrServerError = -32000, got %d", ErrServerError)
	}
}
