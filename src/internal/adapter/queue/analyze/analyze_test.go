package analyze

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/specvital/worker/internal/domain/analysis"
	uc "github.com/specvital/worker/internal/usecase/analysis"
)

const testParserVersion = "v1.0.0-test"

// Mock implementations for testing

type mockVCS struct {
	cloneFn         func(ctx context.Context, url string, token *string) (analysis.Source, error)
	getHeadCommitFn func(ctx context.Context, url string, token *string) (analysis.CommitInfo, error)
}

func (m *mockVCS) Clone(ctx context.Context, url string, token *string) (analysis.Source, error) {
	if m.cloneFn != nil {
		return m.cloneFn(ctx, url, token)
	}
	return nil, nil
}

func (m *mockVCS) GetHeadCommit(ctx context.Context, url string, token *string) (analysis.CommitInfo, error) {
	if m.getHeadCommitFn != nil {
		return m.getHeadCommitFn(ctx, url, token)
	}
	return analysis.CommitInfo{SHA: "test-commit-sha", IsPrivate: false}, nil
}

type mockSource struct {
	branchFn             func() string
	commitSHAFn          func() string
	closeFn              func(ctx context.Context) error
	verifyCommitExistsFn func(ctx context.Context, sha string) (bool, error)
}

func (m *mockSource) Branch() string {
	if m.branchFn != nil {
		return m.branchFn()
	}
	return "main"
}

func (m *mockSource) CommitSHA() string {
	if m.commitSHAFn != nil {
		return m.commitSHAFn()
	}
	return "abc123"
}

func (m *mockSource) CommittedAt() time.Time {
	return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
}

func (m *mockSource) Close(ctx context.Context) error {
	if m.closeFn != nil {
		return m.closeFn(ctx)
	}
	return nil
}

func (m *mockSource) VerifyCommitExists(ctx context.Context, sha string) (bool, error) {
	if m.verifyCommitExistsFn != nil {
		return m.verifyCommitExistsFn(ctx, sha)
	}
	return true, nil
}

type mockParser struct {
	scanFn func(ctx context.Context, src analysis.Source) (*analysis.Inventory, error)
}

func (m *mockParser) Scan(ctx context.Context, src analysis.Source) (*analysis.Inventory, error) {
	if m.scanFn != nil {
		return m.scanFn(ctx, src)
	}
	return &analysis.Inventory{Files: []analysis.TestFile{}}, nil
}

type mockRepository struct {
	createAnalysisRecordFn  func(ctx context.Context, params analysis.CreateAnalysisRecordParams) (analysis.UUID, error)
	recordFailureFn         func(ctx context.Context, analysisID analysis.UUID, errMessage string) error
	saveAnalysisInventoryFn func(ctx context.Context, params analysis.SaveAnalysisInventoryParams) error
}

func (m *mockRepository) CreateAnalysisRecord(ctx context.Context, params analysis.CreateAnalysisRecordParams) (analysis.UUID, error) {
	if m.createAnalysisRecordFn != nil {
		return m.createAnalysisRecordFn(ctx, params)
	}
	return analysis.NewUUID(), nil
}

func (m *mockRepository) RecordFailure(ctx context.Context, analysisID analysis.UUID, errMessage string) error {
	if m.recordFailureFn != nil {
		return m.recordFailureFn(ctx, analysisID, errMessage)
	}
	return nil
}

func (m *mockRepository) SaveAnalysisInventory(ctx context.Context, params analysis.SaveAnalysisInventoryParams) error {
	if m.saveAnalysisInventoryFn != nil {
		return m.saveAnalysisInventoryFn(ctx, params)
	}
	return nil
}

type mockCodebaseRepository struct{}

func (m *mockCodebaseRepository) FindByExternalID(ctx context.Context, host, externalRepoID string) (*analysis.Codebase, error) {
	return nil, analysis.ErrCodebaseNotFound
}

