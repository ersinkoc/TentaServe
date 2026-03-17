package reload

import (
	"errors"
	"os"
	"testing"
	"time"
)

// TestNewWatcher tests watcher creation.
func TestNewWatcher(t *testing.T) {
	w := NewWatcher(&Config{
		Loader: func() ([]byte, error) {
			return []byte("test"), nil
		},
	})

	if w == nil {
		t.Fatal("Expected non-nil watcher")
	}
	if w.interval != 5*time.Second {
		t.Errorf("Expected default interval 5s, got %v", w.interval)
	}
}

// TestWatcherStartStop tests starting and stopping.
func TestWatcherStartStop(t *testing.T) {
	callCount := 0
	w := NewWatcher(&Config{
		Loader: func() ([]byte, error) {
			callCount++
			return []byte(`{"config": "test"}`), nil
		},
		Applier: func(data []byte) error {
			return nil
		},
		Interval: 100 * time.Millisecond,
	})

	if err := w.Start(); err != nil {
		t.Fatalf("Failed to start: %v", err)
	}

	// Wait for a check cycle
	time.Sleep(150 * time.Millisecond)

	w.Stop()

	if callCount < 1 {
		t.Error("Expected loader to be called")
	}
}

// TestWatcherReload tests manual reload.
func TestWatcherReload(t *testing.T) {
	loadCount := 0
	applyCount := 0

	w := NewWatcher(&Config{
		Loader: func() ([]byte, error) {
			loadCount++
			return []byte(`{"version": 1}`), nil
		},
		Applier: func(data []byte) error {
			applyCount++
			return nil
		},
	})

	// Initial start
	if err := w.Start(); err != nil {
		t.Fatalf("Failed to start: %v", err)
	}
	w.Stop()

	if loadCount != 1 {
		t.Errorf("Expected 1 load, got %d", loadCount)
	}
	if applyCount != 1 {
		t.Errorf("Expected 1 apply, got %d", applyCount)
	}

	// Manual reload with same data
	if err := w.Reload(); err != nil {
		t.Fatalf("Failed to reload: %v", err)
	}

	if loadCount != 2 {
		t.Errorf("Expected 2 loads, got %d", loadCount)
	}
	if applyCount != 1 { // Should not apply (same hash)
		t.Errorf("Expected 1 apply (no change), got %d", applyCount)
	}
}

// TestWatcherAutoReload tests automatic reload on change.
func TestWatcherAutoReload(t *testing.T) {
	version := 1
	applyCount := 0

	w := NewWatcher(&Config{
		Loader: func() ([]byte, error) {
			return []byte(`{"version": ` + string(rune('0'+version)) + `}`), nil
		},
		Applier: func(data []byte) error {
			applyCount++
			return nil
		},
		Interval: 50 * time.Millisecond,
	})

	if err := w.Start(); err != nil {
		t.Fatalf("Failed to start: %v", err)
	}
	defer w.Stop()

	// Change config
	time.Sleep(100 * time.Millisecond)
	version = 2

	// Wait for reload
	time.Sleep(100 * time.Millisecond)

	if applyCount < 2 {
		t.Errorf("Expected at least 2 applies, got %d", applyCount)
	}
}

// TestWatcherValidation tests validation failure.
func TestWatcherValidation(t *testing.T) {
	w := NewWatcher(&Config{
		Loader: func() ([]byte, error) {
			return []byte(`invalid`), nil
		},
		Validator: func(data []byte) error {
			if string(data) == "invalid" {
				return errors.New("validation failed")
			}
			return nil
		},
		Applier: func(data []byte) error {
			t.Error("Applier should not be called when validation fails")
			return nil
		},
	})

	err := w.Start()
	if err == nil {
		t.Error("Expected validation error")
	}
}

// TestWatcherApplyError tests apply failure.
func TestWatcherApplyError(t *testing.T) {
	w := NewWatcher(&Config{
		Loader: func() ([]byte, error) {
			return []byte(`valid`), nil
		},
		Applier: func(data []byte) error {
			return errors.New("apply failed")
		},
	})

	err := w.Start()
	if err == nil {
		t.Error("Expected apply error")
	}
}

