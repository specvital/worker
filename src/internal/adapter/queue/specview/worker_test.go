package specview

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"google.golang.org/genai"

	"github.com/specvital/worker/internal/adapter/ai/gemini/batch"
	"github.com/specvital/worker/internal/domain/specview"
	uc "github.com/specvital/worker/internal/usecase/specview"
)

type mockAIProvider struct {
	classifyDomainsFn  func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error)
	convertTestNamesFn func(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, *specview.TokenUsage, error)
	generateSummaryFn  func(ctx context.Context, input specview.Phase3Input) (*specview.Phase3Output, *specview.TokenUsage, error)
	placeNewTestsFn    func(ctx context.Context, input specview.PlacementInput) (*specview.PlacementOutput, *specview.TokenUsage, error)
}

func (m *mockAIProvider) ClassifyDomains(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
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
	}, nil, nil
}

func (m *mockAIProvider) ConvertTestNames(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, *specview.TokenUsage, error) {
	if m.convertTestNamesFn != nil {
		return m.convertTestNamesFn(ctx, input)
	}
	return &specview.Phase2Output{
		Behaviors: []specview.BehaviorSpec{
			{TestIndex: 0, Description: "should do something", Confidence: 0.9},
		},
	}, nil, nil
}

func (m *mockAIProvider) GenerateSummary(ctx context.Context, input specview.Phase3Input) (*specview.Phase3Output, *specview.TokenUsage, error) {
	if m.generateSummaryFn != nil {
		return m.generateSummaryFn(ctx, input)
	}
	return &specview.Phase3Output{Summary: "mock summary"}, nil, nil
}

func (m *mockAIProvider) PlaceNewTests(ctx context.Context, input specview.PlacementInput) (*specview.PlacementOutput, *specview.TokenUsage, error) {
	if m.placeNewTestsFn != nil {
		return m.placeNewTestsFn(ctx, input)
	}
	return nil, nil, nil
}

func (m *mockAIProvider) Close() error {
	return nil
}

type mockRepository struct {
	findCachedBehaviorsFn       func(ctx context.Context, cacheKeyHashes [][]byte) (map[string]string, error)
	findClassificationCacheFn   func(ctx context.Context, fileSignature []byte, language specview.Language, modelID string) (*specview.ClassificationCache, error)
	findDocumentByContentHashFn func(ctx context.Context, userID string, contentHash []byte, language specview.Language, modelID string) (*specview.SpecDocument, error)
	getAnalysisContextFn        func(ctx context.Context, analysisID string) (*specview.AnalysisContext, error)
	getTestDataByAnalysisIDFn   func(ctx context.Context, analysisID string) ([]specview.FileInfo, error)
	recordUsageEventFn          func(ctx context.Context, userID string, documentID string, quotaAmount int) error
	recordUserHistoryFn         func(ctx context.Context, userID string, documentID string) error
	saveBehaviorCacheFn         func(ctx context.Context, entries []specview.BehaviorCacheEntry) error
	saveClassificationCacheFn   func(ctx context.Context, cache *specview.ClassificationCache) error
	saveDocumentFn              func(ctx context.Context, doc *specview.SpecDocument) error
}

func (m *mockRepository) FindCachedBehaviors(ctx context.Context, cacheKeyHashes [][]byte) (map[string]string, error) {
	if m.findCachedBehaviorsFn != nil {
		return m.findCachedBehaviorsFn(ctx, cacheKeyHashes)
	}
	return nil, nil
}

func (m *mockRepository) FindDocumentByContentHash(ctx context.Context, userID string, contentHash []byte, language specview.Language, modelID string) (*specview.SpecDocument, error) {
	if m.findDocumentByContentHashFn != nil {
		return m.findDocumentByContentHashFn(ctx, userID, contentHash, language, modelID)
	}
	return nil, nil
}

