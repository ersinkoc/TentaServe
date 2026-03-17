package auth

import (
	"log/slog"
	"net/http"
)

// Middleware is HTTP middleware that performs authentication.
type Middleware struct {
	plugin Plugin
	logger *slog.Logger
}

// NewMiddleware creates a new authentication middleware.
func NewMiddleware(plugin Plugin, logger *slog.Logger) *Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return &Middleware{
		plugin: plugin,
		logger: logger,
	}
}

// Wrap returns an HTTP handler that wraps the given handler with authentication.
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Attempt authentication
		result, err := m.plugin.Authenticate(r.Context(), r)
		if err != nil {
			m.logger.Error("authentication error",
				slog.String("plugin", m.plugin.Name()),
				slog.String("error", err.Error()),
			)
			// For passthrough, we continue even on error
			// Other strategies may want to return 401 here
			if m.plugin.Name() != "passthrough" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			// Create a default result for passthrough on error
			result = &Result{
				Authenticated: true,
				Claims:        make(map[string]interface{}),
				Headers:       r.Header.Clone(),
			}
		}

		// Log authentication result
		if result.Authenticated {
			m.logger.Debug("request authenticated",
				slog.String("plugin", m.plugin.Name()),
				slog.String("subject", result.Subject),
			)
		} else {
			m.logger.Debug("request not authenticated",
				slog.String("plugin", m.plugin.Name()),
			)
		}

		// Add auth result to context
		ctx := WithResult(r.Context(), result)

		// Forward auth headers to the response
		for name, values := range result.Headers {
			for _, value := range values {
				w.Header().Add(name, value)
			}
		}

		// Continue with the authenticated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Plugin returns the underlying auth plugin.
func (m *Middleware) Plugin() Plugin {
	return m.plugin
}

// DefaultMiddleware creates a middleware with the passthrough strategy.
func DefaultMiddleware(logger *slog.Logger) *Middleware {
	return NewMiddleware(NewPassthrough(), logger)
}
