// Package observability provides logging, metrics, and health checking.
//
// The observability package uses structured logging (log/slog),
// Prometheus-compatible metrics, and health endpoints.
package observability

import (
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/ersinkoc/tentaserve/internal/config"
)

// Logger wraps slog.Logger with Tentaserve-specific functionality.
type Logger struct {
	*slog.Logger
	config config.LoggingConfig
}

// NewLogger creates a new structured logger based on configuration.
func NewLogger(cfg config.LoggingConfig) *Logger {
	level := parseLevel(cfg.Level)

	var output io.Writer
	switch cfg.Output {
	case "stdout":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	default:
		if cfg.Output != "" {
			// Try to open file
			f, err := os.OpenFile(cfg.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err != nil {
				// Fall back to stdout
				output = os.Stdout
			} else {
				output = f
			}
		} else {
			output = os.Stdout
		}
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(output, opts)
	} else {
		handler = slog.NewTextHandler(output, opts)
	}

	logger := slog.New(handler)

	return &Logger{
		Logger: logger,
		config: cfg,
	}
}

// NewDefaultLogger creates a logger with default settings.
func NewDefaultLogger() *Logger {
	return NewLogger(config.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	})
}

// parseLevel converts a string level to slog.Level.
func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// WithRequest adds request-specific fields to the logger.
func (l *Logger) WithRequest(requestID, method, path string) *Logger {
	return &Logger{
		Logger: l.Logger.With(
			slog.String("request_id", requestID),
			slog.String("method", method),
			slog.String("path", path),
		),
		config: l.config,
	}
}

// WithUpstream adds upstream-specific fields to the logger.
func (l *Logger) WithUpstream(name string) *Logger {
	return &Logger{
		Logger: l.Logger.With(
			slog.String("upstream", name),
		),
		config: l.config,
	}
}

// LogRequest logs an HTTP request.
func (l *Logger) LogRequest(method, path, clientIP string, statusCode int, duration time.Duration) {
	l.Logger.LogAttrs(nil, slog.LevelInfo, "request",
		slog.String("method", method),
		slog.String("path", path),
		slog.String("client_ip", clientIP),
		slog.Int("status_code", statusCode),
		slog.Duration("duration", duration),
	)
}
