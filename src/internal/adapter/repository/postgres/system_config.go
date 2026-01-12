package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/specvital/worker/internal/infra/db"
)

// ErrConfigNotFound is returned when a config key does not exist.
var ErrConfigNotFound = errors.New("config not found")

// Well-known config keys
const (
	ConfigKeyParserVersion = "parser_version"
)

// SystemConfigRepository provides access to the system_config table.
type SystemConfigRepository struct {
	pool *pgxpool.Pool
}

// NewSystemConfigRepository creates a new SystemConfigRepository.
func NewSystemConfigRepository(pool *pgxpool.Pool) *SystemConfigRepository {
	return &SystemConfigRepository{pool: pool}
}

// Get retrieves a config value by key.
// Returns ErrConfigNotFound if the key does not exist.
func (r *SystemConfigRepository) Get(ctx context.Context, key string) (string, error) {
	queries := db.New(r.pool)

	value, err := queries.GetSystemConfig(ctx, key)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrConfigNotFound
		}
		return "", fmt.Errorf("get system config %q: %w", key, err)
	}

	return value, nil
}

// GetCurrentParserVersion retrieves the current parser version from system_config.
// Implements analysis.ParserVersionProvider interface.
func (r *SystemConfigRepository) GetCurrentParserVersion(ctx context.Context) (string, error) {
	return r.Get(ctx, ConfigKeyParserVersion)
}

// Upsert inserts or updates a config value.
func (r *SystemConfigRepository) Upsert(ctx context.Context, key, value string) error {
	if key == "" {
		return fmt.Errorf("upsert system config: key cannot be empty")
	}

	queries := db.New(r.pool)

	err := queries.UpsertSystemConfig(ctx, db.UpsertSystemConfigParams{
		Key:   key,
		Value: value,
	})
	if err != nil {
		return fmt.Errorf("upsert system config %q: %w", key, err)
	}

	return nil
}
