package main

import (
	"context"
	"log/slog"
	"regexp"
	"strings"

	"github.com/ersinkoc/tentaserve/internal/config"
	"github.com/ersinkoc/tentaserve/internal/graphql"
	"github.com/ersinkoc/tentaserve/internal/schema"
)

// createGraphQLHandler creates a GraphQL HTTP handler with resolvers for upstream services.
func createGraphQLHandler(cfg *config.Config, logger *slog.Logger) *graphql.Handler {
	// Create handler with config limits
	handler := graphql.NewHandler(graphql.HandlerConfig{
		MaxDepth:      cfg.Schema.Limits.MaxDepth,
		MaxComplexity: cfg.Schema.Limits.MaxComplexity,
		Logger:        logger,
	})

	// Build schema from configuration
	schemaDef, err := buildSchemaFromConfig(cfg, "")
	if err != nil {
		logger.Warn("failed to build schema for GraphQL handler", "error", err)
		// Return handler with no resolvers - it will return empty data
		return handler
	}

	// Register resolvers for each upstream
	registerUpstreams(handler, schemaDef, cfg, logger)

	return handler
}

// registerUpstreams registers resolvers for all upstream services.
func registerUpstreams(handler *graphql.Handler, s *schema.SchemaDefinition, cfg *config.Config, logger *slog.Logger) {
	// Register Query resolvers for each upstream
	for _, upstream := range cfg.Upstreams {
		// Normalize upstream name to valid GraphQL field name
		fieldName := normalizeFieldName(upstream.Name)

		switch upstream.Type {
		case "rest":
			registerRESTUpstream(handler, upstream, fieldName, logger)
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
func registerRESTUpstream(handler *graphql.Handler, upstream config.UpstreamConfig, fieldName string, logger *slog.Logger) {
	// Create a resolver that returns placeholder data for now
	// In a full implementation, this would proxy to the REST API
	resolver := func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		logger.Debug("REST upstream resolver called",
			slog.String("upstream", upstream.Name),
			slog.Any("args", args),
		)

		// Return placeholder data with upstream info
		return map[string]interface{}{
			"__upstream":    upstream.Name,
			"__type":        "rest",
			"__baseUrl":     upstream.BaseURL,
			"__placeholder": true,
			"id":            "placeholder-id",
		}, nil
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
}

// registerGraphQLUpstream registers resolvers for a GraphQL upstream service.
func registerGraphQLUpstream(handler *graphql.Handler, upstream config.UpstreamConfig, fieldName string, logger *slog.Logger) {
	// Create a resolver that returns placeholder data for now
	// In a full implementation, this would proxy to the GraphQL endpoint
	resolver := func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		logger.Debug("GraphQL upstream resolver called",
			slog.String("upstream", upstream.Name),
			slog.String("endpoint", upstream.Endpoint),
			slog.Any("args", args),
		)

		// Return placeholder data with upstream info
		return map[string]interface{}{
			"__upstream":    upstream.Name,
			"__type":        "graphql",
			"__endpoint":    upstream.Endpoint,
			"__placeholder": true,
			"id":            "placeholder-id",
		}, nil
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
}
