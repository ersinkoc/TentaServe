# Tentaserve — Task Breakdown

> Granular implementation tasks for Claude Code sessions.
> Each task has a unique ID, clear scope, acceptance criteria, and dependency list.
> Reference: SPECIFICATION.md (what), IMPLEMENTATION.md (how)

**Naming Convention:** `TS-{phase}{sequence}` (e.g., TS-101 = Phase 1, Task 01)

---

## How to Use This File

```bash
# In Claude Code, reference a task:
"Implement TS-101: Project scaffolding. See TASKS.md, SPECIFICATION.md section 17, IMPLEMENTATION.md section 1."

# Check off completed tasks by changing [ ] to [x]
# Update status notes inline as you go
```

---

## Phase 1: Foundation (Weeks 1-3)

> **Goal:** HTTP server + config + OpenAPI parser + GraphQL parser + basic REST→GQL proxy for a single upstream.

### TS-101: Project Scaffolding
- **Scope:** Create Go module, directory structure per SPECIFICATION.md §17, Makefile, Dockerfile, .gitignore, go.mod with zero dependencies
- **Files to create:**
  - `go.mod` (module `github.com/ersinkoc/tentaserve`, no require block)
  - `cmd/tentaserve/main.go` (minimal entry point with version flag)
  - `internal/` directory tree (empty packages with doc.go files)
  - `Makefile` (per IMPLEMENTATION.md §19.1)
  - `Dockerfile` (per SPECIFICATION.md §16.2)
  - `.gitignore` (bin/, dist/, *.prof, .DS_Store)
  - `tentaserve.example.yaml` (per SPECIFICATION.md §10.2)
  - `llms.txt` (project description for LLMs)
  - `README.md` (badges, one-liner, install, quick start)
- **Acceptance:** `go build ./...` succeeds, `make build` produces binary, `./bin/tentaserve version` prints version
- **Depends on:** Nothing
- **Estimated effort:** Small

---

### TS-102: Error Types & Sentinel Errors
- **Scope:** Define typed errors, sentinel errors, context key types per IMPLEMENTATION.md §1.3-1.4
- **Files:**
  - `internal/errors.go` — `TentaserveError` struct, all sentinel errors
  - `internal/context.go` — context key types, `WithRequestID()`, `RequestID()`, `WithAuthResult()`, etc.
- **Acceptance:** All error types implement `error` interface, `errors.Is()` works with sentinels, `errors.As()` works with `TentaserveError`
- **Depends on:** TS-101
- **Estimated effort:** Small

---

### TS-103: YAML Parser (Zero-Dependency)
- **Scope:** Minimal YAML parser that handles the subset defined in IMPLEMENTATION.md §3.1. Outputs `map[string]any` tree.
- **Files:**
  - `internal/config/yaml/tokenizer.go` — line-by-line tokenization, indentation tracking
  - `internal/config/yaml/parser.go` — tree builder from tokens
  - `internal/config/yaml/marshal.go` — `map[string]any` → typed struct via reflection
  - `internal/config/yaml/parser_test.go` — extensive tests for all supported YAML features
- **Acceptance:** Parses `tentaserve.example.yaml` correctly. Handles nested maps, lists, quoted strings, comments, multi-line strings. Rejects unsupported features with clear errors. Tests cover: simple KV, nested maps 3 levels deep, lists of maps, string quoting variants, boolean/int/float coercion, empty values, comments.
- **Depends on:** TS-101
- **Estimated effort:** Large (600-800 lines estimated)

---

### TS-104: Environment Variable Interpolation
- **Scope:** `${VAR}` and `${VAR:default}` expansion in config strings per IMPLEMENTATION.md §3.3
- **Files:**
  - `internal/config/env.go` — `InterpolateEnv()` function
  - `internal/config/env_test.go`
- **Acceptance:** Expands set vars, uses defaults, errors on missing required vars. Works with nested YAML values. Does not expand inside comments.
- **Depends on:** TS-101
- **Estimated effort:** Small

---

### TS-105: Configuration Struct & Loader
- **Scope:** Define all config structs per SPECIFICATION.md §10.2, loader that reads file → interpolates env → parses YAML → marshals to struct → validates
- **Files:**
  - `internal/config/config.go` — all config structs (`Config`, `ServerConfig`, `GatewayConfig`, `UpstreamConfig`, etc.) with defaults
  - `internal/config/loader.go` — `Load(path string) (*Config, error)` pipeline
  - `internal/config/validator.go` — `Validate()` methods on each sub-struct
  - `internal/config/defaults.go` — default values for all fields
  - `internal/config/config_test.go`
  - `internal/config/loader_test.go`
  - `internal/config/validator_test.go`
- **Acceptance:** Loads `tentaserve.example.yaml`, all defaults applied, validation catches: missing upstream URL, invalid timeout format, negative rate limit values, unknown auth strategy. Duration strings parsed correctly (`30s`, `5m`, `1h`). Memory size strings parsed (`256MB`, `1GB`).
- **Depends on:** TS-103, TS-104
- **Estimated effort:** Medium

---

### TS-106: HTTP Server & Graceful Shutdown
- **Scope:** HTTP server setup, TLS support, graceful shutdown on SIGINT/SIGTERM per IMPLEMENTATION.md §2.1-2.3
- **Files:**
  - `internal/server/server.go` — `NewServer()`, `Run()` with context cancellation
  - `internal/server/server_test.go`
  - Update `cmd/tentaserve/main.go` — signal handling, server startup
- **Acceptance:** Server starts on configured port, responds to requests, gracefully shuts down (in-flight requests complete, new requests rejected). TLS works when cert/key provided. Logs startup/shutdown events.
- **Depends on:** TS-105
- **Estimated effort:** Medium

---

### TS-107: Request ID Middleware
- **Scope:** Generate unique request IDs, inject into context and response headers per IMPLEMENTATION.md §2.4
- **Files:**
  - `internal/gateway/middleware/requestid.go`
  - `internal/gateway/middleware/requestid_test.go`
