package postgres

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/specvital/worker/internal/domain/analysis"
	testdb "github.com/specvital/worker/internal/testutil/postgres"
)

func TestRetentionRepository_DeleteExpiredUserAnalysisHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	analysisRepo := NewAnalysisRepository(pool)
	retentionRepo := NewRetentionRepository(pool)
	ctx := context.Background()

	t.Run("should delete expired records with retention limit", func(t *testing.T) {
		// Create a test user
		var userID string
		err := pool.QueryRow(ctx, "INSERT INTO users (email, username) VALUES ('retention-test@example.com', 'retentionuser') RETURNING id::text").Scan(&userID)
		if err != nil {
			t.Fatalf("failed to create test user: %v", err)
		}

		// Create analysis
		analysisID, err := analysisRepo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "retention-owner",
			Repo:           "retention-repo",
			CommitSHA:      "ret123",
			Branch:         "main",
			ExternalRepoID: "retention-id-1",
			ParserVersion:  testParserVersion,
		})
		if err != nil {
			t.Fatalf("CreateAnalysisRecord failed: %v", err)
		}

		// Save inventory with user (creates user_analysis_history)
		inventory := &analysis.Inventory{
			Files: []analysis.TestFile{
				{
					Path:      "test.go",
					Framework: "go-test",
					Suites: []analysis.TestSuite{
						{
							Name:     "TestSuite",
							Location: analysis.Location{StartLine: 10},
							Tests: []analysis.Test{
								{Name: "Test1", Location: analysis.Location{StartLine: 12}},
							},
						},
					},
				},
			},
		}

		err = analysisRepo.SaveAnalysisInventory(ctx, analysis.SaveAnalysisInventoryParams{
			AnalysisID: analysisID,
			Inventory:  inventory,
			UserID:     &userID,
		})
		if err != nil {
			t.Fatalf("SaveAnalysisInventory failed: %v", err)
		}

		// Set retention_days_at_creation to 1 day and backdate created_at to 2 days ago
		_, err = pool.Exec(ctx, `
			UPDATE user_analysis_history
			SET retention_days_at_creation = 1,
			    created_at = now() - interval '2 days'
			WHERE user_id = $1 AND analysis_id = $2
		`, userID, toPgUUID(analysisID))
		if err != nil {
			t.Fatalf("failed to update history: %v", err)
		}

		// Verify record exists
		var countBefore int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM user_analysis_history WHERE user_id = $1", userID).Scan(&countBefore)
		if err != nil {
			t.Fatalf("failed to query history count: %v", err)
		}
		if countBefore != 1 {
			t.Fatalf("expected 1 history record before delete, got %d", countBefore)
		}

		// Delete expired records
		result, err := retentionRepo.DeleteExpiredUserAnalysisHistory(ctx, 100)
		if err != nil {
			t.Fatalf("DeleteExpiredUserAnalysisHistory failed: %v", err)
		}

		if result.DeletedCount != 1 {
			t.Errorf("expected 1 deleted record, got %d", result.DeletedCount)
		}

		// Verify record is deleted
		var countAfter int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM user_analysis_history WHERE user_id = $1", userID).Scan(&countAfter)
		if err != nil {
			t.Fatalf("failed to query history count after: %v", err)
		}
		if countAfter != 0 {
			t.Errorf("expected 0 history records after delete, got %d", countAfter)
		}
	})

	t.Run("should not delete records with NULL retention (enterprise)", func(t *testing.T) {
		_, err := pool.Exec(ctx, "TRUNCATE codebases CASCADE")
		if err != nil {
			t.Fatalf("failed to truncate: %v", err)
		}

		var userID string
		err = pool.QueryRow(ctx, "INSERT INTO users (email, username) VALUES ('enterprise@example.com', 'enterpriseuser') RETURNING id::text").Scan(&userID)
		if err != nil {
			t.Fatalf("failed to create test user: %v", err)
		}

		analysisID, err := analysisRepo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "enterprise-owner",
			Repo:           "enterprise-repo",
			CommitSHA:      "ent123",
			Branch:         "main",
			ExternalRepoID: "enterprise-id-1",
			ParserVersion:  testParserVersion,
		})
		if err != nil {
			t.Fatalf("CreateAnalysisRecord failed: %v", err)
		}

		inventory := &analysis.Inventory{
			Files: []analysis.TestFile{
				{
					Path:      "test.go",
					Framework: "go-test",
					Suites: []analysis.TestSuite{
						{
							Name:     "TestSuite",
							Location: analysis.Location{StartLine: 10},
							Tests: []analysis.Test{
								{Name: "Test1", Location: analysis.Location{StartLine: 12}},
							},
						},
					},
				},
			},
		}

		err = analysisRepo.SaveAnalysisInventory(ctx, analysis.SaveAnalysisInventoryParams{
			AnalysisID: analysisID,
			Inventory:  inventory,
			UserID:     &userID,
		})
		if err != nil {
			t.Fatalf("SaveAnalysisInventory failed: %v", err)
		}

		// Leave retention_days_at_creation as NULL (enterprise) but backdate created_at
		_, err = pool.Exec(ctx, `
			UPDATE user_analysis_history
			SET created_at = now() - interval '365 days'
			WHERE user_id = $1 AND analysis_id = $2
		`, userID, toPgUUID(analysisID))
		if err != nil {
			t.Fatalf("failed to update history: %v", err)
		}

		// Delete expired records
		result, err := retentionRepo.DeleteExpiredUserAnalysisHistory(ctx, 100)
		if err != nil {
			t.Fatalf("DeleteExpiredUserAnalysisHistory failed: %v", err)
		}

		if result.DeletedCount != 0 {
			t.Errorf("expected 0 deleted records (enterprise unlimited), got %d", result.DeletedCount)
		}

		// Verify record still exists
		var countAfter int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM user_analysis_history WHERE user_id = $1", userID).Scan(&countAfter)
		if err != nil {
			t.Fatalf("failed to query history count: %v", err)
		}
		if countAfter != 1 {
			t.Errorf("expected 1 history record (not deleted), got %d", countAfter)
		}
	})

	t.Run("should respect batch size limit", func(t *testing.T) {
		_, err := pool.Exec(ctx, "TRUNCATE codebases CASCADE")
		if err != nil {
			t.Fatalf("failed to truncate: %v", err)
		}

		var userID string
		err = pool.QueryRow(ctx, "INSERT INTO users (email, username) VALUES ('batch@example.com', 'batchuser') RETURNING id::text").Scan(&userID)
		if err != nil {
			t.Fatalf("failed to create test user: %v", err)
		}

		// Create 5 analyses with expired records
		for i := 0; i < 5; i++ {
			analysisID, err := analysisRepo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
				Owner:          "batch-owner",
				Repo:           "batch-repo",
				CommitSHA:      "batch" + string(rune('0'+i)),
				Branch:         "main",
				ExternalRepoID: "batch-id-" + string(rune('0'+i)),
				ParserVersion:  testParserVersion,
			})
			if err != nil {
				t.Fatalf("CreateAnalysisRecord %d failed: %v", i, err)
			}

			inventory := &analysis.Inventory{
				Files: []analysis.TestFile{
					{
						Path:      "test.go",
						Framework: "go-test",
						Suites: []analysis.TestSuite{
							{
								Name:     "TestSuite",
								Location: analysis.Location{StartLine: 10},
								Tests:    []analysis.Test{{Name: "Test1", Location: analysis.Location{StartLine: 12}}},
							},
						},
					},
				},
			}

			err = analysisRepo.SaveAnalysisInventory(ctx, analysis.SaveAnalysisInventoryParams{
				AnalysisID: analysisID,
				Inventory:  inventory,
				UserID:     &userID,
			})
			if err != nil {
				t.Fatalf("SaveAnalysisInventory %d failed: %v", i, err)
			}

			// Set as expired
			_, err = pool.Exec(ctx, `
				UPDATE user_analysis_history
				SET retention_days_at_creation = 1,
				    created_at = now() - interval '2 days'
				WHERE analysis_id = $1
			`, toPgUUID(analysisID))
			if err != nil {
				t.Fatalf("failed to update history %d: %v", i, err)
			}
		}

		// Delete with batch size 2
		result, err := retentionRepo.DeleteExpiredUserAnalysisHistory(ctx, 2)
		if err != nil {
			t.Fatalf("DeleteExpiredUserAnalysisHistory failed: %v", err)
		}

		if result.DeletedCount != 2 {
			t.Errorf("expected 2 deleted records (batch limit), got %d", result.DeletedCount)
		}

		// HasMore should return true
		if !result.HasMore(2) {
			t.Error("expected HasMore to return true")
		}

		// Delete remaining
		result2, err := retentionRepo.DeleteExpiredUserAnalysisHistory(ctx, 10)
		if err != nil {
			t.Fatalf("second DeleteExpiredUserAnalysisHistory failed: %v", err)
		}

		if result2.DeletedCount != 3 {
			t.Errorf("expected 3 remaining deleted records, got %d", result2.DeletedCount)
		}
	})
}

