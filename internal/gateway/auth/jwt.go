package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash"
	"net/http"
	"strings"
	"time"
)

// JWT is an authentication plugin that validates JWT tokens locally.
// Supports HMAC-SHA algorithms: HS256, HS384, HS512.
type JWT struct {
	// Secret is the HMAC secret key
	Secret []byte

	// Issuer to validate (optional)
	Issuer string

	// Audience to validate (optional)
	Audience string

	// AllowedAlgorithms lists allowed signing algorithms
	AllowedAlgorithms []string

	// HeaderName is the header containing the JWT (default: Authorization)
	HeaderName string

	// HeaderPrefix is the token prefix (default: Bearer)
	HeaderPrefix string
}

// NewJWT creates a new JWT authentication plugin.
func NewJWT(secret []byte) *JWT {
	return &JWT{
		Secret:            secret,
		AllowedAlgorithms: []string{"HS256", "HS384", "HS512"},
		HeaderName:        "Authorization",
		HeaderPrefix:      "Bearer ",
	}
}

// NewJWTWithOptions creates a JWT plugin with custom options.
func NewJWTWithOptions(secret []byte, issuer, audience string, allowedAlgs []string) *JWT {
	j := NewJWT(secret)
	j.Issuer = issuer
	j.Audience = audience
	if len(allowedAlgs) > 0 {
		j.AllowedAlgorithms = allowedAlgs
	}
	return j
}

// Name returns the name of the authentication plugin.
func (j *JWT) Name() string {
	return "jwt"
}

// Authenticate validates the JWT token from the request.
func (j *JWT) Authenticate(ctx context.Context, r *http.Request) (*Result, error) {
	// Extract token from header
	token, err := j.extractToken(r)
	if err != nil {
		return nil, &AuthError{
			Code:    "missing_token",
			Message: err.Error(),
			Status:  http.StatusUnauthorized,
			Headers: map[string]string{
				"WWW-Authenticate": `Bearer realm="api", error="invalid_token"`,
			},
		}
	}

	// Parse and validate token
	claims, err := j.validateToken(token)
	if err != nil {
		return nil, &AuthError{
			Code:    "invalid_token",
			Message: err.Error(),
			Status:  http.StatusUnauthorized,
			Headers: map[string]string{
				"WWW-Authenticate": `Bearer realm="api", error="invalid_token"`,
			},
		}
	}

	// Extract subject from claims
	subject := ""
	if sub, ok := claims["sub"].(string); ok {
		subject = sub
	}

	// Build headers to forward (strip Authorization, add X-User-* headers)
	headers := make(http.Header)
	for name, values := range r.Header {
		// Skip Authorization header
		if equalFold(name, j.HeaderName) {
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

	// Add user info headers
	if subject != "" {
		headers.Set("X-User-ID", subject)
	}
	if email, ok := claims["email"].(string); ok {
		headers.Set("X-User-Email", email)
	}

	return &Result{
		Authenticated: true,
		Subject:       subject,
		Claims:        claims,
		Headers:       headers,
	}, nil
}

// extractToken extracts the JWT from the Authorization header.
func (j *JWT) extractToken(r *http.Request) (string, error) {
	auth := r.Header.Get(j.HeaderName)
	if auth == "" {
		return "", fmt.Errorf("missing %s header", j.HeaderName)
	}

	if j.HeaderPrefix != "" {
		if !strings.HasPrefix(auth, j.HeaderPrefix) {
			return "", fmt.Errorf("invalid %s header format", j.HeaderName)
		}
		return auth[len(j.HeaderPrefix):], nil
	}

	return auth, nil
}

// validateToken parses and validates a JWT token.
func (j *JWT) validateToken(token string) (map[string]interface{}, error) {
	// Split token into parts
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	// Decode header
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid header encoding: %w", err)
	}

	var header map[string]interface{}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("invalid header JSON: %w", err)
	}

	// Check algorithm
	alg, ok := header["alg"].(string)
	if !ok {
		return nil, fmt.Errorf("missing algorithm in header")
	}

	if !j.isAllowedAlgorithm(alg) {
		return nil, fmt.Errorf("algorithm not allowed: %s", alg)
	}

	// Decode payload
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid payload encoding: %w", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, fmt.Errorf("invalid payload JSON: %w", err)
	}

	// Validate signature
	if err := j.verifySignature(parts[0]+"."+parts[1], parts[2], alg); err != nil {
		return nil, fmt.Errorf("invalid signature: %w", err)
	}

	// Validate expiration
	if err := j.validateExpiration(claims); err != nil {
		return nil, err
	}

	// Validate issuer
	if j.Issuer != "" {
		if iss, ok := claims["iss"].(string); !ok || iss != j.Issuer {
			return nil, fmt.Errorf("invalid issuer")
		}
	}

	// Validate audience
	if j.Audience != "" {
		if err := j.validateAudience(claims); err != nil {
			return nil, err
		}
	}

	return claims, nil
}

