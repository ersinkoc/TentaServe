// Package internal provides shared types and utilities for Tentaserve.
//
// This file defines typed errors and sentinel errors used throughout
// the codebase for consistent error handling.
package internal

import (
	"errors"
	"fmt"
)

// Sentinel errors — used with errors.Is() for control flow.
// These represent well-known failure modes that can be checked explicitly.
var (
	// Upstream errors
	ErrUpstreamTimeout     = errors.New("upstream timeout")
	ErrUpstreamUnavailable = errors.New("upstream unavailable")
	ErrUpstreamNotFound    = errors.New("upstream not found")

	// Gateway errors
	ErrCircuitOpen     = errors.New("circuit breaker open")
	ErrRateLimited     = errors.New("rate limit exceeded")
	ErrAuthFailed      = errors.New("authentication failed")
	ErrCacheMiss       = errors.New("cache miss")
	ErrCacheFull       = errors.New("cache full")

	// Configuration errors
	ErrConfigInvalid   = errors.New("invalid configuration")
	ErrConfigNotFound  = errors.New("configuration file not found")
	ErrConfigParse     = errors.New("configuration parse error")

	// Schema errors
	ErrSchemaInvalid   = errors.New("invalid schema")
	ErrSchemaNotFound  = errors.New("schema not found")
	ErrSchemaMismatch  = errors.New("schema type mismatch")

	// GraphQL errors
	ErrQueryTooComplex = errors.New("query exceeds complexity limit")
	ErrQueryTooDeep    = errors.New("query exceeds depth limit")
	ErrQueryInvalid    = errors.New("invalid query")
	ErrQueryNotFound   = errors.New("field not found in schema")

	// Parsing errors
	ErrParseError      = errors.New("parse error")
	ErrRefNotFound     = errors.New("reference not found")
	ErrCircularRef     = errors.New("circular reference detected")
)

// ErrorCode is a machine-readable error identifier.
type ErrorCode string

const (
	// Upstream error codes
	CodeUpstreamTimeout     ErrorCode = "UPSTREAM_TIMEOUT"
	CodeUpstreamUnavailable ErrorCode = "UPSTREAM_UNAVAILABLE"
	CodeUpstreamNotFound    ErrorCode = "UPSTREAM_NOT_FOUND"

	// Gateway error codes
	CodeCircuitOpen     ErrorCode = "CIRCUIT_OPEN"
	CodeRateLimited     ErrorCode = "RATE_LIMITED"
	CodeAuthFailed      ErrorCode = "AUTH_FAILED"
	CodeCacheMiss       ErrorCode = "CACHE_MISS"

	// Configuration error codes
	CodeConfigInvalid   ErrorCode = "CONFIG_INVALID"
	CodeConfigNotFound  ErrorCode = "CONFIG_NOT_FOUND"
	CodeConfigParse     ErrorCode = "CONFIG_PARSE_ERROR"

	// Schema error codes
	CodeSchemaInvalid   ErrorCode = "SCHEMA_INVALID"
	CodeSchemaNotFound  ErrorCode = "SCHEMA_NOT_FOUND"

	// GraphQL error codes
	CodeQueryTooComplex ErrorCode = "QUERY_TOO_COMPLEX"
	CodeQueryTooDeep    ErrorCode = "QUERY_TOO_DEEP"
	CodeQueryInvalid    ErrorCode = "QUERY_INVALID"

	// General error codes
	CodeInternalError   ErrorCode = "INTERNAL_ERROR"
	CodeNotImplemented  ErrorCode = "NOT_IMPLEMENTED"
)

// TentaserveError is a rich error type that provides context for debugging
// and consistent error responses to clients.
type TentaserveError struct {
	// Code is a machine-readable error identifier.
	Code ErrorCode `json:"code"`

	// Message is a human-readable description of the error.
	Message string `json:"message"`

	// Upstream is the name of the upstream service, if applicable.
	Upstream string `json:"upstream,omitempty"`

	// StatusCode is the suggested HTTP status code for this error.
	StatusCode int `json:"-"`

	// Field is the GraphQL field that caused the error, if applicable.
	Field string `json:"field,omitempty"`

	// Path is the query path to the error, if applicable.
	Path []string `json:"path,omitempty"`

	// Cause is the wrapped underlying error.
	Cause error `json:"-"`
}

// Error implements the error interface.
func (e *TentaserveError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %s)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the wrapped error for use with errors.Is and errors.As.
func (e *TentaserveError) Unwrap() error {
	return e.Cause
}

// WithUpstream returns a new TentaserveError with the upstream field set.
func (e *TentaserveError) WithUpstream(name string) *TentaserveError {
	return &TentaserveError{
		Code:       e.Code,
		Message:    e.Message,
		Upstream:   name,
		StatusCode: e.StatusCode,
		Field:      e.Field,
		Path:       e.Path,
		Cause:      e.Cause,
	}
}

// WithField returns a new TentaserveError with the field set.
func (e *TentaserveError) WithField(field string) *TentaserveError {
	return &TentaserveError{
		Code:       e.Code,
		Message:    e.Message,
		Upstream:   e.Upstream,
		StatusCode: e.StatusCode,
		Field:      field,
		Path:       e.Path,
		Cause:      e.Cause,
	}
}

