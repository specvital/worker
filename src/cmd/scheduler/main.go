package main

import (
	"log/slog"
	"os"

	"github.com/specvital/collector/internal/app/bootstrap"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := bootstrap.SchedulerConfig{
		ServiceName: "scheduler",
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}

	if err := bootstrap.StartScheduler(cfg); err != nil {
		slog.Error("scheduler failed", "error", err)
		os.Exit(1)
	}
}
