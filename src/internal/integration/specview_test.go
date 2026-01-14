package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/specvital/core/pkg/domain"
	"github.com/specvital/core/pkg/parser"
	"github.com/specvital/worker/internal/adapter/repository/postgres"
	"github.com/specvital/worker/internal/domain/specview"
	specviewuc "github.com/specvital/worker/internal/usecase/specview"
	testdb "github.com/specvital/worker/internal/testutil/postgres"
)

// mockAIProvider implements specview.AIProvider for integration testing.
type mockAIProvider struct {
	classifyDomainsFn  func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error)
	convertTestNamesFn func(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, *specview.TokenUsage, error)
}

func (m *mockAIProvider) ClassifyDomains(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
	if m.classifyDomainsFn != nil {
		return m.classifyDomainsFn(ctx, input)
	}
	return defaultPhase1Output(input), nil, nil
}

func (m *mockAIProvider) ConvertTestNames(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, *specview.TokenUsage, error) {
	if m.convertTestNamesFn != nil {
		return m.convertTestNamesFn(ctx, input)
	}
	return defaultPhase2Output(input), nil, nil
}

func defaultPhase1Output(input specview.Phase1Input) *specview.Phase1Output {
	var allIndices []int
	for _, file := range input.Files {
		for _, test := range file.Tests {
			allIndices = append(allIndices, test.Index)
		}
	}

	return &specview.Phase1Output{
		Domains: []specview.DomainGroup{
			{
				Name:        "Core Domain",
				Description: "Core functionality",
				Confidence:  0.95,
				Features: []specview.FeatureGroup{
					{
						Name:        "Main Feature",
						Description: "Main feature tests",
						Confidence:  0.90,
						TestIndices: allIndices,
					},
				},
			},
		},
	}
}

func defaultPhase2Output(input specview.Phase2Input) *specview.Phase2Output {
	behaviors := make([]specview.BehaviorSpec, len(input.Tests))
	for i, test := range input.Tests {
		behaviors[i] = specview.BehaviorSpec{
			TestIndex:   test.Index,
			Description: "사용자가 " + test.Name + " 기능을 테스트한다",
			Confidence:  0.9,
		}
	}
	return &specview.Phase2Output{Behaviors: behaviors}
}

// TestSpecViewIntegration_SmallRepo tests complete pipeline with a small repository (10 tests).
func TestSpecViewIntegration_SmallRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	analysisRepo := postgres.NewAnalysisRepository(pool)
	specRepo := postgres.NewSpecDocumentRepository(pool)

	analysisID := setupAnalysisWithTests(t, ctx, analysisRepo, pool, 2, 5)

	aiProvider := &mockAIProvider{}
	uc := specviewuc.NewGenerateSpecViewUseCase(specRepo, aiProvider, "test-model")

	req := specview.SpecViewRequest{
		AnalysisID: analysisID,
		Language:   "Korean",
	}

	result, err := uc.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.CacheHit {
		t.Error("expected cache miss on first execution")
	}
	if result.DocumentID == "" {
		t.Error("expected document ID to be set")
	}
	if len(result.ContentHash) == 0 {
		t.Error("expected content hash to be set")
	}

	verifyDocumentSaved(t, ctx, pool, result.DocumentID)
}

// TestSpecViewIntegration_MediumRepo tests complete pipeline with a medium repository (100 tests).
func TestSpecViewIntegration_MediumRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	analysisRepo := postgres.NewAnalysisRepository(pool)
	specRepo := postgres.NewSpecDocumentRepository(pool)

	analysisID := setupAnalysisWithTests(t, ctx, analysisRepo, pool, 10, 10)

	var phase2CallCount atomic.Int32
	aiProvider := &mockAIProvider{
		classifyDomainsFn: func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
			return multiDomainPhase1Output(input), nil, nil
		},
		convertTestNamesFn: func(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, *specview.TokenUsage, error) {
			phase2CallCount.Add(1)
			time.Sleep(10 * time.Millisecond) // Simulate AI latency
			return defaultPhase2Output(input), nil, nil
		},
	}

	uc := specviewuc.NewGenerateSpecViewUseCase(specRepo, aiProvider, "test-model")

	req := specview.SpecViewRequest{
		AnalysisID: analysisID,
		Language:   "English",
	}

	start := time.Now()
	result, err := uc.Execute(ctx, req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.CacheHit {
		t.Error("expected cache miss")
	}

	if phase2CallCount.Load() < 2 {
		t.Errorf("expected multiple Phase 2 calls, got %d", phase2CallCount.Load())
	}

	// Verify parallelism - with 5 concurrent workers, 10 features should complete faster
	// than sequential execution (10 * 10ms = 100ms sequential, ~20ms parallel)
	if elapsed > 500*time.Millisecond {
		t.Logf("warning: Phase 2 took %v, expected parallel execution to be faster", elapsed)
	}

	verifyDocumentSaved(t, ctx, pool, result.DocumentID)
}