- **Acceptance:** Every response has `X-Request-ID` header. If client sends `X-Request-ID`, it's preserved. IDs are `req_` + 24 hex chars. ID available via `internal.RequestID(ctx)`.
- **Depends on:** TS-102, TS-106
- **Estimated effort:** Small

---

### TS-108: Structured Logging Setup
- **Scope:** Configure `log/slog` with JSON/text output, level filtering per IMPLEMENTATION.md §16.3
- **Files:**
  - `internal/observability/logger.go` — `SetupLogger()`, request logging middleware
  - `internal/observability/logger_test.go`
- **Acceptance:** JSON format by default, respects configured level, includes request_id in all log entries. Request log includes: method, path, status, duration_ms, upstream, client_ip.
- **Depends on:** TS-105, TS-107
- **Estimated effort:** Small

---

### TS-109: Request Router & Classifier
- **Scope:** Classify incoming requests (GraphQL, REST, MCP, health, metrics) and route to appropriate handler per IMPLEMENTATION.md §4
- **Files:**
  - `internal/proxy/router/router.go` — `ClassifyRequest()`, `UpstreamRouter` with prefix matching
  - `internal/proxy/router/router_test.go`
- **Acceptance:** Correctly classifies: `POST /graphql` → GraphQL, `GET /api/users` → REST, `POST /mcp` → MCP, `GET /-/health` → Health, `GET /-/metrics` → Metrics. Upstream resolution matches longest prefix first. Unknown paths return 404.
- **Depends on:** TS-105
- **Estimated effort:** Small

---

### TS-110: OpenAPI Parser
- **Scope:** Parse OpenAPI 3.0/3.1 specs into typed structs, resolve $refs per IMPLEMENTATION.md §6
- **Files:**
  - `internal/openapi/types.go` — `OpenAPISpec`, `PathItem`, `Operation`, `SchemaObject`, `Parameter`, `RequestBody`, `Response`
  - `internal/openapi/parser.go` — parse from `map[string]any` to typed structs
  - `internal/openapi/refs.go` — `$ref` resolution with circular detection
  - `internal/openapi/loader.go` — `LoadOpenAPISpec()` from file/URL/inline
  - `internal/openapi/parser_test.go` — test with real-world-like OpenAPI specs
  - `internal/openapi/refs_test.go`
- **Acceptance:** Parses a multi-path, multi-schema OpenAPI 3.0 spec. Resolves nested `$ref` chains. Detects circular refs without infinite loop. Handles: all HTTP methods, path/query/header params, request bodies, response schemas, enums, oneOf/allOf/anyOf, nullable fields. Test with at least 2 sample specs (simple CRUD + complex nested).
- **Depends on:** TS-103
- **Estimated effort:** Large

---

### TS-111: GraphQL Lexer
- **Scope:** Tokenize GraphQL query strings per IMPLEMENTATION.md §7.1
- **Files:**
  - `internal/graphql/token.go` — `TokenKind` enum, `Token` struct
  - `internal/graphql/lexer.go` — `Lexer` struct, `NextToken()` method
  - `internal/graphql/lexer_test.go`
- **Acceptance:** Correctly tokenizes: names, integers, floats, strings, block strings, all punctuation, spreads (`...`). Handles: unicode in strings, escape sequences, comments (skipped). Reports line/column on errors. Fuzz test with random inputs doesn't panic.
- **Depends on:** TS-101
- **Estimated effort:** Medium

---

### TS-112: GraphQL Parser (AST)
- **Scope:** Recursive descent parser producing AST per IMPLEMENTATION.md §7.2
- **Files:**
  - `internal/graphql/ast.go` — all AST node types
  - `internal/graphql/parser.go` — `Parse(query string) (*Document, error)`
  - `internal/graphql/parser_test.go`
- **Acceptance:** Parses: simple queries, nested selections, aliases, arguments, variables, fragments, inline fragments, directives, mutations, subscriptions. Produces correct AST verified by test assertions on node types and values. Rejects malformed queries with descriptive errors including line/column.
- **Depends on:** TS-111
- **Estimated effort:** Large

---

### TS-113: GraphQL Printer (AST → String)
- **Scope:** Convert AST back to query string (needed for forwarding queries to upstream GraphQL)
- **Files:**
  - `internal/graphql/printer.go` — `Print(doc *Document) string`
  - `internal/graphql/printer_test.go`
- **Acceptance:** Round-trip: `Parse(Print(Parse(query)))` produces semantically equivalent AST. Handles all AST node types. Output is compact (no unnecessary whitespace).
- **Depends on:** TS-112
- **Estimated effort:** Small

---

### TS-114: GraphQL Validator (Depth & Complexity)
- **Scope:** Query validation before execution per IMPLEMENTATION.md §7.3
- **Files:**
  - `internal/graphql/validator.go` — `CheckDepth()`, `CalculateComplexity()`
  - `internal/graphql/validator_test.go`
- **Acceptance:** Rejects queries exceeding max depth (default 10). Calculates complexity with list multiplier. Rejects queries exceeding max complexity (default 1000). Tests with: flat query (depth 1), deeply nested (depth 15), wide query with lists (high complexity), mixed.
- **Depends on:** TS-112
- **Estimated effort:** Small

---

### TS-115: Upstream HTTP Client
- **Scope:** Connection-pooled HTTP client per upstream, retry logic with backoff per IMPLEMENTATION.md §12
- **Files:**
  - `internal/upstream/client.go` — `NewUpstreamClient()`, `Do()`, `DoWithRetry()`
  - `internal/upstream/retry.go` — exponential backoff with jitter
  - `internal/upstream/client_test.go` (using `httptest.Server`)
  - `internal/upstream/retry_test.go`
- **Acceptance:** Each upstream gets isolated client. Retries on 502/503/504 with exponential backoff. Respects context cancellation during retry delays. No retry on 4xx errors. Request body can be re-read on retry.
- **Depends on:** TS-105
- **Estimated effort:** Medium

---

