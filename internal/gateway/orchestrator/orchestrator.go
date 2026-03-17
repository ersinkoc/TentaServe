package orchestrator

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ersinkoc/tentaserve/internal/gateway/auth"
	"github.com/ersinkoc/tentaserve/internal/gateway/breaker"
	"github.com/ersinkoc/tentaserve/internal/gateway/cache"
	"github.com/ersinkoc/tentaserve/internal/gateway/middleware"
)

// Upstream defines an upstream service.
type Upstream struct {
	Name          string            `json:"name"`
	Target        string            `json:"target"`
	PathPrefix    string            `json:"path_prefix"`
	Headers       map[string]string `json:"headers,omitempty"`
	AuthPlugin    string            `json:"auth_plugin,omitempty"`
	AuthConfig    map[string]string `json:"auth_config,omitempty"`
	RateLimit     *RateLimitConfig  `json:"rate_limit,omitempty"`
	Cache         *CacheConfig      `json:"cache,omitempty"`
	Breaker       *BreakerConfig    `json:"breaker,omitempty"`
	HealthCheck   *HealthCheck      `json:"health_check,omitempty"`
}

// RateLimitConfig defines rate limiting configuration.
type RateLimitConfig struct {
	Enabled  bool   `json:"enabled"`
	Rate     int    `json:"rate"`
	Burst    int    `json:"burst"`
	Scope    string `json:"scope"`
	ClientID string `json:"client_id,omitempty"`
}

// CacheConfig defines caching configuration.
type CacheConfig struct {
	Enabled       bool     `json:"enabled"`
	TTL           int      `json:"ttl_seconds"`
	StatusCodes   []int    `json:"status_codes,omitempty"`
	VaryHeaders   []string `json:"vary_headers,omitempty"`
	StaleDuration int      `json:"stale_duration_seconds"`
}

// BreakerConfig defines circuit breaker configuration.
type BreakerConfig struct {
	Enabled          bool `json:"enabled"`
	FailureThreshold uint32 `json:"failure_threshold"`
	SuccessThreshold uint32 `json:"success_threshold"`
	TimeoutSeconds   int    `json:"timeout_seconds"`
}

// HealthCheck defines health check configuration.
type HealthCheck struct {
	Enabled  bool   `json:"enabled"`
	Path     string `json:"path"`
	Interval int    `json:"interval_seconds"`
}

// Config defines gateway configuration.
type Config struct {
	Upstreams []Upstream `json:"upstreams"`
}

// Orchestrator manages the gateway.
type Orchestrator struct {
	mu         sync.RWMutex
	config     *Config
	logger     *slog.Logger
	server     *http.Server
	handlers   map[string]http.Handler

	// Plugin registry
	authPlugins map[string]auth.Plugin

	// Middleware stores
	breakers    map[string]*breaker.Store
	caches      map[string]*cache.Cache

	// Client for proxying
	client     *http.Client
	clientPool sync.Pool
}

// New creates a new orchestrator.
func New(logger *slog.Logger) *Orchestrator {
	if logger == nil {
		logger = slog.Default()
	}

	o := &Orchestrator{
		logger:      logger,
		handlers:    make(map[string]http.Handler),
		authPlugins: make(map[string]auth.Plugin),
		breakers:    make(map[string]*breaker.Store),
		caches:      make(map[string]*cache.Cache),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Register built-in auth plugins
	o.registerAuthPlugins()

	return o
}

// RegisterAuthPlugin registers an auth plugin.
func (o *Orchestrator) RegisterAuthPlugin(name string, plugin auth.Plugin) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.authPlugins[name] = plugin
}

// LoadConfig loads and applies configuration.
func (o *Orchestrator) LoadConfig(config *Config) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if config == nil {
		return errors.New("config is nil")
	}

	o.config = config
	o.buildHandlers()

	o.logger.Info("config loaded", "upstreams", len(config.Upstreams))
	return nil
}

// buildHandlers builds HTTP handlers for all upstreams.
func (o *Orchestrator) buildHandlers() {
	o.handlers = make(map[string]http.Handler)
	o.breakers = make(map[string]*breaker.Store)
	o.caches = make(map[string]*cache.Cache)

	for _, upstream := range o.config.Upstreams {
		handler := o.buildUpstreamHandler(upstream)
		o.handlers[upstream.PathPrefix] = handler

		o.logger.Debug("handler built",
			"upstream", upstream.Name,
			"prefix", upstream.PathPrefix,
		)
	}
}

