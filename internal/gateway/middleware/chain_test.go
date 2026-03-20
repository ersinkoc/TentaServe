package middleware

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestNewChain tests chain creation.
func TestNewChain(t *testing.T) {
	c := NewChain()
	if c.Len() != 0 {
		t.Errorf("Expected empty chain, got %d middlewares", c.Len())
	}

	m1 := func(next http.Handler) http.Handler { return next }
	m2 := func(next http.Handler) http.Handler { return next }

	c = NewChain(m1, m2)
	if c.Len() != 2 {
		t.Errorf("Expected 2 middlewares, got %d", c.Len())
	}
}

// TestChainAppend tests appending middleware to a chain.
func TestChainAppend(t *testing.T) {
	m1 := func(next http.Handler) http.Handler { return next }
	m2 := func(next http.Handler) http.Handler { return next }
	m3 := func(next http.Handler) http.Handler { return next }

	c1 := NewChain(m1)
	c2 := c1.Append(m2, m3)

	if c1.Len() != 1 {
		t.Error("Original chain should not be modified")
	}

	if c2.Len() != 3 {
		t.Errorf("Expected 3 middlewares, got %d", c2.Len())
	}
}

// TestChainThen tests applying a chain to a handler.
func TestChainThen(t *testing.T) {
	// Track middleware execution order
	var order []string

	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m1-before")
			next.ServeHTTP(w, r)
			order = append(order, "m1-after")
		})
	}

	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m2-before")
			next.ServeHTTP(w, r)
			order = append(order, "m2-after")
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
	})

	chain := NewChain(m1, m2)
	wrapped := chain.Then(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	expected := []string{"m1-before", "m2-before", "handler", "m2-after", "m1-after"}
	if len(order) != len(expected) {
		t.Fatalf("Expected order %v, got %v", expected, order)
	}

	for i, v := range expected {
		if order[i] != v {
			t.Errorf("Expected order[%d] = %s, got %s", i, v, order[i])
		}
	}
}

// TestChainThenNil tests applying a chain to a nil handler.
func TestChainThenNil(t *testing.T) {
	chain := NewChain()
	wrapped := chain.Then(nil)

	if wrapped == nil {
		t.Error("Expected non-nil handler for nil input")
	}
}

// TestChainThenFunc tests applying a chain to a handler function.
func TestChainThenFunc(t *testing.T) {
	called := false
	handler := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}

	chain := NewChain()
	wrapped := chain.ThenFunc(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if !called {
		t.Error("Handler was not called")
	}
}

// TestRecovery tests panic recovery middleware.
func TestRecovery(t *testing.T) {
	mockLogger := &mockLogger{}

	recovery := Recovery(mockLogger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	})

	wrapped := recovery(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	// Should not panic
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	if !mockLogger.errorCalled {
		t.Error("Expected error to be logged")
	}
}

// TestRecoveryNoPanic tests recovery middleware when no panic occurs.
func TestRecoveryNoPanic(t *testing.T) {
	mockLogger := &mockLogger{}
	recovery := Recovery(mockLogger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	wrapped := recovery(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if mockLogger.errorCalled {
		t.Error("Expected no error to be logged")
	}
}

// TestResponseCapture tests response capturing.
func TestResponseCapture(t *testing.T) {
	base := httptest.NewRecorder()
	capture := NewResponseCapture(base)

	// Test initial state
	if capture.StatusCode != http.StatusOK {
		t.Errorf("Expected initial status %d, got %d", http.StatusOK, capture.StatusCode)
	}

	if capture.Written() {
		t.Error("Expected not written initially")
	}

	// Test writing
	capture.WriteHeader(http.StatusCreated)
	if capture.StatusCode != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, capture.StatusCode)
	}

	if !capture.Written() {
		t.Error("Expected written after WriteHeader")
	}

	// Test body capture
	body := []byte("Hello, World!")
	n, err := capture.Write(body)
	if err != nil {
		t.Errorf("Write error: %v", err)
	}
	if n != len(body) {
		t.Errorf("Expected %d bytes written, got %d", len(body), n)
	}

	if !bytes.Equal(capture.Body.Bytes(), body) {
		t.Errorf("Expected body %q, got %q", body, capture.Body.Bytes())
	}
}

// TestResponseCaptureAutoHeader tests auto WriteHeader on Write.
func TestResponseCaptureAutoHeader(t *testing.T) {
	base := httptest.NewRecorder()
	capture := NewResponseCapture(base)

	// Write without WriteHeader should auto-set status
	capture.Write([]byte("test"))

	if capture.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, capture.StatusCode)
	}
}

