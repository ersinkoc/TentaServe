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

// --- Coverage boost tests for server ---

func TestNewServer_WithTLS(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 18083
	cfg.Server.TLS = &config.TLSConfig{
		CertFile: "/tmp/cert.pem",
		KeyFile:  "/tmp/key.pem",
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := New(cfg, handler, logger)

	if !server.IsTLS() {
		t.Error("expected TLS to be enabled")
	}
	if server.Addr() != "127.0.0.1:18083" {
		t.Errorf("expected addr 127.0.0.1:18083, got %s", server.Addr())
	}
}

func TestNewServer_CustomTimeouts(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 18084
	cfg.Server.ReadTimeout = 5 * time.Second
	cfg.Server.WriteTimeout = 10 * time.Second
	cfg.Server.IdleTimeout = 60 * time.Second
	cfg.Server.MaxHeaderBytes = 1024

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := New(cfg, handler, logger)

	if server == nil {
		t.Fatal("expected non-nil server")
	}
	if server.IsTLS() {
		t.Error("expected TLS to be disabled")
	}
}

func TestServerAddr(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     int
		expected string
	}{
		{"localhost with port", "127.0.0.1", 8080, "127.0.0.1:8080"},
		{"all interfaces", "0.0.0.0", 3000, "0.0.0.0:3000"},
		{"empty host", "", 9090, ":9090"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Server.Host = tt.host
			cfg.Server.Port = tt.port

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			server := New(cfg, handler, logger)

			if server.Addr() != tt.expected {
				t.Errorf("expected addr %q, got %q", tt.expected, server.Addr())
			}
		})
	}
}

func TestServerIsTLS(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 18085

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Without TLS
	server := New(cfg, handler, logger)
	if server.IsTLS() {
		t.Error("expected TLS=false when TLS config is nil")
	}

	// With TLS
	cfg.Server.TLS = &config.TLSConfig{CertFile: "cert.pem", KeyFile: "key.pem"}
	serverTLS := New(cfg, handler, logger)
	if !serverTLS.IsTLS() {
		t.Error("expected TLS=true when TLS config is set")
	}
}

func TestServerShutdown_Direct(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 18086

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := New(cfg, handler, logger)

	// Start the server
	go func() {
		server.httpServer.ListenAndServe()
	}()
	time.Sleep(50 * time.Millisecond)

	// Direct shutdown call
	err := server.Shutdown()
	if err != nil {
		t.Errorf("expected no error from shutdown, got %v", err)
	}
}

func TestServerRun_WithRequest(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 18087

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := New(cfg, handler, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Run(ctx)
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Make a request
	resp, err := http.Get("http://127.0.0.1:18087/")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Shutdown
	cancel()
	runErr := <-errCh
	if runErr != nil {
		t.Errorf("unexpected error from Run: %v", runErr)
	}
}
