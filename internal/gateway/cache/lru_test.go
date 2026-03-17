package cache

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

// TestConfigDefault tests default config.
func TestConfigDefault(t *testing.T) {
	config := DefaultConfig()

	if config.Enabled {
		t.Error("Expected default enabled to be false")
	}
	if config.MaxSize != 100*1024*1024 {
		t.Errorf("Expected default max size 100MB, got %d", config.MaxSize)
	}
	if config.MaxEntries != 10000 {
		t.Errorf("Expected default max entries 10000, got %d", config.MaxEntries)
	}
	if config.TTL != 5*time.Minute {
		t.Errorf("Expected default TTL 5m, got %v", config.TTL)
	}
}

// TestConfigIsMethodCacheable tests method checking.
func TestConfigIsMethodCacheable(t *testing.T) {
	config := DefaultConfig()

	tests := []struct {
		method   string
		expected bool
	}{
		{"GET", true},
		{"HEAD", true},
		{"POST", false},
		{"PUT", false},
		{"DELETE", false},
		{"PATCH", false},
	}

	for _, tt := range tests {
		if config.IsMethodCacheable(tt.method) != tt.expected {
			t.Errorf("Method %s: expected %v, got %v", tt.method, tt.expected, config.IsMethodCacheable(tt.method))
		}
	}
}

// TestConfigIsStatusCodeCacheable tests status code checking.
func TestConfigIsStatusCodeCacheable(t *testing.T) {
	config := DefaultConfig()

	tests := []struct {
		code     int
		expected bool
	}{
		{200, true},
		{301, true},
		{302, true},
		{404, true},
		{500, false},
		{503, false},
	}

	for _, tt := range tests {
		if config.IsStatusCodeCacheable(tt.code) != tt.expected {
			t.Errorf("Code %d: expected %v, got %v", tt.code, tt.expected, config.IsStatusCodeCacheable(tt.code))
		}
	}
}

// TestCacheNew tests cache creation.
func TestCacheNew(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	cache := New(config)

	if cache == nil {
		t.Fatal("Expected non-nil cache")
	}
	if cache.numShards != 256 {
		t.Errorf("Expected 256 shards, got %d", cache.numShards)
	}
	if len(cache.shards) != 256 {
		t.Errorf("Expected 256 shards, got %d", len(cache.shards))
	}
}

// TestCacheGetSet tests basic get/set operations.
func TestCacheGetSet(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	cache := New(config)

	entry := &Entry{
		StatusCode: 200,
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
		Body:       []byte("test"),
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(time.Hour),
	}

	// Set
	if !cache.Set("key1", entry) {
		t.Error("Expected Set to succeed")
	}

	// Get
	got := cache.Get("key1")
	if got == nil {
		t.Fatal("Expected to get entry")
	}
	if got.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", got.StatusCode)
	}
	if string(got.Body) != "test" {
		t.Errorf("Expected body 'test', got %s", got.Body)
	}
}

// TestCacheGetDisabled tests that disabled cache returns nil.
func TestCacheGetDisabled(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = false
	cache := New(config)

	entry := &Entry{
		StatusCode: 200,
		Body:       []byte("test"),
	}

	cache.Set("key1", entry)

	got := cache.Get("key1")
	if got != nil {
		t.Error("Expected nil from disabled cache")
	}
}

// TestCacheDelete tests delete operation.
func TestCacheDelete(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	cache := New(config)

	entry := &Entry{
		StatusCode: 200,
		Body:       []byte("test"),
	}

	cache.Set("key1", entry)
	if cache.Get("key1") == nil {
		t.Fatal("Expected entry before delete")
	}

	cache.Delete("key1")

	if cache.Get("key1") != nil {
		t.Error("Expected nil after delete")
	}
}

// TestCacheClear tests clearing all entries.
func TestCacheClear(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	cache := New(config)

	for i := 0; i < 10; i++ {
		entry := &Entry{
			StatusCode: 200,
			Body:       []byte(fmt.Sprintf("test%d", i)),
		}
		cache.Set(fmt.Sprintf("key%d", i), entry)
	}

	if cache.Len() != 10 {
		t.Errorf("Expected 10 entries, got %d", cache.Len())
	}

	cache.Clear()

	if cache.Len() != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", cache.Len())
	}
}

