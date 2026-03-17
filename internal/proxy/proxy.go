package proxy

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/ersinkoc/tentaserve/internal/graphql"
	"github.com/ersinkoc/tentaserve/internal/openapi"
	"github.com/ersinkoc/tentaserve/internal/proxy/gql2rest"
	"github.com/ersinkoc/tentaserve/internal/proxy/rest2gql"
	"github.com/ersinkoc/tentaserve/internal/schema"
	"github.com/ersinkoc/tentaserve/internal/upstream"
)

// Proxy is the main REST↔GraphQL proxy that orchestrates query translation.
type Proxy struct {
	// Schema definition built from OpenAPI specs
	schemaDef *schema.SchemaDefinition

	// GraphQL executor
	executor *graphql.Executor

	// Resolver registry
	resolvers *rest2gql.ResolverRegistry

	// Upstream clients by name
	upstreams map[string]*upstream.Client

	// Translator for handling GraphQL requests (REST→GraphQL)
	translator *rest2gql.Translator

	// GQL→REST handler for exposing GraphQL as REST
	gql2restHandler *gql2rest.Handler

	// Base URL for this proxy (used for building URLs)
	baseURL string

	// Mutex for thread-safe updates
	mu sync.RWMutex
}

// ProxyOptions configures the proxy.
type ProxyOptions struct {
	BaseURL string
}

// NewProxy creates a new REST→GraphQL proxy.
func NewProxy(opts ProxyOptions) *Proxy {
	return &Proxy{
		schemaDef:  nil,
		executor:   graphql.NewExecutor(),
		resolvers:  rest2gql.NewResolverRegistry(),
		upstreams:  make(map[string]*upstream.Client),
		translator: nil,
		baseURL:    opts.BaseURL,
	}
}

// RegisterUpstream adds an upstream REST API to the proxy.
func (p *Proxy) RegisterUpstream(name string, client *upstream.Client, spec *openapi.OpenAPISpec) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Store upstream client
	p.upstreams[name] = client

	// Build resolvers from spec
	builder := rest2gql.NewResolverBuilder(client, p.baseURL)
	if err := p.resolvers.BuildFromSpec(builder, spec); err != nil {
		return fmt.Errorf("building resolvers from spec: %w", err)
	}

	// Register resolvers with executor using ForEach
	p.resolvers.ForEach(func(fieldPath string, resolver *rest2gql.Resolver) {
		typeName, fieldName := parseFieldPath(fieldPath)
		if typeName != "" && fieldName != "" {
			// Wrap resolver to match graphql.ResolverFunc signature
			p.executor.RegisterResolver(typeName, fieldName, func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				return resolver.Resolve(ctx, args)
			})
		}
	})

	return nil
}

// parseFieldPath splits a field path into type and field name.
func parseFieldPath(path string) (string, string) {
	for i := 0; i < len(path); i++ {
		if path[i] == '.' {
			return path[:i], path[i+1:]
		}
	}
	return "", ""
}

// GetUpstream returns an upstream client by name.
func (p *Proxy) GetUpstream(name string) (*upstream.Client, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	client, ok := p.upstreams[name]
	return client, ok
}

// GetExecutor returns the GraphQL executor.
func (p *Proxy) GetExecutor() *graphql.Executor {
	return p.executor
}

// GetResolvers returns the resolver registry.
func (p *Proxy) GetResolvers() *rest2gql.ResolverRegistry {
	return p.resolvers
}

// HandleGraphQL handles GraphQL HTTP requests.
func (p *Proxy) HandleGraphQL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Use translator if available, otherwise create one
	translator := p.translator
	if translator == nil {
		p.mu.RLock()
		registry := p.resolvers
		p.mu.RUnlock()
		translator = rest2gql.NewTranslator(rest2gql.DefaultTranslatorOptions(), registry)
	}

	translator.HandleHTTP(w, r)
}

// ExecuteQuery executes a GraphQL query directly.
func (p *Proxy) ExecuteQuery(ctx context.Context, query string, variables map[string]interface{}) *graphql.ExecutionResult {
	// Parse the query
	parser := graphql.NewParserString(query)
	doc, err := parser.Parse()
	if err != nil {
		return &graphql.ExecutionResult{
			Errors: []graphql.ExecutionError{{
				Message: fmt.Sprintf("Parse error: %v", err),
			}},
		}
	}

	// Execute using the proxy's executor
	return p.executor.Execute(ctx, doc, variables)
}

// BuildSchema builds a GraphQL schema from an OpenAPI spec.
func (p *Proxy) BuildSchema(spec *openapi.OpenAPISpec) (*schema.SchemaDefinition, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	builder := rest2gql.NewSchemaBuilder(rest2gql.SchemaBuilderOptions{})
	schemaDef, err := builder.Build(spec)
	if err != nil {
		return nil, fmt.Errorf("building schema: %w", err)
	}

	p.schemaDef = schemaDef
	return schemaDef, nil
}

// GetSchema returns the current schema definition.
func (p *Proxy) GetSchema() *schema.SchemaDefinition {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.schemaDef
}

// UpstreamCount returns the number of registered upstreams.
func (p *Proxy) UpstreamCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.upstreams)
}

// ResolverCount returns the number of registered resolvers.
func (p *Proxy) ResolverCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	count := 0
	p.resolvers.ForEach(func(string, *rest2gql.Resolver) {
		count++
	})
	return count
}

// SetGQL2RESTHandler configures the GQL→REST handler.
func (p *Proxy) SetGQL2RESTHandler(handler *gql2rest.Handler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.gql2restHandler = handler
}

// GetGQL2RESTHandler returns the GQL→REST handler.
func (p *Proxy) GetGQL2RESTHandler() *gql2rest.Handler {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.gql2restHandler
}

// HandleREST handles REST HTTP requests (GQL→REST mode).
// This exposes GraphQL operations as REST endpoints.
func (p *Proxy) HandleREST(w http.ResponseWriter, r *http.Request) {
	p.mu.RLock()
	handler := p.gql2restHandler
	p.mu.RUnlock()

	if handler == nil {
		http.Error(w, "GQL→REST handler not configured", http.StatusServiceUnavailable)
		return
	}

	handler.ServeHTTP(w, r)
}
