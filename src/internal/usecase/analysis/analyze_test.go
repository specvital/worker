package analysis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/specvital/collector/internal/domain/analysis"
)

// Mock implementations

type mockVCS struct {
	cloneFn func(ctx context.Context, url string, token *string) (analysis.Source, error)
}

func (m *mockVCS) Clone(ctx context.Context, url string, token *string) (analysis.Source, error) {
	if m.cloneFn != nil {
		return m.cloneFn(ctx, url, token)
	}
	return nil, nil
}

func (m *mockVCS) GetHeadCommit(ctx context.Context, url string, token *string) (string, error) {
	return "test-commit-sha", nil
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
	return nil, nil
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

type mockCodebaseRepository struct {
	findByExternalIDFn   func(ctx context.Context, host, externalRepoID string) (*analysis.Codebase, error)
	findByOwnerNameFn    func(ctx context.Context, host, owner, name string) (*analysis.Codebase, error)
	findWithLastCommitFn func(ctx context.Context, host, owner, name string) (*analysis.Codebase, error)
	markStaleFn          func(ctx context.Context, id analysis.UUID) error
	unmarkStaleFn        func(ctx context.Context, id analysis.UUID, owner, name string) (*analysis.Codebase, error)
	updateOwnerNameFn    func(ctx context.Context, id analysis.UUID, owner, name string) (*analysis.Codebase, error)
	upsertFn             func(ctx context.Context, params analysis.UpsertCodebaseParams) (*analysis.Codebase, error)
}

func (m *mockCodebaseRepository) FindByExternalID(ctx context.Context, host, externalRepoID string) (*analysis.Codebase, error) {
	if m.findByExternalIDFn != nil {
		return m.findByExternalIDFn(ctx, host, externalRepoID)
	}
	return nil, analysis.ErrCodebaseNotFound
}

func (m *mockCodebaseRepository) FindByOwnerName(ctx context.Context, host, owner, name string) (*analysis.Codebase, error) {
	if m.findByOwnerNameFn != nil {
		return m.findByOwnerNameFn(ctx, host, owner, name)
	}
	return nil, analysis.ErrCodebaseNotFound
}

func (m *mockCodebaseRepository) FindWithLastCommit(ctx context.Context, host, owner, name string) (*analysis.Codebase, error) {
	if m.findWithLastCommitFn != nil {
		return m.findWithLastCommitFn(ctx, host, owner, name)
	}
	return nil, analysis.ErrCodebaseNotFound
}

func (m *mockCodebaseRepository) MarkStale(ctx context.Context, id analysis.UUID) error {
	if m.markStaleFn != nil {
		return m.markStaleFn(ctx, id)
	}
	return nil
}

func (m *mockCodebaseRepository) UnmarkStale(ctx context.Context, id analysis.UUID, owner, name string) (*analysis.Codebase, error) {
	if m.unmarkStaleFn != nil {
		return m.unmarkStaleFn(ctx, id, owner, name)
	}
	return &analysis.Codebase{ID: id, Owner: owner, Name: name}, nil
}

func (m *mockCodebaseRepository) UpdateOwnerName(ctx context.Context, id analysis.UUID, owner, name string) (*analysis.Codebase, error) {
	if m.updateOwnerNameFn != nil {
		return m.updateOwnerNameFn(ctx, id, owner, name)
	}
	return &analysis.Codebase{ID: id, Owner: owner, Name: name}, nil
}

func (m *mockCodebaseRepository) Upsert(ctx context.Context, params analysis.UpsertCodebaseParams) (*analysis.Codebase, error) {
	if m.upsertFn != nil {
		return m.upsertFn(ctx, params)
	}
	return &analysis.Codebase{
		ID:             analysis.NewUUID(),
		Host:           params.Host,
		Owner:          params.Owner,
		Name:           params.Name,
		ExternalRepoID: params.ExternalRepoID,
	}, nil
}

type mockVCSAPIClient struct {
	getRepoIDFn func(ctx context.Context, host, owner, repo string, token *string) (string, error)
}

func (m *mockVCSAPIClient) GetRepoID(ctx context.Context, host, owner, repo string, token *string) (string, error) {
	if m.getRepoIDFn != nil {
		return m.getRepoIDFn(ctx, host, owner, repo, token)
	}
	return "123456", nil
}

type mockTokenLookup struct {
	getOAuthTokenFn func(ctx context.Context, userID string, provider string) (string, error)
}

func (m *mockTokenLookup) GetOAuthToken(ctx context.Context, userID string, provider string) (string, error) {
	if m.getOAuthTokenFn != nil {
		return m.getOAuthTokenFn(ctx, userID, provider)
	}
	return "", nil
}

// Mock helpers to reduce duplication

func newSuccessfulSource() *mockSource {
	return &mockSource{
		branchFn:    func() string { return "main" },
		commitSHAFn: func() string { return "abc123" },
		closeFn:     func(ctx context.Context) error { return nil },
	}
}

func newSuccessfulVCS(src analysis.Source) *mockVCS {
	return &mockVCS{
		cloneFn: func(ctx context.Context, url string, token *string) (analysis.Source, error) {
			return src, nil
		},
	}
}

func newSuccessfulRepository() *mockRepository {
	return &mockRepository{
		createAnalysisRecordFn: func(ctx context.Context, params analysis.CreateAnalysisRecordParams) (analysis.UUID, error) {
			return analysis.NewUUID(), nil
		},
		saveAnalysisInventoryFn: func(ctx context.Context, params analysis.SaveAnalysisInventoryParams) error {
			return nil
		},
	}
}

func newSuccessfulCodebaseRepository() *mockCodebaseRepository {
	return &mockCodebaseRepository{}
}

func newSuccessfulVCSAPIClient() *mockVCSAPIClient {
	return &mockVCSAPIClient{}
}

func newSuccessfulParser() *mockParser {
	return &mockParser{
		scanFn: func(ctx context.Context, src analysis.Source) (*analysis.Inventory, error) {
			return &analysis.Inventory{Files: []analysis.TestFile{}}, nil
		},
	}
}

func newValidRequest() analysis.AnalyzeRequest {
	return analysis.AnalyzeRequest{
		Owner:     "testowner",
		Repo:      "testrepo",
		CommitSHA: "abc123",
	}
}

func TestAnalyzeUseCase_Execute(t *testing.T) {
	tests := []struct {
		name           string
		request        analysis.AnalyzeRequest
		setupMocks     func() (*mockVCS, *mockParser, *mockRepository)
		expectedErr    error
		validateResult func(t *testing.T, vcs *mockVCS, parser *mockParser, repo *mockRepository)
	}{
		{
			name:    "success case - complete workflow succeeds",
			request: newValidRequest(),
			setupMocks: func() (*mockVCS, *mockParser, *mockRepository) {
				src := newSuccessfulSource()
				src.commitSHAFn = func() string { return "abc123def456" }

				vcs := newSuccessfulVCS(src)
				repo := newSuccessfulRepository()
				parser := &mockParser{
					scanFn: func(ctx context.Context, src analysis.Source) (*analysis.Inventory, error) {
						return &analysis.Inventory{
							Files: []analysis.TestFile{{Path: "test.go", Framework: "go"}},
						}, nil
					},
				}
				return vcs, parser, repo
			},
			expectedErr:    nil,
			validateResult: func(t *testing.T, vcs *mockVCS, parser *mockParser, repo *mockRepository) {},
		},
		{
			name: "invalid input - empty owner",
			request: analysis.AnalyzeRequest{
				Owner:     "",
				Repo:      "testrepo",
				CommitSHA: "abc123",
			},
			setupMocks: func() (*mockVCS, *mockParser, *mockRepository) {
				return &mockVCS{}, &mockParser{}, &mockRepository{}
			},
			expectedErr:    analysis.ErrInvalidInput,
			validateResult: func(t *testing.T, vcs *mockVCS, parser *mockParser, repo *mockRepository) {},
		},
		{
			name: "invalid input - empty repo",
			request: analysis.AnalyzeRequest{
				Owner:     "testowner",
				Repo:      "",
				CommitSHA: "abc123",
			},
			setupMocks: func() (*mockVCS, *mockParser, *mockRepository) {
				return &mockVCS{}, &mockParser{}, &mockRepository{}
			},
			expectedErr:    analysis.ErrInvalidInput,
			validateResult: func(t *testing.T, vcs *mockVCS, parser *mockParser, repo *mockRepository) {},
		},
		{
			name:    "clone failed - VCS clone returns error",
			request: newValidRequest(),
			setupMocks: func() (*mockVCS, *mockParser, *mockRepository) {
				vcs := &mockVCS{
					cloneFn: func(ctx context.Context, url string, token *string) (analysis.Source, error) {
						return nil, errors.New("git clone failed")
					},
				}
				return vcs, &mockParser{}, &mockRepository{}
			},
			expectedErr:    ErrCloneFailed,
			validateResult: func(t *testing.T, vcs *mockVCS, parser *mockParser, repo *mockRepository) {},
		},
		{
			name:    "create record failed - repository returns error",
			request: newValidRequest(),
			setupMocks: func() (*mockVCS, *mockParser, *mockRepository) {
				src := newSuccessfulSource()
				vcs := newSuccessfulVCS(src)
				repo := &mockRepository{
					createAnalysisRecordFn: func(ctx context.Context, params analysis.CreateAnalysisRecordParams) (analysis.UUID, error) {
						return analysis.NilUUID, errors.New("database error")
					},
				}
				return vcs, &mockParser{}, repo
			},
			expectedErr:    ErrSaveFailed,
			validateResult: func(t *testing.T, vcs *mockVCS, parser *mockParser, repo *mockRepository) {},
		},
		{
			name:    "scan failed - parser returns error and RecordFailure is called",
			request: newValidRequest(),
			setupMocks: func() (*mockVCS, *mockParser, *mockRepository) {
				src := newSuccessfulSource()
				vcs := newSuccessfulVCS(src)

				testAnalysisID := analysis.NewUUID()
				recordFailureCalled := false
				repo := &mockRepository{
					createAnalysisRecordFn: func(ctx context.Context, params analysis.CreateAnalysisRecordParams) (analysis.UUID, error) {
						return testAnalysisID, nil
					},
					recordFailureFn: func(ctx context.Context, analysisID analysis.UUID, errMessage string) error {
						recordFailureCalled = true
						if analysisID != testAnalysisID {
							t.Errorf("RecordFailure called with wrong analysisID: got %v, want %v", analysisID, testAnalysisID)
						}
						return nil
					},
				}

				parser := &mockParser{
					scanFn: func(ctx context.Context, src analysis.Source) (*analysis.Inventory, error) {
						return nil, errors.New("parser error")
					},
				}

				t.Cleanup(func() {
					if !recordFailureCalled {
						t.Error("RecordFailure was not called when scan failed")
					}
				})

				return vcs, parser, repo
			},
			expectedErr:    ErrScanFailed,
			validateResult: func(t *testing.T, vcs *mockVCS, parser *mockParser, repo *mockRepository) {},
		},
		{
			name:    "save inventory failed - repository returns error and RecordFailure is called",
			request: newValidRequest(),
			setupMocks: func() (*mockVCS, *mockParser, *mockRepository) {
				src := newSuccessfulSource()
				vcs := newSuccessfulVCS(src)

				testAnalysisID := analysis.NewUUID()
				recordFailureCalled := false
				repo := &mockRepository{
					createAnalysisRecordFn: func(ctx context.Context, params analysis.CreateAnalysisRecordParams) (analysis.UUID, error) {
						return testAnalysisID, nil
					},
					recordFailureFn: func(ctx context.Context, analysisID analysis.UUID, errMessage string) error {
						recordFailureCalled = true
						if analysisID != testAnalysisID {
							t.Errorf("RecordFailure called with wrong analysisID: got %v, want %v", analysisID, testAnalysisID)
						}
						return nil
					},
					saveAnalysisInventoryFn: func(ctx context.Context, params analysis.SaveAnalysisInventoryParams) error {
						return errors.New("database save error")
					},
				}

				parser := newSuccessfulParser()

				t.Cleanup(func() {
					if !recordFailureCalled {
						t.Error("RecordFailure was not called when save inventory failed")
					}
				})

				return vcs, parser, repo
			},
			expectedErr:    ErrSaveFailed,
			validateResult: func(t *testing.T, vcs *mockVCS, parser *mockParser, repo *mockRepository) {},
		},
		{
			name:    "nil inventory warning - scan returns nil inventory but succeeds",
			request: newValidRequest(),
			setupMocks: func() (*mockVCS, *mockParser, *mockRepository) {
				src := newSuccessfulSource()
				vcs := newSuccessfulVCS(src)

				testAnalysisID := analysis.NewUUID()
				repo := &mockRepository{
					createAnalysisRecordFn: func(ctx context.Context, params analysis.CreateAnalysisRecordParams) (analysis.UUID, error) {
						return testAnalysisID, nil
					},
					saveAnalysisInventoryFn: func(ctx context.Context, params analysis.SaveAnalysisInventoryParams) error {
						if params.Inventory == nil {
							t.Error("Expected empty inventory but got nil")
						}
						if len(params.Inventory.Files) != 0 {
							t.Errorf("Expected empty files but got %d", len(params.Inventory.Files))
						}
						return nil
					},
				}

				parser := &mockParser{
					scanFn: func(ctx context.Context, src analysis.Source) (*analysis.Inventory, error) {
						return nil, nil
					},
				}

				return vcs, parser, repo
			},
			expectedErr:    nil,
			validateResult: func(t *testing.T, vcs *mockVCS, parser *mockParser, repo *mockRepository) {},
		},
		{
			name:    "scan failed and RecordFailure fails - original error returned, failure logged",
			request: newValidRequest(),
			setupMocks: func() (*mockVCS, *mockParser, *mockRepository) {
				src := newSuccessfulSource()
				vcs := newSuccessfulVCS(src)

				testAnalysisID := analysis.NewUUID()
				recordFailureCalled := false
				repo := &mockRepository{
					createAnalysisRecordFn: func(ctx context.Context, params analysis.CreateAnalysisRecordParams) (analysis.UUID, error) {
						return testAnalysisID, nil
					},
					recordFailureFn: func(ctx context.Context, analysisID analysis.UUID, errMessage string) error {
						recordFailureCalled = true
						return errors.New("database connection lost")
					},
				}

				parser := &mockParser{
					scanFn: func(ctx context.Context, src analysis.Source) (*analysis.Inventory, error) {
						return nil, errors.New("parser error")
					},
				}

				t.Cleanup(func() {
					if !recordFailureCalled {
						t.Error("RecordFailure was not called")
					}
				})

				return vcs, parser, repo
			},
			expectedErr:    ErrScanFailed,
			validateResult: func(t *testing.T, vcs *mockVCS, parser *mockParser, repo *mockRepository) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vcs, parser, repo := tt.setupMocks()
			codebaseRepo := newSuccessfulCodebaseRepository()
			vcsAPI := newSuccessfulVCSAPIClient()
			uc := NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, nil)

			err := uc.Execute(context.Background(), tt.request)

			if tt.expectedErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.expectedErr)
					return
				}
				if !errors.Is(err, tt.expectedErr) {
					t.Errorf("expected error to wrap %v, got %v", tt.expectedErr, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}

			if tt.validateResult != nil {
				tt.validateResult(t, vcs, parser, repo)
			}
		})
	}
}