### TS-116: Schema Engine — Unified Type System
- **Scope:** Internal type representation shared by REST→GQL and GQL→REST per IMPLEMENTATION.md §10.1, SPECIFICATION.md §8.1
- **Files:**
  - `internal/schema/types.go` — `TypeDef`, `FieldDef`, `TypeRef`, `ArgumentDef`, `OperationDef`, `TypeKind` enum
  - `internal/schema/registry.go` — `TypeRegistry` with thread-safe registration and lookup
  - `internal/schema/naming.go` — `toCamelCase()`, `toSnakeCase()`, `toKebabCase()`, `toPascalCase()`
  - `internal/schema/types_test.go`
  - `internal/schema/naming_test.go`
- **Acceptance:** Types can represent all OpenAPI and GraphQL type constructs. Registry supports concurrent read/write. Naming converters handle edge cases: acronyms (URL→url→Url), numbers (user2→user_2), consecutive caps (HTMLParser→html_parser).
- **Depends on:** TS-101
- **Estimated effort:** Medium

---

### TS-117: OpenAPI → GraphQL Schema Builder
- **Scope:** Convert parsed OpenAPI spec to unified types + GraphQL schema definition per IMPLEMENTATION.md §8, SPECIFICATION.md §6.2
- **Files:**
  - `internal/proxy/rest2gql/schema_builder.go` — `BuildSchema(spec *OpenAPISpec) (*SchemaDefinition, error)`
  - `internal/proxy/rest2gql/type_mapper.go` — OpenAPI types → GraphQL types mapping
  - `internal/proxy/rest2gql/relationships.go` — parent-child URL pattern detection per IMPLEMENTATION.md §8.3
  - `internal/proxy/rest2gql/schema_builder_test.go`
- **Acceptance:** Maps all OpenAPI type → GraphQL type conversions per SPECIFICATION.md §6.2 table. GET → Query, POST/PUT/DELETE → Mutation. Detects `/users/{id}/posts` as User→posts relationship. Generates proper argument types (required vs optional). Handles enums, unions (oneOf), merged types (allOf).
- **Depends on:** TS-110, TS-116
- **Estimated effort:** Large

---

### TS-118: REST → GraphQL Resolver Builder
- **Scope:** Generate field resolvers that dispatch REST calls per IMPLEMENTATION.md §8.2
- **Files:**
  - `internal/proxy/rest2gql/resolver.go` — `BuildRESTResolver()` for each operation
  - `internal/proxy/rest2gql/resolver_test.go`
- **Acceptance:** Resolver correctly: interpolates path params (`/users/{id}` → `/users/123`), adds query params, sends JSON body for POST/PUT, forwards auth headers from context, parses JSON response. Test with mock upstream.
- **Depends on:** TS-115, TS-117
- **Estimated effort:** Medium

---

### TS-119: GraphQL Executor (Basic)
- **Scope:** Execute parsed GraphQL queries using registered resolvers per IMPLEMENTATION.md §7.4
- **Files:**
  - `internal/graphql/executor.go` — `Executor` struct, `Execute()`, `resolveSelectionSet()`, `resolveNested()`
  - `internal/graphql/executor_test.go`
- **Acceptance:** Executes flat queries (single field, multiple fields). Handles aliases. Resolves nested object fields. Returns null + error for failed fields (partial response). Default resolver extracts field from parent map. Handles list types.
- **Depends on:** TS-112, TS-116
- **Estimated effort:** Large

---

### TS-120: REST→GraphQL Proxy (End-to-End)
- **Scope:** Wire everything together: request comes in at `POST /graphql`, parse query, resolve against REST upstream, return GraphQL response
- **Files:**
  - `internal/proxy/rest2gql/translator.go` — orchestrator that connects parser → validator → executor → response
  - `internal/proxy/proxy.go` — `Proxy` struct that holds schema + executor + upstreams
  - `internal/proxy/rest2gql/translator_test.go`
  - `internal/proxy/proxy_test.go` (integration test with mock upstream)
- **Acceptance:** Full flow works: send GraphQL query to `/graphql`, get correct data from REST upstream. Test cases: simple field query, nested query (2 levels), query with arguments, mutation (POST), error handling (upstream 404 → null + error).
- **Depends on:** TS-114, TS-117, TS-118, TS-119
- **Estimated effort:** Large

---

### TS-121: CLI Commands (serve, validate, version)
- **Scope:** CLI parsing with flags per SPECIFICATION.md §12
- **Files:**
  - `cmd/tentaserve/main.go` — refactor to use `flag` package for subcommands
  - `cmd/tentaserve/serve.go` — `serve` command with --config, --port, --host, --log-level flags
  - `cmd/tentaserve/validate.go` — `validate` command that loads and validates config
  - `cmd/tentaserve/version.go` — `version` command with build info
- **Acceptance:** `tentaserve serve` starts server. `tentaserve serve --config custom.yaml --port 9090` overrides. `tentaserve validate` exits 0 on valid config, 1 with error details on invalid. `tentaserve version` prints version, go version, OS/arch, build date.
- **Depends on:** TS-105, TS-106
- **Estimated effort:** Medium

---

## Phase 2: Bi-directional & Schema Engine (Weeks 4-6)

> **Goal:** Add GQL→REST direction, DataLoader, query planner, pagination, error mapping.

### TS-201: GraphQL Introspection Client
- **Scope:** Run introspection query against upstream GraphQL, parse result into SchemaDefinition per IMPLEMENTATION.md §7.5
- **Files:**
  - `internal/graphql/introspection.go` — introspection query constant, `Introspect(url string) (*SchemaDefinition, error)`
  - `internal/graphql/introspection_test.go`
- **Acceptance:** Connects to a mock GraphQL server, runs introspection, correctly builds SchemaDefinition with: types, fields, arguments, enums, interfaces, unions, input types, deprecation info. Handles 7-level deep type unwrapping (`NonNull(List(NonNull(Type)))`).
- **Depends on:** TS-115, TS-116
- **Estimated effort:** Medium

---

