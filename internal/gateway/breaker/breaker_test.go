package breaker

import (
	"testing"
	"time"
)

// TestDefaultConfig tests default configuration.
func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if !config.Enabled {
		t.Error("Expected default enabled to be true")
	}
	if config.FailureThreshold != 5 {
		t.Errorf("Expected default failure threshold 5, got %d", config.FailureThreshold)
	}
	if config.SuccessThreshold != 3 {
		t.Errorf("Expected default success threshold 3, got %d", config.SuccessThreshold)
	}
	if config.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", config.Timeout)
	}
	if config.HalfOpenMaxRequests != 3 {
		t.Errorf("Expected default half-open max requests 3, got %d", config.HalfOpenMaxRequests)
	}
}

// TestStateString tests state string representation.
func TestStateString(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{State(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.expected)
		}
	}
}

// TestBreakerNew tests breaker creation.
func TestBreakerNew(t *testing.T) {
	config := DefaultConfig()
	b := New(config)

	if b == nil {
		t.Fatal("Expected non-nil breaker")
	}
	if b.State() != StateClosed {
		t.Errorf("Expected initial state closed, got %s", b.State().String())
	}
}

// TestBreakerAllowWhenClosed tests allowing requests when closed.
func TestBreakerAllowWhenClosed(t *testing.T) {
	b := New(DefaultConfig())

	for i := 0; i < 10; i++ {
		if !b.Allow() {
			t.Errorf("Expected request %d to be allowed when closed", i)
		}
	}
}

// TestBreakerOpenOnFailures tests circuit opens after failures.
func TestBreakerOpenOnFailures(t *testing.T) {
	config := &Config{
		Enabled:          true,
		FailureThreshold: 3,
		Timeout:          time.Hour, // Long timeout to prevent half-open
	}
	b := New(config)

	// Record 3 failures
	for i := 0; i < 3; i++ {
		b.Allow()
		b.Record(ResultFailure)
	}

	if b.State() != StateOpen {
		t.Errorf("Expected state open after 3 failures, got %s", b.State().String())
	}

	// Requests should be rejected
	if b.Allow() {
		t.Error("Expected request to be rejected when open")
	}
}

// TestBreakerRecordSuccess tests recording successes.
func TestBreakerRecordSuccess(t *testing.T) {
	config := &Config{
		Enabled:          true,
		FailureThreshold: 5,
	}
	b := New(config)

	// Record some failures
	for i := 0; i < 2; i++ {
		b.Allow()
		b.Record(ResultFailure)
	}

	// Record successes to reset failure count
	for i := 0; i < 5; i++ {
		b.Allow()
		b.Record(ResultSuccess)
	}

	// Should still be closed
	if b.State() != StateClosed {
		t.Errorf("Expected state closed, got %s", b.State().String())
	}

	// Should be able to make requests
	if !b.Allow() {
		t.Error("Expected request to be allowed")
	}
}

// TestBreakerHalfOpen tests transition to half-open state.
func TestBreakerHalfOpen(t *testing.T) {
	config := &Config{
		Enabled:             true,
		FailureThreshold:    2,
		SuccessThreshold:    2,
		Timeout:             10 * time.Millisecond,
		HalfOpenMaxRequests: 2,
	}
	b := New(config)

	// Open the circuit
	for i := 0; i < 2; i++ {
		b.Allow()
		b.Record(ResultFailure)
	}

	if b.State() != StateOpen {
		t.Fatal("Expected state open")
	}

	// Wait for timeout
	time.Sleep(20 * time.Millisecond)

	// Should transition to half-open
	if !b.Allow() {
		t.Error("Expected request to be allowed in half-open")
	}

	if b.State() != StateHalfOpen {
		t.Errorf("Expected state half-open, got %s", b.State().String())
	}
}

