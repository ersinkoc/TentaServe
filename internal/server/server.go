// Package server provides HTTP server functionality for Tentaserve.
//
// The server package handles HTTP server setup, TLS configuration,
// graceful shutdown, and request routing.
package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/ersinkoc/tentaserve/internal/config"
)

// Server wraps an HTTP server with Tentaserve-specific functionality.
type Server struct {
	httpServer *http.Server
	config     *config.ServerConfig
	logger     *slog.Logger
}

// New creates a new HTTP server with the given configuration and handler.
func New(cfg *config.Config, handler http.Handler, logger *slog.Logger) *Server {
	serverCfg := &cfg.Server

	// Configure TLS if enabled
	var tlsConfig *tls.Config
	if serverCfg.TLS != nil {
		tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			CurvePreferences: []tls.CurveID{
				tls.X25519,
				tls.CurveP256,
			},
			PreferServerCipherSuites: true,
		}
	}

	addr := net.JoinHostPort(serverCfg.Host, strconv.Itoa(serverCfg.Port))

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       serverCfg.ReadTimeout,
		WriteTimeout:      serverCfg.WriteTimeout,
		IdleTimeout:       serverCfg.IdleTimeout,
		MaxHeaderBytes:    serverCfg.MaxHeaderBytes,
		TLSConfig:         tlsConfig,
	}

	return &Server{
		httpServer: httpServer,
		config:     serverCfg,
		logger:     logger,
	}
}

// Run starts the server and blocks until it shuts down.
// It handles graceful shutdown when the context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	// Start server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("starting server",
			slog.String("addr", s.httpServer.Addr),
			slog.Bool("tls", s.config.TLS != nil),
		)

		var err error
		if s.config.TLS != nil {
			err = s.httpServer.ListenAndServeTLS(
				s.config.TLS.CertFile,
				s.config.TLS.KeyFile,
			)
		} else {
			err = s.httpServer.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("server error: %w", err)
			return
		}
		close(errCh)
	}()

	// Wait for context cancellation or server error
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		s.logger.Info("shutdown signal received, starting graceful shutdown")
		return s.Shutdown()
	}
}

// Shutdown gracefully shuts down the server.
// It gives active connections time to complete.
func (s *Server) Shutdown() error {
	// Create a context with timeout for shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	s.logger.Info("shutting down server")

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		s.logger.Error("server shutdown error", slog.Any("error", err))
		return fmt.Errorf("shutdown failed: %w", err)
	}

	s.logger.Info("server stopped gracefully")
	return nil
}

// Addr returns the server address.
func (s *Server) Addr() string {
	return s.httpServer.Addr
}

// IsTLS returns true if TLS is enabled.
func (s *Server) IsTLS() bool {
	return s.config.TLS != nil
}
