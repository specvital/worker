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

type ServerConfig struct {
	Pool            *pgxpool.Pool
	Concurrency     int
	QueueName       string
	ShutdownTimeout time.Duration
	Workers         *river.Workers
}

type Server struct {
	client          *river.Client[pgx.Tx]
	shutdownTimeout time.Duration
}

func NewServer(ctx context.Context, cfg ServerConfig) (*Server, error) {
	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}

	shutdownTimeout := cfg.ShutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = DefaultShutdownTimeout
	}

	queueName := cfg.QueueName
	if queueName == "" {
		queueName = river.QueueDefault
	}

	client, err := river.NewClient(riverpgxv5.New(cfg.Pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			queueName: {MaxWorkers: concurrency},
		},
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
