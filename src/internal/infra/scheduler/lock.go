package scheduler

import (
	"context"
	"fmt"
	"hash/fnv"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DistributedLock uses PostgreSQL advisory locks.
// Advisory locks are session-scoped - connection pool recycling releases the lock.
type DistributedLock struct {
	pool   *pgxpool.Pool
	lockID int64
}

func NewDistributedLock(pool *pgxpool.Pool, key string) *DistributedLock {
	return &DistributedLock{
		pool:   pool,
		lockID: hashKey(key),
	}
}

func hashKey(key string) int64 {
	h := fnv.New64a()
	h.Write([]byte(key))
	return int64(h.Sum64())
}

// TryAcquire attempts to acquire the advisory lock without blocking.
// Returns true if lock was acquired, false if another session holds it.
func (l *DistributedLock) TryAcquire(ctx context.Context) (bool, error) {
	var acquired bool
	err := l.pool.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", l.lockID).Scan(&acquired)
	if err != nil {
		return false, fmt.Errorf("pg_try_advisory_lock: %w", err)
	}
	return acquired, nil
}

// Release releases the advisory lock.
// Always attempts unlock regardless of in-memory state since connection pool
// may have recycled the connection that originally acquired the lock.
// Safe to call multiple times - PostgreSQL returns false if lock wasn't held.
func (l *DistributedLock) Release(ctx context.Context) error {
	var released bool
	err := l.pool.QueryRow(ctx, "SELECT pg_advisory_unlock($1)", l.lockID).Scan(&released)
	if err != nil {
		return fmt.Errorf("pg_advisory_unlock: %w", err)
	}
	return nil
}

// Close is a no-op for PostgreSQL advisory locks.
// Advisory locks are automatically released when the session ends.
func (l *DistributedLock) Close() error {
	return nil
}
