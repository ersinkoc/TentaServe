// Package upstream provides HTTP clients for backend services.
package upstream

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Client is an HTTP client for a specific upstream.
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
	retry      RetryConfig
	headers    map[string]string
	mu         sync.RWMutex
}

// RetryConfig configures retry behavior.
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	Multiplier float64
	Retryable  func(resp *http.Response, err error) bool
}

// DefaultRetryConfig returns sensible retry defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   5 * time.Second,
		Multiplier: 2.0,
		Retryable:  DefaultRetryable,
	}
}

// DefaultRetryable determines if a request should be retried.
func DefaultRetryable(resp *http.Response, err error) bool {
	if err != nil {
		return true // Network errors are retryable
	}
	// Retry on 502, 503, 504
	switch resp.StatusCode {
	case http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}
	return false
}

// ClientOptions configures the client.
type ClientOptions struct {
	BaseURL      string
	Timeout      time.Duration
	Retry        RetryConfig
	Headers      map[string]string
	MaxIdleConns int
}

// NewClient creates a new upstream client.
func NewClient(opts ClientOptions) (*Client, error) {
	baseURL, err := url.Parse(opts.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	maxIdleConns := opts.MaxIdleConns
	if maxIdleConns == 0 {
		maxIdleConns = 10
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        maxIdleConns,
		MaxIdleConnsPerHost: maxIdleConns,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	retry := opts.Retry
	if retry.MaxRetries == 0 && retry.BaseDelay == 0 {
		retry = DefaultRetryConfig()
	}
	if retry.Retryable == nil {
		retry.Retryable = DefaultRetryable
	}

	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
		retry:      retry,
		headers:    opts.Headers,
	}, nil
}

// Do performs an HTTP request with the upstream client.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.DoWithRetry(req, c.retry.MaxRetries)
}

// DoWithRetry performs an HTTP request with retry logic.
func (c *Client) DoWithRetry(req *http.Request, maxRetries int) (*http.Response, error) {
	var lastErr error
	var lastResp *http.Response

	delay := c.retry.BaseDelay

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(delay):
			}
			// Increase delay with jitter
			delay = calculateNextDelay(delay, c.retry.MaxDelay, c.retry.Multiplier)
		}

		// Clone request if we need to retry (body needs to be preserved)
		var bodyReader io.ReadCloser
		if req.Body != nil {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, fmt.Errorf("reading request body: %w", err)
			}
			req.Body = io.NopCloser(NewBodyReader(body))
			bodyReader = io.NopCloser(NewBodyReader(body))
		}

		resp, err := c.doOnce(req)

		// Check if we should retry
		if !c.retry.Retryable(resp, err) {
			return resp, err
		}

		lastErr = err
		lastResp = resp

		// Restore body for next attempt
		if bodyReader != nil {
			req.Body = bodyReader
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("after %d retries: %w", maxRetries, lastErr)
	}

	return lastResp, nil
}

// doOnce performs a single HTTP request.
func (c *Client) doOnce(req *http.Request) (*http.Response, error) {
	// Build full URL
	fullURL := c.baseURL.ResolveReference(req.URL)

	// Create new request
	newReq, err := http.NewRequestWithContext(
		req.Context(),
		req.Method,
		fullURL.String(),
		req.Body,
	)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Copy headers
	for key, values := range req.Header {
		for _, value := range values {
			newReq.Header.Add(key, value)
		}
	}

	// Apply default headers
	c.mu.RLock()
	for key, value := range c.headers {
		if newReq.Header.Get(key) == "" {
			newReq.Header.Set(key, value)
		}
	}
	c.mu.RUnlock()

	// Ensure content type is set for POST/PUT/PATCH
	if req.Body != nil && newReq.Header.Get("Content-Type") == "" {
		newReq.Header.Set("Content-Type", "application/json")
	}

	return c.httpClient.Do(newReq)
}

// SetHeader sets a default header for all requests.
func (c *Client) SetHeader(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.headers == nil {
		c.headers = make(map[string]string)
	}
	c.headers[key] = value
}

// Close closes the client and its connections.
func (c *Client) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}

// Get performs a GET request.
func (c *Client) Get(ctx context.Context, path string, headers http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	return c.Do(req)
}

// Post performs a POST request.
func (c *Client) Post(ctx context.Context, path string, body io.Reader, headers http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	return c.Do(req)
}

// Put performs a PUT request.
func (c *Client) Put(ctx context.Context, path string, body io.Reader, headers http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, path, body)
	if err != nil {
		return nil, err
	}
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	return c.Do(req)
}

// Delete performs a DELETE request.
func (c *Client) Delete(ctx context.Context, path string, headers http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return nil, err
	}
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	return c.Do(req)
}