// TestCacheLRUEviction tests LRU eviction.
func TestCacheLRUEviction(t *testing.T) {
	// Use fewer shards for predictable eviction
	config := &Config{
		Enabled:    true,
		MaxSize:    1000,
		MaxEntries: 5,
		TTL:        time.Hour,
	}
	cache := New(config)
	// Override with single shard for predictable eviction
	cache.numShards = 1
	cache.shards = []*shard{newShard(1000, 5)}

	// Add 10 entries (exceeds max entries)
	for i := 0; i < 10; i++ {
		entry := &Entry{
			StatusCode: 200,
			Body:       []byte(fmt.Sprintf("test%d", i)),
		}
		cache.Set(fmt.Sprintf("key%d", i), entry)
	}

	// Should only have 5 entries
	if cache.Len() != 5 {
		t.Errorf("Expected 5 entries after eviction, got %d", cache.Len())
	}

	// Oldest entries should be evicted
	if cache.Get("key0") != nil {
		t.Error("Expected key0 to be evicted")
	}
	if cache.Get("key1") != nil {
		t.Error("Expected key1 to be evicted")
	}
	if cache.Get("key2") != nil {
		t.Error("Expected key2 to be evicted")
	}
	if cache.Get("key3") != nil {
		t.Error("Expected key3 to be evicted")
	}
	if cache.Get("key4") != nil {
		t.Error("Expected key4 to be evicted")
	}

	// Newest should remain
	if cache.Get("key5") == nil {
		t.Error("Expected key5 to exist")
	}
	if cache.Get("key9") == nil {
		t.Error("Expected key9 to exist")
	}
}

// TestCacheSizeEviction tests size-based eviction.
func TestCacheSizeEviction(t *testing.T) {
	config := &Config{
		Enabled:    true,
		MaxSize:    500, // 500 bytes total
		MaxEntries: 1000,
		TTL:        time.Hour,
	}
	cache := New(config)
	// Override with single shard for predictable eviction
	cache.numShards = 1
	cache.shards = []*shard{newShard(500, 1000)}

	// Add entry that's too large for this shard (shard size / 2 = 250)
	largeEntry := &Entry{
		StatusCode: 200,
		Body:       make([]byte, 400), // ~400 bytes, exceeds 250 limit
	}
	if cache.Set("large", largeEntry) {
		t.Error("Expected large entry to be rejected")
	}

	// Add entries that fit
	for i := 0; i < 5; i++ {
		entry := &Entry{
			StatusCode: 200,
			Body:       []byte(fmt.Sprintf("test entry number %d", i)),
		}
		cache.Set(fmt.Sprintf("key%d", i), entry)
	}
}

// TestCacheConcurrent tests concurrent access.
func TestCacheConcurrent(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	cache := New(config)

	var wg sync.WaitGroup
	numGoroutines := 10
	numOps := 100

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				entry := &Entry{
					StatusCode: 200,
					Body:       []byte(fmt.Sprintf("value-%d-%d", id, j)),
				}
				cache.Set(key, entry)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				cache.Get(key)
			}
		}(i)
	}

	wg.Wait()

	// Cache should have entries (exact count depends on eviction)
	if cache.Len() == 0 {
		t.Error("Expected some entries in cache")
	}
}

// TestCacheStats tests statistics.
func TestCacheStats(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	cache := New(config)

	for i := 0; i < 10; i++ {
		entry := &Entry{
			StatusCode: 200,
			Body:       []byte(fmt.Sprintf("test%d", i)),
		}
		cache.Set(fmt.Sprintf("key%d", i), entry)
	}

	stats := cache.Stats()
	if stats.Entries != 10 {
		t.Errorf("Expected 10 entries, got %d", stats.Entries)
	}
	if stats.Shards != 256 {
		t.Errorf("Expected 256 shards, got %d", stats.Shards)
	}
	if stats.Size == 0 {
		t.Error("Expected non-zero size")
	}
}

// BenchmarkCacheSet benchmarks cache writes.
func BenchmarkCacheSet(b *testing.B) {
	config := DefaultConfig()
	config.Enabled = true
	cache := New(config)

	entry := &Entry{
		StatusCode: 200,
		Body:       []byte("test data"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(fmt.Sprintf("key%d", i), entry)
	}
}

// BenchmarkCacheGet benchmarks cache reads.
func BenchmarkCacheGet(b *testing.B) {
	config := DefaultConfig()
	config.Enabled = true
	cache := New(config)

	entry := &Entry{
		StatusCode: 200,
		Body:       []byte("test data"),
	}
	cache.Set("key", entry)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get("key")
	}
}

// BenchmarkCacheConcurrent benchmarks concurrent access.
func BenchmarkCacheConcurrent(b *testing.B) {
	config := DefaultConfig()
	config.Enabled = true
	cache := New(config)

	entry := &Entry{
		StatusCode: 200,
		Body:       []byte("test data"),
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key%d", i)
			cache.Set(key, entry)
			cache.Get(key)
			i++
		}
	})
}
