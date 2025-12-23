package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/specvital/collector/internal/domain/analysis"
	"github.com/specvital/collector/internal/infra/db"
)

var _ analysis.CodebaseRepository = (*CodebaseRepository)(nil)

type CodebaseRepository struct {
	pool *pgxpool.Pool
}

func NewCodebaseRepository(pool *pgxpool.Pool) *CodebaseRepository {
	return &CodebaseRepository{pool: pool}
}

func (r *CodebaseRepository) FindByExternalID(ctx context.Context, host, externalRepoID string) (*analysis.Codebase, error) {
	queries := db.New(r.pool)

	row, err := queries.FindCodebaseByExternalID(ctx, db.FindCodebaseByExternalIDParams{
		Host:           host,
		ExternalRepoID: externalRepoID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, analysis.ErrCodebaseNotFound
		}
		return nil, fmt.Errorf("find codebase by external ID: %w", err)
	}

	return mapCodebase(row), nil
}

func (r *CodebaseRepository) FindByOwnerName(ctx context.Context, host, owner, name string) (*analysis.Codebase, error) {
	queries := db.New(r.pool)

	row, err := queries.FindCodebaseByOwnerName(ctx, db.FindCodebaseByOwnerNameParams{
		Host:  host,
		Owner: owner,
		Name:  name,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, analysis.ErrCodebaseNotFound
		}
		return nil, fmt.Errorf("find codebase by owner/name: %w", err)
	}

	return mapCodebase(row), nil
}

func (r *CodebaseRepository) MarkStale(ctx context.Context, id analysis.UUID) error {
	queries := db.New(r.pool)

	err := queries.MarkCodebaseStale(ctx, toPgUUID(id))
	if err != nil {
		return fmt.Errorf("mark codebase stale: %w", err)
	}

	return nil
}

func (r *CodebaseRepository) UnmarkStale(ctx context.Context, id analysis.UUID, owner, name string) (*analysis.Codebase, error) {
	queries := db.New(r.pool)

	row, err := queries.UnmarkCodebaseStale(ctx, db.UnmarkCodebaseStaleParams{
		ID:    toPgUUID(id),
		Owner: owner,
		Name:  name,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, analysis.ErrCodebaseNotFound
		}
		return nil, fmt.Errorf("unmark codebase stale: %w", err)
	}

	return mapCodebase(row), nil
}

func (r *CodebaseRepository) UpdateOwnerName(ctx context.Context, id analysis.UUID, owner, name string) (*analysis.Codebase, error) {
	queries := db.New(r.pool)

	row, err := queries.UpdateCodebaseOwnerName(ctx, db.UpdateCodebaseOwnerNameParams{
		ID:    toPgUUID(id),
		Owner: owner,
		Name:  name,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, analysis.ErrCodebaseNotFound
		}
		return nil, fmt.Errorf("update codebase owner/name: %w", err)
	}

	return mapCodebase(row), nil
}

func mapCodebase(row db.Codebasis) *analysis.Codebase {
	return &analysis.Codebase{
		ExternalRepoID: row.ExternalRepoID,
		Host:           row.Host,
		ID:             fromPgUUID(row.ID),
		IsStale:        row.IsStale,
		Name:           row.Name,
		Owner:          row.Owner,
	}
}
