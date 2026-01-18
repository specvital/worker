package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/specvital/worker/internal/adapter/queue/specview"
	"github.com/specvital/worker/internal/app"
	"github.com/specvital/worker/internal/infra/config"
	"github.com/specvital/worker/internal/infra/db"
	infraqueue "github.com/specvital/worker/internal/infra/queue"
)

// SpecGeneratorConfig holds configuration for the spec-generator service.
type SpecGeneratorConfig struct {
	ServiceName       string
	Concurrency       int               // Deprecated: Use QueueWorkers instead
	ShutdownTimeout   time.Duration
	DatabaseURL       string
	GeminiAPIKey      string
	GeminiPhase1Model string
	GeminiPhase2Model string
	QueueWorkers      config.QueueWorkers // Worker allocation per queue
}

// Validate checks that required spec-generator configuration fields are set.
func (c *SpecGeneratorConfig) Validate() error {
	if c.ServiceName == "" {
		return fmt.Errorf("service name is required")
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("database URL is required")
	}
	if c.GeminiAPIKey == "" {
		return fmt.Errorf("gemini API key is required")
	}
	return nil
}

// applyDefaults sets default values for optional spec-generator configuration.
func (c *SpecGeneratorConfig) applyDefaults() {
	if c.Concurrency <= 0 {
		c.Concurrency = defaultConcurrency
	}
	if c.ShutdownTimeout <= 0 {
		c.ShutdownTimeout = infraqueue.DefaultShutdownTimeout
	}
}

// StartSpecGenerator starts the spec-generator service for queue processing.
// Spec-generators consume specview:generate tasks and process them using Gemini AI.
// Horizontal scaling is safe - multiple spec-generator instances share the workload.
func StartSpecGenerator(cfg SpecGeneratorConfig) error {
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

	container, err := app.NewSpecGeneratorContainer(ctx, app.ContainerConfig{
		GeminiAPIKey:      cfg.GeminiAPIKey,
		GeminiPhase1Model: cfg.GeminiPhase1Model,
		GeminiPhase2Model: cfg.GeminiPhase2Model,
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

	queues := buildSpecGeneratorQueues(cfg.QueueWorkers, cfg.Concurrency)
	srv, err := infraqueue.NewServer(ctx, infraqueue.ServerConfig{
		Pool:            pool,
		Queues:          queues,
		ShutdownTimeout: cfg.ShutdownTimeout,
		Workers:         container.Workers,
	})
	if err != nil {
		return fmt.Errorf("queue server: %w", err)
	}

	logQueueSubscription("spec-generator", queues)
	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("start server: %w", err)
	}
	slog.Info("spec-generator ready")

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

// buildSpecGeneratorQueues creates queue allocations for spec-generator service.
func buildSpecGeneratorQueues(qw config.QueueWorkers, legacyConcurrency int) []infraqueue.QueueAllocation {
	// If QueueWorkers is zero-valued, fall back to single default queue
	if qw.Priority == 0 && qw.Default == 0 && qw.Scheduled == 0 {
		concurrency := legacyConcurrency
		if concurrency <= 0 {
			concurrency = defaultConcurrency
		}
		return []infraqueue.QueueAllocation{
			{Name: specview.QueueDefault, MaxWorkers: concurrency},
		}
	}

	return []infraqueue.QueueAllocation{
		{Name: specview.QueuePriority, MaxWorkers: qw.Priority},
		{Name: specview.QueueDefault, MaxWorkers: qw.Default},
		{Name: specview.QueueScheduled, MaxWorkers: qw.Scheduled},
	}
}
