package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/specvital/worker/internal/adapter/repository/postgres"
	"github.com/specvital/worker/internal/infra/buildinfo"
)

const defaultConcurrency = 5

// maskURL returns a sanitized URL for logging (hides credentials).
func maskURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "[invalid-url]"
	}

	host := parsed.Host
	if len(host) > 30 {
		host = host[:30] + "..."
	}

	userPart := ""
	if parsed.User != nil {
		userPart = parsed.User.Username() + ":****@"
	}

	return fmt.Sprintf("%s://%s%s/...", parsed.Scheme, userPart, host)
}

// registerParserVersion writes the current core module version to system_config.
// This allows tracking which parser version produced each analysis.
func registerParserVersion(ctx context.Context, pool *pgxpool.Pool) error {
	version := buildinfo.ExtractCoreVersion()
	if version == "unknown" {
		slog.Warn("parser version unknown, skipping registration")
		return nil
	}

	repo := postgres.NewSystemConfigRepository(pool)
	if err := repo.Upsert(ctx, postgres.ConfigKeyParserVersion, version); err != nil {
		return fmt.Errorf("upsert parser version: %w", err)
	}

	slog.Info("parser version registered",
		"version", version,
		"display", buildinfo.FormatVersionDisplay(version))
	return nil
}
