package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type mockProvider struct {
	data *DashboardData
}

func (m *mockProvider) GetDashboardData() *DashboardData {
	return m.data
}

func TestHandler_ServeDashboard(t *testing.T) {
	handler := NewHandler(Options{})

	req := httptest.NewRequest("GET", "/-/admin", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html, got %s", ct)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Tentaserve") {
		t.Error("expected dashboard to contain Tentaserve title")
	}
	if !strings.Contains(body, "textContent") {
		t.Error("expected safe DOM methods in dashboard JS")
	}
}

func TestHandler_ServeDashboard_TrailingSlash(t *testing.T) {
	handler := NewHandler(Options{})

	req := httptest.NewRequest("GET", "/-/admin/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandler_ServeAPIData(t *testing.T) {
	t.Run("with provider", func(t *testing.T) {
		provider := &mockProvider{
			data: &DashboardData{
				Version:     "0.1.0",
				UptimeMs:    60000,
				Requests:    1500,
				CacheHits:   1000,
				CacheMisses: 500,
				Upstreams: []UpstreamStatus{
					{Name: "users-api", Type: "rest", URL: "http://localhost:3000", Status: "healthy", Latency: 5},
				},
				Tools: []ToolInfo{
					{Name: "get_users", Description: "List users", Upstream: "users-api"},
				},
				Timestamp: time.Now(),
			},
		}

		handler := NewHandler(Options{Provider: provider})

		req := httptest.NewRequest("GET", "/-/admin/api/data", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var data DashboardData
		if err := json.NewDecoder(w.Body).Decode(&data); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if data.Version != "0.1.0" {
			t.Errorf("expected version 0.1.0, got %s", data.Version)
		}
		if data.Requests != 1500 {
			t.Errorf("expected 1500 requests, got %d", data.Requests)
		}
		if len(data.Upstreams) != 1 {
			t.Errorf("expected 1 upstream, got %d", len(data.Upstreams))
		}
		if len(data.Tools) != 1 {
			t.Errorf("expected 1 tool, got %d", len(data.Tools))
		}
	})

	t.Run("without provider", func(t *testing.T) {
		handler := NewHandler(Options{})

		req := httptest.NewRequest("GET", "/-/admin/api/data", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var data DashboardData
		if err := json.NewDecoder(w.Body).Decode(&data); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
	})
}

func TestHandler_BasicAuth(t *testing.T) {
	handler := NewHandler(Options{
		BasicAuth: &BasicAuth{
			Username: "admin",
			Password: "secret",
		},
	})

	t.Run("no credentials", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/-/admin", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}

		wwwAuth := w.Header().Get("WWW-Authenticate")
		if !strings.Contains(wwwAuth, "Basic") {
			t.Errorf("expected WWW-Authenticate Basic header, got %s", wwwAuth)
		}
	})

	t.Run("wrong credentials", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/-/admin", nil)
		req.SetBasicAuth("admin", "wrong")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("correct credentials", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/-/admin", nil)
		req.SetBasicAuth("admin", "secret")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})
}

func TestHandler_NotFound(t *testing.T) {
	handler := NewHandler(Options{})

	req := httptest.NewRequest("GET", "/-/admin/unknown", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