// TestSpecViewIntegration_CacheHit tests cache hit scenario.
func TestSpecViewIntegration_CacheHit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	analysisRepo := postgres.NewAnalysisRepository(pool)
	specRepo := postgres.NewSpecDocumentRepository(pool)

	analysisID := setupAnalysisWithTests(t, ctx, analysisRepo, pool, 1, 3)

	var phase1CallCount atomic.Int32
	aiProvider := &mockAIProvider{
		classifyDomainsFn: func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
			phase1CallCount.Add(1)
			return defaultPhase1Output(input), nil, nil
		},
	}

	uc := specviewuc.NewGenerateSpecViewUseCase(specRepo, aiProvider, "test-model")

	req := specview.SpecViewRequest{
		AnalysisID: analysisID,
		Language:   "Korean",
	}

	// First execution
	result1, err := uc.Execute(ctx, req)
	if err != nil {
		t.Fatalf("first Execute failed: %v", err)
	}

	if result1.CacheHit {
		t.Error("expected cache miss on first execution")
	}
	if phase1CallCount.Load() != 1 {
		t.Errorf("expected 1 Phase 1 call on first execution, got %d", phase1CallCount.Load())
	}

	// Second execution - should hit cache
	result2, err := uc.Execute(ctx, req)
	if err != nil {
		t.Fatalf("second Execute failed: %v", err)
	}

	if !result2.CacheHit {
		t.Error("expected cache hit on second execution")
	}
	if phase1CallCount.Load() != 1 {
		t.Errorf("expected no additional Phase 1 calls, got %d total", phase1CallCount.Load())
	}
	if result2.DocumentID != result1.DocumentID {
		t.Errorf("expected same document ID, got %s and %s", result1.DocumentID, result2.DocumentID)
	}
}

// TestSpecViewIntegration_AIFailureWithFallback tests AI failure with fallback conversion.
func TestSpecViewIntegration_AIFailureWithFallback(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	analysisRepo := postgres.NewAnalysisRepository(pool)
	specRepo := postgres.NewSpecDocumentRepository(pool)

	analysisID := setupAnalysisWithTests(t, ctx, analysisRepo, pool, 3, 3)

	var phase2CallCount atomic.Int32
	aiProvider := &mockAIProvider{
		classifyDomainsFn: func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
			return multiFeaturePhase1Output(input, 3), nil, nil
		},
		convertTestNamesFn: func(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, *specview.TokenUsage, error) {
			count := phase2CallCount.Add(1)
			// Fail first call only
			if count == 1 {
				return nil, nil, errors.New("AI service temporarily unavailable")
			}
			return defaultPhase2Output(input), nil, nil
		},
	}

	// Use default threshold (50%) - 1 out of 3 features failing should succeed
	uc := specviewuc.NewGenerateSpecViewUseCase(specRepo, aiProvider, "test-model")

	req := specview.SpecViewRequest{
		AnalysisID: analysisID,
		Language:   "Korean",
	}

	result, err := uc.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.CacheHit {
		t.Error("expected cache miss")
	}

	// Document should be saved with fallback behaviors for failed feature
	verifyDocumentSaved(t, ctx, pool, result.DocumentID)
}

// TestSpecViewIntegration_PartialPhase2Failure tests job failure when >50% features fail.
func TestSpecViewIntegration_PartialPhase2Failure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	analysisRepo := postgres.NewAnalysisRepository(pool)
	specRepo := postgres.NewSpecDocumentRepository(pool)

	analysisID := setupAnalysisWithTests(t, ctx, analysisRepo, pool, 4, 2)

	aiProvider := &mockAIProvider{
		classifyDomainsFn: func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
			return multiFeaturePhase1Output(input, 4), nil, nil
		},
		convertTestNamesFn: func(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, *specview.TokenUsage, error) {
			// All calls fail
			return nil, nil, errors.New("AI service unavailable")
		},
	}

	// Use 30% threshold - 4 out of 4 features failing should fail the job
	uc := specviewuc.NewGenerateSpecViewUseCase(specRepo, aiProvider, "test-model",
		specviewuc.WithFailureThreshold(0.3),
	)

	req := specview.SpecViewRequest{
		AnalysisID: analysisID,
		Language:   "Korean",
	}

	_, err := uc.Execute(ctx, req)
	if err == nil {
		t.Fatal("expected error when >50% features fail")
	}

	if !errors.Is(err, specviewuc.ErrPartialFeatureFailure) {
		t.Errorf("expected ErrPartialFeatureFailure, got %v", err)
	}

	// No document should be saved
	var docCount int
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM spec_documents").Scan(&docCount)
	if docCount != 0 {
		t.Errorf("expected 0 documents saved on failure, got %d", docCount)
	}
}

