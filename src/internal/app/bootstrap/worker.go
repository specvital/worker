// Package bootstrap provides application startup utilities for worker services.
package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/specvital/collector/internal/app"
	"github.com/specvital/collector/internal/infra/db"
	infraqueue "github.com/specvital/collector/internal/infra/queue"
)

const defaultConcurrency = 5

type WorkerConfig struct {
	ServiceName     string
	Concurrency     int
	ShutdownTimeout time.Duration
	DatabaseURL     string
	EncryptionKey   string
}

func (c *WorkerConfig) Validate() error {
	if c.ServiceName == "" {
		return fmt.Errorf("service name is required")
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("database URL is required")
	}
	if c.EncryptionKey == "" {
		return fmt.Errorf("encryption key is required")
	}
	return nil
}

func (c *WorkerConfig) applyDefaults() {
	if c.Concurrency <= 0 {
		c.Concurrency = defaultConcurrency
	}
	if c.ShutdownTimeout <= 0 {
		c.ShutdownTimeout = infraqueue.DefaultShutdownTimeout
	}
}

// StartWorker starts the worker service for queue processing.
// Workers consume tasks from PostgreSQL-based river queue and process them.
// Horizontal scaling is safe - multiple worker instances share the workload.
func StartWorker(cfg WorkerConfig) error {
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

	container, err := app.NewWorkerContainer(ctx, app.ContainerConfig{
		EncryptionKey: cfg.EncryptionKey,
		Pool:          pool,
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

	slog.Info("worker starting", "concurrency", cfg.Concurrency)
	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("start server: %w", err)
	}
	slog.Info("worker ready", "concurrency", cfg.Concurrency)

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

func maskURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "[invalid-url]"
	}

	host := parsed.Host
	if len(host) > 30 {
		host = host[:30] + "..."
	}

	userPart := ""
	if parsed.User != nil {
		userPart = parsed.User.Username() + ":****@"
	}

	return fmt.Sprintf("%s://%s%s/...", parsed.Scheme, userPart, host)
}
