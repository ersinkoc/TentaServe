package gql2rest

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ErrorMapper maps GraphQL errors to REST HTTP status codes.
type ErrorMapper struct {
	// CustomMappings allows overriding default mappings
	customMappings map[string]int
}

// ErrorMapperOptions configures the error mapper.
type ErrorMapperOptions struct {
	CustomMappings map[string]int // GraphQL code -> HTTP status
}

// NewErrorMapper creates a new GraphQL-to-REST error mapper.
func NewErrorMapper(opts ErrorMapperOptions) *ErrorMapper {
	return &ErrorMapper{
		customMappings: opts.CustomMappings,
	}
}

// MapGraphQLError maps a GraphQL error to an HTTP status code.
// Per SPECIFICATION.md §7.3: GraphQL error extension → HTTP status mapping.
func (em *ErrorMapper) MapGraphQLError(err GraphQLError) int {
	// Check error extensions for code
	if err.Extensions != nil {
		code, ok := err.Extensions["code"].(string)
		if ok {
			// Check custom mappings first
			if em.customMappings != nil {
				if status, ok := em.customMappings[code]; ok {
					return status
				}
			}

			// Default mappings per SPECIFICATION.md §7.3
			switch code {
			case "UNAUTHORIZED":
				return http.StatusUnauthorized // 401
			case "FORBIDDEN":
				return http.StatusForbidden // 403
			case "NOT_FOUND":
				return http.StatusNotFound // 404
			case "VALIDATION_FAILED":
				return http.StatusBadRequest // 400
			case "BAD_USER_INPUT":
				return http.StatusBadRequest // 400
			case "INTERNAL_ERROR":
				return http.StatusInternalServerError // 500
			case "UPSTREAM_ERROR":
				return http.StatusBadGateway // 502
			case "SERVICE_UNAVAILABLE":
				return http.StatusServiceUnavailable // 503
			case "TIMEOUT":
				return http.StatusGatewayTimeout // 504
			case "RATE_LIMITED":
				return http.StatusTooManyRequests // 429
			case "CONFLICT":
				return http.StatusConflict // 409
			}
		}
	}

	// Check error message for common patterns
	msg := err.Message
	if containsString(strings.ToLower(msg), "not found", "does not exist", "not exist") {
		return http.StatusNotFound // 404
	}
	if containsString(strings.ToLower(msg), "unauthorized", "unauthenticated", "not authenticated") {
		return http.StatusUnauthorized // 401
	}
	if containsString(strings.ToLower(msg), "forbidden", "permission denied", "access denied") {
		return http.StatusForbidden // 403
	}
	if containsString(strings.ToLower(msg), "validation", "invalid input", "bad input") {
		return http.StatusBadRequest // 400
	}
	if containsString(strings.ToLower(msg), "timeout", "timed out") {
		return http.StatusGatewayTimeout // 504
	}
	if containsString(strings.ToLower(msg), "rate limit", "too many requests") {
		return http.StatusTooManyRequests // 429
	}

	// Default to internal server error for GraphQL errors
	return http.StatusInternalServerError // 500
}

// MapGraphQLErrorWithBody maps a GraphQL error and returns the HTTP status and JSON body.
func (em *ErrorMapper) MapGraphQLErrorWithBody(err GraphQLError) (int, []byte) {
	status := em.MapGraphQLError(err)

	restErr := RESTError{
		Code:    em.getCodeFromStatus(status),
		Message: err.Message,
	}

	// Add details from extensions
	if err.Extensions != nil {
		if details, ok := err.Extensions["details"].(string); ok {
			restErr.Details = details
		}
	}

	body, _ := json.Marshal(restErr)
	return status, body
}

// MapMultipleErrors maps multiple GraphQL errors and returns the most appropriate status.
func (em *ErrorMapper) MapMultipleErrors(errors []GraphQLError) int {
	if len(errors) == 0 {
		return http.StatusOK
	}

	// Use the first error's code to determine status
	return em.MapGraphQLError(errors[0])
}

// getCodeFromStatus returns a standard code string from HTTP status.
func (em *ErrorMapper) getCodeFromStatus(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusBadRequest:
		return "VALIDATION_FAILED"
	case http.StatusConflict:
		return "CONFLICT"
	case http.StatusTooManyRequests:
		return "RATE_LIMITED"
	case http.StatusInternalServerError:
		return "INTERNAL_ERROR"
	case http.StatusBadGateway:
		return "UPSTREAM_ERROR"
	case http.StatusServiceUnavailable:
		return "SERVICE_UNAVAILABLE"
	case http.StatusGatewayTimeout:
		return "TIMEOUT"
	default:
		return "INTERNAL_ERROR"
	}
}

// containsString checks if the message contains any of the substrings.
func containsString(msg string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(msg, sub) {
			return true
		}
	}
	return false
}
