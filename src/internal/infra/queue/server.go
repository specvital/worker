package queue

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

const (
	DefaultConcurrency     = 5
	DefaultShutdownTimeout = 30 * time.Second
)

// QueueAllocation defines worker count for a specific queue.
type QueueAllocation struct {
	Name       string
	MaxWorkers int
}

type ServerConfig struct {
	Pool            *pgxpool.Pool
	Queues          []QueueAllocation // Multi-queue configuration (preferred)
	Concurrency     int               // Deprecated: Use Queues instead. Kept for backward compatibility.
	QueueName       string            // Deprecated: Use Queues instead. Kept for backward compatibility.
	ShutdownTimeout time.Duration
	Workers         *river.Workers
}

type Server struct {
	client          *river.Client[pgx.Tx]
	shutdownTimeout time.Duration
}

func NewServer(ctx context.Context, cfg ServerConfig) (*Server, error) {
	shutdownTimeout := cfg.ShutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = DefaultShutdownTimeout
	}

	queues := buildQueueConfig(cfg)

	client, err := river.NewClient(riverpgxv5.New(cfg.Pool), &river.Config{
		Queues:  queues,
		Workers: cfg.Workers,
	})
	if err != nil {
		return nil, err
	}

	return &Server{
		client:          client,
		shutdownTimeout: shutdownTimeout,
	}, nil
}

// buildQueueConfig creates River queue configuration from ServerConfig.
// If Queues is set, uses multi-queue mode; otherwise falls back to legacy single-queue mode.
func buildQueueConfig(cfg ServerConfig) map[string]river.QueueConfig {
	if len(cfg.Queues) > 0 {
		queues := make(map[string]river.QueueConfig, len(cfg.Queues))
		for _, q := range cfg.Queues {
			name := q.Name
			if name == "" {
				name = river.QueueDefault
			}
			maxWorkers := q.MaxWorkers
			if maxWorkers <= 0 {
				maxWorkers = DefaultConcurrency
			}
			queues[name] = river.QueueConfig{MaxWorkers: maxWorkers}
		}
		return queues
	}

	// Legacy single-queue mode for backward compatibility
	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}

	queueName := cfg.QueueName
	if queueName == "" {
		queueName = river.QueueDefault
	}

	return map[string]river.QueueConfig{
		queueName: {MaxWorkers: concurrency},
	}
}

func (s *Server) Start(ctx context.Context) error {
	return s.client.Start(ctx)
}

func (s *Server) Stop(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.shutdownTimeout)
	defer cancel()
	return s.client.Stop(ctx)
}

func (s *Server) Client() *river.Client[pgx.Tx] {
	return s.client
}
