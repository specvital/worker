package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/specvital/collector/internal/domain/analysis"
	testdb "github.com/specvital/collector/internal/testutil/postgres"
	"github.com/specvital/core/pkg/crypto"
)

// passthroughEncryptor returns input as-is for testing without real encryption.
type passthroughEncryptor struct{}

func (e *passthroughEncryptor) Encrypt(plaintext string) (string, error) {
	return plaintext, nil
}

func (e *passthroughEncryptor) Decrypt(ciphertext string) (string, error) {
	return ciphertext, nil
}

func (e *passthroughEncryptor) Close() error {
	return nil
}

var _ crypto.Encryptor = (*passthroughEncryptor)(nil)

func TestUserRepository_GetOAuthToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	repo := NewUserRepository(pool, &passthroughEncryptor{})
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
	repo := NewUserRepository(nil, &passthroughEncryptor{})
	if repo == nil {
		t.Error("expected non-nil repository, got nil")
	}
}
