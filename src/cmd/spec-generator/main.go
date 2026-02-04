package main

import (
	"log/slog"
	"os"

	"github.com/specvital/worker/internal/app/bootstrap"
	"github.com/specvital/worker/internal/infra/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	if !cfg.MockMode && cfg.GeminiAPIKey == "" {
		slog.Error("GEMINI_API_KEY is required for spec-generator (set MOCK_MODE=true to skip)")
		os.Exit(1)
	}

	if cfg.MockMode {
		slog.Info("starting in mock mode - AI calls will be simulated")
	}

	if err := bootstrap.StartSpecGenerator(bootstrap.SpecGeneratorConfig{
		ServiceName:       "spec-generator",
		DatabaseURL:       cfg.DatabaseURL,
		Fairness:          cfg.Fairness,
		GeminiAPIKey:      cfg.GeminiAPIKey,
		GeminiPhase1Model: cfg.GeminiPhase1Model,
		GeminiPhase2Model: cfg.GeminiPhase2Model,
		MockMode:          cfg.MockMode,
		QueueWorkers:      cfg.Queue.Specgen,
	}); err != nil {
		slog.Error("spec-generator failed", "error", err)
		os.Exit(1)
	}
}
