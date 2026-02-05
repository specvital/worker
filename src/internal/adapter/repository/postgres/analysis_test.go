package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/specvital/core/pkg/domain"
	"github.com/specvital/core/pkg/parser"
	"github.com/specvital/worker/internal/domain/analysis"
	testdb "github.com/specvital/worker/internal/testutil/postgres"
)

const testParserVersion = "v1.0.0-test"

func TestAnalysisRepository_SaveAnalysisResult(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	repo := NewAnalysisRepository(pool)
	ctx := context.Background()

	t.Run("should save analysis with suites and tests", func(t *testing.T) {
		params := SaveAnalysisResultParams{
			Owner:          "testowner",
			Repo:           "testrepo",
			CommitSHA:      "abc123def456",
			Branch:         "main",
			ExternalRepoID: "12345",
			ParserVersion:  testParserVersion,
			Result: &parser.ScanResult{
				Inventory: &domain.Inventory{
					Files: []domain.TestFile{
						{
							Path:      "src/user_test.go",
							Framework: "go-test",
							Suites: []domain.TestSuite{
								{
									Name: "TestUserService",
									Location: domain.Location{
										StartLine: 10,
									},
									Tests: []domain.Test{
										{
											Name:   "TestCreate",
											Status: "", // maps to active
											Location: domain.Location{
												StartLine: 12,
											},
										},
										{
											Name:   "TestUpdate",
											Status: domain.TestStatusSkipped,
											Location: domain.Location{
												StartLine: 20,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		err := repo.SaveAnalysisResult(ctx, params)
		if err != nil {
			t.Fatalf("SaveAnalysisResult failed: %v", err)
		}

		var analysisCount int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM analyses").Scan(&analysisCount)
		if err != nil {
			t.Fatalf("failed to query analyses: %v", err)
		}
		if analysisCount != 1 {
			t.Errorf("expected 1 analysis, got %d", analysisCount)
		}

		var suiteCount int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_suites").Scan(&suiteCount)
		if err != nil {
			t.Fatalf("failed to query suites: %v", err)
		}
		if suiteCount != 1 {
			t.Errorf("expected 1 suite, got %d", suiteCount)
		}

		var testCount int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_cases").Scan(&testCount)
		if err != nil {
			t.Fatalf("failed to query test cases: %v", err)
		}
		if testCount != 2 {
			t.Errorf("expected 2 tests, got %d", testCount)
		}

		var status string
		err = pool.QueryRow(ctx, "SELECT status FROM analyses").Scan(&status)
		if err != nil {
			t.Fatalf("failed to query analysis status: %v", err)
		}
		if status != "completed" {
			t.Errorf("expected status 'completed', got '%s'", status)
		}
	})

	t.Run("should handle nested suites", func(t *testing.T) {
		_, err := pool.Exec(ctx, "TRUNCATE codebases CASCADE")
		if err != nil {
			t.Fatalf("failed to truncate: %v", err)
		}

		params := SaveAnalysisResultParams{
			Owner:          "owner2",
			Repo:           "repo2",
			CommitSHA:      "def789",
			Branch:         "develop",
			ExternalRepoID: "22222",
			ParserVersion:  testParserVersion,
			Result: &parser.ScanResult{
				Inventory: &domain.Inventory{
					Files: []domain.TestFile{
						{
							Path:      "tests/integration_test.go",
							Framework: "go-test",
							Suites: []domain.TestSuite{
								{
									Name:     "OuterSuite",
									Location: domain.Location{StartLine: 5},
									Tests: []domain.Test{
										{
											Name:     "OuterTest",
											Status:   "", // maps to active
											Location: domain.Location{StartLine: 7},
										},
									},
									Suites: []domain.TestSuite{
										{
											Name:     "InnerSuite",
											Location: domain.Location{StartLine: 15},
											Tests: []domain.Test{
												{
													Name:     "InnerTest",
													Status:   domain.TestStatusTodo,
													Location: domain.Location{StartLine: 17},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		err = repo.SaveAnalysisResult(ctx, params)
		if err != nil {
			t.Fatalf("SaveAnalysisResult failed: %v", err)
		}

		var suiteCount int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_suites").Scan(&suiteCount)
		if err != nil {
			t.Fatalf("failed to query suites: %v", err)
		}
		if suiteCount != 2 {
			t.Errorf("expected 2 suites (outer + inner), got %d", suiteCount)
		}

		var depth1Count int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_suites WHERE depth = 1").Scan(&depth1Count)
		if err != nil {
			t.Fatalf("failed to query depth 1 suites: %v", err)
		}
		if depth1Count != 1 {
			t.Errorf("expected 1 nested suite with depth 1, got %d", depth1Count)
		}

		var todoCount int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_cases WHERE status = 'todo'").Scan(&todoCount)
		if err != nil {
			t.Fatalf("failed to query todo tests: %v", err)
		}
		if todoCount != 1 {
			t.Errorf("expected 1 todo test (from pending status), got %d", todoCount)
		}
	})

	t.Run("should handle implicit suite for file-level tests", func(t *testing.T) {
		_, err := pool.Exec(ctx, "TRUNCATE codebases CASCADE")
		if err != nil {
			t.Fatalf("failed to truncate: %v", err)
		}

		params := SaveAnalysisResultParams{
			Owner:          "owner3",
			Repo:           "repo3",
			CommitSHA:      "ghi012",
			Branch:         "main",
			ExternalRepoID: "33333",
			ParserVersion:  testParserVersion,
			Result: &parser.ScanResult{
				Inventory: &domain.Inventory{
					Files: []domain.TestFile{
						{
							Path:      "simple_test.go",
							Framework: "go-test",
							Tests: []domain.Test{
								{
									Name:     "TestSimple",
									Status:   "", // maps to active
									Location: domain.Location{StartLine: 5},
								},
							},
						},
					},
				},
			},
		}

		err = repo.SaveAnalysisResult(ctx, params)
		if err != nil {
			t.Fatalf("SaveAnalysisResult failed: %v", err)
		}

		var suiteName string
		err = pool.QueryRow(ctx, "SELECT name FROM test_suites").Scan(&suiteName)
		if err != nil {
			t.Fatalf("failed to query suite: %v", err)
		}
		if suiteName != "simple_test.go" {
			t.Errorf("expected implicit suite name 'simple_test.go', got '%s'", suiteName)
		}
	})

	t.Run("should handle nil inventory", func(t *testing.T) {
		_, err := pool.Exec(ctx, "TRUNCATE codebases CASCADE")
		if err != nil {
			t.Fatalf("failed to truncate: %v", err)
		}

		params := SaveAnalysisResultParams{
			Owner:          "owner4",
			Repo:           "repo4",
			CommitSHA:      "jkl345",
			Branch:         "main",
			ExternalRepoID: "44444",
			ParserVersion:  testParserVersion,
			Result:         &parser.ScanResult{Inventory: nil},
		}

		err = repo.SaveAnalysisResult(ctx, params)
		if err != nil {
			t.Fatalf("SaveAnalysisResult failed with nil inventory: %v", err)
		}

		var totalSuites, totalTests int
		err = pool.QueryRow(ctx, "SELECT total_suites, total_tests FROM analyses").Scan(&totalSuites, &totalTests)
		if err != nil {
			t.Fatalf("failed to query analysis: %v", err)
		}
		if totalSuites != 0 || totalTests != 0 {
			t.Errorf("expected 0 suites and 0 tests for nil inventory, got %d and %d", totalSuites, totalTests)
		}
	})
}

func TestAnalysisRepository_TransactionRollback(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	repo := NewAnalysisRepository(pool)
	ctx := context.Background()

	t.Run("should rollback on duplicate commit", func(t *testing.T) {
		params := SaveAnalysisResultParams{
			Owner:          "rollback-owner",
			Repo:           "rollback-repo",
			CommitSHA:      "same-commit-sha",
			Branch:         "main",
			ExternalRepoID: "rollback-id",
			ParserVersion:  testParserVersion,
			Result:         &parser.ScanResult{Inventory: nil},
		}

		err := repo.SaveAnalysisResult(ctx, params)
		if err != nil {
			t.Fatalf("first SaveAnalysisResult failed: %v", err)
		}

		err = repo.SaveAnalysisResult(ctx, params)
		if err == nil {
			t.Error("expected error for duplicate commit, got nil")
		}

		var analysisCount int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM analyses").Scan(&analysisCount)
		if err != nil {
			t.Fatalf("failed to query analyses: %v", err)
		}
		if analysisCount != 1 {
			t.Errorf("expected 1 analysis (duplicate should be rejected), got %d", analysisCount)
		}
	})
}

func TestAnalysisRepository_RecordFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	repo := NewAnalysisRepository(pool)
	ctx := context.Background()

	t.Run("should record failure with error message", func(t *testing.T) {
		analysisID, err := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "test-owner",
			Repo:           "test-repo",
			CommitSHA:      "abc123",
			Branch:         "main",
			ExternalRepoID: "failure-test-1",
			ParserVersion:  testParserVersion,
		})
		if err != nil {
			t.Fatalf("CreateAnalysisRecord failed: %v", err)
		}

		errMessage := "scan failed: parser error"
		err = repo.RecordFailure(ctx, analysisID, errMessage)
		if err != nil {
			t.Fatalf("RecordFailure failed: %v", err)
		}

		var status, savedErrMsg string
		pgID := toPgUUID(analysisID)
		err = pool.QueryRow(ctx, "SELECT status, error_message FROM analyses WHERE id = $1", pgID).Scan(&status, &savedErrMsg)
		if err != nil {
			t.Fatalf("failed to query analysis: %v", err)
		}

		if status != "failed" {
			t.Errorf("expected status 'failed', got '%s'", status)
		}
		if savedErrMsg != errMessage {
			t.Errorf("expected error message '%s', got '%s'", errMessage, savedErrMsg)
		}
	})

	t.Run("should fail with invalid analysis ID", func(t *testing.T) {
		err := repo.RecordFailure(ctx, analysis.NilUUID, "some error")
		if !errors.Is(err, analysis.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("should fail with empty error message", func(t *testing.T) {
		analysisID, _ := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "empty-err-owner",
			Repo:           "empty-err-repo",
			CommitSHA:      "empty123",
			Branch:         "main",
			ExternalRepoID: "empty-err-id",
			ParserVersion:  testParserVersion,
		})
		err := repo.RecordFailure(ctx, analysisID, "")
		if !errors.Is(err, analysis.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput, got %v", err)
		}
	})
}

func TestAnalysisRepository_CreateAnalysisRecord(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	repo := NewAnalysisRepository(pool)
	ctx := context.Background()

	t.Run("should create analysis record", func(t *testing.T) {
		analysisID, err := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "create-owner",
			Repo:           "create-repo",
			CommitSHA:      "def456",
			Branch:         "develop",
			ExternalRepoID: "create-id",
			ParserVersion:  testParserVersion,
		})
		if err != nil {
			t.Fatalf("CreateAnalysisRecord failed: %v", err)
		}

		if analysisID == analysis.NilUUID {
			t.Error("expected valid UUID, got nil UUID")
		}

		var status string
		pgID := toPgUUID(analysisID)
		err = pool.QueryRow(ctx, "SELECT status FROM analyses WHERE id = $1", pgID).Scan(&status)
		if err != nil {
			t.Fatalf("failed to query analysis: %v", err)
		}

		if status != "running" {
			t.Errorf("expected status 'running', got '%s'", status)
		}
	})
}

func TestAnalysisRepository_SaveAnalysisInventory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	repo := NewAnalysisRepository(pool)
	ctx := context.Background()

	t.Run("should save inventory using domain types", func(t *testing.T) {
		analysisID, err := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "domain-owner",
			Repo:           "domain-repo",
			CommitSHA:      "xyz789",
			Branch:         "main",
			ExternalRepoID: "domain-id",
			ParserVersion:  testParserVersion,
		})
		if err != nil {
			t.Fatalf("CreateAnalysisRecord failed: %v", err)
		}

		inventory := &analysis.Inventory{
			Files: []analysis.TestFile{
				{
					Path:      "pkg/service/service_test.go",
					Framework: "go-test",
					Suites: []analysis.TestSuite{
						{
							Name: "TestService",
							Location: analysis.Location{
								StartLine: 15,
								EndLine:   50,
							},
							Tests: []analysis.Test{
								{
									Name: "TestServiceCreate",
									Location: analysis.Location{
										StartLine: 20,
										EndLine:   25,
									},
									Status: analysis.TestStatusActive,
								},
								{
									Name: "TestServiceUpdate",
									Location: analysis.Location{
										StartLine: 30,
										EndLine:   35,
									},
									Status: analysis.TestStatusSkipped,
								},
							},
						},
					},
				},
			},
		}

		err = repo.SaveAnalysisInventory(ctx, analysis.SaveAnalysisInventoryParams{
			AnalysisID: analysisID,
			Inventory:  inventory,
		})
		if err != nil {
			t.Fatalf("SaveAnalysisInventory failed: %v", err)
		}

		var totalSuites, totalTests int
		pgID := toPgUUID(analysisID)
		err = pool.QueryRow(ctx, "SELECT total_suites, total_tests FROM analyses WHERE id = $1", pgID).
			Scan(&totalSuites, &totalTests)
		if err != nil {
			t.Fatalf("failed to query analysis: %v", err)
		}

		if totalSuites != 1 {
			t.Errorf("expected 1 suite, got %d", totalSuites)
		}
		if totalTests != 2 {
			t.Errorf("expected 2 tests, got %d", totalTests)
		}

		var status string
		err = pool.QueryRow(ctx, "SELECT status FROM analyses WHERE id = $1", pgID).Scan(&status)
		if err != nil {
			t.Fatalf("failed to query status: %v", err)
		}
		if status != "completed" {
			t.Errorf("expected status 'completed', got '%s'", status)
		}
	})
}

func Test_truncateErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "short message unchanged",
			input:    "short error",
			expected: "short error",
		},
		{
			name:     "exact limit unchanged",
			input:    string(make([]byte, maxErrorMessageLength)),
			expected: string(make([]byte, maxErrorMessageLength)),
		},
		{
			name:     "long message truncated",
			input:    string(make([]byte, maxErrorMessageLength+100)),
			expected: string(make([]byte, maxErrorMessageLength-15)) + "... (truncated)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateErrorMessage(tt.input)
			if result != tt.expected {
				t.Errorf("expected length %d, got %d", len(tt.expected), len(result))
			}
			if len(result) > maxErrorMessageLength {
				t.Errorf("result exceeds max length: %d > %d", len(result), maxErrorMessageLength)
			}
		})
	}
}

func Test_truncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short string unchanged",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length unchanged",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "long string truncated",
			input:    "hello world",
			maxLen:   8,
			expected: "hello...",
		},
		{
			name:     "empty string",
			input:    "",
			maxLen:   10,
			expected: "",
		},
		{
			name:     "utf8 multibyte safe truncation",
			input:    "한글테스트입니다",
			maxLen:   10,
			expected: "한글...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
			if len(result) > tt.maxLen {
				t.Errorf("result exceeds max length: %d > %d", len(result), tt.maxLen)
			}
		})
	}
}

func TestSaveAnalysisResultParams_Validate(t *testing.T) {
	tests := []struct {
		name    string
		params  SaveAnalysisResultParams
		wantErr bool
	}{
		{
			name: "valid",
			params: SaveAnalysisResultParams{
				Owner:         "owner",
				Repo:          "repo",
				CommitSHA:     "abc123",
				Branch:        "main",
				ParserVersion: testParserVersion,
				Result:        &parser.ScanResult{},
			},
			wantErr: false,
		},
		{
			name: "missing parser version",
			params: SaveAnalysisResultParams{
				Owner:     "owner",
				Repo:      "repo",
				CommitSHA: "abc123",
				Branch:    "main",
				Result:    &parser.ScanResult{},
			},
			wantErr: true,
		},
		{
			name: "missing owner",
			params: SaveAnalysisResultParams{
				Repo:      "repo",
				CommitSHA: "abc123",
				Result:    &parser.ScanResult{},
			},
			wantErr: true,
		},
		{
			name: "missing repo",
			params: SaveAnalysisResultParams{
				Owner:     "owner",
				CommitSHA: "abc123",
				Result:    &parser.ScanResult{},
			},
			wantErr: true,
		},
		{
			name: "missing commit SHA",
			params: SaveAnalysisResultParams{
				Owner:  "owner",
				Repo:   "repo",
				Result: &parser.ScanResult{},
			},
			wantErr: true,
		},
		{
			name: "missing result",
			params: SaveAnalysisResultParams{
				Owner:     "owner",
				Repo:      "repo",
				CommitSHA: "abc123",
				Result:    nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.params.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if !errors.Is(err, analysis.ErrInvalidInput) {
					t.Errorf("expected ErrInvalidInput, got %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func Test_flattenInventory(t *testing.T) {
	t.Run("nil inventory returns empty", func(t *testing.T) {
		suites, tests := flattenInventory(nil, nil)
		if suites != nil || tests != nil {
			t.Errorf("expected nil, nil for nil inventory")
		}
	})

	t.Run("empty inventory returns empty slices", func(t *testing.T) {
		inv := &analysis.Inventory{}
		suites, tests := flattenInventory(inv, nil)
		if len(suites) != 0 || len(tests) != 0 {
			t.Errorf("expected empty slices for empty inventory")
		}
	})

	t.Run("flattens nested suites correctly", func(t *testing.T) {
		inv := &analysis.Inventory{
			Files: []analysis.TestFile{
				{
					Path:      "test.go",
					Framework: "go-test",
					Suites: []analysis.TestSuite{
						{
							Name:     "OuterSuite",
							Location: analysis.Location{StartLine: 10},
							Tests: []analysis.Test{
								{Name: "OuterTest", Location: analysis.Location{StartLine: 12}},
							},
							Suites: []analysis.TestSuite{
								{
									Name:     "InnerSuite",
									Location: analysis.Location{StartLine: 20},
									Tests: []analysis.Test{
										{Name: "InnerTest", Location: analysis.Location{StartLine: 22}},
									},
								},
							},
						},
					},
				},
			},
		}

		fileIDs := map[string]pgtype.UUID{
			"test.go": {Bytes: [16]byte{1}, Valid: true},
		}
		suites, tests := flattenInventory(inv, fileIDs)

		if len(suites) != 2 {
			t.Errorf("expected 2 suites, got %d", len(suites))
		}
		if len(tests) != 2 {
			t.Errorf("expected 2 tests, got %d", len(tests))
		}

		// Check depths
		depthCounts := make(map[int]int)
		for _, s := range suites {
			depthCounts[s.depth]++
		}
		if depthCounts[0] != 1 || depthCounts[1] != 1 {
			t.Errorf("expected 1 suite at depth 0 and 1 at depth 1, got %v", depthCounts)
		}

		// Check parent relationships
		var innerSuite flatSuite
		for _, s := range suites {
			if s.suite.Name == "InnerSuite" {
				innerSuite = s
				break
			}
		}
		if innerSuite.parentTemp == -1 {
			t.Error("inner suite should have a parent")
		}
	})

	t.Run("creates implicit suite for file-level tests", func(t *testing.T) {
		inv := &analysis.Inventory{
			Files: []analysis.TestFile{
				{
					Path:      "simple_test.go",
					Framework: "go-test",
					Tests: []analysis.Test{
						{Name: "TestSimple", Location: analysis.Location{StartLine: 5}},
					},
				},
			},
		}

		fileIDs := map[string]pgtype.UUID{
			"simple_test.go": {Bytes: [16]byte{2}, Valid: true},
		}
		suites, tests := flattenInventory(inv, fileIDs)

		if len(suites) != 1 {
			t.Errorf("expected 1 implicit suite, got %d", len(suites))
		}
		if len(tests) != 1 {
			t.Errorf("expected 1 test, got %d", len(tests))
		}
		if suites[0].suite.Name != "simple_test.go" {
			t.Errorf("expected implicit suite name to be file path, got %s", suites[0].suite.Name)
		}
	})
}

func Test_groupByDepth(t *testing.T) {
	suites := []flatSuite{
		{tempID: 0, depth: 0},
		{tempID: 1, depth: 0},
		{tempID: 2, depth: 1},
		{tempID: 3, depth: 2},
	}

	result := groupByDepth(suites)

	if len(result[0]) != 2 {
		t.Errorf("expected 2 suites at depth 0, got %d", len(result[0]))
	}
	if len(result[1]) != 1 {
		t.Errorf("expected 1 suite at depth 1, got %d", len(result[1]))
	}
	if len(result[2]) != 1 {
		t.Errorf("expected 1 suite at depth 2, got %d", len(result[2]))
	}
}

func Test_maxDepthInSuites(t *testing.T) {
	t.Run("empty map returns -1", func(t *testing.T) {
		result := maxDepthInSuites(map[int][]flatSuite{})
		if result != -1 {
			t.Errorf("expected -1 for empty map, got %d", result)
		}
	})

	t.Run("returns max depth", func(t *testing.T) {
		suitesByDepth := map[int][]flatSuite{
			0: {{tempID: 0}},
			2: {{tempID: 1}},
			5: {{tempID: 2}},
		}
		result := maxDepthInSuites(suitesByDepth)
		if result != 5 {
			t.Errorf("expected max depth 5, got %d", result)
		}
	})
}

func TestAnalysisRepository_DomainHints(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	repo := NewAnalysisRepository(pool)
	ctx := context.Background()

	t.Run("should save and retrieve DomainHints", func(t *testing.T) {
		// Given: DomainHints that contains Calls and Imports
		analysisID, err := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "hints-owner",
			Repo:           "hints-repo",
			CommitSHA:      "hints123",
			Branch:         "main",
			ExternalRepoID: "hints-id-1",
			ParserVersion:  testParserVersion,
		})
		if err != nil {
			t.Fatalf("CreateAnalysisRecord failed: %v", err)
		}

		inventory := &analysis.Inventory{
			Files: []analysis.TestFile{
				{
					Path:      "auth.test.ts",
					Framework: "jest",
					DomainHints: &analysis.DomainHints{
						Calls:   []string{"authService.validateToken", "userRepo.findById"},
						Imports: []string{"@nestjs/jwt", "@nestjs/testing"},
					},
					Suites: []analysis.TestSuite{
						{
							Name:     "AuthService",
							Location: analysis.Location{StartLine: 10},
							Tests: []analysis.Test{
								{Name: "should validate token", Location: analysis.Location{StartLine: 12}},
							},
						},
					},
				},
			},
		}

		// When: SaveAnalysisInventory is called
		err = repo.SaveAnalysisInventory(ctx, analysis.SaveAnalysisInventoryParams{
			AnalysisID: analysisID,
			Inventory:  inventory,
		})
		if err != nil {
			t.Fatalf("SaveAnalysisInventory failed: %v", err)
		}

		// Then: domain_hints is stored as JSONB and matches the original
		var domainHintsJSON []byte
		pgID := toPgUUID(analysisID)
		err = pool.QueryRow(ctx, "SELECT domain_hints FROM test_files WHERE analysis_id = $1", pgID).Scan(&domainHintsJSON)
		if err != nil {
			t.Fatalf("failed to query domain_hints: %v", err)
		}

		if domainHintsJSON == nil {
			t.Fatal("expected domain_hints to be non-nil")
		}

		var retrievedHints analysis.DomainHints
		if err := json.Unmarshal(domainHintsJSON, &retrievedHints); err != nil {
			t.Fatalf("failed to unmarshal domain_hints: %v", err)
		}

		if len(retrievedHints.Calls) != 2 {
			t.Errorf("expected 2 calls, got %d", len(retrievedHints.Calls))
		}
		if retrievedHints.Calls[0] != "authService.validateToken" {
			t.Errorf("expected first call to be 'authService.validateToken', got %q", retrievedHints.Calls[0])
		}
		if retrievedHints.Calls[1] != "userRepo.findById" {
			t.Errorf("expected second call to be 'userRepo.findById', got %q", retrievedHints.Calls[1])
		}

		if len(retrievedHints.Imports) != 2 {
			t.Errorf("expected 2 imports, got %d", len(retrievedHints.Imports))
		}
		if retrievedHints.Imports[0] != "@nestjs/jwt" {
			t.Errorf("expected first import to be '@nestjs/jwt', got %q", retrievedHints.Imports[0])
		}
		if retrievedHints.Imports[1] != "@nestjs/testing" {
			t.Errorf("expected second import to be '@nestjs/testing', got %q", retrievedHints.Imports[1])
		}
	})

	t.Run("should save nil DomainHints as null", func(t *testing.T) {
		_, err := pool.Exec(ctx, "TRUNCATE codebases CASCADE")
		if err != nil {
			t.Fatalf("failed to truncate: %v", err)
		}

		// Given: TestFile with nil DomainHints
		analysisID, err := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "nil-hints-owner",
			Repo:           "nil-hints-repo",
			CommitSHA:      "nilhints123",
			Branch:         "main",
			ExternalRepoID: "nil-hints-id",
			ParserVersion:  testParserVersion,
		})
		if err != nil {
			t.Fatalf("CreateAnalysisRecord failed: %v", err)
		}

		inventory := &analysis.Inventory{
			Files: []analysis.TestFile{
				{
					Path:        "simple.test.ts",
					Framework:   "jest",
					DomainHints: nil,
					Suites: []analysis.TestSuite{
						{
							Name:     "SimpleSuite",
							Location: analysis.Location{StartLine: 5},
							Tests: []analysis.Test{
								{Name: "simple test", Location: analysis.Location{StartLine: 7}},
							},
						},
					},
				},
			},
		}

		// When: SaveAnalysisInventory is called
		err = repo.SaveAnalysisInventory(ctx, analysis.SaveAnalysisInventoryParams{
			AnalysisID: analysisID,
			Inventory:  inventory,
		})
		if err != nil {
			t.Fatalf("SaveAnalysisInventory failed: %v", err)
		}

		// Then: domain_hints is stored as null
		var domainHintsJSON []byte
		pgID := toPgUUID(analysisID)
		err = pool.QueryRow(ctx, "SELECT domain_hints FROM test_files WHERE analysis_id = $1", pgID).Scan(&domainHintsJSON)
		if err != nil {
			t.Fatalf("failed to query domain_hints: %v", err)
		}

		if domainHintsJSON != nil {
			t.Errorf("expected domain_hints to be nil, got %s", string(domainHintsJSON))
		}
	})

	t.Run("should save empty DomainHints arrays", func(t *testing.T) {
		_, err := pool.Exec(ctx, "TRUNCATE codebases CASCADE")
		if err != nil {
			t.Fatalf("failed to truncate: %v", err)
		}

		// Given: TestFile with empty DomainHints arrays
		analysisID, err := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "empty-hints-owner",
			Repo:           "empty-hints-repo",
			CommitSHA:      "emptyhints123",
			Branch:         "main",
			ExternalRepoID: "empty-hints-id",
			ParserVersion:  testParserVersion,
		})
		if err != nil {
			t.Fatalf("CreateAnalysisRecord failed: %v", err)
		}

		inventory := &analysis.Inventory{
			Files: []analysis.TestFile{
				{
					Path:      "empty-hints.test.ts",
					Framework: "jest",
					DomainHints: &analysis.DomainHints{
						Calls:   []string{},
						Imports: []string{},
					},
					Suites: []analysis.TestSuite{
						{
							Name:     "EmptyHintsSuite",
							Location: analysis.Location{StartLine: 5},
							Tests: []analysis.Test{
								{Name: "test with empty hints", Location: analysis.Location{StartLine: 7}},
							},
						},
					},
				},
			},
		}

		// When: SaveAnalysisInventory is called
		err = repo.SaveAnalysisInventory(ctx, analysis.SaveAnalysisInventoryParams{
			AnalysisID: analysisID,
			Inventory:  inventory,
		})
		if err != nil {
			t.Fatalf("SaveAnalysisInventory failed: %v", err)
		}

		// Then: domain_hints is stored with empty arrays
		var domainHintsJSON []byte
		pgID := toPgUUID(analysisID)
		err = pool.QueryRow(ctx, "SELECT domain_hints FROM test_files WHERE analysis_id = $1", pgID).Scan(&domainHintsJSON)
		if err != nil {
			t.Fatalf("failed to query domain_hints: %v", err)
		}

		if domainHintsJSON == nil {
			t.Fatal("expected domain_hints to be non-nil for empty arrays")
		}

		var retrievedHints analysis.DomainHints
		if err := json.Unmarshal(domainHintsJSON, &retrievedHints); err != nil {
			t.Fatalf("failed to unmarshal domain_hints: %v", err)
		}

		if retrievedHints.Calls == nil {
			t.Error("expected Calls to be empty slice, got nil")
		}
		if len(retrievedHints.Calls) != 0 {
			t.Errorf("expected 0 calls, got %d", len(retrievedHints.Calls))
		}

		if retrievedHints.Imports == nil {
			t.Error("expected Imports to be empty slice, got nil")
		}
		if len(retrievedHints.Imports) != 0 {
			t.Errorf("expected 0 imports, got %d", len(retrievedHints.Imports))
		}
	})
}

func TestAnalysisRepository_FileIdRelationship(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	repo := NewAnalysisRepository(pool)
	ctx := context.Background()

	t.Run("should link test_suites to test_files via file_id", func(t *testing.T) {
		// Given: Inventory with multiple files
		analysisID, err := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "fileid-owner",
			Repo:           "fileid-repo",
			CommitSHA:      "fileid123",
			Branch:         "main",
			ExternalRepoID: "fileid-id-1",
			ParserVersion:  testParserVersion,
		})
		if err != nil {
			t.Fatalf("CreateAnalysisRecord failed: %v", err)
		}

		inventory := &analysis.Inventory{
			Files: []analysis.TestFile{
				{
					Path:      "auth/login.test.ts",
					Framework: "jest",
					Suites: []analysis.TestSuite{
						{
							Name:     "LoginService",
							Location: analysis.Location{StartLine: 10},
							Tests: []analysis.Test{
								{Name: "should login user", Location: analysis.Location{StartLine: 12}},
							},
						},
					},
				},
				{
					Path:      "auth/logout.test.ts",
					Framework: "jest",
					Suites: []analysis.TestSuite{
						{
							Name:     "LogoutService",
							Location: analysis.Location{StartLine: 5},
							Tests: []analysis.Test{
								{Name: "should logout user", Location: analysis.Location{StartLine: 7}},
							},
						},
					},
				},
			},
		}

		// When: SaveAnalysisInventory is called
		err = repo.SaveAnalysisInventory(ctx, analysis.SaveAnalysisInventoryParams{
			AnalysisID: analysisID,
			Inventory:  inventory,
		})
		if err != nil {
			t.Fatalf("SaveAnalysisInventory failed: %v", err)
		}

		// Then: test_files are created for each file
		var fileCount int
		pgID := toPgUUID(analysisID)
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_files WHERE analysis_id = $1", pgID).Scan(&fileCount)
		if err != nil {
			t.Fatalf("failed to query test_files count: %v", err)
		}
		if fileCount != 2 {
			t.Errorf("expected 2 test_files, got %d", fileCount)
		}

		// Then: test_suites.file_id references correct test_files.id
		type suiteFileRow struct {
			suiteName string
			filePath  string
		}
		rows, err := pool.Query(ctx, `
			SELECT ts.name, tf.file_path
			FROM test_suites ts
			JOIN test_files tf ON ts.file_id = tf.id
			WHERE tf.analysis_id = $1
			ORDER BY tf.file_path
		`, pgID)
		if err != nil {
			t.Fatalf("failed to query suite-file relationship: %v", err)
		}
		defer rows.Close()

		var results []suiteFileRow
		for rows.Next() {
			var r suiteFileRow
			if err := rows.Scan(&r.suiteName, &r.filePath); err != nil {
				t.Fatalf("failed to scan row: %v", err)
			}
			results = append(results, r)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("row iteration error: %v", err)
		}

		if len(results) != 2 {
			t.Fatalf("expected 2 suite-file relationships, got %d", len(results))
		}

		// Verify LoginService is linked to login.test.ts
		if results[0].suiteName != "LoginService" || results[0].filePath != "auth/login.test.ts" {
			t.Errorf("expected LoginService in auth/login.test.ts, got %q in %q", results[0].suiteName, results[0].filePath)
		}

		// Verify LogoutService is linked to logout.test.ts
		if results[1].suiteName != "LogoutService" || results[1].filePath != "auth/logout.test.ts" {
			t.Errorf("expected LogoutService in auth/logout.test.ts, got %q in %q", results[1].suiteName, results[1].filePath)
		}
	})

	t.Run("should cascade delete test_suites when test_files deleted", func(t *testing.T) {
		_, err := pool.Exec(ctx, "TRUNCATE codebases CASCADE")
		if err != nil {
			t.Fatalf("failed to truncate: %v", err)
		}

		// Given: Inventory with test file and suites
		analysisID, err := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "cascade-owner",
			Repo:           "cascade-repo",
			CommitSHA:      "cascade123",
			Branch:         "main",
			ExternalRepoID: "cascade-id-1",
			ParserVersion:  testParserVersion,
		})
		if err != nil {
			t.Fatalf("CreateAnalysisRecord failed: %v", err)
		}

		inventory := &analysis.Inventory{
			Files: []analysis.TestFile{
				{
					Path:      "cascade.test.ts",
					Framework: "jest",
					Suites: []analysis.TestSuite{
						{
							Name:     "CascadeSuite",
							Location: analysis.Location{StartLine: 10},
							Tests: []analysis.Test{
								{Name: "cascade test", Location: analysis.Location{StartLine: 12}},
							},
						},
					},
				},
			},
		}

		err = repo.SaveAnalysisInventory(ctx, analysis.SaveAnalysisInventoryParams{
			AnalysisID: analysisID,
			Inventory:  inventory,
		})
		if err != nil {
			t.Fatalf("SaveAnalysisInventory failed: %v", err)
		}

		// Verify data exists before delete
		pgID := toPgUUID(analysisID)
		var suiteCountBefore int
		err = pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM test_suites ts
			JOIN test_files tf ON ts.file_id = tf.id
			WHERE tf.analysis_id = $1
		`, pgID).Scan(&suiteCountBefore)
		if err != nil {
			t.Fatalf("failed to query suite count before delete: %v", err)
		}
		if suiteCountBefore != 1 {
			t.Fatalf("expected 1 suite before delete, got %d", suiteCountBefore)
		}

		var testCountBefore int
		err = pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM test_cases tc
			JOIN test_suites ts ON tc.suite_id = ts.id
			JOIN test_files tf ON ts.file_id = tf.id
			WHERE tf.analysis_id = $1
		`, pgID).Scan(&testCountBefore)
		if err != nil {
			t.Fatalf("failed to query test count before delete: %v", err)
		}
		if testCountBefore != 1 {
			t.Fatalf("expected 1 test case before delete, got %d", testCountBefore)
		}

		// When: test_files are deleted
		_, err = pool.Exec(ctx, "DELETE FROM test_files WHERE analysis_id = $1", pgID)
		if err != nil {
			t.Fatalf("failed to delete test_files: %v", err)
		}

		// Then: test_suites should be cascade deleted
		var suiteCountAfter int
		err = pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM test_suites ts
			JOIN test_files tf ON ts.file_id = tf.id
			WHERE tf.analysis_id = $1
		`, pgID).Scan(&suiteCountAfter)
		if err != nil {
			t.Fatalf("failed to query suite count after delete: %v", err)
		}
		if suiteCountAfter != 0 {
			t.Errorf("expected 0 suites after cascade delete, got %d", suiteCountAfter)
		}

		// Then: test_cases should also be cascade deleted
		var testCountAfter int
		err = pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM test_cases tc
			JOIN test_suites ts ON tc.suite_id = ts.id
			JOIN test_files tf ON ts.file_id = tf.id
			WHERE tf.analysis_id = $1
		`, pgID).Scan(&testCountAfter)
		if err != nil {
			t.Fatalf("failed to query test count after delete: %v", err)
		}
		if testCountAfter != 0 {
			t.Errorf("expected 0 test cases after cascade delete, got %d", testCountAfter)
		}
	})

	t.Run("should track test_files.analysis_id for reverse lookup", func(t *testing.T) {
		_, err := pool.Exec(ctx, "TRUNCATE codebases CASCADE")
		if err != nil {
			t.Fatalf("failed to truncate: %v", err)
		}

		// Given: Analysis with multiple files
		analysisID, err := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "reverse-owner",
			Repo:           "reverse-repo",
			CommitSHA:      "reverse123",
			Branch:         "main",
			ExternalRepoID: "reverse-id-1",
			ParserVersion:  testParserVersion,
		})
		if err != nil {
			t.Fatalf("CreateAnalysisRecord failed: %v", err)
		}

		inventory := &analysis.Inventory{
			Files: []analysis.TestFile{
				{
					Path:      "file1.test.ts",
					Framework: "jest",
					Suites: []analysis.TestSuite{
						{
							Name:     "Suite1",
							Location: analysis.Location{StartLine: 5},
							Tests: []analysis.Test{
								{Name: "test1", Location: analysis.Location{StartLine: 7}},
							},
						},
					},
				},
				{
					Path:      "file2.test.ts",
					Framework: "vitest",
					Suites: []analysis.TestSuite{
						{
							Name:     "Suite2",
							Location: analysis.Location{StartLine: 10},
							Tests: []analysis.Test{
								{Name: "test2", Location: analysis.Location{StartLine: 12}},
							},
						},
					},
				},
			},
		}

		err = repo.SaveAnalysisInventory(ctx, analysis.SaveAnalysisInventoryParams{
			AnalysisID: analysisID,
			Inventory:  inventory,
		})
		if err != nil {
			t.Fatalf("SaveAnalysisInventory failed: %v", err)
		}

		// Then: All test_files should have correct analysis_id
		pgID := toPgUUID(analysisID)
		rows, err := pool.Query(ctx, `
			SELECT file_path, framework
			FROM test_files
			WHERE analysis_id = $1
			ORDER BY file_path
		`, pgID)
		if err != nil {
			t.Fatalf("failed to query test_files: %v", err)
		}
		defer rows.Close()

		type fileRow struct {
			filePath  string
			framework pgtype.Text
		}
		var files []fileRow
		for rows.Next() {
			var f fileRow
			if err := rows.Scan(&f.filePath, &f.framework); err != nil {
				t.Fatalf("failed to scan row: %v", err)
			}
			files = append(files, f)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("row iteration error: %v", err)
		}

		if len(files) != 2 {
			t.Fatalf("expected 2 files, got %d", len(files))
		}

		if files[0].filePath != "file1.test.ts" || files[0].framework.String != "jest" {
			t.Errorf("expected file1.test.ts with jest, got %q with %q", files[0].filePath, files[0].framework.String)
		}

		if files[1].filePath != "file2.test.ts" || files[1].framework.String != "vitest" {
			t.Errorf("expected file2.test.ts with vitest, got %q with %q", files[1].filePath, files[1].framework.String)
		}
	})
}

