package auth

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
)

// APIKey is an authentication plugin that validates API keys from headers.
type APIKey struct {
	// Keys is the list of valid API keys
	Keys []string

	// HeaderName is the header containing the API key (default: X-API-Key)
	HeaderName string

	// HeaderPrefix is an optional prefix (e.g., "ApiKey ")
	HeaderPrefix string
}

// NewAPIKey creates a new API key authentication plugin.
func NewAPIKey(keys []string) *APIKey {
	return &APIKey{
		Keys:       keys,
		HeaderName: "X-API-Key",
	}
}

// NewAPIKeyWithOptions creates an API key plugin with custom options.
func NewAPIKeyWithOptions(keys []string, headerName, headerPrefix string) *APIKey {
	return &APIKey{
		Keys:         keys,
		HeaderName:   headerName,
		HeaderPrefix: headerPrefix,
	}
}

// Name returns the name of the authentication plugin.
func (a *APIKey) Name() string {
	return "apikey"
}

// Authenticate validates the API key from the request.
func (a *APIKey) Authenticate(ctx context.Context, r *http.Request) (*Result, error) {
	// Extract API key from header
	apiKey, err := a.extractKey(r)
	if err != nil {
		return nil, &AuthError{
			Code:    "missing_key",
			Message: err.Error(),
			Status:  http.StatusUnauthorized,
			Headers: map[string]string{
				"WWW-Authenticate": `ApiKey realm="api", error="invalid_key"`,
			},
		}
	}

	// Validate the key
	if !a.isValidKey(apiKey) {
		return nil, &AuthError{
			Code:    "invalid_key",
			Message: "invalid API key",
			Status:  http.StatusUnauthorized,
			Headers: map[string]string{
				"WWW-Authenticate": `ApiKey realm="api", error="invalid_key"`,
			},
		}
	}

	// Build headers to forward (strip API key header, add X-User-ID if configured)
	headers := make(http.Header)
	for name, values := range r.Header {
		// Skip API key header
		if equalFold(name, a.HeaderName) {
			continue
		}
		// Skip excluded headers
		if isHopByHopHeader(name) {
			continue
		}
		for _, value := range values {
			headers.Add(name, value)
		}
	}

	return &Result{
		Authenticated: true,
		Subject:       "", // API keys don't have a subject by default
		Claims:        make(map[string]interface{}),
		Headers:       headers,
	}, nil
}

// extractKey extracts the API key from the configured header.
func (a *APIKey) extractKey(r *http.Request) (string, error) {
	key := r.Header.Get(a.HeaderName)
	if key == "" {
		return "", fmt.Errorf("missing %s header", a.HeaderName)
	}

	if a.HeaderPrefix != "" {
		if !strings.HasPrefix(key, a.HeaderPrefix) {
			return "", fmt.Errorf("invalid %s header format", a.HeaderName)
		}
		return key[len(a.HeaderPrefix):], nil
	}

	return key, nil
}

// isValidKey checks if the provided key is in the list of valid keys.
// Uses constant-time comparison to prevent timing attacks.
func (a *APIKey) isValidKey(key string) bool {
	for _, validKey := range a.Keys {
		// Use subtle.ConstantTimeCompare for timing attack resistance
		if subtle.ConstantTimeCompare([]byte(key), []byte(validKey)) == 1 {
			return true
		}
	}
	return false
}
