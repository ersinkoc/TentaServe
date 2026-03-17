package graphql

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

// Handler is an HTTP handler for GraphQL requests.
type Handler struct {
	executor  *Executor
	validator *Validator
	logger    *slog.Logger
}

// HandlerConfig configures the GraphQL handler.
type HandlerConfig struct {
	MaxDepth      int
	MaxComplexity int
	Logger        *slog.Logger
}

// NewHandler creates a new GraphQL HTTP handler.
func NewHandler(config HandlerConfig) *Handler {
	if config.MaxDepth == 0 {
		config.MaxDepth = 10
	}
	if config.MaxComplexity == 0 {
		config.MaxComplexity = 1000
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	return &Handler{
		executor:  NewExecutor(),
		validator: NewValidator(config.MaxDepth, config.MaxComplexity),
		logger:    config.Logger,
	}
}

// Executor returns the handler's executor for registering resolvers.
func (h *Handler) Executor() *Executor {
	return h.executor
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow POST for GraphQL queries
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed. Use POST for GraphQL queries.")
		return
	}

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("Failed to read request body: %v", err))
		return
	}
	defer r.Body.Close()

	// Parse the GraphQL request
	var req GraphQLRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON: %v", err))
		return
	}

	// Validate that query is present
	if req.Query == "" {
		h.writeError(w, http.StatusBadRequest, "Missing required field 'query'")
		return
	}

	// Parse the query
	doc, err := Parse(req.Query)
	if err != nil {
		h.logger.Debug("parse error", "error", err)
		h.writeGraphQLError(w, ExecutionError{
			Message: fmt.Sprintf("Failed to parse query: %v", err),
		})
		return
	}

	// Validate the query
	validationResult := h.validator.Validate(doc)
	if validationResult.HasErrors() {
		h.logger.Debug("validation errors", "errors", validationResult.Errors)
		errors := make([]ExecutionError, len(validationResult.Errors))
		for i, err := range validationResult.Errors {
			errors[i] = ExecutionError{Message: err.Error()}
		}
		h.writeGraphQLErrors(w, errors)
		return
	}

	// Execute the query
	result := h.executor.Execute(r.Context(), doc, req.Variables)

	// Write the response
	h.writeResult(w, result)
}

// GraphQLRequest represents a GraphQL HTTP request body.
type GraphQLRequest struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	OperationName string                 `json:"operationName,omitempty"`
}

// writeResult writes a GraphQL execution result as JSON.
func (h *Handler) writeResult(w http.ResponseWriter, result *ExecutionResult) {
	w.Header().Set("Content-Type", "application/json")

	// Determine status code based on errors
	statusCode := http.StatusOK
	if len(result.Errors) > 0 && result.Data == nil {
		statusCode = http.StatusBadRequest
	}
	w.WriteHeader(statusCode)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		h.logger.Error("failed to encode response", "error", err)
	}
}

// writeError writes a simple HTTP error response.
func (h *Handler) writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]interface{}{
		"errors": []map[string]string{
			{"message": message},
		},
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.Encode(response)
}

// writeGraphQLError writes a single GraphQL error response.
func (h *Handler) writeGraphQLError(w http.ResponseWriter, err ExecutionError) {
	result := &ExecutionResult{
		Errors: []ExecutionError{err},
	}
	h.writeResult(w, result)
}

// writeGraphQLErrors writes multiple GraphQL errors as response.
func (h *Handler) writeGraphQLErrors(w http.ResponseWriter, errors []ExecutionError) {
	result := &ExecutionResult{
		Errors: errors,
	}
	h.writeResult(w, result)
}

// RegisterResolver registers a resolver for a field.
func (h *Handler) RegisterResolver(typeName, fieldName string, resolver ResolverFunc) {
	h.executor.RegisterResolver(typeName, fieldName, resolver)
}

// SetValidator sets a custom validator.
func (h *Handler) SetValidator(validator *Validator) {
	h.validator = validator
}
