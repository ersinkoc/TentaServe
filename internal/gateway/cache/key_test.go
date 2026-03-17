package cache

import (
	"net/http"
	"net/url"
	"testing"
)

// TestNewKeyBuilder tests key builder creation.
func TestNewKeyBuilder(t *testing.T) {
	vary := []string{"Accept", "Accept-Encoding"}
	kb := NewKeyBuilder(vary)

	if kb == nil {
		t.Fatal("Expected non-nil key builder")
	}
	if len(kb.varyHeaders) != 2 {
		t.Errorf("Expected 2 vary headers, got %d", len(kb.varyHeaders))
	}
}

// TestBuildKey tests key generation.
func TestBuildKey(t *testing.T) {
	kb := NewKeyBuilder([]string{"Accept"})

	req := &http.Request{
		Method: "GET",
		URL:    parseURL("https://example.com/api/users?page=2&limit=10"),
		Header: http.Header{
			"Accept": []string{"application/json"},
		},
	}

	key := kb.BuildKey(req)

	// Key should contain method, scheme, host, path, query params, and vary header
	if key == "" {
		t.Error("Expected non-empty key")
	}

	// Should contain method
	if key[:3] != "GET" {
		t.Errorf("Expected key to start with 'GET', got %s", key[:3])
	}
}

// TestBuildKeyWithBody tests key generation with body.
func TestBuildKeyWithBody(t *testing.T) {
	kb := NewKeyBuilder(nil)

	req := &http.Request{
		Method: "POST",
		URL:    parseURL("https://example.com/api/users"),
	}

	body := []byte(`{"name":"test"}`)
	key := kb.BuildKeyWithBody(req, body)

	// Key should contain body hash
	if len(key) < 10 {
		t.Error("Expected longer key with body hash")
	}
}

// TestNormalizeQuery tests query normalization.
func TestNormalizeQuery(t *testing.T) {
	kb := NewKeyBuilder(nil)

	tests := []struct {
		input    string
		expected string
	}{
		{"b=2&a=1", "a=1&b=2"},
		{"a=2&a=1", "a=1&a=2"},
		{"", ""},
		{"z=3&y=2&x=1", "x=1&y=2&z=3"},
	}

	for _, tt := range tests {
		u := parseURL("https://example.com/test?" + tt.input)
		got := kb.normalizeQuery(u.Query())
		if got != tt.expected {
			t.Errorf("normalizeQuery(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// TestBuildHashKey tests hash key generation.
func TestBuildHashKey(t *testing.T) {
	kb := NewKeyBuilder(nil)

	req := &http.Request{
		Method: "GET",
		URL:    parseURL("https://example.com/api/users"),
	}

	key := kb.BuildHashKey(req)

	// Should be 32 characters (hex encoding of 16 bytes)
	if len(key) != 32 {
		t.Errorf("Expected hash key length 32, got %d", len(key))
	}
}

// TestIsCacheableMethod tests method cacheability.
func TestIsCacheableMethod(t *testing.T) {
	tests := []struct {
		method   string
		expected bool
	}{
		{"GET", true},
		{"HEAD", true},
		{"POST", false},
		{"PUT", false},
		{"DELETE", false},
		{"PATCH", false},
	}

	for _, tt := range tests {
		if IsCacheableMethod(tt.method) != tt.expected {
			t.Errorf("IsCacheableMethod(%q) = %v, want %v", tt.method, !tt.expected, tt.expected)
		}
	}
}

// TestIsUncacheableRequest tests uncacheable request detection.
func TestIsUncacheableRequest(t *testing.T) {
	tests := []struct {
		name     string
		headers  http.Header
		expected bool
	}{
		{
			name:     "no cache headers",
			headers:  http.Header{},
			expected: false,
		},
		{
			name:     "no-cache",
			headers:  http.Header{"Cache-Control": []string{"no-cache"}},
			expected: true,
		},
		{
			name:     "no-store",
			headers:  http.Header{"Cache-Control": []string{"no-store"}},
			expected: true,
		},
		{
			name:     "max-age=0",
			headers:  http.Header{"Cache-Control": []string{"max-age=0"}},
			expected: true,
		},
		{
			name:     "pragma no-cache",
			headers:  http.Header{"Pragma": []string{"no-cache"}},
			expected: true,
		},
		{
			name:     "authorization header",
			headers:  http.Header{"Authorization": []string{"Bearer token"}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Header: tt.headers,
			}
			if IsUncacheableRequest(req) != tt.expected {
				t.Errorf("IsUncacheableRequest() = %v, want %v", !tt.expected, tt.expected)
			}
		})
	}
}

// TestIsUncacheableResponse tests uncacheable response detection.
func TestIsUncacheableResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		headers    http.Header
		expected   bool
	}{
		{
			name:       "200 OK",
			statusCode: 200,
			headers:    http.Header{},
			expected:   false,
		},
		{
			name:       "500 Internal Server Error",
			statusCode: 500,
			headers:    http.Header{},
			expected:   true,
		},
		{
			name:       "no-cache",
			statusCode: 200,
			headers:    http.Header{"Cache-Control": []string{"no-cache"}},
			expected:   true,
		},
		{
			name:       "no-store",
			statusCode: 200,
			headers:    http.Header{"Cache-Control": []string{"no-store"}},
			expected:   true,
		},
		{
			name:       "private",
			statusCode: 200,
			headers:    http.Header{"Cache-Control": []string{"private"}},
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if IsUncacheableResponse(tt.statusCode, tt.headers) != tt.expected {
				t.Errorf("IsUncacheableResponse() = %v, want %v", !tt.expected, tt.expected)
			}
		})
	}
}

// TestParseCacheControl tests Cache-Control parsing.
func TestParseCacheControl(t *testing.T) {
	tests := []struct {
		input    string
		expected map[string]string
	}{
		{
			input:    "max-age=3600",
			expected: map[string]string{"max-age": "3600"},
		},
		{
			input:    "max-age=3600, must-revalidate",
			expected: map[string]string{"max-age": "3600", "must-revalidate": ""},
		},
		{
			input:    "private, max-age=600",
			expected: map[string]string{"private": "", "max-age": "600"},
		},
		{
			input:    "",
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseCacheControl(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("ParseCacheControl(%q) = %v, want %v", tt.input, got, tt.expected)
			}
			for k, v := range tt.expected {
				if got[k] != v {
					t.Errorf("ParseCacheControl(%q)[%q] = %q, want %q", tt.input, k, got[k], v)
				}
			}
		})
	}
}

// parseURL is a helper to parse URLs for tests.
func parseURL(s string) *url.URL {
	u, _ := url.Parse(s)
	return u
}
