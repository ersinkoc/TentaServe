package reload

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"
)

// Loader loads configuration from a source.
type Loader func() ([]byte, error)

// Validator validates configuration.
type Validator func(data []byte) error

// Applier applies configuration changes.
type Applier func(data []byte) error

// Watcher watches for configuration changes.
type Watcher struct {
	loader    Loader
	validator Validator
	applier   Applier
	interval  time.Duration
	logger    *slog.Logger

	mu           sync.RWMutex
	currentHash  string
	currentData  []byte
	stop         chan struct{}
	running      bool
	lastModified time.Time
}

// Config defines watcher configuration.
type Config struct {
	// Loader loads raw configuration data
	Loader Loader

	// Validator validates configuration (optional)
	Validator Validator

	// Applier applies the configuration
	Applier Applier

	// Interval for checking changes
	Interval time.Duration

	// Logger for events
	Logger *slog.Logger
}

// NewWatcher creates a new configuration watcher.
func NewWatcher(cfg *Config) *Watcher {
	if cfg.Interval <= 0 {
		cfg.Interval = 5 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &Watcher{
		loader:    cfg.Loader,
		validator: cfg.Validator,
		applier:   cfg.Applier,
		interval:  cfg.Interval,
		logger:    cfg.Logger,
		stop:      make(chan struct{}),
	}
}

// Start begins watching for configuration changes.
func (w *Watcher) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		return nil
	}

	// Load initial config
	data, err := w.load()
	if err != nil {
		return err
	}

	// Validate if validator is set
	if w.validator != nil {
		if err := w.validator(data); err != nil {
			w.logger.Error("config validation failed", "error", err)
			return err
		}
	}

	w.currentData = data
	w.currentHash = hash(data)

	// Apply initial config
	if w.applier != nil {
		if err := w.applier(data); err != nil {
			w.logger.Error("failed to apply initial config", "error", err)
			return err
		}
	}

	w.running = true
	go w.watch()

	w.logger.Info("config watcher started", "interval", w.interval)
	return nil
}

// Stop stops watching for configuration changes.
func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	close(w.stop)
	w.running = false
	w.logger.Info("config watcher stopped")
}

// Reload forces a configuration reload.
func (w *Watcher) Reload() error {
	data, err := w.load()
	if err != nil {
		return err
	}

	return w.applyIfChanged(data)
}

// Current returns the current configuration data.
func (w *Watcher) Current() []byte {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.currentData
}

// Hash returns the current configuration hash.
func (w *Watcher) Hash() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.currentHash
}

func (w *Watcher) watch() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := w.check(); err != nil {
				w.logger.Debug("config check failed", "error", err)
			}
		case <-w.stop:
			return
		}
	}
}

func (w *Watcher) check() error {
	data, err := w.load()
	if err != nil {
		return err
	}

	return w.applyIfChanged(data)
}

func (w *Watcher) load() ([]byte, error) {
	if w.loader == nil {
		return nil, nil
	}
	return w.loader()
}

func (w *Watcher) applyIfChanged(data []byte) error {
	newHash := hash(data)

	w.mu.RLock()
	currentHash := w.currentHash
	w.mu.RUnlock()

	if newHash == currentHash {
		return nil // No change
	}

	// Validate if validator is set
	if w.validator != nil {
		if err := w.validator(data); err != nil {
			w.logger.Error("config validation failed", "error", err)
			return err
		}
	}

	// Apply if applier is set
	if w.applier != nil {
		if err := w.applier(data); err != nil {
			w.logger.Error("config apply failed", "error", err)
			return err
		}
	}

	// Update state
	w.mu.Lock()
	w.currentData = data
	w.currentHash = newHash
	w.lastModified = time.Now()
	w.mu.Unlock()

	w.logger.Info("config reloaded",
		"old_hash", currentHash[:8],
		"new_hash", newHash[:8],
	)

	return nil
}

// hash computes MD5 hash of data.
func hash(data []byte) string {
	h := md5.Sum(data)
	return hex.EncodeToString(h[:])
}

// FileLoader creates a loader for a file.
func FileLoader(path string) Loader {
	return func() ([]byte, error) {
		return os.ReadFile(path)
	}
}

// JSONValidator creates a validator for JSON.
func JSONValidator() Validator {
	return func(data []byte) error {
		var v interface{}
		return json.Unmarshal(data, &v)
	}
}

// Manager manages multiple configuration watchers.
type Manager struct {
	mu       sync.RWMutex
	watchers map[string]*Watcher
	logger   *slog.Logger
}

// NewManager creates a new configuration manager.
func NewManager(logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		watchers: make(map[string]*Watcher),
		logger:   logger,
	}
}

// Add adds a watcher.
func (m *Manager) Add(name string, watcher *Watcher) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.watchers[name] = watcher
}

// Get returns a watcher by name.
func (m *Manager) Get(name string) *Watcher {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.watchers[name]
}

// Remove removes a watcher.
func (m *Manager) Remove(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if w, ok := m.watchers[name]; ok {
		w.Stop()
		delete(m.watchers, name)
	}
}

// StartAll starts all watchers.
func (m *Manager) StartAll() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, w := range m.watchers {
		if err := w.Start(); err != nil {
			m.logger.Error("failed to start watcher", "name", name, "error", err)
			return err
		}
	}
	return nil
}

// StopAll stops all watchers.
func (m *Manager) StopAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, w := range m.watchers {
		w.Stop()
	}
}

// ReloadAll reloads all watchers.
func (m *Manager) ReloadAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, w := range m.watchers {
		if err := w.Reload(); err != nil {
			m.logger.Error("failed to reload", "name", name, "error", err)
		}
	}
}

// CopyReader reads all data from reader and returns it.
func CopyReader(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}
