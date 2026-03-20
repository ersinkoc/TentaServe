package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPrintVersion(t *testing.T) {
	// Just make sure it doesn't panic
	printVersion()
}

func TestPrintUsage(t *testing.T) {
	// Just make sure it doesn't panic
	printUsage()
}

func TestGenerateJWT_HS256(t *testing.T) {
	claims := map[string]interface{}{
		"sub": "user123",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	}

	token, err := generateJWT("mysecret", "HS256", claims)
	if err != nil {
		t.Fatalf("generateJWT failed: %v", err)
	}

	if token == "" {
		t.Fatal("expected non-empty token")
	}

	// Token should have 3 parts
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Errorf("expected 3 parts in token, got %d", len(parts))
	}
}

func TestGenerateJWT_UnsupportedAlgorithm(t *testing.T) {
	claims := map[string]interface{}{
		"sub": "user123",
	}

	_, err := generateJWT("secret", "RS256", claims)
	if err == nil {
		t.Fatal("expected error for unsupported algorithm")
	}
	if !strings.Contains(err.Error(), "unsupported algorithm") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateJWT_Valid(t *testing.T) {
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "test-issuer",
		"aud": "test-audience",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	}

	token, err := generateJWT("mysecret", "HS256", claims)
	if err != nil {
		t.Fatalf("generateJWT failed: %v", err)
	}

	got, err := validateJWT(token, "mysecret", "test-issuer", "test-audience")
	if err != nil {
		t.Fatalf("validateJWT failed: %v", err)
	}

	if got["sub"] != "user123" {
		t.Errorf("expected sub=user123, got %v", got["sub"])
	}
}

func TestValidateJWT_InvalidFormat(t *testing.T) {
	_, err := validateJWT("not.a.valid.token.format", "secret", "", "")
	if err == nil {
		t.Fatal("expected error for invalid token format")
	}
}

