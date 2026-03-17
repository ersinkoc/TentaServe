package rest2gql

import (
	"net/http"
	"testing"
)

// TestNewErrorMapper tests mapper creation.
func TestNewErrorMapper(t *testing.T) {
	mapper := NewErrorMapper(ErrorMapperOptions{
		IncludeUpstreamBody: true,
		CustomMappings:      map[int]string{418: "I_AM_A_TEAPOT"},
	})

	if mapper == nil {
		t.Fatal("Expected ErrorMapper to be created")
	}

	if !mapper.includeUpstreamBody {
		t.Error("Expected includeUpstreamBody to be true")
	}

	if mapper.customMappings[418] != "I_AM_A_TEAPOT" {
		t.Error("Expected custom mapping to be set")
	}
}

// TestErrorMapper_MapRESTError_DefaultMappings tests default error mappings.
func TestErrorMapper_MapRESTError_DefaultMappings(t *testing.T) {
	mapper := NewErrorMapper(ErrorMapperOptions{})

	tests := []struct {
		statusCode int
		expectedCode string
	}{
		{http.StatusBadRequest, "BAD_REQUEST"},
		{http.StatusUnauthorized, "UNAUTHORIZED"},
		{http.StatusForbidden, "FORBIDDEN"},
		{http.StatusNotFound, "NOT_FOUND"},
		{http.StatusConflict, "CONFLICT"},
		{http.StatusTooManyRequests, "RATE_LIMITED"},
		{http.StatusInternalServerError, "INTERNAL_ERROR"},
		{http.StatusBadGateway, "UPSTREAM_ERROR"},
		{http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE"},
		{http.StatusGatewayTimeout, "TIMEOUT"},
		{999, "UNKNOWN_ERROR"}, // Unknown status
	}

	for _, tt := range tests {
		t.Run(tt.expectedCode, func(t *testing.T) {
			err := mapper.MapRESTError(tt.statusCode, nil, "test-upstream")

			if err.Extensions["code"] != tt.expectedCode {
				t.Errorf("Expected code %s, got %v", tt.expectedCode, err.Extensions["code"])
			}

			if err.Extensions["status"] != tt.statusCode {
				t.Errorf("Expected status %d, got %v", tt.statusCode, err.Extensions["status"])
			}

			if err.Extensions["upstream"] != "test-upstream" {
				t.Errorf("Expected upstream 'test-upstream', got %v", err.Extensions["upstream"])
			}
		})
	}
}

// TestErrorMapper_MapRESTError_CustomMappings tests custom error mappings.
func TestErrorMapper_MapRESTError_CustomMappings(t *testing.T) {
	mapper := NewErrorMapper(ErrorMapperOptions{
		CustomMappings: map[int]string{
			418: "I_AM_A_TEAPOT",
			503: "CUSTOM_UNAVAILABLE",
		},
	})

	// Custom mapping
	err := mapper.MapRESTError(418, nil, "test")
	if err.Extensions["code"] != "I_AM_A_TEAPOT" {
		t.Errorf("Expected custom code, got %v", err.Extensions["code"])
	}

	// Overridden mapping
	err = mapper.MapRESTError(503, nil, "test")
	if err.Extensions["code"] != "CUSTOM_UNAVAILABLE" {
		t.Errorf("Expected custom code, got %v", err.Extensions["code"])
	}

	// Default mapping (not overridden)
	err = mapper.MapRESTError(404, nil, "test")
	if err.Extensions["code"] != "NOT_FOUND" {
		t.Errorf("Expected NOT_FOUND, got %v", err.Extensions["code"])
	}
}

// TestErrorMapper_MapRESTError_WithBody tests error with upstream body.
func TestErrorMapper_MapRESTError_WithBody(t *testing.T) {
	mapper := NewErrorMapper(ErrorMapperOptions{
		IncludeUpstreamBody: true,
	})

	body := []byte(`{"message": "User not found", "code": "USER_NOT_FOUND"}`)
	err := mapper.MapRESTError(404, body, "user-service")

	if err.Message != "User not found" {
		t.Errorf("Expected message 'User not found', got %s", err.Message)
	}

	if err.Extensions["upstreamBody"] == nil {
		t.Error("Expected upstreamBody to be included")
	}
}

// TestErrorMapper_MapRESTError_WithoutBody tests error without upstream body.
func TestErrorMapper_MapRESTError_WithoutBody(t *testing.T) {
	mapper := NewErrorMapper(ErrorMapperOptions{
		IncludeUpstreamBody: false,
	})

	body := []byte(`{"message": "Error"}`)
	err := mapper.MapRESTError(500, body, "test")

	if err.Extensions["upstreamBody"] != nil {
		t.Error("Expected upstreamBody to NOT be included")
	}
}

// TestErrorMapper_IsRetryableError tests retryable error detection.
func TestErrorMapper_IsRetryableError(t *testing.T) {
	mapper := NewErrorMapper(ErrorMapperOptions{})

	// Retryable errors
	retryable := []int{502, 503, 504}
	for _, code := range retryable {
		if !mapper.IsRetryableError(code) {
			t.Errorf("Expected %d to be retryable", code)
		}
	}

	// Non-retryable errors
	nonRetryable := []int{400, 401, 403, 404, 500, 429}
	for _, code := range nonRetryable {
		if mapper.IsRetryableError(code) {
			t.Errorf("Expected %d to NOT be retryable", code)
		}
	}
}

// TestErrorMapper_ShouldReturnNull tests null data determination.
func TestErrorMapper_ShouldReturnNull(t *testing.T) {
	mapper := NewErrorMapper(ErrorMapperOptions{})

	// 404 should return null
	if !mapper.ShouldReturnNull(404) {
		t.Error("Expected 404 to return null")
	}

	// Other errors should not
	if mapper.ShouldReturnNull(400) {
		t.Error("Expected 400 to NOT return null")
	}
	if mapper.ShouldReturnNull(500) {
		t.Error("Expected 500 to NOT return null")
	}
}

// TestErrorMapper_ExecutionResult tests execution result creation.
func TestErrorMapper_ExecutionResult(t *testing.T) {
	mapper := NewErrorMapper(ErrorMapperOptions{})

	// 404 - null + error
	result := mapper.ExecutionResult(404, nil, "test")
	if len(result.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(result.Errors))
	}
	if result.Errors[0].Extensions["code"] != "NOT_FOUND" {
		t.Errorf("Expected NOT_FOUND code, got %v", result.Errors[0].Extensions["code"])
	}

	// 500 - error only
	result = mapper.ExecutionResult(500, nil, "test")
	if len(result.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(result.Errors))
	}
	if result.Errors[0].Extensions["code"] != "INTERNAL_ERROR" {
		t.Errorf("Expected INTERNAL_ERROR code, got %v", result.Errors[0].Extensions["code"])
	}
}

// TestExtractField tests field extraction from JSON.
func TestExtractField(t *testing.T) {
	tests := []struct {
		body     string
		field    string
		expected string
	}{
		{`{"message": "Hello"}`, "message", "Hello"},
		{`{"error": "Oops"}`, "error", "Oops"},
		{`{"message":"No spaces"}`, "message", "No spaces"},
		{`{"other": "value"}`, "message", ""},
		{`{}`, "message", ""},
		{"", "message", ""},
	}

	for _, tt := range tests {
		result := extractField(tt.body, tt.field)
		if result != tt.expected {
			t.Errorf("extractField(%q, %q) = %q, want %q", tt.body, tt.field, result, tt.expected)
		}
	}
}
