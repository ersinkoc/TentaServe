package mcp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestNewSSETransport tests SSE transport creation.
func TestNewSSETransport(t *testing.T) {
	server := NewServer(nil, nil)
	transport := NewSSETransport(server, nil, "/mcp")

	if transport == nil {
		t.Fatal("Expected non-nil transport")
	}
	if transport.server != server {
		t.Error("Server not set correctly")
	}
	if transport.endpoint != "/mcp" {
		t.Errorf("Expected endpoint '/mcp', got %s", transport.endpoint)
	}
	if transport.sessions == nil {
		t.Error("Sessions map not initialized")
	}
}

// TestSSETransportSessionCount tests session counting.
func TestSSETransportSessionCount(t *testing.T) {
	server := NewServer(nil, nil)
	transport := NewSSETransport(server, nil, "/mcp")

	if transport.SessionCount() != 0 {
		t.Errorf("Expected 0 sessions, got %d", transport.SessionCount())
	}

	// Add a session manually
	transport.sessionsMu.Lock()
	transport.sessions["test-session"] = &SSESession{
		ID: "test-session",
	}
	transport.sessionsMu.Unlock()

	if transport.SessionCount() != 1 {
		t.Errorf("Expected 1 session, got %d", transport.SessionCount())
	}
}

// TestSSETransportServeHTTPMethodNotAllowed tests method validation.
func TestSSETransportServeHTTPMethodNotAllowed(t *testing.T) {
	server := NewServer(nil, nil)
	transport := NewSSETransport(server, nil, "/mcp")

	// Test PUT method (not allowed)
	req := httptest.NewRequest(http.MethodPut, "/mcp", nil)
	w := httptest.NewRecorder()

	transport.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// TestSSETransportPOSTMissingSession tests POST without session.
func TestSSETransportPOSTMissingSession(t *testing.T) {
	server := NewServer(nil, nil)
	transport := NewSSETransport(server, nil, "/mcp")

	// POST without session parameter
	body := `{"jsonrpc":"2.0","method":"initialize","id":1}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	w := httptest.NewRecorder()

	transport.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestGenerateSessionID tests session ID generation.
func TestGenerateSessionID(t *testing.T) {
	id1 := generateSessionID()
	id2 := generateSessionID()

	if id1 == "" {
		t.Error("Expected non-empty session ID")
	}
	if id1 == id2 {
		t.Error("Expected unique session IDs")
	}
}

// TestSSESessionStructure tests SSE session fields.
func TestSSESessionStructure(t *testing.T) {
	session := &SSESession{
		ID:       "test-id",
		Done:     make(chan struct{}),
		Messages: make(chan *SSEMessage, 10),
	}

	if session.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got %s", session.ID)
	}
	if session.Done == nil {
		t.Error("Expected Done channel")
	}
	if session.Messages == nil {
		t.Error("Expected Messages channel")
	}
}

// TestSSEMessageStructure tests SSE message fields.
func TestSSEMessageStructure(t *testing.T) {
	msg := &SSEMessage{
		Event: "test",
		Data:  []byte("test data"),
	}

	if msg.Event != "test" {
		t.Errorf("Expected event 'test', got %s", msg.Event)
	}
	if string(msg.Data) != "test data" {
		t.Errorf("Expected data 'test data', got %s", string(msg.Data))
	}
}

// TestResourceStructure tests Resource struct fields.
func TestResourceStructure(t *testing.T) {
	r := &Resource{
		URI:         "test://resource",
		Name:        "Test Resource",
		Description: "A test resource",
		MIMEType:    "text/plain",
	}

	if r.URI != "test://resource" {
		t.Errorf("Expected URI 'test://resource', got %s", r.URI)
	}
	if r.Name != "Test Resource" {
		t.Errorf("Expected name 'Test Resource', got %s", r.Name)
	}
	if r.MIMEType != "text/plain" {
		t.Errorf("Expected MIME type 'text/plain', got %s", r.MIMEType)
	}
}

// TestResourceContentStructure tests ResourceContent struct fields.
func TestResourceContentStructure(t *testing.T) {
	c := ResourceContent{
		URI:      "test://resource",
		MIMEType: "text/plain",
		Text:     "Hello",
		Blob:     "base64data",
	}

	if c.URI != "test://resource" {
		t.Errorf("Expected URI 'test://resource', got %s", c.URI)
	}
	if c.Text != "Hello" {
		t.Errorf("Expected text 'Hello', got %s", c.Text)
	}
}

// TestSSETransportBroadcast tests message broadcasting.
func TestSSETransportBroadcast(t *testing.T) {
	server := NewServer(nil, nil)
	transport := NewSSETransport(server, nil, "/mcp")

	// Create a session with a message channel
	messages := make(chan *SSEMessage, 10)
	session := &SSESession{
		ID:       "test-session",
		Messages: messages,
	}

	transport.sessionsMu.Lock()
	transport.sessions["test-session"] = session
	transport.sessionsMu.Unlock()

	// Broadcast a message
	transport.Broadcast("test", []byte("hello"))

	// Check message was received
	select {
	case msg := <-messages:
		if msg.Event != "test" {
			t.Errorf("Expected event 'test', got %s", msg.Event)
		}
		if string(msg.Data) != "hello" {
			t.Errorf("Expected data 'hello', got %s", string(msg.Data))
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for message")
	}
}

// --- Additional tests for coverage ---

// TestSSETransportHandleSSE_NonFlusher tests handleSSE with a non-flusher writer.
func TestSSETransportHandleSSE_NonFlusher(t *testing.T) {
	server := NewServer(nil, nil)
	transport := NewSSETransport(server, nil, "/mcp")

	// Use a custom non-flusher writer
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	w := &nonFlusherWriter{httptest.NewRecorder()}

	transport.handleSSE(w, req)

	// Should get error about streaming not supported
	if w.rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.rec.Code)
	}
}

// nonFlusherWriter wraps httptest.ResponseRecorder but doesn't implement http.Flusher.
type nonFlusherWriter struct {
	rec *httptest.ResponseRecorder
}

func (w *nonFlusherWriter) Header() http.Header         { return w.rec.Header() }
func (w *nonFlusherWriter) Write(b []byte) (int, error) { return w.rec.Write(b) }
func (w *nonFlusherWriter) WriteHeader(code int)        { w.rec.WriteHeader(code) }

// TestSSETransportHandleMessage_InvalidSession tests POST with invalid session.
func TestSSETransportHandleMessage_InvalidSession(t *testing.T) {
	server := NewServer(nil, nil)
	transport := NewSSETransport(server, nil, "/mcp")

	body := `{"jsonrpc":"2.0","method":"test","id":1}`
	req := httptest.NewRequest(http.MethodPost, "/mcp?session=nonexistent", strings.NewReader(body))
	w := httptest.NewRecorder()

	transport.handleMessage(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// TestSSETransportHandleMessage_ValidSession tests POST with a valid session and request.
func TestSSETransportHandleMessage_ValidSession(t *testing.T) {
	server := NewServer(nil, nil)
	transport := NewSSETransport(server, nil, "/mcp")

	// Add a session
	transport.sessionsMu.Lock()
	transport.sessions["test-session"] = &SSESession{
		ID:       "test-session",
		Done:     make(chan struct{}),
		Messages: make(chan *SSEMessage, 10),
	}
	transport.sessionsMu.Unlock()

	// Send initialize request
	body := `{"jsonrpc":"2.0","method":"initialize","id":1}`
	req := httptest.NewRequest(http.MethodPost, "/mcp?session=test-session", strings.NewReader(body))
	w := httptest.NewRecorder()

	transport.handleMessage(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSSETransportHandleMessage_Notification tests POST with a notification (no ID).
func TestSSETransportHandleMessage_Notification(t *testing.T) {
	server := NewServer(nil, nil)
	// Register a handler for the notification method
	server.Register("notifications/test", func(req *Request) (any, *Error) {
		return nil, nil
	})
	transport := NewSSETransport(server, nil, "/mcp")

	// Add a session
	transport.sessionsMu.Lock()
	transport.sessions["test-session"] = &SSESession{
		ID:       "test-session",
		Done:     make(chan struct{}),
		Messages: make(chan *SSEMessage, 10),
	}
	transport.sessionsMu.Unlock()

	// Send notification (no id field)
	body := `{"jsonrpc":"2.0","method":"notifications/test"}`
	req := httptest.NewRequest(http.MethodPost, "/mcp?session=test-session", strings.NewReader(body))
	w := httptest.NewRecorder()

	transport.handleMessage(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("Expected status 202, got %d", w.Code)
	}
}

// TestSSETransportHandleMessage_InvalidJSON tests POST with invalid JSON body.
func TestSSETransportHandleMessage_InvalidJSON(t *testing.T) {
	server := NewServer(nil, nil)
	transport := NewSSETransport(server, nil, "/mcp")

	// Add a session
	transport.sessionsMu.Lock()
	transport.sessions["test-session"] = &SSESession{
		ID:       "test-session",
		Done:     make(chan struct{}),
		Messages: make(chan *SSEMessage, 10),
	}
	transport.sessionsMu.Unlock()

	body := `{invalid json`
	req := httptest.NewRequest(http.MethodPost, "/mcp?session=test-session", strings.NewReader(body))
	w := httptest.NewRecorder()

	transport.handleMessage(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestSSETransportHandleMessage_BatchRequest tests POST with batch JSON-RPC request.
func TestSSETransportHandleMessage_BatchRequest(t *testing.T) {
	server := NewServer(nil, nil)
	transport := NewSSETransport(server, nil, "/mcp")

	// Add a session
	transport.sessionsMu.Lock()
	transport.sessions["test-session"] = &SSESession{
		ID:       "test-session",
		Done:     make(chan struct{}),
		Messages: make(chan *SSEMessage, 10),
	}
	transport.sessionsMu.Unlock()

	body := `[{"jsonrpc":"2.0","method":"initialize","id":1},{"jsonrpc":"2.0","method":"initialize","id":2}]`
	req := httptest.NewRequest(http.MethodPost, "/mcp?session=test-session", strings.NewReader(body))
	w := httptest.NewRecorder()

	transport.handleMessage(w, req)

	// Should return OK with batch response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestWriteJSONRPCError tests writeJSONRPCError.
func TestWriteJSONRPCError(t *testing.T) {
	server := NewServer(nil, nil)
	transport := NewSSETransport(server, nil, "/mcp")

	w := httptest.NewRecorder()
	transport.writeJSONRPCError(w, NewRequestID(1), ErrParse)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", ct)
	}
}

// TestSendSSEMessage tests sendSSEMessage.
func TestSendSSEMessage(t *testing.T) {
	server := NewServer(nil, nil)
	transport := NewSSETransport(server, nil, "/mcp")

	w := httptest.NewRecorder()

	t.Run("with event", func(t *testing.T) {
		msg := &SSEMessage{Event: "test", Data: []byte("hello")}
		transport.sendSSEMessage(w, w, msg)
		body := w.Body.String()
		if !strings.Contains(body, "event: test") {
			t.Errorf("expected 'event: test' in body, got %q", body)
		}
		if !strings.Contains(body, "data: hello") {
			t.Errorf("expected 'data: hello' in body, got %q", body)
		}
	})

	t.Run("without event", func(t *testing.T) {
		w2 := httptest.NewRecorder()
		msg := &SSEMessage{Data: []byte("data-only")}
		transport.sendSSEMessage(w2, w2, msg)
		body := w2.Body.String()
		if strings.Contains(body, "event:") {
			t.Errorf("expected no event line, got %q", body)
		}
		if !strings.Contains(body, "data: data-only") {
			t.Errorf("expected 'data: data-only' in body, got %q", body)
		}
	})
}

// TestSSETransportBroadcast_FullChannel tests broadcast when channel is full.
func TestSSETransportBroadcast_FullChannel(t *testing.T) {
	server := NewServer(nil, nil)
	transport := NewSSETransport(server, nil, "/mcp")

	// Create a session with a very small buffer
	messages := make(chan *SSEMessage, 1)
	// Fill the channel
	messages <- &SSEMessage{Event: "old", Data: []byte("old")}

	session := &SSESession{
		ID:       "full-session",
		Messages: messages,
	}

	transport.sessionsMu.Lock()
	transport.sessions["full-session"] = session
	transport.sessionsMu.Unlock()

	// This should not block - message should be dropped
	transport.Broadcast("new", []byte("new"))

	// The channel should still have the old message
	msg := <-messages
	if msg.Event != "old" {
		t.Errorf("expected old message to still be there, got event %s", msg.Event)
	}
}
