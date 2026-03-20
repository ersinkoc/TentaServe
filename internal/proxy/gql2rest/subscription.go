package gql2rest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ersinkoc/tentaserve/internal/graphql"
)

// SubscriptionHandler handles GraphQL subscription → SSE bridge.
// It converts REST SSE requests to GraphQL subscription queries,
// connects to the upstream GraphQL subscription endpoint, and
// streams events back to the client as Server-Sent Events.
type SubscriptionHandler struct {
	endpoints  []SubscriptionEndpoint
	client     SubscriptionClient
	basePath   string
	mu         sync.RWMutex
	activeSubs map[string]context.CancelFunc
}

// SubscriptionEndpoint represents a generated SSE endpoint for a GraphQL subscription.
type SubscriptionEndpoint struct {
	Path        string
	Field       string // GraphQL subscription field name
	Description string
	Arguments   []Argument
	ReturnType  string
}

// SubscriptionClient connects to upstream GraphQL subscriptions.
type SubscriptionClient interface {
	Subscribe(ctx context.Context, query string, variables map[string]interface{}) (<-chan *SubscriptionEvent, error)
}

// SubscriptionEvent represents an event from a GraphQL subscription.
type SubscriptionEvent struct {
	Data   json.RawMessage `json:"data,omitempty"`
	Errors []GraphQLError  `json:"errors,omitempty"`
}

// SSEEvent represents a Server-Sent Event.
type SSEEvent struct {
	ID    string `json:"id,omitempty"`
	Event string `json:"event,omitempty"`
	Data  string `json:"data"`
}

// SubscriptionHandlerOptions configures the subscription handler.
type SubscriptionHandlerOptions struct {
	BasePath  string
	Endpoints []SubscriptionEndpoint
	Client    SubscriptionClient
}

// NewSubscriptionHandler creates a new subscription handler.
func NewSubscriptionHandler(opts SubscriptionHandlerOptions) *SubscriptionHandler {
	if opts.BasePath == "" {
		opts.BasePath = "/api/stream"
	}
	return &SubscriptionHandler{
		endpoints:  opts.Endpoints,
		client:     opts.Client,
		basePath:   opts.BasePath,
		activeSubs: make(map[string]context.CancelFunc),
	}
}

// ServeHTTP implements the http.Handler interface.
func (h *SubscriptionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only accept GET requests for SSE
	if r.Method != http.MethodGet {
		writeSSEError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "SSE subscriptions require GET")
		return
	}

	// Check Accept header
	accept := r.Header.Get("Accept")
	if accept != "" && !strings.Contains(accept, "text/event-stream") && !strings.Contains(accept, "*/*") {
		writeSSEError(w, http.StatusNotAcceptable, "NOT_ACCEPTABLE", "Expected Accept: text/event-stream")
		return
	}

	// Find matching endpoint
	endpoint, pathParams := h.findEndpoint(r.URL.Path)
	if endpoint == nil {
		writeSSEError(w, http.StatusNotFound, "NOT_FOUND", "Subscription endpoint not found")
		return
	}

	// Check that response writer supports flushing
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeSSEError(w, http.StatusInternalServerError, "STREAMING_NOT_SUPPORTED", "Streaming not supported")
		return
	}

	// Build subscription query
	query, variables := h.buildSubscriptionQuery(r, endpoint, pathParams)

	// Connect to upstream subscription
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	events, err := h.client.Subscribe(ctx, query, variables)
	if err != nil {
		writeSSEError(w, http.StatusBadGateway, "UPSTREAM_ERROR", fmt.Sprintf("Failed to connect to subscription: %s", err))
		return
	}

	// Track active subscription
	subID := fmt.Sprintf("%s-%d", endpoint.Field, time.Now().UnixNano())
	h.mu.Lock()
	h.activeSubs[subID] = cancel
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.activeSubs, subID)
		h.mu.Unlock()
	}()

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// Send initial comment to establish connection
	fmt.Fprintf(w, ": connected to %s\n\n", endpoint.Field)
	flusher.Flush()

	// Set up heartbeat ticker
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	eventID := 0

	// Stream events
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			// Send heartbeat comment
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		case event, ok := <-events:
			if !ok {
				// Channel closed - subscription ended
				fmt.Fprintf(w, "event: complete\ndata: {\"message\":\"subscription ended\"}\n\n")
				flusher.Flush()
				return
			}

			eventID++

			// Handle errors
			if len(event.Errors) > 0 {
				errData, _ := json.Marshal(map[string]interface{}{
					"errors": event.Errors,
				})
				fmt.Fprintf(w, "id: %d\nevent: error\ndata: %s\n\n", eventID, string(errData))
				flusher.Flush()
				continue
			}

			// Unwrap the subscription data
			unwrapped := h.unwrapEventData(event.Data, endpoint.Field)

			// Write SSE event
			fmt.Fprintf(w, "id: %d\nevent: message\ndata: %s\n\n", eventID, string(unwrapped))
			flusher.Flush()
		}
	}
}

