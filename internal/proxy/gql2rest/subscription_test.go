package gql2rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ersinkoc/tentaserve/internal/graphql"
)

// mockSubscriptionClient implements SubscriptionClient for testing.
type mockSubscriptionClient struct {
	events []*SubscriptionEvent
	err    error
	query  string
	vars   map[string]interface{}
}

func (m *mockSubscriptionClient) Subscribe(ctx context.Context, query string, variables map[string]interface{}) (<-chan *SubscriptionEvent, error) {
	m.query = query
	m.vars = variables

	if m.err != nil {
		return nil, m.err
	}

	ch := make(chan *SubscriptionEvent, len(m.events))
	go func() {
		defer close(ch)
		for _, event := range m.events {
			select {
			case <-ctx.Done():
				return
			case ch <- event:
			}
		}
	}()
	return ch, nil
}

func TestSubscriptionHandler_ServeHTTP(t *testing.T) {
	t.Run("successful subscription stream", func(t *testing.T) {
		events := []*SubscriptionEvent{
			{Data: json.RawMessage(`{"orderUpdated": {"id": "1", "status": "shipped"}}`)},
			{Data: json.RawMessage(`{"orderUpdated": {"id": "1", "status": "delivered"}}`)},
		}

		client := &mockSubscriptionClient{events: events}
		handler := NewSubscriptionHandler(SubscriptionHandlerOptions{
			BasePath: "/api/stream",
			Endpoints: []SubscriptionEndpoint{
				{
					Path:  "order-updated",
					Field: "orderUpdated",
					Arguments: []Argument{
						{Name: "orderId", Type: "string", Location: "query"},
					},
				},
			},
			Client: client,
		})

		req := httptest.NewRequest("GET", "/api/stream/order-updated?orderId=456", nil)
		req.Header.Set("Accept", "text/event-stream")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
			t.Errorf("expected Content-Type text/event-stream, got %s", ct)
		}

		body := w.Body.String()
		if !strings.Contains(body, "event: message") {
			t.Error("expected event: message in body")
		}
		// Data is unwrapped from the orderUpdated field
		if !strings.Contains(body, `"id"`) {
			t.Error("expected id field in unwrapped body")
		}
		if !strings.Contains(body, "event: complete") {
			t.Error("expected complete event in body")
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		handler := NewSubscriptionHandler(SubscriptionHandlerOptions{})

		req := httptest.NewRequest("POST", "/api/stream/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})

	t.Run("endpoint not found", func(t *testing.T) {
		handler := NewSubscriptionHandler(SubscriptionHandlerOptions{
			BasePath:  "/api/stream",
			Endpoints: []SubscriptionEndpoint{},
		})

		req := httptest.NewRequest("GET", "/api/stream/nonexistent", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})

	t.Run("upstream error", func(t *testing.T) {
		client := &mockSubscriptionClient{err: fmt.Errorf("connection refused")}
		handler := NewSubscriptionHandler(SubscriptionHandlerOptions{
			BasePath: "/api/stream",
			Endpoints: []SubscriptionEndpoint{
				{Path: "test", Field: "test"},
			},
			Client: client,
		})

		req := httptest.NewRequest("GET", "/api/stream/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadGateway {
			t.Errorf("expected 502, got %d", w.Code)
		}
	})

	t.Run("error events streamed", func(t *testing.T) {
		events := []*SubscriptionEvent{
			{Errors: []GraphQLError{{Message: "something went wrong"}}},
		}

		client := &mockSubscriptionClient{events: events}
		handler := NewSubscriptionHandler(SubscriptionHandlerOptions{
			BasePath: "/api/stream",
			Endpoints: []SubscriptionEndpoint{
				{Path: "test", Field: "test"},
			},
			Client: client,
		})

		req := httptest.NewRequest("GET", "/api/stream/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		body := w.Body.String()
		if !strings.Contains(body, "event: error") {
			t.Error("expected error event in body")
		}
		if !strings.Contains(body, "something went wrong") {
			t.Error("expected error message in body")
		}
	})

	t.Run("path parameter matching", func(t *testing.T) {
		events := []*SubscriptionEvent{
			{Data: json.RawMessage(`{"orderUpdated": {"status": "shipped"}}`)},
		}

		client := &mockSubscriptionClient{events: events}
		handler := NewSubscriptionHandler(SubscriptionHandlerOptions{
			BasePath: "/api/stream",
			Endpoints: []SubscriptionEndpoint{
				{
					Path:  "order-updated/{orderId}",
					Field: "orderUpdated",
					Arguments: []Argument{
						{Name: "orderId", Type: "string", Location: "path"},
					},
				},
			},
			Client: client,
		})

		req := httptest.NewRequest("GET", "/api/stream/order-updated/456", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		// Verify the subscription query includes the variable
		if client.vars["orderId"] != "456" {
			t.Errorf("expected orderId=456, got %v", client.vars["orderId"])
		}
	})
}

func TestSubscriptionHandler_BuildSubscriptionQuery(t *testing.T) {
	handler := NewSubscriptionHandler(SubscriptionHandlerOptions{})

	tests := []struct {
		name     string
		endpoint SubscriptionEndpoint
		params   map[string]string
		queryStr string
		wantSub  string
	}{
		{
			name: "simple subscription no args",
			endpoint: SubscriptionEndpoint{
				Field: "newMessage",
			},
			params:  map[string]string{},
			wantSub: "subscription { newMessage {} }",
		},
		{
			name: "subscription with query args",
			endpoint: SubscriptionEndpoint{
				Field: "orderUpdated",
				Arguments: []Argument{
					{Name: "orderId", Type: "string", Required: true},
				},
			},
			params:   map[string]string{},
			queryStr: "orderId=123",
			wantSub:  "subscription($orderId: String!) { orderUpdated(orderId: $orderId) {} }",
		},
		{
			name: "subscription with path params",
			endpoint: SubscriptionEndpoint{
				Field: "orderUpdated",
				Arguments: []Argument{
					{Name: "orderId", Type: "string", Required: false},
				},
			},
			params:  map[string]string{"orderId": "789"},
			wantSub: "subscription($orderId: String) { orderUpdated(orderId: $orderId) {} }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/api/stream/test"
			if tt.queryStr != "" {
				url += "?" + tt.queryStr
			}
			req := httptest.NewRequest("GET", url, nil)
			query, _ := handler.buildSubscriptionQuery(req, &tt.endpoint, tt.params)

			if query != tt.wantSub {
				t.Errorf("got query:\n  %s\nwant:\n  %s", query, tt.wantSub)
			}
		})
	}
}

func TestGenerateSubscriptionEndpoints(t *testing.T) {
	t.Run("schema with subscriptions", func(t *testing.T) {
		schema := &graphql.Schema{
			SubscriptionType: &graphql.TypeRef{Name: "Subscription"},
			Types: []graphql.IntrospectionType{
				{
					Name: "Subscription",
					Kind: "OBJECT",
					Fields: []graphql.IntrospectionField{
						{
							Name:        "orderUpdated",
							Description: "Notifies when an order is updated",
							Args: []graphql.InputValue{
								{
									Name: "orderId",
									Type: graphql.TypeRef{Kind: "NON_NULL", OfType: &graphql.TypeRef{Kind: "SCALAR", Name: "ID"}},
								},
							},
							Type: graphql.TypeRef{Kind: "OBJECT", Name: "Order"},
						},
						{
							Name:        "newMessage",
							Description: "New chat message",
							Type:        graphql.TypeRef{Kind: "OBJECT", Name: "Message"},
						},
					},
				},
			},
		}

		endpoints := GenerateSubscriptionEndpoints(schema)

		if len(endpoints) != 2 {
			t.Fatalf("expected 2 endpoints, got %d", len(endpoints))
		}

		// Check orderUpdated endpoint
		ep := endpoints[0]
		if ep.Field != "orderUpdated" {
			t.Errorf("expected field orderUpdated, got %s", ep.Field)
		}
		if !strings.Contains(ep.Path, "order-updated") {
			t.Errorf("expected path containing order-updated, got %s", ep.Path)
		}
		if len(ep.Arguments) != 1 {
			t.Errorf("expected 1 argument, got %d", len(ep.Arguments))
		}
		if ep.Arguments[0].Name != "orderId" {
			t.Errorf("expected argument orderId, got %s", ep.Arguments[0].Name)
		}

		// Check newMessage endpoint
		ep2 := endpoints[1]
		if ep2.Field != "newMessage" {
			t.Errorf("expected field newMessage, got %s", ep2.Field)
		}
		if ep2.Path != "new-message" {
			t.Errorf("expected path new-message, got %s", ep2.Path)
		}
	})

	t.Run("schema without subscriptions", func(t *testing.T) {
		schema := &graphql.Schema{
			QueryType: &graphql.TypeRef{Name: "Query"},
		}
		endpoints := GenerateSubscriptionEndpoints(schema)
		if endpoints != nil {
			t.Errorf("expected nil endpoints, got %v", endpoints)
		}
	})

	t.Run("nil schema", func(t *testing.T) {
		endpoints := GenerateSubscriptionEndpoints(nil)
		if endpoints != nil {
			t.Errorf("expected nil endpoints, got %v", endpoints)
		}
	})
}

func TestSubscriptionHandler_ActiveSubscriptions(t *testing.T) {
	handler := NewSubscriptionHandler(SubscriptionHandlerOptions{})
	if handler.ActiveSubscriptions() != 0 {
		t.Errorf("expected 0 active subscriptions initially")
	}
}

func TestSubscriptionHandler_Close(t *testing.T) {
	handler := NewSubscriptionHandler(SubscriptionHandlerOptions{})

	// Add some mock subscriptions
	_, cancel1 := context.WithCancel(context.Background())
	_, cancel2 := context.WithCancel(context.Background())

	handler.mu.Lock()
	handler.activeSubs["sub-1"] = cancel1
	handler.activeSubs["sub-2"] = cancel2
	handler.mu.Unlock()

	if handler.ActiveSubscriptions() != 2 {
		t.Errorf("expected 2 active subscriptions")
	}

	handler.Close()

	if handler.ActiveSubscriptions() != 0 {
		t.Errorf("expected 0 active subscriptions after close")
	}
}

func TestMatchSubscriptionPath(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    map[string]string
	}{
		{"/api/stream/test", "/api/stream/test", map[string]string{}},
		{"/api/stream/order/{id}", "/api/stream/order/123", map[string]string{"id": "123"}},
		{"/api/stream/order/{id}", "/api/stream/test/123", nil},
		{"/api/stream/a/b", "/api/stream/a", nil},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"->"+tt.path, func(t *testing.T) {
			got := matchSubscriptionPath(tt.pattern, tt.path)
			if tt.want == nil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected %v, got nil", tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("param %s: expected %s, got %s", k, v, got[k])
				}
			}
		})
	}
}

func TestToKebabCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"orderUpdated", "order-updated"},
		{"newMessage", "new-message"},
		{"simple", "simple"},
		{"ABCDef", "a-b-c-def"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toKebabCase(tt.input)
			if got != tt.want {
				t.Errorf("toKebabCase(%s) = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}

func TestMapJSONTypeToGraphQL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"string", "String"},
		{"integer", "Int"},
		{"number", "Float"},
		{"boolean", "Boolean"},
		{"object", "String"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapJSONTypeToGraphQL(tt.input)
			if got != tt.want {
				t.Errorf("mapJSONTypeToGraphQL(%s) = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}

func TestHTTPSubscriptionClient(t *testing.T) {
	t.Run("connects to SSE upstream", func(t *testing.T) {
		// Create a mock upstream that sends SSE events
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "no flush", 500)
				return
			}

			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			flusher.Flush()

			// Send two events
			fmt.Fprintf(w, "data: {\"data\":{\"test\":\"event1\"}}\n\n")
			flusher.Flush()

			fmt.Fprintf(w, "data: {\"data\":{\"test\":\"event2\"}}\n\n")
			flusher.Flush()
		}))
		defer server.Close()

		client := NewHTTPSubscriptionClient(server.URL)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		events, err := client.Subscribe(ctx, "subscription { test }", nil)
		if err != nil {
			t.Fatalf("subscribe failed: %v", err)
		}

		var received []*SubscriptionEvent
		for event := range events {
			received = append(received, event)
		}

		if len(received) < 1 {
			t.Errorf("expected at least 1 event, got %d", len(received))
		}
	})

	t.Run("handles upstream error status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "subscription not supported", http.StatusBadRequest)
		}))
		defer server.Close()

		client := NewHTTPSubscriptionClient(server.URL)

		ctx := context.Background()
		_, err := client.Subscribe(ctx, "subscription { test }", nil)
		if err == nil {
			t.Error("expected error for bad status")
		}
		if !strings.Contains(err.Error(), "400") {
			t.Errorf("expected status 400 in error, got: %s", err.Error())
		}
	})
}

func TestUnwrapEventData(t *testing.T) {
	handler := NewSubscriptionHandler(SubscriptionHandlerOptions{})

	t.Run("unwraps nested field", func(t *testing.T) {
		data := json.RawMessage(`{"orderUpdated": {"id": "1", "status": "shipped"}}`)
		result := handler.unwrapEventData(data, "orderUpdated")

		var parsed map[string]interface{}
		if err := json.Unmarshal(result, &parsed); err != nil {
			t.Fatalf("failed to parse result: %v", err)
		}
		if parsed["status"] != "shipped" {
			t.Errorf("expected status=shipped, got %v", parsed["status"])
		}
	})

	t.Run("returns raw data when field not found", func(t *testing.T) {
		data := json.RawMessage(`{"other": "value"}`)
		result := handler.unwrapEventData(data, "missing")

		if string(result) != `{"other": "value"}` {
			t.Errorf("expected raw data back, got %s", string(result))
		}
	})

	t.Run("handles empty data", func(t *testing.T) {
		result := handler.unwrapEventData(nil, "test")
		if string(result) != "null" {
			t.Errorf("expected null, got %s", string(result))
		}
	})
}
