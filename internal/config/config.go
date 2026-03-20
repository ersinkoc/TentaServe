// Package config provides configuration loading and validation for Tentaserve.
//
// The config package supports YAML configuration files with environment
// variable interpolation. All configuration is validated at startup.
package config

import (
	"time"
)

// Config is the top-level configuration structure.
type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Gateway       GatewayConfig       `yaml:"gateway"`
	Upstreams     []UpstreamConfig    `yaml:"upstreams"`
	Schema        SchemaConfig        `yaml:"schema"`
	MCP           MCPConfig           `yaml:"mcp"`
	Observability ObservabilityConfig `yaml:"observability"`
	Admin         AdminConfig         `yaml:"admin"`
}

// ServerConfig contains HTTP server settings.
type ServerConfig struct {
	Host           string        `yaml:"host"`
	Port           int           `yaml:"port"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
	IdleTimeout    time.Duration `yaml:"idle_timeout"`
	MaxHeaderBytes int           `yaml:"max_header_bytes"`
	TLS            *TLSConfig    `yaml:"tls,omitempty"`
}

// TLSConfig contains TLS settings.
type TLSConfig struct {
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// GatewayConfig contains API gateway settings.
type GatewayConfig struct {
	RESTPrefix     string               `yaml:"rest_prefix"`
	GraphQLPath    string               `yaml:"graphql_path"`
	MCPPath        string               `yaml:"mcp_path"`
	RateLimit      RateLimitConfig      `yaml:"rate_limit"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
	Cache          CacheConfig          `yaml:"cache"`
	Auth           GatewayAuthConfig    `yaml:"auth"`
}

// RateLimitConfig contains rate limiting settings.
type RateLimitConfig struct {
	Enabled           bool `yaml:"enabled"`
	RequestsPerSecond int  `yaml:"requests_per_second"`
	BurstSize         int  `yaml:"burst_size"`
}

// CircuitBreakerConfig contains circuit breaker settings.
type CircuitBreakerConfig struct {
	Enabled          bool          `yaml:"enabled"`
	FailureThreshold int           `yaml:"failure_threshold"`
	ResetTimeout     time.Duration `yaml:"reset_timeout"`
	HalfOpenRequests int           `yaml:"half_open_requests"`
}

// GatewayAuthConfig contains authentication settings for the gateway.
type GatewayAuthConfig struct {
	Strategy string       `yaml:"strategy"` // "passthrough", "jwt", "apikey"
	JWT      JWTConfig    `yaml:"jwt"`
	APIKey   APIKeyConfig `yaml:"apikey"`
}

// JWTConfig contains JWT authentication settings.
type JWTConfig struct {
	Enabled           bool     `yaml:"enabled"`
	Secret            string   `yaml:"secret"`
	Issuer            string   `yaml:"issuer"`
	Audience          string   `yaml:"audience"`
	AllowedAlgorithms []string `yaml:"allowed_algorithms"`
	HeaderName        string   `yaml:"header_name"`
	HeaderPrefix      string   `yaml:"header_prefix"`
}

// APIKeyConfig contains API key authentication settings.
type APIKeyConfig struct {
	Enabled    bool              `yaml:"enabled"`
	Keys       []string          `yaml:"keys"`
	HeaderName string            `yaml:"header_name"`
	KeyMap     map[string]string `yaml:"key_map"` // key -> subject mapping
}

// CacheConfig contains caching settings.
type CacheConfig struct {
	Enabled      bool          `yaml:"enabled"`
	MaxSize      int64         `yaml:"max_size"`
	TTL          time.Duration `yaml:"ttl"`
	MaxEntrySize int64         `yaml:"max_entry_size"`
}

// UpstreamConfig defines a backend service.
type UpstreamConfig struct {
	Name     string            `yaml:"name"`
	Type     string            `yaml:"type"` // "rest" or "graphql"
	BaseURL  string            `yaml:"base_url"`
	Endpoint string            `yaml:"endpoint"`
	OpenAPI  *OpenAPIConfig    `yaml:"openapi,omitempty"`
	GraphQL  *GraphQLConfig    `yaml:"graphql,omitempty"`
	Timeout  time.Duration     `yaml:"timeout"`
	Retry    RetryConfig       `yaml:"retry"`
	Auth     AuthConfig        `yaml:"auth"`
	Headers  map[string]string `yaml:"headers"`
}

// OpenAPIConfig contains OpenAPI schema settings.
type OpenAPIConfig struct {
	Source          string        `yaml:"source"`
	RefreshInterval time.Duration `yaml:"refresh_interval"`
}

