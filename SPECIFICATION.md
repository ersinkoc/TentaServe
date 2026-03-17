# Tentaserve — Specification

> **Bi-directional GraphQL↔REST API Gateway with MCP Server Integration**
> Zero-dependency · Single binary · Written in Go

**Version:** 0.1.0-draft
**Author:** Ersin Koç / ECOSTACK TECHNOLOGY OÜ
**Repository:** github.com/ersinkoc/tentaserve
**License:** MIT

---

## Table of Contents

1. [Overview](#1-overview)
2. [Goals & Non-Goals](#2-goals--non-goals)
3. [Architecture](#3-architecture)
4. [Core Concepts](#4-core-concepts)
5. [API Gateway Layer](#5-api-gateway-layer)
6. [REST → GraphQL Proxy](#6-rest--graphql-proxy)
7. [GraphQL → REST Proxy](#7-graphql--rest-proxy)
8. [Schema Engine](#8-schema-engine)
9. [MCP Server Layer](#9-mcp-server-layer)
10. [Configuration](#10-configuration)
11. [Observability](#11-observability)
12. [CLI Interface](#12-cli-interface)
13. [Plugin System](#13-plugin-system)
14. [Security](#14-security)
15. [Performance Requirements](#15-performance-requirements)
16. [Deployment](#16-deployment)
17. [Project Structure](#17-project-structure)
18. [Milestones](#18-milestones)

---

## 1. Overview

### 1.1 What is Tentaserve?

Tentaserve is a self-hosted, zero-external-dependency API gateway written in Go that sits between clients and backend services, providing:

- **Bi-directional protocol translation**: Automatically converts REST APIs into GraphQL schemas and GraphQL APIs into RESTful endpoints.
- **API Gateway capabilities**: Authentication forwarding, rate limiting, response caching, circuit breaking, health checking, and request routing.
- **MCP (Model Context Protocol) server**: Exposes all proxied endpoints as MCP tools, enabling AI agents (Claude Code, etc.) to discover and interact with any API behind Tentaserve without manual tool definitions.

Tentaserve compiles to a single binary with zero runtime dependencies. It reads a YAML configuration file that defines upstream services and their protocols, then automatically builds the translation layer, gateway policies, and MCP tool definitions at startup.

### 1.2 The Problem

Modern architectures involve a mix of REST and GraphQL services. Teams face recurring challenges:

- **Protocol mismatch**: Frontend wants GraphQL, backend offers REST (or vice versa). Custom BFF (Backend-for-Frontend) layers are expensive to build and maintain.
- **Gateway fragmentation**: Separate tools for API gateway (Kong, Envoy), GraphQL gateway (Apollo), and protocol translation (custom code). Each has its own config, deployment, and learning curve.
- **AI agent integration gap**: LLM-based agents (Claude Code, Copilot, custom agents) need MCP tool definitions to interact with APIs. Writing and maintaining these definitions manually doesn't scale — especially when the underlying APIs change frequently.

### 1.3 The Solution

Tentaserve collapses these three concerns into a single binary:

```
┌─────────────────────────────────────────────┐
│  GraphQL Client  │  REST Client  │  AI Agent │
└────────┬─────────┴───────┬───────┴─────┬─────┘
         │                 │             │
         ▼                 ▼             ▼
┌─────────────────────────────────────────────┐
│              TENTASERVE                     │
│                                             │
│  ┌─────────────────────────────────────┐    │
│  │         API Gateway Layer           │    │
│  │  Auth · Rate Limit · Cache · CB     │    │
│  └──────────────┬──────────────────────┘    │
│                 │                            │
│  ┌──────────────┴──────────────────────┐    │
│  │      Bi-directional Proxy           │    │
│  │  REST→GQL        GQL→REST           │    │
│  └──────────────┬──────────────────────┘    │
│                 │                            │
│  ┌──────────────┴──────────────────────┐    │
│  │       Schema Engine + DataLoader    │    │
│  │  Type Mapping · N+1 Opt · Pagination│    │
│  └──────────────┬──────────────────────┘    │
│                 │                            │
│  ┌──────────────┴──────────────────────┐    │
│  │          MCP Server                 │    │
│  │  Auto Tool Gen · JSON Schema        │    │
│  └─────────────────────────────────────┘    │
└────────┬─────────────────┬──────────────────┘
         ▼                 ▼
   ┌───────────┐    ┌───────────────┐
   │ REST APIs │    │ GraphQL APIs  │
   └───────────┘    └───────────────┘
```

### 1.4 Key Differentiators

- **Zero external dependencies**: No cgo, no vendor libraries. Pure Go standard library. Compiles anywhere Go compiles.
- **Single binary deployment**: Download, configure, run. No Docker required (but supports it).
- **MCP-native**: First API gateway with built-in MCP server. AI agents discover and use APIs automatically.
- **Bi-directional by default**: Not just REST→GraphQL. Also GraphQL→REST. Same engine, both directions.
- **Plugin architecture**: Core is minimal. Auth strategies, cache backends, custom transformers — all plugins. Ship your own or use built-in ones.

---

## 2. Goals & Non-Goals

### 2.1 Goals

| Priority | Goal |
|----------|------|
| P0 | REST→GraphQL proxy via OpenAPI 3.x spec import |
| P0 | GraphQL→REST proxy via schema introspection |
| P0 | API gateway (auth forwarding, rate limiting, caching, circuit breaker) |
| P0 | MCP server with auto-generated tool definitions |
| P0 | Zero external dependencies — pure Go stdlib |
| P0 | Single binary, cross-platform (Linux, macOS, Windows) |
| P1 | Hot-reload configuration without downtime |
| P1 | Prometheus-compatible metrics endpoint |
| P1 | Health check endpoint with upstream status |
| P1 | YAML-based configuration with env var interpolation |
| P1 | DataLoader-style batching for N+1 query optimization |
| P2 | GraphQL subscriptions → SSE/WebSocket bridge |
| P2 | Plugin system for custom auth, cache, and transform logic |
| P2 | Web-based admin dashboard |
| P2 | OpenTelemetry trace export |
| P3 | Multi-instance clustering with leader election |
| P3 | Schema stitching across multiple GraphQL upstreams |

### 2.2 Non-Goals

- **Full GraphQL server**: Tentaserve proxies and translates — it does not execute custom resolvers or manage a schema registry.
- **Service mesh**: No sidecar pattern, no mutual TLS mesh. Use Tentaserve as a gateway, not as infrastructure fabric.
- **Database proxy**: Tentaserve talks HTTP/HTTPS to upstreams, not database wire protocols.
- **API design tool**: Tentaserve consumes OpenAPI specs and GraphQL schemas — it does not help you design them.
- **Replacement for full-featured API management platforms**: No developer portal, no API monetization, no usage analytics dashboard. Tentaserve is an infrastructure component, not a SaaS product.

---

## 3. Architecture

### 3.1 Layered Architecture

Tentaserve follows a strict layered architecture where each layer has a single responsibility and communicates only with its immediate neighbors:

```
Layer 0: Transport (HTTP/HTTPS listener, WebSocket upgrade)
Layer 1: Gateway (middleware chain: auth → rate limit → cache → circuit breaker)
Layer 2: Router (protocol detection, request classification)
Layer 3: Proxy (REST→GQL translator, GQL→REST translator)
Layer 4: Schema Engine (type mapping, query planning, DataLoader)
Layer 5: MCP (tool registry, tool execution, JSON-RPC transport)
Layer 6: Upstream (HTTP client pool, health checker, retry logic)
```

### 3.2 Request Lifecycle

A request entering Tentaserve traverses these stages:

```
1. ACCEPT     — Transport accepts TCP connection
2. TLS        — Optional TLS termination
3. PARSE      — HTTP request parsed (method, path, headers, body)
4. CLASSIFY   — Router determines: is this GraphQL, REST, or MCP?
                 • POST /graphql                → GraphQL proxy path
                 • GET/POST/PUT/DELETE /api/*    → REST proxy path
                 • POST /mcp (JSON-RPC)         → MCP server path
                 • GET /-/health                → Health endpoint
                 • GET /-/metrics               → Metrics endpoint
5. GATEWAY    — Middleware chain executes in order:
                 a. Authentication (verify/forward token)
                 b. Rate limiting (token bucket per client)
                 c. Cache lookup (return cached response if hit)
                 d. Circuit breaker check (fail-fast if open)
6. TRANSLATE  — Protocol translation:
                 • REST→GQL: map REST request to GraphQL query
                 • GQL→REST: decompose GraphQL query into REST calls
                 • MCP: resolve tool call to upstream request
7. OPTIMIZE   — Schema engine batches and deduplicates upstream calls
                 (DataLoader pattern: collect → batch → dispatch)
8. DISPATCH   — Send request(s) to upstream service(s)
9. AGGREGATE  — Collect upstream responses, merge if multiple
10. TRANSFORM — Transform upstream response to client's expected format
11. CACHE     — Store response in cache (if cacheable)
12. RESPOND   — Send response to client
```

### 3.3 Concurrency Model

- Each incoming connection is handled by a goroutine.
- Upstream HTTP calls use a shared `*http.Client` with connection pooling per upstream host.
- DataLoader batching uses a per-request batch window (default: 5ms) to collect N+1 queries into a single batch.
- Rate limiter uses atomic operations for lock-free token bucket updates.
- Configuration hot-reload uses copy-on-write: new config is built in the background, then atomically swapped via `atomic.Pointer`.

### 3.4 Memory Model

- Request-scoped allocations use sync.Pool where possible (buffers, JSON encoders).
- Schema definitions are immutable after build — shared across goroutines without locks.
- Cache entries use a concurrent-safe LRU with sharded locks (16 shards by default).
- No global mutable state outside of explicitly synchronized structures.

---

## 4. Core Concepts

### 4.1 Upstream

An upstream is a backend service that Tentaserve proxies. Each upstream has:

- **Name**: unique identifier (e.g., `users-api`, `products-graphql`)
- **Type**: `rest` or `graphql`
- **Base URL**: e.g., `http://users-service:8080`
- **Schema source**: For REST — path to OpenAPI spec (file or URL). For GraphQL — introspection endpoint or SDL file.
- **Health check**: Optional endpoint and interval.
- **Policies**: Per-upstream rate limits, timeouts, retry counts, circuit breaker thresholds.

### 4.2 Route

A route maps an incoming request pattern to an upstream and translation strategy:

- **Path pattern**: e.g., `/api/users/*` → upstream `users-api`
- **Protocol**: What the client sends (REST or GraphQL) vs what the upstream expects.
- **Transform rules**: Custom field mappings, header forwarding, response shaping.

### 4.3 Schema Mapping

When Tentaserve translates between protocols, it builds an internal schema mapping:

- **REST→GQL**: Each OpenAPI path+method becomes a GraphQL query (GET) or mutation (POST/PUT/DELETE). Request/response schemas become GraphQL types.
- **GQL→REST**: Each GraphQL query/mutation field becomes a REST endpoint. Arguments become query params (queries) or JSON body fields (mutations). Return types become JSON response shapes.

### 4.4 Tool (MCP)

A tool is the MCP representation of an API endpoint. Tentaserve auto-generates tools from the unified schema:

- **Name**: derived from operation ID or path (e.g., `get_user_by_id`, `create_order`)
- **Description**: derived from OpenAPI description or GraphQL field description
- **Input schema**: JSON Schema derived from request parameters/arguments
- **Handler**: internally routes to the appropriate upstream call

---

## 5. API Gateway Layer

### 5.1 Authentication Forwarding

Tentaserve does not authenticate users itself. It forwards authentication headers to upstreams and optionally validates tokens before forwarding.

**Built-in strategies:**

| Strategy | Behavior |
|----------|----------|
| `passthrough` | Forward all auth headers unchanged (default) |
| `bearer-validate` | Validate JWT signature locally (HMAC/RSA), forward if valid |
| `api-key` | Check `X-API-Key` header against a static list, forward to upstream |
| `oauth2-introspect` | Call an OAuth2 introspection endpoint to validate the token |

**Configuration example:**

```yaml
gateway:
  auth:
    strategy: bearer-validate
    jwt:
      secret: ${JWT_SECRET}
      algorithms: [HS256, RS256]
      claims:
        issuer: "https://auth.example.com"
    forward_header: Authorization
```

**Plugin interface** (for custom auth):

```go
type AuthPlugin interface {
    Name() string
    Authenticate(ctx context.Context, req *http.Request) (AuthResult, error)
}

type AuthResult struct {
    Authenticated bool
    Identity      map[string]any  // claims, user info — forwarded as headers
    Error         string
}
```

### 5.2 Rate Limiting

Token bucket algorithm, lock-free implementation using `sync/atomic`.

**Scoping options:**

| Scope | Key derivation |
|-------|----------------|
| `global` | Single shared bucket for all clients |
| `per-ip` | Source IP address |
| `per-header` | Value of a specified header (e.g., `X-API-Key`, `Authorization`) |
| `per-path` | Request path pattern |
| `composite` | Combination of multiple keys |

**Configuration:**

```yaml
gateway:
  rate_limit:
    enabled: true
    scope: per-ip
    requests: 1000
    window: 60s
    burst: 50
    response:
      status: 429
      headers:
        Retry-After: "{{.RetryAfter}}"
```

**Per-upstream override:**

```yaml
upstreams:
  - name: users-api
    rate_limit:
      requests: 100
      window: 60s
```

### 5.3 Response Caching

In-memory LRU cache with sharded locks. Optional TTL per cache entry.

**Cache key derivation:**

```
SHA256(method + path + sorted_query_params + normalized_body_hash + vary_headers)
```

**Cache control:**

- Respects upstream `Cache-Control` headers by default.
- Configurable override per upstream (force TTL, ignore upstream headers).
- GraphQL responses: cache by normalized query + variables hash.
- Cache invalidation via `PURGE` method on any cached path.
- Stale-while-revalidate support: serve stale entry while refreshing in background.

**Configuration:**

```yaml
gateway:
  cache:
    enabled: true
    max_entries: 10000
    max_memory: 256MB
    default_ttl: 60s
    stale_while_revalidate: 30s
    vary_headers: [Authorization, Accept-Language]
    exclude_paths: ["/api/auth/*", "/graphql"]  # mutations excluded by default
```

### 5.4 Circuit Breaker

Per-upstream circuit breaker using the standard three-state model:

```
CLOSED → (failure threshold exceeded) → OPEN
OPEN → (timeout elapsed) → HALF-OPEN
HALF-OPEN → (probe succeeds) → CLOSED
HALF-OPEN → (probe fails) → OPEN
```

**Configuration:**

```yaml
gateway:
  circuit_breaker:
    enabled: true
    failure_threshold: 5        # consecutive failures to trip
    success_threshold: 3        # successes in half-open to close
    timeout: 30s                # time in open state before half-open
    sampling_window: 60s        # window for failure counting
    failure_codes: [500, 502, 503, 504]
```

### 5.5 Middleware Chain

Gateway middleware executes in a fixed order. Each middleware can short-circuit the chain:

```
Request → Auth → RateLimit → CacheLookup → CircuitBreaker → [Proxy] → CacheStore → Response
```

Custom middleware can be injected via the plugin system at defined hook points:

- `pre-auth`: Before authentication (e.g., request logging, IP filtering)
- `post-auth`: After authentication (e.g., tenant resolution, feature flags)
- `pre-proxy`: Before upstream dispatch (e.g., request transformation)
- `post-proxy`: After upstream response (e.g., response transformation, enrichment)

---

## 6. REST → GraphQL Proxy

### 6.1 OpenAPI Import

Tentaserve reads an OpenAPI 3.0/3.1 specification and generates a GraphQL schema from it.

**Supported spec sources:**

- Local file path (JSON or YAML)
- Remote URL (fetched at startup and optionally on interval)
- Inline in configuration (for simple APIs)

### 6.2 Mapping Rules

#### Paths → Queries/Mutations

| HTTP Method | GraphQL Operation | Naming Convention |
|-------------|-------------------|-------------------|
| `GET` | Query | `{operationId}` or `get_{resource}` |
| `POST` | Mutation | `create_{resource}` |
| `PUT` | Mutation | `update_{resource}` |
| `PATCH` | Mutation | `patch_{resource}` |
| `DELETE` | Mutation | `delete_{resource}` |

**Example:**

```
GET /api/users/{id}         →  query { user(id: ID!): User }
GET /api/users              →  query { users(page: Int, limit: Int): UserList }
POST /api/users             →  mutation { createUser(input: CreateUserInput!): User }
PUT /api/users/{id}         →  mutation { updateUser(id: ID!, input: UpdateUserInput!): User }
DELETE /api/users/{id}      →  mutation { deleteUser(id: ID!): DeleteResult }
```

#### Types → GraphQL Types

| OpenAPI Type | GraphQL Type |
|--------------|--------------|
| `string` | `String` |
| `string` (format: `date-time`) | `DateTime` (custom scalar) |
| `string` (format: `uuid`) | `ID` |
| `string` (format: `email`) | `String` (with validation directive) |
| `integer` | `Int` |
| `integer` (format: `int64`) | `Int64` (custom scalar) |
| `number` | `Float` |
| `boolean` | `Boolean` |
| `array` | `[T]` |
| `object` | Named type (from schema name or generated) |
| `oneOf` | Union type |
| `allOf` | Merged type (field union) |
| `anyOf` | Union type |
| `enum` | Enum type |

#### Parameters → Arguments

| OpenAPI Parameter Location | GraphQL Mapping |
|----------------------------|-----------------|
| `path` | Required argument |
| `query` | Optional argument (required if `required: true`) |
| `header` | Forwarded as-is (not exposed as argument) |
| `cookie` | Forwarded as-is (not exposed as argument) |

#### Relationships (Nested Resources)

Tentaserve detects parent-child relationships from URL patterns:

```
GET /api/users/{userId}/posts      →  User type gains `posts` field
GET /api/posts/{postId}/comments   →  Post type gains `comments` field
```

This enables nested GraphQL queries:

```graphql
query {
  user(id: "123") {
    name
    posts {
      title
      comments {
        body
      }
    }
  }
}
```

Each nesting level triggers a separate upstream REST call, batched via DataLoader.

### 6.3 Pagination Translation

| REST Pagination Style | GraphQL Mapping |
|----------------------|-----------------|
| Offset (`?page=2&limit=10`) | `users(page: 2, limit: 10)` |
| Cursor (`?after=xyz&first=10`) | Relay-style connection type |
| Link header (`Link: <...>; rel="next"`) | Connection type with `pageInfo` |

**Auto-detection:** Tentaserve inspects the OpenAPI spec for pagination parameters and response shapes. If `page`/`limit` or `offset`/`count` params exist, offset pagination is assumed. If `cursor`/`after`/`before` params exist, cursor pagination is used.

**Relay connection wrapper:**

```graphql
type UserConnection {
  edges: [UserEdge]
  pageInfo: PageInfo!
  totalCount: Int
}

type UserEdge {
  node: User
  cursor: String!
}

type PageInfo {
  hasNextPage: Boolean!
  hasPreviousPage: Boolean!
  startCursor: String
  endCursor: String
}
```

### 6.4 Error Mapping

| REST Status Code | GraphQL Error |
|------------------|---------------|
| 400 | `BAD_USER_INPUT` extension |
| 401 | `UNAUTHENTICATED` extension |
| 403 | `FORBIDDEN` extension |
| 404 | `null` field + error in `errors` array |
| 409 | `CONFLICT` extension |
| 422 | `VALIDATION_ERROR` extension + field-level details |
| 429 | `RATE_LIMITED` extension |
| 500+ | `INTERNAL_SERVER_ERROR` extension |

Upstream error response bodies are included in the GraphQL error's `extensions.upstream` field for debugging (configurable — can be disabled in production).

---

## 7. GraphQL → REST Proxy

### 7.1 Schema Introspection

Tentaserve connects to the upstream GraphQL endpoint and runs an introspection query to obtain the full schema. The schema is then used to generate REST endpoints.

**Schema sources:**

- Introspection query (default) — fetched at startup and refreshable
- SDL file (local path or URL)
- Inline SDL in configuration

### 7.2 Mapping Rules

#### Queries → GET Endpoints

```
query { user(id: ID!): User }       →  GET /api/user?id={id}
query { users(limit: Int): [User] }  →  GET /api/users?limit={limit}
query { searchUsers(q: String!): [User] }  →  GET /api/search-users?q={q}
```

**Naming convention:** GraphQL field name is kebab-cased for the URL path. Arguments become query parameters.

#### Mutations → POST/PUT/DELETE Endpoints

```
mutation { createUser(input: CreateUserInput!): User }     →  POST /api/create-user
mutation { updateUser(id: ID!, input: UpdateUserInput!): User }  →  PUT /api/update-user/{id}
mutation { deleteUser(id: ID!): Boolean }                  →  DELETE /api/delete-user/{id}
```

**Heuristic for method selection:**

| Mutation name prefix | HTTP Method |
|---------------------|-------------|
| `create`, `add`, `insert` | POST |
| `update`, `edit`, `modify`, `patch` | PUT |
| `delete`, `remove`, `destroy` | DELETE |
| (other) | POST (default) |

#### Nested Fields → Query Parameters

Clients can control which fields are returned via a `fields` query parameter:

```
GET /api/user?id=123&fields=name,email,posts.title
```

This translates to:

```graphql
query {
  user(id: "123") {
    name
    email
    posts {
      title
    }
  }
}
```

### 7.3 Response Shaping

GraphQL responses are unwrapped before sending as REST:

```json
// GraphQL response:
{ "data": { "user": { "name": "Alice", "email": "alice@example.com" } } }

// REST response (unwrapped):
{ "name": "Alice", "email": "alice@example.com" }
```

GraphQL errors are mapped to appropriate HTTP status codes:

| GraphQL Error Extension | HTTP Status |
|------------------------|-------------|
| `UNAUTHENTICATED` | 401 |
| `FORBIDDEN` | 403 |
| `BAD_USER_INPUT` | 400 |
| `NOT_FOUND` | 404 |
| (no extension / unknown) | 500 |

### 7.4 Subscription Bridge

GraphQL subscriptions are bridged to Server-Sent Events (SSE):

```
subscription { orderUpdated(orderId: "456") }
  →  GET /api/stream/order-updated?orderId=456
      Accept: text/event-stream
```

Each subscription event is emitted as an SSE `data:` frame. The connection is held open until the client disconnects or the upstream subscription ends.

---

## 8. Schema Engine

### 8.1 Unified Internal Representation

Both REST→GQL and GQL→REST translations pass through a unified internal type system:

```go
type TypeDef struct {
    Name        string
    Kind        TypeKind       // Scalar, Object, Enum, Union, List, NonNull, Input
    Fields      []FieldDef
    EnumValues  []string
    UnionTypes  []string
    Description string
}

type FieldDef struct {
    Name        string
    Type        TypeRef
    Arguments   []ArgumentDef
    Description string
    Deprecated  bool
    DeprecationReason string
    Source      FieldSource    // REST path, GraphQL field, computed
}

type TypeRef struct {
    Name    string
    Kind    TypeKind
    OfType  *TypeRef   // for List and NonNull wrappers
}
```

### 8.2 Type Mapping Engine

The type mapping engine maintains a bidirectional map between:

- OpenAPI schema definitions ↔ Internal types ↔ GraphQL types
- Field names can be remapped via configuration (e.g., `user_name` → `userName`)

**Default naming conventions:**

| Source | Convention | Example |
|--------|-----------|---------|
| OpenAPI → GraphQL | PascalCase types, camelCase fields | `UserResponse` → `UserResponse`, `first_name` → `firstName` |
| GraphQL → REST | snake_case JSON fields, kebab-case URLs | `firstName` → `first_name`, `getUser` → `get-user` |

**Custom field mapping:**

```yaml
upstreams:
  - name: legacy-api
    type: rest
    field_mappings:
      "usr_nm": "userName"
      "usr_email": "email"
      "crt_dt": "createdAt"
```

### 8.3 DataLoader (N+1 Optimization)

When a GraphQL query resolves nested fields that require separate REST calls, Tentaserve uses a DataLoader pattern to batch them:

**Example — without DataLoader:**

```
query { users { posts { title } } }
→ GET /api/users                      (1 call)
→ GET /api/users/1/posts              (N calls, one per user)
→ GET /api/users/2/posts
→ GET /api/users/3/posts
→ ...
```

**With DataLoader:**

```
→ GET /api/users                      (1 call)
→ GET /api/posts?user_ids=1,2,3,...   (1 batched call, if batch endpoint exists)
```

**Batch endpoint discovery:**

1. Check if the upstream OpenAPI spec defines a batch endpoint (e.g., `GET /api/posts?user_ids={ids}`)
2. If not, fall back to parallel individual calls with configurable concurrency limit
3. Configurable batch window (default: 5ms) to collect keys before dispatching

**DataLoader configuration:**

```yaml
schema:
  dataloader:
    enabled: true
    batch_window: 5ms
    max_batch_size: 100
    concurrency: 10
```

### 8.4 Query Planning

For complex GraphQL queries that span multiple upstreams or require multiple REST calls, Tentaserve builds a query plan:

```
query {
  user(id: "1") {        # → GET users-api/users/1
    name
    orders {              # → GET orders-api/orders?user_id=1
      total
      items {             # → GET orders-api/orders/{id}/items (batched)
        product {         # → GET products-api/products/{id} (batched)
          name
          price
        }
      }
    }
  }
}
```

**Query plan:**

```
Step 1: Fetch user from users-api (1 call)
Step 2: Fetch orders from orders-api using user.id (1 call)
Step 3: Fetch items from orders-api using order.ids (1 batched call)
Step 4: Fetch products from products-api using item.product_ids (1 batched call)
```

Steps are executed in dependency order. Steps at the same depth level with no data dependencies are executed in parallel.

---

## 9. MCP Server Layer

### 9.1 Overview

The MCP server is Tentaserve's most distinctive feature. It implements the Model Context Protocol (MCP) specification, exposing all proxied API endpoints as tools that AI agents can discover and invoke.

**Transport:** JSON-RPC 2.0 over HTTP (SSE for streaming) at `POST /mcp`

### 9.2 Auto Tool Generation

At startup (and on schema refresh), Tentaserve walks the unified schema and generates an MCP tool for each operation:

**From REST upstream:**

```yaml
# OpenAPI:
# GET /api/users/{id}
#   summary: Get user by ID
#   parameters:
#     - name: id, in: path, required: true, schema: { type: string }

# Generated MCP tool:
name: "get_user_by_id"
description: "Get user by ID from users-api"
inputSchema:
  type: object
  properties:
    id:
      type: string
      description: "User ID"
  required: ["id"]
```

**From GraphQL upstream:**

```yaml
# GraphQL:
# type Query {
#   """Search products by keyword"""
#   searchProducts(query: String!, limit: Int = 10): [Product!]!
# }

# Generated MCP tool:
name: "search_products"
description: "Search products by keyword from products-graphql"
inputSchema:
  type: object
  properties:
    query:
      type: string
      description: "Search keyword"
    limit:
      type: integer
      description: "Max results"
      default: 10
  required: ["query"]
```

### 9.3 Tool Naming

Tool names are derived from operation identifiers with collision avoidance:

```
Priority 1: operationId (from OpenAPI) or field name (from GraphQL)
Priority 2: {method}_{path_segments} (e.g., get_users_by_id)
Priority 3: {upstream_name}_{method}_{path_segments} (if collision)
```

**Naming rules:**
- snake_case, lowercase
- Max 64 characters
- Alphanumeric + underscore only
- Prefix with upstream name if ambiguous

### 9.4 Tool Categories

Tools are organized into categories (exposed as MCP resource types):

| Category | Derived From | Example |
|----------|--------------|---------|
| `query` | GET requests, GraphQL queries | `get_user`, `list_orders` |
| `mutation` | POST/PUT/DELETE, GraphQL mutations | `create_user`, `delete_order` |
| `subscription` | GraphQL subscriptions | `watch_order_status` |

### 9.5 MCP Protocol Support

**Implemented methods:**

| Method | Description |
|--------|-------------|
| `initialize` | Handshake, capability negotiation |
| `tools/list` | List all available tools |
| `tools/call` | Execute a tool |
| `resources/list` | List upstream service metadata |
| `resources/read` | Read upstream schema/docs |
| `prompts/list` | (Optional) List suggested prompts |

### 9.6 Tool Execution Flow

```
1. Agent sends tools/call with tool name and arguments
2. Tentaserve resolves tool → upstream + operation
3. Gateway middleware is applied (auth, rate limit, etc.)
4. Request is built and dispatched to upstream
5. Response is transformed to JSON
6. Result is returned to agent as MCP tool result
```

### 9.7 Schema-Driven Descriptions

Tentaserve generates rich tool descriptions from the source schemas:

- OpenAPI `summary` and `description` fields → tool description
- Parameter `description` fields → input property descriptions
- Response schema → tool output description
- GraphQL field descriptions and deprecation notices → tool metadata
- Examples from OpenAPI `example` fields → tool input examples

This enables AI agents to understand what each tool does without additional documentation.

### 9.8 MCP Configuration

```yaml
mcp:
  enabled: true
  path: /mcp
  server_info:
    name: "tentaserve"
    version: "0.1.0"
  tool_prefix: ""              # optional prefix for all tool names
  exclude_patterns:             # exclude tools matching patterns
    - "internal_*"
    - "admin_*"
  include_descriptions: true
  include_examples: true
  max_concurrent_calls: 10
```

---

## 10. Configuration

### 10.1 Configuration File

Tentaserve uses a single YAML configuration file. Default path: `tentaserve.yaml` in the working directory.

### 10.2 Full Configuration Schema

```yaml
# tentaserve.yaml

server:
  host: "0.0.0.0"
  port: 8080
  tls:
    enabled: false
    cert_file: ""
    key_file: ""
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 120s
  max_header_bytes: 1048576     # 1MB
  shutdown_timeout: 15s

gateway:
  auth:
    strategy: passthrough       # passthrough | bearer-validate | api-key | oauth2-introspect
    jwt:
      secret: ${JWT_SECRET}
      algorithms: [HS256]
      claims: {}
    api_keys: []
    forward_header: Authorization
  
  rate_limit:
    enabled: false
    scope: per-ip
    requests: 1000
    window: 60s
    burst: 50
  
  cache:
    enabled: false
    max_entries: 10000
    max_memory: "256MB"
    default_ttl: 60s
    stale_while_revalidate: 30s
    vary_headers: []
    exclude_paths: []
  
  circuit_breaker:
    enabled: true
    failure_threshold: 5
    success_threshold: 3
    timeout: 30s
    sampling_window: 60s
    failure_codes: [500, 502, 503, 504]
  
  cors:
    enabled: true
    allow_origins: ["*"]
    allow_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
    allow_headers: ["*"]
    expose_headers: []
    max_age: 86400
    allow_credentials: false

upstreams:
  - name: "users-api"
    type: rest                  # rest | graphql
    url: "http://localhost:3001"
    schema:
      source: "file://./specs/users-openapi.yaml"  # file:// | http:// | inline
      refresh_interval: 0s      # 0 = no refresh
    health:
      path: "/health"
      interval: 30s
      timeout: 5s
    timeout: 10s
    retries: 2
    retry_delay: 100ms
    rate_limit:                 # per-upstream override
      requests: 500
      window: 60s
    field_mappings: {}
    headers:                    # additional headers to forward
      X-Service-Name: "tentaserve"
  
  - name: "products-graphql"
    type: graphql
    url: "http://localhost:3002/graphql"
    schema:
      source: introspection     # introspection | file:// | inline
      refresh_interval: 300s
    health:
      path: "/health"
      interval: 30s
      timeout: 5s
    timeout: 15s

schema:
  naming:
    rest_to_graphql: camelCase  # camelCase | snake_case | preserve
    graphql_to_rest: snake_case
  dataloader:
    enabled: true
    batch_window: 5ms
    max_batch_size: 100
    concurrency: 10
  pagination:
    default_style: offset       # offset | cursor
    default_limit: 20
    max_limit: 100

mcp:
  enabled: true
  path: /mcp
  server_info:
    name: "tentaserve"
    version: "0.1.0"
  tool_prefix: ""
  exclude_patterns: []
  include_descriptions: true
  include_examples: true
  max_concurrent_calls: 10

observability:
  metrics:
    enabled: true
    path: /-/metrics
  health:
    enabled: true
    path: /-/health
  logging:
    level: info                 # debug | info | warn | error
    format: json                # json | text
    output: stderr              # stderr | stdout | file://path
  tracing:
    enabled: false
    exporter: stdout            # stdout | otlp
    endpoint: ""
    sample_rate: 0.1

admin:
  enabled: false
  path: /-/admin
  auth:
    username: ${ADMIN_USER}
    password: ${ADMIN_PASS}
```

### 10.3 Environment Variable Interpolation

Any value in the config can reference environment variables:

```yaml
server:
  port: ${PORT:8080}            # default value after colon
gateway:
  auth:
    jwt:
      secret: ${JWT_SECRET}     # required, no default — fails if unset
```

### 10.4 Hot Reload

Tentaserve watches the configuration file for changes (via polling, not fsnotify — zero deps).

On change:
1. Parse and validate new config
2. Build new schema mappings
3. Atomic swap of config pointer
4. Log the change with diff summary
5. Emit metric `tentaserve_config_reloads_total`

Connections in flight continue with the old config. New connections use the new config.

Hot reload does **not** change:
- `server.host`, `server.port`, `server.tls` (requires restart)

Hot reload **does** change:
- Gateway policies (auth, rate limit, cache, circuit breaker)
- Upstream definitions (add, remove, modify)
- Schema mappings
- MCP configuration

---

## 11. Observability

### 11.1 Metrics

Prometheus-compatible metrics at `/-/metrics`:

**Request metrics:**

| Metric | Type | Labels |
|--------|------|--------|
| `tentaserve_requests_total` | Counter | `method`, `path`, `status`, `upstream` |
| `tentaserve_request_duration_seconds` | Histogram | `method`, `path`, `upstream` |
| `tentaserve_request_size_bytes` | Histogram | `method`, `path` |
| `tentaserve_response_size_bytes` | Histogram | `method`, `path` |

**Gateway metrics:**

| Metric | Type | Labels |
|--------|------|--------|
| `tentaserve_cache_hits_total` | Counter | `upstream` |
| `tentaserve_cache_misses_total` | Counter | `upstream` |
| `tentaserve_rate_limit_hits_total` | Counter | `scope` |
| `tentaserve_circuit_breaker_state` | Gauge | `upstream`, `state` |
| `tentaserve_auth_failures_total` | Counter | `strategy`, `reason` |

**Upstream metrics:**

| Metric | Type | Labels |
|--------|------|--------|
| `tentaserve_upstream_requests_total` | Counter | `upstream`, `status` |
| `tentaserve_upstream_duration_seconds` | Histogram | `upstream` |
| `tentaserve_upstream_health` | Gauge | `upstream` (1=healthy, 0=unhealthy) |
| `tentaserve_upstream_connections_active` | Gauge | `upstream` |

**MCP metrics:**

| Metric | Type | Labels |
|--------|------|--------|
| `tentaserve_mcp_tool_calls_total` | Counter | `tool`, `status` |
| `tentaserve_mcp_tool_duration_seconds` | Histogram | `tool` |
| `tentaserve_mcp_sessions_active` | Gauge | — |

**System metrics:**

| Metric | Type |
|--------|------|
| `tentaserve_uptime_seconds` | Gauge |
| `tentaserve_goroutines` | Gauge |
| `tentaserve_memory_alloc_bytes` | Gauge |
| `tentaserve_config_reloads_total` | Counter |

### 11.2 Health Check

`GET /-/health` returns:

```json
{
  "status": "healthy",
  "version": "0.1.0",
  "uptime": "2h34m12s",
  "upstreams": {
    "users-api": {
      "status": "healthy",
      "latency_ms": 12,
      "last_check": "2025-03-16T10:00:00Z"
    },
    "products-graphql": {
      "status": "degraded",
      "latency_ms": 450,
      "last_check": "2025-03-16T10:00:00Z",
      "error": "high latency"
    }
  }
}
```

**Status codes:**
- `200` — all upstreams healthy
- `200` with `"status": "degraded"` — some upstreams unhealthy but service is operational
- `503` — all upstreams down or critical failure

### 11.3 Structured Logging

JSON-formatted structured logs:

```json
{
  "time": "2025-03-16T10:00:00.123Z",
  "level": "info",
  "msg": "request completed",
  "method": "POST",
  "path": "/graphql",
  "status": 200,
  "duration_ms": 45,
  "upstream": "users-api",
  "cache": "miss",
  "client_ip": "192.168.1.100",
  "request_id": "req_abc123"
}
```

**Log levels:**
- `debug`: Full request/response bodies (development only)
- `info`: Request summaries, config changes, upstream status changes
- `warn`: Rate limit hits, circuit breaker state changes, retry attempts
- `error`: Upstream failures, parse errors, internal errors

---

## 12. CLI Interface

### 12.1 Commands

```
tentaserve                          # Start server (default)
tentaserve serve                    # Start server (explicit)
tentaserve serve --config path.yaml # Custom config path
tentaserve serve --port 9090        # Override port

tentaserve validate                 # Validate config file
tentaserve validate --config path.yaml

tentaserve schema                   # Show generated unified schema
tentaserve schema --upstream users-api  # Show schema for specific upstream
tentaserve schema --format graphql  # Output as GraphQL SDL
tentaserve schema --format openapi  # Output as OpenAPI 3.1

tentaserve tools                    # List generated MCP tools
tentaserve tools --upstream users-api
tentaserve tools --format json      # JSON output for programmatic use

tentaserve health                   # Check health of running instance
tentaserve health --url http://localhost:8080

tentaserve version                  # Print version and build info
```

### 12.2 Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--config`, `-c` | Config file path | `tentaserve.yaml` |
| `--port`, `-p` | Override server port | (from config) |
| `--host` | Override server host | (from config) |
| `--log-level` | Override log level | (from config) |
| `--log-format` | Override log format | (from config) |

---

## 13. Plugin System

### 13.1 Plugin Interface

Tentaserve supports compile-time plugins via Go interfaces. Plugins are registered in `main.go` or via a plugin registry file.

**Core plugin interfaces:**

```go
// Authentication plugin
type AuthPlugin interface {
    Name() string
    Authenticate(ctx context.Context, req *http.Request) (AuthResult, error)
}

// Cache backend plugin
type CachePlugin interface {
    Name() string
    Get(ctx context.Context, key string) ([]byte, bool, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Clear(ctx context.Context) error
}

// Request/Response transformer plugin
type TransformPlugin interface {
    Name() string
    TransformRequest(ctx context.Context, req *ProxyRequest) error
    TransformResponse(ctx context.Context, resp *ProxyResponse) error
}

// Middleware plugin
type MiddlewarePlugin interface {
    Name() string
    HookPoint() HookPoint   // pre-auth, post-auth, pre-proxy, post-proxy
    Execute(ctx context.Context, req *http.Request, next func() error) error
}
```

### 13.2 Built-in Plugins

| Plugin | Type | Description |
|--------|------|-------------|
| `passthrough-auth` | Auth | Forward headers unchanged |
| `jwt-auth` | Auth | Validate JWT tokens locally |
| `api-key-auth` | Auth | Validate API keys against a list |
| `memory-cache` | Cache | In-memory LRU cache |
| `request-logger` | Middleware | Log all requests |
| `cors` | Middleware | CORS header handling |

### 13.3 Future: Runtime Plugins

Post-v1.0, Tentaserve will support runtime plugins via:
- Go plugin system (`plugin.Open`) on Linux
- WebAssembly (Wasm) plugins for cross-platform support

This is explicitly a post-v1.0 feature and not part of the initial implementation.

---

## 14. Security

### 14.1 Transport Security

- TLS 1.2+ for HTTPS termination (Go's `crypto/tls` — no external deps)
- Optional mutual TLS for upstream connections
- HTTP/2 support (automatic with TLS)

### 14.2 Input Validation

- Maximum request body size enforcement (configurable, default: 1MB)
- GraphQL query depth limiting (default: 10 levels)
- GraphQL query complexity analysis (default: 1000 points)
- Request header size limiting
- Path traversal prevention in all file-reading operations

### 14.3 Rate Limiting & Abuse Prevention

- Per-client rate limiting (see section 5.2)
- GraphQL batching limit (max N queries per batch, default: 10)
- Slowloris protection via read/write timeouts
- Maximum concurrent connections per client IP (configurable)

### 14.4 Secrets Management

- Environment variable interpolation for all sensitive config values
- No secrets logged (even at debug level)
- JWT secrets and API keys stored only in memory, never written to disk by Tentaserve
- Config file permissions warning if world-readable (0644 or wider)

### 14.5 Upstream Isolation

- Each upstream has its own HTTP client, connection pool, and circuit breaker
- A failing upstream cannot affect other upstreams
- Timeout enforcement at multiple levels: per-upstream, per-request, gateway-level

---

## 15. Performance Requirements

### 15.1 Targets

| Metric | Target |
|--------|--------|
| Startup time (cold) | < 2 seconds |
| Startup time (with schema fetch) | < 5 seconds |
| P50 added latency (passthrough) | < 2ms |
| P99 added latency (passthrough) | < 10ms |
| P50 added latency (REST→GQL translation) | < 5ms |
| P99 added latency (REST→GQL translation) | < 20ms |
| Throughput (simple proxy) | > 50,000 req/s (single core) |
| Memory usage (idle, 10 upstreams) | < 50MB |
| Memory usage (active, 10k concurrent) | < 200MB |
| Binary size (Linux amd64) | < 15MB |
| Config hot-reload time | < 100ms |

### 15.2 Benchmarking

Every release must include benchmark results for:

- Passthrough proxy (no translation)
- REST→GraphQL translation (simple query)
- REST→GraphQL translation (nested query with DataLoader)
- GraphQL→REST translation
- MCP tool call (end-to-end)
- Cache hit vs miss comparison
- Rate limiter overhead

Benchmarks run against a mock upstream (zero network latency) to isolate Tentaserve's overhead.

---

## 16. Deployment

### 16.1 Single Binary

```bash
# Download
curl -fsSL https://github.com/ersinkoc/tentaserve/releases/latest/download/tentaserve-linux-amd64.tar.gz | tar xz

# Configure
cp tentaserve.example.yaml tentaserve.yaml
vim tentaserve.yaml

# Run
./tentaserve serve
```

### 16.2 Docker

```dockerfile
FROM scratch
COPY tentaserve /tentaserve
COPY tentaserve.yaml /etc/tentaserve/tentaserve.yaml
EXPOSE 8080
ENTRYPOINT ["/tentaserve", "serve", "--config", "/etc/tentaserve/tentaserve.yaml"]
```

Image size: ~15MB (scratch base + static binary).

### 16.3 Systemd

```ini
[Unit]
Description=Tentaserve API Gateway
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/tentaserve serve --config /etc/tentaserve/tentaserve.yaml
ExecReload=/bin/kill -HUP $MAINPID
Restart=always
RestartSec=5
User=tentaserve
Group=tentaserve

[Install]
WantedBy=multi-user.target
```

### 16.4 Supported Platforms

| OS | Architecture | Status |
|----|-------------|--------|
| Linux | amd64, arm64 | Primary |
| macOS | amd64 (Intel), arm64 (Apple Silicon) | Primary |
| Windows | amd64 | Secondary |
| FreeBSD | amd64 | Best-effort |

---

## 17. Project Structure

```
tentaserve/
├── cmd/
│   └── tentaserve/
│       └── main.go                 # Entry point, CLI parsing
├── internal/
│   ├── config/
│   │   ├── config.go               # Config struct and defaults
│   │   ├── loader.go               # YAML parsing, env interpolation
│   │   ├── validator.go            # Config validation
│   │   └── watcher.go              # Hot reload file watcher
│   ├── gateway/
│   │   ├── gateway.go              # Gateway orchestrator
│   │   ├── auth/
│   │   │   ├── auth.go             # Auth plugin interface
│   │   │   ├── passthrough.go      # Passthrough strategy
│   │   │   ├── jwt.go              # JWT validation
│   │   │   └── apikey.go           # API key validation
│   │   ├── ratelimit/
│   │   │   ├── limiter.go          # Token bucket implementation
│   │   │   └── store.go            # Rate limit state storage
│   │   ├── cache/
│   │   │   ├── cache.go            # Cache interface
│   │   │   ├── lru.go              # Sharded LRU implementation
│   │   │   └── key.go              # Cache key derivation
│   │   ├── circuitbreaker/
│   │   │   └── breaker.go          # Circuit breaker state machine
│   │   └── middleware/
│   │       ├── chain.go            # Middleware chain executor
│   │       ├── cors.go             # CORS middleware
│   │       └── requestid.go        # Request ID generator
│   ├── proxy/
│   │   ├── proxy.go                # Proxy orchestrator
│   │   ├── rest2gql/
│   │   │   ├── translator.go       # REST→GraphQL request translation
│   │   │   ├── schema_builder.go   # OpenAPI→GraphQL schema builder
│   │   │   ├── resolver.go         # GraphQL field resolver (dispatches REST calls)
│   │   │   └── pagination.go       # Pagination style translation
│   │   ├── gql2rest/
│   │   │   ├── translator.go       # GraphQL→REST request translation
│   │   │   ├── endpoint_builder.go # GraphQL schema→REST endpoint builder
│   │   │   ├── response.go         # Response unwrapping and shaping
│   │   │   └── subscription.go     # Subscription→SSE bridge
│   │   └── router/
│   │       └── router.go           # Request classification and routing
│   ├── schema/
│   │   ├── types.go                # Unified internal type system
│   │   ├── mapping.go              # Bidirectional type mapping
│   │   ├── dataloader.go           # DataLoader batching
│   │   ├── planner.go              # Query plan builder
│   │   └── naming.go               # Naming convention transformers
│   ├── mcp/
│   │   ├── server.go               # MCP JSON-RPC server
│   │   ├── tools.go                # Tool registry and auto-generation
│   │   ├── handler.go              # Tool call handler
│   │   └── transport.go            # SSE transport layer
│   ├── upstream/
│   │   ├── client.go               # HTTP client pool manager
│   │   ├── health.go               # Health check runner
│   │   └── retry.go                # Retry logic with backoff
│   ├── openapi/
│   │   ├── parser.go               # OpenAPI 3.x parser (zero-dep)
│   │   ├── types.go                # OpenAPI type definitions
│   │   └── loader.go               # File/URL/inline loader
│   ├── graphql/
│   │   ├── parser.go               # GraphQL query parser (zero-dep)
│   │   ├── lexer.go                # GraphQL lexer
│   │   ├── ast.go                  # GraphQL AST types
│   │   ├── printer.go              # AST→query string printer
│   │   ├── introspection.go        # Introspection query runner
│   │   ├── executor.go             # Query executor
│   │   ├── validator.go            # Query validation (depth, complexity)
│   │   └── sdl.go                  # SDL parser
│   ├── observability/
│   │   ├── metrics.go              # Prometheus-format metrics
│   │   ├── health.go               # Health endpoint handler
│   │   └── logger.go               # Structured logger
│   └── plugin/
│       ├── registry.go             # Plugin registration
│       └── interfaces.go           # Plugin interface definitions
├── llms.txt                        # LLM-friendly project description
├── tentaserve.example.yaml         # Example configuration
├── SPECIFICATION.md                # This document
├── IMPLEMENTATION.md               # Implementation guide
├── TASKS.md                        # Task breakdown for Claude Code
├── BRANDING.md                     # Brand guidelines
├── Makefile                        # Build, test, lint targets
├── Dockerfile                      # Multi-stage build
├── go.mod                          # Go module (zero dependencies)
├── go.sum                          # Empty (zero dependencies)
└── README.md                       # Project README
```

---

## 18. Milestones

### Phase 1: Foundation (Weeks 1-3)

**Goal:** Basic proxy with REST→GraphQL translation, no gateway features.

- [ ] Project scaffolding, CI/CD setup
- [ ] YAML config loader with env var interpolation
- [ ] HTTP server with graceful shutdown
- [ ] OpenAPI 3.0 parser (zero-dep)
- [ ] OpenAPI→GraphQL schema builder
- [ ] GraphQL query parser and lexer (zero-dep)
- [ ] GraphQL executor with basic field resolution
- [ ] REST upstream client with connection pooling
- [ ] Basic REST→GraphQL proxy (single upstream, flat queries)
- [ ] CLI: `serve`, `validate`, `version` commands

### Phase 2: Bi-directional & Schema Engine (Weeks 4-6)

**Goal:** Full bi-directional proxy with query optimization.

- [ ] GraphQL introspection client
- [ ] GraphQL→REST endpoint builder
- [ ] GraphQL→REST response shaping
- [ ] Nested query resolution (REST→GraphQL)
- [ ] DataLoader implementation (batching, deduplication)
- [ ] Query planner (multi-upstream, parallel execution)
- [ ] Pagination translation (offset ↔ cursor ↔ Relay)
- [ ] Type mapping engine with custom field mappings
- [ ] CLI: `schema` command
- [ ] Error mapping (both directions)

### Phase 3: Gateway Layer (Weeks 7-9)

**Goal:** Production-ready gateway capabilities.

- [ ] Middleware chain framework
- [ ] Auth: passthrough, JWT validation, API key
- [ ] Rate limiter (token bucket, per-IP/header/path)
- [ ] Response cache (sharded LRU, TTL, stale-while-revalidate)
- [ ] Circuit breaker (three-state model)
- [ ] CORS middleware
- [ ] Request ID generation and propagation
- [ ] Health check runner for upstreams
- [ ] Hot-reload configuration watcher
- [ ] Structured JSON logging
- [ ] Prometheus metrics endpoint

### Phase 4: MCP Server (Weeks 10-11)

**Goal:** Full MCP integration with auto-generated tools.

- [ ] MCP JSON-RPC server (HTTP + SSE transport)
- [ ] Auto tool generation from unified schema
- [ ] Tool naming and collision avoidance
- [ ] Tool call execution pipeline
- [ ] MCP initialize/tools/list/tools/call handlers
- [ ] MCP resources/list and resources/read
- [ ] CLI: `tools` command
- [ ] MCP-specific metrics

### Phase 5: Polish & Release (Weeks 12-13)

**Goal:** Documentation, testing, benchmarks, v0.1.0 release.

- [ ] Comprehensive unit tests (>80% coverage)
- [ ] Integration tests with mock upstreams
- [ ] Benchmark suite
- [ ] GraphQL subscription→SSE bridge
- [ ] Admin dashboard (basic web UI)
- [ ] llms.txt generation
- [ ] README.md with examples
- [ ] Docker image (scratch-based)
- [ ] GitHub Actions CI (build, test, release)
- [ ] Example configurations for common setups
- [ ] v0.1.0 release

---

## Appendix A: Comparison with Alternatives

| Feature | Tentaserve | Kong | Apollo Gateway | Hasura | KrakenD |
|---------|-----------|------|---------------|--------|---------|
| REST→GraphQL | ✅ | ❌ | ❌ | ✅ (DB only) | ❌ |
| GraphQL→REST | ✅ | ❌ | ❌ | ❌ | ❌ |
| API Gateway | ✅ | ✅ | ❌ | ❌ | ✅ |
| MCP Server | ✅ | ❌ | ❌ | ❌ | ❌ |
| Zero deps | ✅ | ❌ | ❌ | ❌ | ❌ |
| Single binary | ✅ | ❌ | ❌ | ❌ | ✅ |
| Self-hosted | ✅ | ✅ | ✅ | ✅ | ✅ |
| Configuration | YAML | YAML/DB | JS/TS | Console | JSON |
| Language | Go | Lua/Go | Node.js | Haskell | Go |

## Appendix B: Glossary

| Term | Definition |
|------|-----------|
| **Upstream** | A backend API service that Tentaserve proxies to |
| **Schema Engine** | Internal component that maps types between REST and GraphQL |
| **DataLoader** | Batching mechanism to solve N+1 query problems |
| **Tool** | An MCP representation of an API endpoint |
| **Circuit Breaker** | Protection mechanism that stops sending requests to failing upstreams |
| **Query Plan** | Execution strategy for complex queries spanning multiple upstreams |
| **Hot Reload** | Ability to update configuration without restarting the server |
| **SDL** | Schema Definition Language (GraphQL's schema format) |
| **SSE** | Server-Sent Events (unidirectional streaming over HTTP) |

---

*This specification is a living document. It will be updated as the project evolves. Implementation details that deviate from this spec should be documented in IMPLEMENTATION.md with rationale.*
