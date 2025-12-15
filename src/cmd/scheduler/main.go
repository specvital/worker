package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const healthCheckInterval = 30 * time.Second

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("scheduler service starting (placeholder mode)")
	slog.Warn("scheduler functionality not yet implemented",
		"see", "WORK-auto-collection-service",
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runHealthCheck(ctx)

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGTERM, syscall.SIGINT)

	sig := <-shutdown
	slog.Info("shutdown signal received", "signal", sig.String())
	cancel()

	slog.Info("scheduler shutdown complete")
}

func runHealthCheck(ctx context.Context) {
	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			slog.Info("scheduler health check", "status", "idle")
		}
	}
}
