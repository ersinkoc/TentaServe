package gql2rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Handler handles REST requests and translates them to GraphQL.
type Handler struct {
	endpoints    []Endpoint
	schema       *Schema
	translator   *Translator
	client       GraphQLClient
	basePath     string
}

// Schema represents a GraphQL schema for the handler.
type Schema struct {
	QueryType    *Type
	MutationType *Type
	Types        map[string]*Type
}

// Type represents a GraphQL type in the handler schema.
type Type struct {
	Name   string
	Fields []Field
}

// Field represents a GraphQL field in the handler schema.
type Field struct {
	Name string
	Type string
	Args []Argument
}

// GraphQLClient executes GraphQL queries against an upstream.
type GraphQLClient interface {
	Execute(ctx context.Context, query string, variables map[string]interface{}) (*GraphQLResponse, error)
}

// HTTPGraphQLClient executes GraphQL queries via HTTP.
type HTTPGraphQLClient struct {
	BaseURL    string
	HTTPClient *http.Client
	Headers    map[string]string
}

// NewHTTPGraphQLClient creates a new HTTP GraphQL client.
func NewHTTPGraphQLClient(baseURL string) *HTTPGraphQLClient {
	return &HTTPGraphQLClient{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		Headers:    make(map[string]string),
	}
}

// Execute executes a GraphQL query via HTTP.
func (c *HTTPGraphQLClient) Execute(ctx context.Context, query string, variables map[string]interface{}) (*GraphQLResponse, error) {
	reqBody := GraphQLRequest{
		Query:     query,
		Variables: variables,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	for key, value := range c.Headers {
		req.Header.Set(key, value)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var gqlResp GraphQLResponse
	if len(body) == 0 {
		// Empty response - return empty GraphQL response
		return &gqlResp, nil
	}
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return &gqlResp, nil
}

// HandlerOptions configures the handler.
type HandlerOptions struct {
	BasePath     string
	Endpoints    []Endpoint
	GraphQLURL   string
	GraphQLClient GraphQLClient
}

// NewHandler creates a new REST handler.
func NewHandler(opts HandlerOptions) *Handler {
	client := opts.GraphQLClient
	if client == nil && opts.GraphQLURL != "" {
		client = NewHTTPGraphQLClient(opts.GraphQLURL)
	}

	return &Handler{
		endpoints: opts.Endpoints,
		basePath:  opts.BasePath,
		client:    client,
		translator: nil, // Will be initialized on first use
	}
}

// ServeHTTP implements the http.Handler interface.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Find matching endpoint
	endpoint, pathParams := h.findEndpoint(r.URL.Path, r.Method)
	if endpoint == nil {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Endpoint not found")
		return
	}

	// Parse request body for mutations
	var body map[string]interface{}
	if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
				h.writeError(w, http.StatusBadRequest, "INVALID_BODY", "Invalid JSON body")
				return
			}
		}
	}

	// Parse fields parameter
	fields := r.URL.Query().Get("fields")

	// Build REST request
	restReq := RESTRequest{
		Method:      r.Method,
		Path:        r.URL.Path,
		QueryParams: make(map[string]string),
		Body:        body,
	}

	// Add query params (excluding 'fields')
	for key, values := range r.URL.Query() {
		if key != "fields" && len(values) > 0 {
			restReq.QueryParams[key] = values[0]
		}
	}

	// Add path params
	for key, value := range pathParams {
		restReq.QueryParams[key] = value
	}

	// Parse fields if provided
	if fields != "" {
		restReq.Fields = ParseFieldsQuery(fields)
	}

	// Create translator if needed
	if h.translator == nil {
		h.translator = NewTranslator(nil) // Schema will be looked up from endpoint
	}

	// Translate to GraphQL
	gqlReq, err := h.translator.Translate(restReq, *endpoint)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "TRANSLATION_ERROR", err.Error())
		return
	}

	// Execute against GraphQL upstream
	ctx := r.Context()
	gqlResp, err := h.client.Execute(ctx, gqlReq.Query, gqlReq.Variables)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "UPSTREAM_ERROR", err.Error())
		return
	}

	// Unwrap response
	result, err := h.unwrapResponse(gqlResp, endpoint.Field)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "UNWRAP_ERROR", err.Error())
		return
	}

	// Write response
	h.writeResponse(w, result)
}

