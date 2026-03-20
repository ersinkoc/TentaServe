package cache

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// Middleware is HTTP middleware that provides response caching.
type Middleware struct {
	cache      *Cache
	keyBuilder *KeyBuilder
	logger     *slog.Logger
}

// NewMiddleware creates a new cache middleware.
func NewMiddleware(cache *Cache, logger *slog.Logger) *Middleware {
	if logger == nil {
		logger = slog.Default()
	}

	return &Middleware{
		cache:      cache,
		keyBuilder: NewKeyBuilder(cache.config.VaryHeaders),
		logger:     logger,
	}
}

// Wrap returns an HTTP handler that wraps the given handler with caching.
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if caching is enabled
		if !m.cache.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Check if method is cacheable
		if !m.cache.config.IsMethodCacheable(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		// Handle PURGE request
		if r.Method == "PURGE" {
			m.handlePurge(w, r)
			return
		}

		// Check if request has cache-busting headers
		if IsUncacheableRequest(r) {
			next.ServeHTTP(w, r)
			return
		}

		// Build cache key
		key := m.keyBuilder.BuildKey(r)

		// Try to get from cache
		entry := m.cache.Get(key)
		if entry != nil {
			// Check if entry is expired
			if !entry.IsExpired() {
				// Cache hit - serve from cache
				m.serveFromCache(w, r, entry, false)
				return
			}

			// Entry is expired - check stale-while-revalidate
			if entry.IsStale(m.cache.config.StaleDuration) {
				// Serve stale and trigger background revalidation
				m.serveFromCache(w, r, entry, true)
				// Background revalidation
				go m.revalidate(r, key, next)
				return
			}
		}

		// Cache miss - capture response
		m.captureAndCache(w, r, key, next)
	})
}

// serveFromCache serves a cached response.
func (m *Middleware) serveFromCache(w http.ResponseWriter, r *http.Request, entry *Entry, stale bool) {
	// Set headers
	for k, v := range entry.Headers {
		for _, val := range v {
			w.Header().Add(k, val)
		}
	}

	// Add cache headers
	w.Header().Set("X-Cache", "HIT")
	if stale {
		w.Header().Set("X-Cache-Status", "STALE")
	} else {
		w.Header().Set("X-Cache-Status", "FRESH")
	}
	w.Header().Set("Age", strconv.Itoa(int(entry.Age().Seconds())))

	// Write response
	w.WriteHeader(entry.StatusCode)
	w.Write(entry.Body)

	m.logger.Debug("cache hit",
		slog.String("key", r.URL.Path),
		slog.Bool("stale", stale),
	)
}

// captureAndCache captures the response and stores it in cache.
func (m *Middleware) captureAndCache(w http.ResponseWriter, r *http.Request, key string, next http.Handler) {
	// Create response writer that captures output
	capture := NewResponseWriter(w)

	// Execute handler
	next.ServeHTTP(capture, r)

	// Check if response should be cached
	if !capture.IsCacheable() {
		return
	}
	if !m.cache.config.IsStatusCodeCacheable(capture.StatusCode()) {
		return
	}

	// Create cache entry
	entry := &Entry{
		StatusCode:  capture.StatusCode(),
		Headers:     capture.CapturedHeaders(),
		Body:        capture.Body(),
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(m.cache.config.TTL),
		VaryHeaders: m.extractVaryHeaders(r),
	}

	// Store in cache
	if m.cache.Set(key, entry) {
		w.Header().Set("X-Cache", "MISS")
		m.logger.Debug("cache stored",
			slog.String("key", key),
			slog.Int("status", capture.StatusCode()),
			slog.Int("size", entry.Size()),
		)
	}
}

// revalidate performs background revalidation of a stale entry.
func (m *Middleware) revalidate(r *http.Request, key string, handler http.Handler) {
	// Clone request
	req := r.Clone(r.Context())

	// Create a response writer to capture
	var buf bytes.Buffer
	capture := &captureResponse{
		statusCode: http.StatusOK,
		headers:    make(http.Header),
		body:       &buf,
	}

	// Execute handler
	handler.ServeHTTP(capture, req)

	// Only cache successful responses
	if !IsUncacheableResponse(capture.statusCode, capture.headers) {
		entry := &Entry{
			StatusCode:  capture.statusCode,
			Headers:     capture.headers,
			Body:        buf.Bytes(),
			CreatedAt:   time.Now(),
			ExpiresAt:   time.Now().Add(m.cache.config.TTL),
			VaryHeaders: m.extractVaryHeaders(req),
		}
		m.cache.Set(key, entry)
	}
}

// handlePurge handles PURGE requests to invalidate cache entries.
func (m *Middleware) handlePurge(w http.ResponseWriter, r *http.Request) {
	// Build key as if this were a GET request for consistency
	purgeReq := r.Clone(r.Context())
	purgeReq.Method = "GET"
	key := m.keyBuilder.BuildKey(purgeReq)
	m.logger.Debug("purging cache", slog.String("key", key))
	m.cache.Delete(key)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"purged"}`))

	m.logger.Info("cache purged",
		slog.String("key", key),
		slog.String("path", r.URL.Path),
	)
}

// extractVaryHeaders extracts vary header values from the request.
func (m *Middleware) extractVaryHeaders(r *http.Request) map[string]string {
	vary := make(map[string]string)
	for _, header := range m.cache.config.VaryHeaders {
		if value := r.Header.Get(header); value != "" {
			vary[header] = value
		}
	}
	return vary
}

// Cache returns the underlying cache.
func (m *Middleware) Cache() *Cache {
	return m.cache
}

// DefaultMiddleware creates a middleware with default configuration.
func DefaultMiddleware(logger *slog.Logger) *Middleware {
	return NewMiddleware(New(DefaultConfig()), logger)
}

// captureResponse is a simple response writer for background revalidation.
type captureResponse struct {
	statusCode  int
	headers     http.Header
	body        *bytes.Buffer
	wroteHeader bool
}

func (c *captureResponse) Header() http.Header {
	return c.headers
}

func (c *captureResponse) WriteHeader(code int) {
	if !c.wroteHeader {
		c.statusCode = code
		c.wroteHeader = true
	}
}

func (c *captureResponse) Write(b []byte) (int, error) {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}
	return c.body.Write(b)
}

// ReadRequestBody reads the request body if present.
func ReadRequestBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	// Restore body so it can be read again
	r.Body = io.NopCloser(bytes.NewReader(body))

	return body, nil
}
