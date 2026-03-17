package cache

import (
	"net/http"
	"testing"
	"time"
)

// TestEntryIsExpired tests expiry checking.
func TestEntryIsExpired(t *testing.T) {
	entry := &Entry{
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}

	if entry.IsExpired() {
		t.Error("Expected entry not expired")
	}

	// Expired entry
	entry.ExpiresAt = time.Now().Add(-time.Hour)
	if !entry.IsExpired() {
		t.Error("Expected entry expired")
	}
}

// TestEntryIsStale tests stale checking.
func TestEntryIsStale(t *testing.T) {
	entry := &Entry{
		CreatedAt:   time.Now().Add(-2 * time.Hour),
		ExpiresAt:   time.Now().Add(-30 * time.Minute),
	}

	// Within stale window (60s after expiry)
	if !entry.IsStale(time.Hour) {
		t.Error("Expected entry to be stale")
	}

	// Outside stale window
	if entry.IsStale(time.Minute) {
		t.Error("Expected entry not to be stale")
	}

	// Not expired yet
	entry.ExpiresAt = time.Now().Add(time.Hour)
	if entry.IsStale(time.Hour) {
		t.Error("Expected fresh entry not to be stale")
	}
}

// TestEntryAge tests age calculation.
func TestEntryAge(t *testing.T) {
	entry := &Entry{
		CreatedAt: time.Now().Add(-5 * time.Minute),
	}

	age := entry.Age()
	if age < 4*time.Minute || age > 6*time.Minute {
		t.Errorf("Expected age ~5m, got %v", age)
	}
}

// TestEntryTTL tests TTL calculation.
func TestEntryTTL(t *testing.T) {
	entry := &Entry{
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}

	ttl := entry.TTL()
	if ttl < 4*time.Minute || ttl > 6*time.Minute {
		t.Errorf("Expected TTL ~5m, got %v", ttl)
	}
}

// TestEntryClone tests deep copying.
func TestEntryClone(t *testing.T) {
	original := &Entry{
		StatusCode: 200,
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
		Body:       []byte("test"),
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(time.Hour),
		VaryHeaders: map[string]string{"Accept": "application/json"},
	}

	clone := original.Clone()

	// Modify clone
	clone.StatusCode = 404
	clone.Headers.Set("Content-Type", "text/plain")
	clone.Body[0] = 'x'
	clone.VaryHeaders["Accept"] = "text/plain"

	// Original should be unchanged
	if original.StatusCode != 200 {
		t.Error("Original status code was modified")
	}
	if original.Headers.Get("Content-Type") != "application/json" {
		t.Error("Original headers were modified")
	}
	if original.Body[0] != 't' {
		t.Error("Original body was modified")
	}
	if original.VaryHeaders["Accept"] != "application/json" {
		t.Error("Original vary headers were modified")
	}
}

// TestEntryNilClone tests cloning nil entry.
func TestEntryNilClone(t *testing.T) {
	var entry *Entry
	clone := entry.Clone()
	if clone != nil {
		t.Error("Expected nil clone for nil entry")
	}
}

// TestEntrySize tests size calculation.
func TestEntrySize(t *testing.T) {
	entry := &Entry{
		Headers: http.Header{
			"Content-Type":   []string{"application/json"},
			"Content-Length": []string{"100"},
		},
		Body: []byte("test body content"),
	}

	size := entry.Size()
	// Just verify size is positive and includes body + headers
	if size <= len(entry.Body) {
		t.Errorf("Expected size > %d, got %d", len(entry.Body), size)
	}
}

// TestResponseWriterCapture tests response capture.
func TestResponseWriterCapture(t *testing.T) {
	rec := &testRecorder{
		HeaderMap: make(http.Header),
	}
	rw := NewResponseWriter(rec)

	// Write header
	rw.WriteHeader(http.StatusCreated)
	if rw.StatusCode() != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, rw.StatusCode())
	}

	// Write body
	body := []byte("test response")
	n, err := rw.Write(body)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(body) {
		t.Errorf("Expected %d bytes written, got %d", len(body), n)
	}

	// Check captured body
	if string(rw.Body()) != string(body) {
		t.Errorf("Expected body %q, got %q", body, rw.Body())
	}
}

// TestResponseWriterAutoWriteHeader tests automatic header writing.
func TestResponseWriterAutoWriteHeader(t *testing.T) {
	rec := &testRecorder{
		HeaderMap: make(http.Header),
	}
	rw := NewResponseWriter(rec)

	// Write without explicit WriteHeader
	rw.Write([]byte("test"))

	if rw.StatusCode() != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rw.StatusCode())
	}
}

// TestResponseWriterIsCacheable tests cacheable status code detection.
func TestResponseWriterIsCacheable(t *testing.T) {
	tests := []struct {
		status   int
		expected bool
	}{
		{200, true},
		{201, true},
		{204, true},
		{301, true},
		{302, true},
		{404, false},
		{500, false},
		{503, false},
	}

	for _, tt := range tests {
		rec := &testRecorder{HeaderMap: make(http.Header)}
		rw := NewResponseWriter(rec)
		rw.WriteHeader(tt.status)

		if rw.IsCacheable() != tt.expected {
			t.Errorf("Status %d: expected cacheable=%v, got %v", tt.status, tt.expected, rw.IsCacheable())
		}
	}
}

// testRecorder is a minimal ResponseRecorder for testing.
type testRecorder struct {
	HeaderMap http.Header
	Code      int
	Body      []byte
}

func (tr *testRecorder) Header() http.Header {
	return tr.HeaderMap
}

func (tr *testRecorder) WriteHeader(code int) {
	tr.Code = code
}

func (tr *testRecorder) Write(b []byte) (int, error) {
	if tr.Code == 0 {
		tr.Code = http.StatusOK
	}
	tr.Body = append(tr.Body, b...)
	return len(b), nil
}