// buildUpstreamHandler builds a handler chain for an upstream.
func (o *Orchestrator) buildUpstreamHandler(upstream Upstream) http.Handler {
	// Base handler: proxy to upstream
	handler := o.buildProxyHandler(upstream)

	// Wrap with middleware (in reverse order of execution)

	// 1. Circuit breaker (outermost)
	if upstream.Breaker != nil && upstream.Breaker.Enabled {
		store := breaker.NewStore(&breaker.Config{
			Enabled:          true,
			FailureThreshold: upstream.Breaker.FailureThreshold,
			SuccessThreshold: upstream.Breaker.SuccessThreshold,
			Timeout:          time.Duration(upstream.Breaker.TimeoutSeconds) * time.Second,
		})
		o.breakers[upstream.Name] = store
		bm := breaker.NewMiddleware(store, o.logger)
		handler = bm.Wrap(handler)
	}

	// 2. Cache
	if upstream.Cache != nil && upstream.Cache.Enabled {
		config := &cache.Config{
			Enabled:       true,
			TTL:           time.Duration(upstream.Cache.TTL) * time.Second,
			StatusCodes:   upstream.Cache.StatusCodes,
			VaryHeaders:   upstream.Cache.VaryHeaders,
			StaleDuration: time.Duration(upstream.Cache.StaleDuration) * time.Second,
		}
		c := cache.New(config)
		o.caches[upstream.Name] = c
		cm := cache.NewMiddleware(c, o.logger)
		handler = cm.Wrap(handler)
	}

	// 3. Rate limit
	if upstream.RateLimit != nil && upstream.RateLimit.Enabled {
		// Rate limiting would be implemented here
		// For now, skip to avoid import cycle
	}

	// 4. Auth (innermost, closest to handler)
	if upstream.AuthPlugin != "" {
		if plugin, ok := o.authPlugins[upstream.AuthPlugin]; ok {
			am := auth.NewMiddleware(plugin, o.logger)
			handler = am.Wrap(handler)
		}
	}

	return handler
}

// buildProxyHandler builds the proxy handler for an upstream.
func (o *Orchestrator) buildProxyHandler(upstream Upstream) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Build target URL
		targetURL := upstream.Target + strings.TrimPrefix(r.URL.Path, upstream.PathPrefix)
		if r.URL.RawQuery != "" {
			targetURL += "?" + r.URL.RawQuery
		}

		// Create request
		req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body)
		if err != nil {
			o.logger.Error("failed to create request", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Copy headers
		for key, values := range r.Header {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}

		// Add upstream headers
		for key, value := range upstream.Headers {
			req.Header.Set(key, value)
		}

		// Execute request
		resp, err := o.client.Do(req)
		if err != nil {
			o.logger.Error("upstream request failed", "error", err, "upstream", upstream.Name)
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Copy response headers
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}

		// Write status and body
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	})
}

// ServeHTTP implements http.Handler.
func (o *Orchestrator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Find matching upstream
	o.mu.RLock()
	handler, prefix := o.findHandler(r.URL.Path)
	o.mu.RUnlock()

	if handler == nil {
		http.NotFound(w, r)
		return
	}

	// Add upstream context
	ctx := context.WithValue(r.Context(), upstreamNameKey{}, prefix)
	r = r.WithContext(ctx)

	handler.ServeHTTP(w, r)
}

// findHandler finds the best matching handler for a path.
func (o *Orchestrator) findHandler(path string) (http.Handler, string) {
	var bestMatch string
	var bestHandler http.Handler

	for prefix, handler := range o.handlers {
		if strings.HasPrefix(path, prefix) {
			if len(prefix) > len(bestMatch) {
				bestMatch = prefix
				bestHandler = handler
			}
		}
	}

	return bestHandler, bestMatch
}

// Start starts the gateway server.
func (o *Orchestrator) Start(addr string) error {
	requestID := middleware.NewRequestID("")
	o.server = &http.Server{
		Addr:         addr,
		Handler:      requestID.Wrap(o),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	o.logger.Info("gateway started", "addr", addr)
	return o.server.ListenAndServe()
}

// Stop stops the gateway server.
func (o *Orchestrator) Stop(ctx context.Context) error {
	if o.server == nil {
		return nil
	}
	return o.server.Shutdown(ctx)
}

// Stats returns gateway statistics.
func (o *Orchestrator) Stats() map[string]interface{} {
	o.mu.RLock()
	defer o.mu.RUnlock()

	stats := map[string]interface{}{
		"upstreams": len(o.config.Upstreams),
	}

	// Circuit breaker stats
	breakerStats := make(map[string]interface{})
	for name, store := range o.breakers {
		breakerStats[name] = store.Stats()
	}
	if len(breakerStats) > 0 {
		stats["breakers"] = breakerStats
	}

	// Cache stats
	cacheStats := make(map[string]interface{})
	for name, c := range o.caches {
		cacheStats[name] = c.Stats()
	}
	if len(cacheStats) > 0 {
		stats["caches"] = cacheStats
	}

	return stats
}

// registerAuthPlugins registers built-in auth plugins.
func (o *Orchestrator) registerAuthPlugins() {
	// Passthrough
	o.authPlugins["passthrough"] = auth.NewPassthrough()

	// JWT
	o.authPlugins["jwt"] = auth.NewJWT(nil)

	// API Key
	o.authPlugins["apikey"] = auth.NewAPIKey(nil)
}

// upstreamNameKey is the context key for upstream name.
type upstreamNameKey struct{}

// UpstreamFromContext returns the upstream name from context.
func UpstreamFromContext(ctx context.Context) string {
	if name, ok := ctx.Value(upstreamNameKey{}).(string); ok {
		return name
	}
	return ""
}

// ErrUpstreamNotFound is returned when an upstream is not found.
var ErrUpstreamNotFound = errors.New("upstream not found")

// GetUpstream returns an upstream by name.
func (o *Orchestrator) GetUpstream(name string) (*Upstream, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	for _, u := range o.config.Upstreams {
		if u.Name == name {
			return &u, nil
		}
	}
	return nil, ErrUpstreamNotFound
}
