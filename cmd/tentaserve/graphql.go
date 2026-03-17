package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/ersinkoc/tentaserve/internal/config"
	"github.com/ersinkoc/tentaserve/internal/graphql"
	"github.com/ersinkoc/tentaserve/internal/openapi"
	gqlproxy "github.com/ersinkoc/tentaserve/internal/proxy/graphql"
	"github.com/ersinkoc/tentaserve/internal/proxy/rest2gql"
	"github.com/ersinkoc/tentaserve/internal/schema"
	"github.com/ersinkoc/tentaserve/internal/upstream"
)

// OpenAPIManager manages OpenAPI specs for the gateway.
type OpenAPIManager struct {
	provider *openapi.Provider
}

// NewOpenAPIManager creates a new OpenAPI manager.
func NewOpenAPIManager(logger *slog.Logger) *OpenAPIManager {
	return &OpenAPIManager{
		provider: openapi.NewProvider(logger, openapi.DefaultProviderOptions()),
	}
}

// RegisterUpstreams registers all REST upstreams that have OpenAPI specs configured.
func (m *OpenAPIManager) RegisterUpstreams(cfg *config.Config) {
	for _, u := range cfg.Upstreams {
		if u.Type == "rest" && u.OpenAPI != nil && u.OpenAPI.Source != "" {
			if err := m.provider.RegisterUpstream(u.Name, u.OpenAPI.Source, u.OpenAPI.RefreshInterval); err != nil {
				slog.Default().Warn("Failed to register OpenAPI spec",
					slog.String("upstream", u.Name),
					slog.Any("error", err),
				)
			}
		}
	}
}

// GetProvider returns the OpenAPI provider.
func (m *OpenAPIManager) GetProvider() *openapi.Provider {
	return m.provider
}

// Shutdown gracefully shuts down the OpenAPI manager.
func (m *OpenAPIManager) Shutdown(ctx context.Context) error {
	return m.provider.Shutdown(ctx)
}

// createGraphQLHandler creates a GraphQL HTTP handler with resolvers for upstream services.
func createGraphQLHandler(cfg *config.Config, logger *slog.Logger, openAPIManager *OpenAPIManager) *graphql.Handler {
	// Create handler with config limits
	handler := graphql.NewHandler(graphql.HandlerConfig{
		MaxDepth:      cfg.Schema.Limits.MaxDepth,
		MaxComplexity: cfg.Schema.Limits.MaxComplexity,
		Logger:        logger,
	})

	// Get OpenAPI provider
	var openAPIProvider *openapi.Provider
	if openAPIManager != nil {
		openAPIProvider = openAPIManager.GetProvider()
	}

	// Build schema from configuration
	schemaDef, err := buildSchemaFromConfig(cfg, "", openAPIProvider)
	if err != nil {
		logger.Warn("failed to build schema for GraphQL handler", "error", err)
		// Return handler with no resolvers - it will return empty data
		return handler
	}

	// Register resolvers for each upstream
	registerUpstreams(handler, schemaDef, cfg, logger, openAPIProvider)

	return handler
}

// registerUpstreams registers resolvers for all upstream services.
func registerUpstreams(handler *graphql.Handler, s *schema.SchemaDefinition, cfg *config.Config, logger *slog.Logger, openAPIProvider *openapi.Provider) {
	// Register Query resolvers for each upstream
	for _, upstream := range cfg.Upstreams {
		// Normalize upstream name to valid GraphQL field name
		fieldName := normalizeFieldName(upstream.Name)

		switch upstream.Type {
		case "rest":
			registerRESTUpstream(handler, upstream, fieldName, logger, openAPIProvider)
		case "graphql":
			registerGraphQLUpstream(handler, upstream, fieldName, logger)
		}
	}
}

