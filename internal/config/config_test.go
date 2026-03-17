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
