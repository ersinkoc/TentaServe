package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("server: %w", err)
	}

	if err := c.Gateway.Validate(); err != nil {
		return fmt.Errorf("gateway: %w", err)
	}

	if err := c.Schema.Validate(); err != nil {
		return fmt.Errorf("schema: %w", err)
	}

	if err := c.MCP.Validate(); err != nil {
		return fmt.Errorf("mcp: %w", err)
	}

	if err := c.Observability.Validate(); err != nil {
		return fmt.Errorf("observability: %w", err)
	}

	if err := c.Admin.Validate(); err != nil {
		return fmt.Errorf("admin: %w", err)
	}

	// Validate upstreams
	upstreamNames := make(map[string]bool)
	for i, u := range c.Upstreams {
		if err := u.Validate(); err != nil {
			return fmt.Errorf("upstreams[%d]: %w", i, err)
		}
		if upstreamNames[u.Name] {
			return fmt.Errorf("upstreams[%d]: duplicate upstream name %q", i, u.Name)
		}
		upstreamNames[u.Name] = true
	}

	return nil
}

// Validate checks the server configuration.
func (s *ServerConfig) Validate() error {
	if s.Port < 1 || s.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", s.Port)
	}

	if s.ReadTimeout <= 0 {
		return fmt.Errorf("read_timeout must be positive")
	}

	if s.WriteTimeout <= 0 {
		return fmt.Errorf("write_timeout must be positive")
	}

	if s.IdleTimeout <= 0 {
		return fmt.Errorf("idle_timeout must be positive")
	}

	if s.MaxHeaderBytes < 1024 {
		return fmt.Errorf("max_header_bytes must be at least 1024")
	}

	if s.TLS != nil {
		if err := s.TLS.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Validate checks the TLS configuration.
func (t *TLSConfig) Validate() error {
	if t.CertFile == "" {
		return fmt.Errorf("cert_file is required when TLS is enabled")
	}

	if t.KeyFile == "" {
		return fmt.Errorf("key_file is required when TLS is enabled")
	}

	return nil
}

// Validate checks the gateway configuration.
func (g *GatewayConfig) Validate() error {
	if g.RESTPrefix == "" {
		return fmt.Errorf("rest_prefix is required")
	}

	if g.GraphQLPath == "" {
		return fmt.Errorf("graphql_path is required")
	}

	if g.MCPPath == "" {
		return fmt.Errorf("mcp_path is required")
	}

	// Paths should not conflict
	if g.RESTPrefix == g.GraphQLPath || g.RESTPrefix == g.MCPPath {
		return fmt.Errorf("rest_prefix must be different from graphql_path and mcp_path")
	}

	if err := g.RateLimit.Validate(); err != nil {
		return fmt.Errorf("rate_limit: %w", err)
	}

	if err := g.CircuitBreaker.Validate(); err != nil {
		return fmt.Errorf("circuit_breaker: %w", err)
	}

	if err := g.Cache.Validate(); err != nil {
		return fmt.Errorf("cache: %w", err)
	}

	return nil
}

// Validate checks the rate limit configuration.
func (r *RateLimitConfig) Validate() error {
	if !r.Enabled {
		return nil
	}

	if r.RequestsPerSecond < 1 {
		return fmt.Errorf("requests_per_second must be at least 1")
	}

	if r.BurstSize < 1 {
		return fmt.Errorf("burst_size must be at least 1")
	}

	return nil
}

// Validate checks the circuit breaker configuration.
func (c *CircuitBreakerConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	if c.FailureThreshold < 1 {
		return fmt.Errorf("failure_threshold must be at least 1")
	}

	if c.ResetTimeout <= 0 {
		return fmt.Errorf("reset_timeout must be positive")
	}

	if c.HalfOpenRequests < 1 {
		return fmt.Errorf("half_open_requests must be at least 1")
	}

	return nil
}

// Validate checks the cache configuration.
func (c *CacheConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	if c.MaxSize < 1024 {
		return fmt.Errorf("max_size must be at least 1024 bytes")
	}

	if c.TTL <= 0 {
		return fmt.Errorf("ttl must be positive")
	}

	if c.MaxEntrySize < 1 {
		return fmt.Errorf("max_entry_size must be at least 1")
	}

	return nil
}

// Validate checks the upstream configuration.
func (u *UpstreamConfig) Validate() error {
	if u.Name == "" {
		return fmt.Errorf("name is required")
	}

	if !isValidName(u.Name) {
		return fmt.Errorf("name must contain only alphanumeric characters, hyphens, and underscores")
	}

	if u.Type != "rest" && u.Type != "graphql" {
		return fmt.Errorf("type must be 'rest' or 'graphql', got %q", u.Type)
	}

	// Validate URL based on type
	if u.Type == "rest" {
		if u.BaseURL == "" {
			return fmt.Errorf("base_url is required for REST upstream")
		}
		if _, err := url.Parse(u.BaseURL); err != nil {
			return fmt.Errorf("invalid base_url: %w", err)
		}
		if u.OpenAPI == nil || u.OpenAPI.Source == "" {
			return fmt.Errorf("openapi.source is required for REST upstream")
		}
	}

	if u.Type == "graphql" {
		if u.Endpoint == "" {
			return fmt.Errorf("endpoint is required for GraphQL upstream")
		}
		if _, err := url.Parse(u.Endpoint); err != nil {
			return fmt.Errorf("invalid endpoint: %w", err)
		}
	}

	if u.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	if err := u.Retry.Validate(); err != nil {
		return fmt.Errorf("retry: %w", err)
	}

	if err := u.Auth.Validate(); err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	return nil
}

// Validate checks the retry configuration.
func (r *RetryConfig) Validate() error {
	if r.MaxAttempts < 0 {
		return fmt.Errorf("max_attempts must be non-negative")
	}

	if r.MaxAttempts > 0 {
		// Default to exponential if not specified
		if r.Backoff == "" {
			r.Backoff = "exponential"
		}

		validBackoffs := map[string]bool{
			"exponential": true,
			"linear":      true,
			"fixed":       true,
		}
		if !validBackoffs[r.Backoff] {
			return fmt.Errorf("backoff must be 'exponential', 'linear', or 'fixed', got %q", r.Backoff)
		}

		// Default base_delay if not set
		if r.BaseDelay == 0 {
			r.BaseDelay = 100 * time.Millisecond
		}

		// Default max_delay if not set
		if r.MaxDelay == 0 {
			r.MaxDelay = 30 * time.Second
		}

		if r.BaseDelay < 0 {
			return fmt.Errorf("base_delay must be non-negative")
		}

		if r.MaxDelay < 0 {
			return fmt.Errorf("max_delay must be non-negative")
		}
	}

	return nil
}

// Validate checks the auth configuration.
func (a *AuthConfig) Validate() error {
	// Default to "none" if not specified
	if a.Strategy == "" {
		a.Strategy = "none"
	}

	validStrategies := map[string]bool{
		"none":    true,
		"forward": true,
		"bearer":  true,
		"basic":   true,
	}

	if !validStrategies[a.Strategy] {
		return fmt.Errorf("strategy must be 'none', 'forward', 'bearer', or 'basic', got %q", a.Strategy)
	}

	if a.Strategy == "forward" && a.Header == "" {
		return fmt.Errorf("header is required when strategy is 'forward'")
	}

	if a.Strategy == "bearer" && a.Token == "" {
		return fmt.Errorf("token is required when strategy is 'bearer'")
	}

	if a.Strategy == "basic" && (a.Username == "" || a.Password == "") {
		return fmt.Errorf("username and password are required when strategy is 'basic'")
	}

	return nil
}

// Validate checks the schema configuration.
func (s *SchemaConfig) Validate() error {
	if err := s.Pagination.Validate(); err != nil {
		return fmt.Errorf("pagination: %w", err)
	}

	if err := s.Limits.Validate(); err != nil {
		return fmt.Errorf("limits: %w", err)
	}

	return nil
}

// Validate checks the pagination configuration.
func (p *PaginationConfig) Validate() error {
	if p.DefaultLimit < 1 {
		return fmt.Errorf("default_limit must be at least 1")
	}

	if p.MaxLimit < p.DefaultLimit {
		return fmt.Errorf("max_limit must be at least default_limit")
	}

	return nil
}

// Validate checks the schema limits configuration.
func (s *SchemaLimits) Validate() error {
	if s.MaxDepth < 1 {
		return fmt.Errorf("max_depth must be at least 1")
	}

	if s.MaxComplexity < 1 {
		return fmt.Errorf("max_complexity must be at least 1")
	}

	return nil
}

// Validate checks the MCP configuration.
func (m *MCPConfig) Validate() error {
	if !m.Enabled {
		return nil
	}

	// MCP validation is minimal since it auto-discovers from upstreams
	return nil
}

// Validate checks the observability configuration.
func (o *ObservabilityConfig) Validate() error {
	if err := o.Logging.Validate(); err != nil {
		return fmt.Errorf("logging: %w", err)
	}

	if err := o.Metrics.Validate(); err != nil {
		return fmt.Errorf("metrics: %w", err)
	}

	if err := o.Health.Validate(); err != nil {
		return fmt.Errorf("health: %w", err)
	}

	if err := o.RequestID.Validate(); err != nil {
		return fmt.Errorf("request_id: %w", err)
	}

	return nil
}

// Validate checks the logging configuration.
func (l *LoggingConfig) Validate() error {
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}

	if !validLevels[l.Level] {
		return fmt.Errorf("level must be 'debug', 'info', 'warn', or 'error', got %q", l.Level)
	}

	validFormats := map[string]bool{
		"json": true,
		"text": true,
	}

	if !validFormats[l.Format] {
		return fmt.Errorf("format must be 'json' or 'text', got %q", l.Format)
	}

	return nil
}

