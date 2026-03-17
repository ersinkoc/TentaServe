package cache

import (
	"container/list"
	"hash/fnv"
	"sync"
	"time"
)

// Config defines cache configuration.
type Config struct {
	// Enabled controls whether caching is active
	Enabled bool

	// MaxSize is the maximum total size in bytes
	MaxSize int

	// MaxEntries is the maximum number of entries
	MaxEntries int

	// TTL is the default cache duration
	TTL time.Duration

	// StaleDuration is how long to serve stale content while revalidating
	StaleDuration time.Duration

	// VaryHeaders are request headers that affect cache key
	VaryHeaders []string

	// Methods is the list of HTTP methods to cache (default: GET, HEAD)
	Methods []string

	// StatusCodes is the list of status codes to cache (default: 200, 301, 404)
	StatusCodes []int
}

// DefaultConfig returns default cache configuration.
func DefaultConfig() *Config {
	return &Config{
		Enabled:       false,
		MaxSize:       100 * 1024 * 1024, // 100MB
		MaxEntries:    10000,
		TTL:           5 * time.Minute,
		StaleDuration: 1 * time.Minute,
		VaryHeaders:   []string{"Accept-Encoding", "Accept-Language"},
		Methods:       []string{"GET", "HEAD"},
		StatusCodes:   []int{200, 301, 302, 404},
	}
}

// IsMethodCacheable returns true if the method should be cached.
func (c *Config) IsMethodCacheable(method string) bool {
	for _, m := range c.Methods {
		if m == method {
			return true
		}
	}
	return false
}

// IsStatusCodeCacheable returns true if the status code should be cached.
func (c *Config) IsStatusCodeCacheable(code int) bool {
	for _, sc := range c.StatusCodes {
		if sc == code {
			return true
		}
	}
	return false
}

// lruEntry is an entry in the LRU list.
type lruEntry struct {
	key   string
	value *Entry
	size  int
}

// shard is a single shard of the sharded cache.
type shard struct {
	mu       sync.RWMutex
	entries  map[string]*list.Element
	lru      *list.List
	size     int
	maxSize  int
	maxCount int
}

// newShard creates a new cache shard.
func newShard(maxSize, maxCount int) *shard {
	return &shard{
		entries:  make(map[string]*list.Element),
		lru:      list.New(),
		maxSize:  maxSize,
		maxCount: maxCount,
	}
}

// Get retrieves an entry from the shard.
func (s *shard) Get(key string) *Entry {
	s.mu.RLock()
	elem, ok := s.entries[key]
	s.mu.RUnlock()

	if !ok {
		return nil
	}

	s.mu.Lock()
	// Move to front (most recently used)
	s.lru.MoveToFront(elem)
	entry := elem.Value.(*lruEntry).value
	s.mu.Unlock()

	return entry.Clone()
}

// Set adds or updates an entry in the shard.
// Returns true if entry was added, false if it was too large.
func (s *shard) Set(key string, value *Entry) bool {
	size := value.Size()

	// Entry too large for this shard
	if size > s.maxSize/2 {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// If key exists, update it
	if elem, ok := s.entries[key]; ok {
		oldEntry := elem.Value.(*lruEntry)
		s.size -= oldEntry.size
		s.size += size
		oldEntry.value = value
		oldEntry.size = size
		s.lru.MoveToFront(elem)
	} else {
		// Add new entry
		entry := &lruEntry{
			key:   key,
			value: value,
			size:  size,
		}
		elem := s.lru.PushFront(entry)
		s.entries[key] = elem
		s.size += size
	}

	// Evict if necessary
	s.evict()

	return true
}

// Delete removes an entry from the shard.
func (s *shard) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if elem, ok := s.entries[key]; ok {
		s.removeElement(elem)
	}
}

// removeElement removes an element from the cache.
func (s *shard) removeElement(elem *list.Element) {
	entry := elem.Value.(*lruEntry)
	s.lru.Remove(elem)
	delete(s.entries, entry.key)
	s.size -= entry.size
}

// evict removes entries until we're under limits.
func (s *shard) evict() {
	for s.lru.Len() > 0 {
		// Check if we're under limits
		if s.lru.Len() <= s.maxCount && s.size <= s.maxSize {
			break
		}

		// Evict oldest
		elem := s.lru.Back()
		if elem == nil {
			break
		}
		s.removeElement(elem)
	}
}

// Len returns the number of entries in the shard.
func (s *shard) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lru.Len()
}

// Size returns the total size of the shard in bytes.
func (s *shard) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.size
}

// Clear removes all entries from the shard.
func (s *shard) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = make(map[string]*list.Element)
	s.lru = list.New()
	s.size = 0
}

// Cache is a sharded LRU cache with TTL support.
type Cache struct {
	config *Config
	shards []*shard
	numShards int
}

// New creates a new sharded cache.
func New(config *Config) *Cache {
	if config == nil {
		config = DefaultConfig()
	}

	numShards := 256
	shardSize := config.MaxSize / numShards
	if shardSize < 1024*1024 {
		shardSize = 1024 * 1024 // Minimum 1MB per shard
	}
	shardCount := config.MaxEntries / numShards
	if shardCount < 10 {
		shardCount = 10 // Minimum 10 entries per shard
	}

	shards := make([]*shard, numShards)
	for i := 0; i < numShards; i++ {
		shards[i] = newShard(shardSize, shardCount)
	}

	return &Cache{
		config:    config,
		shards:    shards,
		numShards: numShards,
	}
}

// getShard returns the shard for a given key.
func (c *Cache) getShard(key string) *shard {
	h := fnv.New32a()
	h.Write([]byte(key))
	return c.shards[h.Sum32()%uint32(c.numShards)]
}

// Get retrieves a cached entry.
func (c *Cache) Get(key string) *Entry {
	if !c.config.Enabled {
		return nil
	}

	shard := c.getShard(key)
	return shard.Get(key)
}

// Set stores an entry in the cache.
func (c *Cache) Set(key string, entry *Entry) bool {
	if !c.config.Enabled {
		return false
	}

	shard := c.getShard(key)
	return shard.Set(key, entry)
}

// Delete removes an entry from the cache.
func (c *Cache) Delete(key string) {
	if !c.config.Enabled {
		return
	}

	shard := c.getShard(key)
	shard.Delete(key)
}

// Len returns the total number of entries in the cache.
func (c *Cache) Len() int {
	count := 0
	for _, s := range c.shards {
		count += s.Len()
	}
	return count
}

// Size returns the total size of the cache in bytes.
func (c *Cache) Size() int {
	size := 0
	for _, s := range c.shards {
		size += s.Size()
	}
	return size
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() {
	for _, s := range c.shards {
		s.Clear()
	}
}

// Stats returns cache statistics.
func (c *Cache) Stats() Stats {
	return Stats{
		Entries: c.Len(),
		Size:    c.Size(),
		Shards:  c.numShards,
	}
}

// Stats contains cache statistics.
type Stats struct {
	Entries int
	Size    int
	Shards  int
}
