package ratelimit

import (
	"context"
	"log/slog"
	"time"
)

// CleanupManager manages periodic cleanup of stale rate limit buckets.
type CleanupManager struct {
	store    *Store
	interval time.Duration
	maxAge   time.Duration
	logger   *slog.Logger
	stop     chan struct{}
	done     chan struct{}
}

// NewCleanupManager creates a new cleanup manager.
// interval: how often to run cleanup
// maxAge: buckets older than this will be removed
func NewCleanupManager(store *Store, interval, maxAge time.Duration, logger *slog.Logger) *CleanupManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &CleanupManager{
		store:    store,
		interval: interval,
		maxAge:   maxAge,
		logger:   logger,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start begins the cleanup goroutine.
func (c *CleanupManager) Start() {
	go c.run()
}

// run is the main cleanup loop.
func (c *CleanupManager) run() {
	defer close(c.done)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			removed := c.store.Cleanup(c.maxAge)
			if removed > 0 {
				c.logger.Debug("cleaned up stale rate limit buckets",
					slog.Int("removed", removed),
					slog.Int("remaining", c.store.Len()),
				)
			}
		case <-c.stop:
			return
		}
	}
}

// Stop stops the cleanup goroutine.
// Blocks until the goroutine has stopped.
func (c *CleanupManager) Stop() {
	close(c.stop)
	<-c.done
}

// StopContext stops the cleanup goroutine with context timeout.
func (c *CleanupManager) StopContext(ctx context.Context) error {
	close(c.stop)

	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// DefaultCleanupManager creates a cleanup manager with sensible defaults.
// Runs cleanup every 5 minutes, removes buckets idle for 10 minutes.
func DefaultCleanupManager(store *Store, logger *slog.Logger) *CleanupManager {
	return NewCleanupManager(store, 5*time.Minute, 10*time.Minute, logger)
}
