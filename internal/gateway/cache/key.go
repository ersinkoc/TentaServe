package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// KeyBuilder builds cache keys from HTTP requests.
type KeyBuilder struct {
	varyHeaders []string
}

// NewKeyBuilder creates a new cache key builder.
func NewKeyBuilder(varyHeaders []string) *KeyBuilder {
	// Sort vary headers for consistent key generation
	vh := make([]string, len(varyHeaders))
	copy(vh, varyHeaders)
	sort.Strings(vh)

	return &KeyBuilder{
		varyHeaders: vh,
	}
}

// BuildKey generates a cache key for the request.
// The key includes: method, scheme, host, path, sorted query params, vary headers, body hash (if present).
func (kb *KeyBuilder) BuildKey(r *http.Request) string {
	var parts []string

	// Method
	parts = append(parts, r.Method)

	// URL components
	parts = append(parts, r.URL.Scheme)
	parts = append(parts, r.URL.Host)
	parts = append(parts, r.URL.Path)

	// Sorted query parameters
	if r.URL.RawQuery != "" {
		parts = append(parts, kb.normalizeQuery(r.URL.Query()))
	}

	// Vary headers
	for _, header := range kb.varyHeaders {
		value := r.Header.Get(header)
		if value != "" {
			parts = append(parts, header+"="+value)
		}
	}

	return strings.Join(parts, "|")
}

// BuildKeyWithBody generates a cache key including request body hash.
func (kb *KeyBuilder) BuildKeyWithBody(r *http.Request, body []byte) string {
	key := kb.BuildKey(r)

	if len(body) > 0 {
		h := sha256.Sum256(body)
		key += "|body=" + hex.EncodeToString(h[:])
	}

	return key
}

// normalizeQuery normalizes query parameters for consistent key generation.
func (kb *KeyBuilder) normalizeQuery(query url.Values) string {
	if len(query) == 0 {
		return ""
	}

	// Get sorted list of keys
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build normalized query string
	var parts []string
	for _, k := range keys {
		values := query[k]
		sort.Strings(values)
		for _, v := range values {
			parts = append(parts, k+"="+v)
		}
	}

	return strings.Join(parts, "&")
}

// BuildHashKey generates a short hash of the key for use as a cache key.
func (kb *KeyBuilder) BuildHashKey(r *http.Request) string {
	key := kb.BuildKey(r)
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:16]) // Use first 128 bits
}

// BuildHashKeyWithBody generates a short hash key including request body.
func (kb *KeyBuilder) BuildHashKeyWithBody(r *http.Request, body []byte) string {
	key := kb.BuildKeyWithBody(r, body)
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:16])
}

// IsCacheableMethod returns true if the HTTP method is cacheable.
func IsCacheableMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead:
		return true
	default:
		return false
	}
}

// IsUncacheableRequest returns true if the request should not be cached.
func IsUncacheableRequest(r *http.Request) bool {
	// Check Cache-Control header
	cacheControl := r.Header.Get("Cache-Control")
	if strings.Contains(cacheControl, "no-cache") {
		return true
	}
	if strings.Contains(cacheControl, "no-store") {
		return true
	}
	if strings.Contains(cacheControl, "max-age=0") {
		return true
	}

	// Check Pragma header (HTTP/1.0)
	pragma := r.Header.Get("Pragma")
	if pragma == "no-cache" {
		return true
	}

	// Authorization requests shouldn't be cached by default
	if r.Header.Get("Authorization") != "" {
		return true
	}

	return false
}

// IsUncacheableResponse returns true if the response should not be cached.
func IsUncacheableResponse(statusCode int, headers http.Header) bool {
	// Check Cache-Control header
	cacheControl := headers.Get("Cache-Control")
	if strings.Contains(cacheControl, "no-cache") {
		return true
	}
	if strings.Contains(cacheControl, "no-store") {
		return true
	}
	if strings.Contains(cacheControl, "private") {
		return true
	}

	// Check Pragma header
	pragma := headers.Get("Pragma")
	if pragma == "no-cache" {
		return true
	}

	// Check status code
	switch statusCode {
	case http.StatusOK,
		http.StatusCreated,
		http.StatusAccepted,
		http.StatusNonAuthoritativeInfo,
		http.StatusNoContent,
		http.StatusResetContent,
		http.StatusPartialContent,
		http.StatusMovedPermanently,
		http.StatusFound,
		http.StatusTemporaryRedirect,
		http.StatusPermanentRedirect,
		http.StatusNotFound,
		http.StatusGone:
		return false
	default:
		// Don't cache other status codes by default
		return true
	}
}

// ParseCacheControl parses Cache-Control header directives.
func ParseCacheControl(header string) map[string]string {
	directives := make(map[string]string)

	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if idx := strings.Index(part, "="); idx > 0 {
			key := strings.TrimSpace(part[:idx])
			value := strings.TrimSpace(part[idx+1:])
			// Remove quotes if present
			if len(value) > 1 && value[0] == '"' && value[len(value)-1] == '"' {
				value = value[1 : len(value)-1]
			}
			directives[key] = value
		} else {
			directives[part] = ""
		}
	}

	return directives
}

// hasher is a hash.Hash wrapper that ignores errors.
type hasher struct {
	hash.Hash
}

// WriteString writes a string to the hash.
func (h *hasher) WriteString(s string) {
	h.Write([]byte(s))
}

// WriteByte writes a byte to the hash.
func (h *hasher) WriteByte(b byte) error {
	_, err := h.Write([]byte{b})
	return err
}