func (m *mockCodebaseRepository) FindByOwnerName(ctx context.Context, host, owner, name string) (*analysis.Codebase, error) {
	return nil, analysis.ErrCodebaseNotFound
}

func (m *mockCodebaseRepository) FindWithLastCommit(ctx context.Context, host, owner, name string) (*analysis.Codebase, error) {
	return nil, analysis.ErrCodebaseNotFound
}

func (m *mockCodebaseRepository) MarkStale(ctx context.Context, id analysis.UUID) error {
	return nil
}

func (m *mockCodebaseRepository) MarkStaleAndUpsert(ctx context.Context, staleID analysis.UUID, params analysis.UpsertCodebaseParams) (*analysis.Codebase, error) {
	return &analysis.Codebase{
		ID:             analysis.NewUUID(),
		Host:           params.Host,
		Owner:          params.Owner,
		Name:           params.Name,
		ExternalRepoID: params.ExternalRepoID,
	}, nil
}

func (m *mockCodebaseRepository) UnmarkStale(ctx context.Context, id analysis.UUID, owner, name string) (*analysis.Codebase, error) {
	return &analysis.Codebase{ID: id, Owner: owner, Name: name}, nil
}

func (m *mockCodebaseRepository) UpdateOwnerName(ctx context.Context, id analysis.UUID, owner, name string) (*analysis.Codebase, error) {
	return &analysis.Codebase{ID: id, Owner: owner, Name: name}, nil
}

func (m *mockCodebaseRepository) UpdateVisibility(ctx context.Context, id analysis.UUID, isPrivate bool) error {
	return nil
}

func (m *mockCodebaseRepository) Upsert(ctx context.Context, params analysis.UpsertCodebaseParams) (*analysis.Codebase, error) {
	return &analysis.Codebase{
		ID:             analysis.NewUUID(),
		Host:           params.Host,
		Owner:          params.Owner,
		Name:           params.Name,
		ExternalRepoID: params.ExternalRepoID,
	}, nil
}

type mockVCSAPIClient struct{}

func (m *mockVCSAPIClient) GetRepoInfo(ctx context.Context, host, owner, repo string, token *string) (analysis.RepoInfo, error) {
	return analysis.RepoInfo{
		ExternalRepoID: "123456",
		Name:           repo,
		Owner:          owner,
	}, nil
}

// Test helper functions

func newSuccessfulMocks() (*mockRepository, *mockVCS, *mockParser) {
	src := &mockSource{
		branchFn:    func() string { return "main" },
		commitSHAFn: func() string { return "abc123" },
		closeFn:     func(ctx context.Context) error { return nil },
	}

	vcs := &mockVCS{
		cloneFn: func(ctx context.Context, url string, token *string) (analysis.Source, error) {
			return src, nil
		},
	}

	repo := &mockRepository{
		createAnalysisRecordFn: func(ctx context.Context, params analysis.CreateAnalysisRecordParams) (analysis.UUID, error) {
			return analysis.NewUUID(), nil
		},
		saveAnalysisInventoryFn: func(ctx context.Context, params analysis.SaveAnalysisInventoryParams) error {
			return nil
		},
	}

	parser := &mockParser{
		scanFn: func(ctx context.Context, src analysis.Source) (*analysis.Inventory, error) {
			return &analysis.Inventory{Files: []analysis.TestFile{}}, nil
		},
	}

	return repo, vcs, parser
}

func newTestJob(args AnalyzeArgs) *river.Job[AnalyzeArgs] {
	return &river.Job[AnalyzeArgs]{
		JobRow: &rivertype.JobRow{
			ID: 1,
		},
		Args: args,
	}
}

// Tests

func TestNewAnalyzeWorker(t *testing.T) {
	repo, vcs, parser := newSuccessfulMocks()
	codebaseRepo := &mockCodebaseRepository{}
	vcsAPI := &mockVCSAPIClient{}
	analyzeUC := uc.NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, nil, uc.WithParserVersion(testParserVersion))

	worker := NewAnalyzeWorker(analyzeUC)

	if worker == nil {
		t.Error("expected worker, got nil")
	}
	if worker.analyzeUC == nil {
		t.Error("expected worker.analyzeUC to be set, got nil")
	}
}

