package app

import (
	"context"
	"fmt"

	"github.com/riverqueue/river"
	"github.com/specvital/core/pkg/crypto"
	"github.com/specvital/worker/internal/adapter/parser"
	"github.com/specvital/worker/internal/adapter/queue/analyze"
	"github.com/specvital/worker/internal/adapter/repository/postgres"
	"github.com/specvital/worker/internal/adapter/vcs"
	infraqueue "github.com/specvital/worker/internal/infra/queue"
	uc "github.com/specvital/worker/internal/usecase/analysis"
)

// AnalyzerContainer holds dependencies for the analyzer worker service.
type AnalyzerContainer struct {
	AnalyzeWorker *analyze.AnalyzeWorker
	Workers       *river.Workers
	QueueClient   *infraqueue.Client
}

// NewAnalyzerContainer creates and initializes a new analyzer container with all required dependencies.
func NewAnalyzerContainer(ctx context.Context, cfg ContainerConfig) (*AnalyzerContainer, error) {
	if err := cfg.ValidateAnalyzer(); err != nil {
		return nil, fmt.Errorf("invalid container config: %w", err)
	}

	encryptor, err := crypto.NewEncryptorFromBase64(cfg.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("create encryptor: %w", err)
	}

	analysisRepo := postgres.NewAnalysisRepository(cfg.Pool)
	codebaseRepo := postgres.NewCodebaseRepository(cfg.Pool)
	userRepo := postgres.NewUserRepository(cfg.Pool, encryptor)
	gitVCS := vcs.NewGitVCS()
	githubAPIClient := vcs.NewGitHubAPIClient(nil)
	coreParser := parser.NewCoreParser()
	analyzeUC := uc.NewAnalyzeUseCase(
		analysisRepo, codebaseRepo, gitVCS, githubAPIClient, coreParser, userRepo,
		uc.WithParserVersion(cfg.ParserVersion),
	)
	analyzeWorker := analyze.NewAnalyzeWorker(analyzeUC)

	workers := river.NewWorkers()
	river.AddWorker(workers, analyzeWorker)

	queueClient, err := infraqueue.NewClient(ctx, cfg.Pool)
	if err != nil {
		return nil, fmt.Errorf("create queue client: %w", err)
	}

	return &AnalyzerContainer{
		AnalyzeWorker: analyzeWorker,
		Workers:       workers,
		QueueClient:   queueClient,
	}, nil
}

// Close releases container resources.
// Uses error accumulation pattern to ensure all resources are cleaned up.
func (c *AnalyzerContainer) Close() error {
	var errs []error

	if c.QueueClient != nil {
		if err := c.QueueClient.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close queue client: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("close analyzer container: %v", errs)
	}
	return nil
}