// findEndpoint finds a matching endpoint for the given path and method.
func (h *Handler) findEndpoint(path, method string) (*Endpoint, map[string]string) {
	for _, ep := range h.endpoints {
		if ep.Method != method {
			continue
		}

		// Try exact match first
		if ep.Path == path {
			return &ep, make(map[string]string)
		}

		// Try pattern match for path params
		params := h.matchPath(ep.Path, path)
		if params != nil {
			return &ep, params
		}
	}
	return nil, nil
}

// matchPath matches a path pattern against an actual path.
// Pattern: /api/users/{id}
// Path: /api/users/123
// Returns: map[string]string{"id": "123"} or nil if no match
func (h *Handler) matchPath(pattern, path string) map[string]string {
	patternParts := splitPath(pattern)
	pathParts := splitPath(path)

	if len(patternParts) != len(pathParts) {
		return nil
	}

	params := make(map[string]string)
	for i, pp := range patternParts {
		if len(pp) > 0 && pp[0] == '{' && pp[len(pp)-1] == '}' {
			// This is a parameter
			paramName := pp[1 : len(pp)-1]
			params[paramName] = pathParts[i]
		} else if pp != pathParts[i] {
			// Static segment doesn't match
			return nil
		}
	}

	return params
}

// splitPath splits a path into segments.
func splitPath(path string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			if i > start {
				parts = append(parts, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		parts = append(parts, path[start:])
	}
	return parts
}

// unwrapResponse unwraps a GraphQL response.
func (h *Handler) unwrapResponse(resp *GraphQLResponse, fieldName string) (*UnwrappedResponse, error) {
	// Check for GraphQL errors
	if len(resp.Errors) > 0 {
		firstError := resp.Errors[0]
		code := "GRAPHQL_ERROR"
		if firstError.Extensions != nil {
			if c, ok := firstError.Extensions["code"].(string); ok {
				code = c
			}
		}

		status := mapErrorCodeToHTTPStatus(code)
		return &UnwrappedResponse{
			Error: &RESTError{
				Code:    code,
				Message: firstError.Message,
			},
			Status: status,
		}, nil
	}

	// Extract data
	var dataMap map[string]json.RawMessage
	if err := json.Unmarshal(resp.Data, &dataMap); err != nil {
		return nil, fmt.Errorf("parsing response data: %w", err)
	}

	fieldData, ok := dataMap[fieldName]
	if !ok {
		return &UnwrappedResponse{
			Data:   nil,
			Status: http.StatusOK,
		}, nil
	}

	var result interface{}
	if err := json.Unmarshal(fieldData, &result); err != nil {
		return nil, fmt.Errorf("parsing field data: %w", err)
	}

	return &UnwrappedResponse{
		Data:   result,
		Status: http.StatusOK,
	}, nil
}

// mapErrorCodeToHTTPStatus maps GraphQL error codes to HTTP status codes.
func mapErrorCodeToHTTPStatus(code string) int {
	switch code {
	case "UNAUTHORIZED":
		return http.StatusUnauthorized
	case "FORBIDDEN":
		return http.StatusForbidden
	case "NOT_FOUND":
		return http.StatusNotFound
	case "VALIDATION_FAILED", "BAD_USER_INPUT":
		return http.StatusBadRequest
	case "INTERNAL_ERROR":
		return http.StatusInternalServerError
	case "TIMEOUT":
		return http.StatusGatewayTimeout
	case "RATE_LIMITED":
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}

// writeResponse writes a successful REST response.
func (h *Handler) writeResponse(w http.ResponseWriter, resp *UnwrappedResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.Status)

	if resp.Error != nil {
		json.NewEncoder(w).Encode(resp.Error)
		return
	}

	if resp.Data == nil {
		w.Write([]byte("null"))
		return
	}

	json.NewEncoder(w).Encode(resp.Data)
}

// writeError writes an error REST response.
func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(&RESTError{
		Code:    code,
		Message: message,
	})
}

