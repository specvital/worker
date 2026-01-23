package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/specvital/core/pkg/domain"
	"github.com/specvital/core/pkg/parser"
	"github.com/specvital/worker/internal/domain/specview"
	testdb "github.com/specvital/worker/internal/testutil/postgres"
)

func TestSpecDocumentRepository_GetTestDataByAnalysisID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	analysisRepo := NewAnalysisRepository(pool)
	specRepo := NewSpecDocumentRepository(pool)
	ctx := context.Background()

	t.Run("should return ErrAnalysisNotFound for non-existent analysis", func(t *testing.T) {
		_, err := specRepo.GetTestDataByAnalysisID(ctx, "00000000-0000-0000-0000-000000000000")
		if !errors.Is(err, specview.ErrAnalysisNotFound) {
			t.Errorf("expected ErrAnalysisNotFound, got %v", err)
		}
	})

	t.Run("should return ErrInvalidInput for invalid UUID format", func(t *testing.T) {
		_, err := specRepo.GetTestDataByAnalysisID(ctx, "invalid-uuid")
		if err == nil {
			t.Error("expected error for invalid UUID")
		}
	})

	t.Run("should return test data with suite paths", func(t *testing.T) {
		analysisID := setupTestAnalysisWithNestedSuites(t, ctx, analysisRepo, pool)

		files, err := specRepo.GetTestDataByAnalysisID(ctx, analysisID.String())
		if err != nil {
			t.Fatalf("GetTestDataByAnalysisID failed: %v", err)
		}

		if len(files) == 0 {
			t.Fatal("expected at least one file")
		}

		foundTest := false
		for _, file := range files {
			for _, test := range file.Tests {
				if test.Name == "TestNestedCreate" {
					foundTest = true
					if test.SuitePath == "" {
						t.Error("expected non-empty suite path for nested test")
					}
					if test.TestCaseID == "" {
						t.Error("expected non-empty TestCaseID")
					}
				}
			}
		}

		if !foundTest {
			t.Error("TestNestedCreate not found in results")
		}
	})
}

func TestSpecDocumentRepository_SaveDocument(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	analysisRepo := NewAnalysisRepository(pool)
	specRepo := NewSpecDocumentRepository(pool)
	ctx := context.Background()

	t.Run("should save complete 4-table hierarchy", func(t *testing.T) {
		analysisID := setupTestAnalysisWithNestedSuites(t, ctx, analysisRepo, pool)
		userID := setupTestUser(t, ctx, pool)

		files, err := specRepo.GetTestDataByAnalysisID(ctx, analysisID.String())
		if err != nil {
			t.Fatalf("GetTestDataByAnalysisID failed: %v", err)
		}

		var testCaseID string
		if len(files) > 0 && len(files[0].Tests) > 0 {
			testCaseID = files[0].Tests[0].TestCaseID
		}

		doc := &specview.SpecDocument{
			AnalysisID:  analysisID.String(),
			ContentHash: []byte("test-hash-123"),
			Language:    "English",
			ModelID:     "gemini-2.5-flash",
			UserID:      userID,
			Domains: []specview.Domain{
				{
					Name:        "User Management",
					Description: "Handles user-related functionality",
					Confidence:  0.95,
					Features: []specview.Feature{
						{
							Name:        "User Creation",
							Description: "Tests for user creation flow",
							Confidence:  0.90,
							Behaviors: []specview.Behavior{
								{
									OriginalName: "TestNestedCreate",
									Description:  "사용자가 생성되어야 한다",
									Confidence:   0.85,
									TestCaseID:   testCaseID,
								},
							},
						},
					},
				},
			},
		}

		err = specRepo.SaveDocument(ctx, doc)
		if err != nil {
			t.Fatalf("SaveDocument failed: %v", err)
		}

		if doc.ID == "" {
			t.Error("expected document ID to be set after save")
		}

		var docCount int
		pool.QueryRow(ctx, "SELECT COUNT(*) FROM spec_documents").Scan(&docCount)
		if docCount != 1 {
			t.Errorf("expected 1 document, got %d", docCount)
		}

		var domainCount int
		pool.QueryRow(ctx, "SELECT COUNT(*) FROM spec_domains").Scan(&domainCount)
		if domainCount != 1 {
			t.Errorf("expected 1 domain, got %d", domainCount)
		}

		var featureCount int
		pool.QueryRow(ctx, "SELECT COUNT(*) FROM spec_features").Scan(&featureCount)
		if featureCount != 1 {
			t.Errorf("expected 1 feature, got %d", featureCount)
		}

		var behaviorCount int
		pool.QueryRow(ctx, "SELECT COUNT(*) FROM spec_behaviors").Scan(&behaviorCount)
		if behaviorCount != 1 {
			t.Errorf("expected 1 behavior, got %d", behaviorCount)
		}
	})

	t.Run("should return nil for non-existent content hash", func(t *testing.T) {
		userID := setupTestUser(t, ctx, pool)
		doc, err := specRepo.FindDocumentByContentHash(ctx, userID, []byte("non-existent"), "English", "model")
		if err != nil {
			t.Fatalf("FindDocumentByContentHash failed: %v", err)
		}
		if doc != nil {
			t.Error("expected nil for non-existent hash")
		}
	})
}

