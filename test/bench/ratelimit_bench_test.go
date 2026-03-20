package bench_test

import (
	"testing"

	"github.com/ersinkoc/tentaserve/internal/gateway/ratelimit"
)

// BenchmarkTokenBucket measures the throughput of single-goroutine Allow1
// calls on a token bucket with a very high rate so that every call succeeds
// (hot path).
func BenchmarkTokenBucket(b *testing.B) {
	b.ReportAllocs()

	// 1 000 000 tokens/sec with burst of 1 000 000 guarantees every
	// Allow1 succeeds during the benchmark window.
	tb := ratelimit.NewTokenBucket(1_000_000, 1_000_000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tb.Allow1()
	}
}

// BenchmarkTokenBucketConcurrent measures contended Allow1 performance
// across GOMAXPROCS goroutines, exercising the lock-free CAS loop.
func BenchmarkTokenBucketConcurrent(b *testing.B) {
	b.ReportAllocs()

	tb := ratelimit.NewTokenBucket(1_000_000, 1_000_000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			tb.Allow1()
		}
	})
}