// TestSpecViewIntegration_Phase1Failure tests job failure when Phase 1 fails.
func TestSpecViewIntegration_Phase1Failure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	analysisRepo := postgres.NewAnalysisRepository(pool)
	specRepo := postgres.NewSpecDocumentRepository(pool)

	analysisID := setupAnalysisWithTests(t, ctx, analysisRepo, pool, 1, 3)

	aiProvider := &mockAIProvider{
		classifyDomainsFn: func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
			return nil, nil, errors.New("Phase 1 AI unavailable")
		},
	}

	uc := specviewuc.NewGenerateSpecViewUseCase(specRepo, aiProvider, "test-model")

	req := specview.SpecViewRequest{
		AnalysisID: analysisID,
		Language:   "Korean",
	}

	_, err := uc.Execute(ctx, req)
	if err == nil {
		t.Fatal("expected error when Phase 1 fails")
	}

	if !errors.Is(err, specviewuc.ErrAIProcessingFailed) {
		t.Errorf("expected ErrAIProcessingFailed, got %v", err)
	}
}

// TestSpecViewIntegration_AnalysisNotFound tests error handling for non-existent analysis.
func TestSpecViewIntegration_AnalysisNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	specRepo := postgres.NewSpecDocumentRepository(pool)

	aiProvider := &mockAIProvider{}
	uc := specviewuc.NewGenerateSpecViewUseCase(specRepo, aiProvider, "test-model")

	req := specview.SpecViewRequest{
		AnalysisID: "00000000-0000-0000-0000-000000000000",
		Language:   "Korean",
	}

	_, err := uc.Execute(ctx, req)
	if err == nil {
		t.Fatal("expected error for non-existent analysis")
	}

	if !errors.Is(err, specviewuc.ErrLoadInventoryFailed) {
		t.Errorf("expected ErrLoadInventoryFailed, got %v", err)
	}
}

// TestSpecViewIntegration_DifferentLanguages tests that different languages produce different documents.
func TestSpecViewIntegration_DifferentLanguages(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	analysisRepo := postgres.NewAnalysisRepository(pool)
	specRepo := postgres.NewSpecDocumentRepository(pool)

	analysisID := setupAnalysisWithTests(t, ctx, analysisRepo, pool, 1, 3)

	aiProvider := &mockAIProvider{}
	uc := specviewuc.NewGenerateSpecViewUseCase(specRepo, aiProvider, "test-model")

	// Execute with Korean
	reqKO := specview.SpecViewRequest{
		AnalysisID: analysisID,
		Language:   "Korean",
	}
	resultKO, err := uc.Execute(ctx, reqKO)
	if err != nil {
		t.Fatalf("Execute KO failed: %v", err)
	}

	// Execute with English - should create new document
	reqEN := specview.SpecViewRequest{
		AnalysisID: analysisID,
		Language:   "English",
	}
	resultEN, err := uc.Execute(ctx, reqEN)
	if err != nil {
		t.Fatalf("Execute EN failed: %v", err)
	}

	if resultEN.CacheHit {
		t.Error("expected cache miss for different language")
	}
	if resultEN.DocumentID == resultKO.DocumentID {
		t.Error("expected different document IDs for different languages")
	}

	// Verify both documents exist
	var docCount int
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM spec_documents").Scan(&docCount)
	if docCount != 2 {
		t.Errorf("expected 2 documents, got %d", docCount)
	}
}

// Helper functions