// findEndpoint finds a matching subscription endpoint.
func (h *SubscriptionHandler) findEndpoint(path string) (*SubscriptionEndpoint, map[string]string) {
	for i, ep := range h.endpoints {
		fullPath := h.basePath + "/" + ep.Path
		if fullPath == path {
			return &h.endpoints[i], make(map[string]string)
		}

		// Try pattern matching
		params := matchSubscriptionPath(fullPath, path)
		if params != nil {
			return &h.endpoints[i], params
		}
	}
	return nil, nil
}

// matchSubscriptionPath matches a path pattern against an actual path.
func matchSubscriptionPath(pattern, path string) map[string]string {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	if len(patternParts) != len(pathParts) {
		return nil
	}

	params := make(map[string]string)
	for i, pp := range patternParts {
		if len(pp) > 0 && pp[0] == '{' && pp[len(pp)-1] == '}' {
			paramName := pp[1 : len(pp)-1]
			params[paramName] = pathParts[i]
		} else if pp != pathParts[i] {
			return nil
		}
	}
	return params
}

// buildSubscriptionQuery builds a GraphQL subscription query from a REST request.
func (h *SubscriptionHandler) buildSubscriptionQuery(r *http.Request, ep *SubscriptionEndpoint, pathParams map[string]string) (string, map[string]interface{}) {
	variables := make(map[string]interface{})
	var argParts []string

	for _, arg := range ep.Arguments {
		var value string
		var found bool

		// Check path params first, then query params
		if v, ok := pathParams[arg.Name]; ok {
			value = v
			found = true
		} else if v := r.URL.Query().Get(arg.Name); v != "" {
			value = v
			found = true
		}

		if found {
			variables[arg.Name] = value
			argParts = append(argParts, fmt.Sprintf("%s: $%s", arg.Name, arg.Name))
		}
	}

	// Build variable declarations
	var varDecls []string
	for _, arg := range ep.Arguments {
		if _, ok := variables[arg.Name]; ok {
			gqlType := mapJSONTypeToGraphQL(arg.Type)
			if arg.Required {
				gqlType += "!"
			}
			varDecls = append(varDecls, fmt.Sprintf("$%s: %s", arg.Name, gqlType))
		}
	}

	// Build the subscription query
	var query string
	varDeclStr := ""
	if len(varDecls) > 0 {
		varDeclStr = "(" + strings.Join(varDecls, ", ") + ")"
	}

	argStr := ""
	if len(argParts) > 0 {
		argStr = "(" + strings.Join(argParts, ", ") + ")"
	}

	// Build field selection based on ?fields= or default
	fields := r.URL.Query().Get("fields")
	selection := "{}"
	if fields != "" {
		selectors := ParseFieldsQuery(fields)
		selection = BuildSelectionSet(selectors)
	}

	query = fmt.Sprintf("subscription%s { %s%s %s }", varDeclStr, ep.Field, argStr, selection)

	return query, variables
}

// unwrapEventData unwraps a subscription event's data.
func (h *SubscriptionHandler) unwrapEventData(data json.RawMessage, fieldName string) json.RawMessage {
	if len(data) == 0 {
		return []byte("null")
	}

	var dataMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &dataMap); err != nil {
		return data
	}

	if fieldData, ok := dataMap[fieldName]; ok {
		return fieldData
	}

	return data
}

// ActiveSubscriptions returns the number of active subscriptions.
func (h *SubscriptionHandler) ActiveSubscriptions() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.activeSubs)
}

