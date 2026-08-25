package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"social-hub/pkg/mcpserver"
	"social-hub/pkg/socialhub"
)

const version = "v0.1.0-alpha"

type commandConfig struct {
	configPath     string
	transport      string
	listen         string
	bearerTokenRef string
	allowedWrites  string
	allowedDeletes string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "social-hub-mcp:", err)
		os.Exit(1)
	}
}

func run() error {
	var command commandConfig
	flag.StringVar(&command.configPath, "config", envOr("SOCIAL_HUB_CONFIG", ""), "path to social-hub YAML or JSON configuration")
	flag.StringVar(&command.transport, "transport", envOr("SOCIAL_HUB_MCP_TRANSPORT", "stdio"), "MCP transport: stdio or http")
	flag.StringVar(&command.listen, "listen", envOr("SOCIAL_HUB_MCP_LISTEN", "127.0.0.1:8080"), "HTTP listen address")
	flag.StringVar(&command.bearerTokenRef, "bearer-token-ref", envOr("SOCIAL_HUB_MCP_BEARER_TOKEN_REF", ""), "env:// reference for HTTP Bearer authentication")
	flag.StringVar(&command.allowedWrites, "allow-write", envOr("SOCIAL_HUB_MCP_ALLOW_WRITE", ""), "comma-separated additive operations to expose")
	flag.StringVar(&command.allowedDeletes, "allow-destructive", envOr("SOCIAL_HUB_MCP_ALLOW_DESTRUCTIVE", ""), "comma-separated destructive operations to expose")
	flag.Parse()

	if command.configPath == "" {
		return errors.New("--config or SOCIAL_HUB_CONFIG is required")
	}
	configFile, err := os.Open(command.configPath)
	if err != nil {
		return fmt.Errorf("open config: %w", err)
	}
	config, err := socialhub.LoadConfig(configFile)
	closeErr := configFile.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return fmt.Errorf("close config: %w", closeErr)
	}

	httpTimeout, err := configuredTimeout(config.Defaults.Timeout)
	if err != nil {
		return err
	}
	policy := mcpserver.Policy{
		AllowedWrites:      parseOperations(command.allowedWrites),
		AllowedDestructive: parseOperations(command.allowedDeletes),
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	service, err := mcpserver.New(ctx, config, policy,
		socialhub.WithHTTPClient(&http.Client{Timeout: httpTimeout}),
		socialhub.WithLogger(logger),
	)
	if err != nil {
		return err
	}
	defer service.Close()
	protocolServer := service.MCPServer(version)

	switch strings.ToLower(command.transport) {
	case "stdio":
		if err := protocolServer.Run(ctx, &mcp.StdioTransport{}); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	case "http":
		return runHTTP(ctx, protocolServer, command.listen, command.bearerTokenRef, logger)
	default:
		return fmt.Errorf("unsupported transport %q; use stdio or http", command.transport)
	}
}

func configuredTimeout(raw string) (time.Duration, error) {
	if raw == "" {
		return 15 * time.Second, nil
	}
	timeout, err := time.ParseDuration(raw)
	if err != nil || timeout <= 0 || timeout > 5*time.Minute {
		return 0, errors.New("defaults.timeout must be a duration greater than zero and no more than 5m")
	}
	return timeout, nil
}

func parseOperations(raw string) []mcpserver.Operation {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	operations := make([]mcpserver.Operation, 0, len(parts))
	for _, part := range parts {
		if operation := strings.TrimSpace(part); operation != "" {
			operations = append(operations, mcpserver.Operation(operation))
		}
	}
	return operations
}

func runHTTP(ctx context.Context, protocolServer *mcp.Server, address, bearerTokenRef string, logger *slog.Logger) error {
	if address == "" {
		return errors.New("HTTP listen address is required")
	}
	if !isLoopbackAddress(address) && bearerTokenRef == "" {
		return errors.New("non-loopback HTTP listeners require --bearer-token-ref")
	}
	var bearerToken string
	if bearerTokenRef != "" {
		var err error
		bearerToken, err = (socialhub.EnvironmentSecretResolver{}).Resolve(ctx, bearerTokenRef)
		if err != nil {
			return fmt.Errorf("resolve MCP bearer token: %w", err)
		}
	}

	mcpHandler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return protocolServer },
		&mcp.StreamableHTTPOptions{
			Stateless:                    true,
			JSONResponse:                 true,
			MaxRequestBodyBytes:          1 << 20,
			PropagateRequestCancellation: true,
		},
	)
	originProtection := http.NewCrossOriginProtection()
	var handler http.Handler = originProtection.Handler(mcpHandler)
	if bearerToken != "" {
		handler = requireBearerToken(bearerToken, handler)
	}
	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	mux.HandleFunc("/healthz", func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	})

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", address, err)
	}
	defer listener.Close()
	httpServer := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- httpServer.Serve(listener)
	}()
	logger.Info("MCP HTTP server listening", "address", listener.Addr().String(), "path", "/mcp", "authenticated", bearerToken != "")

	select {
	case err := <-serveErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return err
		}
		err := <-serveErrors
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}

func requireBearerToken(expected string, next http.Handler) http.Handler {
	expectedBytes := []byte(expected)
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		const prefix = "Bearer "
		authorization := request.Header.Get("Authorization")
		provided := ""
		if strings.HasPrefix(authorization, prefix) {
			provided = strings.TrimPrefix(authorization, prefix)
		}
		if subtle.ConstantTimeCompare([]byte(provided), expectedBytes) != 1 {
			response.Header().Set("WWW-Authenticate", `Bearer realm="social-hub-mcp"`)
			http.Error(response, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(response, request)
	})
}

func isLoopbackAddress(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func envOr(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return fallback
}
