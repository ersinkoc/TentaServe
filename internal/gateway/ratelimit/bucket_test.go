package ratelimit

import (
	"sync"
	"testing"
	"time"
)

// TestTokenBucketNew tests token bucket creation.
func TestTokenBucketNew(t *testing.T) {
	tb := NewTokenBucket(10, 20)

	if tb.Rate() != 10 {
		t.Errorf("Expected rate 10, got %f", tb.Rate())
	}
	if tb.Burst() != 20 {
		t.Errorf("Expected burst 20, got %f", tb.Burst())
	}

	// Should start with full bucket
	tokens := tb.Tokens()
	if tokens < 19 || tokens > 20 {
		t.Errorf("Expected ~20 tokens initially, got %f", tokens)
	}
}

// TestTokenBucketNewNegativeValues tests creation with negative values.
func TestTokenBucketNewNegativeValues(t *testing.T) {
	tb := NewTokenBucket(-5, -10)

	if tb.Rate() != 0 {
		t.Errorf("Expected rate 0 for negative input, got %f", tb.Rate())
	}
	if tb.Burst() != 0 {
		t.Errorf("Expected burst 0 for negative input, got %f", tb.Burst())
	}
}

// TestTokenBucketAllow tests basic token consumption.
func TestTokenBucketAllow(t *testing.T) {
	tb := NewTokenBucket(100, 10)

	// Should allow burst consumption
	for i := 0; i < 10; i++ {
		if !tb.Allow1() {
			t.Errorf("Expected Allow1() to succeed on iteration %d", i)
		}
	}

	// Should reject when empty
	if tb.Allow1() {
		t.Error("Expected Allow1() to fail after consuming burst")
	}
}

// TestTokenBucketAllowN tests consuming multiple tokens at once.
func TestTokenBucketAllowN(t *testing.T) {
	tb := NewTokenBucket(100, 10)

	// Should allow consuming 5 tokens
	if !tb.Allow(5) {
		t.Error("Expected Allow(5) to succeed")
	}

	// Should have ~5 tokens left
	tokens := tb.Tokens()
	if tokens < 4 || tokens > 5.5 {
		t.Errorf("Expected ~5 tokens, got %f", tokens)
	}

	// Should allow consuming 5 more
	if !tb.Allow(5) {
		t.Error("Expected Allow(5) to succeed")
	}

	// Should reject consuming more
	if tb.Allow(1) {
		t.Error("Expected Allow(1) to fail after consuming all tokens")
	}
}

// TestTokenBucketAllowZero tests consuming 0 tokens.
func TestTokenBucketAllowZero(t *testing.T) {
	tb := NewTokenBucket(10, 5)
	tb.Allow(5) // Empty the bucket

	// Allow(0) should always succeed
	if !tb.Allow(0) {
		t.Error("Expected Allow(0) to always succeed")
	}

	// Allow(-1) should also succeed (treated as 0)
	if !tb.Allow(-1) {
		t.Error("Expected Allow(-1) to succeed")
	}
}

// TestTokenBucketRefill tests token refill over time.
func TestTokenBucketRefill(t *testing.T) {
	// 10 tokens per second, burst of 5
	tb := NewTokenBucket(10, 5)

	// Consume all tokens
	for i := 0; i < 5; i++ {
		tb.Allow1()
	}

	// Should be empty
	if tb.Allow1() {
		t.Error("Expected bucket to be empty")
	}

	// Wait for refill (100ms should give ~1 token)
	time.Sleep(150 * time.Millisecond)

	// Should have at least 1 token now
	if !tb.Allow1() {
		t.Error("Expected at least 1 token after refill")
	}
}

