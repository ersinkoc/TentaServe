// Package middleware provides HTTP middleware for the gateway.
//
// Middleware functions wrap HTTP handlers to add cross-cutting concerns
// like request ID generation, logging, authentication, etc.
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/ersinkoc/tentaserve/internal"
)

// RequestID is middleware that generates or propagates request IDs.
// It adds an X-Request-ID header to every request.
type RequestID struct {
	HeaderName string
}

// NewRequestID creates a new RequestID middleware.
func NewRequestID(headerName string) *RequestID {
	if headerName == "" {
		headerName = "X-Request-ID"
	}
	return &RequestID{HeaderName: headerName}
}

// Wrap returns an HTTP handler that wraps the given handler with request ID functionality.
func (m *RequestID) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if client provided a request ID
		requestID := r.Header.Get(m.HeaderName)
		if requestID == "" {
			// Generate a new request ID
			requestID = GenerateRequestID()
		}

		// Add request ID to response headers
		w.Header().Set(m.HeaderName, requestID)

		// Add request ID to context
		ctx := internal.WithRequestID(r.Context(), requestID)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// GenerateRequestID generates a unique request ID.
// It uses crypto/rand for collision resistance.
func GenerateRequestID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b) // crypto/rand never fails on modern OS
	return "req_" + hex.EncodeToString(b)
}

// Chain combines multiple middleware into a single handler.
// Middleware is applied in reverse order, so the first middleware
// in the list is the outermost wrapper.
func Chain(h http.Handler, middleware ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middleware) - 1; i >= 0; i-- {
		h = middleware[i](h)
	}
	return h
}