func TestAnalyzeUseCase_Execute_Timeout(t *testing.T) {
	t.Run("timeout - context timeout triggers during execution", func(t *testing.T) {
		src := newSuccessfulSource()
		vcs := &mockVCS{
			cloneFn: func(ctx context.Context, url string, token *string) (analysis.Source, error) {
				select {
				case <-time.After(200 * time.Millisecond):
					return src, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			},
		}

		repo := &mockRepository{}
		codebaseRepo := newSuccessfulCodebaseRepository()
		vcsAPI := newSuccessfulVCSAPIClient()
		parser := &mockParser{}

		uc := NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, nil, WithAnalysisTimeout(50*time.Millisecond))

		err := uc.Execute(context.Background(), newValidRequest())

		if err == nil {
			t.Error("expected timeout error, got nil")
			return
		}
		if !errors.Is(err, ErrCloneFailed) {
			t.Errorf("expected ErrCloneFailed, got %v", err)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected context.DeadlineExceeded in error chain, got %v", err)
		}
	})
}

func TestAnalyzeUseCase_Options(t *testing.T) {
	tests := []struct {
		name            string
		opts            []Option
		expectedTimeout time.Duration
	}{
		{
			name:            "default timeout - no options provided",
			opts:            nil,
			expectedTimeout: DefaultAnalysisTimeout,
		},
		{
			name:            "custom timeout - WithAnalysisTimeout option",
			opts:            []Option{WithAnalysisTimeout(5 * time.Minute)},
			expectedTimeout: 5 * time.Minute,
		},
		{
			name:            "invalid timeout ignored - zero duration ignored",
			opts:            []Option{WithAnalysisTimeout(0)},
			expectedTimeout: DefaultAnalysisTimeout,
		},
		{
			name:            "invalid timeout ignored - negative duration ignored",
			opts:            []Option{WithAnalysisTimeout(-1 * time.Minute)},
			expectedTimeout: DefaultAnalysisTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{}
			codebaseRepo := newSuccessfulCodebaseRepository()
			vcs := &mockVCS{}
			vcsAPI := newSuccessfulVCSAPIClient()
			parser := &mockParser{}

			uc := NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, nil, tt.opts...)

			if uc.timeout != tt.expectedTimeout {
				t.Errorf("expected timeout %v, got %v", tt.expectedTimeout, uc.timeout)
			}
		})
	}
}

