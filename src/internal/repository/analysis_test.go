package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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

func TestPostgresAnalysisRepository_SaveAnalysisResult(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewPostgresAnalysisRepository(pool)
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

func TestPostgresAnalysisRepository_TransactionRollback(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewPostgresAnalysisRepository(pool)
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

func TestSaveAnalysisResultParams_Validate(t *testing.T) {
	tests := []struct {
		name    string
		params  SaveAnalysisResultParams
		wantErr bool
	}{
		{
			name: "valid params",
			params: SaveAnalysisResultParams{
				Owner:     "owner",
				Repo:      "repo",
				CommitSHA: "abc123",
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
				if !errors.Is(err, ErrInvalidParams) {
					t.Errorf("expected ErrInvalidParams, got %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
