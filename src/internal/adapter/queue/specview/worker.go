package specview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/specvital/worker/internal/adapter/ai/gemini/batch"
	"github.com/specvital/worker/internal/domain/specview"
	uc "github.com/specvital/worker/internal/usecase/specview"
)

// Metadata keys for batch state (sv_ prefix to avoid conflicts with River internals)
const (
	metaKeyBatchJobName = "sv_batch_job_name"
	metaKeyBatchPhase   = "sv_batch_phase"
	metaKeyBatchStarted = "sv_batch_started"
)

// batchMetadata holds batch-related state stored in River job metadata.
type batchMetadata struct {
	JobName string    `json:"sv_batch_job_name,omitempty"`
	Phase   string    `json:"sv_batch_phase,omitempty"`
	Started time.Time `json:"sv_batch_started,omitempty"`
}

// parseBatchMetadata extracts batch state from job metadata.
// Returns empty metadata on parse errors (graceful degradation for non-batch jobs).
func parseBatchMetadata(raw []byte) *batchMetadata {
	if len(raw) == 0 {
		return &batchMetadata{}
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		// Metadata may contain non-JSON or River internal data; treat as empty batch state
		return &batchMetadata{}
	}

	meta := &batchMetadata{}
	if v, ok := m[metaKeyBatchJobName].(string); ok {
		meta.JobName = v
	}
	if v, ok := m[metaKeyBatchPhase].(string); ok {
		meta.Phase = v
	}
	if v, ok := m[metaKeyBatchStarted].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			meta.Started = t
		}
	}
	return meta
}

const (
	// Queue names for specview jobs (underscore required - River disallows colons)
	QueuePriority  = "specview_priority"  // Pro/Enterprise tier users
	QueueDefault   = "specview_default"   // Free tier users
	QueueScheduled = "specview_scheduled" // Scheduler/cron jobs

	jobKind          = "specview:generate"
	maxRetryAttempts = 3
	jobTimeout       = 35 * time.Minute
	initialBackoff   = 10 * time.Second
)

// BatchPhase represents the current phase of a batch job.
type BatchPhase string

const (
	BatchPhaseSubmit BatchPhase = "submit"
	BatchPhasePoll   BatchPhase = "poll"
)

// Args represents the arguments for a spec-view generation job.
type Args struct {
	AnalysisID      string     `json:"analysis_id" river:"unique"`
	ForceRegenerate bool       `json:"force_regenerate,omitempty"` // skip cache and create new version
	Language        string     `json:"language" river:"unique"`    // optional, defaults to "English"
	ModelID         string     `json:"model_id,omitempty"`
	Tier            string     `json:"tier,omitempty"`
	UserID          string     `json:"user_id" river:"unique"` // required: document owner

	// Deprecated: Batch state is now stored in job.Metadata, not Args.
	// Kept for backward compatibility with jobs enqueued before the metadata migration.
	// These fields are no longer read; batch state is retrieved via parseBatchMetadata().
	BatchJobName string     `json:"batch_job_name,omitempty"`
	BatchPhase   BatchPhase `json:"batch_phase,omitempty"`
	BatchStarted time.Time  `json:"batch_started,omitempty"`
}

// Kind returns the unique identifier for this job type.
func (Args) Kind() string { return jobKind }

// InsertOpts returns the River insert options for this job type.
func (Args) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       QueueDefault,
		MaxAttempts: maxRetryAttempts,
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
		},
	}
}

// BatchProvider defines the interface for batch job operations.
// Used for dependency injection to allow testing without real Gemini API.
type BatchProvider interface {
	CreateJob(ctx context.Context, req batch.BatchRequest) (*batch.BatchResult, error)
	CreateClassificationJob(input specview.Phase1Input) (batch.BatchRequest, error)
	GetJobStatus(ctx context.Context, jobName string) (*batch.BatchResult, error)
}

// Batch mode constraints
const (
	// maxBatchWaitTime is the maximum time to wait for a batch job to complete.
	// Gemini batch jobs typically complete within a few hours, 24h is a safe upper bound.
	maxBatchWaitTime = 24 * time.Hour
)

// WorkerConfig holds configuration for the spec-view worker.
type WorkerConfig struct {
	BatchPollInterval time.Duration
	UseBatchAPI       bool
}

// MetadataUpdater abstracts the database operation for updating job metadata.
// This allows testing without a real database connection.
type MetadataUpdater interface {
	UpdateBatchMetadata(ctx context.Context, jobID int64, jobName string, started time.Time) error
}

// Worker processes spec-view generation jobs.
type Worker struct {
	river.WorkerDefaults[Args]
	batchProvider   BatchProvider
	config          WorkerConfig
	metadataUpdater MetadataUpdater
	repository      specview.Repository
	usecase         *uc.GenerateSpecViewUseCase
}

