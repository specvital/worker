package autorefresh

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/specvital/worker/internal/domain/analysis"
)

// Tuned for 1h cron interval:
// - 1 failure: network blip
// - 2 failures: transient issue
// - 3 failures: persistent problem, stop to prevent cascade
const maxConsecutiveEnqueueFailures = 3

var ErrCircuitBreakerOpen = errors.New("circuit breaker: too many consecutive enqueue failures")

type AutoRefreshUseCase struct {
	parserVersionProvider analysis.ParserVersionProvider
	repository            analysis.AutoRefreshRepository
	taskQueue             analysis.TaskQueue
	vcs                   analysis.VCS
}

func NewAutoRefreshUseCase(
	repository analysis.AutoRefreshRepository,
	taskQueue analysis.TaskQueue,
	vcs analysis.VCS,
	parserVersionProvider analysis.ParserVersionProvider,
) *AutoRefreshUseCase {
	return &AutoRefreshUseCase{
		parserVersionProvider: parserVersionProvider,
		repository:            repository,
		taskQueue:             taskQueue,
		vcs:                   vcs,
	}
}

// Returns ErrCircuitBreakerOpen if too many consecutive enqueue failures occur.
func (uc *AutoRefreshUseCase) Execute(ctx context.Context) error {
	codebases, err := uc.repository.GetCodebasesForAutoRefresh(ctx)
	if err != nil {
		return err
	}

	if len(codebases) == 0 {
		slog.InfoContext(ctx, "no codebases eligible for auto-refresh")
		return nil
	}

	currentParserVersion, err := uc.parserVersionProvider.GetCurrentParserVersion(ctx)
	if err != nil {
		slog.WarnContext(ctx, "failed to get current parser version, skipping version-based refresh",
			"error", err,
		)
		currentParserVersion = ""
	}

	now := time.Now()
	var enqueued int
	var consecutiveFailures int

	for _, codebase := range codebases {
		if consecutiveFailures >= maxConsecutiveEnqueueFailures {
			slog.ErrorContext(ctx, "circuit breaker open, aborting auto-refresh",
				"consecutive_failures", consecutiveFailures,
				"enqueued_before_abort", enqueued,
			)
			return fmt.Errorf("%w: %d failures", ErrCircuitBreakerOpen, consecutiveFailures)
		}

		if !analysis.ShouldRefreshAt(
			codebase.LastViewedAt,
			codebase.LastCompletedAt,
			codebase.ConsecutiveFailures,
			now,
		) {
			continue
		}

		repoURL := fmt.Sprintf("https://%s/%s/%s", codebase.Host, codebase.Owner, codebase.Name)
		commitInfo, err := uc.vcs.GetHeadCommit(ctx, repoURL, nil)
		if err != nil {
			consecutiveFailures++
			slog.ErrorContext(ctx, "failed to get head commit for auto-refresh",
				"owner", codebase.Owner,
				"repo", codebase.Name,
				"consecutive_failures", consecutiveFailures,
				"error", err,
			)
			continue
		}

		shouldEnqueue := uc.shouldEnqueueRefresh(codebase, commitInfo.SHA, currentParserVersion)
		if !shouldEnqueue {
			slog.DebugContext(ctx, "skipping auto-refresh: no changes detected",
				"owner", codebase.Owner,
				"repo", codebase.Name,
				"commit", commitInfo.SHA,
				"last_parser_version", codebase.LastParserVersion,
				"current_parser_version", currentParserVersion,
			)
			continue
		}

		if err := uc.taskQueue.EnqueueAnalysis(ctx, codebase.Owner, codebase.Name, commitInfo.SHA); err != nil {
			consecutiveFailures++
			slog.ErrorContext(ctx, "failed to enqueue auto-refresh task",
				"owner", codebase.Owner,
				"repo", codebase.Name,
				"consecutive_failures", consecutiveFailures,
				"error", err,
			)
			continue
		}

		consecutiveFailures = 0
		enqueued++
		slog.DebugContext(ctx, "enqueued auto-refresh task",
			"owner", codebase.Owner,
			"repo", codebase.Name,
			"reason", uc.refreshReason(codebase, commitInfo.SHA, currentParserVersion),
		)
	}

	slog.InfoContext(ctx, "auto-refresh execution completed",
		"total_candidates", len(codebases),
		"enqueued", enqueued,
	)

	return nil
}

func (uc *AutoRefreshUseCase) shouldEnqueueRefresh(
	codebase analysis.CodebaseRefreshInfo,
	headCommitSHA string,
	currentParserVersion string,
) bool {
	if codebase.LastCommitSHA != headCommitSHA {
		return true
	}

	if currentParserVersion != "" && codebase.LastParserVersion != currentParserVersion {
		return true
	}

	return false
}

func (uc *AutoRefreshUseCase) refreshReason(
	codebase analysis.CodebaseRefreshInfo,
	headCommitSHA string,
	currentParserVersion string,
) string {
	if codebase.LastCommitSHA != headCommitSHA {
		return "new_commit"
	}
	if currentParserVersion != "" && codebase.LastParserVersion != currentParserVersion {
		return "parser_version_changed"
	}
	return "unknown"
}