func TestRetentionRepository_DeleteExpiredSpecDocuments(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	analysisRepo := NewAnalysisRepository(pool)
	retentionRepo := NewRetentionRepository(pool)
	ctx := context.Background()

	t.Run("should delete expired spec documents", func(t *testing.T) {
		var userID string
		err := pool.QueryRow(ctx, "INSERT INTO users (email, username) VALUES ('spec-test@example.com', 'specuser') RETURNING id::text").Scan(&userID)
		if err != nil {
			t.Fatalf("failed to create test user: %v", err)
		}

		analysisID, err := analysisRepo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "spec-owner",
			Repo:           "spec-repo",
			CommitSHA:      "spec123",
			Branch:         "main",
			ExternalRepoID: "spec-id-1",
			ParserVersion:  testParserVersion,
		})
		if err != nil {
			t.Fatalf("CreateAnalysisRecord failed: %v", err)
		}

		inventory := &analysis.Inventory{
			Files: []analysis.TestFile{
				{
					Path:      "test.go",
					Framework: "go-test",
					Suites: []analysis.TestSuite{
						{
							Name:     "TestSuite",
							Location: analysis.Location{StartLine: 10},
							Tests:    []analysis.Test{{Name: "Test1", Location: analysis.Location{StartLine: 12}}},
						},
					},
				},
			},
		}

		err = analysisRepo.SaveAnalysisInventory(ctx, analysis.SaveAnalysisInventoryParams{
			AnalysisID: analysisID,
			Inventory:  inventory,
			UserID:     &userID,
		})
		if err != nil {
			t.Fatalf("SaveAnalysisInventory failed: %v", err)
		}

		pgAnalysisID := toPgUUID(analysisID)

		// Insert spec_document with expired retention
		_, err = pool.Exec(ctx, `
			INSERT INTO spec_documents (user_id, analysis_id, content_hash, language, model_id, version, retention_days_at_creation, created_at)
			VALUES ($1, $2, $3, 'en', 'gemini-2.0', 1, 1, now() - interval '2 days')
		`, userID, pgAnalysisID, []byte("hash123"))
		if err != nil {
			t.Fatalf("failed to insert spec_document: %v", err)
		}

		// Verify record exists
		var countBefore int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM spec_documents WHERE user_id = $1", userID).Scan(&countBefore)
		if err != nil {
			t.Fatalf("failed to query spec_documents count: %v", err)
		}
		if countBefore != 1 {
			t.Fatalf("expected 1 spec_document before delete, got %d", countBefore)
		}

		// Delete expired records
		result, err := retentionRepo.DeleteExpiredSpecDocuments(ctx, 100)
		if err != nil {
			t.Fatalf("DeleteExpiredSpecDocuments failed: %v", err)
		}

		if result.DeletedCount != 1 {
			t.Errorf("expected 1 deleted record, got %d", result.DeletedCount)
		}

		// Verify record is deleted
		var countAfter int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM spec_documents WHERE user_id = $1", userID).Scan(&countAfter)
		if err != nil {
			t.Fatalf("failed to query spec_documents count after: %v", err)
		}
		if countAfter != 0 {
			t.Errorf("expected 0 spec_documents after delete, got %d", countAfter)
		}
	})

	t.Run("should not delete spec documents with NULL retention", func(t *testing.T) {
		_, err := pool.Exec(ctx, "TRUNCATE codebases CASCADE")
		if err != nil {
			t.Fatalf("failed to truncate: %v", err)
		}

		var userID string
		err = pool.QueryRow(ctx, "INSERT INTO users (email, username) VALUES ('spec-ent@example.com', 'specentuser') RETURNING id::text").Scan(&userID)
		if err != nil {
			t.Fatalf("failed to create test user: %v", err)
		}

		analysisID, err := analysisRepo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "spec-ent-owner",
			Repo:           "spec-ent-repo",
			CommitSHA:      "specent123",
			Branch:         "main",
			ExternalRepoID: "spec-ent-id-1",
			ParserVersion:  testParserVersion,
		})
		if err != nil {
			t.Fatalf("CreateAnalysisRecord failed: %v", err)
		}

		inventory := &analysis.Inventory{
			Files: []analysis.TestFile{
				{
					Path:      "test.go",
					Framework: "go-test",
					Suites: []analysis.TestSuite{
						{
							Name:     "TestSuite",
							Location: analysis.Location{StartLine: 10},
							Tests:    []analysis.Test{{Name: "Test1", Location: analysis.Location{StartLine: 12}}},
						},
					},
				},
			},
		}

		err = analysisRepo.SaveAnalysisInventory(ctx, analysis.SaveAnalysisInventoryParams{
			AnalysisID: analysisID,
			Inventory:  inventory,
			UserID:     &userID,
		})
		if err != nil {
			t.Fatalf("SaveAnalysisInventory failed: %v", err)
		}

		pgAnalysisID := toPgUUID(analysisID)

		// Insert spec_document with NULL retention (enterprise)
		_, err = pool.Exec(ctx, `
			INSERT INTO spec_documents (user_id, analysis_id, content_hash, language, model_id, version, retention_days_at_creation, created_at)
			VALUES ($1, $2, $3, 'en', 'gemini-2.0', 1, NULL, now() - interval '365 days')
		`, userID, pgAnalysisID, []byte("hash456"))
		if err != nil {
			t.Fatalf("failed to insert spec_document: %v", err)
		}

		// Delete expired records
		result, err := retentionRepo.DeleteExpiredSpecDocuments(ctx, 100)
		if err != nil {
			t.Fatalf("DeleteExpiredSpecDocuments failed: %v", err)
		}

		if result.DeletedCount != 0 {
			t.Errorf("expected 0 deleted records (enterprise unlimited), got %d", result.DeletedCount)
		}
	})
}