// normalizeFieldName converts a string to a valid GraphQL field name.
// It replaces hyphens and other invalid characters with underscores.
func normalizeFieldName(name string) string {
	// Replace hyphens with underscores
	name = strings.ReplaceAll(name, "-", "_")

	// Remove any other non-alphanumeric characters (except underscore)
	re := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	name = re.ReplaceAllString(name, "")

	// Ensure it starts with a letter or underscore
	if len(name) > 0 && (name[0] >= '0' && name[0] <= '9') {
		name = "_" + name
	}

	return name
}

// registerRESTUpstream registers resolvers for a REST upstream service.
func registerRESTUpstream(handler *graphql.Handler, upstreamCfg config.UpstreamConfig, fieldName string, logger *slog.Logger, openAPIProvider *openapi.Provider) {
	// Check if we have an OpenAPI spec for this upstream
	if openAPIProvider != nil && openAPIProvider.HasSpec(upstreamCfg.Name) {
		registerRESTUpstreamFromOpenAPI(handler, upstreamCfg, fieldName, logger, openAPIProvider)
		return
	}

	// Fall back to basic resolver if no OpenAPI spec
	registerRESTUpstreamBasic(handler, upstreamCfg, fieldName, logger)
}

// registerRESTUpstreamFromOpenAPI registers resolvers based on OpenAPI spec.
func registerRESTUpstreamFromOpenAPI(handler *graphql.Handler, upstreamCfg config.UpstreamConfig, fieldName string, logger *slog.Logger, openAPIProvider *openapi.Provider) {
	spec, ok := openAPIProvider.GetSpec(upstreamCfg.Name)
	if !ok {
		logger.Warn("OpenAPI spec not found", slog.String("upstream", upstreamCfg.Name))
		registerRESTUpstreamBasic(handler, upstreamCfg, fieldName, logger)
		return
	}

	// Create upstream HTTP client
	client, err := upstream.NewClient(upstream.ClientOptions{
		BaseURL: upstreamCfg.BaseURL,
		Timeout: 30 * time.Second,
		Headers: upstreamCfg.Headers,
	})
	if err != nil {
		logger.Error("Failed to create upstream client",
			slog.String("upstream", upstreamCfg.Name),
			slog.String("error", err.Error()),
		)
		return
	}

	// Create resolver builder for OpenAPI-based resolution
	resolverBuilder := rest2gql.NewResolverBuilder(client, upstreamCfg.BaseURL)

	// Register the main upstream field resolver
	resolver := func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		logger.Debug("REST upstream resolver (OpenAPI) called",
			slog.String("upstream", upstreamCfg.Name),
			slog.String("spec_version", spec.OpenAPI),
			slog.Any("args", args),
		)

		// Return a context object with upstream info and resolver builder
		return map[string]interface{}{
			"__upstream": upstreamCfg.Name,
			"__type":     "rest",
			"__baseUrl":  upstreamCfg.BaseURL,
			"__client":   client,
			"__builder":  resolverBuilder,
			"__args":     args,
			"__spec":     spec,
		}, nil
	}

	// Register the resolver on Query type
	handler.RegisterResolver("Query", fieldName, resolver)

	// Register field resolvers for each path in the OpenAPI spec
	for path, pathItem := range spec.Paths {
		if pathItem.Get != nil {
			operationName := generateOperationName(path, "Get")
			resolver := createOpenAPIQueryResolver(resolverBuilder, path, "GET", pathItem, pathItem.Get, logger)
			handler.RegisterResolver(fieldName+"Query", operationName, resolver)
		}
		if pathItem.Post != nil {
			operationName := generateOperationName(path, "Post")
			resolver := createOpenAPIMutationResolver(resolverBuilder, path, "POST", pathItem, pathItem.Post, logger)
			handler.RegisterResolver(fieldName+"Mutation", operationName, resolver)
		}
		if pathItem.Put != nil {
			operationName := generateOperationName(path, "Put")
			resolver := createOpenAPIMutationResolver(resolverBuilder, path, "PUT", pathItem, pathItem.Put, logger)
			handler.RegisterResolver(fieldName+"Mutation", operationName, resolver)
		}
		if pathItem.Delete != nil {
			operationName := generateOperationName(path, "Delete")
			resolver := createOpenAPIMutationResolver(resolverBuilder, path, "DELETE", pathItem, pathItem.Delete, logger)
			handler.RegisterResolver(fieldName+"Mutation", operationName, resolver)
		}
		if pathItem.Patch != nil {
			operationName := generateOperationName(path, "Patch")
			resolver := createOpenAPIMutationResolver(resolverBuilder, path, "PATCH", pathItem, pathItem.Patch, logger)
			handler.RegisterResolver(fieldName+"Mutation", operationName, resolver)
		}
	}

	logger.Info("Registered REST upstream with OpenAPI spec",
		slog.String("upstream", upstreamCfg.Name),
		slog.String("spec_title", spec.Info.Title),
		slog.Int("paths", len(spec.Paths)),
	)
}

