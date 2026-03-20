package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Check server defaults
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("expected read_timeout 30s, got %v", cfg.Server.ReadTimeout)
	}

	// Check gateway defaults
	if cfg.Gateway.RESTPrefix != "/api" {
		t.Errorf("expected rest_prefix /api, got %s", cfg.Gateway.RESTPrefix)
	}
	if cfg.Gateway.GraphQLPath != "/graphql" {
		t.Errorf("expected graphql_path /graphql, got %s", cfg.Gateway.GraphQLPath)
	}

	// Check rate limit defaults
	if !cfg.Gateway.RateLimit.Enabled {
		t.Error("expected rate_limit enabled by default")
	}
	if cfg.Gateway.RateLimit.RequestsPerSecond != 100 {
		t.Errorf("expected requests_per_second 100, got %d", cfg.Gateway.RateLimit.RequestsPerSecond)
	}

	// Check cache defaults
	if !cfg.Gateway.Cache.Enabled {
		t.Error("expected cache enabled by default")
	}
}

func TestLoadSimpleConfig(t *testing.T) {
	yaml := `
server:
  port: 9090
  read_timeout: 45s

gateway:
  rest_prefix: "/v1"
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(configPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Check overridden values
	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeout != 45*time.Second {
		t.Errorf("expected read_timeout 45s, got %v", cfg.Server.ReadTimeout)
	}
	if cfg.Gateway.RESTPrefix != "/v1" {
		t.Errorf("expected rest_prefix /v1, got %s", cfg.Gateway.RESTPrefix)
	}

	// Check defaults are preserved
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected default host 0.0.0.0, got %s", cfg.Server.Host)
	}
}

func TestLoadWithEnvInterpolation(t *testing.T) {
	t.Setenv("TEST_PORT", "7070")
	t.Setenv("TEST_HOST", "127.0.0.1")

	yaml := `
server:
  port: ${TEST_PORT}
  host: ${TEST_HOST}
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(configPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Server.Port != 7070 {
		t.Errorf("expected port 7070 from env, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1 from env, got %s", cfg.Server.Host)
	}
}

func TestLoadWithUpstream(t *testing.T) {
	yaml := `
upstreams:
  - name: test-api
    type: rest
    base_url: https://api.example.com
    openapi:
      source: https://api.example.com/openapi.json
      refresh_interval: 1h
    timeout: 30s
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(configPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Upstreams) != 1 {
		t.Fatalf("expected 1 upstream, got %d", len(cfg.Upstreams))
	}

	up := cfg.Upstreams[0]
	if up.Name != "test-api" {
		t.Errorf("expected name 'test-api', got %s", up.Name)
	}
	if up.Type != "rest" {
		t.Errorf("expected type 'rest', got %s", up.Type)
	}
	if up.BaseURL != "https://api.example.com" {
		t.Errorf("expected base_url 'https://api.example.com', got %s", up.BaseURL)
	}
	if up.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", up.Timeout)
	}
}

func TestValidateServerConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ServerConfig
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: ServerConfig{
				Port:           8080,
				ReadTimeout:    30 * time.Second,
				WriteTimeout:   30 * time.Second,
				IdleTimeout:    120 * time.Second,
				MaxHeaderBytes: 1048576,
			},
			wantErr: false,
		},
		{
			name: "invalid port (too low)",
			cfg: ServerConfig{
				Port:           0,
				ReadTimeout:    30 * time.Second,
				WriteTimeout:   30 * time.Second,
				IdleTimeout:    120 * time.Second,
				MaxHeaderBytes: 1048576,
			},
			wantErr: true,
		},
		{
			name: "invalid port (too high)",
			cfg: ServerConfig{
				Port:           70000,
				ReadTimeout:    30 * time.Second,
				WriteTimeout:   30 * time.Second,
				IdleTimeout:    120 * time.Second,
				MaxHeaderBytes: 1048576,
			},
			wantErr: true,
		},
		{
			name: "zero read timeout",
			cfg: ServerConfig{
				Port:           8080,
				ReadTimeout:    0,
				WriteTimeout:   30 * time.Second,
				IdleTimeout:    120 * time.Second,
				MaxHeaderBytes: 1048576,
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateUpstreamConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     UpstreamConfig
		wantErr bool
	}{
		{
			name: "valid REST upstream",
			cfg: UpstreamConfig{
				Name:     "test-api",
				Type:     "rest",
				BaseURL:  "https://api.example.com",
				Timeout:  30 * time.Second,
				OpenAPI:  &OpenAPIConfig{Source: "https://api.example.com/openapi.json"},
			},
			wantErr: false,
		},
		{
			name: "valid GraphQL upstream",
			cfg: UpstreamConfig{
				Name:     "test-graphql",
				Type:     "graphql",
				Endpoint: "https://graphql.example.com/query",
				Timeout:  30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			cfg: UpstreamConfig{
				Type:     "rest",
				BaseURL:  "https://api.example.com",
				Timeout:  30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			cfg: UpstreamConfig{
				Name:     "test",
				Type:     "invalid",
				BaseURL:  "https://api.example.com",
				Timeout:  30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "REST missing openapi source",
			cfg: UpstreamConfig{
				Name:     "test",
				Type:     "rest",
				BaseURL:  "https://api.example.com",
				Timeout:  30 * time.Second,
				OpenAPI:  nil,
			},
			wantErr: true,
		},
		{
			name: "zero timeout",
			cfg: UpstreamConfig{
				Name:     "test",
				Type:     "rest",
				BaseURL:  "https://api.example.com",
				Timeout:  0,
				OpenAPI:  &OpenAPIConfig{Source: "https://api.example.com/openapi.json"},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateAuthConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     AuthConfig
		wantErr bool
	}{
		{
			name: "none auth",
			cfg: AuthConfig{
				Strategy: "none",
			},
			wantErr: false,
		},
		{
			name: "forward auth with header",
			cfg: AuthConfig{
				Strategy: "forward",
				Header:   "Authorization",
			},
			wantErr: false,
		},
		{
			name: "forward auth missing header",
			cfg: AuthConfig{
				Strategy: "forward",
			},
			wantErr: true,
		},
		{
			name: "bearer auth with token",
			cfg: AuthConfig{
				Strategy: "bearer",
				Token:    "secret-token",
			},
			wantErr: false,
		},
		{
			name: "bearer auth missing token",
			cfg: AuthConfig{
				Strategy: "bearer",
			},
			wantErr: true,
		},
		{
			name: "basic auth with credentials",
			cfg: AuthConfig{
				Strategy: "basic",
				Username: "admin",
				Password: "secret",
			},
			wantErr: false,
		},
		{
			name: "basic auth missing credentials",
			cfg: AuthConfig{
				Strategy: "basic",
				Username: "admin",
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestConfigMerge(t *testing.T) {
	base := DefaultConfig()
	base.Server.Host = "127.0.0.1"
	base.Server.Port = 8080

	source := &Config{
		Server: ServerConfig{
			Port: 9090,
		},
		Upstreams: []UpstreamConfig{
			{Name: "new-upstream", Type: "rest"},
		},
	}

	base.Merge(source)

	// Host should be preserved
	if base.Server.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1 after merge, got %s", base.Server.Host)
	}

	// Port should be updated
	if base.Server.Port != 9090 {
		t.Errorf("expected port 9090 after merge, got %d", base.Server.Port)
	}

	// Upstreams should be appended
	if len(base.Upstreams) != 1 {
		t.Errorf("expected 1 upstream after merge, got %d", len(base.Upstreams))
	}
}

func TestConfigString(t *testing.T) {
	cfg := DefaultConfig()
	s := cfg.String()

	expectedParts := []string{
		"server=0.0.0.0:8080",
		"upstreams=0",
		"rate_limit=true",
		"cache=true",
		"mcp=true",
	}

	for _, part := range expectedParts {
		if !contains(s, part) {
			t.Errorf("expected String() to contain %q, got %q", part, s)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- Additional tests for coverage ---

func TestLoadOrDefault_FileNotExists(t *testing.T) {
	cfg, err := LoadOrDefault("/nonexistent/path/tentaserve.yaml")
	if err != nil {
		t.Fatalf("LoadOrDefault should not error on missing file, got: %v", err)
	}
	// Should return default config
	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected default host 0.0.0.0, got %s", cfg.Server.Host)
	}
}

func TestLoadOrDefault_FileExists(t *testing.T) {
	yamlContent := `
server:
  port: 9999
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadOrDefault(configPath)
	if err != nil {
		t.Fatalf("LoadOrDefault failed: %v", err)
	}
	if cfg.Server.Port != 9999 {
		t.Errorf("expected port 9999, got %d", cfg.Server.Port)
	}
}

func TestFixConfigFields_CacheMaxSize(t *testing.T) {
	tests := []struct {
		name      string
		raw       map[string]any
		wantSize  int64
		wantEntry int64
		wantErr   bool
	}{
		{
			name: "valid max_size string",
			raw: map[string]any{
				"gateway": map[string]any{
					"cache": map[string]any{
						"max_size": "128MB",
					},
				},
			},
			wantSize: 128 * 1024 * 1024,
			wantErr:  false,
		},
		{
			name: "valid max_entry_size string",
			raw: map[string]any{
				"gateway": map[string]any{
					"cache": map[string]any{
						"max_entry_size": "2MB",
					},
				},
			},
			wantEntry: 2 * 1024 * 1024,
			wantErr:   false,
		},
		{
			name: "invalid max_size string",
			raw: map[string]any{
				"gateway": map[string]any{
					"cache": map[string]any{
						"max_size": "notanumber",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid max_entry_size string",
			raw: map[string]any{
				"gateway": map[string]any{
					"cache": map[string]any{
						"max_entry_size": "badvalue",
					},
				},
			},
			wantErr: true,
		},
		{
			name:    "no gateway key",
			raw:     map[string]any{},
			wantErr: false,
		},
		{
			name: "gateway but no cache key",
			raw: map[string]any{
				"gateway": map[string]any{},
			},
			wantErr: false,
		},
		{
			name: "empty max_size string",
			raw: map[string]any{
				"gateway": map[string]any{
					"cache": map[string]any{
						"max_size": "",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultConfig()
			err := fixConfigFields(cfg, tc.raw)
			if (err != nil) != tc.wantErr {
				t.Errorf("fixConfigFields() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if !tc.wantErr {
				if tc.wantSize != 0 && cfg.Gateway.Cache.MaxSize != tc.wantSize {
					t.Errorf("expected MaxSize %d, got %d", tc.wantSize, cfg.Gateway.Cache.MaxSize)
				}
				if tc.wantEntry != 0 && cfg.Gateway.Cache.MaxEntrySize != tc.wantEntry {
					t.Errorf("expected MaxEntrySize %d, got %d", tc.wantEntry, cfg.Gateway.Cache.MaxEntrySize)
				}
			}
		})
	}
}

func TestValidateGatewayConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     GatewayConfig
		wantErr bool
	}{
		{
			name: "valid gateway",
			cfg: GatewayConfig{
				RESTPrefix:  "/api",
				GraphQLPath: "/graphql",
				MCPPath:     "/mcp",
				RateLimit:   RateLimitConfig{Enabled: false},
				CircuitBreaker: CircuitBreakerConfig{Enabled: false},
				Cache:       CacheConfig{Enabled: false},
			},
			wantErr: false,
		},
		{
			name: "empty rest_prefix",
			cfg: GatewayConfig{
				RESTPrefix:  "",
				GraphQLPath: "/graphql",
				MCPPath:     "/mcp",
			},
			wantErr: true,
		},
		{
			name: "empty graphql_path",
			cfg: GatewayConfig{
				RESTPrefix:  "/api",
				GraphQLPath: "",
				MCPPath:     "/mcp",
			},
			wantErr: true,
		},
		{
			name: "empty mcp_path",
			cfg: GatewayConfig{
				RESTPrefix:  "/api",
				GraphQLPath: "/graphql",
				MCPPath:     "",
			},
			wantErr: true,
		},
		{
			name: "conflicting rest_prefix and graphql_path",
			cfg: GatewayConfig{
				RESTPrefix:  "/api",
				GraphQLPath: "/api",
				MCPPath:     "/mcp",
				RateLimit:   RateLimitConfig{Enabled: false},
				CircuitBreaker: CircuitBreakerConfig{Enabled: false},
				Cache:       CacheConfig{Enabled: false},
			},
			wantErr: true,
		},
		{
			name: "conflicting rest_prefix and mcp_path",
			cfg: GatewayConfig{
				RESTPrefix:  "/mcp",
				GraphQLPath: "/graphql",
				MCPPath:     "/mcp",
				RateLimit:   RateLimitConfig{Enabled: false},
				CircuitBreaker: CircuitBreakerConfig{Enabled: false},
				Cache:       CacheConfig{Enabled: false},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateRateLimitConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     RateLimitConfig
		wantErr bool
	}{
		{
			name:    "disabled",
			cfg:     RateLimitConfig{Enabled: false},
			wantErr: false,
		},
		{
			name:    "valid enabled",
			cfg:     RateLimitConfig{Enabled: true, RequestsPerSecond: 10, BurstSize: 20},
			wantErr: false,
		},
		{
			name:    "zero requests_per_second",
			cfg:     RateLimitConfig{Enabled: true, RequestsPerSecond: 0, BurstSize: 20},
			wantErr: true,
		},
		{
			name:    "zero burst_size",
			cfg:     RateLimitConfig{Enabled: true, RequestsPerSecond: 10, BurstSize: 0},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateCircuitBreakerConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     CircuitBreakerConfig
		wantErr bool
	}{
		{
			name:    "disabled",
			cfg:     CircuitBreakerConfig{Enabled: false},
			wantErr: false,
		},
		{
			name:    "valid enabled",
			cfg:     CircuitBreakerConfig{Enabled: true, FailureThreshold: 5, ResetTimeout: 30 * time.Second, HalfOpenRequests: 2},
			wantErr: false,
		},
		{
			name:    "zero failure_threshold",
			cfg:     CircuitBreakerConfig{Enabled: true, FailureThreshold: 0, ResetTimeout: 30 * time.Second, HalfOpenRequests: 2},
			wantErr: true,
		},
		{
			name:    "zero reset_timeout",
			cfg:     CircuitBreakerConfig{Enabled: true, FailureThreshold: 5, ResetTimeout: 0, HalfOpenRequests: 2},
			wantErr: true,
		},
		{
			name:    "zero half_open_requests",
			cfg:     CircuitBreakerConfig{Enabled: true, FailureThreshold: 5, ResetTimeout: 30 * time.Second, HalfOpenRequests: 0},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateCacheConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     CacheConfig
		wantErr bool
	}{
		{
			name:    "disabled",
			cfg:     CacheConfig{Enabled: false},
			wantErr: false,
		},
		{
			name:    "valid enabled",
			cfg:     CacheConfig{Enabled: true, MaxSize: 4096, TTL: time.Minute, MaxEntrySize: 1024},
			wantErr: false,
		},
		{
			name:    "too small max_size",
			cfg:     CacheConfig{Enabled: true, MaxSize: 100, TTL: time.Minute, MaxEntrySize: 1024},
			wantErr: true,
		},
		{
			name:    "zero ttl",
			cfg:     CacheConfig{Enabled: true, MaxSize: 4096, TTL: 0, MaxEntrySize: 1024},
			wantErr: true,
		},
		{
			name:    "zero max_entry_size",
			cfg:     CacheConfig{Enabled: true, MaxSize: 4096, TTL: time.Minute, MaxEntrySize: 0},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateTLSConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     TLSConfig
		wantErr bool
	}{
		{
			name:    "valid TLS",
			cfg:     TLSConfig{CertFile: "/path/to/cert.pem", KeyFile: "/path/to/key.pem"},
			wantErr: false,
		},
		{
			name:    "missing cert_file",
			cfg:     TLSConfig{CertFile: "", KeyFile: "/path/to/key.pem"},
			wantErr: true,
		},
		{
			name:    "missing key_file",
			cfg:     TLSConfig{CertFile: "/path/to/cert.pem", KeyFile: ""},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateServerConfigWithTLS(t *testing.T) {
	cfg := ServerConfig{
		Port:           8080,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1048576,
		TLS: &TLSConfig{
			CertFile: "/cert.pem",
			KeyFile:  "/key.pem",
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid config with TLS, got %v", err)
	}

	cfg.TLS.CertFile = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for TLS without cert_file")
	}
}

func TestValidateRetryConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     RetryConfig
		wantErr bool
	}{
		{
			name:    "zero max_attempts",
			cfg:     RetryConfig{MaxAttempts: 0},
			wantErr: false,
		},
		{
			name:    "negative max_attempts",
			cfg:     RetryConfig{MaxAttempts: -1},
			wantErr: true,
		},
		{
			name:    "valid with exponential backoff",
			cfg:     RetryConfig{MaxAttempts: 3, Backoff: "exponential"},
			wantErr: false,
		},
		{
			name:    "valid with linear backoff",
			cfg:     RetryConfig{MaxAttempts: 3, Backoff: "linear"},
			wantErr: false,
		},
		{
			name:    "valid with fixed backoff",
			cfg:     RetryConfig{MaxAttempts: 3, Backoff: "fixed"},
			wantErr: false,
		},
		{
			name:    "invalid backoff strategy",
			cfg:     RetryConfig{MaxAttempts: 3, Backoff: "random"},
			wantErr: true,
		},
		{
			name:    "empty backoff defaults to exponential",
			cfg:     RetryConfig{MaxAttempts: 3, Backoff: ""},
			wantErr: false,
		},
		{
			name:    "negative base_delay",
			cfg:     RetryConfig{MaxAttempts: 3, Backoff: "exponential", BaseDelay: -1},
			wantErr: true,
		},
		{
			name:    "negative max_delay",
			cfg:     RetryConfig{MaxAttempts: 3, Backoff: "exponential", MaxDelay: -1},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateSchemaConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     SchemaConfig
		wantErr bool
	}{
		{
			name: "valid",
			cfg: SchemaConfig{
				Pagination: PaginationConfig{DefaultLimit: 10, MaxLimit: 100},
				Limits:     SchemaLimits{MaxDepth: 10, MaxComplexity: 1000},
			},
			wantErr: false,
		},
		{
			name: "invalid pagination default_limit",
			cfg: SchemaConfig{
				Pagination: PaginationConfig{DefaultLimit: 0, MaxLimit: 100},
				Limits:     SchemaLimits{MaxDepth: 10, MaxComplexity: 1000},
			},
			wantErr: true,
		},
		{
			name: "max_limit less than default_limit",
			cfg: SchemaConfig{
				Pagination: PaginationConfig{DefaultLimit: 50, MaxLimit: 20},
				Limits:     SchemaLimits{MaxDepth: 10, MaxComplexity: 1000},
			},
			wantErr: true,
		},
		{
			name: "invalid max_depth",
			cfg: SchemaConfig{
				Pagination: PaginationConfig{DefaultLimit: 10, MaxLimit: 100},
				Limits:     SchemaLimits{MaxDepth: 0, MaxComplexity: 1000},
			},
			wantErr: true,
		},
		{
			name: "invalid max_complexity",
			cfg: SchemaConfig{
				Pagination: PaginationConfig{DefaultLimit: 10, MaxLimit: 100},
				Limits:     SchemaLimits{MaxDepth: 10, MaxComplexity: 0},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateMCPConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     MCPConfig
		wantErr bool
	}{
		{
			name:    "disabled",
			cfg:     MCPConfig{Enabled: false},
			wantErr: false,
		},
		{
			name:    "enabled",
			cfg:     MCPConfig{Enabled: true},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateObservabilityConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ObservabilityConfig
		wantErr bool
	}{
		{
			name: "valid",
			cfg: ObservabilityConfig{
				Logging:   LoggingConfig{Level: "info", Format: "json"},
				Metrics:   MetricsConfig{Enabled: true, Path: "/-/metrics"},
				Health:    HealthConfig{Enabled: true, Path: "/-/health"},
				RequestID: RequestIDConfig{Header: "X-Request-ID"},
			},
			wantErr: false,
		},
		{
			name: "invalid logging level",
			cfg: ObservabilityConfig{
				Logging:   LoggingConfig{Level: "trace", Format: "json"},
				Metrics:   MetricsConfig{Enabled: true, Path: "/-/metrics"},
				Health:    HealthConfig{Enabled: true, Path: "/-/health"},
				RequestID: RequestIDConfig{Header: "X-Request-ID"},
			},
			wantErr: true,
		},
		{
			name: "invalid logging format",
			cfg: ObservabilityConfig{
				Logging:   LoggingConfig{Level: "info", Format: "xml"},
				Metrics:   MetricsConfig{Enabled: true, Path: "/-/metrics"},
				Health:    HealthConfig{Enabled: true, Path: "/-/health"},
				RequestID: RequestIDConfig{Header: "X-Request-ID"},
			},
			wantErr: true,
		},
		{
			name: "metrics enabled without path",
			cfg: ObservabilityConfig{
				Logging:   LoggingConfig{Level: "info", Format: "json"},
				Metrics:   MetricsConfig{Enabled: true, Path: ""},
				Health:    HealthConfig{Enabled: true, Path: "/-/health"},
				RequestID: RequestIDConfig{Header: "X-Request-ID"},
			},
			wantErr: true,
		},
		{
			name: "health enabled without path",
			cfg: ObservabilityConfig{
				Logging:   LoggingConfig{Level: "info", Format: "json"},
				Metrics:   MetricsConfig{Enabled: true, Path: "/-/metrics"},
				Health:    HealthConfig{Enabled: true, Path: ""},
				RequestID: RequestIDConfig{Header: "X-Request-ID"},
			},
			wantErr: true,
		},
		{
			name: "empty request_id header",
			cfg: ObservabilityConfig{
				Logging:   LoggingConfig{Level: "info", Format: "json"},
				Metrics:   MetricsConfig{Enabled: true, Path: "/-/metrics"},
				Health:    HealthConfig{Enabled: true, Path: "/-/health"},
				RequestID: RequestIDConfig{Header: ""},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateAdminConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     AdminConfig
		wantErr bool
	}{
		{
			name:    "disabled",
			cfg:     AdminConfig{Enabled: false},
			wantErr: false,
		},
		{
			name:    "enabled with path",
			cfg:     AdminConfig{Enabled: true, Path: "/-/admin"},
			wantErr: false,
		},
		{
			name:    "enabled without path",
			cfg:     AdminConfig{Enabled: true, Path: ""},
			wantErr: true,
		},
		{
			name: "enabled with basic auth",
			cfg: AdminConfig{
				Enabled: true,
				Path:    "/-/admin",
				Auth:    &AdminAuth{Type: "basic", Username: "admin", Password: "secret"},
			},
			wantErr: false,
		},
		{
			name: "enabled with bearer auth",
			cfg: AdminConfig{
				Enabled: true,
				Path:    "/-/admin",
				Auth:    &AdminAuth{Type: "bearer", Token: "mytoken"},
			},
			wantErr: false,
		},
		{
			name: "invalid auth type",
			cfg: AdminConfig{
				Enabled: true,
				Path:    "/-/admin",
				Auth:    &AdminAuth{Type: "oauth"},
			},
			wantErr: true,
		},
		{
			name: "basic auth missing password",
			cfg: AdminConfig{
				Enabled: true,
				Path:    "/-/admin",
				Auth:    &AdminAuth{Type: "basic", Username: "admin"},
			},
			wantErr: true,
		},
		{
			name: "bearer auth missing token",
			cfg: AdminConfig{
				Enabled: true,
				Path:    "/-/admin",
				Auth:    &AdminAuth{Type: "bearer"},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateFullConfig(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("default config should be valid, got: %v", err)
	}
}

func TestValidateFullConfigWithDuplicateUpstreams(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Upstreams = []UpstreamConfig{
		{Name: "api1", Type: "rest", BaseURL: "http://a.com", Timeout: 30 * time.Second, OpenAPI: &OpenAPIConfig{Source: "http://a.com/spec"}},
		{Name: "api1", Type: "rest", BaseURL: "http://b.com", Timeout: 30 * time.Second, OpenAPI: &OpenAPIConfig{Source: "http://b.com/spec"}},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for duplicate upstream names")
	}
}

func TestMerge_ExtendedFields(t *testing.T) {
	base := DefaultConfig()

	source := &Config{
		Server: ServerConfig{
			Host:           "192.168.1.1",
			ReadTimeout:    60 * time.Second,
			WriteTimeout:   60 * time.Second,
			IdleTimeout:    240 * time.Second,
			MaxHeaderBytes: 2 << 20,
			TLS:            &TLSConfig{CertFile: "cert.pem", KeyFile: "key.pem"},
		},
		Gateway: GatewayConfig{
			RESTPrefix:  "/v2",
			GraphQLPath: "/gql",
			MCPPath:     "/mcp-v2",
		},
		Observability: ObservabilityConfig{
			Logging: LoggingConfig{Level: "debug", Format: "text", Output: "stderr"},
		},
		Admin: AdminConfig{
			Enabled: true,
			Path:    "/-/admin-v2",
			Auth:    &AdminAuth{Type: "bearer", Token: "tok123"},
		},
	}

	base.Merge(source)

	if base.Server.Host != "192.168.1.1" {
		t.Errorf("expected host 192.168.1.1, got %s", base.Server.Host)
	}
	if base.Server.ReadTimeout != 60*time.Second {
		t.Errorf("expected ReadTimeout 60s, got %v", base.Server.ReadTimeout)
	}
	if base.Server.TLS == nil || base.Server.TLS.CertFile != "cert.pem" {
		t.Error("expected TLS to be merged")
	}
	if base.Gateway.RESTPrefix != "/v2" {
		t.Errorf("expected RESTPrefix /v2, got %s", base.Gateway.RESTPrefix)
	}
	if base.Gateway.GraphQLPath != "/gql" {
		t.Errorf("expected GraphQLPath /gql, got %s", base.Gateway.GraphQLPath)
	}
	if base.Gateway.MCPPath != "/mcp-v2" {
		t.Errorf("expected MCPPath /mcp-v2, got %s", base.Gateway.MCPPath)
	}
	if base.Observability.Logging.Level != "debug" {
		t.Errorf("expected logging level debug, got %s", base.Observability.Logging.Level)
	}
	if base.Observability.Logging.Format != "text" {
		t.Errorf("expected logging format text, got %s", base.Observability.Logging.Format)
	}
	if base.Observability.Logging.Output != "stderr" {
		t.Errorf("expected logging output stderr, got %s", base.Observability.Logging.Output)
	}
	if !base.Admin.Enabled {
		t.Error("expected admin enabled after merge")
	}
	if base.Admin.Path != "/-/admin-v2" {
		t.Errorf("expected admin path /-/admin-v2, got %s", base.Admin.Path)
	}
	if base.Admin.Auth == nil || base.Admin.Auth.Token != "tok123" {
		t.Error("expected admin auth to be merged")
	}
}

func TestIsValidName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantOk  bool
	}{
		{"valid lowercase", "myname", true},
		{"valid uppercase", "MYNAME", true},
		{"valid with digits", "api123", true},
		{"valid with hyphens", "my-api", true},
		{"valid with underscores", "my_api", true},
		{"valid mixed", "My-API_v2", true},
		{"empty string", "", false},
		{"space in name", "my api", false},
		{"special character", "my@api", false},
		{"dot in name", "my.api", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isValidName(tc.input)
			if got != tc.wantOk {
				t.Errorf("isValidName(%q) = %v, want %v", tc.input, got, tc.wantOk)
			}
		})
	}
}

func TestValidateUpstreamGraphQLMissingEndpoint(t *testing.T) {
	cfg := UpstreamConfig{
		Name:     "test-gql",
		Type:     "graphql",
		Endpoint: "",
		Timeout:  30 * time.Second,
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for GraphQL upstream missing endpoint")
	}
}

func TestValidateAuthConfigInvalidStrategy(t *testing.T) {
	cfg := AuthConfig{Strategy: "oauth2"}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for invalid auth strategy")
	}
}

func TestValidateAuthConfigDefaultStrategy(t *testing.T) {
	cfg := AuthConfig{Strategy: ""}
	err := cfg.Validate()
	if err != nil {
		t.Errorf("expected empty strategy to default to none, got error: %v", err)
	}
	if cfg.Strategy != "none" {
		t.Errorf("expected strategy to be set to 'none', got %q", cfg.Strategy)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bad.yaml")
	// Write content that will cause a parse or validation error: invalid port
	content := `
server:
  port: -1
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	_, err := Load(configPath)
	if err == nil {
		t.Error("expected error for invalid config, got nil")
	}
}

func TestLoadNonexistentFile(t *testing.T) {
	_, err := Load("/this/path/does/not/exist.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