func setupAnalysisWithTests(
	t *testing.T,
	ctx context.Context,
	repo *postgres.AnalysisRepository,
	pool *pgxpool.Pool,
	fileCount, testsPerFile int,
) string {
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

	params := postgres.SaveAnalysisResultParams{
		Owner:          "testowner",
		Repo:           "repo" + shortName,
		CommitSHA:      "abc123def456",
		Branch:         "main",
		ExternalRepoID: shortName,
		ParserVersion:  "v1.0.0-test",
		Result:         createTestInventory(fileCount, testsPerFile),
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

	return uuidBytesToString(analysisID)
}

func createTestInventory(fileCount, testsPerFile int) *parser.ScanResult {
	files := make([]domain.TestFile, fileCount)

	for i := 0; i < fileCount; i++ {
		tests := make([]domain.Test, testsPerFile)
		for j := 0; j < testsPerFile; j++ {
			tests[j] = domain.Test{
				Name:   "TestFunc" + string(rune('A'+i)) + string(rune('0'+j)),
				Status: "",
				Location: domain.Location{
					StartLine: 10 + j*10,
				},
			}
		}

		files[i] = domain.TestFile{
			Path:      "src/file_" + string(rune('a'+i)) + "_test.go",
			Framework: "go-test",
			Suites: []domain.TestSuite{
				{
					Name: "Suite" + string(rune('A'+i)),
					Location: domain.Location{
						StartLine: 5,
					},
					Tests: tests,
				},
			},
		}
	}

	return &parser.ScanResult{
		Inventory: &domain.Inventory{
			Files: files,
		},
	}
}

func multiDomainPhase1Output(input specview.Phase1Input) *specview.Phase1Output {
	var domains []specview.DomainGroup
	testIdx := 0

	for i, file := range input.Files {
		var indices []int
		for range file.Tests {
			indices = append(indices, testIdx)
			testIdx++
		}

		domains = append(domains, specview.DomainGroup{
			Name:        "Domain " + string(rune('A'+i)),
			Description: "Domain for " + file.Path,
			Confidence:  0.9,
			Features: []specview.FeatureGroup{
				{
					Name:        "Feature " + string(rune('A'+i)),
					Description: "Feature tests",
					Confidence:  0.85,
					TestIndices: indices,
				},
			},
		})
	}

	return &specview.Phase1Output{Domains: domains}
}

func multiFeaturePhase1Output(input specview.Phase1Input, featureCount int) *specview.Phase1Output {
	var allIndices []int
	for _, file := range input.Files {
		for _, test := range file.Tests {
			allIndices = append(allIndices, test.Index)
		}
	}

	testsPerFeature := len(allIndices) / featureCount
	if testsPerFeature == 0 {
		testsPerFeature = 1
	}

	var features []specview.FeatureGroup
	for i := 0; i < featureCount; i++ {
		start := i * testsPerFeature
		end := start + testsPerFeature
		if end > len(allIndices) {
			end = len(allIndices)
		}
		if i == featureCount-1 {
			end = len(allIndices)
		}

		if start >= len(allIndices) {
			break
		}

		features = append(features, specview.FeatureGroup{
			Name:        "Feature " + string(rune('A'+i)),
			Description: "Feature " + string(rune('A'+i)) + " tests",
			Confidence:  0.85,
			TestIndices: allIndices[start:end],
		})
	}

	return &specview.Phase1Output{
		Domains: []specview.DomainGroup{
			{
				Name:        "Test Domain",
				Description: "Test domain",
				Confidence:  0.9,
				Features:    features,
			},
		},
	}
}

func verifyDocumentSaved(t *testing.T, ctx context.Context, pool *pgxpool.Pool, documentID string) {
	t.Helper()

	var docCount int
	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM spec_documents WHERE id::text = $1", documentID).Scan(&docCount)
	if err != nil {
		t.Fatalf("failed to query document: %v", err)
	}
	if docCount != 1 {
		t.Errorf("expected 1 document with ID %s, got %d", documentID, docCount)
	}

	var domainCount int
	pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM spec_domains d
		JOIN spec_documents doc ON doc.id = d.document_id
		WHERE doc.id::text = $1
	`, documentID).Scan(&domainCount)
	if domainCount == 0 {
		t.Error("expected at least 1 domain")
	}

	var featureCount int
	pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM spec_features f
		JOIN spec_domains d ON d.id = f.domain_id
		JOIN spec_documents doc ON doc.id = d.document_id
		WHERE doc.id::text = $1
	`, documentID).Scan(&featureCount)
	if featureCount == 0 {
		t.Error("expected at least 1 feature")
	}

	var behaviorCount int
	pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM spec_behaviors b
		JOIN spec_features f ON f.id = b.feature_id
		JOIN spec_domains d ON d.id = f.domain_id
		JOIN spec_documents doc ON doc.id = d.document_id
		WHERE doc.id::text = $1
	`, documentID).Scan(&behaviorCount)
	if behaviorCount == 0 {
		t.Error("expected at least 1 behavior")
	}
}

func uuidBytesToString(bytes [16]byte) string {
	return hex.EncodeToString(bytes[0:4]) + "-" +
		hex.EncodeToString(bytes[4:6]) + "-" +
		hex.EncodeToString(bytes[6:8]) + "-" +
		hex.EncodeToString(bytes[8:10]) + "-" +
		hex.EncodeToString(bytes[10:16])
}
