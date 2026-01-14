package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/specvital/core/pkg/domain"
	"github.com/specvital/core/pkg/parser"
	"github.com/specvital/worker/internal/adapter/repository/postgres"
	"github.com/specvital/worker/internal/domain/specview"
	specviewuc "github.com/specvital/worker/internal/usecase/specview"
	testdb "github.com/specvital/worker/internal/testutil/postgres"
)

// BenchmarkSpecView_500Behaviors benchmarks document generation with 500 behaviors.
// Target: < 3 minutes (excluding AI call time since we use mocks).
func BenchmarkSpecView_500Behaviors(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test")
	}

	pool, cleanup := testdb.SetupTestDB(&testing.T{})
	defer cleanup()

	ctx := context.Background()

	// Create analysis with 50 files * 10 tests = 500 behaviors
	analysisID := setupBenchmarkAnalysis(b, ctx, pool, 50, 10)

	aiProvider := &mockAIProvider{
		classifyDomainsFn: func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
			return largeDomainOutput(input, 10), nil, nil
		},
		convertTestNamesFn: func(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, *specview.TokenUsage, error) {
			return defaultPhase2Output(input), nil, nil
		},
	}

	specRepo := postgres.NewSpecDocumentRepository(pool)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Each iteration needs a unique language to avoid cache hits
		lang := specview.Language("en")
		if i%2 == 1 {
			lang = "Korean"
		}

		// Clean up previous documents to test fresh generation
		pool.Exec(ctx, "DELETE FROM spec_documents WHERE analysis_id = (SELECT id FROM analyses LIMIT 1)")

		uc := specviewuc.NewGenerateSpecViewUseCase(specRepo, aiProvider, "benchmark-model")
		req := specview.SpecViewRequest{
			AnalysisID: analysisID,
			Language:   lang,
		}

		_, err := uc.Execute(ctx, req)
		if err != nil {
			b.Fatalf("Execute failed: %v", err)
		}
	}
}