### TS-202: GraphQL → REST Endpoint Builder
- **Scope:** Generate REST endpoints from introspected GraphQL schema per IMPLEMENTATION.md §9.1
- **Files:**
  - `internal/proxy/gql2rest/endpoint_builder.go` — `GenerateEndpoints()`
  - `internal/proxy/gql2rest/method_inference.go` — HTTP method heuristic from mutation names
  - `internal/proxy/gql2rest/endpoint_builder_test.go`
- **Acceptance:** Queries → GET endpoints. Mutations → POST/PUT/DELETE based on name prefix heuristic per SPECIFICATION.md §7.2 table. Arguments become query params (GET) or body fields (POST). Path uses kebab-case. Handles ID arguments as path params where possible (`updateUser(id)` → `PUT /api/update-user/{id}`).
- **Depends on:** TS-201, TS-116
- **Estimated effort:** Medium

---

### TS-203: GraphQL → REST Request Translator
- **Scope:** Convert incoming REST requests to GraphQL queries, execute against upstream, unwrap response per IMPLEMENTATION.md §9.2-9.3
- **Files:**
  - `internal/proxy/gql2rest/translator.go` — `TranslateRESTToGraphQL()`
  - `internal/proxy/gql2rest/response.go` — `UnwrapGraphQLResponse()`
  - `internal/proxy/gql2rest/fields.go` — `?fields=` param to selection set builder
  - `internal/proxy/gql2rest/translator_test.go`
  - `internal/proxy/gql2rest/response_test.go`
- **Acceptance:** GET with query params correctly builds GraphQL query. POST with JSON body correctly builds GraphQL mutation. `?fields=name,email,posts.title` correctly limits selection. Response unwrapped from `{"data":{"user":{...}}}` to just `{...}`. GraphQL errors mapped to HTTP status codes per SPECIFICATION.md §7.3 table.
- **Depends on:** TS-202, TS-113
- **Estimated effort:** Large

---

### TS-204: GQL→REST Proxy (End-to-End)
- **Scope:** Wire GQL→REST direction: REST request → translate to GraphQL → execute against upstream → unwrap → respond
- **Files:**
  - Update `internal/proxy/proxy.go` — add GQL→REST path
  - `internal/proxy/gql2rest/handler.go` — HTTP handler for generated REST endpoints
  - Integration test with mock GraphQL upstream
- **Acceptance:** Full flow: `GET /api/user?id=123` → GraphQL query to upstream → JSON response. Test: simple query, mutation, field selection, error mapping, not-found handling.
- **Depends on:** TS-203, TS-119
- **Estimated effort:** Medium

---

### TS-205: DataLoader Implementation
- **Scope:** Per-request batching DataLoader per IMPLEMENTATION.md §10.2
- **Files:**
  - `internal/schema/dataloader.go` — `DataLoader` struct with `Load()`, `dispatch()`, batch window, max batch size
  - `internal/schema/dataloader_test.go`
- **Acceptance:** Multiple `Load()` calls within batch window are dispatched as single batch. Batch triggers on: timer expiry OR max batch size reached. Each request gets its own DataLoader (no cross-request batching). Context cancellation stops pending loads. Test: 10 concurrent loads → 1 batch call, verify batch function receives all keys.
- **Depends on:** TS-116
- **Estimated effort:** Medium

---

### TS-206: DataLoader Integration with Resolvers
- **Scope:** Wire DataLoader into nested field resolution for REST→GQL proxy
- **Files:**
  - Update `internal/proxy/rest2gql/resolver.go` — use DataLoader for nested fields
  - `internal/schema/dataloader_factory.go` — create DataLoaders per upstream, per request
  - Integration test: query with nested fields, verify batch call count
- **Acceptance:** Query `{ users { posts { title } } }` with 10 users results in: 1 call to `/api/users` + 1 batched call to `/api/posts?user_ids=1,2,...,10` (if batch endpoint exists) OR max N parallel calls (if not). DataLoader reuses results for duplicate keys.
- **Depends on:** TS-205, TS-118, TS-120
- **Estimated effort:** Medium

---

### TS-207: Query Planner
- **Scope:** Build execution plans for queries spanning multiple upstreams per IMPLEMENTATION.md §10.3
- **Files:**
  - `internal/schema/planner.go` — `BuildQueryPlan()`, `PlanStep` struct, topological sort
  - `internal/schema/planner_test.go`
- **Acceptance:** Plan correctly identifies: step dependencies, parallel steps at same depth, data extraction between steps. Test with: single-upstream query (1 step), two-upstream query (2 steps, sequential), three-upstream with parallel leaf steps.
- **Depends on:** TS-116
- **Estimated effort:** Medium

---

### TS-208: Pagination Translation
- **Scope:** Translate between offset, cursor, and Relay pagination styles per SPECIFICATION.md §6.3
- **Files:**
  - `internal/proxy/rest2gql/pagination.go` — pagination style detection, Relay connection type builder, offset↔cursor translation
  - `internal/proxy/rest2gql/pagination_test.go`
- **Acceptance:** Detects pagination style from OpenAPI params (page/limit → offset, after/before → cursor). Generates Relay connection types (edges, pageInfo, totalCount). Translates: `users(first: 10, after: "abc")` → `GET /api/users?cursor=abc&limit=10`. Translates: `GET /api/users?page=2&limit=10` → `users(page: 2, limit: 10)`.
- **Depends on:** TS-117
- **Estimated effort:** Medium

---

### TS-209: Error Mapping (Both Directions) ✓
- **Status:** Completed
- **Files:**
  - `internal/proxy/rest2gql/errors.go` — REST status → GraphQL error with extensions
  - `internal/proxy/rest2gql/errors_test.go` — tests for error mapper
  - `internal/proxy/gql2rest/errors.go` — GraphQL error extension → HTTP status
  - `internal/proxy/gql2rest/errors_test.go` — tests for error mapper
- **Acceptance:** All mappings from spec tables implemented. Upstream error body included in GraphQL `extensions.upstream` (configurable). 404 → null + error in errors array. Unknown GraphQL error → 500.
- **Depends on:** TS-102
- **Estimated effort:** Small

