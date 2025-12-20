package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/specvital/collector/internal/domain/analysis"
	"github.com/specvital/core/pkg/domain"
	"github.com/specvital/core/pkg/parser"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
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
		// Connect to same network as devcontainer
		testcontainers.CustomizeRequest(testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Networks: []string{"specvital-network"},
			},
		}),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	// Get container IP on the shared network
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

	if err := runMigrations(ctx, pool); err != nil {
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

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	schema := `
		CREATE TYPE analysis_status AS ENUM ('pending', 'running', 'completed', 'failed');
		CREATE TYPE test_status AS ENUM ('active', 'skipped', 'todo');

		CREATE TABLE codebases (
			id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
			host varchar(255) DEFAULT 'github.com' NOT NULL,
			owner varchar(255) NOT NULL,
			name varchar(255) NOT NULL,
			default_branch varchar(100),
			created_at timestamptz DEFAULT now() NOT NULL,
			updated_at timestamptz DEFAULT now() NOT NULL,
			UNIQUE (host, owner, name)
		);

		CREATE TABLE analyses (
			id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
			codebase_id uuid NOT NULL REFERENCES codebases(id) ON DELETE CASCADE,
			commit_sha varchar(40) NOT NULL,
			branch_name varchar(255),
			status analysis_status DEFAULT 'pending' NOT NULL,
			error_message text,
			started_at timestamptz,
			completed_at timestamptz,
			created_at timestamptz DEFAULT now() NOT NULL,
			total_suites integer DEFAULT 0 NOT NULL,
			total_tests integer DEFAULT 0 NOT NULL,
			UNIQUE (codebase_id, commit_sha)
		);

		CREATE TABLE test_suites (
			id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
			analysis_id uuid NOT NULL REFERENCES analyses(id) ON DELETE CASCADE,
			parent_id uuid REFERENCES test_suites(id) ON DELETE CASCADE,
			name varchar(500) NOT NULL,
			file_path varchar(1000) NOT NULL,
			line_number integer,
			framework varchar(50),
			depth integer DEFAULT 0 NOT NULL,
			CONSTRAINT chk_no_self_reference CHECK (id <> parent_id)
		);

		CREATE TABLE test_cases (
			id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
			suite_id uuid NOT NULL REFERENCES test_suites(id) ON DELETE CASCADE,
			name varchar(500) NOT NULL,
			line_number integer,
			status test_status DEFAULT 'active' NOT NULL,
			tags jsonb DEFAULT '[]' NOT NULL,
			modifier varchar(50)
		);
	`
	_, err := pool.Exec(ctx, schema)
	return err
}

func TestAnalysisRepository_SaveAnalysisResult(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewAnalysisRepository(pool)
	ctx := context.Background()

	t.Run("should save analysis with suites and tests", func(t *testing.T) {
		params := SaveAnalysisResultParams{
			Owner:     "testowner",
			Repo:      "testrepo",
			CommitSHA: "abc123def456",
			Branch:    "main",
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
			Owner:     "owner2",
			Repo:      "repo2",
			CommitSHA: "def789",
			Branch:    "develop",
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
			Owner:     "owner3",
			Repo:      "repo3",
			CommitSHA: "ghi012",
			Branch:    "main",
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
			Owner:     "owner4",
			Repo:      "repo4",
			CommitSHA: "jkl345",
			Branch:    "main",
			Result:    &parser.ScanResult{Inventory: nil},
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

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewAnalysisRepository(pool)
	ctx := context.Background()

	t.Run("should rollback on duplicate commit", func(t *testing.T) {
		params := SaveAnalysisResultParams{
			Owner:     "rollback-owner",
			Repo:      "rollback-repo",
			CommitSHA: "same-commit-sha",
			Branch:    "main",
			Result:    &parser.ScanResult{Inventory: nil},
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

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewAnalysisRepository(pool)
	ctx := context.Background()

	t.Run("should record failure with error message", func(t *testing.T) {
		analysisID, err := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:     "test-owner",
			Repo:      "test-repo",
			CommitSHA: "abc123",
			Branch:    "main",
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
			Owner:     "empty-err-owner",
			Repo:      "empty-err-repo",
			CommitSHA: "empty123",
			Branch:    "main",
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

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewAnalysisRepository(pool)
	ctx := context.Background()

	t.Run("should create analysis record", func(t *testing.T) {
		analysisID, err := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:     "create-owner",
			Repo:      "create-repo",
			CommitSHA: "def456",
			Branch:    "develop",
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

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewAnalysisRepository(pool)
	ctx := context.Background()

	t.Run("should save inventory using domain types", func(t *testing.T) {
		analysisID, err := repo.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
			Owner:     "domain-owner",
			Repo:      "domain-repo",
			CommitSHA: "xyz789",
			Branch:    "main",
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
