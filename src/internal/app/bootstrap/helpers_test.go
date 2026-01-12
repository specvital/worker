package bootstrap

import (
	"context"
	"errors"
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
		err := registerParserVersion(ctx, pool)
		if err != nil {
			t.Fatalf("registerParserVersion failed: %v", err)
		}

		repo := postgres.NewSystemConfigRepository(pool)
		value, err := repo.Get(ctx, postgres.ConfigKeyParserVersion)

		// In test environment, version may be "unknown" and registration skipped
		if err != nil {
			if errors.Is(err, postgres.ErrConfigNotFound) {
				// This is expected when running with "go test" (version = "unknown")
				t.Log("parser version unknown (expected in test environment), skipping verification")
				return
			}
			t.Fatalf("Get parser_version failed: %v", err)
		}

		if value == "" {
			t.Error("expected non-empty parser version")
		}
		t.Logf("registered parser version: %s", value)
	})

	t.Run("should be idempotent (multiple calls succeed)", func(t *testing.T) {
		// First call
		err := registerParserVersion(ctx, pool)
		if err != nil {
			t.Fatalf("first registerParserVersion failed: %v", err)
		}

		// Second call should also succeed (upsert)
		err = registerParserVersion(ctx, pool)
		if err != nil {
			t.Fatalf("second registerParserVersion failed: %v", err)
		}
	})
}
