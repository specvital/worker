package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/riverqueue/river"
	"github.com/specvital/worker/internal/adapter/ai/gemini"
	"github.com/specvital/worker/internal/adapter/ai/mock"
	specviewqueue "github.com/specvital/worker/internal/adapter/queue/specview"
	"github.com/specvital/worker/internal/adapter/repository/postgres"
	"github.com/specvital/worker/internal/domain/specview"
	infraqueue "github.com/specvital/worker/internal/infra/queue"
	specviewuc "github.com/specvital/worker/internal/usecase/specview"
)

// SpecGeneratorContainer holds dependencies for the spec-generator worker service.
type SpecGeneratorContainer struct {
	AIProvider     specview.AIProvider
	QueueClient    *infraqueue.Client
	SpecViewWorker *specviewqueue.Worker
	Workers        *river.Workers
}

// NewSpecGeneratorContainer creates and initializes a new spec-generator container with all required dependencies.
func NewSpecGeneratorContainer(ctx context.Context, cfg ContainerConfig) (*SpecGeneratorContainer, error) {
	if err := cfg.ValidateSpecGenerator(); err != nil {
		return nil, fmt.Errorf("invalid container config: %w", err)
	}

	var aiProvider specview.AIProvider
	var defaultModelID string

	if cfg.MockMode {
		slog.Info("mock mode enabled, using mock AI provider")
		aiProvider = mock.NewProvider()
		defaultModelID = "mock-model"
	} else {
		geminiProvider, err := gemini.NewProvider(ctx, gemini.Config{
			APIKey:      cfg.GeminiAPIKey,
			Phase1Model: cfg.GeminiPhase1Model,
			Phase2Model: cfg.GeminiPhase2Model,
		})
		if err != nil {
			return nil, fmt.Errorf("create gemini provider: %w", err)
		}
		aiProvider = geminiProvider
		defaultModelID = cfg.GeminiPhase1Model
		if defaultModelID == "" {
			defaultModelID = "gemini-2.5-flash"
		}
	}

	specDocRepo := postgres.NewSpecDocumentRepository(cfg.Pool)
	specViewUC := specviewuc.NewGenerateSpecViewUseCase(
		specDocRepo,
		aiProvider,
		defaultModelID,
	)
	specViewWorker := specviewqueue.NewWorker(specViewUC)

	workers := river.NewWorkers()
	river.AddWorker(workers, specViewWorker)

	queueClient, err := infraqueue.NewClient(ctx, cfg.Pool)
	if err != nil {
		return nil, fmt.Errorf("create queue client: %w", err)
	}

	return &SpecGeneratorContainer{
		AIProvider:     aiProvider,
		QueueClient:    queueClient,
		SpecViewWorker: specViewWorker,
		Workers:        workers,
	}, nil
}

// Close releases container resources.
func (c *SpecGeneratorContainer) Close() error {
	var errs []error

	if c.QueueClient != nil {
		if err := c.QueueClient.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close queue client: %w", err))
		}
	}

	if c.AIProvider != nil {
		if err := c.AIProvider.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close AI provider: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("close spec-generator container: %v", errs)
	}
	return nil
}
