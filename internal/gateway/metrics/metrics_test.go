package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestNewRegistry tests registry creation.
func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("Expected non-nil registry")
	}
	if r.metrics == nil {
		t.Error("Expected metrics map to be initialized")
	}
}

// TestRegisterCounter tests counter registration.
func TestRegisterCounter(t *testing.T) {
	r := NewRegistry()
	c := r.RegisterCounter("test_counter", "Test counter", "label1")

	if c == nil {
		t.Fatal("Expected non-nil counter")
	}

	m := r.Get("test_counter")
	if m == nil {
		t.Fatal("Expected metric to be registered")
	}
	if m.Type != Counter {
		t.Errorf("Expected type counter, got %s", m.Type)
	}
}

// TestRegisterGauge tests gauge registration.
func TestRegisterGauge(t *testing.T) {
	r := NewRegistry()
	g := r.RegisterGauge("test_gauge", "Test gauge", "label1")

	if g == nil {
		t.Fatal("Expected non-nil gauge")
	}

	m := r.Get("test_gauge")
	if m.Type != Gauge {
		t.Errorf("Expected type gauge, got %s", m.Type)
	}
}

// TestRegisterHistogram tests histogram registration.
func TestRegisterHistogram(t *testing.T) {
	r := NewRegistry()
	h := r.RegisterHistogram("test_histogram", "Test histogram", []float64{0.1, 0.5, 1}, "label1")

	if h == nil {
		t.Fatal("Expected non-nil histogram")
	}

	m := r.Get("test_histogram")
	if m.Type != Histogram {
		t.Errorf("Expected type histogram, got %s", m.Type)
	}
}

// TestCounterInc tests counter increment.
func TestCounterInc(t *testing.T) {
	r := NewRegistry()
	c := r.RegisterCounter("counter", "Counter", "method")

	c.Inc("GET")
	c.Inc("GET")
	c.Inc("POST")

	cv := c.With("GET")
	if cv.Value() != 2 {
		t.Errorf("Expected value 2, got %d", cv.Value())
	}

	cv2 := c.With("POST")
	if cv2.Value() != 1 {
		t.Errorf("Expected value 1, got %d", cv2.Value())
	}
}

// TestCounterAdd tests counter add.
func TestCounterAdd(t *testing.T) {
	r := NewRegistry()
	c := r.RegisterCounter("counter", "Counter")

	c.Add(5)
	c.Add(3)

	cv := c.With()
	if cv.Value() != 8 {
		t.Errorf("Expected value 8, got %d", cv.Value())
	}
}

// TestGaugeSet tests gauge set.
func TestGaugeSet(t *testing.T) {
	r := NewRegistry()
	g := r.RegisterGauge("gauge", "Gauge")

	g.Set(42.5)

	gv := g.With()
	if gv.Value() != 42.5 {
		t.Errorf("Expected value 42.5, got %f", gv.Value())
	}
}

// TestGaugeIncDec tests gauge increment and decrement.
func TestGaugeIncDec(t *testing.T) {
	r := NewRegistry()
	g := r.RegisterGauge("gauge", "Gauge")

	g.Inc()
	g.Inc()
	g.Dec()

	gv := g.With()
	if gv.Value() != 1 {
		t.Errorf("Expected value 1, got %f", gv.Value())
	}
}

// TestGaugeAdd tests gauge add.
func TestGaugeAdd(t *testing.T) {
	r := NewRegistry()
	g := r.RegisterGauge("gauge", "Gauge")

	g.Set(10)
	g.Add(5.5)

	gv := g.With()
	if gv.Value() != 15.5 {
		t.Errorf("Expected value 15.5, got %f", gv.Value())
	}
}

// TestHistogramObserve tests histogram observation.
func TestHistogramObserve(t *testing.T) {
	r := NewRegistry()
	h := r.RegisterHistogram("histogram", "Histogram", []float64{0.1, 0.5, 1.0})

	h.Observe(0.05)
	h.Observe(0.3)
	h.Observe(0.8)
	h.Observe(2.0)

	// Should have 4 observations
	// First bucket (<=0.1): 1
	// Second bucket (<=0.5): 2
	// Third bucket (<=1.0): 3
}

// TestRegistryHandler tests metrics HTTP handler.
func TestRegistryHandler(t *testing.T) {
	r := NewRegistry()
	counter := r.RegisterCounter("requests", "Total requests", "method")
	gauge := r.RegisterGauge("active", "Active connections")

	counter.Inc("GET")
	gauge.Set(5)

	handler := r.Handler()

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "# HELP requests Total requests") {
		t.Error("Expected HELP line for requests")
	}
	if !strings.Contains(body, "# TYPE requests counter") {
		t.Error("Expected TYPE line for requests")
	}
	if !strings.Contains(body, "# HELP active Active connections") {
		t.Error("Expected HELP line for active")
	}
	if !strings.Contains(body, "# TYPE active gauge") {
		t.Error("Expected TYPE line for active")
	}
}

// TestMiddleware tests metrics middleware.
func TestMiddleware(t *testing.T) {
	registry := NewRegistry()
	m := NewMiddleware(registry)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response"))
	})

	wrapped := m.Wrap(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Check metrics were recorded
	total := registry.Get("http_requests_total")
	if total == nil {
		t.Fatal("Expected request_total metric")
	}

	duration := registry.Get("http_request_duration_seconds")
	if duration == nil {
		t.Fatal("Expected request_duration metric")
	}
}

