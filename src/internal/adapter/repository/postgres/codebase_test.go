package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/specvital/collector/internal/domain/analysis"
)

func TestCodebaseRepository_FindByExternalID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	analysisRepo := NewAnalysisRepository(pool)
	codebaseRepo := NewCodebaseRepository(pool)
	ctx := context.Background()

	t.Run("should find codebase by external ID", func(t *testing.T) {
		_, err := analysisRepo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "find-ext-owner",
			Repo:           "find-ext-repo",
			CommitSHA:      "find-ext-sha",
			Branch:         "main",
			ExternalRepoID: "ext-id-12345",
		})
		if err != nil {
			t.Fatalf("CreateAnalysisRecord failed: %v", err)
		}

		codebase, err := codebaseRepo.FindByExternalID(ctx, "github.com", "ext-id-12345")
		if err != nil {
			t.Fatalf("FindByExternalID failed: %v", err)
		}

		if codebase.Owner != "find-ext-owner" {
			t.Errorf("expected owner 'find-ext-owner', got '%s'", codebase.Owner)
		}
		if codebase.Name != "find-ext-repo" {
			t.Errorf("expected name 'find-ext-repo', got '%s'", codebase.Name)
		}
		if codebase.ExternalRepoID != "ext-id-12345" {
			t.Errorf("expected external repo ID 'ext-id-12345', got '%s'", codebase.ExternalRepoID)
		}
	})

	t.Run("should return ErrCodebaseNotFound when not exists", func(t *testing.T) {
		_, err := codebaseRepo.FindByExternalID(ctx, "github.com", "non-existent-id")
		if !errors.Is(err, analysis.ErrCodebaseNotFound) {
			t.Errorf("expected ErrCodebaseNotFound, got %v", err)
		}
	})
}

func TestCodebaseRepository_FindByOwnerName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	analysisRepo := NewAnalysisRepository(pool)
	codebaseRepo := NewCodebaseRepository(pool)
	ctx := context.Background()

	t.Run("should find codebase by owner and name", func(t *testing.T) {
		_, err := analysisRepo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "find-owner",
			Repo:           "find-repo",
			CommitSHA:      "find-sha",
			Branch:         "main",
			ExternalRepoID: "owner-name-id-1",
		})
		if err != nil {
			t.Fatalf("CreateAnalysisRecord failed: %v", err)
		}

		codebase, err := codebaseRepo.FindByOwnerName(ctx, "github.com", "find-owner", "find-repo")
		if err != nil {
			t.Fatalf("FindByOwnerName failed: %v", err)
		}

		if codebase.Owner != "find-owner" {
			t.Errorf("expected owner 'find-owner', got '%s'", codebase.Owner)
		}
		if codebase.Name != "find-repo" {
			t.Errorf("expected name 'find-repo', got '%s'", codebase.Name)
		}
		if codebase.ExternalRepoID != "owner-name-id-1" {
			t.Errorf("expected external repo ID 'owner-name-id-1', got '%s'", codebase.ExternalRepoID)
		}
	})

	t.Run("should return ErrCodebaseNotFound when not exists", func(t *testing.T) {
		_, err := codebaseRepo.FindByOwnerName(ctx, "github.com", "non-existent", "repo")
		if !errors.Is(err, analysis.ErrCodebaseNotFound) {
			t.Errorf("expected ErrCodebaseNotFound, got %v", err)
		}
	})

	t.Run("should not find stale codebase by owner name", func(t *testing.T) {
		_, err := analysisRepo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "stale-owner",
			Repo:           "stale-repo",
			CommitSHA:      "stale-sha",
			Branch:         "main",
			ExternalRepoID: "stale-ext-id",
		})
		if err != nil {
			t.Fatalf("CreateAnalysisRecord failed: %v", err)
		}

		_, err = pool.Exec(ctx, "UPDATE codebases SET is_stale = true WHERE owner = 'stale-owner' AND name = 'stale-repo'")
		if err != nil {
			t.Fatalf("failed to mark codebase stale: %v", err)
		}

		_, err = codebaseRepo.FindByOwnerName(ctx, "github.com", "stale-owner", "stale-repo")
		if !errors.Is(err, analysis.ErrCodebaseNotFound) {
			t.Errorf("expected ErrCodebaseNotFound for stale codebase, got %v", err)
		}
	})
}

func TestCodebaseRepository_MarkStale(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	analysisRepo := NewAnalysisRepository(pool)
	codebaseRepo := NewCodebaseRepository(pool)
	ctx := context.Background()

	t.Run("should mark codebase as stale", func(t *testing.T) {
		_, err := analysisRepo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "mark-stale-owner",
			Repo:           "mark-stale-repo",
			CommitSHA:      "mark-stale-sha",
			Branch:         "main",
			ExternalRepoID: "mark-stale-ext-id",
		})
		if err != nil {
			t.Fatalf("CreateAnalysisRecord failed: %v", err)
		}

		codebase, err := codebaseRepo.FindByOwnerName(ctx, "github.com", "mark-stale-owner", "mark-stale-repo")
		if err != nil {
			t.Fatalf("FindByOwnerName failed: %v", err)
		}

		err = codebaseRepo.MarkStale(ctx, codebase.ID)
		if err != nil {
			t.Fatalf("MarkStale failed: %v", err)
		}

		_, err = codebaseRepo.FindByOwnerName(ctx, "github.com", "mark-stale-owner", "mark-stale-repo")
		if !errors.Is(err, analysis.ErrCodebaseNotFound) {
			t.Errorf("expected ErrCodebaseNotFound after marking stale, got %v", err)
		}

		staleCodebase, err := codebaseRepo.FindByExternalID(ctx, "github.com", "mark-stale-ext-id")
		if err != nil {
			t.Fatalf("FindByExternalID failed: %v", err)
		}
		if !staleCodebase.IsStale {
			t.Error("expected codebase to be stale")
		}
	})
}