// GraphQLConfig contains GraphQL introspection settings.
type GraphQLConfig struct {
	Introspection IntrospectionConfig `yaml:"introspection"`
}

// IntrospectionConfig contains GraphQL introspection settings.
type IntrospectionConfig struct {
	Enabled         bool          `yaml:"enabled"`
	RefreshInterval time.Duration `yaml:"refresh_interval"`
}

// RetryConfig contains retry settings.
type RetryConfig struct {
	MaxAttempts int           `yaml:"max_attempts"`
	Backoff     string        `yaml:"backoff"` // "exponential", "linear", "fixed"
	BaseDelay   time.Duration `yaml:"base_delay"`
	MaxDelay    time.Duration `yaml:"max_delay"`
}

// AuthConfig contains authentication settings.
type AuthConfig struct {
	Strategy string `yaml:"strategy"` // "none", "forward", "bearer", "basic"
	Header   string `yaml:"header"`
	Token    string `yaml:"token"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// SchemaConfig contains schema translation settings.
type SchemaConfig struct {
	REST2GraphQL REST2GraphQLConfig `yaml:"rest2graphql"`
	GraphQL2REST GraphQL2RESTConfig `yaml:"graphql2rest"`
	Pagination   PaginationConfig   `yaml:"pagination"`
	Limits       SchemaLimits       `yaml:"limits"`
}

// REST2GraphQLConfig contains REST to GraphQL translation settings.
type REST2GraphQLConfig struct {
	Enabled bool `yaml:"enabled"`
}

// GraphQL2RESTConfig contains GraphQL to REST translation settings.
type GraphQL2RESTConfig struct {
	Enabled        bool   `yaml:"enabled"`
	EndpointPrefix string `yaml:"endpoint_prefix"`
}

// PaginationConfig contains pagination settings.
type PaginationConfig struct {
	DefaultLimit int `yaml:"default_limit"`
	MaxLimit     int `yaml:"max_limit"`
}

// SchemaLimits contains query limits.
type SchemaLimits struct {
	MaxDepth      int  `yaml:"max_depth"`
	MaxComplexity int  `yaml:"max_complexity"`
	Introspection bool `yaml:"introspection"`
}

// MCPConfig contains MCP server settings.
type MCPConfig struct {
	Enabled   bool              `yaml:"enabled"`
	Tools     MCPToolsConfig    `yaml:"tools"`
	Resources MCPResourceConfig `yaml:"resources"`
	Prompts   MCPPromptConfig   `yaml:"prompts"`
	Sampling  MCPSamplingConfig `yaml:"sampling"`
}

// MCPToolsConfig contains MCP tools settings.
type MCPToolsConfig struct {
	AutoDiscover bool   `yaml:"auto_discover"`
	Prefix       string `yaml:"prefix"`
}

// MCPResourceConfig contains MCP resources settings.
type MCPResourceConfig struct {
	AutoExpose bool `yaml:"auto_expose"`
}

// MCPPromptConfig contains MCP prompts settings.
type MCPPromptConfig struct {
	Enabled bool `yaml:"enabled"`
}

// MCPSamplingConfig contains MCP sampling settings.
type MCPSamplingConfig struct {
	Enabled bool `yaml:"enabled"`
}

// ObservabilityConfig contains observability settings.
type ObservabilityConfig struct {
	Logging   LoggingConfig   `yaml:"logging"`
	Metrics   MetricsConfig   `yaml:"metrics"`
	Health    HealthConfig    `yaml:"health"`
	RequestID RequestIDConfig `yaml:"request_id"`
}

// LoggingConfig contains logging settings.
type LoggingConfig struct {
	Level  string `yaml:"level"`  // "debug", "info", "warn", "error"
	Format string `yaml:"format"` // "json", "text"
	Output string `yaml:"output"` // "stdout", "stderr", or file path
}

// MetricsConfig contains metrics settings.
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// HealthConfig contains health check settings.
type HealthConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// RequestIDConfig contains request ID settings.
type RequestIDConfig struct {
	Header    string `yaml:"header"`
	Propagate bool   `yaml:"propagate"`
}

// AdminConfig contains admin interface settings.
type AdminConfig struct {
	Enabled bool       `yaml:"enabled"`
	Path    string     `yaml:"path"`
	Auth    *AdminAuth `yaml:"auth,omitempty"`
}

// AdminAuth contains admin authentication settings.
type AdminAuth struct {
	Type     string `yaml:"type"` // "basic", "bearer"
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Token    string `yaml:"token"`
}
