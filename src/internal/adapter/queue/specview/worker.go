package specview

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/riverqueue/river"

	"github.com/specvital/worker/internal/domain/specview"
	uc "github.com/specvital/worker/internal/usecase/specview"
)

const (
	jobKind          = "specview:generate"
	maxRetryAttempts = 3
	jobTimeout       = 10 * time.Minute
	initialBackoff   = 10 * time.Second
)

// Args represents the arguments for a spec-view generation job.
type Args struct {
	AnalysisID string `json:"analysis_id" river:"unique"`
	Language   string `json:"language" river:"unique"` // optional, defaults to "English"
	ModelID    string `json:"model_id,omitempty"`
}

// Kind returns the unique identifier for this job type.
func (Args) Kind() string { return jobKind }

// InsertOpts returns the River insert options for this job type.
func (Args) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: maxRetryAttempts,
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
		},
	}
}

// Worker processes spec-view generation jobs.
type Worker struct {
	river.WorkerDefaults[Args]
	usecase *uc.GenerateSpecViewUseCase
}

// NewWorker creates a new spec-view worker.
func NewWorker(usecase *uc.GenerateSpecViewUseCase) *Worker {
	return &Worker{usecase: usecase}
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
	args := job.Args

	// Default language to English if not specified
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

	if args.AnalysisID == "" {
		err := errors.New("analysis_id is required")
		slog.WarnContext(ctx, "invalid job arguments, cancelling",
			"job_id", job.ID,
			"error", err,
		)
		return river.JobCancel(err)
	}

	lang := specview.Language(language)

	req := specview.SpecViewRequest{
		AnalysisID: args.AnalysisID,
		Language:   lang,
		ModelID:    args.ModelID,
	}

	result, err := w.usecase.Execute(ctx, req)
	if err != nil {
		return w.handleError(ctx, job, err)
	}

	logFields := []any{
		"job_id", job.ID,
		"analysis_id", args.AnalysisID,
		"document_id", result.DocumentID,
		"cache_hit", result.CacheHit,
	}

	slog.InfoContext(ctx, "specview generation task completed", logFields...)

	return nil
}


func (w *Worker) handleError(ctx context.Context, job *river.Job[Args], err error) error {
	args := job.Args

	if isPermanentError(err) {
		slog.WarnContext(ctx, "permanent error, cancelling job",
			"job_id", job.ID,
			"analysis_id", args.AnalysisID,
			"error", err,
		)
		return river.JobCancel(err)
	}

	slog.ErrorContext(ctx, "specview generation task failed",
		"job_id", job.ID,
		"analysis_id", args.AnalysisID,
		"attempt", job.Attempt,
		"error", err,
	)

	return err
}

func isPermanentError(err error) bool {
	return errors.Is(err, specview.ErrAnalysisNotFound) ||
		errors.Is(err, specview.ErrInvalidInput)
}
