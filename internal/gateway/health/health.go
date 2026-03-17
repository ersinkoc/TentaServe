package health

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Status represents the health status.
type Status string

const (
	// StatusHealthy indicates all checks passed.
	StatusHealthy Status = "healthy"
	// StatusDegraded indicates some non-critical checks failed.
	StatusDegraded Status = "degraded"
	// StatusUnhealthy indicates critical checks failed.
	StatusUnhealthy Status = "unhealthy"
)

// CheckResult is the result of a single health check.
type CheckResult struct {
	Name      string        `json:"name"`
	Status    Status        `json:"status"`
	Message   string        `json:"message,omitempty"`
	Duration  time.Duration `json:"duration_ms"`
	Timestamp time.Time     `json:"timestamp"`
}

// Response is the health check HTTP response.
type Response struct {
	Status    Status          `json:"status"`
	Version   string          `json:"version,omitempty"`
	Uptime    time.Duration   `json:"uptime_ms"`
	Timestamp time.Time       `json:"timestamp"`
	Checks    []*CheckResult  `json:"checks,omitempty"`
}

// Check is a health check function.
type Check func() *CheckResult

// Checker manages health checks.
type Checker struct {
	mu         sync.RWMutex
	checks     map[string]Check
	version    string
	startTime  time.Time
	cache      *Response
	cacheUntil time.Time
	cacheTTL   time.Duration
}

// NewChecker creates a new health checker.
func NewChecker(version string) *Checker {
	return &Checker{
		checks:    make(map[string]Check),
		version:   version,
		startTime: time.Now(),
		cacheTTL:  5 * time.Second,
	}
}

// SetCacheTTL sets the cache TTL.
func (c *Checker) SetCacheTTL(ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cacheTTL = ttl
}

// Register adds a health check.
func (c *Checker) Register(name string, check Check) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[name] = check
	c.cache = nil // Invalidate cache
}

// Unregister removes a health check.
func (c *Checker) Unregister(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.checks, name)
	c.cache = nil // Invalidate cache
}

// Check runs all health checks and returns the result.
func (c *Checker) Check() *Response {
	c.mu.RLock()
	// Check cache
	if c.cache != nil && time.Now().Before(c.cacheUntil) {
		cached := c.cache
		c.mu.RUnlock()
		return cached
	}
	checks := make([]Check, 0, len(c.checks))
	checkNames := make([]string, 0, len(c.checks))
	for name, check := range c.checks {
		checks = append(checks, check)
		checkNames = append(checkNames, name)
	}
	version := c.version
	startTime := c.startTime
	c.mu.RUnlock()

	// Run checks concurrently
	results := make([]*CheckResult, len(checks))
	var wg sync.WaitGroup
	for i, check := range checks {
		wg.Add(1)
		go func(idx int, fn Check) {
			defer wg.Done()
			start := time.Now()
			result := fn()
			result.Duration = time.Since(start)
			results[idx] = result
		}(i, check)
	}
	wg.Wait()

	// Determine overall status
	status := StatusHealthy
	for _, result := range results {
		if result.Status == StatusUnhealthy {
			status = StatusUnhealthy
			break
		}
		if result.Status == StatusDegraded && status == StatusHealthy {
			status = StatusDegraded
		}
	}

	resp := &Response{
		Status:    status,
		Version:   version,
		Uptime:    time.Since(startTime),
		Timestamp: time.Now(),
		Checks:    results,
	}

	// Update cache
	c.mu.Lock()
	c.cache = resp
	c.cacheUntil = time.Now().Add(c.cacheTTL)
	c.mu.Unlock()

	return resp
}

// CheckByName runs a specific health check.
func (c *Checker) CheckByName(name string) *CheckResult {
	c.mu.RLock()
	check, ok := c.checks[name]
	c.mu.RUnlock()

	if !ok {
		return &CheckResult{
			Name:      name,
			Status:    StatusUnhealthy,
			Message:   "check not found",
			Timestamp: time.Now(),
		}
	}

	start := time.Now()
	result := check()
	result.Duration = time.Since(start)
	return result
}

// Handler returns an HTTP handler for health checks.
func (c *Checker) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle specific check
		if checkName := r.URL.Query().Get("check"); checkName != "" {
			result := c.CheckByName(checkName)
			writeJSON(w, result, http.StatusOK)
			return
		}

		// Full health check
		resp := c.Check()

		// Determine HTTP status code
		statusCode := http.StatusOK
		switch resp.Status {
		case StatusHealthy:
			statusCode = http.StatusOK
		case StatusDegraded:
			statusCode = http.StatusOK // Still OK but with warning
		case StatusUnhealthy:
			statusCode = http.StatusServiceUnavailable
		}

		writeJSON(w, resp, statusCode)
	})
}

// LivenessHandler returns a simple liveness check handler.
func (c *Checker) LivenessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "alive",
		})
	})
}

// ReadinessHandler returns a readiness check handler.
func (c *Checker) ReadinessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := c.Check()

		// Only ready if healthy or degraded
		if resp.Status == StatusUnhealthy {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{
				"status": "not ready",
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ready",
		})
	})
}

func writeJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// SimpleCheck creates a simple check function.
func SimpleCheck(name string, fn func() error) Check {
	return func() *CheckResult {
		result := &CheckResult{
			Name:      name,
			Timestamp: time.Now(),
		}
		if err := fn(); err != nil {
			result.Status = StatusUnhealthy
			result.Message = err.Error()
		} else {
			result.Status = StatusHealthy
		}
		return result
	}
}

// SimpleCheckWithTimeout creates a check with timeout.
func SimpleCheckWithTimeout(name string, timeout time.Duration, fn func() error) Check {
	return func() *CheckResult {
		result := &CheckResult{
			Name:      name,
			Timestamp: time.Now(),
		}

		done := make(chan error, 1)
		go func() {
			done <- fn()
		}()

		select {
		case err := <-done:
			if err != nil {
				result.Status = StatusUnhealthy
				result.Message = err.Error()
			} else {
				result.Status = StatusHealthy
			}
		case <-time.After(timeout):
			result.Status = StatusUnhealthy
			result.Message = "check timed out"
		}

		return result
	}
}

// Default creates a default health checker.
func Default(version string) *Checker {
	c := NewChecker(version)
	// Add basic uptime check
	c.Register("uptime", func() *CheckResult {
		return &CheckResult{
			Name:      "uptime",
			Status:    StatusHealthy,
			Timestamp: time.Now(),
		}
	})
	return c
}
