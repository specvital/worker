package specview

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/specvital/worker/internal/domain/specview"
	uc "github.com/specvital/worker/internal/usecase/specview"
)

type mockAIProvider struct {
	classifyDomainsFn   func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, error)
	convertTestNamesFn  func(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, error)
}

func (m *mockAIProvider) ClassifyDomains(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, error) {
	if m.classifyDomainsFn != nil {
		return m.classifyDomainsFn(ctx, input)
	}
	return &specview.Phase1Output{
		Domains: []specview.DomainGroup{
			{
				Name:       "Test Domain",
				Confidence: 0.9,
				Features: []specview.FeatureGroup{
					{
						Name:        "Test Feature",
						Confidence:  0.85,
						TestIndices: []int{0},
					},
				},
			},
		},
	}, nil
}

func (m *mockAIProvider) ConvertTestNames(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, error) {
	if m.convertTestNamesFn != nil {
		return m.convertTestNamesFn(ctx, input)
	}
	return &specview.Phase2Output{
		Behaviors: []specview.BehaviorSpec{
			{TestIndex: 0, Description: "should do something", Confidence: 0.9},
		},
	}, nil
}

type mockRepository struct {
	findDocumentByContentHashFn func(ctx context.Context, contentHash []byte, language specview.Language, modelID string) (*specview.SpecDocument, error)
	getTestDataByAnalysisIDFn   func(ctx context.Context, analysisID string) ([]specview.FileInfo, error)
	saveDocumentFn              func(ctx context.Context, doc *specview.SpecDocument) error
}

func (m *mockRepository) FindDocumentByContentHash(ctx context.Context, contentHash []byte, language specview.Language, modelID string) (*specview.SpecDocument, error) {
	if m.findDocumentByContentHashFn != nil {
		return m.findDocumentByContentHashFn(ctx, contentHash, language, modelID)
	}
	return nil, nil
}

func (m *mockRepository) GetTestDataByAnalysisID(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
	if m.getTestDataByAnalysisIDFn != nil {
		return m.getTestDataByAnalysisIDFn(ctx, analysisID)
	}
	return []specview.FileInfo{
		{
			Path:      "test.go",
			Framework: "testing",
			Tests: []specview.TestInfo{
				{Index: 0, Name: "TestSomething", TestCaseID: "test-case-1"},
			},
		},
	}, nil
}

func (m *mockRepository) SaveDocument(ctx context.Context, doc *specview.SpecDocument) error {
	if m.saveDocumentFn != nil {
		return m.saveDocumentFn(ctx, doc)
	}
	doc.ID = "generated-doc-id"
	return nil
}

func newTestJob(args Args) *river.Job[Args] {
	return &river.Job[Args]{
		JobRow: &rivertype.JobRow{
			ID:      1,
			Attempt: 1,
		},
		Args: args,
	}
}

func newSuccessfulMocks() (*mockRepository, *mockAIProvider) {
	repo := &mockRepository{}
	ai := &mockAIProvider{}
	return repo, ai
}

func TestNewWorker(t *testing.T) {
	repo, ai := newSuccessfulMocks()
	usecase := uc.NewGenerateSpecViewUseCase(repo, ai, "test-model")
	worker := NewWorker(usecase)

	if worker == nil {
		t.Error("expected worker, got nil")
	}
	if worker.usecase == nil {
		t.Error("expected worker.usecase to be set, got nil")
	}
}

func TestArgs_Kind(t *testing.T) {
	args := Args{}
	if args.Kind() != "specview:generate" {
		t.Errorf("expected kind 'specview:generate', got '%s'", args.Kind())
	}
}

func TestArgs_InsertOpts(t *testing.T) {
	args := Args{}
	opts := args.InsertOpts()

	if opts.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts 3, got %d", opts.MaxAttempts)
	}
	if !opts.UniqueOpts.ByArgs {
		t.Error("expected UniqueOpts.ByArgs to be true")
	}
}