---

### TS-210: Custom Field Mappings ✓
- **Status:** Completed
- **Files:**
  - `internal/schema/mapping.go` — `FieldMapper` with bidirectional lookup, `PerUpstreamFieldMapper`, nested structure mapping
  - `internal/schema/mapping_test.go` — comprehensive tests for mappings, conventions, per-upstream mappers
- **Acceptance:** Config `field_mappings: {"usr_nm": "userName"}` correctly renames fields in both directions. Unmapped fields pass through with naming convention applied (camelCase/snake_case/PascalCase/kebab-case). Nested maps and slices are recursively mapped.
- **Depends on:** TS-116
- **Estimated effort:** Small

---

### TS-211: CLI `schema` Command ✓
- **Status:** Completed
- **Files:**
  - `cmd/tentaserve/schema.go` — `schema` command implementation with `--upstream` and `--format` flags
  - `internal/graphql/sdl.go` — SDL printer with `PrintSDL()` function
  - Updated `cmd/tentaserve/main.go` — added schema command to CLI
- **Acceptance:** `tentaserve schema` prints merged GraphQL SDL. `tentaserve schema --upstream users-api` filters to one upstream. `tentaserve schema --format json` outputs OpenAPI-like JSON.
- **Depends on:** TS-117, TS-202
- **Estimated effort:** Medium

---

## Phase 3: Gateway Layer (Weeks 7-9)

> **Goal:** Production-ready gateway: auth, rate limit, cache, circuit breaker, CORS, health, hot-reload, metrics.

### TS-301: Middleware Chain Framework ✓
- **Status:** Completed
- **Files:**
  - `internal/gateway/middleware/chain.go` — `Chain` struct with `Then()`, `Append()`, `Recovery()`, `Capture()`, `Skip()`, `If()`, `Combine()`
  - `internal/gateway/middleware/chain_test.go` — comprehensive tests (19 tests)
- **Acceptance:** Middleware chain executes in order. Recovery middleware catches panics, logs error, returns 500. Response capture wrapper for post-processing. Conditional middleware with Skip and If. All tests pass.
- **Depends on:** TS-107
- **Estimated effort:** Small

---

### TS-302: Authentication — Passthrough ✓
- **Status:** Completed
- **Files:**
  - `internal/gateway/auth/auth.go` — `Plugin` interface, `Result` struct, context helpers
  - `internal/gateway/auth/passthrough.go` — `Passthrough` strategy with header filtering
  - `internal/gateway/auth/middleware.go` — HTTP middleware for auth
  - `internal/gateway/auth/passthrough_test.go` — comprehensive tests (12 tests)
- **Acceptance:** All request headers forwarded to upstream. `AuthResult.Authenticated = true` always. Headers can be filtered by prefix or excluded list. Context helpers for accessing auth info.
- **Depends on:** TS-301
- **Estimated effort:** Small

---

### TS-303: Authentication — JWT Validation ✓
- **Status:** Completed
- **Files:**
  - `internal/gateway/auth/jwt.go` — `JWT` plugin with HS256, HS384, HS512 support
  - `internal/gateway/auth/jwt_test.go` — comprehensive tests (20 tests)
- **Acceptance:** Validates HS256, HS384, and HS512 tokens. Rejects: expired tokens, wrong algorithm, invalid signature, malformed tokens. Extracts claims and stores in context. Configurable issuer and audience validation. Returns 401 with `WWW-Authenticate` header on failure. Custom header name and prefix support.
- **Depends on:** TS-302
- **Estimated effort:** Medium

---

### TS-304: Authentication — API Key ✓
- **Status:** Completed
- **Files:**
  - `internal/gateway/auth/apikey.go` — `APIKey` plugin with constant-time comparison
  - `internal/gateway/auth/apikey_test.go` — comprehensive tests (12 tests)
- **Acceptance:** Accepts requests with valid API key in `X-API-Key` header. Rejects unknown keys with 401. Constant-time comparison to prevent timing attacks. Custom header name and prefix support. WWW-Authenticate header on failure.
- **Depends on:** TS-302
- **Estimated effort:** Small

---

### TS-305: Rate Limiter ✓
- **Status:** Completed
- **Files:**
  - `internal/gateway/ratelimit/bucket.go` — `TokenBucket` with atomic operations (HS256, HS384, HS512)
  - `internal/gateway/ratelimit/store.go` — `Store` with per-client buckets and scoping
  - `internal/gateway/ratelimit/middleware.go` — rate limit HTTP middleware
  - `internal/gateway/ratelimit/cleanup.go` — `CleanupManager` for stale bucket cleanup
  - `internal/gateway/ratelimit/bucket_test.go` — 12 tests
  - `internal/gateway/ratelimit/store_test.go` — 18 tests
  - `internal/gateway/ratelimit/middleware_test.go` — 8 tests
- **Acceptance:** Lock-free token bucket using atomic operations. Rejects with 429 + Retry-After header. Scoping: global, per-ip, per-header, per-path. Per-upstream config override. Stale bucket cleanup with configurable interval. Burst allowance. All 38 tests pass.
- **Depends on:** TS-301, TS-105
- **Estimated effort:** Large

---

### TS-306: Response Cache
- **Scope:** Sharded LRU cache with TTL and stale-while-revalidate per IMPLEMENTATION.md §13
- **Files:**
  - `internal/gateway/cache/lru.go` — `ShardedCache`, `cacheShard` with LRU eviction
  - `internal/gateway/cache/key.go` — `BuildCacheKey()`
  - `internal/gateway/cache/middleware.go` — cache lookup + store middleware
  - `internal/gateway/cache/lru_test.go`
  - `internal/gateway/cache/key_test.go`
  - `internal/gateway/cache/middleware_test.go`
