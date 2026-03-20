package bench_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/ersinkoc/tentaserve/internal/gateway/cache"
)

// newTestCache creates an enabled ShardedCache suitable for benchmarks.
func newTestCache() *cache.Cache {
	cfg := cache.DefaultConfig()
	cfg.Enabled = true
	cfg.MaxEntries = 100000
	cfg.TTL = 5 * time.Minute
	return cache.New(cfg)
}

// newTestEntry creates a cache.Entry with a small JSON-like body.
func newTestEntry(i int) *cache.Entry {
	body := []byte(fmt.Sprintf(`{"id":%d,"name":"item-%d","value":"bench"}`, i, i))
	return &cache.Entry{
		StatusCode: http.StatusOK,
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body:      body,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
}

// BenchmarkCacheHit measures the cost of reading a key that is already present
// in the sharded LRU cache.
func BenchmarkCacheHit(b *testing.B) {
	b.ReportAllocs()

	c := newTestCache()
	const numKeys = 1000
	keys := make([]string, numKeys)
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key-%d", i)
		keys[i] = key
		c.Set(key, newTestEntry(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Get(keys[i%numKeys])
	}
}

// BenchmarkCacheMiss measures the cost of looking up a key that does not exist
// in the cache (miss path).
func BenchmarkCacheMiss(b *testing.B) {
	b.ReportAllocs()

	c := newTestCache()
	// Populate with a disjoint key space so every lookup below is a miss.
	for i := 0; i < 100; i++ {
		c.Set(fmt.Sprintf("existing-%d", i), newTestEntry(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Get(fmt.Sprintf("miss-%d", i))
	}
}

// BenchmarkCacheConcurrent measures throughput of mixed Get/Set operations
// under heavy concurrency (GOMAXPROCS goroutines).
func BenchmarkCacheConcurrent(b *testing.B) {
	b.ReportAllocs()

	c := newTestCache()
	const numKeys = 1000
	keys := make([]string, numKeys)
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key-%d", i)
		keys[i] = key
		c.Set(key, newTestEntry(i))
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		// Each goroutine gets its own counter to spread keys.
		var idx int
		for pb.Next() {
			k := keys[idx%numKeys]
			if idx%4 == 0 {
				// 25 % writes
				c.Set(k, newTestEntry(idx%numKeys))
			} else {
				// 75 % reads
				_ = c.Get(k)
			}
			idx++
		}
	})
}
