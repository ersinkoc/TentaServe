package upstream

import (
	"io"
	"math/rand/v2"
	"sync"
	"time"
)

// calculateNextDelay calculates the next retry delay with exponential backoff.
func calculateNextDelay(current, maxDelay time.Duration, multiplier float64) time.Duration {
	next := time.Duration(float64(current) * multiplier)
	if next > maxDelay {
		next = maxDelay
	}

	// Add jitter: +/- 25% of delay, but don't exceed maxDelay
	jitter := time.Duration(float64(next) * 0.25)
	if jitter > 0 {
		offset := time.Duration(rand.Int64N(int64(jitter)*2)) - jitter
		next += offset
		// Ensure we don't exceed maxDelay after jitter
		if next > maxDelay {
			next = maxDelay
		}
	}

	return next
}

// BodyReader is a reusable body reader for request retries.
type BodyReader struct {
	data   []byte
	offset int
}

// NewBodyReader creates a new body reader from bytes.
func NewBodyReader(data []byte) *BodyReader {
	return &BodyReader{data: data}
}

// Read implements io.Reader.
func (r *BodyReader) Read(p []byte) (n int, err error) {
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}
	n = len(r.data) - r.offset
	if n > len(p) {
		n = len(p)
	}
	copy(p, r.data[r.offset:r.offset+n])
	r.offset += n
	return n, nil
}

// Seek implements io.Seeker for re-reading.
func (r *BodyReader) Seek(offset int64, whence int) (int64, error) {
	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = int64(r.offset) + offset
	case io.SeekEnd:
		newOffset = int64(len(r.data)) + offset
	}
	// Clamp to valid range instead of error
	if newOffset < 0 {
		newOffset = 0
	}
	if newOffset > int64(len(r.data)) {
		newOffset = int64(len(r.data))
	}
	r.offset = int(newOffset)
	return newOffset, nil
}

// Reset seeks back to the beginning.
func (r *BodyReader) Reset() {
	r.offset = 0
}

// Len returns the total length.
func (r *BodyReader) Len() int {
	return len(r.data)
}

// RetryStats tracks retry statistics.
type RetryStats struct {
	mu            sync.Mutex
	TotalAttempts int
	TotalRetries  int
	TotalErrors   int
}

// RecordAttempt records a request attempt.
func (s *RetryStats) RecordAttempt(retry bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalAttempts++
	if retry {
		s.TotalRetries++
	}
}

// RecordError records an error.
func (s *RetryStats) RecordError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalErrors++
}

// GetStats returns current stats.
func (s *RetryStats) GetStats() (attempts, retries, errors int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.TotalAttempts, s.TotalRetries, s.TotalErrors
}

// Backoff provides exponential backoff with configurable options.
type Backoff struct {
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	Multiplier float64
	attempt    int
}

// NewBackoff creates a new backoff calculator.
func NewBackoff(baseDelay, maxDelay time.Duration, multiplier float64) *Backoff {
	if baseDelay == 0 {
		baseDelay = 100 * time.Millisecond
	}
	if maxDelay == 0 {
		maxDelay = 5 * time.Second
	}
	if multiplier == 0 {
		multiplier = 2.0
	}
	return &Backoff{
		BaseDelay:  baseDelay,
		MaxDelay:   maxDelay,
		Multiplier: multiplier,
	}
}

// Next returns the next delay and increments attempt counter.
func (b *Backoff) Next() time.Duration {
	delay := b.BaseDelay
	for i := 0; i < b.attempt; i++ {
		delay = time.Duration(float64(delay) * b.Multiplier)
		if delay > b.MaxDelay {
			delay = b.MaxDelay
			break
		}
	}
	b.attempt++

	// Add jitter, but don't exceed maxDelay
	jitter := time.Duration(float64(delay) * 0.25)
	if jitter > 0 {
		offset := time.Duration(rand.Int64N(int64(jitter)*2)) - jitter
		delay += offset
		if delay > b.MaxDelay {
			delay = b.MaxDelay
		}
	}

	return delay
}

// Reset resets the attempt counter.
func (b *Backoff) Reset() {
	b.attempt = 0
}

// IsRetryableStatus checks if an HTTP status code is retryable.
func IsRetryableStatus(statusCode int) bool {
	switch statusCode {
	case 408, // Request Timeout
		429, // Too Many Requests
		500, // Internal Server Error
		502, // Bad Gateway
		503, // Service Unavailable
		504, // Gateway Timeout
		507, // Insufficient Storage
		508, // Loop Detected
		509, // Bandwidth Limit Exceeded (non-standard)
		510, // Not Extended
		511: // Network Authentication Required
		return true
	}
	return false
}

// IsRetryableError checks if an error is retryable.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	// All network errors are considered retryable
	return true
}

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}
