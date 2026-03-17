package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ersinkoc/tentaserve/internal"
)

func TestRequestID(t *testing.T) {
	middleware := NewRequestID("X-Request-ID")

	handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that request ID is in context
		requestID := internal.RequestID(r.Context())
		if requestID == "" {
			t.Error("expected request ID in context")
		}
		if !strings.HasPrefix(requestID, "req_") {
			t.Errorf("expected request ID to start with 'req_', got %s", requestID)
		}

		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Check response has request ID header
	responseID := rec.Header().Get("X-Request-ID")
	if responseID == "" {
		t.Error("expected X-Request-ID header in response")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestRequestIDPropagation(t *testing.T) {
	middleware := NewRequestID("X-Request-ID")

	handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := internal.RequestID(r.Context())
		if requestID != "client-provided-id" {
			t.Errorf("expected request ID 'client-provided-id', got %s", requestID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "client-provided-id")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Check response preserves client request ID
	responseID := rec.Header().Get("X-Request-ID")
	if responseID != "client-provided-id" {
		t.Errorf("expected request ID 'client-provided-id', got %s", responseID)
	}
}

func TestGenerateRequestID(t *testing.T) {
	id1 := GenerateRequestID()
	id2 := GenerateRequestID()

	if id1 == "" {
		t.Error("expected non-empty request ID")
	}
	if id1 == id2 {
		t.Error("expected unique request IDs")
	}
	if !strings.HasPrefix(id1, "req_") {
		t.Errorf("expected request ID to start with 'req_', got %s", id1)
	}
	// req_ + 24 hex chars = 27 chars
	if len(id1) != 28 {
		t.Errorf("expected request ID length 28, got %d", len(id1))
	}
}

func TestChain(t *testing.T) {
	var order []string

	middleware1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "before1")
			next.ServeHTTP(w, r)
			order = append(order, "after1")
		})
	}

	middleware2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "before2")
			next.ServeHTTP(w, r)
			order = append(order, "after2")
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
	})

	chained := Chain(handler, middleware1, middleware2)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	chained.ServeHTTP(rec, req)

	// middleware1 runs first (outer), then middleware2, then handler
	expected := []string{"before1", "before2", "handler", "after2", "after1"}
	if len(order) != len(expected) {
		t.Errorf("expected order %v, got %v", expected, order)
	}
	for i, v := range expected {
		if i >= len(order) || order[i] != v {
			t.Errorf("expected order[%d] = %s, got %s", i, v, order[i])
		}
	}
}
