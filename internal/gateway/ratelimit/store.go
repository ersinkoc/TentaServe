package ratelimit

import (
	"net/http"
	"sync"
	"time"
)

// Scope defines how rate limiting is scoped.
type Scope string

const (
	// ScopeGlobal applies rate limit across all requests
	ScopeGlobal Scope = "global"
	// ScopeIP applies rate limit per IP address
	ScopeIP Scope = "ip"
	// ScopeHeader applies rate limit per header value
	ScopeHeader Scope = "header"
	// ScopePath applies rate limit per path
	ScopePath Scope = "path"
)

// Config defines rate limiting configuration.
type Config struct {
	// Enabled controls whether rate limiting is active
	Enabled bool

	// Rate is the number of requests per second
	Rate float64

	// Burst is the maximum number of requests that can be made in a burst
	Burst int

	// Scope defines how to scope the rate limit
	Scope Scope

	// HeaderName is the header to use for ScopeHeader (e.g., "X-API-Key")
	HeaderName string

	// PerUpstream overrides for specific upstreams (upstream name -> config)
	PerUpstream map[string]*Config
}

// DefaultConfig returns a default rate limit configuration.
func DefaultConfig() *Config {
	return &Config{
		Enabled:     false,
		Rate:        100,
		Burst:       150,
		Scope:       ScopeIP,
		PerUpstream: make(map[string]*Config),
	}
}

// ClientBucket holds a token bucket and its last access time.
type ClientBucket struct {
	bucket     *TokenBucket
	lastAccess time.Time
}

// Store manages rate limit buckets for different clients.
type Store struct {
	config *Config

	// buckets maps client keys to their token buckets
	buckets sync.Map

	// mu protects the cleanup timer
	mu sync.RWMutex
}

// NewStore creates a new rate limit store.
func NewStore(config *Config) *Store {
	if config == nil {
		config = DefaultConfig()
	}

	return &Store{
		config: config,
	}
}

// Allow checks if the request should be allowed based on rate limiting.
// Returns true if allowed, false if rate limited.
func (s *Store) Allow(r *http.Request, upstream string) (bool, time.Duration) {
	// Get effective config (check per-upstream override)
	config := s.config
	if upstream != "" && s.config.PerUpstream != nil {
		if upstreamConfig, ok := s.config.PerUpstream[upstream]; ok {
			config = upstreamConfig
		}
	}

	if !config.Enabled {
		return true, 0
	}

	// Get or create bucket for this client
	key := s.clientKey(r, config)
	now := time.Now()

	var bucket *TokenBucket
	if cb, ok := s.buckets.Load(key); ok {
		clientBucket := cb.(*ClientBucket)
		clientBucket.lastAccess = now
		bucket = clientBucket.bucket
	} else {
		// Create new bucket
		bucket = NewTokenBucket(config.Rate, float64(config.Burst))
		s.buckets.Store(key, &ClientBucket{
			bucket:     bucket,
			lastAccess: now,
		})
	}

	if bucket.Allow1() {
		return true, 0
	}

	// Rate limited - calculate retry after
	waitTime := bucket.WaitTime(1)
	return false, waitTime
}

// clientKey generates a unique key for the client based on scope.
func (s *Store) clientKey(r *http.Request, config *Config) string {
	switch config.Scope {
	case ScopeGlobal:
		return "global"
	case ScopeIP:
		return "ip:" + s.clientIP(r)
	case ScopeHeader:
		if config.HeaderName != "" {
			return "header:" + config.HeaderName + ":" + r.Header.Get(config.HeaderName)
		}
		return "header:missing"
	case ScopePath:
		return "path:" + r.URL.Path
	default:
		return "ip:" + s.clientIP(r)
	}
}

// clientIP extracts the client IP from the request.
func (s *Store) clientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Take the first IP in the chain
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	// Check X-Real-Ip header
	xri := r.Header.Get("X-Real-Ip")
	if xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	host, _, err := splitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Cleanup removes stale buckets that haven't been accessed recently.
// This should be called periodically by a background goroutine.
func (s *Store) Cleanup(maxAge time.Duration) int {
	removed := 0
	now := time.Now()

	s.buckets.Range(func(key, value interface{}) bool {
		cb := value.(*ClientBucket)
		if now.Sub(cb.lastAccess) > maxAge {
			s.buckets.Delete(key)
			removed++
		}
		return true
	})

	return removed
}

// Len returns the number of buckets in the store.
func (s *Store) Len() int {
	count := 0
	s.buckets.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// Reset clears all buckets from the store.
func (s *Store) Reset() {
	s.buckets = sync.Map{}
}

// splitHostPort splits host:port and returns just the host.
// Handles IPv6 addresses in brackets.
func splitHostPort(addr string) (string, string, error) {
	// Simple implementation for common cases
	// IPv6: [::1]:8080
	// IPv4: 127.0.0.1:8080

	// Check for IPv6 in brackets
	if len(addr) > 0 && addr[0] == '[' {
		// Find closing bracket
		for i := 1; i < len(addr); i++ {
			if addr[i] == ']' {
				host := addr[1:i]
				if i+1 < len(addr) && addr[i+1] == ':' {
					return host, addr[i+2:], nil
				}
				return host, "", nil
			}
		}
	}

	// IPv4 or hostname
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i], addr[i+1:], nil
		}
	}

	// No port found
	return addr, "", nil
}