// Validate checks the metrics configuration.
func (m *MetricsConfig) Validate() error {
	if m.Enabled && m.Path == "" {
		return fmt.Errorf("path is required when metrics is enabled")
	}

	return nil
}

// Validate checks the health configuration.
func (h *HealthConfig) Validate() error {
	if h.Enabled && h.Path == "" {
		return fmt.Errorf("path is required when health is enabled")
	}

	return nil
}

// Validate checks the request ID configuration.
func (r *RequestIDConfig) Validate() error {
	if r.Header == "" {
		return fmt.Errorf("header is required")
	}

	return nil
}

// Validate checks the admin configuration.
func (a *AdminConfig) Validate() error {
	if !a.Enabled {
		return nil
	}

	if a.Path == "" {
		return fmt.Errorf("path is required when admin is enabled")
	}

	if a.Auth != nil {
		if err := a.Auth.Validate(); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}

	return nil
}

// Validate checks the admin auth configuration.
func (a *AdminAuth) Validate() error {
	validTypes := map[string]bool{
		"basic":   true,
		"bearer":  true,
	}

	if !validTypes[a.Type] {
		return fmt.Errorf("type must be 'basic' or 'bearer', got %q", a.Type)
	}

	if a.Type == "basic" && (a.Username == "" || a.Password == "") {
		return fmt.Errorf("username and password are required for basic auth")
	}

	if a.Type == "bearer" && a.Token == "" {
		return fmt.Errorf("token is required for bearer auth")
	}

	return nil
}

