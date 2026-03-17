package mcp

import "fmt"

// JSON-RPC 2.0 error codes as defined in the spec and MCP documentation.
// Standard JSON-RPC error codes:
// -32700: Parse error - Invalid JSON was received by the server.
// -32600: Invalid Request - The JSON sent is not a valid Request object.
// -32601: Method not found - The method does not exist / is not available.
// -32602: Invalid params - Invalid method parameter(s).
// -32603: Internal error - Internal JSON-RPC error.
// -32000 to -32099: Server error - Reserved for implementation-defined server-errors.
const (
	// ErrParseError is returned when the request contains invalid JSON.
	ErrParseError = -32700

	// ErrInvalidRequest is returned when the request is not a valid JSON-RPC request.
	ErrInvalidRequest = -32600

	// ErrMethodNotFound is returned when the requested method doesn't exist.
	ErrMethodNotFound = -32601

	// ErrInvalidParams is returned when the method parameters are invalid.
	ErrInvalidParams = -32602

	// ErrInternalError is returned for internal server errors.
	ErrInternalError = -32603

	// ErrServerError is a generic server error.
	ErrServerError = -32000
)

// Error represents a JSON-RPC error.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Data != nil {
		return fmt.Sprintf("JSON-RPC error %d: %s (data: %v)", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

// NewError creates a new JSON-RPC error with the given code and message.
func NewError(code int, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// NewErrorWithData creates a new JSON-RPC error with additional data.
func NewErrorWithData(code int, message string, data any) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// Predefined errors for common scenarios.
var (
	// ErrParse is returned when JSON parsing fails.
	ErrParse = NewError(ErrParseError, "Parse error")

	// ErrInvalidReq is returned when the request structure is invalid.
	ErrInvalidReq = NewError(ErrInvalidRequest, "Invalid request")

	// ErrMethodMissing is returned when the method is not found.
	ErrMethodMissing = NewError(ErrMethodNotFound, "Method not found")

	// ErrParamsInvalid is returned when parameters are invalid.
	ErrParamsInvalid = NewError(ErrInvalidParams, "Invalid params")

	// ErrInternal is returned for internal errors.
	ErrInternal = NewError(ErrInternalError, "Internal error")

	// ErrServer is returned for generic server errors.
	ErrServer = NewError(ErrServerError, "Server error")
)

// ErrorFromError converts a standard error to a JSON-RPC error.
func ErrorFromError(err error) *Error {
	if err == nil {
		return nil
	}

	// Check if it's already a JSON-RPC error
	if rpcErr, ok := err.(*Error); ok {
		return rpcErr
	}

	return NewError(ErrInternalError, err.Error())
}
