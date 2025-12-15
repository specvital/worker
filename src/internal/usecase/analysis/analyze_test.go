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
	cloneFn func(ctx context.Context, url string) (analysis.Source, error)
}

func (m *mockVCS) Clone(ctx context.Context, url string) (analysis.Source, error) {
	if m.cloneFn != nil {
		return m.cloneFn(ctx, url)
	}
	return nil, nil
}

type mockSource struct {
	branchFn    func() string
	commitSHAFn func() string
	closeFn     func(ctx context.Context) error
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
		cloneFn: func(ctx context.Context, url string) (analysis.Source, error) {
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

func newSuccessfulParser() *mockParser {
	return &mockParser{
		scanFn: func(ctx context.Context, src analysis.Source) (*analysis.Inventory, error) {
			return &analysis.Inventory{Files: []analysis.TestFile{}}, nil
		},
	}
}

func newValidRequest() analysis.AnalyzeRequest {
	return analysis.AnalyzeRequest{
		Owner: "testowner",
		Repo:  "testrepo",
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
				Owner: "",
				Repo:  "testrepo",
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
				Owner: "testowner",
				Repo:  "",
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
					cloneFn: func(ctx context.Context, url string) (analysis.Source, error) {
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
			uc := NewAnalyzeUseCase(repo, vcs, parser)

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
			cloneFn: func(ctx context.Context, url string) (analysis.Source, error) {
				select {
				case <-time.After(200 * time.Millisecond):
					return src, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			},
		}

		repo := &mockRepository{}
		parser := &mockParser{}

		uc := NewAnalyzeUseCase(repo, vcs, parser, WithAnalysisTimeout(50*time.Millisecond))

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
			vcs := &mockVCS{}
			parser := &mockParser{}

			uc := NewAnalyzeUseCase(repo, vcs, parser, tt.opts...)

			if uc.timeout != tt.expectedTimeout {
				t.Errorf("expected timeout %v, got %v", tt.expectedTimeout, uc.timeout)
			}
		})
	}
}

func TestAnalyzeUseCase_MaxConcurrentClones(t *testing.T) {
	t.Run("max concurrent clones - WithMaxConcurrentClones option", func(t *testing.T) {
		repo := &mockRepository{}
		vcs := &mockVCS{}
		parser := &mockParser{}

		uc := NewAnalyzeUseCase(repo, vcs, parser, WithMaxConcurrentClones(5))

		if uc.cloneSem == nil {
			t.Error("expected cloneSem to be initialized")
		}
	})

	t.Run("invalid max concurrent clones - zero value ignored", func(t *testing.T) {
		repo := &mockRepository{}
		vcs := &mockVCS{}
		parser := &mockParser{}

		uc := NewAnalyzeUseCase(repo, vcs, parser, WithMaxConcurrentClones(0))

		if uc.cloneSem == nil {
			t.Error("expected cloneSem to be initialized with default")
		}
	})

	t.Run("invalid max concurrent clones - negative value ignored", func(t *testing.T) {
		repo := &mockRepository{}
		vcs := &mockVCS{}
		parser := &mockParser{}

		uc := NewAnalyzeUseCase(repo, vcs, parser, WithMaxConcurrentClones(-1))

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
		repo := newSuccessfulRepository()
		parser := newSuccessfulParser()

		uc := NewAnalyzeUseCase(repo, vcs, parser)

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

		uc := NewAnalyzeUseCase(repo, vcs, parser)

		err := uc.Execute(context.Background(), newValidRequest())

		if err == nil {
			t.Error("expected error but got nil")
		}
		if !closeCalled {
			t.Error("expected Close to be called even on failure")
		}
	})
}
