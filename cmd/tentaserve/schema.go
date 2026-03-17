package main

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/ersinkoc/tentaserve/internal/config"
	"github.com/ersinkoc/tentaserve/internal/graphql"
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
	s, err := buildSchemaFromConfig(cfg, *upstreamFilter)
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
func buildSchemaFromConfig(cfg *config.Config, upstreamFilter string) (*schema.SchemaDefinition, error) {
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
			if err := buildRESTUpstreamSchema(s, upstream); err != nil {
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
func buildRESTUpstreamSchema(s *schema.SchemaDefinition, upstream config.UpstreamConfig) error {
	// In a real implementation, this would parse the OpenAPI spec
	// For now, create placeholder types based on the upstream name

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
