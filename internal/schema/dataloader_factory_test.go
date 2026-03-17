package schema

import (
	"context"
	"testing"
	"time"
)

// TestNewDataLoaderFactory tests factory creation.
func TestNewDataLoaderFactory(t *testing.T) {
	factory := NewDataLoaderFactory(5*time.Millisecond, 100)

	if factory == nil {
		t.Fatal("Expected factory to be created")
	}

	if factory.batchWindow != 5*time.Millisecond {
		t.Errorf("Expected batchWindow 5ms, got %v", factory.batchWindow)
	}

	if factory.maxBatchSize != 100 {
		t.Errorf("Expected maxBatchSize 100, got %d", factory.maxBatchSize)
	}

	if len(factory.providers) != 0 {
		t.Errorf("Expected 0 providers initially, got %d", len(factory.providers))
	}
}

// TestDataLoaderFactory_RegisterProvider tests provider registration.
func TestDataLoaderFactory_RegisterProvider(t *testing.T) {
	factory := NewDataLoaderFactory(5*time.Millisecond, 100)

	provider := BatchFuncProviderFunc(func(fieldName string) BatchFunc {
		return func(ctx context.Context, keys []string) ([]any, []error) {
			return make([]any, len(keys)), nil
		}
	})

	factory.RegisterProvider("test-upstream", provider)

	if len(factory.providers) != 1 {
		t.Errorf("Expected 1 provider, got %d", len(factory.providers))
	}
}

// TestDataLoaderFactory_CreateForRequest tests creating request DataLoaders.
func TestDataLoaderFactory_CreateForRequest(t *testing.T) {
	factory := NewDataLoaderFactory(5*time.Millisecond, 100)

	provider := BatchFuncProviderFunc(func(fieldName string) BatchFunc {
		return func(ctx context.Context, keys []string) ([]any, []error) {
			return make([]any, len(keys)), nil
		}
	})
	factory.RegisterProvider("test-upstream", provider)

	requestLoaders := factory.CreateForRequest()

	if requestLoaders == nil {
		t.Fatal("Expected request loaders to be created")
	}

	if requestLoaders.LoaderCount() != 0 {
		t.Errorf("Expected 0 loaders initially, got %d", requestLoaders.LoaderCount())
	}
}

// TestRequestDataLoaders_GetLoader tests getting/creating a loader.
func TestRequestDataLoaders_GetLoader(t *testing.T) {
	factory := NewDataLoaderFactory(5*time.Millisecond, 100)

	provider := BatchFuncProviderFunc(func(fieldName string) BatchFunc {
		return func(ctx context.Context, keys []string) ([]any, []error) {
			values := make([]any, len(keys))
			for i, k := range keys {
				values[i] = "value_" + k
			}
			return values, nil
		}
	})
	factory.RegisterProvider("test-upstream", provider)

	requestLoaders := factory.CreateForRequest()

	// Get loader for field
	loader, err := requestLoaders.GetLoader("test-upstream", "users")
	if err != nil {
		t.Fatalf("GetLoader failed: %v", err)
	}

	if loader == nil {
		t.Fatal("Expected loader to be created")
	}

	// Verify it's cached (same instance)
	loader2, err := requestLoaders.GetLoader("test-upstream", "users")
	if err != nil {
		t.Fatalf("GetLoader second call failed: %v", err)
	}

	if loader != loader2 {
		t.Error("Expected same loader instance to be returned")
	}

	if requestLoaders.LoaderCount() != 1 {
		t.Errorf("Expected 1 loader, got %d", requestLoaders.LoaderCount())
	}
}

