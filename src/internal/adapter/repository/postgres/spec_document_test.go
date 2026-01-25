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

func TestSpecDocumentRepository_BehaviorCache(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	specRepo := NewSpecDocumentRepository(pool)
	ctx := context.Background()

	t.Run("should return empty map for empty input", func(t *testing.T) {
		result, err := specRepo.FindCachedBehaviors(ctx, nil)
		if err != nil {
			t.Fatalf("FindCachedBehaviors failed: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d entries", len(result))
		}
	})

	t.Run("should return empty map for non-existent hashes", func(t *testing.T) {
		hashes := [][]byte{
			{0x01, 0x02, 0x03},
			{0x04, 0x05, 0x06},
		}

		result, err := specRepo.FindCachedBehaviors(ctx, hashes)
		if err != nil {
			t.Fatalf("FindCachedBehaviors failed: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected empty map for non-existent hashes, got %d entries", len(result))
		}
	})

	t.Run("should save and retrieve behavior cache", func(t *testing.T) {
		hash1 := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88}
		hash2 := []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00, 0x11}

		entries := []specview.BehaviorCacheEntry{
			{CacheKeyHash: hash1, Description: "사용자가 로그인할 수 있어야 한다"},
			{CacheKeyHash: hash2, Description: "이메일 형식이 유효해야 한다"},
		}

		err := specRepo.SaveBehaviorCache(ctx, entries)
		if err != nil {
			t.Fatalf("SaveBehaviorCache failed: %v", err)
		}

		result, err := specRepo.FindCachedBehaviors(ctx, [][]byte{hash1, hash2})
		if err != nil {
			t.Fatalf("FindCachedBehaviors failed: %v", err)
		}

		if len(result) != 2 {
			t.Errorf("expected 2 entries, got %d", len(result))
		}

		hexKey1 := "1122334455667788"
		if result[hexKey1] != "사용자가 로그인할 수 있어야 한다" {
			t.Errorf("unexpected description for hash1: %s", result[hexKey1])
		}

		hexKey2 := "aabbccddeeff0011"
		if result[hexKey2] != "이메일 형식이 유효해야 한다" {
			t.Errorf("unexpected description for hash2: %s", result[hexKey2])
		}
	})

	t.Run("should upsert on conflict", func(t *testing.T) {
		hash := []byte{0x99, 0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22}

		err := specRepo.SaveBehaviorCache(ctx, []specview.BehaviorCacheEntry{
			{CacheKeyHash: hash, Description: "original description"},
		})
		if err != nil {
			t.Fatalf("SaveBehaviorCache (first) failed: %v", err)
		}

		err = specRepo.SaveBehaviorCache(ctx, []specview.BehaviorCacheEntry{
			{CacheKeyHash: hash, Description: "updated description"},
		})
		if err != nil {
			t.Fatalf("SaveBehaviorCache (second) failed: %v", err)
		}

		result, err := specRepo.FindCachedBehaviors(ctx, [][]byte{hash})
		if err != nil {
			t.Fatalf("FindCachedBehaviors failed: %v", err)
		}

		hexKey := "9988776655443322"
		if result[hexKey] != "updated description" {
			t.Errorf("expected updated description, got: %s", result[hexKey])
		}
	})

	t.Run("should return partial matches", func(t *testing.T) {
		existingHash := []byte{0xde, 0xad, 0xbe, 0xef}
		nonExistingHash := []byte{0xca, 0xfe, 0xba, 0xbe}

		err := specRepo.SaveBehaviorCache(ctx, []specview.BehaviorCacheEntry{
			{CacheKeyHash: existingHash, Description: "existing entry"},
		})
		if err != nil {
			t.Fatalf("SaveBehaviorCache failed: %v", err)
		}

		result, err := specRepo.FindCachedBehaviors(ctx, [][]byte{existingHash, nonExistingHash})
		if err != nil {
			t.Fatalf("FindCachedBehaviors failed: %v", err)
		}

		if len(result) != 1 {
			t.Errorf("expected 1 entry (partial match), got %d", len(result))
		}

		hexKey := "deadbeef"
		if _, ok := result[hexKey]; !ok {
			t.Error("expected existing hash to be in result")
		}
	})

	t.Run("should do nothing for empty entries", func(t *testing.T) {
		err := specRepo.SaveBehaviorCache(ctx, nil)
		if err != nil {
			t.Fatalf("SaveBehaviorCache with nil should not fail: %v", err)
		}

		err = specRepo.SaveBehaviorCache(ctx, []specview.BehaviorCacheEntry{})
		if err != nil {
			t.Fatalf("SaveBehaviorCache with empty slice should not fail: %v", err)
		}
	})
}

