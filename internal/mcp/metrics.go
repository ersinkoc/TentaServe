package mcp

import (
	"net/http"
	"time"

	"github.com/ersinkoc/tentaserve/internal/gateway/metrics"
)

// Metrics holds MCP-specific metrics.
type Metrics struct {
	// Counters
	RequestsTotal       *metrics.CounterVec
	ToolsCalledTotal    *metrics.CounterVec
	ResourcesReadTotal  *metrics.CounterVec
	SSEConnectionsTotal *metrics.CounterVec

	// Gauges
	ActiveConnections   *metrics.GaugeVec
	RegisteredTools     *metrics.GaugeVec
	RegisteredResources *metrics.GaugeVec

	// Histograms
	RequestDuration  *metrics.HistogramVec
	ToolCallDuration *metrics.HistogramVec
}

// NewMetrics creates MCP metrics and registers them with the provided registry.
func NewMetrics(reg *metrics.Registry) *Metrics {
	if reg == nil {
		return nil
	}

	m := &Metrics{
		// Counters
		RequestsTotal: reg.RegisterCounter(
			"mcp_requests_total",
			"Total number of MCP requests",
			"method",
		),
		ToolsCalledTotal: reg.RegisterCounter(
			"mcp_tools_called_total",
			"Total number of tool calls",
			"tool",
			"upstream",
		),
		ResourcesReadTotal: reg.RegisterCounter(
			"mcp_resources_read_total",
			"Total number of resource reads",
			"uri",
		),
		SSEConnectionsTotal: reg.RegisterCounter(
			"mcp_sse_connections_total",
			"Total number of SSE connections established",
		),

		// Gauges
		ActiveConnections: reg.RegisterGauge(
			"mcp_active_connections",
			"Number of currently active SSE connections",
		),
		RegisteredTools: reg.RegisterGauge(
			"mcp_registered_tools",
			"Number of registered MCP tools",
		),
		RegisteredResources: reg.RegisterGauge(
			"mcp_registered_resources",
			"Number of registered MCP resources",
		),

		// Histograms
		RequestDuration: reg.RegisterHistogram(
			"mcp_request_duration_seconds",
			"MCP request duration in seconds",
			metrics.DefaultBuckets,
			"method",
		),
		ToolCallDuration: reg.RegisterHistogram(
			"mcp_tool_call_duration_seconds",
			"Tool call duration in seconds",
			metrics.DefaultBuckets,
			"tool",
		),
	}

	return m
}

// InstrumentHandler wraps an HTTP handler to record metrics.
func (m *Metrics) InstrumentHandler(method string, handler http.HandlerFunc) http.HandlerFunc {
	if m == nil {
		return handler
	}

	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Call the handler
		handler(w, r)

		// Record metrics
		duration := time.Since(start).Seconds()
		m.RequestsTotal.Inc(method)
		m.RequestDuration.Observe(duration, method)
	}
}

// RecordToolCall records a tool call.
func (m *Metrics) RecordToolCall(toolName, upstream string, duration time.Duration) {
	if m == nil {
		return
	}
	m.ToolsCalledTotal.Inc(toolName, upstream)
	m.ToolCallDuration.Observe(duration.Seconds(), toolName)
}

// RecordResourceRead records a resource read.
func (m *Metrics) RecordResourceRead(uri string) {
	if m == nil {
		return
	}
	m.ResourcesReadTotal.Inc(uri)
}

// RecordSSEConnection records a new SSE connection.
func (m *Metrics) RecordSSEConnection() {
	if m == nil {
		return
	}
	m.SSEConnectionsTotal.Inc()
	m.ActiveConnections.Inc()
}

// RecordSSEDisconnection records an SSE disconnection.
func (m *Metrics) RecordSSEDisconnection() {
	if m == nil {
		return
	}
	m.ActiveConnections.Dec()
}

// UpdateToolCount updates the registered tools gauge.
func (m *Metrics) UpdateToolCount(count int) {
	if m == nil {
		return
	}
	m.RegisteredTools.Set(float64(count))
}

// UpdateResourceCount updates the registered resources gauge.
func (m *Metrics) UpdateResourceCount(count int) {
	if m == nil {
		return
	}
	m.RegisteredResources.Set(float64(count))
}
