package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// generateTestToken creates a test JWT token with the given claims and secret.
func generateTestToken(claims map[string]interface{}, secret []byte, alg string) string {
	// Create header
	header := map[string]string{
		"alg": alg,
		"typ": "JWT",
	}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	// Create payload
	payloadJSON, _ := json.Marshal(claims)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	// Create signature
	message := headerB64 + "." + payloadB64
	var sig []byte
	switch alg {
	case "HS256":
		h := hmac.New(sha256.New, secret)
		h.Write([]byte(message))
		sig = h.Sum(nil)
	case "HS384":
		h := hmac.New(sha512.New384, secret)
		h.Write([]byte(message))
		sig = h.Sum(nil)
	case "HS512":
		h := hmac.New(sha512.New, secret)
		h.Write([]byte(message))
		sig = h.Sum(nil)
	}
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	return message + "." + sigB64
}

// TestJWTName tests the JWT plugin name.
func TestJWTName(t *testing.T) {
	j := NewJWT([]byte("secret"))
	if j.Name() != "jwt" {
		t.Errorf("Expected name 'jwt', got %s", j.Name())
	}
}

// TestJWTNewJWTWithOptions tests creating JWT with custom options.
func TestJWTNewJWTWithOptions(t *testing.T) {
	j := NewJWTWithOptions(
		[]byte("my-secret"),
		"my-issuer",
		"my-audience",
		[]string{"HS256"},
	)

	if string(j.Secret) != "my-secret" {
		t.Error("Secret not set correctly")
	}
	if j.Issuer != "my-issuer" {
		t.Error("Issuer not set correctly")
	}
	if j.Audience != "my-audience" {
		t.Error("Audience not set correctly")
	}
	if len(j.AllowedAlgorithms) != 1 || j.AllowedAlgorithms[0] != "HS256" {
		t.Error("AllowedAlgorithms not set correctly")
	}
}

// TestJWTValidToken tests valid token authentication.
func TestJWTValidToken(t *testing.T) {
	secret := []byte("test-secret")
	j := NewJWT(secret)

	claims := map[string]interface{}{
		"sub":   "user123",
		"email": "user@example.com",
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
	}
	token := generateTestToken(claims, secret, "HS256")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	result, err := j.Authenticate(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result.Authenticated {
		t.Error("Expected Authenticated to be true")
	}
	if result.Subject != "user123" {
		t.Errorf("Expected subject 'user123', got %s", result.Subject)
	}
	if result.Claims["email"] != "user@example.com" {
		t.Error("Email claim not found")
	}

	// Check X-User-ID header was set
	if result.Headers.Get("X-User-ID") != "user123" {
		t.Error("X-User-ID header not set")
	}
	if result.Headers.Get("X-User-Email") != "user@example.com" {
		t.Error("X-User-Email header not set")
	}

	// Authorization header should be stripped
	if result.Headers.Get("Authorization") != "" {
		t.Error("Authorization header should be stripped")
	}
}

// TestJWTExpiredToken tests expired token rejection.
func TestJWTExpiredToken(t *testing.T) {
	secret := []byte("test-secret")
	j := NewJWT(secret)

	claims := map[string]interface{}{
		"sub": "user123",
		"exp": float64(time.Now().Add(-time.Hour).Unix()), // Expired 1 hour ago
	}
	token := generateTestToken(claims, secret, "HS256")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	_, err := j.Authenticate(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for expired token")
	}

	authErr, ok := err.(*AuthError)
	if !ok {
		t.Fatal("Expected AuthError")
	}
	if authErr.Status != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, authErr.Status)
	}
	if !strings.Contains(authErr.Message, "expired") {
		t.Errorf("Expected 'expired' in error message, got: %s", authErr.Message)
	}

	// Check WWW-Authenticate header
	if authErr.Headers["WWW-Authenticate"] == "" {
		t.Error("Expected WWW-Authenticate header")
	}
}

