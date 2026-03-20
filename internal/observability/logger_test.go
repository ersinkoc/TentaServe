package observability

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/ersinkoc/tentaserve/internal/config"
)

func TestNewLogger(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	}

	logger := NewLogger(cfg)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}

	if logger.config.Level != "debug" {
		t.Errorf("expected level debug, got %s", logger.config.Level)
	}
}

func TestNewDefaultLogger(t *testing.T) {
	logger := NewDefaultLogger()
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"unknown", slog.LevelInfo}, // default
		{"", slog.LevelInfo},        // default
	}

	for _, tc := range tests {
		result := parseLevel(tc.input)
		if result != tc.expected {
			t.Errorf("parseLevel(%q) = %v, expected %v", tc.input, result, tc.expected)
		}
	}
}

func TestLoggerWithRequest(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := &Logger{
		Logger: slog.New(handler),
		config: config.LoggingConfig{},
	}

	requestLogger := logger.WithRequest("req_abc123", "GET", "/test")
	requestLogger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, `"request_id":"req_abc123"`) {
		t.Errorf("expected request_id in output, got %s", output)
	}
	if !strings.Contains(output, `"method":"GET"`) {
		t.Errorf("expected method in output, got %s", output)
	}
	if !strings.Contains(output, `"path":"/test"`) {
		t.Errorf("expected path in output, got %s", output)
	}
}

func TestLoggerWithUpstream(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := &Logger{
		Logger: slog.New(handler),
		config: config.LoggingConfig{},
	}

	upstreamLogger := logger.WithUpstream("users-api")
	upstreamLogger.Info("upstream request")

	output := buf.String()
	if !strings.Contains(output, `"upstream":"users-api"`) {
		t.Errorf("expected upstream in output, got %s", output)
	}
}

func TestLogRequest(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := &Logger{
		Logger: slog.New(handler),
		config: config.LoggingConfig{},
	}

	logger.LogRequest("POST", "/graphql", "127.0.0.1", 200, 150*time.Millisecond)

	output := buf.String()
	if !strings.Contains(output, `"method":"POST"`) {
		t.Errorf("expected method in output, got %s", output)
	}
	if !strings.Contains(output, `"status_code":200`) {
		t.Errorf("expected status_code in output, got %s", output)
	}
	if !strings.Contains(output, `"duration":150000000`) {
		t.Errorf("expected duration in output, got %s", output)
	}
}

// --- Coverage boost tests for logger ---

func TestNewLogger_TextFormat(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "info",
		Format: "text",
		Output: "stdout",
	}
	logger := NewLogger(cfg)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	if logger.config.Format != "text" {
		t.Errorf("expected format text, got %s", logger.config.Format)
	}
}

func TestNewLogger_JSONFormat(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	}
	logger := NewLogger(cfg)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNewLogger_StderrOutput(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "warn",
		Format: "json",
		Output: "stderr",
	}
	logger := NewLogger(cfg)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNewLogger_EmptyOutput(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "error",
		Format: "json",
		Output: "",
	}
	logger := NewLogger(cfg)
	if logger == nil {
		t.Fatal("expected non-nil logger for empty output")
	}
}

func TestNewLogger_DebugLevel(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "debug",
		Format: "text",
		Output: "stdout",
	}
	logger := NewLogger(cfg)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	if logger.config.Level != "debug" {
		t.Errorf("expected level debug, got %s", logger.config.Level)
	}
}

func TestNewLogger_WarnLevel(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "warn",
		Format: "json",
		Output: "stdout",
	}
	logger := NewLogger(cfg)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNewLogger_ErrorLevel(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "error",
		Format: "json",
		Output: "stdout",
	}
	logger := NewLogger(cfg)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNewLogger_UnknownLevel(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "trace",
		Format: "json",
		Output: "stdout",
	}
	logger := NewLogger(cfg)
	if logger == nil {
		t.Fatal("expected non-nil logger for unknown level")
	}
}

func TestNewLogger_InvalidFilePath(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "/nonexistent/path/to/logfile.log",
	}
	// Should fall back to stdout without panicking
	logger := NewLogger(cfg)
	if logger == nil {
		t.Fatal("expected non-nil logger even with invalid file path")
	}
}

func TestNewLogger_AllLevelFormats(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error", "unknown", ""}
	formats := []string{"json", "text", "other"}
	outputs := []string{"stdout", "stderr", ""}

	for _, level := range levels {
		for _, format := range formats {
			for _, output := range outputs {
				cfg := config.LoggingConfig{
					Level:  level,
					Format: format,
					Output: output,
				}
				logger := NewLogger(cfg)
				if logger == nil {
					t.Errorf("NewLogger returned nil for level=%q format=%q output=%q", level, format, output)
				}
			}
		}
	}
}

func TestLoggerWithRequest_Fields(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := &Logger{
		Logger: slog.New(handler),
		config: config.LoggingConfig{Level: "info", Format: "json"},
	}

	reqLogger := logger.WithRequest("req-123", "POST", "/api/users")
	if reqLogger.config.Level != "info" {
		t.Errorf("expected config to be preserved, got level=%s", reqLogger.config.Level)
	}
	if reqLogger.config.Format != "json" {
		t.Errorf("expected config to be preserved, got format=%s", reqLogger.config.Format)
	}

	reqLogger.Info("test")
	output := buf.String()
	if !strings.Contains(output, "req-123") {
		t.Errorf("expected request_id in output, got %s", output)
	}
}

func TestLoggerWithUpstream_Fields(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := &Logger{
		Logger: slog.New(handler),
		config: config.LoggingConfig{Level: "debug"},
	}

	upLogger := logger.WithUpstream("backend-api")
	if upLogger.config.Level != "debug" {
		t.Errorf("expected config to be preserved, got level=%s", upLogger.config.Level)
	}

	upLogger.Info("upstream call")
	output := buf.String()
	if !strings.Contains(output, "backend-api") {
		t.Errorf("expected upstream in output, got %s", output)
	}
}

func TestLogRequest_AllFields(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := &Logger{
		Logger: slog.New(handler),
		config: config.LoggingConfig{},
	}

	logger.LogRequest("GET", "/health", "10.0.0.1", 404, 5*time.Second)

	output := buf.String()
	if !strings.Contains(output, `"method":"GET"`) {
		t.Errorf("expected method GET, got %s", output)
	}
	if !strings.Contains(output, `"path":"/health"`) {
		t.Errorf("expected path /health, got %s", output)
	}
	if !strings.Contains(output, `"client_ip":"10.0.0.1"`) {
		t.Errorf("expected client_ip, got %s", output)
	}
	if !strings.Contains(output, `"status_code":404`) {
		t.Errorf("expected status_code 404, got %s", output)
	}
}
