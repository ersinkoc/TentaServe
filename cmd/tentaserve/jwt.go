package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

// jwtCmd handles the "jwt" subcommand for token generation and validation.
func jwtCmd(args []string) error {
	if len(args) < 1 {
		printJWTUsage()
		return fmt.Errorf("jwt subcommand required")
	}

	switch args[0] {
	case "generate", "gen":
		return jwtGenerateCmd(args[1:])
	case "validate", "verify":
		return jwtValidateCmd(args[1:])
	case "decode":
		return jwtDecodeCmd(args[1:])
	case "help", "-h", "--help":
		printJWTUsage()
		return nil
	default:
		return fmt.Errorf("unknown jwt subcommand: %s", args[0])
	}
}

// jwtGenerateCmd generates a new JWT token.
func jwtGenerateCmd(args []string) error {
	fs := flag.NewFlagSet("jwt generate", flag.ExitOnError)
	secret := fs.String("secret", "", "JWT secret key (required)")
	secretFile := fs.String("secret-file", "", "File containing JWT secret")
	issuer := fs.String("issuer", "", "Token issuer (iss claim)")
	audience := fs.String("audience", "", "Token audience (aud claim)")
	subject := fs.String("subject", "", "Token subject (sub claim)")
	expires := fs.Duration("expires", 24*time.Hour, "Token expiration duration")
	algorithm := fs.String("alg", "HS256", "Signing algorithm (HS256, HS384, HS512)")
	claimsJSON := fs.String("claims", "", "Additional claims as JSON object")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Get secret
	var secretKey string
	if *secretFile != "" {
		data, err := os.ReadFile(*secretFile)
		if err != nil {
			return fmt.Errorf("reading secret file: %w", err)
		}
		secretKey = strings.TrimSpace(string(data))
	} else if *secret != "" {
		secretKey = *secret
	} else {
		return fmt.Errorf("--secret or --secret-file is required")
	}

	// Build claims
	claims := map[string]interface{}{
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(*expires).Unix(),
	}

	if *issuer != "" {
		claims["iss"] = *issuer
	}
	if *audience != "" {
		claims["aud"] = *audience
	}
	if *subject != "" {
		claims["sub"] = *subject
	}

	// Parse additional claims
	if *claimsJSON != "" {
		var additionalClaims map[string]interface{}
		if err := json.Unmarshal([]byte(*claimsJSON), &additionalClaims); err != nil {
			return fmt.Errorf("parsing claims JSON: %w", err)
		}
		for k, v := range additionalClaims {
			claims[k] = v
		}
	}

	// Generate token
	token, err := generateJWT(secretKey, *algorithm, claims)
	if err != nil {
		return fmt.Errorf("generating token: %w", err)
	}

	fmt.Println(token)
	return nil
}

// jwtValidateCmd validates a JWT token.
func jwtValidateCmd(args []string) error {
	fs := flag.NewFlagSet("jwt validate", flag.ExitOnError)
	token := fs.String("token", "", "JWT token to validate (required)")
	secret := fs.String("secret", "", "JWT secret key (required)")
	secretFile := fs.String("secret-file", "", "File containing JWT secret")
	issuer := fs.String("issuer", "", "Expected issuer")
	audience := fs.String("audience", "", "Expected audience")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *token == "" {
		return fmt.Errorf("--token is required")
	}

	// Get secret
	var secretKey string
	if *secretFile != "" {
		data, err := os.ReadFile(*secretFile)
		if err != nil {
			return fmt.Errorf("reading secret file: %w", err)
		}
		secretKey = strings.TrimSpace(string(data))
	} else if *secret != "" {
		secretKey = *secret
	} else {
		return fmt.Errorf("--secret or --secret-file is required")
	}

	// Validate token
	claims, err := validateJWT(*token, secretKey, *issuer, *audience)
	if err != nil {
		return fmt.Errorf("token validation failed: %w", err)
	}

	fmt.Println("Token is valid!")
	fmt.Println("Claims:")
	claimsJSON, _ := json.MarshalIndent(claims, "", "  ")
	fmt.Println(string(claimsJSON))
	return nil
}

