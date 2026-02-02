package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/specvital/worker/internal/domain/retention"
	"github.com/specvital/worker/internal/infra/db"
)

// RetentionRepository implements retention.CleanupRepository for PostgreSQL.
type RetentionRepository struct {
	pool *pgxpool.Pool
}

// NewRetentionRepository creates a new RetentionRepository.
func NewRetentionRepository(pool *pgxpool.Pool) *RetentionRepository {
	return &RetentionRepository{pool: pool}
}

// DeleteExpiredUserAnalysisHistory removes user analysis history records
// that have exceeded their retention period.
func (r *RetentionRepository) DeleteExpiredUserAnalysisHistory(ctx context.Context, batchSize int) (retention.DeleteResult, error) {
	if batchSize <= 0 {
		batchSize = retention.DefaultBatchSize
	}

	queries := db.New(r.pool)
	deleted, err := queries.DeleteExpiredUserAnalysisHistory(ctx, int32(batchSize))
	if err != nil {
		return retention.DeleteResult{}, fmt.Errorf("delete expired user analysis history: %w", err)
	}

	return retention.DeleteResult{DeletedCount: deleted}, nil
}

// DeleteExpiredSpecDocuments removes spec documents
// that have exceeded their retention period.
func (r *RetentionRepository) DeleteExpiredSpecDocuments(ctx context.Context, batchSize int) (retention.DeleteResult, error) {
	if batchSize <= 0 {
		batchSize = retention.DefaultBatchSize
	}

	queries := db.New(r.pool)
	deleted, err := queries.DeleteExpiredSpecDocuments(ctx, int32(batchSize))
	if err != nil {
		return retention.DeleteResult{}, fmt.Errorf("delete expired spec documents: %w", err)
	}

	return retention.DeleteResult{DeletedCount: deleted}, nil
}

// DeleteOrphanedAnalyses removes analyses that have no references
// in user_analysis_history.
func (r *RetentionRepository) DeleteOrphanedAnalyses(ctx context.Context, batchSize int) (retention.DeleteResult, error) {
	if batchSize <= 0 {
		batchSize = retention.DefaultBatchSize
	}

	queries := db.New(r.pool)
	deleted, err := queries.DeleteOrphanedAnalyses(ctx, int32(batchSize))
	if err != nil {
		return retention.DeleteResult{}, fmt.Errorf("delete orphaned analyses: %w", err)
	}

	return retention.DeleteResult{DeletedCount: deleted}, nil
}

// Compile-time interface check
var _ retention.CleanupRepository = (*RetentionRepository)(nil)
