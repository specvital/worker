package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/specvital/collector/internal/repository"
	"github.com/specvital/core/pkg/parser"
	"github.com/specvital/core/pkg/source"
)

type AnalyzeRequest struct {
	Owner string
	Repo  string
}

func (r AnalyzeRequest) Validate() error {
	if r.Owner == "" {
		return fmt.Errorf("%w: owner is required", ErrInvalidInput)
	}
	if r.Repo == "" {
		return fmt.Errorf("%w: repo is required", ErrInvalidInput)
	}
	if len(r.Owner) > 39 || len(r.Repo) > 100 {
		return fmt.Errorf("%w: owner/repo exceeds length limit", ErrInvalidInput)
	}
	if !isValidGitHubName(r.Owner) || !isValidGitHubName(r.Repo) {
		return fmt.Errorf("%w: invalid characters in owner/repo", ErrInvalidInput)
	}
	return nil
}

func isValidGitHubName(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}

type AnalysisService interface {
	Analyze(ctx context.Context, req AnalyzeRequest) error
}

type analysisService struct {
	analysisRepo repository.AnalysisRepository
}

func NewAnalysisService(repo repository.AnalysisRepository) AnalysisService {
	return &analysisService{
		analysisRepo: repo,
	}
}

func (s *analysisService) Analyze(ctx context.Context, req AnalyzeRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}

	repoURL := fmt.Sprintf("https://github.com/%s/%s", req.Owner, req.Repo)

	gitSrc, err := source.NewGitSource(ctx, repoURL, nil)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCloneFailed, err)
	}
	defer gitSrc.Close()

	result, err := parser.Scan(ctx, gitSrc)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrScanFailed, err)
	}

	if result.Inventory == nil {
		slog.WarnContext(ctx, "scan result has no inventory",
			"owner", req.Owner,
			"repo", req.Repo,
			"commit", gitSrc.CommitSHA(),
		)
	}

	if err := s.analysisRepo.SaveAnalysisResult(ctx, repository.SaveAnalysisResultParams{
		Branch:    gitSrc.Branch(),
		CommitSHA: gitSrc.CommitSHA(),
		Owner:     req.Owner,
		Repo:      req.Repo,
		Result:    result,
	}); err != nil {
		return fmt.Errorf("%w: %w", ErrSaveFailed, err)
	}

	return nil
}