// registerRESTUpstreamBasic registers basic resolvers without OpenAPI spec.
func registerRESTUpstreamBasic(handler *graphql.Handler, upstreamCfg config.UpstreamConfig, fieldName string, logger *slog.Logger) {
	// Create upstream HTTP client
	client, err := upstream.NewClient(upstream.ClientOptions{
		BaseURL: upstreamCfg.BaseURL,
		Timeout: 30 * time.Second,
		Headers: upstreamCfg.Headers,
	})
	if err != nil {
		logger.Error("Failed to create upstream client",
			slog.String("upstream", upstreamCfg.Name),
			slog.String("error", err.Error()),
		)
		return
	}

	// Create resolver builder
	resolverBuilder := rest2gql.NewResolverBuilder(client, upstreamCfg.BaseURL)

	// Create a simple resolver for the upstream field
	resolver := func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		logger.Debug("REST upstream resolver called",
			slog.String("upstream", upstreamCfg.Name),
			slog.Any("args", args),
		)

		// Return a context object that nested resolvers can use
		return map[string]interface{}{
			"__upstream": upstreamCfg.Name,
			"__type":     "rest",
			"__baseUrl":  upstreamCfg.BaseURL,
			"__client":   client,
			"__builder":  resolverBuilder,
			"__args":     args,
		}, nil
	}

	// Register the resolver on Query type using normalized field name
	handler.RegisterResolver("Query", fieldName, resolver)

	// Register a field resolver for 'id' that can extract from parent context
	handler.RegisterResolver(fieldName+"Query", "id", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		if m, ok := parent.(map[string]interface{}); ok {
			return m["id"], nil
		}
		return nil, nil
	})
}

// generateOperationName generates a GraphQL field name from an OpenAPI path and method.
func generateOperationName(path string, method string) string {
	// Remove leading slash and replace special chars
	cleaned := strings.TrimPrefix(path, "/")
	cleaned = strings.ReplaceAll(cleaned, "/", "_")
	cleaned = strings.ReplaceAll(cleaned, "{", "")
	cleaned = strings.ReplaceAll(cleaned, "}", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "_")

	// Convert to camelCase and prefix with method
	name := strings.ToLower(method) + schema.ToPascalCase(cleaned)
	return schema.ToCamelCase(name)
}

// createOpenAPIQueryResolver creates a resolver for an OpenAPI GET operation.
func createOpenAPIQueryResolver(builder *rest2gql.ResolverBuilder, path string, method string, pathItem *openapi.PathItem, operation *openapi.Operation, logger *slog.Logger) func(context.Context, interface{}, map[string]interface{}) (interface{}, error) {
	// Build the REST resolver
	restResolver := builder.BuildRESTResolver(path, method, pathItem, operation)

	return func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		logger.Debug("OpenAPI query resolver called",
			slog.String("path", path),
			slog.String("method", method),
			slog.Any("args", args),
		)

		// Call the REST resolver
		result, err := restResolver.Resolve(ctx, args)
		if err != nil {
			logger.Error("REST resolver failed",
				slog.String("path", path),
				slog.String("method", method),
				slog.String("error", err.Error()),
			)
			return nil, err
		}

		return result, nil
	}
}

