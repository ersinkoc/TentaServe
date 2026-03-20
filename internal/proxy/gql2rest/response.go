package gql2rest

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// GraphQLResponse represents a GraphQL response.
type GraphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []GraphQLError  `json:"errors,omitempty"`
}

// GraphQLError represents a GraphQL error with extensions.
type GraphQLError struct {
	Message    string                 `json:"message"`
	Path       []interface{}          `json:"path,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// UnwrappedResponse represents the unwrapped response data.
type UnwrappedResponse struct {
	Data    interface{} `json:"data,omitempty"`
	Error   *RESTError  `json:"error,omitempty"`
	Status  int         `json:"-"`
	Headers http.Header `json:"-"`
}

// RESTError represents an error in REST format.
type RESTError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// UnwrapResponse unwraps a GraphQL response into a REST response.
// It extracts the data from the "data" key and maps GraphQL errors to HTTP status codes.
func UnwrapResponse(graphQLResp []byte, fieldName string) (*UnwrappedResponse, error) {
	var resp GraphQLResponse
	if err := json.Unmarshal(graphQLResp, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse GraphQL response: %w", err)
	}

	// Check for GraphQL errors
	if len(resp.Errors) > 0 {
		return handleGraphQLErrors(resp.Errors)
	}

	// Extract data for the requested field
	var dataMap map[string]json.RawMessage
	if err := json.Unmarshal(resp.Data, &dataMap); err != nil {
		return nil, fmt.Errorf("failed to parse response data: %w", err)
	}

	// Get the specific field data
	fieldData, ok := dataMap[fieldName]
	if !ok {
		// Field not in response - might be null or missing
		return &UnwrappedResponse{
			Data:   nil,
			Status: http.StatusOK,
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
		}, nil
	}

	// Parse the actual data
	var result interface{}
	if err := json.Unmarshal(fieldData, &result); err != nil {
		return nil, fmt.Errorf("failed to parse field data: %w", err)
	}

	return &UnwrappedResponse{
		Data:   result,
		Status: http.StatusOK,
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}, nil
}

// handleGraphQLErrors converts GraphQL errors to a REST error response.
func handleGraphQLErrors(errors []GraphQLError) (*UnwrappedResponse, error) {
	if len(errors) == 0 {
		return nil, nil
	}

	// Use the first error to determine status code
	firstError := errors[0]
	statusCode := mapErrorCodeToStatus(firstError)

	// Build error details
	var details string
	if len(errors) > 1 {
		details = fmt.Sprintf("+%d more errors", len(errors)-1)
	}

	// Check for code in extensions
	code := "GRAPHQL_ERROR"
	if firstError.Extensions != nil {
		if c, ok := firstError.Extensions["code"].(string); ok {
			code = c
		}
	}

	return &UnwrappedResponse{
		Error: &RESTError{
			Code:    code,
			Message: firstError.Message,
			Details: details,
		},
		Status: statusCode,
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}, nil
}

// mapErrorCodeToStatus maps a GraphQL error to an HTTP status code.
// Per SPECIFICATION.md §7.3 table.
func mapErrorCodeToStatus(err GraphQLError) int {
	// Check error extensions for code
	if err.Extensions != nil {
		code, ok := err.Extensions["code"].(string)
		if ok {
			switch code {
			case "UNAUTHORIZED":
				return http.StatusUnauthorized // 401
			case "FORBIDDEN":
				return http.StatusForbidden // 403
			case "NOT_FOUND":
				return http.StatusNotFound // 404
			case "VALIDATION_FAILED":
				return http.StatusBadRequest // 400
			case "INTERNAL_ERROR":
				return http.StatusInternalServerError // 500
			case "TIMEOUT":
				return http.StatusGatewayTimeout // 504
			case "RATE_LIMITED":
				return http.StatusTooManyRequests // 429
			}
		}
	}

	// Check error message for common patterns
	msg := err.Message
	if contains(msg, "not found", "does not exist") {
		return http.StatusNotFound // 404
	}
	if contains(msg, "unauthorized", "unauthenticated") {
		return http.StatusUnauthorized // 401
	}
	if contains(msg, "forbidden", "permission denied") {
		return http.StatusForbidden // 403
	}
	if contains(msg, "validation", "invalid") {
		return http.StatusBadRequest // 400
	}
	if contains(msg, "timeout", "timed out") {
		return http.StatusGatewayTimeout // 504
	}

	// Default to internal server error for GraphQL errors
	return http.StatusInternalServerError // 500
}

// contains checks if the message contains any of the substrings (case-insensitive).
func contains(msg string, subs ...string) bool {
	msgLower := toLower(msg)
	for _, sub := range subs {
		if containsStr(msgLower, toLower(sub)) {
			return true
		}
	}
	return false
}

// toLower converts string to lowercase.
func toLower(s string) string {
	// Simple implementation - in production use strings.ToLower
	var result []rune
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			result = append(result, r+('a'-'A'))
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

// containsStr checks if s contains substr.
func containsStr(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// MarshalRESTResponse marshals an unwrapped response to JSON.
func MarshalRESTResponse(resp *UnwrappedResponse) ([]byte, error) {
	if resp.Error != nil {
		return json.Marshal(resp.Error)
	}
	return json.Marshal(resp.Data)
}

// IsEmpty checks if the response has no data and no error.
func (r *UnwrappedResponse) IsEmpty() bool {
	return r.Error == nil && r.Data == nil
}

// SetHeader sets a response header.
func (r *UnwrappedResponse) SetHeader(key, value string) {
	if r.Headers == nil {
		r.Headers = make(http.Header)
	}
	r.Headers.Set(key, value)
}
