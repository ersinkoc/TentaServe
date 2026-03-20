package schema

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// TestNewDataLoader tests DataLoader creation.
func TestNewDataLoader(t *testing.T) {
	batchFunc := func(ctx context.Context, keys []string) ([]any, []error) {
		return nil, nil
	}

	dl := NewDataLoader(batchFunc, 5*time.Millisecond, 10)

	if dl == nil {
		t.Fatal("Expected DataLoader to be created")
	}

	if dl.batchFunc == nil {
		t.Error("Expected batchFunc to be set")
	}

	if dl.batchWindow != 5*time.Millisecond {
		t.Errorf("Expected batchWindow 5ms, got %v", dl.batchWindow)
	}

	if dl.maxBatchSize != 10 {
		t.Errorf("Expected maxBatchSize 10, got %d", dl.maxBatchSize)
	}
}

// TestDataLoader_Load_Single tests a single load.
func TestDataLoader_Load_Single(t *testing.T) {
	batchCalled := false
	batchFunc := func(ctx context.Context, keys []string) ([]any, []error) {
		batchCalled = true
		if len(keys) != 1 {
			t.Errorf("Expected 1 key, got %d", len(keys))
		}
		if keys[0] != "key1" {
			t.Errorf("Expected key 'key1', got %s", keys[0])
		}
		return []any{"value1"}, nil
	}

	dl := NewDataLoader(batchFunc, 5*time.Millisecond, 10)

	val, err := dl.Load(context.Background(), "key1")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if val != "value1" {
		t.Errorf("Expected value 'value1', got %v", val)
	}

	if !batchCalled {
		t.Error("Expected batchFunc to be called")
	}
}

// TestDataLoader_Load_MultipleBatched tests that multiple loads are batched.
func TestDataLoader_Load_MultipleBatched(t *testing.T) {
	var batchCallCount int
	var batchKeys []string

	batchFunc := func(ctx context.Context, keys []string) ([]any, []error) {
		batchCallCount++
		batchKeys = keys

		values := make([]any, len(keys))
		for i, k := range keys {
			values[i] = "value_" + k
		}
		return values, nil
	}

	dl := NewDataLoader(batchFunc, 50*time.Millisecond, 100)

	// Load 3 keys concurrently
	var results [3]struct {
		val any
		err error
	}

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx].val, results[idx].err = dl.Load(context.Background(), "key"+string(rune('0'+idx)))
		}(i)
	}

	wg.Wait()

	// Should be 1 batch call
	if batchCallCount != 1 {
		t.Errorf("Expected 1 batch call, got %d", batchCallCount)
	}

	// Should have 3 keys
	if len(batchKeys) != 3 {
		t.Errorf("Expected 3 keys in batch, got %d: %v", len(batchKeys), batchKeys)
	}

	// Check results
	for i := 0; i < 3; i++ {
		expectedVal := "value_key" + string(rune('0'+i))
		if results[i].err != nil {
			t.Errorf("Load %d failed: %v", i, results[i].err)
		}
		if results[i].val != expectedVal {
			t.Errorf("Expected value %q for key%d, got %v", expectedVal, i, results[i].val)
		}
	}
}

// TestDataLoader_Load_MaxBatchSize tests dispatch on max batch size.
func TestDataLoader_Load_MaxBatchSize(t *testing.T) {
	var batchCallCount int

	batchFunc := func(ctx context.Context, keys []string) ([]any, []error) {
		batchCallCount++

		values := make([]any, len(keys))
		for i, k := range keys {
			values[i] = "value_" + k
		}
		return values, nil
	}

	// Small max batch size
	dl := NewDataLoader(batchFunc, 50*time.Millisecond, 3)

	// Load 10 keys
	for i := 0; i < 10; i++ {
		go func(idx int) {
			dl.Load(context.Background(), "key"+string(rune('0'+idx)))
		}(i)
	}

	// Wait for batches to complete
	time.Sleep(100 * time.Millisecond)

	// Should have at least 3 batch calls (10 / 3 = 3.33, rounded up = 4)
	if batchCallCount < 3 {
		t.Errorf("Expected at least 3 batch calls for 10 items with max 3, got %d", batchCallCount)
	}
}

