package app

import (
	"context"
	"fmt"

	"github.com/specvital/worker/internal/adapter/repository/postgres"
	"github.com/specvital/worker/internal/adapter/vcs"
	handlerscheduler "github.com/specvital/worker/internal/handler/scheduler"
	infraqueue "github.com/specvital/worker/internal/infra/queue"
	infrascheduler "github.com/specvital/worker/internal/infra/scheduler"
	"github.com/specvital/worker/internal/usecase/autorefresh"
)

// SchedulerContainer holds dependencies for the scheduler service.
type SchedulerContainer struct {
	AutoRefreshHandler *handlerscheduler.AutoRefreshHandler
	Scheduler          *infrascheduler.Scheduler
	queueClient        *infraqueue.Client
	schedulerLock      *infrascheduler.DistributedLock
}

// NewSchedulerContainer creates and initializes a new scheduler container with all required dependencies.
func NewSchedulerContainer(ctx context.Context, cfg ContainerConfig) (*SchedulerContainer, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid container config: %w", err)
	}

	analysisRepo := postgres.NewAnalysisRepository(cfg.Pool)
	systemConfigRepo := postgres.NewSystemConfigRepository(cfg.Pool)

	queueClient, err := infraqueue.NewClient(ctx, cfg.Pool)
	if err != nil {
		return nil, fmt.Errorf("create queue client: %w", err)
	}

	schedulerLock := infrascheduler.NewDistributedLock(cfg.Pool, schedulerLockKey)

	gitVCS := vcs.NewGitVCS()
	autoRefreshUC := autorefresh.NewAutoRefreshUseCase(analysisRepo, queueClient, gitVCS, systemConfigRepo)
	autoRefreshHandler := handlerscheduler.NewAutoRefreshHandler(autoRefreshUC, schedulerLock)

	scheduler := infrascheduler.New()

	return &SchedulerContainer{
		AutoRefreshHandler: autoRefreshHandler,
		Scheduler:          scheduler,
		queueClient:        queueClient,
		schedulerLock:      schedulerLock,
	}, nil
}

// Close releases container resources.
func (c *SchedulerContainer) Close() error {
	var errs []error

	if c.schedulerLock != nil {
		if err := c.schedulerLock.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close scheduler lock: %w", err))
		}
	}

	if c.queueClient != nil {
		if err := c.queueClient.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close queue client: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("close scheduler container: %v", errs)
	}
	return nil
}
