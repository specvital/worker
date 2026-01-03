package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// SetupTestDB creates a PostgreSQL container with the full schema loaded.
// Returns a connection pool and cleanup function.
func SetupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
		testcontainers.CustomizeRequest(testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Networks: []string{"specvital-network"},
			},
		}),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	containerIP, err := container.ContainerIP(ctx)
	if err != nil {
		container.Terminate(ctx)
		t.Fatalf("failed to get container IP: %v", err)
	}

	connStr := fmt.Sprintf("postgres://test:test@%s:5432/testdb?sslmode=disable", containerIP)

	var pool *pgxpool.Pool
	var lastErr error
	for range 30 {
		pool, lastErr = pgxpool.New(ctx, connStr)
		if lastErr == nil {
			lastErr = pool.Ping(ctx)
			if lastErr == nil {
				break
			}
			pool.Close()
			pool = nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	if pool == nil {
		container.Terminate(ctx)
		t.Fatalf("failed to connect to database after retries: %v", lastErr)
	}

	if err := runMigrations(ctx, pool); err != nil {
		pool.Close()
		container.Terminate(ctx)
		t.Fatalf("failed to run migrations: %v", err)
	}

	cleanup := func() {
		pool.Close()
		terminateCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := container.Terminate(terminateCtx); err != nil {
			t.Logf("warning: failed to terminate container: %v", err)
		}
	}

	return pool, cleanup
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, Schema())
	return err
}
