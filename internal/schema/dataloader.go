package schema

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DataLoader batches multiple load requests into a single batch call.
// It implements the DataLoader pattern for efficient batching of N+1 queries.
//
// Usage:
//   loader := NewDataLoader(batchFunc, 5*time.Millisecond, 100)
//   val1, err1 := loader.Load(ctx, "key1")
//   val2, err2 := loader.Load(ctx, "key2")
//   // Both calls are batched and dispatched together
//
type DataLoader struct {
	batchFunc   BatchFunc
	batchWindow time.Duration
	maxBatchSize int

	// Pending loads waiting to be dispatched
	pending []loadRequest

	// Mutex for thread-safe access
	mu sync.Mutex

	// Timer for batch window
	timer *time.Timer

	// Closed when loader is stopped
	stopCh chan struct{}
}

// BatchFunc is the function called to load a batch of keys.
// It receives all pending keys and returns values in the same order.
type BatchFunc func(ctx context.Context, keys []string) ([]any, []error)

// loadRequest represents a single load request waiting to be dispatched.
type loadRequest struct {
	key      string
	resultCh chan loadResult
}

// loadResult is the result of a single load operation.
type loadResult struct {
	value any
	err   error
}

// NewDataLoader creates a new DataLoader.
//
// batchFunc:      The function to call with batched keys
// batchWindow:    Maximum time to wait before dispatching a batch
// maxBatchSize:   Maximum number of items to batch before immediate dispatch (0 = no limit)
func NewDataLoader(batchFunc BatchFunc, batchWindow time.Duration, maxBatchSize int) *DataLoader {
	return &DataLoader{
		batchFunc:    batchFunc,
		batchWindow:  batchWindow,
		maxBatchSize: maxBatchSize,
		pending:      make([]loadRequest, 0),
		stopCh:       make(chan struct{}),
	}
}

// Load requests a value by key. Multiple concurrent Load calls are batched.
// Returns when the batch is dispatched and the result is available.
func (dl *DataLoader) Load(ctx context.Context, key string) (any, error) {
	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Create result channel
	req := loadRequest{
		key:      key,
		resultCh: make(chan loadResult, 1),
	}

	dl.mu.Lock()

	// Add to pending batch
	dl.pending = append(dl.pending, req)

	// Check if we should dispatch immediately
	shouldDispatch := dl.maxBatchSize > 0 && len(dl.pending) >= dl.maxBatchSize

	// Start batch timer if not already running
	if dl.timer == nil && !shouldDispatch {
		dl.timer = time.AfterFunc(dl.batchWindow, func() {
			dl.dispatch(nil)
		})
	}

	dl.mu.Unlock()

	// Dispatch immediately if max batch size reached
	if shouldDispatch {
		dl.dispatch(nil)
	}

	// Wait for result
	select {
	case result := <-req.resultCh:
		return result.value, result.err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-dl.stopCh:
		return nil, fmt.Errorf("dataloader stopped")
	}
}

// LoadMany loads multiple keys and returns their values.
// This is a convenience method that batches all keys together.
func (dl *DataLoader) LoadMany(ctx context.Context, keys []string) ([]any, []error) {
	if len(keys) == 0 {
		return []any{}, nil
	}

	// Load all keys concurrently
	results := make([]any, len(keys))
	errors := make([]error, len(keys))

	var wg sync.WaitGroup
	for i, key := range keys {
		wg.Add(1)
		go func(idx int, k string) {
			defer wg.Done()
			val, err := dl.Load(ctx, k)
			results[idx] = val
			errors[idx] = err
		}(i, key)
	}

	wg.Wait()

	return results, errors
}

// Clear clears the cache for a specific key (if caching were implemented).
// Currently a no-op since we don't cache results.
func (dl *DataLoader) Clear(key string) {
	// No-op: this DataLoader implementation doesn't cache
}

// ClearAll clears all cached values.
// Currently a no-op since we don't cache results.
func (dl *DataLoader) ClearAll() {
	// No-op: this DataLoader implementation doesn't cache
}

// Stop stops the DataLoader and cancels any pending loads.
func (dl *DataLoader) Stop() {
	dl.mu.Lock()
	close(dl.stopCh)
	if dl.timer != nil {
		dl.timer.Stop()
		dl.timer = nil
	}
	// Cancel pending requests
	for _, req := range dl.pending {
		req.resultCh <- loadResult{err: fmt.Errorf("dataloader stopped")}
	}
	dl.pending = nil
	dl.mu.Unlock()
}

// dispatch sends the pending batch to the batch function.
// If timer is provided, it's the timer that triggered the dispatch.
func (dl *DataLoader) dispatch(timer *time.Timer) {
	dl.mu.Lock()

	// Stop the timer if it's still running
	if timer != nil && dl.timer != nil && timer == dl.timer {
		dl.timer.Stop()
	}
	dl.timer = nil

	// Copy pending requests
	batch := dl.pending
	dl.pending = make([]loadRequest, 0)

	dl.mu.Unlock()

	// No pending requests
	if len(batch) == 0 {
		return
	}

	// Extract keys
	keys := make([]string, len(batch))
	for i, req := range batch {
		keys[i] = req.key
	}

	// Call batch function
	values, errs := dl.batchFunc(context.Background(), keys)

	// Send results back to requesters
	for i, req := range batch {
		var val any
		var err error

		if i < len(values) {
			val = values[i]
		}
		if i < len(errs) && errs[i] != nil {
			err = errs[i]
		}

		req.resultCh <- loadResult{value: val, err: err}
	}
}

// PendingCount returns the number of pending load requests.
func (dl *DataLoader) PendingCount() int {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	return len(dl.pending)
}

// DataLoaderCache provides a simple cache for DataLoader results.
// This is optional - DataLoader works without caching.
type DataLoaderCache struct {
	data map[string]cacheEntry
	mu   sync.RWMutex
}

type cacheEntry struct {
	value     any
	err       error
	expiresAt time.Time
}

// NewDataLoaderCache creates a new cache.
func NewDataLoaderCache() *DataLoaderCache {
	return &DataLoaderCache{
		data: make(map[string]cacheEntry),
	}
}

// Get retrieves a value from the cache.
func (c *DataLoaderCache) Get(key string) (any, error, bool) {
	c.mu.RLock()
	entry, ok := c.data[key]
	c.mu.RUnlock()

	if !ok {
		return nil, nil, false
	}

	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		// Expired
		c.mu.Lock()
		delete(c.data, key)
		c.mu.Unlock()
		return nil, nil, false
	}

	return entry.value, entry.err, true
}

// Set stores a value in the cache.
func (c *DataLoaderCache) Set(key string, value any, err error, ttl time.Duration) {
	c.mu.Lock()
	c.data[key] = cacheEntry{
		value:     value,
		err:       err,
		expiresAt: time.Now().Add(ttl),
	}
	c.mu.Unlock()
}

// Clear removes a key from the cache.
func (c *DataLoaderCache) Clear(key string) {
	c.mu.Lock()
	delete(c.data, key)
	c.mu.Unlock()
}

// ClearAll removes all keys from the cache.
func (c *DataLoaderCache) ClearAll() {
	c.mu.Lock()
	c.data = make(map[string]cacheEntry)
	c.mu.Unlock()
}
