package rest2gql

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ersinkoc/tentaserve/internal/graphql"
)

// Translator translates GraphQL queries to REST upstream calls.
type Translator struct {
	validator *graphql.Validator
	executor  *graphql.Executor
	registry  *ResolverRegistry
}

// TranslatorOptions configures the translator.
type TranslatorOptions struct {
	MaxDepth      int
	MaxComplexity int
}

// DefaultTranslatorOptions returns default options.
func DefaultTranslatorOptions() TranslatorOptions {
	return TranslatorOptions{
		MaxDepth:      10,
		MaxComplexity: 1000,
	}
}

// NewTranslator creates a new translator.
func NewTranslator(opts TranslatorOptions, registry *ResolverRegistry) *Translator {
	executor := graphql.NewExecutor()

	// Register resolvers from registry
	if registry != nil {
		registry.ForEach(func(fieldPath string, resolver *Resolver) {
			// Parse fieldPath (e.g., "Query.users" -> typeName="Query", fieldName="users")
			typeName, fieldName := parseFieldPath(fieldPath)
			if typeName != "" && fieldName != "" {
				// Wrap resolver to match graphql.ResolverFunc signature
				executor.RegisterResolver(typeName, fieldName, func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
					return resolver.Resolve(ctx, args)
				})
			}
		})
	}

	return &Translator{
		validator: graphql.NewValidator(opts.MaxDepth, opts.MaxComplexity),
		executor:  executor,
		registry:  registry,
	}
}

// parseFieldPath splits a field path into type and field name.
func parseFieldPath(path string) (string, string) {
	// Simple parsing: "Type.field" -> ("Type", "field")
	for i := 0; i < len(path); i++ {
		if path[i] == '.' {
			return path[:i], path[i+1:]
		}
	}
	return "", ""
}

// GraphQLRequest represents an incoming GraphQL request.
type GraphQLRequest struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	OperationName string                 `json:"operationName,omitempty"`
}

// GraphQLResponse represents a GraphQL response.
type GraphQLResponse struct {
	Data   interface{}          `json:"data,omitempty"`
	Errors []GraphQLError       `json:"errors,omitempty"`
}

// GraphQLError represents a GraphQL error in the response.
type GraphQLError struct {
	Message    string                 `json:"message"`
	Path       []string               `json:"path,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// Translate parses, validates, and executes a GraphQL query.
func (t *Translator) Translate(ctx context.Context, req *GraphQLRequest) *GraphQLResponse {
	// Parse the query
	parser := graphql.NewParserString(req.Query)
	doc, err := parser.Parse()
	if err != nil {
		return &GraphQLResponse{
			Errors: []GraphQLError{{
				Message: fmt.Sprintf("Parse error: %v", err),
			}},
		}
	}

	// Validate the query
	validationResult := t.validator.Validate(doc)
	if validationResult.HasErrors() {
		errors := make([]GraphQLError, len(validationResult.Errors))
		for i, ve := range validationResult.Errors {
			errors[i] = GraphQLError{
				Message: ve.Error(),
			}
		}
		return &GraphQLResponse{
			Errors: errors,
		}
	}

	// Execute the query
	execResult := t.executor.Execute(ctx, doc, req.Variables)

	// Convert execution result to response
	response := &GraphQLResponse{
		Data: execResult.Data,
	}

	if len(execResult.Errors) > 0 {
		response.Errors = make([]GraphQLError, len(execResult.Errors))
		for i, ee := range execResult.Errors {
			response.Errors[i] = GraphQLError{
				Message:    ee.Message,
				Path:       ee.Path,
				Extensions: ee.Extensions,
			}
		}
	}

	return response
}

// TranslateAndWrite executes a GraphQL request and writes the response.
func (t *Translator) TranslateAndWrite(ctx context.Context, w http.ResponseWriter, req *GraphQLRequest) {
	response := t.Translate(ctx, req)

	// Determine status code
	statusCode := http.StatusOK
	if len(response.Errors) > 0 && response.Data == nil {
		statusCode = http.StatusBadRequest
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// HandleHTTP handles an HTTP GraphQL request.
func (t *Translator) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(GraphQLResponse{
			Errors: []GraphQLError{{
				Message: "Method not allowed. Use POST.",
			}},
		})
		return
	}

	// Parse request body
	var req GraphQLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GraphQLResponse{
			Errors: []GraphQLError{{
				Message: fmt.Sprintf("Invalid JSON: %v", err),
			}},
		})
		return
	}

	// Execute and respond
	t.TranslateAndWrite(r.Context(), w, &req)
}

// RefreshResolvers updates the executor with current registry resolvers.
func (t *Translator) RefreshResolvers() {
	if t.registry == nil {
		return
	}

	t.registry.ForEach(func(fieldPath string, resolver *Resolver) {
		typeName, fieldName := parseFieldPath(fieldPath)
		if typeName != "" && fieldName != "" {
			// Wrap resolver to match graphql.ResolverFunc signature
			t.executor.RegisterResolver(typeName, fieldName, func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				return resolver.Resolve(ctx, args)
			})
		}
	})
}
