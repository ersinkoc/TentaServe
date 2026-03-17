package config

import (
	"time"
)

// DefaultConfig returns a Config with all default values set.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:           "0.0.0.0",
			Port:           8080,
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
			IdleTimeout:    120 * time.Second,
			MaxHeaderBytes: 1 << 20, // 1MB
		},
		Gateway: GatewayConfig{
			RESTPrefix:  "/api",
			GraphQLPath: "/graphql",
			MCPPath:     "/mcp",
			RateLimit: RateLimitConfig{
				Enabled:           true,
				RequestsPerSecond: 100,
				BurstSize:         150,
			},
			CircuitBreaker: CircuitBreakerConfig{
				Enabled:          true,
				FailureThreshold: 5,
				ResetTimeout:     30 * time.Second,
				HalfOpenRequests: 3,
			},
			Cache: CacheConfig{
				Enabled:      true,
				MaxSize:      256 << 20, // 256MB
				TTL:          5 * time.Minute,
				MaxEntrySize: 1 << 20, // 1MB
			},
		},
		Upstreams: []UpstreamConfig{},
		Schema: SchemaConfig{
			REST2GraphQL: REST2GraphQLConfig{
				Enabled: true,
			},
			GraphQL2REST: GraphQL2RESTConfig{
				Enabled:        true,
				EndpointPrefix: "/rest",
			},
			Pagination: PaginationConfig{
				DefaultLimit: 20,
				MaxLimit:     100,
			},
			Limits: SchemaLimits{
				MaxDepth:      10,
				MaxComplexity: 1000,
				Introspection: true,
			},
		},
		MCP: MCPConfig{
			Enabled: true,
			Tools: MCPToolsConfig{
				AutoDiscover: true,
				Prefix:       "",
			},
			Resources: MCPResourceConfig{
				AutoExpose: true,
			},
			Prompts: MCPPromptConfig{
				Enabled: true,
			},
			Sampling: MCPSamplingConfig{
				Enabled: true,
			},
		},
		Observability: ObservabilityConfig{
			Logging: LoggingConfig{
				Level:  "info",
				Format: "json",
				Output: "stdout",
			},
			Metrics: MetricsConfig{
				Enabled: true,
				Path:    "/-/metrics",
			},
			Health: HealthConfig{
				Enabled: true,
				Path:    "/-/health",
			},
			RequestID: RequestIDConfig{
				Header:    "X-Request-ID",
				Propagate: true,
			},
		},
		Admin: AdminConfig{
			Enabled: false,
			Path:    "/-/admin",
		},
	}
}
