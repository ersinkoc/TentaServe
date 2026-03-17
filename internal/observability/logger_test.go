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