func TestAnalyzeUseCase_MaxConcurrentClones(t *testing.T) {
	t.Run("max concurrent clones - WithMaxConcurrentClones option", func(t *testing.T) {
		repo := &mockRepository{}
		codebaseRepo := newSuccessfulCodebaseRepository()
		vcs := &mockVCS{}
		vcsAPI := newSuccessfulVCSAPIClient()
		parser := &mockParser{}

		uc := NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, nil, WithMaxConcurrentClones(5))

		if uc.cloneSem == nil {
			t.Error("expected cloneSem to be initialized")
		}
	})

	t.Run("invalid max concurrent clones - zero value ignored", func(t *testing.T) {
		repo := &mockRepository{}
		codebaseRepo := newSuccessfulCodebaseRepository()
		vcs := &mockVCS{}
		vcsAPI := newSuccessfulVCSAPIClient()
		parser := &mockParser{}

		uc := NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, nil, WithMaxConcurrentClones(0))

		if uc.cloneSem == nil {
			t.Error("expected cloneSem to be initialized with default")
		}
	})

	t.Run("invalid max concurrent clones - negative value ignored", func(t *testing.T) {
		repo := &mockRepository{}
		codebaseRepo := newSuccessfulCodebaseRepository()
		vcs := &mockVCS{}
		vcsAPI := newSuccessfulVCSAPIClient()
		parser := &mockParser{}

		uc := NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, nil, WithMaxConcurrentClones(-1))

		if uc.cloneSem == nil {
			t.Error("expected cloneSem to be initialized with default")
		}
	})
}