func TestSpecDocumentRepository_VersionManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	analysisRepo := NewAnalysisRepository(pool)
	specRepo := NewSpecDocumentRepository(pool)
	ctx := context.Background()

	t.Run("should auto-increment version on save", func(t *testing.T) {
		analysisID := setupTestAnalysisWithNestedSuites(t, ctx, analysisRepo, pool)
		userID := setupTestUser(t, ctx, pool)

		doc1 := &specview.SpecDocument{
			AnalysisID:  analysisID.String(),
			ContentHash: []byte("hash-v1"),
			Language:    "Korean",
			ModelID:     "gemini-2.5-flash",
			UserID:      userID,
			Domains:     []specview.Domain{},
		}

		if err := specRepo.SaveDocument(ctx, doc1); err != nil {
			t.Fatalf("SaveDocument v1 failed: %v", err)
		}

		var version1 int32
		pool.QueryRow(ctx, "SELECT version FROM spec_documents WHERE id = $1", doc1.ID).Scan(&version1)
		if version1 != 1 {
			t.Errorf("expected version 1, got %d", version1)
		}

		doc2 := &specview.SpecDocument{
			AnalysisID:  analysisID.String(),
			ContentHash: []byte("hash-v2"),
			Language:    "Korean",
			ModelID:     "gemini-2.5-flash",
			UserID:      userID,
			Domains:     []specview.Domain{},
		}

		if err := specRepo.SaveDocument(ctx, doc2); err != nil {
			t.Fatalf("SaveDocument v2 failed: %v", err)
		}

		var version2 int32
		pool.QueryRow(ctx, "SELECT version FROM spec_documents WHERE id = $1", doc2.ID).Scan(&version2)
		if version2 != 2 {
			t.Errorf("expected version 2, got %d", version2)
		}
	})

	t.Run("should find only latest version by content hash", func(t *testing.T) {
		analysisID := setupTestAnalysisWithNestedSuites(t, ctx, analysisRepo, pool)
		userID := setupTestUser(t, ctx, pool)
		contentHash := []byte("shared-hash-for-version-test")

		doc1 := &specview.SpecDocument{
			AnalysisID:  analysisID.String(),
			ContentHash: contentHash,
			Language:    "English",
			ModelID:     "gemini-2.5-flash",
			UserID:      userID,
			Domains:     []specview.Domain{},
		}
		if err := specRepo.SaveDocument(ctx, doc1); err != nil {
			t.Fatalf("SaveDocument v1 failed: %v", err)
		}

		doc2 := &specview.SpecDocument{
			AnalysisID:  analysisID.String(),
			ContentHash: contentHash,
			Language:    "English",
			ModelID:     "gemini-2.5-flash",
			UserID:      userID,
			Domains:     []specview.Domain{},
		}
		if err := specRepo.SaveDocument(ctx, doc2); err != nil {
			t.Fatalf("SaveDocument v2 failed: %v", err)
		}

		found, err := specRepo.FindDocumentByContentHash(ctx, userID, contentHash, "English", "gemini-2.5-flash")
		if err != nil {
			t.Fatalf("FindDocumentByContentHash failed: %v", err)
		}

		if found == nil {
			t.Fatal("expected to find document")
		}

		if found.ID != doc2.ID {
			t.Errorf("expected latest version ID %s, got %s", doc2.ID, found.ID)
		}

		if found.Version != 2 {
			t.Errorf("expected version 2, got %d", found.Version)
		}
	})
}

