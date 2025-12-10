package mocks

import (
	"context"

	"github.com/specvital/collector/internal/repository"
)

type MockAnalysisRepository struct {
	SaveAnalysisResultFunc func(ctx context.Context, params repository.SaveAnalysisResultParams) error
}

func (m *MockAnalysisRepository) SaveAnalysisResult(ctx context.Context, params repository.SaveAnalysisResultParams) error {
	if m.SaveAnalysisResultFunc != nil {
		return m.SaveAnalysisResultFunc(ctx, params)
	}
	return nil
}