func TestAnalyzeUseCase_SourceCleanup(t *testing.T) {
	t.Run("source cleanup - Close is called even on success", func(t *testing.T) {
		closeCalled := false
		src := newSuccessfulSource()
		src.closeFn = func(ctx context.Context) error {
			closeCalled = true
			return nil
		}

		vcs := newSuccessfulVCS(src)
		codebaseRepo := newSuccessfulCodebaseRepository()
		vcsAPI := newSuccessfulVCSAPIClient()
		repo := newSuccessfulRepository()
		parser := newSuccessfulParser()

		uc := NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, nil)

		err := uc.Execute(context.Background(), newValidRequest())

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !closeCalled {
			t.Error("expected Close to be called")
		}
	})

	t.Run("source cleanup - Close is called even on failure", func(t *testing.T) {
		closeCalled := false
		src := newSuccessfulSource()
		src.closeFn = func(ctx context.Context) error {
			closeCalled = true
			return nil
		}

		vcs := newSuccessfulVCS(src)
		codebaseRepo := newSuccessfulCodebaseRepository()
		vcsAPI := newSuccessfulVCSAPIClient()

		testAnalysisID := analysis.NewUUID()
		repo := &mockRepository{
			createAnalysisRecordFn: func(ctx context.Context, params analysis.CreateAnalysisRecordParams) (analysis.UUID, error) {
				return testAnalysisID, nil
			},
			recordFailureFn: func(ctx context.Context, analysisID analysis.UUID, errMessage string) error {
				return nil
			},
		}

		parser := &mockParser{
			scanFn: func(ctx context.Context, src analysis.Source) (*analysis.Inventory, error) {
				return nil, errors.New("scan failed")
			},
		}

		uc := NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, nil)

		err := uc.Execute(context.Background(), newValidRequest())

		if err == nil {
			t.Error("expected error but got nil")
		}
		if !closeCalled {
			t.Error("expected Close to be called even on failure")
		}
	})
}