// TestTokenBucketWaitTime tests wait time calculation.
func TestTokenBucketWaitTime(t *testing.T) {
	tb := NewTokenBucket(10, 5)

	// Empty the bucket
	for i := 0; i < 5; i++ {
		tb.Allow1()
	}

	// Wait time for 1 token should be ~100ms (1/10 second)
	waitTime := tb.WaitTime(1)
	if waitTime < 50*time.Millisecond || waitTime > 150*time.Millisecond {
		t.Errorf("Expected wait time ~100ms, got %v", waitTime)
	}

	// Wait time for 2 tokens should be ~200ms
	waitTime = tb.WaitTime(2)
	if waitTime < 150*time.Millisecond || waitTime > 250*time.Millisecond {
		t.Errorf("Expected wait time ~200ms, got %v", waitTime)
	}
}

// TestTokenBucketWaitTimeWhenAvailable tests wait time when tokens available.
func TestTokenBucketWaitTimeWhenAvailable(t *testing.T) {
	tb := NewTokenBucket(10, 5)

	// Wait time for 1 token should be 0 (tokens available)
	waitTime := tb.WaitTime(1)
	if waitTime != 0 {
		t.Errorf("Expected wait time 0, got %v", waitTime)
	}
}

// TestTokenBucketSetRate tests updating the rate.
func TestTokenBucketSetRate(t *testing.T) {
	tb := NewTokenBucket(10, 5)
	tb.SetRate(20)

	if tb.Rate() != 20 {
		t.Errorf("Expected rate 20, got %f", tb.Rate())
	}

	// Negative rate should become 0
	tb.SetRate(-5)
	if tb.Rate() != 0 {
		t.Errorf("Expected rate 0, got %f", tb.Rate())
	}
}

// TestTokenBucketSetBurst tests updating the burst.
func TestTokenBucketSetBurst(t *testing.T) {
	tb := NewTokenBucket(10, 5)
	tb.SetBurst(10)

	if tb.Burst() != 10 {
		t.Errorf("Expected burst 10, got %f", tb.Burst())
	}

	// Negative burst should become 0
	tb.SetBurst(-5)
	if tb.Burst() != 0 {
		t.Errorf("Expected burst 0, got %f", tb.Burst())
	}
}

// TestTokenBucketConcurrent tests concurrent access.
func TestTokenBucketConcurrent(t *testing.T) {
	tb := NewTokenBucket(1000000, 1000) // Very high rate to avoid refill issues

	var allowed int
	var rejected int
	var mu sync.Mutex

	// Run 100 goroutines, each trying to consume 10 tokens
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				if tb.Allow1() {
					mu.Lock()
					allowed++
					mu.Unlock()
				} else {
					mu.Lock()
					rejected++
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()

	// Should allow exactly burst tokens (1000) and reject the rest
	if allowed != 1000 {
		t.Errorf("Expected 1000 allowed, got %d", allowed)
	}
	if rejected != 0 {
		t.Errorf("Expected 0 rejected, got %d", rejected)
	}
}

// TestTokenBucketBurst tests burst allowance.
func TestTokenBucketBurst(t *testing.T) {
	// Rate of 1 per second, burst of 100
	tb := NewTokenBucket(1, 100)

	// Should allow 100 requests immediately (burst)
	allowed := 0
	for i := 0; i < 100; i++ {
		if tb.Allow1() {
			allowed++
		}
	}

	if allowed != 100 {
		t.Errorf("Expected 100 allowed in burst, got %d", allowed)
	}

	// Next request should be rejected
	if tb.Allow1() {
		t.Error("Expected rejection after burst")
	}
}

// BenchmarkTokenBucketAllow benchmarks token consumption.
func BenchmarkTokenBucketAllow(b *testing.B) {
	tb := NewTokenBucket(1000000, float64(b.N)+1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tb.Allow1()
	}
}

// BenchmarkTokenBucketAllowConcurrent benchmarks concurrent token consumption.
func BenchmarkTokenBucketAllowConcurrent(b *testing.B) {
	tb := NewTokenBucket(1000000, float64(b.N)+1000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			tb.Allow1()
		}
	})
}