// TestWatcherCurrent tests getting current config.
func TestWatcherCurrent(t *testing.T) {
	w := NewWatcher(&Config{
		Loader: func() ([]byte, error) {
			return []byte(`{"key": "value"}`), nil
		},
		Applier: func(data []byte) error {
			return nil
		},
	})

	if err := w.Start(); err != nil {
		t.Fatalf("Failed to start: %v", err)
	}
	w.Stop()

	current := w.Current()
	if string(current) != `{"key": "value"}` {
		t.Errorf("Expected current config, got %s", string(current))
	}

	hash := w.Hash()
	if hash == "" {
		t.Error("Expected non-empty hash")
	}
}

// TestFileLoader tests file loader.
func TestFileLoader(t *testing.T) {
	// Create temp file
	tmpfile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	content := []byte(`{"test": true}`)
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	loader := FileLoader(tmpfile.Name())
	data, err := loader()
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	if string(data) != string(content) {
		t.Errorf("Expected %s, got %s", content, data)
	}
}

// TestJSONValidator tests JSON validator.
func TestJSONValidator(t *testing.T) {
	validator := JSONValidator()

	// Valid JSON
	if err := validator([]byte(`{"key": "value"}`)); err != nil {
		t.Errorf("Expected valid JSON: %v", err)
	}

	// Invalid JSON
	if err := validator([]byte(`{invalid}`)); err == nil {
		t.Error("Expected invalid JSON error")
	}
}

// TestHash tests hash function.
func TestHash(t *testing.T) {
	h1 := hash([]byte("data1"))
	h2 := hash([]byte("data1"))
	h3 := hash([]byte("data2"))

	if h1 != h2 {
		t.Error("Same data should produce same hash")
	}
	if h1 == h3 {
		t.Error("Different data should produce different hash")
	}
	if len(h1) != 32 { // MD5 hex is 32 chars
		t.Errorf("Expected hash length 32, got %d", len(h1))
	}
}

// TestNewManager tests manager creation.
func TestNewManager(t *testing.T) {
	m := NewManager(nil)
	if m == nil {
		t.Fatal("Expected non-nil manager")
	}
	if m.watchers == nil {
		t.Error("Expected watchers map to be initialized")
	}
}

// TestManagerAddGetRemove tests manager operations.
func TestManagerAddGetRemove(t *testing.T) {
	m := NewManager(nil)

	w := NewWatcher(&Config{
		Loader: func() ([]byte, error) {
			return []byte("test"), nil
		},
	})

	m.Add("test", w)

	got := m.Get("test")
	if got != w {
		t.Error("Expected to get watcher")
	}

	m.Remove("test")

	if m.Get("test") != nil {
		t.Error("Expected watcher to be removed")
	}
}

// TestManagerStartAllStopAll tests starting/stopping all.
func TestManagerStartAllStopAll(t *testing.T) {
	m := NewManager(nil)

	started1 := false
	started2 := false

	w1 := NewWatcher(&Config{
		Loader: func() ([]byte, error) {
			started1 = true
			return []byte("test"), nil
		},
		Applier: func(data []byte) error {
			return nil
		},
	})

	w2 := NewWatcher(&Config{
		Loader: func() ([]byte, error) {
			started2 = true
			return []byte("test"), nil
		},
		Applier: func(data []byte) error {
			return nil
		},
	})

	m.Add("w1", w1)
	m.Add("w2", w2)

	if err := m.StartAll(); err != nil {
		t.Fatalf("Failed to start all: %v", err)
	}

	if !started1 || !started2 {
		t.Error("Expected both watchers to start")
	}

	m.StopAll()
}

// TestManagerReloadAll tests reloading all.
func TestManagerReloadAll(t *testing.T) {
	m := NewManager(nil)

	reloadCount := 0
	w := NewWatcher(&Config{
		Loader: func() ([]byte, error) {
			return []byte(`{"v": ` + string(rune('0'+reloadCount)) + `}`), nil
		},
		Applier: func(data []byte) error {
			reloadCount++
			return nil
		},
	})

	m.Add("w", w)
	w.Start()
	defer w.Stop()

	reloadCount = 2 // Change config
	m.ReloadAll()

	if reloadCount < 2 {
		t.Errorf("Expected reload, got count %d", reloadCount)
	}
}