func TestAnalyzeUseCase_TokenLookup(t *testing.T) {
	t.Run("token lookup success - token passed to VCS Clone", func(t *testing.T) {
		var capturedToken *string
		src := newSuccessfulSource()

		vcs := &mockVCS{
			cloneFn: func(ctx context.Context, url string, token *string) (analysis.Source, error) {
				capturedToken = token
				return src, nil
			},
		}
		codebaseRepo := newSuccessfulCodebaseRepository()
		vcsAPI := newSuccessfulVCSAPIClient()
		repo := newSuccessfulRepository()
		parser := newSuccessfulParser()

		expectedToken := "test-oauth-token"
		tokenLookup := &mockTokenLookup{
			getOAuthTokenFn: func(ctx context.Context, userID string, provider string) (string, error) {
				if userID != "user-123" {
					t.Errorf("expected userID 'user-123', got '%s'", userID)
				}
				if provider != DefaultOAuthProvider {
					t.Errorf("expected provider '%s', got '%s'", DefaultOAuthProvider, provider)
				}
				return expectedToken, nil
			},
		}

		uc := NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, tokenLookup)

		userID := "user-123"
		req := analysis.AnalyzeRequest{
			Owner:     "testowner",
			Repo:      "testrepo",
			CommitSHA: "abc123",
			UserID:    &userID,
		}

		err := uc.Execute(context.Background(), req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if capturedToken == nil {
			t.Error("expected token to be passed to VCS Clone, got nil")
		} else if *capturedToken != expectedToken {
			t.Errorf("expected token '%s', got '%s'", expectedToken, *capturedToken)
		}
	})

	t.Run("no userID - token is nil", func(t *testing.T) {
		var capturedToken *string
		src := newSuccessfulSource()

		vcs := &mockVCS{
			cloneFn: func(ctx context.Context, url string, token *string) (analysis.Source, error) {
				capturedToken = token
				return src, nil
			},
		}
		codebaseRepo := newSuccessfulCodebaseRepository()
		vcsAPI := newSuccessfulVCSAPIClient()
		repo := newSuccessfulRepository()
		parser := newSuccessfulParser()

		tokenLookupCalled := false
		tokenLookup := &mockTokenLookup{
			getOAuthTokenFn: func(ctx context.Context, userID string, provider string) (string, error) {
				tokenLookupCalled = true
				return "token", nil
			},
		}

		uc := NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, tokenLookup)

		req := analysis.AnalyzeRequest{
			Owner:     "testowner",
			Repo:      "testrepo",
			CommitSHA: "abc123",
			UserID:    nil,
		}

		err := uc.Execute(context.Background(), req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if tokenLookupCalled {
			t.Error("token lookup should not be called when userID is nil")
		}
		if capturedToken != nil {
			t.Error("expected token to be nil when no userID provided")
		}
	})

	t.Run("token not found - graceful degradation to public access", func(t *testing.T) {
		var capturedToken *string
		src := newSuccessfulSource()

		vcs := &mockVCS{
			cloneFn: func(ctx context.Context, url string, token *string) (analysis.Source, error) {
				capturedToken = token
				return src, nil
			},
		}
		codebaseRepo := newSuccessfulCodebaseRepository()
		vcsAPI := newSuccessfulVCSAPIClient()
		repo := newSuccessfulRepository()
		parser := newSuccessfulParser()

		tokenLookup := &mockTokenLookup{
			getOAuthTokenFn: func(ctx context.Context, userID string, provider string) (string, error) {
				return "", analysis.ErrTokenNotFound
			},
		}

		uc := NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, tokenLookup)

		userID := "user-123"
		req := analysis.AnalyzeRequest{
			Owner:     "testowner",
			Repo:      "testrepo",
			CommitSHA: "abc123",
			UserID:    &userID,
		}

		err := uc.Execute(context.Background(), req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if capturedToken != nil {
			t.Error("expected token to be nil when token not found")
		}
	})

	t.Run("token lookup infrastructure error - fails with ErrTokenLookupFailed", func(t *testing.T) {
		src := newSuccessfulSource()

		vcs := &mockVCS{
			cloneFn: func(ctx context.Context, url string, token *string) (analysis.Source, error) {
				return src, nil
			},
		}
		codebaseRepo := newSuccessfulCodebaseRepository()
		vcsAPI := newSuccessfulVCSAPIClient()
		repo := newSuccessfulRepository()
		parser := newSuccessfulParser()

		tokenLookup := &mockTokenLookup{
			getOAuthTokenFn: func(ctx context.Context, userID string, provider string) (string, error) {
				return "", errors.New("database connection failed")
			},
		}

		uc := NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, tokenLookup)

		userID := "user-123"
		req := analysis.AnalyzeRequest{
			Owner:     "testowner",
			Repo:      "testrepo",
			CommitSHA: "abc123",
			UserID:    &userID,
		}

		err := uc.Execute(context.Background(), req)

		if err == nil {
			t.Error("expected error, got nil")
		}
		if !errors.Is(err, ErrTokenLookupFailed) {
			t.Errorf("expected ErrTokenLookupFailed, got %v", err)
		}
	})

	t.Run("empty token returned - graceful degradation to public access", func(t *testing.T) {
		var capturedToken *string
		src := newSuccessfulSource()

		vcs := &mockVCS{
			cloneFn: func(ctx context.Context, url string, token *string) (analysis.Source, error) {
				capturedToken = token
				return src, nil
			},
		}
		codebaseRepo := newSuccessfulCodebaseRepository()
		vcsAPI := newSuccessfulVCSAPIClient()
		repo := newSuccessfulRepository()
		parser := newSuccessfulParser()

		tokenLookup := &mockTokenLookup{
			getOAuthTokenFn: func(ctx context.Context, userID string, provider string) (string, error) {
				return "", nil // empty token with no error
			},
		}

		uc := NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, tokenLookup)

		userID := "user-123"
		req := analysis.AnalyzeRequest{
			Owner:     "testowner",
			Repo:      "testrepo",
			CommitSHA: "abc123",
			UserID:    &userID,
		}

		err := uc.Execute(context.Background(), req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if capturedToken != nil {
			t.Error("expected token to be nil when empty token returned")
		}
	})

	t.Run("nil tokenLookup - proceeds without token", func(t *testing.T) {
		var capturedToken *string
		src := newSuccessfulSource()

		vcs := &mockVCS{
			cloneFn: func(ctx context.Context, url string, token *string) (analysis.Source, error) {
				capturedToken = token
				return src, nil
			},
		}
		codebaseRepo := newSuccessfulCodebaseRepository()
		vcsAPI := newSuccessfulVCSAPIClient()
		repo := newSuccessfulRepository()
		parser := newSuccessfulParser()

		uc := NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, nil)

		userID := "user-123"
		req := analysis.AnalyzeRequest{
			Owner:     "testowner",
			Repo:      "testrepo",
			CommitSHA: "abc123",
			UserID:    &userID,
		}

		err := uc.Execute(context.Background(), req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if capturedToken != nil {
			t.Error("expected token to be nil when tokenLookup is nil")
		}
	})
}

