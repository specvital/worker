package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/specvital/collector/internal/domain/analysis"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupUserTestDB(t *testing.T) (*pgxpool.Pool, func()) {
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
	for i := 0; i < 30; i++ {
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

	if err := runUserMigrations(ctx, pool); err != nil {
		pool.Close()
		container.Terminate(ctx)
		t.Fatalf("failed to run migrations: %v", err)
	}

	cleanup := func() {
		pool.Close()
		container.Terminate(ctx)
	}

	return pool, cleanup
}

func runUserMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	schema := `
		CREATE TYPE oauth_provider AS ENUM ('github');

		CREATE TABLE users (
			id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
			email varchar(255),
			username varchar(255) NOT NULL,
			avatar_url text,
			last_login_at timestamptz,
			created_at timestamptz DEFAULT now() NOT NULL,
			updated_at timestamptz DEFAULT now() NOT NULL
		);

		CREATE TABLE oauth_accounts (
			id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
			user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			provider oauth_provider NOT NULL,
			provider_user_id varchar(255) NOT NULL,
			provider_username varchar(255),
			access_token text,
			scope varchar(500),
			created_at timestamptz DEFAULT now() NOT NULL,
			updated_at timestamptz DEFAULT now() NOT NULL,
			UNIQUE (provider, provider_user_id)
		);

		CREATE INDEX idx_oauth_accounts_user_provider ON oauth_accounts (user_id, provider);
	`
	_, err := pool.Exec(ctx, schema)
	return err
}

func TestUserRepository_GetOAuthToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := setupUserTestDB(t)
	defer cleanup()

	repo := NewUserRepository(pool)
	ctx := context.Background()

	t.Run("should return token for valid user and provider", func(t *testing.T) {
		// Create test user and oauth account
		var userID string
		err := pool.QueryRow(ctx, `
			INSERT INTO users (username) VALUES ('testuser')
			RETURNING id::text
		`).Scan(&userID)
		if err != nil {
			t.Fatalf("failed to create user: %v", err)
		}

		expectedToken := "ghp_test_token_123"
		_, err = pool.Exec(ctx, `
			INSERT INTO oauth_accounts (user_id, provider, provider_user_id, access_token)
			VALUES ($1::uuid, 'github', 'github_user_123', $2)
		`, userID, expectedToken)
		if err != nil {
			t.Fatalf("failed to create oauth account: %v", err)
		}

		token, err := repo.GetOAuthToken(ctx, userID, "github")
		if err != nil {
			t.Fatalf("GetOAuthToken failed: %v", err)
		}
		if token != expectedToken {
			t.Errorf("expected token %q, got %q", expectedToken, token)
		}
	})

	t.Run("should return ErrTokenNotFound for non-existent user", func(t *testing.T) {
		nonExistentUserID := "00000000-0000-0000-0000-000000000000"
		_, err := repo.GetOAuthToken(ctx, nonExistentUserID, "github")
		if !errors.Is(err, analysis.ErrTokenNotFound) {
			t.Errorf("expected ErrTokenNotFound, got %v", err)
		}
	})

	t.Run("should return ErrTokenNotFound for user without oauth account", func(t *testing.T) {
		var userID string
		err := pool.QueryRow(ctx, `
			INSERT INTO users (username) VALUES ('user_without_oauth')
			RETURNING id::text
		`).Scan(&userID)
		if err != nil {
			t.Fatalf("failed to create user: %v", err)
		}

		_, err = repo.GetOAuthToken(ctx, userID, "github")
		if !errors.Is(err, analysis.ErrTokenNotFound) {
			t.Errorf("expected ErrTokenNotFound, got %v", err)
		}
	})

	t.Run("should return ErrTokenNotFound for null access_token", func(t *testing.T) {
		var userID string
		err := pool.QueryRow(ctx, `
			INSERT INTO users (username) VALUES ('user_null_token')
			RETURNING id::text
		`).Scan(&userID)
		if err != nil {
			t.Fatalf("failed to create user: %v", err)
		}

		_, err = pool.Exec(ctx, `
			INSERT INTO oauth_accounts (user_id, provider, provider_user_id, access_token)
			VALUES ($1::uuid, 'github', 'github_null_token', NULL)
		`, userID)
		if err != nil {
			t.Fatalf("failed to create oauth account: %v", err)
		}

		_, err = repo.GetOAuthToken(ctx, userID, "github")
		if !errors.Is(err, analysis.ErrTokenNotFound) {
			t.Errorf("expected ErrTokenNotFound for null token, got %v", err)
		}
	})

	t.Run("should return ErrTokenNotFound for empty access_token", func(t *testing.T) {
		var userID string
		err := pool.QueryRow(ctx, `
			INSERT INTO users (username) VALUES ('user_empty_token')
			RETURNING id::text
		`).Scan(&userID)
		if err != nil {
			t.Fatalf("failed to create user: %v", err)
		}

		_, err = pool.Exec(ctx, `
			INSERT INTO oauth_accounts (user_id, provider, provider_user_id, access_token)
			VALUES ($1::uuid, 'github', 'github_empty_token', '')
		`, userID)
		if err != nil {
			t.Fatalf("failed to create oauth account: %v", err)
		}

		_, err = repo.GetOAuthToken(ctx, userID, "github")
		if !errors.Is(err, analysis.ErrTokenNotFound) {
			t.Errorf("expected ErrTokenNotFound for empty token, got %v", err)
		}
	})

	t.Run("should return error for empty user ID", func(t *testing.T) {
		_, err := repo.GetOAuthToken(ctx, "", "github")
		if err == nil {
			t.Error("expected error for empty user ID, got nil")
		}
	})

	t.Run("should return error for empty provider", func(t *testing.T) {
		_, err := repo.GetOAuthToken(ctx, "some-user-id", "")
		if err == nil {
			t.Error("expected error for empty provider, got nil")
		}
	})

	t.Run("should return error for invalid UUID format", func(t *testing.T) {
		_, err := repo.GetOAuthToken(ctx, "not-a-valid-uuid", "github")
		if err == nil {
			t.Error("expected error for invalid UUID, got nil")
		}
	})
}

func TestNewUserRepository(t *testing.T) {
	repo := NewUserRepository(nil)
	if repo == nil {
		t.Error("expected non-nil repository, got nil")
	}
}