func TestAnalyzeWorker_Work(t *testing.T) {
	tests := []struct {
		name        string
		args        AnalyzeArgs
		setupMocks  func() (*mockRepository, *mockVCS, *mockParser)
		wantErr     bool
		errContains string
	}{
		{
			name: "success case - valid args and use case succeeds",
			args: AnalyzeArgs{
				Owner:     "octocat",
				Repo:      "Hello-World",
				CommitSHA: "abc123",
			},
			setupMocks: func() (*mockRepository, *mockVCS, *mockParser) {
				return newSuccessfulMocks()
			},
			wantErr: false,
		},
		{
			name: "clone failed - VCS clone returns error",
			args: AnalyzeArgs{
				Owner:     "testowner",
				Repo:      "testrepo",
				CommitSHA: "abc123",
			},
			setupMocks: func() (*mockRepository, *mockVCS, *mockParser) {
				repo, _, parser := newSuccessfulMocks()
				vcs := &mockVCS{
					cloneFn: func(ctx context.Context, url string, token *string) (analysis.Source, error) {
						return nil, errors.New("git clone failed")
					},
				}
				return repo, vcs, parser
			},
			wantErr: true,
		},
		{
			name: "scan failed - parser returns error",
			args: AnalyzeArgs{
				Owner:     "testowner",
				Repo:      "testrepo",
				CommitSHA: "abc123",
			},
			setupMocks: func() (*mockRepository, *mockVCS, *mockParser) {
				repo, vcs, _ := newSuccessfulMocks()

				testAnalysisID := analysis.NewUUID()
				repo.createAnalysisRecordFn = func(ctx context.Context, params analysis.CreateAnalysisRecordParams) (analysis.UUID, error) {
					return testAnalysisID, nil
				}
				repo.recordFailureFn = func(ctx context.Context, analysisID analysis.UUID, errMessage string) error {
					return nil
				}

				parser := &mockParser{
					scanFn: func(ctx context.Context, src analysis.Source) (*analysis.Inventory, error) {
						return nil, errors.New("parser error")
					},
				}

				return repo, vcs, parser
			},
			wantErr: true,
		},
		{
			name: "save failed - repository save returns error",
			args: AnalyzeArgs{
				Owner:     "testowner",
				Repo:      "testrepo",
				CommitSHA: "abc123",
			},
			setupMocks: func() (*mockRepository, *mockVCS, *mockParser) {
				repo, vcs, parser := newSuccessfulMocks()

				testAnalysisID := analysis.NewUUID()
				repo.createAnalysisRecordFn = func(ctx context.Context, params analysis.CreateAnalysisRecordParams) (analysis.UUID, error) {
					return testAnalysisID, nil
				}
				repo.recordFailureFn = func(ctx context.Context, analysisID analysis.UUID, errMessage string) error {
					return nil
				}
				repo.saveAnalysisInventoryFn = func(ctx context.Context, params analysis.SaveAnalysisInventoryParams) error {
					return errors.New("database save error")
				}

				return repo, vcs, parser
			},
			wantErr: true,
		},
		{
			name: "invalid input - empty owner",
			args: AnalyzeArgs{
				Owner:     "",
				Repo:      "testrepo",
				CommitSHA: "abc123",
			},
			setupMocks: func() (*mockRepository, *mockVCS, *mockParser) {
				return newSuccessfulMocks()
			},
			wantErr: true,
		},
		{
			name: "invalid input - empty repo",
			args: AnalyzeArgs{
				Owner:     "testowner",
				Repo:      "",
				CommitSHA: "abc123",
			},
			setupMocks: func() (*mockRepository, *mockVCS, *mockParser) {
				return newSuccessfulMocks()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, vcs, parser := tt.setupMocks()
			codebaseRepo := &mockCodebaseRepository{}
			vcsAPI := &mockVCSAPIClient{}
			analyzeUC := uc.NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, nil, uc.WithParserVersion(testParserVersion))
			worker := NewAnalyzeWorker(analyzeUC)

			job := newTestJob(tt.args)
			err := worker.Work(context.Background(), job)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestAnalyzeWorker_Work_ContextPropagation(t *testing.T) {
	t.Run("should propagate context to use case", func(t *testing.T) {
		type ctxKey string
		testKey := ctxKey("test-key")
		testValue := "test-value"

		var capturedCtx context.Context
		repo, _, parser := newSuccessfulMocks()

		src := &mockSource{
			branchFn:    func() string { return "main" },
			commitSHAFn: func() string { return "abc123" },
			closeFn:     func(ctx context.Context) error { return nil },
		}

		vcs := &mockVCS{
			cloneFn: func(ctx context.Context, url string, token *string) (analysis.Source, error) {
				capturedCtx = ctx
				return src, nil
			},
		}

		codebaseRepo := &mockCodebaseRepository{}
		vcsAPI := &mockVCSAPIClient{}
		analyzeUC := uc.NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, nil, uc.WithParserVersion(testParserVersion))
		worker := NewAnalyzeWorker(analyzeUC)

		job := newTestJob(AnalyzeArgs{Owner: "owner", Repo: "repo", CommitSHA: "abc123"})
		ctx := context.WithValue(context.Background(), testKey, testValue)

		err := worker.Work(ctx, job)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capturedCtx == nil {
			t.Fatal("context was not propagated to use case")
		}
		if capturedCtx.Value(testKey) != testValue {
			t.Errorf("expected context value '%s', got '%v'", testValue, capturedCtx.Value(testKey))
		}
	})

	t.Run("should propagate cancelled context", func(t *testing.T) {
		repo, _, parser := newSuccessfulMocks()
		vcs := &mockVCS{
			cloneFn: func(ctx context.Context, url string, token *string) (analysis.Source, error) {
				return nil, ctx.Err()
			},
		}

		codebaseRepo := &mockCodebaseRepository{}
		vcsAPI := &mockVCSAPIClient{}
		analyzeUC := uc.NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, nil, uc.WithParserVersion(testParserVersion))
		worker := NewAnalyzeWorker(analyzeUC)

		job := newTestJob(AnalyzeArgs{Owner: "owner", Repo: "repo", CommitSHA: "abc123"})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := worker.Work(ctx, job)

		if err == nil {
			t.Error("expected error from cancelled context, got nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected error to wrap context.Canceled, got %v", err)
		}
	})
}

func TestAnalyzeWorker_Work_ErrorPropagation(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func() (*mockRepository, *mockVCS, *mockParser)
		args      AnalyzeArgs
		wantError error
	}{
		{
			name: "clone failed error",
			setupMock: func() (*mockRepository, *mockVCS, *mockParser) {
				repo, _, parser := newSuccessfulMocks()
				vcs := &mockVCS{
					cloneFn: func(ctx context.Context, url string, token *string) (analysis.Source, error) {
						return nil, errors.New("clone error")
					},
				}
				return repo, vcs, parser
			},
			args:      AnalyzeArgs{Owner: "owner", Repo: "repo", CommitSHA: "abc123"},
			wantError: uc.ErrCloneFailed,
		},
		{
			name: "scan failed error",
			setupMock: func() (*mockRepository, *mockVCS, *mockParser) {
				repo, vcs, _ := newSuccessfulMocks()

				testAnalysisID := analysis.NewUUID()
				repo.createAnalysisRecordFn = func(ctx context.Context, params analysis.CreateAnalysisRecordParams) (analysis.UUID, error) {
					return testAnalysisID, nil
				}
				repo.recordFailureFn = func(ctx context.Context, analysisID analysis.UUID, errMessage string) error {
					return nil
				}

				parser := &mockParser{
					scanFn: func(ctx context.Context, src analysis.Source) (*analysis.Inventory, error) {
						return nil, errors.New("scan error")
					},
				}

				return repo, vcs, parser
			},
			args:      AnalyzeArgs{Owner: "owner", Repo: "repo", CommitSHA: "abc123"},
			wantError: uc.ErrScanFailed,
		},
		{
			name: "save failed error",
			setupMock: func() (*mockRepository, *mockVCS, *mockParser) {
				repo, vcs, parser := newSuccessfulMocks()

				testAnalysisID := analysis.NewUUID()
				repo.createAnalysisRecordFn = func(ctx context.Context, params analysis.CreateAnalysisRecordParams) (analysis.UUID, error) {
					return testAnalysisID, nil
				}
				repo.recordFailureFn = func(ctx context.Context, analysisID analysis.UUID, errMessage string) error {
					return nil
				}
				repo.saveAnalysisInventoryFn = func(ctx context.Context, params analysis.SaveAnalysisInventoryParams) error {
					return errors.New("save error")
				}

				return repo, vcs, parser
			},
			args:      AnalyzeArgs{Owner: "owner", Repo: "repo", CommitSHA: "abc123"},
			wantError: uc.ErrSaveFailed,
		},
		{
			name: "invalid input error",
			setupMock: func() (*mockRepository, *mockVCS, *mockParser) {
				return newSuccessfulMocks()
			},
			args:      AnalyzeArgs{Owner: "", Repo: "repo", CommitSHA: "abc123"},
			wantError: analysis.ErrInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, vcs, parser := tt.setupMock()
			codebaseRepo := &mockCodebaseRepository{}
			vcsAPI := &mockVCSAPIClient{}
			analyzeUC := uc.NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, nil, uc.WithParserVersion(testParserVersion))
			worker := NewAnalyzeWorker(analyzeUC)

			job := newTestJob(tt.args)
			err := worker.Work(context.Background(), job)

			if err == nil {
				t.Errorf("expected error %v, got nil", tt.wantError)
				return
			}
			if !errors.Is(err, tt.wantError) {
				t.Errorf("expected error to wrap %v, got %v", tt.wantError, err)
			}
		})
	}
}