// TestDataLoader_Load_BatchWindow tests that batch window triggers dispatch.
func TestDataLoader_Load_BatchWindow(t *testing.T) {
	var batchCallCount int
	batchCalled := make(chan bool, 1)

	batchFunc := func(ctx context.Context, keys []string) ([]any, []error) {
		batchCallCount++
		batchCalled <- true

		values := make([]any, len(keys))
		for i := range keys {
			values[i] = "value"
		}
		return values, nil
	}

	// Short batch window
	dl := NewDataLoader(batchFunc, 10*time.Millisecond, 100)

	// Load 1 key
	go dl.Load(context.Background(), "key1")

	// Wait for batch to be triggered by timer
	select {
	case <-batchCalled:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("Batch was not triggered by timer")
	}

	if batchCallCount != 1 {
		t.Errorf("Expected 1 batch call, got %d", batchCallCount)
	}
}

// TestDataLoader_Load_ContextCancellation tests context cancellation.
func TestDataLoader_Load_ContextCancellation(t *testing.T) {
	batchFunc := func(ctx context.Context, keys []string) ([]any, []error) {
		// Simulate slow batch
		time.Sleep(100 * time.Millisecond)
		return make([]any, len(keys)), nil
	}

	dl := NewDataLoader(batchFunc, 50*time.Millisecond, 100)

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := dl.Load(ctx, "key1")
	if err == nil {
		t.Error("Expected error due to context cancellation")
	}

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

// TestDataLoader_Load_MultipleRequests_NoCrossRequest tests no cross-request batching.
func TestDataLoader_Load_MultipleRequests_NoCrossRequest(t *testing.T) {
	// Track which keys go in which batch
	var batches [][]string
	var mu sync.Mutex

	batchFunc := func(ctx context.Context, keys []string) ([]any, []error) {
		mu.Lock()
		batches = append(batches, keys)
		mu.Unlock()

		values := make([]any, len(keys))
		for i := range keys {
			values[i] = "value"
		}
		return values, nil
	}

	// Create two separate DataLoaders (simulating different requests)
	dl1 := NewDataLoader(batchFunc, 50*time.Millisecond, 100)
	dl2 := NewDataLoader(batchFunc, 50*time.Millisecond, 100)

	// Load from both
	go dl1.Load(context.Background(), "dl1_key1")
	go dl1.Load(context.Background(), "dl1_key2")
	go dl2.Load(context.Background(), "dl2_key1")
	go dl2.Load(context.Background(), "dl2_key2")

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Should have at least 2 separate batches (one per DataLoader)
	if len(batches) < 2 {
		t.Errorf("Expected at least 2 separate batches, got %d", len(batches))
	}

	// Verify keys are not mixed
	for _, batch := range batches {
		hasDL1 := false
		hasDL2 := false
		for _, key := range batch {
			if len(key) > 3 && key[:3] == "dl1" {
				hasDL1 = true
			}
			if len(key) > 3 && key[:3] == "dl2" {
				hasDL2 = true
			}
		}
		if hasDL1 && hasDL2 {
			t.Error("Keys from different DataLoaders were mixed in the same batch")
		}
	}
}

// TestDataLoader_Load_MultipleBatches tests multiple batches over time.
func TestDataLoader_Load_MultipleBatches(t *testing.T) {
	var batchCallCount int

	batchFunc := func(ctx context.Context, keys []string) ([]any, []error) {
		batchCallCount++
		values := make([]any, len(keys))
		for i := range keys {
			values[i] = "value_" + keys[i]
		}
		return values, nil
	}

	dl := NewDataLoader(batchFunc, 20*time.Millisecond, 100)

	// First batch
	val1, _ := dl.Load(context.Background(), "key1")
	time.Sleep(30 * time.Millisecond) // Wait for batch to dispatch

	// Second batch
	val2, _ := dl.Load(context.Background(), "key2")
	time.Sleep(30 * time.Millisecond) // Wait for batch to dispatch

	// Third batch
	val3, _ := dl.Load(context.Background(), "key3")
	time.Sleep(30 * time.Millisecond) // Wait for batch to dispatch

	if batchCallCount != 3 {
		t.Errorf("Expected 3 batch calls, got %d", batchCallCount)
	}

	if val1 != "value_key1" {
		t.Errorf("Expected value_key1, got %v", val1)
	}
	if val2 != "value_key2" {
		t.Errorf("Expected value_key2, got %v", val2)
	}
	if val3 != "value_key3" {
		t.Errorf("Expected value_key3, got %v", val3)
	}
}

// TestDataLoader_Load_BatchError tests error handling in batch.
func TestDataLoader_Load_BatchError(t *testing.T) {
	batchFunc := func(ctx context.Context, keys []string) ([]any, []error) {
		values := make([]any, len(keys))
		errs := make([]error, len(keys))

		// Return error for specific key
		for i, key := range keys {
			if key == "error_key" {
				errs[i] = errors.New("error for " + key)
			} else {
				values[i] = "value_" + key
			}
		}
		return values, errs
	}

	dl := NewDataLoader(batchFunc, 50*time.Millisecond, 100)

	var results [2]struct {
		val any
		err error
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		results[0].val, results[0].err = dl.Load(context.Background(), "good_key")
	}()

	go func() {
		defer wg.Done()
		results[1].val, results[1].err = dl.Load(context.Background(), "error_key")
	}()

	wg.Wait()

	if results[0].err != nil {
		t.Errorf("Expected no error for good_key, got %v", results[0].err)
	}
	if results[0].val != "value_good_key" {
		t.Errorf("Expected value_good_key, got %v", results[0].val)
	}

	if results[1].err == nil {
		t.Error("Expected error for error_key")
	}
	if results[1].err != nil && results[1].err.Error() != "error for error_key" {
		t.Errorf("Expected specific error message, got %v", results[1].err)
	}
}

// TestDataLoader_LoadMany tests LoadMany convenience method.
func TestDataLoader_LoadMany(t *testing.T) {
	batchFunc := func(ctx context.Context, keys []string) ([]any, []error) {
		values := make([]any, len(keys))
		for i, k := range keys {
			values[i] = "value_" + k
		}
		return values, nil
	}

	dl := NewDataLoader(batchFunc, 50*time.Millisecond, 100)

	keys := []string{"a", "b", "c"}
	values, errs := dl.LoadMany(context.Background(), keys)

	if len(values) != 3 {
		t.Errorf("Expected 3 values, got %d", len(values))
	}
	if len(errs) != 3 {
		t.Errorf("Expected 3 errors (nil), got %d", len(errs))
	}

	for i, key := range keys {
		if errs[i] != nil {
			t.Errorf("Unexpected error for key %s: %v", key, errs[i])
		}
		expectedVal := "value_" + key
		if values[i] != expectedVal {
			t.Errorf("Expected %s, got %v", expectedVal, values[i])
		}
	}
}

// TestDataLoader_LoadMany_Empty tests LoadMany with empty keys.
func TestDataLoader_LoadMany_Empty(t *testing.T) {
	batchCalled := false
	batchFunc := func(ctx context.Context, keys []string) ([]any, []error) {
		batchCalled = true
		return nil, nil
	}

	dl := NewDataLoader(batchFunc, 50*time.Millisecond, 100)

	values, errs := dl.LoadMany(context.Background(), []string{})

	if batchCalled {
		t.Error("Batch should not be called for empty keys")
	}
	if len(values) != 0 {
		t.Errorf("Expected 0 values, got %d", len(values))
	}
	if len(errs) != 0 {
		t.Errorf("Expected 0 errors, got %d", len(errs))
	}
}

// TestDataLoader_Stop tests stopping the DataLoader.
func TestDataLoader_Stop(t *testing.T) {
	batchFunc := func(ctx context.Context, keys []string) ([]any, []error) {
		return make([]any, len(keys)), nil
	}

	dl := NewDataLoader(batchFunc, 50*time.Millisecond, 100)

	// Stop the DataLoader
	dl.Stop()

	// Try to load after stop
	_, err := dl.Load(context.Background(), "key1")
	if err == nil {
		t.Error("Expected error after stopping DataLoader")
	}
}

// TestDataLoader_PendingCount tests PendingCount method.
func TestDataLoader_PendingCount(t *testing.T) {
	batchFunc := func(ctx context.Context, keys []string) ([]any, []error) {
		return make([]any, len(keys)), nil
	}

	dl := NewDataLoader(batchFunc, 100*time.Millisecond, 100)

	if dl.PendingCount() != 0 {
		t.Errorf("Expected 0 pending initially, got %d", dl.PendingCount())
	}

	// Start a load (will wait for batch window)
	go func() {
		dl.Load(context.Background(), "key1")
	}()

	time.Sleep(10 * time.Millisecond) // Let it add to pending

	if dl.PendingCount() != 1 {
		t.Errorf("Expected 1 pending, got %d", dl.PendingCount())
	}
}

// BenchmarkDataLoader benchmarks concurrent loads.
func BenchmarkDataLoader(b *testing.B) {
	batchFunc := func(ctx context.Context, keys []string) ([]any, []error) {
		values := make([]any, len(keys))
		for i := range keys {
			values[i] = i
		}
		return values, nil
	}

	dl := NewDataLoader(batchFunc, 5*time.Millisecond, 100)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			dl.Load(context.Background(), "key"+string(rune('0'+i%10)))
			i++
		}
	})
}