// TestJWTInvalidSignature tests invalid signature rejection.
func TestJWTInvalidSignature(t *testing.T) {
	secret := []byte("test-secret")
	wrongSecret := []byte("wrong-secret")
	j := NewJWT(secret)

	claims := map[string]interface{}{
		"sub": "user123",
		"exp": float64(time.Now().Add(time.Hour).Unix()),
	}
	// Generate token with wrong secret
	token := generateTestToken(claims, wrongSecret, "HS256")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	_, err := j.Authenticate(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for invalid signature")
	}

	authErr, ok := err.(*AuthError)
	if !ok {
		t.Fatal("Expected AuthError")
	}
	if !strings.Contains(authErr.Message, "signature") {
		t.Errorf("Expected 'signature' in error message, got: %s", authErr.Message)
	}
}

// TestJWTWrongAlgorithm tests rejection of disallowed algorithm.
func TestJWTWrongAlgorithm(t *testing.T) {
	secret := []byte("test-secret")
	j := NewJWT(secret)
	j.AllowedAlgorithms = []string{"HS256"} // Only allow HS256

	claims := map[string]interface{}{
		"sub": "user123",
		"exp": float64(time.Now().Add(time.Hour).Unix()),
	}
	// Generate token with HS512
	token := generateTestToken(claims, secret, "HS512")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	_, err := j.Authenticate(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for wrong algorithm")
	}

	authErr, ok := err.(*AuthError)
	if !ok {
		t.Fatal("Expected AuthError")
	}
	if !strings.Contains(authErr.Message, "algorithm") {
		t.Errorf("Expected 'algorithm' in error message, got: %s", authErr.Message)
	}
}

// TestJWTMissingToken tests missing token handling.
func TestJWTMissingToken(t *testing.T) {
	secret := []byte("test-secret")
	j := NewJWT(secret)

	req := httptest.NewRequest("GET", "/test", nil)
	// No Authorization header

	_, err := j.Authenticate(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for missing token")
	}

	authErr, ok := err.(*AuthError)
	if !ok {
		t.Fatal("Expected AuthError")
	}
	if authErr.Code != "missing_token" {
		t.Errorf("Expected code 'missing_token', got %s", authErr.Code)
	}
	if !strings.Contains(authErr.Message, "missing") {
		t.Errorf("Expected 'missing' in error message, got: %s", authErr.Message)
	}
}

// TestJWTInvalidHeaderFormat tests invalid Authorization header format.
func TestJWTInvalidHeaderFormat(t *testing.T) {
	secret := []byte("test-secret")
	j := NewJWT(secret)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "invalid-format")

	_, err := j.Authenticate(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for invalid header format")
	}

	authErr, ok := err.(*AuthError)
	if !ok {
		t.Fatal("Expected AuthError")
	}
	if authErr.Code != "missing_token" {
		t.Errorf("Expected code 'missing_token', got %s", authErr.Code)
	}
}

// TestJWTInvalidTokenFormat tests invalid token format.
func TestJWTInvalidTokenFormat(t *testing.T) {
	secret := []byte("test-secret")
	j := NewJWT(secret)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer not.a.valid.jwt")

	_, err := j.Authenticate(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for invalid token format")
	}

	authErr, ok := err.(*AuthError)
	if !ok {
		t.Fatal("Expected AuthError")
	}
	if authErr.Code != "invalid_token" {
		t.Errorf("Expected code 'invalid_token', got %s", authErr.Code)
	}
}

// TestJWTIssuerValidation tests issuer validation.
func TestJWTIssuerValidation(t *testing.T) {
	secret := []byte("test-secret")
	j := NewJWTWithOptions(secret, "valid-issuer", "", []string{"HS256"})

	// Valid issuer
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "valid-issuer",
		"exp": float64(time.Now().Add(time.Hour).Unix()),
	}
	token := generateTestToken(claims, secret, "HS256")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	result, err := j.Authenticate(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result.Authenticated {
		t.Error("Expected authenticated for valid issuer")
	}

	// Invalid issuer
	claims["iss"] = "invalid-issuer"
	token = generateTestToken(claims, secret, "HS256")

	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	_, err = j.Authenticate(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for invalid issuer")
	}
	if !strings.Contains(err.Error(), "issuer") {
		t.Errorf("Expected 'issuer' in error message, got: %s", err.Error())
	}
}

