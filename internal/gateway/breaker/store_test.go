package breaker

import (
	"testing"
)

// TestStoreNew tests store creation.
func TestStoreNew(t *testing.T) {
	store := NewStore(nil)
	if store == nil {
		t.Fatal("Expected non-nil store")
	}
	if store.breakers == nil {
		t.Error("Expected breakers map to be initialized")
	}
}

// TestStoreGet tests getting/creating breakers.
func TestStoreGet(t *testing.T) {
	store := NewStore(DefaultConfig())

	// Get a breaker
	b1 := store.Get("upstream1")
	if b1 == nil {
		t.Fatal("Expected non-nil breaker")
	}

	// Get same breaker again
	b2 := store.Get("upstream1")
	if b1 != b2 {
		t.Error("Expected same breaker instance")
	}

	// Get different breaker
	b3 := store.Get("upstream2")
	if b3 == b1 {
		t.Error("Expected different breaker instance")
	}
}

// TestStoreDelete tests deleting breakers.
func TestStoreDelete(t *testing.T) {
	store := NewStore(DefaultConfig())

	store.Get("upstream1")
	store.Delete("upstream1")

	// Should create new breaker
	b := store.Get("upstream1")
	if b.State() != StateClosed {
		t.Error("Expected new breaker to be in closed state")
	}
}

// TestStoreClear tests clearing all breakers.
func TestStoreClear(t *testing.T) {
	store := NewStore(DefaultConfig())

	store.Get("upstream1")
	store.Get("upstream2")
	store.Get("upstream3")

	store.Clear()

	// All should be new
	for _, name := range []string{"upstream1", "upstream2", "upstream3"} {
		b := store.Get(name)
		if b.State() != StateClosed {
			t.Errorf("Expected %s to be reset", name)
		}
	}
}

// TestStoreStats tests getting statistics.
func TestStoreStats(t *testing.T) {
	store := NewStore(DefaultConfig())

	b1 := store.Get("upstream1")
	b1.Allow()
	b1.Record(ResultFailure)
	b1.Allow()
	b1.Record(ResultFailure)

	stats := store.Stats()
	if len(stats) != 1 {
		t.Errorf("Expected 1 stat entry, got %d", len(stats))
	}

	upstream1Stats := stats["upstream1"]
	if upstream1Stats.Failures != 2 {
		t.Errorf("Expected 2 failures, got %d", upstream1Stats.Failures)
	}
}