func TestAnalyzeArgs_Kind(t *testing.T) {
	args := AnalyzeArgs{}
	if args.Kind() != "analysis:analyze" {
		t.Errorf("expected kind 'analysis:analyze', got '%s'", args.Kind())
	}
}

func TestAnalyzeWorker_Work_AlreadyCompleted(t *testing.T) {
	t.Run("should return JobCancel for ErrAlreadyCompleted", func(t *testing.T) {
		repo, vcs, parser := newSuccessfulMocks()

		testAnalysisID := analysis.NewUUID()
		repo.createAnalysisRecordFn = func(ctx context.Context, params analysis.CreateAnalysisRecordParams) (analysis.UUID, error) {
			return testAnalysisID, nil
		}
		repo.recordFailureFn = func(ctx context.Context, analysisID analysis.UUID, errMessage string) error {
			return nil
		}
		repo.saveAnalysisInventoryFn = func(ctx context.Context, params analysis.SaveAnalysisInventoryParams) error {
			// Simulate duplicate key error from repository
			return analysis.ErrAlreadyCompleted
		}

		codebaseRepo := &mockCodebaseRepository{}
		vcsAPI := &mockVCSAPIClient{}
		analyzeUC := uc.NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, nil, uc.WithParserVersion(testParserVersion))
		worker := NewAnalyzeWorker(analyzeUC)

		job := newTestJob(AnalyzeArgs{Owner: "owner", Repo: "repo", CommitSHA: "abc123"})
		err := worker.Work(context.Background(), job)

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		// Check that ErrAlreadyCompleted is wrapped in the error chain
		if !errors.Is(err, analysis.ErrAlreadyCompleted) {
			t.Errorf("expected error to wrap ErrAlreadyCompleted, got %v", err)
		}
	})
}