func TestRetentionRepository_DeleteOrphanedAnalyses(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	retentionRepo := NewRetentionRepository(pool)
	ctx := context.Background()

	t.Run("should delete orphaned analyses older than 1 day", func(t *testing.T) {
		// Create codebase first
		var codebaseID pgtype.UUID
		err := pool.QueryRow(ctx, `
			INSERT INTO codebases (host, owner, name, external_repo_id)
			VALUES ('github.com', 'orphan-owner', 'orphan-repo', 'orphan-ext-1')
			RETURNING id
		`).Scan(&codebaseID)
		if err != nil {
			t.Fatalf("failed to create codebase: %v", err)
		}

		// Create orphaned analysis (no user_analysis_history reference)
		var analysisID pgtype.UUID
		err = pool.QueryRow(ctx, `
			INSERT INTO analyses (codebase_id, commit_sha, status, parser_version, created_at)
			VALUES ($1, 'orphan123', 'completed', 'v1.0.0', now() - interval '2 days')
			RETURNING id
		`, codebaseID).Scan(&analysisID)
		if err != nil {
			t.Fatalf("failed to create orphaned analysis: %v", err)
		}

		// Verify analysis exists and has no history reference
		var countBefore int
		err = pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM analyses a
			LEFT JOIN user_analysis_history uah ON a.id = uah.analysis_id
			WHERE a.id = $1 AND uah.analysis_id IS NULL
		`, analysisID).Scan(&countBefore)
		if err != nil {
			t.Fatalf("failed to query orphaned analysis: %v", err)
		}
		if countBefore != 1 {
			t.Fatalf("expected 1 orphaned analysis, got %d", countBefore)
		}

		// Delete orphaned analyses
		result, err := retentionRepo.DeleteOrphanedAnalyses(ctx, 100)
		if err != nil {
			t.Fatalf("DeleteOrphanedAnalyses failed: %v", err)
		}

		if result.DeletedCount != 1 {
			t.Errorf("expected 1 deleted record, got %d", result.DeletedCount)
		}

		// Verify analysis is deleted
		var countAfter int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM analyses WHERE id = $1", analysisID).Scan(&countAfter)
		if err != nil {
			t.Fatalf("failed to query analysis after: %v", err)
		}
		if countAfter != 0 {
			t.Errorf("expected 0 analyses after delete, got %d", countAfter)
		}
	})

	t.Run("should not delete analyses with user_analysis_history reference", func(t *testing.T) {
		_, err := pool.Exec(ctx, "TRUNCATE codebases CASCADE")
		if err != nil {
			t.Fatalf("failed to truncate: %v", err)
		}

		var userID string
		err = pool.QueryRow(ctx, "INSERT INTO users (email, username) VALUES ('ref-test@example.com', 'refuser') RETURNING id::text").Scan(&userID)
		if err != nil {
			t.Fatalf("failed to create test user: %v", err)
		}

		var codebaseID pgtype.UUID
		err = pool.QueryRow(ctx, `
			INSERT INTO codebases (host, owner, name, external_repo_id)
			VALUES ('github.com', 'ref-owner', 'ref-repo', 'ref-ext-1')
			RETURNING id
		`).Scan(&codebaseID)
		if err != nil {
			t.Fatalf("failed to create codebase: %v", err)
		}

		// Create analysis with user_analysis_history reference
		var analysisID pgtype.UUID
		err = pool.QueryRow(ctx, `
			INSERT INTO analyses (codebase_id, commit_sha, status, parser_version, created_at)
			VALUES ($1, 'ref123', 'completed', 'v1.0.0', now() - interval '2 days')
			RETURNING id
		`, codebaseID).Scan(&analysisID)
		if err != nil {
			t.Fatalf("failed to create analysis: %v", err)
		}

		// Create user_analysis_history reference
		_, err = pool.Exec(ctx, `
			INSERT INTO user_analysis_history (user_id, analysis_id)
			VALUES ($1, $2)
		`, userID, analysisID)
		if err != nil {
			t.Fatalf("failed to create user_analysis_history: %v", err)
		}

		// Delete orphaned analyses
		result, err := retentionRepo.DeleteOrphanedAnalyses(ctx, 100)
		if err != nil {
			t.Fatalf("DeleteOrphanedAnalyses failed: %v", err)
		}

		if result.DeletedCount != 0 {
			t.Errorf("expected 0 deleted records (has reference), got %d", result.DeletedCount)
		}

		// Verify analysis still exists
		var countAfter int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM analyses WHERE id = $1", analysisID).Scan(&countAfter)
		if err != nil {
			t.Fatalf("failed to query analysis after: %v", err)
		}
		if countAfter != 1 {
			t.Errorf("expected 1 analysis (not deleted), got %d", countAfter)
		}
	})

	t.Run("should not delete recent orphaned analyses (less than 1 day old)", func(t *testing.T) {
		_, err := pool.Exec(ctx, "TRUNCATE codebases CASCADE")
		if err != nil {
			t.Fatalf("failed to truncate: %v", err)
		}

		var codebaseID pgtype.UUID
		err = pool.QueryRow(ctx, `
			INSERT INTO codebases (host, owner, name, external_repo_id)
			VALUES ('github.com', 'recent-owner', 'recent-repo', 'recent-ext-1')
			RETURNING id
		`).Scan(&codebaseID)
		if err != nil {
			t.Fatalf("failed to create codebase: %v", err)
		}

		// Create recent orphaned analysis (less than 1 day old)
		var analysisID pgtype.UUID
		err = pool.QueryRow(ctx, `
			INSERT INTO analyses (codebase_id, commit_sha, status, parser_version, created_at)
			VALUES ($1, 'recent123', 'completed', 'v1.0.0', now() - interval '1 hour')
			RETURNING id
		`, codebaseID).Scan(&analysisID)
		if err != nil {
			t.Fatalf("failed to create recent orphaned analysis: %v", err)
		}

		// Delete orphaned analyses
		result, err := retentionRepo.DeleteOrphanedAnalyses(ctx, 100)
		if err != nil {
			t.Fatalf("DeleteOrphanedAnalyses failed: %v", err)
		}

		if result.DeletedCount != 0 {
			t.Errorf("expected 0 deleted records (too recent), got %d", result.DeletedCount)
		}

		// Verify analysis still exists
		var countAfter int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM analyses WHERE id = $1", analysisID).Scan(&countAfter)
		if err != nil {
			t.Fatalf("failed to query analysis after: %v", err)
		}
		if countAfter != 1 {
			t.Errorf("expected 1 analysis (not deleted, too recent), got %d", countAfter)
		}
	})
}

func TestRetentionRepository_DefaultBatchSize(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	retentionRepo := NewRetentionRepository(pool)
	ctx := context.Background()

	t.Run("should use default batch size when 0 or negative", func(t *testing.T) {
		// These should not error even with 0 or negative batch size
		_, err := retentionRepo.DeleteExpiredUserAnalysisHistory(ctx, 0)
		if err != nil {
			t.Errorf("DeleteExpiredUserAnalysisHistory with 0 batch size failed: %v", err)
		}

		_, err = retentionRepo.DeleteExpiredSpecDocuments(ctx, -1)
		if err != nil {
			t.Errorf("DeleteExpiredSpecDocuments with negative batch size failed: %v", err)
		}

		_, err = retentionRepo.DeleteOrphanedAnalyses(ctx, 0)
		if err != nil {
			t.Errorf("DeleteOrphanedAnalyses with 0 batch size failed: %v", err)
		}
	})
}

