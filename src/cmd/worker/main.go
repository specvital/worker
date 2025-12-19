package main

import (
	"log/slog"
	"os"

	"github.com/specvital/collector/internal/app/bootstrap"
	"github.com/specvital/collector/internal/infra/config"

	_ "github.com/specvital/core/pkg/parser/strategies/all"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	if err := bootstrap.StartWorker(bootstrap.WorkerConfig{
		ServiceName:   "worker",
		DatabaseURL:   cfg.DatabaseURL,
		EncryptionKey: cfg.EncryptionKey,
	}); err != nil {
		slog.Error("worker failed", "error", err)
		os.Exit(1)
	}
}