func TestAnalysisRepository_UserAnalysisHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	repo := NewAnalysisRepository(pool)
	ctx := context.Background()

	t.Run("should record history when UserID is provided", func(t *testing.T) {
		// Create a test user
		var userID string
		err := pool.QueryRow(ctx, "INSERT INTO users (email, username) VALUES ('test@example.com', 'testuser') RETURNING id::text").Scan(&userID)
		if err != nil {
			t.Fatalf("failed to create test user: %v", err)
		}

		analysisID, err := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "history-owner",
			Repo:           "history-repo",
			CommitSHA:      "hist123",
			Branch:         "main",
			ExternalRepoID: "history-id-1",
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

		err = repo.SaveAnalysisInventory(ctx, analysis.SaveAnalysisInventoryParams{
			AnalysisID: analysisID,
			Inventory:  inventory,
			UserID:     &userID,
		})
		if err != nil {
			t.Fatalf("SaveAnalysisInventory failed: %v", err)
		}

		var historyCount int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM user_analysis_history WHERE user_id = $1", userID).Scan(&historyCount)
		if err != nil {
			t.Fatalf("failed to query history: %v", err)
		}
		if historyCount != 1 {
			t.Errorf("expected 1 history record, got %d", historyCount)
		}
	})

	t.Run("should not record history when UserID is nil", func(t *testing.T) {
		_, err := pool.Exec(ctx, "TRUNCATE codebases CASCADE")
		if err != nil {
			t.Fatalf("failed to truncate: %v", err)
		}
		_, err = pool.Exec(ctx, "DELETE FROM user_analysis_history")
		if err != nil {
			t.Fatalf("failed to delete history: %v", err)
		}

		analysisID, err := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "anon-owner",
			Repo:           "anon-repo",
			CommitSHA:      "anon123",
			Branch:         "main",
			ExternalRepoID: "anon-id-1",
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

		err = repo.SaveAnalysisInventory(ctx, analysis.SaveAnalysisInventoryParams{
			AnalysisID: analysisID,
			Inventory:  inventory,
			UserID:     nil,
		})
		if err != nil {
			t.Fatalf("SaveAnalysisInventory failed: %v", err)
		}

		var historyCount int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM user_analysis_history").Scan(&historyCount)
		if err != nil {
			t.Fatalf("failed to query history: %v", err)
		}
		if historyCount != 0 {
			t.Errorf("expected 0 history records for nil UserID, got %d", historyCount)
		}
	})

	t.Run("should update updated_at on reanalysis", func(t *testing.T) {
		_, err := pool.Exec(ctx, "TRUNCATE codebases CASCADE")
		if err != nil {
			t.Fatalf("failed to truncate: %v", err)
		}

		var userID string
		err = pool.QueryRow(ctx, "INSERT INTO users (email, username) VALUES ('reanalysis@example.com', 'reanalysisuser') RETURNING id::text").Scan(&userID)
		if err != nil {
			t.Fatalf("failed to create test user: %v", err)
		}

		// First analysis
		analysisID1, err := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "reanalysis-owner",
			Repo:           "reanalysis-repo",
			CommitSHA:      "commit1",
			Branch:         "main",
			ExternalRepoID: "reanalysis-id",
			ParserVersion:  testParserVersion,
		})
		if err != nil {
			t.Fatalf("first CreateAnalysisRecord failed: %v", err)
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

		err = repo.SaveAnalysisInventory(ctx, analysis.SaveAnalysisInventoryParams{
			AnalysisID: analysisID1,
			Inventory:  inventory,
			UserID:     &userID,
		})
		if err != nil {
			t.Fatalf("first SaveAnalysisInventory failed: %v", err)
		}

		var createdAt, updatedAt1 time.Time
		err = pool.QueryRow(ctx,
			"SELECT created_at, updated_at FROM user_analysis_history WHERE user_id = $1 AND analysis_id = $2",
			userID, toPgUUID(analysisID1),
		).Scan(&createdAt, &updatedAt1)
		if err != nil {
			t.Fatalf("failed to query first history: %v", err)
		}

		// Wait a bit to ensure time difference
		time.Sleep(10 * time.Millisecond)

		// Same user re-analyzes same analysis (trigger UPSERT)
		// Need to use a different commit for a new analysis, then manually test UPSERT
		// Actually, the UPSERT is on (user_id, analysis_id), so we need to call SaveAnalysisInventory again
		// But that would fail due to ErrAlreadyCompleted. Let's test the UPSERT directly.

		_, err = pool.Exec(ctx,
			`INSERT INTO user_analysis_history (user_id, analysis_id)
			 VALUES ($1, $2)
			 ON CONFLICT ON CONSTRAINT uq_user_analysis_history_user_analysis
			 DO UPDATE SET updated_at = now()`,
			userID, toPgUUID(analysisID1),
		)
		if err != nil {
			t.Fatalf("UPSERT failed: %v", err)
		}

		var updatedAt2 time.Time
		err = pool.QueryRow(ctx,
			"SELECT updated_at FROM user_analysis_history WHERE user_id = $1 AND analysis_id = $2",
			userID, toPgUUID(analysisID1),
		).Scan(&updatedAt2)
		if err != nil {
			t.Fatalf("failed to query updated history: %v", err)
		}

		if !updatedAt2.After(updatedAt1) {
			t.Errorf("expected updated_at to be updated, got updatedAt1=%v, updatedAt2=%v", updatedAt1, updatedAt2)
		}
	})

	t.Run("different users can analyze same repo", func(t *testing.T) {
		_, err := pool.Exec(ctx, "TRUNCATE codebases CASCADE")
		if err != nil {
			t.Fatalf("failed to truncate: %v", err)
		}

		var userID1, userID2 string
		err = pool.QueryRow(ctx, "INSERT INTO users (email, username) VALUES ('user1@example.com', 'user1') RETURNING id::text").Scan(&userID1)
		if err != nil {
			t.Fatalf("failed to create test user1: %v", err)
		}
		err = pool.QueryRow(ctx, "INSERT INTO users (email, username) VALUES ('user2@example.com', 'user2') RETURNING id::text").Scan(&userID2)
		if err != nil {
			t.Fatalf("failed to create test user2: %v", err)
		}

		// First user analyzes
		analysisID, err := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "shared-owner",
			Repo:           "shared-repo",
			CommitSHA:      "shared123",
			Branch:         "main",
			ExternalRepoID: "shared-id",
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

		err = repo.SaveAnalysisInventory(ctx, analysis.SaveAnalysisInventoryParams{
			AnalysisID: analysisID,
			Inventory:  inventory,
			UserID:     &userID1,
		})
		if err != nil {
			t.Fatalf("first user SaveAnalysisInventory failed: %v", err)
		}

		// Second user records history for same analysis
		_, err = pool.Exec(ctx,
			`INSERT INTO user_analysis_history (user_id, analysis_id) VALUES ($1, $2)`,
			userID2, toPgUUID(analysisID),
		)
		if err != nil {
			t.Fatalf("second user history insert failed: %v", err)
		}

		var historyCount int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM user_analysis_history WHERE analysis_id = $1", toPgUUID(analysisID)).Scan(&historyCount)
		if err != nil {
			t.Fatalf("failed to query history: %v", err)
		}
		if historyCount != 2 {
			t.Errorf("expected 2 history records (one per user), got %d", historyCount)
		}
	})

	t.Run("should store retention_days_at_creation from active subscription", func(t *testing.T) {
		_, err := pool.Exec(ctx, "TRUNCATE codebases CASCADE")
		if err != nil {
			t.Fatalf("failed to truncate: %v", err)
		}

		var userID string
		err = pool.QueryRow(ctx, "INSERT INTO users (email, username) VALUES ('retention@example.com', 'retentionuser') RETURNING id::text").Scan(&userID)
		if err != nil {
			t.Fatalf("failed to create test user: %v", err)
		}

		var planID pgtype.UUID
		err = pool.QueryRow(ctx, `
			INSERT INTO subscription_plans (tier, retention_days, specview_monthly_limit, analysis_monthly_limit)
			VALUES ('pro', 90, 100, 100) RETURNING id
		`).Scan(&planID)
		if err != nil {
			t.Fatalf("failed to create subscription plan: %v", err)
		}

		_, err = pool.Exec(ctx, `
			INSERT INTO user_subscriptions (user_id, plan_id, status, current_period_start, current_period_end)
			VALUES ($1, $2, 'active', now(), now() + interval '1 month')
		`, userID, planID)
		if err != nil {
			t.Fatalf("failed to create user subscription: %v", err)
		}

		analysisID, err := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "retention-owner",
			Repo:           "retention-repo",
			CommitSHA:      "ret123",
			Branch:         "main",
			ExternalRepoID: "retention-id",
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

		err = repo.SaveAnalysisInventory(ctx, analysis.SaveAnalysisInventoryParams{
			AnalysisID: analysisID,
			Inventory:  inventory,
			UserID:     &userID,
		})
		if err != nil {
			t.Fatalf("SaveAnalysisInventory failed: %v", err)
		}

		var retentionDays pgtype.Int4
		err = pool.QueryRow(ctx,
			"SELECT retention_days_at_creation FROM user_analysis_history WHERE user_id = $1",
			userID,
		).Scan(&retentionDays)
		if err != nil {
			t.Fatalf("failed to query retention_days: %v", err)
		}

		if !retentionDays.Valid {
			t.Error("expected retention_days_at_creation to be set")
		}
		if retentionDays.Int32 != 90 {
			t.Errorf("expected retention_days_at_creation = 90, got %d", retentionDays.Int32)
		}
	})

	t.Run("should store NULL retention_days_at_creation for enterprise plan", func(t *testing.T) {
		_, err := pool.Exec(ctx, "TRUNCATE codebases CASCADE")
		if err != nil {
			t.Fatalf("failed to truncate: %v", err)
		}

		var userID string
		err = pool.QueryRow(ctx, "INSERT INTO users (email, username) VALUES ('enterprise@example.com', 'enterpriseuser') RETURNING id::text").Scan(&userID)
		if err != nil {
			t.Fatalf("failed to create test user: %v", err)
		}

		var planID pgtype.UUID
		err = pool.QueryRow(ctx, `
			INSERT INTO subscription_plans (tier, retention_days, specview_monthly_limit, analysis_monthly_limit)
			VALUES ('enterprise', NULL, NULL, NULL) RETURNING id
		`).Scan(&planID)
		if err != nil {
			t.Fatalf("failed to create enterprise plan: %v", err)
		}

		_, err = pool.Exec(ctx, `
			INSERT INTO user_subscriptions (user_id, plan_id, status, current_period_start, current_period_end)
			VALUES ($1, $2, 'active', now(), now() + interval '1 year')
		`, userID, planID)
		if err != nil {
			t.Fatalf("failed to create user subscription: %v", err)
		}

		analysisID, err := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "enterprise-owner",
			Repo:           "enterprise-repo",
			CommitSHA:      "ent123",
			Branch:         "main",
			ExternalRepoID: "enterprise-id",
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

		err = repo.SaveAnalysisInventory(ctx, analysis.SaveAnalysisInventoryParams{
			AnalysisID: analysisID,
			Inventory:  inventory,
			UserID:     &userID,
		})
		if err != nil {
			t.Fatalf("SaveAnalysisInventory failed: %v", err)
		}

		var retentionDays pgtype.Int4
		err = pool.QueryRow(ctx,
			"SELECT retention_days_at_creation FROM user_analysis_history WHERE user_id = $1",
			userID,
		).Scan(&retentionDays)
		if err != nil {
			t.Fatalf("failed to query retention_days: %v", err)
		}

		if retentionDays.Valid {
			t.Errorf("expected retention_days_at_creation to be NULL for enterprise, got %d", retentionDays.Int32)
		}
	})

	t.Run("should store NULL retention_days_at_creation for user without subscription", func(t *testing.T) {
		_, err := pool.Exec(ctx, "TRUNCATE codebases CASCADE")
		if err != nil {
			t.Fatalf("failed to truncate: %v", err)
		}

		var userID string
		err = pool.QueryRow(ctx, "INSERT INTO users (email, username) VALUES ('nosub@example.com', 'nosubuser') RETURNING id::text").Scan(&userID)
		if err != nil {
			t.Fatalf("failed to create test user: %v", err)
		}

		analysisID, err := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "nosub-owner",
			Repo:           "nosub-repo",
			CommitSHA:      "nosub123",
			Branch:         "main",
			ExternalRepoID: "nosub-id",
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

		err = repo.SaveAnalysisInventory(ctx, analysis.SaveAnalysisInventoryParams{
			AnalysisID: analysisID,
			Inventory:  inventory,
			UserID:     &userID,
		})
		if err != nil {
			t.Fatalf("SaveAnalysisInventory failed: %v", err)
		}

		var retentionDays pgtype.Int4
		err = pool.QueryRow(ctx,
			"SELECT retention_days_at_creation FROM user_analysis_history WHERE user_id = $1",
			userID,
		).Scan(&retentionDays)
		if err != nil {
			t.Fatalf("failed to query retention_days: %v", err)
		}

		if retentionDays.Valid {
			t.Errorf("expected retention_days_at_creation to be NULL for user without subscription, got %d", retentionDays.Int32)
		}
	})
}

