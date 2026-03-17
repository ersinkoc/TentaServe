package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ersinkoc/tentaserve/internal/config"
	"github.com/ersinkoc/tentaserve/internal/proxy/router"
)

// RESTHandler handles REST API requests and routes them to upstreams.
type RESTHandler struct {
	router    *router.Router
	upstreams map[string]*upstreamClient
	logger    *slog.Logger
}

// upstreamClient holds HTTP client configuration for an upstream.
type upstreamClient struct {
	name    string
	typ     string // "rest" or "graphql"
	baseURL string
	client  *http.Client
	headers map[string]string
	timeout time.Duration
	// GraphQL specific
	graphqlEndpoint string
}

// NewRESTHandler creates a new REST handler from configuration.
func NewRESTHandler(cfg *config.Config, logger *slog.Logger) *RESTHandler {
	// Build upstream routing table
	upstreams := make(map[string]string)
	for _, u := range cfg.Upstreams {
		// Map each upstream to a path prefix
		prefix := "/" + u.Name
		upstreams[prefix] = u.Name
	}

	routerConfig := &router.Config{
		GraphQLPath: cfg.Gateway.GraphQLPath,
		MCPPath:     cfg.Gateway.MCPPath,
		RESTPrefix:  cfg.Gateway.RESTPrefix,
		HealthPath:  "/-/health",
		MetricsPath: cfg.Observability.Metrics.Path,
		Upstreams:   upstreams,
	}

	// Create upstream clients for both REST and GraphQL
	upstreamClients := make(map[string]*upstreamClient)
	for _, u := range cfg.Upstreams {
		client := &upstreamClient{
			name:            u.Name,
			typ:             u.Type,
			baseURL:         u.BaseURL,
			graphqlEndpoint: u.Endpoint,
			client: &http.Client{
				Timeout: u.Timeout,
			},
			headers: u.Headers,
			timeout: u.Timeout,
		}

		// Use default timeout if not set
		if client.timeout == 0 {
			client.client.Timeout = 30 * time.Second
		}

		upstreamClients[u.Name] = client
	}

	return &RESTHandler{
		router:    router.New(routerConfig),
		upstreams: upstreamClients,
		logger:    logger,
	}
}

// ServeHTTP implements http.Handler.
func (h *RESTHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Classify the request
	classified := h.router.ClassifyRequest(r)

	// Only handle REST requests
	if classified.Type != router.TypeREST {
		h.writeError(w, http.StatusNotFound, "Not found")
		return
	}

	// Find upstream
	if classified.Upstream == "" {
		h.logger.Debug("No upstream found for path", "path", r.URL.Path)
		h.writeError(w, http.StatusNotFound, "No upstream configured for this path")
		return
	}

	// Get upstream client
	upstream, ok := h.upstreams[classified.Upstream]
	if !ok {
		h.logger.Warn("Upstream not found", "upstream", classified.Upstream)
		h.writeError(w, http.StatusServiceUnavailable, "Upstream not available")
		return
	}

	// Route based on upstream type
	switch upstream.typ {
	case "rest":
		// Proxy directly to REST upstream
		h.proxyRequest(w, r, upstream, classified.CleanPath)
	case "graphql":
		// Translate REST to GraphQL and proxy
		h.proxyGraphQL(w, r, upstream, classified.CleanPath)
	default:
		h.logger.Warn("Unknown upstream type", "upstream", classified.Upstream, "type", upstream.typ)
		h.writeError(w, http.StatusServiceUnavailable, "Unknown upstream type")
	}
}

