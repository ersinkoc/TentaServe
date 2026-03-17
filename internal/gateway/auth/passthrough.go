package auth

import (
	"context"
	"net/http"
)

// Passthrough is the default authentication strategy that forwards
// all headers unchanged without validation.
//
// This strategy always returns Authenticated=true and copies all
// request headers to the result for upstream forwarding.
type Passthrough struct {
	// HeaderPrefix filters headers by prefix (empty = all headers)
	HeaderPrefix string

	// ExcludeHeaders contains header names that should not be forwarded
	ExcludeHeaders []string
}

// NewPassthrough creates a new passthrough authentication plugin.
func NewPassthrough() *Passthrough {
	return &Passthrough{
		ExcludeHeaders: []string{
			"Content-Length",    // Will be set by the HTTP client
			"Transfer-Encoding", // Hop-by-hop header
			"Connection",        // Hop-by-hop header
			"Upgrade",           // Hop-by-hop header
			"Proxy-Authorization",
		},
	}
}

// NewPassthroughWithOptions creates a passthrough plugin with custom options.
func NewPassthroughWithOptions(excludeHeaders []string, headerPrefix string) *Passthrough {
	return &Passthrough{
		HeaderPrefix:   headerPrefix,
		ExcludeHeaders: excludeHeaders,
	}
}

// Name returns the name of the authentication plugin.
func (p *Passthrough) Name() string {
	return "passthrough"
}

// Authenticate forwards all headers unchanged.
// Always returns Authenticated=true since no validation is performed.
func (p *Passthrough) Authenticate(ctx context.Context, r *http.Request) (*Result, error) {
	// Copy headers, excluding hop-by-hop and configured headers
	headers := make(http.Header)

	for name, values := range r.Header {
		// Skip excluded headers
		if p.isExcluded(name) {
			continue
		}

		// Check prefix filter if configured
		if p.HeaderPrefix != "" && !hasPrefix(name, p.HeaderPrefix) {
			continue
		}

		// Copy header values
		for _, value := range values {
			headers.Add(name, value)
		}
	}

	return &Result{
		Authenticated: true,
		Subject:       "", // No subject identification in passthrough mode
		Claims:        make(map[string]interface{}),
		Headers:       headers,
	}, nil
}

// isExcluded checks if a header should be excluded.
func (p *Passthrough) isExcluded(name string) bool {
	for _, excluded := range p.ExcludeHeaders {
		if equalFold(name, excluded) {
			return true
		}
	}
	return false
}

// equalFold performs case-insensitive string comparison.
func equalFold(s, t string) bool {
	if len(s) != len(t) {
		return false
	}
	for i := 0; i < len(s); i++ {
		if toLower(s[i]) != toLower(t[i]) {
			return false
		}
	}
	return true
}

// toLower converts a byte to lowercase.
func toLower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}

// hasPrefix checks if s has the given prefix (case-insensitive).
func hasPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		if toLower(s[i]) != toLower(prefix[i]) {
			return false
		}
	}
	return true
}