// WithPath returns a new TentaserveError with the path set.
func (e *TentaserveError) WithPath(path []string) *TentaserveError {
	return &TentaserveError{
		Code:       e.Code,
		Message:    e.Message,
		Upstream:   e.Upstream,
		StatusCode: e.StatusCode,
		Field:      e.Field,
		Path:       path,
		Cause:      e.Cause,
	}
}

// WithCause returns a new TentaserveError with the cause set.
func (e *TentaserveError) WithCause(cause error) *TentaserveError {
	return &TentaserveError{
		Code:       e.Code,
		Message:    e.Message,
		Upstream:   e.Upstream,
		StatusCode: e.StatusCode,
		Field:      e.Field,
		Path:       e.Path,
		Cause:      cause,
	}
}

// Predefined TentaserveError templates.
// These should be cloned and customized using the With* methods.
var (
	// Upstream errors
	ErrUpstreamTimeoutTmpl = &TentaserveError{
		Code:       CodeUpstreamTimeout,
		Message:    "upstream request timed out",
		StatusCode: 504,
	}
	ErrUpstreamUnavailableTmpl = &TentaserveError{
		Code:       CodeUpstreamUnavailable,
		Message:    "upstream service unavailable",
		StatusCode: 503,
	}
	ErrUpstreamNotFoundTmpl = &TentaserveError{
		Code:       CodeUpstreamNotFound,
		Message:    "upstream not found",
		StatusCode: 404,
	}

	// Gateway errors
	ErrCircuitOpenTmpl = &TentaserveError{
		Code:       CodeCircuitOpen,
		Message:    "circuit breaker is open",
		StatusCode: 503,
	}
	ErrRateLimitedTmpl = &TentaserveError{
		Code:       CodeRateLimited,
		Message:    "rate limit exceeded",
		StatusCode: 429,
	}
	ErrAuthFailedTmpl = &TentaserveError{
		Code:       CodeAuthFailed,
		Message:    "authentication failed",
		StatusCode: 401,
	}

	// Configuration errors
	ErrConfigInvalidTmpl = &TentaserveError{
		Code:       CodeConfigInvalid,
		Message:    "invalid configuration",
		StatusCode: 500,
	}
	ErrConfigNotFoundTmpl = &TentaserveError{
		Code:       CodeConfigNotFound,
		Message:    "configuration file not found",
		StatusCode: 500,
	}

	// GraphQL errors
	ErrQueryTooComplexTmpl = &TentaserveError{
		Code:       CodeQueryTooComplex,
		Message:    "query exceeds maximum complexity",
		StatusCode: 400,
	}
	ErrQueryTooDeepTmpl = &TentaserveError{
		Code:       CodeQueryTooDeep,
		Message:    "query exceeds maximum depth",
		StatusCode: 400,
	}
	ErrQueryInvalidTmpl = &TentaserveError{
		Code:       CodeQueryInvalid,
		Message:    "invalid query",
		StatusCode: 400,
	}
)

// NewUpstreamTimeout creates an upstream timeout error.
func NewUpstreamTimeout(upstream string, cause error) *TentaserveError {
	return ErrUpstreamTimeoutTmpl.WithUpstream(upstream).WithCause(cause)
}

// NewUpstreamUnavailable creates an upstream unavailable error.
func NewUpstreamUnavailable(upstream string, cause error) *TentaserveError {
	return ErrUpstreamUnavailableTmpl.WithUpstream(upstream).WithCause(cause)
}

// NewRateLimited creates a rate limit error.
func NewRateLimited(retryAfter int) *TentaserveError {
	return ErrRateLimitedTmpl
}

// NewConfigInvalid creates a configuration error.
func NewConfigInvalid(field string, cause error) *TentaserveError {
	return ErrConfigInvalidTmpl.WithField(field).WithCause(cause)
}

// NewQueryTooComplex creates a query complexity error.
func NewQueryTooComplex(maxComplexity int) *TentaserveError {
	e := *ErrQueryTooComplexTmpl
	e.Message = fmt.Sprintf("query complexity exceeds maximum of %d", maxComplexity)
	return &e
}

// NewQueryTooDeep creates a query depth error.
func NewQueryTooDeep(maxDepth int) *TentaserveError {
	e := *ErrQueryTooDeepTmpl
	e.Message = fmt.Sprintf("query depth exceeds maximum of %d", maxDepth)
	return &e
}

// IsUpstreamError returns true if the error is related to upstream communication.
func IsUpstreamError(err error) bool {
	return errors.Is(err, ErrUpstreamTimeout) ||
		errors.Is(err, ErrUpstreamUnavailable) ||
		errors.Is(err, ErrUpstreamNotFound)
}

// IsClientError returns true if the error is caused by client input.
func IsClientError(err error) bool {
	return errors.Is(err, ErrQueryInvalid) ||
		errors.Is(err, ErrQueryTooDeep) ||
		errors.Is(err, ErrQueryTooComplex) ||
		errors.Is(err, ErrAuthFailed) ||
		errors.Is(err, ErrRateLimited)
}

// IsConfigError returns true if the error is related to configuration.
func IsConfigError(err error) bool {
	return errors.Is(err, ErrConfigInvalid) ||
		errors.Is(err, ErrConfigNotFound) ||
		errors.Is(err, ErrConfigParse)
}
