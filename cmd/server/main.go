package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/QuantumNous/new-api-mcp-server/internal/client"
	"github.com/QuantumNous/new-api-mcp-server/internal/config"
	"github.com/QuantumNous/new-api-mcp-server/internal/handler"
	"github.com/QuantumNous/new-api-mcp-server/internal/observability"
	openapipkg "github.com/QuantumNous/new-api-mcp-server/internal/openapi"
	"github.com/QuantumNous/new-api-mcp-server/internal/registry"
	embeddedSpecs "github.com/QuantumNous/new-api-mcp-server/openapi"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/prometheus/client_golang/prometheus"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	observability.SetupLogging(cfg.LogLevel, cfg.LogFormat, cfg.LogConsoleEnabled, cfg.OTLPEndpoint, nil)

	slog.Info("starting new-api-mcp-server", "version", version, "transport", cfg.Transport)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdownTracing, err := observability.SetupTracing(ctx, cfg.OTLPEndpoint, cfg.ServiceName)
	if err != nil {
		return fmt.Errorf("setup tracing: %w", err)
	}
	defer shutdownTracing(context.Background())

	promRegistry := prometheus.NewRegistry()
	metrics := observability.NewMetrics(promRegistry)

	metricsMux := http.NewServeMux()
	metricsMux.Handle(cfg.MetricsPath, observability.Handler(promRegistry))
	metricsServer := &http.Server{
		Addr:    cfg.MetricsAddr,
		Handler: metricsMux,
	}

	go func() {
		slog.Info("metrics server starting", "addr", cfg.MetricsAddr, "path", cfg.MetricsPath)
		if err := metricsServer.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("metrics server error", "error", err)
		}
	}()

	relayClient := client.New(cfg.BaseURL, cfg.APIKey, cfg.SystemKey, cfg.Timeout)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "new-api-mcp-server",
		Version: version,
	}, nil)

	// Register relay tools
	if cfg.APIKey != "" {
		relayDefs, err := openapipkg.Parse(embeddedSpecs.RelaySpec)
		if err != nil {
			return fmt.Errorf("parse relay spec: %w", err)
		}

		if err := registry.ValidateUniqueNames(relayDefs, ""); err != nil {
			return fmt.Errorf("relay tools: %w", err)
		}

		relayHandler := handler.New(relayClient, client.SourceRelay, metrics)
		count := registry.RegisterTools(server, relayDefs, registry.Options{
			DisabledGroups: cfg.RelayDisabledGroups,
			NamePrefix:     "",
		}, relayHandler.MakeHandler)

		slog.Info("registered relay tools", "count", count)
	} else {
		slog.Warn("NEW_API_KEY not set, relay tools disabled")
	}

	// Register API tools
	if cfg.SystemKey != "" && cfg.APIToolsEnabled {
		apiDefs, err := openapipkg.Parse(embeddedSpecs.APISpec)
		if err != nil {
			return fmt.Errorf("parse api spec: %w", err)
		}

		if err := registry.ValidateUniqueNames(apiDefs, "api_"); err != nil {
			return fmt.Errorf("api tools: %w", err)
		}

		apiHandler := handler.New(relayClient, client.SourceAPI, metrics)
		count := registry.RegisterTools(server, apiDefs, registry.Options{
			NamePrefix: "api_",
		}, apiHandler.MakeHandler)

		slog.Info("registered api tools", "count", count)
	} else {
		slog.Info("API tools disabled")
	}

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		slog.Info("received signal, shutting down", "signal", sig)

		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := metricsServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("metrics server shutdown failed", "error", err)
		}

		slog.Info("graceful shutdown complete")
	}()

	// Run transport
	switch cfg.Transport {
	case "http":
		slog.Info("starting HTTP transport", "addr", cfg.HTTPAddr)
		httpHandler := mcp.NewStreamableHTTPHandler(
			func(r *http.Request) *mcp.Server { return server },
			nil,
		)
		httpServer := &http.Server{
			Addr:    cfg.HTTPAddr,
			Handler: httpHandler,
		}
		go func() {
			<-ctx.Done()
			httpServer.Shutdown(context.Background())
		}()
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			return fmt.Errorf("http server: %w", err)
		}
	default:
		slog.Info("starting stdio transport")
		if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
			return fmt.Errorf("stdio server: %w", err)
		}
	}

	return nil
}