// isAllowedAlgorithm checks if the algorithm is in the allowed list.
func (j *JWT) isAllowedAlgorithm(alg string) bool {
	for _, allowed := range j.AllowedAlgorithms {
		if allowed == alg {
			return true
		}
	}
	return false
}

// verifySignature verifies the HMAC signature.
func (j *JWT) verifySignature(message, signature, alg string) error {
	var hasher hash.Hash

	switch alg {
	case "HS256":
		hasher = hmac.New(sha256.New, j.Secret)
	case "HS384":
		hasher = hmac.New(sha512.New384, j.Secret)
	case "HS512":
		hasher = hmac.New(sha512.New, j.Secret)
	default:
		return fmt.Errorf("unsupported algorithm: %s", alg)
	}

	hasher.Write([]byte(message))
	expectedSig := hasher.Sum(nil)

	sigBytes, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}

	if !hmac.Equal(sigBytes, expectedSig) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}

// validateExpiration validates the token expiration.
func (j *JWT) validateExpiration(claims map[string]interface{}) error {
	// Check exp (expiration time)
	if exp, ok := claims["exp"]; ok {
		var expTime time.Time
		switch v := exp.(type) {
		case float64:
			expTime = time.Unix(int64(v), 0)
		case int64:
			expTime = time.Unix(v, 0)
		default:
			return fmt.Errorf("invalid exp claim type")
		}

		if time.Now().After(expTime) {
			return fmt.Errorf("token expired")
		}
	}

	// Check nbf (not before)
	if nbf, ok := claims["nbf"]; ok {
		var nbfTime time.Time
		switch v := nbf.(type) {
		case float64:
			nbfTime = time.Unix(int64(v), 0)
		case int64:
			nbfTime = time.Unix(v, 0)
		default:
			return fmt.Errorf("invalid nbf claim type")
		}

		if time.Now().Before(nbfTime) {
			return fmt.Errorf("token not yet valid")
		}
	}

	return nil
}

// validateAudience validates the token audience.
func (j *JWT) validateAudience(claims map[string]interface{}) error {
	aud, ok := claims["aud"]
	if !ok {
		return fmt.Errorf("missing audience claim")
	}

	switch v := aud.(type) {
	case string:
		if v != j.Audience {
			return fmt.Errorf("invalid audience")
		}
	case []interface{}:
		found := false
		for _, a := range v {
			if s, ok := a.(string); ok && s == j.Audience {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid audience")
		}
	default:
		return fmt.Errorf("invalid audience claim type")
	}

	return nil
}

// AuthError represents an authentication error with HTTP details.
type AuthError struct {
	Code    string
	Message string
	Status  int
	Headers map[string]string
}

// Error implements the error interface.
func (e *AuthError) Error() string {
	return e.Message
}

// isHopByHopHeader checks if a header is hop-by-hop.
func isHopByHopHeader(name string) bool {
	hopByHop := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"TE",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}

	for _, h := range hopByHop {
		if equalFold(name, h) {
			return true
		}
	}
	return false
}
