package gql2rest

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestUnwrapResponse(t *testing.T) {
	tests := []struct {
		name         string
		response     string
		fieldName    string
		expectedData interface{}
		expectedErr  bool
		expectedCode int
	}{
		{
			name:         "successful response",
			response:     `{"data":{"getUser":{"id":"123","name":"John"}}}`,
			fieldName:    "getUser",
			expectedData: map[string]interface{}{"id": "123", "name": "John"},
			expectedErr:  false,
			expectedCode: http.StatusOK,
		},
		{
			name:         "response with null data",
			response:     `{"data":{"getUser":null}}`,
			fieldName:    "getUser",
			expectedData: nil,
			expectedErr:  false,
			expectedCode: http.StatusOK,
		},
		{
			name:         "GraphQL error with code",
			response:     `{"data":null,"errors":[{"message":"User not found","extensions":{"code":"NOT_FOUND"}}]}`,
			fieldName:    "getUser",
			expectedData: nil,
			expectedErr:  true,
			expectedCode: http.StatusNotFound,
		},
		{
			name:         "GraphQL error without code",
			response:     `{"data":null,"errors":[{"message":"Something went wrong"}]}`,
			fieldName:    "getUser",
			expectedData: nil,
			expectedErr:  true,
			expectedCode: http.StatusInternalServerError,
		},
		{
			name:         "unauthorized error",
			response:     `{"data":null,"errors":[{"message":"Not authenticated","extensions":{"code":"UNAUTHORIZED"}}]}`,
			fieldName:    "getUser",
			expectedData: nil,
			expectedErr:  true,
			expectedCode: http.StatusUnauthorized,
		},
		{
			name:         "forbidden error",
			response:     `{"data":null,"errors":[{"message":"Permission denied","extensions":{"code":"FORBIDDEN"}}]}`,
			fieldName:    "getUser",
			expectedData: nil,
			expectedErr:  true,
			expectedCode: http.StatusForbidden,
		},
		{
			name:         "validation error",
			response:     `{"data":null,"errors":[{"message":"Invalid input","extensions":{"code":"VALIDATION_FAILED"}}]}`,
			fieldName:    "createUser",
			expectedData: nil,
			expectedErr:  true,
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := UnwrapResponse([]byte(tt.response), tt.fieldName)
			if err != nil {
				t.Fatalf("UnwrapResponse failed: %v", err)
			}

			if result.Status != tt.expectedCode {
				t.Errorf("Status = %d, want %d", result.Status, tt.expectedCode)
			}

			if tt.expectedErr {
				if result.Error == nil {
					t.Error("Expected error, got nil")
				} else if result.Error.Message == "" {
					t.Error("Expected error message, got empty")
				}
			} else {
				if result.Error != nil {
					t.Errorf("Expected no error, got: %v", result.Error)
				}
			}
		})
	}
}

func TestMarshalRESTResponse(t *testing.T) {
	tests := []struct {
		name     string
		resp     *UnwrappedResponse
		expected string
	}{
		{
			name: "data response",
			resp: &UnwrappedResponse{
				Data: map[string]interface{}{"id": "123", "name": "John"},
			},
			expected: `{"id":"123","name":"John"}`,
		},
		{
			name: "error response",
			resp: &UnwrappedResponse{
				Error: &RESTError{
					Code:    "NOT_FOUND",
					Message: "User not found",
				},
			},
			expected: `{"code":"NOT_FOUND","message":"User not found"}`,
		},
		{
			name: "null data",
			resp: &UnwrappedResponse{
				Data: nil,
			},
			expected: "null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := MarshalRESTResponse(tt.resp)
			if err != nil {
				t.Fatalf("MarshalRESTResponse failed: %v", err)
			}

			// Parse both for comparison (ignores key order)
			var result, expected interface{}
			json.Unmarshal(data, &result)
			json.Unmarshal([]byte(tt.expected), &expected)

			if !jsonEqual(result, expected) {
				t.Errorf("MarshalRESTResponse() = %s, want %s", string(data), tt.expected)
			}
		})
	}
}

func TestMapErrorCodeToStatus(t *testing.T) {
	tests := []struct {
		code     string
		message  string
		expected int
	}{
		{"UNAUTHORIZED", "Not authenticated", http.StatusUnauthorized},
		{"FORBIDDEN", "Permission denied", http.StatusForbidden},
		{"NOT_FOUND", "Resource not found", http.StatusNotFound},
		{"VALIDATION_FAILED", "Invalid input", http.StatusBadRequest},
		{"INTERNAL_ERROR", "Server error", http.StatusInternalServerError},
		{"TIMEOUT", "Request timed out", http.StatusGatewayTimeout},
		{"RATE_LIMITED", "Too many requests", http.StatusTooManyRequests},
		{"", "Not found", http.StatusNotFound},               // message pattern matching
		{"", "Unauthorized access", http.StatusUnauthorized}, // message pattern matching
		{"", "Some other error", http.StatusInternalServerError}, // default
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			err := GraphQLError{
				Message:   tt.message,
				Extensions: map[string]interface{}{},
			}
			if tt.code != "" {
				err.Extensions["code"] = tt.code
			}

			result := mapErrorCodeToStatus(err)
			if result != tt.expected {
				t.Errorf("mapErrorCodeToStatus() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestUnwrappedResponse_IsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		resp     *UnwrappedResponse
		expected bool
	}{
		{"empty", &UnwrappedResponse{}, true},
		{"with data", &UnwrappedResponse{Data: "something"}, false},
		{"with error", &UnwrappedResponse{Error: &RESTError{}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.resp.IsEmpty() != tt.expected {
				t.Errorf("IsEmpty() = %v, want %v", tt.resp.IsEmpty(), tt.expected)
			}
		})
	}
}

// Helper functions

func jsonEqual(a, b interface{}) bool {
	// Simple comparison - in production use proper JSON comparison
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return string(aJSON) == string(bJSON)
}