// Close cancels all active subscriptions.
func (h *SubscriptionHandler) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, cancel := range h.activeSubs {
		cancel()
		delete(h.activeSubs, id)
	}
}

// mapJSONTypeToGraphQL maps JSON/REST types to GraphQL types.
func mapJSONTypeToGraphQL(jsonType string) string {
	switch jsonType {
	case "string":
		return "String"
	case "integer":
		return "Int"
	case "number":
		return "Float"
	case "boolean":
		return "Boolean"
	default:
		return "String"
	}
}

// writeSSEError writes an error response before SSE headers are sent.
func writeSSEError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(&RESTError{
		Code:    code,
		Message: message,
	})
}

// GenerateSubscriptionEndpoints generates SSE endpoints from a GraphQL schema's subscription type.
func GenerateSubscriptionEndpoints(schema *graphql.Schema) []SubscriptionEndpoint {
	if schema == nil || schema.SubscriptionType == nil {
		return nil
	}

	subType := schema.GetType(schema.SubscriptionType.Name)
	if subType == nil {
		return nil
	}

	var endpoints []SubscriptionEndpoint
	for _, field := range subType.Fields {
		ep := SubscriptionEndpoint{
			Path:        toKebabCase(field.Name),
			Field:       field.Name,
			Description: field.Description,
			ReturnType:  graphql.GetTypeName(&field.Type),
		}

		// Build arguments
		for _, arg := range field.Args {
			unwrapped := graphql.UnwrapType(&arg.Type)
			argType := "string"
			if unwrapped != nil {
				argType = mapGraphQLTypeToJSONType(unwrapped.Name)
			}

			ep.Arguments = append(ep.Arguments, Argument{
				Name:        arg.Name,
				Type:        argType,
				Required:    graphql.IsNonNull(&arg.Type),
				Location:    "query",
				Description: arg.Description,
			})
		}

		// Add path params for ID-like arguments
		for i, arg := range ep.Arguments {
			if strings.Contains(strings.ToLower(arg.Name), "id") {
				ep.Arguments[i].Location = "path"
				ep.Path = ep.Path + "/{" + arg.Name + "}"
				break
			}
		}

		endpoints = append(endpoints, ep)
	}

	return endpoints
}

// mapGraphQLTypeToJSONType maps GraphQL scalar types to JSON types.
func mapGraphQLTypeToJSONType(graphQLType string) string {
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

// HTTPSubscriptionClient implements SubscriptionClient using HTTP streaming.
// This client connects to upstream GraphQL servers that support subscription
// via SSE (like GraphQL over SSE) rather than WebSocket.
type HTTPSubscriptionClient struct {
	URL        string
	HTTPClient *http.Client
	Headers    map[string]string
}

// NewHTTPSubscriptionClient creates a new HTTP-based subscription client.
func NewHTTPSubscriptionClient(url string) *HTTPSubscriptionClient {
	return &HTTPSubscriptionClient{
		URL: url,
		HTTPClient: &http.Client{
			Timeout: 0, // No timeout for streaming
		},
		Headers: make(map[string]string),
	}
}

// Subscribe connects to an upstream GraphQL subscription via HTTP SSE.
func (c *HTTPSubscriptionClient) Subscribe(ctx context.Context, query string, variables map[string]interface{}) (<-chan *SubscriptionEvent, error) {
	reqBody := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling subscription request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("creating subscription request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	for key, value := range c.Headers {
		req.Header.Set(key, value)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connecting to subscription: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("subscription failed with status %d: %s", resp.StatusCode, string(body))
	}

	events := make(chan *SubscriptionEvent, 16)

	go func() {
		defer close(events)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		var dataLines []string

		for scanner.Scan() {
			line := scanner.Text()

			if strings.HasPrefix(line, "data: ") {
				dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
			} else if line == "" && len(dataLines) > 0 {
				// End of event - process accumulated data lines
				data := strings.Join(dataLines, "\n")
				dataLines = nil

				var event SubscriptionEvent
				if err := json.Unmarshal([]byte(data), &event); err != nil {
					// Try wrapping in data field
					event = SubscriptionEvent{
						Data: json.RawMessage(data),
					}
				}

				select {
				case <-ctx.Done():
					return
				case events <- &event:
				}
			} else if line == "" {
				dataLines = nil
			}
		}
	}()

	return events, nil
}
