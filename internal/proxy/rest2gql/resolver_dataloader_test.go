package rest2gql

import (
	"testing"

	"github.com/ersinkoc/tentaserve/internal/openapi"
	"github.com/ersinkoc/tentaserve/internal/upstream"
)

func TestNewDataLoaderResolver(t *testing.T) {
	client, err := upstream.NewClient(upstream.ClientOptions{BaseURL: "http://example.com"})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	baseResolver := NewResolver(ResolverOptions{
		BaseURL:   "http://example.com",
		Client:    client,
		Path:      "/users",
		Method:    "GET",
		Operation: &openapi.Operation{},
	})

	t.Run("with default key extractor", func(t *testing.T) {
		dlr := NewDataLoaderResolver(baseResolver, DataLoaderResolverOptions{
			UpstreamName: "users-api",
			FieldName:    "getUser",
		})

		if dlr == nil {
			t.Fatal("expected non-nil resolver")
		}
		if dlr.upstreamName != "users-api" {
			t.Errorf("expected upstreamName 'users-api', got %s", dlr.upstreamName)
		}
		if dlr.fieldName != "getUser" {
			t.Errorf("expected fieldName 'getUser', got %s", dlr.fieldName)
		}

		// Test default key extractor
		key, ok := dlr.keyExtractor(map[string]interface{}{"id": "123"})
		if !ok {
			t.Error("expected key extraction to succeed")
		}
		if key != "123" {
			t.Errorf("expected key '123', got %s", key)
		}

		// No id key
		_, ok = dlr.keyExtractor(map[string]interface{}{"name": "John"})
		if ok {
			t.Error("expected key extraction to fail without id")
		}
	})

	t.Run("with custom key extractor", func(t *testing.T) {
		customExtractor := func(args map[string]interface{}) (string, bool) {
			if email, ok := args["email"]; ok {
				return email.(string), true
			}
			return "", false
		}

		dlr := NewDataLoaderResolver(baseResolver, DataLoaderResolverOptions{
			UpstreamName: "users-api",
			FieldName:    "getUserByEmail",
			KeyExtractor: customExtractor,
		})

		key, ok := dlr.keyExtractor(map[string]interface{}{"email": "test@example.com"})
		if !ok {
			t.Error("expected custom key extraction to succeed")
		}
		if key != "test@example.com" {
			t.Errorf("expected key 'test@example.com', got %s", key)
		}
	})
}

func TestNewBatchResolver(t *testing.T) {
	client, err := upstream.NewClient(upstream.ClientOptions{BaseURL: "http://example.com"})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	br := NewBatchResolver(BatchResolverOptions{
		Client:      client,
		BaseURL:     "http://example.com",
		BatchPath:   "/api/items",
		IDParamName: "ids",
		ResultKey:   "items",
	})

	if br == nil {
		t.Fatal("expected non-nil batch resolver")
	}
	if br.baseURL != "http://example.com" {
		t.Errorf("expected baseURL 'http://example.com', got %s", br.baseURL)
	}
	if br.batchPath != "/api/items" {
		t.Errorf("expected batchPath '/api/items', got %s", br.batchPath)
	}
	if br.idParamName != "ids" {
		t.Errorf("expected idParamName 'ids', got %s", br.idParamName)
	}
}

func TestBatchResolver_BuildIDParam(t *testing.T) {
	br := &BatchResolver{idParamName: "ids"}

	tests := []struct {
		name     string
		keys     []string
		expected string
	}{
		{"single key", []string{"1"}, "ids=1"},
		{"multiple keys", []string{"1", "2", "3"}, "ids=1,2,3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := br.buildIDParam(tt.keys)
			if result != tt.expected {
				t.Errorf("buildIDParam() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBatchResolver_MakeErrors(t *testing.T) {
	br := &BatchResolver{}

	errs := br.makeErrors(3, errTest)
	if len(errs) != 3 {
		t.Errorf("expected 3 errors, got %d", len(errs))
	}
	for i, err := range errs {
		if err != errTest {
			t.Errorf("error %d: expected errTest, got %v", i, err)
		}
	}
}

var errTest = &testError{}

type testError struct{}

func (e *testError) Error() string { return "test error" }

func TestBatchResolver_ParseBatchResponse(t *testing.T) {
	br := &BatchResolver{resultKey: "items"}

	// parseBatchResponse is a stub - it returns empty slice
	keys := []string{"1", "2", "3"}
	results, err := br.parseBatchResponse(nil, keys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}