func TestWorker_Timeout(t *testing.T) {
	repo, ai := newSuccessfulMocks()
	usecase := uc.NewGenerateSpecViewUseCase(repo, ai, "test-model")
	worker := NewWorker(usecase)

	job := newTestJob(Args{AnalysisID: "test-id", Language: "en"})
	timeout := worker.Timeout(job)

	if timeout != 10*time.Minute {
		t.Errorf("expected timeout 10 minutes, got %v", timeout)
	}
}

func TestWorker_NextRetry(t *testing.T) {
	tests := []struct {
		name            string
		attempt         int
		expectedBackoff time.Duration
	}{
		{"first attempt", 1, 10 * time.Second},
		{"second attempt", 2, 40 * time.Second},
		{"third attempt", 3, 90 * time.Second},
	}

	repo, ai := newSuccessfulMocks()
	usecase := uc.NewGenerateSpecViewUseCase(repo, ai, "test-model")
	worker := NewWorker(usecase)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &river.Job[Args]{
				JobRow: &rivertype.JobRow{
					ID:      1,
					Attempt: tt.attempt,
				},
				Args: Args{AnalysisID: "test-id", Language: "en"},
			}

			before := time.Now()
			nextRetry := worker.NextRetry(job)

			expectedTime := before.Add(tt.expectedBackoff)
			diff := nextRetry.Sub(expectedTime)
			if diff < -100*time.Millisecond || diff > 100*time.Millisecond {
				t.Errorf("expected retry around %v from now, got %v", tt.expectedBackoff, nextRetry.Sub(before))
			}
		})
	}
}