- **Acceptance:** Cache hit returns stored response without upstream call. TTL respected. LRU eviction when max entries reached. Stale-while-revalidate serves stale + background refresh. Cache key includes: method, path, sorted query params, vary headers, body hash. PURGE method invalidates entry. Mutations not cached by default. Concurrent access safe (race detector).
- **Depends on:** TS-301
- **Estimated effort:** Large

---

### TS-307: Circuit Breaker
- **Scope:** Per-upstream circuit breaker per IMPLEMENTATION.md §15
- **Files:**
  - `internal/gateway/circuitbreaker/breaker.go` — three-state machine with atomic ops
  - `internal/gateway/circuitbreaker/middleware.go`
  - `internal/gateway/circuitbreaker/breaker_test.go`
- **Acceptance:** Closed→Open after N consecutive failures. Open→HalfOpen after timeout. HalfOpen→Closed after M successes. HalfOpen→Open on any failure. Only configured failure codes trigger (500, 502, 503, 504). Returns 503 when circuit is open. State changes logged.
- **Depends on:** TS-301, TS-102
- **Estimated effort:** Medium

---

### TS-308: CORS Middleware
- **Scope:** CORS header handling per config per SPECIFICATION.md §10.2
- **Files:**
  - `internal/gateway/middleware/cors.go`
  - `internal/gateway/middleware/cors_test.go`
- **Acceptance:** Preflight (OPTIONS) requests handled correctly. `Access-Control-Allow-Origin` set per config. `Access-Control-Allow-Methods`, `Allow-Headers`, `Expose-Headers`, `Max-Age`, `Allow-Credentials` all configurable. Wildcard origin works. Specific origin list works.
- **Depends on:** TS-301
- **Estimated effort:** Small

---

### TS-309: Health Check Endpoint & Runner
- **Scope:** Health check endpoint + background upstream health checking per SPECIFICATION.md §11.2, IMPLEMENTATION.md §12.3
- **Files:**
  - `internal/upstream/health.go` — `HealthChecker`, background check loop
  - `internal/observability/health.go` — `GET /-/health` handler, response format
  - `internal/upstream/health_test.go`
  - `internal/observability/health_test.go`
- **Acceptance:** `/~-/health` returns JSON with overall status + per-upstream status. Background checks run at configured interval. Status: healthy (all up), degraded (some down), unhealthy (all down). Latency reported per upstream. Returns 200 for healthy/degraded, 503 for unhealthy.
- **Depends on:** TS-115
- **Estimated effort:** Medium

---

### TS-310: Prometheus Metrics Endpoint
- **Scope:** Prometheus text format metrics at `/-/metrics` per IMPLEMENTATION.md §16.1-16.2, SPECIFICATION.md §11.1
- **Files:**
  - `internal/observability/metrics.go` — `MetricsRegistry`, `Counter`, `Histogram`, `Gauge`
  - `internal/observability/metrics_handler.go` — `GET /-/metrics` handler
  - `internal/observability/metrics_test.go`
- **Acceptance:** All metrics from SPECIFICATION.md §11.1 tables implemented: request counters/histograms, cache hit/miss, rate limit hits, circuit breaker state, upstream health, MCP tool calls, system metrics (uptime, goroutines, memory). Output format matches Prometheus text exposition spec. Histogram uses default buckets.
- **Depends on:** TS-106
- **Estimated effort:** Large

---

### TS-311: Config Hot Reload
- **Scope:** Polling-based config file watcher with atomic swap per IMPLEMENTATION.md §3.4
- **Files:**
  - `internal/config/watcher.go` — `ConfigWatcher` with SHA256-based change detection
  - `internal/config/atomic.go` — `AtomicConfig` wrapper
  - `internal/config/watcher_test.go`
  - Wire into server: update gateway, upstreams, schema on config change
- **Acceptance:** Config change detected within 5s (default poll interval). Invalid config logged and rejected (old config retained). Valid config atomically swapped. In-flight requests unaffected. New requests use new config. Metric `tentaserve_config_reloads_total` incremented. Server host/port changes ignored (logged as warning).
- **Depends on:** TS-105
- **Estimated effort:** Medium

---

### TS-312: Gateway Orchestrator (Wire All Middleware)
- **Scope:** Assemble the full middleware chain and connect to proxy handlers
- **Files:**
  - `internal/gateway/gateway.go` — `Gateway` struct, `NewGateway()`, `HandleGraphQL()`, `HandleREST()`, `HandleMCP()`, `HandleHealth()`, `HandleMetrics()`
  - `internal/gateway/gateway_test.go` (integration test)
- **Acceptance:** Full middleware chain: RequestID → Logging → Recovery → CORS → Auth → RateLimit → CacheLookup → CircuitBreaker → [Proxy] → CacheStore. Config determines which middlewares are active. Disabled middlewares are no-ops, not removed from chain. Integration test: send request through full chain to mock upstream, verify all middleware headers/behaviors.
- **Depends on:** TS-301 through TS-311
- **Estimated effort:** Large

---

## Phase 4: MCP Server (Weeks 10-11)

> **Goal:** Full MCP integration with auto-generated tools from unified schema.

### TS-401: MCP JSON-RPC Server
- **Scope:** JSON-RPC 2.0 server at `POST /mcp` per IMPLEMENTATION.md §11.1
- **Files:**
  - `internal/mcp/transport.go` — JSON-RPC request/response types, parse/serialize
  - `internal/mcp/server.go` — `MCPServer`, `Handle()` method dispatch
  - `internal/mcp/errors.go` — JSON-RPC error codes
  - `internal/mcp/transport_test.go`
  - `internal/mcp/server_test.go`
- **Acceptance:** Parses valid JSON-RPC requests. Returns proper JSON-RPC responses. Error codes: -32700 (parse error), -32600 (invalid request), -32601 (method not found), -32000 (server error). Handles batch requests (array of requests).
- **Depends on:** TS-106
- **Estimated effort:** Medium

---

### TS-402: MCP `initialize` Handler
- **Scope:** MCP handshake, capability negotiation
- **Files:**
  - `internal/mcp/handlers.go` — `handleInitialize()`