// isValidName checks if a name contains only valid characters.
func isValidName(name string) bool {
	if name == "" {
		return false
	}

	for _, r := range name {
		if !isValidNameChar(r) {
			return false
		}
	}

	return true
}

// isValidNameChar checks if a rune is valid for a name.
func isValidNameChar(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == '-' || r == '_'
}

// Merge combines the source config into this config.
// Values in the source take precedence over values in this config.
func (c *Config) Merge(source *Config) {
	// Server
	if source.Server.Host != "" {
		c.Server.Host = source.Server.Host
	}
	if source.Server.Port != 0 {
		c.Server.Port = source.Server.Port
	}
	if source.Server.ReadTimeout != 0 {
		c.Server.ReadTimeout = source.Server.ReadTimeout
	}
	if source.Server.WriteTimeout != 0 {
		c.Server.WriteTimeout = source.Server.WriteTimeout
	}
	if source.Server.IdleTimeout != 0 {
		c.Server.IdleTimeout = source.Server.IdleTimeout
	}
	if source.Server.MaxHeaderBytes != 0 {
		c.Server.MaxHeaderBytes = source.Server.MaxHeaderBytes
	}
	if source.Server.TLS != nil {
		c.Server.TLS = source.Server.TLS
	}

	// Gateway
	if source.Gateway.RESTPrefix != "" {
		c.Gateway.RESTPrefix = source.Gateway.RESTPrefix
	}
	if source.Gateway.GraphQLPath != "" {
		c.Gateway.GraphQLPath = source.Gateway.GraphQLPath
	}
	if source.Gateway.MCPPath != "" {
		c.Gateway.MCPPath = source.Gateway.MCPPath
	}

	// Upstreams - append, don't replace
	c.Upstreams = append(c.Upstreams, source.Upstreams...)

	// Schema
	if source.Schema.REST2GraphQL.Enabled != c.Schema.REST2GraphQL.Enabled {
		c.Schema.REST2GraphQL.Enabled = source.Schema.REST2GraphQL.Enabled
	}
	if source.Schema.GraphQL2REST.Enabled != c.Schema.GraphQL2REST.Enabled {
		c.Schema.GraphQL2REST.Enabled = source.Schema.GraphQL2REST.Enabled
	}

	// MCP
	if source.MCP.Enabled != c.MCP.Enabled {
		c.MCP.Enabled = source.MCP.Enabled
	}

	// Observability
	if source.Observability.Logging.Level != "" {
		c.Observability.Logging.Level = source.Observability.Logging.Level
	}
	if source.Observability.Logging.Format != "" {
		c.Observability.Logging.Format = source.Observability.Logging.Format
	}
	if source.Observability.Logging.Output != "" {
		c.Observability.Logging.Output = source.Observability.Logging.Output
	}

	// Admin
	if source.Admin.Enabled {
		c.Admin.Enabled = true
		if source.Admin.Path != "" {
			c.Admin.Path = source.Admin.Path
		}
		if source.Admin.Auth != nil {
			c.Admin.Auth = source.Admin.Auth
		}
	}
}

// String returns a string representation of the config (for logging).
func (c *Config) String() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("server=%s:%d", c.Server.Host, c.Server.Port))
	parts = append(parts, fmt.Sprintf("upstreams=%d", len(c.Upstreams)))
	parts = append(parts, fmt.Sprintf("rate_limit=%v", c.Gateway.RateLimit.Enabled))
	parts = append(parts, fmt.Sprintf("cache=%v", c.Gateway.Cache.Enabled))
	parts = append(parts, fmt.Sprintf("mcp=%v", c.MCP.Enabled))
	return strings.Join(parts, ", ")
}