// TestBreakerCloseFromHalfOpen tests closing circuit from half-open.
func TestBreakerCloseFromHalfOpen(t *testing.T) {
	config := &Config{
		Enabled:             true,
		FailureThreshold:    2,
		SuccessThreshold:    2,
		Timeout:             10 * time.Millisecond,
		HalfOpenMaxRequests: 3,
	}
	b := New(config)

	// Open the circuit
	for i := 0; i < 2; i++ {
		b.Allow()
		b.Record(ResultFailure)
	}

	// Wait for timeout
	time.Sleep(20 * time.Millisecond)

	// First request in half-open
	b.Allow()
	if b.State() != StateHalfOpen {
		t.Fatal("Expected state half-open")
	}

	// Record successes to close
	b.Record(ResultSuccess)
	b.Allow()
	b.Record(ResultSuccess)

	if b.State() != StateClosed {
		t.Errorf("Expected state closed after successes, got %s", b.State().String())
	}
}

// TestBreakerOpenFromHalfOpen tests reopening from half-open on failure.
func TestBreakerOpenFromHalfOpen(t *testing.T) {
	config := &Config{
		Enabled:             true,
		FailureThreshold:    2,
		SuccessThreshold:    3,
		Timeout:             10 * time.Millisecond,
		HalfOpenMaxRequests: 3,
	}
	b := New(config)

	// Open the circuit
	for i := 0; i < 2; i++ {
		b.Allow()
		b.Record(ResultFailure)
	}

	// Wait for timeout
	time.Sleep(20 * time.Millisecond)

	// Request in half-open
	b.Allow()

	// Record failure - should go back to open
	b.Record(ResultFailure)

	if b.State() != StateOpen {
		t.Errorf("Expected state open after failure in half-open, got %s", b.State().String())
	}
}

// TestBreakerDisabled tests disabled breaker.
func TestBreakerDisabled(t *testing.T) {
	config := &Config{
		Enabled:          false,
		FailureThreshold: 1,
	}
	b := New(config)

	// Should always allow
	for i := 0; i < 10; i++ {
		if !b.Allow() {
			t.Error("Expected request to be allowed when disabled")
		}
	}

	// Record failures - should not affect state
	for i := 0; i < 10; i++ {
		b.Record(ResultFailure)
	}

	if b.State() != StateClosed {
		t.Error("Expected state to remain closed when disabled")
	}
}

// TestBreakerReset tests reset functionality.
func TestBreakerReset(t *testing.T) {
	config := &Config{
		Enabled:          true,
		FailureThreshold: 2,
	}
	b := New(config)

	// Open the circuit
	for i := 0; i < 2; i++ {
		b.Allow()
		b.Record(ResultFailure)
	}

	if b.State() != StateOpen {
		t.Fatal("Expected state open")
	}

	// Reset
	b.Reset()

	if b.State() != StateClosed {
		t.Errorf("Expected state closed after reset, got %s", b.State().String())
	}

	if !b.Allow() {
		t.Error("Expected request to be allowed after reset")
	}
}

// TestBreakerStats tests statistics.
func TestBreakerStats(t *testing.T) {
	config := &Config{
		Enabled:          true,
		FailureThreshold: 5,
	}
	b := New(config)

	// Record some failures
	for i := 0; i < 3; i++ {
		b.Allow()
		b.Record(ResultFailure)
	}

	stats := b.Stats()
	if stats.State != "closed" {
		t.Errorf("Expected state closed, got %s", stats.State)
	}
	if stats.Failures != 3 {
		t.Errorf("Expected 3 failures, got %d", stats.Failures)
	}
}

// TestBreakerHalfOpenLimit tests half-open request limit.
func TestBreakerHalfOpenLimit(t *testing.T) {
	config := &Config{
		Enabled:             true,
		FailureThreshold:    1,
		Timeout:             10 * time.Millisecond,
		HalfOpenMaxRequests: 2,
	}
	b := New(config)

	// Open the circuit
	b.Allow()
	b.Record(ResultFailure)

	// Wait for timeout
	time.Sleep(20 * time.Millisecond)

	// First 2 requests should be allowed
	if !b.Allow() {
		t.Error("Expected first request to be allowed")
	}
	if !b.Allow() {
		t.Error("Expected second request to be allowed")
	}

	// Third request should be rejected
	if b.Allow() {
		t.Error("Expected third request to be rejected")
	}
}