func TestResolveCodebase(t *testing.T) {
	t.Run("Case A: new analysis - no codebase exists", func(t *testing.T) {
		src := newSuccessfulSource()
		vcs := newSuccessfulVCS(src)
		repo := newSuccessfulRepository()
		parser := newSuccessfulParser()

		upsertCalled := false
		codebaseRepo := &mockCodebaseRepository{
			findWithLastCommitFn: func(ctx context.Context, host, owner, name string) (*analysis.Codebase, error) {
				return nil, analysis.ErrCodebaseNotFound
			},
			findByExternalIDFn: func(ctx context.Context, host, externalRepoID string) (*analysis.Codebase, error) {
				return nil, analysis.ErrCodebaseNotFound
			},
			upsertFn: func(ctx context.Context, params analysis.UpsertCodebaseParams) (*analysis.Codebase, error) {
				upsertCalled = true
				if params.ExternalRepoID != "123456" {
					t.Errorf("expected externalRepoID '123456', got '%s'", params.ExternalRepoID)
				}
				return &analysis.Codebase{
					ID:             analysis.NewUUID(),
					Host:           params.Host,
					Owner:          params.Owner,
					Name:           params.Name,
					ExternalRepoID: params.ExternalRepoID,
				}, nil
			},
		}
		vcsAPI := newSuccessfulVCSAPIClient()

		uc := NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, nil)
		err := uc.Execute(context.Background(), newValidRequest())

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !upsertCalled {
			t.Error("Upsert should be called for new codebase")
		}
	})

	t.Run("Case B: reanalysis - git fetch verifies same repo", func(t *testing.T) {
		src := &mockSource{
			branchFn:    func() string { return "main" },
			commitSHAFn: func() string { return "abc123" },
			closeFn:     func(ctx context.Context) error { return nil },
			verifyCommitExistsFn: func(ctx context.Context, sha string) (bool, error) {
				if sha != "prev-commit-sha" {
					t.Errorf("expected sha 'prev-commit-sha', got '%s'", sha)
				}
				return true, nil
			},
		}
		vcs := newSuccessfulVCS(src)
		repo := newSuccessfulRepository()
		parser := newSuccessfulParser()

		existingID := analysis.NewUUID()
		codebaseRepo := &mockCodebaseRepository{
			findWithLastCommitFn: func(ctx context.Context, host, owner, name string) (*analysis.Codebase, error) {
				return &analysis.Codebase{
					ID:             existingID,
					Host:           host,
					Owner:          owner,
					Name:           name,
					ExternalRepoID: "123456",
					LastCommitSHA:  "prev-commit-sha",
				}, nil
			},
		}

		apiCalled := false
		vcsAPI := &mockVCSAPIClient{
			getRepoIDFn: func(ctx context.Context, host, owner, repo string, token *string) (string, error) {
				apiCalled = true
				return "123456", nil
			},
		}

		uc := NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, nil)
		err := uc.Execute(context.Background(), newValidRequest())

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if apiCalled {
			t.Error("API should not be called when git fetch succeeds")
		}
	})

	t.Run("Case D: rename/transfer - different owner/name, same external_repo_id", func(t *testing.T) {
		src := newSuccessfulSource()
		vcs := newSuccessfulVCS(src)
		repo := newSuccessfulRepository()
		parser := newSuccessfulParser()

		existingID := analysis.NewUUID()
		updateCalled := false
		codebaseRepo := &mockCodebaseRepository{
			findWithLastCommitFn: func(ctx context.Context, host, owner, name string) (*analysis.Codebase, error) {
				return nil, analysis.ErrCodebaseNotFound
			},
			findByExternalIDFn: func(ctx context.Context, host, externalRepoID string) (*analysis.Codebase, error) {
				return &analysis.Codebase{
					ID:             existingID,
					Host:           host,
					Owner:          "oldowner",
					Name:           "oldrepo",
					ExternalRepoID: "123456",
				}, nil
			},
			updateOwnerNameFn: func(ctx context.Context, id analysis.UUID, owner, name string) (*analysis.Codebase, error) {
				updateCalled = true
				if owner != "testowner" || name != "testrepo" {
					t.Errorf("expected owner/name 'testowner/testrepo', got '%s/%s'", owner, name)
				}
				return &analysis.Codebase{
					ID:             id,
					Host:           "github.com",
					Owner:          owner,
					Name:           name,
					ExternalRepoID: "123456",
				}, nil
			},
		}
		vcsAPI := newSuccessfulVCSAPIClient()

		uc := NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, nil)
		err := uc.Execute(context.Background(), newValidRequest())

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !updateCalled {
			t.Error("UpdateOwnerName should be called for rename/transfer")
		}
	})

	t.Run("Case E: delete and recreate - same owner/name, different external_repo_id", func(t *testing.T) {
		src := &mockSource{
			branchFn:    func() string { return "main" },
			commitSHAFn: func() string { return "abc123" },
			closeFn:     func(ctx context.Context) error { return nil },
			verifyCommitExistsFn: func(ctx context.Context, sha string) (bool, error) {
				return false, nil
			},
		}
		vcs := newSuccessfulVCS(src)
		repo := newSuccessfulRepository()
		parser := newSuccessfulParser()

		oldCodebaseID := analysis.NewUUID()
		markStaleCalled := false
		upsertCalled := false

		codebaseRepo := &mockCodebaseRepository{
			findWithLastCommitFn: func(ctx context.Context, host, owner, name string) (*analysis.Codebase, error) {
				return &analysis.Codebase{
					ID:             oldCodebaseID,
					Host:           host,
					Owner:          owner,
					Name:           name,
					ExternalRepoID: "old-external-id",
					LastCommitSHA:  "old-commit",
				}, nil
			},
			findByExternalIDFn: func(ctx context.Context, host, externalRepoID string) (*analysis.Codebase, error) {
				return nil, analysis.ErrCodebaseNotFound
			},
			markStaleFn: func(ctx context.Context, id analysis.UUID) error {
				markStaleCalled = true
				if id != oldCodebaseID {
					t.Errorf("expected to mark stale ID %v, got %v", oldCodebaseID, id)
				}
				return nil
			},
			upsertFn: func(ctx context.Context, params analysis.UpsertCodebaseParams) (*analysis.Codebase, error) {
				upsertCalled = true
				return &analysis.Codebase{
					ID:             analysis.NewUUID(),
					Host:           params.Host,
					Owner:          params.Owner,
					Name:           params.Name,
					ExternalRepoID: params.ExternalRepoID,
				}, nil
			},
		}

		vcsAPI := &mockVCSAPIClient{
			getRepoIDFn: func(ctx context.Context, host, owner, repo string, token *string) (string, error) {
				return "new-external-id", nil
			},
		}

		uc := NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, nil)
		err := uc.Execute(context.Background(), newValidRequest())

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !markStaleCalled {
			t.Error("MarkStale should be called for delete/recreate case")
		}
		if !upsertCalled {
			t.Error("Upsert should be called to create new codebase")
		}
	})

	t.Run("Case F: force push - git fetch fails but same external_repo_id", func(t *testing.T) {
		src := &mockSource{
			branchFn:    func() string { return "main" },
			commitSHAFn: func() string { return "abc123" },
			closeFn:     func(ctx context.Context) error { return nil },
			verifyCommitExistsFn: func(ctx context.Context, sha string) (bool, error) {
				return false, nil
			},
		}
		vcs := newSuccessfulVCS(src)
		repo := newSuccessfulRepository()
		parser := newSuccessfulParser()

		existingID := analysis.NewUUID()
		codebaseRepo := &mockCodebaseRepository{
			findWithLastCommitFn: func(ctx context.Context, host, owner, name string) (*analysis.Codebase, error) {
				return &analysis.Codebase{
					ID:             existingID,
					Host:           host,
					Owner:          owner,
					Name:           name,
					ExternalRepoID: "123456",
					LastCommitSHA:  "old-commit",
				}, nil
			},
			findByExternalIDFn: func(ctx context.Context, host, externalRepoID string) (*analysis.Codebase, error) {
				return &analysis.Codebase{
					ID:             existingID,
					Host:           host,
					Owner:          "testowner",
					Name:           "testrepo",
					ExternalRepoID: "123456",
				}, nil
			},
		}
		vcsAPI := newSuccessfulVCSAPIClient()

		uc := NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, nil)
		err := uc.Execute(context.Background(), newValidRequest())

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("API failure - analysis fails", func(t *testing.T) {
		src := newSuccessfulSource()
		vcs := newSuccessfulVCS(src)
		repo := newSuccessfulRepository()
		parser := newSuccessfulParser()

		codebaseRepo := &mockCodebaseRepository{
			findWithLastCommitFn: func(ctx context.Context, host, owner, name string) (*analysis.Codebase, error) {
				return nil, analysis.ErrCodebaseNotFound
			},
		}

		vcsAPI := &mockVCSAPIClient{
			getRepoIDFn: func(ctx context.Context, host, owner, repo string, token *string) (string, error) {
				return "", errors.New("API rate limit exceeded")
			},
		}

		uc := NewAnalyzeUseCase(repo, codebaseRepo, vcs, vcsAPI, parser, nil)
		err := uc.Execute(context.Background(), newValidRequest())

		if err == nil {
			t.Error("expected error when API fails")
		}
		if !errors.Is(err, ErrCodebaseResolutionFailed) {
			t.Errorf("expected ErrCodebaseResolutionFailed, got %v", err)
		}
	})
}
