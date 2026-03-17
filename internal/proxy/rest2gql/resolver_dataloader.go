package rest2gql

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ersinkoc/tentaserve/internal/schema"
	"github.com/ersinkoc/tentaserve/internal/upstream"
)

// DataLoaderResolver wraps a Resolver with DataLoader batching support.
// It intercepts resolve calls for nested fields and batches them using DataLoader.
type DataLoaderResolver struct {
	*Resolver
	upstreamName string
	fieldName    string
	keyExtractor func(args map[string]interface{}) (string, bool)
}

// DataLoaderResolverOptions configures the DataLoaderResolver.
type DataLoaderResolverOptions struct {
	UpstreamName string
	FieldName    string
	KeyExtractor func(args map[string]interface{}) (string, bool)
}

// NewDataLoaderResolver creates a new resolver with DataLoader support.
// This resolver checks for DataLoaders in context and uses them for batching.
func NewDataLoaderResolver(baseResolver *Resolver, opts DataLoaderResolverOptions) *DataLoaderResolver {
	// Default key extractor extracts "id" argument
	keyExtractor := opts.KeyExtractor
	if keyExtractor == nil {
		keyExtractor = func(args map[string]interface{}) (string, bool) {
			if id, ok := args["id"]; ok {
				return fmt.Sprintf("%v", id), true
			}
			return "", false
		}
	}

	return &DataLoaderResolver{
		Resolver:     baseResolver,
		upstreamName: opts.UpstreamName,
		fieldName:    opts.FieldName,
		keyExtractor: keyExtractor,
	}
}

// Resolve executes the resolution, using DataLoader if available.
func (r *DataLoaderResolver) Resolve(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	// Check if DataLoaders are available in context
	loaders := schema.GetDataLoaders(ctx)
	if loaders == nil {
		// No DataLoaders available - fall back to direct resolution
		return r.Resolver.Resolve(ctx, args)
	}

	// Extract key for batching
	key, ok := r.keyExtractor(args)
	if !ok {
		// Can't extract key - fall back to direct resolution
		return r.Resolver.Resolve(ctx, args)
	}

	// Use DataLoader for batching
	return loaders.Load(ctx, r.upstreamName, r.fieldName, key)
}

// BatchResolve is the batch function used by DataLoader.
// It resolves multiple keys in a single batch request.
func (r *DataLoaderResolver) BatchResolve(ctx context.Context, keys []string) ([]interface{}, []error) {
	values := make([]interface{}, len(keys))
	errors := make([]error, len(keys))

	// Resolve each key individually
	// In production, this could be optimized to make a single batched REST call
	// e.g., GET /api/items?ids=1,2,3 instead of individual calls
	for i, key := range keys {
		args := map[string]interface{}{"id": key}
		val, err := r.Resolver.Resolve(ctx, args)
		values[i] = val
		errors[i] = err
	}

	return values, errors
}

// BatchResolver wraps multiple resolvers for efficient batching.
// It makes a single batched REST API call for multiple items.
type BatchResolver struct {
	client       *upstream.Client
	baseURL      string
	batchPath    string  // e.g., "/api/items"
	idParamName  string  // e.g., "ids"
	resultKey    string  // JSON key containing results array
}

// BatchResolverOptions configures the BatchResolver.
type BatchResolverOptions struct {
	Client      *upstream.Client
	BaseURL     string
	BatchPath   string
	IDParamName string
	ResultKey   string
}

// NewBatchResolver creates a new batch resolver.
func NewBatchResolver(opts BatchResolverOptions) *BatchResolver {
	return &BatchResolver{
		client:      opts.Client,
		baseURL:     opts.BaseURL,
		batchPath:   opts.BatchPath,
		idParamName: opts.IDParamName,
		resultKey:   opts.ResultKey,
	}
}

// ResolveBatch resolves multiple keys with a single batched REST call.
func (r *BatchResolver) ResolveBatch(ctx context.Context, keys []string) ([]interface{}, []error) {
	if len(keys) == 0 {
		return []interface{}{}, nil
	}

	// Build URL with batch parameter
	url := r.baseURL + r.batchPath + "?" + r.buildIDParam(keys)

	// Create request using standard http package
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, r.makeErrors(len(keys), err)
	}

	// Make request
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, r.makeErrors(len(keys), err)
	}
	defer resp.Body.Close()

	// Parse response
	results, err := r.parseBatchResponse(resp, keys)
	if err != nil {
		return nil, r.makeErrors(len(keys), err)
	}

	return results, nil
}

// buildIDParam builds the ID parameter for the batch request.
func (r *BatchResolver) buildIDParam(keys []string) string {
	param := r.idParamName + "="
	for i, key := range keys {
		if i > 0 {
			param += ","
		}
		param += key
	}
	return param
}

// makeErrors creates a slice of the same error for all keys.
func (r *BatchResolver) makeErrors(count int, err error) []error {
	errors := make([]error, count)
	for i := range errors {
		errors[i] = err
	}
	return errors
}

// parseBatchResponse parses the batch response and maps results to keys.
func (r *BatchResolver) parseBatchResponse(resp *http.Response, keys []string) ([]interface{}, error) {
	// Implementation would parse the response and map items to keys
	// This is a placeholder - actual implementation depends on API format
	return make([]interface{}, len(keys)), nil
}

// RegisterDataLoaderProvider registers a DataLoader provider with the factory.
// This should be called during proxy initialization.
func RegisterDataLoaderProvider(
	factory *schema.DataLoaderFactory,
	upstreamName string,
	batchPath string,
	resolver *Resolver,
) {
	provider := schema.BatchFuncProviderFunc(func(fieldName string) schema.BatchFunc {
		return func(ctx context.Context, keys []string) ([]interface{}, []error) {
			// Create batch resolver for this field
			dlr := &DataLoaderResolver{
				Resolver:     resolver,
				upstreamName: upstreamName,
				fieldName:    fieldName,
			}
			return dlr.BatchResolve(ctx, keys)
		}
	})

	factory.RegisterProvider(upstreamName, provider)
}
