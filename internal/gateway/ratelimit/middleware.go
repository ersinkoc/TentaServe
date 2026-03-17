package ratelimit

import (
	"log/slog"
	"net/http"
	"strconv"
)

// Middleware is HTTP middleware that performs rate limiting.
type Middleware struct {
	store  *Store
	logger *slog.Logger
}

// NewMiddleware creates a new rate limit middleware.
func NewMiddleware(store *Store, logger *slog.Logger) *Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return &Middleware{
		store:  store,
		logger: logger,
	}
}

// Wrap returns an HTTP handler that wraps the given handler with rate limiting.
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get upstream from context (set by upstream middleware)
		upstream := ""
		if u := r.Context().Value("upstream"); u != nil {
			upstream = u.(string)
		}

		// Check rate limit
		allowed, retryAfter := m.store.Allow(r, upstream)

		if !allowed {
			// Rate limited
			clientKey := m.store.clientKey(r, m.store.config)
			m.logger.Warn("rate limit exceeded",
				slog.String("client", clientKey),
				slog.String("upstream", upstream),
				slog.String("path", r.URL.Path),
			)

			// Set Retry-After header
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limit exceeded","retry_after":` + strconv.Itoa(int(retryAfter.Seconds())) + `}`))
			return
		}

		// Continue to next handler
		next.ServeHTTP(w, r)
	})
}

// Store returns the underlying rate limit store.
func (m *Middleware) Store() *Store {
	return m.store
}

// DefaultMiddleware creates a middleware with default configuration.
func DefaultMiddleware(logger *slog.Logger) *Middleware {
	return NewMiddleware(NewStore(DefaultConfig()), logger)
}