- **Acceptance:** Returns server info (name, version) and capabilities (tools, resources). Follows MCP spec for initialize response format.
- **Depends on:** TS-401
- **Estimated effort:** Small

---

### TS-403: Tool Registry & Auto-Generation
- **Scope:** Build MCP tools from unified schema per IMPLEMENTATION.md §11.3-11.5
- **Files:**
  - `internal/mcp/tools.go` — `ToolRegistry`, `Tool` struct, `BuildFromSchema()`
  - `internal/mcp/toolname.go` — `generateToolName()`, `sanitizeName()`, collision avoidance
  - `internal/mcp/toolschema.go` — `generateInputSchema()`, `generateToolDescription()`
  - `internal/mcp/tools_test.go`
  - `internal/mcp/toolname_test.go`
  - `internal/mcp/toolschema_test.go`
- **Acceptance:** Generates tools from: REST upstreams (via OpenAPI operationId or path), GraphQL upstreams (via field names). Tool names: snake_case, max 64 chars, no collisions. Input schemas: valid JSON Schema with types, descriptions, required fields, defaults. Descriptions include upstream name and operation summary. Exclude patterns from config respected.
- **Depends on:** TS-116, TS-117, TS-201
- **Estimated effort:** Large

---

### TS-404: MCP `tools/list` Handler
- **Scope:** Return all registered tools per MCP spec
- **Files:**
  - Update `internal/mcp/handlers.go` — `handleToolsList()`
- **Acceptance:** Returns array of tools with name, description, inputSchema. Pagination support via cursor (optional for v0.1). Tools sorted alphabetically by name.
- **Depends on:** TS-403
- **Estimated effort:** Small

---

### TS-405: MCP `tools/call` Handler
- **Scope:** Execute a tool call: resolve tool → apply gateway → dispatch to upstream → return result per SPECIFICATION.md §9.6
- **Files:**
  - `internal/mcp/handler.go` — `handleToolsCall()`
  - `internal/mcp/executor.go` — tool call execution pipeline
  - `internal/mcp/handler_test.go`
  - `internal/mcp/executor_test.go`
- **Acceptance:** Resolves tool name to upstream + operation. Builds upstream request from tool arguments. Applies gateway middleware (auth, rate limit). Returns JSON result. Handles errors: unknown tool, invalid arguments, upstream failure. Test with mock upstream: call tool, verify correct REST/GraphQL call made, correct result returned.
- **Depends on:** TS-403, TS-312
- **Estimated effort:** Large

---

### TS-406: MCP `resources/list` and `resources/read`
- **Scope:** Expose upstream metadata as MCP resources
- **Files:**
  - Update `internal/mcp/handlers.go` — `handleResourcesList()`, `handleResourcesRead()`
- **Acceptance:** Lists upstreams as resources (name, type, URL). Read returns schema for a specific upstream (GraphQL SDL or OpenAPI JSON). Useful for AI agents to understand the API structure.
- **Depends on:** TS-401, TS-211
- **Estimated effort:** Small

---

### TS-407: MCP SSE Transport (Streaming)
- **Scope:** Server-Sent Events transport for MCP streaming responses
- **Files:**
  - `internal/mcp/sse.go` — SSE writer for streaming tool results
  - `internal/mcp/sse_test.go`
- **Acceptance:** Supports `GET /mcp` with `Accept: text/event-stream` for SSE. Tool call results streamed as SSE events. Connection cleanup on client disconnect.
- **Depends on:** TS-401
- **Estimated effort:** Medium

---

### TS-408: CLI `tools` Command
- **Scope:** List generated MCP tools from CLI per SPECIFICATION.md §12.1
- **Files:**
  - `cmd/tentaserve/tools.go`
- **Acceptance:** `tentaserve tools` lists all tools (name, description, upstream). `tentaserve tools --upstream users-api` filters. `tentaserve tools --format json` outputs JSON array of tool definitions.
- **Depends on:** TS-403
- **Estimated effort:** Small

---

### TS-409: MCP Metrics
- **Scope:** MCP-specific metrics per SPECIFICATION.md §11.1
- **Files:**
  - Update `internal/mcp/server.go` — instrument all handlers
  - Update `internal/observability/metrics.go` — add MCP metric definitions
- **Acceptance:** Metrics emitted: `tentaserve_mcp_tool_calls_total{tool,status}`, `tentaserve_mcp_tool_duration_seconds{tool}`, `tentaserve_mcp_sessions_active`.
- **Depends on:** TS-310, TS-405
- **Estimated effort:** Small

---

## Phase 5: Polish & Release (Weeks 12-13)

> **Goal:** Testing, docs, benchmarks, subscription bridge, Docker, CI, v0.1.0 release.

### TS-501: Comprehensive Unit Tests
- **Scope:** Fill coverage gaps, target >80% overall per IMPLEMENTATION.md §18.3
- **Files:** `*_test.go` across all packages
- **Acceptance:** `go test -cover ./...` reports >80% overall. Critical packages (graphql, config, schema) >85%. No test relies on external services or network.
- **Depends on:** All Phase 1-4 tasks
- **Estimated effort:** Large

---

### TS-502: Integration Tests
- **Scope:** Multi-component integration tests with mock upstreams
- **Files:**
  - `test/integration/rest2gql_test.go` — full REST→GraphQL flow
  - `test/integration/gql2rest_test.go` — full GraphQL→REST flow
  - `test/integration/mcp_test.go` — MCP tool discovery + execution
  - `test/integration/gateway_test.go` — auth + rate limit + cache + circuit breaker
  - `test/integration/helpers_test.go` — mock upstream builders
- **Acceptance:** Each test starts real Tentaserve + mock upstreams using `httptest`. Tests cover: happy path, error cases, edge cases (empty responses, large payloads, timeout). All pass with race detector (`go test -race`).
- **Depends on:** All Phase 1-4 tasks
- **Estimated effort:** Large

---

