package gql2rest

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestNewErrorMapper(t *testing.T) {
	t.Run("with custom mappings", func(t *testing.T) {
		em := NewErrorMapper(ErrorMapperOptions{
			CustomMappings: map[string]int{
				"CUSTOM_ERROR": 418,
			},
		})
		if em == nil {
			t.Fatal("NewErrorMapper returned nil")
		}
		if em.customMappings == nil {
			t.Fatal("customMappings should not be nil")
		}
		if em.customMappings["CUSTOM_ERROR"] != 418 {
			t.Errorf("expected custom mapping 418, got %d", em.customMappings["CUSTOM_ERROR"])
		}
	})

	t.Run("without custom mappings", func(t *testing.T) {
		em := NewErrorMapper(ErrorMapperOptions{})
		if em == nil {
			t.Fatal("NewErrorMapper returned nil")
		}
		if em.customMappings != nil {
			t.Error("expected nil customMappings when none provided")
		}
	})
}

func TestMapGraphQLError_ExtensionCodes(t *testing.T) {
	em := NewErrorMapper(ErrorMapperOptions{})

	tests := []struct {
		name     string
		code     string
		expected int
	}{
		{"UNAUTHORIZED", "UNAUTHORIZED", http.StatusUnauthorized},
		{"FORBIDDEN", "FORBIDDEN", http.StatusForbidden},
		{"NOT_FOUND", "NOT_FOUND", http.StatusNotFound},
		{"VALIDATION_FAILED", "VALIDATION_FAILED", http.StatusBadRequest},
		{"BAD_USER_INPUT", "BAD_USER_INPUT", http.StatusBadRequest},
		{"INTERNAL_ERROR", "INTERNAL_ERROR", http.StatusInternalServerError},
		{"UPSTREAM_ERROR", "UPSTREAM_ERROR", http.StatusBadGateway},
		{"SERVICE_UNAVAILABLE", "SERVICE_UNAVAILABLE", http.StatusServiceUnavailable},
		{"TIMEOUT", "TIMEOUT", http.StatusGatewayTimeout},
		{"RATE_LIMITED", "RATE_LIMITED", http.StatusTooManyRequests},
		{"CONFLICT", "CONFLICT", http.StatusConflict},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := GraphQLError{
				Message:    "test error",
				Extensions: map[string]interface{}{"code": tt.code},
			}
			status := em.MapGraphQLError(err)
			if status != tt.expected {
				t.Errorf("MapGraphQLError(code=%s) = %d, want %d", tt.code, status, tt.expected)
			}
		})
	}
}

func TestMapGraphQLError_CustomMappings(t *testing.T) {
	em := NewErrorMapper(ErrorMapperOptions{
		CustomMappings: map[string]int{
			"CUSTOM_CODE": 418,
			"NOT_FOUND":   422, // override default
		},
	})

	t.Run("custom code", func(t *testing.T) {
		err := GraphQLError{
			Message:    "teapot",
			Extensions: map[string]interface{}{"code": "CUSTOM_CODE"},
		}
		status := em.MapGraphQLError(err)
		if status != 418 {
			t.Errorf("expected 418, got %d", status)
		}
	})

	t.Run("overridden default", func(t *testing.T) {
		err := GraphQLError{
			Message:    "not found override",
			Extensions: map[string]interface{}{"code": "NOT_FOUND"},
		}
		status := em.MapGraphQLError(err)
		if status != 422 {
			t.Errorf("expected 422, got %d", status)
		}
	})
}

func TestMapGraphQLError_MessagePatterns(t *testing.T) {
	em := NewErrorMapper(ErrorMapperOptions{})

	tests := []struct {
		name     string
		message  string
		expected int
	}{
		{"not found", "Resource not found", http.StatusNotFound},
		{"does not exist", "User does not exist", http.StatusNotFound},
		{"not exist", "Record not exist", http.StatusNotFound},
		{"unauthorized", "Unauthorized access", http.StatusUnauthorized},
		{"unauthenticated", "User unauthenticated", http.StatusUnauthorized},
		{"not authenticated", "not authenticated", http.StatusUnauthorized},
		{"forbidden", "forbidden resource", http.StatusForbidden},
		{"permission denied", "permission denied for user", http.StatusForbidden},
		{"access denied", "access denied", http.StatusForbidden},
		{"validation", "validation error on field", http.StatusBadRequest},
		{"invalid input", "invalid input data", http.StatusBadRequest},
		{"bad input", "bad input supplied", http.StatusBadRequest},
		{"timeout", "operation timeout", http.StatusGatewayTimeout},
		{"timed out", "request timed out", http.StatusGatewayTimeout},
		{"rate limit", "rate limit exceeded", http.StatusTooManyRequests},
		{"too many requests", "too many requests sent", http.StatusTooManyRequests},
		{"generic error", "something went wrong", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := GraphQLError{
				Message: tt.message,
			}
			status := em.MapGraphQLError(err)
			if status != tt.expected {
				t.Errorf("MapGraphQLError(msg=%q) = %d, want %d", tt.message, status, tt.expected)
			}
		})
	}
}

