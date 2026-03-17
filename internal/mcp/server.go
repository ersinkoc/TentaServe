package mcp

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
)

// Handler is a function that handles an MCP request and returns a result or error.
type Handler func(req *Request) (any, *Error)

// Server is an MCP JSON-RPC server.
type Server struct {
	mu       sync.RWMutex
	handlers map[string]Handler
	logger   *slog.Logger
	name     string
	version  string
	metrics  *Metrics
}

// ServerInfo contains information about the MCP server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// NewServer creates a new MCP server.
func NewServer(logger *slog.Logger, metrics *Metrics) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	s := &Server{
		handlers: make(map[string]Handler),
		logger:   logger,
		name:     "tentaserve",
		version:  "0.1.0",
		metrics:  metrics,
	}

	// Register built-in handlers
	s.registerBuiltinHandlers()

	return s
}

// SetServerInfo sets the server name and version.
func (s *Server) SetServerInfo(name, version string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.name = name
	s.version = version
}

// ServerInfo returns the current server info.
func (s *Server) ServerInfo() ServerInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return ServerInfo{
		Name:    s.name,
		Version: s.version,
	}
}

// Register registers a handler for a method.
func (s *Server) Register(method string, handler Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[method] = handler
	s.logger.Debug("registered MCP handler", "method", method)
}

// Unregister removes a handler for a method.
func (s *Server) Unregister(method string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.handlers, method)
	s.logger.Debug("unregistered MCP handler", "method", method)
}

// Handle processes a JSON-RPC request and returns a response.
func (s *Server) Handle(req *Request) *Response {
	if req == nil {
		return NewErrorResponse(nil, ErrInvalidReq)
	}

	// Validate request
	if !req.IsValid() {
		return req.NewErrorResponse(ErrInvalidReq)
	}

	s.mu.RLock()
	handler, ok := s.handlers[req.Method]
	s.mu.RUnlock()

	if !ok {
		s.logger.Debug("method not found", "method", req.Method)
		return req.NewErrorResponse(ErrMethodMissing)
	}

	// Execute handler
	result, err := handler(req)
	if err != nil {
		s.logger.Debug("handler returned error", "method", req.Method, "error", err)
		return req.NewErrorResponse(err)
	}

	return req.NewResponse(result)
}

// HandleBatch processes a batch of JSON-RPC requests and returns responses.
// Notifications are processed but not included in the response.
func (s *Server) HandleBatch(reqs []*Request) []*Response {
	if len(reqs) == 0 {
		// Empty batch is invalid
		return []*Response{NewErrorResponse(nil, ErrInvalidReq)}
	}

	var responses []*Response
	for _, req := range reqs {
		if req == nil {
			// null values in batch are invalid
			responses = append(responses, NewErrorResponse(nil, ErrInvalidReq))
			continue
		}

		resp := s.Handle(req)
		// Don't include notifications in response
		if !req.IsNotification() {
			responses = append(responses, resp)
		}
	}

	return responses
}

// ServeHTTP implements http.Handler for POST /mcp endpoint.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Use json.Decoder for efficiency
	var rawMessage json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&rawMessage); err != nil {
		s.logger.Debug("failed to decode request", "error", err)
		s.writeError(w, nil, ErrParse)
		return
	}

	// Check if it's a batch request
	if len(rawMessage) > 0 && rawMessage[0] == '[' {
		// Batch request
		var batchReqs []*Request
		if err := json.Unmarshal(rawMessage, &batchReqs); err != nil {
			s.logger.Debug("failed to parse batch request", "error", err)
			s.writeError(w, nil, ErrParse)
			return
		}

		// Validate and fix up requests
		for _, req := range batchReqs {
			if req == nil {
				continue
			}
			if req.JSONRPC == "" {
				req.JSONRPC = "2.0"
			}
		}

		responses := s.HandleBatch(batchReqs)
		s.writeResponses(w, responses)
	} else {
		// Single request
		var req Request
		if err := json.Unmarshal(rawMessage, &req); err != nil {
			s.logger.Debug("failed to parse request", "error", err)
			s.writeError(w, nil, ErrParse)
			return
		}

		// Set default jsonrpc version if not provided
		if req.JSONRPC == "" {
			req.JSONRPC = "2.0"
		}

		// Handle notification (no response needed)
		if req.IsNotification() {
			s.Handle(&req)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		resp := s.Handle(&req)
		s.writeResponse(w, resp)
	}
}

// writeResponse writes a single JSON-RPC response.
func (s *Server) writeResponse(w http.ResponseWriter, resp *Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Error("failed to encode response", "error", err)
	}
}

// writeResponses writes a batch of JSON-RPC responses.
func (s *Server) writeResponses(w http.ResponseWriter, responses []*Response) {
	if len(responses) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(responses); err != nil {
		s.logger.Error("failed to encode batch response", "error", err)
	}
}

// writeError writes a JSON-RPC error response.
func (s *Server) writeError(w http.ResponseWriter, id *RequestID, err *Error) {
	resp := NewErrorResponse(id, err)
	s.writeResponse(w, resp)
}

// registerBuiltinHandlers registers the built-in MCP handlers.
func (s *Server) registerBuiltinHandlers() {
	// initialize handler
	s.Register("initialize", s.handleInitialize)
}

// handleInitialize handles the MCP initialize request.
func (s *Server) handleInitialize(req *Request) (any, *Error) {
	// Parse params
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
		Capabilities    any    `json:"capabilities,omitempty"`
		ClientInfo      any    `json:"clientInfo,omitempty"`
	}

	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, NewError(ErrInvalidParams, "Invalid params: "+err.Error())
		}
	}

	s.logger.Debug("MCP initialize",
		"protocol_version", params.ProtocolVersion,
		"client", params.ClientInfo,
	)

	// Build response
	result := map[string]any{
		"protocolVersion": "2024-11-05", // MCP protocol version
		"serverInfo": map[string]string{
			"name":    s.name,
			"version": s.version,
		},
		"capabilities": map[string]any{
			"tools": map[string]any{},
			"resources": map[string]any{
				"subscribe":   false,
				"listChanged": false,
			},
		},
	}

	return result, nil
}

// GetHandler returns the handler for a method (for testing).
func (s *Server) GetHandler(method string) Handler {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.handlers[method]
}

// HandlerNames returns a list of registered handler names (for testing).
func (s *Server) HandlerNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.handlers))
	for name := range s.handlers {
		names = append(names, name)
	}
	return names
}
