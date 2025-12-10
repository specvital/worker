package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/specvital/collector/internal/config"
	"github.com/specvital/collector/internal/db"
	"github.com/specvital/collector/internal/jobs"
	"github.com/specvital/collector/internal/queue"
	"github.com/specvital/collector/internal/repository"
	"github.com/specvital/collector/internal/service"
)

const (
	workerConcurrency = 5
	shutdownTimeout   = 30 * time.Second
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

	slog.Info("postgres connected")

	srv, err := queue.NewServer(queue.ServerConfig{
		RedisURL:    cfg.RedisURL,
		Concurrency: workerConcurrency,
	})
	if err != nil {
		slog.Error("failed to create queue server", "error", err)
		pool.Close()
		os.Exit(1)
	}

	analysisRepo := repository.NewPostgresAnalysisRepository(pool)
	analysisSvc := service.NewAnalysisService(analysisRepo)

	mux := queue.NewServeMux()
	analyzeHandler := jobs.NewAnalyzeHandler(analysisSvc)
	mux.HandleFunc(jobs.TypeAnalyze, analyzeHandler.ProcessTask)

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGTERM, syscall.SIGINT)

	errChan := make(chan error, 1)
	go func() {
		slog.Info("worker ready", "concurrency", workerConcurrency)
		if err := srv.Run(mux); err != nil {
			errChan <- err
		}
	}()

	select {
	case sig := <-shutdown:
		slog.Info("shutdown signal received", "signal", sig.String())
	case err := <-errChan:
		slog.Error("worker failed", "error", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	srv.Shutdown()
	<-shutdownCtx.Done()

	pool.Close()
	slog.Info("collector shutdown complete")
}

func maskURL(url string) string {
	if len(url) > 20 {
		return url[:20] + "..."
	}
	return url
}
