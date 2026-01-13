// Package bootstrap provides application startup utilities for analyzer and scheduler services.
package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/specvital/worker/internal/app"
	"github.com/specvital/worker/internal/infra/buildinfo"
	"github.com/specvital/worker/internal/infra/db"
	infraqueue "github.com/specvital/worker/internal/infra/queue"
)

// AnalyzerConfig holds configuration for the analyzer service.
type AnalyzerConfig struct {
	ServiceName       string
	Concurrency       int
	ShutdownTimeout   time.Duration
	DatabaseURL       string
	EncryptionKey     string
	GeminiAPIKey      string
	GeminiPhase1Model string
	GeminiPhase2Model string
}

// Validate checks that required analyzer configuration fields are set.
func (c *AnalyzerConfig) Validate() error {
	if c.ServiceName == "" {
		return fmt.Errorf("service name is required")
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("database URL is required")
	}
	if c.EncryptionKey == "" {
		return fmt.Errorf("encryption key is required")
	}
	if c.GeminiAPIKey == "" {
		return fmt.Errorf("gemini API key is required")
	}
	return nil
}

// applyDefaults sets default values for optional analyzer configuration.
func (c *AnalyzerConfig) applyDefaults() {
	if c.Concurrency <= 0 {
		c.Concurrency = defaultConcurrency
	}
	if c.ShutdownTimeout <= 0 {
		c.ShutdownTimeout = infraqueue.DefaultShutdownTimeout
	}
}

// StartAnalyzer starts the analyzer service for queue processing.
// Analyzers consume tasks from PostgreSQL-based river queue and process them.
// Horizontal scaling is safe - multiple analyzer instances share the workload.
func StartAnalyzer(cfg AnalyzerConfig) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	cfg.applyDefaults()

	slog.Info("starting service", "name", cfg.ServiceName)
	slog.Info("config loaded", "database_url", maskURL(cfg.DatabaseURL))

	ctx := context.Background()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database connection: %w", err)
	}
	defer pool.Close()

	slog.Info("postgres connected")

	parserVersion := buildinfo.ExtractCoreVersion()
	if err := registerParserVersion(ctx, pool, parserVersion); err != nil {
		return fmt.Errorf("register parser version: %w", err)
	}

	container, err := app.NewAnalyzerContainer(ctx, app.ContainerConfig{
		EncryptionKey:     cfg.EncryptionKey,
		GeminiAPIKey:      cfg.GeminiAPIKey,
		GeminiPhase1Model: cfg.GeminiPhase1Model,
		GeminiPhase2Model: cfg.GeminiPhase2Model,
		ParserVersion:     parserVersion,
		Pool:              pool,
	})
	if err != nil {
		return fmt.Errorf("container: %w", err)
	}
	defer func() {
		if err := container.Close(); err != nil {
			slog.Error("failed to close container", "error", err)
		}
	}()

	srv, err := infraqueue.NewServer(ctx, infraqueue.ServerConfig{
		Pool:            pool,
		Concurrency:     cfg.Concurrency,
		ShutdownTimeout: cfg.ShutdownTimeout,
		Workers:         container.Workers,
	})
	if err != nil {
		return fmt.Errorf("queue server: %w", err)
	}

	slog.Info("analyzer starting", "concurrency", cfg.Concurrency)
	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("start server: %w", err)
	}
	slog.Info("analyzer ready", "concurrency", cfg.Concurrency)

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGTERM, syscall.SIGINT)

	sig := <-shutdown
	slog.Info("shutdown signal received", "signal", sig.String())

	if err := srv.Stop(ctx); err != nil {
		slog.Error("queue server stop error", "error", err)
	}
	slog.Info("queue server stopped")

	slog.Info("service shutdown complete", "name", cfg.ServiceName)
	return nil
}
