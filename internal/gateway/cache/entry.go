package cache

import (
	"bytes"
	"net/http"
	"time"
)

// Entry represents a cached HTTP response.
type Entry struct {
	// StatusCode is the HTTP status code
	StatusCode int

	// Headers are the response headers
	Headers http.Header

	// Body is the response body
	Body []byte

	// CreatedAt is when this entry was cached
	CreatedAt time.Time

	// ExpiresAt is when this entry becomes stale
	ExpiresAt time.Time

	// VaryHeaders are the request headers that affect cache key
	VaryHeaders map[string]string
}

// IsExpired returns true if the entry has passed its TTL.
func (e *Entry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// IsStale returns true if the entry is within stale-while-revalidate window.
// staleDuration is how long after expiry the entry can still be served.
func (e *Entry) IsStale(staleDuration time.Duration) bool {
	if !e.IsExpired() {
		return false
	}
	return time.Now().Before(e.ExpiresAt.Add(staleDuration))
}

// Age returns the age of the cached entry.
func (e *Entry) Age() time.Duration {
	return time.Since(e.CreatedAt)
}

// TTL returns the remaining time until expiry.
func (e *Entry) TTL() time.Duration {
	return e.ExpiresAt.Sub(time.Now())
}

// Clone returns a deep copy of the entry.
func (e *Entry) Clone() *Entry {
	if e == nil {
		return nil
	}

	// Clone headers
	headers := make(http.Header, len(e.Headers))
	for k, v := range e.Headers {
		headers[k] = append([]string(nil), v...)
	}

	// Clone body
	body := make([]byte, len(e.Body))
	copy(body, e.Body)

	// Clone vary headers
	varyHeaders := make(map[string]string, len(e.VaryHeaders))
	for k, v := range e.VaryHeaders {
		varyHeaders[k] = v
	}

	return &Entry{
		StatusCode:  e.StatusCode,
		Headers:     headers,
		Body:        body,
		CreatedAt:   e.CreatedAt,
		ExpiresAt:   e.ExpiresAt,
		VaryHeaders: varyHeaders,
	}
}

// Size returns the approximate size of the entry in bytes.
func (e *Entry) Size() int {
	size := len(e.Body)
	for k, v := range e.Headers {
		size += len(k)
		for _, val := range v {
			size += len(val)
		}
	}
	return size
}

// ResponseWriter is a wrapper that captures the response for caching.
type ResponseWriter struct {
	http.ResponseWriter
	statusCode  int
	body        *bytes.Buffer
	headers     http.Header
	wroteHeader bool
}

// NewResponseWriter creates a new response capture writer.
func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           &bytes.Buffer{},
		headers:        make(http.Header),
	}
}

// WriteHeader captures the status code.
func (rw *ResponseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.statusCode = code
		rw.wroteHeader = true
		// Copy headers to capture before sending
		for k, v := range rw.ResponseWriter.Header() {
			rw.headers[k] = append([]string(nil), v...)
		}
		rw.ResponseWriter.WriteHeader(code)
	}
}

// Write captures the body.
func (rw *ResponseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

// Header returns the response headers.
func (rw *ResponseWriter) Header() http.Header {
	return rw.ResponseWriter.Header()
}

// StatusCode returns the captured status code.
func (rw *ResponseWriter) StatusCode() int {
	return rw.statusCode
}

// Body returns the captured body.
func (rw *ResponseWriter) Body() []byte {
	return rw.body.Bytes()
}

// CapturedHeaders returns the captured headers at WriteHeader time.
func (rw *ResponseWriter) CapturedHeaders() http.Header {
	return rw.headers
}

// IsCacheable returns true if the response is cacheable based on status code.
func (rw *ResponseWriter) IsCacheable() bool {
	// Only cache successful responses
	switch rw.statusCode {
	case http.StatusOK,
		http.StatusCreated,
		http.StatusAccepted,
		http.StatusNonAuthoritativeInfo,
		http.StatusNoContent,
		http.StatusResetContent,
		http.StatusPartialContent,
		http.StatusMovedPermanently,
		http.StatusFound,
		http.StatusTemporaryRedirect,
		http.StatusPermanentRedirect:
		return true
	default:
		return false
	}
}
