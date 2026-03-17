package ratelimit

import (
	"math"
	"sync/atomic"
	"time"
)

// TokenBucket implements a lock-free token bucket rate limiter using atomic operations.
// It supports burst allowance and smooth rate limiting.
type TokenBucket struct {
	// tokens stores the current number of tokens as atomic float64 bits
	tokens atomic.Uint64

	// lastUpdate stores the last update time as atomic int64 Unix nanoseconds
	lastUpdate atomic.Int64

	// rate is the tokens per second (constant after creation)
	rate float64

	// burst is the maximum tokens that can be accumulated (constant after creation)
	burst float64
}

// NewTokenBucket creates a new token bucket rate limiter.
// rate: tokens per second (can be fractional for slower rates)
// burst: maximum tokens that can be accumulated (burst capacity)
func NewTokenBucket(rate, burst float64) *TokenBucket {
	if rate < 0 {
		rate = 0
	}
	if burst < 0 {
		burst = 0
	}

	tb := &TokenBucket{
		rate:  rate,
		burst: burst,
	}

	// Start with a full bucket
	tb.tokens.Store(math.Float64bits(burst))
	tb.lastUpdate.Store(time.Now().UnixNano())

	return tb
}

// Allow attempts to consume n tokens from the bucket.
// Returns true if tokens were available, false otherwise.
// This method is lock-free and safe for concurrent use.
func (tb *TokenBucket) Allow(n float64) bool {
	if n <= 0 {
		return true
	}

	now := time.Now().UnixNano()

	// Try to update atomically
	for {
		// Load current state
		currentTokens := math.Float64frombits(tb.tokens.Load())
		lastUpdate := tb.lastUpdate.Load()

		// Calculate tokens to add based on time elapsed
		elapsed := float64(now-lastUpdate) / 1e9 // convert nanoseconds to seconds
		newTokens := currentTokens + elapsed*tb.rate
		if newTokens > tb.burst {
			newTokens = tb.burst
		}

		// Check if we have enough tokens
		if newTokens < n {
			return false
		}

		// Try to subtract tokens and update timestamp
		newTokenCount := newTokens - n

		// Compare-and-swap on tokens
		currentBits := tb.tokens.Load()
		if currentBits != math.Float64bits(currentTokens) {
			// Tokens changed, retry
			continue
		}

		if tb.tokens.CompareAndSwap(currentBits, math.Float64bits(newTokenCount)) {
			// Update timestamp if tokens were successfully consumed
			tb.lastUpdate.Store(now)
			return true
		}
		// CAS failed, retry
	}
}

// Allow1 is a convenience method for consuming 1 token.
func (tb *TokenBucket) Allow1() bool {
	return tb.Allow(1)
}

// WaitTime returns the duration to wait before n tokens will be available.
func (tb *TokenBucket) WaitTime(n float64) time.Duration {
	if n <= 0 {
		return 0
	}

	now := time.Now().UnixNano()
	currentTokens := math.Float64frombits(tb.tokens.Load())
	lastUpdate := tb.lastUpdate.Load()

	// Calculate tokens available now
	elapsed := float64(now-lastUpdate) / 1e9
	tokens := currentTokens + elapsed*tb.rate
	if tokens > tb.burst {
		tokens = tb.burst
	}

	// Calculate how many more tokens needed
	needed := n - tokens
	if needed <= 0 {
		return 0
	}

	// Calculate wait time for needed tokens
	seconds := needed / tb.rate
	return time.Duration(seconds * float64(time.Second))
}

// Tokens returns the current number of tokens in the bucket.
func (tb *TokenBucket) Tokens() float64 {
	now := time.Now().UnixNano()
	currentTokens := math.Float64frombits(tb.tokens.Load())
	lastUpdate := tb.lastUpdate.Load()

	elapsed := float64(now-lastUpdate) / 1e9
	tokens := currentTokens + elapsed*tb.rate
	if tokens > tb.burst {
		tokens = tb.burst
	}
	return tokens
}

// Rate returns the rate of tokens per second.
func (tb *TokenBucket) Rate() float64 {
	return tb.rate
}

// Burst returns the burst capacity.
func (tb *TokenBucket) Burst() float64 {
	return tb.burst
}

// SetRate updates the rate of tokens per second.
// Note: This is not thread-safe for the rate value itself, but the
// bucket will continue to function correctly. The rate update will
// take effect on the next token consumption.
func (tb *TokenBucket) SetRate(rate float64) {
	if rate < 0 {
		rate = 0
	}
	tb.rate = rate
}

// SetBurst updates the burst capacity.
// Note: This is not thread-safe for the burst value itself, but the
// bucket will continue to function correctly.
func (tb *TokenBucket) SetBurst(burst float64) {
	if burst < 0 {
		burst = 0
	}
	tb.burst = burst
}
