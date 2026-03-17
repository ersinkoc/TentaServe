package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// SSETransport implements Server-Sent Events transport for MCP.
type SSETransport struct {
	server     *Server
	logger     *slog.Logger
	sessions   map[string]*SSESession
	sessionsMu sync.RWMutex
	endpoint   string
}

// SSESession represents an active SSE connection.
type SSESession struct {
	ID       string
	Writer   http.ResponseWriter
	Flusher  http.Flusher
	Done     chan struct{}
	Messages chan *SSEMessage
}

// SSEMessage represents a message to be sent via SSE.
type SSEMessage struct {
	Event string
	Data  []byte
}

// NewSSETransport creates a new SSE transport.
func NewSSETransport(server *Server, logger *slog.Logger, endpoint string) *SSETransport {
	if logger == nil {
		logger = slog.Default()
	}
	return &SSETransport{
		server:   server,
		logger:   logger,
		sessions: make(map[string]*SSESession),
		endpoint: endpoint,
	}
}

// ServeHTTP handles SSE connections and message posting.
func (t *SSETransport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		t.handleSSE(w, r)
	case http.MethodPost:
		t.handleMessage(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSSE handles SSE connection establishment.
func (t *SSETransport) handleSSE(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Create session
	sessionID := generateSessionID()
	session := &SSESession{
		ID:       sessionID,
		Writer:   w,
		Flusher:  flusher,
		Done:     make(chan struct{}),
		Messages: make(chan *SSEMessage, 100),
	}

	t.sessionsMu.Lock()
	t.sessions[sessionID] = session
	t.sessionsMu.Unlock()

	// Cleanup on disconnect
	defer func() {
		t.sessionsMu.Lock()
		delete(t.sessions, sessionID)
		t.sessionsMu.Unlock()
		close(session.Done)
	}()

	// Send endpoint event
	endpointMsg := &SSEMessage{
		Event: "endpoint",
		Data:  []byte(fmt.Sprintf("%s?session=%s", t.endpoint, sessionID)),
	}
	t.sendSSEMessage(w, flusher, endpointMsg)

	// Send initial notification
	info := t.server.ServerInfo()
	if info.Name != "" {
		infoData, _ := json.Marshal(info)
		t.sendSSEMessage(w, flusher, &SSEMessage{
			Event: "notification",
			Data:  infoData,
		})
	}

	// Keep connection alive and send messages
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-session.Done:
			return
		case msg := <-session.Messages:
			t.sendSSEMessage(w, flusher, msg)
		case <-ticker.C:
			// Send keepalive comment
			fmt.Fprintf(w, ":keepalive\n\n")
			flusher.Flush()
		}
	}
}

// handleMessage handles POST requests with JSON-RPC messages.
func (t *SSETransport) handleMessage(w http.ResponseWriter, r *http.Request) {
	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Get session ID from query param
	sessionID := r.URL.Query().Get("session")
	if sessionID == "" {
		http.Error(w, "Missing session ID", http.StatusBadRequest)
		return
	}

	// Check if session exists
	t.sessionsMu.RLock()
	_, exists := t.sessions[sessionID]
	t.sessionsMu.RUnlock()

	if !exists {
		http.Error(w, "Invalid session ID", http.StatusNotFound)
		return
	}

	// Parse and handle request
	req, err := ParseRequest(body)
	if err != nil {
		// Try batch
		reqs, err := ParseRequestBatch(body)
		if err != nil {
			t.writeJSONRPCError(w, nil, ErrParse)
			return
		}
		if reqs != nil {
			// Handle batch
			responses := t.server.HandleBatch(reqs)
			w.Header().Set("Content-Type", "application/json")
			data, _ := SerializeResponseBatch(responses)
			w.Write(data)
			return
		}
		// Single request parse error
		t.writeJSONRPCError(w, nil, ErrParse)
		return
	}

	// Handle single request
	resp := t.server.Handle(req)

	// Only send response if not a notification
	if !req.IsNotification() && resp != nil {
		w.Header().Set("Content-Type", "application/json")
		data, _ := SerializeResponse(resp)
		w.Write(data)
	} else {
		// Empty response for notifications
		w.WriteHeader(http.StatusAccepted)
	}
}

// sendSSEMessage sends a message via SSE.
func (t *SSETransport) sendSSEMessage(w http.ResponseWriter, flusher http.Flusher, msg *SSEMessage) {
	if msg.Event != "" {
		fmt.Fprintf(w, "event: %s\n", msg.Event)
	}
	fmt.Fprintf(w, "data: %s\n\n", string(msg.Data))
	flusher.Flush()
}

// writeJSONRPCError writes a JSON-RPC error response.
func (t *SSETransport) writeJSONRPCError(w http.ResponseWriter, id *RequestID, rpcErr *Error) {
	resp := NewErrorResponse(id, rpcErr)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	data, _ := SerializeResponse(resp)
	w.Write(data)
}

// Broadcast sends a message to all connected sessions.
func (t *SSETransport) Broadcast(event string, data []byte) {
	t.sessionsMu.RLock()
	sessions := make([]*SSESession, 0, len(t.sessions))
	for _, s := range t.sessions {
		sessions = append(sessions, s)
	}
	t.sessionsMu.RUnlock()

	msg := &SSEMessage{
		Event: event,
		Data:  data,
	}

	for _, session := range sessions {
		select {
		case session.Messages <- msg:
		default:
			// Channel full, drop message
		}
	}
}

// SessionCount returns the number of active sessions.
func (t *SSETransport) SessionCount() int {
	t.sessionsMu.RLock()
	defer t.sessionsMu.RUnlock()
	return len(t.sessions)
}

// generateSessionID generates a unique session ID.
func generateSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
