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
