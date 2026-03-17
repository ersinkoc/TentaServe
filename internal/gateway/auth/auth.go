// Package auth provides authentication strategies for the gateway.
//
// The auth package supports multiple authentication strategies:
// - Passthrough: forwards all headers unchanged (default)
// - JWT: validates JWT tokens locally
// - APIKey: validates API keys against a configured list
package auth

import (
	"context"
	"net/http"
)

// Result holds the result of an authentication attempt.
type Result struct {
	// Authenticated indicates whether the request is authenticated
	Authenticated bool

	// Subject is the identifier of the authenticated entity (e.g., user ID)
	Subject string

	// Claims contains any additional claims from the authentication token
	Claims map[string]interface{}

	// Headers to forward to the upstream
	Headers http.Header
}

// Plugin is the interface for authentication strategies.
type Plugin interface {
	// Name returns the name of the authentication plugin
	Name() string

	// Authenticate attempts to authenticate the request.
	// Returns the authentication result and any error.
	Authenticate(ctx context.Context, r *http.Request) (*Result, error)
}

// contextKey is the type for context keys used by the auth package.
type contextKey int

const (
	// authResultKey is the context key for the authentication result
	authResultKey contextKey = iota
)

// WithResult adds an authentication result to the context.
func WithResult(ctx context.Context, result *Result) context.Context {
	return context.WithValue(ctx, authResultKey, result)
}

// ResultFromContext retrieves the authentication result from the context.
// Returns nil if no result is present.
func ResultFromContext(ctx context.Context) *Result {
	if result, ok := ctx.Value(authResultKey).(*Result); ok {
		return result
	}
	return nil
}

// IsAuthenticated checks if the context contains an authenticated result.
func IsAuthenticated(ctx context.Context) bool {
	if result := ResultFromContext(ctx); result != nil {
		return result.Authenticated
	}
	return false
}

// SubjectFromContext retrieves the authenticated subject from the context.
// Returns empty string if not authenticated.
func SubjectFromContext(ctx context.Context) string {
	if result := ResultFromContext(ctx); result != nil {
		return result.Subject
	}
	return ""
}