// TestRequestDataLoaders_GetLoader_NoProvider tests error when no provider.
func TestRequestDataLoaders_GetLoader_NoProvider(t *testing.T) {
	factory := NewDataLoaderFactory(5*time.Millisecond, 100)
	requestLoaders := factory.CreateForRequest()

	_, err := requestLoaders.GetLoader("unknown-upstream", "field")
	if err == nil {
		t.Error("Expected error for unknown upstream")
	}

	if err.Error() != `no batch provider registered for upstream "unknown-upstream"` {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestRequestDataLoaders_Load tests loading a single key.
func TestRequestDataLoaders_Load(t *testing.T) {
	factory := NewDataLoaderFactory(50*time.Millisecond, 100)

	provider := BatchFuncProviderFunc(func(fieldName string) BatchFunc {
		return func(ctx context.Context, keys []string) ([]any, []error) {
			values := make([]any, len(keys))
			for i, k := range keys {
				values[i] = "value_" + k
			}
			return values, nil
		}
	})
	factory.RegisterProvider("test-upstream", provider)

	requestLoaders := factory.CreateForRequest()

	val, err := requestLoaders.Load(context.Background(), "test-upstream", "users", "123")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if val != "value_123" {
		t.Errorf("Expected value_123, got %v", val)
	}
}

// TestRequestDataLoaders_LoadMany tests loading multiple keys.
func TestRequestDataLoaders_LoadMany(t *testing.T) {
	factory := NewDataLoaderFactory(50*time.Millisecond, 100)

	batchCalled := false
	provider := BatchFuncProviderFunc(func(fieldName string) BatchFunc {
		return func(ctx context.Context, keys []string) ([]any, []error) {
			batchCalled = true
			values := make([]any, len(keys))
			for i, k := range keys {
				values[i] = "value_" + k
			}
			return values, nil
		}
	})
	factory.RegisterProvider("test-upstream", provider)

	requestLoaders := factory.CreateForRequest()

	keys := []string{"1", "2", "3"}
	values, errs := requestLoaders.LoadMany(context.Background(), "test-upstream", "users", keys)

	if !batchCalled {
		t.Error("Expected batch to be called")
	}

	if len(values) != 3 {
		t.Errorf("Expected 3 values, got %d", len(values))
	}

	for i := range errs {
		if errs[i] != nil {
			t.Errorf("Unexpected error at index %d: %v", i, errs[i])
		}
	}

	for i, key := range keys {
		expectedVal := "value_" + key
		if values[i] != expectedVal {
			t.Errorf("Expected %s at index %d, got %v", expectedVal, i, values[i])
		}
	}
}

// TestRequestDataLoaders_LoadMany_NoProvider tests error handling.
func TestRequestDataLoaders_LoadMany_NoProvider(t *testing.T) {
	factory := NewDataLoaderFactory(50*time.Millisecond, 100)
	requestLoaders := factory.CreateForRequest()

	keys := []string{"1", "2", "3"}
	values, errs := requestLoaders.LoadMany(context.Background(), "unknown", "field", keys)

	if values != nil {
		t.Error("Expected nil values when provider not found")
	}

	if len(errs) != 3 {
		t.Fatalf("Expected 3 errors, got %d", len(errs))
	}

	for i, err := range errs {
		if err == nil {
			t.Errorf("Expected error at index %d", i)
		}
	}
}

// TestRequestDataLoaders_Stop tests stopping all loaders.
func TestRequestDataLoaders_Stop(t *testing.T) {
	factory := NewDataLoaderFactory(50*time.Millisecond, 100)

	provider := BatchFuncProviderFunc(func(fieldName string) BatchFunc {
		return func(ctx context.Context, keys []string) ([]any, []error) {
			return make([]any, len(keys)), nil
		}
	})
	factory.RegisterProvider("test-upstream", provider)

	requestLoaders := factory.CreateForRequest()

	// Create some loaders
	requestLoaders.GetLoader("test-upstream", "users")
	requestLoaders.GetLoader("test-upstream", "posts")

	if requestLoaders.LoaderCount() != 2 {
		t.Fatalf("Expected 2 loaders, got %d", requestLoaders.LoaderCount())
	}

	// Stop all
	requestLoaders.Stop()

	if requestLoaders.LoaderCount() != 0 {
		t.Errorf("Expected 0 loaders after stop, got %d", requestLoaders.LoaderCount())
	}
}

// TestWithDataLoaders tests context utilities.
func TestWithDataLoaders(t *testing.T) {
	factory := NewDataLoaderFactory(50*time.Millisecond, 100)
	requestLoaders := factory.CreateForRequest()

	ctx := WithDataLoaders(context.Background(), requestLoaders)

	// Get back from context
	retrieved := GetDataLoaders(ctx)
	if retrieved != requestLoaders {
		t.Error("Expected to retrieve same RequestDataLoaders from context")
	}
}

// TestGetDataLoaders_NotFound tests retrieval when not in context.
func TestGetDataLoaders_NotFound(t *testing.T) {
	ctx := context.Background()

	retrieved := GetDataLoaders(ctx)
	if retrieved != nil {
		t.Error("Expected nil when DataLoaders not in context")
	}
}

// TestMustGetDataLoaders_Success tests successful MustGetDataLoaders.
func TestMustGetDataLoaders_Success(t *testing.T) {
	factory := NewDataLoaderFactory(50*time.Millisecond, 100)
	requestLoaders := factory.CreateForRequest()

	ctx := WithDataLoaders(context.Background(), requestLoaders)

	retrieved, err := MustGetDataLoaders(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if retrieved != requestLoaders {
		t.Error("Expected to retrieve same RequestDataLoaders")
	}
}

// TestMustGetDataLoaders_NotFound tests MustGetDataLoaders error.
func TestMustGetDataLoaders_NotFound(t *testing.T) {
	ctx := context.Background()

	_, err := MustGetDataLoaders(ctx)
	if err == nil {
		t.Error("Expected error when DataLoaders not in context")
	}
	if err.Error() != "no DataLoaders in context" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestDataLoader_PerRequestIsolation tests per-request isolation.
func TestDataLoader_PerRequestIsolation(t *testing.T) {
	factory := NewDataLoaderFactory(100*time.Millisecond, 100)

	var batchCallCount int
	provider := BatchFuncProviderFunc(func(fieldName string) BatchFunc {
		return func(ctx context.Context, keys []string) ([]any, []error) {
			batchCallCount++
			values := make([]any, len(keys))
			for i := range keys {
				values[i] = i
			}
			return values, nil
		}
	})
	factory.RegisterProvider("test-upstream", provider)

	// Create two request loaders (simulating two HTTP requests)
	request1 := factory.CreateForRequest()
	request2 := factory.CreateForRequest()

	// Load from both requests
	request1.Load(context.Background(), "test-upstream", "users", "key1")
	request2.Load(context.Background(), "test-upstream", "users", "key2")

	time.Sleep(150 * time.Millisecond) // Wait for batches

	// Should be 2 separate batches (one per request)
	if batchCallCount != 2 {
		t.Errorf("Expected 2 separate batches (one per request), got %d", batchCallCount)
	}
}

// TestDataLoader_MultipleFields tests multiple fields per upstream.
func TestDataLoader_MultipleFields(t *testing.T) {
	factory := NewDataLoaderFactory(50*time.Millisecond, 100)

	provider := BatchFuncProviderFunc(func(fieldName string) BatchFunc {
		return func(ctx context.Context, keys []string) ([]any, []error) {
			values := make([]any, len(keys))
			for i := range keys {
				values[i] = fieldName + "_" + keys[i]
			}
			return values, nil
		}
	})
	factory.RegisterProvider("test-upstream", provider)

	requestLoaders := factory.CreateForRequest()

	// Load from different fields
	val1, _ := requestLoaders.Load(context.Background(), "test-upstream", "users", "1")
	val2, _ := requestLoaders.Load(context.Background(), "test-upstream", "posts", "1")

	time.Sleep(100 * time.Millisecond) // Wait for batches

	if val1 != "users_1" {
		t.Errorf("Expected users_1, got %v", val1)
	}
	if val2 != "posts_1" {
		t.Errorf("Expected posts_1, got %v", val2)
	}

	// Should have 2 separate loaders
	if requestLoaders.LoaderCount() != 2 {
		t.Errorf("Expected 2 loaders (one per field), got %d", requestLoaders.LoaderCount())
	}
}
