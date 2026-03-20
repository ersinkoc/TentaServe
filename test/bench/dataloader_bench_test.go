package bench_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ersinkoc/tentaserve/internal/schema"
)

// trivialBatch is a BatchFunc that echoes back "value-<key>" for every key.
// It simulates a zero-latency data source so the benchmark measures only
// DataLoader overhead (channel coordination, batching, dispatch).
func trivialBatch(_ context.Context, keys []string) ([]any, []error) {
	values := make([]any, len(keys))
	errs := make([]error, len(keys))
	for i, k := range keys {
		values[i] = "value-" + k
	}
	return values, errs
}

// BenchmarkDataLoaderBatch measures throughput when many concurrent Load
// calls are batched together. The maxBatchSize is set so that every batch
// dispatches as soon as the goroutines have enqueued enough requests.
func BenchmarkDataLoaderBatch(b *testing.B) {
	b.ReportAllocs()

	const batchSize = 50
	loader := schema.NewDataLoader(trivialBatch, 2*time.Millisecond, batchSize)
	defer loader.Stop()
	ctx := context.Background()

	b.ResetTimer()

	// We run b.N loads spread across concurrent goroutines. Each goroutine
	// issues loads that will be batched by the DataLoader.
	var wg sync.WaitGroup
	// Determine a reasonable concurrency level.
	concurrency := 8
	perGoroutine := b.N / concurrency
	if perGoroutine < 1 {
		perGoroutine = 1
	}

	for g := 0; g < concurrency; g++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				key := fmt.Sprintf("key-%d", offset+i)
				_, err := loader.Load(ctx, key)
				if err != nil {
					b.Error(err)
					return
				}
			}
		}(g * perGoroutine)
	}

	wg.Wait()
}

// BenchmarkDataLoaderIndividual measures the cost of individual (unbatched)
// Load calls that each trigger their own batch dispatch via the time window.
// This is the worst-case path where every call is its own batch of size 1.
func BenchmarkDataLoaderIndividual(b *testing.B) {
	b.ReportAllocs()

	// Use maxBatchSize=1 so every single Load triggers an immediate dispatch,
	// removing any batching benefit and measuring pure per-call overhead.
	loader := schema.NewDataLoader(trivialBatch, 10*time.Millisecond, 1)
	defer loader.Stop()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		_, err := loader.Load(ctx, key)
		if err != nil {
			b.Fatal(err)
		}
	}
}
