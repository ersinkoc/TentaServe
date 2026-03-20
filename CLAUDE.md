# CLAUDE.md

## Project Overview

Tentaserve is a self-hosted, zero-dependency API gateway written in Go 1.22+ that provides bi-directional REST/GraphQL translation and MCP (Model Context Protocol) server for AI agents.

## Build & Test

```bash
make build          # Build binary
make test           # Run all tests with race detector
go test -cover ./...  # Run with coverage
go test -bench=. ./test/bench/  # Run benchmarks
go vet ./...        # Static analysis
gofmt -l .          # Check formatting
```

## Architecture

- `cmd/tentaserve/` - CLI entry point (serve, validate, schema, tools, jwt, version)
- `internal/config/` - YAML config parser (custom zero-dep), env interpolation, validation
- `internal/graphql/` - GraphQL lexer, parser, executor, printer, validator, introspection
- `internal/openapi/` - OpenAPI 3.0/3.1 parser, $ref resolver, loader
- `internal/proxy/` - Protocol translation (rest2gql, gql2rest, graphql proxy, router)
- `internal/schema/` - Unified type system, DataLoader, query planner, field mapping
- `internal/gateway/` - Middleware (auth, ratelimit, cache, breaker, cors, health, metrics)
- `internal/mcp/` - MCP JSON-RPC server, tool registry, SSE transport
- `internal/admin/` - Admin dashboard at /-/admin
- `internal/llmstxt/` - Dynamic llms.txt generation
- `internal/server/` - HTTP server with TLS and graceful shutdown
- `internal/upstream/` - Connection-pooled HTTP client with retry

## Key Constraints

- **Zero external dependencies** - only Go stdlib
- **Single binary** - cross-compiles to all platforms
- All tests must pass with `go test -race`
- `go vet` and `gofmt` must be clean
