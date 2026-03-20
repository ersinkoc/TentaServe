// Package gql2rest converts GraphQL schemas to REST endpoints.
package gql2rest

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ersinkoc/tentaserve/internal/graphql"
)

// Endpoint represents a generated REST endpoint.
type Endpoint struct {
	Path        string
	Method      string
	GraphQLType string // "Query" or "Mutation"
	Field       string // GraphQL field name
	Description string
	Arguments   []Argument
	ReturnType  string
}

// Argument represents an endpoint argument.
type Argument struct {
	Name        string
	Type        string
	Required    bool
	Location    string // "path", "query", or "body"
	Description string
}

// EndpointBuilder generates REST endpoints from GraphQL schemas.
type EndpointBuilder struct {
	basePath     string
	mutationsUse string // "POST" for all mutations, or "heuristic" for method inference
}

// EndpointBuilderOptions configures the endpoint builder.
type EndpointBuilderOptions struct {
	BasePath     string // e.g., "/api"
	MutationsUse string // "POST" or "heuristic"
}

// NewEndpointBuilder creates a new endpoint builder.
func NewEndpointBuilder(opts EndpointBuilderOptions) *EndpointBuilder {
	if opts.BasePath == "" {
		opts.BasePath = "/api"
	}
	if opts.MutationsUse == "" {
		opts.MutationsUse = "heuristic"
	}
	return &EndpointBuilder{
		basePath:     opts.BasePath,
		mutationsUse: opts.MutationsUse,
	}
}

// GenerateEndpoints generates REST endpoints from a GraphQL schema.
func (b *EndpointBuilder) GenerateEndpoints(schema *graphql.Schema) []Endpoint {
	var endpoints []Endpoint

	// Generate Query endpoints
	if schema.QueryType != nil {
		queryType := schema.GetType(schema.QueryType.Name)
		if queryType != nil {
			for _, field := range queryType.Fields {
				endpoint := b.buildQueryEndpoint(field)
				endpoints = append(endpoints, endpoint)
			}
		}
	}

	// Generate Mutation endpoints
	if schema.MutationType != nil {
		mutationType := schema.GetType(schema.MutationType.Name)
		if mutationType != nil {
			for _, field := range mutationType.Fields {
				endpoint := b.buildMutationEndpoint(field)
				endpoints = append(endpoints, endpoint)
			}
		}
	}

	return endpoints
}

// buildQueryEndpoint builds an endpoint for a Query field.
func (b *EndpointBuilder) buildQueryEndpoint(field graphql.IntrospectionField) Endpoint {
	path := b.toKebabCase(field.Name)
	args := b.buildArguments(field.Args, "query")

	// If there's an ID argument, consider making it a path param
	path = b.applyPathParams(path, args)

	return Endpoint{
		Path:        b.basePath + "/" + path,
		Method:      "GET",
		GraphQLType: "Query",
		Field:       field.Name,
		Description: field.Description,
		Arguments:   args,
		ReturnType:  graphql.GetTypeName(&field.Type),
	}
}

// buildMutationEndpoint builds an endpoint for a Mutation field.
func (b *EndpointBuilder) buildMutationEndpoint(field graphql.IntrospectionField) Endpoint {
	path := b.toKebabCase(field.Name)
	method := b.inferHTTPMethod(field.Name)

	// For PUT/DELETE/PATCH with ID args, use path params
	// For POST, use body
	args := b.buildMutationArguments(field.Args, method)
	path = b.applyPathParams(path, args)

	return Endpoint{
		Path:        b.basePath + "/" + path,
		Method:      method,
		GraphQLType: "Mutation",
		Field:       field.Name,
		Description: field.Description,
		Arguments:   args,
		ReturnType:  graphql.GetTypeName(&field.Type),
	}
}

// buildMutationArguments builds arguments for mutations.
// ID arguments become path params for PUT/DELETE/PATCH, others go in body.
func (b *EndpointBuilder) buildMutationArguments(inputs []graphql.InputValue, method string) []Argument {
	var args []Argument
	for _, input := range inputs {
		unwrapped := graphql.UnwrapType(&input.Type)
		argType := "string"
		if unwrapped != nil {
			argType = b.mapGraphQLTypeToJSON(unwrapped.Name)
		}

		// Determine location: ID args become path params for PUT/DELETE/PATCH
		location := "body"
		if (method == "PUT" || method == "DELETE" || method == "PATCH") &&
			strings.Contains(strings.ToLower(input.Name), "id") {
			location = "path"
		}

		arg := Argument{
			Name:        input.Name,
			Type:        argType,
			Required:    graphql.IsNonNull(&input.Type),
			Location:    location,
			Description: input.Description,
		}
		args = append(args, arg)
	}
	return args
}

// inferHTTPMethod infers HTTP method from mutation name.
func (b *EndpointBuilder) inferHTTPMethod(mutationName string) string {
	if b.mutationsUse == "POST" {
		return "POST"
	}

	name := strings.ToLower(mutationName)

	// Check prefixes for method inference
	if hasPrefix(name, "create", "add", "new", "insert", "post") {
		return "POST"
	}
	if hasPrefix(name, "update", "edit", "modify", "patch", "put") {
		return "PUT"
	}
	if hasPrefix(name, "delete", "remove", "destroy", "drop") {
		return "DELETE"
	}
	if hasPrefix(name, "upsert", "merge") {
		return "PUT"
	}

	// Default to POST for mutations
	return "POST"
}

