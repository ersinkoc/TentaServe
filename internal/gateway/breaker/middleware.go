package breaker

import (
	"context"
	"log/slog"
	"net/http"
)

// Middleware is HTTP middleware that provides circuit breaker functionality.
type Middleware struct {
	store  *Store
	logger *slog.Logger
}

// NewMiddleware creates a new circuit breaker middleware.
func NewMiddleware(store *Store, logger *slog.Logger) *Middleware {
	if logger == nil {
		logger = slog.Default()
	}

	return &Middleware{
		store:  store,
		logger: logger,
	}
}

// Wrap returns an HTTP handler that wraps the given handler with circuit breaker.
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get upstream key from context or request
		key := m.extractKey(r)
		if key == "" {
			key = r.URL.Path
		}

		breaker := m.store.Get(key)

		// Check if request should be allowed
		if !breaker.Allow() {
			m.logger.Debug("circuit breaker open",
				slog.String("key", key),
				slog.String("path", r.URL.Path),
			)
			m.serveError(w, r)
			return
		}

		// Wrap response writer to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Execute handler
		next.ServeHTTP(rw, r)

		// Record result based on status code
		if rw.statusCode >= http.StatusInternalServerError {
			// Server errors count as failures
			breaker.Record(ResultFailure)
			m.logger.Debug("circuit breaker recorded failure",
				slog.String("key", key),
				slog.Int("status", rw.statusCode),
				slog.String("state", breaker.State().String()),
			)
		} else {
			// Success (including client errors like 404)
			breaker.Record(ResultSuccess)
		}
	})
}

// serveError returns an error response when circuit is open.
func (m *Middleware) serveError(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Circuit-Breaker", "open")
	w.WriteHeader(http.StatusServiceUnavailable)
	w.Write([]byte(`{"error":"Service temporarily unavailable","code":"CIRCUIT_OPEN"}`))
}

// extractKey extracts the upstream key from the request.
// This can be overridden to use upstream-specific keys.
func (m *Middleware) extractKey(r *http.Request) string {
	// Check for upstream name in context
	if key, ok := r.Context().Value(upstreamKey{}).(string); ok {
		return key
	}
	return ""
}

type upstreamKey struct{}

// WithUpstreamKey adds an upstream key to the context.
func WithUpstreamKey(r *http.Request, key string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), upstreamKey{}, key))
}

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// DefaultMiddleware creates a middleware with default configuration.
func DefaultMiddleware(logger *slog.Logger) *Middleware {
	return NewMiddleware(NewStore(nil), logger)
}