// proxyRequest forwards the request to the upstream.
func (h *RESTHandler) proxyRequest(w http.ResponseWriter, r *http.Request, upstream *upstreamClient, cleanPath string) {
	// Build upstream URL
	upstreamURL, err := url.JoinPath(upstream.baseURL, cleanPath)
	if err != nil {
		h.logger.Error("Failed to build upstream URL", "error", err)
		h.writeError(w, http.StatusInternalServerError, "Failed to build upstream URL")
		return
	}

	// Parse query parameters
	if r.URL.RawQuery != "" {
		upstreamURL = upstreamURL + "?" + r.URL.RawQuery
	}

	// Create request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("Failed to read request body", "error", err)
		h.writeError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}
	defer r.Body.Close()

	req, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, strings.NewReader(string(body)))
	if err != nil {
		h.logger.Error("Failed to create upstream request", "error", err)
		h.writeError(w, http.StatusInternalServerError, "Failed to create request")
		return
	}

	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Add upstream-specific headers
	for key, value := range upstream.headers {
		req.Header.Set(key, value)
	}

	// Remove hop-by-hop headers
	req.Header.Del("Connection")
	req.Header.Del("Keep-Alive")
	req.Header.Del("Proxy-Authenticate")
	req.Header.Del("Proxy-Authorization")
	req.Header.Del("TE")
	req.Header.Del("Trailers")
	req.Header.Del("Transfer-Encoding")
	req.Header.Del("Upgrade")

	// Execute request
	h.logger.Debug("Proxying request",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("upstream_url", upstreamURL),
	)

	resp, err := upstream.client.Do(req)
	if err != nil {
		h.logger.Error("Upstream request failed", "error", err)
		h.writeError(w, http.StatusBadGateway, "Upstream request failed: "+err.Error())
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Set content type if not set
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}

	// Write status
	w.WriteHeader(resp.StatusCode)

	// Copy body
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		h.logger.Error("Failed to copy response body", "error", err)
	}
}

// proxyGraphQL translates REST request to GraphQL and forwards to GraphQL upstream.
func (h *RESTHandler) proxyGraphQL(w http.ResponseWriter, r *http.Request, upstream *upstreamClient, cleanPath string) {
	// Build GraphQL query from REST request
	// Extract resource name from path
	pathParts := strings.Split(strings.Trim(cleanPath, "/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		h.writeError(w, http.StatusBadRequest, "Invalid path")
		return
	}

	resourceName := pathParts[0]
	resourceID := ""
	if len(pathParts) > 1 {
		resourceID = pathParts[1]
	}

	// Build GraphQL query
	var query string
	var variables map[string]interface{}

	switch r.Method {
	case http.MethodGet:
		if resourceID != "" {
			// Get single resource
			query = fmt.Sprintf(`query Get%s($id: ID!) { %s(id: $id) { id } }`, resourceName, resourceName)
			variables = map[string]interface{}{"id": resourceID}
		} else {
			// List resources
			query = fmt.Sprintf(`query List%s { %s { id } }`, resourceName, resourceName)
		}
	case http.MethodPost:
		query = fmt.Sprintf(`mutation Create%s($input: Create%sInput!) { create%s(input: $input) { id } }`, resourceName, resourceName, resourceName)
		// Parse body as variables
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		json.Unmarshal(body, &variables)
	case http.MethodPut, http.MethodPatch:
		if resourceID == "" {
			h.writeError(w, http.StatusBadRequest, "ID required for update")
			return
		}
		query = fmt.Sprintf(`mutation Update%s($id: ID!, $input: Update%sInput!) { update%s(id: $id, input: $input) { id } }`, resourceName, resourceName, resourceName)
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		json.Unmarshal(body, &variables)
		variables["id"] = resourceID
	case http.MethodDelete:
		if resourceID == "" {
			h.writeError(w, http.StatusBadRequest, "ID required for delete")
			return
		}
		query = fmt.Sprintf(`mutation Delete%s($id: ID!) { delete%s(id: $id) { success } }`, resourceName, resourceName)
		variables = map[string]interface{}{"id": resourceID}
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Build GraphQL request
	gqlReq := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}

	reqBody, err := json.Marshal(gqlReq)
	if err != nil {
		h.logger.Error("Failed to marshal GraphQL request", "error", err)
		h.writeError(w, http.StatusInternalServerError, "Failed to build GraphQL request")
		return
	}

	// Determine endpoint
	endpoint := upstream.graphqlEndpoint
	if endpoint == "" {
		endpoint = upstream.baseURL
	}

	// Create request to GraphQL upstream
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		h.logger.Error("Failed to create GraphQL request", "error", err)
		h.writeError(w, http.StatusInternalServerError, "Failed to create request")
		return
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add upstream-specific headers
	for key, value := range upstream.headers {
		req.Header.Set(key, value)
	}

	// Execute request
	h.logger.Debug("Proxying to GraphQL upstream",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("endpoint", endpoint),
		slog.String("query", query),
	)

	resp, err := upstream.client.Do(req)
	if err != nil {
		h.logger.Error("GraphQL upstream request failed", "error", err)
		h.writeError(w, http.StatusBadGateway, "Upstream request failed: "+err.Error())
		return
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		h.logger.Error("Failed to read GraphQL response", "error", err)
		h.writeError(w, http.StatusBadGateway, "Failed to read upstream response")
		return
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

// writeError writes an error response.
func (h *RESTHandler) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(`{"error":"` + message + `"}`))
}