func TestCodebaseRepository_UnmarkStale(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	analysisRepo := NewAnalysisRepository(pool)
	codebaseRepo := NewCodebaseRepository(pool)
	ctx := context.Background()

	t.Run("should unmark stale and update owner/name", func(t *testing.T) {
		_, err := analysisRepo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "unmark-old-owner",
			Repo:           "unmark-old-repo",
			CommitSHA:      "unmark-sha",
			Branch:         "main",
			ExternalRepoID: "unmark-stale-ext-id",
		})
		if err != nil {
			t.Fatalf("CreateAnalysisRecord failed: %v", err)
		}

		codebase, _ := codebaseRepo.FindByOwnerName(ctx, "github.com", "unmark-old-owner", "unmark-old-repo")
		_ = codebaseRepo.MarkStale(ctx, codebase.ID)

		updated, err := codebaseRepo.UnmarkStale(ctx, codebase.ID, "unmark-new-owner", "unmark-new-repo")
		if err != nil {
			t.Fatalf("UnmarkStale failed: %v", err)
		}

		if updated.IsStale {
			t.Error("expected codebase to not be stale after unmark")
		}
		if updated.Owner != "unmark-new-owner" {
			t.Errorf("expected owner 'unmark-new-owner', got '%s'", updated.Owner)
		}
		if updated.Name != "unmark-new-repo" {
			t.Errorf("expected name 'unmark-new-repo', got '%s'", updated.Name)
		}

		found, err := codebaseRepo.FindByOwnerName(ctx, "github.com", "unmark-new-owner", "unmark-new-repo")
		if err != nil {
			t.Fatalf("FindByOwnerName after unmark failed: %v", err)
		}
		if found.ID != codebase.ID {
			t.Error("expected same codebase ID after unmark")
		}
	})

	t.Run("should return ErrCodebaseNotFound for non-existent ID", func(t *testing.T) {
		nonExistentID := analysis.UUID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
		_, err := codebaseRepo.UnmarkStale(ctx, nonExistentID, "owner", "name")
		if !errors.Is(err, analysis.ErrCodebaseNotFound) {
			t.Errorf("expected ErrCodebaseNotFound, got %v", err)
		}
	})
}

func TestCodebaseRepository_UpdateOwnerName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	analysisRepo := NewAnalysisRepository(pool)
	codebaseRepo := NewCodebaseRepository(pool)
	ctx := context.Background()

	t.Run("should update owner and name", func(t *testing.T) {
		_, err := analysisRepo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "update-old-owner",
			Repo:           "update-old-repo",
			CommitSHA:      "update-sha",
			Branch:         "main",
			ExternalRepoID: "update-ext-id",
		})
		if err != nil {
			t.Fatalf("CreateAnalysisRecord failed: %v", err)
		}

		codebase, _ := codebaseRepo.FindByOwnerName(ctx, "github.com", "update-old-owner", "update-old-repo")

		updated, err := codebaseRepo.UpdateOwnerName(ctx, codebase.ID, "update-new-owner", "update-new-repo")
		if err != nil {
			t.Fatalf("UpdateOwnerName failed: %v", err)
		}

		if updated.Owner != "update-new-owner" {
			t.Errorf("expected owner 'update-new-owner', got '%s'", updated.Owner)
		}
		if updated.Name != "update-new-repo" {
			t.Errorf("expected name 'update-new-repo', got '%s'", updated.Name)
		}

		_, err = codebaseRepo.FindByOwnerName(ctx, "github.com", "update-old-owner", "update-old-repo")
		if !errors.Is(err, analysis.ErrCodebaseNotFound) {
			t.Errorf("expected old owner/name to not be found, got %v", err)
		}

		found, err := codebaseRepo.FindByOwnerName(ctx, "github.com", "update-new-owner", "update-new-repo")
		if err != nil {
			t.Fatalf("FindByOwnerName with new values failed: %v", err)
		}
		if found.ID != codebase.ID {
			t.Error("expected same codebase ID after update")
		}
	})

	t.Run("should return ErrCodebaseNotFound for non-existent ID", func(t *testing.T) {
		nonExistentID := analysis.UUID{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20}
		_, err := codebaseRepo.UpdateOwnerName(ctx, nonExistentID, "owner", "name")
		if !errors.Is(err, analysis.ErrCodebaseNotFound) {
			t.Errorf("expected ErrCodebaseNotFound, got %v", err)
		}
	})
}
