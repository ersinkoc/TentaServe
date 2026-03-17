package schema

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DataLoaderFactory creates DataLoaders per request per upstream.
// This ensures no cross-request batching and proper isolation.
type DataLoaderFactory struct {
	// Upstream-specific batch function providers
	providers map[string]BatchFuncProvider

	// Options for creating DataLoaders
	batchWindow  time.Duration
	maxBatchSize int

	mu sync.RWMutex
}

// BatchFuncProvider provides a batch function for a specific upstream and field.
// This allows custom batch logic per upstream type (REST, GraphQL, etc.)
type BatchFuncProvider interface {
	// GetBatchFunc returns a batch function for the given field
	GetBatchFunc(fieldName string) BatchFunc
}

// BatchFuncProviderFunc is a function that implements BatchFuncProvider.
type BatchFuncProviderFunc func(fieldName string) BatchFunc

// GetBatchFunc implements BatchFuncProvider.
func (f BatchFuncProviderFunc) GetBatchFunc(fieldName string) BatchFunc {
	return f(fieldName)
}

// NewDataLoaderFactory creates a new DataLoader factory.
//
// batchWindow:  Maximum time to wait before dispatching a batch
// maxBatchSize: Maximum number of items per batch (0 = unlimited)
func NewDataLoaderFactory(batchWindow time.Duration, maxBatchSize int) *DataLoaderFactory {
	return &DataLoaderFactory{
		providers:    make(map[string]BatchFuncProvider),
		batchWindow:  batchWindow,
		maxBatchSize: maxBatchSize,
	}
}

// RegisterProvider registers a batch function provider for an upstream.
func (f *DataLoaderFactory) RegisterProvider(upstreamName string, provider BatchFuncProvider) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.providers[upstreamName] = provider
}

// RegisterProviderFunc registers a simple function provider for an upstream.
func (f *DataLoaderFactory) RegisterProviderFunc(upstreamName string, fn func(fieldName string) BatchFunc) {
	f.RegisterProvider(upstreamName, BatchFuncProviderFunc(fn))
}

// CreateForRequest creates a DataLoaderCache for a new request.
// Returns a RequestDataLoaders that holds DataLoaders for this request.
func (f *DataLoaderFactory) CreateForRequest() *RequestDataLoaders {
	f.mu.RLock()
	providers := make(map[string]BatchFuncProvider, len(f.providers))
	for k, v := range f.providers {
		providers[k] = v
	}
	batchWindow := f.batchWindow
	maxBatchSize := f.maxBatchSize
	f.mu.RUnlock()

	return &RequestDataLoaders{
		providers:    providers,
		batchWindow:  batchWindow,
		maxBatchSize: maxBatchSize,
		loaders:      make(map[string]*DataLoader),
	}
}

// RequestDataLoaders holds DataLoaders for a single request.
// This ensures per-request isolation - no batching across requests.
type RequestDataLoaders struct {
	providers    map[string]BatchFuncProvider
	batchWindow  time.Duration
	maxBatchSize int
	loaders      map[string]*DataLoader
	mu           sync.RWMutex
}

// GetLoader gets or creates a DataLoader for a specific upstream and field.
func (r *RequestDataLoaders) GetLoader(upstreamName, fieldName string) (*DataLoader, error) {
	key := upstreamName + ":" + fieldName

	r.mu.RLock()
	loader, ok := r.loaders[key]
	r.mu.RUnlock()

	if ok {
		return loader, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if loader, ok := r.loaders[key]; ok {
		return loader, nil
	}

	// Get provider for upstream
	provider, ok := r.providers[upstreamName]
	if !ok {
		return nil, fmt.Errorf("no batch provider registered for upstream %q", upstreamName)
	}

	// Get batch function for field
	batchFunc := provider.GetBatchFunc(fieldName)
	if batchFunc == nil {
		return nil, fmt.Errorf("no batch function for field %q in upstream %q", fieldName, upstreamName)
	}

	// Create new DataLoader
	loader = NewDataLoader(batchFunc, r.batchWindow, r.maxBatchSize)
	r.loaders[key] = loader

	return loader, nil
}

// Load loads a single key using the appropriate DataLoader.
func (r *RequestDataLoaders) Load(ctx context.Context, upstreamName, fieldName, key string) (any, error) {
	loader, err := r.GetLoader(upstreamName, fieldName)
	if err != nil {
		return nil, err
	}
	return loader.Load(ctx, key)
}

// LoadMany loads multiple keys using the appropriate DataLoader.
func (r *RequestDataLoaders) LoadMany(ctx context.Context, upstreamName, fieldName string, keys []string) ([]any, []error) {
	loader, err := r.GetLoader(upstreamName, fieldName)
	if err != nil {
		// Return errors for all keys
		errs := make([]error, len(keys))
		for i := range keys {
			errs[i] = err
		}
		return nil, errs
	}
	return loader.LoadMany(ctx, keys)
}

// Stop stops all DataLoaders for this request.
func (r *RequestDataLoaders) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, loader := range r.loaders {
		loader.Stop()
	}
	r.loaders = make(map[string]*DataLoader)
}

// LoaderCount returns the number of active DataLoaders.
func (r *RequestDataLoaders) LoaderCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.loaders)
}

// ContextKey is the key type for storing RequestDataLoaders in context.
type dataloaderCtxKey struct{}

var (
	// DataLoaderContextKey is used to store RequestDataLoaders in context.
	DataLoaderContextKey = dataloaderCtxKey{}
)

// WithDataLoaders adds RequestDataLoaders to the context.
func WithDataLoaders(ctx context.Context, loaders *RequestDataLoaders) context.Context {
	return context.WithValue(ctx, DataLoaderContextKey, loaders)
}

// GetDataLoaders retrieves RequestDataLoaders from the context.
// Returns nil if not found.
func GetDataLoaders(ctx context.Context) *RequestDataLoaders {
	if v := ctx.Value(DataLoaderContextKey); v != nil {
		if loaders, ok := v.(*RequestDataLoaders); ok {
			return loaders
		}
	}
	return nil
}

// MustGetDataLoaders retrieves RequestDataLoaders from the context.
// Returns an error if not found.
func MustGetDataLoaders(ctx context.Context) (*RequestDataLoaders, error) {
	loaders := GetDataLoaders(ctx)
	if loaders == nil {
		return nil, fmt.Errorf("no DataLoaders in context")
	}
	return loaders, nil
}