func TestValidateJWT_WrongSecret(t *testing.T) {
	claims := map[string]interface{}{
		"sub": "user123",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	}

	token, err := generateJWT("correct-secret", "HS256", claims)
	if err != nil {
		t.Fatalf("generateJWT failed: %v", err)
	}

	_, err = validateJWT(token, "wrong-secret", "", "")
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
	if !strings.Contains(err.Error(), "signature mismatch") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateJWT_Expired(t *testing.T) {
	claims := map[string]interface{}{
		"sub": "user123",
		"exp": time.Now().Add(-1 * time.Hour).Unix(), // expired 1 hour ago
	}

	token, err := generateJWT("secret", "HS256", claims)
	if err != nil {
		t.Fatalf("generateJWT failed: %v", err)
	}

	_, err = validateJWT(token, "secret", "", "")
	if err == nil {
		t.Fatal("expected error for expired token")
	}
	if !strings.Contains(err.Error(), "token expired") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateJWT_InvalidIssuer(t *testing.T) {
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "wrong-issuer",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	}

	token, err := generateJWT("secret", "HS256", claims)
	if err != nil {
		t.Fatalf("generateJWT failed: %v", err)
	}

	_, err = validateJWT(token, "secret", "expected-issuer", "")
	if err == nil {
		t.Fatal("expected error for invalid issuer")
	}
	if !strings.Contains(err.Error(), "invalid issuer") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateJWT_InvalidAudience(t *testing.T) {
	claims := map[string]interface{}{
		"sub": "user123",
		"aud": "wrong-audience",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	}

	token, err := generateJWT("secret", "HS256", claims)
	if err != nil {
		t.Fatalf("generateJWT failed: %v", err)
	}

	_, err = validateJWT(token, "secret", "", "expected-audience")
	if err == nil {
		t.Fatal("expected error for invalid audience")
	}
	if !strings.Contains(err.Error(), "invalid audience") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateJWT_NoExpiry(t *testing.T) {
	// Token without exp claim should still validate
	claims := map[string]interface{}{
		"sub": "user123",
	}

	token, err := generateJWT("secret", "HS256", claims)
	if err != nil {
		t.Fatalf("generateJWT failed: %v", err)
	}

	got, err := validateJWT(token, "secret", "", "")
	if err != nil {
		t.Fatalf("validateJWT failed: %v", err)
	}
	if got["sub"] != "user123" {
		t.Errorf("expected sub=user123, got %v", got["sub"])
	}
}

func TestDecodeJWTParts_Valid(t *testing.T) {
	claims := map[string]interface{}{
		"sub":  "user123",
		"role": "admin",
		"exp":  time.Now().Add(1 * time.Hour).Unix(),
	}

	token, err := generateJWT("secret", "HS256", claims)
	if err != nil {
		t.Fatalf("generateJWT failed: %v", err)
	}

	header, payload, err := decodeJWTParts(token)
	if err != nil {
		t.Fatalf("decodeJWTParts failed: %v", err)
	}

	if header["alg"] != "HS256" {
		t.Errorf("expected alg=HS256, got %v", header["alg"])
	}
	if header["typ"] != "JWT" {
		t.Errorf("expected typ=JWT, got %v", header["typ"])
	}
	if payload["sub"] != "user123" {
		t.Errorf("expected sub=user123, got %v", payload["sub"])
	}
	if payload["role"] != "admin" {
		t.Errorf("expected role=admin, got %v", payload["role"])
	}
}

func TestDecodeJWTParts_InvalidFormat(t *testing.T) {
	_, _, err := decodeJWTParts("only-one-part")
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestDecodeJWTParts_InvalidBase64Header(t *testing.T) {
	_, _, err := decodeJWTParts("!!!invalid!!!.payload.sig")
	if err == nil {
		t.Fatal("expected error for invalid base64 header")
	}
}

func TestDecodeJWTParts_InvalidJSONHeader(t *testing.T) {
	// Valid base64 but invalid JSON
	_, _, err := decodeJWTParts("bm90anNvbg.cGF5bG9hZA.c2ln")
	if err == nil {
		t.Fatal("expected error for invalid JSON header")
	}
}

func TestValidateCmd_ValidConfig(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")

	configContent := `
server:
  host: "0.0.0.0"
  port: 8080
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	err := validateCmd([]string{"--config", configPath})
	if err != nil {
		t.Fatalf("validateCmd failed: %v", err)
	}
}

func TestValidateCmd_MissingFile(t *testing.T) {
	err := validateCmd([]string{"--config", "/nonexistent/path/config.yaml"})
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestValidateCmd_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bad.yaml")

	// Write content that will fail validation (invalid port)
	badContent := `
server:
  port: -1
`
	if err := os.WriteFile(configPath, []byte(badContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	err := validateCmd([]string{"--config", configPath})
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}

func TestJwtCmd_Help(t *testing.T) {
	err := jwtCmd([]string{"help"})
	if err != nil {
		t.Fatalf("jwtCmd help failed: %v", err)
	}
}

func TestJwtCmd_NoArgs(t *testing.T) {
	err := jwtCmd([]string{})
	if err == nil {
		t.Fatal("expected error for no args")
	}
}

func TestJwtCmd_UnknownSubcommand(t *testing.T) {
	err := jwtCmd([]string{"unknown"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
	if !strings.Contains(err.Error(), "unknown jwt subcommand") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestJwtCmd_Generate(t *testing.T) {
	err := jwtCmd([]string{"generate", "--secret", "testsecret", "--subject", "testuser"})
	if err != nil {
		t.Fatalf("jwtCmd generate failed: %v", err)
	}
}

func TestJwtCmd_Generate_NoSecret(t *testing.T) {
	err := jwtCmd([]string{"gen"})
	if err == nil {
		t.Fatal("expected error for generate without secret")
	}
}

func TestJwtCmd_Generate_WithSecretFile(t *testing.T) {
	tmpDir := t.TempDir()
	secretFile := filepath.Join(tmpDir, "secret.txt")
	if err := os.WriteFile(secretFile, []byte("file-secret"), 0644); err != nil {
		t.Fatalf("failed to write secret file: %v", err)
	}

	err := jwtCmd([]string{"gen", "--secret-file", secretFile, "--subject", "user1"})
	if err != nil {
		t.Fatalf("jwtCmd gen with secret-file failed: %v", err)
	}
}

func TestJwtCmd_Generate_WithClaims(t *testing.T) {
	err := jwtCmd([]string{"gen", "--secret", "mysecret", "--claims", `{"role":"admin"}`})
	if err != nil {
		t.Fatalf("jwtCmd gen with claims failed: %v", err)
	}
}

func TestJwtCmd_Generate_WithAllOptions(t *testing.T) {
	err := jwtCmd([]string{
		"gen",
		"--secret", "mysecret",
		"--issuer", "test-iss",
		"--audience", "test-aud",
		"--subject", "test-sub",
		"--expires", "2h",
	})
	if err != nil {
		t.Fatalf("jwtCmd gen with all options failed: %v", err)
	}
}

func TestJwtCmd_Validate(t *testing.T) {
	// First generate a token
	claims := map[string]interface{}{
		"sub": "user1",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	}
	token, err := generateJWT("testsecret", "HS256", claims)
	if err != nil {
		t.Fatalf("generateJWT failed: %v", err)
	}

	err = jwtCmd([]string{"validate", "--token", token, "--secret", "testsecret"})
	if err != nil {
		t.Fatalf("jwtCmd validate failed: %v", err)
	}
}

func TestJwtCmd_Validate_NoToken(t *testing.T) {
	err := jwtCmd([]string{"verify", "--secret", "testsecret"})
	if err == nil {
		t.Fatal("expected error for validate without token")
	}
}

func TestJwtCmd_Validate_NoSecret(t *testing.T) {
	err := jwtCmd([]string{"validate", "--token", "eyJ.test.sig"})
	if err == nil {
		t.Fatal("expected error for validate without secret")
	}
}

func TestJwtCmd_Decode(t *testing.T) {
	claims := map[string]interface{}{
		"sub": "user1",
	}
	token, err := generateJWT("secret", "HS256", claims)
	if err != nil {
		t.Fatalf("generateJWT failed: %v", err)
	}

	err = jwtCmd([]string{"decode", "--token", token})
	if err != nil {
		t.Fatalf("jwtCmd decode failed: %v", err)
	}
}

func TestJwtCmd_Decode_NoToken(t *testing.T) {
	err := jwtCmd([]string{"decode"})
	if err == nil {
		t.Fatal("expected error for decode without token")
	}
}

func TestValidateCmd_WithUpstreams(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")

	configContent := `
server:
  host: "0.0.0.0"
  port: 8080
upstreams:
  - name: gql-api
    type: graphql
    endpoint: "http://localhost:4000/graphql"
    timeout: 30s
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	err := validateCmd([]string{"--config", configPath})
	if err != nil {
		t.Fatalf("validateCmd failed: %v", err)
	}
}

func TestPrintJWTUsage(t *testing.T) {
	// Should not panic
	printJWTUsage()
}

func TestValidateJWT_InvalidBase64Payload(t *testing.T) {
	// Craft a token with valid header but invalid payload base64
	claims := map[string]interface{}{"sub": "test"}
	token, _ := generateJWT("secret", "HS256", claims)
	parts := strings.Split(token, ".")
	// Replace payload with invalid base64
	badToken := parts[0] + ".!!!invalid!!!" + "." + parts[2]
	_, err := validateJWT(badToken, "secret", "", "")
	if err == nil {
		t.Fatal("expected error for invalid base64 payload")
	}
}

func TestValidateJWT_TwoParts(t *testing.T) {
	_, err := validateJWT("only.two", "secret", "", "")
	if err == nil {
		t.Fatal("expected error for two-part token")
	}
}