func TestMapGraphQLErrorWithBody(t *testing.T) {
	em := NewErrorMapper(ErrorMapperOptions{})

	t.Run("basic error with body", func(t *testing.T) {
		err := GraphQLError{
			Message:    "User not found",
			Extensions: map[string]interface{}{"code": "NOT_FOUND"},
		}
		status, body := em.MapGraphQLErrorWithBody(err)
		if status != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", status)
		}
		var restErr RESTError
		if e := json.Unmarshal(body, &restErr); e != nil {
			t.Fatalf("failed to unmarshal body: %v", e)
		}
		if restErr.Code != "NOT_FOUND" {
			t.Errorf("expected code NOT_FOUND, got %s", restErr.Code)
		}
		if restErr.Message != "User not found" {
			t.Errorf("expected message 'User not found', got %s", restErr.Message)
		}
	})

	t.Run("error with details extension", func(t *testing.T) {
		err := GraphQLError{
			Message: "Validation failed",
			Extensions: map[string]interface{}{
				"code":    "VALIDATION_FAILED",
				"details": "name field is required",
			},
		}
		status, body := em.MapGraphQLErrorWithBody(err)
		if status != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", status)
		}
		var restErr RESTError
		if e := json.Unmarshal(body, &restErr); e != nil {
			t.Fatalf("failed to unmarshal body: %v", e)
		}
		if restErr.Details != "name field is required" {
			t.Errorf("expected details 'name field is required', got %s", restErr.Details)
		}
	})

	t.Run("error without extensions", func(t *testing.T) {
		err := GraphQLError{
			Message: "something broke",
		}
		status, body := em.MapGraphQLErrorWithBody(err)
		if status != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", status)
		}
		if body == nil {
			t.Fatal("expected non-nil body")
		}
	})
}

func TestMapMultipleErrors(t *testing.T) {
	em := NewErrorMapper(ErrorMapperOptions{})

	t.Run("empty errors", func(t *testing.T) {
		status := em.MapMultipleErrors([]GraphQLError{})
		if status != http.StatusOK {
			t.Errorf("expected 200 for empty errors, got %d", status)
		}
	})

	t.Run("single error", func(t *testing.T) {
		errs := []GraphQLError{
			{Message: "not found", Extensions: map[string]interface{}{"code": "NOT_FOUND"}},
		}
		status := em.MapMultipleErrors(errs)
		if status != http.StatusNotFound {
			t.Errorf("expected 404, got %d", status)
		}
	})

	t.Run("multiple errors uses first", func(t *testing.T) {
		errs := []GraphQLError{
			{Message: "forbidden", Extensions: map[string]interface{}{"code": "FORBIDDEN"}},
			{Message: "not found", Extensions: map[string]interface{}{"code": "NOT_FOUND"}},
		}
		status := em.MapMultipleErrors(errs)
		if status != http.StatusForbidden {
			t.Errorf("expected 403 (first error), got %d", status)
		}
	})
}

func TestGetCodeFromStatus(t *testing.T) {
	em := NewErrorMapper(ErrorMapperOptions{})

	tests := []struct {
		status   int
		expected string
	}{
		{http.StatusUnauthorized, "UNAUTHORIZED"},
		{http.StatusForbidden, "FORBIDDEN"},
		{http.StatusNotFound, "NOT_FOUND"},
		{http.StatusBadRequest, "VALIDATION_FAILED"},
		{http.StatusConflict, "CONFLICT"},
		{http.StatusTooManyRequests, "RATE_LIMITED"},
		{http.StatusInternalServerError, "INTERNAL_ERROR"},
		{http.StatusBadGateway, "UPSTREAM_ERROR"},
		{http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE"},
		{http.StatusGatewayTimeout, "TIMEOUT"},
		{299, "INTERNAL_ERROR"}, // unknown status defaults
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := em.getCodeFromStatus(tt.status)
			if result != tt.expected {
				t.Errorf("getCodeFromStatus(%d) = %q, want %q", tt.status, result, tt.expected)
			}
		})
	}
}

func TestContainsString(t *testing.T) {
	tests := []struct {
		name     string
		msg      string
		subs     []string
		expected bool
	}{
		{"match first", "hello world", []string{"hello", "foo"}, true},
		{"match second", "hello world", []string{"foo", "world"}, true},
		{"no match", "hello world", []string{"foo", "bar"}, false},
		{"empty subs", "hello world", []string{}, false},
		{"empty msg", "", []string{"foo"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsString(tt.msg, tt.subs...)
			if result != tt.expected {
				t.Errorf("containsString(%q, %v) = %v, want %v", tt.msg, tt.subs, result, tt.expected)
			}
		})
	}
}
