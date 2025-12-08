package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/specvital/collector/internal/config"
	"github.com/specvital/collector/internal/db"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("starting collector")

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Info("config loaded",
		"database_url", maskURL(cfg.DatabaseURL),
		"redis_url", maskURL(cfg.RedisURL),
	)

	ctx := context.Background()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	slog.Info("postgres connected")

	// pool will be used by sqlc Queries in Commit 4
	_ = pool

	slog.Info("collector initialized")
}

func maskURL(url string) string {
	if len(url) > 20 {
		return url[:20] + "..."
	}
	return url
}
