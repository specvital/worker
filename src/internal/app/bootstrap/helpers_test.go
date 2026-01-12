package bootstrap

import (
	"context"
	"testing"

	"github.com/specvital/worker/internal/adapter/repository/postgres"
	testdb "github.com/specvital/worker/internal/testutil/postgres"
)

func TestRegisterParserVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("should register parser version to system_config", func(t *testing.T) {
		testVersion := "v1.0.0-0.20260112121406-abc123def456"
		err := registerParserVersion(ctx, pool, testVersion)
		if err != nil {
			t.Fatalf("registerParserVersion failed: %v", err)
		}

		repo := postgres.NewSystemConfigRepository(pool)
		value, err := repo.Get(ctx, postgres.ConfigKeyParserVersion)
		if err != nil {
			t.Fatalf("Get parser_version failed: %v", err)
		}

		if value != testVersion {
			t.Errorf("expected %q, got %q", testVersion, value)
		}
	})

	t.Run("should be idempotent (multiple calls succeed)", func(t *testing.T) {
		testVersion := "v1.0.0-0.20260112121406-abc123def456"
		// First call
		err := registerParserVersion(ctx, pool, testVersion)
		if err != nil {
			t.Fatalf("first registerParserVersion failed: %v", err)
		}

		// Second call should also succeed (upsert)
		err = registerParserVersion(ctx, pool, testVersion)
		if err != nil {
			t.Fatalf("second registerParserVersion failed: %v", err)
		}
	})

	t.Run("should skip registration when version is unknown", func(t *testing.T) {
		err := registerParserVersion(ctx, pool, "unknown")
		if err != nil {
			t.Fatalf("registerParserVersion failed: %v", err)
		}
		// No error means it successfully skipped
	})
}