func TestAnalysisRepository_SaveAnalysisBatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	repo := NewAnalysisRepository(pool)
	ctx := context.Background()

	t.Run("should save batch of files", func(t *testing.T) {
		analysisID, err := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "batch-owner",
			Repo:           "batch-repo",
			CommitSHA:      "batch123",
			Branch:         "main",
			ExternalRepoID: "batch-id-1",
			ParserVersion:  testParserVersion,
		})
		if err != nil {
			t.Fatalf("CreateAnalysisRecord failed: %v", err)
		}

		files := []analysis.TestFile{
			{
				Path:      "auth/login.test.ts",
				Framework: "jest",
				Suites: []analysis.TestSuite{
					{
						Name:     "LoginService",
						Location: analysis.Location{StartLine: 10},
						Tests: []analysis.Test{
							{Name: "should login user", Location: analysis.Location{StartLine: 12}},
							{Name: "should reject invalid password", Location: analysis.Location{StartLine: 20}},
						},
					},
				},
			},
			{
				Path:      "auth/logout.test.ts",
				Framework: "jest",
				Suites: []analysis.TestSuite{
					{
						Name:     "LogoutService",
						Location: analysis.Location{StartLine: 5},
						Tests: []analysis.Test{
							{Name: "should logout user", Location: analysis.Location{StartLine: 7}},
						},
					},
				},
			},
		}

		stats, err := repo.SaveAnalysisBatch(ctx, analysis.SaveAnalysisBatchParams{
			AnalysisID: analysisID,
			Files:      files,
		})
		if err != nil {
			t.Fatalf("SaveAnalysisBatch failed: %v", err)
		}

		if stats.FilesProcessed != 2 {
			t.Errorf("expected 2 files processed, got %d", stats.FilesProcessed)
		}
		if stats.SuitesProcessed != 2 {
			t.Errorf("expected 2 suites processed, got %d", stats.SuitesProcessed)
		}
		if stats.TestsProcessed != 3 {
			t.Errorf("expected 3 tests processed, got %d", stats.TestsProcessed)
		}

		pgID := toPgUUID(analysisID)
		var fileCount int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_files WHERE analysis_id = $1", pgID).Scan(&fileCount)
		if err != nil {
			t.Fatalf("failed to query test_files: %v", err)
		}
		if fileCount != 2 {
			t.Errorf("expected 2 test_files in DB, got %d", fileCount)
		}
	})

	t.Run("should accumulate multiple batches", func(t *testing.T) {
		_, err := pool.Exec(ctx, "TRUNCATE codebases CASCADE")
		if err != nil {
			t.Fatalf("failed to truncate: %v", err)
		}

		analysisID, err := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "multi-batch-owner",
			Repo:           "multi-batch-repo",
			CommitSHA:      "multi123",
			Branch:         "main",
			ExternalRepoID: "multi-batch-id",
			ParserVersion:  testParserVersion,
		})
		if err != nil {
			t.Fatalf("CreateAnalysisRecord failed: %v", err)
		}

		batch1 := []analysis.TestFile{
			{
				Path:      "batch1/file1.test.ts",
				Framework: "jest",
				Suites: []analysis.TestSuite{
					{
						Name:     "Suite1",
						Location: analysis.Location{StartLine: 1},
						Tests: []analysis.Test{
							{Name: "test1", Location: analysis.Location{StartLine: 2}},
						},
					},
				},
			},
		}

		batch2 := []analysis.TestFile{
			{
				Path:      "batch2/file2.test.ts",
				Framework: "vitest",
				Suites: []analysis.TestSuite{
					{
						Name:     "Suite2",
						Location: analysis.Location{StartLine: 1},
						Tests: []analysis.Test{
							{Name: "test2", Location: analysis.Location{StartLine: 2}},
						},
					},
				},
			},
		}

		stats1, err := repo.SaveAnalysisBatch(ctx, analysis.SaveAnalysisBatchParams{
			AnalysisID: analysisID,
			Files:      batch1,
		})
		if err != nil {
			t.Fatalf("first SaveAnalysisBatch failed: %v", err)
		}

		stats2, err := repo.SaveAnalysisBatch(ctx, analysis.SaveAnalysisBatchParams{
			AnalysisID: analysisID,
			Files:      batch2,
		})
		if err != nil {
			t.Fatalf("second SaveAnalysisBatch failed: %v", err)
		}

		totalFiles := stats1.FilesProcessed + stats2.FilesProcessed
		totalTests := stats1.TestsProcessed + stats2.TestsProcessed

		if totalFiles != 2 {
			t.Errorf("expected total 2 files, got %d", totalFiles)
		}
		if totalTests != 2 {
			t.Errorf("expected total 2 tests, got %d", totalTests)
		}

		pgID := toPgUUID(analysisID)
		var fileCount int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_files WHERE analysis_id = $1", pgID).Scan(&fileCount)
		if err != nil {
			t.Fatalf("failed to query test_files: %v", err)
		}
		if fileCount != 2 {
			t.Errorf("expected 2 test_files in DB after two batches, got %d", fileCount)
		}
	})

	t.Run("should reject empty files", func(t *testing.T) {
		analysisID, _ := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "empty-batch-owner",
			Repo:           "empty-batch-repo",
			CommitSHA:      "empty123",
			Branch:         "main",
			ExternalRepoID: "empty-batch-id",
			ParserVersion:  testParserVersion,
		})

		_, err := repo.SaveAnalysisBatch(ctx, analysis.SaveAnalysisBatchParams{
			AnalysisID: analysisID,
			Files:      []analysis.TestFile{},
		})
		if !errors.Is(err, analysis.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput for empty files, got %v", err)
		}
	})

	t.Run("should reject nil UUID", func(t *testing.T) {
		_, err := repo.SaveAnalysisBatch(ctx, analysis.SaveAnalysisBatchParams{
			AnalysisID: analysis.NilUUID,
			Files: []analysis.TestFile{
				{Path: "test.ts", Framework: "jest"},
			},
		})
		if !errors.Is(err, analysis.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput for nil UUID, got %v", err)
		}
	})
}

