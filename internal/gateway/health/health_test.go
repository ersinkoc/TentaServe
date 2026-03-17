package health

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestNewChecker tests checker creation.
func TestNewChecker(t *testing.T) {
	c := NewChecker("1.0.0")
	if c == nil {
		t.Fatal("Expected non-nil checker")
	}
	if c.version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", c.version)
	}
	if c.cacheTTL != 5*time.Second {
		t.Errorf("Expected default cache TTL 5s, got %v", c.cacheTTL)
	}
}

// TestDefault tests default checker.
func TestDefault(t *testing.T) {
	c := Default("1.0.0")
	if c == nil {
		t.Fatal("Expected non-nil checker")
	}
	// Should have uptime check
	if len(c.checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(c.checks))
	}
}

// TestRegisterUnregister tests check registration.
func TestRegisterUnregister(t *testing.T) {
	c := NewChecker("1.0.0")

	check := func() *CheckResult {
		return &CheckResult{Name: "test", Status: StatusHealthy}
	}

	c.Register("test-check", check)
	if len(c.checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(c.checks))
	}

	c.Unregister("test-check")
	if len(c.checks) != 0 {
		t.Errorf("Expected 0 checks, got %d", len(c.checks))
	}
}

// TestCheckHealthy tests healthy response.
func TestCheckHealthy(t *testing.T) {
	c := NewChecker("1.0.0")
	c.Register("check1", func() *CheckResult {
		return &CheckResult{Name: "check1", Status: StatusHealthy}
	})
	c.Register("check2", func() *CheckResult {
		return &CheckResult{Name: "check2", Status: StatusHealthy}
	})

	resp := c.Check()

	if resp.Status != StatusHealthy {
		t.Errorf("Expected status healthy, got %s", resp.Status)
	}
	if resp.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", resp.Version)
	}
	if len(resp.Checks) != 2 {
		t.Errorf("Expected 2 checks, got %d", len(resp.Checks))
	}
	if resp.Uptime < 0 {
		t.Error("Expected non-negative uptime")
	}
}

// TestCheckUnhealthy tests unhealthy response.
func TestCheckUnhealthy(t *testing.T) {
	c := NewChecker("1.0.0")
	c.Register("check1", func() *CheckResult {
		return &CheckResult{Name: "check1", Status: StatusHealthy}
	})
	c.Register("check2", func() *CheckResult {
		return &CheckResult{Name: "check2", Status: StatusUnhealthy, Message: "failed"}
	})

	resp := c.Check()

	if resp.Status != StatusUnhealthy {
		t.Errorf("Expected status unhealthy, got %s", resp.Status)
	}
}

// TestCheckDegraded tests degraded response.
func TestCheckDegraded(t *testing.T) {
	c := NewChecker("1.0.0")
	c.Register("check1", func() *CheckResult {
		return &CheckResult{Name: "check1", Status: StatusHealthy}
	})
	c.Register("check2", func() *CheckResult {
		return &CheckResult{Name: "check2", Status: StatusDegraded, Message: "slow"}
	})

	resp := c.Check()

	if resp.Status != StatusDegraded {
		t.Errorf("Expected status degraded, got %s", resp.Status)
	}
}

// TestCheckCaching tests response caching.
func TestCheckCaching(t *testing.T) {
	c := NewChecker("1.0.0")
	c.SetCacheTTL(1 * time.Hour) // Long cache

	callCount := 0
	c.Register("check1", func() *CheckResult {
		callCount++
		return &CheckResult{Name: "check1", Status: StatusHealthy}
	})

	c.Check()
	c.Check()
	c.Check()

	if callCount != 1 {
		t.Errorf("Expected 1 check call (cached), got %d", callCount)
	}
}

// TestCheckByName tests specific check.
func TestCheckByName(t *testing.T) {
	c := NewChecker("1.0.0")
	c.Register("specific", func() *CheckResult {
		return &CheckResult{Name: "specific", Status: StatusHealthy}
	})

	result := c.CheckByName("specific")
	if result.Status != StatusHealthy {
		t.Errorf("Expected healthy, got %s", result.Status)
	}
	if result.Name != "specific" {
		t.Errorf("Expected name specific, got %s", result.Name)
	}
}

// TestCheckByNameNotFound tests missing check.
func TestCheckByNameNotFound(t *testing.T) {
	c := NewChecker("1.0.0")

	result := c.CheckByName("missing")
	if result.Status != StatusUnhealthy {
		t.Errorf("Expected unhealthy, got %s", result.Status)
	}
	if result.Message != "check not found" {
		t.Errorf("Expected 'check not found', got %s", result.Message)
	}
}

