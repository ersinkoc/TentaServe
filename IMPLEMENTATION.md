# Tentaserve — Implementation Guide

> Technical decisions, patterns, algorithms, and implementation details for Tentaserve.
> This document bridges SPECIFICATION.md (what to build) to TASKS.md (how to build it).

**Version:** 0.1.0-draft
**Companion:** SPECIFICATION.md, TASKS.md

---

## Table of Contents

1. [Foundational Decisions](#1-foundational-decisions)
2. [HTTP Server Implementation](#2-http-server-implementation)
3. [Configuration System](#3-configuration-system)
4. [Request Router](#4-request-router)
5. [Gateway Middleware Chain](#5-gateway-middleware-chain)
6. [OpenAPI Parser](#6-openapi-parser)
7. [GraphQL Parser & Executor](#7-graphql-parser--executor)
8. [REST → GraphQL Translation](#8-rest--graphql-translation)
9. [GraphQL → REST Translation](#9-graphql--rest-translation)
10. [Schema Engine & DataLoader](#10-schema-engine--dataloader)
11. [MCP Server](#11-mcp-server)
12. [Upstream Client](#12-upstream-client)
13. [Caching Layer](#13-caching-layer)
14. [Rate Limiter](#14-rate-limiter)
15. [Circuit Breaker](#15-circuit-breaker)
16. [Observability](#16-observability)
17. [Plugin System](#17-plugin-system)
18. [Testing Strategy](#18-testing-strategy)
19. [Build & Release](#19-build--release)
20. [Performance Optimization Playbook](#20-performance-optimization-playbook)

---

## 1. Foundational Decisions

### 1.1 Zero-Dependency Constraint

Every line of code in Tentaserve uses only the Go standard library. No exceptions.

**What this means concretely:**

| Need | Stdlib Solution | NOT Using |
|------|----------------|-----------|
| HTTP server | `net/http` | Gin, Echo, Fiber |
| JSON | `encoding/json` | jsoniter, easyjson |
| YAML parsing | Custom parser (section 3) | gopkg.in/yaml.v3 |
| JWT validation | Custom parser (section 5.3) | golang-jwt/jwt |
| Metrics | Custom Prometheus text format | prometheus/client_golang |
| Logging | `log/slog` (stdlib since Go 1.21) | zerolog, zap |
| Testing | `testing` + `net/http/httptest` | testify, gomock |
| UUID | `crypto/rand` based V4 | google/uuid |
| Concurrency | `sync`, `sync/atomic` | errgroup (it's stdlib but we use raw goroutines for control) |

**Exception policy:** If a stdlib solution requires >500 lines of boilerplate for what a library does in 10 lines, document the trade-off in this file and implement the minimal subset needed. Never import the library.

### 1.2 Go Version & Build Tags

- **Minimum Go version:** 1.22 (for `net/http` routing enhancements and `log/slog`)
- **Target Go version:** 1.23+
- **CGO:** Disabled (`CGO_ENABLED=0` in all builds)
- **Build tags:** None required for core functionality

### 1.3 Error Handling Philosophy

Tentaserve uses typed errors with sentinel values for well-known failure modes:

```go
// internal/errors.go
package internal

import "errors"

// Sentinel errors — used with errors.Is() for control flow
var (
    ErrUpstreamTimeout     = errors.New("upstream timeout")
    ErrUpstreamUnavailable = errors.New("upstream unavailable")
    ErrCircuitOpen         = errors.New("circuit breaker open")
    ErrRateLimited         = errors.New("rate limit exceeded")
    ErrAuthFailed          = errors.New("authentication failed")
    ErrCacheMiss           = errors.New("cache miss")
    ErrConfigInvalid       = errors.New("invalid configuration")
    ErrSchemaInvalid       = errors.New("invalid schema")
    ErrQueryTooComplex     = errors.New("query exceeds complexity limit")
    ErrQueryTooDeep        = errors.New("query exceeds depth limit")
)

// Typed error for rich context
type TentaserveError struct {
    Code       string // machine-readable: "UPSTREAM_TIMEOUT", "RATE_LIMITED", etc.
    Message    string // human-readable
    Upstream   string // which upstream, if applicable
    StatusCode int    // suggested HTTP status code
    Cause      error  // wrapped original error
}

func (e *TentaserveError) Error() string { return e.Message }
func (e *TentaserveError) Unwrap() error { return e.Cause }
```

**Rules:**
- Never `panic` in production code (only in tests for impossible states)
- Always wrap errors with context: `fmt.Errorf("parsing upstream %s: %w", name, err)`
- Use `errors.Is()` and `errors.As()` for error inspection, never string matching
- Log errors at the boundary (HTTP handler), not inside business logic

### 1.4 Context Propagation

Every function that does I/O or might be cancelled accepts `context.Context` as first parameter.

Request-scoped values stored in context:

```go
type contextKey int

const (
    ctxKeyRequestID contextKey = iota
    ctxKeyAuthResult
    ctxKeyUpstreamName
    ctxKeyStartTime
)

func RequestID(ctx context.Context) string {
    v, _ := ctx.Value(ctxKeyRequestID).(string)
    return v
}

func WithRequestID(ctx context.Context, id string) context.Context {
    return context.WithValue(ctx, ctxKeyRequestID, id)
}
```

### 1.5 Naming Conventions

| Entity | Convention | Example |
|--------|-----------|---------|
| Package names | Short, lowercase, no underscores | `ratelimit`, `rest2gql`, `circuitbreaker` |
| Interface names | Noun or -er suffix | `Cache`, `Authenticator`, `Translator` |
| Struct names | PascalCase, descriptive | `TokenBucketLimiter`, `SchemaBuilder` |
| Private fields | camelCase | `maxEntries`, `failureCount` |
| Constants | PascalCase or ALL_CAPS for env keys | `DefaultBatchWindow`, `ENV_JWT_SECRET` |
| Files | snake_case | `schema_builder.go`, `token_bucket.go` |
| Test files | `*_test.go` colocated | `schema_builder_test.go` |

---

## 2. HTTP Server Implementation

### 2.1 Server Setup

Use Go 1.22's enhanced `net/http` ServeMux with method-based routing:

```go
func NewServer(cfg *config.Config, gateway *gateway.Gateway) *http.Server {
    mux := http.NewServeMux()

    // Protocol-specific endpoints
    mux.HandleFunc("POST /graphql", gateway.HandleGraphQL)
    mux.HandleFunc("POST /mcp", gateway.HandleMCP)

    // REST proxy — catch-all under configured prefix
    mux.HandleFunc("/api/", gateway.HandleREST)

    // Observability
    mux.HandleFunc("GET /-/health", gateway.HandleHealth)
    mux.HandleFunc("GET /-/metrics", gateway.HandleMetrics)

    // Admin (optional)
    if cfg.Admin.Enabled {
        mux.HandleFunc("/-/admin/", gateway.HandleAdmin)
    }

    return &http.Server{
        Addr:              net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port)),
        Handler:           mux,
        ReadTimeout:       cfg.Server.ReadTimeout,
        WriteTimeout:      cfg.Server.WriteTimeout,
        IdleTimeout:       cfg.Server.IdleTimeout,
        MaxHeaderBytes:    cfg.Server.MaxHeaderBytes,
    }
}
```

### 2.2 Graceful Shutdown

```go
func Run(ctx context.Context, srv *http.Server) error {
    errCh := make(chan error, 1)
    go func() {
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            errCh <- err
        }
        close(errCh)
    }()

    select {
    case err := <-errCh:
        return fmt.Errorf("server failed: %w", err)
    case <-ctx.Done():
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
        defer cancel()
        return srv.Shutdown(shutdownCtx)
    }
}
```

Main entry listens for `SIGINT`/`SIGTERM` and cancels the root context.

### 2.3 TLS Support

When TLS is enabled, use `tls.LoadX509KeyPair` and configure `*tls.Config` with:

```go
tlsCfg := &tls.Config{
    MinVersion:               tls.VersionTLS12,
    PreferServerCipherSuites: true,
    CurvePreferences:         []tls.CurveID{tls.X25519, tls.CurveP256},
}
```

Then call `srv.ListenAndServeTLS(certFile, keyFile)` instead of `ListenAndServe`.

### 2.4 Request ID Generation

Use `crypto/rand` for unique, collision-resistant IDs:

```go
func GenerateRequestID() string {
    b := make([]byte, 12)
    _, _ = rand.Read(b) // crypto/rand never fails on modern OS
    return "req_" + hex.EncodeToString(b)
}
```

Inject via middleware as the first step in the chain. Propagate in `X-Request-ID` header to upstreams.

---

## 3. Configuration System

### 3.1 YAML Parser (Zero-Dependency)

Since we cannot import `gopkg.in/yaml.v3`, we implement a minimal YAML subset parser.

**Supported YAML features (minimal subset):**

- Key-value pairs (`key: value`)
- Nested maps (indentation-based)
- Lists (`- item`)
- String values (quoted and unquoted)
- Integer, float, boolean values
- Null values (`~` or empty)
- Comments (`# comment`)
- Multi-line strings (literal `|` and folded `>`)
- Environment variable interpolation (`${VAR}`, `${VAR:default}`)

**NOT supported (and not needed):**

- Anchors and aliases (`&`, `*`)
- Tags (`!!str`, `!!int`)
- Complex keys (multi-line keys)
- Flow collections (`{a: 1, b: 2}`, `[1, 2, 3]`)
- Multiple documents (`---`)

**Implementation approach:**

```
1. Read file into string
2. Perform env var interpolation (regex: \$\{([^}]+)\})
3. Tokenize: line-by-line, track indentation level
4. Build tree: each indentation increase creates a child map/list
5. Marshal tree into Config struct using reflection
```

**The parser outputs a `map[string]any` tree**, which is then mapped to the typed `Config` struct via a custom marshal function using `reflect`. This approach:
- Keeps the parser generic and testable
- Separates parsing from schema-specific validation
- Allows config validation to produce user-friendly error messages

**Estimated complexity:** ~600-800 lines for parser + marshaler. This is within the acceptable threshold for the zero-dep constraint.

### 3.2 Config Struct

```go
type Config struct {
    Server       ServerConfig       `yaml:"server"`
    Gateway      GatewayConfig      `yaml:"gateway"`
    Upstreams    []UpstreamConfig   `yaml:"upstreams"`
    Schema       SchemaConfig       `yaml:"schema"`
    MCP          MCPConfig          `yaml:"mcp"`
    Observability ObservabilityConfig `yaml:"observability"`
    Admin        AdminConfig        `yaml:"admin"`
}
```

Each sub-struct has a `Validate() error` method. Top-level `Config.Validate()` calls all sub-validators.

### 3.3 Environment Variable Interpolation

```go
var envVarRegex = regexp.MustCompile(`\$\{([^}]+)\}`)

func InterpolateEnv(input string) (string, error) {
    var missingVars []string
    result := envVarRegex.ReplaceAllStringFunc(input, func(match string) string {
        inner := match[2 : len(match)-1] // strip ${ and }
        parts := strings.SplitN(inner, ":", 2)
        name := parts[0]
        value := os.Getenv(name)
        if value == "" && len(parts) == 2 {
            return parts[1] // default value
        }
        if value == "" {
            missingVars = append(missingVars, name)
            return match // leave as-is for error reporting
        }
        return value
    })
    if len(missingVars) > 0 {
        return "", fmt.Errorf("missing required env vars: %s", strings.Join(missingVars, ", "))
    }
    return result, nil
}
```

### 3.4 Hot Reload

Since we can't use `fsnotify` (external dep), implement polling-based file watch:

```go
type ConfigWatcher struct {
    path     string
    interval time.Duration
    lastHash [32]byte // sha256 of file content
    onChange func(*Config)
}

func (w *ConfigWatcher) Run(ctx context.Context) {
    ticker := time.NewTicker(w.interval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            data, err := os.ReadFile(w.path)
            if err != nil {
                slog.Warn("config watch: read failed", "error", err)
                continue
            }
            hash := sha256.Sum256(data)
            if hash == w.lastHash {
                continue
            }
            cfg, err := ParseAndValidate(data)
            if err != nil {
                slog.Error("config watch: invalid config", "error", err)
                continue
            }
            w.lastHash = hash
            w.onChange(cfg)
        }
    }
}
```

The `onChange` callback atomically swaps the config pointer:

```go
type AtomicConfig struct {
    ptr atomic.Pointer[Config]
}

func (a *AtomicConfig) Load() *Config   { return a.ptr.Load() }
func (a *AtomicConfig) Store(cfg *Config) { a.ptr.Store(cfg) }
```

Default poll interval: 5 seconds. Configurable but not exposed to user config (compile-time constant).

---

## 4. Request Router

### 4.1 Request Classification

The router inspects the incoming request and determines which processing pipeline to use:

```go
type RequestType int

const (
    RequestTypeGraphQL RequestType = iota
    RequestTypeREST
    RequestTypeMCP
    RequestTypeHealth
    RequestTypeMetrics
    RequestTypeAdmin
    RequestTypeUnknown
)

func ClassifyRequest(r *http.Request) RequestType {
    path := r.URL.Path

    // Observability endpoints (bypass gateway)
    switch path {
    case "/-/health":
        return RequestTypeHealth
    case "/-/metrics":
        return RequestTypeMetrics
    }

    // Admin
    if strings.HasPrefix(path, "/-/admin") {
        return RequestTypeAdmin
    }

    // MCP
    if path == "/mcp" && r.Method == http.MethodPost {
        return RequestTypeMCP
    }

    // GraphQL — detect by path or content-type
    if path == "/graphql" {
        return RequestTypeGraphQL
    }

    // REST — everything under /api/ or custom prefix
    if strings.HasPrefix(path, "/api/") {
        return RequestTypeREST
    }

    return RequestTypeUnknown
}
```

### 4.2 Upstream Resolution

For REST requests, the router matches the path to an upstream:

```go
type UpstreamRoute struct {
    PathPrefix string          // e.g., "/api/users"
    Upstream   *UpstreamConfig
    StripPrefix string         // prefix to strip before forwarding
}

// Routes are sorted longest-prefix-first at config load time
type Router struct {
    routes []UpstreamRoute
}

func (r *Router) Resolve(path string) (*UpstreamRoute, bool) {
    for _, route := range r.routes {
        if strings.HasPrefix(path, route.PathPrefix) {
            return &route, true
        }
    }
    return nil, false
}
```

For GraphQL requests, all configured upstreams of type `graphql` and `rest` (with generated schema) are available. The schema engine determines which upstream handles which field.

---

## 5. Gateway Middleware Chain

### 5.1 Chain Architecture

```go
type Middleware func(next http.Handler) http.Handler

type Chain struct {
    middlewares []Middleware
}

func (c *Chain) Then(final http.Handler) http.Handler {
    h := final
    for i := len(c.middlewares) - 1; i >= 0; i-- {
        h = c.middlewares[i](h)
    }
    return h
}
```

**Fixed execution order:**

```go
chain := NewChain(
    RequestIDMiddleware,        // 1. Generate request ID
    LoggingMiddleware,          // 2. Request/response logging
    RecoveryMiddleware,         // 3. Panic recovery
    CORSMiddleware,             // 4. CORS headers
    AuthMiddleware,             // 5. Authentication
    RateLimitMiddleware,        // 6. Rate limiting
    CacheLookupMiddleware,      // 7. Cache check (short-circuits on hit)
    CircuitBreakerMiddleware,   // 8. Circuit breaker check
    // → proxy handler
    CacheStoreMiddleware,       // 9. Cache store (post-proxy, via response wrapper)
)
```

### 5.2 Response Capture

For cache storage, we need to capture the response body. Use a response wrapper:

```go
type ResponseCapture struct {
    http.ResponseWriter
    statusCode int
    body       bytes.Buffer
    written    bool
}

func (rc *ResponseCapture) WriteHeader(code int) {
    rc.statusCode = code
    rc.written = true
    rc.ResponseWriter.WriteHeader(code)
}

func (rc *ResponseCapture) Write(b []byte) (int, error) {
    rc.body.Write(b)
    return rc.ResponseWriter.Write(b)
}
```

### 5.3 JWT Validation (Zero-Dependency)

JWT is three base64url-encoded segments separated by dots. We only need to validate, not issue.

```go
func ValidateJWT(tokenString string, secret []byte, algorithms []string) (*Claims, error) {
    parts := strings.SplitN(tokenString, ".", 3)
    if len(parts) != 3 {
        return nil, fmt.Errorf("malformed token: expected 3 parts, got %d", len(parts))
    }

    // Decode header
    headerJSON, err := base64URLDecode(parts[0])
    if err != nil {
        return nil, fmt.Errorf("invalid header encoding: %w", err)
    }
    var header struct {
        Alg string `json:"alg"`
        Typ string `json:"typ"`
    }
    if err := json.Unmarshal(headerJSON, &header); err != nil {
        return nil, fmt.Errorf("invalid header JSON: %w", err)
    }

    // Verify algorithm is allowed
    if !slices.Contains(algorithms, header.Alg) {
        return nil, fmt.Errorf("algorithm %q not allowed", header.Alg)
    }

    // Verify signature
    signingInput := parts[0] + "." + parts[1]
    signature, err := base64URLDecode(parts[2])
    if err != nil {
        return nil, fmt.Errorf("invalid signature encoding: %w", err)
    }

    switch header.Alg {
    case "HS256":
        mac := hmac.New(sha256.New, secret)
        mac.Write([]byte(signingInput))
        expected := mac.Sum(nil)
        if !hmac.Equal(signature, expected) {
            return nil, errors.New("invalid signature")
        }
    case "HS384":
        mac := hmac.New(sha512.New384, secret)
        mac.Write([]byte(signingInput))
        expected := mac.Sum(nil)
        if !hmac.Equal(signature, expected) {
            return nil, errors.New("invalid signature")
        }
    case "RS256":
        // Parse RSA public key from PEM, verify with crypto/rsa
        return nil, errors.New("RS256 not yet implemented")
    default:
        return nil, fmt.Errorf("unsupported algorithm: %s", header.Alg)
    }

    // Decode and validate claims
    claimsJSON, err := base64URLDecode(parts[1])
    if err != nil {
        return nil, fmt.Errorf("invalid claims encoding: %w", err)
    }
    var claims Claims
    if err := json.Unmarshal(claimsJSON, &claims); err != nil {
        return nil, fmt.Errorf("invalid claims JSON: %w", err)
    }

    // Check expiration
    now := time.Now().Unix()
    if claims.Exp > 0 && now > claims.Exp {
        return nil, errors.New("token expired")
    }
    if claims.Nbf > 0 && now < claims.Nbf {
        return nil, errors.New("token not yet valid")
    }

    return &claims, nil
}

type Claims struct {
    Sub    string         `json:"sub"`
    Iss    string         `json:"iss"`
    Aud    json.RawMessage `json:"aud"` // can be string or []string
    Exp    int64          `json:"exp"`
    Nbf    int64          `json:"nbf"`
    Iat    int64          `json:"iat"`
    Extra  map[string]any `json:"-"`    // catch-all for custom claims
}
```

`base64URLDecode` handles the URL-safe variant (no padding):

```go
func base64URLDecode(s string) ([]byte, error) {
    // Add padding if necessary
    switch len(s) % 4 {
    case 2:
        s += "=="
    case 3:
        s += "="
    }
    return base64.URLEncoding.DecodeString(s)
}
```

---

## 6. OpenAPI Parser

### 6.1 Parsing Strategy

OpenAPI 3.x specs are JSON or YAML documents. Since we already have a YAML parser (section 3), we parse to `map[string]any` and then extract the relevant structures.

**We only parse what Tentaserve needs:**

```go
type OpenAPISpec struct {
    Info    OpenAPIInfo
    Paths   map[string]PathItem // path → methods
    Components ComponentsObject
}

type PathItem struct {
    Get     *Operation
    Post    *Operation
    Put     *Operation
    Patch   *Operation
    Delete  *Operation
    Parameters []Parameter // shared across methods
}

type Operation struct {
    OperationID string
    Summary     string
    Description string
    Parameters  []Parameter
    RequestBody *RequestBody
    Responses   map[string]Response // status code → response
    Tags        []string
    Deprecated  bool
}

type SchemaObject struct {
    Type       string            // string, integer, number, boolean, array, object
    Format     string            // date-time, uuid, email, int64, etc.
    Properties map[string]SchemaObject
    Items      *SchemaObject     // for arrays
    Required   []string
    Enum       []string
    OneOf      []SchemaObject
    AllOf      []SchemaObject
    AnyOf      []SchemaObject
    Ref        string            // $ref pointer
    Nullable   bool
    Default    any
    Example    any
    Description string
}
```

### 6.2 $ref Resolution

OpenAPI specs heavily use `$ref` for schema reuse. Implement a single-pass resolver:

```go
func (p *Parser) ResolveRef(ref string) (*SchemaObject, error) {
    // ref format: "#/components/schemas/User"
    if !strings.HasPrefix(ref, "#/") {
        return nil, fmt.Errorf("external refs not supported: %s", ref)
    }
    path := strings.Split(ref[2:], "/") // ["components", "schemas", "User"]
    node := p.root // the raw map[string]any from YAML parse
    for _, segment := range path {
        m, ok := node.(map[string]any)
        if !ok {
            return nil, fmt.Errorf("ref path broken at %s in %s", segment, ref)
        }
        node, ok = m[segment]
        if !ok {
            return nil, fmt.Errorf("ref not found: %s", ref)
        }
    }
    return p.parseSchemaObject(node)
}
```

**Circular ref detection:** Track visited refs in a `map[string]bool`. If revisited, emit the type name as a reference instead of inlining.

### 6.3 Loader

Support three schema sources:

```go
func LoadOpenAPISpec(source string) ([]byte, error) {
    switch {
    case strings.HasPrefix(source, "file://"):
        return os.ReadFile(strings.TrimPrefix(source, "file://"))
    case strings.HasPrefix(source, "http://"), strings.HasPrefix(source, "https://"):
        resp, err := http.Get(source)
        if err != nil {
            return nil, err
        }
        defer resp.Body.Close()
        return io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB max
    default:
        // Inline YAML string
        return []byte(source), nil
    }
}
```

---

## 7. GraphQL Parser & Executor

### 7.1 Lexer

The GraphQL lexer tokenizes the input query string. Implement as a state machine:

**Token types:**

```go
type TokenKind int

const (
    TokenEOF TokenKind = iota
    TokenName           // field names, type names
    TokenInt            // integer literal
    TokenFloat          // float literal
    TokenString         // "string literal"
    TokenBlockString    // """block string"""
    TokenBang           // !
    TokenDollar         // $
    TokenAmpersand      // &
    TokenParenL         // (
    TokenParenR         // )
    TokenBracketL       // [
    TokenBracketR       // ]
    TokenBraceL         // {
    TokenBraceR         // }
    TokenColon          // :
    TokenEquals         // =
    TokenAt             // @
    TokenPipe           // |
    TokenSpread         // ...
)
```

**Implementation:** Single-pass, byte-by-byte scan. No regex. ~200 lines.

### 7.2 Parser

Recursive descent parser producing an AST:

```go
type Document struct {
    Definitions []Definition
}

type Definition interface{ isDefinition() }

type OperationDefinition struct {
    Operation    OperationType // Query, Mutation, Subscription
    Name         string
    Variables    []VariableDefinition
    Directives   []Directive
    SelectionSet SelectionSet
}

type SelectionSet struct {
    Selections []Selection
}

type Selection interface{ isSelection() }

type Field struct {
    Alias        string
    Name         string
    Arguments    []Argument
    Directives   []Directive
    SelectionSet *SelectionSet
}

type FragmentSpread struct {
    Name       string
    Directives []Directive
}

type InlineFragment struct {
    TypeCondition string
    Directives    []Directive
    SelectionSet  SelectionSet
}
```

**Parser structure:** Each AST node type has a `parse*` method:

```
parseDocument → parseDefinition → parseOperationDefinition → parseSelectionSet → parseSelection → parseField
```

### 7.3 Query Validation

Before execution, validate the parsed query:

**Depth check:**

```go
func CheckDepth(selSet *SelectionSet, maxDepth, currentDepth int) error {
    if currentDepth > maxDepth {
        return fmt.Errorf("%w: depth %d exceeds max %d", ErrQueryTooDeep, currentDepth, maxDepth)
    }
    for _, sel := range selSet.Selections {
        if field, ok := sel.(*Field); ok && field.SelectionSet != nil {
            if err := CheckDepth(field.SelectionSet, maxDepth, currentDepth+1); err != nil {
                return err
            }
        }
    }
    return nil
}
```

**Complexity check:**

Each field scores 1 point. List fields score `limit * childComplexity`. Total must not exceed `maxComplexity` (default: 1000).

```go
func CalculateComplexity(selSet *SelectionSet, schema *SchemaDefinition, parentType string) int {
    total := 0
    for _, sel := range selSet.Selections {
        field, ok := sel.(*Field)
        if !ok {
            continue
        }
        cost := 1
        fieldDef := schema.GetField(parentType, field.Name)
        if fieldDef != nil && fieldDef.Type.IsList() {
            limit := extractLimitArg(field.Arguments, 20) // default page size
            childCost := 0
            if field.SelectionSet != nil {
                childCost = CalculateComplexity(field.SelectionSet, schema, fieldDef.Type.ElementName())
            }
            cost = limit * max(childCost, 1)
        } else if field.SelectionSet != nil {
            cost += CalculateComplexity(field.SelectionSet, schema, fieldDef.Type.Name)
        }
        total += cost
    }
    return total
}
```

### 7.4 Executor

The GraphQL executor walks the AST and resolves each field:

```go
type Executor struct {
    schema    *SchemaDefinition
    resolvers map[string]FieldResolver // "TypeName.fieldName" → resolver
}

type FieldResolver func(ctx context.Context, parent any, args map[string]any) (any, error)

func (e *Executor) Execute(ctx context.Context, doc *Document, vars map[string]any) (*ExecutionResult, error) {
    op := doc.Definitions[0].(*OperationDefinition) // simplified — handle named ops too
    rootType := e.schema.QueryType
    if op.Operation == OperationMutation {
        rootType = e.schema.MutationType
    }
    data, errs := e.resolveSelectionSet(ctx, op.SelectionSet, rootType, nil)
    return &ExecutionResult{Data: data, Errors: errs}, nil
}

func (e *Executor) resolveSelectionSet(ctx context.Context, selSet SelectionSet, typeName string, parent any) (map[string]any, []GraphQLError) {
    result := make(map[string]any)
    var errs []GraphQLError
    for _, sel := range selSet.Selections {
        field := sel.(*Field)
        key := field.Alias
        if key == "" {
            key = field.Name
        }
        resolverKey := typeName + "." + field.Name
        resolver, ok := e.resolvers[resolverKey]
        if !ok {
            // Default resolver: extract field from parent map
            resolver = defaultResolver(field.Name)
        }
        value, err := resolver(ctx, parent, extractArgs(field.Arguments))
        if err != nil {
            errs = append(errs, newGraphQLError(err, field))
            result[key] = nil
            continue
        }
        // Recurse for object/list types with sub-selections
        if field.SelectionSet != nil && value != nil {
            fieldType := e.schema.GetField(typeName, field.Name).Type
            value, subErrs := e.resolveNested(ctx, field.SelectionSet, fieldType, value)
            errs = append(errs, subErrs...)
            result[key] = value
        } else {
            result[key] = value
        }
    }
    return result, errs
}
```

### 7.5 Introspection

For connecting to upstream GraphQL services, implement the standard introspection query:

```go
const IntrospectionQuery = `
query IntrospectionQuery {
    __schema {
        queryType { name }
        mutationType { name }
        subscriptionType { name }
        types { ...FullType }
        directives { name description locations args { ...InputValue } }
    }
}

fragment FullType on __Type {
    kind name description
    fields(includeDeprecated: true) { name description args { ...InputValue } type { ...TypeRef } isDeprecated deprecationReason }
    inputFields { ...InputValue }
    interfaces { ...TypeRef }
    enumValues(includeDeprecated: true) { name description isDeprecated deprecationReason }
    possibleTypes { ...TypeRef }
}

fragment InputValue on __InputValue { name description type { ...TypeRef } defaultValue }
fragment TypeRef on __Type { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name } } } } } } } }
`
```

Parse the introspection result into `SchemaDefinition` used by the schema engine.

---

## 8. REST → GraphQL Translation

### 8.1 Schema Building Pipeline

```
OpenAPI Spec (YAML/JSON)
  → Parse to OpenAPISpec struct
  → Resolve all $refs
  → Walk paths: for each path+method, create Operation
  → Walk schemas: for each component schema, create GraphQL Type
  → Detect relationships (parent-child URL patterns)
  → Build SchemaDefinition with types + queries + mutations
  → Register resolvers (each resolver knows its upstream + method + path)
```

### 8.2 Resolver Generation

For each OpenAPI operation, generate a resolver function:

```go
func BuildRESTResolver(upstream *UpstreamConfig, method, pathTemplate string, params []Parameter) FieldResolver {
    return func(ctx context.Context, parent any, args map[string]any) (any, error) {
        // Build URL from path template + args
        url := upstream.URL + interpolatePath(pathTemplate, args)

        // Add query parameters
        query := buildQueryParams(params, args)
        if len(query) > 0 {
            url += "?" + query.Encode()
        }

        // Build request
        var body io.Reader
        if method != http.MethodGet {
            if input, ok := args["input"]; ok {
                bodyBytes, _ := json.Marshal(input)
                body = bytes.NewReader(bodyBytes)
            }
        }

        req, _ := http.NewRequestWithContext(ctx, method, url, body)
        if body != nil {
            req.Header.Set("Content-Type", "application/json")
        }

        // Forward auth headers from context
        forwardHeaders(ctx, req)

        // Dispatch via upstream client (handles retry, circuit breaker)
        resp, err := upstream.Client.Do(req)
        if err != nil {
            return nil, err
        }
        defer resp.Body.Close()

        // Parse response
        var result any
        if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
            return nil, fmt.Errorf("decode upstream response: %w", err)
        }

        return result, nil
    }
}
```

### 8.3 Relationship Detection

Scan path patterns for parent-child relationships:

```go
// Input paths:
//   /users/{userId}
//   /users/{userId}/posts
//   /users/{userId}/posts/{postId}
//   /users/{userId}/posts/{postId}/comments

// Detection algorithm:
// 1. For each path, extract resource segments (non-parameter parts)
// 2. If path A is a prefix of path B, and B has exactly one more resource segment,
//    then B's resource is a child of A's resource type

func DetectRelationships(paths map[string]PathItem) []Relationship {
    var rels []Relationship
    pathList := sortedKeys(paths)
    for i, parent := range pathList {
        for _, child := range pathList[i+1:] {
            if isChildPath(parent, child) {
                rels = append(rels, Relationship{
                    ParentPath:    parent,
                    ChildPath:     child,
                    ParentType:    extractResourceType(parent), // "User"
                    ChildType:     extractResourceType(child),  // "Post"
                    ChildField:    extractResourceName(child),  // "posts"
                    ForeignKey:    extractParamName(parent),    // "userId"
                })
            }
        }
    }
    return rels
}
```

These relationships add fields to parent GraphQL types and register nested resolvers.

---

## 9. GraphQL → REST Translation

### 9.1 Endpoint Generation

Walk the introspected schema and generate REST endpoints:

```go
type GeneratedEndpoint struct {
    Method      string // GET, POST, PUT, DELETE
    Path        string // /api/user, /api/create-user
    GraphQLOp   string // query or mutation
    FieldName   string // the GraphQL field name
    Arguments   []ArgumentDef
    ReturnType  *TypeDef
}

func GenerateEndpoints(schema *SchemaDefinition) []GeneratedEndpoint {
    var endpoints []GeneratedEndpoint

    // Queries → GET
    if schema.QueryType != "" {
        queryType := schema.GetType(schema.QueryType)
        for _, field := range queryType.Fields {
            endpoints = append(endpoints, GeneratedEndpoint{
                Method:    http.MethodGet,
                Path:      "/api/" + toKebabCase(field.Name),
                GraphQLOp: "query",
                FieldName: field.Name,
                Arguments: field.Arguments,
                ReturnType: &field.Type,
            })
        }
    }

    // Mutations → POST/PUT/DELETE based on naming heuristic
    if schema.MutationType != "" {
        mutType := schema.GetType(schema.MutationType)
        for _, field := range mutType.Fields {
            method := inferHTTPMethod(field.Name)
            endpoints = append(endpoints, GeneratedEndpoint{
                Method:    method,
                Path:      "/api/" + toKebabCase(field.Name),
                GraphQLOp: "mutation",
                FieldName: field.Name,
                Arguments: field.Arguments,
                ReturnType: &field.Type,
            })
        }
    }

    return endpoints
}
```

### 9.2 Request Translation

When a REST request arrives for a generated endpoint:

```go
func TranslateRESTToGraphQL(endpoint GeneratedEndpoint, r *http.Request) (string, map[string]any, error) {
    vars := make(map[string]any)

    // Extract arguments from query params (GET) or body (POST/PUT)
    if r.Method == http.MethodGet {
        for _, arg := range endpoint.Arguments {
            if val := r.URL.Query().Get(arg.Name); val != "" {
                vars[arg.Name] = coerceValue(val, arg.Type)
            }
        }
    } else {
        var body map[string]any
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            return "", nil, fmt.Errorf("invalid request body: %w", err)
        }
        vars = body
    }

    // Handle field selection via ?fields= param
    fieldSelection := r.URL.Query().Get("fields")
    selectionSet := buildSelectionSet(endpoint.ReturnType, fieldSelection)

    // Build GraphQL query
    query := buildGraphQLQuery(endpoint.GraphQLOp, endpoint.FieldName, vars, selectionSet)

    return query, vars, nil
}
```

### 9.3 Response Unwrapping

Strip the GraphQL `data` wrapper before sending REST response:

```go
func UnwrapGraphQLResponse(gqlResp *ExecutionResult, fieldName string) (any, int, error) {
    if len(gqlResp.Errors) > 0 {
        statusCode := inferStatusFromErrors(gqlResp.Errors)
        return map[string]any{
            "error":   gqlResp.Errors[0].Message,
            "details": gqlResp.Errors,
        }, statusCode, nil
    }

    data, ok := gqlResp.Data.(map[string]any)
    if !ok {
        return nil, 500, errors.New("unexpected response format")
    }

    return data[fieldName], 200, nil
}
```

### 9.4 Subscription → SSE Bridge

For GraphQL subscriptions exposed as REST:

```go
func HandleSSESubscription(w http.ResponseWriter, r *http.Request, upstream *UpstreamClient, subQuery string) {
    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "streaming not supported", http.StatusInternalServerError)
        return
    }

    // Connect to upstream WebSocket for subscription
    ctx := r.Context()
    events, err := upstream.Subscribe(ctx, subQuery)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadGateway)
        return
    }

    for {
        select {
        case <-ctx.Done():
            return
        case event, ok := <-events:
            if !ok {
                return
            }
            data, _ := json.Marshal(event)
            fmt.Fprintf(w, "data: %s\n\n", data)
            flusher.Flush()
        }
    }
}
```

---

## 10. Schema Engine & DataLoader

### 10.1 Unified Type Registry

The schema engine maintains a central registry of all types from all upstreams:

```go
type TypeRegistry struct {
    types      map[string]*TypeDef      // name → type
    operations map[string]*OperationDef // "Query.user" → operation
    sources    map[string]string        // "User" → "users-api" (which upstream owns it)
    mu         sync.RWMutex
}

func (tr *TypeRegistry) Register(upstream string, types []*TypeDef) {
    tr.mu.Lock()
    defer tr.mu.Unlock()
    for _, t := range types {
        tr.types[t.Name] = t
        tr.sources[t.Name] = upstream
    }
}

func (tr *TypeRegistry) GetType(name string) *TypeDef {
    tr.mu.RLock()
    defer tr.mu.RUnlock()
    return tr.types[name]
}
```

### 10.2 DataLoader Implementation

Per-request DataLoader that batches calls within a time window:

```go
type DataLoader struct {
    batchFn     func(ctx context.Context, keys []string) (map[string]any, error)
    batchWindow time.Duration
    maxBatch    int

    mu      sync.Mutex
    pending map[string][]chan result
    timer   *time.Timer
}

type result struct {
    value any
    err   error
}

func (dl *DataLoader) Load(ctx context.Context, key string) (any, error) {
    ch := make(chan result, 1)

    dl.mu.Lock()
    if dl.pending == nil {
        dl.pending = make(map[string][]chan result)
    }
    dl.pending[key] = append(dl.pending[key], ch)

    // Start batch timer on first load
    if dl.timer == nil {
        dl.timer = time.AfterFunc(dl.batchWindow, func() {
            dl.dispatch(ctx)
        })
    }

    // Dispatch immediately if max batch size reached
    totalPending := 0
    for _, chs := range dl.pending {
        totalPending += len(chs)
    }
    if totalPending >= dl.maxBatch {
        dl.timer.Stop()
        go dl.dispatch(ctx)
    }
    dl.mu.Unlock()

    // Wait for result
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    case r := <-ch:
        return r.value, r.err
    }
}

func (dl *DataLoader) dispatch(ctx context.Context) {
    dl.mu.Lock()
    pending := dl.pending
    dl.pending = nil
    dl.timer = nil
    dl.mu.Unlock()

    keys := make([]string, 0, len(pending))
    for k := range pending {
        keys = append(keys, k)
    }

    results, err := dl.batchFn(ctx, keys)

    for key, channels := range pending {
        var r result
        if err != nil {
            r = result{err: err}
        } else if val, ok := results[key]; ok {
            r = result{value: val}
        } else {
            r = result{err: fmt.Errorf("key %q not found in batch result", key)}
        }
        for _, ch := range channels {
            ch <- r
        }
    }
}
```

**Per-request scoping:** Each incoming GraphQL request gets its own DataLoader instance (stored in context). This prevents cross-request batching and ensures proper isolation.

### 10.3 Query Planner

For queries spanning multiple upstreams:

```go
type QueryPlan struct {
    Steps []PlanStep
}

type PlanStep struct {
    StepID      int
    Upstream    string
    Operation   string          // GraphQL query or REST call
    DependsOn   []int           // step IDs that must complete first
    ExtractKeys string          // field to extract as keys for next step
    InjectAs    string          // how to inject results into next step
}

func BuildQueryPlan(selSet SelectionSet, registry *TypeRegistry) *QueryPlan {
    // Topological sort of field dependencies:
    // 1. Root fields → no dependencies, group by upstream
    // 2. Nested fields → depend on parent step, group by upstream
    // 3. Steps at same depth with different upstreams → parallel
    // 4. Steps at same depth with same upstream → batch
    // ...
}
```

---

## 11. MCP Server

### 11.1 JSON-RPC Transport

MCP uses JSON-RPC 2.0 over HTTP with SSE for server-to-client streaming:

```go
type JSONRPCRequest struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      any             `json:"id"`
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      any             `json:"id"`
    Result  any             `json:"result,omitempty"`
    Error   *JSONRPCError   `json:"error,omitempty"`
}

type JSONRPCError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Data    any    `json:"data,omitempty"`
}
```

### 11.2 Handler Dispatch

```go
type MCPServer struct {
    toolRegistry *ToolRegistry
    serverInfo   ServerInfo
}

func (s *MCPServer) Handle(w http.ResponseWriter, r *http.Request) {
    var req JSONRPCRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSONRPCError(w, nil, -32700, "Parse error")
        return
    }

    var result any
    var err error

    switch req.Method {
    case "initialize":
        result, err = s.handleInitialize(req.Params)
    case "tools/list":
        result, err = s.handleToolsList(req.Params)
    case "tools/call":
        result, err = s.handleToolsCall(r.Context(), req.Params)
    case "resources/list":
        result, err = s.handleResourcesList(req.Params)
    case "resources/read":
        result, err = s.handleResourcesRead(req.Params)
    default:
        writeJSONRPCError(w, req.ID, -32601, "Method not found")
        return
    }

    if err != nil {
        writeJSONRPCError(w, req.ID, -32000, err.Error())
        return
    }

    writeJSONRPCResult(w, req.ID, result)
}
```

### 11.3 Tool Registry

```go
type ToolRegistry struct {
    tools map[string]*Tool
    mu    sync.RWMutex
}

type Tool struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    InputSchema json.RawMessage `json:"inputSchema"`
    Handler     ToolHandler     `json:"-"`
    Upstream    string          `json:"-"`
    Operation   string          `json:"-"` // internal operation ref
}

type ToolHandler func(ctx context.Context, args map[string]any) (any, error)

func (tr *ToolRegistry) BuildFromSchema(registry *TypeRegistry, upstreams []*UpstreamConfig) {
    tr.mu.Lock()
    defer tr.mu.Unlock()
    tr.tools = make(map[string]*Tool)

    for _, upstream := range upstreams {
        ops := registry.GetOperations(upstream.Name)
        for _, op := range ops {
            tool := &Tool{
                Name:        generateToolName(upstream.Name, op),
                Description: generateToolDescription(upstream.Name, op),
                InputSchema: generateInputSchema(op),
                Handler:     buildToolHandler(upstream, op),
                Upstream:    upstream.Name,
                Operation:   op.Name,
            }
            tr.tools[tool.Name] = tool
        }
    }
}
```

### 11.4 Tool Name Generation

```go
func generateToolName(upstreamName string, op *OperationDef) string {
    // Priority 1: operationId from OpenAPI
    if op.OperationID != "" {
        return sanitizeName(op.OperationID)
    }
    // Priority 2: method + path segments
    name := toSnakeCase(op.Method + "_" + pathToName(op.Path))
    return sanitizeName(name)
}

func sanitizeName(name string) string {
    // Lowercase, replace non-alphanum with _, trim, max 64 chars
    result := strings.Map(func(r rune) rune {
        if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' {
            return r
        }
        if r >= 'A' && r <= 'Z' {
            return r + 32 // lowercase
        }
        return '_'
    }, name)
    // Collapse multiple underscores
    for strings.Contains(result, "__") {
        result = strings.ReplaceAll(result, "__", "_")
    }
    result = strings.Trim(result, "_")
    if len(result) > 64 {
        result = result[:64]
    }
    return result
}
```

### 11.5 Input Schema Generation

Convert operation parameters to JSON Schema:

```go
func generateInputSchema(op *OperationDef) json.RawMessage {
    schema := map[string]any{
        "type":       "object",
        "properties": map[string]any{},
    }
    props := schema["properties"].(map[string]any)
    var required []string

    for _, arg := range op.Arguments {
        prop := map[string]any{
            "type": graphQLTypeToJSONSchemaType(arg.Type),
        }
        if arg.Description != "" {
            prop["description"] = arg.Description
        }
        if arg.Default != nil {
            prop["default"] = arg.Default
        }
        props[arg.Name] = prop
        if arg.Required {
            required = append(required, arg.Name)
        }
    }

    if len(required) > 0 {
        schema["required"] = required
    }

    data, _ := json.Marshal(schema)
    return data
}
```

---

## 12. Upstream Client

### 12.1 Connection Pooling

Each upstream gets its own `*http.Client` with tuned transport:

```go
func NewUpstreamClient(cfg *UpstreamConfig) *http.Client {
    transport := &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 20,
        MaxConnsPerHost:     50,
        IdleConnTimeout:     90 * time.Second,
        TLSHandshakeTimeout: 10 * time.Second,
        DisableCompression:  false,
        ForceAttemptHTTP2:   true,
    }

    return &http.Client{
        Transport: transport,
        Timeout:   cfg.Timeout,
    }
}
```

### 12.2 Retry Logic

Exponential backoff with jitter:

```go
func (u *UpstreamClient) DoWithRetry(ctx context.Context, req *http.Request, maxRetries int, baseDelay time.Duration) (*http.Response, error) {
    var lastErr error
    for attempt := 0; attempt <= maxRetries; attempt++ {
        if attempt > 0 {
            delay := baseDelay * time.Duration(1<<(attempt-1))
            jitter := time.Duration(rand.Int63n(int64(delay / 2)))
            delay += jitter

            select {
            case <-ctx.Done():
                return nil, ctx.Err()
            case <-time.After(delay):
            }

            // Clone request for retry (body needs re-reading)
            req = req.Clone(ctx)
        }

        resp, err := u.client.Do(req)
        if err != nil {
            lastErr = err
            continue
        }

        // Only retry on specific status codes
        if resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
            resp.Body.Close()
            lastErr = fmt.Errorf("upstream returned %d", resp.StatusCode)
            continue
        }

        return resp, nil
    }

    return nil, fmt.Errorf("all %d retries exhausted: %w", maxRetries, lastErr)
}
```

### 12.3 Health Checker

Background goroutine that periodically checks upstream health:

```go
type HealthChecker struct {
    upstreams []*UpstreamConfig
    status    sync.Map // upstream name → HealthStatus
}

type HealthStatus struct {
    Healthy   bool
    LatencyMs int64
    LastCheck time.Time
    Error     string
}

func (hc *HealthChecker) Run(ctx context.Context) {
    for _, u := range hc.upstreams {
        if u.Health.Path == "" {
            continue
        }
        go hc.checkLoop(ctx, u)
    }
}

func (hc *HealthChecker) checkLoop(ctx context.Context, u *UpstreamConfig) {
    ticker := time.NewTicker(u.Health.Interval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            start := time.Now()
            checkCtx, cancel := context.WithTimeout(ctx, u.Health.Timeout)
            req, _ := http.NewRequestWithContext(checkCtx, http.MethodGet, u.URL+u.Health.Path, nil)
            resp, err := u.Client.Do(req)
            cancel()

            status := HealthStatus{
                LastCheck: time.Now(),
                LatencyMs: time.Since(start).Milliseconds(),
            }
            if err != nil {
                status.Healthy = false
                status.Error = err.Error()
            } else {
                resp.Body.Close()
                status.Healthy = resp.StatusCode >= 200 && resp.StatusCode < 300
                if !status.Healthy {
                    status.Error = fmt.Sprintf("status %d", resp.StatusCode)
                }
            }
            hc.status.Store(u.Name, status)
        }
    }
}
```

---

## 13. Caching Layer

### 13.1 Sharded LRU Cache

16 shards to reduce lock contention:

```go
const numShards = 16

type ShardedCache struct {
    shards [numShards]*cacheShard
    maxMem int64
}

type cacheShard struct {
    mu       sync.RWMutex
    items    map[string]*cacheEntry
    order    *list.List // doubly-linked list for LRU ordering
    maxItems int
}

type cacheEntry struct {
    key       string
    value     []byte
    expiresAt time.Time
    element   *list.Element
    size      int
}

func (sc *ShardedCache) shardFor(key string) *cacheShard {
    h := fnv32a(key)
    return sc.shards[h%numShards]
}

func fnv32a(s string) uint32 {
    h := uint32(2166136261)
    for i := 0; i < len(s); i++ {
        h ^= uint32(s[i])
        h *= 16777619
    }
    return h
}
```

### 13.2 Cache Key Derivation

```go
func BuildCacheKey(r *http.Request, varyHeaders []string) string {
    h := sha256.New()
    h.Write([]byte(r.Method))
    h.Write([]byte(r.URL.Path))

    // Sorted query parameters for consistency
    params := r.URL.Query()
    keys := make([]string, 0, len(params))
    for k := range params {
        keys = append(keys, k)
    }
    sort.Strings(keys)
    for _, k := range keys {
        h.Write([]byte(k))
        h.Write([]byte(strings.Join(params[k], ",")))
    }

    // Vary headers
    for _, header := range varyHeaders {
        h.Write([]byte(r.Header.Get(header)))
    }

    // Body hash for POST requests (GraphQL)
    if r.Method == http.MethodPost && r.Body != nil {
        body, _ := io.ReadAll(r.Body)
        r.Body = io.NopCloser(bytes.NewReader(body)) // re-wrap for downstream
        h.Write(body)
    }

    return hex.EncodeToString(h.Sum(nil))
}
```

### 13.3 Stale-While-Revalidate

```go
func (s *cacheShard) Get(key string) ([]byte, CacheStatus) {
    s.mu.RLock()
    entry, ok := s.items[key]
    s.mu.RUnlock()

    if !ok {
        return nil, CacheMiss
    }

    now := time.Now()
    if now.Before(entry.expiresAt) {
        s.promote(key) // move to front of LRU
        return entry.value, CacheHit
    }

    // Stale — return value but signal revalidation needed
    if now.Before(entry.expiresAt.Add(entry.staleWindow)) {
        s.promote(key)
        return entry.value, CacheStale
    }

    // Expired beyond stale window
    s.mu.Lock()
    s.evict(key)
    s.mu.Unlock()
    return nil, CacheMiss
}
```

On `CacheStale`, the middleware serves the stale response immediately and dispatches a background goroutine to refresh the cache entry.

---

## 14. Rate Limiter

### 14.1 Lock-Free Token Bucket

```go
type TokenBucket struct {
    tokens    atomic.Int64
    maxTokens int64
    refillRate int64         // tokens per second
    lastRefill atomic.Int64  // unix nano
}

func (tb *TokenBucket) Allow() bool {
    now := time.Now().UnixNano()
    last := tb.lastRefill.Load()
    elapsed := now - last
    if elapsed <= 0 {
        elapsed = 0
    }

    // Calculate tokens to add
    tokensToAdd := (elapsed * tb.refillRate) / int64(time.Second)
    if tokensToAdd > 0 {
        // Try to update last refill time
        if tb.lastRefill.CompareAndSwap(last, now) {
            current := tb.tokens.Add(tokensToAdd)
            if current > tb.maxTokens {
                tb.tokens.Store(tb.maxTokens)
            }
        }
    }

    // Try to consume one token
    for {
        current := tb.tokens.Load()
        if current <= 0 {
            return false
        }
        if tb.tokens.CompareAndSwap(current, current-1) {
            return true
        }
    }
}
```

### 14.2 Per-Client Store

```go
type RateLimitStore struct {
    buckets sync.Map // key → *TokenBucket
    config  RateLimitConfig
}

func (s *RateLimitStore) GetBucket(key string) *TokenBucket {
    if v, ok := s.buckets.Load(key); ok {
        return v.(*TokenBucket)
    }
    bucket := NewTokenBucket(s.config.Requests, s.config.Window)
    actual, _ := s.buckets.LoadOrStore(key, bucket)
    return actual.(*TokenBucket)
}

func (s *RateLimitStore) DeriveKey(r *http.Request) string {
    switch s.config.Scope {
    case "global":
        return "global"
    case "per-ip":
        ip, _, _ := net.SplitHostPort(r.RemoteAddr)
        return "ip:" + ip
    case "per-header":
        return "hdr:" + r.Header.Get(s.config.ScopeHeader)
    case "per-path":
        return "path:" + r.URL.Path
    default:
        return "global"
    }
}
```

### 14.3 Cleanup

Stale buckets (no requests in 2× window) are cleaned up periodically:

```go
func (s *RateLimitStore) Cleanup(ctx context.Context) {
    ticker := time.NewTicker(s.config.Window * 2)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            threshold := time.Now().Add(-s.config.Window * 2).UnixNano()
            s.buckets.Range(func(key, value any) bool {
                bucket := value.(*TokenBucket)
                if bucket.lastRefill.Load() < threshold {
                    s.buckets.Delete(key)
                }
                return true
            })
        }
    }
}
```

---

## 15. Circuit Breaker

### 15.1 State Machine

```go
type CircuitState int32

const (
    CircuitClosed   CircuitState = iota
    CircuitOpen
    CircuitHalfOpen
)

type CircuitBreaker struct {
    state            atomic.Int32
    failureCount     atomic.Int64
    successCount     atomic.Int64
    lastFailureTime  atomic.Int64
    config           CircuitBreakerConfig
}

func (cb *CircuitBreaker) Allow() error {
    state := CircuitState(cb.state.Load())

    switch state {
    case CircuitClosed:
        return nil
    case CircuitOpen:
        // Check if timeout has elapsed
        lastFail := time.Unix(0, cb.lastFailureTime.Load())
        if time.Since(lastFail) > cb.config.Timeout {
            // Transition to half-open
            cb.state.CompareAndSwap(int32(CircuitOpen), int32(CircuitHalfOpen))
            cb.successCount.Store(0)
            return nil
        }
        return ErrCircuitOpen
    case CircuitHalfOpen:
        return nil // allow probe requests
    }

    return nil
}

func (cb *CircuitBreaker) RecordSuccess() {
    state := CircuitState(cb.state.Load())
    switch state {
    case CircuitHalfOpen:
        count := cb.successCount.Add(1)
        if count >= int64(cb.config.SuccessThreshold) {
            cb.state.Store(int32(CircuitClosed))
            cb.failureCount.Store(0)
        }
    case CircuitClosed:
        cb.failureCount.Store(0) // reset consecutive failures
    }
}

func (cb *CircuitBreaker) RecordFailure() {
    cb.lastFailureTime.Store(time.Now().UnixNano())
    state := CircuitState(cb.state.Load())

    switch state {
    case CircuitClosed:
        count := cb.failureCount.Add(1)
        if count >= int64(cb.config.FailureThreshold) {
            cb.state.Store(int32(CircuitOpen))
        }
    case CircuitHalfOpen:
        cb.state.Store(int32(CircuitOpen))
        cb.failureCount.Store(0)
    }
}
```

---

## 16. Observability

### 16.1 Prometheus Metrics (Zero-Dep)

Emit Prometheus text exposition format without the client library:

```go
type MetricsRegistry struct {
    counters   sync.Map // name → *Counter
    histograms sync.Map // name → *Histogram
    gauges     sync.Map // name → *Gauge
}

type Counter struct {
    values sync.Map // label-key → atomic.Int64
}

func (c *Counter) Inc(labels map[string]string) {
    key := labelsToKey(labels)
    v, _ := c.values.LoadOrStore(key, &atomic.Int64{})
    v.(*atomic.Int64).Add(1)
}

// Serialize to Prometheus text format
func (mr *MetricsRegistry) WriteTo(w io.Writer) {
    mr.counters.Range(func(name, value any) bool {
        counter := value.(*Counter)
        fmt.Fprintf(w, "# TYPE %s counter\n", name)
        counter.values.Range(func(labelKey, val any) bool {
            labels := keyToLabels(labelKey.(string))
            fmt.Fprintf(w, "%s{%s} %d\n", name, labels, val.(*atomic.Int64).Load())
            return true
        })
        return true
    })
    // Similar for histograms and gauges...
}
```

### 16.2 Histogram Implementation

Use pre-defined buckets (same defaults as Prometheus client):

```go
type Histogram struct {
    buckets    []float64           // upper bounds
    counts     []atomic.Int64      // count per bucket
    sum        atomic.Int64        // sum * 1e6 (microsecond precision)
    totalCount atomic.Int64
    labels     string
}

var defaultBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

func (h *Histogram) Observe(value float64) {
    h.totalCount.Add(1)
    h.sum.Add(int64(value * 1e6))
    for i, bound := range h.buckets {
        if value <= bound {
            h.counts[i].Add(1)
        }
    }
}
```

### 16.3 Structured Logging

Use Go 1.21+ `log/slog`:

```go
func SetupLogger(cfg LoggingConfig) *slog.Logger {
    var handler slog.Handler
    opts := &slog.HandlerOptions{
        Level: parseLevel(cfg.Level),
    }

    switch cfg.Format {
    case "json":
        handler = slog.NewJSONHandler(output(cfg.Output), opts)
    default:
        handler = slog.NewTextHandler(output(cfg.Output), opts)
    }

    return slog.New(handler)
}
```

---

## 17. Plugin System

### 17.1 Compile-Time Registration

```go
// internal/plugin/registry.go
type Registry struct {
    auth       map[string]AuthPlugin
    cache      map[string]CachePlugin
    transforms map[string]TransformPlugin
    middleware map[string]MiddlewarePlugin
}

var defaultRegistry = &Registry{
    auth:       make(map[string]AuthPlugin),
    cache:      make(map[string]CachePlugin),
    transforms: make(map[string]TransformPlugin),
    middleware: make(map[string]MiddlewarePlugin),
}

func RegisterAuth(p AuthPlugin)       { defaultRegistry.auth[p.Name()] = p }
func RegisterCache(p CachePlugin)     { defaultRegistry.cache[p.Name()] = p }
func RegisterTransform(p TransformPlugin) { defaultRegistry.transforms[p.Name()] = p }
func RegisterMiddleware(p MiddlewarePlugin) { defaultRegistry.middleware[p.Name()] = p }
```

Built-in plugins register via `init()`:

```go
// internal/gateway/auth/jwt.go
func init() {
    plugin.RegisterAuth(&JWTAuth{})
}
```

Users adding custom plugins add their registration in `cmd/tentaserve/main.go` before starting the server.

---

## 18. Testing Strategy

### 18.1 Test Categories

| Category | Location | Runs In | Purpose |
|----------|----------|---------|---------|
| Unit | `*_test.go` colocated | `go test ./...` | Individual functions and types |
| Integration | `internal/integration_test/` | `go test -tags=integration` | Multi-component flows |
| Benchmark | `*_test.go` with `Benchmark*` | `go test -bench=.` | Performance regression detection |
| End-to-end | `test/e2e/` | CI with mock upstreams | Full request lifecycle |

### 18.2 Mock Upstream Server

```go
func NewMockUpstream(t *testing.T) *httptest.Server {
    mux := http.NewServeMux()
    mux.HandleFunc("GET /api/users/{id}", func(w http.ResponseWriter, r *http.Request) {
        id := r.PathValue("id")
        json.NewEncoder(w).Encode(map[string]any{
            "id": id, "name": "User " + id, "email": id + "@example.com",
        })
    })
    mux.HandleFunc("GET /api/users", func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode([]map[string]any{
            {"id": "1", "name": "Alice"},
            {"id": "2", "name": "Bob"},
        })
    })
    mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(200)
    })
    return httptest.NewServer(mux)
}
```

### 18.3 Coverage Targets

| Package | Target |
|---------|--------|
| `config` | 90% |
| `gateway/*` | 85% |
| `proxy/*` | 80% |
| `schema` | 85% |
| `mcp` | 80% |
| `graphql` | 90% (parser/lexer are critical) |
| `openapi` | 85% |
| Overall | >80% |

---

## 19. Build & Release

### 19.1 Makefile Targets

```makefile
BINARY    := tentaserve
VERSION   := $(shell git describe --tags --always --dirty)
LDFLAGS   := -s -w -X main.version=$(VERSION)
CGO_ENABLED := 0

.PHONY: build test lint bench clean release

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/tentaserve

test:
	go test -race -cover ./...

lint:
	go vet ./...
	staticcheck ./...

bench:
	go test -bench=. -benchmem ./...

clean:
	rm -rf bin/ dist/

release:
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 ./cmd/tentaserve
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64 ./cmd/tentaserve
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64 ./cmd/tentaserve
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64 ./cmd/tentaserve
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe ./cmd/tentaserve
```

### 19.2 CI Pipeline (GitHub Actions)

```
on: [push, pull_request]
jobs:
  test:    go test -race -cover ./...
  lint:    go vet + staticcheck
  build:   cross-compile all platforms
  bench:   run benchmarks, compare with main branch
  release: on tag push, build + create GitHub release with binaries
```

---

## 20. Performance Optimization Playbook

### 20.1 Buffer Pooling

```go
var bufferPool = sync.Pool{
    New: func() any {
        return new(bytes.Buffer)
    },
}

func GetBuffer() *bytes.Buffer {
    buf := bufferPool.Get().(*bytes.Buffer)
    buf.Reset()
    return buf
}

func PutBuffer(buf *bytes.Buffer) {
    if buf.Cap() > 1<<20 { // don't pool buffers > 1MB
        return
    }
    bufferPool.Put(buf)
}
```

### 20.2 JSON Encoder Pooling

```go
var encoderPool = sync.Pool{
    New: func() any {
        buf := new(bytes.Buffer)
        return json.NewEncoder(buf)
    },
}
```

### 20.3 Inlining Hints

Keep hot-path functions small enough for Go compiler inlining (generally <80 nodes in the AST). Split complex functions into a fast-path check + slow-path call:

```go
func (s *cacheShard) Get(key string) ([]byte, bool) {
    s.mu.RLock()
    entry, ok := s.items[key]
    s.mu.RUnlock()
    if !ok {
        return nil, false
    }
    if time.Now().Before(entry.expiresAt) {
        return entry.value, true // fast path: cache hit
    }
    return s.getSlow(key, entry) // slow path: expired, stale check
}
```

### 20.4 Profiling Checkpoints

Before each release, run:

```bash
# CPU profile
go test -bench=BenchmarkProxyE2E -cpuprofile=cpu.prof ./...
go tool pprof cpu.prof

# Memory profile
go test -bench=BenchmarkProxyE2E -memprofile=mem.prof ./...
go tool pprof mem.prof

# Allocation count
go test -bench=. -benchmem ./... | grep allocs/op
```

**Target: <5 allocations per proxied request on the hot path.**

---

*This implementation guide is a living document. When implementation deviates from this guide, update this document with the rationale for the change.*