func (m *mockRepository) GetAnalysisContext(ctx context.Context, analysisID string) (*specview.AnalysisContext, error) {
	if m.getAnalysisContextFn != nil {
		return m.getAnalysisContextFn(ctx, analysisID)
	}
	return &specview.AnalysisContext{Host: "github.com", Owner: "test-owner", Repo: "test-repo"}, nil
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

func (m *mockRepository) RecordUsageEvent(ctx context.Context, userID string, documentID string, quotaAmount int) error {
	if m.recordUsageEventFn != nil {
		return m.recordUsageEventFn(ctx, userID, documentID, quotaAmount)
	}
	return nil
}

func (m *mockRepository) RecordUserHistory(ctx context.Context, userID string, documentID string) error {
	if m.recordUserHistoryFn != nil {
		return m.recordUserHistoryFn(ctx, userID, documentID)
	}
	return nil
}

func (m *mockRepository) SaveBehaviorCache(ctx context.Context, entries []specview.BehaviorCacheEntry) error {
	if m.saveBehaviorCacheFn != nil {
		return m.saveBehaviorCacheFn(ctx, entries)
	}
	return nil
}

func (m *mockRepository) SaveDocument(ctx context.Context, doc *specview.SpecDocument) error {
	if m.saveDocumentFn != nil {
		return m.saveDocumentFn(ctx, doc)
	}
	doc.ID = "generated-doc-id"
	return nil
}

func (m *mockRepository) FindClassificationCache(ctx context.Context, fileSignature []byte, language specview.Language, modelID string) (*specview.ClassificationCache, error) {
	if m.findClassificationCacheFn != nil {
		return m.findClassificationCacheFn(ctx, fileSignature, language, modelID)
	}
	return nil, nil
}

func (m *mockRepository) SaveClassificationCache(ctx context.Context, cache *specview.ClassificationCache) error {
	if m.saveClassificationCacheFn != nil {
		return m.saveClassificationCacheFn(ctx, cache)
	}
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

// newTestJobWithBatchMetadata creates a test job with batch state in metadata (not Args).
// This reflects the actual behavior where batch state is stored in River's job.Metadata.
func newTestJobWithBatchMetadata(args Args, jobName string, started time.Time) *river.Job[Args] {
	metadata := []byte(`{"sv_batch_job_name":"` + jobName + `","sv_batch_phase":"poll","sv_batch_started":"` + started.Format(time.RFC3339) + `"}`)
	return &river.Job[Args]{
		JobRow: &rivertype.JobRow{
			ID:       1,
			Attempt:  1,
			Metadata: metadata,
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

	if timeout != 35*time.Minute {
		t.Errorf("expected timeout 35 minutes, got %v", timeout)
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
				UserID:     "test-user-001",
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
				UserID:     "test-user-001",
			},
			setupMocks: func() (*mockRepository, *mockAIProvider) {
				repo, ai := newSuccessfulMocks()
				repo.findDocumentByContentHashFn = func(ctx context.Context, userID string, contentHash []byte, language specview.Language, modelID string) (*specview.SpecDocument, error) {
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
				UserID:     "test-user-001",
			},
			setupMocks: func() (*mockRepository, *mockAIProvider) {
				return newSuccessfulMocks()
			},
			wantErr:    true,
			wantCancel: true,
		},
		{
			name: "invalid args - empty user ID",
			args: Args{
				AnalysisID: "valid-analysis-id",
				Language:   "en",
				UserID:     "",
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
				UserID:     "test-user-001",
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
				UserID:     "test-user-001",
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
				UserID:     "test-user-001",
			},
			setupMocks: func() (*mockRepository, *mockAIProvider) {
				repo, ai := newSuccessfulMocks()
				ai.classifyDomainsFn = func(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
					return nil, nil, errors.New("AI service temporarily unavailable")
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
				UserID:     "test-user-001",
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
				UserID:     "test-user-001",
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

		job := newTestJob(Args{AnalysisID: "test-id", Language: "en", UserID: "test-user-001"})
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

		job := newTestJob(Args{AnalysisID: "test-id", Language: "en", UserID: "test-user-001"})
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

// mockBatchProvider implements BatchProvider for testing.
type mockBatchProvider struct {
	createClassificationJobFn func(input specview.Phase1Input) (batch.BatchRequest, error)
	createJobFn               func(ctx context.Context, req batch.BatchRequest) (*batch.BatchResult, error)
	getJobStatusFn            func(ctx context.Context, jobName string) (*batch.BatchResult, error)
}

func (m *mockBatchProvider) CreateClassificationJob(input specview.Phase1Input) (batch.BatchRequest, error) {
	if m.createClassificationJobFn != nil {
		return m.createClassificationJobFn(input)
	}
	return batch.BatchRequest{
		AnalysisID: input.AnalysisID,
		Model:      "test-model",
	}, nil
}

func (m *mockBatchProvider) CreateJob(ctx context.Context, req batch.BatchRequest) (*batch.BatchResult, error) {
	if m.createJobFn != nil {
		return m.createJobFn(ctx, req)
	}
	return &batch.BatchResult{
		JobName: "batch-job-123",
		State:   batch.JobStatePending,
	}, nil
}

func (m *mockBatchProvider) GetJobStatus(ctx context.Context, jobName string) (*batch.BatchResult, error) {
	if m.getJobStatusFn != nil {
		return m.getJobStatusFn(ctx, jobName)
	}
	return &batch.BatchResult{
		JobName: jobName,
		State:   batch.JobStateRunning,
	}, nil
}

// mockMetadataUpdater implements MetadataUpdater for testing.
type mockMetadataUpdater struct {
	updateFn func(ctx context.Context, jobID int64, jobName string, started time.Time) error
}

func (m *mockMetadataUpdater) UpdateBatchMetadata(ctx context.Context, jobID int64, jobName string, started time.Time) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, jobID, jobName, started)
	}
	return nil
}

func newBatchTestWorker(
	repo *mockRepository,
	ai *mockAIProvider,
	batchProvider *mockBatchProvider,
	config WorkerConfig,
) *Worker {
	usecase := uc.NewGenerateSpecViewUseCase(repo, ai, "test-model")
	return &Worker{
		batchProvider:   batchProvider,
		config:          config,
		metadataUpdater: &mockMetadataUpdater{},
		repository:      repo,
		usecase:         usecase,
	}
}

func TestNewWorkerWithBatch(t *testing.T) {
	repo, ai := newSuccessfulMocks()
	batchProvider := &mockBatchProvider{}
	config := WorkerConfig{
		UseBatchAPI:       true,
		BatchPollInterval: 30 * time.Second,
	}

	worker := newBatchTestWorker(repo, ai, batchProvider, config)

	if worker == nil {
		t.Fatal("expected worker, got nil")
	}
	if worker.batchProvider == nil {
		t.Error("expected batchProvider to be set")
	}
	if !worker.config.UseBatchAPI {
		t.Error("expected UseBatchAPI to be true")
	}
}

func TestWorker_IsBatchMode(t *testing.T) {
	tests := []struct {
		name               string
		metadata           []byte
		config             WorkerConfig
		hasBatchProv       bool
		hasMetadataUpdater bool
		expectedBatch      bool
	}{
		{
			name:               "poll phase in metadata always returns true",
			metadata:           []byte(`{"sv_batch_phase":"poll"}`),
			config:             WorkerConfig{UseBatchAPI: false},
			hasBatchProv:       false,
			hasMetadataUpdater: false,
			expectedBatch:      true,
		},
		{
			name:               "batch enabled with provider and updater",
			metadata:           nil,
			config:             WorkerConfig{UseBatchAPI: true},
			hasBatchProv:       true,
			hasMetadataUpdater: true,
			expectedBatch:      true,
		},
		{
			name:               "batch enabled without provider",
			metadata:           nil,
			config:             WorkerConfig{UseBatchAPI: true},
			hasBatchProv:       false,
			hasMetadataUpdater: true,
			expectedBatch:      false,
		},
		{
			name:               "batch enabled without updater",
			metadata:           nil,
			config:             WorkerConfig{UseBatchAPI: true},
			hasBatchProv:       true,
			hasMetadataUpdater: false,
			expectedBatch:      false,
		},
		{
			name:               "batch disabled",
			metadata:           nil,
			config:             WorkerConfig{UseBatchAPI: false},
			hasBatchProv:       true,
			hasMetadataUpdater: true,
			expectedBatch:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, ai := newSuccessfulMocks()
			var batchProv BatchProvider
			if tt.hasBatchProv {
				batchProv = &mockBatchProvider{}
			}
			var metaUpdater MetadataUpdater
			if tt.hasMetadataUpdater {
				metaUpdater = &mockMetadataUpdater{}
			}

			usecase := uc.NewGenerateSpecViewUseCase(repo, ai, "test-model")
			worker := &Worker{
				usecase:         usecase,
				config:          tt.config,
				batchProvider:   batchProv,
				metadataUpdater: metaUpdater,
			}

			// Create mock job with metadata
			job := &river.Job[Args]{
				JobRow: &rivertype.JobRow{
					Metadata: tt.metadata,
				},
				Args: Args{},
			}

			result := worker.isBatchMode(job)
			if result != tt.expectedBatch {
				t.Errorf("expected isBatchMode=%v, got %v", tt.expectedBatch, result)
			}
		})
	}
}

func TestWorker_SubmitBatchJob_Snooze(t *testing.T) {
	t.Run("should return JobSnooze on successful submit", func(t *testing.T) {
		repo, ai := newSuccessfulMocks()
		batchProvider := &mockBatchProvider{
			createJobFn: func(ctx context.Context, req batch.BatchRequest) (*batch.BatchResult, error) {
				return &batch.BatchResult{
					JobName: "batch-job-456",
					State:   batch.JobStatePending,
				}, nil
			},
		}
		config := WorkerConfig{
			UseBatchAPI:       true,
			BatchPollInterval: 30 * time.Second,
		}

		worker := newBatchTestWorker(repo, ai, batchProvider, config)
		job := newTestJob(Args{
			AnalysisID: "test-analysis",
			UserID:     "test-user",
		})

		err := worker.Work(context.Background(), job)

		var snoozeErr *rivertype.JobSnoozeError
		if !errors.As(err, &snoozeErr) {
			t.Errorf("expected JobSnoozeError, got %T: %v", err, err)
		}
		// Note: Batch state (JobName, Phase, Started) is now stored in job.Metadata via SQL,
		// not in job.Args. Since we don't have a real DB in unit tests, we cannot verify
		// the metadata update here. Integration tests should cover this.
	})

	t.Run("should return error when no test files", func(t *testing.T) {
		repo, ai := newSuccessfulMocks()
		repo.getTestDataByAnalysisIDFn = func(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
			return []specview.FileInfo{}, nil
		}
		batchProvider := &mockBatchProvider{}
		config := WorkerConfig{UseBatchAPI: true, BatchPollInterval: 30 * time.Second}

		worker := newBatchTestWorker(repo, ai, batchProvider, config)
		job := newTestJob(Args{
			AnalysisID: "empty-analysis",
			UserID:     "test-user",
			BatchPhase: BatchPhaseSubmit,
		})

		err := worker.Work(context.Background(), job)

		var cancelErr *rivertype.JobCancelError
		if !errors.As(err, &cancelErr) {
			t.Errorf("expected JobCancelError for empty files, got %T: %v", err, err)
		}
	})

	t.Run("should return cancel on analysis not found", func(t *testing.T) {
		repo, ai := newSuccessfulMocks()
		repo.getTestDataByAnalysisIDFn = func(ctx context.Context, analysisID string) ([]specview.FileInfo, error) {
			return nil, specview.ErrAnalysisNotFound
		}
		batchProvider := &mockBatchProvider{}
		config := WorkerConfig{UseBatchAPI: true, BatchPollInterval: 30 * time.Second}

		worker := newBatchTestWorker(repo, ai, batchProvider, config)
		job := newTestJob(Args{
			AnalysisID: "not-found",
			UserID:     "test-user",
			BatchPhase: BatchPhaseSubmit,
		})

		err := worker.Work(context.Background(), job)

		var cancelErr *rivertype.JobCancelError
		if !errors.As(err, &cancelErr) {
			t.Errorf("expected JobCancelError, got %T: %v", err, err)
		}
	})
}

func TestWorker_PollBatchJob(t *testing.T) {
	t.Run("poll succeeded - should complete job", func(t *testing.T) {
		repo, ai := newSuccessfulMocks()
		batchProvider := &mockBatchProvider{
			getJobStatusFn: func(ctx context.Context, jobName string) (*batch.BatchResult, error) {
				// Create a valid batch response with Phase1Output JSON
				phase1JSON := `{"domains":[{"name":"Test Domain","description":"Test description","confidence":0.9,"features":[{"name":"Feature 1","description":"Feature desc","confidence":0.85,"test_indices":[0]}]}]}`
				return &batch.BatchResult{
					JobName: jobName,
					State:   batch.JobStateSucceeded,
					Responses: []*genai.InlinedResponse{
						{
							Response: &genai.GenerateContentResponse{
								Candidates: []*genai.Candidate{
									{
										Content: &genai.Content{
											Parts: []*genai.Part{
												{Text: phase1JSON},
											},
										},
									},
								},
							},
						},
					},
					TokenUsage: &specview.TokenUsage{
						PromptTokens:     1000,
						CandidatesTokens: 500,
						TotalTokens:      1500,
					},
				}, nil
			},
		}
		config := WorkerConfig{UseBatchAPI: true, BatchPollInterval: 30 * time.Second}

		worker := newBatchTestWorker(repo, ai, batchProvider, config)
		job := newTestJobWithBatchMetadata(
			Args{AnalysisID: "test-analysis", UserID: "test-user"},
			"batch-job-789",
			time.Now().Add(-5*time.Minute),
		)

		err := worker.Work(context.Background(), job)

		if err != nil {
			t.Errorf("expected success (nil), got %v", err)
		}
	})

	t.Run("poll running - should re-snooze", func(t *testing.T) {
		repo, ai := newSuccessfulMocks()
		batchProvider := &mockBatchProvider{
			getJobStatusFn: func(ctx context.Context, jobName string) (*batch.BatchResult, error) {
				return &batch.BatchResult{
					JobName: jobName,
					State:   batch.JobStateRunning,
				}, nil
			},
		}
		config := WorkerConfig{UseBatchAPI: true, BatchPollInterval: 30 * time.Second}

		worker := newBatchTestWorker(repo, ai, batchProvider, config)
		job := newTestJobWithBatchMetadata(
			Args{AnalysisID: "test-analysis", UserID: "test-user"},
			"batch-job-running",
			time.Now().Add(-2*time.Minute),
		)

		err := worker.Work(context.Background(), job)

		var snoozeErr *rivertype.JobSnoozeError
		if !errors.As(err, &snoozeErr) {
			t.Errorf("expected JobSnoozeError for running job, got %T: %v", err, err)
		}
	})

	t.Run("poll failed - should return error", func(t *testing.T) {
		repo, ai := newSuccessfulMocks()
		batchProvider := &mockBatchProvider{
			getJobStatusFn: func(ctx context.Context, jobName string) (*batch.BatchResult, error) {
				return &batch.BatchResult{
					JobName: jobName,
					State:   batch.JobStateFailed,
					Error:   errors.New("internal batch error"),
				}, nil
			},
		}
		config := WorkerConfig{UseBatchAPI: true, BatchPollInterval: 30 * time.Second}

		worker := newBatchTestWorker(repo, ai, batchProvider, config)
		job := newTestJobWithBatchMetadata(
			Args{AnalysisID: "test-analysis", UserID: "test-user"},
			"batch-job-failed",
			time.Now().Add(-10*time.Minute),
		)

		err := worker.Work(context.Background(), job)

		if err == nil {
			t.Error("expected error for failed batch job, got nil")
		}

		// Should be retryable (not JobCancelError)
		var cancelErr *rivertype.JobCancelError
		if errors.As(err, &cancelErr) {
			t.Errorf("expected retryable error, got JobCancelError: %v", err)
		}
	})

	t.Run("poll expired - should return JobCancel", func(t *testing.T) {
		repo, ai := newSuccessfulMocks()
		batchProvider := &mockBatchProvider{
			getJobStatusFn: func(ctx context.Context, jobName string) (*batch.BatchResult, error) {
				return &batch.BatchResult{
					JobName: jobName,
					State:   batch.JobStateExpired,
				}, nil
			},
		}
		config := WorkerConfig{UseBatchAPI: true, BatchPollInterval: 30 * time.Second}

		worker := newBatchTestWorker(repo, ai, batchProvider, config)
		job := newTestJobWithBatchMetadata(
			Args{AnalysisID: "test-analysis", UserID: "test-user"},
			"batch-job-expired",
			time.Now().Add(-1*time.Hour),
		)

		err := worker.Work(context.Background(), job)

		var cancelErr *rivertype.JobCancelError
		if !errors.As(err, &cancelErr) {
			t.Errorf("expected JobCancelError for expired batch, got %T: %v", err, err)
		}
	})

	t.Run("poll cancelled - should return JobCancel", func(t *testing.T) {
		repo, ai := newSuccessfulMocks()
		batchProvider := &mockBatchProvider{
			getJobStatusFn: func(ctx context.Context, jobName string) (*batch.BatchResult, error) {
				return &batch.BatchResult{
					JobName: jobName,
					State:   batch.JobStateCancelled,
				}, nil
			},
		}
		config := WorkerConfig{UseBatchAPI: true, BatchPollInterval: 30 * time.Second}

		worker := newBatchTestWorker(repo, ai, batchProvider, config)
		job := newTestJobWithBatchMetadata(
			Args{AnalysisID: "test-analysis", UserID: "test-user"},
			"batch-job-cancelled",
			time.Now().Add(-1*time.Hour),
		)

		err := worker.Work(context.Background(), job)

		var cancelErr *rivertype.JobCancelError
		if !errors.As(err, &cancelErr) {
			t.Errorf("expected JobCancelError for cancelled batch, got %T: %v", err, err)
		}
	})

	t.Run("poll without batch_job_name - should cancel", func(t *testing.T) {
		repo, ai := newSuccessfulMocks()
		batchProvider := &mockBatchProvider{}
		config := WorkerConfig{UseBatchAPI: true, BatchPollInterval: 30 * time.Second}

		worker := newBatchTestWorker(repo, ai, batchProvider, config)
		// Create job with poll phase but empty job name in metadata
		job := &river.Job[Args]{
			JobRow: &rivertype.JobRow{
				ID:       1,
				Attempt:  1,
				Metadata: []byte(`{"sv_batch_phase":"poll","sv_batch_job_name":""}`),
			},
			Args: Args{AnalysisID: "test-analysis", UserID: "test-user"},
		}

		err := worker.Work(context.Background(), job)

		var cancelErr *rivertype.JobCancelError
		if !errors.As(err, &cancelErr) {
			t.Errorf("expected JobCancelError for missing batch_job_name, got %T: %v", err, err)
		}
	})

	t.Run("poll exceeded max wait time - should return error", func(t *testing.T) {
		repo, ai := newSuccessfulMocks()
		batchProvider := &mockBatchProvider{
			getJobStatusFn: func(ctx context.Context, jobName string) (*batch.BatchResult, error) {
				return &batch.BatchResult{
					JobName: jobName,
					State:   batch.JobStateRunning,
				}, nil
			},
		}
		config := WorkerConfig{UseBatchAPI: true, BatchPollInterval: 30 * time.Second}

		worker := newBatchTestWorker(repo, ai, batchProvider, config)
		job := newTestJobWithBatchMetadata(
			Args{AnalysisID: "test-analysis", UserID: "test-user"},
			"batch-job-timeout",
			time.Now().Add(-25*time.Hour), // exceeds 24h max
		)

		err := worker.Work(context.Background(), job)

		if err == nil {
			t.Error("expected error for exceeded max wait time, got nil")
		}
		if !errors.Is(err, nil) && err.Error() == "" {
			t.Errorf("expected non-empty error message, got empty")
		}

		// Should be retryable (not JobCancelError)
		var cancelErr *rivertype.JobCancelError
		if errors.As(err, &cancelErr) {
			t.Errorf("expected retryable error, got JobCancelError: %v", err)
		}
	})

	t.Run("poll succeeded but parse fails - should return error", func(t *testing.T) {
		repo, ai := newSuccessfulMocks()
		batchProvider := &mockBatchProvider{
			getJobStatusFn: func(ctx context.Context, jobName string) (*batch.BatchResult, error) {
				// Return empty responses that will fail parsing
				return &batch.BatchResult{
					JobName:   jobName,
					State:     batch.JobStateSucceeded,
					Responses: nil, // no responses - will fail parsing
				}, nil
			},
		}
		config := WorkerConfig{UseBatchAPI: true, BatchPollInterval: 30 * time.Second}

		worker := newBatchTestWorker(repo, ai, batchProvider, config)
		job := newTestJobWithBatchMetadata(
			Args{AnalysisID: "test-analysis", UserID: "test-user"},
			"batch-job-bad-response",
			time.Now().Add(-5*time.Minute),
		)

		err := worker.Work(context.Background(), job)

		if err == nil {
			t.Error("expected error for parse failure, got nil")
		}
	})
}
