package openapi

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Provider manages OpenAPI specs for multiple upstreams.
type Provider struct {
	specs   map[string]*UpstreamSpec
	mutex   sync.RWMutex
	logger  *slog.Logger
	options ProviderOptions
}

// UpstreamSpec holds an OpenAPI spec for an upstream.
type UpstreamSpec struct {
	Name            string
	Source          string
	Spec            *OpenAPISpec
	LastLoaded      time.Time
	RefreshInterval time.Duration
	stopChan        chan struct{}
}

// ProviderOptions configures the provider.
type ProviderOptions struct {
	// DefaultRefreshInterval is the default interval for refreshing specs
	DefaultRefreshInterval time.Duration
	// AutoRefresh enables automatic spec refresh
	AutoRefresh bool
}

// DefaultProviderOptions returns default options.
func DefaultProviderOptions() ProviderOptions {
	return ProviderOptions{
		DefaultRefreshInterval: 5 * time.Minute,
		AutoRefresh:            true,
	}
}

// NewProvider creates a new OpenAPI provider.
func NewProvider(logger *slog.Logger, options ProviderOptions) *Provider {
	if logger == nil {
		logger = slog.Default()
	}
	return &Provider{
		specs:   make(map[string]*UpstreamSpec),
		logger:  logger,
		options: options,
	}
}

// RegisterUpstream registers an upstream with its OpenAPI spec source.
func (p *Provider) RegisterUpstream(name string, source string, refreshInterval time.Duration) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if _, exists := p.specs[name]; exists {
		return fmt.Errorf("upstream %s already registered", name)
	}

	if refreshInterval == 0 {
		refreshInterval = p.options.DefaultRefreshInterval
	}

	spec := &UpstreamSpec{
		Name:            name,
		Source:          source,
		RefreshInterval: refreshInterval,
		stopChan:        make(chan struct{}),
	}

	p.specs[name] = spec

	// Load the spec immediately
	if err := p.loadSpec(spec); err != nil {
		p.logger.Warn("Failed to load OpenAPI spec",
			slog.String("upstream", name),
			slog.String("source", source),
			slog.Any("error", err),
		)
		// Don't return error - spec can be loaded later
	}

	// Start background refresh if enabled
	if p.options.AutoRefresh && refreshInterval > 0 {
		go p.refreshLoop(spec)
	}

	p.logger.Info("Registered upstream OpenAPI spec",
		slog.String("upstream", name),
		slog.String("source", source),
	)

	return nil
}

// UnregisterUpstream removes an upstream from the provider.
func (p *Provider) UnregisterUpstream(name string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if spec, exists := p.specs[name]; exists {
		close(spec.stopChan)
		delete(p.specs, name)
		p.logger.Info("Unregistered upstream OpenAPI spec", slog.String("upstream", name))
	}
}

// GetSpec returns the OpenAPI spec for an upstream.
func (p *Provider) GetSpec(name string) (*OpenAPISpec, bool) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	spec, exists := p.specs[name]
	if !exists || spec.Spec == nil {
		return nil, false
	}

	return spec.Spec, true
}

// GetAllSpecs returns all loaded specs.
func (p *Provider) GetAllSpecs() map[string]*OpenAPISpec {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	result := make(map[string]*OpenAPISpec)
	for name, spec := range p.specs {
		if spec.Spec != nil {
			result[name] = spec.Spec
		}
	}

	return result
}

// RefreshSpec manually refreshes a spec for an upstream.
func (p *Provider) RefreshSpec(name string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	spec, exists := p.specs[name]
	if !exists {
		return fmt.Errorf("upstream %s not found", name)
	}

	return p.loadSpec(spec)
}

// loadSpec loads a spec from source.
func (p *Provider) loadSpec(spec *UpstreamSpec) error {
	if spec.Source == "" {
		return fmt.Errorf("no source configured for upstream %s", spec.Name)
	}

	start := time.Now()
	openapiSpec, err := LoadOpenAPISpec(spec.Source)
	if err != nil {
		return fmt.Errorf("loading spec from %s: %w", spec.Source, err)
	}

	spec.Spec = openapiSpec
	spec.LastLoaded = time.Now()

	p.logger.Debug("Loaded OpenAPI spec",
		slog.String("upstream", spec.Name),
		slog.String("source", spec.Source),
		slog.String("version", openapiSpec.OpenAPI),
		slog.String("title", openapiSpec.Info.Title),
		slog.Duration("duration", time.Since(start)),
	)

	return nil
}

// refreshLoop runs the background refresh loop for a spec.
func (p *Provider) refreshLoop(spec *UpstreamSpec) {
	ticker := time.NewTicker(spec.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := p.RefreshSpec(spec.Name); err != nil {
				p.logger.Warn("Failed to refresh OpenAPI spec",
					slog.String("upstream", spec.Name),
					slog.Any("error", err),
				)
			} else {
				p.logger.Debug("Refreshed OpenAPI spec",
					slog.String("upstream", spec.Name),
				)
			}
		case <-spec.stopChan:
			return
		}
	}
}

// Shutdown stops the provider and all background refresh loops.
func (p *Provider) Shutdown(ctx context.Context) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for name, spec := range p.specs {
		close(spec.stopChan)
		delete(p.specs, name)
	}

	return nil
}

// HasSpec checks if a spec is loaded for an upstream.
func (p *Provider) HasSpec(name string) bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	spec, exists := p.specs[name]
	return exists && spec.Spec != nil
}

// GetUpstreamNames returns all registered upstream names.
func (p *Provider) GetUpstreamNames() []string {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	names := make([]string, 0, len(p.specs))
	for name := range p.specs {
		names = append(names, name)
	}

	return names
}
