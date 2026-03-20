package metrics

import (
	"net/http"
	"strconv"
	"time"
)

// Middleware collects HTTP metrics.
type Middleware struct {
	registry        *Registry
	requestTotal    *CounterVec
	requestDuration *HistogramVec
	responseSize    *HistogramVec
	activeRequests  *GaugeVec
}

// NewMiddleware creates a new metrics middleware.
func NewMiddleware(registry *Registry) *Middleware {
	m := &Middleware{registry: registry}

	// Register metrics
	m.requestTotal = registry.RegisterCounter(
		"http_requests_total",
		"Total number of HTTP requests",
		"method", "path", "status",
	)

	m.requestDuration = registry.RegisterHistogram(
		"http_request_duration_seconds",
		"HTTP request duration in seconds",
		DefaultBuckets,
		"method", "path",
	)

	m.responseSize = registry.RegisterHistogram(
		"http_response_size_bytes",
		"HTTP response size in bytes",
		[]float64{100, 1000, 10000, 100000, 1000000, 10000000},
		"method", "path",
	)

	m.activeRequests = registry.RegisterGauge(
		"http_requests_active",
		"Number of active HTTP requests",
		"method",
	)

	return m
}

// Wrap returns an HTTP handler that collects metrics.
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Track active requests
		m.activeRequests.Inc(r.Method)
		defer m.activeRequests.Dec(r.Method)

		// Wrap response writer to capture status code and size
		rw := &metricsResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Execute handler
		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()
		path := sanitizePath(r.URL.Path)

		// Record metrics
		m.requestTotal.Inc(
			r.Method,
			path,
			strconv.Itoa(rw.statusCode),
		)

		m.requestDuration.Observe(duration, r.Method, path)
		m.responseSize.Observe(float64(rw.bytesWritten), r.Method, path)
	})
}

// metricsResponseWriter wraps http.ResponseWriter to capture metrics.
type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
	written      bool
}

func (rw *metricsResponseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *metricsResponseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += int64(n)
	return n, err
}

// sanitizePath sanitizes the path for metric labels.
func sanitizePath(path string) string {
	if path == "" {
		return "/"
	}
	// Limit path length for label cardinality
	if len(path) > 100 {
		path = path[:100]
	}
	return path
}

// DefaultMiddleware creates a middleware with a new registry.
func DefaultMiddleware() (*Middleware, *Registry) {
	registry := NewRegistry()
	return NewMiddleware(registry), registry
}