// NewWorker creates a new spec-view worker.
func NewWorker(usecase *uc.GenerateSpecViewUseCase) *Worker {
	return &Worker{usecase: usecase}
}

// NewWorkerWithBatch creates a spec-view worker with Batch API support.
func NewWorkerWithBatch(
	usecase *uc.GenerateSpecViewUseCase,
	batchProvider BatchProvider,
	repository specview.Repository,
	dbPool *pgxpool.Pool,
	config WorkerConfig,
) *Worker {
	var metadataUpdater MetadataUpdater
	if dbPool != nil {
		metadataUpdater = &pgxMetadataUpdater{pool: dbPool}
	}
	return &Worker{
		batchProvider:   batchProvider,
		config:          config,
		metadataUpdater: metadataUpdater,
		repository:      repository,
		usecase:         usecase,
	}
}

// pgxMetadataUpdater implements MetadataUpdater using pgxpool.
type pgxMetadataUpdater struct {
	pool *pgxpool.Pool
}

func (u *pgxMetadataUpdater) UpdateBatchMetadata(ctx context.Context, jobID int64, jobName string, started time.Time) error {
	metaJSON, err := json.Marshal(map[string]string{
		metaKeyBatchJobName: jobName,
		metaKeyBatchPhase:   string(BatchPhasePoll),
		metaKeyBatchStarted: started.Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("marshal batch metadata: %w", err)
	}

	query := `
		UPDATE river_job
		SET metadata = COALESCE(metadata, '{}'::jsonb) || $1::jsonb
		WHERE id = $2
	`
	result, err := u.pool.Exec(ctx, query, metaJSON, jobID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("job not found: %d", jobID)
	}
	return nil
}

// Timeout returns the maximum duration for this job.
func (w *Worker) Timeout(job *river.Job[Args]) time.Duration {
	return jobTimeout
}

// NextRetry returns the next retry time with exponential backoff.
// Backoff: 10s, 40s, 90s (attempt² × 10s)
func (w *Worker) NextRetry(job *river.Job[Args]) time.Time {
	attempt := job.Attempt
	backoff := time.Duration(attempt*attempt) * initialBackoff
	return time.Now().Add(backoff)
}

// Work processes a spec-view generation job.
func (w *Worker) Work(ctx context.Context, job *river.Job[Args]) error {
	// Validate required fields
	if err := w.validateArgs(ctx, job); err != nil {
		return err
	}

	// Route to batch mode if enabled and in batch phase
	if w.isBatchMode(job) {
		return w.workBatch(ctx, job)
	}

	// Standard mode
	return w.workStandard(ctx, job)
}

// workStandard executes the standard (non-batch) processing flow.
func (w *Worker) workStandard(ctx context.Context, job *river.Job[Args]) error {
	startTime := time.Now()
	args := job.Args

	language := args.Language
	if language == "" {
		language = "English"
	}

	slog.InfoContext(ctx, "processing specview generation task",
		"job_id", job.ID,
		"analysis_id", args.AnalysisID,
		"language", language,
		"model_id", args.ModelID,
		"attempt", job.Attempt,
	)

	lang := specview.Language(language)

	req := specview.SpecViewRequest{
		AnalysisID:      args.AnalysisID,
		ForceRegenerate: args.ForceRegenerate,
		Language:        lang,
		ModelID:         args.ModelID,
		UserID:          args.UserID,
	}

	result, err := w.usecase.Execute(ctx, req)
	if err != nil {
		return w.handleError(ctx, job, err)
	}

	durationMs := time.Since(startTime).Milliseconds()
	logFields := []any{
		"job_id", job.ID,
		"analysis_id", args.AnalysisID,
		"document_id", result.DocumentID,
		"cache_hit", result.CacheHit,
		"duration_ms", durationMs,
	}
	if result.AnalysisContext != nil {
		logFields = append(logFields,
			"host", result.AnalysisContext.Host,
			"owner", result.AnalysisContext.Owner,
			"repo", result.AnalysisContext.Repo,
		)
	}

	slog.InfoContext(ctx, "specview generation task completed", logFields...)

	return nil
}

// workBatch handles batch mode job processing (submit or poll phase).
func (w *Worker) workBatch(ctx context.Context, job *river.Job[Args]) error {
	meta := parseBatchMetadata(job.Metadata)

	switch BatchPhase(meta.Phase) {
	case BatchPhasePoll:
		return w.pollBatchJob(ctx, job, meta)
	default:
		// Initial batch mode entry - start with submit phase
		return w.submitBatchJob(ctx, job)
	}
}

// submitBatchJob creates a batch job and returns JobSnooze for polling.
func (w *Worker) submitBatchJob(ctx context.Context, job *river.Job[Args]) error {
	args := job.Args
	language := args.Language
	if language == "" {
		language = "English"
	}

	slog.InfoContext(ctx, "submitting batch job",
		"job_id", job.ID,
		"analysis_id", args.AnalysisID,
		"language", language,
	)

	// Load test data for batch job
	files, err := w.repository.GetTestDataByAnalysisID(ctx, args.AnalysisID)
	if err != nil {
		if errors.Is(err, specview.ErrAnalysisNotFound) {
			return river.JobCancel(err)
		}
		return fmt.Errorf("load test data: %w", err)
	}

	if len(files) == 0 {
		return river.JobCancel(fmt.Errorf("no test files found for analysis %s", args.AnalysisID))
	}

	// Create batch request
	input := specview.Phase1Input{
		AnalysisID: args.AnalysisID,
		Files:      files,
		Language:   specview.Language(language),
	}

	batchReq, err := w.batchProvider.CreateClassificationJob(input)
	if err != nil {
		return fmt.Errorf("create classification job: %w", err)
	}

	// Submit batch job
	result, err := w.batchProvider.CreateJob(ctx, batchReq)
	if err != nil {
		return fmt.Errorf("submit batch job: %w", err)
	}

	// Persist batch state to job metadata via SQL
	// River doesn't persist job.Args changes on snooze, so we use metadata instead
	if err := w.metadataUpdater.UpdateBatchMetadata(ctx, job.ID, result.JobName, time.Now()); err != nil {
		return fmt.Errorf("update batch metadata: %w", err)
	}

	slog.InfoContext(ctx, "batch job submitted, snoozing for poll",
		"job_id", job.ID,
		"analysis_id", args.AnalysisID,
		"batch_job_name", result.JobName,
		"poll_interval", w.config.BatchPollInterval,
	)

	return river.JobSnooze(w.config.BatchPollInterval)
}

// pollBatchJob checks batch job status and completes, re-snoozes, or fails.
func (w *Worker) pollBatchJob(ctx context.Context, job *river.Job[Args], meta *batchMetadata) error {
	args := job.Args

	if meta.JobName == "" {
		return river.JobCancel(errors.New("batch_job_name is required for poll phase"))
	}

	elapsedTime := time.Since(meta.Started)

	// Check maximum wait time to prevent infinite polling
	if elapsedTime > maxBatchWaitTime {
		slog.ErrorContext(ctx, "batch job exceeded maximum wait time",
			"job_id", job.ID,
			"analysis_id", args.AnalysisID,
			"batch_job_name", meta.JobName,
			"elapsed_time", elapsedTime.Round(time.Second),
			"max_wait_time", maxBatchWaitTime,
		)
		return fmt.Errorf("batch job exceeded maximum wait time of %v", maxBatchWaitTime)
	}

	slog.InfoContext(ctx, "polling batch job status",
		"job_id", job.ID,
		"analysis_id", args.AnalysisID,
		"batch_job_name", meta.JobName,
		"elapsed_time", elapsedTime.Round(time.Second),
	)

	result, err := w.batchProvider.GetJobStatus(ctx, meta.JobName)
	if err != nil {
		return fmt.Errorf("get batch job status: %w", err)
	}

	switch result.State {
	case batch.JobStateSucceeded:
		return w.completeBatchJob(ctx, job, result, meta)

	case batch.JobStateFailed:
		slog.ErrorContext(ctx, "batch job failed",
			"job_id", job.ID,
			"analysis_id", args.AnalysisID,
			"batch_job_name", meta.JobName,
			"error", result.Error,
			"elapsed_time", elapsedTime.Round(time.Second),
		)
		if result.Error != nil {
			return fmt.Errorf("batch job failed: %w", result.Error)
		}
		return errors.New("batch job failed")

	case batch.JobStateExpired:
		slog.WarnContext(ctx, "batch job expired",
			"job_id", job.ID,
			"analysis_id", args.AnalysisID,
			"batch_job_name", meta.JobName,
			"elapsed_time", elapsedTime.Round(time.Second),
		)
		return river.JobCancel(batch.ErrJobExpired)

	case batch.JobStateCancelled:
		slog.WarnContext(ctx, "batch job cancelled",
			"job_id", job.ID,
			"analysis_id", args.AnalysisID,
			"batch_job_name", meta.JobName,
		)
		return river.JobCancel(batch.ErrJobCancelled)

	default:
		// Still running (pending/running) - snooze again
		slog.InfoContext(ctx, "batch job still running, re-snoozing",
			"job_id", job.ID,
			"analysis_id", args.AnalysisID,
			"batch_job_name", meta.JobName,
			"state", result.State,
			"elapsed_time", elapsedTime.Round(time.Second),
		)
		return river.JobSnooze(w.config.BatchPollInterval)
	}
}

// completeBatchJob processes the completed batch result and generates the document.
func (w *Worker) completeBatchJob(ctx context.Context, job *river.Job[Args], result *batch.BatchResult, meta *batchMetadata) error {
	args := job.Args
	elapsedTime := time.Since(meta.Started)

	slog.InfoContext(ctx, "batch job completed, parsing results",
		"job_id", job.ID,
		"analysis_id", args.AnalysisID,
		"batch_job_name", meta.JobName,
		"elapsed_time", elapsedTime.Round(time.Second),
	)

	// Parse batch result
	classificationResult, err := batch.ParseClassificationResponse(result)
	if err != nil {
		return fmt.Errorf("parse batch result: %w", err)
	}

	if classificationResult.Output == nil {
		return errors.New("batch: parsed result has nil output")
	}

	// Log token usage from batch job
	if classificationResult.TokenUsage != nil {
		slog.InfoContext(ctx, "batch job token usage",
			"job_id", job.ID,
			"analysis_id", args.AnalysisID,
			"prompt_tokens", classificationResult.TokenUsage.PromptTokens,
			"candidates_tokens", classificationResult.TokenUsage.CandidatesTokens,
			"total_tokens", classificationResult.TokenUsage.TotalTokens,
		)
	}

	// Build request for Phase 2/3 processing
	language := args.Language
	if language == "" {
		language = "English"
	}

	req := specview.SpecViewRequest{
		AnalysisID:      args.AnalysisID,
		ForceRegenerate: args.ForceRegenerate,
		Language:        specview.Language(language),
		ModelID:         args.ModelID,
		UserID:          args.UserID,
	}

	// Execute Phase 2 and 3 using the batch-classified domains
	ucResult, err := w.usecase.ExecutePhase2And3FromBatch(ctx, req, classificationResult.Output)
	if err != nil {
		return w.handleError(ctx, job, err)
	}

	slog.InfoContext(ctx, "batch mode document generation completed",
		"job_id", job.ID,
		"analysis_id", args.AnalysisID,
		"document_id", ucResult.DocumentID,
		"domain_count", len(classificationResult.Output.Domains),
		"batch_elapsed_time", elapsedTime.Round(time.Second),
	)

	return nil
}

// validateArgs validates required job arguments.
func (w *Worker) validateArgs(ctx context.Context, job *river.Job[Args]) error {
	args := job.Args

	if args.AnalysisID == "" {
		err := errors.New("analysis_id is required")
		slog.WarnContext(ctx, "invalid job arguments, cancelling",
			"job_id", job.ID,
			"error", err,
		)
		return river.JobCancel(err)
	}

	if args.UserID == "" {
		err := errors.New("user_id is required")
		slog.WarnContext(ctx, "invalid job arguments, cancelling",
			"job_id", job.ID,
			"error", err,
		)
		return river.JobCancel(err)
	}

	return nil
}

// isBatchMode determines if the job should use batch mode.
func (w *Worker) isBatchMode(job *river.Job[Args]) bool {
	// Check metadata for batch phase (persisted state)
	meta := parseBatchMetadata(job.Metadata)
	if meta.Phase == string(BatchPhasePoll) {
		return true
	}

	// Batch mode if:
	// 1. Batch API is enabled in config
	// 2. BatchProvider is available
	// 3. MetadataUpdater is available (for persisting batch state)
	return w.config.UseBatchAPI && w.batchProvider != nil && w.metadataUpdater != nil
}

func (w *Worker) handleError(ctx context.Context, job *river.Job[Args], err error) error {
	args := job.Args

	if isPermanentError(err) {
		slog.WarnContext(ctx, "permanent error, cancelling job",
			"job_id", job.ID,
			"analysis_id", args.AnalysisID,
			"attempt", job.Attempt,
			"max_attempts", maxRetryAttempts,
			"will_retry", false,
			"error", err,
		)
		return river.JobCancel(err)
	}

	willRetry := job.Attempt < maxRetryAttempts
	slog.ErrorContext(ctx, "specview generation task failed",
		"job_id", job.ID,
		"analysis_id", args.AnalysisID,
		"attempt", job.Attempt,
		"max_attempts", maxRetryAttempts,
		"will_retry", willRetry,
		"error", err,
	)

	return err
}

func isPermanentError(err error) bool {
	return errors.Is(err, specview.ErrAnalysisNotFound) ||
		errors.Is(err, specview.ErrInvalidInput)
}
