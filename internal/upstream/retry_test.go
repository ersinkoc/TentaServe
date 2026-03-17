package upstream

import (
	"io"
	"testing"
	"time"
)

func TestCalculateNextDelay(t *testing.T) {
	base := 100 * time.Millisecond
	max := 5 * time.Second
	multiplier := 2.0

	delay := calculateNextDelay(base, max, multiplier)

	// Should be around 200ms (100 * 2) with jitter
	if delay < 150*time.Millisecond || delay > 250*time.Millisecond {
		t.Errorf("Expected delay around 200ms, got %v", delay)
	}

	// Test max delay cap
	delay = calculateNextDelay(3*time.Second, max, multiplier)
	if delay > max {
		t.Errorf("Expected delay <= max (5s), got %v", delay)
	}
}

func TestBodyReader(t *testing.T) {
	data := []byte("Hello, World!")
	reader := NewBodyReader(data)

	// Test Read
	buf := make([]byte, 5)
	n, err := reader.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != 5 {
		t.Errorf("Expected 5 bytes, got %d", n)
	}
	if string(buf) != "Hello" {
		t.Errorf("Expected 'Hello', got %q", string(buf))
	}

	// Test Seek
	pos, err := reader.Seek(0, io.SeekStart)
	if err != nil {
		t.Fatalf("Seek failed: %v", err)
	}
	if pos != 0 {
		t.Errorf("Expected position 0, got %d", pos)
	}

	// Read again (13 bytes - exact size of data)
	buf = make([]byte, 13)
	n, err = reader.Read(buf)
	// First read fills buffer without EOF (13 bytes available)
	if err != nil {
		t.Errorf("Expected no error on full read, got %v", err)
	}
	if n != 13 {
		t.Errorf("Expected 13 bytes, got %d", n)
	}
	if string(buf[:n]) != "Hello, World!" {
		t.Errorf("Expected full content, got %q", string(buf[:n]))
	}

	// Read again to get EOF
	n, err = reader.Read(buf)
	if err != io.EOF {
		t.Errorf("Expected EOF, got %v", err)
	}
	if n != 0 {
		t.Errorf("Expected 0 bytes after EOF, got %d", n)
	}
}

func TestBodyReader_Seek(t *testing.T) {
	data := []byte("0123456789")
	reader := NewBodyReader(data)

	tests := []struct {
		offset int64
		whence int
		want   int64
	}{
		{0, io.SeekStart, 0},
		{5, io.SeekStart, 5},
		{3, io.SeekCurrent, 8},    // 5 + 3 = 8 (after reading 5 bytes)
		{-2, io.SeekEnd, 8},       // 10 - 2 = 8
		{-1, io.SeekStart, 0},     // clamp to 0
	}

	for _, tt := range tests {
		reader.Reset()
		reader.Read(make([]byte, 5)) // read first 5 bytes
		pos, err := reader.Seek(tt.offset, tt.whence)
		if err != nil {
			t.Errorf("Seek(%d, %d) error = %v", tt.offset, tt.whence, err)
			continue
		}
		if pos != tt.want {
			t.Errorf("Seek(%d, %d) = %d, want %d", tt.offset, tt.whence, pos, tt.want)
		}
	}
}

func TestBodyReader_Reset(t *testing.T) {
	data := []byte("Hello")
	reader := NewBodyReader(data)

	// Read all
	buf := make([]byte, 5)
	reader.Read(buf)

	// Reset
	reader.Reset()

	// Read again
	buf = make([]byte, 5)
	n, err := reader.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("Read after reset failed: %v", err)
	}
	if n != 5 {
		t.Errorf("Expected 5 bytes after reset, got %d", n)
	}
}

func TestBodyReader_Len(t *testing.T) {
	data := []byte("Hello, World!")
	reader := NewBodyReader(data)

	if reader.Len() != len(data) {
		t.Errorf("Expected Len() = %d, got %d", len(data), reader.Len())
	}
}

