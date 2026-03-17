package mcp

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ersinkoc/tentaserve/internal/gateway/metrics"
)

// TestNewMetrics tests the creation of MCP metrics.
func TestNewMetrics(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewMetrics(reg)

	if m == nil {
		t.Fatal("Expected non-nil metrics")
	}

	// Check that all metrics were created
	if m.RequestsTotal == nil {
		t.Error("RequestsTotal should not be nil")
	}
	if m.ToolsCalledTotal == nil {
		t.Error("ToolsCalledTotal should not be nil")
	}
	if m.ResourcesReadTotal == nil {
		t.Error("ResourcesReadTotal should not be nil")
	}
	if m.SSEConnectionsTotal == nil {
		t.Error("SSEConnectionsTotal should not be nil")
	}
	if m.ActiveConnections == nil {
		t.Error("ActiveConnections should not be nil")
	}
	if m.RegisteredTools == nil {
		t.Error("RegisteredTools should not be nil")
	}
	if m.RegisteredResources == nil {
		t.Error("RegisteredResources should not be nil")
	}
	if m.RequestDuration == nil {
		t.Error("RequestDuration should not be nil")
	}
	if m.ToolCallDuration == nil {
		t.Error("ToolCallDuration should not be nil")
	}
}

// TestNewMetricsNilRegistry tests that NewMetrics returns nil for nil registry.
func TestNewMetricsNilRegistry(t *testing.T) {
	m := NewMetrics(nil)
	if m != nil {
		t.Error("Expected nil metrics for nil registry")
	}
}

// TestMetricsInstrumentHandler tests the handler instrumentation.
func TestMetricsInstrumentHandler(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewMetrics(reg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	instrumented := m.InstrumentHandler("test_method", handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	instrumented.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// TestMetricsInstrumentHandlerNilMetrics tests that nil metrics returns original handler.
func TestMetricsInstrumentHandlerNilMetrics(t *testing.T) {
	var m *Metrics

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	instrumented := m.InstrumentHandler("test_method", handler)

	// Should be the same handler (no instrumentation)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	instrumented.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// TestRecordToolCall tests recording tool call metrics.
func TestRecordToolCall(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewMetrics(reg)

	// Should not panic
	m.RecordToolCall("test_tool", "test_upstream", 100*time.Millisecond)
}

// TestRecordToolCallNilMetrics tests that nil metrics doesn't panic.
func TestRecordToolCallNilMetrics(t *testing.T) {
	var m *Metrics
	// Should not panic
	m.RecordToolCall("test_tool", "test_upstream", 100*time.Millisecond)
}

// TestRecordResourceRead tests recording resource read metrics.
func TestRecordResourceRead(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewMetrics(reg)

	// Should not panic
	m.RecordResourceRead("test://resource")
}

// TestRecordResourceReadNilMetrics tests that nil metrics doesn't panic.
func TestRecordResourceReadNilMetrics(t *testing.T) {
	var m *Metrics
	// Should not panic
	m.RecordResourceRead("test://resource")
}

// TestRecordSSEConnection tests recording SSE connection metrics.
func TestRecordSSEConnection(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewMetrics(reg)

	// Should not panic
	m.RecordSSEConnection()
}

// TestRecordSSEConnectionNilMetrics tests that nil metrics doesn't panic.
func TestRecordSSEConnectionNilMetrics(t *testing.T) {
	var m *Metrics
	// Should not panic
	m.RecordSSEConnection()
}

// TestRecordSSEDisconnection tests recording SSE disconnection metrics.
func TestRecordSSEDisconnection(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewMetrics(reg)

	// First connect
	m.RecordSSEConnection()
	// Then disconnect
	m.RecordSSEDisconnection()
}

// TestRecordSSEDisconnectionNilMetrics tests that nil metrics doesn't panic.
func TestRecordSSEDisconnectionNilMetrics(t *testing.T) {
	var m *Metrics
	// Should not panic
	m.RecordSSEDisconnection()
}

// TestUpdateToolCount tests updating tool count gauge.
func TestUpdateToolCount(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewMetrics(reg)

	// Should not panic
	m.UpdateToolCount(10)
	m.UpdateToolCount(5)
	m.UpdateToolCount(0)
}

// TestUpdateToolCountNilMetrics tests that nil metrics doesn't panic.
func TestUpdateToolCountNilMetrics(t *testing.T) {
	var m *Metrics
	// Should not panic
	m.UpdateToolCount(10)
}

// TestUpdateResourceCount tests updating resource count gauge.
func TestUpdateResourceCount(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewMetrics(reg)

	// Should not panic
	m.UpdateResourceCount(10)
	m.UpdateResourceCount(5)
	m.UpdateResourceCount(0)
}

// TestUpdateResourceCountNilMetrics tests that nil metrics doesn't panic.
func TestUpdateResourceCountNilMetrics(t *testing.T) {
	var m *Metrics
	// Should not panic
	m.UpdateResourceCount(10)
}

// TestMetricsIntegrationWithServer tests metrics work with MCP server.
func TestMetricsIntegrationWithServer(t *testing.T) {
	reg := metrics.NewRegistry()
	mcpMetrics := NewMetrics(reg)

	server := NewServer(nil, mcpMetrics)
	if server == nil {
		t.Fatal("Expected non-nil server")
	}

	if server.metrics != mcpMetrics {
		t.Error("Server should have the provided metrics")
	}
}

// TestMetricsServerNilMetrics tests server works with nil metrics.
func TestMetricsServerNilMetrics(t *testing.T) {
	server := NewServer(nil, nil)
	if server == nil {
		t.Fatal("Expected non-nil server")
	}

	if server.metrics != nil {
		t.Error("Server should have nil metrics when nil is passed")
	}
}