func TestWorker_Work(t *testing.T) {
	tests := []struct {
		name       string
		args       Args
		setupMocks func() (*mockRepository, *mockAIProvider)
		wantErr    bool
		wantCancel bool
	}{
		{
			name: "success case",
			args: Args{
				AnalysisID: "valid-analysis-id",
				Language:   "en",
			},
			setupMocks: func() (*mockRepository, *mockAIProvider) {
				return newSuccessfulMocks()
			},
			wantErr: false,
		},
		{
			name: "success with cache hit",
			args: Args{
				AnalysisID: "cached-analysis-id",
				Language:   "ko",
			},
			setupMocks: func() (*mockRepository, *mockAIProvider) {
				repo, ai := newSuccessfulMocks()
				repo.findDocumentByContentHashFn = func(ctx context.Context, contentHash []byte, language specview.Language, modelID string) (*specview.SpecDocument, error) {
					return &specview.SpecDocument{ID: "cached-doc-id"}, nil
				}
				return repo, ai
			},
			wantErr: false,
		},
		{
			name: "invalid args - empty analysis ID",
			args: Args{
				AnalysisID: "",
				Language:   "en",
			},
			setupMocks: func() (*mockRepository, *mockAIProvider) {
				return newSuccessfulMocks()
			},
			wantErr:    true,
			wantCancel: true,
		},
		{
			name: "empty language defaults to English",
			args: Args{
				AnalysisID: "valid-id",
				Language:   "",
			},
			setupMocks: func() (*mockRepository, *mockAIProvider) {
				return newSuccessfulMocks()
			},
			wantErr:    false,
			wantCancel: false,
		},
		{
			name: "analysis not found - permanent error",
			args: Args{
				AnalysisID: "nonexistent-id",
				Language:   "en",
			},
			setupMocks: func() (*mockRepository, *mockAIProvider) {
				repo, ai := newSuccessfulMocks()
				repo.getTestDataByAnalysisIDFn = func(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
					return nil, specview.ErrAnalysisNotFound
				}
				return repo, ai
			},
			wantErr:    true,
			wantCancel: true,
		},
		{
			name: "AI processing failed - transient error",
			args: Args{
				AnalysisID: "valid-id",
				Language:   "en",
			},
			setupMocks: func() (*mockRepository, *mockAIProvider) {
				repo, ai := newSuccessfulMocks()
				ai.classifyDomainsFn = func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, error) {
					return nil, errors.New("AI service temporarily unavailable")
				}
				return repo, ai
			},
			wantErr:    true,
			wantCancel: false,
		},
		{
			name: "save document failed - transient error",
			args: Args{
				AnalysisID: "valid-id",
				Language:   "en",
			},
			setupMocks: func() (*mockRepository, *mockAIProvider) {
				repo, ai := newSuccessfulMocks()
				repo.saveDocumentFn = func(ctx context.Context, doc *specview.SpecDocument) error {
					return errors.New("database connection lost")
				}
				return repo, ai
			},
			wantErr:    true,
			wantCancel: false,
		},
		{
			name: "success with custom model ID",
			args: Args{
				AnalysisID: "valid-id",
				Language:   "ja",
				ModelID:    "custom-model",
			},
			setupMocks: func() (*mockRepository, *mockAIProvider) {
				return newSuccessfulMocks()
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, ai := tt.setupMocks()
			usecase := uc.NewGenerateSpecViewUseCase(repo, ai, "test-model")
			worker := NewWorker(usecase)

			job := newTestJob(tt.args)
			err := worker.Work(context.Background(), job)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}

				var cancelErr *rivertype.JobCancelError
				isCancelled := errors.As(err, &cancelErr)
				if tt.wantCancel && !isCancelled {
					t.Errorf("expected JobCancelError, got %T: %v", err, err)
				}
				if !tt.wantCancel && isCancelled {
					t.Errorf("expected retryable error, got JobCancelError: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestWorker_Work_ContextPropagation(t *testing.T) {
	t.Run("should propagate context to use case", func(t *testing.T) {
		type ctxKey string
		testKey := ctxKey("test-key")
		testValue := "test-value"

		var capturedCtx context.Context
		repo, ai := newSuccessfulMocks()
		repo.getTestDataByAnalysisIDFn = func(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
			capturedCtx = ctx
			return []specview.FileInfo{
				{Path: "test.go", Tests: []specview.TestInfo{{Index: 0, Name: "TestX"}}},
			}, nil
		}

		usecase := uc.NewGenerateSpecViewUseCase(repo, ai, "test-model")
		worker := NewWorker(usecase)

		job := newTestJob(Args{AnalysisID: "test-id", Language: "en"})
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
		repo, ai := newSuccessfulMocks()
		repo.getTestDataByAnalysisIDFn = func(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
			return nil, ctx.Err()
		}

		usecase := uc.NewGenerateSpecViewUseCase(repo, ai, "test-model")
		worker := NewWorker(usecase)

		job := newTestJob(Args{AnalysisID: "test-id", Language: "en"})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := worker.Work(ctx, job)
		if err == nil {
			t.Error("expected error from cancelled context, got nil")
		}
	})
}

func TestIsPermanentError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		permanent bool
	}{
		{
			name:      "analysis not found",
			err:       specview.ErrAnalysisNotFound,
			permanent: true,
		},
		{
			name:      "invalid input",
			err:       specview.ErrInvalidInput,
			permanent: true,
		},
		{
			name:      "wrapped analysis not found",
			err:       errors.New("wrapped: " + specview.ErrAnalysisNotFound.Error()),
			permanent: false,
		},
		{
			name:      "generic error",
			err:       errors.New("some error"),
			permanent: false,
		},
		{
			name:      "AI unavailable",
			err:       specview.ErrAIUnavailable,
			permanent: false,
		},
		{
			name:      "rate limited",
			err:       specview.ErrRateLimited,
			permanent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPermanentError(tt.err)
			if result != tt.permanent {
				t.Errorf("expected isPermanentError(%v) = %v, got %v", tt.err, tt.permanent, result)
			}
		})
	}
}