func TestSpecDocumentRepository_ClassificationCache(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	specRepo := NewSpecDocumentRepository(pool)
	ctx := context.Background()

	t.Run("should return nil for non-existent cache", func(t *testing.T) {
		signature := []byte{0x01, 0x02, 0x03, 0x04}
		result, err := specRepo.FindClassificationCache(ctx, signature, "English", "model-1")
		if err != nil {
			t.Fatalf("FindClassificationCache failed: %v", err)
		}
		if result != nil {
			t.Error("expected nil for non-existent cache")
		}
	})

	t.Run("should save and retrieve classification cache", func(t *testing.T) {
		signature := []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
		cache := &specview.ClassificationCache{
			FileSignature: signature,
			Language:      "Korean",
			ModelID:       "gemini-2.5-flash",
			ClassificationResult: &specview.Phase1Output{
				Domains: []specview.DomainGroup{
					{
						Name:        "Authentication",
						Description: "User authentication features",
						Confidence:  0.95,
						Features: []specview.FeatureGroup{
							{
								Name:        "Login",
								Description: "User login functionality",
								Confidence:  0.90,
								TestIndices: []int{0, 1, 2},
							},
						},
					},
				},
			},
			TestIndexMap: map[string]specview.TestIdentity{
				specview.TestKey("auth_test.go", "AuthSuite", "test_login"): {
					DomainIndex:  0,
					FeatureIndex: 0,
					FilePath:     "auth_test.go",
					SuitePath:    "AuthSuite",
					TestIndex:    0,
				},
			},
		}

		err := specRepo.SaveClassificationCache(ctx, cache)
		if err != nil {
			t.Fatalf("SaveClassificationCache failed: %v", err)
		}

		result, err := specRepo.FindClassificationCache(ctx, signature, "Korean", "gemini-2.5-flash")
		if err != nil {
			t.Fatalf("FindClassificationCache failed: %v", err)
		}

		if result == nil {
			t.Fatal("expected to find cache")
		}

		if result.ID == "" {
			t.Error("expected non-empty ID")
		}
		if result.Language != "Korean" {
			t.Errorf("expected language Korean, got %s", result.Language)
		}
		if result.ModelID != "gemini-2.5-flash" {
			t.Errorf("expected model gemini-2.5-flash, got %s", result.ModelID)
		}
		if len(result.ClassificationResult.Domains) != 1 {
			t.Errorf("expected 1 domain, got %d", len(result.ClassificationResult.Domains))
		}
		if result.ClassificationResult.Domains[0].Name != "Authentication" {
			t.Errorf("expected domain name Authentication, got %s", result.ClassificationResult.Domains[0].Name)
		}
		if len(result.TestIndexMap) != 1 {
			t.Errorf("expected 1 test index map entry, got %d", len(result.TestIndexMap))
		}
	})

	t.Run("should upsert on conflict", func(t *testing.T) {
		signature := []byte{0x11, 0x22, 0x33, 0x44, 0x55}
		cache1 := &specview.ClassificationCache{
			FileSignature: signature,
			Language:      "English",
			ModelID:       "model-v1",
			ClassificationResult: &specview.Phase1Output{
				Domains: []specview.DomainGroup{
					{Name: "OriginalDomain", Confidence: 0.8},
				},
			},
			TestIndexMap: map[string]specview.TestIdentity{},
		}

		err := specRepo.SaveClassificationCache(ctx, cache1)
		if err != nil {
			t.Fatalf("SaveClassificationCache (first) failed: %v", err)
		}

		cache2 := &specview.ClassificationCache{
			FileSignature: signature,
			Language:      "English",
			ModelID:       "model-v1",
			ClassificationResult: &specview.Phase1Output{
				Domains: []specview.DomainGroup{
					{Name: "UpdatedDomain", Confidence: 0.95},
				},
			},
			TestIndexMap: map[string]specview.TestIdentity{},
		}

		err = specRepo.SaveClassificationCache(ctx, cache2)
		if err != nil {
			t.Fatalf("SaveClassificationCache (second) failed: %v", err)
		}

		result, err := specRepo.FindClassificationCache(ctx, signature, "English", "model-v1")
		if err != nil {
			t.Fatalf("FindClassificationCache failed: %v", err)
		}

		if result.ClassificationResult.Domains[0].Name != "UpdatedDomain" {
			t.Errorf("expected updated domain name, got %s", result.ClassificationResult.Domains[0].Name)
		}
	})

	t.Run("should distinguish by language", func(t *testing.T) {
		signature := []byte{0x77, 0x88, 0x99, 0xaa}

		cacheEn := &specview.ClassificationCache{
			FileSignature: signature,
			Language:      "English",
			ModelID:       "model-test",
			ClassificationResult: &specview.Phase1Output{
				Domains: []specview.DomainGroup{{Name: "EnglishDomain"}},
			},
			TestIndexMap: map[string]specview.TestIdentity{},
		}

		cacheKo := &specview.ClassificationCache{
			FileSignature: signature,
			Language:      "Korean",
			ModelID:       "model-test",
			ClassificationResult: &specview.Phase1Output{
				Domains: []specview.DomainGroup{{Name: "KoreanDomain"}},
			},
			TestIndexMap: map[string]specview.TestIdentity{},
		}

		if err := specRepo.SaveClassificationCache(ctx, cacheEn); err != nil {
			t.Fatalf("SaveClassificationCache (English) failed: %v", err)
		}
		if err := specRepo.SaveClassificationCache(ctx, cacheKo); err != nil {
			t.Fatalf("SaveClassificationCache (Korean) failed: %v", err)
		}

		resultEn, err := specRepo.FindClassificationCache(ctx, signature, "English", "model-test")
		if err != nil {
			t.Fatalf("FindClassificationCache (English) failed: %v", err)
		}
		if resultEn.ClassificationResult.Domains[0].Name != "EnglishDomain" {
			t.Errorf("expected EnglishDomain, got %s", resultEn.ClassificationResult.Domains[0].Name)
		}

		resultKo, err := specRepo.FindClassificationCache(ctx, signature, "Korean", "model-test")
		if err != nil {
			t.Fatalf("FindClassificationCache (Korean) failed: %v", err)
		}
		if resultKo.ClassificationResult.Domains[0].Name != "KoreanDomain" {
			t.Errorf("expected KoreanDomain, got %s", resultKo.ClassificationResult.Domains[0].Name)
		}
	})

	t.Run("should distinguish by model_id", func(t *testing.T) {
		signature := []byte{0xde, 0xad, 0xbe, 0xef}

		cacheV1 := &specview.ClassificationCache{
			FileSignature: signature,
			Language:      "English",
			ModelID:       "model-v1",
			ClassificationResult: &specview.Phase1Output{
				Domains: []specview.DomainGroup{{Name: "V1Domain"}},
			},
			TestIndexMap: map[string]specview.TestIdentity{},
		}

		cacheV2 := &specview.ClassificationCache{
			FileSignature: signature,
			Language:      "English",
			ModelID:       "model-v2",
			ClassificationResult: &specview.Phase1Output{
				Domains: []specview.DomainGroup{{Name: "V2Domain"}},
			},
			TestIndexMap: map[string]specview.TestIdentity{},
		}

		if err := specRepo.SaveClassificationCache(ctx, cacheV1); err != nil {
			t.Fatalf("SaveClassificationCache (v1) failed: %v", err)
		}
		if err := specRepo.SaveClassificationCache(ctx, cacheV2); err != nil {
			t.Fatalf("SaveClassificationCache (v2) failed: %v", err)
		}

		resultV1, err := specRepo.FindClassificationCache(ctx, signature, "English", "model-v1")
		if err != nil {
			t.Fatalf("FindClassificationCache (v1) failed: %v", err)
		}
		if resultV1.ClassificationResult.Domains[0].Name != "V1Domain" {
			t.Errorf("expected V1Domain, got %s", resultV1.ClassificationResult.Domains[0].Name)
		}

		resultV2, err := specRepo.FindClassificationCache(ctx, signature, "English", "model-v2")
		if err != nil {
			t.Fatalf("FindClassificationCache (v2) failed: %v", err)
		}
		if resultV2.ClassificationResult.Domains[0].Name != "V2Domain" {
			t.Errorf("expected V2Domain, got %s", resultV2.ClassificationResult.Domains[0].Name)
		}
	})

	t.Run("should fail on nil cache", func(t *testing.T) {
		err := specRepo.SaveClassificationCache(ctx, nil)
		if !errors.Is(err, specview.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("should fail on empty file signature", func(t *testing.T) {
		cache := &specview.ClassificationCache{
			FileSignature:        nil,
			Language:             "English",
			ModelID:              "model",
			ClassificationResult: &specview.Phase1Output{},
			TestIndexMap:         map[string]specview.TestIdentity{},
		}
		err := specRepo.SaveClassificationCache(ctx, cache)
		if !errors.Is(err, specview.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("should fail on empty language", func(t *testing.T) {
		cache := &specview.ClassificationCache{
			FileSignature:        []byte{0x01},
			Language:             "",
			ModelID:              "model",
			ClassificationResult: &specview.Phase1Output{},
			TestIndexMap:         map[string]specview.TestIdentity{},
		}
		err := specRepo.SaveClassificationCache(ctx, cache)
		if !errors.Is(err, specview.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("should fail on empty model ID", func(t *testing.T) {
		cache := &specview.ClassificationCache{
			FileSignature:        []byte{0x01},
			Language:             "English",
			ModelID:              "",
			ClassificationResult: &specview.Phase1Output{},
			TestIndexMap:         map[string]specview.TestIdentity{},
		}
		err := specRepo.SaveClassificationCache(ctx, cache)
		if !errors.Is(err, specview.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("should fail on nil classification result", func(t *testing.T) {
		cache := &specview.ClassificationCache{
			FileSignature:        []byte{0x01},
			Language:             "English",
			ModelID:              "model",
			ClassificationResult: nil,
			TestIndexMap:         map[string]specview.TestIdentity{},
		}
		err := specRepo.SaveClassificationCache(ctx, cache)
		if !errors.Is(err, specview.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput, got %v", err)
		}
	})
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
