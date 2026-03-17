package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/ersinkoc/tentaserve/internal/config"
)

func TestNewServer(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 18080

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	server := New(cfg, handler, logger)

	if server.Addr() != "127.0.0.1:18080" {
		t.Errorf("expected addr 127.0.0.1:18080, got %s", server.Addr())
	}

	if server.IsTLS() {
		t.Error("expected TLS to be disabled by default")
	}
}

func TestServerRunAndShutdown(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 18081

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := New(cfg, handler, logger)

	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel() // Signal shutdown after a short delay
	}()

	err := server.Run(ctx)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestServerGracefulShutdown(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 18082

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow request
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := New(cfg, handler, logger)

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		server.Run(ctx)
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Trigger shutdown
	cancel()

	// Give shutdown time to complete
	time.Sleep(100 * time.Millisecond)
}
