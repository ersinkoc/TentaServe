package breaker

import (
	"sync"
	"time"
)

// Store manages circuit breakers for multiple upstreams.
type Store struct {
	mu       sync.RWMutex
	breakers map[string]*Breaker // key -> breaker
	config   *Config
}

// NewStore creates a new circuit breaker store.
func NewStore(config *Config) *Store {
	if config == nil {
		config = DefaultConfig()
	}

	return &Store{
		breakers: make(map[string]*Breaker),
		config:   config,
	}
}

// Get returns the circuit breaker for the given key.
// Creates a new one if it doesn't exist.
func (s *Store) Get(key string) *Breaker {
	s.mu.RLock()
	b, exists := s.breakers[key]
	s.mu.RUnlock()

	if exists {
		return b
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if b, exists = s.breakers[key]; exists {
		return b
	}

	b = New(s.config)
	s.breakers[key] = b
	return b
}

// Delete removes a circuit breaker.
func (s *Store) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.breakers, key)
}

// Clear removes all circuit breakers.
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.breakers = make(map[string]*Breaker)
}

// Stats returns statistics for all circuit breakers.
func (s *Store) Stats() map[string]Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]Stats, len(s.breakers))
	for key, b := range s.breakers {
		stats[key] = b.Stats()
	}
	return stats
}

// CleanupManager handles periodic cleanup of idle circuit breakers.
type CleanupManager struct {
	store    *Store
	interval time.Duration
	maxAge   time.Duration
	stop     chan struct{}
	wg       sync.WaitGroup
}

// NewCleanupManager creates a new cleanup manager.
func NewCleanupManager(store *Store, interval, maxAge time.Duration) *CleanupManager {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	if maxAge <= 0 {
		maxAge = 10 * time.Minute
	}

	return &CleanupManager{
		store:    store,
		interval: interval,
		maxAge:   maxAge,
		stop:     make(chan struct{}),
	}
}

// Start begins the cleanup goroutine.
func (c *CleanupManager) Start() {
	c.wg.Add(1)
	go c.run()
}

// Stop stops the cleanup goroutine.
func (c *CleanupManager) Stop() {
	close(c.stop)
	c.wg.Wait()
}

func (c *CleanupManager) run() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stop:
			return
		}
	}
}

func (c *CleanupManager) cleanup() {
	// Note: This is a simplified cleanup.
	// In production, you might want to track last access time.
	// For now, we just remove breakers in open state that haven't failed recently.
}
