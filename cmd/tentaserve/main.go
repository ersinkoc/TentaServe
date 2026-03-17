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
	"time"

	"github.com/ersinkoc/tentaserve/internal/config"
	"github.com/ersinkoc/tentaserve/internal/gateway/auth"
	"github.com/ersinkoc/tentaserve/internal/gateway/metrics"
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
	case "schema":
		if err := schemaCmd(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "tools":
		if err := toolsCmd(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "jwt":
		if err := jwtCmd(os.Args[2:]); err != nil {
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

	// Create metrics registry if enabled
	var metricsReg *metrics.Registry
	if cfg.Observability.Metrics.Enabled {
		metricsReg = metrics.NewRegistry()
	}

	// Create MCP server if enabled
	mcpServer, err := NewMCPServer(cfg, logger.Logger, metricsReg)
	if err != nil {
		logger.Warn("failed to create MCP server", slog.Any("error", err))
	}

	// Create handler with MCP integration
	handler := createHandler(cfg, logger, mcpServer, metricsReg)

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

		// Shutdown MCP server
		if mcpServer != nil {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			if err := mcpServer.Shutdown(shutdownCtx); err != nil {
				logger.Warn("MCP server shutdown error", slog.Any("error", err))
			}
		}

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

func createHandler(cfg *config.Config, logger *observability.Logger, mcpServer *MCPServer, metricsReg *metrics.Registry) http.Handler {
	// Create a basic handler
	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("GET /-/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})

	// Metrics endpoint
	if cfg.Observability.Metrics.Enabled && metricsReg != nil {
		mux.Handle("GET "+cfg.Observability.Metrics.Path, metricsReg.Handler())
	} else {
		mux.HandleFunc("GET /-/metrics", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("# Metrics disabled\n"))
		})
	}

	// GraphQL endpoint - create real handler with resolvers
	graphqlHandler := createGraphQLHandler(cfg, logger.Logger)
	mux.Handle("POST "+cfg.Gateway.GraphQLPath, graphqlHandler)

	// MCP endpoint - integrate with MCP server if enabled
	if mcpServer != nil && cfg.MCP.Enabled {
		mcpServer.RegisterHandlers(mux, cfg.Gateway.MCPPath)
	} else {
		// Fallback placeholder
		mux.HandleFunc("POST "+cfg.Gateway.MCPPath, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"jsonrpc":"2.0","error":{"code":-32601,"message":"MCP not enabled"},"id":null}`))
		})
	}

	// REST endpoints - create handler with upstream routing
	restHandler := NewRESTHandler(cfg, logger.Logger)
	mux.Handle(cfg.Gateway.RESTPrefix+"/", restHandler)

	// Apply middleware chain
	requestID := middleware.NewRequestID(cfg.Observability.RequestID.Header)
	chain := middleware.NewChain(
		requestID.Wrap,
		loggingMiddleware(logger),
	)

	// Add auth middleware if configured
	authMiddleware := createAuthMiddleware(cfg, logger)
	if authMiddleware != nil {
		chain = middleware.NewChain(
			requestID.Wrap,
			authMiddleware.Wrap,
			loggingMiddleware(logger),
		)
	}

	handler := chain.Then(mux)

	return handler
}

// createAuthMiddleware creates authentication middleware based on config.
func createAuthMiddleware(cfg *config.Config, logger *observability.Logger) *auth.Middleware {
	if !cfg.Gateway.Auth.JWT.Enabled && !cfg.Gateway.Auth.APIKey.Enabled {
		return nil
	}

	var plugin auth.Plugin

	switch cfg.Gateway.Auth.Strategy {
	case "jwt":
		if cfg.Gateway.Auth.JWT.Secret == "" {
			logger.Warn("JWT auth enabled but no secret configured")
			return nil
		}
		jwtConfig := auth.NewJWT([]byte(cfg.Gateway.Auth.JWT.Secret))
		jwtConfig.Issuer = cfg.Gateway.Auth.JWT.Issuer
		jwtConfig.Audience = cfg.Gateway.Auth.JWT.Audience
		if len(cfg.Gateway.Auth.JWT.AllowedAlgorithms) > 0 {
			jwtConfig.AllowedAlgorithms = cfg.Gateway.Auth.JWT.AllowedAlgorithms
		}
		if cfg.Gateway.Auth.JWT.HeaderName != "" {
			jwtConfig.HeaderName = cfg.Gateway.Auth.JWT.HeaderName
		}
		if cfg.Gateway.Auth.JWT.HeaderPrefix != "" {
			jwtConfig.HeaderPrefix = cfg.Gateway.Auth.JWT.HeaderPrefix
		}
		plugin = jwtConfig
		logger.Info("JWT authentication enabled",
			slog.String("issuer", jwtConfig.Issuer),
			slog.String("audience", jwtConfig.Audience),
		)
	case "apikey":
		if len(cfg.Gateway.Auth.APIKey.Keys) == 0 && len(cfg.Gateway.Auth.APIKey.KeyMap) == 0 {
			logger.Warn("API Key auth enabled but no keys configured")
			return nil
		}
		// Note: APIKey plugin implementation would go here
		// For now, fall through to passthrough
		plugin = auth.NewPassthrough()
	default:
		// Default to passthrough
		plugin = auth.NewPassthrough()
	}

	return auth.NewMiddleware(plugin, logger.Logger)
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
  schema     Show generated GraphQL schema
  tools      List MCP tools generated from upstreams
  jwt        JWT token utilities (generate, validate, decode)
  version    Print version information
  help       Show this help message

Examples:
  tentaserve serve --config tentaserve.yaml
  tentaserve serve --port 9090
  tentaserve validate --config tentaserve.yaml
  tentaserve schema --upstream users-api
  tentaserve tools --format table
  tentaserve jwt generate --secret mysecret --subject user123
  tentaserve jwt validate --token eyJ... --secret mysecret`)
}