// createOpenAPIMutationResolver creates a resolver for an OpenAPI mutation operation (POST, PUT, DELETE, PATCH).
func createOpenAPIMutationResolver(builder *rest2gql.ResolverBuilder, path string, method string, pathItem *openapi.PathItem, operation *openapi.Operation, logger *slog.Logger) func(context.Context, interface{}, map[string]interface{}) (interface{}, error) {
	// Build the REST resolver
	restResolver := builder.BuildRESTResolver(path, method, pathItem, operation)

	return func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		logger.Debug("OpenAPI mutation resolver called",
			slog.String("path", path),
			slog.String("method", method),
			slog.Any("args", args),
		)

		// Call the REST resolver
		result, err := restResolver.Resolve(ctx, args)
		if err != nil {
			logger.Error("REST resolver failed",
				slog.String("path", path),
				slog.String("method", method),
				slog.String("error", err.Error()),
			)
			return nil, err
		}

		return result, nil
	}
}

// registerGraphQLUpstream registers resolvers for a GraphQL upstream service.
func registerGraphQLUpstream(handler *graphql.Handler, upstreamCfg config.UpstreamConfig, fieldName string, logger *slog.Logger) {
	// Create GraphQL proxy client
	client := gqlproxy.NewClient(gqlproxy.ClientOptions{
		Endpoint: upstreamCfg.Endpoint,
		Timeout:  30 * time.Second,
		Headers:  upstreamCfg.Headers,
	})

	// Create a resolver that proxies queries to the GraphQL endpoint
	resolver := func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		logger.Debug("GraphQL upstream resolver called",
			slog.String("upstream", upstreamCfg.Name),
			slog.String("endpoint", upstreamCfg.Endpoint),
			slog.Any("args", args),
		)

		// Extract the query from the context (set by the GraphQL executor)
		query := ""
		if q, ok := ctx.Value("graphql.query").(string); ok {
			query = q
		}

		// Extract variables from args
		variables := make(map[string]interface{})
		for key, value := range args {
			variables[key] = value
		}

		// Execute the query against the upstream
		req := gqlproxy.QueryRequest{
			Query:     query,
			Variables: variables,
		}

		resp, err := client.Execute(ctx, req)
		if err != nil {
			logger.Error("GraphQL upstream query failed",
				slog.String("upstream", upstreamCfg.Name),
				slog.String("error", err.Error()),
			)
			return nil, err
		}

		// Check for GraphQL errors
		if len(resp.Errors) > 0 {
			err := resp.Errors[0]
			logger.Error("GraphQL upstream returned error",
				slog.String("upstream", upstreamCfg.Name),
				slog.String("error", err.Message),
			)
			return nil, &err
		}

		// Return the data
		var result map[string]interface{}
		if err := json.Unmarshal(resp.Data, &result); err != nil {
			return resp.Data, nil // Return raw if can't unmarshal
		}

		// Add upstream metadata
		result["__upstream"] = upstreamCfg.Name
		result["__type"] = "graphql"
		result["__endpoint"] = upstreamCfg.Endpoint

		return result, nil
	}

	// Register the resolver on Query type using normalized field name
	handler.RegisterResolver("Query", fieldName, resolver)

	// Also register resolvers for any fields on the upstream type
	handler.RegisterResolver(fieldName+"Query", "id", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		if m, ok := parent.(map[string]interface{}); ok {
			return m["id"], nil
		}
		return nil, nil
	})

	logger.Info("Registered GraphQL upstream",
		slog.String("upstream", upstreamCfg.Name),
		slog.String("endpoint", upstreamCfg.Endpoint),
	)
}