### TS-503: Benchmark Suite
- **Scope:** Performance benchmarks per SPECIFICATION.md §15.2
- **Files:**
  - `test/bench/proxy_bench_test.go` — passthrough, REST→GQL, GQL→REST, MCP
  - `test/bench/cache_bench_test.go` — cache hit vs miss
  - `test/bench/ratelimit_bench_test.go` — token bucket under contention
  - `test/bench/dataloader_bench_test.go` — batch vs individual
- **Acceptance:** Results documented in `BENCHMARKS.md`. Targets from spec met: <2ms P50 passthrough, <5ms P50 translation, >50k req/s single core. Each benchmark uses `testing.B` with `b.ReportAllocs()`.
- **Depends on:** TS-501
- **Estimated effort:** Medium

---

### TS-504: GraphQL Subscription → SSE Bridge
- **Scope:** Bridge upstream GraphQL subscriptions to SSE for REST clients per IMPLEMENTATION.md §9.4
- **Files:**
  - `internal/proxy/gql2rest/subscription.go` — `HandleSSESubscription()`
  - `internal/proxy/gql2rest/subscription_test.go`
- **Acceptance:** `GET /api/stream/{subscription-name}?args` → SSE stream. Each GraphQL subscription event emitted as `data:` frame. Connection closes on client disconnect. Upstream WebSocket reconnect on failure.
- **Depends on:** TS-204
- **Estimated effort:** Medium

---

### TS-505: Admin Dashboard (Basic Web UI)
- **Scope:** Minimal web UI at `/-/admin` showing: upstreams status, metrics summary, active config, MCP tools list
- **Files:**
  - `internal/admin/handler.go` — serves embedded HTML
  - `internal/admin/dashboard.html` — single-page dashboard (embedded via `embed` package)
  - `internal/admin/api.go` — JSON API for dashboard data
- **Acceptance:** Single HTML page, no external dependencies (inline CSS/JS). Shows: upstream health, request count, cache hit rate, active tools, config summary. Auto-refreshes every 5s. Protected by basic auth if configured.
- **Depends on:** TS-309, TS-310, TS-403
- **Estimated effort:** Medium

---

### TS-506: llms.txt Generation
- **Scope:** Auto-generate `llms.txt` from running config describing all available endpoints and tools
- **Files:**
  - `internal/llmstxt/generator.go` — `Generate(config *Config, registry *ToolRegistry) string`
  - Static `llms.txt` for the project itself
- **Acceptance:** Output describes: what Tentaserve is, available endpoints (GraphQL, REST, MCP), all MCP tools with descriptions, configuration summary. Useful for AI agents to understand the running instance.
- **Depends on:** TS-403
- **Estimated effort:** Small

---

### TS-507: Docker Image & CI
- **Scope:** Multi-stage Docker build + GitHub Actions CI pipeline per IMPLEMENTATION.md §19.2
- **Files:**
  - `Dockerfile` (finalize)
  - `.github/workflows/ci.yml` — test, lint, build on push/PR
  - `.github/workflows/release.yml` — build + release on tag push
- **Acceptance:** Docker image < 20MB (scratch base). CI runs: `go vet`, `go test -race`, cross-compile. Release creates GitHub release with binaries for all platforms + Docker image.
- **Depends on:** TS-501
- **Estimated effort:** Medium

---

### TS-508: README & Documentation
- **Scope:** Comprehensive README with quickstart, examples, configuration reference
- **Files:**
  - `README.md` — full README with: badges, one-liner, feature list, quickstart (3 steps), architecture diagram, configuration reference, MCP usage example, comparison table, contributing guide
  - `docs/examples/` — example configs for common setups (single REST upstream, multiple upstreams, GraphQL upstream, MCP-only mode)
- **Acceptance:** New user can go from zero to running Tentaserve in < 5 minutes following README. All config options documented. MCP tool usage example with Claude Code.
- **Depends on:** All tasks
- **Estimated effort:** Medium

---

### TS-509: v0.1.0 Release Checklist
- **Scope:** Final release preparation
- **Tasks:**
  - [ ] All Phase 1-4 tasks complete
  - [ ] Test coverage > 80%
  - [ ] Benchmarks documented
  - [ ] README complete
  - [ ] CHANGELOG.md written
  - [ ] Git tag `v0.1.0`
  - [ ] GitHub release with binaries
  - [ ] Docker image published
  - [ ] `go install github.com/ersinkoc/tentaserve/cmd/tentaserve@v0.1.0` works
  - [ ] Example configs tested end-to-end
  - [ ] llms.txt accurate
- **Depends on:** All tasks
- **Estimated effort:** Small

---

## Task Summary

| Phase | Tasks | Critical Path |
|-------|-------|---------------|
| Phase 1: Foundation | TS-101 → TS-121 (21 tasks) | TS-103 → TS-105 → TS-110 → TS-117 → TS-120 |
| Phase 2: Bi-directional | TS-201 → TS-211 (11 tasks) | TS-201 → TS-203 → TS-204 |
| Phase 3: Gateway | TS-301 → TS-312 (12 tasks) | TS-301 → TS-305/306/307 → TS-312 |
| Phase 4: MCP | TS-401 → TS-409 (9 tasks) | TS-401 → TS-403 → TS-405 |
| Phase 5: Polish | TS-501 → TS-509 (9 tasks) | TS-501 → TS-502 → TS-509 |
| **Total** | **62 tasks** | |

## Dependency Graph (Critical Path)

```
TS-101 ─┬─ TS-103 ── TS-105 ── TS-106 ─────────────────────── TS-312 ── TS-405
         │              │                                          │
         │              ├── TS-110 ── TS-117 ── TS-118 ── TS-120 ─┘
         │              │                                   │
         │              └── TS-115 ──────────────────────── ┤
         │                                                  │
         ├─ TS-111 ── TS-112 ── TS-119 ────────────────── ┤
         │                                                  │
         ├─ TS-116 ── TS-205 ── TS-206 ────────────────── ┘
         │
         └─ TS-102
```

---

*Update this file as tasks are completed. Each task should be implementable in a single Claude Code session (30-90 minutes). If a task takes longer, consider splitting it.*
