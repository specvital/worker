package analyze

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/riverqueue/river"

	"github.com/specvital/worker/internal/domain/analysis"
	"github.com/specvital/worker/internal/domain/quota"
	uc "github.com/specvital/worker/internal/usecase/analysis"
)

const (
	// Queue names for analysis jobs (underscore required - River disallows colons)
	QueuePriority  = "analysis_priority"  // Pro/Enterprise tier users
	QueueDefault   = "analysis_default"   // Free tier users
	QueueScheduled = "analysis_scheduled" // Background/batch jobs

	maxRetryAttempts = 3
)

type AnalyzeArgs struct {
	CommitSHA string  `json:"commit_sha" river:"unique"`
	Owner     string  `json:"owner" river:"unique"`
	Repo      string  `json:"repo" river:"unique"`
	Tier      string  `json:"tier,omitempty"`
	UserID    *string `json:"user_id,omitempty"`
}

func (AnalyzeArgs) Kind() string { return "analysis:analyze" }

func (AnalyzeArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       QueueDefault,
		MaxAttempts: maxRetryAttempts,
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
		},
	}
}

type AnalyzeWorker struct {
	river.WorkerDefaults[AnalyzeArgs]
	analyzeUC *uc.AnalyzeUseCase
	quotaRepo quota.ReservationRepository
}

func NewAnalyzeWorker(analyzeUC *uc.AnalyzeUseCase, quotaRepo quota.ReservationRepository) *AnalyzeWorker {
	return &AnalyzeWorker{
		analyzeUC: analyzeUC,
		quotaRepo: quotaRepo,
	}
}

func (w *AnalyzeWorker) Timeout(job *river.Job[AnalyzeArgs]) time.Duration {
	return 5 * time.Minute // Match NeonDB idle_in_transaction_session_timeout (default 5min)
}

// Exponential backoff: 1st retry +1s, 2nd +4s, 3rd +9s
func (w *AnalyzeWorker) NextRetry(job *river.Job[AnalyzeArgs]) time.Time {
	attempt := job.Attempt
	backoff := time.Duration(attempt*attempt) * time.Second
	return time.Now().Add(backoff)
}

func (w *AnalyzeWorker) Work(ctx context.Context, job *river.Job[AnalyzeArgs]) error {
	args := job.Args

	// Release quota reservation on completion or final failure.
	defer quota.ReleaseReservation(w.quotaRepo, job.ID, "analyze")

	slog.InfoContext(ctx, "processing analyze task",
		"job_id", job.ID,
		"owner", args.Owner,
		"repo", args.Repo,
		"commit", args.CommitSHA,
	)

	req := analysis.AnalyzeRequest{
		Owner:     args.Owner,
		Repo:      args.Repo,
		CommitSHA: args.CommitSHA,
		UserID:    args.UserID,
	}

	if err := w.analyzeUC.Execute(ctx, req); err != nil {
		if errors.Is(err, analysis.ErrAlreadyCompleted) {
			slog.InfoContext(ctx, "analysis already completed, cancelling job",
				"job_id", job.ID,
				"owner", args.Owner,
				"repo", args.Repo,
				"commit", args.CommitSHA,
			)
			return river.JobCancel(err)
		}

		slog.ErrorContext(ctx, "analyze task failed",
			"job_id", job.ID,
			"owner", args.Owner,
			"repo", args.Repo,
			"commit", args.CommitSHA,
			"error", err,
		)
		return err
	}

	slog.InfoContext(ctx, "analyze task completed",
		"job_id", job.ID,
		"owner", args.Owner,
		"repo", args.Repo,
		"commit", args.CommitSHA,
	)

	return nil
}