// BenchmarkSpecView_4WayJoin benchmarks the 4-way JOIN query performance.
// Target: < 100ms query time.
func BenchmarkSpecView_4WayJoin(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test")
	}

	pool, cleanup := testdb.SetupTestDB(&testing.T{})
	defer cleanup()

	ctx := context.Background()

	// Setup: Create a document with substantial data
	analysisID := setupBenchmarkAnalysis(b, ctx, pool, 50, 10)

	aiProvider := &mockAIProvider{
		classifyDomainsFn: func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
			return largeDomainOutput(input, 10), nil, nil
		},
	}

	specRepo := postgres.NewSpecDocumentRepository(pool)
	uc := specviewuc.NewGenerateSpecViewUseCase(specRepo, aiProvider, "benchmark-model")

	req := specview.SpecViewRequest{
		AnalysisID: analysisID,
		Language:   "Korean",
	}

	result, err := uc.Execute(ctx, req)
	if err != nil {
		b.Fatalf("Execute failed: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Simulate the 4-way JOIN query used for document retrieval
		var totalBehaviors int
		err := pool.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM spec_behaviors b
			JOIN spec_features f ON f.id = b.feature_id
			JOIN spec_domains d ON d.id = f.domain_id
			JOIN spec_documents doc ON doc.id = d.document_id
			WHERE doc.id::text = $1
		`, result.DocumentID).Scan(&totalBehaviors)

		if err != nil {
			b.Fatalf("4-way JOIN query failed: %v", err)
		}

		if totalBehaviors == 0 {
			b.Fatal("expected behaviors from 4-way JOIN")
		}
	}
}

// BenchmarkSpecView_BulkInsert benchmarks COPY-based bulk insert performance.
func BenchmarkSpecView_BulkInsert(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test")
	}

	pool, cleanup := testdb.SetupTestDB(&testing.T{})
	defer cleanup()

	ctx := context.Background()

	// Create analysis with 100 files * 10 tests = 1000 behaviors
	analysisID := setupBenchmarkAnalysis(b, ctx, pool, 100, 10)

	aiProvider := &mockAIProvider{
		classifyDomainsFn: func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
			return singleDomainLargeOutput(input), nil, nil
		},
	}

	specRepo := postgres.NewSpecDocumentRepository(pool)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Clean up previous documents
		pool.Exec(ctx, "DELETE FROM spec_documents")

		uc := specviewuc.NewGenerateSpecViewUseCase(specRepo, aiProvider, "benchmark-model")
		req := specview.SpecViewRequest{
			AnalysisID: analysisID,
			Language:   "English",
		}

		_, err := uc.Execute(ctx, req)
		if err != nil {
			b.Fatalf("Execute failed: %v", err)
		}
	}
}

// BenchmarkSpecView_ContentHashCalculation benchmarks content hash generation.
func BenchmarkSpecView_ContentHashCalculation(b *testing.B) {
	files := generateLargeFileInfo(100, 10)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		hash := specview.GenerateContentHash(files, "Korean")
		if len(hash) == 0 {
			b.Fatal("expected non-empty hash")
		}
	}
}

// Helper functions for benchmarks

func setupBenchmarkAnalysis(
	b *testing.B,
	ctx context.Context,
	pool *pgxpool.Pool,
	fileCount, testsPerFile int,
) string {
	b.Helper()

	randBytes := make([]byte, 8)
	rand.Read(randBytes)
	randSuffix := hex.EncodeToString(randBytes)

	analysisRepo := postgres.NewAnalysisRepository(pool)
	params := postgres.SaveAnalysisResultParams{
		Owner:          "benchmark",
		Repo:           "repo-" + randSuffix,
		CommitSHA:      "abc123",
		Branch:         "main",
		ExternalRepoID: "ext-" + randSuffix,
		ParserVersion:  "v1.0.0",
		Result:         createLargeInventory(fileCount, testsPerFile),
	}

	err := analysisRepo.SaveAnalysisResult(ctx, params)
	if err != nil {
		b.Fatalf("SaveAnalysisResult failed: %v", err)
	}

	var analysisID [16]byte
	err = pool.QueryRow(ctx, `
		SELECT a.id FROM analyses a
		JOIN codebases c ON c.id = a.codebase_id
		WHERE c.external_repo_id = $1
		ORDER BY a.created_at DESC
		LIMIT 1
	`, "ext-"+randSuffix).Scan(&analysisID)

	if err != nil {
		b.Fatalf("failed to get analysis ID: %v", err)
	}

	return uuidBytesToString(analysisID)
}

func createLargeInventory(fileCount, testsPerFile int) *parser.ScanResult {
	files := make([]domain.TestFile, fileCount)

	for i := 0; i < fileCount; i++ {
		tests := make([]domain.Test, testsPerFile)
		for j := 0; j < testsPerFile; j++ {
			tests[j] = domain.Test{
				Name:   "Test_" + string(rune('A'+i%26)) + "_" + string(rune('0'+j%10)),
				Status: "",
				Location: domain.Location{
					StartLine: 10 + j*5,
				},
			}
		}

		files[i] = domain.TestFile{
			Path:      "src/module_" + string(rune('a'+i%26)) + "/file_" + string(rune('0'+i%10)) + "_test.go",
			Framework: "go-test",
			Suites: []domain.TestSuite{
				{
					Name: "Suite_" + string(rune('A'+i%26)),
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

func largeDomainOutput(input specview.Phase1Input, domainCount int) *specview.Phase1Output {
	var allIndices []int
	for _, file := range input.Files {
		for _, test := range file.Tests {
			allIndices = append(allIndices, test.Index)
		}
	}

	testsPerDomain := len(allIndices) / domainCount
	if testsPerDomain == 0 {
		testsPerDomain = 1
	}

	var domains []specview.DomainGroup
	for i := 0; i < domainCount; i++ {
		start := i * testsPerDomain
		end := start + testsPerDomain
		if end > len(allIndices) {
			end = len(allIndices)
		}
		if i == domainCount-1 {
			end = len(allIndices)
		}
		if start >= len(allIndices) {
			break
		}

		// Split domain tests into features
		domainIndices := allIndices[start:end]
		featuresPerDomain := 5
		testsPerFeature := len(domainIndices) / featuresPerDomain
		if testsPerFeature == 0 {
			testsPerFeature = 1
		}

		var features []specview.FeatureGroup
		for j := 0; j < featuresPerDomain; j++ {
			fStart := j * testsPerFeature
			fEnd := fStart + testsPerFeature
			if fEnd > len(domainIndices) {
				fEnd = len(domainIndices)
			}
			if j == featuresPerDomain-1 {
				fEnd = len(domainIndices)
			}
			if fStart >= len(domainIndices) {
				break
			}

			features = append(features, specview.FeatureGroup{
				Name:        "Feature_" + string(rune('A'+i)) + string(rune('0'+j)),
				Description: "Feature description",
				Confidence:  0.85,
				TestIndices: domainIndices[fStart:fEnd],
			})
		}

		domains = append(domains, specview.DomainGroup{
			Name:        "Domain_" + string(rune('A'+i)),
			Description: "Domain description",
			Confidence:  0.9,
			Features:    features,
		})
	}

	return &specview.Phase1Output{Domains: domains}
}

func singleDomainLargeOutput(input specview.Phase1Input) *specview.Phase1Output {
	var allIndices []int
	for _, file := range input.Files {
		for _, test := range file.Tests {
			allIndices = append(allIndices, test.Index)
		}
	}

	return &specview.Phase1Output{
		Domains: []specview.DomainGroup{
			{
				Name:        "Bulk Test Domain",
				Description: "Single domain for bulk insert testing",
				Confidence:  0.95,
				Features: []specview.FeatureGroup{
					{
						Name:        "All Tests",
						Description: "All tests in single feature",
						Confidence:  0.9,
						TestIndices: allIndices,
					},
				},
			},
		},
	}
}

func generateLargeFileInfo(fileCount, testsPerFile int) []specview.FileInfo {
	files := make([]specview.FileInfo, fileCount)
	testIdx := 0

	for i := 0; i < fileCount; i++ {
		tests := make([]specview.TestInfo, testsPerFile)
		for j := 0; j < testsPerFile; j++ {
			tests[j] = specview.TestInfo{
				Index:      testIdx,
				Name:       "Test_" + string(rune('A'+i%26)) + "_" + string(rune('0'+j)),
				SuitePath:  "Suite > Nested",
				TestCaseID: "tc-" + string(rune('0'+testIdx%10)),
			}
			testIdx++
		}

		files[i] = specview.FileInfo{
			Path:      "src/module/file_" + string(rune('a'+i%26)) + "_test.go",
			Framework: "go-test",
			Tests:     tests,
		}
	}

	return files
}
