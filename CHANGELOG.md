# Changelog

All notable changes to Tentaserve will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [0.1.0] - 2026-03-20

### Added

- **Bi-directional Protocol Translation**
  - REST to GraphQL: auto-generates GraphQL schema from OpenAPI 3.0/3.1 specs
  - GraphQL to REST: introspects GraphQL schemas and generates REST endpoints
  - DataLoader batching to prevent N+1 queries
  - Query planner for multi-upstream execution
  - Pagination translation (offset, cursor, Relay connection styles)
  - Custom field mappings with naming convention support

- **API Gateway**
  - JWT authentication (HS256/384/512), API Key, and passthrough auth
  - Token bucket rate limiting with per-IP/per-header/per-path scoping
  - Sharded LRU response cache with TTL and stale-while-revalidate
  - Circuit breaker (3-state machine) per upstream
  - CORS middleware with preflight handling
  - Request ID propagation
  - Config hot reload with SHA256 change detection

- **MCP Server (Model Context Protocol)**
  - JSON-RPC 2.0 transport at configurable path
  - Auto-discovers and exposes all API endpoints as MCP tools
  - Tool naming with collision avoidance (snake_case, max 64 chars)
  - JSON Schema input validation
  - Server-Sent Events streaming
  - Resource listing for upstream metadata
  - Per-tool metrics tracking

- **GraphQL Subscription to SSE Bridge**
  - REST clients can consume GraphQL subscriptions via Server-Sent Events
  - Automatic endpoint generation from subscription types
  - Heartbeat and connection cleanup

- **Admin Dashboard**
  - Web UI at `/-/admin` showing upstream health, metrics, and MCP tools
  - JSON API for dashboard data
  - Optional basic authentication

- **Observability**
  - Structured logging (JSON/text format) via slog
  - Prometheus-format metrics endpoint
  - Per-upstream health checks with degraded/unhealthy states
  - Dynamic llms.txt generation for AI agents

- **CLI**
  - `serve` - Start the gateway
  - `validate` - Validate configuration
  - `schema` - Print merged GraphQL SDL or OpenAPI JSON
  - `tools` - List generated MCP tools
  - `jwt` - JWT generation, validation, decoding utilities
  - `version` - Print build info

- **Infrastructure**
  - Zero external dependencies (Go stdlib only)
  - Single static binary, cross-platform
  - Multi-stage Docker build (< 20MB image)
  - GitHub Actions CI/CD (lint, test, build, release)
  - Custom zero-dependency YAML parser with env var interpolation