// buildArguments builds argument definitions from GraphQL input values.
func (b *EndpointBuilder) buildArguments(inputs []graphql.InputValue, defaultLocation string) []Argument {
	var args []Argument
	for _, input := range inputs {
		unwrapped := graphql.UnwrapType(&input.Type)
		argType := "string"
		if unwrapped != nil {
			argType = b.mapGraphQLTypeToJSON(unwrapped.Name)
		}

		arg := Argument{
			Name:        input.Name,
			Type:        argType,
			Required:    graphql.IsNonNull(&input.Type),
			Location:    defaultLocation,
			Description: input.Description,
		}
		args = append(args, arg)
	}
	return args
}

// applyPathParams converts ID arguments to path parameters.
func (b *EndpointBuilder) applyPathParams(path string, args []Argument) string {
	for i, arg := range args {
		// If argument name contains "id" and it's a string/int type, make it a path param
		if strings.Contains(strings.ToLower(arg.Name), "id") && arg.Location != "body" {
			args[i].Location = "path"
			// Add path parameter to URL
			path = path + "/{" + arg.Name + "}"
			return path
		}
	}
	return path
}

// toKebabCase converts camelCase or PascalCase to kebab-case.
func (b *EndpointBuilder) toKebabCase(s string) string {
	// Insert hyphen before uppercase letters (except the first character)
	re := regexp.MustCompile(`([a-z0-9])([A-Z])`)
	s = re.ReplaceAllString(s, "${1}-${2}")
	// Handle consecutive uppercase (e.g., "getURL" -> "get-url")
	re2 := regexp.MustCompile(`([A-Z])([A-Z][a-z])`)
	s = re2.ReplaceAllString(s, "${1}-${2}")
	return strings.ToLower(s)
}

// mapGraphQLTypeToJSON maps GraphQL scalar types to JSON/JavaScript types.
func (b *EndpointBuilder) mapGraphQLTypeToJSON(graphQLType string) string {
	switch graphQLType {
	case "String", "ID":
		return "string"
	case "Int":
		return "integer"
	case "Float":
		return "number"
	case "Boolean":
		return "boolean"
	default:
		return "object"
	}
}

// hasPrefix checks if string has any of the given prefixes.
func hasPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// GenerateOpenAPISpec generates an OpenAPI spec from endpoints.
func (b *EndpointBuilder) GenerateOpenAPISpec(endpoints []Endpoint) map[string]interface{} {
	paths := make(map[string]interface{})

	for _, ep := range endpoints {
		pathItem := b.buildPathItem(ep)
		paths[ep.Path] = pathItem
	}

	return paths
}

// buildPathItem builds an OpenAPI path item for an endpoint.
func (b *EndpointBuilder) buildPathItem(ep Endpoint) map[string]interface{} {
	operation := map[string]interface{}{
		"summary":     ep.Description,
		"operationId": ep.Field,
		"tags":        []string{ep.GraphQLType},
	}

	// Add parameters
	var parameters []map[string]interface{}
	for _, arg := range ep.Arguments {
		if arg.Location == "path" {
			parameters = append(parameters, map[string]interface{}{
				"name":     arg.Name,
				"in":       "path",
				"required": true,
				"schema": map[string]interface{}{
					"type": arg.Type,
				},
			})
		} else if arg.Location == "query" {
			param := map[string]interface{}{
				"name":   arg.Name,
				"in":     "query",
				"schema": map[string]interface{}{"type": arg.Type},
			}
			if arg.Description != "" {
				param["description"] = arg.Description
			}
			parameters = append(parameters, param)
		}
	}

	if len(parameters) > 0 {
		operation["parameters"] = parameters
	}

	// Add request body for POST/PUT/PATCH
	if ep.Method == "POST" || ep.Method == "PUT" || ep.Method == "PATCH" {
		bodyFields := make(map[string]interface{})
		var requiredFields []string

		for _, arg := range ep.Arguments {
			if arg.Location == "body" {
				bodyFields[arg.Name] = map[string]interface{}{
					"type": arg.Type,
				}
				if arg.Required {
					requiredFields = append(requiredFields, arg.Name)
				}
			}
		}

		if len(bodyFields) > 0 {
			requestBody := map[string]interface{}{
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": map[string]interface{}{
							"type":       "object",
							"properties": bodyFields,
						},
					},
				},
			}
			if len(requiredFields) > 0 {
				requestBody["required"] = requiredFields
			}
			operation["requestBody"] = requestBody
		}
	}

	// Add responses
	operation["responses"] = map[string]interface{}{
		"200": map[string]interface{}{
			"description": "Success",
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{},
			},
		},
	}

	// Wrap in path item
	pathItem := make(map[string]interface{})
	switch strings.ToLower(ep.Method) {
	case "get":
		pathItem["get"] = operation
	case "post":
		pathItem["post"] = operation
	case "put":
		pathItem["put"] = operation
	case "patch":
		pathItem["patch"] = operation
	case "delete":
		pathItem["delete"] = operation
	}

	return pathItem
}

// String returns a string representation of an endpoint.
func (e Endpoint) String() string {
	return fmt.Sprintf("%s %s -> %s.%s", e.Method, e.Path, e.GraphQLType, e.Field)
}