// TestMiddlewareMultipleRequests tests multiple requests.
func TestMiddlewareMultipleRequests(t *testing.T) {
	registry := NewRegistry()
	m := NewMiddleware(registry)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := m.Wrap(handler)

	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}

	// Check counter increased
	cv := m.requestTotal.With("GET", "/test", "200")
	if cv.Value() != 10 {
		t.Errorf("Expected 10 requests, got %d", cv.Value())
	}
}

// TestMiddlewareErrorStatus tests error status codes.
func TestMiddlewareErrorStatus(t *testing.T) {
	registry := NewRegistry()
	m := NewMiddleware(registry)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	wrapped := m.Wrap(handler)

	req := httptest.NewRequest("POST", "/api", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	cv := m.requestTotal.With("POST", "/api", "500")
	if cv.Value() != 1 {
		t.Errorf("Expected 1 request, got %d", cv.Value())
	}
}

// TestDefaultMiddleware tests default middleware creation.
func TestDefaultMiddleware(t *testing.T) {
	m, registry := DefaultMiddleware()
	if m == nil {
		t.Fatal("Expected non-nil middleware")
	}
	if registry == nil {
		t.Fatal("Expected non-nil registry")
	}

	// Check all metrics were registered
	if registry.Get("http_requests_total") == nil {
		t.Error("Expected request_total metric")
	}
	if registry.Get("http_request_duration_seconds") == nil {
		t.Error("Expected request_duration metric")
	}
	if registry.Get("http_requests_active") == nil {
		t.Error("Expected active_requests metric")
	}
}

// TestLabelsKey tests label key generation.
func TestLabelsKey(t *testing.T) {
	tests := []struct {
		names  []string
		values []string
		want   string
	}{
		{[]string{"a", "b"}, []string{"1", "2"}, "1\x002"},
		{[]string{"a"}, []string{"1"}, "1"},
		{[]string{}, []string{}, ""},
	}

	for _, tt := range tests {
		got := labelsKey(tt.names, tt.values)
		if got != tt.want {
			t.Errorf("labelsKey(%v, %v) = %q, want %q", tt.names, tt.values, got, tt.want)
		}
	}
}

// TestFormatLabels tests label formatting.
func TestFormatLabels(t *testing.T) {
	names := []string{"method", "status"}
	values := []string{"GET", "200"}

	got := formatLabels(names, values)
	want := `{method="GET",status="200"}`
	if got != want {
		t.Errorf("formatLabels() = %q, want %q", got, want)
	}
}

// TestEscapeLabel tests label escaping.
func TestEscapeLabel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{`with"quotes`, `with\"quotes`},
		{"with\\backslash", "with\\\\backslash"},
		{"with\nnewline", "with\\nnewline"},
	}

	for _, tt := range tests {
		got := escapeLabel(tt.input)
		if got != tt.want {
			t.Errorf("escapeLabel(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestDefaultBuckets tests default buckets.
func TestDefaultBuckets(t *testing.T) {
	if len(DefaultBuckets) != 11 {
		t.Errorf("Expected 11 default buckets, got %d", len(DefaultBuckets))
	}
	if DefaultBuckets[0] != 0.005 {
		t.Errorf("Expected first bucket 0.005, got %f", DefaultBuckets[0])
	}
	if DefaultBuckets[10] != 10 {
		t.Errorf("Expected last bucket 10, got %f", DefaultBuckets[10])
	}
}

// TestMetricsConcurrency tests concurrent access.
func TestMetricsConcurrency(t *testing.T) {
	r := NewRegistry()
	c := r.RegisterCounter("counter", "Counter", "label")

	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func(val int) {
			for j := 0; j < 100; j++ {
				c.Inc(string(rune('0' + val%10)))
			}
			done <- true
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}

	// Total should be 10000
	total := uint64(0)
	for i := 0; i < 10; i++ {
		total += c.With(string(rune('0' + i))).Value()
	}
	if total != 10000 {
		t.Errorf("Expected total 10000, got %d", total)
	}
}

// TestSanitizePath tests path sanitization.
func TestSanitizePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "/"},
		{"/api/users", "/api/users"},
	}

	for _, tt := range tests {
		got := sanitizePath(tt.input)
		if got != tt.want {
			t.Errorf("sanitizePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestActiveRequests tests active request tracking.
func TestActiveRequests(t *testing.T) {
	registry := NewRegistry()
	m := NewMiddleware(registry)

	block := make(chan bool)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block to keep request active
		<-block
		w.WriteHeader(http.StatusOK)
	})

	wrapped := m.Wrap(handler)

	// Start a request
	go func() {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}()

	// Wait a bit for request to start
	time.Sleep(10 * time.Millisecond)

	// Check active requests
	g := m.activeRequests.With("GET")
	if g.Value() != 1 {
		t.Errorf("Expected 1 active request, got %f", g.Value())
	}

	// Unblock
	block <- true

	// Wait for completion
	time.Sleep(10 * time.Millisecond)

	if g.Value() != 0 {
		t.Errorf("Expected 0 active requests, got %f", g.Value())
	}
}