// TestHandler tests health HTTP handler.
func TestHandler(t *testing.T) {
	c := NewChecker("1.0.0")
	c.Register("test", func() *CheckResult {
		return &CheckResult{Name: "test", Status: StatusHealthy}
	})

	handler := c.Handler()

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Status != StatusHealthy {
		t.Errorf("Expected healthy, got %s", resp.Status)
	}
	if resp.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", resp.Version)
	}
}

// TestHandlerUnhealthy tests unhealthy HTTP response.
func TestHandlerUnhealthy(t *testing.T) {
	c := NewChecker("1.0.0")
	c.Register("test", func() *CheckResult {
		return &CheckResult{Name: "test", Status: StatusUnhealthy, Message: "failed"}
	})

	handler := c.Handler()

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

// TestHandlerSpecificCheck tests specific check endpoint.
func TestHandlerSpecificCheck(t *testing.T) {
	c := NewChecker("1.0.0")
	c.Register("specific", func() *CheckResult {
		return &CheckResult{Name: "specific", Status: StatusHealthy}
	})

	handler := c.Handler()

	req := httptest.NewRequest("GET", "/health?check=specific", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var result CheckResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result.Name != "specific" {
		t.Errorf("Expected specific, got %s", result.Name)
	}
}

// TestLivenessHandler tests liveness endpoint.
func TestLivenessHandler(t *testing.T) {
	c := NewChecker("1.0.0")
	handler := c.LivenessHandler()

	req := httptest.NewRequest("GET", "/live", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp["status"] != "alive" {
		t.Errorf("Expected alive, got %s", resp["status"])
	}
}

// TestReadinessHandlerHealthy tests readiness when healthy.
func TestReadinessHandlerHealthy(t *testing.T) {
	c := NewChecker("1.0.0")
	c.Register("test", func() *CheckResult {
		return &CheckResult{Name: "test", Status: StatusHealthy}
	})

	handler := c.ReadinessHandler()

	req := httptest.NewRequest("GET", "/ready", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp["status"] != "ready" {
		t.Errorf("Expected ready, got %s", resp["status"])
	}
}

// TestReadinessHandlerUnhealthy tests readiness when unhealthy.
func TestReadinessHandlerUnhealthy(t *testing.T) {
	c := NewChecker("1.0.0")
	c.Register("test", func() *CheckResult {
		return &CheckResult{Name: "test", Status: StatusUnhealthy}
	})

	handler := c.ReadinessHandler()

	req := httptest.NewRequest("GET", "/ready", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp["status"] != "not ready" {
		t.Errorf("Expected not ready, got %s", resp["status"])
	}
}

// TestSimpleCheck tests simple check helper.
func TestSimpleCheck(t *testing.T) {
	// Healthy check
	check := SimpleCheck("simple", func() error {
		return nil
	})

	result := check()
	if result.Status != StatusHealthy {
		t.Errorf("Expected healthy, got %s", result.Status)
	}

	// Unhealthy check
	check = SimpleCheck("simple", func() error {
		return errors.New("failed")
	})

	result = check()
	if result.Status != StatusUnhealthy {
		t.Errorf("Expected unhealthy, got %s", result.Status)
	}
	if result.Message != "failed" {
		t.Errorf("Expected message 'failed', got %s", result.Message)
	}
}

// TestSimpleCheckWithTimeout tests timeout check helper.
func TestSimpleCheckWithTimeout(t *testing.T) {
	// Fast check
	check := SimpleCheckWithTimeout("fast", 1*time.Second, func() error {
		return nil
	})

	result := check()
	if result.Status != StatusHealthy {
		t.Errorf("Expected healthy, got %s", result.Status)
	}

	// Slow check that times out
	check = SimpleCheckWithTimeout("slow", 1*time.Millisecond, func() error {
		time.Sleep(10 * time.Millisecond)
		return nil
	})

	result = check()
	if result.Status != StatusUnhealthy {
		t.Errorf("Expected unhealthy (timeout), got %s", result.Status)
	}
	if result.Message != "check timed out" {
		t.Errorf("Expected timeout message, got %s", result.Message)
	}
}

// TestCheckResultDuration tests duration is recorded.
func TestCheckResultDuration(t *testing.T) {
	c := NewChecker("1.0.0")
	c.Register("slow", func() *CheckResult {
		time.Sleep(10 * time.Millisecond)
		return &CheckResult{Name: "slow", Status: StatusHealthy}
	})

	result := c.CheckByName("slow")
	if result.Duration < 10*time.Millisecond {
		t.Errorf("Expected duration >= 10ms, got %v", result.Duration)
	}
}
