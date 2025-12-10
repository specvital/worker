package queue

import (
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

const (
	DefaultConcurrency     = 5
	DefaultShutdownTimeout = 30 * time.Second
)

type ServerConfig struct {
	Concurrency     int
	RedisURL        string
	ShutdownTimeout time.Duration
}

func NewServer(cfg ServerConfig) (*asynq.Server, error) {
	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}

	shutdownTimeout := cfg.ShutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = DefaultShutdownTimeout
	}

	opt, err := asynq.ParseRedisURI(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis URI: %w", err)
	}

	return asynq.NewServer(opt, asynq.Config{
		Concurrency:     concurrency,
		ShutdownTimeout: shutdownTimeout,
	}), nil
}

func NewServeMux() *asynq.ServeMux {
	return asynq.NewServeMux()
}
