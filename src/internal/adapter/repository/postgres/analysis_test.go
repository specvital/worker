package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/specvital/collector/internal/domain/analysis"
	testdb "github.com/specvital/collector/internal/testutil/postgres"
	"github.com/specvital/core/pkg/domain"
	"github.com/specvital/core/pkg/parser"
)

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
				Owner:     "owner",
				Repo:      "repo",
				CommitSHA: "abc123",
				Branch:    "main",
				Result:    &parser.ScanResult{},
			},
			wantErr: false,
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
		suites, tests := flattenInventory(nil)
		if suites != nil || tests != nil {
			t.Errorf("expected nil, nil for nil inventory")
		}
	})

	t.Run("empty inventory returns empty slices", func(t *testing.T) {
		inv := &analysis.Inventory{}
		suites, tests := flattenInventory(inv)
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

		suites, tests := flattenInventory(inv)

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

		suites, tests := flattenInventory(inv)

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
}
