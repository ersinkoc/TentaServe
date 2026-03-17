package rest2gql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ersinkoc/tentaserve/internal/openapi"
	"github.com/ersinkoc/tentaserve/internal/upstream"
)

// Resolver resolves GraphQL fields by dispatching REST calls.
type Resolver struct {
	client    *upstream.Client
	baseURL   string
	path      string // The path pattern like "/users/{id}"
	pathItem  *openapi.PathItem
	method    string
	operation *openapi.Operation
}

// ResolverOptions configures the resolver.
type ResolverOptions struct {
	BaseURL   string
	Client    *upstream.Client
	Path      string
	PathItem  *openapi.PathItem
	Method    string
	Operation *openapi.Operation
}

// NewResolver creates a new REST resolver.
func NewResolver(opts ResolverOptions) *Resolver {
	return &Resolver{
		client:    opts.Client,
		baseURL:   opts.BaseURL,
		path:      opts.Path,
		pathItem:  opts.PathItem,
		method:    opts.Method,
		operation: opts.Operation,
	}
}

// Resolve executes the REST call and returns the result.
func (r *Resolver) Resolve(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if r.operation == nil {
		return nil, fmt.Errorf("no operation defined")
	}

	// Build the URL with path parameters interpolated
	url, err := r.buildURL(args)
	if err != nil {
		return nil, fmt.Errorf("building URL: %w", err)
	}

	// Build request body for mutations
	body, err := r.buildBody(args)
	if err != nil {
		return nil, fmt.Errorf("building body: %w", err)
	}

	// Create the HTTP request
	req, err := r.createRequest(ctx, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Add headers from context (auth, etc.)
	r.addHeadersFromContext(ctx, req)

	// Execute the request
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	// Parse the response
	result, err := r.parseResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return result, nil
}

// buildURL constructs the full URL with path parameters interpolated.
func (r *Resolver) buildURL(args map[string]interface{}) (string, error) {
	if r.operation == nil {
		return "", fmt.Errorf("no operation defined")
	}

	// Start with base URL
	base := r.baseURL
	if base == "" {
		base = "/"
	}

	// Get the path - we need to reconstruct it from operation info
	// For now, use the path pattern from the resolver
	path := r.path
	if path == "" {
		path = "/"
	}

	// Interpolate path parameters
	for _, param := range r.operation.Parameters {
		if param.In == "path" {
			value, ok := args[param.Name]
			if !ok {
				return "", fmt.Errorf("missing path parameter: %s", param.Name)
			}
			placeholder := "{" + param.Name + "}"
			path = strings.ReplaceAll(path, placeholder, fmt.Sprintf("%v", value))
		}
	}

	// Build query parameters
	queryParams := make([]string, 0)
	for _, param := range r.operation.Parameters {
		if param.In == "query" {
			value, ok := args[param.Name]
			if ok {
				queryParams = append(queryParams, fmt.Sprintf("%s=%v", param.Name, value))
			}
		}
	}

	url := base + path
	if len(queryParams) > 0 {
		url = url + "?" + strings.Join(queryParams, "&")
	}

	return url, nil
}

// buildBody creates the request body for mutations.
func (r *Resolver) buildBody(args map[string]interface{}) (io.Reader, error) {
	// Only build body for POST, PUT, PATCH
	if r.method != "POST" && r.method != "PUT" && r.method != "PATCH" {
		return nil, nil
	}

	// Check for input argument (common convention for mutations)
	if input, ok := args["input"]; ok {
		jsonBody, err := json.Marshal(input)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(jsonBody), nil
	}

	// Build body from non-path, non-query args
	bodyArgs := make(map[string]interface{})
	for key, value := range args {
		// Skip path and query parameters
		isParam := false
		for _, param := range r.operation.Parameters {
			if param.Name == key && (param.In == "path" || param.In == "query") {
				isParam = true
				break
			}
		}
		if !isParam {
			bodyArgs[key] = value
		}
	}

	if len(bodyArgs) > 0 {
		jsonBody, err := json.Marshal(bodyArgs)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(jsonBody), nil
	}

	return nil, nil
}

// createRequest creates an HTTP request.
func (r *Resolver) createRequest(ctx context.Context, url string, body io.Reader) (*http.Request, error) {
	method := r.method
	if method == "" {
		method = "GET"
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	// Set content type for requests with body
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Set Accept header
	req.Header.Set("Accept", "application/json")

	return req, nil
}

// addHeadersFromContext forwards headers from context (auth, etc.).
func (r *Resolver) addHeadersFromContext(ctx context.Context, req *http.Request) {
	// Extract auth token from context if present
	if authHeader := ctx.Value("Authorization"); authHeader != nil {
		if str, ok := authHeader.(string); ok {
			req.Header.Set("Authorization", str)
		}
	}

	// Extract other common headers
	for _, header := range []string{"X-Request-ID", "X-User-ID", "X-Tenant-ID"} {
		if val := ctx.Value(header); val != nil {
			if str, ok := val.(string); ok {
				req.Header.Set(header, str)
			}
		}
	}
}

// parseResponse parses the HTTP response into a Go value.
func (r *Resolver) parseResponse(resp *http.Response) (interface{}, error) {
	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upstream error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	// Read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Handle empty response
	if len(body) == 0 {
		return nil, nil
	}

	// Parse JSON
	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}

	return result, nil
}

// ResolverBuilder builds resolvers for GraphQL fields.
type ResolverBuilder struct {
	client  *upstream.Client
	baseURL string
}

// NewResolverBuilder creates a new resolver builder.
func NewResolverBuilder(client *upstream.Client, baseURL string) *ResolverBuilder {
	return &ResolverBuilder{
		client:  client,
		baseURL: baseURL,
	}
}

// BuildRESTResolver creates a resolver for a REST operation.
func (b *ResolverBuilder) BuildRESTResolver(path string, method string, pathItem *openapi.PathItem, operation *openapi.Operation) *Resolver {
	return NewResolver(ResolverOptions{
		BaseURL:   b.baseURL,
		Client:    b.client,
		Path:      path,
		PathItem:  pathItem,
		Method:    method,
		Operation: operation,
	})
}

// ResolverRegistry holds resolvers for GraphQL fields.
type ResolverRegistry struct {
	resolvers map[string]*Resolver
}

// NewResolverRegistry creates a new resolver registry.
func NewResolverRegistry() *ResolverRegistry {
	return &ResolverRegistry{
		resolvers: make(map[string]*Resolver),
	}
}

// Register adds a resolver for a field.
func (r *ResolverRegistry) Register(fieldPath string, resolver *Resolver) {
	r.resolvers[fieldPath] = resolver
}

// Lookup retrieves a resolver for a field.
func (r *ResolverRegistry) Lookup(fieldPath string) (*Resolver, bool) {
	resolver, ok := r.resolvers[fieldPath]
	return resolver, ok
}

// ForEach iterates over all resolvers in the registry.
func (r *ResolverRegistry) ForEach(fn func(fieldPath string, resolver *Resolver)) {
	for path, resolver := range r.resolvers {
		fn(path, resolver)
	}
}

// BuildFromSpec creates resolvers from an OpenAPI spec.
func (r *ResolverRegistry) BuildFromSpec(builder *ResolverBuilder, spec *openapi.OpenAPISpec) error {
	if spec == nil || len(spec.Paths) == 0 {
		return fmt.Errorf("no paths in spec")
	}

	for path, pathItem := range spec.Paths {
		// GET -> Query resolver
		if pathItem.Get != nil {
			resolver := builder.BuildRESTResolver(path, "GET", pathItem, pathItem.Get)
			r.Register("Query."+pathToFieldName(path), resolver)
		}

		// POST -> Mutation resolver
		if pathItem.Post != nil {
			resolver := builder.BuildRESTResolver(path, "POST", pathItem, pathItem.Post)
			r.Register("Mutation."+pathToFieldName(path), resolver)
		}

		// PUT -> Mutation resolver
		if pathItem.Put != nil {
			resolver := builder.BuildRESTResolver(path, "PUT", pathItem, pathItem.Put)
			r.Register("Mutation."+pathToFieldName(path), resolver)
		}

		// DELETE -> Mutation resolver
		if pathItem.Delete != nil {
			resolver := builder.BuildRESTResolver(path, "DELETE", pathItem, pathItem.Delete)
			r.Register("Mutation."+pathToFieldName(path), resolver)
		}

		// PATCH -> Mutation resolver
		if pathItem.Patch != nil {
			resolver := builder.BuildRESTResolver(path, "PATCH", pathItem, pathItem.Patch)
			r.Register("Mutation."+pathToFieldName(path), resolver)
		}
	}

	return nil
}

// pathToFieldName converts a path to a field name.
func pathToFieldName(path string) string {
	// Remove leading slash and replace special chars
	cleaned := strings.TrimPrefix(path, "/")
	cleaned = strings.ReplaceAll(cleaned, "/", "_")
	cleaned = strings.ReplaceAll(cleaned, "{", "")
	cleaned = strings.ReplaceAll(cleaned, "}", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "_")
	return cleaned
}
