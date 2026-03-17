package cors

import (
	"net/http"
	"strconv"
	"strings"
)

// Config defines CORS configuration.
type Config struct {
	// Enabled controls whether CORS is active
	Enabled bool

	// AllowedOrigins is a list of allowed origins (use * for any)
	AllowedOrigins []string

	// AllowedMethods is a list of allowed HTTP methods
	AllowedMethods []string

	// AllowedHeaders is a list of allowed headers (use * for any)
	AllowedHeaders []string

	// ExposedHeaders is a list of headers exposed to the client
	ExposedHeaders []string

	// AllowCredentials allows cookies/auth headers
	AllowCredentials bool

	// MaxAge is the preflight cache duration in seconds
	MaxAge int

	// AllowPrivateNetwork allows private network requests
	AllowPrivateNetwork bool
}

// DefaultConfig returns default CORS configuration.
func DefaultConfig() *Config {
	return &Config{
		Enabled:          true,
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   []string{},
		AllowCredentials: false,
		MaxAge:           86400, // 24 hours
	}
}

// CORS is CORS middleware.
type CORS struct {
	config *Config
}

// New creates a new CORS handler.
func New(config *Config) *CORS {
	if config == nil {
		config = DefaultConfig()
	}
	return &CORS{config: config}
}

// Middleware returns an HTTP middleware function.
func (c *CORS) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !c.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Handle preflight request
		if r.Method == http.MethodOptions {
			c.handlePreflight(w, r)
			return
		}

		// Handle actual request
		c.handleActual(w, r, next)
	})
}

// handlePreflight handles CORS preflight requests.
func (c *CORS) handlePreflight(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Check if origin is allowed
	if !c.isOriginAllowed(origin) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Set CORS headers
	c.setPreflightHeaders(w, r, origin)
	w.WriteHeader(http.StatusNoContent)
}

// handleActual handles actual CORS requests.
func (c *CORS) handleActual(w http.ResponseWriter, r *http.Request, next http.Handler) {
	origin := r.Header.Get("Origin")
	if origin != "" && c.isOriginAllowed(origin) {
		c.setActualHeaders(w, origin)
	}

	next.ServeHTTP(w, r)
}

// setPreflightHeaders sets preflight response headers.
func (c *CORS) setPreflightHeaders(w http.ResponseWriter, r *http.Request, origin string) {
	// Access-Control-Allow-Origin
	w.Header().Set("Access-Control-Allow-Origin", origin)

	// Access-Control-Allow-Methods
	if len(c.config.AllowedMethods) > 0 {
		w.Header().Set("Access-Control-Allow-Methods", strings.Join(c.config.AllowedMethods, ", "))
	}

	// Access-Control-Allow-Headers
	requestedHeaders := r.Header.Get("Access-Control-Request-Headers")
	if requestedHeaders != "" {
		if c.allowsAllHeaders() {
			w.Header().Set("Access-Control-Allow-Headers", requestedHeaders)
		} else {
			allowed := c.filterAllowedHeaders(requestedHeaders)
			if len(allowed) > 0 {
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(allowed, ", "))
			}
		}
	} else if len(c.config.AllowedHeaders) > 0 && !c.allowsAllHeaders() {
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(c.config.AllowedHeaders, ", "))
	}

	// Access-Control-Allow-Credentials
	if c.config.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	// Access-Control-Max-Age
	if c.config.MaxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", strconv.Itoa(c.config.MaxAge))
	}

	// Access-Control-Allow-Private-Network
	if c.config.AllowPrivateNetwork && r.Header.Get("Access-Control-Request-Private-Network") == "true" {
		w.Header().Set("Access-Control-Allow-Private-Network", "true")
	}
}

// setActualHeaders sets actual response headers.
func (c *CORS) setActualHeaders(w http.ResponseWriter, origin string) {
	w.Header().Set("Access-Control-Allow-Origin", origin)

	if c.config.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	if len(c.config.ExposedHeaders) > 0 {
		w.Header().Set("Access-Control-Expose-Headers", strings.Join(c.config.ExposedHeaders, ", "))
	}

	// Vary header for caching
	w.Header().Add("Vary", "Origin")
}

// isOriginAllowed checks if an origin is allowed.
func (c *CORS) isOriginAllowed(origin string) bool {
	if len(c.config.AllowedOrigins) == 0 {
		return false
	}

	for _, allowed := range c.config.AllowedOrigins {
		if allowed == "*" || strings.EqualFold(allowed, origin) {
			return true
		}
	}

	return false
}

// allowsAllHeaders checks if any header is allowed.
func (c *CORS) allowsAllHeaders() bool {
	for _, h := range c.config.AllowedHeaders {
		if h == "*" {
			return true
		}
	}
	return false
}

// filterAllowedHeaders filters requested headers to only allowed ones.
func (c *CORS) filterAllowedHeaders(requested string) []string {
	requestedList := strings.Split(requested, ",")
	allowedSet := make(map[string]bool, len(c.config.AllowedHeaders))
	for _, h := range c.config.AllowedHeaders {
		allowedSet[strings.ToLower(strings.TrimSpace(h))] = true
	}

	var result []string
	for _, h := range requestedList {
		h = strings.TrimSpace(h)
		if allowedSet[strings.ToLower(h)] {
			result = append(result, h)
		}
	}

	return result
}

// Default returns CORS middleware with default configuration.
func Default() *CORS {
	return New(nil)
}

// WithOrigins creates CORS middleware with specific origins.
func WithOrigins(origins ...string) *CORS {
	config := DefaultConfig()
	config.AllowedOrigins = origins
	return New(config)
}
