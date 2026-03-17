package breaker

import (
	"errors"
	"sync"
	"time"
)

// State represents the circuit breaker state.
type State int

const (
	// StateClosed allows requests through normally.
	StateClosed State = iota
	// StateOpen rejects requests immediately (fail-fast).
	StateOpen
	// StateHalfOpen allows a test request to check recovery.
	StateHalfOpen
)

// String returns the state name.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Config defines circuit breaker configuration.
type Config struct {
	// Enabled controls whether circuit breaker is active
	Enabled bool

	// FailureThreshold is the number of failures before opening
	FailureThreshold uint32

	// SuccessThreshold is the number of successes needed to close from half-open
	SuccessThreshold uint32

	// Timeout is how long to wait before trying half-open
	Timeout time.Duration

	// HalfOpenMaxRequests is the number of test requests allowed in half-open state
	HalfOpenMaxRequests uint32
}

// DefaultConfig returns default circuit breaker configuration.
func DefaultConfig() *Config {
	return &Config{
		Enabled:             true,
		FailureThreshold:    5,
		SuccessThreshold:    3,
		Timeout:             30 * time.Second,
		HalfOpenMaxRequests: 3,
	}
}

// Result represents the outcome of a protected operation.
type Result int

const (
	// ResultSuccess indicates the operation succeeded.
	ResultSuccess Result = iota
	// ResultFailure indicates the operation failed.
	ResultFailure
)

// Breaker is a circuit breaker implementation.
type Breaker struct {
	config *Config
	mu     sync.RWMutex

	state           State
	failures        uint32
	successes       uint32
	lastFailureTime time.Time
	halfOpenCount   uint32
}

// New creates a new circuit breaker.
func New(config *Config) *Breaker {
	if config == nil {
		config = DefaultConfig()
	}

	return &Breaker{
		config: config,
		state:  StateClosed,
	}
}

// Allow checks if a request should be allowed through.
// Returns true if the request can proceed, false if it should be rejected.
func (b *Breaker) Allow() bool {
	if !b.config.Enabled {
		return true
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		return true

	case StateOpen:
		// Check if timeout has elapsed to transition to half-open
		if time.Since(b.lastFailureTime) > b.config.Timeout {
			b.state = StateHalfOpen
			b.halfOpenCount = 1 // Count this transition request
			return true
		}
		return false

	case StateHalfOpen:
		// Allow limited requests in half-open state
		if b.halfOpenCount < b.config.HalfOpenMaxRequests {
			b.halfOpenCount++
			return true
		}
		return false

	default:
		return true
	}
}

// Record records the result of a request.
func (b *Breaker) Record(result Result) {
	if !b.config.Enabled {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		if result == ResultFailure {
			b.failures++
			b.lastFailureTime = time.Now()
			if b.failures >= b.config.FailureThreshold {
				b.state = StateOpen
			}
		} else {
			// Reset failures on success
			if b.failures > 0 {
				b.failures--
			}
		}

	case StateOpen:
		// Should not happen if Allow() is checked first
		// But handle it anyway

	case StateHalfOpen:
		if result == ResultFailure {
			// Back to open
			b.state = StateOpen
			b.failures = b.config.FailureThreshold
			b.lastFailureTime = time.Now()
			b.halfOpenCount = 0
		} else {
			b.successes++
			if b.successes >= b.config.SuccessThreshold {
				// Close the circuit
				b.state = StateClosed
				b.failures = 0
				b.successes = 0
				b.halfOpenCount = 0
			}
		}
	}
}

// State returns the current state.
func (b *Breaker) State() State {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

// Stats returns current statistics.
func (b *Breaker) Stats() Stats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return Stats{
		State:         b.state.String(),
		Failures:      b.failures,
		Successes:     b.successes,
		HalfOpenCount: b.halfOpenCount,
	}
}

// Reset resets the circuit breaker to closed state.
func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.state = StateClosed
	b.failures = 0
	b.successes = 0
	b.halfOpenCount = 0
}

// Stats contains circuit breaker statistics.
type Stats struct {
	State         string
	Failures      uint32
	Successes     uint32
	HalfOpenCount uint32
}

// ErrCircuitOpen is returned when the circuit is open.
var ErrCircuitOpen = errors.New("circuit breaker is open")