func TestAnalysisRepository_FinalizeAnalysis(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := testdb.SetupTestDB(t)
	defer cleanup()

	repo := NewAnalysisRepository(pool)
	ctx := context.Background()

	t.Run("should finalize analysis with totals", func(t *testing.T) {
		analysisID, err := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "finalize-owner",
			Repo:           "finalize-repo",
			CommitSHA:      "finalize123",
			Branch:         "main",
			ExternalRepoID: "finalize-id-1",
			ParserVersion:  testParserVersion,
		})
		if err != nil {
			t.Fatalf("CreateAnalysisRecord failed: %v", err)
		}

		files := []analysis.TestFile{
			{
				Path:      "test.ts",
				Framework: "jest",
				Suites: []analysis.TestSuite{
					{
						Name:     "Suite",
						Location: analysis.Location{StartLine: 1},
						Tests: []analysis.Test{
							{Name: "test1", Location: analysis.Location{StartLine: 2}},
							{Name: "test2", Location: analysis.Location{StartLine: 3}},
						},
					},
				},
			},
		}

		_, err = repo.SaveAnalysisBatch(ctx, analysis.SaveAnalysisBatchParams{
			AnalysisID: analysisID,
			Files:      files,
		})
		if err != nil {
			t.Fatalf("SaveAnalysisBatch failed: %v", err)
		}

		committedAt := time.Now().Add(-time.Hour)
		err = repo.FinalizeAnalysis(ctx, analysis.FinalizeAnalysisParams{
			AnalysisID:  analysisID,
			CommittedAt: committedAt,
			TotalSuites: 1,
			TotalTests:  2,
		})
		if err != nil {
			t.Fatalf("FinalizeAnalysis failed: %v", err)
		}

		pgID := toPgUUID(analysisID)
		var status string
		var totalSuites, totalTests int
		err = pool.QueryRow(ctx, "SELECT status, total_suites, total_tests FROM analyses WHERE id = $1", pgID).
			Scan(&status, &totalSuites, &totalTests)
		if err != nil {
			t.Fatalf("failed to query analysis: %v", err)
		}

		if status != "completed" {
			t.Errorf("expected status 'completed', got '%s'", status)
		}
		if totalSuites != 1 {
			t.Errorf("expected 1 total suite, got %d", totalSuites)
		}
		if totalTests != 2 {
			t.Errorf("expected 2 total tests, got %d", totalTests)
		}
	})

	t.Run("should record user history on finalize", func(t *testing.T) {
		_, err := pool.Exec(ctx, "TRUNCATE codebases CASCADE")
		if err != nil {
			t.Fatalf("failed to truncate: %v", err)
		}

		var userID string
		err = pool.QueryRow(ctx, "INSERT INTO users (email, username) VALUES ('finalize@example.com', 'finalizeuser') RETURNING id::text").Scan(&userID)
		if err != nil {
			t.Fatalf("failed to create test user: %v", err)
		}

		analysisID, err := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "finalize-user-owner",
			Repo:           "finalize-user-repo",
			CommitSHA:      "finalizeuser123",
			Branch:         "main",
			ExternalRepoID: "finalize-user-id",
			ParserVersion:  testParserVersion,
		})
		if err != nil {
			t.Fatalf("CreateAnalysisRecord failed: %v", err)
		}

		err = repo.FinalizeAnalysis(ctx, analysis.FinalizeAnalysisParams{
			AnalysisID:  analysisID,
			TotalSuites: 0,
			TotalTests:  0,
			UserID:      &userID,
		})
		if err != nil {
			t.Fatalf("FinalizeAnalysis failed: %v", err)
		}

		var historyCount int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM user_analysis_history WHERE user_id = $1", userID).Scan(&historyCount)
		if err != nil {
			t.Fatalf("failed to query history: %v", err)
		}
		if historyCount != 1 {
			t.Errorf("expected 1 history record, got %d", historyCount)
		}
	})

	t.Run("should reject nil UUID", func(t *testing.T) {
		err := repo.FinalizeAnalysis(ctx, analysis.FinalizeAnalysisParams{
			AnalysisID: analysis.NilUUID,
		})
		if !errors.Is(err, analysis.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput for nil UUID, got %v", err)
		}
	})

	t.Run("should reject double finalize", func(t *testing.T) {
		_, err := pool.Exec(ctx, "TRUNCATE codebases CASCADE")
		if err != nil {
			t.Fatalf("failed to truncate: %v", err)
		}

		analysisID, err := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "double-finalize-owner",
			Repo:           "double-finalize-repo",
			CommitSHA:      "double123",
			Branch:         "main",
			ExternalRepoID: "double-finalize-id",
			ParserVersion:  testParserVersion,
		})
		if err != nil {
			t.Fatalf("CreateAnalysisRecord failed: %v", err)
		}

		err = repo.FinalizeAnalysis(ctx, analysis.FinalizeAnalysisParams{
			AnalysisID: analysisID,
		})
		if err != nil {
			t.Fatalf("first FinalizeAnalysis failed: %v", err)
		}

		err = repo.FinalizeAnalysis(ctx, analysis.FinalizeAnalysisParams{
			AnalysisID: analysisID,
		})
		if !errors.Is(err, analysis.ErrAlreadyCompleted) {
			t.Errorf("expected ErrAlreadyCompleted for double finalize, got %v", err)
		}
	})

	t.Run("should reject negative totals", func(t *testing.T) {
		analysisID, _ := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:          "negative-owner",
			Repo:           "negative-repo",
			CommitSHA:      "negative123",
			Branch:         "main",
			ExternalRepoID: "negative-id",
			ParserVersion:  testParserVersion,
		})

		err := repo.FinalizeAnalysis(ctx, analysis.FinalizeAnalysisParams{
			AnalysisID:  analysisID,
			TotalSuites: -1,
			TotalTests:  0,
		})
		if !errors.Is(err, analysis.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput for negative suites, got %v", err)
		}

		err = repo.FinalizeAnalysis(ctx, analysis.FinalizeAnalysisParams{
			AnalysisID:  analysisID,
			TotalSuites: 0,
			TotalTests:  -1,
		})
		if !errors.Is(err, analysis.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput for negative tests, got %v", err)
		}
	})
}