func TestRetryStats(t *testing.T) {
	stats := &RetryStats{}

	// Record attempts
	stats.RecordAttempt(false)
	stats.RecordAttempt(true)
	stats.RecordAttempt(true)
	stats.RecordError()

	attempts, retries, errors := stats.GetStats()
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
	if retries != 2 {
		t.Errorf("Expected 2 retries, got %d", retries)
	}
	if errors != 1 {
		t.Errorf("Expected 1 error, got %d", errors)
	}
}

func TestBackoff(t *testing.T) {
	b := NewBackoff(100*time.Millisecond, 5*time.Second, 2.0)

	delays := []time.Duration{
		b.Next(),
		b.Next(),
		b.Next(),
	}

	// Each delay should be approximately double the base, with jitter
	// First: ~100ms, Second: ~200ms, Third: ~400ms
	if delays[0] < 75*time.Millisecond || delays[0] > 125*time.Millisecond {
		t.Errorf("First delay out of range: %v", delays[0])
	}
	if delays[1] < 150*time.Millisecond || delays[1] > 250*time.Millisecond {
		t.Errorf("Second delay out of range: %v", delays[1])
	}
	if delays[2] < 300*time.Millisecond || delays[2] > 500*time.Millisecond {
		t.Errorf("Third delay out of range: %v", delays[2])
	}

	// Test reset
	b.Reset()
	if b.attempt != 0 {
		t.Error("Expected attempt counter to be reset")
	}
}

func TestBackoff_MaxDelay(t *testing.T) {
	b := NewBackoff(100*time.Millisecond, 500*time.Millisecond, 2.0)

	// Generate delays until we hit max
	for i := 0; i < 10; i++ {
		delay := b.Next()
		if delay > 500*time.Millisecond {
			t.Errorf("Delay exceeded max: %v", delay)
		}
	}
}

func TestIsRetryableStatus(t *testing.T) {
	retryable := []int{408, 429, 500, 502, 503, 504, 507, 508, 509, 510, 511}
	nonRetryable := []int{200, 201, 400, 401, 403, 404, 405}

	for _, code := range retryable {
		if !IsRetryableStatus(code) {
			t.Errorf("Expected %d to be retryable", code)
		}
	}

	for _, code := range nonRetryable {
		if IsRetryableStatus(code) {
			t.Errorf("Expected %d to not be retryable", code)
		}
	}
}

func TestIsRetryableError(t *testing.T) {
	if IsRetryableError(nil) {
		t.Error("Expected nil error to not be retryable")
	}

	if !IsRetryableError(io.EOF) {
		t.Error("Expected io.EOF to be retryable")
	}

	if !IsRetryableError(io.ErrUnexpectedEOF) {
		t.Error("Expected ErrUnexpectedEOF to be retryable")
	}
}

func TestCircuitState(t *testing.T) {
	tests := []struct {
		state CircuitState
		want  string
	}{
		{CircuitClosed, "closed"},
		{CircuitOpen, "open"},
		{CircuitHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}

	for _, tt := range tests {
		got := tt.state.String()
		if got != tt.want {
			t.Errorf("CircuitState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestNewBackoff_Defaults(t *testing.T) {
	b := NewBackoff(0, 0, 0)

	if b.BaseDelay != 100*time.Millisecond {
		t.Errorf("Expected default base delay 100ms, got %v", b.BaseDelay)
	}
	if b.MaxDelay != 5*time.Second {
		t.Errorf("Expected default max delay 5s, got %v", b.MaxDelay)
	}
	if b.Multiplier != 2.0 {
		t.Errorf("Expected default multiplier 2.0, got %f", b.Multiplier)
	}
}

// Benchmark Backoff
func BenchmarkBackoff_Next(b *testing.B) {
	backoff := NewBackoff(100*time.Millisecond, 5*time.Second, 2.0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = backoff.Next()
		backoff.Reset()
	}
}
