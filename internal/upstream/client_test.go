package upstream

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		opts    ClientOptions
		wantErr bool
	}{
		{
			name: "valid URL",
			opts: ClientOptions{
				BaseURL: "http://localhost:8080",
			},
			wantErr: false,
		},
		{
			name: "valid URL with path",
			opts: ClientOptions{
				BaseURL: "http://api.example.com/v1",
			},
			wantErr: false,
		},
		{
			name: "HTTPS URL",
			opts: ClientOptions{
				BaseURL: "https://secure.example.com",
			},
			wantErr: false,
		},
		{
			name: "invalid URL",
			opts: ClientOptions{
				BaseURL: "://invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && client == nil {
				t.Error("NewClient() returned nil client without error")
			}
			if client != nil {
				_ = client.Close()
			}
		})
	}
}

func TestClient_Get(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/users" {
			t.Errorf("Expected /users, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": 1, "name": "John"}`))
	}))
	defer server.Close()

	client, err := NewClient(ClientOptions{
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	resp, err := client.Get(context.Background(), "/users", nil)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "John") {
		t.Errorf("Expected body to contain 'John', got %s", string(body))
	}
}

func TestClient_Post(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", ct)
		}

		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "name") {
			t.Errorf("Expected body to contain 'name', got %s", string(body))
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": 1}`))
	}))
	defer server.Close()

	client, err := NewClient(ClientOptions{
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	body := strings.NewReader(`{"name": "John"}`)
	resp, err := client.Post(context.Background(), "/users", body, nil)
	if err != nil {
		t.Fatalf("Post failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}
}

func TestClient_Retry(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(ClientOptions{
		BaseURL: server.URL,
		Retry: RetryConfig{
			MaxRetries: 3,
			BaseDelay:  10 * time.Millisecond,
			MaxDelay:   100 * time.Millisecond,
			Multiplier: 2.0,
		},
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	resp, err := client.Get(context.Background(), "/test", nil)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if requestCount != 3 {
		t.Errorf("Expected 3 requests (2 retries), got %d", requestCount)
	}
}

func TestClient_RetryNoRetryOn4xx(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusBadRequest) // 400 - not retryable
	}))
	defer server.Close()

	client, err := NewClient(ClientOptions{
		BaseURL: server.URL,
		Retry: RetryConfig{
			MaxRetries: 3,
			BaseDelay:  10 * time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	resp, err := client.Get(context.Background(), "/test", nil)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}

	if requestCount != 1 {
		t.Errorf("Expected 1 request (no retries on 4xx), got %d", requestCount)
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(ClientOptions{
		BaseURL: server.URL,
		Retry: RetryConfig{
			MaxRetries: 3,
			BaseDelay:  50 * time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = client.Get(ctx, "/test", nil)
	if err == nil {
		t.Error("Expected error for cancelled context")
	}
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestClient_SetHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Bearer token123" {
			t.Errorf("Expected Authorization header, got %s", auth)
		}
		if custom := r.Header.Get("X-Custom"); custom != "value" {
			t.Errorf("Expected X-Custom header, got %s", custom)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(ClientOptions{
		BaseURL: server.URL,
		Headers: map[string]string{
			"Authorization": "Bearer token123",
		},
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	client.SetHeader("X-Custom", "value")

	headers := http.Header{}
	headers.Set("X-Request-Id", "abc123")
	resp, err := client.Get(context.Background(), "/test", headers)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer resp.Body.Close()
}

func TestClient_RequestBodyRetry(t *testing.T) {
	var bodies []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(body))

		if len(bodies) < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(ClientOptions{
		BaseURL: server.URL,
		Retry: RetryConfig{
			MaxRetries: 3,
			BaseDelay:  10 * time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	body := strings.NewReader(`{"name": "John", "email": "john@example.com"}`)
	resp, err := client.Post(context.Background(), "/users", body, nil)
	if err != nil {
		t.Fatalf("Post failed: %v", err)
	}
	defer resp.Body.Close()

	// Both requests should have same body
	if len(bodies) != 2 {
		t.Errorf("Expected 2 requests, got %d", len(bodies))
	}
	if bodies[0] != bodies[1] {
		t.Errorf("Bodies don't match: %q vs %q", bodies[0], bodies[1])
	}
}

func TestClient_PathResolution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Path", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tests := []struct {
		name         string
		baseURL      string
		path         string
		expectedPath string
	}{
		{
			name:         "simple path",
			baseURL:      server.URL,
			path:         "/users",
			expectedPath: "/users",
		},
		{
			name:         "path with base",
			baseURL:      server.URL + "/api",
			path:         "/users",
			expectedPath: "/users", // Client replaces the path
		},
		{
			name:         "path without leading slash",
			baseURL:      server.URL,
			path:         "users",
			expectedPath: "/users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(ClientOptions{
				BaseURL: tt.baseURL,
			})
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}
			defer client.Close()

			resp, err := client.Get(context.Background(), tt.path, nil)
			if err != nil {
				t.Fatalf("Get failed: %v", err)
			}
			defer resp.Body.Close()

			actualPath := resp.Header.Get("X-Path")
			if actualPath != tt.expectedPath {
				t.Errorf("Expected path %q, got %q", tt.expectedPath, actualPath)
			}
		})
	}
}

func TestDefaultRetryable(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{"OK", http.StatusOK, false},
		{"Created", http.StatusCreated, false},
		{"BadRequest", http.StatusBadRequest, false},
		{"Unauthorized", http.StatusUnauthorized, false},
		{"Forbidden", http.StatusForbidden, false},
		{"NotFound", http.StatusNotFound, false},
		{"BadGateway", http.StatusBadGateway, true},
		{"ServiceUnavailable", http.StatusServiceUnavailable, true},
		{"GatewayTimeout", http.StatusGatewayTimeout, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: tt.statusCode}
			got := DefaultRetryable(resp, nil)
			if got != tt.want {
				t.Errorf("DefaultRetryable() = %v, want %v", got, tt.want)
			}
		})
	}

	// Test with error
	if !DefaultRetryable(nil, io.EOF) {
		t.Error("Expected retryable for network error")
	}
}
