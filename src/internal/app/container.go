// Package app provides the application container for dependency injection.
// It serves as the composition root, wiring all dependencies together.
package app

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/specvital/collector/internal/adapter/parser"
	"github.com/specvital/collector/internal/adapter/repository/postgres"
	"github.com/specvital/collector/internal/adapter/vcs"
	"github.com/specvital/collector/internal/handler/queue"
	uc "github.com/specvital/collector/internal/usecase/analysis"
)

type ContainerConfig struct {
	Pool *pgxpool.Pool
}

func (c ContainerConfig) Validate() error {
	if c.Pool == nil {
		return fmt.Errorf("pool is required")
	}
	return nil
}

type Container struct {
	AnalyzeHandler *queue.AnalyzeHandler
}

func NewContainer(cfg ContainerConfig) (*Container, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid container config: %w", err)
	}

	analysisRepo := postgres.NewAnalysisRepository(cfg.Pool)
	gitVCS := vcs.NewGitVCS()
	coreParser := parser.NewCoreParser()
	analyzeUC := uc.NewAnalyzeUseCase(analysisRepo, gitVCS, coreParser)
	analyzeHandler := queue.NewAnalyzeHandler(analyzeUC)

	return &Container{
		AnalyzeHandler: analyzeHandler,
	}, nil
}
