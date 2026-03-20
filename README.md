# Tentaserve

> **Every protocol. One binary.**

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.22-blue?logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Zero Dependencies](https://img.shields.io/badge/dependencies-zero-purple)]()

Tentaserve is a self-hosted API gateway that translates between REST and GraphQL — in both directions. It also exposes every endpoint as an [MCP (Model Context Protocol)](https://modelcontextprotocol.io) tool, so AI agents like Claude can use your APIs without any manual configuration.

**Zero dependencies. Single Go binary. Works anywhere.**

---

## Features

- **Bi-directional Translation** — REST APIs become GraphQL. GraphQL APIs become REST. No code changes needed.
- **MCP Server Built-in** — Every endpoint is automatically a tool for AI agents.
- **Zero Dependencies** — Only uses Go standard library. No Docker required.
- **API Gateway** — Rate limiting, caching, circuit breaking, auth forwarding.
- **Single Binary** — One file to deploy. Cross-compiles to any platform.

---

## Quick Start

### Installation

```bash
# Download latest release
curl -L https://github.com/ersinkoc/tentaserve/releases/latest/download/tentaserve-linux-amd64 -o tentaserve
chmod +x tentaserve

# Or build from source
git clone https://github.com/ersinkoc/tentaserve.git
cd tentaserve
make build
```

### Configuration

Create `tentaserve.yaml`:

```yaml
server:
  port: 8080

upstreams:
  - name: "api"
    type: "rest"
    base_url: "https://api.example.com"
    openapi:
      source: "https://api.example.com/openapi.json"

gateway:
  rest_prefix: "/api"
  graphql_path: "/graphql"
  mcp_path: "/mcp"
```

### Run

```bash
./tentaserve serve --config tentaserve.yaml
```

Now you can:
- Query your REST API via GraphQL at `POST /graphql`
- Access your API via MCP at `POST /mcp`

---

## Example: REST → GraphQL

Your REST API has:
```
GET    /users/{id}
GET    /users/{id}/posts
POST   /users/{id}/posts
```

Tentaserve automatically generates:

```graphql
type Query {
  user(id: ID!): User
}

type User {
  id: ID!
  posts: [Post!]!
}

type Mutation {
  createUserPost(userId: ID!, input: PostInput!): Post
}
```

Query it:
```graphql
query {
  user(id: "123") {
    posts {
      title
      createdAt
    }
  }
}
```

---

## Example: GraphQL → REST

Your GraphQL API has:
```graphql
type Query {
  searchProducts(filter: ProductFilter!): [Product!]!
}
```

Tentaserve automatically exposes:
```
POST /rest/searchProducts
Content-Type: application/json

{"filter": {"category": "electronics", "maxPrice": 500}}
```

---

## Example: MCP for AI Agents

AI agents can discover and use your APIs automatically:

```json
// MCP tools/list response
{
  "tools": [
    {
      "name": "get_user_by_id",
      "description": "Get a user by their ID",
      "inputSchema": {
        "type": "object",
        "properties": {
          "id": {"type": "string"}
        },
        "required": ["id"]
      }
    },
    {
      "name": "create_user_post",
      "description": "Create a post for a user",
      "inputSchema": { ... }
    }
  ]
}
```

---

## Architecture

```
                  Clients
        GraphQL │ REST │ MCP/AI
            │        │       │
            v        v       v
   ┌─────────────────────────────────┐
   │       HTTP/HTTPS Server         │
   │         Request Router          │
   └────────┬────────┬────────┬──────┘
            │        │        │
            v        v        v
   ┌─────────────────────────────────┐
   │      Gateway Middleware         │
   │  RequestID > Auth > RateLimit   │
   │  > Cache > CircuitBreaker       │
   └────────┬────────┬────────┬──────┘
            │        │        │
   ┌────────┘   ┌────┘   ┌───┘
   v            v         v
┌──────┐  ┌──────┐  ┌─────────┐
│GraphQL│  │ REST │  │   MCP   │
│Handler│  │Handler│ │Handler  │
└───┬───┘  └───┬──┘  └────┬───┘
    │          │           │
    v          v           v
┌─────────────────────────────────┐
│     Schema Engine / Proxy       │
│  REST<>GraphQL Translation      │
│  DataLoader / Query Planner     │
└────────┬───────────┬────────────┘
         │           │
         v           v
   ┌──────────┐ ┌────────────┐
   │ REST APIs│ │GraphQL APIs│
   └──────────┘ └────────────┘
```

---

## CLI Commands

```bash
tentaserve serve      # Start the gateway
tentaserve validate   # Validate configuration file
tentaserve schema     # Print merged GraphQL SDL
tentaserve tools      # List generated MCP tools
tentaserve jwt        # JWT generation/validation utilities
tentaserve version    # Print version info
```

Flags:
```bash
tentaserve serve --config custom.yaml --port 9090 --log-level debug
tentaserve schema --upstream users-api --format json
tentaserve tools --upstream users-api --format json
```

---

## GraphQL Subscriptions via SSE

Tentaserve bridges GraphQL subscriptions to Server-Sent Events:

```graphql
subscription { orderUpdated(orderId: "456") { status } }
```

Becomes:
```bash
curl -N http://localhost:8080/api/stream/order-updated?orderId=456 \
  -H "Accept: text/event-stream"

# Stream output:
# event: message
# data: {"status":"shipped"}
#
# event: message
# data: {"status":"delivered"}
```

---

## Admin Dashboard

Enable the admin dashboard at `/-/admin`:

```yaml
admin:
  enabled: true
  path: "/-/admin"
  auth:
    type: "basic"
    username: "admin"
    password: "${ADMIN_PASSWORD}"
```

The dashboard shows: upstream health, request metrics, cache hit rate, and active MCP tools. Auto-refreshes every 5 seconds.

---

## Configuration Reference

See [tentaserve.example.yaml](tentaserve.example.yaml) for a full annotated example.

| Section | Key Options |
|---------|-------------|
| `server` | `host`, `port`, `read_timeout`, `write_timeout`, TLS config |
| `gateway` | `rest_prefix`, `graphql_path`, `mcp_path` |
| `gateway.rate_limit` | `enabled`, `requests_per_second`, `burst_size` |
| `gateway.cache` | `enabled`, `max_size`, `ttl`, `max_entry_size` |
| `gateway.circuit_breaker` | `enabled`, `failure_threshold`, `reset_timeout` |
| `upstreams[]` | `name`, `type`, `base_url`/`endpoint`, `auth`, `timeout`, `retry` |
| `schema.limits` | `max_depth`, `max_complexity`, `introspection` |
| `mcp` | `enabled`, `tools.auto_discover`, `tools.prefix` |
| `observability` | `logging.level`, `metrics.enabled`, `health.enabled` |

Environment variable interpolation: `${VAR}` or `${VAR:default}`.

---

## Building

```bash
# Build for current platform
make build

# Run tests
make test

# Run tests with race detector
make test -race

# Build for all platforms (Linux, macOS, Windows x AMD64/ARM64)
make build-all

# Build Docker image (< 20MB)
make docker

# Generate test coverage
make test-coverage
```

---

## Deployment

### Binary

```bash
# Download and run
curl -L https://github.com/ersinkoc/tentaserve/releases/latest/download/tentaserve-linux-amd64 -o tentaserve
chmod +x tentaserve
./tentaserve serve --config tentaserve.yaml
```

### Docker

```bash
docker run -p 8080:8080 -v $(pwd)/tentaserve.yaml:/etc/tentaserve/config.yaml \
  ghcr.io/ersinkoc/tentaserve:latest serve --config /etc/tentaserve/config.yaml
```

### Go Install

```bash
go install github.com/ersinkoc/tentaserve/cmd/tentaserve@latest
```

---

## Why Tentaserve?

| | Tentaserve | Kong | Apollo Gateway | Hasura |
|---|:---:|:---:|:---:|:---:|
| Bi-directional translation | ✅ | ❌ | ❌ | ❌ |
| MCP support | ✅ | ❌ | ❌ | ❌ |
| Zero dependencies | ✅ | ❌ | ❌ | ❌ |
| Single binary | ✅ | ❌ | ❌ | ❌ |
| Self-hosted | ✅ | ✅ | ✅ | ✅ |

---

## License

MIT License — see [LICENSE](LICENSE) for details.

---

<p align="center">
  <sub>Built with 🐙 by <a href="https://x.com/ersinkoc">Ersin KOÇ</a></sub>
</p>