func TestSpecDocumentRepository_FindDocumentByContentHash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	analysisRepo := NewAnalysisRepository(pool)
	specRepo := NewSpecDocumentRepository(pool)
	ctx := context.Background()

	t.Run("should find existing document by content hash", func(t *testing.T) {
		analysisID := setupTestAnalysisWithNestedSuites(t, ctx, analysisRepo, pool)
		userID := setupTestUser(t, ctx, pool)
		contentHash := []byte("unique-hash-for-find-test")

		doc := &specview.SpecDocument{
			AnalysisID:  analysisID.String(),
			ContentHash: contentHash,
			Language:    "Korean",
			ModelID:     "gemini-2.5-flash",
			UserID:      userID,
			Domains:     []specview.Domain{},
		}

		err := specRepo.SaveDocument(ctx, doc)
		if err != nil {
			t.Fatalf("SaveDocument failed: %v", err)
		}

		found, err := specRepo.FindDocumentByContentHash(ctx, userID, contentHash, "Korean", "gemini-2.5-flash")
		if err != nil {
			t.Fatalf("FindDocumentByContentHash failed: %v", err)
		}

		if found == nil {
			t.Fatal("expected to find document")
		}

		if found.ID != doc.ID {
			t.Errorf("expected ID %s, got %s", doc.ID, found.ID)
		}
	})

	t.Run("should not find document with different language", func(t *testing.T) {
		analysisID := setupTestAnalysisWithNestedSuites(t, ctx, analysisRepo, pool)
		userID := setupTestUser(t, ctx, pool)
		contentHash := []byte("hash-for-lang-test")

		doc := &specview.SpecDocument{
			AnalysisID:  analysisID.String(),
			ContentHash: contentHash,
			Language:    "English",
			ModelID:     "gemini-2.5-flash",
			UserID:      userID,
			Domains:     []specview.Domain{},
		}

		err := specRepo.SaveDocument(ctx, doc)
		if err != nil {
			t.Fatalf("SaveDocument failed: %v", err)
		}

		found, err := specRepo.FindDocumentByContentHash(ctx, userID, contentHash, "Korean", "gemini-2.5-flash")
		if err != nil {
			t.Fatalf("FindDocumentByContentHash failed: %v", err)
		}

		if found != nil {
			t.Error("should not find document with different language")
		}
	})

	t.Run("should not find document owned by different user (user isolation)", func(t *testing.T) {
		analysisID := setupTestAnalysisWithNestedSuites(t, ctx, analysisRepo, pool)
		userA := setupTestUser(t, ctx, pool)
		userB := setupTestUser(t, ctx, pool)
		contentHash := []byte("hash-for-user-isolation-test")

		doc := &specview.SpecDocument{
			AnalysisID:  analysisID.String(),
			ContentHash: contentHash,
			Language:    "English",
			ModelID:     "gemini-2.5-flash",
			UserID:      userA,
			Domains:     []specview.Domain{},
		}

		err := specRepo.SaveDocument(ctx, doc)
		if err != nil {
			t.Fatalf("SaveDocument failed: %v", err)
		}

		found, err := specRepo.FindDocumentByContentHash(ctx, userB, contentHash, "English", "gemini-2.5-flash")
		if err != nil {
			t.Fatalf("FindDocumentByContentHash failed: %v", err)
		}

		if found != nil {
			t.Error("should not find document owned by different user")
		}

		foundByOwner, err := specRepo.FindDocumentByContentHash(ctx, userA, contentHash, "English", "gemini-2.5-flash")
		if err != nil {
			t.Fatalf("FindDocumentByContentHash failed: %v", err)
		}

		if foundByOwner == nil {
			t.Error("owner should be able to find their own document")
		}
	})
}

func setupTestUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
	t.Helper()

	randBytes := make([]byte, 4)
	if _, err := rand.Read(randBytes); err != nil {
		t.Fatalf("failed to generate random bytes: %v", err)
	}
	randSuffix := hex.EncodeToString(randBytes)
	username := "testuser" + randSuffix

	var userID [16]byte
	err := pool.QueryRow(ctx, `
		INSERT INTO users (username)
		VALUES ($1)
		RETURNING id
	`, username).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	return uuidBytesToString(userID)
}

func setupTestAnalysisWithNestedSuites(t *testing.T, ctx context.Context, repo *AnalysisRepository, pool *pgxpool.Pool) *uuidWrapper {
	t.Helper()

	randBytes := make([]byte, 4)
	if _, err := rand.Read(randBytes); err != nil {
		t.Fatalf("failed to generate random bytes: %v", err)
	}
	randSuffix := hex.EncodeToString(randBytes)

	shortName := t.Name()
	if len(shortName) > 20 {
		shortName = shortName[:20]
	}
	shortName = shortName + randSuffix

	params := SaveAnalysisResultParams{
		Owner:          "testowner",
		Repo:           "repo" + shortName,
		CommitSHA:      "abc123def456",
		Branch:         "main",
		ExternalRepoID: shortName,
		ParserVersion:  "v1.0.0-test",
		Result:         createNestedSuiteInventory(),
	}

	err := repo.SaveAnalysisResult(ctx, params)
	if err != nil {
		t.Fatalf("SaveAnalysisResult failed: %v", err)
	}

	var analysisID [16]byte
	err = pool.QueryRow(ctx, `
		SELECT a.id FROM analyses a
		JOIN codebases c ON c.id = a.codebase_id
		WHERE c.external_repo_id = $1
		ORDER BY a.created_at DESC
		LIMIT 1
	`, shortName).Scan(&analysisID)

	if err != nil {
		t.Fatalf("failed to get analysis ID: %v", err)
	}

	return &uuidWrapper{id: analysisID}
}

type uuidWrapper struct {
	id [16]byte
}

func (u *uuidWrapper) String() string {
	return uuidBytesToString(u.id)
}

func createNestedSuiteInventory() *parser.ScanResult {
	return &parser.ScanResult{
		Inventory: &domain.Inventory{
			Files: []domain.TestFile{
				{
					Path:      "src/user_test.go",
					Framework: "go-test",
					Suites: []domain.TestSuite{
						{
							Name: "UserService",
							Location: domain.Location{
								StartLine: 10,
							},
							Suites: []domain.TestSuite{
								{
									Name: "Create",
									Location: domain.Location{
										StartLine: 15,
									},
									Tests: []domain.Test{
										{
											Name:   "TestNestedCreate",
											Status: "",
											Location: domain.Location{
												StartLine: 20,
											},
										},
									},
								},
							},
							Tests: []domain.Test{
								{
									Name:   "TestTopLevel",
									Status: "",
									Location: domain.Location{
										StartLine: 12,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
