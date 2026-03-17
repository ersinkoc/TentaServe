package rest2gql

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/ersinkoc/tentaserve/internal/graphql"
)

// ErrorMapper maps REST HTTP errors to GraphQL errors.
type ErrorMapper struct {
	// IncludeUpstreamBody includes the upstream response body in the error extensions
	includeUpstreamBody bool

	// CustomMappings allows overriding default mappings
	customMappings map[int]string
}

// ErrorMapperOptions configures the error mapper.
type ErrorMapperOptions struct {
	IncludeUpstreamBody bool
	CustomMappings      map[int]string
}

// NewErrorMapper creates a new REST-to-GraphQL error mapper.
func NewErrorMapper(opts ErrorMapperOptions) *ErrorMapper {
	return &ErrorMapper{
		includeUpstreamBody: opts.IncludeUpstreamBody,
		customMappings:      opts.CustomMappings,
	}
}

// MapRESTError maps a REST HTTP error to a GraphQL error.
// Per SPECIFICATION.md §6.4: REST status → GraphQL error mapping.
func (em *ErrorMapper) MapRESTError(statusCode int, body []byte, upstreamName string) *graphql.ExecutionError {
	// Check custom mappings first
	if em.customMappings != nil {
		if code, ok := em.customMappings[statusCode]; ok {
			return em.createError(statusCode, code, body, upstreamName)
		}
	}

	// Default mappings per SPECIFICATION.md §6.4
	var code string
	switch statusCode {
	case http.StatusBadRequest: // 400
		code = "BAD_REQUEST"
	case http.StatusUnauthorized: // 401
		code = "UNAUTHORIZED"
	case http.StatusForbidden: // 403
		code = "FORBIDDEN"
	case http.StatusNotFound: // 404
		code = "NOT_FOUND"
	case http.StatusConflict: // 409
		code = "CONFLICT"
	case http.StatusTooManyRequests: // 429
		code = "RATE_LIMITED"
	case http.StatusInternalServerError: // 500
		code = "INTERNAL_ERROR"
	case http.StatusBadGateway: // 502
		code = "UPSTREAM_ERROR"
	case http.StatusServiceUnavailable: // 503
		code = "SERVICE_UNAVAILABLE"
	case http.StatusGatewayTimeout: // 504
		code = "TIMEOUT"
	default:
		code = "UNKNOWN_ERROR"
	}

	return em.createError(statusCode, code, body, upstreamName)
}

// createError creates a GraphQL execution error.
func (em *ErrorMapper) createError(statusCode int, code string, body []byte, upstreamName string) *graphql.ExecutionError {
	message := em.formatMessage(statusCode, code, body)

	extensions := map[string]interface{}{
		"code":     code,
		"status":   statusCode,
		"upstream": upstreamName,
	}

	// Include upstream body if configured
	if em.includeUpstreamBody && len(body) > 0 {
		// Limit body size to prevent huge error messages
		bodyStr := string(body)
		if len(bodyStr) > 1000 {
			bodyStr = bodyStr[:1000] + "..."
		}
		extensions["upstreamBody"] = bodyStr
	}

	return &graphql.ExecutionError{
		Message:    message,
		Extensions: extensions,
	}
}

// formatMessage creates a human-readable error message.
func (em *ErrorMapper) formatMessage(statusCode int, code string, body []byte) string {
	// Try to extract message from body if it's JSON
	if len(body) > 0 {
		// Simple extraction - look for "message" or "error" fields
		bodyStr := string(body)
		if msg := extractField(bodyStr, "message"); msg != "" {
			return msg
		}
		if msg := extractField(bodyStr, "error"); msg != "" {
			return msg
		}
	}

	// Default message based on status code
	return fmt.Sprintf("Upstream error: %s (HTTP %d)", code, statusCode)
}

// extractField extracts a string field from JSON-like body.
func extractField(body, field string) string {
	// Simple extraction - look for "field": "value" pattern
	pattern := fmt.Sprintf(`"%s"`, field)
	idx := strings.Index(body, pattern)
	if idx == -1 {
		return ""
	}

	// Find value after field
	valStart := idx + len(pattern)
	for valStart < len(body) && (body[valStart] == ':' || body[valStart] == ' ' || body[valStart] == '\t') {
		valStart++
	}

	// Skip quotes if present
	if valStart < len(body) && body[valStart] == '"' {
		valStart++
		valEnd := valStart
		for valEnd < len(body) && body[valEnd] != '"' {
			valEnd++
		}
		return body[valStart:valEnd]
	}

	return ""
}

// IsRetryableError determines if an error is retryable.
func (em *ErrorMapper) IsRetryableError(statusCode int) bool {
	switch statusCode {
	case http.StatusBadGateway,      // 502
		http.StatusServiceUnavailable, // 503
		http.StatusGatewayTimeout:     // 504
		return true
	default:
		return false
	}
}

// ShouldReturnNull determines if a REST error should result in null data.
// Per SPEC: 404 returns null + error, others return error only.
func (em *ErrorMapper) ShouldReturnNull(statusCode int) bool {
	return statusCode == http.StatusNotFound
}

// ExecutionResult creates an execution result from a REST error.
func (em *ErrorMapper) ExecutionResult(statusCode int, body []byte, upstreamName string) *graphql.ExecutionResult {
	err := em.MapRESTError(statusCode, body, upstreamName)

	result := &graphql.ExecutionResult{
		Errors: []graphql.ExecutionError{*err},
	}

	// For 404, data is null
	if em.ShouldReturnNull(statusCode) {
		result.Data = nil
	}

	return result
}
