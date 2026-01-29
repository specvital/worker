package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/specvital/worker/internal/adapter/ai/gemini"
	"github.com/specvital/worker/internal/adapter/ai/gemini/batch"
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
	BatchProvider  *batch.Provider
	Middleware     []rivertype.WorkerMiddleware
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

	// Build UseCase options
	var usecaseOpts []specviewuc.Option
	if cfg.SpecView.UseBatchAPI {
		usecaseOpts = append(usecaseOpts, specviewuc.WithBatchConfig(specviewuc.BatchConfig{
			BatchThreshold: cfg.SpecView.BatchThreshold,
			UseBatchAPI:    cfg.SpecView.UseBatchAPI,
		}))
	}

	specViewUC := specviewuc.NewGenerateSpecViewUseCase(
		specDocRepo,
		aiProvider,
		defaultModelID,
		usecaseOpts...,
	)

	// Create Batch Provider (only when Batch API is enabled and not in mock mode)
	var batchProvider *batch.Provider
	if cfg.SpecView.UseBatchAPI && !cfg.MockMode {
		phase1Model := cfg.GeminiPhase1Model
		if phase1Model == "" {
			phase1Model = "gemini-2.5-flash"
		}

		var batchErr error
		batchProvider, batchErr = batch.NewProvider(ctx, batch.BatchConfig{
			APIKey:         cfg.GeminiAPIKey,
			BatchThreshold: cfg.SpecView.BatchThreshold,
			Phase1Model:    phase1Model,
			PollInterval:   cfg.SpecView.BatchPollInterval,
			UseBatchAPI:    cfg.SpecView.UseBatchAPI,
		})
		if batchErr != nil {
			return nil, fmt.Errorf("create batch provider: %w", batchErr)
		}
		slog.Info("batch API enabled for SpecView worker",
			"poll_interval", cfg.SpecView.BatchPollInterval,
			"batch_threshold", cfg.SpecView.BatchThreshold,
		)
	}

	// Create Worker (with or without Batch support)
	var specViewWorker *specviewqueue.Worker
	if batchProvider != nil {
		workerConfig := specviewqueue.WorkerConfig{
			BatchPollInterval: cfg.SpecView.BatchPollInterval,
			UseBatchAPI:       cfg.SpecView.UseBatchAPI,
		}
		specViewWorker = specviewqueue.NewWorkerWithBatch(
			specViewUC,
			batchProvider,
			specDocRepo,
			workerConfig,
		)
	} else {
		specViewWorker = specviewqueue.NewWorker(specViewUC)
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, specViewWorker)

	queueClient, err := infraqueue.NewClient(ctx, cfg.Pool)
	if err != nil {
		return nil, fmt.Errorf("create queue client: %w", err)
	}

	var middleware []rivertype.WorkerMiddleware
	fm, err := NewFairnessMiddleware(cfg.Fairness)
	if err != nil {
		return nil, fmt.Errorf("create fairness middleware: %w", err)
	}
	if fm != nil {
		middleware = append(middleware, fm)
	}

	return &SpecGeneratorContainer{
		AIProvider:     aiProvider,
		BatchProvider:  batchProvider,
		Middleware:     middleware,
		QueueClient:    queueClient,
		SpecViewWorker: specViewWorker,
		Workers:        workers,
	}, nil
}

// Close releases container resources.
func (c *SpecGeneratorContainer) Close() error {
	var errs []error

	if c.BatchProvider != nil {
		if err := c.BatchProvider.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close batch provider: %w", err))
		}
	}

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