// jwtDecodeCmd decodes a JWT token without verification.
func jwtDecodeCmd(args []string) error {
	fs := flag.NewFlagSet("jwt decode", flag.ExitOnError)
	token := fs.String("token", "", "JWT token to decode (required)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *token == "" {
		return fmt.Errorf("--token is required")
	}

	// Decode without verification
	header, payload, err := decodeJWTParts(*token)
	if err != nil {
		return fmt.Errorf("decoding token: %w", err)
	}

	fmt.Println("Header:")
	headerJSON, _ := json.MarshalIndent(header, "", "  ")
	fmt.Println(string(headerJSON))
	fmt.Println("\nPayload:")
	payloadJSON, _ := json.MarshalIndent(payload, "", "  ")
	fmt.Println(string(payloadJSON))
	return nil
}

// generateJWT creates a signed JWT token.
func generateJWT(secret, algorithm string, claims map[string]interface{}) (string, error) {
	// Build header
	header := map[string]string{
		"alg": algorithm,
		"typ": "JWT",
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}

	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	// Base64 encode
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	message := headerB64 + "." + payloadB64

	// Sign
	var signature []byte
	switch algorithm {
	case "HS256":
		h := hmac.New(sha256.New, []byte(secret))
		h.Write([]byte(message))
		signature = h.Sum(nil)
	default:
		return "", fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	signatureB64 := base64.RawURLEncoding.EncodeToString(signature)
	return message + "." + signatureB64, nil
}

// validateJWT validates a JWT token and returns its claims.
func validateJWT(token, secret, expectedIssuer, expectedAudience string) (map[string]interface{}, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	// Decode header
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid header: %w", err)
	}

	var header map[string]interface{}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("invalid header JSON: %w", err)
	}

	alg, ok := header["alg"].(string)
	if !ok {
		return nil, fmt.Errorf("missing algorithm")
	}
	_ = alg // Algorithm checked during signature verification

	// Decode payload
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid payload: %w", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, fmt.Errorf("invalid payload JSON: %w", err)
	}

	// Verify signature
	message := parts[0] + "." + parts[1]
	hasher := hmac.New(sha256.New, []byte(secret))
	hasher.Write([]byte(message))
	expectedSig := hasher.Sum(nil)

	actualSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding: %w", err)
	}

	if !hmac.Equal(actualSig, expectedSig) {
		return nil, fmt.Errorf("signature mismatch")
	}

	// Check expiration
	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return nil, fmt.Errorf("token expired")
		}
	}

	// Check issuer
	if expectedIssuer != "" {
		if iss, ok := claims["iss"].(string); !ok || iss != expectedIssuer {
			return nil, fmt.Errorf("invalid issuer")
		}
	}

	// Check audience
	if expectedAudience != "" {
		if aud, ok := claims["aud"].(string); !ok || aud != expectedAudience {
			return nil, fmt.Errorf("invalid audience")
		}
	}

	return claims, nil
}

// decodeJWTParts decodes a JWT without verifying the signature.
func decodeJWTParts(token string) (header, payload map[string]interface{}, err error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, nil, fmt.Errorf("invalid token format")
	}

	// Decode header
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, nil, fmt.Errorf("invalid header: %w", err)
	}

	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, nil, fmt.Errorf("invalid header JSON: %w", err)
	}

	// Decode payload
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, nil, fmt.Errorf("invalid payload: %w", err)
	}

	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return nil, nil, fmt.Errorf("invalid payload JSON: %w", err)
	}

	return header, payload, nil
}

func printJWTUsage() {
	fmt.Println(`JWT Token Utilities

Usage:
  tentaserve jwt <subcommand> [flags]

Subcommands:
  generate, gen    Generate a new JWT token
  validate, verify Validate a JWT token
  decode           Decode a JWT token without verification
  help             Show this help message

Examples:
  # Generate a token
  tentaserve jwt generate --secret "my-secret" --subject "user123" --expires 1h

  # Generate with claims
  tentaserve jwt gen --secret-file secret.txt --claims '{"role":"admin"}'

  # Validate a token
  tentaserve jwt validate --token "eyJ..." --secret "my-secret"

  # Decode a token
  tentaserve jwt decode --token "eyJ..."`)
}
