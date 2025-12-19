package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/specvital/collector/internal/app"
	"github.com/specvital/collector/internal/infra/db"
)

const (
	autoRefreshSchedule      = "@every 1h"
	schedulerShutdownTimeout = 30 * time.Second
)

type SchedulerConfig struct {
	ServiceName     string
	DatabaseURL     string
	ShutdownTimeout time.Duration
}

func (c *SchedulerConfig) Validate() error {
	if c.ServiceName == "" {
		return fmt.Errorf("service name is required")
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("database URL is required")
	}
	return nil
}

func (c *SchedulerConfig) applyDefaults() {
	if c.ShutdownTimeout <= 0 {
		c.ShutdownTimeout = schedulerShutdownTimeout
	}
}

// StartScheduler starts the scheduler service with cron-based jobs.
//
// Uses PostgreSQL advisory lock-based distributed locking to prevent duplicate execution
// when multiple scheduler instances are deployed (e.g., during blue-green deployment).
func StartScheduler(cfg SchedulerConfig) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	cfg.applyDefaults()

	slog.Info("starting service", "name", cfg.ServiceName)
	slog.Info("config loaded", "database_url", maskURL(cfg.DatabaseURL))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database connection: %w", err)
	}
	defer pool.Close()

	slog.Info("postgres connected")

	container, err := app.NewSchedulerContainer(ctx, app.ContainerConfig{
		Pool: pool,
	})
	if err != nil {
		return fmt.Errorf("container: %w", err)
	}
	defer func() {
		if err := container.Close(); err != nil {
			slog.Error("failed to close container", "error", err)
		}
	}()

	if err := container.Scheduler.AddFunc(autoRefreshSchedule, func() {
		container.AutoRefreshHandler.RunWithContext(ctx)
	}); err != nil {
		return fmt.Errorf("add auto-refresh schedule: %w", err)
	}

	container.Scheduler.Start()
	slog.Info("scheduler started", "schedule", autoRefreshSchedule)

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGTERM, syscall.SIGINT)

	sig := <-shutdown
	slog.Info("shutdown signal received", "signal", sig.String())

	cancel()

	if err := container.Scheduler.StopWithTimeout(cfg.ShutdownTimeout); err != nil {
		slog.Warn("scheduler shutdown timeout", "error", err)
	}
	slog.Info("scheduler stopped")

	slog.Info("service shutdown complete", "name", cfg.ServiceName)
	return nil
}
