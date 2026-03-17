package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/ersinkoc/tentaserve/internal/config"
	"github.com/ersinkoc/tentaserve/internal/gateway/middleware"
	"github.com/ersinkoc/tentaserve/internal/observability"
	"github.com/ersinkoc/tentaserve/internal/server"
)

// Build info - set by ldflags during build
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
	goVersion = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		if err := serveCmd(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "validate":
		if err := validateCmd(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "version":
		printVersion()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func serveCmd(args []string) error {
	// Parse flags
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", "tentaserve.yaml", "Path to configuration file")
	port := fs.Int("port", 0, "Server port (overrides config)")
	host := fs.String("host", "", "Server host (overrides config)")
	logLevel := fs.String("log-level", "", "Log level: debug, info, warn, error (overrides config)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Load configuration
	cfg, err := config.LoadOrDefault(*configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Apply command-line overrides
	if *port != 0 {
		cfg.Server.Port = *port
	}
	if *host != "" {
		cfg.Server.Host = *host
	}
	if *logLevel != "" {
		cfg.Observability.Logging.Level = *logLevel
	}

	// Setup logger
	logger := observability.NewLogger(cfg.Observability.Logging)

	logger.Info("starting Tentaserve",
		slog.String("version", version),
		slog.String("config", *configPath),
		slog.String("addr", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)),
	)

	// Create a simple handler (placeholder for actual gateway)
	handler := createHandler(cfg, logger)

	// Create server
	srv := server.New(cfg, handler, logger.Logger)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logger.Info("received signal", slog.String("signal", sig.String()))
		cancel()
	}()

	// Run server
	if err := srv.Run(ctx); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

func validateCmd(args []string) error {
	// Parse flags
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	configPath := fs.String("config", "tentaserve.yaml", "Path to configuration file")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Load and validate configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	fmt.Println("Configuration is valid!")
	fmt.Printf("Server: %s:%d\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Printf("Upstreams: %d\n", len(cfg.Upstreams))
	for _, u := range cfg.Upstreams {
		fmt.Printf("  - %s (%s)\n", u.Name, u.Type)
	}

	return nil
}

func createHandler(cfg *config.Config, logger *observability.Logger) http.Handler {
	// Create a basic handler
	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("GET /-/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})

	// Metrics endpoint (placeholder)
	mux.HandleFunc("GET /-/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("# Tentaserve metrics\n"))
	})

	// GraphQL endpoint (placeholder)
	mux.HandleFunc("POST "+cfg.Gateway.GraphQLPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":null}`))
	})

	// MCP endpoint (placeholder)
	mux.HandleFunc("POST "+cfg.Gateway.MCPPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"tools":[]}`))
	})

	// REST endpoints (placeholder)
	mux.HandleFunc(cfg.Gateway.RESTPrefix+"/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"REST endpoint"}`))
	})

	// Apply middleware chain
	requestID := middleware.NewRequestID(cfg.Observability.RequestID.Header)
	handler := middleware.Chain(mux,
		requestID.Wrap,
		loggingMiddleware(logger),
	)

	return handler
}

func loggingMiddleware(logger *observability.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// TODO: Implement proper request logging with duration
			next.ServeHTTP(w, r)
		})
	}
}

func printVersion() {
	fmt.Printf("tentaserve version %s\n", version)
	fmt.Printf("  commit:    %s\n", commit)
	fmt.Printf("  built:     %s\n", buildDate)
	fmt.Printf("  go:        %s\n", goVersion)
}

func printUsage() {
	fmt.Println(`Tentaserve - Bi-directional GraphQL↔REST gateway with MCP server

Usage:
  tentaserve <command> [flags]

Commands:
  serve      Start the gateway server
  validate   Validate configuration file
  version    Print version information
  help       Show this help message

Examples:
  tentaserve serve --config tentaserve.yaml
  tentaserve serve --port 9090
  tentaserve validate --config tentaserve.yaml`)
}