// TestJWTAudienceValidationString tests audience validation with string claim.
func TestJWTAudienceValidationString(t *testing.T) {
	secret := []byte("test-secret")
	j := NewJWTWithOptions(secret, "", "valid-audience", []string{"HS256"})

	// Valid audience (string)
	claims := map[string]interface{}{
		"sub": "user123",
		"aud": "valid-audience",
		"exp": float64(time.Now().Add(time.Hour).Unix()),
	}
	token := generateTestToken(claims, secret, "HS256")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	result, err := j.Authenticate(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result.Authenticated {
		t.Error("Expected authenticated for valid audience")
	}

	// Invalid audience
	claims["aud"] = "invalid-audience"
	token = generateTestToken(claims, secret, "HS256")

	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	_, err = j.Authenticate(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for invalid audience")
	}
	if !strings.Contains(err.Error(), "audience") {
		t.Errorf("Expected 'audience' in error message, got: %s", err.Error())
	}
}

// TestJWTAudienceValidationArray tests audience validation with array claim.
func TestJWTAudienceValidationArray(t *testing.T) {
	secret := []byte("test-secret")
	j := NewJWTWithOptions(secret, "", "valid-audience", []string{"HS256"})

	// Valid audience (array containing target)
	claims := map[string]interface{}{
		"sub": "user123",
		"aud": []interface{}{"other-audience", "valid-audience"},
		"exp": float64(time.Now().Add(time.Hour).Unix()),
	}
	token := generateTestToken(claims, secret, "HS256")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	result, err := j.Authenticate(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result.Authenticated {
		t.Error("Expected authenticated for valid audience in array")
	}

	// Invalid audience (array not containing target)
	claims["aud"] = []interface{}{"other-audience", "another-audience"}
	token = generateTestToken(claims, secret, "HS256")

	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	_, err = j.Authenticate(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for invalid audience in array")
	}
}

// TestJWTMissingAudience tests missing audience claim when validation is required.
func TestJWTMissingAudience(t *testing.T) {
	secret := []byte("test-secret")
	j := NewJWTWithOptions(secret, "", "required-audience", []string{"HS256"})

	// No audience claim
	claims := map[string]interface{}{
		"sub": "user123",
		"exp": float64(time.Now().Add(time.Hour).Unix()),
	}
	token := generateTestToken(claims, secret, "HS256")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	_, err := j.Authenticate(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for missing audience")
	}
	if !strings.Contains(err.Error(), "audience") {
		t.Errorf("Expected 'audience' in error message, got: %s", err.Error())
	}
}

// TestJWTNotBefore tests not before (nbf) claim validation.
func TestJWTNotBefore(t *testing.T) {
	secret := []byte("test-secret")
	j := NewJWT(secret)

	// Token not yet valid
	claims := map[string]interface{}{
		"sub": "user123",
		"exp": float64(time.Now().Add(time.Hour).Unix()),
		"nbf": float64(time.Now().Add(time.Hour).Unix()), // Not valid for another hour
	}
	token := generateTestToken(claims, secret, "HS256")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	_, err := j.Authenticate(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for not yet valid token")
	}
	if !strings.Contains(err.Error(), "not yet valid") {
		t.Errorf("Expected 'not yet valid' in error message, got: %s", err.Error())
	}
}

// TestJWTAlgorithms tests different HMAC algorithms.
func TestJWTAlgorithms(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long!!!")

	tests := []struct {
		alg string
	}{
		{"HS256"},
		{"HS384"},
		{"HS512"},
	}

	for _, tt := range tests {
		j := NewJWT(secret)

		claims := map[string]interface{}{
			"sub": "user123",
			"exp": float64(time.Now().Add(time.Hour).Unix()),
		}
		token := generateTestToken(claims, secret, tt.alg)

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		result, err := j.Authenticate(context.Background(), req)
		if err != nil {
			t.Errorf("%s: Unexpected error: %v", tt.alg, err)
			continue
		}
		if !result.Authenticated {
			t.Errorf("%s: Expected authenticated", tt.alg)
		}
	}
}

// TestJWTCustomHeader tests custom header name and prefix.
func TestJWTCustomHeader(t *testing.T) {
	secret := []byte("test-secret")
	j := NewJWT(secret)
	j.HeaderName = "X-Custom-Auth"
	j.HeaderPrefix = "Token "

	claims := map[string]interface{}{
		"sub": "user123",
		"exp": float64(time.Now().Add(time.Hour).Unix()),
	}
	token := generateTestToken(claims, secret, "HS256")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Custom-Auth", "Token "+token)

	result, err := j.Authenticate(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result.Authenticated {
		t.Error("Expected authenticated")
	}

	// Should fail with wrong header
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	_, err = j.Authenticate(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for wrong header name")
	}
}

// TestJWTNoPrefix tests token without prefix.
func TestJWTNoPrefix(t *testing.T) {
	secret := []byte("test-secret")
	j := NewJWT(secret)
	j.HeaderPrefix = "" // No prefix

	claims := map[string]interface{}{
		"sub": "user123",
		"exp": float64(time.Now().Add(time.Hour).Unix()),
	}
	token := generateTestToken(claims, secret, "HS256")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", token) // No "Bearer " prefix

	result, err := j.Authenticate(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result.Authenticated {
		t.Error("Expected authenticated")
	}
}

// TestJWTNoSubject tests token without subject claim.
func TestJWTNoSubject(t *testing.T) {
	secret := []byte("test-secret")
	j := NewJWT(secret)

	claims := map[string]interface{}{
		"email": "user@example.com",
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
		// No "sub" claim
	}
	token := generateTestToken(claims, secret, "HS256")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	result, err := j.Authenticate(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result.Authenticated {
		t.Error("Expected authenticated")
	}
	if result.Subject != "" {
		t.Errorf("Expected empty subject, got %s", result.Subject)
	}
	// X-User-ID should not be set
	if result.Headers.Get("X-User-ID") != "" {
		t.Error("X-User-ID should not be set when no subject")
	}
}

// TestJWTCustomClaims tests that custom claims are preserved.
func TestJWTCustomClaims(t *testing.T) {
	secret := []byte("test-secret")
	j := NewJWT(secret)

	claims := map[string]interface{}{
		"sub":  "user123",
		"exp":  float64(time.Now().Add(time.Hour).Unix()),
		"role": "admin",
		"org":  "engineering",
	}
	token := generateTestToken(claims, secret, "HS256")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	result, err := j.Authenticate(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Claims["role"] != "admin" {
		t.Errorf("Expected role claim 'admin', got %v", result.Claims["role"])
	}
	if result.Claims["org"] != "engineering" {
		t.Errorf("Expected org claim 'engineering', got %v", result.Claims["org"])
	}
}

// TestJWTForwardingHeaders tests that non-auth headers are forwarded.
func TestJWTForwardingHeaders(t *testing.T) {
	secret := []byte("test-secret")
	j := NewJWT(secret)

	claims := map[string]interface{}{
		"sub": "user123",
		"exp": float64(time.Now().Add(time.Hour).Unix()),
	}
	token := generateTestToken(claims, secret, "HS256")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Custom-Header", "custom-value")
	req.Header.Set("Accept", "application/json")

	result, err := j.Authenticate(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Headers.Get("X-Custom-Header") != "custom-value" {
		t.Error("X-Custom-Header not forwarded")
	}
	if result.Headers.Get("Accept") != "application/json" {
		t.Error("Accept header not forwarded")
	}
}

// BenchmarkJWTValidate benchmarks JWT validation.
func BenchmarkJWTValidate(b *testing.B) {
	secret := []byte("test-secret")
	j := NewJWT(secret)

	claims := map[string]interface{}{
		"sub":   "user123",
		"email": "user@example.com",
		"role":  "admin",
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
	}
	token := generateTestToken(claims, secret, "HS256")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		j.Authenticate(ctx, req)
	}
}