// TestCaptureMiddleware tests the capture middleware.
func TestCaptureMiddleware(t *testing.T) {
	var captured *ResponseCapture

	captureMiddleware := Capture(func(rc *ResponseCapture) {
		captured = rc
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("response body"))
	})

	wrapped := captureMiddleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if captured == nil {
		t.Fatal("Expected capture to be set")
	}

	if captured.StatusCode != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, captured.StatusCode)
	}

	if !bytes.Equal(captured.Body.Bytes(), []byte("response body")) {
		t.Errorf("Expected body %q, got %q", "response body", captured.Body.Bytes())
	}
}

// TestSkipMiddleware tests the skip middleware.
func TestSkipMiddleware(t *testing.T) {
	innerCalled := false

	innerMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			innerCalled = true
			next.ServeHTTP(w, r)
		})
	}

	// Skip for /skip path
	condition := func(r *http.Request) bool {
		return r.URL.Path == "/skip"
	}

	skipMiddleware := Skip(condition, innerMiddleware)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := skipMiddleware(handler)

	// Test path that should skip
	req1 := httptest.NewRequest("GET", "/skip", nil)
	rec1 := httptest.NewRecorder()
	innerCalled = false
	wrapped.ServeHTTP(rec1, req1)

	if innerCalled {
		t.Error("Inner middleware should be skipped for /skip")
	}

	// Test path that should not skip
	req2 := httptest.NewRequest("GET", "/other", nil)
	rec2 := httptest.NewRecorder()
	innerCalled = false
	wrapped.ServeHTTP(rec2, req2)

	if !innerCalled {
		t.Error("Inner middleware should be called for /other")
	}
}

// TestIfMiddleware tests the conditional middleware.
func TestIfMiddleware(t *testing.T) {
	innerCalled := false

	innerMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			innerCalled = true
			next.ServeHTTP(w, r)
		})
	}

	// Apply for /apply path
	condition := func(r *http.Request) bool {
		return r.URL.Path == "/apply"
	}

	ifMiddleware := If(condition, innerMiddleware)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := ifMiddleware(handler)

	// Test path that should apply middleware
	req1 := httptest.NewRequest("GET", "/apply", nil)
	rec1 := httptest.NewRecorder()
	innerCalled = false
	wrapped.ServeHTTP(rec1, req1)

	if !innerCalled {
		t.Error("Inner middleware should be applied for /apply")
	}

	// Test path that should skip middleware
	req2 := httptest.NewRequest("GET", "/other", nil)
	rec2 := httptest.NewRecorder()
	innerCalled = false
	wrapped.ServeHTTP(rec2, req2)

	if innerCalled {
		t.Error("Inner middleware should be skipped for /other")
	}
}

// TestCombine tests combining multiple middleware.
func TestCombine(t *testing.T) {
	var order []string

	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m1")
			next.ServeHTTP(w, r)
		})
	}

	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m2")
			next.ServeHTTP(w, r)
		})
	}

	combined := Combine(m1, m2)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
	})

	wrapped := combined(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	expected := []string{"m1", "m2", "handler"}
	if len(order) != len(expected) {
		t.Fatalf("Expected order %v, got %v", expected, order)
	}
}

// TestNoOp tests the no-op middleware.
func TestNoOp(t *testing.T) {
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	wrapped := NoOp(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if !called {
		t.Error("Handler should be called")
	}
}

// TestWrap tests wrapping a standard middleware function.
func TestWrap(t *testing.T) {
	fn := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test", "wrapped")
			next.ServeHTTP(w, r)
		})
	}

	wrapped := Wrap(fn)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	chain := NewChain(wrapped)
	final := chain.Then(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	final.ServeHTTP(rec, req)

	if rec.Header().Get("X-Test") != "wrapped" {
		t.Error("Expected X-Test header to be set")
	}
}

// BenchmarkChain benchmarks the chain middleware.
func BenchmarkChain(b *testing.B) {
	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}

	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}

	m3 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}

	chain := NewChain(m1, m2, m3)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := chain.Then(handler)

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}
}

// mockLogger is a mock logger for testing.
type mockLogger struct {
	errorCalled bool
}

func (m *mockLogger) Error(ctx interface{}, msg string, keysAndValues ...interface{}) {
	m.errorCalled = true
}

func (m *mockLogger) Info(ctx interface{}, msg string, keysAndValues ...interface{})  {}
func (m *mockLogger) Debug(ctx interface{}, msg string, keysAndValues ...interface{}) {}
func (m *mockLogger) Warn(ctx interface{}, msg string, keysAndValues ...interface{})  {}

// errorString returns an error with the given message.
func errorString(s string) error {
	return errors.New(s)
}
