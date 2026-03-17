package main

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/ersinkoc/tentaserve/internal/config"
	"github.com/ersinkoc/tentaserve/internal/graphql"
	"github.com/ersinkoc/tentaserve/internal/openapi"
	"github.com/ersinkoc/tentaserve/internal/proxy/rest2gql"
	"github.com/ersinkoc/tentaserve/internal/schema"
)

func schemaCmd(args []string) error {
	// Parse flags
	fs := flag.NewFlagSet("schema", flag.ExitOnError)
	configPath := fs.String("config", "tentaserve.yaml", "Path to configuration file")
	format := fs.String("format", "sdl", "Output format: sdl, json")
	upstreamFilter := fs.String("upstream", "", "Filter to specific upstream (default: all)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Load configuration
	cfg, err := config.LoadOrDefault(*configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Build schema from configuration
	s, err := buildSchemaFromConfig(cfg, *upstreamFilter, nil)
	if err != nil {
		return fmt.Errorf("building schema: %w", err)
	}

	// Output based on format
	switch *format {
	case "sdl":
		output := graphql.PrintSDL(s)
		fmt.Println(output)
	case "json":
		// Output as JSON (OpenAPI-like structure)
		output, err := schemaToJSON(s)
		if err != nil {
			return fmt.Errorf("converting to JSON: %w", err)
		}
		fmt.Println(output)
	default:
		return fmt.Errorf("unknown format: %s (supported: sdl, json)", *format)
	}

	return nil
}

// buildSchemaFromConfig builds a schema definition from configuration.
func buildSchemaFromConfig(cfg *config.Config, upstreamFilter string, openAPIProvider *openapi.Provider) (*schema.SchemaDefinition, error) {
	s := schema.NewSchemaDefinition()

	// Add built-in scalars
	for _, t := range schema.BuiltinScalars() {
		s.AddType(t)
	}

	// Build schema from each upstream
	for _, upstream := range cfg.Upstreams {
		// Apply upstream filter if specified
		if upstreamFilter != "" && upstream.Name != upstreamFilter {
			continue
		}

		// Build schema based on upstream type
		switch upstream.Type {
		case "rest":
			if err := buildRESTUpstreamSchema(s, upstream, openAPIProvider); err != nil {
				return nil, fmt.Errorf("building REST schema for %s: %w", upstream.Name, err)
			}
		case "graphql":
			if err := buildGraphQLUpstreamSchema(s, upstream); err != nil {
				return nil, fmt.Errorf("building GraphQL schema for %s: %w", upstream.Name, err)
			}
		}
	}

	// Set root query type if not already set
	if s.Query == nil {
		queryType := &schema.TypeDef{
			Name: "Query",
			Kind: schema.TypeKindObject,
		}
		s.AddType(queryType)
		s.Query = &schema.OperationDef{
			Name: "Query",
			Type: "query",
		}
	}

	return s, nil
}

// buildRESTUpstreamSchema builds schema from REST upstream configuration.
func buildRESTUpstreamSchema(s *schema.SchemaDefinition, upstream config.UpstreamConfig, openAPIProvider *openapi.Provider) error {
	// Check if we have an OpenAPI spec for this upstream
	if openAPIProvider != nil && openAPIProvider.HasSpec(upstream.Name) {
		return buildRESTSchemaFromOpenAPI(s, upstream, openAPIProvider)
	}

	// Fall back to placeholder schema if no OpenAPI spec available
	return buildPlaceholderRESTSchema(s, upstream)
}

// buildRESTSchemaFromOpenAPI builds a GraphQL schema from an OpenAPI spec.
func buildRESTSchemaFromOpenAPI(s *schema.SchemaDefinition, upstream config.UpstreamConfig, openAPIProvider *openapi.Provider) error {
	spec, ok := openAPIProvider.GetSpec(upstream.Name)
	if !ok {
		return fmt.Errorf("OpenAPI spec not found for upstream %s", upstream.Name)
	}

	// Use schema builder to convert OpenAPI to GraphQL
	builder := rest2gql.NewSchemaBuilder(rest2gql.SchemaBuilderOptions{
		TypePrefix: schema.ToPascalCase(upstream.Name),
	})

	schemaDef, err := builder.Build(spec)
	if err != nil {
		return fmt.Errorf("building GraphQL schema from OpenAPI: %w", err)
	}

	// Merge the built schema into our schema definition
	for _, name := range schemaDef.AllTypes() {
		if typeDef, ok := schemaDef.GetType(name); ok {
			s.AddType(typeDef)
		}
	}

	// Merge Query and Mutation fields
	if schemaDef.Query != nil && len(schemaDef.Query.Fields) > 0 {
		queryType, ok := s.GetType("Query")
		if !ok {
			queryType = &schema.TypeDef{
				Name:   "Query",
				Kind:   schema.TypeKindObject,
				Fields: []*schema.FieldDef{},
			}
			s.AddType(queryType)
		}

		// Add upstream prefix to field names to avoid conflicts
		for _, field := range schemaDef.Query.Fields {
			// Clone field with prefixed name
			prefixedField := &schema.FieldDef{
				Name:        normalizeFieldName(upstream.Name) + "_" + field.Name,
				Description: field.Description,
				Type:        field.Type,
				Arguments:   field.Arguments,
			}
			queryType.Fields = append(queryType.Fields, prefixedField)
		}

		if s.Query == nil {
			s.Query = &schema.OperationDef{
				Name: "Query",
				Type: "query",
			}
		}
		s.Query.Fields = queryType.Fields
	}

	if schemaDef.Mutation != nil && len(schemaDef.Mutation.Fields) > 0 {
		mutationType, ok := s.GetType("Mutation")
		if !ok {
			mutationType = &schema.TypeDef{
				Name:   "Mutation",
				Kind:   schema.TypeKindObject,
				Fields: []*schema.FieldDef{},
			}
			s.AddType(mutationType)
		}

		// Add upstream prefix to field names to avoid conflicts
		for _, field := range schemaDef.Mutation.Fields {
			prefixedField := &schema.FieldDef{
				Name:        normalizeFieldName(upstream.Name) + "_" + field.Name,
				Description: field.Description,
				Type:        field.Type,
				Arguments:   field.Arguments,
			}
			mutationType.Fields = append(mutationType.Fields, prefixedField)
		}

		if s.Mutation == nil {
			s.Mutation = &schema.OperationDef{
				Name: "Mutation",
				Type: "mutation",
			}
		}
		s.Mutation.Fields = mutationType.Fields
	}

	return nil
}

// buildPlaceholderRESTSchema creates a placeholder schema for REST upstreams without OpenAPI specs.
func buildPlaceholderRESTSchema(s *schema.SchemaDefinition, upstream config.UpstreamConfig) error {
	// Add a placeholder type for this upstream
	upstreamType := &schema.TypeDef{
		Name: upstream.Name + "Query",
		Kind: schema.TypeKindObject,
		Fields: []*schema.FieldDef{
			{
				Name:        "id",
				Description: "The ID of the " + upstream.Name,
				Type:        schema.IDType(),
			},
		},
	}
	s.AddType(upstreamType)

	// Add to Query type
	queryType, ok := s.GetType("Query")
	if !ok {
		queryType = &schema.TypeDef{
			Name:   "Query",
			Kind:   schema.TypeKindObject,
			Fields: []*schema.FieldDef{},
		}
		s.AddType(queryType)
	}

	// Add field for this upstream
	field := &schema.FieldDef{
		Name:        upstream.Name,
		Description: "Query root for " + upstream.Name,
		Type:        schema.NamedType(upstreamType.Name),
	}
	queryType.Fields = append(queryType.Fields, field)

	// Update schema Query operation
	if s.Query == nil {
		s.Query = &schema.OperationDef{
			Name: "Query",
			Type: "query",
		}
	}

	return nil
}

// buildGraphQLUpstreamSchema builds schema from GraphQL upstream configuration.
func buildGraphQLUpstreamSchema(s *schema.SchemaDefinition, upstream config.UpstreamConfig) error {
	// In a real implementation, this would introspect the GraphQL endpoint
	// For now, add a placeholder comment

	// Add placeholder type
	upstreamType := &schema.TypeDef{
		Name: upstream.Name + "Query",
		Kind: schema.TypeKindObject,
		Fields: []*schema.FieldDef{
			{
				Name:        "id",
				Description: "The ID",
				Type:        schema.IDType(),
			},
		},
	}
	s.AddType(upstreamType)

	return nil
}

// schemaToJSON converts schema to JSON representation.
func schemaToJSON(s *schema.SchemaDefinition) (string, error) {
	// Build a JSON representation similar to OpenAPI
	result := map[string]interface{}{
		"openapi": "3.1.0",
		"info": map[string]string{
			"title":       "Tentaserve API",
			"description": "Auto-generated API from Tentaserve schema",
			"version":     "1.0.0",
		},
		"paths": map[string]interface{}{},
	}

	// Add paths for each type
	paths := map[string]interface{}{}

	// Query path
	if s.Query != nil {
		for _, field := range s.Query.Fields {
			path := "/api/" + field.Name
			paths[path] = map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": field.Name,
					"responses": map[string]interface{}{
						"200": map[string]string{
							"description": "Success",
						},
					},
				},
			}
		}
	}

	result["paths"] = paths

	// Convert to JSON
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}
